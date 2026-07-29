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
	CurrentPhase  Phase       `json:"current_phase,omitempty"` // Deprecated: legacy wire field; never inferred or used for routing.
	Depth         Depth       `json:"depth,omitempty"`         // Deprecated: legacy wire field; never selects capabilities.
	Interaction   Interaction `json:"interaction,omitempty"`   // canonical execution mode; kept as interaction during migration
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
// Phase is a legacy wire value retained for decoding persisted sessions and
// cycles created before v9.
// ---------------------------------------------------------------------------

// Deprecated: capability availability, artifact relations, and work order must
// not be inferred from Phase.
type Phase string

const (
	PhaseReady    Phase = ""
	PhaseFramer   Phase = "framer"
	PhaseExplorer Phase = "explorer"
	PhaseDecider  Phase = "decider"
	PhaseWorker   Phase = "worker"
	PhaseMeasure  Phase = "measure"
)

// ---------------------------------------------------------------------------
// Legacy Depth and current interaction controls.
// ---------------------------------------------------------------------------

// Depth is retained only to decode persisted pre-v9 sessions and cycles.
//
// Deprecated: it does not include, skip, or order capabilities.
type Depth string

const (
	DepthStandard Depth = "standard"
	DepthDeep     Depth = "deep"
)

// ExecutionMode is the canonical session/runtime execution vocabulary.
// Guardrails still refer to this as "interaction" during migration.
type ExecutionMode string

const (
	ExecutionModeCheckpointed ExecutionMode = "checkpointed" // pause at checkpoints for user input
	ExecutionModeAutonomous   ExecutionMode = "autonomous"   // perform delegated actions without checkpoints
)

const (
	legacyExecutionModeCollaborative = "collaborative"
	legacyExecutionModeSymbiotic     = "symbiotic"
)

// Legacy aliases kept so the surrounding runtime can migrate incrementally.
const (
	ExecutionModeCollaborative ExecutionMode = ExecutionModeCheckpointed
	ExecutionModeSymbiotic     ExecutionMode = ExecutionModeCheckpointed
)

// Interaction is the legacy field name used by existing guardrail code.
type Interaction = ExecutionMode

const (
	InteractionCheckpointed  = ExecutionModeCheckpointed
	InteractionCollaborative = ExecutionModeCheckpointed
	InteractionSymbiotic     = ExecutionModeCheckpointed
	InteractionAutonomous    = ExecutionModeAutonomous
)

func ParseExecutionMode(raw string) (ExecutionMode, bool) {
	switch raw {
	case string(ExecutionModeCheckpointed), legacyExecutionModeCollaborative, legacyExecutionModeSymbiotic:
		return ExecutionModeCheckpointed, true
	case string(ExecutionModeAutonomous):
		return ExecutionModeAutonomous, true
	default:
		return ExecutionModeCheckpointed, false
	}
}

func NormalizeExecutionMode(raw string) ExecutionMode {
	mode, ok := ParseExecutionMode(raw)
	if ok {
		return mode
	}
	return ExecutionModeCheckpointed
}

func ExecutionModeFromAutonomous(enabled bool) ExecutionMode {
	if enabled {
		return ExecutionModeAutonomous
	}
	return ExecutionModeCheckpointed
}

func (s Session) ExecutionMode() ExecutionMode {
	return NormalizeExecutionMode(string(s.Interaction))
}

func (s *Session) SetExecutionMode(mode ExecutionMode) {
	s.Interaction = NormalizeExecutionMode(string(mode))
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
// Warnings carry FPF-discipline soft-violations the tool detected but
// chose not to reject. Providers SHOULD render warnings to the model
// as part of the tool result so the agent can self-correct without an
// operator intervention.
type ToolResult struct {
	DisplayText string        // shown to LLM and user
	Meta        *ArtifactMeta // non-nil for artifact-producing tools
	Warnings    []string      // FPF soft violations + advisory notes
}

// ArtifactMeta carries structured artifact identity from tool execution.
// Legacy cycle fields remain available to old hosts, but refs do not imply a
// causal, temporal, or work-order relation.
type ArtifactMeta struct {
	Kind        string          // "problem" | "solution" | "decision" | "note" | "evidence"
	ArtifactRef string          // "prob-20260329-004"
	Operation   string          // "frame" | "explore" | "decide" | "measure" | "evidence" | "compare" | "adopt"
	Governance  *GovernanceMeta // non-nil when framer proposes ceremony level

	// Adopt-specific: related refs found for the adopted problem.
	AdoptPortfolioRef    string // existing solution portfolio
	ComparedPortfolioRef string // exact portfolio with persisted comparison data
	AdoptDecisionRef     string // existing decision

	// Measure-specific: verdict from haft_decision(measure).
	MeasureVerdict string // "accepted" | "partial" | "failed"
}

// GovernanceMeta is retained for decoding pre-v9 tool results.
//
// Deprecated: no capability routing is derived from RecommendedDepth.
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
// Cycle — legacy persisted compatibility entity
// ---------------------------------------------------------------------------

// CycleStatus is retained for persisted pre-v9 cycles.
type CycleStatus string

const (
	CycleActive    CycleStatus = "active"
	CycleComplete  CycleStatus = "complete"
	CycleAbandoned CycleStatus = "abandoned"
)

// Cycle is retained for decoding and updating persisted pre-v9 cycle records.
// This compatibility type must not be used to infer a mandatory project
// workflow from refs, Phase, Depth, Status, or their storage order. Each ref
// names only its stated artifact relation.
type Cycle struct {
	ID                   string         `json:"id"`
	SessionID            string         `json:"session_id"`
	ProblemRef           string         `json:"problem_ref,omitempty"`
	PortfolioRef         string         `json:"portfolio_ref,omitempty"`
	ComparedPortfolioRef string         `json:"compared_portfolio_ref,omitempty"`
	SelectedPortfolioRef string         `json:"selected_portfolio_ref,omitempty"`
	SelectedVariantRef   string         `json:"selected_variant_ref,omitempty"`
	DecisionRef          string         `json:"decision_ref,omitempty"`
	Phase                Phase          `json:"phase"` // Deprecated: legacy wire field; not inferred.
	Depth                Depth          `json:"depth"` // Deprecated: legacy wire field; not used for routing.
	Status               CycleStatus    `json:"status"`
	LineageRef           string         `json:"lineage_ref,omitempty"` // previous cycle (reframe after measure fail)
	WeakestLink          string         `json:"weakest_link,omitempty"`
	Assurance            AssuranceTuple `json:"assurance"`
	REff                 float64        `json:"r_eff"`
	CLMin                int            `json:"cl_min"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`

	// Structured governance and skip records (JSON-serialized in DB)
	Governance []GovernanceEntry `json:"governance,omitempty"`
	SkipLog    []SkipEntry       `json:"skip_log,omitempty"`
}

// GovernanceEntry is a legacy persisted compatibility record.
//
// Deprecated: it must not be used to infer a current capability or work order.
type GovernanceEntry struct {
	Recommended   Depth       `json:"recommended"`
	Chosen        Depth       `json:"chosen"`
	ChosenBy      string      `json:"chosen_by"` // "user" | "autonomous_delegation"
	Mode          Interaction `json:"mode"`
	SkippedPhases []Phase     `json:"skipped_phases,omitempty"`
	Timestamp     time.Time   `json:"timestamp"`
}

// SkipEntry is a legacy persisted compatibility record.
//
// Deprecated: v9 has no mandatory project phases to skip.
type SkipEntry struct {
	Phase            Phase  `json:"phase"`
	Reason           string `json:"reason"`
	AcceptedRisk     string `json:"accepted_risk"`
	ResidualEvidence string `json:"residual_evidence"` // what evidence is still required
	ReopenTrigger    string `json:"reopen_trigger"`    // what would trigger reopening
}
