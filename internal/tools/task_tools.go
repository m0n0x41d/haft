package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/tasks"
)

// --- TaskCreate ---

type TaskCreateTool struct{ mgr *tasks.Manager }

func NewTaskCreateTool(mgr *tasks.Manager) *TaskCreateTool { return &TaskCreateTool{mgr: mgr} }
func (t *TaskCreateTool) Name() string                     { return "task_create" }
func (t *TaskCreateTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "task_create",
		Description: "Create a background task that runs a shell command. Returns immediately with a task ID. Use task_get to check status and task_output to read results.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Shell command to run in the background",
				},
			},
			"required": []string{"command"},
		},
	}
}
func (t *TaskCreateTool) Execute(ctx context.Context, argsJSON string) (agent.ToolResult, error) {
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}
	if args.Command == "" {
		return agent.ToolResult{}, fmt.Errorf("command is required")
	}
	task, err := t.mgr.Create(ctx, tasks.TypeBash, args.Command)
	if err != nil {
		return agent.ToolResult{}, err
	}
	return agent.PlainResult(fmt.Sprintf("Task created: %s (running in background)", task.ID)), nil
}

// --- TaskGet ---

type TaskGetTool struct{ mgr *tasks.Manager }

func NewTaskGetTool(mgr *tasks.Manager) *TaskGetTool { return &TaskGetTool{mgr: mgr} }
func (t *TaskGetTool) Name() string                  { return "task_get" }
func (t *TaskGetTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "task_get",
		Description: "Get the status and summary of a background task.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{
					"type":        "string",
					"description": "Task ID returned by task_create",
				},
			},
			"required": []string{"task_id"},
		},
	}
}
func (t *TaskGetTool) Execute(_ context.Context, argsJSON string) (agent.ToolResult, error) {
	var args struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}
	task, ok := t.mgr.Get(args.TaskID)
	if !ok {
		return agent.ToolResult{}, fmt.Errorf("task not found: %s", args.TaskID)
	}
	result := fmt.Sprintf("Task %s: %s\nCommand: %s", task.ID, task.State, task.Command)
	if task.Error != "" {
		result += fmt.Sprintf("\nError: %s", task.Error)
	}
	if task.Output != "" {
		result += fmt.Sprintf("\nOutput (last):\n%s", task.Output)
	}
	return agent.PlainResult(result), nil
}

// --- TaskList ---

type TaskListTool struct{ mgr *tasks.Manager }

func NewTaskListTool(mgr *tasks.Manager) *TaskListTool { return &TaskListTool{mgr: mgr} }
func (t *TaskListTool) Name() string                   { return "task_list" }
func (t *TaskListTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "task_list",
		Description: "List all background tasks with their status.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}
func (t *TaskListTool) Execute(_ context.Context, _ string) (agent.ToolResult, error) {
	return agent.PlainResult(tasks.FormatTaskList(t.mgr.List())), nil
}

// --- TaskStop ---

type TaskStopTool struct{ mgr *tasks.Manager }

func NewTaskStopTool(mgr *tasks.Manager) *TaskStopTool { return &TaskStopTool{mgr: mgr} }
func (t *TaskStopTool) Name() string                   { return "task_stop" }
func (t *TaskStopTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "task_stop",
		Description: "Stop a running background task.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{
					"type":        "string",
					"description": "Task ID to stop",
				},
			},
			"required": []string{"task_id"},
		},
	}
}
func (t *TaskStopTool) Execute(_ context.Context, argsJSON string) (agent.ToolResult, error) {
	var args struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}
	if err := t.mgr.Stop(args.TaskID); err != nil {
		return agent.ToolResult{}, err
	}
	return agent.PlainResult(fmt.Sprintf("Task %s stop signal sent.", args.TaskID)), nil
}

// --- TaskUpdate ---

type TaskUpdateTool struct{ mgr *tasks.Manager }

func NewTaskUpdateTool(mgr *tasks.Manager) *TaskUpdateTool { return &TaskUpdateTool{mgr: mgr} }
func (t *TaskUpdateTool) Name() string                     { return "task_update" }
func (t *TaskUpdateTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "task_update",
		Description: "Update the status of a task (e.g., mark as completed or failed).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{
					"type":        "string",
					"description": "Task ID to update",
				},
				"state": map[string]any{
					"type":        "string",
					"description": "New state",
					"enum":        []any{"completed", "failed"},
				},
			},
			"required": []string{"task_id", "state"},
		},
	}
}
func (t *TaskUpdateTool) Execute(_ context.Context, argsJSON string) (agent.ToolResult, error) {
	var args struct {
		TaskID string `json:"task_id"`
		State  string `json:"state"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}
	if err := t.mgr.Update(args.TaskID, tasks.TaskState(args.State)); err != nil {
		return agent.ToolResult{}, err
	}
	return agent.PlainResult(fmt.Sprintf("Task %s updated to %s.", args.TaskID, args.State)), nil
}

// --- TaskOutput ---

type TaskOutputTool struct{ mgr *tasks.Manager }

func NewTaskOutputTool(mgr *tasks.Manager) *TaskOutputTool { return &TaskOutputTool{mgr: mgr} }
func (t *TaskOutputTool) Name() string                     { return "task_output" }
func (t *TaskOutputTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "task_output",
		Description: "Read the full output log of a background task.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{
					"type":        "string",
					"description": "Task ID to read output from",
				},
			},
			"required": []string{"task_id"},
		},
	}
}
func (t *TaskOutputTool) Execute(_ context.Context, argsJSON string) (agent.ToolResult, error) {
	var args struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}
	output, err := t.mgr.Output(args.TaskID)
	if err != nil {
		return agent.ToolResult{}, err
	}
	if output == "" {
		return agent.PlainResult("(no output yet)"), nil
	}
	return agent.PlainResult(output), nil
}
