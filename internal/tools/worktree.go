package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/m0n0x41d/haft/internal/agent"
)

// WorktreeState tracks the active worktree for the session.
// Thread-safe — tools may be called from subagent goroutines.
type WorktreeState struct {
	mu           sync.Mutex
	worktreePath string
	branchName   string
	originalDir  string
	projectRoot  string
}

func NewWorktreeState(projectRoot string) *WorktreeState {
	return &WorktreeState{projectRoot: projectRoot}
}

func (w *WorktreeState) IsActive() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.worktreePath != ""
}

func (w *WorktreeState) Path() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.worktreePath
}

// EnterWorktreeTool creates a git worktree for isolated work.
type EnterWorktreeTool struct {
	state *WorktreeState
}

func NewEnterWorktreeTool(state *WorktreeState) *EnterWorktreeTool {
	return &EnterWorktreeTool{state: state}
}

func (t *EnterWorktreeTool) Name() string { return "enter_worktree" }

func (t *EnterWorktreeTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name: "enter_worktree",
		Description: `Create a git worktree for isolated work. The worktree is a separate checkout of the repository where you can make changes without affecting the main working directory.

Use this when:
- You want to experiment without risk to the main branch
- You need to work on a separate branch in parallel
- You want isolation for a potentially destructive operation

The worktree is automatically cleaned up when you call exit_worktree.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"branch": map[string]any{
					"type":        "string",
					"description": "Branch name for the worktree. Created if it doesn't exist.",
				},
			},
			"required": []string{"branch"},
		},
	}
}

func (t *EnterWorktreeTool) Execute(ctx context.Context, argsJSON string) (agent.ToolResult, error) {
	var args struct {
		Branch string `json:"branch"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}
	if args.Branch == "" {
		return agent.ToolResult{}, fmt.Errorf("branch is required")
	}

	if t.state.IsActive() {
		return agent.PlainResult("Already in a worktree. Call exit_worktree first."), nil
	}

	// Create worktree directory
	worktreeDir := filepath.Join(os.TempDir(), "haft-worktree-"+args.Branch)

	// Try creating the worktree
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-B", args.Branch, worktreeDir)
	cmd.Dir = t.state.projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return agent.ToolResult{}, fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(output)), err)
	}

	t.state.mu.Lock()
	t.state.worktreePath = worktreeDir
	t.state.branchName = args.Branch
	t.state.originalDir = t.state.projectRoot
	t.state.mu.Unlock()

	return agent.PlainResult(fmt.Sprintf(
		"Created worktree at %s on branch '%s'. All file operations now target this worktree. Call exit_worktree when done.",
		worktreeDir, args.Branch,
	)), nil
}

// ExitWorktreeTool removes the git worktree and returns to the main working directory.
type ExitWorktreeTool struct {
	state *WorktreeState
}

func NewExitWorktreeTool(state *WorktreeState) *ExitWorktreeTool {
	return &ExitWorktreeTool{state: state}
}

func (t *ExitWorktreeTool) Name() string { return "exit_worktree" }

func (t *ExitWorktreeTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "exit_worktree",
		Description: `Exit the current git worktree and return to the main working directory. The worktree branch is preserved (not deleted). The worktree directory is removed.`,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}

func (t *ExitWorktreeTool) Execute(ctx context.Context, _ string) (agent.ToolResult, error) {
	if !t.state.IsActive() {
		return agent.PlainResult("Not in a worktree."), nil
	}

	t.state.mu.Lock()
	worktreePath := t.state.worktreePath
	branchName := t.state.branchName
	projectRoot := t.state.originalDir
	t.state.worktreePath = ""
	t.state.branchName = ""
	t.state.mu.Unlock()

	// Remove the worktree
	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try to clean up the directory even if git fails
		_ = os.RemoveAll(worktreePath)
		return agent.PlainResult(fmt.Sprintf(
			"Worktree removed (with warnings: %s). Branch '%s' preserved. Back to main directory.",
			strings.TrimSpace(string(output)), branchName,
		)), nil
	}

	return agent.PlainResult(fmt.Sprintf(
		"Worktree removed. Branch '%s' preserved. Back to main directory.",
		branchName,
	)), nil
}
