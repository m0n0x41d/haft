// Package tasks provides background task management.
// Tasks run as goroutines with output capture, cancellation, and status tracking.
package tasks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TaskState tracks a task's lifecycle.
type TaskState string

const (
	StatePending   TaskState = "pending"
	StateRunning   TaskState = "running"
	StateCompleted TaskState = "completed"
	StateFailed    TaskState = "failed"
	StateKilled    TaskState = "killed"
)

// TaskType identifies how the task runs.
type TaskType string

const (
	TypeBash  TaskType = "bash"  // shell command
	TypeAgent TaskType = "agent" // subagent (future)
)

// Task is a background job.
type Task struct {
	ID          string    `json:"id"`
	Type        TaskType  `json:"type"`
	Command     string    `json:"command"`
	State       TaskState `json:"state"`
	Output      string    `json:"output,omitempty"`
	Error       string    `json:"error,omitempty"`
	ExitCode    int       `json:"exit_code,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`

	outputFile string
	cancel     context.CancelFunc
}

// Manager tracks and runs background tasks.
type Manager struct {
	mu          sync.RWMutex
	tasks       map[string]*Task
	nextID      int
	projectRoot string
	outputDir   string
}

// NewManager creates a task manager.
func NewManager(projectRoot string) *Manager {
	outputDir := filepath.Join(projectRoot, ".haft", "task-output")
	_ = os.MkdirAll(outputDir, 0o755)
	return &Manager{
		tasks:       make(map[string]*Task),
		projectRoot: projectRoot,
		outputDir:   outputDir,
	}
}

// Create starts a new background task.
func (m *Manager) Create(ctx context.Context, taskType TaskType, command string) (*Task, error) {
	m.mu.Lock()
	m.nextID++
	id := fmt.Sprintf("task_%d", m.nextID)
	m.mu.Unlock()

	outputFile := filepath.Join(m.outputDir, id+".log")

	task := &Task{
		ID:         id,
		Type:       taskType,
		Command:    command,
		State:      StatePending,
		CreatedAt:  time.Now().UTC(),
		outputFile: outputFile,
	}

	m.mu.Lock()
	m.tasks[id] = task
	m.mu.Unlock()

	switch taskType {
	case TypeBash:
		go m.runBash(ctx, task)
	default:
		return nil, fmt.Errorf("unsupported task type: %s", taskType)
	}

	return task, nil
}

// Get returns a task by ID.
func (m *Manager) Get(id string) (*Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tasks[id]
	return t, ok
}

// List returns all tasks.
func (m *Manager) List() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		result = append(result, t)
	}
	return result
}

// Stop cancels a running task.
func (m *Manager) Stop(id string) error {
	m.mu.RLock()
	task, ok := m.tasks[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}
	if task.cancel != nil {
		task.cancel()
	}
	return nil
}

// Output reads the captured output of a task.
func (m *Manager) Output(id string) (string, error) {
	m.mu.RLock()
	task, ok := m.tasks[id]
	m.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("task not found: %s", id)
	}

	data, err := os.ReadFile(task.outputFile)
	if err != nil {
		// Task might still be running with no output yet
		if task.Output != "" {
			return task.Output, nil
		}
		return "", nil
	}
	return string(data), nil
}

// Update modifies a task's metadata (used by task_update tool).
func (m *Manager) Update(id string, state TaskState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}
	task.State = state
	if state == StateCompleted || state == StateFailed || state == StateKilled {
		task.CompletedAt = time.Now().UTC()
	}
	return nil
}

func (m *Manager) runBash(ctx context.Context, task *Task) {
	taskCtx, cancel := context.WithCancel(ctx)
	task.cancel = cancel
	defer cancel()

	m.setState(task, StateRunning)

	// Open output file
	f, err := os.Create(task.outputFile)
	if err != nil {
		m.failTask(task, fmt.Sprintf("create output file: %s", err))
		return
	}
	defer f.Close()

	cmd := exec.CommandContext(taskCtx, "sh", "-c", task.Command)
	cmd.Dir = m.projectRoot
	cmd.Stdout = f
	cmd.Stderr = f

	err = cmd.Run()

	// Read output
	data, _ := os.ReadFile(task.outputFile)
	output := string(data)
	// Truncate stored output
	if len(output) > 10_000 {
		output = output[:10_000] + "\n... (truncated, use task_output for full log)"
	}

	m.mu.Lock()
	task.Output = output
	if err != nil {
		if taskCtx.Err() != nil {
			task.State = StateKilled
			task.Error = "canceled"
		} else {
			task.State = StateFailed
			task.Error = err.Error()
			if exitErr, ok := err.(*exec.ExitError); ok {
				task.ExitCode = exitErr.ExitCode()
			}
		}
	} else {
		task.State = StateCompleted
	}
	task.CompletedAt = time.Now().UTC()
	m.mu.Unlock()
}

func (m *Manager) setState(task *Task, state TaskState) {
	m.mu.Lock()
	task.State = state
	m.mu.Unlock()
}

func (m *Manager) failTask(task *Task, msg string) {
	m.mu.Lock()
	task.State = StateFailed
	task.Error = msg
	task.CompletedAt = time.Now().UTC()
	m.mu.Unlock()
}

// FormatTaskList returns a human-readable task list.
func FormatTaskList(tasks []*Task) string {
	if len(tasks) == 0 {
		return "No tasks."
	}
	var b strings.Builder
	for _, t := range tasks {
		icon := stateIcon(t.State)
		fmt.Fprintf(&b, "%s %s [%s] %s", icon, t.ID, t.State, t.Command)
		if t.Error != "" {
			fmt.Fprintf(&b, " (error: %s)", t.Error)
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func stateIcon(s TaskState) string {
	switch s {
	case StatePending:
		return "\u25CB" // ○
	case StateRunning:
		return "\u25CF" // ●
	case StateCompleted:
		return "\u2713" // ✓
	case StateFailed:
		return "\u2717" // ✗
	case StateKilled:
		return "\u25A0" // ■
	default:
		return "?"
	}
}
