package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/agent"
)

// GlobTool finds files matching a glob pattern.
// Uses ripgrep --files when available (.gitignore aware, fast).
type GlobTool struct {
	projectRoot string
}

type globArgs struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

func (t *GlobTool) Name() string { return "glob" }

func (t *GlobTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "glob",
		Description: "Find files matching a glob pattern (e.g. '**/*.go', 'src/**/*.ts'). Returns file paths sorted by modification time (newest first). Respects .gitignore when ripgrep is available.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Glob pattern to match files (e.g. '**/*.go')",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Base directory to search in (default: project root)",
				},
			},
			"required": []any{"pattern"},
		},
	}
}

func (t *GlobTool) Execute(_ context.Context, argsJSON string) (agent.ToolResult, error) {
	var args globArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}
	if args.Pattern == "" {
		return agent.ToolResult{}, fmt.Errorf("pattern is required")
	}

	baseDir := t.projectRoot
	if args.Path != "" {
		baseDir = args.Path
	}

	// Try ripgrep first (fast, .gitignore aware)
	if rgAvailable() {
		return t.globWithRipgrep(args.Pattern, baseDir)
	}

	return t.globWithFallback(args.Pattern, baseDir)
}

func (t *GlobTool) globWithRipgrep(pattern, baseDir string) (agent.ToolResult, error) {
	files, err := rgFiles(baseDir, pattern)
	if err != nil {
		// Fallback on rg error
		return t.globWithFallback(pattern, baseDir)
	}

	if len(files) == 0 {
		return agent.PlainResult("No files matched the pattern."), nil
	}

	// Sort by mtime (newest first)
	sortByMtime(files)

	return agent.PlainResult(strings.Join(files, "\n")), nil
}

func (t *GlobTool) globWithFallback(pattern, baseDir string) (agent.ToolResult, error) {
	var matches []string
	err := filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == ".haft" {
				return filepath.SkipDir
			}
			return nil
		}
		relPath, _ := filepath.Rel(baseDir, path)
		matched, _ := filepath.Match(pattern, filepath.Base(relPath))
		if !matched {
			matched = matchDoubleStarGlob(relPath, pattern)
		}
		if matched {
			matches = append(matches, relPath)
		}
		return nil
	})
	if err != nil {
		return agent.ToolResult{}, fmt.Errorf("walk: %w", err)
	}

	if len(matches) == 0 {
		return agent.PlainResult("No files matched the pattern."), nil
	}

	// Sort by mtime (newest first)
	sortByMtime(matches)

	return agent.PlainResult(strings.Join(matches, "\n")), nil
}

// matchDoubleStarGlob handles ** patterns.
func matchDoubleStarGlob(path, pattern string) bool {
	if !strings.Contains(pattern, "**") {
		return false
	}
	parts := strings.SplitN(pattern, "**", 2)
	prefix := parts[0]
	suffix := strings.TrimLeft(parts[1], "/\\")
	if prefix != "" && !strings.HasPrefix(path, strings.TrimRight(prefix, "/\\")) {
		return false
	}
	if suffix == "" {
		return true
	}
	matched, _ := filepath.Match(suffix, filepath.Base(path))
	return matched
}

// sortByMtime sorts file paths by modification time (newest first).
func sortByMtime(files []string) {
	type fileWithTime struct {
		path string
		mod  int64
	}
	fwt := make([]fileWithTime, len(files))
	for i, f := range files {
		info, err := os.Stat(f)
		if err == nil {
			fwt[i] = fileWithTime{path: f, mod: info.ModTime().UnixNano()}
		} else {
			fwt[i] = fileWithTime{path: f, mod: 0}
		}
	}
	sort.Slice(fwt, func(i, j int) bool {
		return fwt[i].mod > fwt[j].mod
	})
	for i, f := range fwt {
		files[i] = f.path
	}
}
