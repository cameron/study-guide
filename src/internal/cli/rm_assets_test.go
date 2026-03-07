package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCmdRmAssets_RemovesAllStepAssetsAndKeepsMetadata(t *testing.T) {
	tmp := t.TempDir()
	studyRoot := filepath.Join(tmp, "study")
	sessionSlug := "18-02-2026-boehmer"
	seedStudyForRmAssetsTest(t, studyRoot, sessionSlug)
	mustPopulateFocusWindowsFromStepTimes(t, studyRoot)

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

	if err := cmdIngestPhotos([]string{"--assets-dir", assetsDir}); err != nil {
		t.Fatalf("cmdIngestPhotos returned error: %v", err)
	}
	assertAssetCount(t, studyRoot, sessionSlug, 3)

	if err := cmdRmAssets(nil); err != nil {
		t.Fatalf("cmdRmAssets returned error: %v", err)
	}

	assertAssetCount(t, studyRoot, sessionSlug, 0)

	mustExist := []string{
		filepath.Join(studyRoot, "study.sg.md"),
		filepath.Join(studyRoot, "protocol.sg.md"),
		filepath.Join(studyRoot, "session", sessionSlug, "session.sg.md"),
		filepath.Join(studyRoot, "session", sessionSlug, "step", "01-first-exposure", "step.sg.md"),
		filepath.Join(studyRoot, "session", sessionSlug, "step", "02-ground", "step.sg.md"),
		filepath.Join(studyRoot, "session", sessionSlug, "step", "03-second-exposure", "step.sg.md"),
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

func seedStudyForRmAssetsTest(t *testing.T, studyRoot, sessionSlug string) {
	t.Helper()
	mustWriteFile(
		t,
		filepath.Join(studyRoot, "study.sg.md"),
		"---\nstatus: WIP\ncreated_on: 10:00:00 18-02-2026\n---\n\n# Study\n\n# Hypotheses\n\n# Discussion\n\n# Conclusion\n",
	)
	mustWriteFile(
		t,
		filepath.Join(studyRoot, "protocol.sg.md"),
		"# Protocol Summary\n\nSummary\n\n# Steps\n\n## First Exposure\n\n## Ground\n\n## Second Exposure\n",
	)
	mustWriteFile(t, filepath.Join(studyRoot, "subject-requirements.yaml"), "type: person\n")
	mustWriteSessionFile(t, studyRoot, sessionSlug, map[string]any{})
	mustWriteStepFile(t, filepath.Join(studyRoot, "session", sessionSlug, "step", "01-first-exposure", "step.sg.md"), map[string]any{
		"time_started":  "23:24:00 18-02-2026",
		"time_finished": "23:35:00 18-02-2026",
	}, "")
	mustWriteStepFile(t, filepath.Join(studyRoot, "session", sessionSlug, "step", "02-ground", "step.sg.md"), map[string]any{
		"time_started":  "23:35:01 18-02-2026",
		"time_finished": "23:45:00 18-02-2026",
	}, "")
	mustWriteStepFile(t, filepath.Join(studyRoot, "session", sessionSlug, "step", "03-second-exposure", "step.sg.md"), map[string]any{
		"time_started":  "23:45:01 18-02-2026",
		"time_finished": "23:55:00 18-02-2026",
	}, "")
}
