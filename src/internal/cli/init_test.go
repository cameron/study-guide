package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestProtocolTitlesModel_EnterAddsStepAndContinues(t *testing.T) {
	m := newProtocolTitlesModel()
	m.input.SetValue("Baseline")

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(protocolTitlesModel)

	if len(m.steps) != 1 {
		t.Fatalf("expected one step after enter, got %d", len(m.steps))
	}
	if m.steps[0] != "Baseline" {
		t.Fatalf("expected first step to be Baseline, got %q", m.steps[0])
	}
	if m.done {
		t.Fatalf("expected model to continue collecting steps after non-empty enter")
	}
	if m.input.Value() != "" {
		t.Fatalf("expected input to clear after adding step, got %q", m.input.Value())
	}
}

func TestProtocolTitlesModel_EmptyEnterFinishesAfterAtLeastOneStep(t *testing.T) {
	m := newProtocolTitlesModel()
	m.steps = []string{"Baseline"}

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(protocolTitlesModel)

	if !m.done {
		t.Fatalf("expected empty enter to finish when at least one step exists")
	}
}

func TestProtocolTitlesModel_EmptyEnterDoesNotFinishBeforeFirstStep(t *testing.T) {
	m := newProtocolTitlesModel()

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(protocolTitlesModel)

	if m.done {
		t.Fatalf("expected empty enter to keep collecting until at least one step exists")
	}
}

func TestEnsureProtocolFile_WritesTitlesOnly(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "protocol.sg.md")
	steps := []string{
		"Baseline",
		"WiFi",
		"Grounding",
	}
	if err := ensureProtocolFile(path, steps); err != nil {
		t.Fatalf("ensureProtocolFile error: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	got := string(b)
	for _, token := range []string{
		"## Baseline",
		"## WiFi",
		"## Grounding",
	} {
		if !strings.Contains(got, token) {
			t.Fatalf("expected token %q in protocol scaffold\ncontent:\n%s", token, got)
		}
	}
	if strings.Contains(got, "|") {
		t.Fatalf("expected titles-only scaffold; found outline delimiter in content:\n%s", got)
	}
}

func TestProtocolTitlesModel_View_TitleHasBackgroundStyle(t *testing.T) {
	m := newProtocolTitlesModel()
	out := m.View().Content
	firstLine := strings.SplitN(out, "\n", 2)[0]
	re := regexp.MustCompile(`\x1b\[[0-9;]*48;`)
	if !re.MatchString(firstLine) {
		t.Fatalf("expected title line to include background ANSI style, got %q", firstLine)
	}
}
