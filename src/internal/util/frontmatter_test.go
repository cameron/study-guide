package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFrontmatterFileOrdersPriorityKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "item.sg.md")
	fm := map[string]any{
		"time_finished": "10:00:00 01-01-2026",
		"subject_ids":   []string{"a", "b"},
		"time_started":  "09:00:00 01-01-2026",
		"status":        "WIP",
	}
	if err := WriteFrontmatterFile(path, fm, "# Body\n"); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	s := string(raw)
	statusPos := strings.Index(s, "status:")
	startedPos := strings.Index(s, "time_started:")
	finishedPos := strings.Index(s, "time_finished:")
	subjectPos := strings.Index(s, "subject_ids:")
	if !(statusPos >= 0 && startedPos > statusPos && finishedPos > startedPos && subjectPos > finishedPos) {
		t.Fatalf("unexpected key ordering:\n%s", s)
	}
}

func TestReadWriteFrontmatterRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "item.sg.md")
	wantBody := "# Notes\n\ncontent\n"
	if err := WriteFrontmatterFile(path, map[string]any{"uuid": "u1", "name": "N"}, wantBody); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	fm, body, err := ReadFrontmatterFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if fm["uuid"] != "u1" || fm["name"] != "N" {
		t.Fatalf("unexpected frontmatter: %#v", fm)
	}
	if strings.TrimPrefix(body, "\n") != wantBody {
		t.Fatalf("unexpected body: %q", body)
	}
}
