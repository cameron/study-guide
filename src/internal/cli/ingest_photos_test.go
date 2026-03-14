package cli

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
		"focus_windows": []map[string]any{
			{"time_started": "10:00:00 01-01-2026", "time_finished": "10:04:59 01-01-2026"},
		},
	}, "# A\n")
	mustWriteStepFile(t, filepath.Join(sessionDir, "step", "b", "step.sg.md"), map[string]any{
		"time_started": "10:05:00 01-01-2026",
		"focus_windows": []map[string]any{
			{"time_started": "10:05:00 01-01-2026", "time_finished": "10:08:59 01-01-2026"},
		},
	}, "# B\n")
	mustWriteStepFile(t, filepath.Join(sessionDir, "step", "c", "step.sg.md"), map[string]any{
		"time_started":  "10:09:00 01-01-2026",
		"time_finished": "10:15:00 01-01-2026",
		"focus_windows": []map[string]any{
			{"time_started": "10:09:00 01-01-2026", "time_finished": "10:15:00 01-01-2026"},
		},
	}, "# C\n")

	windows, err := loadStepWindows(sessionDir, protocol)
	if err != nil {
		t.Fatalf("loadStepWindows returned error: %v", err)
	}
	if len(windows) != 3 {
		t.Fatalf("expected 3 windows, got %d", len(windows))
	}

	assertTimeEqual(t, windows[0].Start, "10:00:00 01-01-2026")
	assertTimeEqual(t, windows[0].End, "10:04:59 01-01-2026")
	if windows[0].Last {
		t.Fatalf("expected first window Last=false")
	}

	assertTimeEqual(t, windows[1].Start, "10:05:00 01-01-2026")
	assertTimeEqual(t, windows[1].End, "10:08:59 01-01-2026")
	if windows[1].Last {
		t.Fatalf("expected second window Last=false")
	}

	assertTimeEqual(t, windows[2].Start, "10:09:00 01-01-2026")
	assertTimeEqual(t, windows[2].End, "10:15:00 01-01-2026")
	if !windows[2].Last {
		t.Fatalf("expected last window Last=true")
	}
}

func TestLoadStepWindows_UsesFocusWindowsForOwnership(t *testing.T) {
	tmp := t.TempDir()
	sessionDir := filepath.Join(tmp, "session", "s1")
	protocol := store.Protocol{Steps: []store.ProtocolStep{
		{Name: "A", Slug: "a"},
		{Name: "B", Slug: "b"},
	}}

	mustWriteStepFile(t, filepath.Join(sessionDir, "step", "a", "step.sg.md"), map[string]any{
		"time_started":  "10:00:00 01-01-2026",
		"time_finished": "10:04:59 01-01-2026",
		"focus_windows": []map[string]any{
			{"time_started": "10:00:00 01-01-2026", "time_finished": "10:01:00 01-01-2026"},
			{"time_started": "10:03:00 01-01-2026", "time_finished": "10:04:59 01-01-2026"},
		},
	}, "# A\n")
	mustWriteStepFile(t, filepath.Join(sessionDir, "step", "b", "step.sg.md"), map[string]any{
		"time_started":  "10:05:00 01-01-2026",
		"time_finished": "10:10:00 01-01-2026",
		"focus_windows": []map[string]any{
			{"time_started": "10:05:00 01-01-2026", "time_finished": "10:10:00 01-01-2026"},
		},
	}, "# B\n")

	windows, err := loadStepWindows(sessionDir, protocol)
	if err != nil {
		t.Fatalf("loadStepWindows returned error: %v", err)
	}
	if len(windows) != 3 {
		t.Fatalf("expected 3 ownership windows from focus_windows, got %d", len(windows))
	}
	assertTimeEqual(t, windows[0].Start, "10:00:00 01-01-2026")
	assertTimeEqual(t, windows[0].End, "10:01:00 01-01-2026")
	if windows[0].StepSlug != "a" {
		t.Fatalf("expected first focus window to map to step a, got %q", windows[0].StepSlug)
	}
	assertTimeEqual(t, windows[1].Start, "10:03:00 01-01-2026")
	assertTimeEqual(t, windows[1].End, "10:04:59 01-01-2026")
	if windows[1].StepSlug != "a" {
		t.Fatalf("expected second focus window to map to step a, got %q", windows[1].StepSlug)
	}
	assertTimeEqual(t, windows[2].Start, "10:05:00 01-01-2026")
	assertTimeEqual(t, windows[2].End, "10:10:00 01-01-2026")
	if windows[2].StepSlug != "b" {
		t.Fatalf("expected third focus window to map to step b, got %q", windows[2].StepSlug)
	}
}

func TestLoadStepWindows_HonorsExplicitNonFinalFinishAtNextStepBoundary(t *testing.T) {
	tmp := t.TempDir()
	sessionDir := filepath.Join(tmp, "session", "s1")
	protocol := store.Protocol{Steps: []store.ProtocolStep{
		{Name: "A", Slug: "a"},
		{Name: "B", Slug: "b"},
	}}

	mustWriteStepFile(t, filepath.Join(sessionDir, "step", "a", "step.sg.md"), map[string]any{
		"time_started":  "10:00:00 01-01-2026",
		"time_finished": "10:05:00 01-01-2026",
		"focus_windows": []map[string]any{
			{"time_started": "10:00:00 01-01-2026", "time_finished": "10:05:00 01-01-2026"},
		},
	}, "# A\n")
	mustWriteStepFile(t, filepath.Join(sessionDir, "step", "b", "step.sg.md"), map[string]any{
		"time_started":  "10:05:00 01-01-2026",
		"time_finished": "10:10:00 01-01-2026",
		"focus_windows": []map[string]any{
			{"time_started": "10:05:00 01-01-2026", "time_finished": "10:10:00 01-01-2026"},
		},
	}, "# B\n")

	windows, err := loadStepWindows(sessionDir, protocol)
	if err != nil {
		t.Fatalf("loadStepWindows returned error: %v", err)
	}
	if len(windows) != 2 {
		t.Fatalf("expected 2 ownership windows, got %d", len(windows))
	}
	assertTimeEqual(t, windows[0].End, "10:05:00 01-01-2026")
}

func TestLoadStepWindows_RequiresFocusWindows(t *testing.T) {
	tmp := t.TempDir()
	sessionDir := filepath.Join(tmp, "session", "s1")
	protocol := store.Protocol{Steps: []store.ProtocolStep{{Name: "A", Slug: "a"}}}

	mustWriteStepFile(t, filepath.Join(sessionDir, "step", "a", "step.sg.md"), map[string]any{
		"time_started":  "10:00:00 01-01-2026",
		"time_finished": "10:10:00 01-01-2026",
	}, "# A\n")

	_, err := loadStepWindows(sessionDir, protocol)
	if err == nil {
		t.Fatalf("expected error for missing focus_windows")
	}
	if !strings.Contains(err.Error(), "focus_windows") {
		t.Fatalf("expected focus_windows error, got: %v", err)
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
			"focus_windows": []map[string]any{
				{"time_started": "10:00:00 01-01-2026", "time_finished": "10:00:30 01-01-2026"},
			},
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
			End:      mustParseTS(t, "10:04:59 01-01-2026"),
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
		{at: "10:04:58 01-01-2026", want: 0},
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

func TestDedupeCapturedAssetsByCaptureInstant_KeepsNewestModTime(t *testing.T) {
	if _, err := exec.LookPath("exiftool"); err != nil {
		t.Skip("exiftool not available")
	}

	tmp := t.TempDir()
	older := filepath.Join(tmp, "older.heic")
	newer := filepath.Join(tmp, "newer.heic")
	src := filepath.Join("..", "..", "..", "fixtures", "four-concurrently-data", "20260307-120156_8909c86f.heic")
	mustCopyFile(t, src, older)
	mustCopyFile(t, src, newer)

	oldTS := mustParseTS(t, "12:00:00 01-01-2026")
	newTS := mustParseTS(t, "12:00:01 01-01-2026")
	if err := os.Chtimes(older, oldTS, oldTS); err != nil {
		t.Fatalf("Chtimes older error: %v", err)
	}
	if err := os.Chtimes(newer, newTS, newTS); err != nil {
		t.Fatalf("Chtimes newer error: %v", err)
	}

	assets, err := buildCapturedAssets([]string{older, newer}, exifCaptureTime)
	if err != nil {
		t.Fatalf("buildCapturedAssets returned error: %v", err)
	}
	got := dedupeCapturedAssetsByCaptureInstant(assets)
	if len(got) != 1 {
		t.Fatalf("expected one deduped asset, got %d (%#v)", len(got), got)
	}
	if got[0].path != newer {
		t.Fatalf("expected newer file to win dedupe: got=%q want=%q", got[0].path, newer)
	}
}

func TestDedupeCapturedAssetsByCaptureInstant_DoesNotDedupeSecondPrecisionOnly(t *testing.T) {
	a := capturedAsset{path: "/tmp/a.heic", captureTime: mustParseTS(t, "10:00:00 01-01-2026")}
	b := capturedAsset{path: "/tmp/b.heic", captureTime: mustParseTS(t, "10:00:00 01-01-2026")}
	got := dedupeCapturedAssetsByCaptureInstant([]capturedAsset{a, b})
	if len(got) != 2 {
		t.Fatalf("expected second-precision captures to remain distinct, got %d", len(got))
	}
}

func TestDedupeCapturedAssetsByCaptureInstant_PrefersRenderPath(t *testing.T) {
	tmp := t.TempDir()
	original := filepath.Join(tmp, "Photos Library.photoslibrary", "originals", "1", "A.heic")
	render := filepath.Join(tmp, "Photos Library.photoslibrary", "resources", "renders", "1", "A_1_201_a.heic")
	mustWriteFile(t, original, "original")
	mustWriteFile(t, render, "render")

	older := mustParseTS(t, "12:00:00 01-01-2026")
	newer := mustParseTS(t, "12:00:01 01-01-2026")
	if err := os.Chtimes(original, newer, newer); err != nil {
		t.Fatalf("Chtimes original error: %v", err)
	}
	if err := os.Chtimes(render, older, older); err != nil {
		t.Fatalf("Chtimes render error: %v", err)
	}

	at := mustParseTS(t, "10:00:00 01-01-2026").Add(123 * time.Millisecond)
	got := dedupeCapturedAssetsByCaptureInstant([]capturedAsset{
		{path: original, captureTime: at},
		{path: render, captureTime: at},
	})
	if len(got) != 1 {
		t.Fatalf("expected one deduped asset, got %d", len(got))
	}
	if got[0].path != render {
		t.Fatalf("expected render path to win dedupe: got=%q want=%q", got[0].path, render)
	}
}

func TestCmdIngestPhotos_AllSessions_FromAssetsDir(t *testing.T) {
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

	if err := cmdIngestPhotos([]string{"--assets-dir", assetsDir}); err != nil {
		t.Fatalf("cmdIngestPhotos first run error: %v", err)
	}
	assertAssetCount(t, studyRoot, sessionA, 3)

	if err := cmdIngestPhotos([]string{"--assets-dir", assetsDir}); err != nil {
		t.Fatalf("cmdIngestPhotos second run error: %v", err)
	}
	assertAssetCount(t, studyRoot, sessionA, 3)
}

func TestCmdIngestPhotos_StudyCompleteFixture_FromAssetsDir(t *testing.T) {
	tmp := t.TempDir()
	studyRoot := filepath.Join(tmp, "study")
	mustCopyDir(t, filepath.Join("..", "..", "..", "fixtures", "study-complete"), studyRoot)
	mustPopulateFocusWindowsFromStepTimes(t, studyRoot)

	sessionSlug := "18-02-2026-boehmer"
	stepRoot := filepath.Join(studyRoot, "session", sessionSlug, "step")
	for _, step := range []string{"01-first-exposure", "02-ground", "03-second-exposure"} {
		assetDir := filepath.Join(stepRoot, step, "asset")
		if err := os.RemoveAll(assetDir); err != nil {
			t.Fatalf("RemoveAll %s error: %v", assetDir, err)
		}
		if err := os.MkdirAll(assetDir, 0o755); err != nil {
			t.Fatalf("MkdirAll %s error: %v", assetDir, err)
		}
	}

	assetsDir, err := filepath.Abs(filepath.Join("..", "..", "..", "fixtures", "study-complete-assets"))
	if err != nil {
		t.Fatalf("Abs assets dir error: %v", err)
	}

	assetTimes := map[string]string{
		"20260218-232533_583457f3.heic": "23:25:33 18-02-2026",
		"20260218-234841_f428ad30.heic": "23:48:41 18-02-2026",
		"20260218-234906_df3cf56d.heic": "23:49:06 18-02-2026",
		"20260218-234913_b0946212.heic": "23:49:13 18-02-2026",
		"20260218-234933_d724160a.heic": "23:49:33 18-02-2026",
		"20260219-000224_1decaf8d.heic": "00:02:24 19-02-2026",
		"20260219-000319_c49dc602.heic": "00:03:19 19-02-2026",
		"20260222-124813_65e44e5a.png":  "12:48:13 22-02-2026",
		"20260302-225319_83be6e9a.png":  "22:53:19 02-03-2026",
	}
	origCapture := exifCaptureTimeFn
	exifCaptureTimeFn = func(path string) (time.Time, error) {
		raw, ok := assetTimes[filepath.Base(path)]
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
		t.Fatalf("cmdIngestPhotos error: %v", err)
	}

	assertStepAssetCount(t, studyRoot, sessionSlug, "01-first-exposure", 1)
	assertStepAssetCount(t, studyRoot, sessionSlug, "02-ground", 4)
	assertStepAssetCount(t, studyRoot, sessionSlug, "03-second-exposure", 2)
	assertAssetCount(t, studyRoot, sessionSlug, 7)
}

func TestCmdIngestPhotos_FourConcurrentlyFixture_UsesEmbeddedSubjectStepMetadata(t *testing.T) {
	if _, err := exec.LookPath("exiftool"); err != nil {
		t.Skip("exiftool not available")
	}

	tmp := t.TempDir()
	studyRoot := filepath.Join(tmp, "study")
	mustCopyDir(t, filepath.Join("..", "..", "..", "fixtures", "four-concurrently"), studyRoot)
	for _, slug := range []string{"13-03-2026-boehmer", "13-03-2026-marco", "14-03-2026-test"} {
		if err := os.RemoveAll(filepath.Join(studyRoot, "session", slug)); err != nil {
			t.Fatalf("RemoveAll fixture session %s error: %v", slug, err)
		}
	}
	mustWidenPointFocusWindows(t, studyRoot, time.Second)

	preCount := countIngestedAssetsInStudy(t, studyRoot)
	if preCount != 0 {
		t.Fatalf("fixture precondition failed: expected zero pre-ingested assets, got %d", preCount)
	}

	assetsDir, err := filepath.Abs(filepath.Join("..", "..", "..", "fixtures", "four-concurrently-data"))
	if err != nil {
		t.Fatalf("Abs assets dir error: %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(studyRoot); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if err := cmdIngestPhotos([]string{"--assets-dir", assetsDir}); err != nil {
		t.Fatalf("cmdIngestPhotos error: %v", err)
	}

	postCount := countIngestedAssetsInStudy(t, studyRoot)
	if postCount != 16 {
		t.Fatalf("expected 16 ingested assets, got %d", postCount)
	}

	sessionRoot := filepath.Join(studyRoot, "session")
	err = filepath.WalkDir(sessionRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.Contains(path, string(filepath.Separator)+"asset"+string(filepath.Separator)) {
			return nil
		}
		if strings.EqualFold(filepath.Base(path), ".DS_Store") {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(path), "/")
		if len(parts) < 5 {
			return nil
		}
		n := len(parts)
		sessionSlug := parts[n-5]
		stepSlug := parts[n-3]
		subject, step, err := readEmbeddedFixtureSubjectStep(path)
		if err != nil {
			return err
		}
		if !strings.HasSuffix(sessionSlug, "-"+subject) {
			return errors.New("embedded subject does not match destination session for " + path)
		}
		if step != stepSlug {
			return errors.New("embedded step does not match destination step for " + path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("embedded metadata validation failed: %v", err)
	}
}

func TestCmdIngestPhotos_ParsesExifOncePerSourceAcrossSessions(t *testing.T) {
	tmp := t.TempDir()
	studyRoot := filepath.Join(tmp, "study")
	mustCopyDir(t, filepath.Join("..", "..", "..", "fixtures", "study-eg"), studyRoot)
	mustPopulateFocusWindowsFromStepTimes(t, studyRoot)
	mustCopyDir(
		t,
		filepath.Join(studyRoot, "session", "18-02-2026-boehmer"),
		filepath.Join(studyRoot, "session", "18-02-2026-boehmer-copy"),
	)

	assetsDir, err := filepath.Abs(filepath.Join("testdata", "ingest-assets"))
	if err != nil {
		t.Fatalf("Abs assets dir error: %v", err)
	}
	sources, err := collectImageFiles(assetsDir)
	if err != nil {
		t.Fatalf("collectImageFiles error: %v", err)
	}
	assetTimes := map[string]string{
		"first-a.jpg":  "23:25:00 18-02-2026",
		"first-b.jpg":  "23:30:00 18-02-2026",
		"ground-a.jpg": "23:40:00 18-02-2026",
		"outside.jpg":  "22:00:00 18-02-2026",
	}
	calls := 0
	origCapture := exifCaptureTimeFn
	exifCaptureTimeFn = func(path string) (time.Time, error) {
		calls++
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
		t.Fatalf("cmdIngestPhotos error: %v", err)
	}
	if calls != len(sources) {
		t.Fatalf("expected EXIF parser calls once per source: got=%d want=%d", calls, len(sources))
	}
}

func TestCmdIngestPhotos_DefaultMode_UsesConfiguredPhotosLibraryPath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("default ingest mode is macOS-only")
	}

	tmp := t.TempDir()
	studyRoot := filepath.Join(tmp, "study")
	mustCopyDir(t, filepath.Join("..", "..", "..", "fixtures", "study-eg"), studyRoot)
	mustPopulateFocusWindowsFromStepTimes(t, studyRoot)
	sessionA := "18-02-2026-boehmer"

	photosDir := filepath.Join(tmp, "photos-library-feed")
	if err := os.MkdirAll(photosDir, 0o755); err != nil {
		t.Fatalf("MkdirAll photos dir error: %v", err)
	}
	srcAssetsDir, err := filepath.Abs(filepath.Join("testdata", "ingest-assets"))
	if err != nil {
		t.Fatalf("Abs assets dir error: %v", err)
	}
	assets, err := collectImageFiles(srcAssetsDir)
	if err != nil {
		t.Fatalf("collectImageFiles src assets error: %v", err)
	}
	for _, src := range assets {
		dst := filepath.Join(photosDir, filepath.Base(src))
		b, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("ReadFile asset %s error: %v", src, err)
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			t.Fatalf("WriteFile asset %s error: %v", dst, err)
		}
	}
	farAway := filepath.Join(photosDir, "far-away.jpg")
	mustWriteFile(t, farAway, "far")

	home := filepath.Join(tmp, "home")
	configPath := filepath.Join(home, ".study-guide", "config")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll config dir error: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("photos_library_path: "+photosDir+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile config error: %v", err)
	}
	t.Setenv("HOME", home)

	assetTimes := map[string]string{
		"first-a.jpg":  "23:25:00 18-02-2026",
		"first-b.jpg":  "23:30:00 18-02-2026",
		"ground-a.jpg": "23:40:00 18-02-2026",
		"outside.jpg":  "22:00:00 18-02-2026",
	}
	for name, raw := range assetTimes {
		path := filepath.Join(photosDir, name)
		ts := mustParseTS(t, raw)
		if err := os.Chtimes(path, ts, ts); err != nil {
			t.Fatalf("Chtimes %s error: %v", path, err)
		}
	}
	farTS := mustParseTS(t, "10:00:00 01-01-2028")
	if err := os.Chtimes(farAway, farTS, farTS); err != nil {
		t.Fatalf("Chtimes farAway error: %v", err)
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

	if err := cmdIngestPhotos(nil); err != nil {
		t.Fatalf("cmdIngestPhotos default mode error: %v", err)
	}
	assertAssetCount(t, studyRoot, sessionA, 3)
}

func TestCollectPhotosLibraryImageFilesBySQLite_MapsRowsToOriginalPaths(t *testing.T) {
	tmp := t.TempDir()
	libraryRoot := filepath.Join(tmp, "Photos Library.photoslibrary")
	originalsRoot := filepath.Join(libraryRoot, "originals")
	if err := os.MkdirAll(filepath.Join(originalsRoot, "4"), 0o755); err != nil {
		t.Fatalf("MkdirAll originals dir error: %v", err)
	}
	existing := filepath.Join(originalsRoot, "4", "A.heic")
	mustWriteFile(t, existing, "a")

	origQuery := runSQLiteQueryFn
	runSQLiteQueryFn = func(dbPath, query string) (string, error) {
		return "4|A.heic\n4|MISSING.heic\n4|clip.mov\ninvalid\n", nil
	}
	defer func() { runSQLiteQueryFn = origQuery }()

	got, err := collectPhotosLibraryImageFilesBySQLite(
		filepath.Join(libraryRoot, "database", "Photos.sqlite"),
		originalsRoot,
		mustParseTS(t, "10:00:00 01-01-2026"),
		mustParseTS(t, "11:00:00 01-01-2026"),
		0,
	)
	if err != nil {
		t.Fatalf("collectPhotosLibraryImageFilesBySQLite returned error: %v", err)
	}
	if len(got) != 1 || got[0] != existing {
		t.Fatalf("unexpected mapped originals from sqlite rows: %#v", got)
	}
}

func TestCmdIngestPhotos_DefaultMode_UsesSQLiteAssetDiscoveryWhenDatabaseExists(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("default ingest mode is macOS-only")
	}

	tmp := t.TempDir()
	studyRoot := filepath.Join(tmp, "study")
	mustCopyDir(t, filepath.Join("..", "..", "..", "fixtures", "study-eg"), studyRoot)
	mustPopulateFocusWindowsFromStepTimes(t, studyRoot)
	sessionA := "18-02-2026-boehmer"

	libraryRoot := filepath.Join(tmp, "Photos Library.photoslibrary")
	originalsDir := filepath.Join(libraryRoot, "originals", "4")
	if err := os.MkdirAll(originalsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll originals dir error: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(libraryRoot, "database"), 0o755); err != nil {
		t.Fatalf("MkdirAll database dir error: %v", err)
	}
	dbPath := filepath.Join(libraryRoot, "database", "Photos.sqlite")
	mustWriteFile(t, dbPath, "")

	src := filepath.Join("testdata", "ingest-assets", "first-a.jpg")
	dbNamedAsset := filepath.Join(originalsDir, "4B7F46CC-5DBF-4185-A5A5-14D6096E8FB6.heic")
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("ReadFile asset %s error: %v", src, err)
	}
	if err := os.WriteFile(dbNamedAsset, b, 0o644); err != nil {
		t.Fatalf("WriteFile asset %s error: %v", dbNamedAsset, err)
	}
	// Outside mtime envelope; should still be discovered through SQLite metadata rows.
	farTS := mustParseTS(t, "10:00:00 01-01-2028")
	if err := os.Chtimes(dbNamedAsset, farTS, farTS); err != nil {
		t.Fatalf("Chtimes %s error: %v", dbNamedAsset, err)
	}

	home := filepath.Join(tmp, "home")
	configPath := filepath.Join(home, ".study-guide", "config")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll config dir error: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("photos_library_path: "+libraryRoot+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile config error: %v", err)
	}
	t.Setenv("HOME", home)

	origQuery := runSQLiteQueryFn
	runSQLiteQueryFn = func(gotDBPath, query string) (string, error) {
		if gotDBPath != dbPath {
			t.Fatalf("unexpected db path: got=%q want=%q", gotDBPath, dbPath)
		}
		return "4|4B7F46CC-5DBF-4185-A5A5-14D6096E8FB6.heic\n", nil
	}
	defer func() { runSQLiteQueryFn = origQuery }()

	origCapture := exifCaptureTimeFn
	exifCaptureTimeFn = func(path string) (time.Time, error) {
		if path != dbNamedAsset {
			return time.Time{}, errors.New("unexpected asset")
		}
		return mustParseTS(t, "23:25:33 18-02-2026"), nil
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

	if err := cmdIngestPhotos(nil); err != nil {
		t.Fatalf("cmdIngestPhotos default mode error: %v", err)
	}
	assertAssetCount(t, studyRoot, sessionA, 1)
}

func TestParseIngestPhotosArgs_RejectsPositionalAlbumName(t *testing.T) {
	_, err := parseIngestPhotosArgs([]string{"SG Ingest"})
	if err == nil {
		t.Fatalf("expected positional album name to be rejected")
	}
	if !strings.Contains(err.Error(), "album name positional arguments are no longer supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDefaultPhotosLibrarySourceDir_FailsLoudlyWhenExpectedPathsMissing(t *testing.T) {
	home := t.TempDir()
	_, err := defaultPhotosLibrarySourceDir(home)
	if err == nil {
		t.Fatalf("expected missing Photos Library path error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Photos Library") {
		t.Fatalf("expected Photos Library wording, got %q", msg)
	}
	if !strings.Contains(msg, "originals") {
		t.Fatalf("expected checked subdir in error, got %q", msg)
	}
}

func TestDefaultPhotosLibrarySourceDir_UsesConfiguredPath(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".study-guide", "config")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll config dir error: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("photos_library_path: ~/ctx/photo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile config error: %v", err)
	}
	want := filepath.Join(home, "ctx", "photo")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatalf("MkdirAll configured path error: %v", err)
	}

	got, err := defaultPhotosLibrarySourceDir(home)
	if err != nil {
		t.Fatalf("defaultPhotosLibrarySourceDir returned error: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected configured source dir: got=%q want=%q", got, want)
	}
}

func TestDefaultPhotosLibrarySourceDir_ConfiguredPackageRootResolvesOriginals(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".study-guide", "config")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll config dir error: %v", err)
	}
	packageRoot := filepath.Join(home, "Pictures", "Photos Library.photoslibrary")
	originalsDir := filepath.Join(packageRoot, "originals")
	derivativesDir := filepath.Join(packageRoot, "derivatives")
	if err := os.MkdirAll(originalsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll originals dir error: %v", err)
	}
	if err := os.MkdirAll(derivativesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll derivatives dir error: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("photos_library_path: ~/Pictures/Photos Library.photoslibrary\n"), 0o644); err != nil {
		t.Fatalf("WriteFile config error: %v", err)
	}

	got, err := defaultPhotosLibrarySourceDir(home)
	if err != nil {
		t.Fatalf("defaultPhotosLibrarySourceDir returned error: %v", err)
	}
	if got != originalsDir {
		t.Fatalf("expected configured package root to resolve to originals dir: got=%q want=%q", got, originalsDir)
	}
}

func TestDefaultPhotosLibrarySourceDir_ConfiguredPathMissingFailsLoudly(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".study-guide", "config")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll config dir error: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("photos_library_path: ~/ctx/photo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile config error: %v", err)
	}

	_, err := defaultPhotosLibrarySourceDir(home)
	if err == nil {
		t.Fatalf("expected missing configured path error")
	}
	if !strings.Contains(err.Error(), filepath.Join(home, "ctx", "photo")) {
		t.Fatalf("expected configured path in error, got %q", err.Error())
	}
}

func TestDefaultPhotosLibrarySourceDir_UsesLegacySingularConfigKey(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".study-guide", "config")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll config dir error: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("photo_library_path: ~/ctx/photo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile config error: %v", err)
	}
	want := filepath.Join(home, "ctx", "photo")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatalf("MkdirAll configured path error: %v", err)
	}

	got, err := defaultPhotosLibrarySourceDir(home)
	if err != nil {
		t.Fatalf("defaultPhotosLibrarySourceDir returned error: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected configured source dir: got=%q want=%q", got, want)
	}
}

func TestDefaultPhotosLibrarySourceDir_WarnsOnUnrecognizedConfigKeys(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".study-guide", "config")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll config dir error: %v", err)
	}
	want := filepath.Join(home, "ctx", "photo")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatalf("MkdirAll configured path error: %v", err)
	}
	raw := "photos_library_path: ~/ctx/photo\nunexpected_key: true\n"
	if err := os.WriteFile(configPath, []byte(raw), 0o644); err != nil {
		t.Fatalf("WriteFile config error: %v", err)
	}

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe error: %v", err)
	}
	os.Stderr = w
	_, callErr := defaultPhotosLibrarySourceDir(home)
	_ = w.Close()
	os.Stderr = origStderr
	if callErr != nil {
		t.Fatalf("defaultPhotosLibrarySourceDir returned error: %v", callErr)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll stderr error: %v", err)
	}
	if !strings.Contains(string(out), "unrecognized config key") {
		t.Fatalf("expected unrecognized key warning, got %q", string(out))
	}
}

func TestCollectImageFilesByMTime_FiltersByEnvelope(t *testing.T) {
	tmp := t.TempDir()
	inRange := filepath.Join(tmp, "in-range.jpg")
	outOfRange := filepath.Join(tmp, "out-of-range.jpg")
	mustWriteFile(t, inRange, "a")
	mustWriteFile(t, outOfRange, "b")

	start := mustParseTS(t, "10:00:00 01-01-2026")
	end := mustParseTS(t, "11:00:00 01-01-2026")
	if err := os.Chtimes(inRange, start, mustParseTS(t, "10:30:00 01-01-2026")); err != nil {
		t.Fatalf("Chtimes inRange error: %v", err)
	}
	if err := os.Chtimes(outOfRange, start, mustParseTS(t, "13:30:00 01-01-2026")); err != nil {
		t.Fatalf("Chtimes outOfRange error: %v", err)
	}

	got, err := collectImageFilesByMTime(tmp, start, end, 0)
	if err != nil {
		t.Fatalf("collectImageFilesByMTime returned error: %v", err)
	}
	if len(got) != 1 || got[0] != inRange {
		t.Fatalf("unexpected files in envelope: %#v", got)
	}
}

func TestCollectImageFilesByMTime_ExcludesDerivativeAndPreviewSubtrees(t *testing.T) {
	tmp := t.TempDir()
	original := filepath.Join(tmp, "originals", "in-range.jpg")
	derivative := filepath.Join(tmp, "derivatives", "in-range.jpg")
	preview := filepath.Join(tmp, "previews", "in-range.jpg")
	mustWriteFile(t, original, "a")
	mustWriteFile(t, derivative, "b")
	mustWriteFile(t, preview, "c")

	start := mustParseTS(t, "10:00:00 01-01-2026")
	end := mustParseTS(t, "11:00:00 01-01-2026")
	inRange := mustParseTS(t, "10:30:00 01-01-2026")
	if err := os.Chtimes(original, start, inRange); err != nil {
		t.Fatalf("Chtimes original error: %v", err)
	}
	if err := os.Chtimes(derivative, start, inRange); err != nil {
		t.Fatalf("Chtimes derivative error: %v", err)
	}
	if err := os.Chtimes(preview, start, inRange); err != nil {
		t.Fatalf("Chtimes preview error: %v", err)
	}

	got, err := collectImageFilesByMTime(tmp, start, end, 0)
	if err != nil {
		t.Fatalf("collectImageFilesByMTime returned error: %v", err)
	}
	if len(got) != 1 || got[0] != original {
		t.Fatalf("unexpected files with derivatives/previews excluded: %#v", got)
	}
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

func mustCopyFile(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", src, err)
	}
	if err := util.EnsureDir(filepath.Dir(dst)); err != nil {
		t.Fatalf("EnsureDir %s error: %v", dst, err)
	}
	if err := os.WriteFile(dst, b, 0o644); err != nil {
		t.Fatalf("WriteFile %s error: %v", dst, err)
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

func mustPopulateFocusWindowsFromStepTimes(t *testing.T, studyRoot string) {
	t.Helper()
	protocol, err := store.ParseProtocol(studyRoot)
	if err != nil {
		t.Fatalf("ParseProtocol error: %v", err)
	}
	sessions, err := listSessionSlugs(studyRoot)
	if err != nil {
		t.Fatalf("listSessionSlugs error: %v", err)
	}
	for _, slug := range sessions {
		sessionDir := filepath.Join(studyRoot, "session", slug)
		starts := make([]time.Time, len(protocol.Steps))
		finishes := make([]time.Time, len(protocol.Steps))
		stepFiles := make([]string, len(protocol.Steps))
		skipSession := false
		for i, st := range protocol.Steps {
			stepPath := filepath.Join(sessionDir, "step", st.Slug, "step.sg.md")
			stepFiles[i] = stepPath
			fm, _, err := util.ReadFrontmatterFile(stepPath)
			if err != nil {
				if os.IsNotExist(err) {
					skipSession = true
					break
				}
				t.Fatalf("ReadFrontmatterFile %s error: %v", stepPath, err)
			}
			started := strings.TrimSpace(asString(fm["time_started"]))
			if started == "" {
				t.Fatalf("step missing time_started: %s", stepPath)
			}
			startTS, err := util.ParseTimestamp(started)
			if err != nil {
				t.Fatalf("ParseTimestamp started %s error: %v", stepPath, err)
			}
			starts[i] = startTS
			finished := strings.TrimSpace(asString(fm["time_finished"]))
			if finished != "" {
				finishTS, err := util.ParseTimestamp(finished)
				if err != nil {
					t.Fatalf("ParseTimestamp finished %s error: %v", stepPath, err)
				}
				finishes[i] = finishTS
			}
		}
		if skipSession {
			continue
		}
		last := len(protocol.Steps) - 1
		if finishes[last].IsZero() {
			t.Fatalf("last step missing time_finished: %s", stepFiles[last])
		}
		if finishes[last].Before(starts[last]) {
			finishes[last] = starts[last]
		}
		for i := 0; i < last; i++ {
			if finishes[i].IsZero() || !finishes[i].Before(starts[i+1]) {
				finishes[i] = starts[i+1].Add(-1 * time.Second)
			}
			if finishes[i].Before(starts[i]) {
				finishes[i] = starts[i]
			}
		}
		for i, st := range protocol.Steps {
			stepPath := filepath.Join(sessionDir, "step", st.Slug, "step.sg.md")
			fm, body, err := util.ReadFrontmatterFile(stepPath)
			if err != nil {
				t.Fatalf("ReadFrontmatterFile %s error: %v", stepPath, err)
			}
			fm["focus_windows"] = []map[string]any{
				{
					"time_started":  starts[i].Format(util.TimestampLayout),
					"time_finished": finishes[i].Format(util.TimestampLayout),
				},
			}
			if i == last {
				fm["time_finished"] = finishes[i].Format(util.TimestampLayout)
			}
			if err := util.WriteFrontmatterFile(stepPath, fm, body); err != nil {
				t.Fatalf("WriteFrontmatterFile %s error: %v", stepPath, err)
			}
		}
	}
}

func mustWidenPointFocusWindows(t *testing.T, studyRoot string, by time.Duration) {
	t.Helper()
	stepRoot := filepath.Join(studyRoot, "session")
	err := filepath.WalkDir(stepRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || filepath.Base(path) != "step.sg.md" {
			return nil
		}
		fm, body, err := util.ReadFrontmatterFile(path)
		if err != nil {
			return err
		}
		stepFinishRaw := strings.TrimSpace(asString(fm["time_finished"]))
		stepFinish := time.Time{}
		if stepFinishRaw != "" {
			if parsed, err := util.ParseTimestamp(stepFinishRaw); err == nil {
				stepFinish = parsed
			}
		}
		rawWindows, ok := fm["focus_windows"]
		if !ok {
			return nil
		}
		list, ok := rawWindows.([]any)
		if !ok {
			return nil
		}
		changed := false
		for i := range list {
			entry, ok := list[i].(map[string]any)
			if !ok {
				continue
			}
			startRaw := strings.TrimSpace(asString(entry["time_started"]))
			endRaw := strings.TrimSpace(asString(entry["time_finished"]))
			if startRaw == "" || endRaw == "" {
				continue
			}
			startTS, err := util.ParseTimestamp(startRaw)
			if err != nil {
				continue
			}
			endTS, err := util.ParseTimestamp(endRaw)
			if err != nil {
				continue
			}
			if startTS.Equal(endTS) {
				widenedEnd := startTS.Add(by)
				if !stepFinish.IsZero() && widenedEnd.After(stepFinish) {
					widenedEnd = stepFinish
				}
				if !widenedEnd.Equal(endTS) {
					entry["time_finished"] = widenedEnd.Format(util.TimestampLayout)
					changed = true
				}
			}
			list[i] = entry
		}
		if !changed {
			return nil
		}
		fm["focus_windows"] = list
		return util.WriteFrontmatterFile(path, fm, body)
	})
	if err != nil {
		t.Fatalf("mustWidenPointFocusWindows failed: %v", err)
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

func countIngestedAssetsInStudy(t *testing.T, studyRoot string) int {
	t.Helper()
	sessionRoot := filepath.Join(studyRoot, "session")
	count := 0
	err := filepath.WalkDir(sessionRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"asset"+string(filepath.Separator)) && !strings.EqualFold(filepath.Base(path), ".DS_Store") {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk assets failed: %v", err)
	}
	return count
}

func readEmbeddedFixtureSubjectStep(path string) (string, string, error) {
	cmd := exec.Command("exiftool", "-s3", "-ImageDescription", path)
	out, err := cmd.Output()
	if err != nil {
		return "", "", err
	}
	desc := strings.TrimSpace(string(out))
	const prefix = "sg_subject="
	if !strings.HasPrefix(desc, prefix) || !strings.Contains(desc, ";sg_step=") {
		return "", "", errors.New("invalid embedded fixture metadata in " + path)
	}
	chunks := strings.SplitN(strings.TrimPrefix(desc, prefix), ";sg_step=", 2)
	if len(chunks) != 2 {
		return "", "", errors.New("invalid embedded fixture metadata in " + path)
	}
	subject := strings.TrimSpace(chunks[0])
	step := strings.TrimSpace(chunks[1])
	if subject == "" || step == "" {
		return "", "", errors.New("empty embedded fixture metadata in " + path)
	}
	return subject, step, nil
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

func assertStepAssetCount(t *testing.T, studyRoot, sessionSlug, stepSlug string, want int) {
	t.Helper()
	assetRoot := filepath.Join(studyRoot, "session", sessionSlug, "step", stepSlug, "asset")
	entries, err := os.ReadDir(assetRoot)
	if err != nil {
		t.Fatalf("ReadDir %s error: %v", assetRoot, err)
	}
	got := 0
	for _, e := range entries {
		if !e.IsDir() {
			got++
		}
	}
	if got != want {
		t.Fatalf("session %s step %s asset count=%d want=%d", sessionSlug, stepSlug, got, want)
	}
}
