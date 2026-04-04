// Package protocol defines the L2 message contract between Go backend and TS TUI.
// Pure types — no I/O, no dependencies beyond standard library.
package protocol

// ---------------------------------------------------------------------------
// Backend → TUI notifications
// ---------------------------------------------------------------------------

// Init bootstraps the TUI on startup.
type Init struct {
	Session     SessionInfo `json:"session"`
	ProjectRoot string      `json:"projectRoot"`
	Width       int         `json:"width"`
	Height      int         `json:"height"`
	Messages    []MsgInfo   `json:"messages,omitempty"`
}

type SessionInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Model string `json:"model"`
}

type MsgInfo struct {
	ID       string     `json:"id"`
	Role     string     `json:"role"` // "user" | "assistant"
	Text     string     `json:"text"`
	Thinking string     `json:"thinking,omitempty"`
	Tools    []ToolCall `json:"tools,omitempty"`
}

// MsgUpdate carries streaming assistant message state.
type MsgUpdate struct {
	ID        string     `json:"id"`
	Text      string     `json:"text"`
	Thinking  string     `json:"thinking,omitempty"`
	Tools     []ToolCall `json:"tools,omitempty"`
	Streaming bool       `json:"streaming"`
}

type ToolCall struct {
	CallID     string     `json:"callId"`
	Name       string     `json:"name"`
	Args       string     `json:"args"`
	Output     string     `json:"output,omitempty"`
	IsError    bool       `json:"isError,omitempty"`
	Running    bool       `json:"running"`
	SubagentID string     `json:"subagentId,omitempty"`
	Children   []ToolCall `json:"children,omitempty"`
}

// ToolStart signals a tool call is about to execute.
type ToolStart struct {
	CallID     string `json:"callId"`
	Name       string `json:"name"`
	Args       string `json:"args"`
	SubagentID string `json:"subagentId,omitempty"`
}

// ToolProgress carries streaming tool output (e.g. bash lines).
type ToolProgress struct {
	CallID string `json:"callId"`
	Text   string `json:"text"`
}

// ToolDone signals tool completion.
type ToolDone struct {
	CallID     string `json:"callId"`
	Name       string `json:"name"`
	Output     string `json:"output"`
	IsError    bool   `json:"isError"`
	SubagentID string `json:"subagentId,omitempty"`
}

// TokenUpdate carries token consumption.
type TokenUpdate struct {
	Used  int `json:"used"`
	Limit int `json:"limit"`
}

// SessionTitle updates the session title.
type SessionTitle struct {
	Title string `json:"title"`
}

// CycleUpdate carries FPF lemniscate cycle state.
type CycleUpdate struct {
	CycleID      string  `json:"cycleId"`
	ProblemRef   string  `json:"problemRef"`
	ProblemTitle string  `json:"problemTitle"`
	PortfolioRef string  `json:"portfolioRef,omitempty"`
	DecisionRef  string  `json:"decisionRef,omitempty"`
	Phase        string  `json:"phase"`  // frame|explore|compare|decide|implement|measure
	Status       string  `json:"status"` // active|complete|abandoned
	REff         float64 `json:"rEff"`
}

// SubagentStart signals a subagent was spawned.
type SubagentStart struct {
	SubagentID   string `json:"subagentId"`
	ParentCallID string `json:"parentCallId"`
	Name         string `json:"name"`
	Task         string `json:"task"`
}

// SubagentDone signals a subagent completed.
type SubagentDone struct {
	SubagentID string `json:"subagentId"`
	Summary    string `json:"summary"`
	IsError    bool   `json:"isError"`
}

// OverseerAlert carries background health findings.
type OverseerAlert struct {
	Alerts   []string          `json:"alerts"`
	Findings []OverseerFinding `json:"findings,omitempty"`
}

type OverseerFinding struct {
	Type          string                  `json:"type"`
	Category      string                  `json:"category,omitempty"`
	ArtifactID    string                  `json:"artifactId,omitempty"`
	Title         string                  `json:"title,omitempty"`
	Kind          string                  `json:"kind,omitempty"`
	Summary       string                  `json:"summary"`
	Reason        string                  `json:"reason,omitempty"`
	DaysStale     int                     `json:"daysStale,omitempty"`
	REff          float64                 `json:"rEff,omitempty"`
	TotalED       float64                 `json:"totalED,omitempty"`
	Budget        float64                 `json:"budget,omitempty"`
	Excess        float64                 `json:"excess,omitempty"`
	DriftItems    []OverseerDriftItem     `json:"driftItems,omitempty"`
	DebtBreakdown []OverseerDebtBreakdown `json:"debtBreakdown,omitempty"`
}

type OverseerDriftItem struct {
	Path         string   `json:"path"`
	Status       string   `json:"status"`
	LinesChanged string   `json:"linesChanged,omitempty"`
	Invariants   []string `json:"invariants,omitempty"`
}

type OverseerDebtBreakdown struct {
	DecisionID      string  `json:"decisionId"`
	DecisionTitle   string  `json:"decisionTitle"`
	TotalED         float64 `json:"totalED"`
	ExpiredEvidence int     `json:"expiredEvidence"`
	MostOverdueDays int     `json:"mostOverdueDays"`
}

// DriftUpdate carries file/spec drift state.
type DriftUpdate struct {
	Drifted  int `json:"drifted"`
	Stale    int `json:"stale"`
	Coverage int `json:"coverage"` // percentage
}

// LSPUpdate carries language server state.
type LSPUpdate struct {
	Servers  map[string]string `json:"servers"`
	Errors   int               `json:"errors"`
	Warnings int               `json:"warnings"`
}

// Error is a recoverable error message.
type Error struct {
	Message string `json:"message"`
}

// CoordDone signals end of a coordinator turn.
type CoordDone struct{}

// ---------------------------------------------------------------------------
// Backend → TUI requests (expect response)
// ---------------------------------------------------------------------------

// PermissionAsk requests tool approval from the user.
type PermissionAsk struct {
	ToolName    string `json:"toolName"`
	Args        string `json:"args"`
	Description string `json:"description"`
	FilePath    string `json:"filePath,omitempty"`
	Diff        string `json:"diff,omitempty"`
	Adds        int    `json:"adds,omitempty"`
	Dels        int    `json:"dels,omitempty"`
}

// PermissionReply is the TUI's response to PermissionAsk.
type PermissionReply struct {
	Action string `json:"action"` // "allow" | "allow_session" | "deny"
}

// QuestionAsk is AskUserQuestion — agent asks the user something.
type QuestionAsk struct {
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"`
}

// QuestionReply is the user's answer.
type QuestionReply struct {
	Answer string `json:"answer"`
}
