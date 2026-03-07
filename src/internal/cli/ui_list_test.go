package cli

import "testing"

func TestHasNonSpaceText(t *testing.T) {
	if hasNonSpaceText("") {
		t.Fatalf("empty text must be non-match")
	}
	if hasNonSpaceText("   \t\n") {
		t.Fatalf("whitespace-only text must be non-match")
	}
	if !hasNonSpaceText("p") {
		t.Fatalf("single rune text must match")
	}
	if !hasNonSpaceText("  p  ") {
		t.Fatalf("mixed whitespace and printable text must match")
	}
}
