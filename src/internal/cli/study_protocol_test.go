package cli

import "strings"

func injectProtocolIntoStudy(studyContent, methodsSummary string, stepNames ...string) string {
	var b strings.Builder
	if strings.TrimSpace(methodsSummary) != "" {
		b.WriteString(strings.TrimSpace(methodsSummary))
	}
	if len(stepNames) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("## Protocol")
		for _, step := range stepNames {
			if strings.TrimSpace(step) == "" {
				continue
			}
			b.WriteString("\n\n### ")
			b.WriteString(strings.TrimSpace(step))
		}
	}
	return replaceSection(studyContent, "Methods", b.String())
}
