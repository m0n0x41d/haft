package artifact

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Kind identifies the type of artifact.
type Kind string

const (
	KindNote              Kind = "Note"
	KindProblemCard       Kind = "ProblemCard"
	KindSolutionPortfolio Kind = "SolutionPortfolio"
	KindDecisionRecord    Kind = "DecisionRecord"
	KindEvidencePack      Kind = "EvidencePack"
	KindRefreshReport     Kind = "RefreshReport"
)

// validKinds is the set of all valid artifact kinds (unexported — use ParseKind at boundaries).
var validKinds = map[Kind]bool{
	KindNote: true, KindProblemCard: true, KindSolutionPortfolio: true,
	KindDecisionRecord: true, KindEvidencePack: true, KindRefreshReport: true,
}

// IsValid returns true if the kind is a recognized artifact kind.
func (k Kind) IsValid() bool { return validKinds[k] }

// ParseKind validates and returns a Kind, or an error if unrecognized.
func ParseKind(s string) (Kind, error) {
	k := Kind(s)
	if !k.IsValid() {
		return "", fmt.Errorf("invalid artifact kind: %q", s)
	}
	return k, nil
}

// IDPrefix returns the stable ID prefix for this artifact kind.
func (k Kind) IDPrefix() string {
	switch k {
	case KindNote:
		return "note"
	case KindProblemCard:
		return "prob"
	case KindSolutionPortfolio:
		return "sol"
	case KindDecisionRecord:
		return "dec"
	case KindEvidencePack:
		return "evid"
	case KindRefreshReport:
		return "ref"
	default:
		return "art"
	}
}

// Dir returns the .haft/ subdirectory for this kind.
func (k Kind) Dir() string {
	switch k {
	case KindNote:
		return "notes"
	case KindProblemCard:
		return "problems"
	case KindSolutionPortfolio:
		return "solutions"
	case KindDecisionRecord:
		return "decisions"
	case KindEvidencePack:
		return "evidence"
	case KindRefreshReport:
		return "refresh"
	default:
		return "artifacts"
	}
}

// UserFacingLabel renders artifact kinds as plain-language labels.
func (k Kind) UserFacingLabel() string {
	switch k {
	case KindProblemCard:
		return "problem"
	case KindSolutionPortfolio:
		return "solution portfolio"
	case KindDecisionRecord:
		return "decision"
	case KindEvidencePack:
		return "evidence pack"
	case KindRefreshReport:
		return "refresh report"
	default:
		return strings.TrimSpace(string(k))
	}
}

// UserFacingHeading renders artifact kinds as list headings.
func (k Kind) UserFacingHeading(count int) string {
	switch k {
	case KindProblemCard:
		if count == 1 {
			return "Problem"
		}
		return "Problems"
	case KindSolutionPortfolio:
		if count == 1 {
			return "Solution Portfolio"
		}
		return "Solution Portfolios"
	case KindDecisionRecord:
		if count == 1 {
			return "Decision"
		}
		return "Decisions"
	case KindEvidencePack:
		if count == 1 {
			return "Evidence Pack"
		}
		return "Evidence Packs"
	case KindRefreshReport:
		if count == 1 {
			return "Refresh Report"
		}
		return "Refresh Reports"
	default:
		return strings.TrimSpace(string(k))
	}
}

// Status represents artifact lifecycle status.
type Status string

const (
	StatusActive     Status = "active"
	StatusAddressed  Status = "addressed"
	StatusSuperseded Status = "superseded"
	StatusDeprecated Status = "deprecated"
	StatusRefreshDue Status = "refresh_due"
)

var validStatuses = map[Status]bool{
	StatusActive: true, StatusAddressed: true, StatusSuperseded: true, StatusDeprecated: true, StatusRefreshDue: true,
}

// IsValid returns true if the status is recognized.
func (s Status) IsValid() bool { return validStatuses[s] }

// ParseStatus validates and returns a Status, or an error if unrecognized.
func ParseStatus(s string) (Status, error) {
	st := Status(s)
	if !st.IsValid() {
		return "", fmt.Errorf("invalid artifact status: %q", s)
	}
	return st, nil
}

// Mode represents the decision depth mode.
type Mode string

const (
	ModeNote     Mode = "note"
	ModeTactical Mode = "tactical"
	ModeStandard Mode = "standard"
	ModeDeep     Mode = "deep"
)

var validModes = map[Mode]bool{
	ModeNote: true, ModeTactical: true, ModeStandard: true, ModeDeep: true,
}

// IsValid returns true if the mode is recognized.
func (m Mode) IsValid() bool { return validModes[m] }

// ParseMode validates and returns a Mode, or an error if unrecognized.
func ParseMode(s string) (Mode, error) {
	m := Mode(s)
	if !m.IsValid() {
		return "", fmt.Errorf("invalid decision mode: %q", s)
	}
	return m, nil
}

// DerivedStatus is computed from artifact completeness, never stored.
type DerivedStatus string

const (
	DerivedUnderframed DerivedStatus = "UNDERFRAMED"
	DerivedFramed      DerivedStatus = "FRAMED"
	DerivedExploring   DerivedStatus = "EXPLORING"
	DerivedCompared    DerivedStatus = "COMPARED"
	DerivedDecided     DerivedStatus = "DECIDED"
	DerivedApplied     DerivedStatus = "APPLIED"
	DerivedRefreshDue  DerivedStatus = "REFRESH_DUE"
)

// Link represents a relationship between two artifacts.
type Link struct {
	Ref  string `yaml:"ref" json:"ref"`
	Type string `yaml:"type" json:"type"` // informs, based_on, supersedes, contradicts, refines
}

// Meta is the common frontmatter for all artifacts.
type Meta struct {
	ID         string    `yaml:"id" json:"id"`
	Kind       Kind      `yaml:"kind" json:"kind"`
	Version    int       `yaml:"version" json:"version"`
	Status     Status    `yaml:"status" json:"status"`
	Context    string    `yaml:"context,omitempty" json:"context,omitempty"`
	Mode       Mode      `yaml:"mode,omitempty" json:"mode,omitempty"`
	Title      string    `yaml:"title" json:"title"`
	ValidUntil string    `yaml:"valid_until,omitempty" json:"valid_until,omitempty"`
	CreatedAt  time.Time `yaml:"created_at" json:"created_at"`
	UpdatedAt  time.Time `yaml:"updated_at" json:"updated_at"`
	Links      []Link    `yaml:"links,omitempty" json:"links,omitempty"`
}

// Artifact holds metadata + markdown body for any artifact type.
type Artifact struct {
	Meta           Meta   `yaml:"meta" json:"meta"`
	Body           string `yaml:"-" json:"body"`            // markdown content after frontmatter
	SearchKeywords string `yaml:"-" json:"search_keywords"` // agent-generated synonyms/related terms for FTS5
	StructuredData string `yaml:"-" json:"structured_data"` // JSON: canonical structured fields (eliminates markdown re-parsing)
}

type ProblemType string

const (
	ProblemTypeOptimization ProblemType = "optimization"
	ProblemTypeDiagnosis    ProblemType = "diagnosis"
	ProblemTypeSearch       ProblemType = "search"
	ProblemTypeSynthesis    ProblemType = "synthesis"
)

// GovernanceMode declares whether a decision's affected_files act as exact
// file-level governance or as a module-level scope (recursive directory
// coverage that auto-captures newly added sibling files as governed drift).
//
// Defaults to "module" when unset — preserves haft <=6.2 behavior where
// every affected_file path silently widened to its parent directory.
//
// Pick "exact" when the decision is genuinely about specific files and you
// do NOT want sibling additions to count as governed drift. This honors
// FPF X-SCOPE: every claim has explicit where + under what + when.
type GovernanceMode string

const (
	GovernanceModeModule GovernanceMode = "module"
	GovernanceModeExact  GovernanceMode = "exact"
)

// IsValid reports whether the value is a recognized governance mode.
func (m GovernanceMode) IsValid() bool {
	return m == GovernanceModeModule || m == GovernanceModeExact
}

// EffectiveGovernanceMode resolves the mode for a decision, defaulting to
// "module" when unset (preserves backward compatibility with pre-6.2.x
// decisions that have no governance_mode field).
func (df DecisionFields) EffectiveGovernanceMode() GovernanceMode {
	if df.GovernanceMode == "" {
		return GovernanceModeModule
	}
	return df.GovernanceMode
}

// ParseGovernanceMode validates and returns a GovernanceMode, or an error if
// unrecognized. Empty input is treated as the default mode.
func ParseGovernanceMode(value string) (GovernanceMode, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", nil
	}
	mode := GovernanceMode(v)
	if !mode.IsValid() {
		return "", fmt.Errorf("governance_mode must be %q or %q (got %q)", GovernanceModeModule, GovernanceModeExact, value)
	}
	return mode, nil
}

func ParseProblemType(value string) (ProblemType, error) {
	normalized := ProblemType(strings.TrimSpace(value))
	switch normalized {
	case "":
		return "", nil
	case ProblemTypeOptimization, ProblemTypeDiagnosis, ProblemTypeSearch, ProblemTypeSynthesis:
		return normalized, nil
	default:
		return "", fmt.Errorf("problem_type must be optimization, diagnosis, search, or synthesis")
	}
}

// ProblemFields holds structured data for a ProblemCard. Stored as JSON in StructuredData.
type ProblemFields struct {
	ProblemType           ProblemType                `json:"problem_type,omitempty"`
	Signal                string                     `json:"signal"`
	Constraints           []string                   `json:"constraints,omitempty"`
	OptimizationTargets   []string                   `json:"optimization_targets,omitempty"`
	ObservationIndicators []string                   `json:"observation_indicators,omitempty"`
	Acceptance            string                     `json:"acceptance,omitempty"`
	BlastRadius           string                     `json:"blast_radius,omitempty"`
	Reversibility         string                     `json:"reversibility,omitempty"`
	Characterizations     []CharacterizationSnapshot `json:"characterizations,omitempty"`
}

// DecisionFields holds structured data for a DecisionRecord. Stored as JSON in StructuredData.
type DecisionFields struct {
	ProblemRefs          []string             `json:"problem_refs,omitempty"`
	SelectedTitle        string               `json:"selected_title"`
	WhySelected          string               `json:"why_selected"`
	SelectionPolicy      string               `json:"selection_policy,omitempty"`
	CounterArgument      string               `json:"counterargument,omitempty"`
	WeakestLink          string               `json:"weakest_link,omitempty"`
	WhyNotOthers         []RejectionReason    `json:"why_not_others,omitempty"`
	Claims               []DecisionClaim      `json:"claims,omitempty"`
	Predictions          []DecisionPrediction `json:"predictions,omitempty"`
	PreConditions        []string             `json:"pre_conditions,omitempty"`
	RollbackTriggers     []string             `json:"rollback_triggers,omitempty"`
	RollbackSteps        []string             `json:"rollback_steps,omitempty"`
	RollbackBlastRadius  string               `json:"rollback_blast_radius,omitempty"`
	Invariants           []string             `json:"invariants,omitempty"`
	PostConds            []string             `json:"post_conditions,omitempty"`
	Admissibility        []string             `json:"admissibility,omitempty"`
	EvidenceRequirements []string             `json:"evidence_requirements,omitempty"`
	RefreshTriggers      []string             `json:"refresh_triggers,omitempty"`
	FirstModuleCoverage  bool                 `json:"first_module_coverage,omitempty"`
	DriftManifests       []DriftScopeManifest `json:"drift_manifests,omitempty"`
	// GovernanceMode declares how affected_files relate to drift detection.
	// "module" (default, preserves pre-6.2.x behavior): each affected_file
	// widens to its parent directory; sibling additions count as governed
	// drift. "exact": only the listed files are governed. See GovernanceMode.
	GovernanceMode GovernanceMode `json:"governance_mode,omitempty"`
}

type decisionFieldsJSON DecisionFields

func (df DecisionFields) MarshalJSON() ([]byte, error) {
	encoded := decisionFieldsJSON(df)
	encoded.Claims = normalizeDecisionClaims(encoded.Claims)

	if len(encoded.Claims) == 0 {
		encoded.Claims = decisionClaimsFromPredictions(encoded.Predictions)
	}

	encoded.Predictions = nil

	return json.Marshal(encoded)
}

func (df *DecisionFields) UnmarshalJSON(data []byte) error {
	decoded := decisionFieldsJSON{}

	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	decoded.Claims = normalizeDecisionClaims(decoded.Claims)
	if len(decoded.Claims) == 0 {
		decoded.Claims = decisionClaimsFromPredictions(decoded.Predictions)
	}

	decoded.Predictions = decisionPredictionsFromClaims(decoded.Claims)
	*df = DecisionFields(decoded)

	return nil
}

// DriftScopeManifest stores the baseline file set for one governed scope.
type DriftScopeManifest struct {
	Scope string   `json:"scope"`
	Files []string `json:"files,omitempty"`
}

// UnmarshalProblemFields extracts structured fields from an artifact's StructuredData.
// Returns zero value if StructuredData is empty or not a ProblemCard.
func (a *Artifact) UnmarshalProblemFields() ProblemFields {
	if a.StructuredData == "" {
		return ProblemFields{}
	}
	var pf ProblemFields
	_ = json.Unmarshal([]byte(a.StructuredData), &pf)
	return pf
}

func ProblemTypeLabel(a *Artifact) string {
	if a == nil {
		return ""
	}

	fields := a.UnmarshalProblemFields()
	if fields.ProblemType == "" {
		return ""
	}

	return string(fields.ProblemType)
}

// UnmarshalDecisionFields extracts structured fields from an artifact's StructuredData.
func (a *Artifact) UnmarshalDecisionFields() DecisionFields {
	if a.StructuredData == "" {
		return DecisionFields{}
	}
	var df DecisionFields
	_ = json.Unmarshal([]byte(a.StructuredData), &df)
	df.Claims = normalizeDecisionClaims(df.Claims)
	if len(df.Claims) == 0 {
		df.Claims = decisionClaimsFromPredictions(df.Predictions)
	}
	df.Predictions = decisionPredictionsFromClaims(df.Claims)
	return df
}

// UnmarshalPortfolioFields extracts structured fields from an artifact's StructuredData.
func (a *Artifact) UnmarshalPortfolioFields() PortfolioFields {
	if a.StructuredData == "" {
		return PortfolioFields{}
	}
	var pf PortfolioFields
	_ = json.Unmarshal([]byte(a.StructuredData), &pf)
	return pf
}

// PortfolioHasComparison reports whether a portfolio already contains
// persisted comparison output in structured data or legacy rendered form.
func PortfolioHasComparison(a *Artifact) bool {
	if a == nil || a.Meta.Kind != KindSolutionPortfolio {
		return false
	}

	fields := a.UnmarshalPortfolioFields()
	if fields.Comparison != nil {
		return true
	}

	return strings.Contains(a.Body, "## Comparison") ||
		strings.Contains(a.Body, "## Non-Dominated Set")
}

// ResolveComparedPortfolioRef reports the portfolio ref only when the stored
// portfolio already contains persisted comparison output.
func ResolveComparedPortfolioRef(ctx context.Context, store ArtifactStore, portfolioRef string) string {
	if strings.TrimSpace(portfolioRef) == "" {
		return ""
	}

	portfolio, err := store.Get(ctx, portfolioRef)
	if err != nil || !PortfolioHasComparison(portfolio) {
		return ""
	}

	return portfolio.Meta.ID
}

// GenerateID creates a unique artifact ID with the format
// `<prefix>-YYYYMMDD-<6 hex chars>` (e.g. `dec-20260418-a3f7c1`).
//
// The 32-bit random hex suffix is sourced from crypto/rand to prevent
// filename collisions when multiple branches create artifacts on the same
// day (issue #63). Sequential per-day counters lose meaning across branches
// and produce mechanically-unmergeable conflicts in `.haft/`. The hex
// suffix makes branch merges that touched `.haft/` on the same day
// collision-free in practice (~4.3B values per kind per day — birthday-paradox
// collision probability stays below 10^-6 for the first few thousand IDs).
//
// The seq parameter is preserved for backward-compatible call sites and
// may be useful for in-process ordering, but is no longer rendered into the
// ID. NextSequence may still be called by creators; its return value is
// unused for ID construction.
func GenerateID(kind Kind, seq int) string {
	_ = seq // legacy parameter; collision resistance is provided by hex suffix
	date := time.Now().Format("20060102")
	return fmt.Sprintf("%s-%s-%s", kind.IDPrefix(), date, randomIDSuffix())
}

// randomIDSuffix returns an 8-character lowercase hex string sourced from
// crypto/rand. Falls back to a deterministic non-zero value on the
// effectively-impossible case where crypto/rand fails — caller still gets a
// valid ID rather than a panic.
func randomIDSuffix() string {
	bytes := make([]byte, 4) // 4 bytes = 8 hex chars
	if _, err := rand.Read(bytes); err != nil {
		return "fffffffe"
	}
	return hex.EncodeToString(bytes)
}

// --- Domain-specific structured content ---
// These are parsed from the markdown body by tools, not stored in frontmatter.
// The Body field holds everything as markdown. These types exist for
// programmatic access when tools need structured data.

// Variant represents a solution option in a SolutionPortfolio.
type Variant struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Strengths          []string `json:"strengths,omitempty"`
	WeakestLink        string   `json:"weakest_link"`
	NoveltyMarker      string   `json:"novelty_marker,omitempty"`
	Risks              []string `json:"risks,omitempty"`
	SteppingStone      bool     `json:"stepping_stone,omitempty"`
	SteppingStoneBasis string   `json:"stepping_stone_basis,omitempty"`
	DiversityRole      string   `json:"diversity_role,omitempty"`
	AssumptionNotes    string   `json:"assumption_notes,omitempty"`
	RollbackNotes      string   `json:"rollback_notes,omitempty"`
	EvidenceRefs       []string `json:"evidence_refs,omitempty"`
}

const (
	MissingDataPolicyExplicitAbstain = "explicit_abstain"
	MissingDataPolicyZero            = "zero"
	MissingDataPolicyExclude         = "exclude"
)

// NormRule captures a single normalization rule for a comparison dimension.
type NormRule struct {
	Dimension string `json:"dimension"`
	Method    string `json:"method"`
}

// ParityPlan captures the conditions under which a comparison is fair.
type ParityPlan struct {
	BaselineSet       []string   `json:"baseline_set,omitempty"`
	Window            string     `json:"window,omitempty"`
	Budget            string     `json:"budget,omitempty"`
	Normalization     []NormRule `json:"normalization,omitempty"`
	MissingDataPolicy string     `json:"missing_data_policy,omitempty"`
	PinnedConditions  []string   `json:"pinned_conditions,omitempty"`
}

// IsStructured reports whether the plan is complete enough for strict parity enforcement.
func (p ParityPlan) IsStructured() bool {
	return len(p.BaselineSet) > 0 &&
		p.Window != "" &&
		p.Budget != "" &&
		p.MissingDataPolicy != ""
}

// CharacterizationSnapshot stores a single characterization revision in structured form.
type CharacterizationSnapshot struct {
	Version    int                   `json:"version"`
	Dimensions []ComparisonDimension `json:"dimensions,omitempty"`
	ParityPlan *ParityPlan           `json:"parity_plan,omitempty"`
}

// PortfolioFields holds structured data for a SolutionPortfolio. Stored as JSON in StructuredData.
type PortfolioFields struct {
	ProblemRef               string            `json:"problem_ref,omitempty"`
	Variants                 []Variant         `json:"variants,omitempty"`
	Comparison               *ComparisonResult `json:"comparison,omitempty"`
	NoSteppingStoneRationale string            `json:"no_stepping_stone_rationale,omitempty"`
}

type DominatedVariantExplanation struct {
	Variant     string   `json:"variant"`
	DominatedBy []string `json:"dominated_by,omitempty"`
	Summary     string   `json:"summary"`
}

type ParetoTradeoffNote struct {
	Variant string `json:"variant"`
	Summary string `json:"summary"`
}

// ComparisonResult holds the outcome of comparing variants.
type ComparisonResult struct {
	Dimensions              []string                      `json:"dimensions"`
	Scores                  map[string]map[string]string  `json:"scores"` // variant_id -> dimension -> value
	NonDominatedSet         []string                      `json:"non_dominated_set"`
	Incomparable            [][]string                    `json:"incomparable,omitempty"`
	DominatedVariants       []DominatedVariantExplanation `json:"dominated_variants,omitempty"`
	ParetoTradeoffs         []ParetoTradeoffNote          `json:"pareto_tradeoffs,omitempty"`
	PolicyApplied           string                        `json:"policy_applied,omitempty"`
	SelectedRef             string                        `json:"selected_ref,omitempty"`
	RecommendationRationale string                        `json:"recommendation_rationale,omitempty"`
	ParityPlan              *ParityPlan                   `json:"parity_plan,omitempty"`
}

// ClaimStatus is the canonical runtime verification state for a prediction or claim.
type ClaimStatus string

const (
	ClaimStatusUnverified   ClaimStatus = "unverified"
	ClaimStatusSupported    ClaimStatus = "supported"
	ClaimStatusWeakened     ClaimStatus = "weakened"
	ClaimStatusRefuted      ClaimStatus = "refuted"
	ClaimStatusInconclusive ClaimStatus = "inconclusive"
)

// DecisionClaim is the canonical stored runtime state for one decision claim.
type DecisionClaim struct {
	ID          string      `json:"id"`
	Claim       string      `json:"claim"`
	Observable  string      `json:"observable"`
	Threshold   string      `json:"threshold"`
	Status      ClaimStatus `json:"status,omitempty"`
	VerifyAfter string      `json:"verify_after,omitempty"` // RFC3339 or YYYY-MM-DD — when async evidence should be gathered
}

// DecisionPrediction is a compatibility projection of a stored decision claim.
type DecisionPrediction struct {
	Claim       string      `json:"claim"`
	Observable  string      `json:"observable"`
	Threshold   string      `json:"threshold"`
	Status      ClaimStatus `json:"status,omitempty"`
	VerifyAfter string      `json:"verify_after,omitempty"`
}

// EvidenceItem represents a single piece of evidence.
type EvidenceItem struct {
	ID              string   `json:"id"`
	Type            string   `json:"type"` // measurement, test, research, benchmark, audit
	Content         string   `json:"content"`
	Verdict         string   `json:"verdict,omitempty"` // supports, weakens, refutes
	CarrierRef      string   `json:"carrier_ref,omitempty"`
	CongruenceLevel int      `json:"congruence_level,omitempty"` // 0-3
	FormalityLevel  int      `json:"formality_level,omitempty"`  // F0-F3 (legacy 0-9 normalized on read)
	ClaimRefs       []string `json:"claim_refs,omitempty"`
	ClaimScope      []string `json:"claim_scope,omitempty"`
	ValidUntil      string   `json:"valid_until,omitempty"`
}

// WriteWarning is returned when the operation succeeded but with non-fatal warnings.
// Callers should check errors with errors.As(*WriteWarning) and surface warnings to user.
type WriteWarning struct {
	Warnings []string
}

func (w *WriteWarning) Error() string {
	return fmt.Sprintf("completed with %d warning(s): %s", len(w.Warnings), w.Warnings[0])
}

// AffectedFile tracks which files a decision touches.
type AffectedFile struct {
	Path string `json:"path"`
	Hash string `json:"hash,omitempty"` // SHA256 at baseline time
}

// AffectedSymbol captures a symbol-level baseline snapshot.
// Used for tree-sitter powered drift detection at function/type granularity.
type AffectedSymbol struct {
	FilePath   string `json:"file_path"`
	SymbolName string `json:"symbol_name"`
	SymbolKind string `json:"symbol_kind"` // func, type, class, interface, method
	Line       int    `json:"line"`
	EndLine    int    `json:"end_line"`
	Hash       string `json:"hash"` // SHA256 of symbol source
}

// DriftStatus represents the state of a file relative to its baseline.
type DriftStatus string

const (
	DriftNone       DriftStatus = "no_drift"
	DriftModified   DriftStatus = "modified"
	DriftAdded      DriftStatus = "added"
	DriftMissing    DriftStatus = "file_missing"
	DriftNoBaseline DriftStatus = "no_baseline"
)

// DriftItem describes drift for a single file.
type DriftItem struct {
	Path         string      `json:"path"`
	Status       DriftStatus `json:"status"`
	LinesChanged string      `json:"lines_changed,omitempty"` // e.g., "+8 -2"
	Invariants   []string    `json:"invariants,omitempty"`
}

// DriftReport describes drift for a single decision.
type DriftReport struct {
	DecisionID        string         `json:"decision_id"`
	DecisionTitle     string         `json:"decision_title"`
	HasBaseline       bool           `json:"has_baseline"`
	LikelyImplemented bool           `json:"likely_implemented,omitempty"` // no baseline but files changed in git since decision
	Files             []DriftItem    `json:"files,omitempty"`
	ImpactedModules   []ModuleImpact `json:"impacted_modules,omitempty"` // Level C: impact propagation
}

// ModuleImpact describes a dependent module affected by drift propagation.
type ModuleImpact struct {
	ModuleID    string   `json:"module_id"`
	ModulePath  string   `json:"module_path"`
	DecisionIDs []string `json:"decision_ids,omitempty"` // decisions governing this module
	IsBlind     bool     `json:"is_blind"`               // no decisions = unmonitored impact
}
