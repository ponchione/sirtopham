package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"os/exec"
)

// TestRun implements the test_run tool — auto-detects the test ecosystem and
// runs the test suite, returning structured failures-only output.
type TestRun struct{}

type testRunInput struct {
	Ecosystem      string  `json:"ecosystem,omitempty"`
	Path           string  `json:"path,omitempty"`
	Filter         string  `json:"filter,omitempty"`
	Verbose        bool    `json:"verbose,omitempty"`
	TimeoutSeconds *int    `json:"timeout_seconds,omitempty"`
}

func (TestRun) Name() string        { return "test_run" }
func (TestRun) Description() string { return "Run tests and return structured failures-only output" }
func (TestRun) ToolPurity() Purity  { return Mutating }

func (TestRun) Schema() json.RawMessage {
	return json.RawMessage(`{
		"name": "test_run",
		"description": "Auto-detects Go/Python/TypeScript and runs the test suite. Returns structured output with failures only. Exits non-zero on test failure but does not report that as a tool error.",
		"input_schema": {
			"type": "object",
			"properties": {
				"ecosystem": {
					"type": "string",
					"enum": ["go", "python", "typescript"],
					"description": "Override ecosystem detection"
				},
				"path": {
					"type": "string",
					"description": "Subdirectory or package path to test (relative to project root)"
				},
				"filter": {
					"type": "string",
					"description": "Test name filter (maps to -run for Go)"
				},
				"verbose": {
					"type": "boolean",
					"description": "Enable verbose output (default: false)"
				},
				"timeout_seconds": {
					"type": "integer",
					"description": "Test timeout in seconds (default: 300)"
				}
			}
		}
	}`)
}

func (t TestRun) Execute(ctx context.Context, projectRoot string, input json.RawMessage) (*ToolResult, error) {
	var params testRunInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return &ToolResult{
				Success: false,
				Content: fmt.Sprintf("Invalid input: %v", err),
				Error:   err.Error(),
			}, nil
		}
	}

	timeoutSec := 300
	if params.TimeoutSeconds != nil && *params.TimeoutSeconds > 0 {
		timeoutSec = *params.TimeoutSeconds
	}

	// Resolve the target directory.
	targetDir := projectRoot
	if params.Path != "" {
		resolved, err := resolvePath(projectRoot, params.Path)
		if err != nil {
			return &ToolResult{
				Success: false,
				Content: err.Error(),
				Error:   err.Error(),
			}, nil
		}
		targetDir = resolved
	}

	// Detect or use provided ecosystem.
	ecosystem := params.Ecosystem
	if ecosystem == "" {
		ecosystem = detectTestEcosystem(targetDir, projectRoot)
	}
	if ecosystem == "" {
		return &ToolResult{
			Success: false,
			Content: "Could not detect test ecosystem. No go.mod, pyproject.toml, setup.py, setup.cfg, or package.json found.",
			Error:   "no ecosystem detected",
		}, nil
	}

	switch ecosystem {
	case "go":
		return t.runGoTests(ctx, projectRoot, targetDir, params.Path, params.Filter, timeoutSec)
	case "python":
		return runPythonTests(ctx, projectRoot, params, time.Duration(timeoutSec)*time.Second)
	case "typescript":
		return runTypeScriptTests(ctx, projectRoot, params, time.Duration(timeoutSec)*time.Second)
	default:
		return &ToolResult{
			Success: false,
			Content: fmt.Sprintf("Unknown ecosystem: %s", ecosystem),
			Error:   "unknown ecosystem",
		}, nil
	}
}

// detectTestEcosystem walks from detectDir up to projectRoot looking for
// ecosystem marker files. Returns "" if nothing is found.
func detectTestEcosystem(detectDir, projectRoot string) string {
	dir := detectDir
	for {
		if fileExists(filepath.Join(dir, "go.mod")) {
			return "go"
		}
		if fileExists(filepath.Join(dir, "pyproject.toml")) ||
			fileExists(filepath.Join(dir, "setup.py")) ||
			fileExists(filepath.Join(dir, "setup.cfg")) {
			return "python"
		}
		if fileExists(filepath.Join(dir, "package.json")) {
			return "typescript"
		}

		// Stop at project root.
		if dir == projectRoot {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Filesystem root reached without hitting projectRoot.
			break
		}
		dir = parent
	}
	return ""
}

// runPythonTests runs pytest and returns structured output.
func runPythonTests(ctx context.Context, projectRoot string, params testRunInput, timeout time.Duration) (*ToolResult, error) {
	pytestPath, err := lookupCommandPath("pytest")
	if err != nil {
		return &ToolResult{
			Success: false,
			Content: "pytest is required but not found in PATH. Install: pip install pytest",
			Error:   "pytest not found",
		}, nil
	}

	useJSON := pytestJSONReportAvailable(ctx, pytestPath, projectRoot)

	args := []string{}
	if useJSON {
		args = append(args, "--json-report", "--json-report-file=-", "-q")
	} else {
		args = append(args, "-q", "--tb=short", "--no-header")
	}
	if params.Filter != "" {
		args = append(args, "-k", params.Filter)
	}
	if params.Path != "" {
		args = append(args, params.Path)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout+10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, pytestPath, args...)
	cmd.Dir = projectRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Run() //nolint:errcheck

	var result testRunResult
	if useJSON {
		result = parsePytestJSON(stdout.String())
	} else {
		result = parsePytestShort(stdout.String())
	}

	if stderr.Len() > 0 && len(result.BuildErrors) == 0 && result.Passed == 0 && result.Failed == 0 {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			result.BuildErrors = append(result.BuildErrors, stderrStr)
		}
	}

	return &ToolResult{Success: true, Content: formatTestResult(result)}, nil
}

// pytestJSONReportAvailable checks whether pytest-json-report is installed.
func pytestJSONReportAvailable(ctx context.Context, pytestPath, projectRoot string) bool {
	cmd := exec.CommandContext(ctx, pytestPath, "--co", "--json-report", "--json-report-file=/dev/null", "-q")
	cmd.Dir = projectRoot
	err := cmd.Run()
	return err == nil
}

// runTypeScriptTests runs jest or vitest and returns structured output.
func runTypeScriptTests(ctx context.Context, projectRoot string, params testRunInput, timeout time.Duration) (*ToolResult, error) {
	runner, runnerArgs := detectTSTestRunner(projectRoot)

	npxPath, err := lookupCommandPath("npx")
	if err != nil {
		return &ToolResult{
			Success: false,
			Content: "npx is required but not found in PATH. Install Node.js.",
			Error:   "npx not found",
		}, nil
	}

	args := append([]string{runner}, runnerArgs...)
	if runner == "vitest" {
		args = append(args, "run", "--reporter=json")
	} else {
		args = append(args, "--json", "--forceExit")
	}
	if params.Filter != "" {
		args = append(args, "--testNamePattern", params.Filter)
	}
	if params.Path != "" {
		args = append(args, params.Path)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout+10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, npxPath, args...)
	cmd.Dir = projectRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Run() //nolint:errcheck

	result := parseJestJSON(stdout.String())

	if stderr.Len() > 0 && len(result.BuildErrors) == 0 && result.Passed == 0 && result.Failed == 0 {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			result.BuildErrors = append(result.BuildErrors, stderrStr)
		}
	}

	return &ToolResult{Success: true, Content: formatTestResult(result)}, nil
}

// detectTSTestRunner returns the test runner and any extra args based on config files present.
func detectTSTestRunner(projectRoot string) (runner string, args []string) {
	vitestConfigs := []string{"vitest.config.ts", "vitest.config.js", "vitest.config.mts"}
	for _, cfg := range vitestConfigs {
		if fileExists(filepath.Join(projectRoot, cfg)) {
			return "vitest", nil
		}
	}
	return "jest", nil
}

// runGoTests runs `go test -json` and returns structured output.
func (TestRun) runGoTests(ctx context.Context, projectRoot, targetDir, pathParam, filter string, timeoutSec int) (*ToolResult, error) {
	goPath, err := lookupCommandPath("go")
	if err != nil {
		return &ToolResult{
			Success: false,
			Content: "go not found in PATH",
			Error:   "go not found",
		}, nil
	}

	args := []string{"test", "-json"}
	if filter != "" {
		args = append(args, "-run", filter)
	}
	args = append(args, fmt.Sprintf("-timeout=%ds", timeoutSec))

	// Determine package argument.
	if pathParam != "" {
		// If the path looks like a Go package path (no path separators that would
		// make it a directory), pass it directly; otherwise use ./...
		if strings.Contains(pathParam, "/") || strings.Contains(pathParam, string(filepath.Separator)) {
			// It's a subdir — use ./... relative to targetDir.
			args = append(args, "./...")
		} else {
			args = append(args, pathParam)
		}
	} else {
		args = append(args, "./...")
	}

	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec+10)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, goPath, args...)
	cmd.Dir = targetDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Ignore the error — go test exits non-zero on failure, which is expected.
	cmd.Run() //nolint:errcheck

	result := parseGoTestJSON(stdout.String())

	// Check stderr for build errors not captured in JSON.
	stderrStr := strings.TrimSpace(stderr.String())
	if stderrStr != "" && len(result.BuildErrors) == 0 {
		result.BuildErrors = append(result.BuildErrors, stderrStr)
	}

	content := formatTestResult(result)
	return &ToolResult{
		Success: true,
		Content: content,
	}, nil
}

