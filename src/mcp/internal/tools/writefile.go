package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/m0n0x41d/quint-code/internal/agent"
)

// WriteFileTool creates or overwrites a file.
type WriteFileTool struct{}

type writeArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (t *WriteFileTool) Name() string { return "write" }

func (t *WriteFileTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "write",
		Description: "Write content to a file. Creates the file and parent directories if they don't exist. Overwrites if the file exists.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to write",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Content to write to the file",
				},
			},
			"required": []any{"path", "content"},
		},
	}
}

func (t *WriteFileTool) Execute(_ context.Context, argsJSON string) (string, error) {
	var args writeArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	if args.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	dir := filepath.Dir(args.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(args.Path, []byte(args.Content), 0o644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return fmt.Sprintf("Written %d bytes to %s", len(args.Content), args.Path), nil
}
