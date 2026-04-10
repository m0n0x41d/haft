package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// AgentKind identifies a supported coding agent.
type AgentKind string

const (
	AgentClaude AgentKind = "claude"
	AgentCodex  AgentKind = "codex"
	AgentHaft   AgentKind = "haft"
)

const (
	taskOutputMaxLines      = 500
	taskOutputFlushInterval = 350 * time.Millisecond
)

// InstalledAgent describes a detected agent binary.
type InstalledAgent struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
}

// TaskState tracks a running or persisted agent task.
type TaskState struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Agent          string `json:"agent"`
	Project        string `json:"project"`
	ProjectPath    string `json:"project_path"`
	Status         string `json:"status"` // pending, running, completed, failed, cancelled, interrupted
	Prompt         string `json:"prompt"`
	Branch         string `json:"branch"`
	Worktree       bool   `json:"worktree"`
	WorktreePath   string `json:"worktree_path"`
	ReusedWorktree bool   `json:"reused_worktree"`
	StartedAt      string `json:"started_at"`
	CompletedAt    string `json:"completed_at"`
	ErrorMessage   string `json:"error_message"`
	Output         string `json:"output"` // bounded output tail
	AutoRun        bool   `json:"auto_run"`       // true = agent runs without pausing
}

type TaskOutputEvent struct {
	ID     string `json:"id"`
	Chunk  string `json:"chunk"`
	Output string `json:"output"`
}

// taskRunner manages running agent subprocesses.
type taskRunner struct {
	mu    sync.Mutex
	tasks map[string]*runningTask
	seq   int
	app   *App
	store *desktopTaskStore
}

type runningTask struct {
	state     TaskState
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	output    *taskOutputBuffer
	flushStop chan struct{}
	flushDone chan struct{}
	flushOnce sync.Once
}

type taskOutputWriter struct {
	buffer   *taskOutputBuffer
	onUpdate func(chunk string, output string)
}

type taskOutputBuffer struct {
	mu       sync.Mutex
	lines    []string
	partial  string
	maxLines int
}

type worktreeHandle struct {
	Path   string
	Reused bool
}

func newTaskRunner(app *App, store *desktopTaskStore) *taskRunner {
	return &taskRunner{
		tasks: make(map[string]*runningTask),
		app:   app,
		store: store,
	}
}

func (r *taskRunner) restore(ctx context.Context, projectPath string) error {
	if r == nil || r.store == nil || projectPath == "" {
		return nil
	}

	return r.store.MarkRunningTasksInterrupted(ctx, projectPath)
}

func (r *taskRunner) hasRunningTasks() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, rt := range r.tasks {
		if rt.state.Status == "running" {
			return true
		}
	}

	return false
}

func (r *taskRunner) shutdown() {
	if r == nil {
		return
	}

	r.mu.Lock()

	// Copy state snapshots under lock to avoid races with finalizeTask.
	type shutdownItem struct {
		rt     *runningTask
		state  TaskState
		wasRun bool
	}
	items := make([]shutdownItem, 0, len(r.tasks))
	for _, rt := range r.tasks {
		items = append(items, shutdownItem{
			rt:     rt,
			state:  rt.state, // copy
			wasRun: rt.state.Status == "running",
		})
	}

	r.mu.Unlock()

	for _, item := range items {
		if !item.wasRun {
			continue
		}

		item.rt.cancel()
		item.rt.stopFlusher()

		state := item.state
		state.Status = "interrupted"
		state.ErrorMessage = "Desktop app shut down before the task completed."
		state.CompletedAt = nowRFC3339()
		state.Output = item.rt.output.String()

		if err := r.persistState(state); err != nil {
			r.app.emitAppError("shutdown tasks", err)
		}
	}
}

func (r *taskRunner) nextTaskID() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seq++

	return fmt.Sprintf("task-%d-%d", time.Now().Unix(), r.seq)
}

func (r *taskRunner) register(rt *runningTask) {
	r.mu.Lock()
	r.tasks[rt.state.ID] = rt
	r.mu.Unlock()
}

func (r *taskRunner) list(ctx context.Context, projectPath string) ([]TaskState, error) {
	if r == nil {
		return []TaskState{}, nil
	}

	persisted := make([]TaskState, 0)

	if r.store != nil && projectPath != "" {
		tasks, err := r.store.ListTasks(ctx, projectPath)
		if err != nil {
			return nil, err
		}

		persisted = tasks
	}

	r.mu.Lock()

	live := make(map[string]TaskState, len(r.tasks))

	for id, rt := range r.tasks {
		state := rt.state
		state.Output = rt.output.String()
		live[id] = state
	}

	r.mu.Unlock()

	result := make([]TaskState, 0, len(persisted)+len(live))
	seen := make(map[string]bool, len(persisted))

	for _, state := range persisted {
		if current, ok := live[state.ID]; ok {
			state = current
		}

		result = append(result, state)
		seen[state.ID] = true
	}

	for id, state := range live {
		if seen[id] {
			continue
		}

		result = append(result, state)
	}

	sort.Slice(result, func(i int, j int) bool {
		return result[i].StartedAt > result[j].StartedAt
	})

	return result, nil
}

func (r *taskRunner) currentOutput(id string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rt, ok := r.tasks[id]
	if !ok {
		return "", false
	}

	return rt.output.String(), true
}

func (r *taskRunner) stopFlusher(id string) {
	r.mu.Lock()
	rt, ok := r.tasks[id]
	r.mu.Unlock()

	if ok {
		rt.stopFlusher()
	}
}

func (r *taskRunner) startOutputFlusher(rt *runningTask) {
	ticker := time.NewTicker(taskOutputFlushInterval)

	go func() {
		defer close(rt.flushDone)
		defer ticker.Stop()

		lastFlushed := ""

		for {
			select {
			case <-ticker.C:
				snapshot := rt.output.String()
				if snapshot == lastFlushed {
					continue
				}

				lastFlushed = snapshot

				if err := r.flushOutput(rt.state.ID, snapshot); err != nil {
					r.app.emitAppError("task output persistence", err)
				}
			case <-rt.flushStop:
				snapshot := rt.output.String()

				if snapshot != lastFlushed {
					if err := r.flushOutput(rt.state.ID, snapshot); err != nil {
						r.app.emitAppError("task output persistence", err)
					}
				}

				return
			}
		}
	}()
}

func (r *taskRunner) flushOutput(id string, output string) error {
	if r == nil || r.store == nil {
		return nil
	}

	return r.store.UpdateOutput(context.Background(), id, output)
}

func (r *taskRunner) persistState(state TaskState) error {
	if r == nil || r.store == nil {
		return nil
	}

	return r.store.UpsertTask(context.Background(), state)
}

func (r *taskRunner) emitTaskOutput(id string, chunk string, output string) {
	if r == nil || r.app == nil || r.app.ctx == nil {
		return
	}

	runtime.EventsEmit(
		r.app.ctx,
		"task.output",
		TaskOutputEvent{
			ID:     id,
			Chunk:  chunk,
			Output: output,
		},
	)
}

func (r *taskRunner) emitTaskStatus(state TaskState) {
	if r == nil || r.app == nil || r.app.ctx == nil {
		return
	}

	runtime.EventsEmit(r.app.ctx, "task.status", state)
}

func (r *taskRunner) setAutoRun(id string, autoRun bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	rt, ok := r.tasks[id]
	if !ok {
		// Task might be persisted but not in memory — update store directly
		if r.store != nil {
			return r.store.SetAutoRun(context.Background(), id, autoRun)
		}
		return fmt.Errorf("task not found: %s", id)
	}

	rt.state.AutoRun = autoRun
	if r.store != nil {
		_ = r.store.SetAutoRun(context.Background(), id, autoRun)
	}
	return nil
}

func (r *taskRunner) finalizeTask(rt *runningTask, waitErr error) {
	rt.stopFlusher()

	r.mu.Lock()

	current, ok := r.tasks[rt.state.ID]
	if !ok {
		r.mu.Unlock()
		return
	}

	state := current.state
	state.Output = current.output.String()
	state.CompletedAt = nowRFC3339()

	switch {
	case state.Status == "cancelled":
		// Preserve explicit cancellation from CancelTask.
	case waitErr != nil:
		state.Status = "failed"
		state.ErrorMessage = waitErr.Error()
	default:
		state.Status = "completed"
		state.ErrorMessage = ""
	}

	current.state = state
	delete(r.tasks, rt.state.ID)

	r.mu.Unlock()

	if state.Status == "cancelled" {
		message, cleanupErr := cleanupWorktree(rt.state.ProjectPath, state, false)
		if cleanupErr != nil {
			state.ErrorMessage = cleanupErr.Error()
			state.Output = appendTaskNote(state.Output, cleanupErr.Error())
		}

		if message != "" {
			state.Output = appendTaskNote(state.Output, message)
		}
	}

	if err := r.persistState(state); err != nil {
		r.app.emitAppError("finalize task", err)
	}

	r.emitTaskStatus(state)
	r.app.notifyTaskState(state)
}

func (rt *runningTask) stopFlusher() {
	rt.flushOnce.Do(func() {
		close(rt.flushStop)
		<-rt.flushDone
	})
}

func newTaskOutputBuffer(maxLines int, seed string) *taskOutputBuffer {
	buffer := &taskOutputBuffer{maxLines: maxLines}

	if seed != "" {
		buffer.Append(seed)
	}

	return buffer
}

func (w *taskOutputWriter) Write(p []byte) (int, error) {
	chunk := string(p)
	output := w.buffer.Append(chunk)

	if w.onUpdate != nil {
		w.onUpdate(chunk, output)
	}

	return len(p), nil
}

func (b *taskOutputBuffer) Append(chunk string) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	text := b.partial + chunk
	parts := strings.Split(text, "\n")

	if strings.HasSuffix(text, "\n") {
		b.lines = append(b.lines, parts[:len(parts)-1]...)
		b.partial = ""
	} else {
		if len(parts) > 1 {
			b.lines = append(b.lines, parts[:len(parts)-1]...)
		}

		if len(parts) > 0 {
			b.partial = parts[len(parts)-1]
		}
	}

	if b.maxLines > 0 && len(b.lines) > b.maxLines {
		b.lines = append([]string(nil), b.lines[len(b.lines)-b.maxLines:]...)
	}

	return b.snapshotLocked()
}

func (b *taskOutputBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.snapshotLocked()
}

func (b *taskOutputBuffer) snapshotLocked() string {
	if len(b.lines) == 0 {
		return b.partial
	}

	if b.partial == "" {
		return strings.Join(b.lines, "\n")
	}

	parts := append(append([]string(nil), b.lines...), b.partial)
	return strings.Join(parts, "\n")
}

// --- App binding methods ---

// DetectAgents finds installed coding agents.
func (a *App) DetectAgents() ([]InstalledAgent, error) {
	var agents []InstalledAgent

	if path, err := exec.LookPath("claude"); err == nil {
		version := getVersion(path, "--version")
		agents = append(agents, InstalledAgent{
			Kind:    string(AgentClaude),
			Name:    "Claude Code",
			Path:    path,
			Version: version,
		})
	}

	if path, err := exec.LookPath("codex"); err == nil {
		version := getVersion(path, "--version")
		agents = append(agents, InstalledAgent{
			Kind:    string(AgentCodex),
			Name:    "Codex",
			Path:    path,
			Version: version,
		})
	}

	if path, err := exec.LookPath("haft"); err == nil {
		version := getVersion(path, "version")
		agents = append(agents, InstalledAgent{
			Kind:    string(AgentHaft),
			Name:    "Haft Agent",
			Path:    path,
			Version: version,
		})
	}

	return agents, nil
}

// SpawnTask creates and starts a new agent task.
func (a *App) SpawnTask(agentKind string, prompt string, useWorktree bool, branchName string) (*TaskState, error) {
	return a.spawnTaskWithTitle(agentKind, prompt, useWorktree, branchName, "")
}

func (a *App) spawnTaskWithTitle(
	agentKind string,
	prompt string,
	useWorktree bool,
	branchName string,
	title string,
) (*TaskState, error) {
	if a.projectRoot == "" {
		return nil, fmt.Errorf("no active project")
	}

	if a.dbConn == nil {
		return nil, fmt.Errorf("no database connection")
	}

	if a.tasks == nil {
		a.tasks = newTaskRunner(a, newDesktopTaskStore(a.dbConn.GetRawDB()))
	}

	agentKind = normalizeAgentKind(agentKind, string(AgentClaude))
	branchName = strings.TrimSpace(branchName)

	if useWorktree && branchName == "" {
		branchName = fmt.Sprintf("haft-task-%d", time.Now().Unix())
	}

	cfg := defaultDesktopConfig()
	warnings := make([]string, 0)

	loadedConfig, err := loadDesktopConfig()
	if err == nil && loadedConfig != nil {
		cfg = *loadedConfig
	} else if err != nil {
		warnings = append(warnings, fmt.Sprintf("warning: desktop config could not be loaded: %v", err))
	}

	workDir := a.projectRoot
	worktree := worktreeHandle{}

	state := TaskState{
		ID:          a.tasks.nextTaskID(),
		Title:       firstNonEmpty(strings.TrimSpace(title), truncate(prompt, 60)),
		Agent:       agentKind,
		Project:     a.projectName,
		ProjectPath: a.projectRoot,
		Status:      "running",
		Prompt:      prompt,
		Branch:      branchName,
		Worktree:    useWorktree,
		StartedAt:   nowRFC3339(),
	}

	if useWorktree {
		worktree, err = createWorktree(a.projectRoot, branchName)
		if err != nil {
			return nil, fmt.Errorf("create worktree: %w", err)
		}

		workDir = worktree.Path
		state.WorktreePath = worktree.Path
		state.ReusedWorktree = worktree.Reused
	}

	if cfg.AutoWireMCP {
		if err := wireHaftMCP(agentKind, a.projectRoot); err != nil {
			warnings = append(warnings, fmt.Sprintf("warning: failed to wire Haft MCP: %v", err))
		}
	}

	args := buildAgentArgs(AgentKind(agentKind), prompt)
	if len(args) == 0 {
		return nil, fmt.Errorf("unsupported agent: %s", agentKind)
	}

	// Use task timeout from config to prevent zombie agent processes.
	timeout := time.Duration(cfg.TaskTimeoutMinutes) * time.Minute
	if timeout <= 0 {
		timeout = 300 * time.Minute // fallback: 5 hours
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = workDir
	cmd.Env = append(
		os.Environ(),
		"HAFT_PROJECT_ROOT="+a.projectRoot,
		"HAFT_TASK_ID="+state.ID,
		"HAFT_TASK_BRANCH="+state.Branch,
		"HAFT_TASK_WORKTREE="+state.WorktreePath,
	)

	initialOutput := ""
	if len(warnings) > 0 {
		initialOutput = strings.Join(warnings, "\n") + "\n"
	}

	rt := &runningTask{
		state:     state,
		cmd:       cmd,
		cancel:    cancel,
		output:    newTaskOutputBuffer(taskOutputMaxLines, initialOutput),
		flushStop: make(chan struct{}),
		flushDone: make(chan struct{}),
	}

	writer := &taskOutputWriter{
		buffer: rt.output,
		onUpdate: func(chunk string, output string) {
			a.tasks.emitTaskOutput(state.ID, chunk, output)
		},
	}

	cmd.Stdout = writer
	cmd.Stderr = writer

	rt.state.Output = rt.output.String()

	if err := a.tasks.persistState(rt.state); err != nil {
		cancel()

		if state.WorktreePath != "" && !state.ReusedWorktree {
			_, _ = cleanupWorktree(a.projectRoot, state, true)
		}

		return nil, fmt.Errorf("persist task: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()

		rt.state.Status = "failed"
		rt.state.ErrorMessage = err.Error()
		rt.state.CompletedAt = nowRFC3339()
		rt.state.Output = appendTaskNote(rt.state.Output, err.Error())

		_ = a.tasks.persistState(rt.state)

		if state.WorktreePath != "" && !state.ReusedWorktree {
			_, _ = cleanupWorktree(a.projectRoot, state, true)
		}

		return nil, fmt.Errorf("start %s: %w", agentKind, err)
	}

	a.tasks.register(rt)
	a.tasks.startOutputFlusher(rt)
	a.tasks.emitTaskStatus(rt.state)

	go func() {
		waitErr := cmd.Wait()
		a.tasks.finalizeTask(rt, waitErr)
	}()

	result := rt.state
	return &result, nil
}

// ListTasks returns all non-archived tasks for the active project.
func (a *App) ListTasks() ([]TaskState, error) {
	if a.tasks == nil {
		return []TaskState{}, nil
	}

	return a.tasks.list(a.ctx, a.projectRoot)
}

// GetTaskOutput returns the current buffered output for a task.
func (a *App) GetTaskOutput(id string) (string, error) {
	if a.tasks != nil {
		if output, ok := a.tasks.currentOutput(id); ok {
			return output, nil
		}
	}

	if a.tasks == nil || a.tasks.store == nil {
		return "", fmt.Errorf("task not found: %s", id)
	}

	output, err := a.tasks.store.GetTaskOutput(a.ctx, id)
	if err != nil {
		return "", fmt.Errorf("get task output %s: %w", id, err)
	}

	return output, nil
}

// CancelTask stops a running task.
func (a *App) CancelTask(id string) error {
	if a.tasks == nil {
		return fmt.Errorf("no tasks")
	}

	a.tasks.mu.Lock()
	rt, ok := a.tasks.tasks[id]
	var stateCopy TaskState
	if ok {
		rt.state.Status = "cancelled"
		rt.state.ErrorMessage = ""
		rt.state.Output = rt.output.String()
		stateCopy = rt.state // copy under lock
	}
	a.tasks.mu.Unlock()

	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}

	if err := a.tasks.persistState(stateCopy); err != nil {
		return fmt.Errorf("persist cancelled task: %w", err)
	}

	rt.cancel()
	a.tasks.emitTaskStatus(stateCopy)

	return nil
}

// SetTaskAutoRun toggles auto-run mode for a task.
// Auto-run: agent proceeds without user intervention.
// Checkpointed: agent pauses at natural breakpoints, user clicks "Continue".
func (a *App) SetTaskAutoRun(id string, autoRun bool) error {
	if a.tasks == nil {
		return fmt.Errorf("no task runner")
	}
	return a.tasks.setAutoRun(id, autoRun)
}

// ArchiveTask hides a completed task and cleans up its worktree when safe.
func (a *App) ArchiveTask(id string) error {
	if a.tasks == nil || a.tasks.store == nil {
		return fmt.Errorf("no task store")
	}

	a.tasks.mu.Lock()
	_, running := a.tasks.tasks[id]
	a.tasks.mu.Unlock()

	if running {
		return fmt.Errorf("cannot archive a running task")
	}

	state, err := a.tasks.store.GetTask(a.ctx, id)
	if err != nil {
		return fmt.Errorf("load task %s: %w", id, err)
	}

	if state.WorktreePath != "" {
		message, cleanupErr := cleanupWorktree(a.projectRoot, *state, false)
		if cleanupErr != nil {
			return cleanupErr
		}

		if message != "" {
			state.Output = appendTaskNote(state.Output, message)

			if err := a.tasks.persistState(*state); err != nil {
				return fmt.Errorf("persist task cleanup note: %w", err)
			}
		}
	}

	if err := a.tasks.store.ArchiveTask(a.ctx, id); err != nil {
		return fmt.Errorf("archive task %s: %w", id, err)
	}

	return nil
}

func (a *App) HandoffTask(id string, targetAgent string) (*TaskState, error) {
	if a.tasks == nil || a.tasks.store == nil {
		return nil, fmt.Errorf("no task store")
	}

	source, err := a.loadTaskState(strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}

	nextAgent := normalizeAgentKind(targetAgent, string(AgentClaude))
	if nextAgent == source.Agent {
		return nil, fmt.Errorf("handoff target must be different from the source agent")
	}

	useWorktree := source.Worktree && source.Status != "running"
	branch := ""
	if useWorktree {
		branch = source.Branch
	}

	prompt := buildHandoffPrompt(*source, nextAgent)

	return a.spawnTaskWithTitle(
		nextAgent,
		prompt,
		useWorktree,
		branch,
		fmt.Sprintf("Handoff: %s", source.Title),
	)
}

// --- Helpers ---

func (a *App) loadTaskState(id string) (*TaskState, error) {
	if a.tasks == nil {
		return nil, fmt.Errorf("task not found: %s", id)
	}

	a.tasks.mu.Lock()
	rt, ok := a.tasks.tasks[id]
	a.tasks.mu.Unlock()

	if ok {
		state := rt.state
		state.Output = rt.output.String()
		return &state, nil
	}

	state, err := a.tasks.store.GetTask(a.ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load task %s: %w", id, err)
	}

	return state, nil
}

func getVersion(path string, flag string) string {
	out, err := exec.Command(path, flag).Output()
	if err != nil {
		return "unknown"
	}

	return strings.TrimSpace(strings.Split(string(out), "\n")[0])
}

func buildAgentArgs(kind AgentKind, prompt string) []string {
	switch kind {
	case AgentClaude:
		return []string{
			"claude", "-p", prompt,
			"--verbose",
			"--output-format", "text",
		}
	case AgentCodex:
		return []string{
			"codex", "exec",
			"--full-auto",
			prompt,
		}
	case AgentHaft:
		return []string{"haft", "agent", prompt}
	default:
		return nil
	}
}

func createWorktree(projectRoot string, branch string) (worktreeHandle, error) {
	wtDir := filepath.Join(projectRoot, ".haft", "worktrees", branch)

	if err := os.MkdirAll(filepath.Dir(wtDir), 0o755); err != nil {
		return worktreeHandle{}, err
	}

	if info, err := os.Stat(wtDir); err == nil && info.IsDir() {
		if isGitWorktree(wtDir) {
			return worktreeHandle{Path: wtDir, Reused: true}, nil
		}

		return worktreeHandle{}, fmt.Errorf("path already exists and is not a git worktree: %s", wtDir)
	}

	createNew := exec.Command("git", "worktree", "add", "-b", branch, wtDir)
	createNew.Dir = projectRoot
	firstOutput, firstErr := createNew.CombinedOutput()
	if firstErr == nil {
		return worktreeHandle{Path: wtDir, Reused: false}, nil
	}

	attachExisting := exec.Command("git", "worktree", "add", wtDir, branch)
	attachExisting.Dir = projectRoot
	secondOutput, secondErr := attachExisting.CombinedOutput()
	if secondErr == nil {
		return worktreeHandle{Path: wtDir, Reused: false}, nil
	}

	if isGitWorktree(wtDir) {
		return worktreeHandle{Path: wtDir, Reused: true}, nil
	}

	return worktreeHandle{}, fmt.Errorf(
		"git worktree add failed: %s / %s",
		strings.TrimSpace(string(firstOutput)),
		strings.TrimSpace(string(secondOutput)),
	)
}

func cleanupWorktree(projectRoot string, state TaskState, force bool) (string, error) {
	if state.WorktreePath == "" {
		return "", nil
	}

	if state.ReusedWorktree {
		return fmt.Sprintf("Skipped cleanup for reused worktree %s.", state.WorktreePath), nil
	}

	if _, err := os.Stat(state.WorktreePath); os.IsNotExist(err) {
		return fmt.Sprintf("Worktree %s was already removed.", state.WorktreePath), nil
	}

	dirty, err := isWorktreeDirty(state.WorktreePath)
	if err != nil {
		return "", err
	}

	if dirty && !force {
		return "", fmt.Errorf("worktree %s has uncommitted changes; refusing to remove automatically", state.WorktreePath)
	}

	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, state.WorktreePath)

	cmd := exec.Command("git", args...)
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git worktree remove %s: %s", state.WorktreePath, strings.TrimSpace(string(output)))
	}

	return fmt.Sprintf("Removed worktree %s.", state.WorktreePath), nil
}

func isGitWorktree(path string) bool {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--is-inside-work-tree")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(output)) == "true"
}

func isWorktreeDirty(path string) (bool, error) {
	cmd := exec.Command("git", "-C", path, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status %s: %w", path, err)
	}

	return strings.TrimSpace(string(output)) != "", nil
}

// wireHaftMCP injects haft MCP server into the agent's config.
func wireHaftMCP(agentKind string, projectRoot string) error {
	switch AgentKind(agentKind) {
	case AgentClaude:
		return wireHaftMCPClaude(projectRoot)
	case AgentCodex:
		return nil
	default:
		return nil
	}
}

// wireHaftMCPClaude adds haft to ~/.claude.json mcpServers if not present.
func wireHaftMCPClaude(projectRoot string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	haftPath, err := exec.LookPath("haft")
	if err != nil {
		return fmt.Errorf("haft binary not found in PATH")
	}

	configPath := filepath.Join(home, ".claude.json")
	config := make(map[string]interface{})

	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", configPath, err)
	}

	if len(data) > 0 {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("parse %s: %w", configPath, err)
		}
	}

	var servers map[string]interface{}

	raw, exists := config["mcpServers"]
	if exists {
		var ok bool
		servers, ok = raw.(map[string]interface{})
		if !ok {
			return fmt.Errorf("mcpServers in %s is not an object", configPath)
		}
	} else {
		servers = make(map[string]interface{})
		config["mcpServers"] = servers
	}

	if _, has := servers["haft"]; has {
		return nil
	}

	servers["haft"] = map[string]interface{}{
		"type":    "stdio",
		"command": haftPath,
		"args":    []string{"serve", "--project", projectRoot},
	}

	output, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return atomicWriteFile(configPath, append(output, '\n'), 0o644)
}

func appendTaskNote(output string, note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return output
	}

	if output == "" {
		return "[haft] " + note
	}

	return output + "\n[haft] " + note
}

func buildHandoffPrompt(source TaskState, targetAgent string) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("## Task Handoff: %s\n\n", source.Title))
	prompt.WriteString(fmt.Sprintf("Previous agent: %s\n", source.Agent))
	prompt.WriteString(fmt.Sprintf("Target agent: %s\n", targetAgent))
	prompt.WriteString(fmt.Sprintf("Previous status: %s\n", source.Status))
	if source.Project != "" {
		prompt.WriteString(fmt.Sprintf("Project: %s\n", source.Project))
	}
	if source.Branch != "" {
		prompt.WriteString(fmt.Sprintf("Branch: %s\n", source.Branch))
	}
	if source.WorktreePath != "" {
		prompt.WriteString(fmt.Sprintf("Workspace: %s\n", source.WorktreePath))
	}
	prompt.WriteString("\n")

	prompt.WriteString("## Original Brief\n")
	prompt.WriteString(strings.TrimSpace(source.Prompt))
	prompt.WriteString("\n\n")

	prompt.WriteString("## Recent Output Tail\n")
	prompt.WriteString("```text\n")
	prompt.WriteString(lastTaskOutputLines(source.Output, 120))
	prompt.WriteString("\n```\n\n")

	prompt.WriteString("## Instructions\n")
	prompt.WriteString("1. Read the original brief and recent output before touching the code.\n")
	prompt.WriteString("2. Reconstruct the current repo state from the workspace instead of trusting the previous agent blindly.\n")
	prompt.WriteString("3. Continue the work, call out anything already done, and make the remaining risks explicit.\n")
	if source.Status == "running" {
		prompt.WriteString("4. The previous task was still marked running when this handoff was created. Avoid assuming its workspace state is final.\n")
	} else {
		prompt.WriteString("4. Treat the previous output as context, not proof. Verify what landed before you continue.\n")
	}

	return prompt.String()
}

func lastTaskOutputLines(output string, maxLines int) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return "(no task output recorded yet)"
	}

	lines := strings.Split(trimmed, "\n")
	if maxLines <= 0 || len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}

	start := len(lines) - maxLines
	tail := lines[start:]
	return strings.Join(tail, "\n")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[:n-3] + "..."
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
