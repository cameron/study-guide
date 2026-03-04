package store

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestSubjectStoreCreateEditResolveRemove(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SG_SUBJECT_DIR", dir)

	path1, err := SaveSubject(Subject{Name: "Alice Boehmer", Type: "person"})
	if err != nil {
		t.Fatalf("save subject 1 failed: %v", err)
	}
	s1, err := ResolveSubject("Alice")
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	uuidV4 := regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`)
	if !uuidV4.MatchString(s1.UUID) {
		t.Fatalf("uuid is not v4: %s", s1.UUID)
	}

	edited := s1
	edited.Email = "alice@example.com"
	edited.Notes = "Updated notes"
	pathEdited, err := SaveSubject(edited)
	if err != nil {
		t.Fatalf("edit save failed: %v", err)
	}
	if pathEdited != path1 {
		t.Fatalf("expected same path after edit: %s != %s", pathEdited, path1)
	}
	s1After, err := ResolveSubject(s1.UUID)
	if err != nil {
		t.Fatalf("resolve by uuid failed: %v", err)
	}
	if s1After.UUID != s1.UUID {
		t.Fatalf("uuid changed after edit: %s != %s", s1After.UUID, s1.UUID)
	}
	if s1After.Email != "alice@example.com" {
		t.Fatalf("edit not persisted: %#v", s1After)
	}

	_, err = SaveSubject(Subject{Name: "Alice Smith", Type: "person"})
	if err != nil {
		t.Fatalf("save subject 2 failed: %v", err)
	}
	if _, err := ResolveSubject("Alice"); err == nil {
		t.Fatalf("expected ambiguous resolve error")
	}

	if err := RemoveSubject(s1.UUID); err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	if _, statErr := os.Stat(path1); !os.IsNotExist(statErr) {
		t.Fatalf("subject file still exists after remove: %v", statErr)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one remaining subject file, got %d", len(entries))
	}
	if filepath.Ext(entries[0].Name()) != ".md" {
		t.Fatalf("unexpected remaining file: %s", entries[0].Name())
	}
}
