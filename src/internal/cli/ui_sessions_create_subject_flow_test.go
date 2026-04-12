package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"study-guide/src/internal/store"
	"study-guide/src/internal/util"
)

func TestSessionsUI_CreateSubjectFlowCreatesSubjectAndSessionAndReturnsToBrowse(t *testing.T) {
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

	alphaPath, err := store.SaveSubject(store.Subject{
		Name:      "Alpha Subject",
		Type:      "person",
		CreatedOn: "11:59:59 14-03-2026",
	})
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

	if m.view != sessionsViewBrowse {
		t.Fatalf("expected completed subject form to return to browse mode, got %v", m.view)
	}
	if got := selectedSubjectNames(m.selectedSubjects()); got != "Beta Subject" {
		t.Fatalf("expected newly created subject to be selected for the created session, got %q", got)
	}

	view := stripANSI(m.View().Content)
	if !strings.Contains(view, "Sessions") || !strings.Contains(view, "[enter] next step") {
		t.Fatalf("expected browse header after returning from subject form, got:\n%s", view)
	}
	if !strings.Contains(view, "Beta Subject") {
		t.Fatalf("expected browse view to show created session subject, got:\n%s", view)
	}

	allSubjects, err := store.ListSubjects()
	if err != nil {
		t.Fatalf("ListSubjects failed: %v", err)
	}
	if got := selectedSubjectNames(allSubjects); got != "Alpha Subject,Beta Subject" {
		t.Fatalf("expected saved subject list to include new subject, got %q", got)
	}

	sessionDirs, err := os.ReadDir(filepath.Join(studyRoot, "session"))
	if err != nil {
		t.Fatalf("read session dir failed: %v", err)
	}
	if len(sessionDirs) != 1 {
		t.Fatalf("expected one created session, got %d", len(sessionDirs))
	}
	sessionFM, sessionBody, err := util.ReadFrontmatterFile(filepath.Join(studyRoot, "session", sessionDirs[0].Name(), "session.sg.md"))
	if err != nil {
		t.Fatalf("read created session failed: %v", err)
	}
	if len(sessionFM) != 0 {
		t.Fatalf("expected created session frontmatter to remain empty, got %#v", sessionFM)
	}
	if !strings.Contains(sessionBody, "Beta Subject ("+allSubjects[1].UUID+")") {
		t.Fatalf("expected created session body to reference new subject, got:\n%s", sessionBody)
	}
	sessionSlug := sessionDirs[0].Name()
	for _, stepSlug := range []string{"01-step-one"} {
		stepFM, stepBody, err := util.ReadFrontmatterFile(filepath.Join(studyRoot, "session", sessionSlug, "step", stepSlug, "step.sg.md"))
		if err != nil {
			t.Fatalf("read created step failed: %v", err)
		}
		if len(stepFM) != 0 {
			t.Fatalf("expected created step frontmatter to remain empty, got %#v", stepFM)
		}
		if strings.TrimSpace(stepBody) != "" {
			t.Fatalf("expected created step body to remain empty, got %q", stepBody)
		}
	}
}

func TestSessionCreatePicker_ListHeightExpandsWithViewport(t *testing.T) {
	m := newSessionCreatePickerModel(
		[]store.Subject{{UUID: "abc12345-0000-0000-0000-000000000000", Name: "Alpha Subject"}},
		map[string]bool{},
	)

	initialHeight := m.list.Height()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(sessionCreatePickerModel)

	if got, want := m.list.Height(), 22; got != want {
		t.Fatalf("expected shared picker list height to track viewport height, got %d want %d", got, want)
	}
	if m.list.Height() <= initialHeight {
		t.Fatalf("expected list height to grow beyond initial fixed height, got initial=%d current=%d", initialHeight, m.list.Height())
	}
}

func selectedSubjectNames(subjects []store.Subject) string {
	names := make([]string, 0, len(subjects))
	for _, s := range subjects {
		names = append(names, s.Name)
	}
	return strings.Join(names, ",")
}

func listItemTitles(items []list.Item) string {
	titles := make([]string, 0, len(items))
	for _, item := range items {
		title, ok := selectedListItemTitle(item)
		if !ok {
			titles = append(titles, "")
			continue
		}
		titles = append(titles, title)
	}
	return strings.Join(titles, ",")
}
