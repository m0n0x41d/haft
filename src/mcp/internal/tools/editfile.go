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
// Enforces read-before-edit: file must be read in the current session before editing.
type EditFileTool struct {
	registry *Registry // for read-before-edit check
}

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

func (t *EditFileTool) Execute(_ context.Context, argsJSON string) (agent.ToolResult, error) {
	var args editArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}
	if args.Path == "" {
		return agent.ToolResult{}, fmt.Errorf("path is required")
	}
	if args.OldString == "" {
		return agent.ToolResult{}, fmt.Errorf("old_string is required")
	}

	// Read-before-edit enforcement (SoTA: CC, Crush, OpenCode all require this)
	if t.registry != nil && !t.registry.WasFileRead(args.Path) {
		return agent.PlainResult(fmt.Sprintf(
			"Read the file first before editing. Use: read(file_path=\"%s\")\n"+
				"This ensures you see the current content and don't make blind edits.", args.Path)), nil
	}

	content, err := os.ReadFile(args.Path)
	if err != nil {
		return agent.ToolResult{}, fmt.Errorf("read file: %w", err)
	}

	original := string(content)

	// Try exact match first
	if !strings.Contains(original, args.OldString) {
		// Fuzzy fallback: try with normalized whitespace
		if normalized, found := fuzzyMatch(original, args.OldString); found {
			original = strings.Replace(original, normalized, args.NewString, 1)
			if err := os.WriteFile(args.Path, []byte(original), 0o644); err != nil {
				return agent.ToolResult{}, fmt.Errorf("write file: %w", err)
			}
			return agent.PlainResult(fmt.Sprintf("Edited %s (fuzzy match — whitespace normalized)", args.Path)), nil
		}
		return agent.PlainResult("old_string not found in file. Make sure it matches exactly, including whitespace and indentation. Re-read the file to see current content."), nil
	}

	updated := strings.Replace(original, args.OldString, args.NewString, 1)
	if err := os.WriteFile(args.Path, []byte(updated), 0o644); err != nil {
		return agent.ToolResult{}, fmt.Errorf("write file: %w", err)
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

	return agent.PlainResult(strings.TrimRight(diff.String(), "\n")), nil
}

// fuzzyMatch tries to find old_string in content with normalized whitespace.
// Returns the actual matched string from content and true if found.
func fuzzyMatch(content, needle string) (string, bool) {
	// Normalize: collapse runs of whitespace, trim trailing per line
	normalizeWS := func(s string) string {
		lines := strings.Split(s, "\n")
		for i, line := range lines {
			lines[i] = strings.TrimRight(line, " \t")
		}
		return strings.Join(lines, "\n")
	}

	normNeedle := normalizeWS(needle)
	normContent := normalizeWS(content)

	idx := strings.Index(normContent, normNeedle)
	if idx < 0 {
		return "", false
	}

	// Map back to original content: find the corresponding substring
	// by counting newlines up to the match point
	origLines := strings.Split(content, "\n")
	normLines := strings.Split(normContent, "\n")
	needleLines := strings.Split(normNeedle, "\n")

	// Find which line the match starts on
	pos := 0
	startLine := -1
	for i, line := range normLines {
		if pos == idx {
			startLine = i
			break
		}
		pos += len(line) + 1 // +1 for newline
		if pos > idx {
			startLine = i
			break
		}
	}
	if startLine < 0 {
		return "", false
	}

	endLine := startLine + len(needleLines)
	if endLine > len(origLines) {
		return "", false
	}

	return strings.Join(origLines[startLine:endLine], "\n"), true
}
