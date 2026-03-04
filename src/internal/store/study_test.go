package store

import (
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
	if p.Steps[0].Name != "First Exposure" || p.Steps[0].Slug != "first-exposure" {
		t.Fatalf("unexpected first step: %#v", p.Steps[0])
	}
	if p.Steps[1].Name != "Second Exposure" || p.Steps[1].Slug != "second-exposure" {
		t.Fatalf("unexpected second step: %#v", p.Steps[1])
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
