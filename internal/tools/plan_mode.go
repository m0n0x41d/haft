package tools

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/agent"
)

// PlanModeController is called by the tools to toggle plan mode on the session.
type PlanModeController interface {
	SetPlanMode(enabled bool)
	IsPlanMode() bool
}

// EnterPlanModeTool switches the session to read-only planning mode.
// Write tools are disabled until ExitPlanMode is called.
type EnterPlanModeTool struct {
	controller PlanModeController
}

func NewEnterPlanModeTool(ctrl PlanModeController) *EnterPlanModeTool {
	return &EnterPlanModeTool{controller: ctrl}
}

func (t *EnterPlanModeTool) Name() string { return "enter_plan_mode" }

func (t *EnterPlanModeTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "enter_plan_mode",
		Description: "Enter plan mode. All write/edit tools are disabled. Use this when you need to explore and design before implementing. Call exit_plan_mode when ready to implement.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}

func (t *EnterPlanModeTool) Execute(_ context.Context, _ string) (agent.ToolResult, error) {
	if t.controller.IsPlanMode() {
		return agent.PlainResult("Already in plan mode."), nil
	}
	t.controller.SetPlanMode(true)
	return agent.PlainResult("Entered plan mode. Write tools (write, edit, multiedit, bash with mutations) are now disabled. Use exit_plan_mode when ready to implement."), nil
}

// ExitPlanModeTool restores normal mode with all tools available.
type ExitPlanModeTool struct {
	controller PlanModeController
}

func NewExitPlanModeTool(ctrl PlanModeController) *ExitPlanModeTool {
	return &ExitPlanModeTool{controller: ctrl}
}

func (t *ExitPlanModeTool) Name() string { return "exit_plan_mode" }

func (t *ExitPlanModeTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "exit_plan_mode",
		Description: "Exit plan mode and restore write tools. Call this when planning is complete and you're ready to implement.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}

func (t *ExitPlanModeTool) Execute(_ context.Context, _ string) (agent.ToolResult, error) {
	if !t.controller.IsPlanMode() {
		return agent.PlainResult("Not in plan mode."), nil
	}
	t.controller.SetPlanMode(false)
	return agent.PlainResult("Exited plan mode. All tools are now available."), nil
}

// PlanModeGuard checks if a tool should be blocked in plan mode.
// Returns true if the tool is blocked.
func PlanModeGuard(ctrl PlanModeController, toolName string) bool {
	if !ctrl.IsPlanMode() {
		return false // not blocked
	}
	return planModeBlockedTools[toolName]
}

// PlanModeBlockMessage returns the error message shown when a write tool is blocked.
func PlanModeBlockMessage(toolName string) string {
	return fmt.Sprintf("Tool '%s' is blocked in plan mode. Use exit_plan_mode first.", toolName)
}

// planModeBlockedTools are tools disabled in plan mode.
// More permissive than read-only subagents: allows bash (for tests/exploration)
// and haft tools. Only blocks direct file mutations.
var planModeBlockedTools = map[string]bool{
	"write":     true,
	"edit":      true,
	"multiedit": true,
}
