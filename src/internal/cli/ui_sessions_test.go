package cli

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"study-guide/src/internal/store"
	"study-guide/src/internal/util"
)

func TestSessionsUI_SelectedRowStyle_DoesNotHighlightWholeRow(t *testing.T) {
	st := sessionsSelectedRowStyle(table.DefaultStyles().Selected)
	if _, ok := st.GetBackground().(lipgloss.NoColor); !ok {
		t.Fatalf("expected selected row background to be cleared, got %T", st.GetBackground())
	}
	if st.GetReverse() {
		t.Fatalf("expected selected row reverse=false")
	}
}

func TestSessionsUI_FilterInputDefaults(t *testing.T) {
	fi := newSessionsFilterInput()
	if fi.Prompt != " filter: " {
		t.Fatalf("unexpected prompt: %q", fi.Prompt)
	}
	if fi.Placeholder != "by subject or slug" {
		t.Fatalf("unexpected placeholder: %q", fi.Placeholder)
	}
	if !fi.Focused() {
		t.Fatalf("expected filter input to be focused")
	}
}

func TestSessionsUI_BrowseLayoutWidths(t *testing.T) {
	m := sessionsSwitchboardModel{
		table: table.New(),
		width: 220,
	}
	m.applyBrowseTableLayout()
	cols := m.table.Columns()
	if len(cols) != 5 {
		t.Fatalf("expected 5 columns, got %d", len(cols))
	}
	if cols[0].Title != "SLUG" || cols[1].Title != "SUBJECT" || cols[2].Title != "ACTIVE" || cols[3].Title != "STEP" || cols[4].Title != "NEXT" {
		t.Fatalf("unexpected browse headers: %#v", cols)
	}
	if cols[0].Width != 35 || cols[1].Width != 35 || cols[2].Width != 24 || cols[3].Width != 48 {
		t.Fatalf(
			"unexpected fixed widths: slug=%d subject=%d active=%d step=%d",
			cols[0].Width,
			cols[1].Width,
			cols[2].Width,
			cols[3].Width,
		)
	}
	if cols[4].Width < 16 {
		t.Fatalf("expected NEXT width >= 16, got %d", cols[4].Width)
	}
}

func TestSessionsUI_BrowseLayoutFitsNarrowViewports(t *testing.T) {
	m := sessionsSwitchboardModel{
		table: table.New(),
		width: 120,
	}
	m.applyBrowseTableLayout()
	cols := m.table.Columns()
	totalCols := cols[0].Width + cols[1].Width + cols[2].Width + cols[3].Width + cols[4].Width + 12
	if totalCols > 120 {
		t.Fatalf("expected columns to fit viewport, got total=%d viewport=120", totalCols)
	}
}

func TestSessionsUI_BrowseRowSnapshots(t *testing.T) {
	m := sessionsSwitchboardModel{
		actionCursor: sessionActionCursorActive,
		browseEntries: []browseEntry{
			{
				kind: browseEntrySession,
				record: sessionRecord{
					Slug: "18-02-2026-boehmer",
				},
			},
		},
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "SLUG", Width: 1},
				{Title: "SUBJECT", Width: 1},
				{Title: "ACTIVE", Width: 1},
				{Title: "STEP", Width: 1},
				{Title: "NEXT", Width: 1},
			}),
			table.WithRows([]table.Row{{"18-02-2026-boehmer", "", "", "", ""}}),
		),
	}

	empty := browseEntry{kind: browseEntryEmpty}
	slug, subject, active, step, next := m.renderEntryRow(empty)
	if slug != "no active sessions" || subject != "" || active != "" || step != "" || next != "" {
		t.Fatalf("unexpected empty row snapshot: %q | %q | %q | %q | %q", slug, subject, active, step, next)
	}

	rec := sessionRecord{
		Slug:          "18-02-2026-boehmer",
		SubjectNames:  []string{"Cameron Boehmer"},
		CurrentStep:   "Ground",
		ProgressSteps: 2,
		StepCount:     3,
		NextStep:      "Second Exposure",
	}
	slug, subject, active, step, next = m.renderEntryRow(browseEntry{kind: browseEntrySession, record: rec})
	if slug != "18-02-2026-boehmer" || subject != "Cameron Boehmer" || step != "[2/3] Ground" {
		t.Fatalf("unexpected unarmed row snapshot: %q | %q | %q", slug, subject, step)
	}
	if strings.TrimSpace(stripInternalMarkers(stripANSI(active))) != "{activate}" {
		t.Fatalf("unexpected focused active-cell snapshot: %q", stripInternalMarkers(stripANSI(active)))
	}
	if strings.TrimSpace(stripInternalMarkers(stripANSI(next))) != "Second Exposure" {
		t.Fatalf("unexpected unfocused next-step snapshot: %q", stripInternalMarkers(stripANSI(next)))
	}

	m.actionCursor = sessionActionCursorNextStep
	_, _, active, _, next = m.renderEntryRow(browseEntry{kind: browseEntrySession, record: rec})
	if strings.TrimSpace(stripInternalMarkers(stripANSI(active))) != "activate" {
		t.Fatalf("unexpected unfocused active-cell snapshot: %q", stripInternalMarkers(stripANSI(active)))
	}
	if strings.TrimSpace(stripInternalMarkers(stripANSI(next))) != "{Second Exposure}" {
		t.Fatalf("unexpected focused next-step snapshot: %q", stripInternalMarkers(stripANSI(next)))
	}
}

func TestSessionsUI_FocusInvariant_ExactlyOneActionCellHighlighted(t *testing.T) {
	m := sessionsSwitchboardModel{
		actionCursor: sessionActionCursorActive,
		browseEntries: []browseEntry{
			{
				kind: browseEntrySession,
				record: sessionRecord{
					Slug: "s1",
				},
			},
		},
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "SLUG", Width: 1},
				{Title: "SUBJECT", Width: 1},
				{Title: "ACTIVE", Width: 1},
				{Title: "STEP", Width: 1},
				{Title: "NEXT", Width: 1},
			}),
			table.WithRows([]table.Row{{"s1", "", "", "", ""}}),
		),
	}
	rec := sessionRecord{
		Slug:          "s1",
		SubjectNames:  []string{"Alpha"},
		CurrentStep:   "Ground",
		ProgressSteps: 1,
		StepCount:     3,
		NextStep:      "Second Exposure",
	}

	_, _, active, step, next := m.renderEntryRow(browseEntry{kind: browseEntrySession, record: rec})
	assertSingleFocusedActionCell(t, active, step, next)
	if strings.Contains(stripANSI(step), "{") || strings.Contains(stripANSI(step), "}") {
		t.Fatalf("step column must never show focus markers, got %q", stripANSI(step))
	}

	m.actionCursor = sessionActionCursorNextStep
	_, _, active, step, next = m.renderEntryRow(browseEntry{kind: browseEntrySession, record: rec})
	assertSingleFocusedActionCell(t, active, step, next)
	if strings.Contains(stripANSI(step), "{") || strings.Contains(stripANSI(step), "}") {
		t.Fatalf("step column must never show focus markers, got %q", stripANSI(step))
	}
}

func TestSessionsUI_Interaction_RightLeftMovesSingleVisibleFocus(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:         sessionsViewBrowse,
		actionCursor: sessionActionCursorActive,
		browseRecords: []sessionRecord{
			{
				Slug:          "s1",
				SubjectNames:  []string{"Alpha"},
				CurrentStep:   "Ground",
				ProgressSteps: 1,
				StepCount:     3,
				NextStep:      "Second Exposure",
			},
		},
		browseEntries: []browseEntry{
			{
				kind: browseEntrySession,
				record: sessionRecord{
					Slug: "s1",
				},
			},
		},
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "SLUG", Width: 1},
				{Title: "SUBJECT", Width: 1},
				{Title: "ACTIVE", Width: 1},
				{Title: "STEP", Width: 1},
				{Title: "NEXT", Width: 1},
			}),
			table.WithRows([]table.Row{{"s1", "", "", "", ""}}),
		),
		filter: newSessionsFilterInput(),
	}
	rec := sessionRecord{
		Slug:          "s1",
		SubjectNames:  []string{"Alpha"},
		CurrentStep:   "Ground",
		ProgressSteps: 1,
		StepCount:     3,
		NextStep:      "Second Exposure",
	}
	m.applyBrowseEntries()

	_, _, active, step, next := m.renderEntryRow(browseEntry{kind: browseEntrySession, record: rec})
	assertSingleFocusedActionCell(t, active, step, next)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = updated.(sessionsSwitchboardModel)
	_, _, active, step, next = m.renderEntryRow(browseEntry{kind: browseEntrySession, record: rec})
	assertSingleFocusedActionCell(t, active, step, next)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(sessionsSwitchboardModel)
	_, _, active, step, next = m.renderEntryRow(browseEntry{kind: browseEntrySession, record: rec})
	assertSingleFocusedActionCell(t, active, step, next)
}

func TestSessionsUI_TableRowMapping_ActiveAndStepDoNotShift(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:         sessionsViewBrowse,
		actionCursor: sessionActionCursorActive,
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "SLUG", Width: 20},
				{Title: "SUBJECT", Width: 20},
				{Title: "ACTIVE", Width: 12},
				{Title: "STEP", Width: 24},
				{Title: "NEXT", Width: 16},
			}),
		),
		browseRecords: []sessionRecord{
			{
				Slug:          "s1",
				SubjectNames:  []string{"Alpha"},
				ProgressSteps: 3,
				StepCount:     3,
				CurrentStep:   "Third Step",
				NextStep:      "conclude",
				Active:        true,
			},
		},
	}

	m.applyBrowseEntries()
	rows := m.table.Rows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 table row, got %d", len(rows))
	}
	if stripInternalMarkers(rows[0][2]) != "{active}" {
		t.Fatalf("expected ACTIVE column focus token in col 3, got %q", stripInternalMarkers(rows[0][2]))
	}
	if !strings.HasPrefix(rows[0][3], "[3/3]") {
		t.Fatalf("expected STEP column progress text in col 4, got %q", rows[0][3])
	}
	if stripInternalMarkers(rows[0][4]) != "conclude" {
		t.Fatalf("expected NEXT column conclude in col 5, got %q", stripInternalMarkers(rows[0][4]))
	}

	m.actionCursor = sessionActionCursorNextStep
	m.applyBrowseEntries()
	rows = m.table.Rows()
	if stripInternalMarkers(rows[0][2]) != "active" {
		t.Fatalf("expected ACTIVE column unfocused text in col 3, got %q", stripInternalMarkers(rows[0][2]))
	}
	if stripInternalMarkers(rows[0][4]) != "{conclude}" {
		t.Fatalf("expected NEXT column focus token in col 5, got %q", stripInternalMarkers(rows[0][4]))
	}
}

func TestSessionsUI_ViewStylesFocusedTokensPostRender(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:         sessionsViewBrowse,
		actionCursor: sessionActionCursorActive,
		filter:       newSessionsFilterInput(),
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "SLUG", Width: 20},
				{Title: "SUBJECT", Width: 20},
				{Title: "ACTIVE", Width: 12},
				{Title: "STEP", Width: 24},
				{Title: "NEXT", Width: 16},
			}),
		),
		browseRecords: []sessionRecord{
			{
				Slug:          "s1",
				SubjectNames:  []string{"Alpha"},
				ProgressSteps: 1,
				StepCount:     3,
				CurrentStep:   "Ground",
				NextStep:      "Second Exposure",
			},
		},
	}
	m.applyBrowseEntries()
	out := m.View()
	if !strings.Contains(out, "{activate}") {
		t.Fatalf("expected focused token in view output, got:\n%s", stripANSI(out))
	}
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("expected ANSI styling in view output for focused token")
	}
}

func TestSessionsUI_View_DefaultActionCellsAreStyledAsCTA(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:         sessionsViewBrowse,
		actionCursor: sessionActionCursorActive,
		filter:       newSessionsFilterInput(),
		browseEntries: []browseEntry{
			{
				kind: browseEntrySession,
				record: sessionRecord{Slug: "s1"},
			},
		},
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "SLUG", Width: 20},
				{Title: "SUBJECT", Width: 20},
				{Title: "ACTIVE", Width: 12},
				{Title: "STEP", Width: 24},
				{Title: "NEXT", Width: 16},
			}),
			table.WithRows([]table.Row{{"s1", "", "", "", ""}}),
		),
	}
	rec := sessionRecord{
		Slug:          "s1",
		SubjectNames:  []string{"Alpha"},
		ProgressSteps: 1,
		StepCount:     3,
		CurrentStep:   "Ground",
		NextStep:      "Second Exposure",
	}
	m.browseRecords = []sessionRecord{rec}
	m.applyBrowseEntries()

	out := m.View()
	if count := strings.Count(out, actionCellANSIPrefix); count < 2 {
		t.Fatalf("expected CTA styling in both ACTIVE and NEXT cells, prefix_count=%d", count)
	}
}

func TestSessionsUI_DefaultActionCursorIsActiveColumn(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:         sessionsViewBrowse,
		actionCursor: sessionActionCursorActive,
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "SLUG", Width: 1},
				{Title: "SUBJECT", Width: 1},
				{Title: "ACTIVE", Width: 1},
				{Title: "STEP", Width: 1},
				{Title: "NEXT", Width: 1},
			}),
			table.WithRows([]table.Row{{"s1", "", "", "", ""}}),
		),
		browseEntries: []browseEntry{
			{
				kind: browseEntrySession,
				record: sessionRecord{
					Slug: "s1",
				},
			},
		},
		browseRecords: []sessionRecord{
			{Slug: "s1"},
		},
	}

	if m.actionCursor != sessionActionCursorActive {
		t.Fatalf("expected default action cursor in ACTIVE column, got %q", m.actionCursor)
	}
}

func TestSessionsUI_ActionCursorMovesLeftRight(t *testing.T) {
	m := sessionsSwitchboardModel{
		actionCursor: sessionActionCursorActive,
	}

	m.moveActionCursorRight()
	if m.actionCursor != sessionActionCursorNextStep {
		t.Fatalf("expected right-arrow to move cursor to NEXT STEP, got %q", m.actionCursor)
	}

	m.moveActionCursorLeft()
	if m.actionCursor != sessionActionCursorActive {
		t.Fatalf("expected left-arrow to move cursor back to ACTIVE, got %q", m.actionCursor)
	}
}

func TestSessionsUI_EnterOnActiveColumnActivatesSessionAndStartsFirstStep(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SG_SUBJECT_DIR", filepath.Join(root, ".subjects"))
	protocol := testProtocol()
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\n---\n\n# Study\n")
	mustWriteSessionFile(t, root, "s1", map[string]any{
		"time_started": "10:00:00 01-01-2026",
		"subject_ids":  []string{"sub-1"},
	})

	m := sessionsSwitchboardModel{
		root:         root,
		protocol:     protocol,
		view:         sessionsViewBrowse,
		actionCursor: sessionActionCursorActive,
		filter:       newSessionsFilterInput(),
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "SLUG", Width: 1},
				{Title: "SUBJECT", Width: 1},
				{Title: "ACTIVE", Width: 1},
				{Title: "STEP", Width: 1},
				{Title: "NEXT", Width: 1},
			}),
			table.WithRows([]table.Row{{"s1", "", "", "", ""}}),
		),
		browseEntries: []browseEntry{
			{
				kind: browseEntrySession,
				record: sessionRecord{
					Slug:       "s1",
					NextAction: "start",
				},
			},
		},
	}

	_, _ = m.handleBrowseEnter()

	fm, _, err := util.ReadFrontmatterFile(filepath.Join(root, "study.sg.md"))
	if err != nil {
		t.Fatalf("read study frontmatter failed: %v", err)
	}
	if got := strings.TrimSpace(asString(fm["active_session_slug"])); got != "s1" {
		t.Fatalf("expected active_session_slug=s1, got %q", got)
	}
	stepFM, _, err := util.ReadFrontmatterFile(filepath.Join(root, "session", "s1", "step", "first-step", "step.sg.md"))
	if err != nil {
		t.Fatalf("read first step frontmatter failed: %v", err)
	}
	if asString(stepFM["time_started"]) == "" {
		t.Fatalf("expected first step to auto-start on activation")
	}
}

func TestSessionsUI_EnterOnNextStepAdvancesOnce(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SG_SUBJECT_DIR", filepath.Join(root, ".subjects"))
	protocol := testProtocol()
	slug := "01-01-2026-alpha"
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\n---\n\n# Study\n")
	mustWriteSessionFile(t, root, slug, map[string]any{
		"time_started": "10:00:00 01-01-2026",
		"subject_ids":  []string{"sub-1"},
	})

	m := sessionsSwitchboardModel{
		root:         root,
		protocol:     protocol,
		view:         sessionsViewBrowse,
		actionCursor: sessionActionCursorNextStep,
		filter:       newSessionsFilterInput(),
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "SLUG", Width: 1},
				{Title: "SUBJECT", Width: 1},
				{Title: "ACTIVE", Width: 1},
				{Title: "STEP", Width: 1},
				{Title: "NEXT", Width: 1},
			}),
			table.WithRows([]table.Row{{slug, "", "", "", ""}}),
		),
		browseEntries: []browseEntry{
			{
				kind: browseEntrySession,
				record: sessionRecord{
					Slug:       slug,
					NextAction: "start",
				},
			},
		},
	}

	out, _ := m.handleBrowseEnter()
	got := out.(sessionsSwitchboardModel)
	if !strings.Contains(got.message, "state=started") {
		t.Fatalf("expected started transition message, got %q", got.message)
	}
}

func TestSessionsUI_CreateViewSnapshot(t *testing.T) {
	m := sessionsSwitchboardModel{
		subjects:          []store.Subject{{UUID: "abc12345-0000-0000-0000-000000000000", Name: "Alpha Subject"}},
		selectedBySubject: map[string]bool{},
	}
	m.refreshCreateList()
	out := stripANSI(m.View())

	expectedInOrder := []string{
		"Create Session",
		"  " + sessionsCreateInfoText,
		"  [ ] Alpha Subject (abc12345)",
		"  Create new subject",
	}
	last := -1
	for _, token := range expectedInOrder {
		idx := strings.Index(out, token)
		if idx < 0 {
			t.Fatalf("missing snapshot token: %q\noutput:\n%s", token, out)
		}
		if idx <= last {
			t.Fatalf("snapshot order mismatch around token %q\noutput:\n%s", token, out)
		}
		last = idx
	}

	lines := strings.Split(out, "\n")
	subjectLine := -1
	createSubjectLine := -1
	createLine := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[ ] Alpha Subject (abc12345)" {
			subjectLine = i
		}
		if trimmed == "Create new subject" && strings.HasPrefix(line, "  ") {
			createSubjectLine = i
		}
		if trimmed == "Create" && strings.HasPrefix(line, "  ") {
			createLine = i
		}
	}
	if subjectLine < 0 || createSubjectLine < 0 || createLine < 0 {
		t.Fatalf("expected subject, create-subject, and create action lines in snapshot\noutput:\n%s", out)
	}
	if createSubjectLine <= subjectLine {
		t.Fatalf("expected Create new subject below subject line, got subject=%d create-subject=%d", subjectLine, createSubjectLine)
	}
	if createLine <= createSubjectLine {
		t.Fatalf("expected Create action below Create new subject line, got create-subject=%d create=%d", createSubjectLine, createLine)
	}
}

func TestSessionsUI_CreateDelegateNoHorizontalShift(t *testing.T) {
	d := newCreateListDelegate()
	if d.Styles.SelectedTitle.GetBorderLeftSize() != 0 {
		t.Fatalf("expected no selected left border, got %d", d.Styles.SelectedTitle.GetBorderLeftSize())
	}
	if d.Styles.SelectedTitle.GetPaddingLeft() != d.Styles.NormalTitle.GetPaddingLeft() {
		t.Fatalf(
			"expected same left padding, got selected=%d normal=%d",
			d.Styles.SelectedTitle.GetPaddingLeft(),
			d.Styles.NormalTitle.GetPaddingLeft(),
		)
	}
}

func TestSessionCreatePicker_ViewMatchesSessionsCreateView(t *testing.T) {
	subjects := []store.Subject{{UUID: "abc12345-0000-0000-0000-000000000000", Name: "Alpha Subject"}}
	selected := map[string]bool{}

	m := sessionsSwitchboardModel{
		subjects:          subjects,
		selectedBySubject: selected,
	}
	m.refreshCreateList()
	switchboardView := stripANSI(m.View())

	picker := newSessionCreatePickerModel(subjects, selected)
	pickerView := stripANSI(picker.View())

	expectedInOrder := []string{
		"Create Session",
		"  " + sessionsCreateInfoText,
		"  [ ] Alpha Subject (abc12345)",
		"  Create new subject",
	}
	lastSwitchboard := -1
	lastPicker := -1
	for _, token := range expectedInOrder {
		idxSwitchboard := strings.Index(switchboardView, token)
		if idxSwitchboard < 0 {
			t.Fatalf("switchboard view missing token: %q\noutput:\n%s", token, switchboardView)
		}
		if idxSwitchboard <= lastSwitchboard {
			t.Fatalf("switchboard token order mismatch around %q\noutput:\n%s", token, switchboardView)
		}
		lastSwitchboard = idxSwitchboard

		idxPicker := strings.Index(pickerView, token)
		if idxPicker < 0 {
			t.Fatalf("picker view missing token: %q\noutput:\n%s", token, pickerView)
		}
		if idxPicker <= lastPicker {
			t.Fatalf("picker token order mismatch around %q\noutput:\n%s", token, pickerView)
		}
		lastPicker = idxPicker
	}
}

func stripANSI(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

func stripInternalMarkers(s string) string {
	return strings.NewReplacer("\x1e", "", "\x1f", "").Replace(s)
}

func assertSingleFocusedActionCell(t *testing.T, activeCell, stepCell, nextCell string) {
	t.Helper()
	activePlain := stripANSI(activeCell)
	stepPlain := stripANSI(stepCell)
	nextPlain := stripANSI(nextCell)
	focused := 0
	if strings.Contains(activePlain, "{") && strings.Contains(activePlain, "}") {
		focused++
	}
	if strings.Contains(stepPlain, "{") || strings.Contains(stepPlain, "}") {
		focused++
	}
	if strings.Contains(nextPlain, "{") && strings.Contains(nextPlain, "}") {
		focused++
	}
	if focused != 1 {
		t.Fatalf(
			"expected exactly one focused actionable cell, got=%d active=%q step=%q next=%q",
			focused,
			activePlain,
			stepPlain,
			nextPlain,
		)
	}
}
