package embeddedprompts

import (
	"embed"
	"sort"
	"strings"
)

//go:embed assets/*.md
var promptFS embed.FS

var roleToAsset = map[string]string{
	"orchestrator":        "sirtophamhatt.md",
	"planner":             "gordon.md",
	"epic-decomposer":     "edward.md",
	"task-decomposer":     "emily.md",
	"coder":               "thomas.md",
	"correctness-auditor": "percy.md",
	"quality-auditor":     "james.md",
	"performance-auditor": "spencer.md",
	"security-auditor":    "diesel.md",
	"integration-auditor": "toby.md",
	"test-writer":         "rosie.md",
	"resolver":            "victor.md",
	"docs-arbiter":        "harold.md",
}

func Get(role string) (string, bool) {
	filename, ok := roleToAsset[strings.TrimSpace(role)]
	if !ok {
		return "", false
	}
	data, err := promptFS.ReadFile("assets/" + filename)
	if err != nil {
		return "", false
	}
	return string(data), true
}

func Has(role string) bool {
	_, ok := roleToAsset[strings.TrimSpace(role)]
	return ok
}

func Keys() []string {
	keys := make([]string, 0, len(roleToAsset))
	for key := range roleToAsset {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
