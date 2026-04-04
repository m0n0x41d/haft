package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Part — discriminated union for message content.
// Sealed via unexported marker method. Each variant carries typed data.
// ---------------------------------------------------------------------------

// PartKind discriminates Part variants in serialized form.
type PartKind string

const (
	PartKindText       PartKind = "text"
	PartKindToolCall   PartKind = "tool_call"
	PartKindToolResult PartKind = "tool_result"
	PartKindImage      PartKind = "image"
)

// Part is sealed: only types in this package implement it.
type Part interface {
	partKind() PartKind
}

// TextPart carries plain text (user input, assistant response, system prompt).
type TextPart struct {
	Text string `json:"text"`
}

func (TextPart) partKind() PartKind { return PartKindText }

// ToolCallPart represents an LLM-requested tool invocation.
type ToolCallPart struct {
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name"`
	Arguments  string `json:"arguments"` // raw JSON
}

func (ToolCallPart) partKind() PartKind { return PartKindToolCall }

// ToolResultPart carries the output of an executed tool.
type ToolResultPart struct {
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}

func (ToolResultPart) partKind() PartKind { return PartKindToolResult }

type ImagePart struct {
	Filename string `json:"filename"`
	MIMEType string `json:"mime_type"`
	Data     []byte `json:"data"`
}

func (ImagePart) partKind() PartKind { return PartKindImage }

// ---------------------------------------------------------------------------
// Part serialization — JSON with "kind" discriminator.
// ---------------------------------------------------------------------------

type partEnvelope struct {
	Kind PartKind        `json:"kind"`
	Data json.RawMessage `json:"data"`
}

// MarshalParts encodes a Part slice to JSON.
func MarshalParts(parts []Part) ([]byte, error) {
	envelopes := make([]partEnvelope, len(parts))
	for i, p := range parts {
		data, err := json.Marshal(p)
		if err != nil {
			return nil, fmt.Errorf("marshal part %d: %w", i, err)
		}
		envelopes[i] = partEnvelope{Kind: p.partKind(), Data: data}
	}
	return json.Marshal(envelopes)
}

// UnmarshalParts decodes a Part slice from JSON.
func UnmarshalParts(data []byte) ([]Part, error) {
	var envelopes []partEnvelope
	if err := json.Unmarshal(data, &envelopes); err != nil {
		return nil, fmt.Errorf("unmarshal part envelopes: %w", err)
	}
	parts := make([]Part, len(envelopes))
	for i, env := range envelopes {
		var p Part
		var err error
		switch env.Kind {
		case PartKindText:
			var v TextPart
			err = json.Unmarshal(env.Data, &v)
			p = v
		case PartKindToolCall:
			var v ToolCallPart
			err = json.Unmarshal(env.Data, &v)
			p = v
		case PartKindToolResult:
			var v ToolResultPart
			err = json.Unmarshal(env.Data, &v)
			p = v
		case PartKindImage:
			var v ImagePart
			err = json.Unmarshal(env.Data, &v)
			p = v
		default:
			return nil, fmt.Errorf("unknown part kind %q at index %d", env.Kind, i)
		}
		if err != nil {
			return nil, fmt.Errorf("unmarshal part %d (%s): %w", i, env.Kind, err)
		}
		parts[i] = p
	}
	return parts, nil
}

// ---------------------------------------------------------------------------
// Message
// ---------------------------------------------------------------------------

// Role identifies who authored a message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is one turn in the conversation.
type Message struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      Role      `json:"role"`
	Parts     []Part    `json:"parts"`
	Model     string    `json:"model,omitempty"`
	Tokens    int       `json:"tokens,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Text returns the concatenated text of all TextParts.
func (m Message) Text() string {
	var s string
	for _, p := range m.Parts {
		if tp, ok := p.(TextPart); ok {
			s += tp.Text
		}
	}
	return s
}

// ToolCalls returns all ToolCallParts in the message.
func (m Message) ToolCalls() []ToolCallPart {
	var calls []ToolCallPart
	for _, p := range m.Parts {
		if tc, ok := p.(ToolCallPart); ok {
			calls = append(calls, tc)
		}
	}
	return calls
}

func (m *Message) AppendText(delta string) {
	for i, p := range m.Parts {
		if tp, ok := p.(TextPart); ok {
			m.Parts[i] = TextPart{Text: tp.Text + delta}
			return
		}
	}
	m.Parts = append(m.Parts, TextPart{Text: delta})
}

func (m *Message) AppendThinking(delta string) {
	for i, p := range m.Parts {
		if tp, ok := p.(TextPart); ok && strings.HasPrefix(tp.Text, "[thinking]") {
			m.Parts[i] = TextPart{Text: tp.Text + delta}
			return
		}
	}
	m.Parts = append(m.Parts, TextPart{Text: "[thinking]" + delta})
}

func (m Message) Clone() Message {
	parts := make([]Part, len(m.Parts))
	copy(parts, m.Parts)
	m.Parts = parts
	return m
}

// ---------------------------------------------------------------------------
// Session
// ---------------------------------------------------------------------------

// Session is a persistent agent conversation.
type Session struct {
	ID            string      `json:"id"`
	ParentID      string      `json:"parent_id,omitempty"` // non-empty = subagent child session
	Title         string      `json:"title"`
	Model         string      `json:"model"`
	CurrentPhase  Phase       `json:"current_phase,omitempty"` // lemniscate phase state
	Depth         Depth       `json:"depth,omitempty"`         // which phases to include
	Interaction   Interaction `json:"interaction,omitempty"`   // pause between phases?
	Yolo          bool        `json:"yolo,omitempty"`
	ActiveCycleID string      `json:"active_cycle_id,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// ---------------------------------------------------------------------------
// ToolSchema — tool definition sent to the LLM.
// ---------------------------------------------------------------------------

// ToolSchema describes a tool the LLM can call.
type ToolSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema object
}

// ---------------------------------------------------------------------------
// Phase — lemniscate phase identity
// ---------------------------------------------------------------------------

// Phase identifies a lemniscate phase.
type Phase string

const (
	PhaseReady    Phase = ""         // resting state: cycle complete or awaiting first task
	PhaseFramer   Phase = "framer"   // left cycle: characterize + frame the problem
	PhaseExplorer Phase = "explorer" // right cycle: generate distinct variants
	PhaseDecider  Phase = "decider"  // right cycle: compare + select with rationale
	PhaseWorker   Phase = "worker"   // right cycle: implement the chosen approach
	PhaseMeasure  Phase = "measure"  // right cycle: validate against acceptance
)

// ---------------------------------------------------------------------------
// Depth × Interaction — orthogonal session controls.
// Depth determines WHICH phases to include. Interaction determines WHETHER
// to pause between phases. These were previously conflated in LemniscateMode.
// ---------------------------------------------------------------------------

// Depth controls ceremony density within phases (all phases are always mandatory).
// FPF B.5.2: explorer phase cannot be skipped. Ceremony varies in CONTENT, not STRUCTURE.
type Depth string

const (
	DepthStandard Depth = "standard" // all phases: frame → explore → decide → work → measure
	DepthDeep     Depth = "deep"     // standard + parity enforcement, rich evidence reqs
)

// Interaction controls whether the agent pauses between phases.
type Interaction string

const (
	InteractionSymbiotic  Interaction = "symbiotic"  // pause between phases for user input
	InteractionAutonomous Interaction = "autonomous" // auto-chain phases, no pauses
)

// ---------------------------------------------------------------------------
// NavStatus — artifact completeness state for transition validation.
// Values mirror artifact.DerivedStatus. Coordinator maps between them.
// Defined here so phase.go stays pure L2 (no artifact import).
// ---------------------------------------------------------------------------

// NavStatus represents what artifacts exist, used by the transition gate.
type NavStatus string

const (
	NavUnderframed NavStatus = "UNDERFRAMED"
	NavFramed      NavStatus = "FRAMED"
	NavExploring   NavStatus = "EXPLORING"
	NavCompared    NavStatus = "COMPARED"
	NavDecided     NavStatus = "DECIDED"
	NavRefreshDue  NavStatus = "REFRESH_DUE"
)

// AgentDef defines an agent's behavior.
// v2: single unified prompt, no phase pipeline. FPF enforced by tool guardrails.
type AgentDef struct {
	Name         string // agent name (e.g., "haft", "code")
	Lemniscate   bool   // enables FPF cycle tracking (artifact binding, guardrails)
	SystemPrompt string // unified agent prompt (replaces per-phase prompts)
	MaxToolCalls int    // per-turn budget (0 = default 200)
}

// ---------------------------------------------------------------------------
// Permission
// ---------------------------------------------------------------------------

// PermissionLevel determines whether a tool call needs user approval.
type PermissionLevel int

const (
	PermissionAllowed       PermissionLevel = iota // execute without asking
	PermissionNeedsApproval                        // ask user before executing
	PermissionDenied                               // never execute
)

// ---------------------------------------------------------------------------
// Tool Results — typed boundary between tools and coordinator (L2)
// ---------------------------------------------------------------------------

// ToolResult is the typed return value from tool execution.
// DisplayText goes to LLM and user. Meta is consumed by the coordinator only.
type ToolResult struct {
	DisplayText string        // shown to LLM and user
	Meta        *ArtifactMeta // non-nil for artifact-producing tools
}

// ArtifactMeta carries structured artifact identity from tool execution.
// The coordinator uses this to bind cycle refs — no string parsing.
type ArtifactMeta struct {
	Kind        string          // "problem" | "solution" | "decision" | "note" | "evidence"
	ArtifactRef string          // "prob-20260329-004"
	Operation   string          // "frame" | "explore" | "decide" | "measure" | "evidence" | "compare" | "adopt"
	Governance  *GovernanceMeta // non-nil when framer proposes ceremony level

	// Adopt-specific: related refs found for the adopted problem.
	AdoptPortfolioRef string // existing solution portfolio
	AdoptDecisionRef  string // existing decision

	// Measure-specific: verdict from haft_decision(measure).
	MeasureVerdict string // "accepted" | "partial" | "failed"
}

// GovernanceMeta carries the framer's ceremony recommendation.
type GovernanceMeta struct {
	RecommendedDepth Depth
	Rationale        string
}

// PlainResult creates a ToolResult with no artifact metadata.
// Used by non-artifact tools (bash, read, write, edit, glob, grep).
func PlainResult(text string) ToolResult {
	return ToolResult{DisplayText: text}
}

// ---------------------------------------------------------------------------
// Cycle — first-class reasoning cycle entity (L0)
// ---------------------------------------------------------------------------

// CycleStatus tracks the lifecycle of a reasoning cycle.
type CycleStatus string

const (
	CycleActive    CycleStatus = "active"
	CycleComplete  CycleStatus = "complete"
	CycleAbandoned CycleStatus = "abandoned"
)

// Cycle is a single reasoning cycle within a session.
// Binds artifact refs as they're created, tracks governance decisions,
// and carries assurance metrics. Session.ActiveCycleID points here.
type Cycle struct {
	ID           string         `json:"id"`
	SessionID    string         `json:"session_id"`
	ProblemRef   string         `json:"problem_ref,omitempty"`
	PortfolioRef string         `json:"portfolio_ref,omitempty"`
	DecisionRef  string         `json:"decision_ref,omitempty"`
	Phase        Phase          `json:"phase"`
	Depth        Depth          `json:"depth"`
	Status       CycleStatus    `json:"status"`
	LineageRef   string         `json:"lineage_ref,omitempty"` // previous cycle (reframe after measure fail)
	WeakestLink  string         `json:"weakest_link,omitempty"`
	Assurance    AssuranceTuple `json:"assurance"`
	REff         float64        `json:"r_eff"`
	CLMin        int            `json:"cl_min"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`

	// Structured governance and skip records (JSON-serialized in DB)
	Governance []GovernanceEntry `json:"governance,omitempty"`
	SkipLog    []SkipEntry       `json:"skip_log,omitempty"`
}

// GovernanceEntry records a ceremony-level decision.
// Who recommended what, who chose what, and what was skipped.
type GovernanceEntry struct {
	Recommended   Depth       `json:"recommended"`
	Chosen        Depth       `json:"chosen"`
	ChosenBy      string      `json:"chosen_by"` // "user" | "autonomous_delegation"
	Mode          Interaction `json:"mode"`
	SkippedPhases []Phase     `json:"skipped_phases,omitempty"`
	Timestamp     time.Time   `json:"timestamp"`
}

// SkipEntry records why a phase was skipped (FPF B.5.1 CC-B5.1.2).
type SkipEntry struct {
	Phase            Phase  `json:"phase"`
	Reason           string `json:"reason"`
	AcceptedRisk     string `json:"accepted_risk"`
	ResidualEvidence string `json:"residual_evidence"` // what evidence is still required
	ReopenTrigger    string `json:"reopen_trigger"`    // what would trigger reopening
}

// Prediction is a testable claim emitted by the Decider.
// Stored on the Decision artifact. Measure checks each one.
type Prediction struct {
	Claim      string `json:"claim"`
	Observable string `json:"observable"`
	Threshold  string `json:"threshold"`
	Verdict    string `json:"verdict,omitempty"` // "" | "supported" | "refuted" | "inconclusive"
}

// PhaseSpec maps a runtime phase to FPF spec concepts.
type PhaseSpec struct {
	Phase            Phase    `json:"phase"`
	FPFStage         string   `json:"fpf_stage"` // "Explore" | "Shape" | "Evidence" | "Operate"
	ADIRole          string   `json:"adi_role"`  // "abduction" | "deduction" | "induction"
	AllowedCreate    []string `json:"allowed_create"`
	AllowedUpdate    []string `json:"allowed_update"`
	CompletionSignal string   `json:"completion_signal"`
	SkipDepths       []Depth  `json:"skip_depths"` // which depths allow skipping
}
