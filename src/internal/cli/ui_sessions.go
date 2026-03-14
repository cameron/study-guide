package cli

import (
	"fmt"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"

	"study-guide/src/internal/store"
	"study-guide/src/internal/util"
)

type sessionsView int

const (
	sessionsViewBrowse sessionsView = iota
	sessionsViewCreate
	sessionsViewCreateSubject
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

	createLookup           map[string]string
	actionCursor           sessionActionCursor
	activeSessionSlug      string
	finishedSessionCount   int
	inProgressSessionCount int
	subjects               []store.Subject
	selectedBySubject      map[string]bool
	createSubjectForm      formModel
	publishFunc            func(string) error

	message string
	err     error
	width   int
}

var subtleTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
var filterPromptStyle = lipgloss.NewStyle().Foreground(paletteBlueAccentAdaptive)
var filterQueryStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
var filterPlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
var selectedRowTintLight = color.RGBA{R: 0xd9, G: 0xdc, B: 0xef, A: 0xff}
var selectedRowTintDark = color.RGBA{R: 0x26, G: 0x2b, B: 0x3a, A: 0xff}

const selectedRowBackgroundANSILight = "\x1b[48;2;217;220;239m"
const selectedRowBackgroundANSIDark = "\x1b[48;2;38;43;58m"

const sessionsCreateInfoText = "select one or more subjects, then choose Create; esc to cancel"
const sessionsBrowseFilterPlaceholder = "by subject or slug"
const sessionsCreateFilterPlaceholder = "by subject name"
const sessionsCreateItemIndent = "  "
const sessionsCreateActionCreateSubject = "(+) New subject"
const sessionsCreateActionCreateSession = "-> Create Session"
const emptyActionCellSpaceFill = 8
const focusedActionMarker = "↳ "

type sessionActionCursor string

const (
	sessionActionCursorFocus    sessionActionCursor = "focus"
	sessionActionCursorNextStep sessionActionCursor = "next-step"
)

func sessionsNextStepTextStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
}

func sessionsCreateItemLabel(label string) string {
	return sessionsCreateItemIndent + label
}

type createListDelegate struct {
	list.DefaultDelegate
}

func createListShouldUseSelectedStyle(filterState list.FilterState, filterValue string, isSelected bool) bool {
	if !isSelected {
		return false
	}
	if filterState == list.Filtering && strings.TrimSpace(filterValue) == "" {
		return false
	}
	return true
}

func (d createListDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	var title string
	if i, ok := item.(list.DefaultItem); ok {
		title = i.Title()
	} else {
		return
	}
	if m.Width() <= 0 {
		return
	}
	s := &d.Styles
	textWidth := m.Width() - s.NormalTitle.GetPaddingLeft() - s.NormalTitle.GetPaddingRight()
	title = ansi.Truncate(title, textWidth, "…")
	isSelected := index == m.Index()
	isFiltered := m.FilterState() == list.Filtering || m.FilterState() == list.FilterApplied
	useSelected := createListShouldUseSelectedStyle(m.FilterState(), m.FilterValue(), isSelected)
	matchedRunes := m.MatchesForItem(index)

	if m.FilterState() == list.Filtering && strings.TrimSpace(m.FilterValue()) == "" {
		fmt.Fprint(w, s.DimmedTitle.Render(title)) //nolint:errcheck
		return
	}
	if useSelected {
		if isFiltered {
			unmatched := s.SelectedTitle.Inline(true)
			matched := unmatched.Inherit(s.FilterMatch)
			title = lipgloss.StyleRunes(title, matchedRunes, matched, unmatched)
		}
		fmt.Fprint(w, s.SelectedTitle.Render(title)) //nolint:errcheck
		return
	}
	if isFiltered {
		unmatched := s.NormalTitle.Inline(true)
		matched := unmatched.Inherit(s.FilterMatch)
		title = lipgloss.StyleRunes(title, matchedRunes, matched, unmatched)
	}
	fmt.Fprint(w, s.NormalTitle.Render(title)) //nolint:errcheck
}

func newCreateListDelegate() createListDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.Padding(0, 0, 0, 0)
	delegate.Styles.DimmedTitle = delegate.Styles.DimmedTitle.Padding(0, 0, 0, 0)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Border(lipgloss.NormalBorder(), false, false, false, false).
		Padding(0, 0, 0, 0)
	return createListDelegate{DefaultDelegate: delegate}
}

func newSessionsFilterInput() textinput.Model {
	fi := textinput.New()
	fi.Prompt = " filter: "
	fi.Placeholder = sessionsBrowseFilterPlaceholder
	fi.CharLimit = 120
	fi.SetWidth(60)
	fi.Focus()
	applyFilterInputAccentStyle(&fi)
	return fi
}

func applyFilterInputAccentStyle(input *textinput.Model) {
	styles := input.Styles()
	styles.Focused.Prompt = filterPromptStyle
	styles.Blurred.Prompt = filterPromptStyle
	styles.Focused.Text = filterQueryStyle
	styles.Blurred.Text = filterQueryStyle
	styles.Focused.Placeholder = filterPlaceholderStyle
	styles.Blurred.Placeholder = filterPlaceholderStyle
	input.SetStyles(styles)
}

func sessionsSelectedRowStyle(base lipgloss.Style) lipgloss.Style {
	return base.
		Bold(false)
}

func actionCellStyle(isFocused bool) lipgloss.Style {
	fg := lipgloss.Color("255")
	bg := compat.AdaptiveColor{
		Light: color.RGBA{R: 0x02, G: 0x68, B: 0x46, A: 0xff},
		Dark:  color.RGBA{R: 0x4f, G: 0x61, B: 0x22, A: 0xff},
	}
	if isFocused {
		bg = compat.AdaptiveColor{
			Light: color.RGBA{R: 0x04, G: 0xb5, B: 0x75, A: 0xff},
			Dark:  color.RGBA{R: 0x93, G: 0xad, B: 0x3f, A: 0xff},
		}
	}
	return lipgloss.NewStyle().Foreground(fg).Background(bg).Bold(isFocused)
}

func selectedRowBackgroundANSI() string {
	if compat.HasDarkBackground {
		return selectedRowBackgroundANSIDark
	}
	return selectedRowBackgroundANSILight
}

func renderActionCell(text string, isFocused, inSelectedRow bool, width int) string {
	if text == "" {
		text = strings.Repeat(" ", emptyActionCellSpaceFill)
	}
	out := actionCellStyle(isFocused).Render(text)
	_ = inSelectedRow
	_ = width
	return out
}

func runSessionsSwitchboard(root string, protocol store.Protocol) error {
	m, err := newSessionsSwitchboardModel(root, protocol)
	if err != nil {
		return err
	}
	res, err := runInteractiveProgram(m)
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
			{Title: "FOCUSED", Width: 24},
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

	createList := newCreateSessionListModel(nil)

	m := sessionsSwitchboardModel{
		root:              root,
		protocol:          protocol,
		view:              sessionsViewBrowse,
		table:             tbl,
		filter:            fi,
		list:              createList,
		selectedBySubject: map[string]bool{},
		actionCursor:      sessionActionCursorFocus,
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
		if m.view == sessionsViewCreateSubject {
			return m.updateCreateSubjectForm(msg)
		}
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+n":
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
		case "shift+enter", "shift+return":
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
			if m.view == sessionsViewCreate {
				return m.handleCreateShortcut()
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
		case "left":
			if m.view == sessionsViewBrowse {
				m.moveActionCursorLeft()
				m.applyBrowseEntries()
				return m, nil
			}
		case "right":
			if m.view == sessionsViewBrowse {
				m.moveActionCursorRight()
				m.applyBrowseEntries()
				return m, nil
			}
		case "ctrl+b":
			if m.view == sessionsViewBrowse {
				return m.handleBrowseReverse()
			}
		}
	}

	if m.view == sessionsViewCreateSubject {
		return m.updateCreateSubjectForm(msg)
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

	if key, ok := msg.(tea.KeyPressMsg); ok {
		startListFilteringOnTextInputWithoutInlineFilter(&m.list, key)
	}
	oldFilter := m.list.FilterValue()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	autoSelectTopEntryInFilteredList(&m.list, oldFilter)
	resetListFilterIfEmpty(&m.list)
	return m, cmd
}

func (m sessionsSwitchboardModel) View() tea.View {
	if m.view == sessionsViewCreateSubject {
		return m.createSubjectForm.View()
	}
	if m.view == sessionsViewCreate {
		var b strings.Builder
		b.WriteString(renderScreenTitle("Create Session"))
		b.WriteString("\n")
		b.WriteString(subtleTextStyle.Render(sessionsCreateItemLabel(sessionsCreateInfoText)))
		b.WriteString("\n")
		b.WriteString(m.list.FilterInput.View())
		b.WriteString("\n")
		b.WriteString(m.list.View())
		if strings.TrimSpace(m.message) != "" {
			b.WriteString("\n")
			b.WriteString(subtleTextStyle.Render(m.message))
		}
		return tea.NewView(b.String())
	}

	var b strings.Builder
	b.WriteString(renderScreenTitle("Sessions"))
	b.WriteString("\n")
	b.WriteString(m.filter.View())
	b.WriteString("\n")
	b.WriteString(m.table.View())
	b.WriteString("\n")
	b.WriteString(subtleTextStyle.Render("enter to activate cell; ctrl+b to step backwards; ctrl+n to create session; p to publish; esc to quit"))
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
		if m.actionCursor == sessionActionCursorFocus {
			now := util.NowTimestamp()
			if strings.TrimSpace(m.activeSessionSlug) != "" && m.activeSessionSlug != rec.Slug {
				if err := closeFocusedSessionWindows(m.root, m.activeSessionSlug, m.protocol, now); err != nil {
					m.message = "focus failed: " + err.Error()
					return m, nil
				}
			}
			if err := setActiveSessionSlug(m.root, rec.Slug); err != nil {
				m.message = "focus failed: " + err.Error()
				return m, nil
			}
			if rec.NextAction == "start" && rec.ProgressSteps == 0 {
				res, err := advanceSessionOnce(m.root, rec.Slug, m.protocol)
				if err != nil {
					m.message = "focus failed: " + err.Error()
					return m, nil
				}
				if err := m.refreshBrowseList(); err != nil {
					m.err = err
					return m, tea.Quit
				}
				m.table.SetCursor(0)
				m.applyBrowseEntries()
				m.message = fmt.Sprintf("session=%s state=focused+%s step=%s", rec.Slug, res.State, res.StepSlug)
				return m, nil
			}
			if err := syncFocusedSessionWindows(m.root, rec.Slug, m.protocol, now); err != nil {
				m.message = "focus failed: " + err.Error()
				return m, nil
			}
			if err := m.refreshBrowseList(); err != nil {
				m.err = err
				return m, tea.Quit
			}
			m.table.SetCursor(0)
			m.applyBrowseEntries()
			m.message = fmt.Sprintf("session=%s state=focused", rec.Slug)
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

func (m sessionsSwitchboardModel) handleBrowseReverse() (tea.Model, tea.Cmd) {
	entry, ok := m.selectedBrowseEntry()
	if !ok || entry.kind != browseEntrySession {
		return m, nil
	}
	rec := entry.record
	if rec.NextAction == "invalid" {
		m.message = "invalid: " + rec.InvalidReason
		return m, nil
	}
	res, err := reverseSessionOnce(m.root, rec.Slug, m.protocol)
	if err != nil {
		m.message = "reverse failed: " + err.Error()
		return m, nil
	}
	if err := m.refreshBrowseList(); err != nil {
		m.err = err
		return m, tea.Quit
	}
	m.message = fmt.Sprintf("session=%s state=%s step=%s", rec.Slug, res.State, res.StepSlug)
	return m, nil
}

func (m sessionsSwitchboardModel) handleCreateEnter() (tea.Model, tea.Cmd) {
	choice, ok := selectedListItemTitle(m.list.SelectedItem())
	if !ok {
		return m, nil
	}
	switch token := m.createLookup[choice]; token {
	case "create":
		return m.handleCreateShortcut()
	case "create-subject":
		form, err := newSubjectCreateFormModel(m.root)
		if err != nil {
			m.message = "create subject failed: " + err.Error()
			return m, nil
		}
		m.createSubjectForm = form
		m.message = ""
		m.view = sessionsViewCreateSubject
		return m, textinput.Blink
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

func (m sessionsSwitchboardModel) handleCreateShortcut() (tea.Model, tea.Cmd) {
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
	if m.activeSessionSlug != "" && len(filtered) > 1 {
		focusedIdx := -1
		for i := range filtered {
			if filtered[i].Slug == m.activeSessionSlug {
				focusedIdx = i
				break
			}
		}
		if focusedIdx > 0 {
			focused := filtered[focusedIdx]
			copy(filtered[1:focusedIdx+1], filtered[0:focusedIdx])
			filtered[0] = focused
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
	focusW := widths[2]
	stepW := widths[3]
	nextW := total - overhead - slugW - subjectW - focusW - stepW
	if nextW < nextMin {
		nextW = nextMin
	}
	m.table.SetColumns([]table.Column{
		{Title: "SLUG", Width: slugW},
		{Title: "SUBJECT", Width: subjectW},
		{Title: "FOCUSED", Width: focusW},
		{Title: "STEP", Width: stepW},
		{Title: "NEXT", Width: nextW},
	})
	m.table.SetWidth(total)
}

func (m sessionsSwitchboardModel) renderEntryRow(e browseEntry) (string, string, string, string, string) {
	switch e.kind {
	case browseEntryEmpty:
		return "no open sessions", "", "", "", ""
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
		selectedSlug := ""
		if sel, ok := m.selectedBrowseEntry(); ok && sel.kind == browseEntrySession {
			selectedSlug = sel.record.Slug
		}
		if selectedSlug == "" && len(m.browseEntries) > 0 && m.browseEntries[0].kind == browseEntrySession {
			selectedSlug = m.browseEntries[0].record.Slug
		}
		focusWidth := 1
		nextWidth := 1
		cols := m.table.Columns()
		if len(cols) >= 5 {
			if cols[2].Width > 0 {
				focusWidth = cols[2].Width
			}
			if cols[4].Width > 0 {
				nextWidth = cols[4].Width
			}
		}
		isSelected := rec.Slug == selectedSlug
		focusBaseText := ""
		if rec.Active {
			focusBaseText = "focused"
		}
		focusStyled := renderActionCell(focusBaseText, false, isSelected, focusWidth)
		nextStyled := renderActionCell(next, false, isSelected, nextWidth)
		if isSelected {
			if !rec.Active {
				focusBaseText = ""
				focusStyled = renderActionCell(focusBaseText, false, true, focusWidth)
			}
			if m.actionCursor == sessionActionCursorFocus {
				if rec.Active {
					focusStyled = renderActionCell(focusedActionMarker+"focused", true, true, focusWidth)
				} else {
					focusStyled = renderActionCell(focusedActionMarker+"focus", true, true, focusWidth)
				}
			} else {
				nextStyled = renderActionCell(focusedActionMarker+next, true, true, nextWidth)
			}
			return rec.Slug, subjectText, focusStyled, stepText, nextStyled
		}
		return rec.Slug, subjectText, focusStyled, stepText, nextStyled
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
	items, createLookup := buildCreateSessionItems(m.subjects, m.selectedBySubject)
	m.createLookup = createLookup
	m.list = newCreateSessionListModel(items)
	m.view = sessionsViewCreate
}

func (m sessionsSwitchboardModel) updateCreateSubjectForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, _ := m.createSubjectForm.Update(msg)
	m.createSubjectForm = next.(formModel)
	if !m.createSubjectForm.done {
		return m, nil
	}
	if m.createSubjectForm.canceled {
		m.refreshCreateList()
		m.message = ""
		return m, nil
	}
	path, err := saveCreatedSubject(formValues(m.createSubjectForm))
	if err != nil {
		m.refreshCreateList()
		m.message = "create subject failed: " + err.Error()
		return m, nil
	}
	subs, err := store.ListSubjects()
	if err != nil {
		m.refreshCreateList()
		m.message = "refresh subjects failed: " + err.Error()
		return m, nil
	}
	m.subjects = subs
	m.refreshCreateList()
	m.message = "created " + path
	return m, nil
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
	m.actionCursor = sessionActionCursorFocus
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
