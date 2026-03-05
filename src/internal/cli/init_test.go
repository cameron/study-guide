package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseOutlineSteps_LineBasedWithOptionalDescription(t *testing.T) {
	raw := strings.Join([]string{
		"Baseline | initial baseline",
		"WiFi",
		"Grounding | hold for 3 minutes",
		"",
	}, "\n")
	got := parseOutlineSteps(raw)
	want := []protocolOutlineStep{
		{Name: "Baseline", Description: "initial baseline"},
		{Name: "WiFi"},
		{Name: "Grounding", Description: "hold for 3 minutes"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseOutlineSteps mismatch: got=%#v want=%#v", got, want)
	}
}

func TestEnsureProtocolFile_WritesOptionalStepDescriptions(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "protocol.sg.md")
	steps := []protocolOutlineStep{
		{Name: "Baseline", Description: "initial baseline"},
		{Name: "WiFi"},
		{Name: "Grounding", Description: "hold for 3 minutes"},
	}
	if err := ensureProtocolFile(path, steps); err != nil {
		t.Fatalf("ensureProtocolFile error: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	got := string(b)
	for _, token := range []string{
		"## Baseline\n\ninitial baseline",
		"## WiFi",
		"## Grounding\n\nhold for 3 minutes",
	} {
		if !strings.Contains(got, token) {
			t.Fatalf("expected token %q in protocol scaffold\ncontent:\n%s", token, got)
		}
	}
}
