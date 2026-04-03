package server

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/ponchione/sirtopham/internal/config"
	"github.com/ponchione/sirtopham/internal/pathglob"
)

// ProjectHandler serves project info, file tree, and file content endpoints.
type ProjectHandler struct {
	cfg    *config.Config
	logger *slog.Logger

	langOnce sync.Once
	langVal  string // cached primary language
}

// NewProjectHandler creates a handler and registers routes on the server.
func NewProjectHandler(s *Server, cfg *config.Config, logger *slog.Logger) *ProjectHandler {
	h := &ProjectHandler{cfg: cfg, logger: logger}

	s.HandleFunc("GET /api/project", h.handleProject)
	s.HandleFunc("GET /api/project/tree", h.handleTree)
	s.HandleFunc("GET /api/project/file", h.handleFile)

	return h
}

// ── GET /api/project ─────────────────────────────────────────────────

type projectInfoResponse struct {
	RootPath string `json:"root_path"`
	Language string `json:"language,omitempty"`
	Name     string `json:"name"`
}

func (h *ProjectHandler) handleProject(w http.ResponseWriter, _ *http.Request) {
	name := filepath.Base(h.cfg.ProjectRoot)

	h.langOnce.Do(func() {
		h.langVal = detectPrimaryLanguage(h.cfg.ProjectRoot, h.cfg.Index.Include)
		h.logger.Info("cached primary language", "language", h.langVal)
	})

	writeJSON(w, http.StatusOK, projectInfoResponse{
		RootPath: h.cfg.ProjectRoot,
		Language: h.langVal,
		Name:     name,
	})
}

// ── GET /api/project/tree ────────────────────────────────────────────

type treeNode struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"` // "dir" or "file"
	Children []treeNode `json:"children,omitempty"`
}

func (h *ProjectHandler) handleTree(w http.ResponseWriter, r *http.Request) {
	maxDepth := 3
	if d := r.URL.Query().Get("depth"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed >= 1 && parsed <= 10 {
			maxDepth = parsed
		}
	}

	root := h.cfg.ProjectRoot
	excludes := h.cfg.Index.Exclude

	tree := buildTree(root, root, excludes, 0, maxDepth)
	writeJSON(w, http.StatusOK, tree)
}

// ── GET /api/project/file ────────────────────────────────────────────

type fileResponse struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Language  string `json:"language"`
	LineCount int    `json:"line_count"`
}

func (h *ProjectHandler) handleFile(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'path' is required")
		return
	}

	// Path traversal protection.
	if strings.Contains(relPath, "..") || filepath.IsAbs(relPath) {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	absPath := filepath.Join(h.cfg.ProjectRoot, relPath)

	// Ensure the resolved path is within project root.
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	rootResolved, _ := filepath.EvalSymlinks(h.cfg.ProjectRoot)
	if !strings.HasPrefix(resolved, rootResolved+string(filepath.Separator)) && resolved != rootResolved {
		writeError(w, http.StatusBadRequest, "path outside project root")
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if info.IsDir() {
		writeError(w, http.StatusBadRequest, "path is a directory")
		return
	}
	if info.Size() > 1<<20 { // 1MB limit
		writeError(w, http.StatusRequestEntityTooLarge, "file too large (>1MB)")
		return
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		h.logger.Error("read file", "error", err, "path", absPath)
		writeError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	lang := langFromExtension(filepath.Ext(relPath))
	lines := strings.Count(string(data), "\n") + 1

	writeJSON(w, http.StatusOK, fileResponse{
		Path:      relPath,
		Content:   string(data),
		Language:  lang,
		LineCount: lines,
	})
}

// ── Helpers ──────────────────────────────────────────────────────────

func buildTree(root, dir string, excludes []string, depth, maxDepth int) treeNode {
	name := filepath.Base(dir)
	if depth == 0 {
		name = "."
	}

	node := treeNode{Name: name, Type: "dir"}

	if depth >= maxDepth {
		return node
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return node
	}

	for _, entry := range entries {
		entryName := entry.Name()
		relPath, _ := filepath.Rel(root, filepath.Join(dir, entryName))

		if shouldExclude(relPath, entryName, excludes) {
			continue
		}

		if entry.IsDir() {
			child := buildTree(root, filepath.Join(dir, entryName), excludes, depth+1, maxDepth)
			node.Children = append(node.Children, child)
		} else {
			node.Children = append(node.Children, treeNode{Name: entryName, Type: "file"})
		}
	}

	return node
}

func shouldExclude(relPath, name string, excludes []string) bool {
	// Always exclude hidden dirs/files starting with .
	if strings.HasPrefix(name, ".") {
		return true
	}

	for _, pattern := range excludes {
		if pathglob.Match(pattern, relPath) || pathglob.Match(pattern, name) {
			return true
		}
	}
	return false
}

func detectPrimaryLanguage(root string, includes []string) string {
	// Count extensions from the include patterns.
	counts := map[string]int{}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if ext != "" {
			counts[ext]++
		}
		return nil
	})

	// Find the most common code extension.
	langMap := map[string]string{
		".go":   "go",
		".py":   "python",
		".js":   "javascript",
		".ts":   "typescript",
		".tsx":  "typescript",
		".jsx":  "javascript",
		".rs":   "rust",
		".java": "java",
		".rb":   "ruby",
		".c":    "c",
		".cpp":  "cpp",
		".cs":   "csharp",
	}

	bestLang := ""
	bestCount := 0
	for ext, count := range counts {
		if lang, ok := langMap[ext]; ok && count > bestCount {
			bestCount = count
			bestLang = lang
		}
	}
	return bestLang
}

func langFromExtension(ext string) string {
	m := map[string]string{
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
	if lang, ok := m[strings.ToLower(ext)]; ok {
		return lang
	}
	return "text"
}
