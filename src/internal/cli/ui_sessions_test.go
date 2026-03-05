package cli

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

func TestSessionsSelectedRowStyle_UsesAdaptiveTint(t *testing.T) {
	st := sessionsSelectedRowStyle(table.DefaultStyles().Selected)

	bg, ok := st.GetBackground().(lipgloss.AdaptiveColor)
	if !ok {
		t.Fatalf("expected adaptive background color, got %T", st.GetBackground())
	}
	if bg.Dark != sessionsSelectedRowBgDark {
		t.Fatalf("unexpected dark selected background: %q", bg.Dark)
	}
	if bg.Light != sessionsSelectedRowBgLight {
		t.Fatalf("unexpected light selected background: %q", bg.Light)
	}
	if st.GetReverse() {
		t.Fatalf("did not expect reverse style for selected row")
	}
	if st.GetBold() {
		t.Fatalf("did not expect bold style for selected row")
	}
	if _, ok := st.GetForeground().(lipgloss.NoColor); !ok {
		t.Fatalf("expected no explicit foreground color, got %T", st.GetForeground())
	}
}

func TestApplyBrowseTableLayout_GivesStepColumnMoreWidth(t *testing.T) {
	m := sessionsSwitchboardModel{
		table: table.New(),
		width: 120,
	}

	m.applyBrowseTableLayout()

	cols := m.table.Columns()
	if len(cols) != 4 {
		t.Fatalf("expected 4 columns, got %d", len(cols))
	}
	if cols[2].Title != "STEP" {
		t.Fatalf("expected third column to be STEP, got %q", cols[2].Title)
	}
	if cols[2].Width < 30 {
		t.Fatalf("expected STEP width >= 30, got %d", cols[2].Width)
	}
}

func TestApplyBrowseTableLayout_IncreasesAllBaseColumnWidths(t *testing.T) {
	m := sessionsSwitchboardModel{
		table: table.New(),
		width: 220,
	}

	m.applyBrowseTableLayout()

	cols := m.table.Columns()
	if len(cols) != 4 {
		t.Fatalf("expected 4 columns, got %d", len(cols))
	}
	if cols[0].Width < 35 {
		t.Fatalf("expected SLUG width >= 35, got %d", cols[0].Width)
	}
	if cols[1].Width < 35 {
		t.Fatalf("expected SUBJECT width >= 35, got %d", cols[1].Width)
	}
	if cols[2].Width < 48 {
		t.Fatalf("expected STEP width >= 48, got %d", cols[2].Width)
	}
	if cols[3].Width < 32 {
		t.Fatalf("expected NEXT STEP width >= 32, got %d", cols[3].Width)
	}
}

func TestSessionsNextStepTextStyle_UsesBrighterGrey(t *testing.T) {
	st := sessionsNextStepTextStyle()
	fg, ok := st.GetForeground().(lipgloss.Color)
	if !ok {
		t.Fatalf("expected lipgloss.Color foreground, got %T", st.GetForeground())
	}
	if string(fg) != "246" {
		t.Fatalf("expected next step foreground 246, got %q", string(fg))
	}
}

func TestSessionsSelectedRowStyle_UsesSlightBlueHue(t *testing.T) {
	st := sessionsSelectedRowStyle(table.DefaultStyles().Selected)
	bg, ok := st.GetBackground().(lipgloss.AdaptiveColor)
	if !ok {
		t.Fatalf("expected adaptive background color, got %T", st.GetBackground())
	}
	if bg.Light != "#d9dcef" {
		t.Fatalf("expected light blue-tinted selection color, got %q", bg.Light)
	}
	if bg.Dark != "#262b3a" {
		t.Fatalf("expected dark blue-tinted selection color, got %q", bg.Dark)
	}
}

func TestNewSessionsFilterInput_Placeholder(t *testing.T) {
	fi := newSessionsFilterInput()
	if fi.Placeholder != "by subject or slug" {
		t.Fatalf("expected placeholder %q, got %q", "by subject or slug", fi.Placeholder)
	}
	if fi.Prompt != " filter: " {
		t.Fatalf("expected prompt %q, got %q", " filter: ", fi.Prompt)
	}
	if !fi.Focused() {
		t.Fatalf("expected filter input to be focused")
	}
}

func TestRenderEntryRow_EmptyStateText(t *testing.T) {
	m := sessionsSwitchboardModel{}
	slug, subject, step, next := m.renderEntryRow(browseEntry{kind: browseEntryEmpty})
	if slug != "no active sessions" {
		t.Fatalf("expected empty-state slug text %q, got %q", "no active sessions", slug)
	}
	if subject != "" || step != "" || next != "" {
		t.Fatalf("expected remaining columns empty, got subject=%q step=%q next=%q", subject, step, next)
	}
}

func TestRefreshCreateList_UsesSimpleCreateHeader(t *testing.T) {
	m := sessionsSwitchboardModel{}
	m.refreshCreateList()
	if m.list.Title != "Create Session" {
		t.Fatalf("expected create list title %q, got %q", "Create Session", m.list.Title)
	}
	items := m.list.Items()
	hasCreate := false
	hasDone := false
	for _, it := range items {
		s, ok := it.(listItem)
		if !ok {
			continue
		}
		if string(s) == "Create" {
			hasCreate = true
		}
		if string(s) == "Done" {
			hasDone = true
		}
	}
	if !hasCreate {
		t.Fatalf("expected Create action in create list items")
	}
	if hasDone {
		t.Fatalf("did not expect legacy Done action in create list items")
	}
}

func TestView_CreateModeShowsInstructionDirectlyBelowHeader(t *testing.T) {
	m := sessionsSwitchboardModel{}
	m.refreshCreateList()
	out := m.View()
	headerIdx := strings.Index(out, "Create Session")
	if headerIdx < 0 {
		t.Fatalf("expected output to include create header")
	}
	infoIdx := strings.Index(out, sessionsCreateInfoText)
	if infoIdx < 0 {
		t.Fatalf("expected output to include create helper text %q", sessionsCreateInfoText)
	}
	itemsIdx := strings.Index(out, "No subjects available")
	if itemsIdx < 0 {
		t.Fatalf("expected output to include create list item text")
	}
	if !(headerIdx < infoIdx && infoIdx < itemsIdx) {
		t.Fatalf("expected header -> info -> items order, got header=%d info=%d items=%d", headerIdx, infoIdx, itemsIdx)
	}
}
