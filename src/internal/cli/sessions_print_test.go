package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdSessionsWithArgs_PrintOutputsSessionStepTimingTable(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\n---\n\n# Study\n")
	mustWriteFile(t, filepath.Join(root, "protocol.sg.md"), "# Protocol Summary\n\nSummary\n\n# Steps\n\n## First Step\n\n## Second Step\n\n")

	mustWriteSessionFile(t, root, "01-01-2026-alpha", map[string]any{})
	mustWriteStepFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-first-step", "step.sg.md"), map[string]any{
		"time_started":  "10:01:00 01-01-2026",
		"time_finished": "10:02:00 01-01-2026",
	}, "")
	mustWriteStepFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "02-second-step", "step.sg.md"), map[string]any{
		"time_started": "10:03:00 01-01-2026",
	}, "")

	mustWriteSessionFile(t, root, "02-01-2026-beta", map[string]any{})

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	out := captureStdout(t, func() {
		if err := cmdSessionsWithArgs([]string{"print"}); err != nil {
			t.Fatalf("cmdSessionsWithArgs print returned error: %v", err)
		}
	})
	out = stripANSI(out)
	out = strings.ReplaceAll(out, "\r\n", "\n")

	if strings.Contains(out, "| SESSION | STEP | START | END |") {
		t.Fatalf("expected non-markdown table output, got:\n%s", out)
	}
	for _, want := range []string{
		"SESSION",
		"STEP",
		"START",
		"END",
		"01-01-2026-alpha",
		"02-01-2026-beta",
		"01-first-step",
		"02-second-step",
		"10:01:00 01-01-2026",
		"10:02:00 01-01-2026",
		"10:03:00 01-01-2026",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	lines := strings.Split(out, "\n")
	header := findLineContaining(lines, "SESSION")
	row := findLineContaining(lines, "01-01-2026-alpha")
	if header == "" || row == "" {
		t.Fatalf("expected header and first data row in output, got:\n%s", out)
	}
	headerStep := strings.Index(header, "STEP")
	headerStart := strings.Index(header, "START")
	headerEnd := strings.Index(header, "END")
	rowStep := strings.Index(row, "01-first-step")
	rowStart := strings.Index(row, "10:01:00 01-01-2026")
	rowEnd := strings.Index(row, "10:02:00 01-01-2026")
	if headerStep != rowStep || headerStart != rowStart || headerEnd != rowEnd {
		t.Fatalf("expected aligned columns, header=%q row=%q", header, row)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe error: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close write pipe error: %v", err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy stdout error: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close read pipe error: %v", err)
	}
	return buf.String()
}

func findLineContaining(lines []string, token string) string {
	for _, line := range lines {
		if strings.Contains(line, token) {
			return line
		}
	}
	return ""
}
