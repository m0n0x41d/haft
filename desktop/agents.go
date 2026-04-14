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
	adoptConfirmationGuard  = "Present options. Do not execute resolution without user confirmation."
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
	Output         string `json:"output"`   // bounded output tail
	AutoRun        bool   `json:"auto_run"` // true = agent runs without pausing
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
	state     TaskState
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	plan      taskRunPlan
	output    *taskOutputBuffer
	pty       *os.File
	flushStop chan struct{}
	flushDone chan struct{}
	flushOnce sync.Once
	readDone  chan struct{}
	inputStop chan struct{}
	inputDone chan struct{}
	inputOnce sync.Once
	ptyOnce   sync.Once
	autoRun   atomic.Bool
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

type taskRunPlan struct {
	ForceCheckpointed bool
	Verification      *taskVerificationPlan
}

type taskVerificationPlan struct {
	DecisionRef string
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
		item.rt.stopInputLoop()
		item.rt.closePTY()
		item.rt.waitForOutputReader()
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

	runtime.EventsEmit(r.app.ctx, "task.status", state)
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

func (r *taskRunner) finalizeTask(rt *runningTask, waitErr error) {
	rt.stopInputLoop()
	rt.closePTY()
	rt.waitForOutputReader()
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

	if state.Status == "completed" && current.plan.Verification != nil {
		state = r.recordVerificationPass(state, current.plan.Verification)
	}

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

func (rt *runningTask) startInputLoop() {
	ticker := time.NewTicker(taskApprovalPulse)

	go func() {
		defer close(rt.inputDone)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if !rt.autoRun.Load() || rt.pty == nil {
					continue
				}

				if _, err := io.WriteString(rt.pty, "y\n"); err != nil {
					return
				}
			case <-rt.inputStop:
				return
			}
		}
	}()
}

func (rt *runningTask) stopInputLoop() {
	rt.inputOnce.Do(func() {
		close(rt.inputStop)
		<-rt.inputDone
	})
}

func (rt *runningTask) closePTY() {
	rt.ptyOnce.Do(func() {
		if rt.pty != nil {
			_ = rt.pty.Close()
		}
	})
}

func (rt *runningTask) waitForOutputReader() {
	if rt.readDone != nil {
		<-rt.readDone
	}
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

func (r *taskRunner) startPTYReader(rt *runningTask) {
	go func() {
		defer close(rt.readDone)

		if rt.pty == nil {
			return
		}

		buffer := make([]byte, 4096)

		for {
			count, err := rt.pty.Read(buffer)
			if count > 0 {
				chunk := string(buffer[:count])
				output := rt.output.Append(chunk)
				r.emitTaskOutput(rt.state.ID, chunk, output)
			}

			if err == nil {
				continue
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
		taskRunPlan{ForceCheckpointed: true},
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

	plan := taskRunPlan{}
	if len(plans) > 0 {
		plan = plans[0]
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
	timeout := time.Duration(cfg.TaskTimeoutMinutes) * time.Minute
	if timeout <= 0 {
		timeout = 300 * time.Minute // fallback: 5 hours
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = workDir
	cmd.Env = append(
		os.Environ(),
		"TERM=xterm-256color",
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
		plan:      plan,
		output:    newTaskOutputBuffer(taskOutputMaxLines, initialOutput),
		flushStop: make(chan struct{}),
		flushDone: make(chan struct{}),
		readDone:  make(chan struct{}),
		inputStop: make(chan struct{}),
		inputDone: make(chan struct{}),
	}
	rt.autoRun.Store(state.AutoRun)

	rt.state.Output = rt.output.String()

	if err := a.tasks.persistState(rt.state); err != nil {
		cancel()

		if state.WorktreePath != "" && !state.ReusedWorktree {
			_, _ = cleanupWorktree(a.projectRoot, state, true)
		}

		return nil, fmt.Errorf("persist task: %w", err)
	}

	ptyFile, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: 32,
		Cols: 120,
	})
	if err != nil {
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
	rt.pty = ptyFile

	a.tasks.register(rt)
	a.tasks.startPTYReader(rt)
	rt.startInputLoop()
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
	switch kind {
	case AgentClaude:
		return buildClaudeArgs(prompt)
	case AgentCodex:
		return buildCodexArgs(prompt, workDir)
	case AgentHaft:
		return buildHaftArgs(prompt)
	default:
		return nil
	}
}

func buildClaudeArgs(prompt string) []string {
	args := []string{"claude", "-p", prompt}
	args = append(args, "--verbose")
	args = append(args, "--output-format", "text")
	args = append(args, "--permission-mode", "default")
	return args
}

func buildCodexArgs(prompt string, workDir string) []string {
	args := []string{"codex", "exec"}
	args = append(args, "--cd", workDir)
	args = append(args, "--full-auto")
	args = append(args, "-c", "mcp_servers={}")
	args = append(args, prompt)
	return args
}

func buildHaftArgs(prompt string) []string {
	return []string{"haft", "agent", prompt}
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
