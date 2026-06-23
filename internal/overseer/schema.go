package overseer

const (
	ReviewPacketSchemaVersion   = "overseer.review_packet.v1"
	ReviewResultSchemaVersion   = "overseer.review_result.v1"
	MaintenanceRunSchemaVersion = "overseer.maintenance_run.v1"
	RiskPolicyVersion           = "overseer.risk.v1"
	ScopePolicyVersion          = "overseer.scope.v1"
	DefaultToolName             = "haft"
)

type Packet struct {
	SchemaVersion         string                `json:"schema_version"`
	PacketID              string                `json:"packet_id"`
	PacketHash            string                `json:"packet_hash"`
	CreatedAt             string                `json:"created_at"`
	Producer              Producer              `json:"producer"`
	Subject               Subject               `json:"subject"`
	RepoState             RepoState             `json:"repo_state"`
	ChangedFiles          []ChangedFile         `json:"changed_files"`
	DeterministicFindings DeterministicFindings `json:"deterministic_findings"`
	Risk                  Risk                  `json:"risk"`
	ReviewRequest         ReviewRequest         `json:"review_request"`
	ContextBudget         ContextBudget         `json:"context_budget"`
	Omissions             []Omission            `json:"omissions"`
}

type Producer struct {
	Tool           string            `json:"tool"`
	Version        string            `json:"version"`
	PolicyVersions map[string]string `json:"policy_versions"`
}

type Subject struct {
	Kind                 string `json:"kind"`
	Ref                  string `json:"ref"`
	SHA                  string `json:"sha"`
	ParentSHA            string `json:"parent_sha,omitempty"`
	DiffHash             string `json:"diff_hash"`
	ArtifactSnapshotHash string `json:"artifact_snapshot_hash"`
}

type RepoState struct {
	GitRoot                  string `json:"git_root"`
	Branch                   string `json:"branch"`
	WorktreeDirtyAfterCommit bool   `json:"worktree_dirty_after_commit"`
	UntrackedFilesCount      int    `json:"untracked_files_count"`
}

type ChangedFile struct {
	Path           string                `json:"path"`
	Status         string                `json:"status"`
	Language       string                `json:"language,omitempty"`
	DiffStats      DiffStats             `json:"diff_stats"`
	InlineDiffRef  string                `json:"inline_diff_ref,omitempty"`
	FullDiffHandle string                `json:"full_diff_handle,omitempty"`
	Governance     ChangedFileGovernance `json:"governance"`
}

type DiffStats struct {
	Added   int `json:"added"`
	Deleted int `json:"deleted"`
}

type ChangedFileGovernance struct {
	ModuleState          string         `json:"module_state"`
	AffectedDecisions    []ArtifactRef  `json:"affected_decisions"`
	AffectedSpecSections []string       `json:"affected_spec_sections"`
	AffectedInvariants   []InvariantRef `json:"affected_invariants"`
	PathPolicies         []string       `json:"path_policies"`
}

type ArtifactRef struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
}

type InvariantRef struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	SourceRef string `json:"source_ref"`
}

type DeterministicFindings struct {
	Stale        []FindingSummary `json:"stale"`
	Drift        []FindingSummary `json:"drift"`
	SpecHealth   []FindingSummary `json:"spec_health"`
	CoverageGaps []FindingSummary `json:"coverage_gaps"`
	Suppressed   SuppressedDebt   `json:"suppressed"`
}

type FindingSummary struct {
	ID       string   `json:"id"`
	Title    string   `json:"title,omitempty"`
	Kind     string   `json:"kind,omitempty"`
	Category string   `json:"category,omitempty"`
	Reason   string   `json:"reason,omitempty"`
	Paths    []string `json:"paths,omitempty"`
}

type SuppressedDebt struct {
	UnrelatedStale        int `json:"unrelated_stale"`
	UnrelatedDrift        int `json:"unrelated_drift"`
	UnrelatedSpecHealth   int `json:"unrelated_spec_health"`
	UnrelatedCoverageGaps int `json:"unrelated_coverage_gaps"`
}

type Risk struct {
	Level          string     `json:"level"`
	Score          int        `json:"score"`
	PolicyVersion  string     `json:"policy_version"`
	RulesTriggered []RiskRule `json:"rules_triggered"`
	LLMReview      string     `json:"llm_review"`
}

type RiskRule struct {
	RuleID           string   `json:"rule_id"`
	Source           string   `json:"source"`
	Basis            string   `json:"basis"`
	ReviewModesAdded []string `json:"review_modes_added"`
}

type ReviewRequest struct {
	Authority     string   `json:"authority"`
	Modes         []string `json:"modes"`
	MustNotReview []string `json:"must_not_review"`
	HumanBound    bool     `json:"human_bound"`
}

type ContextBudget struct {
	MaxPacketBytes        int    `json:"max_packet_bytes"`
	MaxChangedFilesListed int    `json:"max_changed_files_listed"`
	MaxInlineDiffBytes    int    `json:"max_inline_diff_bytes"`
	MaxArtifactRefs       int    `json:"max_artifact_refs"`
	FullSourcePolicy      string `json:"full_source_policy"`
	OmissionPolicy        string `json:"omission_policy"`
}

type Omission struct {
	Kind        string `json:"kind"`
	Path        string `json:"path,omitempty"`
	Count       int    `json:"count,omitempty"`
	FetchHandle string `json:"fetch_handle,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

type ReviewRun struct {
	SchemaVersion string              `json:"schema_version"`
	ReviewRunID   string              `json:"review_run_id"`
	PacketID      string              `json:"packet_id"`
	PacketHash    string              `json:"packet_hash"`
	CreatedAt     string              `json:"created_at,omitempty"`
	ReviewedAt    string              `json:"reviewed_at,omitempty"`
	Mode          string              `json:"mode,omitempty"`
	Budget        ContextBudget       `json:"budget,omitempty"`
	Reviewer      Reviewer            `json:"reviewer"`
	Authority     ReviewAuthority     `json:"authority"`
	Verdict       string              `json:"verdict"`
	ScopeCoverage ScopeCoverage       `json:"scope_coverage"`
	Findings      []ReviewFinding     `json:"findings"`
	NonFindings   []NonFinding        `json:"non_findings_under_scope"`
	Dispositions  []ReviewDisposition `json:"dispositions,omitempty"`
}

type Reviewer struct {
	Agent                          string   `json:"agent"`
	ModelOrRuntime                 string   `json:"model_or_runtime,omitempty"`
	SessionRelationToAuthor        string   `json:"session_relation_to_author"`
	InputSources                   []string `json:"input_sources"`
	AuthorSessionTranscriptVisible bool     `json:"author_session_transcript_visible"`
}

type ReviewAuthority struct {
	Status string   `json:"status"`
	Cannot []string `json:"cannot"`
}

type ScopeCoverage struct {
	ModesReviewed []string `json:"modes_reviewed"`
	FilesReviewed []string `json:"files_reviewed"`
	FetchesUsed   []string `json:"fetches_used"`
	Abstentions   []string `json:"abstentions"`
}

type ReviewFinding struct {
	ID                  string              `json:"id"`
	Severity            string              `json:"severity"`
	Confidence          string              `json:"confidence"`
	Category            string              `json:"category"`
	Claim               string              `json:"claim"`
	Title               string              `json:"title,omitempty"`
	ConcreteHarm        string              `json:"concrete_harm"`
	Description         string              `json:"description,omitempty"`
	Locations           []FindingLocation   `json:"locations"`
	AffectedArtifacts   []string            `json:"affected_artifacts"`
	AffectedInvariants  []string            `json:"affected_invariants"`
	Basis               FindingBasis        `json:"basis"`
	MinimalFix          string              `json:"minimal_fix"`
	Recommendation      string              `json:"recommendation,omitempty"`
	Verification        FindingVerification `json:"verification"`
	FalsePositiveChecks []string            `json:"false_positive_checks"`
	SupportPosture      string              `json:"support_posture"`
	CountsForREff       bool                `json:"counts_for_r_eff"`
}

type FindingLocation struct {
	Path        string `json:"path"`
	LineStart   int    `json:"line_start,omitempty"`
	LineEnd     int    `json:"line_end,omitempty"`
	EvidenceRef string `json:"evidence_ref,omitempty"`
}

type FindingBasis struct {
	PacketRefs   []string `json:"packet_refs"`
	CodeRefs     []string `json:"code_refs"`
	ArtifactRefs []string `json:"artifact_refs"`
}

type FindingVerification struct {
	SuggestedCommands     []string `json:"suggested_commands"`
	RequiresHumanDecision bool     `json:"requires_human_decision"`
}

type NonFinding struct {
	Claim string `json:"claim"`
	Basis string `json:"basis"`
	Scope string `json:"scope"`
}

type ReviewDisposition struct {
	FindingID string `json:"finding_id"`
	Status    string `json:"status"`
	Actor     string `json:"actor,omitempty"`
	Reason    string `json:"reason,omitempty"`
	CommitSHA string `json:"commit_sha,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type ReviewResultInput struct {
	SchemaVersion string          `json:"schema_version,omitempty"`
	Mode          string          `json:"mode,omitempty"`
	Reviewer      Reviewer        `json:"reviewer,omitempty"`
	Verdict       string          `json:"verdict,omitempty"`
	ScopeCoverage ScopeCoverage   `json:"scope_coverage,omitempty"`
	Findings      []ReviewFinding `json:"findings"`
	NonFindings   []NonFinding    `json:"non_findings_under_scope,omitempty"`
}

type ReviewReminder struct {
	ReviewRunID        string   `json:"review_run_id"`
	PacketID           string   `json:"packet_id"`
	UnresolvedFindings []string `json:"unresolved_findings"`
	Message            string   `json:"message"`
}

type StatusSignal struct {
	Severity         string `json:"severity"`
	Source           string `json:"source"`
	Title            string `json:"title"`
	Detail           string `json:"detail,omitempty"`
	Command          string `json:"command,omitempty"`
	ReviewRunID      string `json:"review_run_id,omitempty"`
	PacketID         string `json:"packet_id,omitempty"`
	FindingID        string `json:"finding_id,omitempty"`
	MaintenanceRunID string `json:"maintenance_run_id,omitempty"`
}

type StatusSummary struct {
	HasSignals          bool           `json:"has_signals"`
	Signals             []StatusSignal `json:"signals"`
	LatestReviewRunID   string         `json:"latest_review_run_id,omitempty"`
	LatestPacketID      string         `json:"latest_packet_id,omitempty"`
	LatestMaintenanceID string         `json:"latest_maintenance_id,omitempty"`
	// LatestExecutedMaintenanceID points at the newest maintenance run with
	// autonomous executed actions. It can differ from LatestMaintenanceID when a
	// later report-only maintain run has no executed ledger.
	LatestExecutedMaintenanceID string `json:"latest_executed_maintenance_id,omitempty"`
	SuppressedCount             int    `json:"suppressed_count"`
	// ExecutedActions surfaces the autonomous maintenance ledger of the latest
	// run — session-start disclosure: autonomy is visible, never silent.
	ExecutedActions []MaintenanceAction `json:"executed_actions,omitempty"`
}

type MaintenanceInput struct {
	CreatedAt               string
	Stale                   []FindingSummary
	Drift                   []MaintenanceDriftFinding
	SpecHealth              []FindingSummary
	CoverageGaps            []FindingSummary
	Executed                []MaintenanceAction
	ReconciliationProposals []MaintenanceReconciliationProposal
}

// MaintenanceAction is one ledger entry of the maintenance execute-phase
// (dec-20260611-overseer-maintenance-executor). Every autonomous act carries
// its actor context (the maintenance run), prior state, and a one-step undo.
// Executed actions are allowlisted maintenance effects only: they cannot encode
// decision lifecycle binding such as supersede, retire, merge, or approval.
type MaintenanceAction struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"` // auto_rebaseline | observable_run | revalidate_stale
	DecisionRef  string   `json:"decision_ref"`
	Title        string   `json:"title,omitempty"`
	Rung         int      `json:"rung"`
	Detail       string   `json:"detail,omitempty"` // command run / what was extended / gate reason
	Outcome      string   `json:"outcome"`          // applied | proposed | evidence_attached | failed
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	PriorState   string   `json:"prior_state,omitempty"` // JSON snapshot for undo (baseline files+symbols)
	Undo         string   `json:"undo,omitempty"`        // operator command restoring prior state
}

type MaintenanceReconciliationProposal struct {
	ID                string   `json:"id"`
	Kind              string   `json:"kind"`
	GroupID           string   `json:"group_id,omitempty"`
	Category          string   `json:"category,omitempty"`
	Reason            string   `json:"reason"`
	DecisionRefs      []string `json:"decision_refs,omitempty"`
	Fanout            int      `json:"fanout,omitempty"`
	FallbackTargets   []string `json:"fallback_targets,omitempty"`
	ScopeRepairHints  []string `json:"scope_repair_hints,omitempty"`
	SuggestedCommand  string   `json:"suggested_command"`
	AuthorityBoundary string   `json:"authority_boundary"`
}

type MaintenanceDriftFinding struct {
	ID            string   `json:"id"`
	Title         string   `json:"title,omitempty"`
	Summary       string   `json:"summary,omitempty"`
	Paths         []string `json:"paths,omitempty"`
	HasBaseline   bool     `json:"has_baseline"`
	SymbolVerdict string   `json:"symbol_verdict,omitempty"`
	Materiality   string   `json:"materiality,omitempty"`
	Action        string   `json:"action"`
	Reason        string   `json:"reason"`
}

type MaintenanceRun struct {
	SchemaVersion           string                              `json:"schema_version"`
	MaintenanceID           string                              `json:"maintenance_id"`
	CreatedAt               string                              `json:"created_at,omitempty"`
	Verdict                 string                              `json:"verdict"`
	Authority               ReviewAuthority                     `json:"authority"`
	Summary                 MaintenanceSummary                  `json:"summary"`
	Signals                 []StatusSignal                      `json:"signals"`
	Suppressed              []MaintenanceSuppression            `json:"suppressed"`
	ReconciliationProposals []MaintenanceReconciliationProposal `json:"reconciliation_proposals,omitempty"`
	AfterAction             MaintenanceAfterActionReport        `json:"after_action"`
	Executed                []MaintenanceAction                 `json:"executed,omitempty"`
}

type MaintenanceAfterActionReport struct {
	AutoClosedItems           []MaintenanceAfterActionItem `json:"auto_closed_items,omitempty"`
	EvidenceChecked           []MaintenanceAfterActionItem `json:"evidence_checked,omitempty"`
	RemainingOperatorJudgment []MaintenanceAfterActionItem `json:"remaining_operator_judgment,omitempty"`
	UndoCommands              []string                     `json:"undo_commands,omitempty"`
	AuthorityBoundary         string                       `json:"authority_boundary"`
}

type MaintenanceAfterActionItem struct {
	Ref          string   `json:"ref,omitempty"`
	Title        string   `json:"title,omitempty"`
	Action       string   `json:"action,omitempty"`
	Outcome      string   `json:"outcome,omitempty"`
	Command      string   `json:"command,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	Reason       string   `json:"reason,omitempty"`
}

type MaintenanceSummary struct {
	SignalCount                 int `json:"signal_count"`
	SuppressedCount             int `json:"suppressed_count"`
	AutoResolvableDrift         int `json:"auto_resolvable_drift"`
	ConfirmRequiredDrift        int `json:"confirm_required_drift"`
	ReviewRequiredDrift         int `json:"review_required_drift"`
	StaleCount                  int `json:"stale_count"`
	SpecHealthCount             int `json:"spec_health_count"`
	CoverageGapCount            int `json:"coverage_gap_count"`
	ExecutedCount               int `json:"executed_count,omitempty"`
	ReconciliationProposalCount int `json:"reconciliation_proposal_count,omitempty"`
}

type MaintenanceSuppression struct {
	ID     string `json:"id"`
	Title  string `json:"title,omitempty"`
	Source string `json:"source"`
	Action string `json:"action"`
	Reason string `json:"reason"`
	Detail string `json:"detail,omitempty"`
}

type GovernanceInput struct {
	Stale        []FindingSummary
	Drift        []FindingSummary
	SpecHealth   []FindingSummary
	CoverageGaps []FindingSummary
	Suppressed   SuppressedDebt
}

type BuildInput struct {
	CreatedAt    string
	Producer     Producer
	Subject      Subject
	RepoState    RepoState
	ChangedFiles []ChangedFile
	Governance   GovernanceInput
	Budget       ContextBudget
}
