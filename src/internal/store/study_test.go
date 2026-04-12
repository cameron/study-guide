package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"study-guide/src/internal/util"
)

func TestParseProtocolMarkdown(t *testing.T) {
	md := `# Comparison Study

# Introduction

Intro text.

# Methods

Summary text.

## Protocol

### First Exposure

### Second Exposure

# Results

Observed.`
	p, err := ParseProtocolMarkdown(md)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if p.Summary != "Summary text." {
		t.Fatalf("unexpected summary: %q", p.Summary)
	}
	if len(p.Steps) != 2 {
		t.Fatalf("unexpected steps count: %d", len(p.Steps))
	}
	if p.Steps[0].Name != "First Exposure" || p.Steps[0].Slug != "01-first-exposure" {
		t.Fatalf("unexpected first step: %#v", p.Steps[0])
	}
	if p.Steps[1].Name != "Second Exposure" || p.Steps[1].Slug != "02-second-exposure" {
		t.Fatalf("unexpected second step: %#v", p.Steps[1])
	}
}

func TestParseStudyDocumentMarkdown_PreservesLeadAndSectionsInOrder(t *testing.T) {
	md := `# Comparison Study

_A reduction in rouleaux reveals a new mechanism?_

Abstract text.

# Introduction

Intro text.

# Methods

Summary text.

## Protocol

### First Exposure

# Results

Observed.`

	doc := ParseStudyDocumentMarkdown(md)
	if doc.Title != "Comparison Study" {
		t.Fatalf("unexpected title: %q", doc.Title)
	}
	if doc.Lead != "_A reduction in rouleaux reveals a new mechanism?_\n\nAbstract text." {
		t.Fatalf("unexpected lead: %q", doc.Lead)
	}
	if len(doc.Sections) != 3 {
		t.Fatalf("unexpected section count: %d", len(doc.Sections))
	}
	if doc.Sections[0].Name != "Introduction" || doc.Sections[0].Content != "Intro text." {
		t.Fatalf("unexpected first section: %#v", doc.Sections[0])
	}
	if doc.Sections[1].Name != "Methods" || !strings.Contains(doc.Sections[1].Content, "## Protocol") {
		t.Fatalf("unexpected methods section: %#v", doc.Sections[1])
	}
	if doc.Sections[2].Name != "Results" || doc.Sections[2].Content != "Observed." {
		t.Fatalf("unexpected last section: %#v", doc.Sections[2])
	}
}

func TestParseProtocolMarkdown_RejectsDuplicateNormalizedStepNames(t *testing.T) {
	md := `# Study

# Methods

Summary text.

## Protocol

### WiFi

### Grounding

### WiFi

### Grounding
`
	_, err := ParseProtocolMarkdown(md)
	if err == nil {
		t.Fatalf("expected duplicate normalized step names to fail")
	}
	if !strings.Contains(err.Error(), "duplicate protocol step title") {
		t.Fatalf("expected duplicate step title error, got: %v", err)
	}
}

func TestParseProtocolMarkdown_ParsesOptionalStepDescriptions(t *testing.T) {
	md := `# Study

# Methods

Summary text.

## Protocol

### First Exposure

Set baseline illumination.

### Ground

Grounding notes line one.
Grounding notes line two.
`
	p, err := ParseProtocolMarkdown(md)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(p.Steps) != 2 {
		t.Fatalf("unexpected steps count: %d", len(p.Steps))
	}
	if got := p.Steps[0].Description; got != "Set baseline illumination." {
		t.Fatalf("unexpected first description: %q", got)
	}
	if got := p.Steps[1].Description; got != "Grounding notes line one.\nGrounding notes line two." {
		t.Fatalf("unexpected second description: %q", got)
	}
}

func TestParseProtocolMarkdownRequiresSections(t *testing.T) {
	_, err := ParseProtocolMarkdown("# Study\n\n# Methods\n\nSummary only.\n")
	if err == nil || !strings.Contains(err.Error(), "## Protocol") {
		t.Fatalf("expected missing protocol subsection error, got: %v", err)
	}

	_, err = ParseProtocolMarkdown("# Study\n\n# Results\n\nObserved.\n")
	if err == nil || !strings.Contains(err.Error(), "# Methods") {
		t.Fatalf("expected missing methods section error, got: %v", err)
	}
}

func TestExtractStudyTitle(t *testing.T) {
	body := "# Study Title\n\n# Introduction\n"
	if got := ExtractStudyTitle(body); got != "Study Title" {
		t.Fatalf("unexpected title: %q", got)
	}
}

func TestReadSubjectRequirements_LoadsRequiredAndFixedFields(t *testing.T) {
	root := t.TempDir()
	raw := "" +
		"type: person\n" +
		"favorite_color: green\n" +
		"required_fields:\n" +
		"  - name\n" +
		"  - favorite_color\n"
	if err := os.WriteFile(filepath.Join(root, "subject-requirements.yaml"), []byte(raw), 0o644); err != nil {
		t.Fatalf("write requirements failed: %v", err)
	}

	req, err := ReadSubjectRequirements(root)
	if err != nil {
		t.Fatalf("ReadSubjectRequirements returned error: %v", err)
	}
	if len(req.RequiredFields) != 2 || req.RequiredFields[0] != "name" || req.RequiredFields[1] != "favorite_color" {
		t.Fatalf("unexpected required fields: %#v", req.RequiredFields)
	}
	if got := req.FixedFields["type"]; got != "person" {
		t.Fatalf("expected fixed type=person, got %q", got)
	}
	if got := req.FixedFields["favorite_color"]; got != "green" {
		t.Fatalf("expected fixed favorite_color=green, got %q", got)
	}
}

func TestReadSubjectRequirements_LoadsCompactFixedFields(t *testing.T) {
	root := t.TempDir()
	raw := "" +
		"type:person\n" +
		"favorite_color:green\n"
	if err := os.WriteFile(filepath.Join(root, "subject-requirements.yaml"), []byte(raw), 0o644); err != nil {
		t.Fatalf("write requirements failed: %v", err)
	}

	req, err := ReadSubjectRequirements(root)
	if err != nil {
		t.Fatalf("ReadSubjectRequirements returned error: %v", err)
	}
	if got := req.FixedFields["type"]; got != "person" {
		t.Fatalf("expected fixed type=person, got %q", got)
	}
	if got := req.FixedFields["favorite_color"]; got != "green" {
		t.Fatalf("expected fixed favorite_color=green, got %q", got)
	}
}

func TestParseProtocol_RenamesSessionStepDirectoriesWhenStepNamesChange(t *testing.T) {
	root := t.TempDir()
	protocol := `# Study

# Methods

Summary text.

## Protocol

### Renamed Step

### Second Step
`
	if err := os.WriteFile(filepath.Join(root, "study.sg.md"), []byte(protocol), 0o644); err != nil {
		t.Fatalf("write study failed: %v", err)
	}
	oldStepDir := filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-original-step")
	if err := os.MkdirAll(filepath.Join(oldStepDir, "asset"), 0o755); err != nil {
		t.Fatalf("mkdir old step dir failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(filepath.Join(oldStepDir, "step.sg.md"), map[string]any{
		"time_started": "10:00:00 01-01-2026",
	}, "notes"); err != nil {
		t.Fatalf("write old step file failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldStepDir, "asset", "sample.jpg"), []byte("asset"), 0o644); err != nil {
		t.Fatalf("write asset failed: %v", err)
	}

	parsed, err := ParseProtocol(root)
	if err != nil {
		t.Fatalf("ParseProtocol failed: %v", err)
	}
	if len(parsed.Steps) != 2 || parsed.Steps[0].Slug != "01-renamed-step" {
		t.Fatalf("unexpected parsed steps: %#v", parsed.Steps)
	}

	newStepDir := filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-renamed-step")
	if _, err := os.Stat(newStepDir); err != nil {
		t.Fatalf("expected renamed step dir to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newStepDir, "step.sg.md")); err != nil {
		t.Fatalf("expected step.sg.md to move with renamed dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newStepDir, "asset", "sample.jpg")); err != nil {
		t.Fatalf("expected asset to move with renamed dir: %v", err)
	}
	if _, err := os.Stat(oldStepDir); !os.IsNotExist(err) {
		t.Fatalf("expected old step dir to be removed, got err=%v", err)
	}
}

func TestParseProtocol_ReordersSessionStepDirectoriesByStepTitle(t *testing.T) {
	root := t.TempDir()
	protocol := `# Study

# Methods

Summary text.

## Protocol

### Second Step

### First Step
`
	if err := os.WriteFile(filepath.Join(root, "study.sg.md"), []byte(protocol), 0o644); err != nil {
		t.Fatalf("write study failed: %v", err)
	}

	firstDir := filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-first-step")
	secondDir := filepath.Join(root, "session", "01-01-2026-alpha", "step", "02-second-step")
	if err := os.MkdirAll(filepath.Join(firstDir, "asset"), 0o755); err != nil {
		t.Fatalf("mkdir first step dir failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(secondDir, "asset"), 0o755); err != nil {
		t.Fatalf("mkdir second step dir failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(filepath.Join(firstDir, "step.sg.md"), map[string]any{
		"time_started": "10:00:00 01-01-2026",
	}, "first notes"); err != nil {
		t.Fatalf("write first step file failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(filepath.Join(secondDir, "step.sg.md"), map[string]any{
		"time_started": "10:05:00 01-01-2026",
	}, "second notes"); err != nil {
		t.Fatalf("write second step file failed: %v", err)
	}

	parsed, err := ParseProtocol(root)
	if err != nil {
		t.Fatalf("ParseProtocol failed: %v", err)
	}
	if len(parsed.Steps) != 2 || parsed.Steps[0].Slug != "01-second-step" || parsed.Steps[1].Slug != "02-first-step" {
		t.Fatalf("unexpected parsed steps: %#v", parsed.Steps)
	}

	if _, err := os.Stat(filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-second-step", "step.sg.md")); err != nil {
		t.Fatalf("expected second step to move to first ordinal: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "session", "01-01-2026-alpha", "step", "02-first-step", "step.sg.md")); err != nil {
		t.Fatalf("expected first step to move to second ordinal: %v", err)
	}
	_, secondBody, err := util.ReadFrontmatterFile(filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-second-step", "step.sg.md"))
	if err != nil {
		t.Fatalf("read moved second step failed: %v", err)
	}
	if strings.TrimSpace(secondBody) != "second notes" {
		t.Fatalf("expected second step metadata to follow its title, got body %q", secondBody)
	}
	_, firstBody, err := util.ReadFrontmatterFile(filepath.Join(root, "session", "01-01-2026-alpha", "step", "02-first-step", "step.sg.md"))
	if err != nil {
		t.Fatalf("read moved first step failed: %v", err)
	}
	if strings.TrimSpace(firstBody) != "first notes" {
		t.Fatalf("expected first step metadata to follow its title, got body %q", firstBody)
	}
	if _, err := os.Stat(firstDir); !os.IsNotExist(err) {
		t.Fatalf("expected old first step dir to be removed, got err=%v", err)
	}
	if _, err := os.Stat(secondDir); !os.IsNotExist(err) {
		t.Fatalf("expected old second step dir to be removed, got err=%v", err)
	}
}

func TestParseProtocol_AddsNewStepWithoutReassigningExistingDirectories(t *testing.T) {
	root := t.TempDir()
	protocol := `# Study

# Methods

Summary text.

## Protocol

### First Step

### Inserted Step

### Second Step
`
	if err := os.WriteFile(filepath.Join(root, "study.sg.md"), []byte(protocol), 0o644); err != nil {
		t.Fatalf("write study failed: %v", err)
	}

	firstDir := filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-first-step")
	secondDir := filepath.Join(root, "session", "01-01-2026-alpha", "step", "02-second-step")
	if err := os.MkdirAll(filepath.Join(firstDir, "asset"), 0o755); err != nil {
		t.Fatalf("mkdir first step dir failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(secondDir, "asset"), 0o755); err != nil {
		t.Fatalf("mkdir second step dir failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(filepath.Join(firstDir, "step.sg.md"), map[string]any{
		"time_started": "10:00:00 01-01-2026",
	}, "first notes"); err != nil {
		t.Fatalf("write first step file failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(filepath.Join(secondDir, "step.sg.md"), map[string]any{
		"time_started": "10:05:00 01-01-2026",
	}, "second notes"); err != nil {
		t.Fatalf("write second step file failed: %v", err)
	}

	parsed, err := ParseProtocol(root)
	if err != nil {
		t.Fatalf("ParseProtocol failed: %v", err)
	}
	if len(parsed.Steps) != 3 || parsed.Steps[0].Slug != "01-first-step" || parsed.Steps[1].Slug != "02-inserted-step" || parsed.Steps[2].Slug != "03-second-step" {
		t.Fatalf("unexpected parsed steps: %#v", parsed.Steps)
	}

	if _, err := os.Stat(filepath.Join(root, "session", "01-01-2026-alpha", "step", "01-first-step", "step.sg.md")); err != nil {
		t.Fatalf("expected first step dir to remain in place: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "session", "01-01-2026-alpha", "step", "03-second-step", "step.sg.md")); err != nil {
		t.Fatalf("expected second step dir to move after inserted step: %v", err)
	}
	_, secondBody, err := util.ReadFrontmatterFile(filepath.Join(root, "session", "01-01-2026-alpha", "step", "03-second-step", "step.sg.md"))
	if err != nil {
		t.Fatalf("read moved second step failed: %v", err)
	}
	if strings.TrimSpace(secondBody) != "second notes" {
		t.Fatalf("expected second step metadata to follow its title, got body %q", secondBody)
	}
	if _, err := os.Stat(filepath.Join(root, "session", "01-01-2026-alpha", "step", "02-second-step")); !os.IsNotExist(err) {
		t.Fatalf("expected old second step dir to be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "session", "01-01-2026-alpha", "step", "02-inserted-step")); !os.IsNotExist(err) {
		t.Fatalf("did not expect a historical dir to be synthesized for new step, got err=%v", err)
	}
}
