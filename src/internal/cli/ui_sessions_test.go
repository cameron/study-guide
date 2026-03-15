package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"study-guide/src/internal/store"
	"study-guide/src/internal/util"
)

func testSessionsBrowseColumns() []table.Column {
	return []table.Column{
		{Title: "SUBJECT", Width: 20},
		{Title: "CURRENT STEP", Width: 24},
		{Title: "NEXT STEP", Width: 16},
	}
}

func testSessionsBrowseRow(subject, current, next string) table.Row {
	return table.Row{subject, current, next}
}

func TestSessionsUI_SelectedRowStyle_UsesDefaultPinkWithoutBackgroundTint(t *testing.T) {
	base := table.DefaultStyles().Selected
	st := sessionsSelectedRowStyle(base)
	if fmt.Sprint(st.GetForeground()) != fmt.Sprint(base.GetForeground()) {
		t.Fatalf("expected selected row to keep Bubble default pink foreground, got %q", fmt.Sprint(st.GetForeground()))
	}
	if _, ok := st.GetBackground().(lipgloss.NoColor); !ok {
		t.Fatalf("expected selected row to avoid custom row background tint, got %T", st.GetBackground())
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
	view := fi.View()
	if !regexp.MustCompile(`\x1b\[[0-9;]*38;2;(120;240;255|20;144;160)m`).MatchString(view) {
		t.Fatalf("expected turquoise prompt styling in filter input, got %q", view)
	}
	if !regexp.MustCompile(`\x1b\[[0-9;]*38;5;245m`).MatchString(view) {
		t.Fatalf("expected dim placeholder styling in filter input, got %q", view)
	}
	fi.SetValue("alp")
	typedView := fi.View()
	if !regexp.MustCompile(`\x1b\[[0-9;]*38;5;212m`).MatchString(typedView) {
		t.Fatalf("expected pink query styling in filter input when non-empty, got %q", typedView)
	}
}

func TestSessionsUI_CreateFilterInput_UsesSharedAccentStyles(t *testing.T) {
	m := sessionsSwitchboardModel{
		subjects:          []store.Subject{{UUID: "abc12345-0000-0000-0000-000000000000", Name: "Alpha"}},
		selectedBySubject: map[string]bool{},
	}
	m.refreshCreateList()
	styles := m.list.FilterInput.Styles()
	if !regexp.MustCompile(`\x1b\[[0-9;]*38;2;(120;240;255|20;144;160)m`).MatchString(styles.Focused.Prompt.Render("Filter: ")) {
		t.Fatalf("expected turquoise prompt styling in create filter input")
	}
	if !regexp.MustCompile(`\x1b\[[0-9;]*38;5;245m`).MatchString(styles.Focused.Placeholder.Render("by subject name")) {
		t.Fatalf("expected dim placeholder styling in create filter input")
	}
}

func TestSessionCreatePicker_FilterInput_UsesSharedAccentStyles(t *testing.T) {
	m := newSessionCreatePickerModel(
		[]store.Subject{{UUID: "abc12345-0000-0000-0000-000000000000", Name: "Alpha"}},
		map[string]bool{},
	)
	styles := m.list.FilterInput.Styles()
	if !regexp.MustCompile(`\x1b\[[0-9;]*38;2;(120;240;255|20;144;160)m`).MatchString(styles.Focused.Prompt.Render("Filter: ")) {
		t.Fatalf("expected turquoise prompt styling in shared picker filter input")
	}
	if !regexp.MustCompile(`\x1b\[[0-9;]*38;5;245m`).MatchString(styles.Focused.Placeholder.Render("by subject name")) {
		t.Fatalf("expected dim placeholder styling in shared picker filter input")
	}
}

func TestSessionsUI_BrowseLayoutWidths(t *testing.T) {
	m := sessionsSwitchboardModel{
		table: table.New(),
		width: 220,
	}
	m.applyBrowseTableLayout()
	cols := m.table.Columns()
	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(cols))
	}
	if cols[0].Title != "SUBJECT" || cols[1].Title != "CURRENT STEP" || cols[2].Title != "NEXT STEP" {
		t.Fatalf("unexpected browse headers: %#v", cols)
	}
	if cols[0].Width != 35 || cols[1].Width != 48 {
		t.Fatalf(
			"unexpected fixed widths: subject=%d current=%d",
			cols[0].Width,
			cols[1].Width,
		)
	}
	if cols[2].Width < 16 {
		t.Fatalf("expected NEXT STEP width >= 16, got %d", cols[2].Width)
	}
}

func TestSessionsUI_BrowseLayoutFitsNarrowViewports(t *testing.T) {
	m := sessionsSwitchboardModel{
		table: table.New(),
		width: 120,
	}
	m.applyBrowseTableLayout()
	cols := m.table.Columns()
	totalCols := cols[0].Width + cols[1].Width + cols[2].Width + 8
	if totalCols > 120 {
		t.Fatalf("expected columns to fit viewport, got total=%d viewport=120", totalCols)
	}
}

func TestSessionsUI_BrowseRowSnapshots(t *testing.T) {
	m := sessionsSwitchboardModel{
		browseEntries: []browseEntry{
			{
				kind: browseEntrySession,
				record: sessionRecord{
					Slug: "18-02-2026-boehmer",
				},
			},
		},
		table: table.New(
			table.WithColumns(testSessionsBrowseColumns()),
			table.WithRows([]table.Row{testSessionsBrowseRow("Cameron Boehmer", "", "")}),
		),
	}

	empty := browseEntry{kind: browseEntryEmpty}
	subject, current, next := m.renderEntryRow(empty)
	if subject != "no open sessions" || current != "" || next != "" {
		t.Fatalf("unexpected empty row snapshot: %q | %q | %q", subject, current, next)
	}

	rec := sessionRecord{
		Slug:          "18-02-2026-boehmer",
		SubjectNames:  []string{"Cameron Boehmer"},
		CurrentStep:   "Ground",
		ProgressSteps: 2,
		StepCount:     3,
		NextStep:      "Second Exposure",
	}
	subject, current, next = m.renderEntryRow(browseEntry{kind: browseEntrySession, record: rec})
	if subject != "Cameron Boehmer" || current != "[2/3] Ground" || next != "Second Exposure" {
		t.Fatalf("unexpected row snapshot: %q | %q | %q", subject, current, next)
	}
}

func TestSessionsUI_EnterOnSelectedRowFocusesAndStartsFirstStep(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SG_SUBJECT_DIR", filepath.Join(root, ".subjects"))
	protocol := testProtocol()
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\n---\n\n# Study\n")
	mustWriteSessionFile(t, root, "s1", map[string]any{
		"time_started": "10:00:00 01-01-2026",
		"subject_ids":  []string{"sub-1"},
	})

	m := sessionsSwitchboardModel{
		root:     root,
		protocol: protocol,
		view:     sessionsViewBrowse,
		filter:   newSessionsFilterInput(),
		table: table.New(
			table.WithColumns(testSessionsBrowseColumns()),
			table.WithRows([]table.Row{testSessionsBrowseRow("", "", "")}),
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
		t.Fatalf("expected first step to start on enter")
	}
}

func TestSessionsUI_EnterOnFocusedRowAdvancesOnce(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SG_SUBJECT_DIR", filepath.Join(root, ".subjects"))
	protocol := testProtocol()
	slug := "01-01-2026-alpha"
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\nactive_session_slug: "+slug+"\n---\n\n# Study\n")
	mustWriteSessionFile(t, root, slug, map[string]any{
		"time_started": "10:00:00 01-01-2026",
		"subject_ids":  []string{"sub-1"},
	})
	mustWriteStepFile(t, filepath.Join(root, "session", slug, "step", "first-step", "step.sg.md"), map[string]any{
		"time_started": "10:01:00 01-01-2026",
		"focus_windows": []map[string]any{
			{"time_started": "10:01:00 01-01-2026"},
		},
	}, "")

	m := sessionsSwitchboardModel{
		root:              root,
		protocol:          protocol,
		view:              sessionsViewBrowse,
		activeSessionSlug: slug,
		filter:            newSessionsFilterInput(),
		table: table.New(
			table.WithColumns(testSessionsBrowseColumns()),
			table.WithRows([]table.Row{testSessionsBrowseRow("", "", "")}),
		),
		browseEntries: []browseEntry{
			{
				kind: browseEntrySession,
				record: sessionRecord{
					Slug:          slug,
					NextAction:    "advance",
					ProgressSteps: 1,
					Active:        true,
				},
			},
		},
	}

	_, _ = m.handleBrowseEnter()

	firstStepFM, _, err := util.ReadFrontmatterFile(filepath.Join(root, "session", slug, "step", "first-step", "step.sg.md"))
	if err != nil {
		t.Fatalf("read first step failed: %v", err)
	}
	if strings.TrimSpace(asString(firstStepFM["time_finished"])) == "" {
		t.Fatalf("expected first step to finish on enter")
	}
	secondStepFM, _, err := util.ReadFrontmatterFile(filepath.Join(root, "session", slug, "step", "second-step", "step.sg.md"))
	if err != nil {
		t.Fatalf("read second step failed: %v", err)
	}
	if strings.TrimSpace(asString(secondStepFM["time_started"])) == "" {
		t.Fatalf("expected second step to start on enter")
	}
}

func TestSessionsUI_EscUnfocusesActiveSessionAndClosesWindow(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SG_SUBJECT_DIR", filepath.Join(root, ".subjects"))
	protocol := testProtocol()
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\nactive_session_slug: s1\n---\n\n# Study\n")
	mustWriteSessionFile(t, root, "s1", map[string]any{
		"subject_ids": []string{"sub-1"},
	})
	mustWriteStepFile(t, filepath.Join(root, "session", "s1", "step", "first-step", "step.sg.md"), map[string]any{
		"time_started": "10:01:00 01-01-2026",
		"focus_windows": []map[string]any{
			{"time_started": "10:01:00 01-01-2026"},
		},
	}, "")

	m := sessionsSwitchboardModel{
		root:              root,
		protocol:          protocol,
		view:              sessionsViewBrowse,
		activeSessionSlug: "s1",
		filter:            newSessionsFilterInput(),
		table: table.New(
			table.WithColumns(testSessionsBrowseColumns()),
			table.WithRows([]table.Row{testSessionsBrowseRow("", "", "")}),
		),
		browseEntries: []browseEntry{
			{
				kind: browseEntrySession,
				record: sessionRecord{
					Slug:          "s1",
					NextAction:    "advance",
					ProgressSteps: 1,
					Active:        true,
				},
			},
		},
	}

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd != nil {
		t.Fatalf("expected esc with focused session to stay in program")
	}
	got := updated.(sessionsSwitchboardModel)

	fm, _, err := util.ReadFrontmatterFile(filepath.Join(root, "study.sg.md"))
	if err != nil {
		t.Fatalf("read study frontmatter failed: %v", err)
	}
	if gotSlug := strings.TrimSpace(asString(fm["active_session_slug"])); gotSlug != "" {
		t.Fatalf("expected active_session_slug to be cleared, got %q", gotSlug)
	}

	stepFM, _, err := util.ReadFrontmatterFile(filepath.Join(root, "session", "s1", "step", "first-step", "step.sg.md"))
	if err != nil {
		t.Fatalf("read first step frontmatter failed: %v", err)
	}
	windows := focusWindowsFromFM(stepFM)
	if len(windows) != 1 {
		t.Fatalf("expected 1 focus window for s1, got %d", len(windows))
	}
	if strings.TrimSpace(asString(windows[0]["time_finished"])) == "" {
		t.Fatalf("expected s1 focus window to be closed after unfocus")
	}
	if got.activeSessionSlug != "" {
		t.Fatalf("expected model activeSessionSlug to be cleared, got %q", got.activeSessionSlug)
	}
}

func TestSessionsUI_EscQuitsWhenNoSessionIsFocused(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:   sessionsViewBrowse,
		filter: newSessionsFilterInput(),
		table: table.New(
			table.WithColumns(testSessionsBrowseColumns()),
			table.WithRows([]table.Row{testSessionsBrowseRow("Alpha", "[1/2] Ground", "Second")}),
		),
	}

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatalf("expected esc without a focused session to quit")
	}
}

func TestSessionsUI_FocusedSessionPinnedToTop(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:              sessionsViewBrowse,
		activeSessionSlug: "s2",
		filter:            newSessionsFilterInput(),
		table: table.New(
			table.WithColumns(testSessionsBrowseColumns()),
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
	if stripANSI(rows[0][0]) != "Beta" {
		t.Fatalf("expected focused session s2 pinned to top, got first row %q", rows[0][0])
	}
}

func TestSessionsUI_FocusedRowUsesDistinctANSIStyling(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:   sessionsViewBrowse,
		filter: newSessionsFilterInput(),
		table: table.New(
			table.WithColumns(testSessionsBrowseColumns()),
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
				Active:        true,
			},
		},
		activeSessionSlug: "s1",
	}
	m.table.SetWidth(120)
	m.applyBrowseEntries()
	row := strings.Join(m.table.Rows()[0], "")
	if countBackgroundANSI(row) < 1 {
		t.Fatalf("expected focused row to include background ANSI styling, got %q", row)
	}
}

func TestSessionsUI_View_LayoutOmitsStatusLinesButShowsKeyHints(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:              sessionsViewBrowse,
		activeSessionSlug: "s-focused",
		filter:            newSessionsFilterInput(),
		table: table.New(
			table.WithColumns(testSessionsBrowseColumns()),
			table.WithRows([]table.Row{testSessionsBrowseRow("Alpha", "[1/2] Ground", "Second")}),
			table.WithHeight(4),
		),
	}
	out := stripANSI(m.View().Content)
	if !strings.Contains(out, "Sessions") {
		t.Fatalf("expected Sessions title in output:\n%s", out)
	}
	firstLine := strings.SplitN(out, "\n", 2)[0]
	if !strings.Contains(firstLine, "[enter] next step // [ctrl+b] step backwards // [ctrl+a] open assets // [ctrl+n] create session // [p] publish // [esc] unfocus/quit") {
		t.Fatalf("expected browse key hint on title line, got:\n%s", out)
	}
	if !strings.Contains(out, " filter: ") {
		t.Fatalf("expected filter input in output:\n%s", out)
	}
	if strings.Contains(out, "enter to activate cell; ctrl+b to step backwards; ctrl+n to create session; p to publish; esc to quit") {
		t.Fatalf("did not expect legacy browse key hint footer in output:\n%s", out)
	}
	for _, hidden := range []string{
		"focused session:",
		"current step:",
	} {
		if strings.Contains(out, hidden) {
			t.Fatalf("did not expect %q in browse output:\n%s", hidden, out)
		}
	}
}

func TestSessionsUI_View_LayoutOmitsSessionStateMessageLine(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:              sessionsViewBrowse,
		activeSessionSlug: "s-focused",
		message:           "session=s-focused state=advanced step=ground",
		filter:            newSessionsFilterInput(),
		table: table.New(
			table.WithColumns(testSessionsBrowseColumns()),
			table.WithRows([]table.Row{testSessionsBrowseRow("Alpha", "[1/2] Ground", "Second")}),
			table.WithHeight(4),
		),
	}

	out := stripANSI(m.View().Content)
	if strings.Contains(out, "session=s-focused state=advanced step=ground") {
		t.Fatalf("did not expect session state message line in browse output:\n%s", out)
	}
}

func TestSessionsUI_BrowseView_TitleHasBackgroundStyle(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:   sessionsViewBrowse,
		filter: newSessionsFilterInput(),
		table:  table.New(),
	}
	out := m.View().Content
	firstLine := strings.SplitN(out, "\n", 2)[0]
	if countBackgroundANSI(firstLine) < 1 {
		t.Fatalf("expected browse title line to include background ANSI style, got %q", firstLine)
	}
}

func TestSessionsUI_BrowseFilter_MatchesSessionSlugWithoutVisibleSlugColumn(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:   sessionsViewBrowse,
		filter: newSessionsFilterInput(),
		table: table.New(
			table.WithColumns(testSessionsBrowseColumns()),
			table.WithRows([]table.Row{}),
		),
		browseRecords: []sessionRecord{
			{Slug: "18-02-2026-boehmer", SubjectNames: []string{"Cameron Boehmer"}, NextStep: "Second Exposure", ProgressSteps: 1, StepCount: 3},
			{Slug: "19-02-2026-alpha", SubjectNames: []string{"Alice Alpha"}, NextStep: "Second Exposure", ProgressSteps: 1, StepCount: 3},
		},
	}

	m.filter.SetValue("boehmer")
	m.applyBrowseEntries()

	rows := m.table.Rows()
	if len(rows) != 1 {
		t.Fatalf("expected one filtered row, got %d", len(rows))
	}
	if rows[0][0] != "Cameron Boehmer" {
		t.Fatalf("expected slug filter to retain matching subject row, got %q", rows[0][0])
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
		activeSessionSlug: "s1",
		filter:            newSessionsFilterInput(),
		table: table.New(
			table.WithColumns(testSessionsBrowseColumns()),
			table.WithRows([]table.Row{testSessionsBrowseRow("", "", "")}),
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

	secondStepFM, _, err := util.ReadFrontmatterFile(filepath.Join(root, "session", "s2", "step", "second-step", "step.sg.md"))
	if err != nil {
		t.Fatalf("read s2 second step failed: %v", err)
	}
	s2Windows := focusWindowsFromFM(secondStepFM)
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

func TestSessionsUI_FocusSwitch_SelectsTopRowAfterFocusedSessionPinned(t *testing.T) {
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
	}, "")
	mustWriteStepFile(t, filepath.Join(root, "session", "s2", "step", "first-step", "step.sg.md"), map[string]any{
		"time_started": "10:02:00 01-01-2026",
	}, "")

	m := sessionsSwitchboardModel{
		root:              root,
		protocol:          protocol,
		view:              sessionsViewBrowse,
		activeSessionSlug: "s1",
		filter:            newSessionsFilterInput(),
		table: table.New(
			table.WithColumns(testSessionsBrowseColumns()),
			table.WithRows([]table.Row{
				testSessionsBrowseRow("", "", ""),
				testSessionsBrowseRow("", "", ""),
			}),
		),
		browseEntries: []browseEntry{
			{
				kind: browseEntrySession,
				record: sessionRecord{
					Slug:       "s1",
					NextAction: "advance",
					Active:     true,
				},
			},
			{
				kind: browseEntrySession,
				record: sessionRecord{
					Slug:       "s2",
					NextAction: "advance",
				},
			},
		},
	}
	m.table.SetCursor(1)

	updated, _ := m.handleBrowseEnter()
	got := updated.(sessionsSwitchboardModel)
	if got.table.Cursor() != 0 {
		t.Fatalf("expected selected row cursor to move to top after focusing, got %d", got.table.Cursor())
	}
	entry, ok := got.selectedBrowseEntry()
	if !ok || entry.kind != browseEntrySession {
		t.Fatalf("expected selected browse entry to exist")
	}
	if entry.record.Slug != "s2" {
		t.Fatalf("expected selected row to be focused top session s2, got %q", entry.record.Slug)
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
		root:     root,
		protocol: protocol,
		view:     sessionsViewBrowse,
		filter:   newSessionsFilterInput(),
		table: table.New(
			table.WithColumns(testSessionsBrowseColumns()),
			table.WithRows([]table.Row{testSessionsBrowseRow("", "", "")}),
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

func TestSessionsUI_CtrlAOpensSelectedSessionCurrentStepAssetFolder(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SG_SUBJECT_DIR", filepath.Join(root, ".subjects"))
	protocol := testProtocol()
	slug := "01-01-2026-alpha"
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\nactive_session_slug: "+slug+"\n---\n\n# Study\n")
	mustWriteSessionFile(t, root, slug, map[string]any{
		"subject_ids": []string{"sub-1"},
	})
	mustWriteStepFile(t, filepath.Join(root, "session", slug, "step", "first-step", "step.sg.md"), map[string]any{
		"time_started": "10:01:00 01-01-2026",
	}, "")

	var opened string
	m := sessionsSwitchboardModel{
		root:              root,
		protocol:          protocol,
		view:              sessionsViewBrowse,
		activeSessionSlug: slug,
		filter:            newSessionsFilterInput(),
		table: table.New(
			table.WithColumns(testSessionsBrowseColumns()),
			table.WithRows([]table.Row{testSessionsBrowseRow("", "", "")}),
		),
		browseEntries: []browseEntry{
			{
				kind: browseEntrySession,
				record: sessionRecord{
					Slug:          slug,
					NextAction:    "advance",
					ProgressSteps: 1,
					Active:        true,
				},
			},
		},
		openPathFunc: func(path string) error {
			opened = path
			return nil
		},
	}

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(sessionsSwitchboardModel)

	want := filepath.Join(root, "session", slug, "step", "first-step", "asset")
	if opened != want {
		t.Fatalf("expected opened asset path %q, got %q", want, opened)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected asset directory to exist, got %v", err)
	}
	if m.message != "opened assets: "+want {
		t.Fatalf("expected opened-assets message, got %q", m.message)
	}
}

func TestSessionsUI_CreateAndPicker_TitleHaveBackgroundStyle(t *testing.T) {
	subjects := []store.Subject{{UUID: "abc12345-0000-0000-0000-000000000000", Name: "Alpha Subject"}}

	switchboard := sessionsSwitchboardModel{
		subjects:          subjects,
		selectedBySubject: map[string]bool{},
	}
	switchboard.refreshCreateList()
	switchboardOut := switchboard.View().Content
	switchboardTitle := strings.SplitN(switchboardOut, "\n", 2)[0]
	if countBackgroundANSI(switchboardTitle) < 1 {
		t.Fatalf("expected sessions create title to include background ANSI style, got %q", switchboardTitle)
	}

	picker := newSessionCreatePickerModel(subjects, map[string]bool{})
	pickerOut := picker.View().Content
	pickerTitle := strings.SplitN(pickerOut, "\n", 2)[0]
	if countBackgroundANSI(pickerTitle) < 1 {
		t.Fatalf("expected shared picker title to include background ANSI style, got %q", pickerTitle)
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
		"  Alpha Subject (abc12345)",
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
		if trimmed == "Alpha Subject (abc12345)" {
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

func TestSessionsUI_CreateDelegateSelectedTitleKeepsDefaultPinkForeground(t *testing.T) {
	d := newCreateListDelegate()
	selectedFG := fmt.Sprint(d.Styles.SelectedTitle.GetForeground())
	if selectedFG != "{238 111 248 255}" {
		t.Fatalf("expected selected-title foreground to stay default pink, got %q", selectedFG)
	}
}

func TestCreateListShouldUseSelectedStyle(t *testing.T) {
	if createListShouldUseSelectedStyle(list.Filtering, "", true) {
		t.Fatalf("expected empty filtering query to use dimmed style, not selected")
	}
	if !createListShouldUseSelectedStyle(list.Filtering, "a", true) {
		t.Fatalf("expected non-empty filtering query to keep selected style")
	}
	if !createListShouldUseSelectedStyle(list.FilterApplied, "a", true) {
		t.Fatalf("expected applied filter state to keep selected style")
	}
	if createListShouldUseSelectedStyle(list.Filtering, "a", false) {
		t.Fatalf("did not expect unselected rows to use selected style")
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
		"  Alpha Subject (abc12345)",
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

func TestSessionsUI_CreateModeTypingStartsFuzzyAutocompleteBySubjectName(t *testing.T) {
	m := sessionsSwitchboardModel{
		subjects: []store.Subject{
			{UUID: "abc12345-0000-0000-0000-000000000000", Name: "Alpha Subject"},
			{UUID: "def67890-0000-0000-0000-000000000000", Name: "Beta Subject"},
		},
		selectedBySubject: map[string]bool{},
	}
	m.refreshCreateList()

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(sessionsSwitchboardModel)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	m = updated.(sessionsSwitchboardModel)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated.(sessionsSwitchboardModel)

	if !m.list.SettingFilter() {
		t.Fatalf("expected create-mode list to enter filtering state on typing")
	}
	if m.list.FilterValue() != "alp" {
		t.Fatalf("expected create-mode filter query to be alp, got %q", m.list.FilterValue())
	}
	items := m.list.Items()
	if len(items) < 2 {
		t.Fatalf("expected at least two subject items, got %d", len(items))
	}
	if items[0].FilterValue() != "Alpha Subject" || items[1].FilterValue() != "Beta Subject" {
		t.Fatalf("expected subject-name filter values, got %q and %q", items[0].FilterValue(), items[1].FilterValue())
	}
}

func TestSessionCreatePicker_TypingStartsFuzzyAutocompleteBySubjectName(t *testing.T) {
	m := newSessionCreatePickerModel(
		[]store.Subject{
			{UUID: "abc12345-0000-0000-0000-000000000000", Name: "Alpha Subject"},
			{UUID: "def67890-0000-0000-0000-000000000000", Name: "Beta Subject"},
		},
		map[string]bool{},
	)

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(sessionCreatePickerModel)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	m = updated.(sessionCreatePickerModel)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated.(sessionCreatePickerModel)

	if !m.list.SettingFilter() {
		t.Fatalf("expected shared create picker list to enter filtering state on typing")
	}
	if m.list.FilterValue() != "alp" {
		t.Fatalf("expected picker filter query to be alp, got %q", m.list.FilterValue())
	}
	items := m.list.Items()
	if len(items) < 2 {
		t.Fatalf("expected at least two subject items, got %d", len(items))
	}
	if items[0].FilterValue() != "Alpha Subject" || items[1].FilterValue() != "Beta Subject" {
		t.Fatalf("expected subject-name filter values, got %q and %q", items[0].FilterValue(), items[1].FilterValue())
	}
}

func TestSessionCreatePicker_ShiftEnterActsAsCreateShortcut(t *testing.T) {
	subjects := []store.Subject{{UUID: "abc12345-0000-0000-0000-000000000000", Name: "Alpha Subject"}}
	m := newSessionCreatePickerModel(subjects, map[string]bool{"abc12345-0000-0000-0000-000000000000": true})

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	m = updated.(sessionCreatePickerModel)

	if !m.done {
		t.Fatalf("expected shift+enter to complete picker as create shortcut")
	}
	if m.canceled {
		t.Fatalf("did not expect shift+enter to cancel picker")
	}
	if m.requestCreateSubject {
		t.Fatalf("did not expect shift+enter to trigger create-subject flow")
	}
}

func TestSessionsUI_CreateModeShiftEnterWithoutSelectionShowsCreateMessage(t *testing.T) {
	m := sessionsSwitchboardModel{
		view:              sessionsViewCreate,
		root:              t.TempDir(),
		subjects:          []store.Subject{{UUID: "abc12345-0000-0000-0000-000000000000", Name: "Alpha Subject"}},
		selectedBySubject: map[string]bool{},
	}
	m.refreshCreateList()

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	m = updated.(sessionsSwitchboardModel)

	if m.message != "select a subject before Create" {
		t.Fatalf("expected create validation message on shift+enter, got %q", m.message)
	}
}

func TestSessionsUI_CreateViewShowsFilterInputBeforeTyping(t *testing.T) {
	m := sessionsSwitchboardModel{
		subjects:          []store.Subject{{UUID: "abc12345-0000-0000-0000-000000000000", Name: "Alpha Subject"}},
		selectedBySubject: map[string]bool{},
	}
	m.refreshCreateList()

	out := stripANSI(m.View().Content)
	if !strings.Contains(out, "Filter:") {
		t.Fatalf("expected create-mode view to show filter input before typing, got:\n%s", out)
	}
}

func TestSessionCreatePicker_ViewShowsFilterInputBeforeTyping(t *testing.T) {
	m := newSessionCreatePickerModel(
		[]store.Subject{{UUID: "abc12345-0000-0000-0000-000000000000", Name: "Alpha Subject"}},
		map[string]bool{},
	)

	out := stripANSI(m.View().Content)
	if !strings.Contains(out, "Filter:") {
		t.Fatalf("expected shared picker view to show filter input before typing, got:\n%s", out)
	}
}

func TestSessionCreatePicker_ClearingFilterDoesNotDuplicateFilterLineAndKeepsRowsSelectable(t *testing.T) {
	m := newSessionCreatePickerModel(
		[]store.Subject{
			{UUID: "abc12345-0000-0000-0000-000000000000", Name: "Alpha Subject"},
			{UUID: "def67890-0000-0000-0000-000000000000", Name: "Beta Subject"},
		},
		map[string]bool{},
	)

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = updated.(sessionCreatePickerModel)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = updated.(sessionCreatePickerModel)

	if m.list.FilterValue() != "" {
		t.Fatalf("expected cleared filter value, got %q", m.list.FilterValue())
	}
	if got := strings.Count(stripANSI(m.View().Content), "Filter:"); got != 1 {
		t.Fatalf("expected exactly one Filter line after clear, got %d", got)
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(sessionCreatePickerModel)
	if len(m.SelectedSubjects()) != 1 {
		t.Fatalf("expected enter to choose currently selected subject after clear")
	}
}

func TestSessionsUI_CreateModeEnterOnSubjectMovesToSubjectSpecificCreatePrompt(t *testing.T) {
	m := sessionsSwitchboardModel{
		view: sessionsViewCreate,
		subjects: []store.Subject{
			{UUID: "abc12345-0000-0000-0000-000000000000", Name: "Alpha Subject"},
			{UUID: "def67890-0000-0000-0000-000000000000", Name: "Beta Subject"},
		},
		selectedBySubject: map[string]bool{},
	}
	m.refreshCreateList()

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(sessionsSwitchboardModel)

	selected := m.selectedSubjects()
	if len(selected) != 1 || selected[0].Name != "Alpha Subject" {
		t.Fatalf("expected only Alpha Subject selected, got %#v", selected)
	}
	if m.list.Index() != len(m.list.Items())-1 {
		t.Fatalf("expected selection to move to create row, got index=%d", m.list.Index())
	}
	out := stripANSI(m.View().Content)
	if !strings.Contains(out, "-> Create Session with Alpha Subject") {
		t.Fatalf("expected subject-specific create prompt, got:\n%s", out)
	}
	if strings.Contains(out, "-> Create Session with Beta Subject") {
		t.Fatalf("did not expect non-selected subject in create prompt, got:\n%s", out)
	}
}

func TestSessionCreatePicker_EnterOnSubjectMovesToSubjectSpecificCreatePrompt(t *testing.T) {
	m := newSessionCreatePickerModel(
		[]store.Subject{
			{UUID: "abc12345-0000-0000-0000-000000000000", Name: "Alpha Subject"},
			{UUID: "def67890-0000-0000-0000-000000000000", Name: "Beta Subject"},
		},
		map[string]bool{},
	)

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(sessionCreatePickerModel)

	selected := m.SelectedSubjects()
	if len(selected) != 1 || selected[0].Name != "Alpha Subject" {
		t.Fatalf("expected only Alpha Subject selected, got %#v", selected)
	}
	if m.list.Index() != len(m.list.Items())-1 {
		t.Fatalf("expected selection to move to create row, got index=%d", m.list.Index())
	}
	out := stripANSI(m.View().Content)
	if !strings.Contains(out, "-> Create Session with Alpha Subject") {
		t.Fatalf("expected subject-specific create prompt, got:\n%s", out)
	}
}

func TestSessionCreatePicker_FilteringAutoSelectsTopEntry(t *testing.T) {
	m := newSessionCreatePickerModel(
		[]store.Subject{
			{UUID: "abc12345-0000-0000-0000-000000000000", Name: "Alpha Subject"},
			{UUID: "def67890-0000-0000-0000-000000000000", Name: "Beta Subject"},
			{UUID: "ghi11111-0000-0000-0000-000000000000", Name: "Gamma Subject"},
		},
		map[string]bool{},
	)

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(sessionCreatePickerModel)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(sessionCreatePickerModel)
	if m.list.Index() == 0 {
		t.Fatalf("expected pre-filter selection to move away from top")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(sessionCreatePickerModel)
	if m.list.Index() != 0 {
		t.Fatalf("expected filtering to auto-select top entry, got index=%d", m.list.Index())
	}
}

func TestSessionsUI_CreateModeFilteringAutoSelectsTopEntry(t *testing.T) {
	m := sessionsSwitchboardModel{
		view: sessionsViewCreate,
		subjects: []store.Subject{
			{UUID: "abc12345-0000-0000-0000-000000000000", Name: "Alpha Subject"},
			{UUID: "def67890-0000-0000-0000-000000000000", Name: "Beta Subject"},
			{UUID: "ghi11111-0000-0000-0000-000000000000", Name: "Gamma Subject"},
		},
		selectedBySubject: map[string]bool{},
	}
	m.refreshCreateList()

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(sessionsSwitchboardModel)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(sessionsSwitchboardModel)
	if m.list.Index() == 0 {
		t.Fatalf("expected pre-filter selection to move away from top")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(sessionsSwitchboardModel)
	if m.list.Index() != 0 {
		t.Fatalf("expected filtering to auto-select top entry, got index=%d", m.list.Index())
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

func countBackgroundANSI(s string) int {
	re := regexp.MustCompile(`\x1b\[[0-9;]*48;`)
	return len(re.FindAllString(s, -1))
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
