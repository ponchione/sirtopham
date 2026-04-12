package initializer

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// InstallOptions configure a single Install() call.
type InstallOptions struct {
	// ConfigPath is the path to the yard.yaml file to substitute. Required.
	ConfigPath string

	// SodoryardAgentsDir is the absolute path to the sodoryard install's
	// agents/ directory. This value replaces every occurrence of
	// {{SODORYARD_AGENTS_DIR}} in the config file. Required.
	SodoryardAgentsDir string
}

// InstallResult describes what Install() did.
type InstallResult struct {
	// Substitutions is the number of {{SODORYARD_AGENTS_DIR}} occurrences
	// that were replaced. Zero means the file was already fully substituted
	// and the call was a no-op.
	Substitutions int

	// ConfigPath is the absolute path to the file that was modified
	// (or would have been modified, if Substitutions == 0).
	ConfigPath string
}

// installPlaceholder is the literal token that gets substituted.
const installPlaceholder = "{{SODORYARD_AGENTS_DIR}}"

// Install reads opts.ConfigPath, replaces every occurrence of
// {{SODORYARD_AGENTS_DIR}} with opts.SodoryardAgentsDir, and writes the
// result back. Idempotent: running on an already-substituted file is a
// no-op (no occurrences left to replace).
//
// Install does NOT validate that opts.SodoryardAgentsDir exists on disk.
// Install does NOT touch any placeholder other than {{SODORYARD_AGENTS_DIR}}.
// Install does NOT write a backup of the original file.
func Install(opts InstallOptions) (*InstallResult, error) {
	if strings.TrimSpace(opts.SodoryardAgentsDir) == "" {
		return nil, errors.New("install: SodoryardAgentsDir is required")
	}
	if strings.TrimSpace(opts.ConfigPath) == "" {
		return nil, errors.New("install: ConfigPath is required")
	}

	data, err := os.ReadFile(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("install: read %s: %w", opts.ConfigPath, err)
	}

	original := string(data)
	count := strings.Count(original, installPlaceholder)
	if count == 0 {
		return &InstallResult{Substitutions: 0, ConfigPath: opts.ConfigPath}, nil
	}

	updated := strings.ReplaceAll(original, installPlaceholder, opts.SodoryardAgentsDir)
	if err := os.WriteFile(opts.ConfigPath, []byte(updated), 0o644); err != nil {
		return nil, fmt.Errorf("install: write %s: %w", opts.ConfigPath, err)
	}

	return &InstallResult{Substitutions: count, ConfigPath: opts.ConfigPath}, nil
}
