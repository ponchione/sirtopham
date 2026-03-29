package goparser

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ponchione/sirtopham/internal/codeintel"
)

// repoRoot returns the root of the sirtopham repository.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	// thisFile is internal/codeintel/goparser/goparser_test.go → root is ../../../
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	return abs
}

func TestNew(t *testing.T) {
	root := repoRoot(t)
	p, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p == nil {
		t.Fatal("parser is nil")
	}
}

func TestParse_GoFile(t *testing.T) {
	root := repoRoot(t)
	p, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Parse the codeintel types.go file — should find FuncRef, RawChunk, Chunk, etc.
	typesPath := filepath.Join(root, "internal", "codeintel", "types.go")
	content, err := os.ReadFile(typesPath)
	if err != nil {
		t.Fatalf("read types.go: %v", err)
	}

	chunks, err := p.Parse(typesPath, content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("expected chunks from types.go, got 0")
	}

	// Should find the FuncRef type.
	found := false
	for _, c := range chunks {
		if c.Name == "FuncRef" && c.ChunkType == codeintel.ChunkTypeType {
			found = true
			break
		}
	}
	if !found {
		t.Error("FuncRef type not found in parsed chunks")
	}
}

func TestParse_FunctionWithCalls(t *testing.T) {
	root := repoRoot(t)
	p, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Parse hash.go — ChunkID calls sha256.Sum256 and fmt.Sprintf.
	hashPath := filepath.Join(root, "internal", "codeintel", "hash.go")
	content, err := os.ReadFile(hashPath)
	if err != nil {
		t.Fatalf("read hash.go: %v", err)
	}

	chunks, err := p.Parse(hashPath, content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var chunkID *codeintel.RawChunk
	for i := range chunks {
		if chunks[i].Name == "ChunkID" {
			chunkID = &chunks[i]
			break
		}
	}
	if chunkID == nil {
		t.Fatal("ChunkID function not found")
	}

	if chunkID.ChunkType != codeintel.ChunkTypeFunction {
		t.Errorf("ChunkID type = %q, want %q", chunkID.ChunkType, codeintel.ChunkTypeFunction)
	}

	// Should have calls.
	if len(chunkID.Calls) == 0 {
		t.Error("expected ChunkID to have non-empty Calls")
	}

	// Should have imports.
	if len(chunkID.Imports) == 0 {
		t.Error("expected ChunkID to have non-empty Imports")
	}
}

func TestParse_NonGoFile(t *testing.T) {
	root := repoRoot(t)
	p, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	chunks, err := p.Parse("readme.md", []byte("# Hello\nworld"))
	if err != nil {
		t.Fatalf("Parse non-Go: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for non-Go file, got %d", len(chunks))
	}
}

func TestParse_MethodDetection(t *testing.T) {
	root := repoRoot(t)
	p, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Parse config.go — has methods like Validate(), normalize(), etc.
	cfgPath := filepath.Join(root, "internal", "config", "config.go")
	content, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config.go: %v", err)
	}

	chunks, err := p.Parse(cfgPath, content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var foundMethod bool
	for _, c := range chunks {
		if c.ChunkType == codeintel.ChunkTypeMethod {
			foundMethod = true
			break
		}
	}
	if !foundMethod {
		t.Error("expected at least one method chunk from config.go")
	}
}

func TestParserImplementsInterface(t *testing.T) {
	var _ codeintel.Parser = (*Parser)(nil)
}
