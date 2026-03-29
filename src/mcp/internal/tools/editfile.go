package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/m0n0x41d/quint-code/internal/agent"
)

// EditFileTool replaces exact text in a file.
type EditFileTool struct{}

type editArgs struct {
	Path      string `json:"path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

func (t *EditFileTool) Name() string { return "edit" }

func (t *EditFileTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "edit",
		Description: "Edit a file by replacing an exact string match. The old_string must match exactly (including whitespace and indentation). The replacement is applied once.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to edit",
				},
				"old_string": map[string]any{
					"type":        "string",
					"description": "Exact text to find in the file",
				},
				"new_string": map[string]any{
					"type":        "string",
					"description": "Text to replace old_string with",
				},
			},
			"required": []any{"path", "old_string", "new_string"},
		},
	}
}

func (t *EditFileTool) Execute(_ context.Context, argsJSON string) (string, error) {
	var args editArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	if args.Path == "" {
		return "", fmt.Errorf("path is required")
	}
	if args.OldString == "" {
		return "", fmt.Errorf("old_string is required")
	}

	content, err := os.ReadFile(args.Path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	original := string(content)
	if !strings.Contains(original, args.OldString) {
		return "old_string not found in file. Make sure it matches exactly, including whitespace and indentation.", nil
	}

	updated := strings.Replace(original, args.OldString, args.NewString, 1)
	if err := os.WriteFile(args.Path, []byte(updated), 0o644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	// Show colored diff with -/+ prefixes (rendered by TUI)
	oldLines := strings.Split(args.OldString, "\n")
	newLines := strings.Split(args.NewString, "\n")

	var diff strings.Builder
	diff.WriteString(fmt.Sprintf("Edited %s (-%d +%d lines)\n", args.Path, len(oldLines), len(newLines)))
	diff.WriteString("--- old\n")
	for _, line := range oldLines {
		if len(line) > 200 {
			line = line[:200] + "…"
		}
		diff.WriteString("-" + line + "\n")
	}
	diff.WriteString("+++ new\n")
	for _, line := range newLines {
		if len(line) > 200 {
			line = line[:200] + "…"
		}
		diff.WriteString("+" + line + "\n")
	}

	return strings.TrimRight(diff.String(), "\n"), nil
}
