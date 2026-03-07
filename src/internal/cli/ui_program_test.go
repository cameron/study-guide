package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInteractiveProgram_IsCentralizedToSingleNewProgramCall(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob go files failed: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("expected at least one go file")
	}

	total := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s failed: %v", name, err)
		}
		total += strings.Count(string(raw), "tea.NewProgram(")
	}
	if total != 1 {
		t.Fatalf("expected exactly one tea.NewProgram call in cli package, got %d", total)
	}
}

func TestInteractiveProgram_UsesAltScreen(t *testing.T) {
	raw, err := os.ReadFile("ui_program.go")
	if err != nil {
		t.Fatalf("read ui_program.go failed: %v", err)
	}
	src := string(raw)
	if !strings.Contains(src, "tea.NewProgram(") {
		t.Fatalf("expected ui_program.go to construct Bubble Tea program")
	}
	if !strings.Contains(src, "v.AltScreen = true") {
		t.Fatalf("expected ui_program.go to force AltScreen on rendered views")
	}
}
