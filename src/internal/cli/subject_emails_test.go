package cli

import (
	"strings"
	"testing"
)

func TestRunHelp_DoesNotListSubjectEmails(t *testing.T) {
	out := captureStdout(t, func() {
		if code := Run([]string{"help"}); code != 0 {
			t.Fatalf("Run(help) code=%d want=0", code)
		}
	})
	if strings.Contains(out, "subject create|edit|search|print|ls|emails|rm") {
		t.Fatalf("expected help to omit subject emails, got:\n%s", out)
	}
	if !strings.Contains(out, "subject create|edit|search|print|ls|rm") {
		t.Fatalf("expected subject help line without emails, got:\n%s", out)
	}
}

func TestRunSubjectEmails_IsUnknownSubcommand(t *testing.T) {
	stderr := captureStderr(t, func() {
		if code := Run([]string{"subject", "emails"}); code != 1 {
			t.Fatalf("Run(subject emails) code=%d want=1", code)
		}
	})
	if !strings.Contains(stderr, "unknown subject subcommand: emails") {
		t.Fatalf("expected unknown subject subcommand error, got:\n%s", stderr)
	}
}
