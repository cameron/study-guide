package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"study-guide/src/internal/util"
)

func stubExportThumbnailFn(t *testing.T) {
	t.Helper()
	orig := exportImageThumbnailFn
	exportImageThumbnailFn = func(src, dst string, maxDim int) error {
		if err := util.EnsureDir(filepath.Dir(dst)); err != nil {
			return err
		}
		return os.WriteFile(dst, []byte("thumb:"+filepath.Base(src)+":"+strconv.Itoa(maxDim)), 0o644)
	}
	t.Cleanup(func() {
		exportImageThumbnailFn = orig
	})
}

func TestRunExport_DefaultDestinationCopiesAnonymizedStudySnapshot(t *testing.T) {
	stubExportThumbnailFn(t)
	root := filepath.Join(t.TempDir(), "study")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\nactive_session_slug: 01-01-2026-alpha\nhero_comparison:\n  left:\n    session: 01-01-2026-alpha\n    step: 01-step-one\n    asset_index: 0\n  right:\n    session: 02-01-2026-bravo\n    step: 01-step-one\n    asset_index: 0\n---\n\n# Comparison Study\n\nStudy preamble.\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\nProtocol summary should stay in the exported study.\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n", "Protocol summary should stay in the exported study.", "Step One", "Step Two"))
	mustWriteFile(t, filepath.Join(root, "subject-requirements.yaml"), "type: person\n")
	mustWriteFile(t, filepath.Join(root, "publish", "site", "index.html"), "derived")
	mustWriteFile(t, filepath.Join(root, "export", "old.txt"), "stale")
	mustWriteFile(t, filepath.Join(root, "bin", "sg"), "tooling")
	mustWriteFile(t, filepath.Join(root, ".git", "config"), "[core]\n")
	mustWriteFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-step-one", ".git", "config"), "[nested]\n")
	mustWriteFile(t, filepath.Join(root, ".tmux.workspace"), "layout")

	alphaSessionPath := filepath.Join(root, "session", "01-01-2026-alpha", "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(alphaSessionPath)); err != nil {
		t.Fatalf("EnsureDir session alpha failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(alphaSessionPath, map[string]any{}, "# Subjects\n\nAlpha Example (11111111-1111-4111-8111-111111111111)\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile session alpha failed: %v", err)
	}
	mustWriteStepFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-step-one", "step.sg.md"), map[string]any{
		"time_started": "10:00:00 01-01-2026",
	}, "")
	mustWriteFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-step-one", "asset", "one.jpg"), "one")
	mustWriteFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-step-one", "asset", "raw.heic"), "raw-heic")

	bravoSessionPath := filepath.Join(root, "session", "02-01-2026-bravo", "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(bravoSessionPath)); err != nil {
		t.Fatalf("EnsureDir session bravo failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(bravoSessionPath, map[string]any{}, "# Subjects\n\nBravo Sample (22222222-2222-4222-8222-222222222222)\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile session bravo failed: %v", err)
	}
	mustWriteStepFile(t, filepath.Join(root, "session", "02-01-2026-bravo", "step", "01-step-one", "step.sg.md"), map[string]any{
		"time_started": "10:00:00 02-01-2026",
	}, "")
	mustWriteFile(t, filepath.Join(root, "session", "02-01-2026-bravo", "step", "01-step-one", "asset", "two.jpg"), "two")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"export", "--once"}); code != 0 {
		t.Fatalf("Run(export) code=%d want=0", code)
	}

	exportRoot := filepath.Join(root, "export", filepath.Base(root))
	if _, err := os.Stat(filepath.Join(exportRoot, "session", "session-1", "step", "01-step-one", "asset", "one.jpg")); err != nil {
		t.Fatalf("expected exported asset under anonymized session path, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(exportRoot, "session", "session-1", "step", "01-step-one", "asset", "img", "144", "one.jpg")); err != nil {
		t.Fatalf("expected default exported derived image, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(exportRoot, "session", "session-1", "step", "01-step-one", "asset", "img", "144", "raw.jpg")); err != nil {
		t.Fatalf("expected exported derived image for heic source, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(exportRoot, "session", "session-1", "step", "01-step-one", "asset", "raw.heic")); !os.IsNotExist(err) {
		t.Fatalf("expected raw heic asset to be excluded from export, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(exportRoot, "publish")); !os.IsNotExist(err) {
		t.Fatalf("expected export to skip derived publish directory, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(exportRoot, "export", "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected export to skip pre-existing export directory, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(exportRoot, "bin")); !os.IsNotExist(err) {
		t.Fatalf("expected export to skip tooling bin directory, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(exportRoot, ".git")); !os.IsNotExist(err) {
		t.Fatalf("expected export to skip repo metadata directory, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(exportRoot, "session", "session-1", "step", "01-step-one", ".git")); !os.IsNotExist(err) {
		t.Fatalf("expected export to skip nested repo metadata directory, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(exportRoot, ".tmux.workspace")); !os.IsNotExist(err) {
		t.Fatalf("expected export to skip root tooling metadata file, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(exportRoot, "session", "01-01-2026-alpha")); !os.IsNotExist(err) {
		t.Fatalf("expected original session slug to be omitted from export, got err=%v", err)
	}
	subjectMapBytes, err := os.ReadFile(filepath.Join(root, "subject-map.txt"))
	if err != nil {
		t.Fatalf("ReadFile subject-map.txt failed: %v", err)
	}
	subjectMap := string(subjectMapBytes)
	for _, want := range []string{
		"Subject 1: Alpha Example",
		"Subject 2: Bravo Sample",
	} {
		if !strings.Contains(subjectMap, want) {
			t.Fatalf("expected study-root subject map to include %q, got:\n%s", want, subjectMap)
		}
	}

	studyFM, studyBody, err := util.ReadFrontmatterFile(filepath.Join(exportRoot, "study.sg.md"))
	if err != nil {
		t.Fatalf("ReadFrontmatterFile exported study failed: %v", err)
	}
	if got := studyFM["active_session_slug"]; got != "session-1" {
		t.Fatalf("expected exported active_session_slug to be anonymized, got %#v", got)
	}
	heroFM, ok := studyFM["hero_comparison"].(map[string]any)
	if !ok {
		t.Fatalf("expected exported study hero_comparison frontmatter, got %#v", studyFM["hero_comparison"])
	}
	leftHero, ok := heroFM["left"].(map[string]any)
	if !ok {
		t.Fatalf("expected exported study hero_comparison.left object, got %#v", heroFM["left"])
	}
	rightHero, ok := heroFM["right"].(map[string]any)
	if !ok {
		t.Fatalf("expected exported study hero_comparison.right object, got %#v", heroFM["right"])
	}
	if got := leftHero["session"]; got != "session-1" {
		t.Fatalf("expected exported hero_comparison.left.session to be anonymized, got %#v", got)
	}
	if got := rightHero["session"]; got != "session-2" {
		t.Fatalf("expected exported hero_comparison.right.session to be anonymized, got %#v", got)
	}
	originalFM, _, err := util.ReadFrontmatterFile(filepath.Join(root, "study.sg.md"))
	if err != nil {
		t.Fatalf("ReadFrontmatterFile source study failed: %v", err)
	}
	if got := originalFM["active_session_slug"]; got != "01-01-2026-alpha" {
		t.Fatalf("expected source active_session_slug to remain unchanged, got %#v", got)
	}
	originalHeroFM, ok := originalFM["hero_comparison"].(map[string]any)
	if !ok {
		t.Fatalf("expected source study hero_comparison frontmatter, got %#v", originalFM["hero_comparison"])
	}
	originalLeftHero, ok := originalHeroFM["left"].(map[string]any)
	if !ok {
		t.Fatalf("expected source study hero_comparison.left object, got %#v", originalHeroFM["left"])
	}
	originalRightHero, ok := originalHeroFM["right"].(map[string]any)
	if !ok {
		t.Fatalf("expected source study hero_comparison.right object, got %#v", originalHeroFM["right"])
	}
	if got := originalLeftHero["session"]; got != "01-01-2026-alpha" {
		t.Fatalf("expected source hero_comparison.left.session to remain unchanged, got %#v", got)
	}
	if got := originalRightHero["session"]; got != "02-01-2026-bravo" {
		t.Fatalf("expected source hero_comparison.right.session to remain unchanged, got %#v", got)
	}
	if !strings.Contains(studyBody, "# Results\n\nObserved rouleaux.") {
		t.Fatalf("expected exported study to preserve existing results content, got:\n%s", studyBody)
	}
	for _, want := range []string{
		"Study preamble.",
		"# Methods\n\nProtocol summary should stay in the exported study.",
		"## Protocol\n\n### Step One\n\n### Step Two",
		"# Discussion\n\nNotes.",
		"# Conclusion\n\nDone.",
	} {
		if !strings.Contains(studyBody, want) {
			t.Fatalf("expected exported study to preserve %q, got:\n%s", want, studyBody)
		}
	}
	for _, want := range []string{
		"## Session session-1",
		"Subjects: Subject 1",
		"### Step 01-step-one",
		"session/session-1/step/01-step-one/asset/one.jpg",
		"session/session-1/step/01-step-one/asset/img/144/raw.jpg",
	} {
		if !strings.Contains(studyBody, want) {
			t.Fatalf("expected exported study results to include %q, got:\n%s", want, studyBody)
		}
	}
	if strings.Contains(studyBody, "session/session-1/step/01-step-one/asset/raw.heic") {
		t.Fatalf("did not expect exported study results to reference excluded raw heic asset, got:\n%s", studyBody)
	}

	sessionBodyBytes, err := os.ReadFile(filepath.Join(exportRoot, "session", "session-1", "session.sg.md"))
	if err != nil {
		t.Fatalf("ReadFile exported session failed: %v", err)
	}
	sessionBody := string(sessionBodyBytes)
	if !strings.Contains(sessionBody, "Subject 1") {
		t.Fatalf("expected exported session subject to be anonymized, got:\n%s", sessionBody)
	}
	for _, unwanted := range []string{
		"Alpha Example",
		"11111111-1111-4111-8111-111111111111",
	} {
		if strings.Contains(sessionBody, unwanted) {
			t.Fatalf("did not expect %q in exported session body:\n%s", unwanted, sessionBody)
		}
	}
}

func TestRunExport_CustomDestinationWritesSnapshotThere(t *testing.T) {
	stubExportThumbnailFn(t)
	root := filepath.Join(t.TempDir(), "study")
	dest := filepath.Join(t.TempDir(), "snapshot")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n", "One capture step.", "Step One"))
	alphaSessionPath := filepath.Join(root, "session", "01-01-2026-alpha", "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(alphaSessionPath)); err != nil {
		t.Fatalf("EnsureDir session alpha failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(alphaSessionPath, map[string]any{}, "# Subjects\n\nAlpha Example\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile session alpha failed: %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"export", "--once", dest}); code != 0 {
		t.Fatalf("Run(export custom-dest) code=%d want=0", code)
	}

	exportRoot := filepath.Join(dest, filepath.Base(root))
	if _, err := os.Stat(filepath.Join(exportRoot, "study.sg.md")); err != nil {
		t.Fatalf("expected exported study at custom destination, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "export", filepath.Base(root), "study.sg.md")); !os.IsNotExist(err) {
		t.Fatalf("did not expect default export output when custom destination is provided, got err=%v", err)
	}
}

func TestRunExport_PrintsTimestampedCompletionLine(t *testing.T) {
	stubExportThumbnailFn(t)
	root := filepath.Join(t.TempDir(), "study")
	dest := filepath.Join(t.TempDir(), "snapshot")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n", "One capture step.", "Step One"))
	alphaSessionPath := filepath.Join(root, "session", "01-01-2026-alpha", "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(alphaSessionPath)); err != nil {
		t.Fatalf("EnsureDir session alpha failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(alphaSessionPath, map[string]any{}, "# Subjects\n\nAlpha Example\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile session alpha failed: %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	out := captureStdout(t, func() {
		if code := Run([]string{"export", "--once", dest}); code != 0 {
			t.Fatalf("Run(export custom-dest) code=%d want=0", code)
		}
	})

	exportRoot := filepath.Join(dest, filepath.Base(root))
	pattern := regexp.MustCompile(`(?m)^exported at [0-9]{2}:[0-9]{2}:[0-9]{2} [0-9]{2}-[0-9]{2}-[0-9]{4} to ` + regexp.QuoteMeta(exportRoot) + `$`)
	if !pattern.MatchString(strings.TrimSpace(out)) {
		t.Fatalf("expected timestamped export completion line, got %q", out)
	}
}

func TestRunExport_StudyMethodsIncludeProtocolSummaryAndSteps(t *testing.T) {
	stubExportThumbnailFn(t)
	root := filepath.Join(t.TempDir(), "study")
	dest := filepath.Join(t.TempDir(), "snapshot")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\nAuthored methods note.\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n", "Authored methods note.", "Step One", "Step Two"))
	alphaSessionPath := filepath.Join(root, "session", "01-01-2026-alpha", "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(alphaSessionPath)); err != nil {
		t.Fatalf("EnsureDir session alpha failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(alphaSessionPath, map[string]any{}, "# Subjects\n\nAlpha Example\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile session alpha failed: %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"export", "--once", dest}); code != 0 {
		t.Fatalf("Run(export custom-dest) code=%d want=0", code)
	}

	_, studyBody, err := util.ReadFrontmatterFile(filepath.Join(dest, filepath.Base(root), "study.sg.md"))
	if err != nil {
		t.Fatalf("ReadFrontmatterFile exported study failed: %v", err)
	}
	for _, want := range []string{
		"# Methods\n\nAuthored methods note.",
		"## Protocol",
		"### Step One",
		"### Step Two",
	} {
		if !strings.Contains(studyBody, want) {
			t.Fatalf("expected exported study methods to include %q, got:\n%s", want, studyBody)
		}
	}
}

func TestRunExport_PreservesStudyPreambleBeforeIntroduction(t *testing.T) {
	stubExportThumbnailFn(t)
	root := filepath.Join(t.TempDir(), "study")
	dest := filepath.Join(t.TempDir(), "snapshot")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n_A reduction in rouleaux reveals a new mechanism?_\n\nAbstract section one.\n\nAbstract section two.\n\n_This paper is a work-in-progress._\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\nAuthored methods note.\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n", "Authored methods note.", "Step One"))
	alphaSessionPath := filepath.Join(root, "session", "01-01-2026-alpha", "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(alphaSessionPath)); err != nil {
		t.Fatalf("EnsureDir session alpha failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(alphaSessionPath, map[string]any{}, "# Subjects\n\nAlpha Example\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile session alpha failed: %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"export", "--once", dest}); code != 0 {
		t.Fatalf("Run(export custom-dest) code=%d want=0", code)
	}

	_, studyBody, err := util.ReadFrontmatterFile(filepath.Join(dest, filepath.Base(root), "study.sg.md"))
	if err != nil {
		t.Fatalf("ReadFrontmatterFile exported study failed: %v", err)
	}
	wantPreamble := "# Comparison Study\n\n_A reduction in rouleaux reveals a new mechanism?_\n\nAbstract section one.\n\nAbstract section two.\n\n_This paper is a work-in-progress._\n\n# Introduction"
	if !strings.Contains(studyBody, wantPreamble) {
		t.Fatalf("expected exported study to preserve full pre-Introduction preamble, got:\n%s", studyBody)
	}
}

func TestRunExport_ThumbnailSizesCanBeConfigured(t *testing.T) {
	stubExportThumbnailFn(t)
	root := filepath.Join(t.TempDir(), "study")
	dest := filepath.Join(t.TempDir(), "snapshot")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n", "One capture step.", "Step One"))
	alphaSessionPath := filepath.Join(root, "session", "01-01-2026-alpha", "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(alphaSessionPath)); err != nil {
		t.Fatalf("EnsureDir session alpha failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(alphaSessionPath, map[string]any{}, "# Subjects\n\nAlpha Example\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile session alpha failed: %v", err)
	}
	mustWriteStepFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-step-one", "step.sg.md"), map[string]any{
		"time_started": "10:00:00 01-01-2026",
	}, "")
	mustWriteFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-step-one", "asset", "one.jpg"), "one")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"export", "--once", "--imgsize=96,240", dest}); code != 0 {
		t.Fatalf("Run(export --imgsize=...) code=%d want=0", code)
	}

	exportRoot := filepath.Join(dest, filepath.Base(root))
	for _, size := range []string{"96", "240"} {
		if _, err := os.Stat(filepath.Join(exportRoot, "session", "session-1", "step", "01-step-one", "asset", "img", size, "one.jpg")); err != nil {
			t.Fatalf("expected exported derived image for size %s, stat error: %v", size, err)
		}
	}
	if _, err := os.Stat(filepath.Join(exportRoot, "session", "session-1", "step", "01-step-one", "asset", "img", "144")); !os.IsNotExist(err) {
		t.Fatalf("did not expect default derived image size when explicit sizes are provided, got err=%v", err)
	}
}

func TestRunExport_ReusesCachedThumbnailsAcrossFullRebuilds(t *testing.T) {
	root := filepath.Join(t.TempDir(), "study")
	dest := filepath.Join(t.TempDir(), "snapshot")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n", "One capture step.", "Step One"))
	alphaSessionPath := filepath.Join(root, "session", "01-01-2026-alpha", "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(alphaSessionPath)); err != nil {
		t.Fatalf("EnsureDir session alpha failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(alphaSessionPath, map[string]any{}, "# Subjects\n\nAlpha Example\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile session alpha failed: %v", err)
	}
	mustWriteStepFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-step-one", "step.sg.md"), map[string]any{
		"time_started": "10:00:00 01-01-2026",
	}, "")
	mustWriteFile(t, filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-step-one", "asset", "one.jpg"), "one")

	orig := exportImageThumbnailFn
	thumbnailCalls := 0
	exportImageThumbnailFn = func(src, dst string, maxDim int) error {
		thumbnailCalls++
		if err := util.EnsureDir(filepath.Dir(dst)); err != nil {
			return err
		}
		return os.WriteFile(dst, []byte("thumb:"+filepath.Base(src)+":"+strconv.Itoa(maxDim)), 0o644)
	}
	t.Cleanup(func() {
		exportImageThumbnailFn = orig
	})

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"export", "--once", dest}); code != 0 {
		t.Fatalf("Run(export first pass) code=%d want=0", code)
	}
	if thumbnailCalls != 1 {
		t.Fatalf("expected first export to render one thumbnail, got %d", thumbnailCalls)
	}

	if code := Run([]string{"export", "--once", dest}); code != 0 {
		t.Fatalf("Run(export second pass) code=%d want=0", code)
	}
	if thumbnailCalls != 1 {
		t.Fatalf("expected second export to reuse cached thumbnail, got %d renders", thumbnailCalls)
	}

	exportRoot := filepath.Join(dest, filepath.Base(root))
	if _, err := os.Stat(filepath.Join(exportRoot, "session", "session-1", "step", "01-step-one", "asset", "img", "144", "one.jpg")); err != nil {
		t.Fatalf("expected exported derived image after rebuild, stat error: %v", err)
	}
}

func TestStageExportRebuildCache_UsesTmpLocationInsteadOfDestinationParent(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "snapshot")
	mustWriteFile(t, filepath.Join(dest, "session", "session-1", "step", "01-step-one", "asset", "img", "144", "one.jpg"), "cached-thumb")

	cache, err := stageExportRebuildCache(dest)
	if err != nil {
		t.Fatalf("stageExportRebuildCache error: %v", err)
	}
	if cache.root == "" || cache.parentDir == "" {
		t.Fatalf("expected cache paths, got %#v", cache)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(cache.parentDir)
	})

	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("expected destination to be moved aside, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(cache.root, "session", "session-1", "step", "01-step-one", "asset", "img", "144", "one.jpg")); err != nil {
		t.Fatalf("expected cached export contents to move into cache root, stat error: %v", err)
	}
	if parent := filepath.Dir(dest); strings.HasPrefix(cache.parentDir, parent+string(os.PathSeparator)) || cache.parentDir == parent {
		t.Fatalf("expected cache parent outside destination parent %q, got %q", parent, cache.parentDir)
	}
	if cache.parentDir != filepath.Clean("/tmp") && !strings.HasPrefix(cache.parentDir, filepath.Clean("/tmp")+string(os.PathSeparator)) {
		t.Fatalf("expected cache parent under /tmp, got %q", cache.parentDir)
	}
}
