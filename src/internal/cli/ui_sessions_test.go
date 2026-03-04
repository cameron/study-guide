package cli

import (
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
