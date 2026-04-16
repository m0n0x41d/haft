package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/creack/pty"
	"github.com/m0n0x41d/haft/internal/artifact"
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
	taskOutputMaxChars      = 64000
	taskOutputFlushInterval = 350 * time.Millisecond
	taskApprovalPulse       = 500 * time.Millisecond
	taskTurnSettleTimeout   = 2 * time.Second
	adoptConfirmationGuard  = "Present options. Do not execute resolution without user confirmation."
)

// InstalledAgent describes a detected agent binary.
type InstalledAgent struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
}

// ChatBlock is the canonical persisted transcript unit for desktop chat tasks.
type ChatBlock struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Role     string `json:"role,omitempty"`
	Text     string `json:"text,omitempty"`
	Name     string `json:"name,omitempty"`
	CallID   string `json:"call_id,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
	Input    string `json:"input,omitempty"`
	Output   string `json:"output,omitempty"`
	IsError  bool   `json:"is_error,omitempty"`
}

// TaskState tracks a running or persisted agent task.
type TaskState struct {
	ID             string      `json:"id"`
	Title          string      `json:"title"`
	Agent          string      `json:"agent"`
	Project        string      `json:"project"`
	ProjectPath    string      `json:"project_path"`
	Status         string      `json:"status"` // pending, running, completed, failed, cancelled, interrupted
	Prompt         string      `json:"prompt"`
	Branch         string      `json:"branch"`
	Worktree       bool        `json:"worktree"`
	WorktreePath   string      `json:"worktree_path"`
	ReusedWorktree bool        `json:"reused_worktree"`
	StartedAt      string      `json:"started_at"`
	CompletedAt    string      `json:"completed_at"`
	ErrorMessage   string      `json:"error_message"`
	Output         string      `json:"output"`      // bounded legacy output tail
	ChatBlocks     []ChatBlock `json:"chat_blocks"` // canonical structured transcript
	RawOutput      string      `json:"raw_output"`  // bounded raw fallback for transcript rendering
	AutoRun        bool        `json:"auto_run"`    // true = agent runs without pausing
}

type TaskOutputEvent struct {
	ID     string `json:"id"`
	Chunk  string `json:"chunk"`
	Output string `json:"output"`
}

// PullRequestResult captures the outcome of the Create PR action.
type PullRequestResult struct {
	TaskID            string   `json:"task_id"`
	DecisionRef       string   `json:"decision_ref"`
	Branch            string   `json:"branch"`
	Title             string   `json:"title"`
	Body              string   `json:"body"`
	URL               string   `json:"url"`
	Pushed            bool     `json:"pushed"`
	DraftCreated      bool     `json:"draft_created"`
	CopiedToClipboard bool     `json:"copied_to_clipboard"`
	Warnings          []string `json:"warnings"`
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
	state      TaskState
	plan       taskRunPlan
	workDir    string
	sessionID  string
	output     *taskOutputBuffer
	transcript *taskTranscript
	turn       *taskTurn
	flushStop  chan struct{}
	flushDone  chan struct{}
	flushOnce  sync.Once
	autoRun    atomic.Bool
}

type taskTurn struct {
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	pty       *os.File
	readDone  chan struct{}
	inputStop chan struct{}
	inputDone chan struct{}
	waitDone  chan struct{}
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

type taskTranscript struct {
	agent         AgentKind
	sessionID     string
	partialLine   string
	blocks        []ChatBlock
	toolBlockIDs  map[string]string
	blockSequence int
}

type parsedTaskLine struct {
	handled   bool
	sessionID string
	blocks    []ChatBlock
}

type claudeStreamEnvelope struct {
	Type            string              `json:"type"`
	Subtype         string              `json:"subtype"`
	SessionID       string              `json:"session_id"`
	Message         claudeStreamMessage `json:"message"`
	ParentToolUseID string              `json:"parent_tool_use_id"`
	Result          json.RawMessage     `json:"result"`
	IsError         bool                `json:"is_error"`
	Error           *claudeStreamError  `json:"error"`
}

type claudeStreamMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type claudeStreamContentBlock struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	ToolUseID string          `json:"tool_use_id"`
	Name      string          `json:"name"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	Data      string          `json:"data"`
	Input     json.RawMessage `json:"input"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

type claudeStreamError struct {
	Message string `json:"message"`
}

type codexStreamEnvelope struct {
	Type       string          `json:"type"`
	ThreadID   string          `json:"thread_id"`
	Text       string          `json:"text"`
	Delta      string          `json:"delta"`
	Message    string          `json:"message"`
	OutputText string          `json:"output_text"`
	Error      string          `json:"error"`
	Item       codexStreamItem `json:"item"`
}

type codexStreamItem struct {
	ID               string          `json:"id"`
	Type             string          `json:"type"`
	Name             string          `json:"name"`
	Server           string          `json:"server"`
	Tool             string          `json:"tool"`
	Status           string          `json:"status"`
	Text             string          `json:"text"`
	Command          string          `json:"command"`
	Query            string          `json:"query"`
	Description      string          `json:"description"`
	AggregatedOutput string          `json:"aggregated_output"`
	ExitCode         *int            `json:"exit_code"`
	Input            json.RawMessage `json:"input"`
	Arguments        json.RawMessage `json:"arguments"`
	Output           json.RawMessage `json:"output"`
	Result           json.RawMessage `json:"result"`
}

type worktreeHandle struct {
	Path   string
	Reused bool
}

type taskRunPlan struct {
	ForceCheckpointed bool
	Conversational    bool
	Verification      *taskVerificationPlan
}

type taskVerificationPlan struct {
	DecisionRef string
}

func defaultTaskRunPlan(agentKind AgentKind) taskRunPlan {
	return taskRunPlan{
		Conversational: supportsTaskResume(agentKind),
	}
}

func mergeTaskRunPlan(base taskRunPlan, override taskRunPlan) taskRunPlan {
	if override.ForceCheckpointed {
		base.ForceCheckpointed = true
	}

	if override.Conversational {
		base.Conversational = true
	}

	if override.Verification != nil {
		base.Verification = override.Verification
	}

	return base
}

var desktopClipboardWriter = func(ctx context.Context, text string) error {
	if ctx == nil {
		return fmt.Errorf("clipboard is unavailable")
	}

	if err := runtime.ClipboardSetText(ctx, text); err != nil {
		return fmt.Errorf("clipboard copy failed: %w", err)
	}

	return nil
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
	items := make([]*runningTask, 0, len(r.tasks))
	for _, rt := range r.tasks {
		if rt.state.Status != "running" {
			continue
		}

		items = append(items, rt)
	}

	r.mu.Unlock()

	for _, rt := range items {
		r.mu.Lock()
		current, ok := r.tasks[rt.state.ID]
		if !ok {
			r.mu.Unlock()
			continue
		}

		current.state.Status = "interrupted"
		if strings.TrimSpace(current.state.ErrorMessage) == "" {
			current.state.ErrorMessage = "Desktop app shut down before the task completed."
		}
		turn := current.turn
		r.mu.Unlock()

		if turn != nil && turn.cancel != nil {
			turn.cancel()
			<-turn.waitDone
			continue
		}

		r.finalizeTask(current, nil)
	}
}

func (r *taskRunner) finishTaskTurn(rt *runningTask, turn *taskTurn, waitErr error) {
	rt.stopInputLoop(turn)
	rt.closePTY(turn)
	rt.waitForOutputReader(turn)

	r.mu.Lock()

	current, ok := r.tasks[rt.state.ID]
	if !ok || current.turn != turn {
		r.mu.Unlock()
		return
	}

	current.turn = nil
	current.state.Output = current.output.String()
	current.state.RawOutput = current.state.Output
	current.state.ChatBlocks = current.transcript.Blocks()

	if current.sessionID == "" {
		current.sessionID = current.transcript.SessionID()
	}

	_ = normalizeTaskState(current.state)

	// Conversational agents (claude/codex) exit after each turn but can be
	// resumed with --resume. Keep the task alive when the process exited
	// cleanly and we have a session ID to resume from.
	keepAlive := current.plan.Conversational &&
		waitErr == nil &&
		strings.TrimSpace(current.sessionID) != ""

	dlog.Debug().
		Bool("keep_alive", keepAlive).
		Str("task_id", rt.state.ID).
		Str("session_id", current.sessionID).
		Bool("conversational", current.plan.Conversational).
		Msg("finishTaskTurn")

	if keepAlive {
		current.turn = nil
		current.state.Status = "idle"

		state := current.state
		state = normalizeTaskState(state)

		r.mu.Unlock()

		if err := r.persistState(state); err != nil {
			r.app.emitAppError("task turn persist", err)
		}

		r.emitTaskStatus(state)
		return
	}

	r.mu.Unlock()

	r.finalizeTask(current, waitErr)
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
		state.RawOutput = state.Output
		state.ChatBlocks = rt.transcript.Blocks()
		state = normalizeTaskState(state)
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

func (r *taskRunner) snapshotTaskState(id string) (TaskState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rt, ok := r.tasks[id]
	if !ok {
		return TaskState{}, false
	}

	state := rt.state
	state.Output = rt.output.String()
	state.RawOutput = state.Output
	state.ChatBlocks = rt.transcript.Blocks()

	return normalizeTaskState(state), true
}

func taskStateFlushSignature(state TaskState) string {
	chatBlocksJSON, err := marshalTaskChatBlocks(state.ChatBlocks)
	if err != nil {
		chatBlocksJSON = fmt.Sprintf("chat-blocks:%d", len(state.ChatBlocks))
	}

	return strings.Join([]string{
		state.Output,
		state.RawOutput,
		chatBlocksJSON,
		state.Status,
		state.ErrorMessage,
		state.CompletedAt,
	}, "\x00")
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
				state, ok := r.snapshotTaskState(rt.state.ID)
				if !ok {
					return
				}

				signature := taskStateFlushSignature(state)
				if signature == lastFlushed {
					continue
				}

				lastFlushed = signature

				if err := r.persistState(state); err != nil {
					r.app.emitAppError("task output persistence", err)
				}
			case <-rt.flushStop:
				state, ok := r.snapshotTaskState(rt.state.ID)
				if !ok {
					return
				}

				signature := taskStateFlushSignature(state)
				if signature != lastFlushed {
					if err := r.persistState(state); err != nil {
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

	return r.store.UpsertTask(context.Background(), normalizeTaskState(state))
}

func (r *taskRunner) emitTaskOutput(id string, chunk string, output string) {
	if r == nil || r.app == nil || !r.app.canEmitEvents() {
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
	if r == nil || r.app == nil || !r.app.canEmitEvents() {
		return
	}

	runtime.EventsEmit(r.app.ctx, "task.status", normalizeTaskState(state))
}

func (r *taskRunner) setAutoRun(id string, autoRun bool) error {
	r.mu.Lock()

	rt, ok := r.tasks[id]
	if !ok {
		r.mu.Unlock()

		// Task might be persisted but not in memory — update store directly
		if r.store != nil {
			return r.store.SetAutoRun(context.Background(), id, autoRun)
		}
		return fmt.Errorf("task not found: %s", id)
	}

	rt.state.AutoRun = autoRun
	rt.autoRun.Store(autoRun)
	state := rt.state

	r.mu.Unlock()

	if r.store != nil {
		if err := r.store.SetAutoRun(context.Background(), id, autoRun); err != nil {
			return err
		}
	}

	r.emitTaskStatus(state)
	return nil
}

func (r *taskRunner) startTaskTurn(rt *runningTask, prompt string, recordUserInput bool) error {
	if r == nil || rt == nil {
		return fmt.Errorf("task runner is not initialized")
	}

	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return fmt.Errorf("task input is empty")
	}

	if recordUserInput {
		display := rt.transcript.AppendLocalBlock(ChatBlock{
			Type: "text",
			Role: "user",
			Text: prompt,
		})
		output := rt.output.Append(display)

		r.mu.Lock()
		current, ok := r.tasks[rt.state.ID]
		if ok {
			current.state.Output = output
			current.state.RawOutput = output
			current.state.ChatBlocks = current.transcript.Blocks()
		}
		r.mu.Unlock()

		if display != "" {
			r.emitTaskOutput(rt.state.ID, display, output)
		}

		if state, ok := r.snapshotTaskState(rt.state.ID); ok {
			if err := r.persistState(state); err != nil {
				r.app.emitAppError("task input persistence", err)
			}

			r.emitTaskStatus(state)
		}
	}

	args := buildAgentTurnArgs(AgentKind(rt.state.Agent), prompt, rt.workDir, rt.sessionID)
	if len(args) == 0 {
		return fmt.Errorf("unsupported agent: %s", rt.state.Agent)
	}

	cfg := defaultDesktopConfig()
	if loadedConfig, err := loadDesktopConfig(); err == nil && loadedConfig != nil {
		cfg = *loadedConfig
	}

	timeout := time.Duration(cfg.TaskTimeoutMinutes) * time.Minute
	if timeout <= 0 {
		timeout = 300 * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = rt.workDir
	cmd.Env = append(
		r.app.userEnv,
		"TERM=xterm-256color",
		"HAFT_PROJECT_ROOT="+rt.state.ProjectPath,
		"HAFT_TASK_ID="+rt.state.ID,
		"HAFT_TASK_BRANCH="+rt.state.Branch,
		"HAFT_TASK_WORKTREE="+rt.state.WorktreePath,
	)

	turn := &taskTurn{
		cmd:       cmd,
		cancel:    cancel,
		readDone:  make(chan struct{}),
		inputStop: make(chan struct{}),
		inputDone: make(chan struct{}),
		waitDone:  make(chan struct{}),
	}

	ptyFile, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: 32,
		Cols: 120,
	})
	if err != nil {
		cancel()
		return fmt.Errorf("start %s: %w", rt.state.Agent, err)
	}

	turn.pty = ptyFile

	r.mu.Lock()
	current, ok := r.tasks[rt.state.ID]
	if !ok {
		r.mu.Unlock()
		_ = ptyFile.Close()
		cancel()
		return fmt.Errorf("task not found: %s", rt.state.ID)
	}

	if current.turn != nil {
		r.mu.Unlock()
		_ = ptyFile.Close()
		cancel()
		return fmt.Errorf("task %s is still processing the previous turn", rt.state.ID)
	}

	current.turn = turn
	current.state.Output = current.output.String()
	current.state.RawOutput = current.state.Output
	current.state.ChatBlocks = current.transcript.Blocks()
	r.mu.Unlock()

	r.startPTYReader(rt, turn)
	rt.startInputLoop(turn)

	go func() {
		waitErr := turn.cmd.Wait()
		r.finishTaskTurn(rt, turn, waitErr)
		close(turn.waitDone)
	}()

	return nil
}

func (r *taskRunner) writeTaskInput(id string, data string) error {
	if r == nil {
		return fmt.Errorf("task runner is not initialized")
	}

	id = strings.TrimSpace(id)
	data = strings.TrimSpace(data)

	if id == "" {
		return fmt.Errorf("task id is required")
	}

	if data == "" {
		return fmt.Errorf("task input is empty")
	}

	for {
		r.mu.Lock()
		rt, ok := r.tasks[id]
		if !ok {
			r.mu.Unlock()
			return fmt.Errorf("task not found: %s", id)
		}

		if rt.state.Status != "running" && rt.state.Status != "idle" {
			status := rt.state.Status
			r.mu.Unlock()
			return fmt.Errorf("task %s is %s, not running", id, status)
		}

		if !rt.plan.Conversational {
			r.mu.Unlock()
			return fmt.Errorf("task %s does not accept follow-up input", id)
		}

		if !supportsTaskResume(AgentKind(rt.state.Agent)) {
			r.mu.Unlock()
			return fmt.Errorf("task %s agent does not support follow-up input", id)
		}

		if rt.sessionID == "" {
			rt.sessionID = rt.transcript.SessionID()
		}

		if strings.TrimSpace(rt.sessionID) == "" {
			r.mu.Unlock()
			return fmt.Errorf("task %s is not ready for follow-up input yet", id)
		}

		if rt.turn == nil {
			rt.state.Status = "running"
			r.mu.Unlock()
			return r.startTaskTurn(rt, data, true)
		}

		waitDone := rt.turn.waitDone
		r.mu.Unlock()

		select {
		case <-waitDone:
			continue
		case <-time.After(taskTurnSettleTimeout):
			return fmt.Errorf("task %s is still processing the previous turn", id)
		}
	}
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
	state.RawOutput = state.Output
	state.ChatBlocks = current.transcript.Blocks()
	state.CompletedAt = nowRFC3339()

	switch {
	case state.Status == "cancelled", state.Status == "interrupted":
		// Preserve explicit shutdown/cancellation state.
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

	if state.Status == "completed" && current.plan.Verification != nil {
		state = r.recordVerificationPass(state, current.plan.Verification)
	}

	state = normalizeTaskState(state)

	if err := r.persistState(state); err != nil {
		r.app.emitAppError("finalize task", err)
	}

	r.emitTaskStatus(state)
	r.app.notifyTaskState(state)
}

func (r *taskRunner) recordVerificationPass(state TaskState, plan *taskVerificationPlan) TaskState {
	if r == nil || r.app == nil || r.app.store == nil || plan == nil {
		return state
	}

	projectRoot := strings.TrimSpace(state.WorktreePath)
	if projectRoot == "" {
		projectRoot = strings.TrimSpace(state.ProjectPath)
	}

	result, err := artifact.RecordVerificationPass(
		context.Background(),
		r.app.store,
		projectRoot,
		artifact.VerificationPassInput{
			DecisionRef: plan.DecisionRef,
			CarrierRef:  taskVerificationCarrierRef(state.ID),
			Summary:     taskVerificationSummary(state),
		},
	)
	if err != nil {
		state.Output = appendTaskNote(
			state.Output,
			fmt.Sprintf("Post-execution verification incomplete: %v", err),
		)
		return state
	}

	state.Status = "Ready for PR"
	state.Output = appendTaskNote(
		state.Output,
		fmt.Sprintf(
			"Post-execution verification passed: baselined %d file(s); evidence %s linked to %s.",
			len(result.Baseline),
			result.Evidence.ID,
			plan.DecisionRef,
		),
	)

	return state
}

func (rt *runningTask) stopFlusher() {
	rt.flushOnce.Do(func() {
		close(rt.flushStop)
		<-rt.flushDone
	})
}

func (rt *runningTask) startInputLoop(turn *taskTurn) {
	if turn == nil || turn.pty == nil {
		return
	}

	ticker := time.NewTicker(taskApprovalPulse)

	go func() {
		defer close(turn.inputDone)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if !rt.autoRun.Load() || turn.pty == nil {
					continue
				}

				if _, err := io.WriteString(turn.pty, "y\n"); err != nil {
					return
				}
			case <-turn.inputStop:
				return
			}
		}
	}()
}

func (rt *runningTask) stopInputLoop(turn *taskTurn) {
	if turn == nil || turn.inputStop == nil {
		return
	}

	close(turn.inputStop)
	<-turn.inputDone
}

func (rt *runningTask) closePTY(turn *taskTurn) {
	if turn == nil || turn.pty == nil {
		return
	}

	_ = turn.pty.Close()
}

func (rt *runningTask) waitForOutputReader(turn *taskTurn) {
	if turn != nil && turn.readDone != nil {
		<-turn.readDone
	}
}

func newTaskTranscript(agent AgentKind) *taskTranscript {
	return &taskTranscript{
		agent:        agent,
		blocks:       []ChatBlock{},
		toolBlockIDs: make(map[string]string),
	}
}

func (t *taskTranscript) AppendChunk(chunk string) string {
	if t == nil {
		return ""
	}

	sanitized := stripANSI(chunk)
	if sanitized == "" {
		return ""
	}

	text := t.partialLine + sanitized
	lines := strings.Split(text, "\n")

	if strings.HasSuffix(text, "\n") {
		t.partialLine = ""
	} else if len(lines) > 0 {
		t.partialLine = lines[len(lines)-1]
		lines = lines[:len(lines)-1]
	}

	var display strings.Builder

	for _, line := range lines {
		display.WriteString(t.appendLine(strings.TrimSuffix(line, "\r")))
	}

	return display.String()
}

func (t *taskTranscript) FlushPendingLine() string {
	if t == nil || t.partialLine == "" {
		return ""
	}

	line := strings.TrimSuffix(t.partialLine, "\r")
	t.partialLine = ""
	return t.appendLine(line)
}

func (t *taskTranscript) SessionID() string {
	if t == nil {
		return ""
	}

	return t.sessionID
}

func (t *taskTranscript) Blocks() []ChatBlock {
	if t == nil || len(t.blocks) == 0 {
		return []ChatBlock{}
	}

	return append([]ChatBlock(nil), t.blocks...)
}

func (t *taskTranscript) AppendLocalBlock(block ChatBlock) string {
	if t == nil {
		return ""
	}

	return t.appendBlock(block)
}

func (t *taskTranscript) appendLine(line string) string {
	parsed := parseTaskStreamLine(t.agent, line)
	if !parsed.handled {
		return t.appendRawLine(line)
	}

	if parsed.sessionID != "" {
		t.sessionID = stripANSI(parsed.sessionID)
	}

	var display strings.Builder

	for _, block := range parsed.blocks {
		display.WriteString(t.appendBlock(block))
	}

	return display.String()
}

func (t *taskTranscript) appendRawLine(line string) string {
	if line == "" {
		return "\n"
	}

	block := ChatBlock{
		Type: "text",
		Text: line,
	}

	return t.appendBlock(block)
}

func (t *taskTranscript) appendBlock(block ChatBlock) string {
	block = normalizeChatBlock(block)

	if block.Type == "" {
		return ""
	}

	if block.ID == "" {
		t.blockSequence++
		block.ID = fmt.Sprintf("block-%d", t.blockSequence)
	}

	if block.Type == "tool_use" && block.CallID != "" {
		t.toolBlockIDs[block.CallID] = block.ID
	}

	if block.Type == "tool_result" && block.ParentID == "" && block.CallID != "" {
		block.ParentID = t.toolBlockIDs[block.CallID]
	}

	t.blocks = append(t.blocks, block)

	return displayTextForBlock(block)
}

func normalizeChatBlock(block ChatBlock) ChatBlock {
	block.ID = stripANSI(block.ID)
	block.Type = stripANSI(block.Type)
	block.Role = stripANSI(block.Role)
	block.Text = stripANSI(block.Text)
	block.Name = stripANSI(block.Name)
	block.CallID = stripANSI(block.CallID)
	block.ParentID = stripANSI(block.ParentID)
	block.Input = stripANSI(block.Input)
	block.Output = stripANSI(block.Output)

	return block
}

func displayTextForBlock(block ChatBlock) string {
	switch block.Type {
	case "text", "thinking":
		text := block.Text
		if block.Type == "text" && block.Role == "user" {
			text = "[user] " + text
		}

		return ensureTrailingNewline(text)
	case "tool_use":
		label := firstNonEmpty(block.Name, "tool_use")
		input := strings.TrimSpace(block.Input)
		if input == "" {
			return ensureTrailingNewline("[tool] " + label)
		}

		return ensureTrailingNewline(fmt.Sprintf("[tool] %s\n%s", label, input))
	case "tool_result":
		if strings.TrimSpace(block.Output) == "" {
			return ""
		}

		return ensureTrailingNewline(block.Output)
	default:
		return ""
	}
}

func displayTextForBlocks(blocks []ChatBlock) string {
	if len(blocks) == 0 {
		return ""
	}

	var display strings.Builder

	for _, block := range blocks {
		display.WriteString(displayTextForBlock(normalizeChatBlock(block)))
	}

	return normalizeTaskOutput(display.String())
}

func ensureTrailingNewline(text string) string {
	if text == "" {
		return ""
	}

	if strings.HasSuffix(text, "\n") {
		return text
	}

	return text + "\n"
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

	snapshot := b.snapshotLocked()
	normalized := normalizeTaskOutput(snapshot)

	if normalized != snapshot {
		b.lines = nil
		b.partial = normalized
	}

	return normalized
}

func (b *taskOutputBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	snapshot := b.snapshotLocked()
	normalized := normalizeTaskOutput(snapshot)

	if normalized != snapshot {
		b.lines = nil
		b.partial = normalized
	}

	return normalized
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

func normalizeTaskOutput(output string) string {
	stripped := stripANSI(output)
	bounded := trimTaskOutputLines(stripped, taskOutputMaxLines)
	bounded = trimTaskOutputRunes(bounded, taskOutputMaxChars)
	return bounded
}

// stripANSI removes ANSI escape sequences from terminal output.
func stripANSI(s string) string {
	// Match: ESC[ ... final byte (0x40-0x7E) — covers CSI sequences
	// Also: ESC] ... ST — covers OSC sequences
	// Also: ESC followed by single char — covers simple escapes
	result := make([]byte, 0, len(s))
	i := 0
	for i < len(s) {
		if s[i] == 0x1b { // ESC
			i++
			if i >= len(s) {
				break
			}
			if s[i] == '[' { // CSI sequence
				i++
				for i < len(s) && s[i] < 0x40 {
					i++
				}
				if i < len(s) {
					i++ // skip final byte
				}
			} else if s[i] == ']' { // OSC sequence
				i++
				for i < len(s) && s[i] != 0x07 && !(i+1 < len(s) && s[i] == 0x1b && s[i+1] == '\\') {
					i++
				}
				if i < len(s) {
					if s[i] == 0x07 {
						i++
					} else if i+1 < len(s) {
						i += 2
					}
				}
			} else {
				i++ // simple escape — skip one char
			}
		} else {
			result = append(result, s[i])
			i++
		}
	}
	return string(result)
}

func parseTaskStreamLine(agent AgentKind, line string) parsedTaskLine {
	line = stripANSI(strings.TrimSuffix(line, "\r"))

	if line == "" {
		return parsedTaskLine{handled: false}
	}

	switch agent {
	case AgentClaude:
		if parsed := parseClaudeStreamLine(line); parsed.handled {
			return parsed
		}
	case AgentCodex:
		if parsed := parseCodexStreamLine(line); parsed.handled {
			return parsed
		}
	}

	return parsedTaskLine{handled: false}
}

func parseClaudeStreamLine(line string) parsedTaskLine {
	var envelope claudeStreamEnvelope
	if err := json.Unmarshal([]byte(line), &envelope); err != nil {
		return parsedTaskLine{}
	}

	switch envelope.Type {
	case "system", "rate_limit_event":
		return parsedTaskLine{handled: true, sessionID: envelope.SessionID}
	case "result":
		resultText := formatJSONText(envelope.Result)
		if strings.TrimSpace(resultText) == "" {
			return parsedTaskLine{handled: true, sessionID: envelope.SessionID}
		}

		role := "assistant"
		if envelope.IsError {
			role = "system"
		}

		return parsedTaskLine{
			handled:   true,
			sessionID: envelope.SessionID,
			blocks: []ChatBlock{{
				Type:    "text",
				Role:    role,
				Text:    resultText,
				IsError: envelope.IsError,
			}},
		}
	case "error":
		errorText := ""
		if envelope.Error != nil {
			errorText = envelope.Error.Message
		}
		errorText = firstNonEmpty(errorText, line)
		return parsedTaskLine{
			handled: true,
			blocks: []ChatBlock{{
				Type: "text",
				Role: "system",
				Text: errorText,
			}},
		}
	case "assistant", "user", "message":
		role := firstNonEmpty(envelope.Message.Role, defaultClaudeMessageRole(envelope.Type))
		contentBlocks := parseClaudeMessageContent(envelope.Message.Content)
		blocks := make([]ChatBlock, 0, len(contentBlocks))

		for _, content := range contentBlocks {
			switch content.Type {
			case "text":
				blocks = append(blocks, ChatBlock{
					Type: "text",
					Role: role,
					Text: firstNonEmpty(content.Text, formatJSONText(content.Content)),
				})
			case "thinking", "redacted_thinking":
				thinking := firstNonEmpty(
					content.Thinking,
					content.Text,
					formatJSONText(content.Content),
				)

				if thinking == "" && content.Type == "redacted_thinking" {
					thinking = "[redacted thinking]"
				}

				blocks = append(blocks, ChatBlock{
					Type: "thinking",
					Role: role,
					Text: thinking,
				})
			case "tool_use":
				blocks = append(blocks, ChatBlock{
					Type:   "tool_use",
					Role:   role,
					Name:   content.Name,
					CallID: firstNonEmpty(content.ID, content.ToolUseID),
					Input:  formatJSONInput(content.Input),
				})
			case "tool_result":
				blocks = append(blocks, ChatBlock{
					Type:    "tool_result",
					Role:    role,
					CallID:  firstNonEmpty(content.ToolUseID, content.ID, envelope.ParentToolUseID),
					Output:  formatJSONText(content.Content),
					IsError: content.IsError,
				})
			default:
				blocks = append(blocks, ChatBlock{
					Type: "text",
					Role: role,
					Text: rawJSONText(content),
				})
			}
		}

		return parsedTaskLine{
			handled:   true,
			sessionID: envelope.SessionID,
			blocks:    blocks,
		}
	default:
		return parsedTaskLine{}
	}
}

func defaultClaudeMessageRole(envelopeType string) string {
	if strings.TrimSpace(envelopeType) == "user" {
		return "user"
	}

	return "assistant"
}

func parseClaudeMessageContent(raw json.RawMessage) []claudeStreamContentBlock {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return []claudeStreamContentBlock{}
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return []claudeStreamContentBlock{{
			Type: "text",
			Text: text,
		}}
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil {
		blocks := make([]claudeStreamContentBlock, 0, len(items))

		for _, item := range items {
			blocks = append(blocks, parseClaudeContentBlock(item))
		}

		return blocks
	}

	return []claudeStreamContentBlock{parseClaudeContentBlock(raw)}
}

func parseClaudeContentBlock(raw json.RawMessage) claudeStreamContentBlock {
	var block claudeStreamContentBlock
	if err := json.Unmarshal(raw, &block); err == nil {
		return normalizeClaudeContentBlock(block)
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return claudeStreamContentBlock{
			Type: "text",
			Text: text,
		}
	}

	var value any
	if err := json.Unmarshal(raw, &value); err == nil {
		return claudeStreamContentBlock{
			Type: "text",
			Text: rawJSONText(value),
		}
	}

	return claudeStreamContentBlock{
		Type: "text",
		Text: stripANSI(strings.TrimSpace(string(raw))),
	}
}

func normalizeClaudeContentBlock(block claudeStreamContentBlock) claudeStreamContentBlock {
	if strings.TrimSpace(block.Type) != "" {
		return block
	}

	switch {
	case strings.TrimSpace(block.Thinking) != "":
		block.Type = "thinking"
	case strings.TrimSpace(block.Text) != "":
		block.Type = "text"
	case strings.TrimSpace(block.Name) != "" || hasJSONValue(block.Input):
		block.Type = "tool_use"
	case strings.TrimSpace(block.ToolUseID) != "" || hasJSONValue(block.Content):
		block.Type = "tool_result"
	}

	return block
}

func parseCodexStreamLine(line string) parsedTaskLine {
	var envelope codexStreamEnvelope
	if err := json.Unmarshal([]byte(line), &envelope); err != nil {
		return parsedTaskLine{}
	}

	switch envelope.Type {
	case "thread.started", "turn.started", "turn.completed":
		return parsedTaskLine{
			handled:   true,
			sessionID: envelope.ThreadID,
		}
	case "turn.failed", "error":
		message := firstNonEmpty(envelope.Error, envelope.Message, envelope.Text, envelope.OutputText, line)
		return parsedTaskLine{
			handled: true,
			blocks: []ChatBlock{{
				Type: "text",
				Role: "system",
				Text: message,
			}},
		}
	case "item.started":
		if block, ok := codexToolUseBlock(envelope.Item); ok {
			return parsedTaskLine{handled: true, blocks: []ChatBlock{block}}
		}

		return parsedTaskLine{handled: true}
	case "item.completed":
		if block, ok := codexNarrativeItemBlock(envelope.Item); ok {
			return parsedTaskLine{handled: true, blocks: []ChatBlock{block}}
		}

		if block, ok := codexToolResultBlock(envelope.Item); ok {
			return parsedTaskLine{handled: true, blocks: []ChatBlock{block}}
		}

		return parsedTaskLine{handled: true}
	case "agent_message", "assistant_message", "assistant_message_delta", "assistant_response", "assistant", "agent_message_delta", "message", "message_delta":
		text := firstNonEmpty(envelope.Text, envelope.Delta, envelope.OutputText, envelope.Message)
		if strings.TrimSpace(text) == "" {
			return parsedTaskLine{handled: true}
		}

		return parsedTaskLine{
			handled: true,
			blocks: []ChatBlock{{
				Type: "text",
				Role: "assistant",
				Text: text,
			}},
		}
	case "reasoning", "reasoning_delta", "reasoning_summary", "reasoning_summary_delta":
		text := firstNonEmpty(envelope.Text, envelope.Delta, envelope.OutputText, envelope.Message)
		if strings.TrimSpace(text) == "" {
			return parsedTaskLine{handled: true}
		}

		return parsedTaskLine{
			handled: true,
			blocks: []ChatBlock{{
				Type: "thinking",
				Role: "assistant",
				Text: text,
			}},
		}
	default:
		return parsedTaskLine{}
	}
}

func codexNarrativeItemBlock(item codexStreamItem) (ChatBlock, bool) {
	text := firstNonEmpty(item.Text, formatJSONText(item.Output), formatJSONText(item.Result))
	if strings.TrimSpace(text) == "" {
		return ChatBlock{}, false
	}

	switch strings.TrimSpace(item.Type) {
	case "agent_message", "assistant_message", "assistant_response", "assistant", "message":
		return ChatBlock{
			Type: "text",
			Role: "assistant",
			Text: text,
		}, true
	case "reasoning", "reasoning_summary":
		return ChatBlock{
			Type: "thinking",
			Role: "assistant",
			Text: text,
		}, true
	case "error":
		return ChatBlock{
			Type:    "text",
			Role:    "system",
			Text:    text,
			IsError: true,
		}, true
	default:
		return ChatBlock{}, false
	}
}

func codexToolUseBlock(item codexStreamItem) (ChatBlock, bool) {
	if !isCodexToolItem(item) {
		return ChatBlock{}, false
	}

	return ChatBlock{
		Type:   "tool_use",
		Role:   "assistant",
		Name:   codexToolName(item),
		CallID: item.ID,
		Input:  codexToolInput(item),
	}, true
}

func codexToolResultBlock(item codexStreamItem) (ChatBlock, bool) {
	if !isCodexToolItem(item) {
		return ChatBlock{}, false
	}

	return ChatBlock{
		Type:    "tool_result",
		Role:    "assistant",
		Name:    codexToolName(item),
		CallID:  item.ID,
		Output:  codexToolOutput(item),
		IsError: codexToolFailed(item),
	}, true
}

func isCodexToolItem(item codexStreamItem) bool {
	itemType := strings.TrimSpace(item.Type)
	if itemType == "" || isCodexNarrativeItemType(itemType) {
		return false
	}

	switch itemType {
	case "command_execution", "mcp_tool_call", "file_change", "web_search", "web_search_call", "todo_list", "file_search", "tool_call":
		return true
	}

	return strings.TrimSpace(item.Command) != "" ||
		strings.TrimSpace(item.Query) != "" ||
		strings.TrimSpace(item.Description) != "" ||
		strings.TrimSpace(item.Server) != "" ||
		strings.TrimSpace(item.Tool) != "" ||
		strings.TrimSpace(item.AggregatedOutput) != "" ||
		hasJSONValue(item.Arguments) ||
		hasJSONValue(item.Input) ||
		hasJSONValue(item.Output) ||
		hasJSONValue(item.Result) ||
		item.ExitCode != nil
}

func isCodexNarrativeItemType(itemType string) bool {
	switch strings.TrimSpace(itemType) {
	case "", "agent_message", "assistant_message", "assistant_response", "assistant", "message", "reasoning", "reasoning_summary", "error":
		return true
	default:
		return false
	}
}

func codexToolName(item codexStreamItem) string {
	if item.Type == "mcp_tool_call" {
		server := strings.TrimSpace(item.Server)
		name := strings.TrimSpace(firstNonEmpty(item.Name, item.Tool, item.Type))
		if server == "" {
			return name
		}

		return server + ":" + name
	}

	return firstNonEmpty(item.Name, item.Tool, item.Type)
}

func hasJSONValue(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

func codexToolInput(item codexStreamItem) string {
	switch {
	case strings.TrimSpace(item.Command) != "":
		return item.Command
	case strings.TrimSpace(item.Query) != "":
		return item.Query
	case strings.TrimSpace(item.Description) != "":
		return item.Description
	}

	input := formatJSONInput(item.Arguments)
	if input != "" {
		return input
	}

	return formatJSONInput(item.Input)
}

func codexToolOutput(item codexStreamItem) string {
	switch {
	case strings.TrimSpace(item.AggregatedOutput) != "":
		return item.AggregatedOutput
	case strings.TrimSpace(item.Text) != "":
		return item.Text
	}

	output := formatJSONText(item.Output)
	if output != "" {
		return output
	}

	return formatJSONText(item.Result)
}

func codexToolFailed(item codexStreamItem) bool {
	status := strings.TrimSpace(strings.ToLower(item.Status))
	if status == "failed" || status == "error" {
		return true
	}

	return item.ExitCode != nil && *item.ExitCode != 0
}

func formatJSONInput(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return stripANSI(trimmed)
	}

	switch typed := value.(type) {
	case string:
		return stripANSI(typed)
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return stripANSI(trimmed)
		}

		return stripANSI(string(data))
	}
}

func formatJSONText(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return stripANSI(trimmed)
	}

	return formatJSONValue(value)
}

func formatJSONValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return stripANSI(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			part := strings.TrimSpace(formatJSONValue(item))
			if part == "" {
				continue
			}

			parts = append(parts, part)
		}

		return strings.Join(parts, "\n")
	case map[string]any:
		content := formatStructuredTextMap(typed)
		if content != "" {
			return content
		}

		data, err := json.Marshal(typed)
		if err != nil {
			return ""
		}

		return stripANSI(string(data))
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		return fmt.Sprintf("%v", typed)
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func formatStructuredTextMap(value map[string]any) string {
	text := stripANSI(firstString(
		value["text"],
		value["thinking"],
		value["summary"],
		value["content"],
	))
	if text != "" {
		return text
	}

	if content, ok := value["content"]; ok {
		return formatJSONValue(content)
	}

	return ""
}

func firstString(values ...any) string {
	for _, value := range values {
		text, ok := value.(string)
		if ok && strings.TrimSpace(text) != "" {
			return text
		}
	}

	return ""
}

func rawJSONText(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}

	return stripANSI(string(data))
}

func trimTaskOutputLines(output string, maxLines int) string {
	if output == "" || maxLines <= 0 {
		return output
	}

	lines := strings.Split(output, "\n")

	if len(lines) <= maxLines {
		return output
	}

	start := len(lines) - maxLines
	tail := lines[start:]
	return strings.Join(tail, "\n")
}

func trimTaskOutputRunes(output string, maxRunes int) string {
	if output == "" || maxRunes <= 0 {
		return output
	}

	runes := []rune(output)

	if len(runes) <= maxRunes {
		return output
	}

	start := len(runes) - maxRunes
	return string(runes[start:])
}

func (r *taskRunner) startPTYReader(rt *runningTask, turn *taskTurn) {
	go func() {
		defer close(turn.readDone)

		if turn == nil || turn.pty == nil {
			return
		}

		buffer := make([]byte, 4096)

		for {
			count, err := turn.pty.Read(buffer)
			if count > 0 {
				chunk := string(buffer[:count])
				displayChunk := rt.transcript.AppendChunk(chunk)
				output := ""
				if displayChunk != "" {
					output = rt.output.Append(displayChunk)
				}

				r.mu.Lock()
				current, ok := r.tasks[rt.state.ID]
				if ok && current.turn == turn {
					if displayChunk == "" {
						output = current.output.String()
					}

					if current.sessionID == "" {
						current.sessionID = current.transcript.SessionID()
					}

					current.state.Output = output
					current.state.RawOutput = output
					current.state.ChatBlocks = current.transcript.Blocks()
				}
				r.mu.Unlock()

				if displayChunk != "" {
					r.emitTaskOutput(rt.state.ID, displayChunk, output)
				}
			}

			if err == nil {
				continue
			}

			if trailing := rt.transcript.FlushPendingLine(); trailing != "" {
				output := rt.output.Append(trailing)

				r.mu.Lock()
				current, ok := r.tasks[rt.state.ID]
				if ok && current.turn == turn {
					current.state.Output = output
					current.state.RawOutput = output
					current.state.ChatBlocks = current.transcript.Blocks()
				}
				r.mu.Unlock()

				r.emitTaskOutput(rt.state.ID, trailing, output)
			}

			if !isTaskTerminalClosed(err) {
				r.app.emitAppError("task output", err)
			}

			return
		}
	}()
}

func isTaskTerminalClosed(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
		return true
	}

	return strings.Contains(err.Error(), "input/output error")
}

func normalizeTaskState(state TaskState) TaskState {
	if state.ChatBlocks == nil {
		state.ChatBlocks = []ChatBlock{}
	}

	for index := range state.ChatBlocks {
		state.ChatBlocks[index] = normalizeChatBlock(state.ChatBlocks[index])
	}

	state.Output = normalizeTaskOutput(state.Output)
	state.RawOutput = normalizeTaskOutput(state.RawOutput)

	if state.Output == "" && state.RawOutput != "" {
		state.Output = state.RawOutput
	}

	if state.RawOutput == "" {
		state.RawOutput = state.Output
	}

	return state
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
	return a.spawnTaskWithTitle(
		agentKind,
		prompt,
		useWorktree,
		branchName,
		"",
		taskRunPlan{Conversational: true},
	)
}

// Implement creates a decision-anchored task in a dedicated worktree.
func (a *App) Implement(decisionRef string) (*TaskState, error) {
	return a.implementDecisionTask(decisionRef, "", "")
}

// Adopt creates a checkpointed investigation task for a governance finding.
func (a *App) Adopt(findingRef string) (*TaskState, error) {
	finding, item, err := a.resolveGovernanceFinding(strings.TrimSpace(findingRef))
	if err != nil {
		return nil, err
	}

	var detail DecisionDetailView
	var prompt string

	if item.Category == artifact.StaleCategoryDecisionStale && len(item.DriftItems) > 0 {
		context, driftErr := a.loadDriftAdoptionContext(finding.ID)
		if driftErr != nil {
			return nil, driftErr
		}

		detail = context.Detail
		prompt = buildAdoptDriftPrompt(*context)
	} else {
		context, staleErr := a.loadStaleAdoptionContext(finding.ID)
		if staleErr != nil {
			return nil, staleErr
		}

		detail = context.Detail
		prompt = buildAdoptStalePrompt(*context)
	}

	prompt = ensureAdoptConfirmationGuard(prompt)

	return a.spawnTaskWithTitle(
		"",
		prompt,
		false,
		"",
		decisionTaskTitle("Adopt", detail),
		taskRunPlan{
			ForceCheckpointed: true,
			Conversational:    true,
		},
	)
}

func ensureAdoptConfirmationGuard(prompt string) string {
	trimmed := strings.TrimSpace(prompt)

	if strings.Contains(trimmed, adoptConfirmationGuard) {
		return trimmed + "\n"
	}

	if trimmed == "" {
		return adoptConfirmationGuard + "\n"
	}

	return trimmed + "\n\n" + adoptConfirmationGuard + "\n"
}

func (a *App) implementDecisionTask(
	decisionID string,
	agentKind string,
	branchName string,
) (*TaskState, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}

	dec, detail, err := a.loadDecisionDetail(decisionID)
	if err != nil {
		return nil, fmt.Errorf("decision not found: %w", err)
	}

	guard := a.buildDecisionImplementGuard(dec)
	if guard.BlockedReason != "" {
		return nil, fmt.Errorf("%s", guard.BlockedReason)
	}

	problems := a.loadDecisionProblems(detail.ProblemRefs)

	// Enrich with invariants from all governing decisions so the task sees
	// the full decision context before editing.
	detail = a.enrichWithGraphInvariants(detail)

	prompt := buildImplementationPrompt(dec, detail, problems, a.loadImplementationPromptContext(dec))
	branchName = decisionFeatureBranchName(
		branchName,
		detail.SelectedTitle,
		detail.Title,
		dec.Meta.Title,
		decisionID,
	)

	return a.spawnTaskWithTitle(
		agentKind,
		prompt,
		true,
		branchName,
		decisionTaskTitle("Implement", detail),
		taskRunPlan{
			ForceCheckpointed: true,
			Conversational:    true,
			Verification:      buildImplementVerificationPlan(dec, detail),
		},
	)
}

func (a *App) loadImplementationPromptContext(decision *artifact.Artifact) implementationPromptContext {
	context := implementationPromptContext{
		WorkflowMarkdown: a.loadWorkflowMarkdown(),
	}

	portfolio := a.loadDecisionPortfolio(decision)
	if portfolio == nil {
		return context
	}

	fields := portfolio.UnmarshalPortfolioFields()
	if fields.Comparison == nil {
		return context
	}

	context.PortfolioRationale = strings.TrimSpace(fields.Comparison.RecommendationRationale)
	return context
}

func (a *App) loadWorkflowMarkdown() string {
	if strings.TrimSpace(a.projectRoot) == "" {
		return ""
	}

	path := filepath.Join(a.projectRoot, ".haft", "workflow.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}

func (a *App) spawnTaskWithTitle(
	agentKind string,
	prompt string,
	useWorktree bool,
	branchName string,
	title string,
	plans ...taskRunPlan,
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

	plan := defaultTaskRunPlan(AgentKind(agentKind))
	if len(plans) > 0 {
		plan = mergeTaskRunPlan(plan, plans[0])
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

	autoRun := cfg.DefaultAutoRun
	if plan.ForceCheckpointed {
		autoRun = false
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
		ChatBlocks:  []ChatBlock{},
		AutoRun:     autoRun,
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

	if !state.AutoRun {
		warnings = append(warnings, "checkpointed mode active: the task waits on agent approval prompts until Auto-run is enabled.")
	}

	args := buildAgentArgs(AgentKind(agentKind), prompt, workDir)
	if len(args) == 0 {
		if state.WorktreePath != "" && !state.ReusedWorktree {
			_, _ = cleanupWorktree(a.projectRoot, state, true)
		}

		return nil, fmt.Errorf("unsupported agent: %s", agentKind)
	}

	// Use task timeout from config to prevent zombie agent processes.
	initialOutput := ""
	if len(warnings) > 0 {
		initialOutput = strings.Join(warnings, "\n") + "\n"
	}

	rt := &runningTask{
		state:      state,
		plan:       plan,
		workDir:    workDir,
		output:     newTaskOutputBuffer(taskOutputMaxLines, initialOutput),
		transcript: newTaskTranscript(AgentKind(agentKind)),
		flushStop:  make(chan struct{}),
		flushDone:  make(chan struct{}),
	}
	rt.autoRun.Store(state.AutoRun)

	rt.state.Output = rt.output.String()
	rt.state = normalizeTaskState(rt.state)

	if err := a.tasks.persistState(rt.state); err != nil {
		if state.WorktreePath != "" && !state.ReusedWorktree {
			_, _ = cleanupWorktree(a.projectRoot, state, true)
		}

		return nil, fmt.Errorf("persist task: %w", err)
	}

	a.tasks.register(rt)
	a.tasks.startOutputFlusher(rt)

	if err := a.tasks.startTaskTurn(rt, prompt, true); err != nil {
		a.tasks.mu.Lock()
		delete(a.tasks.tasks, rt.state.ID)
		a.tasks.mu.Unlock()
		rt.stopFlusher()

		rt.state.Status = "failed"
		rt.state.ErrorMessage = err.Error()
		rt.state.CompletedAt = nowRFC3339()
		rt.state.Output = appendTaskNote(rt.state.Output, err.Error())
		rt.state = normalizeTaskState(rt.state)

		_ = a.tasks.persistState(rt.state)

		if state.WorktreePath != "" && !state.ReusedWorktree {
			_, _ = cleanupWorktree(a.projectRoot, state, true)
		}

		return nil, err
	}

	a.tasks.emitTaskStatus(rt.state)

	result := rt.state
	return &result, nil
}

// ListTasks returns all non-archived tasks for the active project.
func (a *App) ListTasks() ([]TaskState, error) {
	if a.tasks == nil {
		dlog.Debug().Msg("ListTasks: task runner is nil")
		return []TaskState{}, nil
	}

	tasks, err := a.tasks.list(a.ctx, a.projectRoot)
	dlog.Debug().
		Str("project_root", a.projectRoot).
		Int("count", len(tasks)).
		Err(err).
		Msg("ListTasks")
	return tasks, err
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

// WriteTaskInput sends a follow-up message to a running conversational task.
func (a *App) WriteTaskInput(id string, data string) error {
	if a.tasks == nil {
		return fmt.Errorf("no task runner")
	}

	return a.tasks.writeTaskInput(id, data)
}

// CancelTask stops a running task.
func (a *App) CancelTask(id string) error {
	if a.tasks == nil {
		return fmt.Errorf("no tasks")
	}

	a.tasks.mu.Lock()
	rt, ok := a.tasks.tasks[id]
	if ok {
		rt.state.Status = "cancelled"
		rt.state.ErrorMessage = ""
	}
	turn := (*taskTurn)(nil)
	if ok {
		turn = rt.turn
	}
	a.tasks.mu.Unlock()

	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}

	if turn != nil && turn.cancel != nil {
		if stateCopy, ok := a.tasks.snapshotTaskState(id); ok {
			if err := a.tasks.persistState(stateCopy); err != nil {
				return fmt.Errorf("persist cancelled task: %w", err)
			}

			a.tasks.emitTaskStatus(stateCopy)
		}

		turn.cancel()
		return nil
	}

	a.tasks.finalizeTask(rt, nil)
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

// CreatePullRequest pushes a verified implementation branch and opens a draft PR
// when possible. If automatic PR creation is unavailable, the PR body is copied
// to the clipboard for manual creation.
func (a *App) CreatePullRequest(taskID string) (*PullRequestResult, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}

	task, err := a.loadTaskState(strings.TrimSpace(taskID))
	if err != nil {
		return nil, err
	}

	if task.Status != "Ready for PR" {
		return nil, fmt.Errorf("task %s is %s, not Ready for PR", task.ID, task.Status)
	}

	if strings.TrimSpace(task.Branch) == "" {
		return nil, fmt.Errorf("task %s has no branch to publish", task.ID)
	}

	repoPath := firstNonEmpty(
		strings.TrimSpace(task.WorktreePath),
		strings.TrimSpace(task.ProjectPath),
		strings.TrimSpace(a.projectRoot),
	)
	if repoPath == "" {
		return nil, fmt.Errorf("task %s has no repository path", task.ID)
	}

	decisionRef := decisionRefFromTaskPrompt(task.Prompt)
	if decisionRef == "" {
		return nil, fmt.Errorf("task %s does not include a decision reference", task.ID)
	}

	decision, detail, err := a.loadDecisionDetail(decisionRef)
	if err != nil {
		return nil, fmt.Errorf("load decision %s: %w", decisionRef, err)
	}

	verification, err := a.loadTaskVerificationEvidence(decisionRef, task.ID)
	if err != nil {
		return nil, err
	}

	promptContext := a.loadImplementationPromptContext(decision)
	result := &PullRequestResult{
		TaskID:      task.ID,
		DecisionRef: decisionRef,
		Branch:      task.Branch,
		Title:       buildPullRequestTitle(*task, detail),
		Body:        buildPullRequestBody(*task, detail, promptContext.PortfolioRationale, verification),
		Warnings:    []string{},
	}

	pushErr := pushTaskBranch(repoPath, task.Branch)
	if pushErr == nil {
		result.Pushed = true
	} else {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Branch push failed: %v", pushErr))
	}

	if result.Pushed {
		url, draftErr := createDraftPullRequest(repoPath, result.Title, result.Body, task.Branch)
		if draftErr == nil {
			result.DraftCreated = true
			result.URL = url
		} else {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Draft PR creation failed: %v", draftErr))
		}
	}

	if !result.DraftCreated {
		clipboardErr := desktopClipboardWriter(a.ctx, result.Body)
		if clipboardErr == nil {
			result.CopiedToClipboard = true
		} else {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Clipboard copy failed: %v", clipboardErr))
		}
	}

	return result, nil
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
		state.RawOutput = state.Output
		state.ChatBlocks = rt.transcript.Blocks()
		state = normalizeTaskState(state)
		return &state, nil
	}

	state, err := a.tasks.store.GetTask(a.ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load task %s: %w", id, err)
	}

	return state, nil
}

func (a *App) loadTaskVerificationEvidence(decisionRef string, taskID string) (*artifact.EvidenceItem, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}

	items, err := a.store.GetEvidenceItems(a.ctx, decisionRef)
	if err != nil {
		return nil, fmt.Errorf("load evidence for %s: %w", decisionRef, err)
	}

	carrierRef := taskVerificationCarrierRef(taskID)
	for _, item := range items {
		if item.CarrierRef != carrierRef {
			continue
		}
		if item.Type != "audit" {
			continue
		}
		if item.Verdict == "superseded" {
			continue
		}

		copy := item
		return &copy, nil
	}

	return nil, fmt.Errorf("verification evidence for task %s is missing", taskID)
}

func getVersion(path string, flag string) string {
	out, err := exec.Command(path, flag).Output()
	if err != nil {
		return "unknown"
	}

	return strings.TrimSpace(strings.Split(string(out), "\n")[0])
}

func buildAgentArgs(kind AgentKind, prompt string, workDir string) []string {
	return buildAgentTurnArgs(kind, prompt, workDir, "")
}

func buildAgentTurnArgs(kind AgentKind, prompt string, workDir string, sessionID string) []string {
	switch kind {
	case AgentClaude:
		if strings.TrimSpace(sessionID) != "" {
			return buildClaudeResumeArgs(prompt, sessionID)
		}

		return buildClaudeArgs(prompt)
	case AgentCodex:
		if strings.TrimSpace(sessionID) != "" {
			return buildCodexResumeArgs(prompt, sessionID)
		}

		return buildCodexArgs(prompt, workDir)
	case AgentHaft:
		return buildHaftArgs(prompt)
	default:
		return nil
	}
}

func buildClaudeArgs(prompt string) []string {
	args := []string{"claude", "-p"}
	args = append(args, "--verbose")
	args = append(args, "--output-format", "stream-json")
	args = append(args, "--permission-mode", "default")
	args = append(args, prompt)
	return args
}

func buildClaudeResumeArgs(prompt string, sessionID string) []string {
	args := []string{"claude", "-p", "--resume", sessionID}
	args = append(args, "--verbose")
	args = append(args, "--output-format", "stream-json")
	args = append(args, "--permission-mode", "default")
	args = append(args, prompt)
	return args
}

func buildCodexArgs(prompt string, workDir string) []string {
	args := []string{"codex", "exec"}
	args = append(args, "--cd", workDir)
	args = append(args, "--ask-for-approval", "untrusted")
	args = append(args, "--json")
	args = append(args, "-c", "mcp_servers={}")
	args = append(args, prompt)
	return args
}

func buildCodexResumeArgs(prompt string, sessionID string) []string {
	args := []string{"codex", "exec", "resume"}
	args = append(args, "--json")
	args = append(args, sessionID)
	args = append(args, prompt)
	return args
}

func buildHaftArgs(prompt string) []string {
	return []string{"haft", "agent", prompt}
}

func supportsTaskResume(kind AgentKind) bool {
	switch kind {
	case AgentClaude, AgentCodex:
		return true
	default:
		return false
	}
}

func decisionFeatureBranchName(branchName string, labels ...string) string {
	branchName = strings.TrimSpace(branchName)
	if branchName != "" {
		return branchName
	}

	for _, label := range labels {
		slug := branchSlug(label)
		if slug != "" {
			return "feat/" + slug
		}
	}

	return "feat/decision"
}

func branchSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}

	var slug strings.Builder
	lastDash := false

	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			slug.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			slug.WriteRune(r)
			lastDash = false
		case !lastDash && slug.Len() > 0:
			slug.WriteByte('-')
			lastDash = true
		}
	}

	return strings.Trim(slug.String(), "-")
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

func buildImplementVerificationPlan(
	decision *artifact.Artifact,
	detail DecisionDetailView,
) *taskVerificationPlan {
	if decision == nil {
		return nil
	}

	decisionFields := decision.UnmarshalDecisionFields()
	if len(decisionFields.Invariants) == 0 {
		return nil
	}
	if len(detail.AffectedFiles) == 0 {
		return nil
	}

	return &taskVerificationPlan{DecisionRef: decision.Meta.ID}
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

func taskVerificationCarrierRef(taskID string) string {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return ""
	}

	return "desktop-task:" + taskID
}

func taskVerificationSummary(state TaskState) string {
	lines := []string{
		fmt.Sprintf("Task: %s", strings.TrimSpace(state.ID)),
		fmt.Sprintf("Branch: %s", strings.TrimSpace(state.Branch)),
		fmt.Sprintf("Worktree: %s", strings.TrimSpace(state.WorktreePath)),
	}
	filtered := make([]string, 0, len(lines))

	for _, line := range lines {
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			filtered = append(filtered, line)
			continue
		}
		if strings.TrimSpace(parts[1]) == "" {
			continue
		}

		filtered = append(filtered, line)
	}

	return strings.Join(filtered, "\n")
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

func decisionRefFromTaskPrompt(prompt string) string {
	return taskPromptMetaValue(prompt, "Decision ID")
}

func taskPromptMetaValue(prompt string, label string) string {
	prefix := strings.TrimSpace(label) + ":"
	lines := strings.Split(prompt, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}

		return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
	}

	return ""
}

func buildPullRequestTitle(task TaskState, detail DecisionDetailView) string {
	return firstNonEmpty(
		strings.TrimSpace(detail.SelectedTitle),
		strings.TrimSpace(detail.Title),
		strings.TrimSpace(task.Title),
		strings.TrimSpace(task.Branch),
	)
}

func buildPullRequestBody(
	task TaskState,
	detail DecisionDetailView,
	portfolioRationale string,
	verification *artifact.EvidenceItem,
) string {
	var body strings.Builder

	writeSectionTitle(&body, "Summary", buildPullRequestTitle(task, detail))
	writeMetaLine(&body, "Decision ID", detail.ID)
	writeMetaLine(&body, "Branch", task.Branch)
	writeBlankLine(&body)

	writeParagraphSection(
		&body,
		"Decision Rationale",
		buildPullRequestRationale(detail, portfolioRationale),
	)
	writeStringListSection(
		&body,
		"Invariants",
		compactNonEmptyStrings(detail.Invariants),
		"- ",
	)
	writeStringListSection(
		&body,
		"Verification Result",
		buildPullRequestVerificationLines(verification),
		"- ",
	)

	return strings.TrimSpace(body.String())
}

func buildPullRequestRationale(detail DecisionDetailView, portfolioRationale string) string {
	parts := compactNonEmptyStrings([]string{
		strings.TrimSpace(portfolioRationale),
		strings.TrimSpace(detail.WhySelected),
	})

	if strings.TrimSpace(detail.SelectionPolicy) != "" {
		parts = append(parts, "Selection policy: "+strings.TrimSpace(detail.SelectionPolicy))
	}

	return strings.Join(parts, "\n\n")
}

func buildPullRequestVerificationLines(verification *artifact.EvidenceItem) []string {
	if verification == nil {
		return []string{"Verification evidence is unavailable."}
	}

	lines := []string{
		"Post-execution verification passed.",
		fmt.Sprintf("Evidence: %s", verification.ID),
		fmt.Sprintf("Verdict: %s (CL%d)", verification.Verdict, verification.CongruenceLevel),
	}

	contentLines := strings.Split(strings.TrimSpace(verification.Content), "\n")
	for _, line := range contentLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == "Desktop post-execution verification pass recorded." {
			continue
		}
		if strings.HasPrefix(trimmed, "Decision:") {
			continue
		}
		if strings.HasPrefix(trimmed, "Task:") {
			continue
		}
		if strings.HasPrefix(trimmed, "Worktree:") {
			continue
		}

		lines = append(lines, trimmed)
	}

	return compactNonEmptyStrings(lines)
}

func compactNonEmptyStrings(values []string) []string {
	compacted := make([]string, 0, len(values))

	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}

		compacted = append(compacted, trimmed)
	}

	return compacted
}

func pushTaskBranch(repoPath string, branch string) error {
	branchRef := "HEAD:" + strings.TrimSpace(branch)
	attempts := [][]string{
		{"push", "--set-upstream", "origin", branchRef},
		{"push", "origin", branchRef},
	}

	var lastErr error
	for _, args := range attempts {
		output, err := runCommand(repoPath, "git", args...)
		if err == nil {
			return nil
		}

		lastErr = commandFailure("git", args, output, err)
	}

	return lastErr
}

func createDraftPullRequest(repoPath string, title string, body string, branch string) (string, error) {
	file, err := os.CreateTemp("", "haft-pr-body-*.md")
	if err != nil {
		return "", fmt.Errorf("create PR body file: %w", err)
	}
	defer os.Remove(file.Name())

	if _, err := file.WriteString(body); err != nil {
		file.Close()
		return "", fmt.Errorf("write PR body file: %w", err)
	}

	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close PR body file: %w", err)
	}

	output, err := runCommand(
		repoPath,
		"gh",
		"pr",
		"create",
		"--draft",
		"--title",
		title,
		"--body-file",
		file.Name(),
		"--head",
		strings.TrimSpace(branch),
	)
	if err != nil {
		args := []string{
			"pr",
			"create",
			"--draft",
			"--title",
			title,
			"--body-file",
			file.Name(),
			"--head",
			strings.TrimSpace(branch),
		}
		return "", commandFailure("gh", args, output, err)
	}

	return lastNonEmptyLine(output), nil
}

func runCommand(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	return string(output), err
}

func lastNonEmptyLine(value string) string {
	lines := strings.Split(strings.TrimSpace(value), "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimSpace(lines[index])
		if line != "" {
			return line
		}
	}

	return ""
}

func commandFailure(name string, args []string, output string, err error) error {
	parts := compactNonEmptyStrings([]string{
		strings.TrimSpace(output),
		err.Error(),
	})
	message := strings.Join(parts, "; ")
	if message == "" {
		message = "command failed"
	}

	return fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), message)
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
