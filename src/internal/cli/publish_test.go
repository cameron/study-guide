package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"study-guide/src/internal/store"
	"study-guide/src/internal/util"
)

func TestRunPublish_GeneratesSessionComparisonPage(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root := filepath.Join(t.TempDir(), "study")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n# Hypotheses\n\nObserve changes.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n")
	mustWriteFile(t, filepath.Join(root, "protocol.sg.md"), "# Protocol Summary\n\nTwo capture steps.\n\n# Steps\n\n## Step One\n\n## Step Two\n\n")
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

	if code := Run([]string{"publish"}); code != 0 {
		t.Fatalf("Run(publish) code=%d want=0", code)
	}

	indexPath := filepath.Join(root, "publish", "site", "index.html")
	indexHTML, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", indexPath, err)
	}
	if !strings.Contains(string(indexHTML), `session/01-01-2026-example/index.html`) {
		t.Fatalf("expected publish index to link to session page, got:\n%s", string(indexHTML))
	}

	sessionPagePath := filepath.Join(root, "publish", "site", "session", sessionSlug, "index.html")
	sessionHTML, err := os.ReadFile(sessionPagePath)
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", sessionPagePath, err)
	}
	page := string(sessionHTML)
	for _, want := range []string{
		"Alpha Example",
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
	if !strings.Contains(page, `class="header-link" href="../../index.html">Up</a><span class="session-nav"><span class="header-sep">|</span><a class="session-link next-session-link" href="../02-01-2026-example/index.html">Next</a></span><span class="header-sep">|</span><span class="header-date">01-01-2026</span><span class="header-sep">|</span><span class="header-subject">Alpha Example</span>`) {
		t.Fatalf("expected compact one-line header order, got:\n%s", page)
	}
	if strings.Contains(page, `<button type="button">Up</button>`) {
		t.Fatalf("expected back control to remain a link, got:\n%s", page)
	}
	if !strings.Contains(page, `href="../02-01-2026-example/index.html"`) {
		t.Fatalf("expected next session link, got:\n%s", page)
	}
	nextSessionPagePath := filepath.Join(root, "publish", "site", "session", nextSessionSlug, "index.html")
	nextSessionHTML, err := os.ReadFile(nextSessionPagePath)
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", nextSessionPagePath, err)
	}
	if !strings.Contains(string(nextSessionHTML), `href="../01-01-2026-example/index.html"`) {
		t.Fatalf("expected previous session link on second session page, got:\n%s", string(nextSessionHTML))
	}
	firstThumb := strings.Index(string(indexHTML), "session/01-01-2026-example/assets/01-step-one/one.jpg")
	secondThumb := strings.Index(string(indexHTML), "session/01-01-2026-example/assets/02-step-two/two.jpg")
	if firstThumb < 0 || secondThumb < 0 {
		t.Fatalf("expected publish index to include chrono-ordered thumbnails, got:\n%s", string(indexHTML))
	}
	if firstThumb > secondThumb {
		t.Fatalf("expected publish index thumbnails in chronological order, got:\n%s", string(indexHTML))
	}
}

func TestRunPublish_RendersHEICAssetsAsJPEGPreviews(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

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
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n# Hypotheses\n\nObserve changes.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n")
	mustWriteFile(t, filepath.Join(root, "protocol.sg.md"), "# Protocol Summary\n\nOne capture step.\n\n# Steps\n\n## Step One\n\n")
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

	if code := Run([]string{"publish"}); code != 0 {
		t.Fatalf("Run(publish) code=%d want=0", code)
	}
	if previewCalls != 1 {
		t.Fatalf("expected one HEIC preview render on first publish, got %d", previewCalls)
	}

	if code := Run([]string{"publish"}); code != 0 {
		t.Fatalf("Run(publish second pass) code=%d want=0", code)
	}
	if previewCalls != 1 {
		t.Fatalf("expected cached HEIC preview to be reused on second publish, got %d renders", previewCalls)
	}

	sessionPagePath := filepath.Join(root, "publish", "site", "session", sessionSlug, "index.html")
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
	if _, err := os.Stat(filepath.Join(root, "publish", "site", "session", sessionSlug, "assets", "01-step-one", "one.jpg")); err != nil {
		t.Fatalf("expected JPEG preview output, stat error: %v", err)
	}
}

func TestRunPublish_FailsWhenProtocolCannotBeParsed(t *testing.T) {
	root := filepath.Join(t.TempDir(), "study")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Comparison Study\n\n# Hypotheses\n\nObserve changes.\n\n# Discussion\n\nNotes.\n\n# Conclusion\n\nDone.\n")
	mustWriteFile(t, filepath.Join(root, "protocol.sg.md"), "# Steps\n\n## Step One\n\n")
	mustWriteFile(t, filepath.Join(root, "subject-requirements.yaml"), "type: person\n")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if code := Run([]string{"publish"}); code == 0 {
		t.Fatalf("Run(publish) code=%d want non-zero when protocol cannot be parsed", code)
	}
	if _, err := os.Stat(filepath.Join(root, "publish", "site", "index.html")); err == nil {
		t.Fatalf("expected publish output to be skipped when protocol cannot be parsed")
	}
}
