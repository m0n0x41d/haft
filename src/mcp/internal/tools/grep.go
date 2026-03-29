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
// Uses ripgrep when available (10-100x faster, .gitignore aware, multiline).
// Falls back to pure Go implementation when rg is not on PATH.
type GrepTool struct {
	projectRoot string
}

type grepArgs struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path,omitempty"`
	Glob       string `json:"glob,omitempty"`
	FileType   string `json:"type,omitempty"`
	OutputMode string `json:"output_mode,omitempty"` // "content" | "files_with_matches" | "count"
	ContextA   int    `json:"-A,omitempty"`
	ContextB   int    `json:"-B,omitempty"`
	ContextC   int    `json:"-C,omitempty"`
	Context    int    `json:"context,omitempty"` // alias for -C
	Multiline  bool   `json:"multiline,omitempty"`
	CaseInsens bool   `json:"-i,omitempty"`
	HeadLimit  int    `json:"head_limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	LineNums   *bool  `json:"-n,omitempty"` // show line numbers (default true for content)
}

func (t *GrepTool) Name() string { return "grep" }

func (t *GrepTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "grep",
		Description: "Search file contents using regex. Powered by ripgrep with Go fallback.\nOutput modes: \"content\" shows matching lines, \"files_with_matches\" shows only paths (default), \"count\" shows match counts.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern":     map[string]any{"type": "string", "description": "Regex pattern to search for"},
				"path":        map[string]any{"type": "string", "description": "File or directory to search (default: project root)"},
				"glob":        map[string]any{"type": "string", "description": "File glob filter (e.g. '*.go', '*.{ts,tsx}')"},
				"type":        map[string]any{"type": "string", "description": "File type filter (go, py, js, ts, rust, etc.)"},
				"output_mode": map[string]any{"type": "string", "enum": []string{"content", "files_with_matches", "count"}, "description": "Output format (default: files_with_matches)"},
				"multiline":   map[string]any{"type": "boolean", "description": "Match across line boundaries"},
				"-A":          map[string]any{"type": "integer", "description": "Lines after each match (content mode)"},
				"-B":          map[string]any{"type": "integer", "description": "Lines before each match (content mode)"},
				"-C":          map[string]any{"type": "integer", "description": "Context lines around each match (content mode)"},
				"context":     map[string]any{"type": "integer", "description": "Alias for -C"},
				"-i":          map[string]any{"type": "boolean", "description": "Case insensitive search"},
				"-n":          map[string]any{"type": "boolean", "description": "Show line numbers (default true for content mode)"},
				"head_limit":  map[string]any{"type": "integer", "description": "Max results to return (default: unlimited)"},
				"offset":      map[string]any{"type": "integer", "description": "Skip first N results"},
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

	// Default output mode
	if args.OutputMode == "" {
		args.OutputMode = "files_with_matches"
	}

	// Context alias
	if args.Context > 0 && args.ContextC == 0 {
		args.ContextC = args.Context
	}

	searchPath := t.projectRoot
	if args.Path != "" {
		searchPath = args.Path
	}

	// Try ripgrep first
	if rgAvailable() {
		return t.executeWithRipgrep(args, searchPath)
	}

	// Fallback to pure Go
	return t.executeWithFallback(args, searchPath)
}

func (t *GrepTool) executeWithRipgrep(args grepArgs, searchPath string) (agent.ToolResult, error) {
	params := rgSearchParams{
		Pattern:    args.Pattern,
		Path:       searchPath,
		Glob:       args.Glob,
		FileType:   args.FileType,
		OutputMode: args.OutputMode,
		ContextA:   args.ContextA,
		ContextB:   args.ContextB,
		ContextC:   args.ContextC,
		Multiline:  args.Multiline,
		CaseInsens: args.CaseInsens,
		HeadLimit:  args.HeadLimit,
		Offset:     args.Offset,
	}

	matches, err := rgSearch(params)
	if err != nil {
		return agent.ToolResult{}, err
	}

	output := formatRgResults(matches, args.OutputMode, args.HeadLimit, args.Offset)
	return agent.PlainResult(output), nil
}

func (t *GrepTool) executeWithFallback(args grepArgs, searchPath string) (agent.ToolResult, error) {
	re, err := regexp.Compile(args.Pattern)
	if err != nil {
		return agent.ToolResult{}, fmt.Errorf("invalid regex: %w", err)
	}

	info, err := os.Stat(searchPath)
	if err != nil {
		return agent.ToolResult{}, fmt.Errorf("stat %s: %w", searchPath, err)
	}

	limit := 200
	if args.HeadLimit > 0 {
		limit = args.HeadLimit
	}

	var results []string

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
			matches := grepFile(path, re, limit-len(results))
			results = append(results, matches...)
			if len(results) >= limit {
				return filepath.SkipAll
			}
			return nil
		})
	} else {
		results = grepFile(searchPath, re, limit)
	}

	// Apply offset
	if args.Offset > 0 && args.Offset < len(results) {
		results = results[args.Offset:]
	}

	if len(results) == 0 {
		return agent.PlainResult("No matches found."), nil
	}

	// Format based on output mode
	switch args.OutputMode {
	case "files_with_matches":
		seen := make(map[string]bool)
		var files []string
		for _, r := range results {
			path := strings.SplitN(r, ":", 2)[0]
			if !seen[path] {
				seen[path] = true
				files = append(files, path)
			}
		}
		return agent.PlainResult(strings.Join(files, "\n")), nil

	case "count":
		counts := make(map[string]int)
		var order []string
		for _, r := range results {
			path := strings.SplitN(r, ":", 2)[0]
			if _, ok := counts[path]; !ok {
				order = append(order, path)
			}
			counts[path]++
		}
		var lines []string
		for _, path := range order {
			lines = append(lines, fmt.Sprintf("%s:%d", path, counts[path]))
		}
		return agent.PlainResult(strings.Join(lines, "\n")), nil

	default: // "content"
		output := strings.Join(results, "\n")
		if len(results) >= limit {
			output += fmt.Sprintf("\n\n... (truncated at %d matches)", limit)
		}
		return agent.PlainResult(output), nil
	}
}

func grepFile(path string, re *regexp.Regexp, limit int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	// Binary detection: read first 512 bytes
	probe := make([]byte, 512)
	n, _ := f.Read(probe)
	for i := 0; i < n; i++ {
		if probe[i] == 0 {
			return nil // binary file
		}
	}
	// Seek back to start
	if _, err := f.Seek(0, 0); err != nil {
		return nil
	}

	var matches []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
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
