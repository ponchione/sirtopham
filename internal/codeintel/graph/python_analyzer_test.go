package graph

import (
	"path/filepath"
	"testing"
)

func TestPythonAnalyzer_Analyze(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "app", "service.py"), `import os
from app.models import User

class AuthService:
    def __init__(self, db):
        self.db = db

    def authenticate(self, username, password):
        user = self.lookup(username)
        return user is not None

    def lookup(self, username):
        return self.db.find(username)

def create_service(db):
    return AuthService(db)
`)

	writeFile(t, filepath.Join(dir, "app", "models.py"), `class User:
    def __init__(self, name, email):
        self.name = name
        self.email = email

    def display_name(self):
        return self.name

def create_user(name, email):
    return User(name, email)
`)

	writeFile(t, filepath.Join(dir, "app", "__init__.py"), "")

	analyzer := NewPythonAnalyzer(dir, nil, nil)
	result, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	symByID := make(map[string]Symbol)
	for _, s := range result.Symbols {
		symByID[s.ID] = s
	}

	if _, ok := symByID["py:app.service:class:AuthService"]; !ok {
		t.Errorf("missing AuthService; have: %v", symIDs(result.Symbols))
	}
	if _, ok := symByID["py:app.models:class:User"]; !ok {
		t.Errorf("missing User")
	}
	if _, ok := symByID["py:app.service:method:AuthService.authenticate"]; !ok {
		t.Errorf("missing authenticate method")
	}
	if _, ok := symByID["py:app.service:function:create_service"]; !ok {
		t.Errorf("missing create_service")
	}

	if len(result.Edges) == 0 {
		t.Error("expected edges, got none")
	}

	foundImport := false
	for _, e := range result.Edges {
		if e.EdgeType == "IMPORTS" && e.TargetID == "py:app.models:module:app.models" {
			foundImport = true
		}
	}
	if !foundImport {
		t.Error("missing IMPORTS edge: service -> models")
	}

	authMethodID := "py:app.service:method:AuthService.authenticate"
	lookupID := "py:app.service:method:AuthService.lookup"
	foundSelfCall := false
	for _, e := range result.Edges {
		if e.EdgeType == "CALLS" && e.SourceID == authMethodID && e.TargetID == lookupID {
			foundSelfCall = true
		}
	}
	if !foundSelfCall {
		t.Error("missing CALLS edge: authenticate -> self.lookup")
	}
}

func TestPythonAnalyzer_EmptyProject(t *testing.T) {
	dir := t.TempDir()
	analyzer := NewPythonAnalyzer(dir, nil, nil)
	result, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Symbols) != 0 {
		t.Errorf("expected 0 symbols, got %d", len(result.Symbols))
	}
}

func TestPythonAnalyzer_ModulePath(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"lib/auth/service.py", "lib.auth.service"},
		{"main.py", "main"},
		{"lib/__init__.py", "lib"},
	}
	for _, tt := range tests {
		got := pythonModulePath(tt.input)
		if got != tt.want {
			t.Errorf("pythonModulePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPythonAnalyzer_SymbolProperties(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mod.py"), `class _Private:
    pass

class Public:
    def _hidden(self):
        pass

    def visible(self):
        pass

def _internal():
    pass

def exported():
    pass
`)

	analyzer := NewPythonAnalyzer(dir, nil, nil)
	result, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	symByName := make(map[string]Symbol)
	for _, s := range result.Symbols {
		symByName[s.Name] = s
	}

	if s := symByName["Public"]; !s.Exported {
		t.Error("Public should be exported")
	}
	if s := symByName["_Private"]; s.Exported {
		t.Error("_Private should not be exported")
	}
	if s := symByName["exported"]; !s.Exported {
		t.Error("exported should be exported")
	}
	if s := symByName["_internal"]; s.Exported {
		t.Error("_internal should not be exported")
	}
}

func symIDs(syms []Symbol) []string {
	ids := make([]string, len(syms))
	for i, s := range syms {
		ids[i] = s.ID
	}
	return ids
}
