package initializer

import (
	"strings"
	"testing"
)

func TestEmbeddedTemplatesContainsYardYaml(t *testing.T) {
	content, err := readEmbeddedFile("templates/init/yard.yaml.example")
	if err != nil {
		t.Fatalf("readEmbeddedFile: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "{{PROJECT_ROOT}}") {
		t.Fatalf("expected embedded yard.yaml.example to contain {{PROJECT_ROOT}} placeholder")
	}
	if !strings.Contains(text, "agent_roles:") {
		t.Fatalf("expected embedded yard.yaml.example to contain agent_roles section")
	}
	if !strings.Contains(text, "orchestrator:") {
		t.Fatalf("expected embedded yard.yaml.example to contain orchestrator role")
	}
	if !strings.Contains(text, "builtin:orchestrator") {
		t.Fatalf("expected embedded yard.yaml.example to contain builtin orchestrator prompt marker")
	}
	if strings.Contains(text, "{{SODORYARD_AGENTS_DIR}}") {
		t.Fatalf("expected embedded yard.yaml.example to avoid {{SODORYARD_AGENTS_DIR}} placeholder")
	}
}

func TestEmbeddedTemplatesContainsBrainGitkeeps(t *testing.T) {
	wantSections := []string{"architecture", "conventions", "epics", "logs", "plans", "receipts", "specs", "tasks"}
	for _, section := range wantSections {
		path := "templates/init/brain/" + section + "/.gitkeep"
		if _, err := readEmbeddedFile(path); err != nil {
			t.Errorf("expected embedded %s to exist: %v", path, err)
		}
	}
}

func TestListBrainSectionDirs(t *testing.T) {
	dirs, err := listBrainSectionDirs()
	if err != nil {
		t.Fatalf("listBrainSectionDirs: %v", err)
	}
	want := []string{"architecture", "conventions", "epics", "logs", "plans", "receipts", "specs", "tasks"}
	if len(dirs) != len(want) {
		t.Fatalf("listBrainSectionDirs returned %d dirs, want %d: %v", len(dirs), len(want), dirs)
	}
	for i, w := range want {
		if dirs[i] != w {
			t.Errorf("dirs[%d] = %q, want %q", i, dirs[i], w)
		}
	}
}
