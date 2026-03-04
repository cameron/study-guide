package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"study-guide/src/internal/util"
)

type Subject struct {
	UUID      string
	Type      string
	Name      string
	Email     string
	Phone     string
	Age       string
	Sex       string
	Notes     string
	CreatedOn string
	UpdatedOn string
	Path      string
}

func SubjectFromFM(path string, fm map[string]any, body string) Subject {
	s := Subject{Path: path}
	s.UUID = asString(fm["uuid"])
	s.Type = asString(fm["type"])
	s.Name = asString(fm["name"])
	s.Email = asString(fm["email"])
	s.Phone = asString(fm["phone"])
	s.Age = asString(fm["age"])
	s.Sex = asString(fm["sex"])
	s.CreatedOn = asString(fm["created_on"])
	s.UpdatedOn = asString(fm["updated_on"])
	s.Notes = extractNotes(body)
	return s
}

func (s Subject) Frontmatter() map[string]any {
	fm := map[string]any{
		"uuid": s.UUID,
		"type": s.Type,
		"name": s.Name,
	}
	if s.Email != "" {
		fm["email"] = s.Email
	}
	if s.Phone != "" {
		fm["phone"] = s.Phone
	}
	if s.Age != "" {
		fm["age"] = s.Age
	}
	if s.Sex != "" {
		fm["sex"] = s.Sex
	}
	if strings.TrimSpace(s.CreatedOn) == "" {
		fm["created_on"] = util.NowTimestamp()
	} else {
		fm["created_on"] = s.CreatedOn
	}
	fm["updated_on"] = util.NowTimestamp()
	return fm
}

func SaveSubject(s Subject) (string, error) {
	dir, err := util.HomeSubjectDir()
	if err != nil {
		return "", err
	}
	if err := util.EnsureDir(dir); err != nil {
		return "", err
	}
	base := util.Slugify(s.Name)
	if base == "item" {
		base = "subject"
	}
	if s.UUID == "" {
		u, err := util.NewUUIDv4()
		if err != nil {
			return "", err
		}
		s.UUID = u
	}
	if s.Type == "" {
		s.Type = "person"
	}
	path := s.Path
	if strings.TrimSpace(path) == "" {
		name := fmt.Sprintf("%s-%s.sg.md", base, strings.Split(s.UUID, "-")[0])
		path = filepath.Join(dir, name)
	}
	body := ""
	if strings.TrimSpace(s.Notes) != "" {
		body = "# Notes\n\n" + strings.TrimSpace(s.Notes) + "\n"
	}
	if err := util.WriteFrontmatterFile(path, s.Frontmatter(), body); err != nil {
		return "", err
	}
	return path, nil
}

func ListSubjects() ([]Subject, error) {
	dir, err := util.HomeSubjectDir()
	if err != nil {
		return nil, err
	}
	if err := util.EnsureDir(dir); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Subject
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sg.md") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		fm, body, err := util.ReadFrontmatterFile(p)
		if err != nil {
			continue
		}
		out = append(out, SubjectFromFM(p, fm, body))
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

func FindSubject(query string) ([]Subject, error) {
	subs, err := ListSubjects()
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var out []Subject
	for _, s := range subs {
		if strings.Contains(strings.ToLower(s.Name), q) || strings.HasPrefix(strings.ToLower(s.UUID), q) {
			out = append(out, s)
		}
	}
	return out, nil
}

func RemoveSubject(query string) error {
	s, err := ResolveSubject(query)
	if err != nil {
		return err
	}
	return os.Remove(s.Path)
}

func ResolveSubject(query string) (Subject, error) {
	matches, err := FindSubject(query)
	if err != nil {
		return Subject{}, err
	}
	if len(matches) == 0 {
		return Subject{}, fmt.Errorf("subject not found: %s", query)
	}
	if len(matches) > 1 {
		return Subject{}, fmt.Errorf("ambiguous subject id/name: %s", query)
	}
	return matches[0], nil
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func extractNotes(body string) string {
	b := strings.TrimSpace(body)
	if strings.HasPrefix(b, "# Notes") {
		b = strings.TrimSpace(strings.TrimPrefix(b, "# Notes"))
	}
	return b
}
