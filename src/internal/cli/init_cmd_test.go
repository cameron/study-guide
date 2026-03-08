package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdInit_BlankStudyNameFallsBackToFolderDerivedName(t *testing.T) {
	tmp := t.TempDir()
	studyDir := filepath.Join(tmp, "my-awesome_study")
	if err := os.MkdirAll(studyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(studyDir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	origRunForm := runFormRunner
	origRunProtocolTitlesPrompt := runProtocolTitlesPromptRunner
	defer func() {
		runFormRunner = origRunForm
		runProtocolTitlesPromptRunner = origRunProtocolTitlesPrompt
	}()

	runFormRunner = func(_ string, _ []formField) (map[string]string, bool, error) {
		return map[string]string{"study_name": ""}, false, nil
	}
	runProtocolTitlesPromptRunner = func() ([]string, bool, error) {
		return []string{"Baseline"}, false, nil
	}

	if err := cmdInit(); err != nil {
		t.Fatalf("cmdInit failed: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(studyDir, "study.sg.md"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.Contains(string(b), "\n# My Awesome Study\n") {
		t.Fatalf("expected folder-derived study title in study.sg.md, got:\n%s", string(b))
	}
}

func TestCmdInit_RequiresAtLeastOneProtocolStep(t *testing.T) {
	tmp := t.TempDir()
	studyDir := filepath.Join(tmp, "required-step")
	if err := os.MkdirAll(studyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(studyDir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	origRunForm := runFormRunner
	origRunProtocolTitlesPrompt := runProtocolTitlesPromptRunner
	defer func() {
		runFormRunner = origRunForm
		runProtocolTitlesPromptRunner = origRunProtocolTitlesPrompt
	}()

	runFormRunner = func(_ string, _ []formField) (map[string]string, bool, error) {
		return map[string]string{"study_name": "Required Step Study"}, false, nil
	}
	runProtocolTitlesPromptRunner = func() ([]string, bool, error) {
		return []string{}, false, nil
	}

	err = cmdInit()
	if err == nil {
		t.Fatalf("expected cmdInit to fail when no protocol steps are provided")
	}
	if !strings.Contains(err.Error(), "at least one protocol step is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
