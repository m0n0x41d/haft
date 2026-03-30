package tui

import "github.com/m0n0x41d/haft/internal/agent"

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
	ProblemTitle string // human-readable problem name
	PortfolioRef string
	DecisionRef  string
	Phase        agent.Phase
	Status       agent.CycleStatus
	REff         float64
}

// OverseerAlertMsg carries background health check findings to the TUI.
type OverseerAlertMsg struct {
	Alerts []string // e.g., ["⚑ 2 drifted", "⏳ 1 expiring"]
}

// LSPUpdateMsg carries language server state to the status bar.
type LSPUpdateMsg struct {
	Servers map[string]string // name → "ready", "starting", "error"
	Errors  int
	Warnings int
}

// ModelSwitchMsg requests a model/provider switch mid-session.
// Sent from the model picker overlay → coordinator swaps LLM provider.
type ModelSwitchMsg struct {
	Model    string // new model ID
	Provider string // provider ID
	APIKey   string // credentials (if newly entered)
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
