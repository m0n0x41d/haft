package fpf

import "time"

type DependencySuggestion struct {
	HolonID string
	Title   string
	Type    string
	Layer   string
}

type Contract struct {
	Invariants         []string          `json:"invariants,omitempty"`
	AntiPatterns       []string          `json:"anti_patterns,omitempty"`
	AcceptanceCriteria []string          `json:"acceptance_criteria,omitempty"`
	AffectedScope      []string          `json:"affected_scope,omitempty"`
	AffectedHashes     map[string]string `json:"affected_hashes,omitempty"`
}

type CheckResult struct {
	Verdict   string   `json:"verdict"`
	Evidence  []string `json:"evidence"`
	Reasoning string   `json:"reasoning"`
}

type VerifyResult struct {
	TypeCheck       CheckResult `json:"type_check"`
	ConstraintCheck CheckResult `json:"constraint_check"`
	LogicCheck      CheckResult `json:"logic_check"`
	OverallVerdict  string      `json:"overall_verdict"`
	Risks           []string    `json:"risks,omitempty"`
	Predictions     []string    `json:"predictions"`
}

type Observation struct {
	Description     string   `json:"description"`
	Evidence        []string `json:"evidence"`
	Supports        bool     `json:"supports"`
	TestsPrediction string   `json:"tests_prediction,omitempty"`
}

type TestResult struct {
	Observations   []Observation `json:"observations"`
	OverallVerdict string        `json:"overall_verdict"`
	Reasoning      string        `json:"reasoning"`
}

type InternalizeResult struct {
	Status                string
	Phase                 string
	SuggestedPhase        string
	Role                  string
	ContextID             string
	ContextChanges        []string
	LayerCounts           map[string]int
	ArchivedCounts        map[string]int
	RecentHolons          []HolonSummary
	DecayWarnings         []DecayWarning
	OpenDecisions         []DecisionSummary
	ResolvedDecisions     []DecisionSummary
	NextAction            string
	ActiveContexts        []DecisionContextSummary
	AffectedScopeWarnings []AffectedScopeWarning
}

type DecisionContextSummary struct {
	ID               string
	Title            string
	Scope            string
	Stage            ContextStage
	HypothesisCount  int
	DiversityWarning string
}

type HolonSummary struct {
	ID        string
	Title     string
	Layer     string
	Kind      string
	RScore    float64
	UpdatedAt time.Time
}

type DecayWarning struct {
	EvidenceID string
	HolonID    string
	HolonTitle string
	ExpiresAt  time.Time
	DaysLeft   int
}

type AffectedScopeWarning struct {
	DecisionID    string
	DecisionTitle string
	FilePath      string
	ChangeType    string
	OldHash       string
	NewHash       string
}

type ProjectContext struct {
	Vocabulary string
	Invariants string
	TechStack  []string
}

type ResolveInput struct {
	DecisionID       string `json:"decision_id"`
	Resolution       string `json:"resolution"`
	Reference        string `json:"reference"`
	SupersededBy     string `json:"superseded_by"`
	Notes            string `json:"notes"`
	ValidUntil       string `json:"valid_until"`
	CriteriaVerified bool   `json:"criteria_verified"`
}

type DecisionSummary struct {
	ID         string
	Title      string
	CreatedAt  time.Time
	Resolution string
	ResolvedAt time.Time
	Notes      string
	Reference  string
}

type ConstraintSource struct {
	DRRID       string
	DRRTitle    string
	Constraints []string
}

type InheritedConstraints struct {
	Invariants   []ConstraintSource
	AntiPatterns []ConstraintSource
}

type DRRInfo struct {
	ID        string
	Title     string
	Contract  *Contract
	DependsOn []string
	WinnerID  string
}

type CodeChangeImpact struct {
	Type       string
	File       string
	EvidenceID string
	HolonID    string
	HolonTitle string
	HolonLayer string
	PreviousR  float64
	Reason     string
}

type CodeChangeDetectionResult struct {
	FromCommit    string
	ToCommit      string
	ChangedFiles  []string
	Impacts       []CodeChangeImpact
	TotalStale    int
	TotalAffected int
}

type ImplementationWarnings struct {
	ChangedFiles     []ChangedFileWarning
	DependencyIssues []DependencyIssueWarning
}

func (w *ImplementationWarnings) HasAny() bool {
	return len(w.ChangedFiles) > 0 || len(w.DependencyIssues) > 0
}

type ChangedFileWarning struct {
	FilePath    string
	CommitCount int
}

type DependencyIssueWarning struct {
	HolonID    string
	HolonTitle string
	Layer      string
	REff       float64
	Reason     string
}

type CompactResult struct {
	Mode            string
	RetentionDays   int64
	CompactedCount  int
	EligibleCount   int64
	CompactedHolons []string
	Errors          []string
}
