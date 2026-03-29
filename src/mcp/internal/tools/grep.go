package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/m0n0x41d/quint-code/internal/agent"
)

// GrepTool searches file contents with regex.
type GrepTool struct {
	projectRoot string
}

type grepArgs struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"` // file or directory (default: project root)
	Glob    string `json:"glob,omitempty"` // file glob filter (e.g. "*.go")
}

func (t *GrepTool) Name() string { return "grep" }

func (t *GrepTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "grep",
		Description: "Search file contents using regex. Returns matching lines with file paths and line numbers. Searches recursively in directories.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Regular expression pattern to search for",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "File or directory to search (default: project root)",
				},
				"glob": map[string]any{
					"type":        "string",
					"description": "File glob filter (e.g. '*.go', '*.ts')",
				},
			},
			"required": []any{"pattern"},
		},
	}
}

func (t *GrepTool) Execute(_ context.Context, argsJSON string) (agent.ToolResult, error) {
	var args grepArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}
	if args.Pattern == "" {
		return agent.ToolResult{}, fmt.Errorf("pattern is required")
	}

	re, err := regexp.Compile(args.Pattern)
	if err != nil {
		return agent.ToolResult{}, fmt.Errorf("invalid regex: %w", err)
	}

	searchPath := t.projectRoot
	if args.Path != "" {
		searchPath = args.Path
	}

	info, err := os.Stat(searchPath)
	if err != nil {
		return agent.ToolResult{}, fmt.Errorf("stat %s: %w", searchPath, err)
	}

	var results []string
	const maxResults = 200

	if info.IsDir() {
		_ = filepath.WalkDir(searchPath, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				if d != nil && d.IsDir() {
					name := d.Name()
					if name == ".git" || name == "node_modules" || name == ".quint" {
						return filepath.SkipDir
					}
				}
				return nil
			}
			if args.Glob != "" {
				matched, _ := filepath.Match(args.Glob, filepath.Base(path))
				if !matched {
					return nil
				}
			}
			matches := grepFile(path, re, maxResults-len(results))
			results = append(results, matches...)
			if len(results) >= maxResults {
				return filepath.SkipAll
			}
			return nil
		})
	} else {
		results = grepFile(searchPath, re, maxResults)
	}

	if len(results) == 0 {
		return agent.PlainResult("No matches found."), nil
	}

	output := strings.Join(results, "\n")
	if len(results) >= maxResults {
		output += fmt.Sprintf("\n\n... (truncated at %d matches)", maxResults)
	}
	return agent.PlainResult(output), nil
}

func grepFile(path string, re *regexp.Regexp, limit int) []string {
	// Skip binary files by checking first 512 bytes
	probe, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if len(probe) > 512 {
		probe = probe[:512]
	}
	for _, b := range probe {
		if b == 0 {
			return nil // binary file
		}
	}

	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var matches []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB line limit
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			matches = append(matches, fmt.Sprintf("%s:%d:%s", path, lineNum, line))
			if len(matches) >= limit {
				break
			}
		}
	}
	return matches
}
