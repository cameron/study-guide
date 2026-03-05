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

type ProtocolStep struct {
	Name        string
	Slug        string
	Description string
}

func ReadRequiredSubjectFields(studyRoot string) ([]string, error) {
	p := filepath.Join(studyRoot, "subject-requirements.yaml")
	if _, err := os.Stat(p); err != nil {
		return nil, nil
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var cfg struct {
		RequiredFields []string `yaml:"required_fields"`
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	var out []string
	for _, s := range cfg.RequiredFields {
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
	p := filepath.Join(studyRoot, "protocol.sg.md")
	b, err := os.ReadFile(p)
	if err != nil {
		return Protocol{}, err
	}
	return ParseProtocolMarkdown(string(b))
}

func ParseProtocolMarkdown(md string) (Protocol, error) {
	lines := strings.Split(md, "\n")
	inSummary := false
	inSteps := false
	seenSummary := false
	seenSteps := false
	var summaryLines []string
	var steps []ProtocolStep
	stepDescriptions := map[int][]string{}
	currentStepIdx := -1

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "# ") {
			switch line {
			case "# Protocol Summary":
				seenSummary = true
				inSummary = true
				inSteps = false
				currentStepIdx = -1
				continue
			case "# Steps":
				seenSteps = true
				inSteps = true
				inSummary = false
				currentStepIdx = -1
				continue
			default:
				inSummary = false
				inSteps = false
				currentStepIdx = -1
			}
		}
		if inSummary {
			summaryLines = append(summaryLines, raw)
		}
		if inSteps && strings.HasPrefix(line, "## ") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "## "))
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
		if inSteps && currentStepIdx >= 0 && !strings.HasPrefix(line, "#") {
			stepDescriptions[currentStepIdx] = append(stepDescriptions[currentStepIdx], raw)
		}
	}

	if !seenSummary {
		return Protocol{}, fmt.Errorf("protocol.sg.md missing required section: # Protocol Summary")
	}
	if !seenSteps {
		return Protocol{}, fmt.Errorf("protocol.sg.md missing required section: # Steps")
	}
	for i := range steps {
		base := util.Slugify(steps[i].Name)
		if strings.TrimSpace(base) == "" {
			base = "step"
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
	for _, line := range strings.Split(body, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(l, "# "))
		}
	}
	return ""
}
