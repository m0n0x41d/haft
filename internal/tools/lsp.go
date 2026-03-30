package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/lsp"
)

// ---------------------------------------------------------------------------
// LSP Diagnostics Tool
// ---------------------------------------------------------------------------

type LSPDiagnosticsTool struct {
	manager     *lsp.Manager
	projectRoot string
}

func NewLSPDiagnosticsTool(manager *lsp.Manager, projectRoot string) *LSPDiagnosticsTool {
	return &LSPDiagnosticsTool{manager: manager, projectRoot: projectRoot}
}

func (t *LSPDiagnosticsTool) Name() string { return "lsp_diagnostics" }

func (t *LSPDiagnosticsTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name: "lsp_diagnostics",
		Description: `Get language server diagnostics (errors, warnings) for a file or the whole project.

Use after editing code to check for type errors, import issues, etc.
The language server is started automatically when you first check a file.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file": map[string]any{
					"type":        "string",
					"description": "File path to check. If empty, returns project-wide diagnostics.",
				},
			},
		},
	}
}

func (t *LSPDiagnosticsTool) Execute(ctx context.Context, argsJSON string) (agent.ToolResult, error) {
	var args struct {
		File string `json:"file"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}

	// Ensure LSP is running for this file type
	if args.File != "" {
		t.manager.NotifyFileChanged(ctx, args.File)
		t.manager.WaitForDiagnostics(ctx, args.File)
	}

	// Small delay for diagnostics to propagate
	time.Sleep(500 * time.Millisecond)

	diags := t.manager.GetDiagnostics(args.File)
	counts := lsp.CountDiagnostics(diags)

	var b strings.Builder
	if len(diags) == 0 {
		b.WriteString("No diagnostics found.")
	} else {
		b.WriteString(lsp.FormatDiagnostics(diags, t.projectRoot))
	}

	b.WriteString(fmt.Sprintf("\nSummary: %d errors, %d warnings, %d info, %d hints",
		counts.Error, counts.Warning, counts.Info, counts.Hint))

	return agent.PlainResult(b.String()), nil
}

// ---------------------------------------------------------------------------
// LSP References Tool
// ---------------------------------------------------------------------------

type LSPReferencesTool struct {
	manager     *lsp.Manager
	projectRoot string
}

func NewLSPReferencesTool(manager *lsp.Manager, projectRoot string) *LSPReferencesTool {
	return &LSPReferencesTool{manager: manager, projectRoot: projectRoot}
}

func (t *LSPReferencesTool) Name() string { return "lsp_references" }

func (t *LSPReferencesTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name: "lsp_references",
		Description: `Find all references to a symbol at a given file position.

Returns a list of locations where the symbol is used, including the declaration.
Line and column are 1-based. Use after reading a file to understand symbol usage.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file":   map[string]any{"type": "string", "description": "File path containing the symbol"},
				"line":   map[string]any{"type": "integer", "description": "Line number (1-based)"},
				"column": map[string]any{"type": "integer", "description": "Column number (1-based)"},
			},
			"required": []any{"file", "line", "column"},
		},
	}
}

func (t *LSPReferencesTool) Execute(ctx context.Context, argsJSON string) (agent.ToolResult, error) {
	var args struct {
		File   string `json:"file"`
		Line   int    `json:"line"`
		Column int    `json:"column"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}
	if args.File == "" || args.Line == 0 || args.Column == 0 {
		return agent.ToolResult{}, fmt.Errorf("file, line, and column are required")
	}

	// Ensure LSP is running
	t.manager.EnsureForFile(ctx, args.File)
	time.Sleep(1 * time.Second) // give server time to index

	locs, err := t.manager.FindReferences(ctx, args.File, args.Line, args.Column)
	if err != nil {
		return agent.ToolResult{}, fmt.Errorf("find references: %w", err)
	}
	if len(locs) == 0 {
		return agent.PlainResult("No references found."), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d references:\n", len(locs))
	for _, loc := range locs {
		b.WriteString("  " + lsp.FormatLocation(loc, t.projectRoot) + "\n")
	}
	return agent.PlainResult(b.String()), nil
}

// ---------------------------------------------------------------------------
// LSP Restart Tool
// ---------------------------------------------------------------------------

type LSPRestartTool struct {
	manager *lsp.Manager
}

func NewLSPRestartTool(manager *lsp.Manager) *LSPRestartTool {
	return &LSPRestartTool{manager: manager}
}

func (t *LSPRestartTool) Name() string { return "lsp_restart" }

func (t *LSPRestartTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name: "lsp_restart",
		Description: `Restart one or all language servers.

Use when diagnostics seem stale or a server is not responding.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"server": map[string]any{
					"type":        "string",
					"description": "Server name to restart (e.g., 'gopls'). If empty, restarts all.",
				},
			},
		},
	}
}

func (t *LSPRestartTool) Execute(ctx context.Context, argsJSON string) (agent.ToolResult, error) {
	var args struct {
		Server string `json:"server"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &args)

	if err := t.manager.RestartServer(ctx, args.Server); err != nil {
		return agent.ToolResult{}, err
	}

	if args.Server != "" {
		return agent.PlainResult(fmt.Sprintf("Restarted %s.", args.Server)), nil
	}
	return agent.PlainResult("Restarted all language servers."), nil
}
