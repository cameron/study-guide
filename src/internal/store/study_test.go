package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
