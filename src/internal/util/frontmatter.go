package util

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func ReadFrontmatterFile(path string) (map[string]any, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	return ParseFrontmatter(string(b))
}

func ParseFrontmatter(s string) (map[string]any, string, error) {
	out := map[string]any{}
	if !strings.HasPrefix(s, "---\n") {
		return out, s, nil
	}
	rest := s[len("---\n"):]
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return nil, "", fmt.Errorf("invalid frontmatter: missing closing delimiter")
	}
	fm := rest[:idx]
	body := rest[idx+len("\n---\n"):]
	if strings.TrimSpace(fm) != "" {
		if err := yaml.Unmarshal([]byte(fm), &out); err != nil {
			return nil, "", err
		}
	}
	return out, body, nil
}

func WriteFrontmatterFile(path string, fm map[string]any, body string) error {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	y, err := yaml.Marshal(toOrderedNode(fm))
	if err != nil {
		return err
	}
	buf.Write(y)
	buf.WriteString("---\n")
	if body != "" {
		if !strings.HasPrefix(body, "\n") {
			buf.WriteString("\n")
		}
		buf.WriteString(body)
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func toOrderedNode(m map[string]any) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	keys := orderedKeys(m)
	for _, k := range keys {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}
		valNode := &yaml.Node{}
		_ = valNode.Encode(m[k])
		n.Content = append(n.Content, keyNode, valNode)
	}
	return n
}

func orderedKeys(m map[string]any) []string {
	priority := map[string]int{
		"uuid":           10,
		"type":           20,
		"name":           30,
		"status":         40,
		"created_on":     50,
		"updated_on":     60,
		"time_started":   70,
		"time_finished":  80,
		"subject_ids":    90,
		"pi_subject_ids": 100,
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		pi, okI := priority[keys[i]]
		pj, okJ := priority[keys[j]]
		switch {
		case okI && okJ:
			if pi == pj {
				return keys[i] < keys[j]
			}
			return pi < pj
		case okI:
			return true
		case okJ:
			return false
		default:
			return keys[i] < keys[j]
		}
	})
	return keys
}
