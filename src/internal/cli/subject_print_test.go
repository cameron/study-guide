package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"study-guide/src/internal/store"
	"study-guide/src/internal/util"
)

func TestRunSubjectPrint_DefaultsToCurrentStudySubjects(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SG_SUBJECT_DIR", filepath.Join(root, ".subjects"))

	alphaPath, err := store.SaveSubject(store.Subject{Name: "Alpha Able", Type: "person"})
	if err != nil {
		t.Fatalf("SaveSubject alpha failed: %v", err)
	}
	_ = alphaPath
	alpha, err := store.ResolveSubject("Alpha")
	if err != nil {
		t.Fatalf("ResolveSubject alpha failed: %v", err)
	}
	betaPath, err := store.SaveSubject(store.Subject{Name: "Beta Baker", Type: "person"})
	if err != nil {
		t.Fatalf("SaveSubject beta failed: %v", err)
	}
	_ = betaPath
	beta, err := store.ResolveSubject("Beta")
	if err != nil {
		t.Fatalf("ResolveSubject beta failed: %v", err)
	}
	if _, err := store.SaveSubject(store.Subject{Name: "Gamma Guest", Type: "person"}); err != nil {
		t.Fatalf("SaveSubject gamma failed: %v", err)
	}

	mustWriteFile(t, filepath.Join(root, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\n---\n\n# Study\n")
	writeSubjectPrintSession(t, filepath.Join(root, "session", "01-01-2026-alpha", "session.sg.md"), "# Subjects\n\n- Alpha Able ("+alpha.UUID+")\n- Beta Baker ("+beta.UUID+")\n")
	writeSubjectPrintSession(t, filepath.Join(root, "session", "02-01-2026-repeat", "session.sg.md"), "# Subjects\n\n- Alpha Able ("+alpha.UUID+")\n")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	out := captureStdout(t, func() {
		if code := Run([]string{"subject", "print"}); code != 0 {
			t.Fatalf("Run(subject print) code=%d want=0", code)
		}
	})
	out = stripANSI(strings.ReplaceAll(out, "\r\n", "\n"))

	for _, want := range []string{"Alpha Able", alpha.UUID, "Beta Baker", beta.UUID} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Gamma Guest") {
		t.Fatalf("expected non-study subjects to be excluded, got:\n%s", out)
	}
	if strings.Count(out, "Alpha Able") != 1 {
		t.Fatalf("expected repeated study subjects to be de-duplicated, got:\n%s", out)
	}
}

func TestRunSubjectPrint_AllFlagPrintsAllSubjects(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SG_SUBJECT_DIR", filepath.Join(root, ".subjects"))

	if _, err := store.SaveSubject(store.Subject{Name: "Alpha Able", Type: "person"}); err != nil {
		t.Fatalf("SaveSubject alpha failed: %v", err)
	}
	if _, err := store.SaveSubject(store.Subject{Name: "Gamma Guest", Type: "person"}); err != nil {
		t.Fatalf("SaveSubject gamma failed: %v", err)
	}

	mustWriteFile(t, filepath.Join(root, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\n---\n\n# Study\n")
	writeSubjectPrintSession(t, filepath.Join(root, "session", "01-01-2026-alpha", "session.sg.md"), "# Subjects\n\n- Alpha Able\n")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	out := captureStdout(t, func() {
		if code := Run([]string{"subject", "print", "--all"}); code != 0 {
			t.Fatalf("Run(subject print --all) code=%d want=0", code)
		}
	})
	out = stripANSI(strings.ReplaceAll(out, "\r\n", "\n"))

	for _, want := range []string{"Alpha Able", "Gamma Guest"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func writeSubjectPrintSession(t *testing.T, path string, body string) {
	t.Helper()
	if err := util.EnsureDir(filepath.Dir(path)); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(path, map[string]any{}, body); err != nil {
		t.Fatalf("WriteFrontmatterFile failed: %v", err)
	}
}
