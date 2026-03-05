package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"study-guide/src/internal/store"
)

type sessionsView int

const (
	sessionsViewBrowse sessionsView = iota
	sessionsViewCreate
)

type browseEntryKind int

const (
	browseEntrySession browseEntryKind = iota
	browseEntryEmpty
)

type browseEntry struct {
	kind   browseEntryKind
	record sessionRecord
}

type sessionsSwitchboardModel struct {
	root     string
	protocol store.Protocol

	view sessionsView

	table  table.Model
	filter textinput.Model
	list   list.Model

	browseRecords []sessionRecord
	browseEntries []browseEntry

	createLookup      map[string]string
	armedSessionSlug  string
	subjects          []store.Subject
	selectedBySubject map[string]bool

	message string
	err     error
	width   int
}

var subtleTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

const sessionsCreateInfoText = "select one or more subjects, then choose Create; esc to cancel"

func sessionsNextStepTextStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
}

func newSessionsFilterInput() textinput.Model {
	fi := textinput.New()
	fi.Prompt = " filter: "
	fi.Placeholder = "by subject or slug"
	fi.CharLimit = 120
	fi.Width = 60
	fi.Focus()
	return fi
}

const (
	// Approximate 15% luminance delta from pure white/black for selection tint.
	sessionsSelectedRowBgLight = "#d9dcef"
	sessionsSelectedRowBgDark  = "#262b3a"
)

func sessionsSelectedRowStyle(base lipgloss.Style) lipgloss.Style {
	return base.
		Foreground(lipgloss.NoColor{}).
		Background(lipgloss.AdaptiveColor{
			Light: sessionsSelectedRowBgLight,
			Dark:  sessionsSelectedRowBgDark,
		}).
		Reverse(false).
		Bold(false)
}

func runSessionsSwitchboard(root string, protocol store.Protocol) error {
	m, err := newSessionsSwitchboardModel(root, protocol)
	if err != nil {
		return err
	}
	res, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	out := res.(sessionsSwitchboardModel)
	return out.err
}

func newSessionsSwitchboardModel(root string, protocol store.Protocol) (sessionsSwitchboardModel, error) {
	tbl := table.New(
		table.WithColumns([]table.Column{
			{Title: "SLUG", Width: 24},
			{Title: "SUBJECT", Width: 30},
			{Title: "STEP", Width: 24},
			{Title: "NEXT STEP", Width: 52},
		}),
		table.WithRows(nil),
		table.WithFocused(true),
		table.WithHeight(14),
	)
	tblStyles := table.DefaultStyles()
	tblStyles.Selected = sessionsSelectedRowStyle(tblStyles.Selected)
	tbl.SetStyles(tblStyles)
	fi := newSessionsFilterInput()

	createList := list.New([]list.Item{}, list.NewDefaultDelegate(), 100, 18)
	createList.SetShowHelp(false)
	createList.SetShowPagination(false)
	createList.SetShowStatusBar(false)

	m := sessionsSwitchboardModel{
		root:              root,
		protocol:          protocol,
		view:              sessionsViewBrowse,
		table:             tbl,
		filter:            fi,
		list:              createList,
		selectedBySubject: map[string]bool{},
		width:             120,
	}
	m.applyBrowseTableLayout()
	if err := m.refreshBrowseList(); err != nil {
		return sessionsSwitchboardModel{}, err
	}
	return m, nil
}

func (m sessionsSwitchboardModel) Init() tea.Cmd { return textinput.Blink }

func (m sessionsSwitchboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(msg.Width-2, 60)
		m.applyBrowseTableLayout()
		m.list.SetSize(max(msg.Width-2, 60), max(msg.Height-8, 8))
		m.filter.Width = max(msg.Width-18, 20)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+n", "shift+enter", "shift+return":
			if m.view == sessionsViewBrowse {
				subs, err := store.ListSubjects()
				if err != nil {
					m.err = err
					return m, tea.Quit
				}
				m.subjects = subs
				m.selectedBySubject = map[string]bool{}
				m.refreshCreateList()
				m.message = ""
				return m, nil
			}
		case "esc":
			switch m.view {
			case sessionsViewBrowse:
				if m.armedSessionSlug != "" {
					m.armedSessionSlug = ""
					m.applyBrowseEntries()
					m.message = ""
					return m, nil
				}
				return m, tea.Quit
			case sessionsViewCreate:
				if err := m.refreshBrowseList(); err != nil {
					m.err = err
					return m, tea.Quit
				}
				return m, nil
			}
		case "enter":
			if m.view == sessionsViewBrowse {
				return m.handleBrowseEnter()
			}
			if m.view == sessionsViewCreate {
				return m.handleCreateEnter()
			}
		}
	}

	if m.view == sessionsViewBrowse {
		oldFilter := m.filter.Value()
		var cmdFilter tea.Cmd
		m.filter, cmdFilter = m.filter.Update(msg)
		if m.filter.Value() != oldFilter {
			m.applyBrowseEntries()
		}
		var cmdTable tea.Cmd
		m.table, cmdTable = m.table.Update(msg)
		return m, tea.Batch(cmdFilter, cmdTable)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m sessionsSwitchboardModel) View() string {
	if m.view == sessionsViewCreate {
		var b strings.Builder
		header := m.list.Styles.TitleBar.Render(m.list.Styles.Title.Render("Create Session"))
		b.WriteString(header)
		b.WriteString("\n")
		b.WriteString(subtleTextStyle.Render(sessionsCreateInfoText))
		b.WriteString("\n")
		b.WriteString(m.list.View())
		if strings.TrimSpace(m.message) != "" {
			b.WriteString("\n")
			b.WriteString(subtleTextStyle.Render(m.message))
		}
		return b.String()
	}

	current := "current step: -"
	if entry, ok := m.selectedBrowseEntry(); ok && entry.kind == browseEntrySession {
		if entry.record.CurrentStep != "" {
			current = "current step: " + entry.record.CurrentStep
		} else if entry.record.NextAction == "start" && entry.record.NextStep != "" {
			current = "current step: (not started)"
		}
	}

	var b strings.Builder
	b.WriteString(m.filter.View())
	b.WriteString("\n")
	b.WriteString(m.table.View())
	b.WriteString("\n")
	b.WriteString(subtleTextStyle.Render(current))
	if strings.TrimSpace(m.message) != "" {
		b.WriteString("\n")
		b.WriteString(subtleTextStyle.Render(m.message))
	}
	if m.armedSessionSlug != "" {
		b.WriteString("\n")
		b.WriteString(subtleTextStyle.Render("esc to cancel"))
	} else {
		b.WriteString("\n")
		b.WriteString(subtleTextStyle.Render("ctrl+n to create new; esc to quit"))
	}
	return b.String()
}

func (m sessionsSwitchboardModel) handleBrowseEnter() (tea.Model, tea.Cmd) {
	entry, ok := m.selectedBrowseEntry()
	if !ok {
		return m, nil
	}

	switch entry.kind {
	case browseEntryEmpty:
		return m, nil
	case browseEntrySession:
		rec := entry.record
		if rec.NextAction == "invalid" {
			m.message = "invalid: " + rec.InvalidReason
			return m, nil
		}
		if m.armedSessionSlug != rec.Slug {
			m.armedSessionSlug = rec.Slug
			m.applyBrowseEntries()
			m.message = ""
			return m, nil
		}
		res, err := advanceSessionOnce(m.root, rec.Slug, m.protocol)
		if err != nil {
			m.message = "advance failed: " + err.Error()
			return m, nil
		}
		m.armedSessionSlug = ""
		if err := m.refreshBrowseList(); err != nil {
			m.err = err
			return m, tea.Quit
		}
		m.message = fmt.Sprintf("session=%s state=%s step=%s", rec.Slug, res.State, res.StepSlug)
		return m, nil
	default:
		return m, nil
	}
}

func (m sessionsSwitchboardModel) handleCreateEnter() (tea.Model, tea.Cmd) {
	it, ok := m.list.SelectedItem().(listItem)
	if !ok {
		return m, nil
	}
	choice := string(it)
	switch token := m.createLookup[choice]; token {
	case "create":
		selected := m.selectedSubjects()
		if len(selected) == 0 {
			m.message = "select at least one subject before Create"
			return m, nil
		}
		slug, _, err := createSessionScaffold(m.root, selected)
		if err != nil {
			m.message = "create failed: " + err.Error()
			return m, nil
		}
		if err := m.refreshBrowseList(); err != nil {
			m.err = err
			return m, tea.Quit
		}
		m.message = "session created: " + slug
		return m, nil
	case "":
		return m, nil
	default:
		uid := strings.TrimPrefix(token, "subject:")
		m.selectedBySubject[uid] = !m.selectedBySubject[uid]
		m.refreshCreateList()
		m.message = fmt.Sprintf("selected subjects: %d", len(m.selectedSubjects()))
		return m, nil
	}
}

func (m *sessionsSwitchboardModel) refreshBrowseList() error {
	m.view = sessionsViewBrowse
	subjects, err := store.ListSubjects()
	if err != nil {
		return err
	}
	subjectByID := make(map[string]store.Subject, len(subjects))
	for _, s := range subjects {
		subjectByID[s.UUID] = s
	}
	records, err := loadSessionRecords(m.root, m.protocol, subjectByID)
	if err != nil {
		return err
	}
	m.browseRecords = m.browseRecords[:0]
	for _, r := range records {
		if !r.Complete {
			m.browseRecords = append(m.browseRecords, r)
		}
	}
	m.filter.Focus()
	m.applyBrowseEntries()
	return nil
}

func (m *sessionsSwitchboardModel) applyBrowseEntries() {
	selectedSlug := ""
	if entry, ok := m.selectedBrowseEntry(); ok && entry.kind == browseEntrySession {
		selectedSlug = entry.record.Slug
	}

	query := strings.TrimSpace(m.filter.Value())
	filtered := m.browseRecords
	if query != "" {
		hay := make([]string, len(m.browseRecords))
		for i, r := range m.browseRecords {
			hay[i] = strings.ToLower(r.Slug + " " + strings.Join(r.SubjectNames, " "))
		}
		matches := fuzzy.Find(strings.ToLower(query), hay)
		filtered = make([]sessionRecord, 0, len(matches))
		for _, match := range matches {
			filtered = append(filtered, m.browseRecords[match.Index])
		}
	}

	entries := make([]browseEntry, 0, len(filtered)+1)
	for _, r := range filtered {
		entries = append(entries, browseEntry{kind: browseEntrySession, record: r})
	}
	if len(filtered) == 0 {
		entries = append(entries, browseEntry{kind: browseEntryEmpty})
	}
	m.browseEntries = entries

	rows := make([]table.Row, 0, len(entries))
	targetCursor := 0
	for i, e := range entries {
		slug, subject, step, nextStep := m.renderEntryRow(e)
		rows = append(rows, table.Row{slug, subject, step, nextStep})
		if e.kind == browseEntrySession && e.record.Slug == selectedSlug {
			targetCursor = i
		}
	}
	m.table.SetRows(rows)
	if targetCursor >= 0 && targetCursor < len(rows) {
		m.table.SetCursor(targetCursor)
	} else {
		m.table.SetCursor(0)
	}
}

func (m *sessionsSwitchboardModel) applyBrowseTableLayout() {
	total := max(m.width, 60)
	// Keep slug/subject/step readable and give the remainder to NEXT STEP.
	slugW := 35
	subjectW := 35
	stepW := 48
	// Account for separators/padding inside the table renderer.
	overhead := 12
	nextW := total - slugW - subjectW - stepW - overhead
	if nextW < 32 {
		nextW = 32
	}
	m.table.SetColumns([]table.Column{
		{Title: "SLUG", Width: slugW},
		{Title: "SUBJECT", Width: subjectW},
		{Title: "STEP", Width: stepW},
		{Title: "NEXT STEP", Width: nextW},
	})
	m.table.SetWidth(total)
}

func (m sessionsSwitchboardModel) renderEntryRow(e browseEntry) (string, string, string, string) {
	switch e.kind {
	case browseEntryEmpty:
		return "no active sessions", "", "", ""
	case browseEntrySession:
		rec := e.record
		subjectText := strings.Join(rec.SubjectNames, ", ")
		if subjectText == "" {
			subjectText = "(unknown subjects)"
		}
		current := rec.CurrentStep
		if current == "" {
			current = "-"
		}
		stepNum := rec.ProgressSteps
		if stepNum < 0 {
			stepNum = 0
		}
		if rec.StepCount <= 0 {
			rec.StepCount = len(m.protocol.Steps)
		}
		if rec.StepCount <= 0 {
			rec.StepCount = 1
		}
		if stepNum > rec.StepCount {
			stepNum = rec.StepCount
		}
		stepText := fmt.Sprintf("[%d/%d] %s", stepNum, rec.StepCount, current)
		next := rec.NextStep
		if strings.TrimSpace(next) == "" {
			next = "-"
		}
		nextText := next
		if rec.Slug == m.armedSessionSlug {
			if next != "-" {
				nextText = next + " (enter to advance)"
			}
			return rec.Slug, subjectText, stepText, lipgloss.NewStyle().
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("62")).
				Bold(true).
				Padding(0, 1).
				Render(nextText)
		}
		return rec.Slug, subjectText, stepText, sessionsNextStepTextStyle().Render(nextText)
	default:
		return "", "", "", ""
	}
}

func (m sessionsSwitchboardModel) selectedBrowseEntry() (browseEntry, bool) {
	if m.view != sessionsViewBrowse {
		return browseEntry{}, false
	}
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.browseEntries) {
		return browseEntry{}, false
	}
	return m.browseEntries[idx], true
}

func (m *sessionsSwitchboardModel) refreshCreateList() {
	items := make([]list.Item, 0, len(m.subjects)+1)
	m.createLookup = map[string]string{}
	if len(m.subjects) == 0 {
		items = append(items, listItem("No subjects available"))
	} else {
		for _, s := range m.subjects {
			marker := "[ ]"
			if m.selectedBySubject[s.UUID] {
				marker = "[x]"
			}
			label := fmt.Sprintf("%s %s (%s)", marker, s.Name, strings.Split(s.UUID, "-")[0])
			items = append(items, listItem(label))
			m.createLookup[label] = "subject:" + s.UUID
		}
	}
	items = append(items, listItem("Create"))
	m.createLookup["Create"] = "create"

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	m.list = list.New(items, delegate, 100, 18)
	m.list.Title = "Create Session"
	m.list.SetShowTitle(false)
	m.list.SetShowHelp(false)
	m.list.SetShowStatusBar(false)
	m.list.SetShowPagination(false)
	m.view = sessionsViewCreate
}

func (m *sessionsSwitchboardModel) selectedSubjects() []store.Subject {
	out := make([]store.Subject, 0, len(m.subjects))
	for _, s := range m.subjects {
		if m.selectedBySubject[s.UUID] {
			out = append(out, s)
		}
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
