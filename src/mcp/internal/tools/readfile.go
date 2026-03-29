package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/m0n0x41d/quint-code/internal/agent"
)

// ReadFileTool reads file contents with line numbers.
type ReadFileTool struct{}

type readArgs struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"` // 1-based line number to start from
	Limit  int    `json:"limit,omitempty"`  // max lines to return (0 = all)
}

func (t *ReadFileTool) Name() string { return "read" }

func (t *ReadFileTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "read",
		Description: "Read a file's contents with line numbers. Supports offset (start line, 1-based) and limit (max lines) for large files.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to read (absolute or relative to project root)",
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "Line number to start reading from (1-based, default: 1)",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of lines to return (default: 2000)",
				},
			},
			"required": []any{"path"},
		},
	}
}

func (t *ReadFileTool) Execute(_ context.Context, argsJSON string) (string, error) {
	var args readArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	if args.Path == "" {
		return "", fmt.Errorf("path is required")
	}
	if args.Limit <= 0 {
		args.Limit = 2000
	}
	if args.Offset <= 0 {
		args.Offset = 1
	}

	content, err := os.ReadFile(args.Path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	startIdx := args.Offset - 1
	if startIdx >= len(lines) {
		return fmt.Sprintf("File has %d lines, offset %d is past end", len(lines), args.Offset), nil
	}

	endIdx := startIdx + args.Limit
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	var b strings.Builder
	for i := startIdx; i < endIdx; i++ {
		fmt.Fprintf(&b, "%6d\t%s\n", i+1, lines[i])
	}

	if endIdx < len(lines) {
		fmt.Fprintf(&b, "\n... (%d more lines)", len(lines)-endIdx)
	}

	return b.String(), nil
}
