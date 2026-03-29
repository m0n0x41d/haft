package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/m0n0x41d/quint-code/internal/agent"
)

const bashTimeout = 120 * time.Second

// BashTool executes shell commands.
type BashTool struct {
	projectRoot string
}

type bashArgs struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // seconds, 0 = default 120s
}

func (t *BashTool) Name() string { return "bash" }

func (t *BashTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "bash",
		Description: "Execute a shell command and return its output (stdout + stderr combined). Working directory is the project root.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The shell command to execute",
				},
				"timeout": map[string]any{
					"type":        "integer",
					"description": "Timeout in seconds (default: 120, max: 600)",
				},
			},
			"required": []any{"command"},
		},
	}
}

func (t *BashTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args bashArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	if args.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	timeout := bashTimeout
	if args.Timeout > 0 && args.Timeout <= 600 {
		timeout = time.Duration(args.Timeout) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", args.Command)
	cmd.Dir = t.projectRoot

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()

	output := strings.TrimRight(buf.String(), "\n")

	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Sprintf("Command timed out after %ds.\nPartial output:\n%s", int(timeout.Seconds()), output), nil
	}

	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			return fmt.Sprintf("Exit code: %d\n%s", exitErr.ExitCode(), output), nil
		}
		return "", fmt.Errorf("exec: %w", err)
	}

	return output, nil
}
