package cli

import (
	"regexp"
	"strings"
	"testing"
)

func TestFormModel_InputWidthFitsPlaceholder(t *testing.T) {
	m := newFormModel("Create Subject", []formField{
		{Name: "favorite_color", Label: "Favorite Color"},
	})

	if len(m.inputs) != 1 {
		t.Fatalf("expected one input, got %d", len(m.inputs))
	}

	if got, want := m.inputs[0].Width(), len(m.inputs[0].Placeholder); got < want {
		t.Fatalf("expected input width >= placeholder length (%d), got %d", want, got)
	}
}

func TestProtocolTitlesModel_InputWidthFitsPlaceholder(t *testing.T) {
	m := newProtocolTitlesModel()
	if got, want := m.input.Width(), len(m.input.Placeholder); got < want {
		t.Fatalf("expected input width >= placeholder length (%d), got %d", want, got)
	}
}

func TestFormModel_View_TitleHasBackgroundStyle(t *testing.T) {
	m := newFormModel("Create Subject", []formField{
		{Name: "name", Label: "Name"},
	})
	out := m.View().Content
	firstLine := strings.SplitN(out, "\n", 2)[0]
	re := regexp.MustCompile(`\x1b\[[0-9;]*48;`)
	if !re.MatchString(firstLine) {
		t.Fatalf("expected title line to include background ANSI style, got %q", firstLine)
	}
}
