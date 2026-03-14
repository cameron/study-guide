package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"study-guide/src/internal/store"
	"study-guide/src/internal/util"
)

func TestSessionsUI_CreateSubjectFlowReturnsToCreateModeWithPreservedSelection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	studyRoot := t.TempDir()
	if err := util.WriteFrontmatterFile(
		filepath.Join(studyRoot, "study.sg.md"),
		map[string]any{"status": "WIP", "created_on": "12:00:00 14-03-2026"},
		"# Study\n",
	); err != nil {
		t.Fatalf("write study.sg.md failed: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(studyRoot, "subject-requirements.yaml"),
		[]byte("type: person\nrequired_fields:\n  - name\n"),
		0o644,
	); err != nil {
		t.Fatalf("write subject-requirements.yaml failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(studyRoot, "session"), 0o755); err != nil {
		t.Fatalf("mkdir session failed: %v", err)
	}

	alphaPath, err := store.SaveSubject(store.Subject{Name: "Alpha Subject", Type: "person"})
	if err != nil {
		t.Fatalf("seed subject failed: %v", err)
	}
	alphaFM, alphaBody, err := util.ReadFrontmatterFile(alphaPath)
	if err != nil {
		t.Fatalf("read seeded subject failed: %v", err)
	}
	_ = store.SubjectFromFM(alphaPath, alphaFM, alphaBody)

	m, err := newSessionsSwitchboardModel(studyRoot, store.Protocol{
		Steps: []store.ProtocolStep{{Name: "Step One", Slug: "01-step-one"}},
	})
	if err != nil {
		t.Fatalf("newSessionsSwitchboardModel failed: %v", err)
	}

	subjects, err := store.ListSubjects()
	if err != nil {
		t.Fatalf("ListSubjects failed: %v", err)
	}
	m.subjects = subjects
	m.selectedBySubject = map[string]bool{}
	m.refreshCreateList()

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(sessionsSwitchboardModel)
	if got := selectedSubjectNames(m.selectedSubjects()); got != "Alpha Subject" {
		t.Fatalf("expected Alpha Subject selected before create-subject flow, got %q", got)
	}

	m.list.Select(1)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(sessionsSwitchboardModel)
	if m.view != sessionsViewCreateSubject {
		t.Fatalf("expected enter on New subject to enter embedded subject-create state, got %v", m.view)
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'B', Text: "B"})
	m = updated.(sessionsSwitchboardModel)
	for _, key := range []tea.KeyPressMsg{
		{Code: 'e', Text: "e"},
		{Code: 't', Text: "t"},
		{Code: 'a', Text: "a"},
		{Code: tea.KeySpace, Text: " "},
		{Code: 'S', Text: "S"},
		{Code: 'u', Text: "u"},
		{Code: 'b', Text: "b"},
		{Code: 'j', Text: "j"},
		{Code: 'e', Text: "e"},
		{Code: 'c', Text: "c"},
		{Code: 't', Text: "t"},
	} {
		updated, _ = m.Update(key)
		m = updated.(sessionsSwitchboardModel)
	}
	for i := 0; i < 7; i++ {
		updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m = updated.(sessionsSwitchboardModel)
	}

	if m.view != sessionsViewCreate {
		t.Fatalf("expected completed subject form to return to create mode, got %v", m.view)
	}
	if got := selectedSubjectNames(m.selectedSubjects()); got != "Alpha Subject" {
		t.Fatalf("expected prior selection preserved after returning from subject form, got %q", got)
	}

	view := stripANSI(m.View().Content)
	if !strings.Contains(view, "Create Session") {
		t.Fatalf("expected create-mode header after returning from subject form, got:\n%s", view)
	}
	if strings.Contains(view, "Enter to continue. Tab/Shift+Tab to move. Esc to cancel.") {
		t.Fatalf("expected subject form instructions cleared after returning to create mode, got:\n%s", view)
	}

	allSubjects, err := store.ListSubjects()
	if err != nil {
		t.Fatalf("ListSubjects failed: %v", err)
	}
	if got := selectedSubjectNames(allSubjects); got != "Alpha Subject,Beta Subject" {
		t.Fatalf("expected saved subject list to include new subject, got %q", got)
	}

	items := m.list.Items()
	if len(items) != 4 {
		t.Fatalf("expected Alpha, Beta, New subject, and Create entries after refresh, got %d", len(items))
	}
}

func selectedSubjectNames(subjects []store.Subject) string {
	names := make([]string, 0, len(subjects))
	for _, s := range subjects {
		names = append(names, s.Name)
	}
	return strings.Join(names, ",")
}
