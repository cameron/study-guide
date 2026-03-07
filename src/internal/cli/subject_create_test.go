package cli

import (
	"os"
	"path/filepath"
	"testing"

	"study-guide/src/internal/store"
)

func TestSubjectCreateFormFields_IncludeFixedAndCustomRequiredFields(t *testing.T) {
	req := store.SubjectRequirements{
		RequiredFields: []string{"name", "favorite_color"},
		FixedFields: map[string]string{
			"type":           "person",
			"favorite_color": "green",
		},
	}

	fields := subjectCreateFormFieldsFromRequirements(req)
	byName := map[string]formField{}
	for _, f := range fields {
		byName[f.Name] = f
	}

	if !byName["name"].Required {
		t.Fatalf("expected name to remain required")
	}
	color, ok := byName["favorite_color"]
	if !ok {
		t.Fatalf("expected custom field favorite_color to be prompted")
	}
	if !color.Required {
		t.Fatalf("expected favorite_color to be required")
	}
	if !color.ReadOnly {
		t.Fatalf("expected favorite_color to be fixed/read-only")
	}
	if color.Value != "green" {
		t.Fatalf("expected favorite_color default fixed value green, got %q", color.Value)
	}
	fixedType, ok := byName["type"]
	if !ok {
		t.Fatalf("expected fixed type to be included")
	}
	if !fixedType.ReadOnly || fixedType.Value != "person" {
		t.Fatalf("expected fixed type read-only value person, got %#v", fixedType)
	}
}

func TestSubjectCreateRequirements_UsesProvidedStudyRoot(t *testing.T) {
	studyRoot := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(studyRoot, "subject-requirements.yaml"),
		[]byte("type: person\n"),
		0o644,
	); err != nil {
		t.Fatalf("write requirements failed: %v", err)
	}

	cwd := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	req, err := subjectCreateRequirements(studyRoot)
	if err != nil {
		t.Fatalf("subjectCreateRequirements returned error: %v", err)
	}
	if got := req.FixedFields["type"]; got != "person" {
		t.Fatalf("expected fixed type=person, got %q", got)
	}
}
