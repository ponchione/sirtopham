package graph

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// writeFile is a test helper that creates a file with content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestGoAnalyzer_Analyze(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/testmod\n\ngo 1.21\n")

	writeFile(t, filepath.Join(dir, "pkg", "types.go"), `package pkg

type Greeter interface {
	Greet() string
}

type Base struct {
	Name string
}
`)

	writeFile(t, filepath.Join(dir, "pkg", "impl.go"), `package pkg

import "fmt"

type Hello struct {
	Base
}

func (h Hello) Greet() string {
	return fmt.Sprintf("Hello, %s", h.Name)
}

func NewHello(name string) *Hello {
	return &Hello{Base: Base{Name: name}}
}
`)

	analyzer, err := NewGoAnalyzer(dir)
	if err != nil {
		t.Fatalf("NewGoAnalyzer: %v", err)
	}

	result, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	symByID := make(map[string]Symbol)
	for _, s := range result.Symbols {
		symByID[s.ID] = s
	}

	greeterID := "go:example.com/testmod/pkg:interface:Greeter"
	if _, ok := symByID[greeterID]; !ok {
		t.Errorf("missing symbol %s", greeterID)
	}

	baseID := "go:example.com/testmod/pkg:type:Base"
	if _, ok := symByID[baseID]; !ok {
		t.Errorf("missing symbol %s", baseID)
	}

	helloID := "go:example.com/testmod/pkg:type:Hello"
	if _, ok := symByID[helloID]; !ok {
		t.Errorf("missing symbol %s", helloID)
	}

	greetID := "go:example.com/testmod/pkg:method:Hello.Greet"
	if _, ok := symByID[greetID]; !ok {
		t.Errorf("missing symbol %s", greetID)
	}

	newHelloID := "go:example.com/testmod/pkg:function:NewHello"
	if _, ok := symByID[newHelloID]; !ok {
		t.Errorf("missing symbol %s", newHelloID)
	}

	edgesByType := make(map[string][]Edge)
	for _, e := range result.Edges {
		edgesByType[e.EdgeType] = append(edgesByType[e.EdgeType], e)
	}

	foundImpl := false
	for _, e := range edgesByType["IMPLEMENTS"] {
		if e.SourceID == helloID && e.TargetID == greeterID {
			foundImpl = true
		}
	}
	if !foundImpl {
		t.Error("missing IMPLEMENTS edge: Hello -> Greeter")
	}

	foundEmbed := false
	for _, e := range edgesByType["EMBEDS"] {
		if e.SourceID == helloID && e.TargetID == baseID {
			foundEmbed = true
		}
	}
	if !foundEmbed {
		t.Error("missing EMBEDS edge: Hello -> Base")
	}

	foundCall := false
	for _, e := range edgesByType["CALLS"] {
		if e.SourceID == greetID && e.TargetID == "go:fmt:function:Sprintf" {
			foundCall = true
		}
	}
	if !foundCall {
		t.Error("missing CALLS edge: Hello.Greet -> fmt.Sprintf")
	}

	foundBoundary := false
	for _, b := range result.BoundarySymbols {
		if b.Package == "fmt" {
			foundBoundary = true
		}
	}
	if !foundBoundary {
		t.Error("missing boundary symbol for fmt")
	}
}

func TestGoAnalyzer_SymbolProperties(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/proptest\n\ngo 1.21\n")
	writeFile(t, filepath.Join(dir, "main.go"), `package main

func publicFunc() {}
func privateFunc() {}

type MyStruct struct{}

func (m *MyStruct) DoThing() {}
`)

	analyzer, err := NewGoAnalyzer(dir)
	if err != nil {
		t.Fatalf("NewGoAnalyzer: %v", err)
	}

	result, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	symByName := make(map[string]Symbol)
	for _, s := range result.Symbols {
		symByName[s.Name] = s
	}

	if s, ok := symByName["DoThing"]; !ok {
		t.Error("missing DoThing")
	} else {
		if !s.Exported { t.Error("DoThing should be exported") }
		if s.Kind != "method" { t.Errorf("DoThing kind = %q, want method", s.Kind) }
		if s.Receiver != "MyStruct" { t.Errorf("DoThing receiver = %q, want MyStruct", s.Receiver) }
		if s.Language != "go" { t.Errorf("DoThing language = %q, want go", s.Language) }
	}

	if s, ok := symByName["privateFunc"]; !ok {
		t.Error("missing privateFunc")
	} else if s.Exported {
		t.Error("privateFunc should not be exported")
	}
}

func TestGoAnalyzer_EmptyModule(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/empty\n\ngo 1.21\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n")

	analyzer, err := NewGoAnalyzer(dir)
	if err != nil {
		t.Fatalf("NewGoAnalyzer: %v", err)
	}

	result, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if len(result.Symbols) != 0 {
		names := make([]string, len(result.Symbols))
		for i, s := range result.Symbols { names[i] = s.Name }
		sort.Strings(names)
		t.Errorf("expected 0 symbols, got %d: %v", len(result.Symbols), names)
	}
}
