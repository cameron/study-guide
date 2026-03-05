package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCmdRmAssets_RemovesAllStepAssetsAndKeepsMetadata(t *testing.T) {
	tmp := t.TempDir()
	studyRoot := filepath.Join(tmp, "study")
	mustCopyDir(t, filepath.Join("..", "..", "..", "test-data", "study-complete"), studyRoot)
	sessionSlug := "18-02-2026-boehmer"

	assertAssetCount(t, studyRoot, sessionSlug, 9)

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(studyRoot); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if err := cmdRmAssets(nil); err != nil {
		t.Fatalf("cmdRmAssets returned error: %v", err)
	}

	assertAssetCount(t, studyRoot, sessionSlug, 0)

	mustExist := []string{
		filepath.Join(studyRoot, "study.sg.md"),
		filepath.Join(studyRoot, "protocol.sg.md"),
		filepath.Join(studyRoot, "session", sessionSlug, "session.sg.md"),
		filepath.Join(studyRoot, "session", sessionSlug, "step", "first-exposure", "step.sg.md"),
		filepath.Join(studyRoot, "session", sessionSlug, "step", "ground", "step.sg.md"),
		filepath.Join(studyRoot, "session", sessionSlug, "step", "second-exposure", "step.sg.md"),
	}
	for _, p := range mustExist {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected metadata file to remain: %s (%v)", p, err)
		}
	}
}

func TestCmdRmAssets_RejectsUnexpectedArgs(t *testing.T) {
	if err := cmdRmAssets([]string{"extra"}); err == nil {
		t.Fatalf("expected usage error for extra args")
	}
}

