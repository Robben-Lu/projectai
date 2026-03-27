package applescript

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Run executes an AppleScript snippet via osascript and returns stdout.
func Run(script string) (string, error) {
	cmd := exec.Command("osascript", "-e", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("applescript: %s", msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RunMultiline executes a multi-line AppleScript via osascript.
func RunMultiline(lines []string) (string, error) {
	args := make([]string, 0, len(lines)*2)
	for _, line := range lines {
		args = append(args, "-e", line)
	}
	cmd := exec.Command("osascript", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("applescript: %s", msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}
