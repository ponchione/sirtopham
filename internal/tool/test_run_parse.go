package tool

import (
	"encoding/json"
	"fmt"
	"strings"
)

// testRunResult holds the structured output from a test run.
type testRunResult struct {
	Ecosystem   string
	Passed      int
	Failed      int
	Skipped     int
	Failures    []testFailure
	BuildErrors []string
	Summary     string
}

// testFailure describes a single failing test case.
type testFailure struct {
	Test    string
	Package string
	File    string
	Output  string
}

// formatTestResult formats a testRunResult for agent consumption.
func formatTestResult(r testRunResult) string {
	var sb strings.Builder

	if len(r.BuildErrors) > 0 {
		sb.WriteString("BUILD ERRORS:\n")
		for _, e := range r.BuildErrors {
			sb.WriteString(e)
			if !strings.HasSuffix(e, "\n") {
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}

	total := r.Passed + r.Failed + r.Skipped
	sb.WriteString(formatTestSummary(r.Ecosystem, total, r.Passed, r.Failed, r.Skipped))
	sb.WriteString("\n")

	if len(r.Failures) > 0 {
		sb.WriteString("\nFAILURES:\n")
		for _, f := range r.Failures {
			header := fmt.Sprintf("--- %s/%s", f.Package, f.Test)
			sb.WriteString(header)
			sb.WriteString("\n")
			if strings.TrimSpace(f.Output) != "" {
				sb.WriteString(f.Output)
				if !strings.HasSuffix(f.Output, "\n") {
					sb.WriteString("\n")
				}
			}
		}
	}

	return sb.String()
}

// formatTestSummary returns a one-line summary string.
func formatTestSummary(ecosystem string, total, passed, failed, skipped int) string {
	eco := strings.ToUpper(ecosystem)
	status := "PASS"
	if failed > 0 {
		status = "FAIL"
	}
	return fmt.Sprintf("%s %s — %d passed, %d failed, %d skipped, %d total",
		eco, status, passed, failed, skipped, total)
}

// pytestReport is the top-level structure from `pytest --json-report`.
type pytestReport struct {
	Summary pytestSummary `json:"summary"`
	Tests   []pytestTest  `json:"tests"`
}

type pytestSummary struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Total   int `json:"total"`
}

type pytestTest struct {
	NodeID  string      `json:"nodeid"`
	Outcome string      `json:"outcome"`
	Call    *pytestCall `json:"call,omitempty"`
}

type pytestCall struct {
	LongRepr string `json:"longrepr"`
}

// parsePytestJSON parses the output of `pytest --json-report --json-report-file=-`.
func parsePytestJSON(raw string) testRunResult {
	r := testRunResult{Ecosystem: "python"}

	var report pytestReport
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		r.BuildErrors = append(r.BuildErrors, fmt.Sprintf("failed to parse pytest JSON output: %v", err))
		return r
	}

	r.Passed = report.Summary.Passed
	r.Failed = report.Summary.Failed
	r.Skipped = report.Summary.Skipped

	for _, test := range report.Tests {
		if test.Outcome == "failed" {
			out := ""
			if test.Call != nil {
				out = test.Call.LongRepr
			}
			r.Failures = append(r.Failures, testFailure{
				Test:   test.NodeID,
				Output: out,
			})
		}
	}

	return r
}

// parsePytestShort parses the short output of `pytest -q --tb=short --no-header`
// for when pytest-json-report is not installed.
func parsePytestShort(raw string) testRunResult {
	r := testRunResult{Ecosystem: "python"}

	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "FAILED ") {
			// Format: "FAILED tests/test_foo.py::test_bar - ErrorType: message"
			rest := strings.TrimPrefix(line, "FAILED ")
			name := rest
			output := ""
			if idx := strings.Index(rest, " - "); idx >= 0 {
				name = rest[:idx]
				output = rest[idx+3:]
			}
			r.Failures = append(r.Failures, testFailure{
				Test:   name,
				Output: output,
			})
		}

		// Summary line: "1 passed, 1 failed in 0.34s" or "3 passed in 0.12s"
		if strings.Contains(line, " passed") || strings.Contains(line, " failed") || strings.Contains(line, " error") {
			parsePytestSummaryLine(line, &r)
		}
	}

	return r
}

// parsePytestSummaryLine parses a pytest summary line like "1 passed, 2 failed in 0.34s".
func parsePytestSummaryLine(line string, result *testRunResult) {
	// Strip the " in X.XXs" suffix if present.
	if idx := strings.Index(line, " in "); idx >= 0 {
		line = line[:idx]
	}
	parts := strings.Split(line, ", ")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		fields := strings.Fields(part)
		if len(fields) < 2 {
			continue
		}
		n := 0
		fmt.Sscanf(fields[0], "%d", &n)
		label := fields[1]
		switch label {
		case "passed":
			result.Passed = n
		case "failed":
			result.Failed = n
		case "skipped":
			result.Skipped = n
		}
	}
}

// jestReport is the top-level structure from `jest --json` or `vitest run --reporter=json`.
type jestReport struct {
	NumPassedTests  int             `json:"numPassedTests"`
	NumFailedTests  int             `json:"numFailedTests"`
	NumPendingTests int             `json:"numPendingTests"`
	TestResults     []jestTestSuite `json:"testResults"`
}

type jestTestSuite struct {
	TestFilePath string           `json:"testFilePath"`
	TestResults  []jestTestResult `json:"testResults"`
}

type jestTestResult struct {
	FullName        string   `json:"fullName"`
	Status          string   `json:"status"`
	FailureMessages []string `json:"failureMessages"`
}

// parseJestJSON parses the output of `jest --json` or `vitest run --reporter=json`.
func parseJestJSON(raw string) testRunResult {
	r := testRunResult{Ecosystem: "typescript"}

	var report jestReport
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		r.BuildErrors = append(r.BuildErrors, fmt.Sprintf("failed to parse jest JSON output: %v", err))
		return r
	}

	r.Passed = report.NumPassedTests
	r.Failed = report.NumFailedTests
	r.Skipped = report.NumPendingTests

	for _, suite := range report.TestResults {
		for _, test := range suite.TestResults {
			if test.Status == "failed" {
				r.Failures = append(r.Failures, testFailure{
					Test:   test.FullName,
					File:   suite.TestFilePath,
					Output: strings.Join(test.FailureMessages, "\n"),
				})
			}
		}
	}

	return r
}

// goTestEvent is one line of `go test -json` output.
type goTestEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Output  string  `json:"Output"`
	Elapsed float64 `json:"Elapsed"`
}

// parseGoTestJSON parses the output of `go test -json`.
func parseGoTestJSON(raw string) testRunResult {
	r := testRunResult{Ecosystem: "go"}

	// Per-test accumulated output: key = "package/TestName"
	testOutputs := make(map[string]*strings.Builder)
	// Package-level output (no Test field): key = package
	packageOutputs := make(map[string]*strings.Builder)
	// Track which packages had any test actions (run/pass/fail/skip on a Test)
	packagesWithTests := make(map[string]bool)

	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var ev goTestEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			// Non-JSON line — skip.
			continue
		}

		if ev.Test != "" {
			key := ev.Package + "/" + ev.Test
			switch ev.Action {
			case "run":
				packagesWithTests[ev.Package] = true
			case "output":
				if testOutputs[key] == nil {
					testOutputs[key] = &strings.Builder{}
				}
				testOutputs[key].WriteString(ev.Output)
			case "pass":
				packagesWithTests[ev.Package] = true
				r.Passed++
			case "fail":
				packagesWithTests[ev.Package] = true
				r.Failed++
				out := ""
				if testOutputs[key] != nil {
					out = testOutputs[key].String()
				}
				r.Failures = append(r.Failures, testFailure{
					Test:    ev.Test,
					Package: ev.Package,
					Output:  out,
				})
			case "skip":
				packagesWithTests[ev.Package] = true
				r.Skipped++
			}
		} else {
			// Package-level event.
			switch ev.Action {
			case "output":
				if packageOutputs[ev.Package] == nil {
					packageOutputs[ev.Package] = &strings.Builder{}
				}
				packageOutputs[ev.Package].WriteString(ev.Output)
			case "fail":
				// Package-level fail with no tests = build error.
				if !packagesWithTests[ev.Package] {
					out := ""
					if packageOutputs[ev.Package] != nil {
						out = packageOutputs[ev.Package].String()
					}
					if strings.TrimSpace(out) != "" {
						r.BuildErrors = append(r.BuildErrors, strings.TrimRight(out, "\n"))
					} else {
						r.BuildErrors = append(r.BuildErrors, fmt.Sprintf("build failed: %s", ev.Package))
					}
				}
			}
		}
	}

	total := r.Passed + r.Failed + r.Skipped
	r.Summary = formatTestSummary("go", total, r.Passed, r.Failed, r.Skipped)
	return r
}
