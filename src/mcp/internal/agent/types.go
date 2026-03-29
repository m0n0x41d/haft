package agent

import (
	"encoding/json"
	"fmt"
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

// ---------------------------------------------------------------------------
// Session
// ---------------------------------------------------------------------------

// Session is a persistent agent conversation.
type Session struct {
	ID           string      `json:"id"`
	ParentID     string      `json:"parent_id,omitempty"` // non-empty = subagent child session
	Title        string      `json:"title"`
	Model        string      `json:"model"`
	CurrentPhase Phase       `json:"current_phase,omitempty"` // lemniscate phase state
	Depth        Depth       `json:"depth,omitempty"`         // which phases to include
	Interaction  Interaction `json:"interaction,omitempty"`   // pause between phases?
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
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

// Depth controls which lemniscate phases are included.
type Depth string

const (
	DepthTactical Depth = "tactical" // frame → decide → work → measure (skip explorer)
	DepthStandard Depth = "standard" // frame → explore → decide → work → measure
	DepthDeep     Depth = "deep"     // standard + parity enforcement, evidence reqs
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

// PhaseDef defines a single lemniscate phase — its prompt, tools, and identity.
type PhaseDef struct {
	Phase        Phase    // phase identity
	Name         string   // display name (e.g., "haft-framer")
	SystemPrompt string   // phase-specific system prompt
	AllowedTools []string // tool allowlist (deterministic gating)
	MaxToolCalls int      // per-phase budget (0 = use global default)
}

// AgentDef defines an agent's behavior. The default "haft" agent has Lemniscate=true.
type AgentDef struct {
	Name         string // agent name (e.g., "haft", "code")
	Lemniscate   bool   // enables phase transitions
	DefaultDepth Depth  // recommended depth for this agent
	Phases       []PhaseDef
}

// PhaseByID returns the PhaseDef for a given phase, or nil if not found.
func (a AgentDef) PhaseByID(phase Phase) *PhaseDef {
	for i := range a.Phases {
		if a.Phases[i].Phase == phase {
			return &a.Phases[i]
		}
	}
	return nil
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
