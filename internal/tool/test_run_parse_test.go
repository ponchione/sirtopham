package tool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGoTestJSON_AllPass(t *testing.T) {
	input := `{"Action":"run","Test":"TestFoo","Package":"example.com/pkg"}
{"Action":"output","Test":"TestFoo","Package":"example.com/pkg","Output":"=== RUN   TestFoo\n"}
{"Action":"pass","Test":"TestFoo","Package":"example.com/pkg","Elapsed":0.001}
{"Action":"run","Test":"TestBar","Package":"example.com/pkg"}
{"Action":"output","Test":"TestBar","Package":"example.com/pkg","Output":"=== RUN   TestBar\n"}
{"Action":"pass","Test":"TestBar","Package":"example.com/pkg","Elapsed":0.002}
{"Action":"pass","Package":"example.com/pkg","Elapsed":0.003}`

	r := parseGoTestJSON(input)

	if r.Ecosystem != "go" {
		t.Errorf("Ecosystem = %q, want 'go'", r.Ecosystem)
	}
	if r.Passed != 2 {
		t.Errorf("Passed = %d, want 2", r.Passed)
	}
	if r.Failed != 0 {
		t.Errorf("Failed = %d, want 0", r.Failed)
	}
	if r.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0", r.Skipped)
	}
	if len(r.Failures) != 0 {
		t.Errorf("Failures = %v, want empty", r.Failures)
	}
	if len(r.BuildErrors) != 0 {
		t.Errorf("BuildErrors = %v, want empty", r.BuildErrors)
	}
	if !strings.Contains(r.Summary, "GO PASS") {
		t.Errorf("Summary = %q, expected GO PASS", r.Summary)
	}
}

func TestParseGoTestJSON_WithFailure(t *testing.T) {
	input := `{"Action":"run","Test":"TestPass","Package":"example.com/mypkg"}
{"Action":"pass","Test":"TestPass","Package":"example.com/mypkg","Elapsed":0.001}
{"Action":"run","Test":"TestFail","Package":"example.com/mypkg"}
{"Action":"output","Test":"TestFail","Package":"example.com/mypkg","Output":"    foo_test.go:12: got 0, want 1\n"}
{"Action":"fail","Test":"TestFail","Package":"example.com/mypkg","Elapsed":0.002}
{"Action":"fail","Package":"example.com/mypkg","Elapsed":0.003}`

	r := parseGoTestJSON(input)

	if r.Passed != 1 {
		t.Errorf("Passed = %d, want 1", r.Passed)
	}
	if r.Failed != 1 {
		t.Errorf("Failed = %d, want 1", r.Failed)
	}
	if len(r.Failures) != 1 {
		t.Fatalf("len(Failures) = %d, want 1", len(r.Failures))
	}

	f := r.Failures[0]
	if f.Test != "TestFail" {
		t.Errorf("Failure.Test = %q, want 'TestFail'", f.Test)
	}
	if f.Package != "example.com/mypkg" {
		t.Errorf("Failure.Package = %q, want 'example.com/mypkg'", f.Package)
	}
	if !strings.Contains(f.Output, "got 0, want 1") {
		t.Errorf("Failure.Output = %q, expected 'got 0, want 1'", f.Output)
	}
	if !strings.Contains(r.Summary, "GO FAIL") {
		t.Errorf("Summary = %q, expected GO FAIL", r.Summary)
	}
	// Build errors should be empty since tests ran.
	if len(r.BuildErrors) != 0 {
		t.Errorf("BuildErrors = %v, want empty", r.BuildErrors)
	}
}

func TestParseGoTestJSON_BuildError(t *testing.T) {
	// Package-level fail with output but no Test field actions (no tests ran).
	input := `{"Action":"output","Package":"example.com/broken","Output":"# example.com/broken\n"}
{"Action":"output","Package":"example.com/broken","Output":"./broken.go:5:2: undefined: missingFunc\n"}
{"Action":"fail","Package":"example.com/broken","Elapsed":0.1}`

	r := parseGoTestJSON(input)

	if r.Passed != 0 {
		t.Errorf("Passed = %d, want 0", r.Passed)
	}
	if r.Failed != 0 {
		t.Errorf("Failed = %d, want 0 (build error, not test failure)", r.Failed)
	}
	if len(r.BuildErrors) == 0 {
		t.Fatal("BuildErrors is empty, expected at least one build error")
	}
	combined := strings.Join(r.BuildErrors, "\n")
	if !strings.Contains(combined, "undefined") && !strings.Contains(combined, "broken") {
		t.Errorf("BuildErrors = %v, expected build error content", r.BuildErrors)
	}
}

func TestFormatTestResult_BuildErrors(t *testing.T) {
	r := testRunResult{
		Ecosystem:   "go",
		BuildErrors: []string{"./foo.go:5:2: undefined: bar"},
	}
	out := formatTestResult(r)
	if !strings.Contains(out, "BUILD ERRORS") {
		t.Errorf("expected BUILD ERRORS section, got:\n%s", out)
	}
	if !strings.Contains(out, "undefined: bar") {
		t.Errorf("expected build error text, got:\n%s", out)
	}
}

func TestFormatTestResult_WithFailures(t *testing.T) {
	r := testRunResult{
		Ecosystem: "go",
		Passed:    1,
		Failed:    1,
		Failures: []testFailure{
			{Test: "TestBad", Package: "example.com/pkg", Output: "    got 0 want 1\n"},
		},
	}
	out := formatTestResult(r)
	if !strings.Contains(out, "FAILURES") {
		t.Errorf("expected FAILURES section, got:\n%s", out)
	}
	if !strings.Contains(out, "--- example.com/pkg/TestBad") {
		t.Errorf("expected failure header, got:\n%s", out)
	}
	if !strings.Contains(out, "got 0 want 1") {
		t.Errorf("expected failure output, got:\n%s", out)
	}
	if !strings.Contains(out, "GO FAIL") {
		t.Errorf("expected GO FAIL in summary, got:\n%s", out)
	}
}

func TestParsePytestJSON_AllPass(t *testing.T) {
	input := `{"summary":{"passed":3,"total":3},"tests":[{"nodeid":"tests/test_auth.py::test_login","outcome":"passed"},{"nodeid":"tests/test_auth.py::test_logout","outcome":"passed"},{"nodeid":"tests/test_auth.py::test_signup","outcome":"passed"}]}`
	result := parsePytestJSON(input)
	if result.Passed != 3 {
		t.Fatalf("expected 3 passed, got %d", result.Passed)
	}
	if result.Failed != 0 {
		t.Fatalf("expected 0 failed, got %d", result.Failed)
	}
}

func TestParsePytestJSON_WithFailure(t *testing.T) {
	input := `{"summary":{"passed":1,"failed":1,"total":2},"tests":[{"nodeid":"tests/test_auth.py::test_login","outcome":"passed"},{"nodeid":"tests/test_auth.py::test_signup","outcome":"failed","call":{"longrepr":"AssertionError: expected 200, got 401"}}]}`
	result := parsePytestJSON(input)
	if result.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", result.Failed)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(result.Failures))
	}
	if !strings.Contains(result.Failures[0].Output, "expected 200, got 401") {
		t.Fatalf("expected failure output, got: %s", result.Failures[0].Output)
	}
}

func TestParsePytestShort_WithFailure(t *testing.T) {
	input := "FAILED tests/test_auth.py::test_signup - AssertionError: expected 200\n1 passed, 1 failed in 0.34s"
	result := parsePytestShort(input)
	if result.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", result.Failed)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(result.Failures))
	}
}

func TestFormatTestSummary(t *testing.T) {
	s := formatTestSummary("go", 5, 5, 0, 0)
	if s != "GO PASS — 5 passed, 0 failed, 0 skipped, 5 total" {
		t.Errorf("unexpected summary: %q", s)
	}

	s = formatTestSummary("python", 3, 2, 1, 0)
	if s != "PYTHON FAIL — 2 passed, 1 failed, 0 skipped, 3 total" {
		t.Errorf("unexpected summary: %q", s)
	}
}

func TestParseJestJSON_AllPass(t *testing.T) {
	input := `{"numPassedTests":5,"numFailedTests":0,"numPendingTests":1,"testResults":[]}`
	result := parseJestJSON(input)
	if result.Passed != 5 {
		t.Fatalf("expected 5 passed, got %d", result.Passed)
	}
	if result.Failed != 0 {
		t.Fatalf("expected 0 failed, got %d", result.Failed)
	}
	if result.Skipped != 1 {
		t.Fatalf("expected 1 skipped, got %d", result.Skipped)
	}
}

func TestParseJestJSON_WithFailure(t *testing.T) {
	input := `{"numPassedTests":2,"numFailedTests":1,"numPendingTests":0,"testResults":[{"testFilePath":"/app/src/components/Button.test.tsx","testResults":[{"fullName":"Button renders correctly","status":"passed"},{"fullName":"Button handles click","status":"failed","failureMessages":["Expected: 1\nReceived: 0"]}]}]}`
	result := parseJestJSON(input)
	if result.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", result.Failed)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(result.Failures))
	}
	f := result.Failures[0]
	if f.Test != "Button handles click" {
		t.Fatalf("expected test name, got: %s", f.Test)
	}
	if !strings.Contains(f.Output, "Expected: 1") {
		t.Fatalf("expected failure output, got: %s", f.Output)
	}
	if !strings.Contains(f.File, "Button.test.tsx") {
		t.Fatalf("expected file path, got: %s", f.File)
	}
}

func TestDetectTSTestRunner(t *testing.T) {
	t.Run("default jest", func(t *testing.T) {
		dir := t.TempDir()
		runner, _ := detectTSTestRunner(dir)
		if runner != "jest" {
			t.Fatalf("expected jest, got %s", runner)
		}
	})
	t.Run("vitest config", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "vitest.config.ts"), []byte("export default {}"), 0o644)
		runner, _ := detectTSTestRunner(dir)
		if runner != "vitest" {
			t.Fatalf("expected vitest, got %s", runner)
		}
	})
}
