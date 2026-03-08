package cli

import (
	"regexp"
	"testing"
)

func TestRenderScreenTitle_UsesTurquoiseBackground(t *testing.T) {
	out := renderScreenTitle("Sessions")
	bg := regexp.MustCompile(`\x1b\[[0-9;]*48;2;(120;240;255|20;144;160)m`)
	if !bg.MatchString(out) {
		t.Fatalf("expected turquoise title background ANSI color, got %q", out)
	}
}
