package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"study-guide/src/internal/store"
	"study-guide/src/internal/util"
)

func stubPublishThumbnailFn(t *testing.T) {
	t.Helper()
	orig := publishImageThumbnailFn
	publishImageThumbnailFn = func(src, dst string) error {
		if err := util.EnsureDir(filepath.Dir(dst)); err != nil {
			return err
		}
		return os.WriteFile(dst, []byte("thumb:"+src), 0o644)
	}
	t.Cleanup(func() {
		publishImageThumbnailFn = orig
	})
}

func TestRunPublish_RendersStudyHeroComparisonFromIndexedStepAssets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubPublishThumbnailFn(t)

	root := filepath.Join(t.TempDir(), "study")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\nhero_comparison:\n  left:\n    session: 01-01-2026-example\n    step: 01-step-one\n    asset_index: 1\n  right:\n    session: 01-01-2026-example\n    step: 02-step-two\n    asset_index: 0\n---\n\n# Comparison Study\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\nPlaceholder methods should be replaced by protocol summary.\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n", "Two capture steps.", "Step One", "Step Two"))
	mustWriteFile(t, filepath.Join(root, "subject-requirements.yaml"), "type: person\n")

	subject := store.Subject{
		UUID: "11111111-1111-4111-8111-111111111111",
		Type: "person",
		Name: "Alpha Example",
		Path: filepath.Join(home, ".study-guide", "subject", "alpha-example.sg.md"),
	}
	if _, err := store.SaveSubject(subject); err != nil {
		t.Fatalf("SaveSubject error: %v", err)
	}

	sessionSlug := "01-01-2026-example"
	sessionPath := filepath.Join(root, "session", sessionSlug, "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(sessionPath)); err != nil {
		t.Fatalf("EnsureDir session error: %v", err)
	}
	if err := util.WriteFrontmatterFile(sessionPath, map[string]any{}, "# Subjects\n\nAlpha Example (11111111-1111-4111-8111-111111111111)\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile session error: %v", err)
	}
	mustWriteStepFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "step.sg.md"), map[string]any{
		"time_started":  "10:00:00 01-01-2026",
		"time_finished": "10:10:00 01-01-2026",
	}, "")
	mustWriteStepFile(t, filepath.Join(root, "session", sessionSlug, "step", "02-step-two", "step.sg.md"), map[string]any{
		"time_started":  "10:20:00 01-01-2026",
		"time_finished": "10:30:00 01-01-2026",
	}, "")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "asset", "b.jpg"), "step-one-b")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "asset", "a.jpg"), "step-one-a")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "02-step-two", "asset", "c.jpg"), "step-two-c")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"publish", "--once"}); code != 0 {
		t.Fatalf("Run(publish) code=%d want=0", code)
	}

	publishRoot := filepath.Join(root, "publish", filepath.Base(root))
	indexPath := filepath.Join(publishRoot, "site", "index.html")
	indexHTML, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", indexPath, err)
	}
	page := string(indexHTML)
	for _, want := range []string{
		`class="hero-comparison"`,
		`class="hero-comparison-image"`,
		`hero/01-01-2026-example-01-step-one-1-b.jpg`,
		`hero/01-01-2026-example-02-step-two-0-c.jpg`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("expected publish index to contain %q, got:\n%s", want, page)
		}
	}
	if strings.Contains(page, `hero/01-01-2026-example-01-step-one-1-a.jpg`) {
		t.Fatalf("expected hero comparison to resolve asset_index against sorted step assets, got:\n%s", page)
	}
	for _, heroPath := range []string{
		filepath.Join(publishRoot, "site", "hero", "01-01-2026-example-01-step-one-1-b.jpg"),
		filepath.Join(publishRoot, "site", "hero", "01-01-2026-example-02-step-two-0-c.jpg"),
	} {
		if _, err := os.Stat(heroPath); err != nil {
			t.Fatalf("expected published hero asset %s, stat error: %v", heroPath, err)
		}
	}
}

func TestRunPublish_AllowsHeroComparisonToReferenceAnonymizedPublishedSessionSlug(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubPublishThumbnailFn(t)

	root := filepath.Join(t.TempDir(), "study")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\nhero_comparison:\n  left:\n    session: session-1\n    step: 01-step-one\n    asset_index: 1\n  right:\n    session: session-1\n    step: 02-step-two\n    asset_index: 0\n---\n\n# Comparison Study\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\nPlaceholder methods should be replaced by protocol summary.\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n", "Two capture steps.", "Step One", "Step Two"))
	mustWriteFile(t, filepath.Join(root, "subject-requirements.yaml"), "type: person\n")

	subject := store.Subject{
		UUID: "11111111-1111-4111-8111-111111111111",
		Type: "person",
		Name: "Alpha Example",
		Path: filepath.Join(home, ".study-guide", "subject", "alpha-example.sg.md"),
	}
	if _, err := store.SaveSubject(subject); err != nil {
		t.Fatalf("SaveSubject error: %v", err)
	}

	sessionSlug := "01-01-2026-example"
	sessionPath := filepath.Join(root, "session", sessionSlug, "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(sessionPath)); err != nil {
		t.Fatalf("EnsureDir session error: %v", err)
	}
	if err := util.WriteFrontmatterFile(sessionPath, map[string]any{}, "# Subjects\n\nAlpha Example (11111111-1111-4111-8111-111111111111)\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile session error: %v", err)
	}
	mustWriteStepFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "step.sg.md"), map[string]any{
		"time_started":  "10:00:00 01-01-2026",
		"time_finished": "10:10:00 01-01-2026",
	}, "")
	mustWriteStepFile(t, filepath.Join(root, "session", sessionSlug, "step", "02-step-two", "step.sg.md"), map[string]any{
		"time_started":  "10:20:00 01-01-2026",
		"time_finished": "10:30:00 01-01-2026",
	}, "")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "asset", "b.jpg"), "step-one-b")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "asset", "a.jpg"), "step-one-a")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "02-step-two", "asset", "c.jpg"), "step-two-c")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"publish", "--once"}); code != 0 {
		t.Fatalf("Run(publish) code=%d want=0", code)
	}

	publishRoot := filepath.Join(root, "publish", filepath.Base(root))
	indexHTML, err := os.ReadFile(filepath.Join(publishRoot, "site", "index.html"))
	if err != nil {
		t.Fatalf("ReadFile publish index error: %v", err)
	}
	page := string(indexHTML)
	for _, want := range []string{
		`hero/session-1-01-step-one-1-b.jpg`,
		`hero/session-1-02-step-two-0-c.jpg`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("expected publish index to contain %q, got:\n%s", want, page)
		}
	}
}

func TestRunPublish_UsesExplicitStepRenderAssetIndicesOrderAndSkipsUnlistedAssets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubPublishThumbnailFn(t)

	root := filepath.Join(t.TempDir(), "study")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\nPlaceholder methods should be replaced by protocol summary.\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n", "One capture step.", "Step One"))
	mustWriteFile(t, filepath.Join(root, "subject-requirements.yaml"), "type: person\n")

	subject := store.Subject{
		UUID: "11111111-1111-4111-8111-111111111111",
		Type: "person",
		Name: "Alpha Example",
		Path: filepath.Join(home, ".study-guide", "subject", "alpha-example.sg.md"),
	}
	if _, err := store.SaveSubject(subject); err != nil {
		t.Fatalf("SaveSubject error: %v", err)
	}

	sessionSlug := "01-01-2026-example"
	sessionPath := filepath.Join(root, "session", sessionSlug, "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(sessionPath)); err != nil {
		t.Fatalf("EnsureDir session error: %v", err)
	}
	if err := util.WriteFrontmatterFile(sessionPath, map[string]any{}, "# Subjects\n\nAlpha Example (11111111-1111-4111-8111-111111111111)\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile session error: %v", err)
	}
	mustWriteStepFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "step.sg.md"), map[string]any{
		"time_started":         "10:00:00 01-01-2026",
		"time_finished":        "10:10:00 01-01-2026",
		"render_asset_indices": []int{2, 0},
	}, "")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "asset", "b.jpg"), "step-one-b")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "asset", "a.jpg"), "step-one-a")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "asset", "c.jpg"), "step-one-c")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"publish", "--once"}); code != 0 {
		t.Fatalf("Run(publish) code=%d want=0", code)
	}

	publishRoot := filepath.Join(root, "publish", filepath.Base(root))
	sessionPagePath := filepath.Join(publishRoot, "site", "session", "session-1", "index.html")
	sessionHTML, err := os.ReadFile(sessionPagePath)
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", sessionPagePath, err)
	}
	page := string(sessionHTML)
	cPos := strings.Index(page, "assets/01-step-one/c.jpg")
	aPos := strings.Index(page, "assets/01-step-one/a.jpg")
	bPos := strings.Index(page, "assets/01-step-one/b.jpg")
	if cPos < 0 || aPos < 0 {
		t.Fatalf("expected session page to contain explicitly ordered assets, got:\n%s", page)
	}
	if bPos >= 0 {
		t.Fatalf("expected unlisted asset to be omitted from session page, got:\n%s", page)
	}
	if cPos > aPos {
		t.Fatalf("expected step assets to render in explicit index order, got:\n%s", page)
	}

	indexPath := filepath.Join(publishRoot, "site", "index.html")
	indexHTML, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", indexPath, err)
	}
	indexPage := string(indexHTML)
	cThumb := strings.Index(indexPage, "thumbs/session-1/01-step-one/c.jpg")
	aThumb := strings.Index(indexPage, "thumbs/session-1/01-step-one/a.jpg")
	bThumb := strings.Index(indexPage, "thumbs/session-1/01-step-one/b.jpg")
	if cThumb < 0 || aThumb < 0 {
		t.Fatalf("expected index page thumbnails to follow explicit asset order, got:\n%s", indexPage)
	}
	if bThumb >= 0 {
		t.Fatalf("expected unlisted asset thumbnail to be omitted from index page, got:\n%s", indexPage)
	}
	if cThumb > aThumb {
		t.Fatalf("expected index thumbnails to follow explicit index order, got:\n%s", indexPage)
	}
}

func TestRunPublish_ExcludesUnfocusableStepsFromSessionGallery(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubPublishThumbnailFn(t)

	root := filepath.Join(t.TempDir(), "study")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\nPlaceholder methods should be replaced by protocol summary.\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n", "Three protocol steps.", "Step One", "Step Two", "Step Three"))
	mustWriteFile(t, filepath.Join(root, "subject-requirements.yaml"), "type: person\n")

	subject := store.Subject{
		UUID: "11111111-1111-4111-8111-111111111111",
		Type: "person",
		Name: "Alpha Example",
		Path: filepath.Join(home, ".study-guide", "subject", "alpha-example.sg.md"),
	}
	if _, err := store.SaveSubject(subject); err != nil {
		t.Fatalf("SaveSubject error: %v", err)
	}

	sessionSlug := "01-01-2026-example"
	sessionPath := filepath.Join(root, "session", sessionSlug, "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(sessionPath)); err != nil {
		t.Fatalf("EnsureDir session error: %v", err)
	}
	if err := util.WriteFrontmatterFile(sessionPath, map[string]any{}, "# Subjects\n\nAlpha Example (11111111-1111-4111-8111-111111111111)\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile session error: %v", err)
	}
	mustWriteStepFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "step.sg.md"), map[string]any{
		"time_started":  "10:00:00 01-01-2026",
		"time_finished": "10:10:00 01-01-2026",
	}, "")
	mustWriteStepFile(t, filepath.Join(root, "session", sessionSlug, "step", "02-step-two", "step.sg.md"), map[string]any{
		"time_started":  "10:10:00 01-01-2026",
		"time_finished": "10:20:00 01-01-2026",
		"unfocusable":   true,
	}, "")
	mustWriteStepFile(t, filepath.Join(root, "session", sessionSlug, "step", "03-step-three", "step.sg.md"), map[string]any{
		"time_started":  "10:20:00 01-01-2026",
		"time_finished": "10:30:00 01-01-2026",
	}, "")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "asset", "one.jpg"), "step-one")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "02-step-two", "asset", "two.jpg"), "step-two")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "03-step-three", "asset", "three.jpg"), "step-three")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"publish", "--once"}); code != 0 {
		t.Fatalf("Run(publish) code=%d want=0", code)
	}

	publishRoot := filepath.Join(root, "publish", filepath.Base(root))
	sessionPagePath := filepath.Join(publishRoot, "site", "session", "session-1", "index.html")
	sessionHTML, err := os.ReadFile(sessionPagePath)
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", sessionPagePath, err)
	}
	page := string(sessionHTML)
	if strings.Contains(page, "Step Two") || strings.Contains(page, "assets/02-step-two/two.jpg") {
		t.Fatalf("expected unfocusable step to be omitted from session gallery, got:\n%s", page)
	}
	for _, want := range []string{"Step One", "Step Three", "assets/01-step-one/one.jpg", "assets/03-step-three/three.jpg"} {
		if !strings.Contains(page, want) {
			t.Fatalf("expected session gallery to contain %q, got:\n%s", want, page)
		}
	}

	indexHTML, err := os.ReadFile(filepath.Join(publishRoot, "site", "index.html"))
	if err != nil {
		t.Fatalf("ReadFile publish index error: %v", err)
	}
	indexPage := string(indexHTML)
	if strings.Contains(indexPage, "thumbs/session-1/02-step-two/two.jpg") {
		t.Fatalf("expected unfocusable step thumbnail to be omitted from publish index, got:\n%s", indexPage)
	}
	if _, err := os.Stat(filepath.Join(publishRoot, "site", "session", "session-1", "assets", "02-step-two", "two.jpg")); !os.IsNotExist(err) {
		t.Fatalf("expected no published asset for unfocusable step, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(publishRoot, "site", "thumbs", "session-1", "02-step-two", "two.jpg")); !os.IsNotExist(err) {
		t.Fatalf("expected no published thumbnail for unfocusable step, got err=%v", err)
	}
}

func TestPublishReferenceFixture_DemonstratesIndexedHeroAndExplicitRenderOrder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubPublishThumbnailFn(t)

	root := filepath.Join(t.TempDir(), "study")
	mustCopyDir(t, filepath.Join("..", "..", "..", "fixtures", "publish-reference"), root)

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"publish", "--once"}); code != 0 {
		t.Fatalf("Run(publish) code=%d want=0", code)
	}

	publishRoot := filepath.Join(root, "publish", filepath.Base(root))
	indexHTML, err := os.ReadFile(filepath.Join(publishRoot, "site", "index.html"))
	if err != nil {
		t.Fatalf("ReadFile publish index error: %v", err)
	}
	indexPage := string(indexHTML)
	for _, want := range []string{
		"hero/reference-session-01-step-one-1-b.jpg",
		"hero/reference-session-02-step-two-0-ground.jpg",
	} {
		if !strings.Contains(indexPage, want) {
			t.Fatalf("expected fixture publish index to contain %q, got:\n%s", want, indexPage)
		}
	}

	sessionHTML, err := os.ReadFile(filepath.Join(publishRoot, "site", "session", "session-1", "index.html"))
	if err != nil {
		t.Fatalf("ReadFile publish session error: %v", err)
	}
	sessionPage := string(sessionHTML)
	cPos := strings.Index(sessionPage, "assets/01-step-one/c.jpg")
	aPos := strings.Index(sessionPage, "assets/01-step-one/a.jpg")
	bPos := strings.Index(sessionPage, "assets/01-step-one/b.jpg")
	if cPos < 0 || aPos < 0 {
		t.Fatalf("expected fixture session page to include ordered step assets, got:\n%s", sessionPage)
	}
	if bPos >= 0 {
		t.Fatalf("expected fixture session page to omit unspecified step asset, got:\n%s", sessionPage)
	}
	if cPos > aPos {
		t.Fatalf("expected fixture session page to preserve explicit render order, got:\n%s", sessionPage)
	}
}

func TestRunPublish_GeneratesSessionComparisonPageWithAnonymizedSubjectsByDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubPublishThumbnailFn(t)

	root := filepath.Join(t.TempDir(), "study")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\nPlaceholder methods should be replaced by protocol summary.\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n", "Two capture steps.", "Step One", "Step Two"))
	mustWriteFile(t, filepath.Join(root, "subject-requirements.yaml"), "type: person\n")

	subject := store.Subject{
		UUID: "11111111-1111-4111-8111-111111111111",
		Type: "person",
		Name: "Alpha Example",
		Path: filepath.Join(home, ".study-guide", "subject", "alpha-example.sg.md"),
	}
	if _, err := store.SaveSubject(subject); err != nil {
		t.Fatalf("SaveSubject error: %v", err)
	}

	sessionSlug := "01-01-2026-example"
	sessionPath := filepath.Join(root, "session", sessionSlug, "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(sessionPath)); err != nil {
		t.Fatalf("EnsureDir session error: %v", err)
	}
	if err := util.WriteFrontmatterFile(sessionPath, map[string]any{}, "# Subjects\n\nAlpha Example (11111111-1111-4111-8111-111111111111)\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile session error: %v", err)
	}
	mustWriteStepFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "step.sg.md"), map[string]any{
		"time_started":  "10:00:00 01-01-2026",
		"time_finished": "10:10:00 01-01-2026",
	}, "")
	mustWriteStepFile(t, filepath.Join(root, "session", sessionSlug, "step", "02-step-two", "step.sg.md"), map[string]any{
		"time_started":  "10:20:00 01-01-2026",
		"time_finished": "10:30:00 01-01-2026",
	}, "")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "asset", "one.jpg"), "one")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "02-step-two", "asset", "two.jpg"), "two")

	nextSessionSlug := "02-01-2026-example"
	nextSessionPath := filepath.Join(root, "session", nextSessionSlug, "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(nextSessionPath)); err != nil {
		t.Fatalf("EnsureDir next session error: %v", err)
	}
	if err := util.WriteFrontmatterFile(nextSessionPath, map[string]any{}, "# Subjects\n\nAlpha Example (11111111-1111-4111-8111-111111111111)\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile next session error: %v", err)
	}
	mustWriteStepFile(t, filepath.Join(root, "session", nextSessionSlug, "step", "01-step-one", "step.sg.md"), map[string]any{
		"time_started":  "10:00:00 02-01-2026",
		"time_finished": "10:10:00 02-01-2026",
	}, "")
	mustWriteStepFile(t, filepath.Join(root, "session", nextSessionSlug, "step", "02-step-two", "step.sg.md"), map[string]any{
		"time_started":  "10:20:00 02-01-2026",
		"time_finished": "10:30:00 02-01-2026",
	}, "")
	mustWriteFile(t, filepath.Join(root, "session", nextSessionSlug, "step", "01-step-one", "asset", "three.jpg"), "three")
	mustWriteFile(t, filepath.Join(root, "session", nextSessionSlug, "step", "02-step-two", "asset", "four.jpg"), "four")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"publish", "--once"}); code != 0 {
		t.Fatalf("Run(publish) code=%d want=0", code)
	}

	publishRoot := filepath.Join(root, "publish", filepath.Base(root))
	indexPath := filepath.Join(publishRoot, "site", "index.html")
	indexHTML, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", indexPath, err)
	}
	if strings.Contains(string(indexHTML), `session/01-01-2026-example/index.html`) {
		t.Fatalf("expected anonymous publish index to avoid subject-derived session slug, got:\n%s", string(indexHTML))
	}
	if !strings.Contains(string(indexHTML), `session/session-1/index.html`) {
		t.Fatalf("expected publish index to link to anonymized session page, got:\n%s", string(indexHTML))
	}
	if !strings.Contains(string(indexHTML), `<h3><a href="session/session-1/index.html">Subject 1</a></h3>`) {
		t.Fatalf("expected publish index session title to use anonymized subject label by default, got:\n%s", string(indexHTML))
	}
	for _, want := range []string{
		"<h2>Introduction</h2><pre>Observe changes.</pre>",
		"<h2>Methods</h2><pre>Two capture steps.</pre><h3>Protocol</h3><ol><li>Step One</li><li>Step Two</li></ol>",
		"<h2>Results</h2><pre>Observed rouleaux.</pre><section><h3><a href=\"session/session-1/index.html\">Subject 1</a></h3>",
	} {
		if !strings.Contains(string(indexHTML), want) {
			t.Fatalf("expected publish index to contain %q, got:\n%s", want, string(indexHTML))
		}
	}
	if intro := strings.Index(string(indexHTML), "<h2>Introduction</h2>"); intro < 0 {
		t.Fatalf("expected publish index to include Introduction heading, got:\n%s", string(indexHTML))
	} else if methods := strings.Index(string(indexHTML), "<h2>Methods</h2>"); methods < 0 || methods < intro {
		t.Fatalf("expected Methods heading after Introduction, got:\n%s", string(indexHTML))
	} else if results := strings.Index(string(indexHTML), "<h2>Results</h2>"); results < 0 || results < methods {
		t.Fatalf("expected Results heading after Methods, got:\n%s", string(indexHTML))
	}
	for _, unwanted := range []string{
		"<h2>Hypotheses</h2>",
		"<h2>Protocol Summary</h2>",
		"<h2>Protocol Steps</h2>",
		"<h2>Sessions</h2>",
		"Started: 10:00:00 01-01-2026",
		"Finished: 10:30:00 01-01-2026",
		"<ul><li><strong>Step One</strong>",
		"images: 1",
		"Compare photos across steps",
	} {
		if strings.Contains(string(indexHTML), unwanted) {
			t.Fatalf("expected publish index session card to omit %q, got:\n%s", unwanted, string(indexHTML))
		}
	}

	sessionPagePath := filepath.Join(publishRoot, "site", "session", "session-1", "index.html")
	sessionHTML, err := os.ReadFile(sessionPagePath)
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", sessionPagePath, err)
	}
	page := string(sessionHTML)
	for _, want := range []string{
		"Subject 1",
		"01-01-2026",
		"Step One",
		"Step Two",
		"comparison-page",
		"step-column",
		"session-nav",
		`class="header-link" href="../../index.html"`,
		"image-size-controls",
		"orientation-controls",
		`type="radio"`,
		`name="layout-orientation"`,
		`value="columns"`,
		`value="rows"`,
		`checked`,
		"setOrientation('columns')",
		"setOrientation('rows')",
		`data-orientation="columns"`,
		"comparison-columns rows",
		"grid-template-columns:120px 1fr",
		".comparison-columns.rows .step-column h2",
		".comparison-columns.rows .step-images",
		"flex-direction:row",
		"width:var(--image-size)",
		"height:auto",
		"object-fit:contain",
		"overflow:visible",
		"setImageSize('50px')",
		"setImageSize('40vw')",
		`type="range"`,
		`min="50"`,
		`max="400"`,
		`setImageSizeFromSlider(this.value)`,
		`sliderValue <= 100`,
		`document.getElementById('image-size-slider').value = value`,
		`localStorage.getItem('sg_publish_image_size')`,
		`localStorage.setItem('sg_publish_image_size', size)`,
		`applyStoredImageSize()`,
		`localStorage.getItem('sg_publish_orientation')`,
		`localStorage.setItem('sg_publish_orientation', value)`,
		`applyStoredOrientation()`,
		"header-line",
		"header-link",
		"header-date",
		"header-subject",
		"header-sep",
		"gap:0",
		"padding:0 0 0",
		"margin-top:2px",
		"overflow-y:auto",
		"assets/01-step-one/one.jpg",
		"assets/02-step-two/two.jpg",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("expected session page to contain %q, got:\n%s", want, page)
		}
	}
	for _, unwanted := range []string{
		"wip-badge",
		"Back to index</a></p><div",
		"border-bottom:1px solid #d2c5b3",
		"padding:6px 0 0",
		".orientation-controls label{display:flex;align-items:center;gap:3px;border:",
	} {
		if strings.Contains(page, unwanted) {
			t.Fatalf("expected session page to omit %q, got:\n%s", unwanted, page)
		}
	}
	if !strings.Contains(page, `class="header-link" href="../../index.html">Up</a><span class="session-nav"><span class="header-sep">|</span><a class="session-link next-session-link" href="../session-2/index.html">Next</a></span><span class="header-sep">|</span><span class="header-date">01-01-2026</span><span class="header-sep">|</span><span class="header-subject">Subject 1</span>`) {
		t.Fatalf("expected compact one-line header order, got:\n%s", page)
	}
	if strings.Contains(page, `<button type="button">Up</button>`) {
		t.Fatalf("expected back control to remain a link, got:\n%s", page)
	}
	if !strings.Contains(page, `href="../session-2/index.html"`) {
		t.Fatalf("expected next session link, got:\n%s", page)
	}
	nextSessionPagePath := filepath.Join(publishRoot, "site", "session", "session-2", "index.html")
	nextSessionHTML, err := os.ReadFile(nextSessionPagePath)
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", nextSessionPagePath, err)
	}
	if !strings.Contains(string(nextSessionHTML), `href="../session-1/index.html"`) {
		t.Fatalf("expected previous session link on second session page, got:\n%s", string(nextSessionHTML))
	}
	firstThumb := strings.Index(string(indexHTML), "session/session-1/assets/01-step-one/one.jpg")
	secondThumb := strings.Index(string(indexHTML), "session/session-1/assets/02-step-two/two.jpg")
	if firstThumb >= 0 || secondThumb >= 0 {
		t.Fatalf("expected publish index to avoid full-size session asset paths, got:\n%s", string(indexHTML))
	}
	firstThumb = strings.Index(string(indexHTML), "thumbs/session-1/01-step-one/one.jpg")
	secondThumb = strings.Index(string(indexHTML), "thumbs/session-1/02-step-two/two.jpg")
	if firstThumb < 0 || secondThumb < 0 {
		t.Fatalf("expected publish index to include chrono-ordered thumbnails, got:\n%s", string(indexHTML))
	}
	if firstThumb > secondThumb {
		t.Fatalf("expected publish index thumbnails in chronological order, got:\n%s", string(indexHTML))
	}
	for _, thumbPath := range []string{
		filepath.Join(publishRoot, "site", "thumbs", "session-1", "01-step-one", "one.jpg"),
		filepath.Join(publishRoot, "site", "thumbs", "session-1", "02-step-two", "two.jpg"),
	} {
		if _, err := os.Stat(thumbPath); err != nil {
			t.Fatalf("expected published thumbnail %s, stat error: %v", thumbPath, err)
		}
	}
}

func TestRunPublish_RendersStudyPreambleBeforeIntroduction(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubPublishThumbnailFn(t)

	root := filepath.Join(t.TempDir(), "study")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n_A reduction in rouleaux reveals a new mechanism?_\n\nAbstract text.\n\n_This paper is a work-in-progress._\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\nPlaceholder methods should be replaced by protocol summary.\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n", "Two capture steps.", "Step One", "Step Two"))

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"publish", "--once"}); code != 0 {
		t.Fatalf("Run(publish) code=%d want=0", code)
	}

	publishRoot := filepath.Join(root, "publish", filepath.Base(root))
	indexHTML, err := os.ReadFile(filepath.Join(publishRoot, "site", "index.html"))
	if err != nil {
		t.Fatalf("ReadFile publish index error: %v", err)
	}
	page := string(indexHTML)
	if !strings.Contains(page, "<pre>_A reduction in rouleaux reveals a new mechanism?_\n\nAbstract text.\n\n_This paper is a work-in-progress._</pre><h2>Introduction</h2><pre>Observe changes.</pre>") {
		t.Fatalf("expected publish index to preserve pre-Introduction lead content before Introduction, got:\n%s", page)
	}
	if strings.Contains(page, "<h2>Study Preamble</h2>") {
		t.Fatalf("expected publish index to avoid inventing a Study Preamble heading, got:\n%s", page)
	}
}

func TestRenderPublishText_LabelsMethodsStepListAsProtocol(t *testing.T) {
	doc := store.ParseStudyDocumentMarkdown("# Introduction\n\nObserve changes.\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n")
	got := renderPublishText(
		doc,
		map[string]any{
			"status":     "WIP",
			"created_on": "09:00:00 01-01-2026",
		},
		store.Protocol{
			Summary: "Two capture steps.",
			Steps: []store.ProtocolStep{
				{Name: "Step One"},
				{Name: "Step Two"},
			},
		},
		nil,
	)

	for _, want := range []string{
		"\n\nMethods\nTwo capture steps.\n\nProtocol\n\n- Step One\n- Step Two\n\nResults\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected publish text to contain %q, got:\n%s", want, got)
		}
	}
}

func TestRenderPublishText_RendersStudyPreambleBeforeIntroduction(t *testing.T) {
	doc := store.ParseStudyDocumentMarkdown("# Comparison Study\n\n_A reduction in rouleaux reveals a new mechanism?_\n\nAbstract text.\n\n_This paper is a work-in-progress._\n\n# Introduction\n\nObserve changes.\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n")
	got := renderPublishText(
		doc,
		map[string]any{
			"status":     "WIP",
			"created_on": "09:00:00 01-01-2026",
		},
		store.Protocol{
			Summary: "Two capture steps.",
			Steps: []store.ProtocolStep{
				{Name: "Step One"},
				{Name: "Step Two"},
			},
		},
		nil,
	)

	if !strings.Contains(got, "\n\n_A reduction in rouleaux reveals a new mechanism?_\n\nAbstract text.\n\n_This paper is a work-in-progress._\n\nIntroduction\nObserve changes.") {
		t.Fatalf("expected publish text to preserve pre-Introduction lead content before Introduction, got:\n%s", got)
	}
	if strings.Contains(got, "\n\nStudy Preamble\n") {
		t.Fatalf("expected publish text to avoid inventing a Study Preamble heading, got:\n%s", got)
	}
}

func TestRunPublish_CustomDestinationWritesUnderStudyFolderName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubPublishThumbnailFn(t)

	root := filepath.Join(t.TempDir(), "sauna-study")
	dest := filepath.Join(t.TempDir(), "published")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\nPlaceholder methods.\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n", "One capture step.", "Step One"))
	mustWriteFile(t, filepath.Join(root, "subject-requirements.yaml"), "type: person\n")
	sessionPath := filepath.Join(root, "session", "01-01-2026-example", "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(sessionPath)); err != nil {
		t.Fatalf("EnsureDir session error: %v", err)
	}
	if err := util.WriteFrontmatterFile(sessionPath, map[string]any{}, "# Subjects\n\nAlpha Example\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile session error: %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"publish", "--once", dest}); code != 0 {
		t.Fatalf("Run(publish custom-dest) code=%d want=0", code)
	}

	publishRoot := filepath.Join(dest, filepath.Base(root))
	if _, err := os.Stat(filepath.Join(publishRoot, "site", "index.html")); err != nil {
		t.Fatalf("expected published index at custom destination, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(publishRoot, "study.pdf")); err != nil {
		t.Fatalf("expected published pdf at custom destination, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "publish", filepath.Base(root), "site", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("did not expect default publish output when custom destination is provided, got err=%v", err)
	}
}

func TestRunPublish_SessionListUsesAnonymizedLabelsByDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubPublishThumbnailFn(t)

	root := filepath.Join(t.TempDir(), "study")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\nPlaceholder methods should be replaced by protocol summary.\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n", "One capture step.", "Step One"))
	mustWriteFile(t, filepath.Join(root, "subject-requirements.yaml"), "type: person\n")

	for _, subject := range []store.Subject{
		{
			UUID: "11111111-1111-4111-8111-111111111111",
			Type: "person",
			Name: "Alpha Example",
			Path: filepath.Join(home, ".study-guide", "subject", "alpha-example.sg.md"),
		},
		{
			UUID: "22222222-2222-4222-8222-222222222222",
			Type: "person",
			Name: "Bravo Sample",
			Path: filepath.Join(home, ".study-guide", "subject", "bravo-sample.sg.md"),
		},
	} {
		if _, err := store.SaveSubject(subject); err != nil {
			t.Fatalf("SaveSubject error: %v", err)
		}
	}

	sessionSlug := "01-01-2026-example-sample"
	sessionPath := filepath.Join(root, "session", sessionSlug, "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(sessionPath)); err != nil {
		t.Fatalf("EnsureDir session error: %v", err)
	}
	if err := util.WriteFrontmatterFile(sessionPath, map[string]any{}, "# Subjects\n\nAlpha Example (11111111-1111-4111-8111-111111111111)\nBravo Sample (22222222-2222-4222-8222-222222222222)\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile session error: %v", err)
	}
	mustWriteStepFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "step.sg.md"), map[string]any{
		"time_started":  "10:00:00 01-01-2026",
		"time_finished": "10:10:00 01-01-2026",
	}, "")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "asset", "one.jpg"), "one")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"publish", "--once"}); code != 0 {
		t.Fatalf("Run(publish) code=%d want=0", code)
	}

	publishRoot := filepath.Join(root, "publish", filepath.Base(root))
	indexPath := filepath.Join(publishRoot, "site", "index.html")
	indexHTML, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", indexPath, err)
	}
	if strings.Contains(string(indexHTML), `session/01-01-2026-example-sample/index.html`) {
		t.Fatalf("expected anonymous publish list to avoid subject-derived session slug, got:\n%s", string(indexHTML))
	}
	if !strings.Contains(string(indexHTML), `<h3><a href="session/session-1/index.html">Subject 1, Subject 2</a></h3>`) {
		t.Fatalf("expected publish index multi-subject title to use anonymized labels by default, got:\n%s", string(indexHTML))
	}

	subjectMapPath := filepath.Join(root, "subject-map.txt")
	subjectMap, err := os.ReadFile(subjectMapPath)
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", subjectMapPath, err)
	}
	for _, want := range []string{
		"Subject 1: Alpha Example",
		"Subject 2: Bravo Sample",
	} {
		if !strings.Contains(string(subjectMap), want) {
			t.Fatalf("expected publish subject map to contain %q, got:\n%s", want, string(subjectMap))
		}
	}
}

func TestRunPublish_WithSubjectNamesFlagPreservesSubjectNames(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubPublishThumbnailFn(t)

	root := filepath.Join(t.TempDir(), "study")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\nPlaceholder methods should be replaced by protocol summary.\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n", "One capture step.", "Step One"))
	mustWriteFile(t, filepath.Join(root, "subject-requirements.yaml"), "type: person\n")

	for _, subject := range []store.Subject{
		{
			UUID: "11111111-1111-4111-8111-111111111111",
			Type: "person",
			Name: "Alpha Example",
			Path: filepath.Join(home, ".study-guide", "subject", "alpha-example.sg.md"),
		},
		{
			UUID: "22222222-2222-4222-8222-222222222222",
			Type: "person",
			Name: "Bravo Sample",
			Path: filepath.Join(home, ".study-guide", "subject", "bravo-sample.sg.md"),
		},
	} {
		if _, err := store.SaveSubject(subject); err != nil {
			t.Fatalf("SaveSubject error: %v", err)
		}
	}

	sessionSlug := "01-01-2026-example-sample"
	sessionPath := filepath.Join(root, "session", sessionSlug, "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(sessionPath)); err != nil {
		t.Fatalf("EnsureDir session error: %v", err)
	}
	if err := util.WriteFrontmatterFile(sessionPath, map[string]any{}, "# Subjects\n\nAlpha Example (11111111-1111-4111-8111-111111111111)\nBravo Sample (22222222-2222-4222-8222-222222222222)\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile session error: %v", err)
	}
	mustWriteStepFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "step.sg.md"), map[string]any{
		"time_started":  "10:00:00 01-01-2026",
		"time_finished": "10:10:00 01-01-2026",
	}, "")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "asset", "one.jpg"), "one")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"publish", "--once", "--with-subject-names"}); code != 0 {
		t.Fatalf("Run(publish --with-subject-names) code=%d want=0", code)
	}

	publishRoot := filepath.Join(root, "publish", filepath.Base(root))
	indexPath := filepath.Join(publishRoot, "site", "index.html")
	indexHTML, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", indexPath, err)
	}
	if !strings.Contains(string(indexHTML), `<h3><a href="session/01-01-2026-example-sample/index.html">Example, Sample</a></h3>`) {
		t.Fatalf("expected publish index multi-subject title to use last names with subject names enabled, got:\n%s", string(indexHTML))
	}

	sessionPagePath := filepath.Join(publishRoot, "site", "session", sessionSlug, "index.html")
	sessionHTML, err := os.ReadFile(sessionPagePath)
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", sessionPagePath, err)
	}
	if !strings.Contains(string(sessionHTML), `<span class="header-subject">Example, Sample</span>`) {
		t.Fatalf("expected session page header to preserve subject-based display name with flag, got:\n%s", string(sessionHTML))
	}

}

func TestRunPublish_RendersHEICAssetsAsJPEGPreviews(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubPublishThumbnailFn(t)

	origPreview := publishImagePreviewFn
	previewCalls := 0
	publishImagePreviewFn = func(src, dst string) error {
		previewCalls++
		if got, want := filepath.Ext(src), ".heic"; got != want {
			t.Fatalf("preview source ext=%q want %q", got, want)
		}
		if got, want := filepath.Ext(dst), ".jpg"; got != want {
			t.Fatalf("preview dest ext=%q want %q", got, want)
		}
		return os.WriteFile(dst, []byte("jpg-preview"), 0o644)
	}
	defer func() { publishImagePreviewFn = origPreview }()

	root := filepath.Join(t.TempDir(), "study")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\nPlaceholder methods should be replaced by protocol summary.\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n", "One capture step.", "Step One"))
	mustWriteFile(t, filepath.Join(root, "subject-requirements.yaml"), "type: person\n")

	subject := store.Subject{
		UUID: "11111111-1111-4111-8111-111111111111",
		Type: "person",
		Name: "Alpha Example",
		Path: filepath.Join(home, ".study-guide", "subject", "alpha-example.sg.md"),
	}
	if _, err := store.SaveSubject(subject); err != nil {
		t.Fatalf("SaveSubject error: %v", err)
	}

	sessionSlug := "01-01-2026-example"
	sessionPath := filepath.Join(root, "session", sessionSlug, "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(sessionPath)); err != nil {
		t.Fatalf("EnsureDir session error: %v", err)
	}
	if err := util.WriteFrontmatterFile(sessionPath, map[string]any{}, "# Subjects\n\nAlpha Example (11111111-1111-4111-8111-111111111111)\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile session error: %v", err)
	}
	mustWriteStepFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "step.sg.md"), map[string]any{
		"time_started":  "10:00:00 01-01-2026",
		"time_finished": "10:10:00 01-01-2026",
	}, "")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "asset", "one.heic"), "heic")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"publish", "--once"}); code != 0 {
		t.Fatalf("Run(publish) code=%d want=0", code)
	}
	if previewCalls != 1 {
		t.Fatalf("expected one HEIC preview render on first publish, got %d", previewCalls)
	}

	if code := Run([]string{"publish", "--once"}); code != 0 {
		t.Fatalf("Run(publish second pass) code=%d want=0", code)
	}
	if previewCalls != 1 {
		t.Fatalf("expected cached HEIC preview to be reused on second publish, got %d renders", previewCalls)
	}

	publishRoot := filepath.Join(root, "publish", filepath.Base(root))
	sessionPagePath := filepath.Join(publishRoot, "site", "session", "session-1", "index.html")
	sessionHTML, err := os.ReadFile(sessionPagePath)
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", sessionPagePath, err)
	}
	page := string(sessionHTML)
	if !strings.Contains(page, "assets/01-step-one/one.jpg") {
		t.Fatalf("expected HEIC preview jpg reference, got:\n%s", page)
	}
	if strings.Contains(page, "assets/01-step-one/one.heic") {
		t.Fatalf("expected raw HEIC reference to be replaced, got:\n%s", page)
	}
	if _, err := os.Stat(filepath.Join(publishRoot, "site", "session", "session-1", "assets", "01-step-one", "one.jpg")); err != nil {
		t.Fatalf("expected JPEG preview output, stat error: %v", err)
	}
}

func TestRunPublish_StartsMultipleHEICPreviewRendersBeforeTheFirstFinishes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubPublishThumbnailFn(t)

	origPreview := publishImagePreviewFn
	started := make(chan string, 2)
	release := make(chan struct{})
	publishImagePreviewFn = func(src, dst string) error {
		if err := util.EnsureDir(filepath.Dir(dst)); err != nil {
			return err
		}
		started <- filepath.Base(src)
		<-release
		return os.WriteFile(dst, []byte("jpg-preview"), 0o644)
	}
	t.Cleanup(func() {
		publishImagePreviewFn = origPreview
	})

	root := filepath.Join(t.TempDir(), "study")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\nPlaceholder methods should be replaced by protocol summary.\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n", "Two capture steps.", "Step One", "Step Two"))
	mustWriteFile(t, filepath.Join(root, "subject-requirements.yaml"), "type: person\n")

	subject := store.Subject{
		UUID: "11111111-1111-4111-8111-111111111111",
		Type: "person",
		Name: "Alpha Example",
		Path: filepath.Join(home, ".study-guide", "subject", "alpha-example.sg.md"),
	}
	if _, err := store.SaveSubject(subject); err != nil {
		t.Fatalf("SaveSubject error: %v", err)
	}

	sessionSlug := "01-01-2026-example"
	sessionPath := filepath.Join(root, "session", sessionSlug, "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(sessionPath)); err != nil {
		t.Fatalf("EnsureDir session error: %v", err)
	}
	if err := util.WriteFrontmatterFile(sessionPath, map[string]any{}, "# Subjects\n\nAlpha Example (11111111-1111-4111-8111-111111111111)\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile session error: %v", err)
	}
	mustWriteStepFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "step.sg.md"), map[string]any{
		"time_started":  "10:00:00 01-01-2026",
		"time_finished": "10:10:00 01-01-2026",
	}, "")
	mustWriteStepFile(t, filepath.Join(root, "session", sessionSlug, "step", "02-step-two", "step.sg.md"), map[string]any{
		"time_started":  "10:20:00 01-01-2026",
		"time_finished": "10:30:00 01-01-2026",
	}, "")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "01-step-one", "asset", "one.heic"), "heic-1")
	mustWriteFile(t, filepath.Join(root, "session", sessionSlug, "step", "02-step-two", "asset", "two.heic"), "heic-2")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	runDone := make(chan int, 1)
	go func() {
		runDone <- Run([]string{"publish", "--once"})
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("timed out waiting for first HEIC preview render to start")
	}

	startedSecond := false
	select {
	case <-started:
		startedSecond = true
	case <-time.After(250 * time.Millisecond):
	}
	close(release)

	select {
	case code := <-runDone:
		if code != 0 {
			t.Fatalf("Run(publish) code=%d want=0", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for publish to finish")
	}

	if !startedSecond {
		t.Fatalf("expected publish to start a second HEIC preview render before the first finished")
	}
}

func TestRunPublish_FailsWhenProtocolCannotBeParsed(t *testing.T) {
	root := filepath.Join(t.TempDir(), "study")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n# Introduction\n\nObserve changes.\n\n# Methods\n\nPlaceholder methods should be replaced by protocol summary.\n\n# Results\n\nObserved rouleaux.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n")
	mustWriteFile(t, filepath.Join(root, "subject-requirements.yaml"), "type: person\n")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"publish", "--once"}); code == 0 {
		t.Fatalf("Run(publish) code=%d want non-zero when protocol cannot be parsed", code)
	}
	if _, err := os.Stat(filepath.Join(root, "publish", filepath.Base(root), "site", "index.html")); err == nil {
		t.Fatalf("expected publish output to be skipped when protocol cannot be parsed")
	}
}
