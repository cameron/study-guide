package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"

	"study-guide/src/internal/store"
	"study-guide/src/internal/util"
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
	actionCursor      sessionActionCursor
	activeSessionSlug string
	finishedSessionCount   int
	inProgressSessionCount int
	subjects          []store.Subject
	selectedBySubject map[string]bool
	publishFunc       func(string) error

	message string
	err     error
	width   int
}

var subtleTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
var brightHintStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
var focusedTokenPattern = regexp.MustCompile(`\{[^{}\n]+\}`)
var actionCellTokenPattern = regexp.MustCompile("\x1e([^\x1e\x1f\n]*)\x1f")
const actionCellANSIPrefix = "\x1b[38;5;252;48;5;238m"
const actionCellANSISuffix = "\x1b[0m"
const focusedTokenANSIPrefix = "\x1b[1;38;5;230;48;5;62m"
const focusedTokenANSISuffix = "\x1b[0m"

const sessionsCreateInfoText = "select one or more subjects, then choose Create; esc to cancel"
const sessionsCreateItemIndent = "  "
const sessionsCreateActionCreateSubject = "(+) New subject"
const sessionsCreateActionCreateSession = "-> Create Session"

type sessionActionCursor string

const (
	sessionActionCursorActive   sessionActionCursor = "active"
	sessionActionCursorNextStep sessionActionCursor = "next-step"
)

func sessionsNextStepTextStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
}

func sessionsCreateItemLabel(label string) string {
	return sessionsCreateItemIndent + label
}

func newCreateListDelegate() list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.Padding(0, 0, 0, 0)
	delegate.Styles.DimmedTitle = delegate.Styles.DimmedTitle.Padding(0, 0, 0, 0)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Border(lipgloss.NormalBorder(), false, false, false, false).
		Padding(0, 0, 0, 0)
	return delegate
}

func newSessionsFilterInput() textinput.Model {
	fi := textinput.New()
	fi.Prompt = " filter: "
	fi.Placeholder = "by subject or slug"
	fi.CharLimit = 120
	fi.SetWidth(60)
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
		Background(lipgloss.NoColor{}).
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
			{Title: "ACTIVE", Width: 24},
			{Title: "STEP", Width: 24},
			{Title: "NEXT", Width: 52},
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
		actionCursor:      sessionActionCursorActive,
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
		m.filter.SetWidth(max(msg.Width-18, 20))
	case tea.KeyPressMsg:
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
		case "p":
			if m.view == sessionsViewBrowse {
				if err := m.publishStudy(); err != nil {
					m.message = "publish failed: " + err.Error()
					return m, nil
				}
				if m.canPublishFromBrowse() {
					m.message = fmt.Sprintf("published with %d sessions", m.finishedSessionCount)
				} else {
					m.message = "published"
				}
				return m, nil
			}
		case "left", "h":
			if m.view == sessionsViewBrowse {
				m.moveActionCursorLeft()
				m.applyBrowseEntries()
				return m, nil
			}
		case "right", "l":
			if m.view == sessionsViewBrowse {
				m.moveActionCursorRight()
				m.applyBrowseEntries()
				return m, nil
			}
		}
	}

	if m.view == sessionsViewBrowse {
		oldCursor := m.table.Cursor()
		oldFilter := m.filter.Value()
		var cmdFilter tea.Cmd
		m.filter, cmdFilter = m.filter.Update(msg)
		if m.filter.Value() != oldFilter {
			m.applyBrowseEntries()
		}
		var cmdTable tea.Cmd
		m.table, cmdTable = m.table.Update(msg)
		if m.table.Cursor() != oldCursor {
			m.applyBrowseEntries()
		}
		return m, tea.Batch(cmdFilter, cmdTable)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m sessionsSwitchboardModel) View() tea.View {
	if m.view == sessionsViewCreate {
		var b strings.Builder
		header := m.list.Styles.TitleBar.Render(m.list.Styles.Title.Render("Create Session"))
		b.WriteString(header)
		b.WriteString("\n")
		b.WriteString(subtleTextStyle.Render(sessionsCreateItemLabel(sessionsCreateInfoText)))
		b.WriteString("\n")
		b.WriteString(m.list.View())
		if strings.TrimSpace(m.message) != "" {
			b.WriteString("\n")
			b.WriteString(subtleTextStyle.Render(m.message))
		}
		return tea.NewView(b.String())
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
	tableView := m.table.View()
	tableView = styleBrowseActionCells(tableView)
	tableView = styleBrowseFocusedTokens(tableView)
	b.WriteString(tableView)
	b.WriteString("\n")
	b.WriteString(subtleTextStyle.Render(current))
	if strings.TrimSpace(m.message) != "" {
		b.WriteString("\n")
		b.WriteString(subtleTextStyle.Render(m.message))
	}
	b.WriteString("\n")
	footer := subtleTextStyle.Render("ctrl+n to create new; esc to quit")
	if m.canPublishFromBrowse() {
		footer += "  " + brightHintStyle.Render(fmt.Sprintf("p publish with %d sessions", m.finishedSessionCount))
	}
	b.WriteString(footer)
	return tea.NewView(b.String())
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
		if m.actionCursor == sessionActionCursorActive {
			if err := setActiveSessionSlug(m.root, rec.Slug); err != nil {
				m.message = "activate failed: " + err.Error()
				return m, nil
			}
			if rec.NextAction == "start" && rec.ProgressSteps == 0 {
				res, err := advanceSessionOnce(m.root, rec.Slug, m.protocol)
				if err != nil {
					m.message = "activate failed: " + err.Error()
					return m, nil
				}
				if err := m.refreshBrowseList(); err != nil {
					m.err = err
					return m, tea.Quit
				}
				m.message = fmt.Sprintf("session=%s state=activated+%s step=%s", rec.Slug, res.State, res.StepSlug)
				return m, nil
			}
			if err := m.refreshBrowseList(); err != nil {
				m.err = err
				return m, tea.Quit
			}
			m.message = fmt.Sprintf("session=%s state=activated", rec.Slug)
			return m, nil
		}
		res, err := advanceSessionOnce(m.root, rec.Slug, m.protocol)
		if err != nil {
			m.message = "advance failed: " + err.Error()
			return m, nil
		}
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
	case "create-subject":
		if err := subjectCreate(); err != nil {
			m.message = "create subject failed: " + err.Error()
			return m, nil
		}
		subs, err := store.ListSubjects()
		if err != nil {
			m.message = "refresh subjects failed: " + err.Error()
			return m, nil
		}
		m.subjects = subs
		m.refreshCreateList()
		m.message = ""
		return m, nil
	case "":
		return m, nil
	default:
		uid := strings.TrimPrefix(token, "subject:")
		m.selectedBySubject[uid] = !m.selectedBySubject[uid]
		m.refreshCreateList()
		m.message = ""
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
	m.finishedSessionCount = 0
	m.inProgressSessionCount = 0
	for _, r := range records {
		if r.Complete {
			m.finishedSessionCount++
			continue
		}
		m.inProgressSessionCount++
	}
	activeSlug, err := readActiveSessionSlug(m.root)
	if err != nil {
		return err
	}
	m.activeSessionSlug = activeSlug
	m.browseRecords = m.browseRecords[:0]
	for _, r := range records {
		r.Active = r.Slug == activeSlug
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
	if selectedSlug == "" && len(entries) > 0 && entries[0].kind == browseEntrySession {
		selectedSlug = entries[0].record.Slug
	}
	for i, e := range entries {
		slug, subject, active, step, nextStep := m.renderEntryRow(e)
		rows = append(rows, table.Row{slug, subject, active, step, nextStep})
		if e.kind == browseEntrySession && e.record.Slug == selectedSlug {
			targetCursor = i
		}
	}
	m.table.SetRows(rows)
	m.table.UpdateViewport()
	if targetCursor >= 0 && targetCursor < len(rows) {
		m.table.SetCursor(targetCursor)
	} else {
		m.table.SetCursor(0)
	}
}

func (m *sessionsSwitchboardModel) applyBrowseTableLayout() {
	total := max(m.width, 60)
	// Fit columns to viewport while preserving readability.
	const (
		overhead = 12
		nextMin  = 16
	)
	pref := []int{35, 35, 24, 48}
	mins := []int{14, 14, 8, 20}
	sumPref := 0
	for _, w := range pref {
		sumPref += w
	}
	budget := total - overhead - nextMin
	if budget < 0 {
		budget = 0
	}
	widths := append([]int(nil), pref...)
	if sumPref > budget {
		deficit := sumPref - budget
		for deficit > 0 {
			progress := false
			for i := range widths {
				if widths[i] > mins[i] {
					widths[i]--
					deficit--
					progress = true
					if deficit == 0 {
						break
					}
				}
			}
			if !progress {
				break
			}
		}
	}
	slugW := widths[0]
	subjectW := widths[1]
	activeW := widths[2]
	stepW := widths[3]
	nextW := total - overhead - slugW - subjectW - activeW - stepW
	if nextW < nextMin {
		nextW = nextMin
	}
	m.table.SetColumns([]table.Column{
		{Title: "SLUG", Width: slugW},
		{Title: "SUBJECT", Width: subjectW},
		{Title: "ACTIVE", Width: activeW},
		{Title: "STEP", Width: stepW},
		{Title: "NEXT", Width: nextW},
	})
	m.table.SetWidth(total)
}

func (m sessionsSwitchboardModel) renderEntryRow(e browseEntry) (string, string, string, string, string) {
	switch e.kind {
	case browseEntryEmpty:
		return "no active sessions", "", "", "", ""
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
		if current == "-" && stepNum > 0 && stepNum <= len(m.protocol.Steps) {
			current = m.protocol.Steps[stepNum-1].Name
		}
		stepText := fmt.Sprintf("[%d/%d] %s", stepNum, rec.StepCount, current)
		next := rec.NextStep
		if strings.TrimSpace(next) == "" {
			next = "-"
		}
		activeBaseText := ""
		if rec.Active {
			activeBaseText = "active"
		}
		activeStyled := encodeActionCellToken(activeBaseText)
		nextStyled := encodeActionCellToken(next)
		selectedSlug := ""
		if sel, ok := m.selectedBrowseEntry(); ok && sel.kind == browseEntrySession {
			selectedSlug = sel.record.Slug
		}
		if selectedSlug == "" && len(m.browseEntries) > 0 && m.browseEntries[0].kind == browseEntrySession {
			selectedSlug = m.browseEntries[0].record.Slug
		}
		if rec.Slug == selectedSlug {
			if !rec.Active {
				activeBaseText = "activate"
				activeStyled = encodeActionCellToken(activeBaseText)
			}
			if m.actionCursor == sessionActionCursorActive {
				activeStyled = encodeActionCellToken("{" + activeBaseText + "}")
			} else {
				nextStyled = encodeActionCellToken("{" + next + "}")
			}
			return rec.Slug, subjectText, activeStyled, stepText, nextStyled
		}
		return rec.Slug, subjectText, activeStyled, stepText, nextStyled
	default:
		return "", "", "", "", ""
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
		items = append(items, listItem(sessionsCreateItemLabel("No subjects available")))
	} else {
		for _, s := range m.subjects {
			marker := "[ ]"
			if m.selectedBySubject[s.UUID] {
				marker = "[x]"
			}
			label := sessionsCreateItemLabel(fmt.Sprintf("%s %s (%s)", marker, s.Name, strings.Split(s.UUID, "-")[0]))
			items = append(items, listItem(label))
			m.createLookup[label] = "subject:" + s.UUID
		}
	}
	createSubjectLabel := sessionsCreateItemLabel(sessionsCreateActionCreateSubject)
	items = append(items, listItem(createSubjectLabel))
	m.createLookup[createSubjectLabel] = "create-subject"
	createLabel := sessionsCreateItemLabel(sessionsCreateActionCreateSession)
	items = append(items, listItem(createLabel))
	m.createLookup[createLabel] = "create"

	delegate := newCreateListDelegate()
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

func (m *sessionsSwitchboardModel) moveActionCursorRight() {
	m.actionCursor = sessionActionCursorNextStep
}

func (m *sessionsSwitchboardModel) moveActionCursorLeft() {
	m.actionCursor = sessionActionCursorActive
}

func (m sessionsSwitchboardModel) canPublishFromBrowse() bool {
	return m.finishedSessionCount > 0 && m.inProgressSessionCount == 0
}

func (m sessionsSwitchboardModel) publishStudy() error {
	if m.publishFunc != nil {
		return m.publishFunc(m.root)
	}
	return cmdPublishAtRoot(m.root)
}

func readActiveSessionSlug(root string) (string, error) {
	studyPath := filepath.Join(root, "study.sg.md")
	fm, _, err := util.ReadFrontmatterFile(studyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(asString(fm["active_session_slug"])), nil
}

func setActiveSessionSlug(root, sessionSlug string) error {
	studyPath := filepath.Join(root, "study.sg.md")
	return setFrontmatterField(studyPath, "active_session_slug", sessionSlug)
}

func styleBrowseFocusedTokens(s string) string {
	return focusedTokenPattern.ReplaceAllStringFunc(s, func(token string) string {
		return focusedTokenANSIPrefix + token + focusedTokenANSISuffix
	})
}

func styleBrowseActionCells(s string) string {
	return actionCellTokenPattern.ReplaceAllString(s, actionCellANSIPrefix+"$1"+actionCellANSISuffix)
}

func encodeActionCellToken(s string) string {
	return "\x1e" + s + "\x1f"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
