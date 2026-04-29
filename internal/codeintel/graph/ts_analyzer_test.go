package graph

import (
	"os/exec"
	"testing"
)

func TestTSAnalyzer_NewRequiresNode(t *testing.T) {
	_, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not on PATH, skipping")
	}

	analyzer, err := NewTSAnalyzer()
	if err != nil {
		t.Fatalf("NewTSAnalyzer: %v", err)
	}

	if analyzer.scriptPath == "" {
		t.Error("scriptPath should not be empty")
	}
}

func TestTSAnalyzer_AnalyzeSmallProject(t *testing.T) {
	_, err := exec.LookPath("npx")
	if err != nil {
		t.Skip("npx not on PATH, skipping")
	}

	analyzer, err := NewTSAnalyzer()
	if err != nil {
		t.Fatalf("NewTSAnalyzer: %v", err)
	}

	dir := t.TempDir()

	writeFile(t, dir+"/tsconfig.json", `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "strict": true
  },
  "include": ["*.ts"]
}`)

	writeFile(t, dir+"/greet.ts", `export function greet(name: string): string {
  return "Hello, " + name;
}

export class Greeter {
  name: string;
  constructor(name: string) {
    this.name = name;
  }
  sayHello(): string {
    return greet(this.name);
  }
}
`)

	result, err := analyzer.Analyze(dir, "")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if len(result.Symbols) == 0 {
		t.Error("expected symbols, got none")
	}

	foundGreet := false
	foundGreeter := false
	for _, s := range result.Symbols {
		if s.Name == "greet" && s.Kind == "function" {
			foundGreet = true
		}
		if s.Name == "Greeter" && s.Kind == "class" {
			foundGreeter = true
		}
	}
	if !foundGreet {
		t.Error("missing greet function symbol")
	}
	if !foundGreeter {
		t.Error("missing Greeter class symbol")
	}
}
