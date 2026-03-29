package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/m0n0x41d/quint-code/internal/agent"
)

// SpawnAndWaitFunc spawns a subagent and BLOCKS until it completes.
// Returns the subagent's summary. This is the Crush/Claude Code foreground pattern:
// the parent's tool execution blocks, so the LLM cannot call other tools
// between spawn and completion.
type SpawnAndWaitFunc func(ctx context.Context, agentType, task, model string) (summary string, err error)

// SpawnAgentTool allows the LLM to spawn subagents for investigation.
// BLOCKING: the tool call does not return until the subagent completes.
// For parallel spawning: LLM requests multiple spawn_agent calls in one response,
// coordinator executes them concurrently (independent tool calls).
type SpawnAgentTool struct {
	spawnAndWait SpawnAndWaitFunc
}

func NewSpawnAgentTool(fn SpawnAndWaitFunc) *SpawnAgentTool {
	return &SpawnAgentTool{spawnAndWait: fn}
}

func (t *SpawnAgentTool) Name() string { return "spawn_agent" }

func (t *SpawnAgentTool) Schema() agent.ToolSchema {
	visible := agent.VisibleSubagents()
	typeEnum := make([]any, 0, len(visible))
	descriptions := make([]string, 0, len(visible))
	for _, def := range visible {
		typeEnum = append(typeEnum, def.Name)
		descriptions = append(descriptions, fmt.Sprintf("- %s: %s", def.Name, def.Description))
	}

	return agent.ToolSchema{
		Name:        "spawn_agent",
		Description: "Spawn a subagent for investigation. BLOCKS until the subagent completes and returns its findings directly as the tool result. Use this instead of reading many files yourself.\n\nAvailable types:\n" + strings.Join(descriptions, "\n"),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent_type": map[string]any{
					"type":        "string",
					"description": "Type of subagent to spawn",
					"enum":        typeEnum,
				},
				"task": map[string]any{
					"type":        "string",
					"description": "What the subagent should investigate or accomplish",
				},
				"model": map[string]any{
					"type":        "string",
					"description": "Optional model override (default: same as parent)",
				},
			},
			"required": []string{"agent_type", "task"},
		},
	}
}

func (t *SpawnAgentTool) Execute(ctx context.Context, argsJSON string) (agent.ToolResult, error) {
	var args struct {
		AgentType string `json:"agent_type"`
		Task      string `json:"task"`
		Model     string `json:"model"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}

	if args.AgentType == "" {
		return agent.ToolResult{}, fmt.Errorf("agent_type is required")
	}
	if args.Task == "" {
		return agent.ToolResult{}, fmt.Errorf("task is required")
	}

	// BLOCKS until subagent completes
	summary, err := t.spawnAndWait(ctx, args.AgentType, args.Task, args.Model)
	if err != nil {
		return agent.ToolResult{}, err
	}

	return agent.PlainResult(summary), nil
}
