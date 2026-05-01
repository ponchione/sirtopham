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

var roleToPersona = map[string]string{
	"orchestrator":        "Sir Topham Hatt",
	"planner":             "Gordon",
	"epic-decomposer":     "Edward",
	"task-decomposer":     "Emily",
	"coder":               "Thomas",
	"correctness-auditor": "Percy",
	"quality-auditor":     "James",
	"performance-auditor": "Spencer",
	"security-auditor":    "Diesel",
	"integration-auditor": "Toby",
	"test-writer":         "Rosie",
	"resolver":            "Victor",
	"docs-arbiter":        "Harold",
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

func PersonaName(role string) (string, bool) {
	name, ok := roleToPersona[strings.TrimSpace(role)]
	return name, ok
}

func PersonaAliases(role string) []string {
	role = strings.TrimSpace(role)
	aliases := []string{}
	if persona, ok := PersonaName(role); ok {
		aliases = append(aliases, persona)
	}
	if filename, ok := roleToAsset[role]; ok {
		base := strings.TrimSuffix(filename, ".md")
		if base != "" && !containsAlias(aliases, base) {
			aliases = append(aliases, base)
		}
	}
	return aliases
}

func Keys() []string {
	keys := make([]string, 0, len(roleToAsset))
	for key := range roleToAsset {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func containsAlias(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
