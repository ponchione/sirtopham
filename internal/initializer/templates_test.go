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
	if !strings.Contains(string(content), "{{PROJECT_ROOT}}") {
		t.Fatalf("expected embedded yard.yaml.example to contain {{PROJECT_ROOT}} placeholder")
	}
	if !strings.Contains(string(content), "agent_roles:") {
		t.Fatalf("expected embedded yard.yaml.example to contain agent_roles section")
	}
	if !strings.Contains(string(content), "orchestrator:") {
		t.Fatalf("expected embedded yard.yaml.example to contain orchestrator role")
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
