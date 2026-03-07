package cli

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"study-guide/src/internal/store"
	"study-guide/src/internal/util"
)

func TestSessionsUI_SelectedRowStyle_UsesBackgroundTint(t *testing.T) {
	st := sessionsSelectedRowStyle(table.DefaultStyles().Selected)
	if _, ok := st.GetBackground().(lipgloss.NoColor); ok {
		t.Fatalf("expected selected row background tint, got %T", st.GetBackground())
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
	if cols[0].Title != "SLUG" || cols[1].Title != "SUBJECT" || cols[2].Title != "FOCUSED" || cols[3].Title != "STEP" || cols[4].Title != "NEXT" {
		t.Fatalf("unexpected browse headers: %#v", cols)
	}
	if cols[0].Width != 35 || cols[1].Width != 35 || cols[2].Width != 24 || cols[3].Width != 48 {
		t.Fatalf(
			"unexpected fixed widths: slug=%d subject=%d focused=%d step=%d",
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
		actionCursor: sessionActionCursorFocus,
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
				{Title: "FOCUSED", Width: 1},
				{Title: "STEP", Width: 1},
				{Title: "NEXT", Width: 1},
			}),
			table.WithRows([]table.Row{{"18-02-2026-boehmer", "", "", "", ""}}),
		),
	}

	empty := browseEntry{kind: browseEntryEmpty}
	slug, subject, focused, step, next := m.renderEntryRow(empty)
	if slug != "no open sessions" || subject != "" || focused != "" || step != "" || next != "" {
		t.Fatalf("unexpected empty row snapshot: %q | %q | %q | %q | %q", slug, subject, focused, step, next)
	}

	rec := sessionRecord{
		Slug:          "18-02-2026-boehmer",
		SubjectNames:  []string{"Cameron Boehmer"},
		CurrentStep:   "Ground",
		ProgressSteps: 2,
		StepCount:     3,
		NextStep:      "Second Exposure",
	}
	slug, subject, focused, step, next = m.renderEntryRow(browseEntry{kind: browseEntrySession, record: rec})
	if slug != "18-02-2026-boehmer" || subject != "Cameron Boehmer" || step != "[2/3] Ground" {
		t.Fatalf("unexpected unarmed row snapshot: %q | %q | %q", slug, subject, step)
	}
	if strings.TrimSpace(stripInternalMarkers(stripANSI(focused))) != "{focus}" {
		t.Fatalf("unexpected focused focused-cell snapshot: %q", stripInternalMarkers(stripANSI(focused)))
	}
	if strings.TrimSpace(stripInternalMarkers(stripANSI(next))) != "Second Exposure" {
		t.Fatalf("unexpected unfocused next-step snapshot: %q", stripInternalMarkers(stripANSI(next)))
	}

	m.actionCursor = sessionActionCursorNextStep
	_, _, focused, _, next = m.renderEntryRow(browseEntry{kind: browseEntrySession, record: rec})
	if strings.TrimSpace(stripInternalMarkers(stripANSI(focused))) != "" {
		t.Fatalf("unexpected unfocused focused-cell snapshot: %q", stripInternalMarkers(stripANSI(focused)))
	}
	if strings.TrimSpace(stripInternalMarkers(stripANSI(next))) != "{Second Exposure}" {
		t.Fatalf("unexpected focused next-step snapshot: %q", stripInternalMarkers(stripANSI(next)))
	}
}

func TestSessionsUI_FocusInvariant_ExactlyOneActionCellHighlighted(t *testing.T) {
	m := sessionsSwitchboardModel{
		actionCursor: sessionActionCursorFocus,
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
				{Title: "FOCUSED", Width: 1},
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

	_, _, focused, step, next := m.renderEntryRow(browseEntry{kind: browseEntrySession, record: rec})
	assertSingleFocusedActionCell(t, focused, step, next)
	if strings.Contains(stripANSI(step), "{") || strings.Contains(stripANSI(step), "}") {
		t.Fatalf("step column must never show focus markers, got %q", stripANSI(step))
	}

	m.actionCursor = sessionActionCursorNextStep
	_, _, focused, step, next = m.renderEntryRow(browseEntry{kind: browseEntrySession, record: rec})
	assertSingleFocusedActionCell(t, focused, step, next)
	if strings.Contains(stripANSI(step), "{") || strings.Contains(stripANSI(step), "}") {
		t.Fatalf("step column must never show focus markers, got %q", stripANSI(step))
	}
}

func TestSessionsUI_Interaction_RightLeftMovesSingleVisibleFocus(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:         sessionsViewBrowse,
		actionCursor: sessionActionCursorFocus,
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
				{Title: "FOCUSED", Width: 1},
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
	m.table.SetWidth(120)
	m.applyBrowseEntries()

	_, _, focused, step, next := m.renderEntryRow(browseEntry{kind: browseEntrySession, record: rec})
	assertSingleFocusedActionCell(t, focused, step, next)

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(sessionsSwitchboardModel)
	_, _, focused, step, next = m.renderEntryRow(browseEntry{kind: browseEntrySession, record: rec})
	assertSingleFocusedActionCell(t, focused, step, next)

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(sessionsSwitchboardModel)
	_, _, focused, step, next = m.renderEntryRow(browseEntry{kind: browseEntrySession, record: rec})
	assertSingleFocusedActionCell(t, focused, step, next)
}

func TestSessionsUI_TableRowMapping_ActiveAndStepDoNotShift(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:         sessionsViewBrowse,
		actionCursor: sessionActionCursorFocus,
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "SLUG", Width: 20},
				{Title: "SUBJECT", Width: 20},
				{Title: "FOCUSED", Width: 12},
				{Title: "STEP", Width: 24},
				{Title: "NEXT", Width: 16},
			}),
			table.WithHeight(4),
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
	if stripInternalMarkers(rows[0][2]) != "{focused}" {
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
	if stripInternalMarkers(rows[0][2]) != "focused" {
		t.Fatalf("expected ACTIVE column unfocused text in col 3, got %q", stripInternalMarkers(rows[0][2]))
	}
	if stripInternalMarkers(rows[0][4]) != "{conclude}" {
		t.Fatalf("expected NEXT column focus token in col 5, got %q", stripInternalMarkers(rows[0][4]))
	}
}

func TestSessionsUI_FocusedSessionPinnedToTop(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:              sessionsViewBrowse,
		actionCursor:      sessionActionCursorFocus,
		activeSessionSlug: "s2",
		filter:            newSessionsFilterInput(),
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "SLUG", Width: 20},
				{Title: "SUBJECT", Width: 20},
				{Title: "FOCUSED", Width: 12},
				{Title: "STEP", Width: 24},
				{Title: "NEXT", Width: 16},
			}),
			table.WithHeight(4),
		),
		browseRecords: []sessionRecord{
			{Slug: "s1", SubjectNames: []string{"Alpha"}, NextStep: "Second", ProgressSteps: 1, StepCount: 3},
			{Slug: "s2", SubjectNames: []string{"Beta"}, NextStep: "Second", ProgressSteps: 1, StepCount: 3, Active: true},
		},
	}
	m.applyBrowseEntries()
	rows := m.table.Rows()
	if len(rows) < 2 {
		t.Fatalf("expected at least 2 rows, got %d", len(rows))
	}
	if rows[0][0] != "s2" {
		t.Fatalf("expected focused session s2 pinned to top, got first row %q", rows[0][0])
	}
}

func TestSessionsUI_ViewStylesFocusedTokensPostRender(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:         sessionsViewBrowse,
		actionCursor: sessionActionCursorFocus,
		filter:       newSessionsFilterInput(),
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "SLUG", Width: 20},
				{Title: "SUBJECT", Width: 20},
				{Title: "FOCUSED", Width: 12},
				{Title: "STEP", Width: 24},
				{Title: "NEXT", Width: 16},
			}),
			table.WithHeight(4),
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
	m.table.SetWidth(120)
	m.applyBrowseEntries()
	out := m.View().Content
	if !strings.Contains(out, "{focus}") {
		t.Fatalf("expected focused token in view output, got:\n%s", stripANSI(out))
	}
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("expected ANSI styling in view output for focused token")
	}
}

func TestSessionsUI_View_DefaultActionCellsAreStyledAsCTA(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:         sessionsViewBrowse,
		actionCursor: sessionActionCursorFocus,
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
				{Title: "FOCUSED", Width: 12},
				{Title: "STEP", Width: 24},
				{Title: "NEXT", Width: 16},
			}),
			table.WithRows([]table.Row{{"s1", "", "", "", ""}}),
			table.WithHeight(4),
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
	m.table.SetWidth(120)
	m.applyBrowseEntries()

	out := m.View().Content
	if count := strings.Count(out, actionCellANSIPrefix); count < 2 {
		t.Fatalf("expected CTA styling in both ACTIVE and NEXT cells, prefix_count=%d", count)
	}
}

func TestSessionsUI_View_LayoutShowsHeaderFocusedAndFilterOrder(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:              sessionsViewBrowse,
		actionCursor:      sessionActionCursorFocus,
		activeSessionSlug: "s-focused",
		filter:            newSessionsFilterInput(),
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "SLUG", Width: 20},
				{Title: "SUBJECT", Width: 20},
				{Title: "FOCUSED", Width: 12},
				{Title: "STEP", Width: 24},
				{Title: "NEXT", Width: 16},
			}),
			table.WithRows([]table.Row{{"s-focused", "Alpha", "focused", "[1/2] Ground", "Second"}}),
			table.WithHeight(4),
		),
	}
	out := stripANSI(m.View().Content)
	wantOrder := []string{
		"Sessions",
		"focused session: s-focused",
		" filter: ",
	}
	last := -1
	for _, token := range wantOrder {
		idx := strings.Index(out, token)
		if idx < 0 {
			t.Fatalf("expected token %q in view output:\n%s", token, out)
		}
		if idx < last {
			t.Fatalf("expected token order %v, got output:\n%s", wantOrder, out)
		}
		last = idx
	}
}

func TestSessionsUI_DefaultActionCursorIsActiveColumn(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:         sessionsViewBrowse,
		actionCursor: sessionActionCursorFocus,
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "SLUG", Width: 1},
				{Title: "SUBJECT", Width: 1},
				{Title: "FOCUSED", Width: 1},
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

	if m.actionCursor != sessionActionCursorFocus {
		t.Fatalf("expected default action cursor in ACTIVE column, got %q", m.actionCursor)
	}
}

func TestSessionsUI_ActionCursorMovesLeftRight(t *testing.T) {
	m := sessionsSwitchboardModel{
		actionCursor: sessionActionCursorFocus,
	}

	m.moveActionCursorRight()
	if m.actionCursor != sessionActionCursorNextStep {
		t.Fatalf("expected right-arrow to move cursor to NEXT STEP, got %q", m.actionCursor)
	}

	m.moveActionCursorLeft()
	if m.actionCursor != sessionActionCursorFocus {
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
		actionCursor: sessionActionCursorFocus,
		filter:       newSessionsFilterInput(),
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "SLUG", Width: 1},
				{Title: "SUBJECT", Width: 1},
				{Title: "FOCUSED", Width: 1},
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
				{Title: "FOCUSED", Width: 1},
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

func TestSessionsUI_FocusSwitch_ClosesPreviousWindowAndOpensNewWindow(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SG_SUBJECT_DIR", filepath.Join(root, ".subjects"))
	protocol := testProtocol()
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\nactive_session_slug: s1\n---\n\n# Study\n")
	mustWriteSessionFile(t, root, "s1", map[string]any{
		"subject_ids": []string{"sub-1"},
	})
	mustWriteSessionFile(t, root, "s2", map[string]any{
		"subject_ids": []string{"sub-2"},
	})
	mustWriteStepFile(t, filepath.Join(root, "session", "s1", "step", "first-step", "step.sg.md"), map[string]any{
		"time_started": "10:01:00 01-01-2026",
		"focus_windows": []map[string]any{
			{"time_started": "10:01:00 01-01-2026"},
		},
	}, "")
	mustWriteStepFile(t, filepath.Join(root, "session", "s2", "step", "first-step", "step.sg.md"), map[string]any{
		"time_started": "10:02:00 01-01-2026",
	}, "")

	m := sessionsSwitchboardModel{
		root:              root,
		protocol:          protocol,
		view:              sessionsViewBrowse,
		actionCursor:      sessionActionCursorFocus,
		activeSessionSlug: "s1",
		filter:            newSessionsFilterInput(),
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "SLUG", Width: 1},
				{Title: "SUBJECT", Width: 1},
				{Title: "FOCUSED", Width: 1},
				{Title: "STEP", Width: 1},
				{Title: "NEXT", Width: 1},
			}),
			table.WithRows([]table.Row{{"s2", "", "", "", ""}}),
		),
		browseEntries: []browseEntry{
			{
				kind: browseEntrySession,
				record: sessionRecord{
					Slug:          "s2",
					NextAction:    "advance",
					ProgressSteps: 1,
				},
			},
		},
	}

	_, _ = m.handleBrowseEnter()

	s1FM, _, err := util.ReadFrontmatterFile(filepath.Join(root, "session", "s1", "step", "first-step", "step.sg.md"))
	if err != nil {
		t.Fatalf("read s1 step failed: %v", err)
	}
	s1Windows := focusWindowsFromFM(s1FM)
	if len(s1Windows) != 1 {
		t.Fatalf("expected 1 focus window for s1, got %d", len(s1Windows))
	}
	if strings.TrimSpace(asString(s1Windows[0]["time_finished"])) == "" {
		t.Fatalf("expected s1 focus window to be closed after switching focus")
	}

	s2FM, _, err := util.ReadFrontmatterFile(filepath.Join(root, "session", "s2", "step", "first-step", "step.sg.md"))
	if err != nil {
		t.Fatalf("read s2 step failed: %v", err)
	}
	s2Windows := focusWindowsFromFM(s2FM)
	if len(s2Windows) != 1 {
		t.Fatalf("expected 1 focus window for s2, got %d", len(s2Windows))
	}
	if strings.TrimSpace(asString(s2Windows[0]["time_started"])) == "" {
		t.Fatalf("expected s2 focus window to be opened on focus")
	}
	if strings.TrimSpace(asString(s2Windows[0]["time_finished"])) != "" {
		t.Fatalf("expected s2 focus window to remain open while focused")
	}
}

func TestSessionsUI_CtrlBReversesSelectedSessionStep(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SG_SUBJECT_DIR", filepath.Join(root, ".subjects"))
	protocol := testProtocol()
	slug := "01-01-2026-alpha"
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\n---\n\n# Study\n")
	mustWriteSessionFile(t, root, slug, map[string]any{
		"subject_ids": []string{"sub-1"},
	})
	mustWriteStepFile(t, filepath.Join(root, "session", slug, "step", "first-step", "step.sg.md"), map[string]any{
		"time_started": "10:01:00 01-01-2026",
	}, "")

	m := sessionsSwitchboardModel{
		root:         root,
		protocol:     protocol,
		view:         sessionsViewBrowse,
		actionCursor: sessionActionCursorFocus,
		filter:       newSessionsFilterInput(),
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "SLUG", Width: 1},
				{Title: "SUBJECT", Width: 1},
				{Title: "FOCUSED", Width: 1},
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
					NextAction: "advance",
				},
			},
		},
	}

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	got := updated.(sessionsSwitchboardModel)
	if !strings.Contains(got.message, "state=reversed") {
		t.Fatalf("expected reversed transition message, got %q", got.message)
	}

	stepFM, _, err := util.ReadFrontmatterFile(filepath.Join(root, "session", slug, "step", "first-step", "step.sg.md"))
	if err != nil {
		t.Fatalf("read first step failed: %v", err)
	}
	if strings.TrimSpace(asString(stepFM["time_started"])) != "" {
		t.Fatalf("expected first step time_started to be cleared")
	}
}

func TestSessionsUI_CreateViewSnapshot(t *testing.T) {
	m := sessionsSwitchboardModel{
		subjects:          []store.Subject{{UUID: "abc12345-0000-0000-0000-000000000000", Name: "Alpha Subject"}},
		selectedBySubject: map[string]bool{},
	}
	m.refreshCreateList()
	out := stripANSI(m.View().Content)

	expectedInOrder := []string{
		"Create Session",
		"  " + sessionsCreateInfoText,
		"  [ ] Alpha Subject (abc12345)",
		"  (+) New subject",
		"  -> Create Session",
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
		if trimmed == "(+) New subject" && strings.HasPrefix(line, "  ") {
			createSubjectLine = i
		}
		if trimmed == "-> Create Session" && strings.HasPrefix(line, "  ") {
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
	switchboardView := stripANSI(m.View().Content)

	picker := newSessionCreatePickerModel(subjects, selected)
	pickerView := stripANSI(picker.View().Content)

	expectedInOrder := []string{
		"Create Session",
		"  " + sessionsCreateInfoText,
		"  [ ] Alpha Subject (abc12345)",
		"  (+) New subject",
		"  -> Create Session",
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

func TestSessionsUI_ViewShowsPublishHintWhenEligible(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:               sessionsViewBrowse,
		filter:             newSessionsFilterInput(),
		table:              table.New(),
		finishedSessionCount:   2,
		inProgressSessionCount: 0,
	}
	out := stripANSI(m.View().Content)
	if !strings.Contains(out, "p publish with 2 sessions") {
		t.Fatalf("expected publish hint in browse footer, got:\n%s", out)
	}
}

func TestSessionsUI_ViewHidesPublishHintWhenIneligible(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:               sessionsViewBrowse,
		filter:             newSessionsFilterInput(),
		table:              table.New(),
		finishedSessionCount:   2,
		inProgressSessionCount: 1,
	}
	out := stripANSI(m.View().Content)
	if strings.Contains(out, "p publish with") {
		t.Fatalf("did not expect publish hint in browse footer, got:\n%s", out)
	}
}

func TestSessionsUI_KeyPTriggersPublishAlways(t *testing.T) {
	publishCalls := 0
	m := sessionsSwitchboardModel{
		view:                   sessionsViewBrowse,
		filter:                 newSessionsFilterInput(),
		table:                  table.New(),
		finishedSessionCount:   1,
		inProgressSessionCount: 0,
		publishFunc: func(string) error {
			publishCalls++
			return nil
		},
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated.(sessionsSwitchboardModel)
	if publishCalls != 1 {
		t.Fatalf("expected publish to run once when eligible, got %d", publishCalls)
	}

	m.finishedSessionCount = 1
	m.inProgressSessionCount = 1
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated.(sessionsSwitchboardModel)
	if publishCalls != 2 {
		t.Fatalf("expected publish to run even when hint is hidden, got %d calls", publishCalls)
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
			"expected exactly one focused actionable cell, got=%d focused=%q step=%q next=%q",
			focused,
			activePlain,
			stepPlain,
			nextPlain,
		)
	}
}

func focusWindowsFromFM(fm map[string]any) []map[string]any {
	raw, ok := fm["focus_windows"]
	if !ok {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, m)
	}
	return out
}
