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
