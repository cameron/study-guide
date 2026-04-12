package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"study-guide/src/internal/util"
)

type Protocol struct {
	Summary string
	Steps   []ProtocolStep
}

type StudySection struct {
	Name    string
	Content string
}

type StudyDocument struct {
	Title    string
	Lead     string
	Sections []StudySection
}

func (d StudyDocument) SectionContent(name string) string {
	for _, sec := range d.Sections {
		if sec.Name == name {
			return sec.Content
		}
	}
	return ""
}

type ProtocolStep struct {
	Name        string
	Slug        string
	Description string
}

type SubjectRequirements struct {
	RequiredFields []string
	FixedFields    map[string]string
}

func ReadSubjectRequirements(studyRoot string) (SubjectRequirements, error) {
	p := filepath.Join(studyRoot, "subject-requirements.yaml")
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return SubjectRequirements{FixedFields: map[string]string{}}, nil
		}
		return SubjectRequirements{}, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return SubjectRequirements{}, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		compactDoc, compactErr := parseCompactFixedFields(raw)
		if compactErr != nil {
			return SubjectRequirements{}, err
		}
		doc = compactDoc
	}

	req := SubjectRequirements{
		RequiredFields: []string{},
		FixedFields:    map[string]string{},
	}
	if arr, ok := doc["required_fields"].([]any); ok {
		for _, v := range arr {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" {
				req.RequiredFields = append(req.RequiredFields, s)
			}
		}
	}
	for k, v := range doc {
		key := strings.TrimSpace(k)
		if key == "" || key == "required_fields" {
			continue
		}
		switch v.(type) {
		case string, int, int64, float64, bool:
			req.FixedFields[key] = strings.TrimSpace(fmt.Sprint(v))
		}
	}
	return req, nil
}

func parseCompactFixedFields(raw []byte) (map[string]any, error) {
	doc := map[string]any{}
	lines := strings.Split(string(raw), "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("invalid compact fixed-field line: %q", line)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return nil, fmt.Errorf("invalid compact fixed-field key: %q", line)
		}
		doc[key] = value
	}
	return doc, nil
}

func ReadRequiredSubjectFields(studyRoot string) ([]string, error) {
	req, err := ReadSubjectRequirements(studyRoot)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, s := range req.RequiredFields {
		if strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out, nil
}

func ParseProtocolSteps(studyRoot string) ([]string, error) {
	proto, err := ParseProtocol(studyRoot)
	if err != nil {
		return nil, err
	}
	steps := make([]string, 0, len(proto.Steps))
	for _, s := range proto.Steps {
		steps = append(steps, s.Name)
	}
	return steps, nil
}

func ParseProtocol(studyRoot string) (Protocol, error) {
	p := filepath.Join(studyRoot, "study.sg.md")
	b, err := os.ReadFile(p)
	if err != nil {
		return Protocol{}, err
	}
	protocol, err := ParseProtocolMarkdown(string(b))
	if err != nil {
		return Protocol{}, err
	}
	if err := reconcileSessionStepDirectories(studyRoot, protocol); err != nil {
		return Protocol{}, err
	}
	return protocol, nil
}

func ParseProtocolMarkdown(md string) (Protocol, error) {
	doc := ParseStudyDocumentMarkdown(md)
	methods := doc.SectionContent("Methods")
	if strings.TrimSpace(methods) == "" {
		return Protocol{}, fmt.Errorf("study.sg.md missing required section: # Methods")
	}
	return parseProtocolFromMethods(methods)
}

func ParseStudyDocumentMarkdown(md string) StudyDocument {
	lines := strings.Split(md, "\n")
	seenTitle := false
	currentSectionIdx := -1
	var title string
	var leadLines []string
	var sections []StudySection

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "# ") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "# "))
			if !seenTitle {
				seenTitle = true
				title = name
				currentSectionIdx = -1
				continue
			}
			sections = append(sections, StudySection{Name: name})
			currentSectionIdx = len(sections) - 1
			continue
		}
		if !seenTitle {
			continue
		}
		if currentSectionIdx >= 0 {
			sec := &sections[currentSectionIdx]
			if sec.Content == "" {
				sec.Content = raw
			} else {
				sec.Content += "\n" + raw
			}
			continue
		}
		leadLines = append(leadLines, raw)
	}
	for i := range sections {
		sections[i].Content = strings.TrimSpace(sections[i].Content)
	}
	return StudyDocument{
		Title:    strings.TrimSpace(title),
		Lead:     strings.TrimSpace(strings.Join(leadLines, "\n")),
		Sections: sections,
	}
}

func parseProtocolFromMethods(methods string) (Protocol, error) {
	lines := strings.Split(methods, "\n")
	inProtocol := false
	seenProtocol := false
	var summaryLines []string
	var steps []ProtocolStep
	stepDescriptions := map[int][]string{}
	currentStepIdx := -1

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "## ") {
			switch line {
			case "## Protocol":
				seenProtocol = true
				inProtocol = true
				currentStepIdx = -1
				continue
			default:
				inProtocol = false
				currentStepIdx = -1
			}
		}
		if !inProtocol {
			summaryLines = append(summaryLines, raw)
		}
		if inProtocol && strings.HasPrefix(line, "### ") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "### "))
			if name == "" {
				currentStepIdx = -1
				continue
			}
			steps = append(steps, ProtocolStep{
				Name: name,
			})
			currentStepIdx = len(steps) - 1
			continue
		}
		if inProtocol && currentStepIdx >= 0 && !strings.HasPrefix(line, "#") {
			stepDescriptions[currentStepIdx] = append(stepDescriptions[currentStepIdx], raw)
		}
	}

	if !seenProtocol {
		return Protocol{}, fmt.Errorf("study.sg.md missing required section: # Methods -> ## Protocol")
	}
	for i := range steps {
		base := util.Slugify(steps[i].Name)
		if strings.TrimSpace(base) == "" {
			base = "step"
		}
		for j := 0; j < i; j++ {
			if util.Slugify(steps[j].Name) == base {
				return Protocol{}, fmt.Errorf("duplicate protocol step title after slug normalization: %s", steps[i].Name)
			}
		}
		steps[i].Slug = fmt.Sprintf("%02d-%s", i+1, base)
		steps[i].Description = strings.TrimSpace(strings.Join(stepDescriptions[i], "\n"))
	}

	return Protocol{
		Summary: strings.TrimSpace(strings.Join(summaryLines, "\n")),
		Steps:   steps,
	}, nil
}

func ExtractStudyTitle(body string) string {
	return ParseStudyDocumentMarkdown(body).Title
}

func reconcileSessionStepDirectories(studyRoot string, protocol Protocol) error {
	sessionRoot := filepath.Join(studyRoot, "session")
	entries, err := os.ReadDir(sessionRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := reconcileSessionStepDirectory(filepath.Join(sessionRoot, entry.Name()), protocol); err != nil {
			return err
		}
	}
	return nil
}

func reconcileSessionStepDirectory(sessionDir string, protocol Protocol) error {
	stepRoot := filepath.Join(sessionDir, "step")
	entries, err := os.ReadDir(stepRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	type existingStepDir struct {
		name      string
		titleSlug string
		ordinal   string
	}

	existingByName := map[string]existingStepDir{}
	existingByTitle := map[string]existingStepDir{}
	existingByOrdinal := map[string]existingStepDir{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info := existingStepDir{
			name:      entry.Name(),
			titleSlug: stepSlugTitle(entry.Name()),
			ordinal:   stepSlugOrdinalPrefix(entry.Name()),
		}
		if strings.TrimSpace(info.titleSlug) == "" {
			continue
		}
		if _, ok := existingByTitle[info.titleSlug]; ok {
			return fmt.Errorf("session step directory reconcile ambiguous for title %s", info.titleSlug)
		}
		if _, ok := existingByOrdinal[info.ordinal]; ok {
			return fmt.Errorf("session step directory reconcile ambiguous for ordinal %s", strings.TrimSuffix(info.ordinal, "-"))
		}
		existingByName[info.name] = info
		existingByTitle[info.titleSlug] = info
		existingByOrdinal[info.ordinal] = info
	}

	assignedOld := map[string]bool{}
	moves := map[string]string{}
	targetsByOld := map[string]string{}
	unmatchedSteps := make([]ProtocolStep, 0)

	for _, step := range protocol.Steps {
		targetName := step.Slug
		if info, ok := existingByTitle[stepSlugTitle(step.Slug)]; ok {
			assignedOld[info.name] = true
			targetsByOld[info.name] = targetName
			continue
		}
		unmatchedSteps = append(unmatchedSteps, step)
	}

	for _, step := range unmatchedSteps {
		ordinal := stepSlugOrdinalPrefix(step.Slug)
		info, ok := existingByOrdinal[ordinal]
		if !ok || assignedOld[info.name] {
			continue
		}
		assignedOld[info.name] = true
		targetsByOld[info.name] = step.Slug
	}

	for name := range existingByName {
		if assignedOld[name] {
			continue
		}
		return fmt.Errorf("unsupported protocol step reconcile for session %s: stale step directory %s", sessionDir, name)
	}

	for oldName, newName := range targetsByOld {
		if oldName == newName {
			continue
		}
		moves[oldName] = newName
	}
	if len(moves) == 0 {
		return nil
	}

	tempByOld := map[string]string{}
	moveIdx := 0
	for oldName := range moves {
		moveIdx++
		tempByOld[oldName] = filepath.Join(stepRoot, fmt.Sprintf(".reconcile-%03d-%s", moveIdx, oldName))
	}
	for oldName, tempPath := range tempByOld {
		if err := os.Rename(filepath.Join(stepRoot, oldName), tempPath); err != nil {
			return err
		}
	}
	for oldName, newName := range moves {
		if err := os.Rename(tempByOld[oldName], filepath.Join(stepRoot, newName)); err != nil {
			return err
		}
	}
	return nil
}

func stepSlugOrdinalPrefix(slug string) string {
	prefix, _, ok := strings.Cut(slug, "-")
	if !ok {
		return slug
	}
	return prefix + "-"
}

func stepSlugTitle(slug string) string {
	_, rest, ok := strings.Cut(slug, "-")
	if !ok {
		return ""
	}
	return strings.TrimSpace(rest)
}
