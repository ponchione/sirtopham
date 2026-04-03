package index

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func currentRevision(ctx context.Context, projectRoot string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = projectRoot
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if strings.Contains(stderr, "not a git repository") || strings.Contains(stderr, "unknown revision") {
				return "", nil
			}
		}
		return "", fmt.Errorf("resolve git HEAD: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func dirtyTrackedFiles(ctx context.Context, projectRoot string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain", "--untracked-files=no")
	cmd.Dir = projectRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if strings.Contains(msg, "not a git repository") {
			return nil, nil
		}
		return nil, fmt.Errorf("git status --porcelain: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) < 3 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if before, after, ok := strings.Cut(path, " -> "); ok {
			_ = before
			path = after
		}
		files = append(files, path)
	}
	return files, nil
}
