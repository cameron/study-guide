package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRunNoArgs_EmptyDirRunsInitThenSessions(t *testing.T) {
	tmp := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	origInit := cmdInitRunner
	origSessions := cmdSessionsRunner
	defer func() {
		cmdInitRunner = origInit
		cmdSessionsRunner = origSessions
	}()

	var calls []string
	cmdInitRunner = func() error {
		calls = append(calls, "init")
		return os.WriteFile(filepath.Join(tmp, "study.sg.md"), []byte("---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\n---\n"), 0o644)
	}
	cmdSessionsRunner = func() error {
		calls = append(calls, "sessions")
		return nil
	}

	code := Run(nil)
	if code != 0 {
		t.Fatalf("Run(nil) code=%d want=0", code)
	}
	if !reflect.DeepEqual(calls, []string{"init", "sessions"}) {
		t.Fatalf("unexpected call order: %#v", calls)
	}
}

func TestRunNoArgs_NonEmptyDirWithoutStudyFileRunsInitThenSessions(t *testing.T) {
	tmp := t.TempDir()
	mustWriteFile(t, filepath.Join(tmp, "notes.txt"), "scratch")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	origInit := cmdInitRunner
	origSessions := cmdSessionsRunner
	defer func() {
		cmdInitRunner = origInit
		cmdSessionsRunner = origSessions
	}()

	var calls []string
	cmdInitRunner = func() error {
		calls = append(calls, "init")
		return os.WriteFile(filepath.Join(tmp, "study.sg.md"), []byte("---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\n---\n"), 0o644)
	}
	cmdSessionsRunner = func() error {
		calls = append(calls, "sessions")
		return nil
	}

	code := Run(nil)
	if code != 0 {
		t.Fatalf("Run(nil) code=%d want=0", code)
	}
	if !reflect.DeepEqual(calls, []string{"init", "sessions"}) {
		t.Fatalf("unexpected call order: %#v", calls)
	}
}

func TestRunNoArgs_StudyRootRunsSessions(t *testing.T) {
	tmp := t.TempDir()
	mustWriteFile(t, filepath.Join(tmp, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\n---\n")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	origInit := cmdInitRunner
	origSessions := cmdSessionsRunner
	defer func() {
		cmdInitRunner = origInit
		cmdSessionsRunner = origSessions
	}()

	var calls []string
	cmdInitRunner = func() error {
		calls = append(calls, "init")
		return nil
	}
	cmdSessionsRunner = func() error {
		calls = append(calls, "sessions")
		return nil
	}

	code := Run(nil)
	if code != 0 {
		t.Fatalf("Run(nil) code=%d want=0", code)
	}
	if !reflect.DeepEqual(calls, []string{"sessions"}) {
		t.Fatalf("unexpected call order: %#v", calls)
	}
}

func TestRunNoArgs_EmptyDirClearsScreenBeforeSessions(t *testing.T) {
	tmp := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	origInit := cmdInitRunner
	origSessions := cmdSessionsRunner
	defer func() {
		cmdInitRunner = origInit
		cmdSessionsRunner = origSessions
	}()

	cmdInitRunner = func() error {
		if err := os.WriteFile(filepath.Join(tmp, "study.sg.md"), []byte("---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\n---\n"), 0o644); err != nil {
			return err
		}
		_, _ = os.Stdout.WriteString("init-finished\n")
		return nil
	}
	cmdSessionsRunner = func() error {
		_, _ = os.Stdout.WriteString("sessions-started\n")
		return nil
	}

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe failed: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	if code := Run(nil); code != 0 {
		t.Fatalf("Run(nil) code=%d want=0", code)
	}
	_ = w.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom failed: %v", err)
	}
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("\x1b[2J\x1b[H")) {
		t.Fatalf("expected clear-screen sequence between init and sessions output, got: %q", out)
	}
}
