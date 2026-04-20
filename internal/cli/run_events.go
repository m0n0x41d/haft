package cli

import "time"

// RunEvent is the sum type for all events emitted by the haft run pipeline.
// The TUI (or plain-text fallback) consumes these to render output.
// Exactly one field is non-nil per event.
type RunEvent struct {
	PhaseBegan        *PhaseBegan
	MetaInfo          *MetaInfo
	TaskStatusChanged *TaskStatusChanged
	AgentChunk        *AgentChunk
	BuildResult       *BuildResult
	InvariantResult   *InvariantResult
	PlanLoaded        *PlanLoaded
	Summary           *Summary
	PipelineDone      *PipelineDone
}

// PhaseBegan signals the start of a named pipeline phase (Plan, Execute, Verify, Baseline, etc.).
type PhaseBegan struct {
	Name string
}

// MetaInfo carries a labeled metadata line (decision ref, agent name, mode, etc.).
type MetaInfo struct {
	Label string
	Value string
}

// TaskStatus enumerates the lifecycle states of a plan task.
type TaskStatus int

const (
	TaskPending TaskStatus = iota
	TaskRunning
	TaskPassed
	TaskFailed
	TaskSkipped
)

// TaskStatusChanged reports a task transition (pending→running, running→passed, etc.).
type TaskStatusChanged struct {
	TaskID    string
	TaskTitle string
	Status    TaskStatus
	Elapsed   time.Duration // zero until task completes
	Detail    string        // optional detail (error message on failure)
}

// AgentChunkKind identifies the semantic type of a streaming agent fragment.
type AgentChunkKind int

const (
	ChunkText     AgentChunkKind = iota // assistant text output
	ChunkThinking                       // reasoning/thinking block
	ChunkToolUse                        // tool invocation (name + truncated args)
	ChunkRaw                            // unstructured / fallback
)

// AgentChunk carries one fragment of live agent output.
type AgentChunk struct {
	Kind     AgentChunkKind
	Text     string // content fragment
	ToolName string // populated only for ChunkToolUse
	ToolArgs string // populated only for ChunkToolUse (truncated)
	Done     bool   // true on the final chunk of the agent session
}

// BuildResult reports the outcome of a build/compilation check.
type BuildResult struct {
	Command string
	OK      bool
	Output  string // stderr/stdout on failure
}

// InvariantResult reports the outcome of a single invariant check.
type InvariantResult struct {
	Source string // decision ID the invariant belongs to
	Text   string // invariant description
	Pass   bool
	Reason string // explanation on failure
}

// PlanLoaded signals that a plan has been parsed and is ready for execution.
type PlanLoaded struct {
	DecisionRef string
	Title       string
	TaskCount   int
	PlanFile    string
	Tasks       []PlanTaskSummary
}

// PlanTaskSummary is a slim view of a plan task for display purposes.
type PlanTaskSummary struct {
	ID         string
	Title      string
	Acceptance string
	Files      []string
}

// StatusLevel classifies a summary line's severity.
type StatusLevel int

const (
	StatusOK StatusLevel = iota
	StatusWarn
	StatusFail
)

// Summary carries a status message (ok/warn/fail) for display.
type Summary struct {
	Level   StatusLevel
	Message string
}

// PipelineDone signals that the entire run pipeline has finished.
type PipelineDone struct {
	Elapsed time.Duration
	Success bool
}

// ---------------------------------------------------------------------------
// EventSender wraps a channel for sending RunEvents from the pipeline.
// All methods are non-blocking best-effort sends — a slow consumer does not
// block the pipeline. The channel must be buffered (256 recommended).
// ---------------------------------------------------------------------------

// EventSender emits RunEvents into a channel. Zero business logic — just helpers
// that mirror the current runUI method surface.
type EventSender struct {
	ch chan<- RunEvent
}

// NewEventSender creates an EventSender writing to ch.
func NewEventSender(ch chan<- RunEvent) *EventSender {
	return &EventSender{ch: ch}
}

func (s *EventSender) send(e RunEvent) {
	select {
	case s.ch <- e:
	default:
	}
}

// Phase emits a PhaseBegan event.
func (s *EventSender) Phase(name string) {
	s.send(RunEvent{PhaseBegan: &PhaseBegan{Name: name}})
}

// Meta emits a MetaInfo event.
func (s *EventSender) Meta(label, value string) {
	s.send(RunEvent{MetaInfo: &MetaInfo{Label: label, Value: value}})
}

// TaskStatus emits a TaskStatusChanged event.
func (s *EventSender) TaskStatus(id, title string, status TaskStatus, elapsed time.Duration, detail string) {
	s.send(RunEvent{TaskStatusChanged: &TaskStatusChanged{
		TaskID:    id,
		TaskTitle: title,
		Status:    status,
		Elapsed:   elapsed,
		Detail:    detail,
	}})
}

// Agent emits an AgentChunk event.
func (s *EventSender) Agent(kind AgentChunkKind, text string) {
	s.send(RunEvent{AgentChunk: &AgentChunk{Kind: kind, Text: text}})
}

// AgentTool emits an AgentChunk for a tool invocation.
func (s *EventSender) AgentTool(name, args string) {
	s.send(RunEvent{AgentChunk: &AgentChunk{
		Kind:     ChunkToolUse,
		ToolName: name,
		ToolArgs: args,
	}})
}

// AgentDone emits the terminal AgentChunk.
func (s *EventSender) AgentDone() {
	s.send(RunEvent{AgentChunk: &AgentChunk{Done: true}})
}

// Build emits a BuildResult event.
func (s *EventSender) Build(command string, ok bool, output string) {
	s.send(RunEvent{BuildResult: &BuildResult{
		Command: command,
		OK:      ok,
		Output:  output,
	}})
}

// Invariant emits an InvariantResult event.
func (s *EventSender) Invariant(source, text string, pass bool, reason string) {
	s.send(RunEvent{InvariantResult: &InvariantResult{
		Source: source,
		Text:   text,
		Pass:   pass,
		Reason: reason,
	}})
}

// Plan emits a PlanLoaded event.
func (s *EventSender) Plan(ref, title, planFile string, tasks []PlanTaskSummary) {
	s.send(RunEvent{PlanLoaded: &PlanLoaded{
		DecisionRef: ref,
		Title:       title,
		TaskCount:   len(tasks),
		PlanFile:    planFile,
		Tasks:       tasks,
	}})
}

// OK emits a Summary with StatusOK.
func (s *EventSender) OK(msg string) {
	s.send(RunEvent{Summary: &Summary{Level: StatusOK, Message: msg}})
}

// Warn emits a Summary with StatusWarn.
func (s *EventSender) Warn(msg string) {
	s.send(RunEvent{Summary: &Summary{Level: StatusWarn, Message: msg}})
}

// Fail emits a Summary with StatusFail.
func (s *EventSender) Fail(msg string) {
	s.send(RunEvent{Summary: &Summary{Level: StatusFail, Message: msg}})
}

// Done emits PipelineDone.
func (s *EventSender) Done(elapsed time.Duration, success bool) {
	s.send(RunEvent{PipelineDone: &PipelineDone{
		Elapsed: elapsed,
		Success: success,
	}})
}

// Close closes the underlying channel. Call once after the pipeline finishes.
func (s *EventSender) Close() {
	close(s.ch)
}
