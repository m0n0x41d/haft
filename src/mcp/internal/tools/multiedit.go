package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/m0n0x41d/quint-code/internal/agent"
)

// MultiEditTool applies multiple edits to a single file atomically.
// All edits are validated before any are applied — if one fails, none apply.
// Edits are applied in reverse order (bottom-up) to preserve line numbers.
type MultiEditTool struct {
	registry *Registry
}

type multiEditArgs struct {
	Path  string     `json:"path"`
	Edits []editPair `json:"edits"`
}

type editPair struct {
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

func (t *MultiEditTool) Name() string { return "multiedit" }

func (t *MultiEditTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "multiedit",
		Description: "Apply multiple edits to a single file atomically. All edits are validated before any are applied — if one edit fails to match, none are applied. Use this instead of multiple edit calls when making several changes to one file.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to edit",
				},
				"edits": map[string]any{
					"type":        "array",
					"description": "Array of {old_string, new_string} pairs to apply",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"old_string": map[string]any{"type": "string", "description": "Exact text to find"},
							"new_string": map[string]any{"type": "string", "description": "Replacement text"},
						},
						"required": []any{"old_string", "new_string"},
					},
				},
			},
			"required": []any{"path", "edits"},
		},
	}
}

func (t *MultiEditTool) Execute(_ context.Context, argsJSON string) (agent.ToolResult, error) {
	var args multiEditArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}
	if args.Path == "" {
		return agent.ToolResult{}, fmt.Errorf("path is required")
	}
	if len(args.Edits) == 0 {
		return agent.ToolResult{}, fmt.Errorf("at least one edit is required")
	}

	// Read-before-edit enforcement
	if t.registry != nil && !t.registry.WasFileRead(args.Path) {
		return agent.PlainResult(fmt.Sprintf(
			"Read the file first before editing. Use: read(file_path=\"%s\")", args.Path)), nil
	}

	content, err := os.ReadFile(args.Path)
	if err != nil {
		return agent.ToolResult{}, fmt.Errorf("read file: %w", err)
	}
	original := string(content)

	// Phase 1: Validate ALL edits match before applying any
	for i, edit := range args.Edits {
		if edit.OldString == "" {
			return agent.PlainResult(fmt.Sprintf("Edit %d: old_string is empty", i+1)), nil
		}
		if !strings.Contains(original, edit.OldString) {
			// Try fuzzy match
			if _, found := fuzzyMatch(original, edit.OldString); !found {
				return agent.PlainResult(fmt.Sprintf(
					"Edit %d failed: old_string not found in file. No edits applied.\nMake sure it matches exactly. Re-read the file to see current content.",
					i+1)), nil
			}
		}
	}

	// Phase 2: Apply edits in reverse order (bottom-up preserves positions)
	result := original
	applied := 0
	for i := len(args.Edits) - 1; i >= 0; i-- {
		edit := args.Edits[i]
		if strings.Contains(result, edit.OldString) {
			result = strings.Replace(result, edit.OldString, edit.NewString, 1)
			applied++
		} else if matched, found := fuzzyMatch(result, edit.OldString); found {
			result = strings.Replace(result, matched, edit.NewString, 1)
			applied++
		}
	}

	if err := os.WriteFile(args.Path, []byte(result), 0o644); err != nil {
		return agent.ToolResult{}, fmt.Errorf("write file: %w", err)
	}

	return agent.PlainResult(fmt.Sprintf("Applied %d/%d edits to %s", applied, len(args.Edits), args.Path)), nil
}
