package tool

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ponchione/sodoryard/internal/pathguard"
)

// resolvePath resolves a relative path against projectRoot and validates it.
// Returns the absolute path or an error if the path is invalid.
//
// Rejects:
//   - Absolute paths (starting with /)
//   - Paths that escape the project root after resolution (e.g., ../../etc/passwd)
func resolvePath(projectRoot, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	resolved, err := pathguard.Resolve(projectRoot, path)
	if errors.Is(err, pathguard.ErrAbsolutePath) {
		return "", fmt.Errorf("absolute paths are not allowed; use a path relative to the project root")
	}
	if errors.Is(err, pathguard.ErrEscapesRoot) {
		return "", fmt.Errorf("path escapes project root. All file operations are restricted to the project directory")
	}
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	return resolved, nil
}

func failureResult(content, err string) *ToolResult {
	return &ToolResult{Success: false, Content: content, Error: err}
}

func invalidInputResult(err error) *ToolResult {
	return failureResult(fmt.Sprintf("Invalid input: %v", err), err.Error())
}

func requiredFieldResult(field string) *ToolResult {
	return failureResult(fmt.Sprintf("%s is required", field), "empty "+field)
}

func resolvePathResult(projectRoot, relPath string) (string, *ToolResult) {
	absPath, err := resolvePath(projectRoot, relPath)
	if err != nil {
		return "", failureResult(err.Error(), err.Error())
	}
	return absPath, nil
}

// listDirFiles returns a sorted list of file names in the given directory.
// Returns nil if the directory doesn't exist or can't be read.
func listDirFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// fileNotFoundError returns an enriched error message listing files in the
// same directory as the missing file.
func fileNotFoundError(projectRoot, relPath string) string {
	dir := filepath.Dir(filepath.Join(projectRoot, relPath))
	files := listDirFiles(dir)
	if len(files) == 0 {
		return fmt.Sprintf("File not found: %s", relPath)
	}

	// Truncate if too many files.
	shown := files
	suffix := ""
	if len(files) > 20 {
		shown = files[:20]
		suffix = fmt.Sprintf(" ... and %d more", len(files)-20)
	}

	relDir := filepath.Dir(relPath)
	if relDir == "." {
		relDir = ""
	} else {
		relDir += "/"
	}
	return fmt.Sprintf("File not found: %s. Files in %s: %s%s",
		relPath, relDir, strings.Join(shown, ", "), suffix)
}

// isBinaryContent checks if data contains null bytes, indicating binary content.
// Checks only the first 8KB for efficiency.
func isBinaryContent(data []byte) bool {
	limit := 8192
	if len(data) < limit {
		limit = len(data)
	}
	for i := 0; i < limit; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// fileExists reports whether the file or directory at path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
