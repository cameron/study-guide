package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"study-guide/src/internal/util"
)

func TestCollectStatusIssuesReportsMissingFieldsSectionsAndSteps(t *testing.T) {
	root := t.TempDir()
	if err := util.EnsureDir(filepath.Join(root, "session", "01-01-2026-boehmer", "step", "first-step")); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(filepath.Join(root, "study.sg.md"), map[string]any{
		"status": "WIP",
	}, "# Example Study\n\n# Hypotheses\n"); err != nil {
		t.Fatalf("write study failed: %v", err)
	}
	protocol := "# Protocol Summary\n\nSummary\n\n# Steps\n\n## First Step\n\n## Second Step\n"
	if err := os.WriteFile(filepath.Join(root, "protocol.sg.md"), []byte(protocol), 0o644); err != nil {
		t.Fatalf("write protocol failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(filepath.Join(root, "session", "01-01-2026-boehmer", "session.sg.md"), map[string]any{
		"time_started": "10:00:00 01-01-2026",
		"subject_ids":  []string{},
	}, ""); err != nil {
		t.Fatalf("write session failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(filepath.Join(root, "session", "01-01-2026-boehmer", "step", "first-step", "step.sg.md"), map[string]any{}, ""); err != nil {
		t.Fatalf("write step failed: %v", err)
	}

	issues, err := collectStatusIssues(root)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}

	mustContain := []string{
		"study.sg.md missing required field: created_on",
		"study.sg.md missing section: # Discussion",
		"study.sg.md missing section: # Conclusion",
		"session missing required field subject_ids: 01-01-2026-boehmer",
		"step missing time_started: ",
		"step missing time_finished: ",
		"session missing step file for protocol step second-step: 01-01-2026-boehmer",
	}
	for _, want := range mustContain {
		found := false
		for _, issue := range issues {
			if strings.Contains(issue, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing expected issue containing %q\nissues=%v", want, issues)
		}
	}
}
