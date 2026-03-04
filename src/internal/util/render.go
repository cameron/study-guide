package util

import (
	"fmt"
	"os/exec"
	"strings"
)

func RenderMarkdown(md string) string {
	cmd := exec.Command("glow", "-")
	cmd.Stdin = strings.NewReader(md)
	out, err := cmd.Output()
	if err != nil {
		return md
	}
	return string(out)
}

func PrintMarkdown(md string) {
	fmt.Print(RenderMarkdown(md))
}
