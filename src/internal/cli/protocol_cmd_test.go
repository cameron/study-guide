package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunProtocolReconcile_RenamesSessionStepDirectories(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\n---\n\n# Study\n\n# Introduction\n\n\n# Methods\n\n\n# Results\n\n\n# Discussion\n\n\n# Conclusion\n", "Summary", "Renamed Step", "Second Step"))
	mustWriteFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-original-step", "step.sg.md"), "---\ntime_started: 10:00:00 01-01-2026\n---\n")
	mustWriteFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-original-step", "asset", "sample.jpg"), "asset")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	out := captureStdout(t, func() {
		if code := Run([]string{"protocol", "reconcile"}); code != 0 {
			t.Fatalf("Run(protocol reconcile) code=%d want=0", code)
		}
	})

	if !strings.Contains(out, "reconciled protocol step directories") {
		t.Fatalf("expected success output, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-renamed-step", "step.sg.md")); err != nil {
		t.Fatalf("expected renamed step metadata to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-renamed-step", "asset", "sample.jpg")); err != nil {
		t.Fatalf("expected renamed asset to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-original-step")); !os.IsNotExist(err) {
		t.Fatalf("expected original step directory to be removed, got err=%v", err)
	}
}

func TestRunHelp_ListsProtocolReconcileCommand(t *testing.T) {
	out := captureStdout(t, func() {
		if code := Run([]string{"help"}); code != 0 {
			t.Fatalf("Run(help) code=%d want=0", code)
		}
	})
	if !strings.Contains(out, "\n  protocol reconcile\n") {
		t.Fatalf("expected help to list protocol reconcile, got:\n%s", out)
	}
}
