package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"study-guide/src/internal/store"
	"study-guide/src/internal/util"
)

func TestLoadStepWindows_UsesProtocolOrderAndWindowRules(t *testing.T) {
	tmp := t.TempDir()
	sessionDir := filepath.Join(tmp, "session", "s1")
	protocol := store.Protocol{Steps: []store.ProtocolStep{
		{Name: "A", Slug: "a"},
		{Name: "B", Slug: "b"},
		{Name: "C", Slug: "c"},
	}}

	mustWriteStepFile(t, filepath.Join(sessionDir, "step", "a", "step.sg.md"), map[string]any{
		"time_started": "10:00:00 01-01-2026",
	}, "# A\n")
	mustWriteStepFile(t, filepath.Join(sessionDir, "step", "b", "step.sg.md"), map[string]any{
		"time_started": "10:05:00 01-01-2026",
	}, "# B\n")
	mustWriteStepFile(t, filepath.Join(sessionDir, "step", "c", "step.sg.md"), map[string]any{
		"time_started":  "10:09:00 01-01-2026",
		"time_finished": "10:15:00 01-01-2026",
	}, "# C\n")

	windows, err := loadStepWindows(sessionDir, protocol)
	if err != nil {
		t.Fatalf("loadStepWindows returned error: %v", err)
	}
	if len(windows) != 3 {
		t.Fatalf("expected 3 windows, got %d", len(windows))
	}

	assertTimeEqual(t, windows[0].Start, "10:00:00 01-01-2026")
	assertTimeEqual(t, windows[0].End, "10:05:00 01-01-2026")
	if windows[0].Last {
		t.Fatalf("expected first window Last=false")
	}

	assertTimeEqual(t, windows[1].Start, "10:05:00 01-01-2026")
	assertTimeEqual(t, windows[1].End, "10:09:00 01-01-2026")
	if windows[1].Last {
		t.Fatalf("expected second window Last=false")
	}

	assertTimeEqual(t, windows[2].Start, "10:09:00 01-01-2026")
	assertTimeEqual(t, windows[2].End, "10:15:00 01-01-2026")
	if !windows[2].Last {
		t.Fatalf("expected last window Last=true")
	}
}

func TestLoadStepWindows_RequiresTimeFields(t *testing.T) {
	t.Run("missing started", func(t *testing.T) {
		tmp := t.TempDir()
		sessionDir := filepath.Join(tmp, "session", "s1")
		protocol := store.Protocol{Steps: []store.ProtocolStep{{Name: "A", Slug: "a"}}}

		mustWriteStepFile(t, filepath.Join(sessionDir, "step", "a", "step.sg.md"), map[string]any{}, "# A\n")

		_, err := loadStepWindows(sessionDir, protocol)
		if err == nil {
			t.Fatalf("expected error for missing time_started")
		}
	})

	t.Run("missing last finished", func(t *testing.T) {
		tmp := t.TempDir()
		sessionDir := filepath.Join(tmp, "session", "s1")
		protocol := store.Protocol{Steps: []store.ProtocolStep{{Name: "A", Slug: "a"}}}

		mustWriteStepFile(t, filepath.Join(sessionDir, "step", "a", "step.sg.md"), map[string]any{
			"time_started": "10:00:00 01-01-2026",
		}, "# A\n")

		_, err := loadStepWindows(sessionDir, protocol)
		if err == nil {
			t.Fatalf("expected error for missing last time_finished")
		}
	})
}

func TestFindStepWindowIndex_Boundaries(t *testing.T) {
	w := []stepWindow{
		{
			StepSlug: "a",
			Start:    mustParseTS(t, "10:00:00 01-01-2026"),
			End:      mustParseTS(t, "10:05:00 01-01-2026"),
		},
		{
			StepSlug: "b",
			Start:    mustParseTS(t, "10:05:00 01-01-2026"),
			End:      mustParseTS(t, "10:10:00 01-01-2026"),
			Last:     true,
		},
	}

	cases := []struct {
		at   string
		want int
	}{
		{at: "09:59:59 01-01-2026", want: -1},
		{at: "10:00:00 01-01-2026", want: 0},
		{at: "10:04:59 01-01-2026", want: 0},
		{at: "10:05:00 01-01-2026", want: 1},
		{at: "10:10:00 01-01-2026", want: 1},
		{at: "10:10:01 01-01-2026", want: -1},
	}

	for _, tc := range cases {
		got := findStepWindowIndex(mustParseTS(t, tc.at), w)
		if got != tc.want {
			t.Fatalf("findStepWindowIndex(%s)=%d want=%d", tc.at, got, tc.want)
		}
	}
}

func TestCollectSessionAssetHashes_DedupesByContent(t *testing.T) {
	tmp := t.TempDir()
	sessionDir := filepath.Join(tmp, "session", "s1")

	assetA := filepath.Join(sessionDir, "step", "a", "asset", "a1.jpg")
	assetB := filepath.Join(sessionDir, "step", "b", "asset", "b1.jpg")
	assetC := filepath.Join(sessionDir, "step", "c", "asset", "c1.jpg")
	nonAsset := filepath.Join(sessionDir, "step", "a", "step.sg.md")

	mustWriteFile(t, assetA, "same")
	mustWriteFile(t, assetB, "same")
	mustWriteFile(t, assetC, "different")
	mustWriteFile(t, nonAsset, "not-an-asset")

	hashes, err := collectSessionAssetHashes(sessionDir)
	if err != nil {
		t.Fatalf("collectSessionAssetHashes returned error: %v", err)
	}
	if len(hashes) != 2 {
		t.Fatalf("expected 2 unique hashes, got %d", len(hashes))
	}

	ha, err := fileSHA8(assetA)
	if err != nil {
		t.Fatalf("fileSHA8(assetA) error: %v", err)
	}
	hc, err := fileSHA8(assetC)
	if err != nil {
		t.Fatalf("fileSHA8(assetC) error: %v", err)
	}
	if !hashes[ha] || !hashes[hc] {
		t.Fatalf("missing expected hashes in result map")
	}
}

func TestCmdIngestPhotos_AllSessions_FromAssetsDir(t *testing.T) {
	tmp := t.TempDir()
	studyRoot := filepath.Join(tmp, "study")
	mustCopyDir(t, filepath.Join("..", "..", "..", "study-eg"), studyRoot)

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

	if err := cmdIngestPhotos([]string{"--assets-dir", assetsDir}); err != nil {
		t.Fatalf("cmdIngestPhotos first run error: %v", err)
	}
	assertAssetCount(t, studyRoot, sessionA, 3)

	if err := cmdIngestPhotos([]string{"--assets-dir", assetsDir}); err != nil {
		t.Fatalf("cmdIngestPhotos second run error: %v", err)
	}
	assertAssetCount(t, studyRoot, sessionA, 3)
}

func mustWriteStepFile(t *testing.T, path string, fm map[string]any, body string) {
	t.Helper()
	if err := util.EnsureDir(filepath.Dir(path)); err != nil {
		t.Fatalf("EnsureDir error: %v", err)
	}
	if err := util.WriteFrontmatterFile(path, fm, body); err != nil {
		t.Fatalf("WriteFrontmatterFile error: %v", err)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := util.EnsureDir(filepath.Dir(path)); err != nil {
		t.Fatalf("EnsureDir error: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
}

func mustParseTS(t *testing.T, raw string) time.Time {
	t.Helper()
	ts, err := util.ParseTimestamp(raw)
	if err != nil {
		t.Fatalf("ParseTimestamp(%q) error: %v", raw, err)
	}
	return ts
}

func assertTimeEqual(t *testing.T, got time.Time, wantRaw string) {
	t.Helper()
	want := mustParseTS(t, wantRaw)
	if !got.Equal(want) {
		t.Fatalf("time mismatch: got=%s want=%s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func mustCopyDir(t *testing.T, srcRoot, dstRoot string) {
	t.Helper()
	err := filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dstRoot, rel)
		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, b, 0o644)
	})
	if err != nil {
		t.Fatalf("copy dir %s -> %s failed: %v", srcRoot, dstRoot, err)
	}
}

func assertAssetCount(t *testing.T, studyRoot, sessionSlug string, want int) {
	t.Helper()
	assetRoot := filepath.Join(studyRoot, "session", sessionSlug, "step")
	count := 0
	err := filepath.WalkDir(assetRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"asset"+string(filepath.Separator)) {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk assets failed: %v", err)
	}
	if count != want {
		t.Fatalf("session %s asset count=%d want=%d", sessionSlug, count, want)
	}
}
