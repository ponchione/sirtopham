package langutil

import "strings"

var extensionToLanguage = map[string]string{
	".go":   "go",
	".py":   "python",
	".js":   "javascript",
	".ts":   "typescript",
	".tsx":  "tsx",
	".jsx":  "jsx",
	".rs":   "rust",
	".java": "java",
	".rb":   "ruby",
	".c":    "c",
	".cpp":  "cpp",
	".cs":   "csharp",
	".h":    "c",
	".sql":  "sql",
	".md":   "markdown",
	".json": "json",
	".yaml": "yaml",
	".yml":  "yaml",
	".toml": "toml",
	".html": "html",
	".css":  "css",
	".sh":   "shell",
	".bash": "shell",
}

func FromExtension(ext string) (string, bool) {
	lang, ok := extensionToLanguage[strings.ToLower(ext)]
	return lang, ok
}

func FromExtensionOr(ext string, fallback string) string {
	if lang, ok := FromExtension(ext); ok {
		return lang
	}
	return fallback
}
