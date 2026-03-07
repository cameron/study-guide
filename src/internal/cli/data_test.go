package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunDataIngest_FromAssetsDir(t *testing.T) {
	tmp := t.TempDir()
	studyRoot := filepath.Join(tmp, "study")
	mustCopyDir(t, filepath.Join("..", "..", "..", "fixtures", "study-eg"), studyRoot)
	mustPopulateFocusWindowsFromStepTimes(t, studyRoot)

	sessionA := "18-02-2026-boehmer"
	assetsDir, err := filepath.Abs(filepath.Join("testdata", "ingest-assets"))
	if err != nil {
		t.Fatalf("Abs assets dir error: %v", err)
	}

	assetTimes := map[string]string{
		"first-a.jpg":  "23:25:00 18-02-2026",
		"first-b.jpg":  "23:30:00 18-02-2026",
		"ground-a.jpg": "23:40:00 18-02-2026",
		"outside.jpg":  "22:00:00 18-02-2026",
	}
	origCapture := exifCaptureTimeFn
	exifCaptureTimeFn = func(path string) (time.Time, error) {
		base := filepath.Base(path)
		if base == "no-exif.jpg" {
			return time.Time{}, errors.New("no exif")
		}
		if base == "first-a-dup.jpg" {
			base = "first-a.jpg"
		}
		raw, ok := assetTimes[base]
		if !ok {
			return time.Time{}, errors.New("unexpected asset")
		}
		return mustParseTS(t, raw), nil
	}
	defer func() { exifCaptureTimeFn = origCapture }()

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(studyRoot); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"data", "ingest", "--assets-dir", assetsDir}); code != 0 {
		t.Fatalf("Run(data ingest) code=%d want=0", code)
	}
	assertAssetCount(t, studyRoot, sessionA, 3)
}

func TestRunDataLs_PrintsSortedRowsAndTotal(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\n---\n\n# Study\n")
	mustWriteFile(t, filepath.Join(root, "protocol.sg.md"), "# Protocol Summary\n\nSummary\n\n# Steps\n\n## Step One\n\n## Step Two\n\n")
	mustWriteFile(t, filepath.Join(root, "subject-requirements.yaml"), "type: person\n")

	mustWriteFile(t, filepath.Join(root, "session", "02-01-2026-beta", "step", "02-step-two", "asset", "z-last.jpg"), "a")
	mustWriteFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "02-step-two", "asset", "b-two.jpg"), "b")
	mustWriteFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-step-one", "asset", "a-one.jpg"), "c")
	mustWriteFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-step-one", "asset", ".DS_Store"), "ignored")
	mustWriteFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-step-one", "step.sg.md"), "---\n---\n")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	out := captureStdout(t, func() {
		if code := Run([]string{"data", "ls"}); code != 0 {
			t.Fatalf("Run(data ls) code=%d want=0", code)
		}
	})
	out = stripANSI(strings.ReplaceAll(out, "\r\n", "\n"))

	for _, want := range []string{"SESSION", "STEP", "FILE", "01-01-2026-alpha", "02-01-2026-beta", "assets total: 3"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, ".DS_Store") {
		t.Fatalf("expected output to exclude .DS_Store entries, got:\n%s", out)
	}

	first := strings.Index(out, "01-01-2026-alpha")
	second := strings.Index(out, "02-01-2026-beta")
	if first == -1 || second == -1 || first >= second {
		t.Fatalf("expected alpha rows before beta rows, got:\n%s", out)
	}
	if strings.Index(out, "01-step-one") > strings.Index(out, "02-step-two") {
		t.Fatalf("expected step sort order within session, got:\n%s", out)
	}
}

func TestRunDataClean_RemovesAssetFilesAndKeepsMetadata(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\n---\n\n# Study\n")
	mustWriteFile(t, filepath.Join(root, "protocol.sg.md"), "# Protocol Summary\n\nSummary\n\n# Steps\n\n## Step One\n\n## Step Two\n\n")
	mustWriteFile(t, filepath.Join(root, "subject-requirements.yaml"), "type: person\n")

	mustWriteFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-step-one", "asset", "a-one.jpg"), "a")
	mustWriteFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-step-one", "asset", "b-one.jpg"), "b")
	mustWriteFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "02-step-two", "asset", "c-two.jpg"), "c")
	mustWriteFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-step-one", "step.sg.md"), "---\n---\n")
	mustWriteFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "session.sg.md"), "---\n---\n")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	out := captureStdout(t, func() {
		if code := Run([]string{"data", "clean"}); code != 0 {
			t.Fatalf("Run(data clean) code=%d want=0", code)
		}
	})
	if !strings.Contains(out, "removed asset files: 3") {
		t.Fatalf("expected clean summary, got:\n%s", out)
	}

	assertAssetCount(t, root, "01-01-2026-alpha", 0)
	for _, mustExist := range []string{
		filepath.Join(root, "study.sg.md"),
		filepath.Join(root, "protocol.sg.md"),
		filepath.Join(root, "session", "01-01-2026-alpha", "session.sg.md"),
		filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-step-one", "step.sg.md"),
	} {
		if _, err := os.Stat(mustExist); err != nil {
			t.Fatalf("expected metadata file to remain: %s (%v)", mustExist, err)
		}
	}
}

func TestRunLegacyIngestPhotosCommandIsUnknown(t *testing.T) {
	stderr := captureStderr(t, func() {
		if code := Run([]string{"ingest-photos"}); code != 2 {
			t.Fatalf("Run(ingest-photos) code=%d want=2", code)
		}
	})
	if !strings.Contains(stderr, "unknown command: ingest-photos") {
		t.Fatalf("expected unknown command error, got %q", stderr)
	}
}

func TestRunHelp_ListsDataSubcommandsIndependently(t *testing.T) {
	out := captureStdout(t, func() {
		if code := Run([]string{"help"}); code != 0 {
			t.Fatalf("Run(help) code=%d want=0", code)
		}
	})
	if !strings.Contains(out, "\n  data ingest [--assets-dir <path>]\n") {
		t.Fatalf("expected independent data ingest help line, got:\n%s", out)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe error: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close write pipe error: %v", err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy stderr error: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close read pipe error: %v", err)
	}
	return buf.String()
}
