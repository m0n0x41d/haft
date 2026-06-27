package method

import "time"

const (
	CatalogID      = "swe-core"
	CatalogVersion = "1.0.0"
)

type Definition struct {
	ID                  string        `json:"id" yaml:"id"`
	Version             string        `json:"version" yaml:"version"`
	Title               string        `json:"title" yaml:"title"`
	Summary             string        `json:"summary" yaml:"summary"`
	Intent              string        `json:"intent" yaml:"intent"`
	ProblemContext      string        `json:"problem_context,omitempty" yaml:"problem_context,omitempty"`
	FirstUsefulMove     string        `json:"first_useful_move,omitempty" yaml:"first_useful_move,omitempty"`
	ExpectedOutputKinds []string      `json:"expected_output_kinds,omitempty" yaml:"expected_output_kinds,omitempty"`
	FitFunctionRefs     []string      `json:"fit_function_refs,omitempty" yaml:"fit_function_refs,omitempty"`
	CarrierRefs         []string      `json:"carrier_refs,omitempty" yaml:"carrier_refs,omitempty"`
	Lifecycle           Lifecycle     `json:"lifecycle" yaml:"lifecycle"`
	SourcePosture       SourcePosture `json:"source_posture" yaml:"source_posture"`
	AppliesTo           Applicability `json:"applies_to" yaml:"applies_to"`
	DoesNotApplyTo      Applicability `json:"does_not_apply_to,omitempty" yaml:"does_not_apply_to,omitempty"`
	HardGates           []Gate        `json:"hard_gates" yaml:"hard_gates"`
	SoftGates           []string      `json:"soft_gates,omitempty" yaml:"soft_gates,omitempty"`
	Procedure           []string      `json:"procedure" yaml:"procedure"`
	AntiPatterns        []string      `json:"anti_patterns,omitempty" yaml:"anti_patterns,omitempty"`
	RequiredEvidence    []string      `json:"required_evidence,omitempty" yaml:"required_evidence,omitempty"`
	Waiver              WaiverPolicy  `json:"waiver" yaml:"waiver"`
	Priority            int           `json:"priority" yaml:"priority"`
}

type Lifecycle struct {
	Status           string   `json:"status" yaml:"status"`
	ValidFrom        string   `json:"valid_from,omitempty" yaml:"valid_from,omitempty"`
	ValidUntil       string   `json:"valid_until,omitempty" yaml:"valid_until,omitempty"`
	SuccessorRefs    []string `json:"successor_refs,omitempty" yaml:"successor_refs,omitempty"`
	MergedFrom       []string `json:"merged_from,omitempty" yaml:"merged_from,omitempty"`
	RetirementReason string   `json:"retirement_reason,omitempty" yaml:"retirement_reason,omitempty"`
}

type Applicability struct {
	TaskKinds     []string `json:"task_kinds,omitempty" yaml:"task_kinds,omitempty"`
	ChangeIntents []string `json:"change_intents,omitempty" yaml:"change_intents,omitempty"`
	RiskSignals   []string `json:"risk_signals,omitempty" yaml:"risk_signals,omitempty"`
	PathContains  []string `json:"path_contains,omitempty" yaml:"path_contains,omitempty"`
}

type Gate struct {
	ID               string       `json:"gate_id" yaml:"gate_id"`
	Kind             string       `json:"gate_kind" yaml:"gate_kind"`
	CheckLevel       string       `json:"check_level" yaml:"check_level"`
	PassCondition    string       `json:"pass_condition" yaml:"pass_condition"`
	RequiredEvidence []string     `json:"required_evidence,omitempty" yaml:"required_evidence,omitempty"`
	Waiver           WaiverPolicy `json:"waiver" yaml:"waiver"`
}

type WaiverPolicy struct {
	Allowed        bool `json:"allowed" yaml:"allowed"`
	RequiresReason bool `json:"requires_reason" yaml:"requires_reason"`
}

type SourcePosture struct {
	SourceKind        string `json:"source_kind" yaml:"source_kind"`
	SourceEdition     string `json:"source_edition" yaml:"source_edition"`
	Normativity       string `json:"normativity" yaml:"normativity"`
	AuthorityBoundary string `json:"authority_boundary" yaml:"authority_boundary"`
}

type RiskSignal struct {
	ID       string `json:"id"`
	Source   string `json:"source,omitempty"`
	Evidence string `json:"evidence,omitempty"`
}

type PullInput struct {
	Task                 string             `json:"task"`
	DeclaredTaskKind     string             `json:"declared_task_kind,omitempty"`
	ChangeIntent         string             `json:"change_intent,omitempty"`
	IntendedFiles        []string           `json:"intended_files,omitempty"`
	UserScopeConstraints []string           `json:"user_scope_constraints,omitempty"`
	RiskSignals          []RiskSignal       `json:"risk_signals,omitempty"`
	ArtifactRefs         ArtifactRefs       `json:"artifact_refs,omitempty"`
	CarryThrough         []CarryThroughItem `json:"carry_through,omitempty"`
	CeremonyRequest      string             `json:"ceremony_request,omitempty"`
	ResponseBudget       ResponseBudget     `json:"response_budget,omitempty"`
	Context              string             `json:"context,omitempty"`
}

type ArtifactRefs struct {
	ProblemRef    string `json:"problem_ref,omitempty"`
	DecisionRef   string `json:"decision_ref,omitempty"`
	CommissionRef string `json:"commission_ref,omitempty"`
}

type ResponseBudget struct {
	MaxMethods int    `json:"max_methods,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

type MethodCard struct {
	ID               string        `json:"id"`
	Version          string        `json:"version"`
	Title            string        `json:"title"`
	WhyApplies       string        `json:"why_applies"`
	Intent           string        `json:"intent"`
	Lifecycle        Lifecycle     `json:"lifecycle"`
	SourcePosture    SourcePosture `json:"source_posture"`
	HardGates        []Gate        `json:"hard_gates,omitempty"`
	SoftGates        []string      `json:"soft_gates,omitempty"`
	Procedure        []string      `json:"procedure,omitempty"`
	AntiPatterns     []string      `json:"anti_patterns,omitempty"`
	RequiredEvidence []string      `json:"required_evidence,omitempty"`
	Waiver           WaiverPolicy  `json:"waiver"`
	RequiredCloseout bool          `json:"required_closeout"`
}

type MethodRun struct {
	ID                   string             `json:"id"`
	CatalogID            string             `json:"catalog_id"`
	CatalogVersion       string             `json:"catalog_version"`
	Status               string             `json:"status"`
	TaskSignature        TaskSignature      `json:"task_signature"`
	DeterministicContext ContextSnapshot    `json:"deterministic_context,omitempty"`
	Methods              []MethodCard       `json:"methods,omitempty"`
	CarryThrough         []CarryThroughItem `json:"carry_through,omitempty"`
	Checkpoints          []CheckpointRecord `json:"checkpoints,omitempty"`
	OpenedAt             string             `json:"opened_at"`
	ClosedAt             string             `json:"closed_at,omitempty"`
	Closeout             *Closeout          `json:"closeout,omitempty"`
}

type TaskSignature struct {
	Task                 string       `json:"task"`
	NormalizedTaskKind   string       `json:"normalized_task_kind"`
	ChangeIntent         string       `json:"change_intent,omitempty"`
	IntendedFiles        []string     `json:"intended_files,omitempty"`
	RiskSignals          []RiskSignal `json:"risk_signals,omitempty"`
	UserScopeConstraints []string     `json:"user_scope_constraints,omitempty"`
	Ceremony             string       `json:"ceremony"`
	CeremonyReason       string       `json:"ceremony_reason"`
}

type ContextSnapshot struct {
	PathPolicyMatches []string `json:"path_policy_matches,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`
}

type CloseInput struct {
	PullID       string             `json:"pull_id"`
	ChangedFiles []string           `json:"changed_files,omitempty"`
	GateResults  []GateResult       `json:"gate_results,omitempty"`
	Verification Verification       `json:"verification,omitempty"`
	Waivers      []Waiver           `json:"waivers,omitempty"`
	CarryThrough []CarryThroughItem `json:"carry_through,omitempty"`
}

type GateResult struct {
	GateID       string   `json:"gate_id"`
	Status       string   `json:"status"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	WaiverReason string   `json:"waiver_reason,omitempty"`
}

type Verification struct {
	Commands  []string `json:"commands,omitempty"`
	Result    string   `json:"result,omitempty"`
	OutputRef string   `json:"output_ref,omitempty"`
}

type Waiver struct {
	GateID string `json:"gate_id"`
	Reason string `json:"reason"`
}

type CarryThroughItem struct {
	SourceRef     string   `json:"source_ref"`
	SourceItemRef string   `json:"source_item_ref"`
	AcceptanceRef string   `json:"acceptance_ref"`
	Disposition   string   `json:"disposition"`
	TargetRefs    []string `json:"target_refs,omitempty"`
	Reason        string   `json:"reason,omitempty"`
	EvidenceRefs  []string `json:"evidence_refs,omitempty"`
	UpdatedAt     string   `json:"updated_at,omitempty"`
}

type CheckpointRecord struct {
	RecordKind        string   `json:"record_kind"`
	CheckpointID      string   `json:"checkpoint_id"`
	RunRef            string   `json:"run_ref"`
	TargetRef         string   `json:"target_ref,omitempty"`
	CheckRef          string   `json:"check_ref,omitempty"`
	TargetDigest      string   `json:"target_digest,omitempty"`
	Sequence          int      `json:"sequence,omitempty"`
	CloseToken        string   `json:"close_token,omitempty"`
	CloseTokenHash    string   `json:"close_token_hash,omitempty"`
	OpenedAt          string   `json:"opened_at,omitempty"`
	ExpiresAt         string   `json:"expires_at,omitempty"`
	Outcome           string   `json:"outcome,omitempty"`
	ObservationRefs   []string `json:"observation_refs,omitempty"`
	ResultingDigest   string   `json:"resulting_digest,omitempty"`
	NextTargetRef     string   `json:"next_target_ref,omitempty"`
	ClosedAt          string   `json:"closed_at,omitempty"`
	AuthorityBoundary string   `json:"authority_boundary,omitempty"`
}

type Closeout struct {
	ChangedFiles []string           `json:"changed_files,omitempty"`
	GateResults  []GateResult       `json:"gate_results,omitempty"`
	Verification Verification       `json:"verification,omitempty"`
	Waivers      []Waiver           `json:"waivers,omitempty"`
	CarryThrough []CarryThroughItem `json:"carry_through,omitempty"`
	ClosedAt     string             `json:"closed_at"`
}

func nowRFC3339(now time.Time) string {
	return now.UTC().Format(time.RFC3339)
}
