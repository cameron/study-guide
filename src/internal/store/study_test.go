package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"study-guide/src/internal/util"
)

func TestParseProtocolMarkdown(t *testing.T) {
	md := `# Protocol Summary

Summary text.

# Steps

## First Exposure

## Second Exposure

# Actions

Optional`
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

func TestParseProtocolMarkdown_DuplicateStepNamesGetUniqueSlugs(t *testing.T) {
	md := `# Protocol Summary

Summary text.

# Steps

## WiFi

## Grounding

## WiFi

## Grounding
`
	p, err := ParseProtocolMarkdown(md)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(p.Steps) != 4 {
		t.Fatalf("unexpected steps count: %d", len(p.Steps))
	}
	got := []string{p.Steps[0].Slug, p.Steps[1].Slug, p.Steps[2].Slug, p.Steps[3].Slug}
	want := []string{"01-wifi", "02-grounding", "03-wifi", "04-grounding"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected duplicate slug disambiguation: got=%v want=%v", got, want)
	}
}

func TestParseProtocolMarkdown_ParsesOptionalStepDescriptions(t *testing.T) {
	md := `# Protocol Summary

Summary text.

# Steps

## First Exposure

Set baseline illumination.

## Ground

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
	_, err := ParseProtocolMarkdown("# Steps\n\n## One\n")
	if err == nil || !strings.Contains(err.Error(), "# Protocol Summary") {
		t.Fatalf("expected missing summary error, got: %v", err)
	}

	_, err = ParseProtocolMarkdown("# Protocol Summary\n\nText\n")
	if err == nil || !strings.Contains(err.Error(), "# Steps") {
		t.Fatalf("expected missing steps error, got: %v", err)
	}
}

func TestExtractStudyTitle(t *testing.T) {
	body := "# Study Title\n\n# Hypotheses\n"
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
	protocol := `# Protocol Summary

Summary text.

# Steps

## Renamed Step

## Second Step
`
	if err := os.WriteFile(filepath.Join(root, "protocol.sg.md"), []byte(protocol), 0o644); err != nil {
		t.Fatalf("write protocol failed: %v", err)
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
