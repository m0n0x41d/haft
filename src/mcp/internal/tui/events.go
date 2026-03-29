package tui

import "github.com/m0n0x41d/quint-code/internal/agent"

// Events flow from Coordinator -> TUI via the Bus.
// Each event type implements tea.Msg (any type does in BubbleTea v2).

// StreamDeltaMsg carries a text chunk from the streaming LLM response.
type StreamDeltaMsg struct {
	Text string
}

// ThinkingDeltaMsg carries a reasoning/thinking text chunk.
type ThinkingDeltaMsg struct {
	Text string
}

// StreamDoneMsg signals the end of an LLM response.
type StreamDoneMsg struct {
	Message agent.Message
}

// ToolStartMsg signals a tool call is about to execute.
type ToolStartMsg struct {
	ToolCallID string
	ToolName   string
	Args       string
	SubagentID string // non-empty = child tool call (renders nested under spawn)
}

// ToolDoneMsg signals a tool call has completed.
type ToolDoneMsg struct {
	ToolCallID string
	ToolName   string
	Output     string
	IsError    bool
	SubagentID string // non-empty = child tool call
}

// PermissionAskMsg requests user approval for a tool call.
// The coordinator blocks on Reply until the TUI responds.
type PermissionAskMsg struct {
	ToolName string
	Args     string
	Reply    chan<- bool
}

// ErrorMsg signals a recoverable error.
type ErrorMsg struct {
	Err error
}

// PhaseChangeMsg signals a lemniscate phase transition.
type PhaseChangeMsg struct {
	From agent.Phase
	To   agent.Phase
	Name string // display name (e.g., "haft-worker")
}

// PhasePauseMsg signals the coordinator paused for user approval.
// Reply true = proceed to next phase. Reply false = stay, let user discuss.
type PhasePauseMsg struct {
	Phase   agent.Phase
	Summary string
	Reply   chan<- bool
}

// TokenUpdateMsg carries token usage info to the TUI.
type TokenUpdateMsg struct {
	Used  int
	Limit int
}

// SessionTitleMsg updates the session title in the TUI.
type SessionTitleMsg struct {
	Title string
}

// AutonomyToggleMsg signals the user toggled autonomous mode.
type AutonomyToggleMsg struct {
	Autonomous bool
}

// ---------------------------------------------------------------------------
// Subagent events
// ---------------------------------------------------------------------------

// SubagentStartMsg signals a subagent was spawned.
type SubagentStartMsg struct {
	SubagentID string // unique ID for this subagent instance
	Name       string // "explore", "title", "compact"
	Task       string // what the subagent was asked to do
}

// SubagentDoneMsg signals a subagent completed.
type SubagentDoneMsg struct {
	SubagentID string
	Summary    string // compressed result
	IsError    bool
}

// CycleUpdateMsg notifies the TUI that the active cycle changed.
type CycleUpdateMsg struct {
	CycleID      string
	ProblemRef   string
	PortfolioRef string
	DecisionRef  string
	Phase        agent.Phase
	Status       agent.CycleStatus
}

// CoordinatorDoneMsg signals the coordinator goroutine has finished.
type CoordinatorDoneMsg struct{}

// clearNotificationMsg clears the status bar notification after a timeout.
type clearNotificationMsg struct{}

// clearQuitConfirmMsg resets the quit confirmation state after timeout.
type clearQuitConfirmMsg struct{}

// compactDoneMsg signals that forced compaction completed.
type compactDoneMsg struct {
	before int
	after  int
	err    error
}
