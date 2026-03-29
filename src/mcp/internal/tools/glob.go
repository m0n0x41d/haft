package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/m0n0x41d/quint-code/internal/agent"
)

// GlobTool finds files matching a glob pattern.
type GlobTool struct {
	projectRoot string
}

type globArgs struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"` // base directory (default: project root)
}

func (t *GlobTool) Name() string { return "glob" }

func (t *GlobTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "glob",
		Description: "Find files matching a glob pattern (e.g. '**/*.go', 'src/**/*.ts'). Returns matching file paths sorted alphabetically.",
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

	var matches []string
	err := filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == ".quint" {
				return filepath.SkipDir
			}
			return nil
		}
		relPath, _ := filepath.Rel(baseDir, path)
		matched, _ := filepath.Match(args.Pattern, filepath.Base(relPath))
		if !matched {
			matched = matchDoubleStarGlob(relPath, args.Pattern)
		}
		if matched {
			matches = append(matches, relPath)
		}
		return nil
	})
	if err != nil {
		return agent.ToolResult{}, fmt.Errorf("walk: %w", err)
	}

	sort.Strings(matches)

	if len(matches) == 0 {
		return agent.PlainResult("No files matched the pattern."), nil
	}
	return agent.PlainResult(strings.Join(matches, "\n")), nil
}

// matchDoubleStarGlob handles ** patterns by splitting on ** and matching segments.
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
