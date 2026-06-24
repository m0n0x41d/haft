package artifact

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/reff"
)

// Kind identifies the type of artifact.
type Kind string

const (
	KindNote              Kind = "Note"
	KindProblemCard       Kind = "ProblemCard"
	KindSolutionPortfolio Kind = "SolutionPortfolio"
	KindDecisionRecord    Kind = "DecisionRecord"
	KindWorkCommission    Kind = "WorkCommission"
	KindMethodRun         Kind = "MethodRun"
	KindEvidencePack      Kind = "EvidencePack"
	KindRefreshReport     Kind = "RefreshReport"
)

// validKinds is the set of all valid artifact kinds (unexported — use ParseKind at boundaries).
var validKinds = map[Kind]bool{
	KindNote: true, KindProblemCard: true, KindSolutionPortfolio: true,
	KindDecisionRecord: true, KindWorkCommission: true, KindMethodRun: true,
	KindEvidencePack: true, KindRefreshReport: true,
}

// IsValid returns true if the kind is a recognized artifact kind.
func (k Kind) IsValid() bool { return validKinds[k] }

// ValidKindNames returns recognized artifact kind names in stable display order.
func ValidKindNames() []string {
	return []string{
		string(KindNote),
		string(KindProblemCard),
		string(KindSolutionPortfolio),
		string(KindDecisionRecord),
		string(KindWorkCommission),
		string(KindMethodRun),
		string(KindEvidencePack),
		string(KindRefreshReport),
	}
}

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
	case KindWorkCommission:
		return "wc"
	case KindMethodRun:
		return "mpull"
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
	case KindWorkCommission:
		return "commissions"
	case KindMethodRun:
		return "method-runs"
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
	case KindWorkCommission:
		return "work commission"
	case KindMethodRun:
		return "method run"
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
	case KindWorkCommission:
		if count == 1 {
			return "Work Commission"
		}
		return "Work Commissions"
	case KindMethodRun:
		if count == 1 {
			return "Method Run"
		}
		return "Method Runs"
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
	Profile               *ProblemCardProfile        `json:"profile,omitempty"`
	Constraints           []string                   `json:"constraints,omitempty"`
	OptimizationTargets   []string                   `json:"optimization_targets,omitempty"`
	ObservationIndicators []string                   `json:"observation_indicators,omitempty"`
	Acceptance            string                     `json:"acceptance,omitempty"`
	BlastRadius           string                     `json:"blast_radius,omitempty"`
	Reversibility         string                     `json:"reversibility,omitempty"`
	Characterizations     []CharacterizationSnapshot `json:"characterizations,omitempty"`
	Semantic              *SemanticEnvelope          `json:"semantic,omitempty"`
}

type ProblemCardProfile struct {
	Level                string   `json:"level"`
	SourceKind           string   `json:"source_kind,omitempty"`
	Readiness            string   `json:"readiness"`
	BoundaryStatus       string   `json:"boundary_status"`
	WhyNow               string   `json:"why_now,omitempty"`
	Scope                string   `json:"scope,omitempty"`
	AcceptanceProbe      string   `json:"acceptance_probe,omitempty"`
	FreshnessDisposition string   `json:"freshness_disposition,omitempty"`
	Blockers             []string `json:"blockers,omitempty"`
}

type SemanticStatus string

const (
	SemanticStatusExact    SemanticStatus = "exact"
	SemanticStatusLegacy   SemanticStatus = "legacy"
	SemanticStatusDegraded SemanticStatus = "degraded"
)

type SemanticEnvelope struct {
	SchemaVersion         int                   `json:"schema_version"`
	Status                SemanticStatus        `json:"status"`
	Profile               FPFProfileRef         `json:"profile"`
	SemanticEdition       SemanticEditionRef    `json:"semantic_edition"`
	ReferenceScheme       ReferenceScheme       `json:"reference_scheme"`
	CarrierBinding        CarrierBinding        `json:"carrier_binding"`
	PublicationProjection PublicationProjection `json:"publication_projection"`
	PublicationUnit       PublicationUnit       `json:"publication_unit"`
	EvidencePathRefs      []string              `json:"evidence_path_refs,omitempty"`
	Warnings              []string              `json:"warnings,omitempty"`
}

type FPFProfileRef struct {
	ID         string `json:"id"`
	SourceKind string `json:"source_kind"`
	SourceRef  string `json:"source_ref"`
	Hash       string `json:"hash"`
	ValidUntil string `json:"valid_until,omitempty"`
}

type SemanticEditionRef struct {
	ID        string `json:"id"`
	Family    string `json:"family"`
	Version   int    `json:"version"`
	CreatedAt string `json:"created_at,omitempty"`
	Hash      string `json:"hash,omitempty"`
}

type ReferenceScheme struct {
	Primary string   `json:"primary"`
	Anchors []string `json:"anchors,omitempty"`
}

type CarrierBinding struct {
	CarrierKind   string `json:"carrier_kind"`
	CarrierRef    string `json:"carrier_ref"`
	StorageKind   string `json:"storage_kind"`
	SourceOfTruth string `json:"source_of_truth"`
}

type PublicationProjection struct {
	ProjectionKind string   `json:"projection_kind"`
	Views          []string `json:"views,omitempty"`
	SyncPolicy     string   `json:"sync_policy"`
	Hash           string   `json:"hash,omitempty"`
}

type PublicationUnit struct {
	SchemaVersion    int                       `json:"schema_version"`
	SourceEditionPin SourceEditionPin          `json:"source_edition_pin"`
	PublicationHash  string                    `json:"publication_hash"`
	CarrierHash      string                    `json:"carrier_hash"`
	OmittedFields    []string                  `json:"omitted_fields,omitempty"`
	Losses           []PublicationLoss         `json:"losses,omitempty"`
	Recoverability   PublicationRecoverability `json:"recoverability"`
}

type SourceEditionPin struct {
	Ref    string `json:"ref"`
	Hash   string `json:"hash,omitempty"`
	Status string `json:"status"`
}

type PublicationLoss struct {
	Field       string `json:"field"`
	Reason      string `json:"reason"`
	Recoverable bool   `json:"recoverable"`
}

type PublicationRecoverability struct {
	Status    string   `json:"status"`
	Mechanism []string `json:"mechanism,omitempty"`
}

// DecisionFields holds structured data for a DecisionRecord. Stored as JSON in StructuredData.
type DecisionFields struct {
	ProblemRefs             []string                `json:"problem_refs,omitempty"`
	DecisionSubjectRef      string                  `json:"decision_subject_ref,omitempty"`
	ChoiceResult            *ChoiceResult           `json:"choice_result,omitempty"`
	TransformationRecord    *TransformationRecord   `json:"transformation_record,omitempty"`
	SelectedTitle           string                  `json:"selected_title"`
	WhySelected             string                  `json:"why_selected"`
	SelectionPolicy         string                  `json:"selection_policy,omitempty"`
	CounterArgument         string                  `json:"counterargument,omitempty"`
	WeakestLink             string                  `json:"weakest_link,omitempty"`
	TaskContext             string                  `json:"task_context,omitempty"`
	SectionRefs             []string                `json:"section_refs,omitempty"`
	WhyNotOthers            []RejectionReason       `json:"why_not_others,omitempty"`
	Claims                  []DecisionClaim         `json:"claims,omitempty"`
	Predictions             []DecisionPrediction    `json:"predictions,omitempty"`
	PreConditions           []string                `json:"pre_conditions,omitempty"`
	RollbackTriggers        []string                `json:"rollback_triggers,omitempty"`
	RollbackSteps           []string                `json:"rollback_steps,omitempty"`
	RollbackBlastRadius     string                  `json:"rollback_blast_radius,omitempty"`
	Invariants              []string                `json:"invariants,omitempty"`
	PostConds               []string                `json:"post_conditions,omitempty"`
	Admissibility           []string                `json:"admissibility,omitempty"`
	EvidenceRequirements    []string                `json:"evidence_requirements,omitempty"`
	RefreshTriggers         []string                `json:"refresh_triggers,omitempty"`
	Skips                   []string                `json:"_skips,omitempty"`
	SkipReason              string                  `json:"_skip_reason,omitempty"`
	FirstModuleCoverage     bool                    `json:"first_module_coverage,omitempty"`
	ImplementationFootprint ImplementationFootprint `json:"implementation_footprint,omitempty"`
	GovernanceTargets       []GovernanceTarget      `json:"governance_targets,omitempty"`
	DriftWatchTargets       []DriftWatchTarget      `json:"drift_watch_targets,omitempty"`
	DriftManifests          []DriftScopeManifest    `json:"drift_manifests,omitempty"`
	BindingTargets          []BindingTarget         `json:"binding_targets,omitempty"`
	BindingDiagnostics      []BindingDiagnostic     `json:"binding_diagnostics,omitempty"`
	// GovernanceMode declares how affected_files relate to drift detection.
	// "module" (default, preserves pre-6.2.x behavior): each affected_file
	// widens to its parent directory; sibling additions count as governed
	// drift. "exact": only the listed files are governed. See GovernanceMode.
	GovernanceMode GovernanceMode `json:"governance_mode,omitempty"`
}

type ImplementationFootprint struct {
	Files    []string `json:"files,omitempty"`
	Commits  []string `json:"commits,omitempty"`
	WorkRefs []string `json:"work_refs,omitempty"`
}

type GovernanceTarget struct {
	Kind          string         `json:"kind"`
	Ref           string         `json:"ref,omitempty"`
	BindingTarget *BindingTarget `json:"binding_target,omitempty"`
}

type DriftWatchTarget struct {
	TargetRef     string         `json:"target_ref"`
	Trigger       string         `json:"trigger"`
	BindingTarget *BindingTarget `json:"binding_target,omitempty"`
}

// BindingTarget is the canonical code-object binding model for decision drift.
// affected_files and affected_symbols remain backward-compatible projections.
type BindingTarget struct {
	Kind             string `json:"kind"`
	TargetRef        string `json:"target_ref,omitempty"`
	FilePath         string `json:"file_path,omitempty"`
	Language         string `json:"language,omitempty"`
	SymbolName       string `json:"symbol_name,omitempty"`
	SymbolKind       string `json:"symbol_kind,omitempty"`
	Receiver         string `json:"receiver,omitempty"`
	Line             int    `json:"line,omitempty"`
	EndLine          int    `json:"end_line,omitempty"`
	BodyHash         string `json:"body_hash,omitempty"`
	AnchorHash       string `json:"anchor_hash,omitempty"`
	TextHash         string `json:"text_hash,omitempty"`
	NearestSymbol    string `json:"nearest_symbol,omitempty"`
	ModulePath       string `json:"module_path,omitempty"`
	ModuleHash       string `json:"module_hash,omitempty"`
	Reason           string `json:"reason,omitempty"`
	WhySymbolFailed  string `json:"why_symbol_failed,omitempty"`
	WhyRangeFailed   string `json:"why_range_failed,omitempty"`
	LanguageSupport  string `json:"language_support,omitempty"`
	Confidence       string `json:"confidence,omitempty"`
	ResolutionSource string `json:"resolution_source,omitempty"`
}

type BindingDiagnostic struct {
	FilePath string `json:"file_path,omitempty"`
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type decisionFieldsJSON DecisionFields

func (df DecisionFields) MarshalJSON() ([]byte, error) {
	encoded := decisionFieldsJSON(df)
	encoded.Claims = normalizeDecisionClaims(encoded.Claims)
	encoded.TransformationRecord = NormalizeTransformationRecord(encoded.TransformationRecord)

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
	decoded.TransformationRecord = NormalizeTransformationRecord(decoded.TransformationRecord)
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
	df.ChoiceResult = NormalizeChoiceResult(df.ChoiceResult)
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
	if pf.Comparison != nil {
		normalized := normalizeComparisonRecommendationAlias(*pf.Comparison)
		pf.Comparison = &normalized
	}
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

var idSlugUnsafeChars = regexp.MustCompile(`[^a-z0-9]+`)

const maxIDSlugLength = 48

// GenerateID creates a unique artifact ID with the format
// `<prefix>-YYYYMMDD-<8 hex chars>` (e.g. `dec-20260418-a3f7c1d2`).
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
	return GenerateIDWithTaskContext(kind, seq, "")
}

// GenerateIDWithTaskContext creates an artifact ID with optional filename-safe
// task context before the random suffix, e.g. `dec-20260424-task-4-a3f7c1d2`.
// Empty or fully-invalid context falls back to the default GenerateID shape.
func GenerateIDWithTaskContext(kind Kind, seq int, taskContext string) string {
	_ = seq // legacy parameter; collision resistance is provided by hex suffix
	date := time.Now().Format("20060102")
	slug := sanitizeIDSlug(taskContext)
	suffix := randomIDSuffix()

	return formatGeneratedID(kind.IDPrefix(), date, slug, suffix)
}

func formatGeneratedID(prefix, date, slug, suffix string) string {
	if slug == "" {
		return fmt.Sprintf("%s-%s-%s", prefix, date, suffix)
	}

	return fmt.Sprintf("%s-%s-%s-%s", prefix, date, slug, suffix)
}

func sanitizeIDSlug(value string) string {
	slug := strings.TrimSpace(value)
	slug = strings.ToLower(slug)
	slug = idSlugUnsafeChars.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	slug = truncateIDSlug(slug)
	slug = strings.Trim(slug, "-")

	return slug
}

func truncateIDSlug(slug string) string {
	if len(slug) <= maxIDSlugLength {
		return slug
	}

	return slug[:maxIDSlugLength]
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

// ChoiceNextMove is the exact next action produced by a human choice.
type ChoiceNextMove string

const (
	ChoiceNextMoveChooseNow        ChoiceNextMove = "choose_now"
	ChoiceNextMoveRejectCurrentSet ChoiceNextMove = "reject_current_set"
	ChoiceNextMoveProbeAgain       ChoiceNextMove = "probe_again"
	ChoiceNextMoveReroute          ChoiceNextMove = "reroute"
)

// ValidChoiceNextMoveNames returns exact ChoiceResult next_move values.
func ValidChoiceNextMoveNames() []string {
	return []string{
		string(ChoiceNextMoveChooseNow),
		string(ChoiceNextMoveRejectCurrentSet),
		string(ChoiceNextMoveProbeAgain),
		string(ChoiceNextMoveReroute),
	}
}

// ChoiceResult records the human-side choice outcome. ComparisonResult may
// recommend; it does not create this object.
type ChoiceResult struct {
	SubjectRef      string         `json:"subject_ref"`
	OptionSet       []string       `json:"option_set,omitempty"`
	ComparisonBasis []string       `json:"comparison_basis,omitempty"`
	ChoiceRule      string         `json:"choice_rule,omitempty"`
	NextMove        ChoiceNextMove `json:"next_move"`
	VariantRef      string         `json:"variant_ref,omitempty"`
	ProblemRefs     []string       `json:"problem_refs,omitempty"`
	PortfolioRef    string         `json:"portfolio_ref,omitempty"`
	Reason          string         `json:"reason,omitempty"`
	Reversibility   string         `json:"reversibility,omitempty"`
	ReopenCondition string         `json:"reopen_condition,omitempty"`
}

const TransformationRecordSchemaVersion = 1

// TransformationRecord describes the target object-state transformation only.
// Method, work authorization, evidence, and publication remain separate
// record families.
type TransformationRecord struct {
	SchemaVersion     int      `json:"schema_version"`
	TransformedEntity string   `json:"transformed_entity"`
	InitialState      string   `json:"initial_state"`
	PostState         string   `json:"post_state"`
	Relation          string   `json:"relation"`
	Context           string   `json:"context"`
	Window            string   `json:"window,omitempty"`
	MethodRefs        []string `json:"method_refs,omitempty"`
	WorkRefs          []string `json:"work_refs,omitempty"`
	EvidenceRefs      []string `json:"evidence_refs,omitempty"`
	PublicationRefs   []string `json:"publication_refs,omitempty"`
}

func NormalizeTransformationRecord(record *TransformationRecord) *TransformationRecord {
	if record == nil {
		return nil
	}

	normalized := &TransformationRecord{
		SchemaVersion:     record.SchemaVersion,
		TransformedEntity: strings.TrimSpace(record.TransformedEntity),
		InitialState:      strings.TrimSpace(record.InitialState),
		PostState:         strings.TrimSpace(record.PostState),
		Relation:          strings.TrimSpace(record.Relation),
		Context:           strings.TrimSpace(record.Context),
		Window:            strings.TrimSpace(record.Window),
		MethodRefs:        compactStrings(record.MethodRefs),
		WorkRefs:          compactStrings(record.WorkRefs),
		EvidenceRefs:      compactStrings(record.EvidenceRefs),
		PublicationRefs:   compactStrings(record.PublicationRefs),
	}
	if normalized.SchemaVersion == 0 {
		normalized.SchemaVersion = TransformationRecordSchemaVersion
	}

	return normalized
}

func ValidateTransformationRecord(record *TransformationRecord) error {
	normalized := NormalizeTransformationRecord(record)
	if normalized == nil {
		return nil
	}

	if normalized.SchemaVersion != TransformationRecordSchemaVersion {
		return fmt.Errorf("transformation_record.schema_version %d is unsupported; expected %d",
			normalized.SchemaVersion,
			TransformationRecordSchemaVersion)
	}

	missing := []string{}
	if normalized.TransformedEntity == "" {
		missing = append(missing, "transformed_entity")
	}
	if normalized.InitialState == "" {
		missing = append(missing, "initial_state")
	}
	if normalized.PostState == "" {
		missing = append(missing, "post_state")
	}
	if normalized.Relation == "" {
		missing = append(missing, "relation")
	}
	if normalized.Context == "" {
		missing = append(missing, "context")
	}
	if len(missing) > 0 {
		return fmt.Errorf("transformation_record missing required field(s): %s", strings.Join(missing, ", "))
	}

	return nil
}

// NormalizeChoiceResult trims a choice without inventing one from legacy data.
func NormalizeChoiceResult(choice *ChoiceResult) *ChoiceResult {
	if choice == nil {
		return nil
	}

	normalized := &ChoiceResult{
		SubjectRef:      strings.TrimSpace(choice.SubjectRef),
		OptionSet:       compactStrings(choice.OptionSet),
		ComparisonBasis: compactStrings(choice.ComparisonBasis),
		ChoiceRule:      strings.TrimSpace(choice.ChoiceRule),
		NextMove:        ChoiceNextMove(strings.TrimSpace(string(choice.NextMove))),
		VariantRef:      strings.TrimSpace(choice.VariantRef),
		ProblemRefs:     compactStrings(choice.ProblemRefs),
		PortfolioRef:    strings.TrimSpace(choice.PortfolioRef),
		Reason:          strings.TrimSpace(choice.Reason),
		Reversibility:   strings.TrimSpace(choice.Reversibility),
		ReopenCondition: strings.TrimSpace(choice.ReopenCondition),
	}

	return normalized
}

// ValidateChoiceResult enforces exact choice next_move semantics.
func ValidateChoiceResult(choice *ChoiceResult) error {
	normalized := NormalizeChoiceResult(choice)
	if normalized == nil {
		return nil
	}

	if normalized.SubjectRef == "" {
		return fmt.Errorf("choice_result.subject_ref is required — subject is the chooser-bearing human/team/system, not the decision question text")
	}
	if !isValidChoiceNextMove(normalized.NextMove) {
		return fmt.Errorf("choice_result.next_move %q is invalid; expected one of: %s",
			normalized.NextMove,
			strings.Join(ValidChoiceNextMoveNames(), ", "))
	}
	if normalized.NextMove == ChoiceNextMoveChooseNow && normalized.VariantRef == "" {
		return fmt.Errorf("choice_result.variant_ref is required when next_move=choose_now")
	}
	if len(normalized.OptionSet) > 0 && normalized.VariantRef != "" && !stringInSlice(normalized.OptionSet, normalized.VariantRef) {
		return fmt.Errorf("choice_result.variant_ref %q is outside choice_result.option_set", normalized.VariantRef)
	}

	return nil
}

type DecisionChoiceResultInput struct {
	ProblemRefs     []string
	PortfolioRef    string
	SelectedTitle   string
	WhySelected     string
	WhyNotOthers    []RejectionReason
	SelectionPolicy string
	ReopenCondition string
}

// NewDecisionChoiceResult creates the exact choice emitted by explicit h-decide.
func NewDecisionChoiceResult(input DecisionChoiceResultInput) *ChoiceResult {
	choice := &ChoiceResult{
		SubjectRef:      "operator",
		OptionSet:       decisionChoiceOptionSet(input),
		ComparisonBasis: decisionChoiceComparisonBasis(input),
		ChoiceRule:      strings.TrimSpace(input.SelectionPolicy),
		NextMove:        ChoiceNextMoveChooseNow,
		VariantRef:      strings.TrimSpace(input.SelectedTitle),
		ProblemRefs:     compactStrings(input.ProblemRefs),
		PortfolioRef:    strings.TrimSpace(input.PortfolioRef),
		Reason:          strings.TrimSpace(input.WhySelected),
		ReopenCondition: strings.TrimSpace(input.ReopenCondition),
	}

	return choice
}

func decisionChoiceOptionSet(input DecisionChoiceResultInput) []string {
	options := []string{input.SelectedTitle}
	for _, rejected := range input.WhyNotOthers {
		options = append(options, rejected.Variant)
	}

	return compactStrings(options)
}

func decisionChoiceComparisonBasis(input DecisionChoiceResultInput) []string {
	basis := []string{}
	if input.WhySelected != "" {
		basis = append(basis, fmt.Sprintf("selected %s: %s", input.SelectedTitle, input.WhySelected))
	}
	for _, rejected := range input.WhyNotOthers {
		if rejected.Variant == "" || rejected.Reason == "" {
			continue
		}
		basis = append(basis, fmt.Sprintf("rejected %s: %s", rejected.Variant, rejected.Reason))
	}

	return compactStrings(basis)
}

func stringInSlice(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}

	return false
}

func isValidChoiceNextMove(nextMove ChoiceNextMove) bool {
	for _, valid := range ValidChoiceNextMoveNames() {
		if string(nextMove) == valid {
			return true
		}
	}

	return false
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
	LegacyRecommendationRef string                        `json:"legacy_recommendation_ref,omitempty"`
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

// ClaimLifecycleStatus is the governance lifecycle of a decision claim. Empty
// legacy values read as active through EffectiveClaimLifecycleStatus.
type ClaimLifecycleStatus string

const (
	ClaimLifecycleActive     ClaimLifecycleStatus = "active"
	ClaimLifecycleRefreshDue ClaimLifecycleStatus = "refresh_due"
	ClaimLifecycleSuperseded ClaimLifecycleStatus = "superseded"
	ClaimLifecycleDeprecated ClaimLifecycleStatus = "deprecated"
)

// DecisionClaim is the canonical stored runtime state for one decision claim.
type DecisionClaim struct {
	ID                   string               `json:"id"`
	Claim                string               `json:"claim"`
	Observable           string               `json:"observable"`
	Threshold            string               `json:"threshold"`
	Status               ClaimStatus          `json:"status,omitempty"`
	LifecycleStatus      ClaimLifecycleStatus `json:"lifecycle_status,omitempty"`
	SuccessorRef         string               `json:"successor_ref,omitempty"`
	RetiredReason        string               `json:"retired_reason,omitempty"`
	GovernanceTargetRefs []string             `json:"governance_target_refs,omitempty"`
	VerifyAfter          string               `json:"verify_after,omitempty"`  // RFC3339 or YYYY-MM-DD — when async evidence should be gathered
	Realizability        RealizabilityVerdict `json:"realizability,omitempty"` // C.28 CounterfactualSamplingRealizabilityProfile verdict
	// Probability is the optional elicited p(this claim holds) in [0,1], a noisy
	// forecast captured at /h-decide time. Paired with the verified Status
	// (supported→1 / refuted→0) it forms a Forecast for decomposed-Brier
	// calibration (dec-20260603-c3c7fa88). Additive: nil means no forecast.
	Probability *float64 `json:"probability,omitempty"`
	// Command is the optional machine-checkable form of Observable: an
	// allowlist-class command (go test/build/vet, grep/rg) the maintenance
	// loop may execute out-of-band (dec-20260611-overseer-maintenance-executor).
	// Empty means the observable needs judgment. Additive.
	Command string `json:"command,omitempty"`
}

type ClaimLifecycleSummary struct {
	Active               int      `json:"active"`
	RefreshDue           int      `json:"refresh_due"`
	Superseded           int      `json:"superseded"`
	Deprecated           int      `json:"deprecated"`
	GovernanceTargetRefs []string `json:"governance_target_refs,omitempty"`
}

// DecisionPrediction is a compatibility projection of a stored decision claim.
type DecisionPrediction struct {
	Claim         string               `json:"claim"`
	Observable    string               `json:"observable"`
	Threshold     string               `json:"threshold"`
	Status        ClaimStatus          `json:"status,omitempty"`
	VerifyAfter   string               `json:"verify_after,omitempty"`
	Realizability RealizabilityVerdict `json:"realizability,omitempty"`
	Probability   *float64             `json:"probability,omitempty"`
	Command       string               `json:"command,omitempty"`
}

// EvidenceItem represents a single piece of evidence.
type EvidenceItem struct {
	ID                 string                     `json:"id"`
	Type               string                     `json:"type"` // measurement, test, research, benchmark, audit
	Content            string                     `json:"content"`
	Verdict            string                     `json:"verdict,omitempty"` // supports, weakens, refutes
	CarrierRef         string                     `json:"carrier_ref,omitempty"`
	CongruenceLevel    int                        `json:"congruence_level,omitempty"` // 0-3
	FormalityLevel     int                        `json:"formality_level,omitempty"`  // F0-F9; legacy F0-F3 remains readable
	FormalityScale     *reff.FormalityScale       `json:"formality_scale,omitempty"`
	FormalityBridge    *reff.FormalityBridge      `json:"formality_bridge,omitempty"`
	ClaimRefs          []string                   `json:"claim_refs,omitempty"`
	ClaimScope         []string                   `json:"claim_scope,omitempty"`
	ValidUntil         string                     `json:"valid_until,omitempty"`
	CausalSupportBasis CausalEvidenceSupportBasis `json:"causal_support_basis,omitempty"` // C.28 basis for causal-use claim support
	// Provenance distinguishes who collected this evidence: "" (human/agent in
	// session), "machine" (maintenance loop ran an allowlisted observable), or
	// "llm-review" (overseer reviewer proposal). Machine evidence must always
	// be distinguishable from human-reviewed evidence (invariant of
	// dec-20260611-overseer-maintenance-executor). Additive.
	Provenance string `json:"provenance,omitempty"`
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

// DriftMateriality names the semantic posture of a drift item. It is additive
// presentation metadata: legacy consumers can ignore it and keep reading status.
type DriftMateriality string

const (
	DriftMaterialityMaterialSymbol         DriftMateriality = "material_symbol"
	DriftMaterialityMaterialSemanticTarget DriftMateriality = "material_semantic_target"
	DriftMaterialityAdjacentFileChurn      DriftMateriality = "adjacent_file_churn"
	DriftMaterialityCarrierOnly            DriftMateriality = "carrier_only"
	DriftMaterialityGeneratedOrIgnored     DriftMateriality = "generated_or_ignored"
	DriftMaterialityUnknownLegacyFileScope DriftMateriality = "unknown_legacy_file_scope"
	DriftMaterialityNeedsBindingResolution DriftMateriality = "needs_binding_resolution"
)

// DriftTriggerKind describes the mechanical trigger that made the item appear.
type DriftTriggerKind string

const (
	DriftTriggerFileHash        DriftTriggerKind = "file_hash"
	DriftTriggerMissingFile     DriftTriggerKind = "missing_file"
	DriftTriggerMissingBaseline DriftTriggerKind = "missing_baseline"
	DriftTriggerScopeManifest   DriftTriggerKind = "scope_manifest"
)

// DriftItem describes drift for a single file.
type DriftItem struct {
	Path             string            `json:"path"`
	Status           DriftStatus       `json:"status"`
	LinesChanged     string            `json:"lines_changed,omitempty"` // e.g., "+8 -2"
	Invariants       []string          `json:"invariants,omitempty"`
	ClaimRefs        []string          `json:"claim_refs,omitempty"`
	EvidenceRefs     []string          `json:"evidence_refs,omitempty"`
	Symbols          []SymbolDriftItem `json:"symbols,omitempty"` // symbol-level breakdown for a modified file
	Materiality      DriftMateriality  `json:"materiality,omitempty"`
	TriggerKind      DriftTriggerKind  `json:"trigger_kind,omitempty"`
	ChangedTargetRef string            `json:"changed_target_ref,omitempty"`
	TargetKind       string            `json:"target_kind,omitempty"`
	TargetStatus     string            `json:"target_status,omitempty"`
	FallbackKind     string            `json:"fallback_kind,omitempty"`
	FallbackReason   string            `json:"fallback_reason,omitempty"`
	AuditOnly        bool              `json:"audit_only,omitempty"`
	SuppressedReason string            `json:"suppressed_reason,omitempty"`
}

// SymbolDriftItem is the per-symbol change inside a modified governed file.
// It is the deterministic signal the session-start triage partitions on:
// a file whose only symbol drift is "added" is structurally additive; any
// "modified" or "removed" symbol means a governed body changed.
type SymbolDriftItem struct {
	SymbolName   string   `json:"symbol_name"`
	SymbolKind   string   `json:"symbol_kind"` // func, type, class, interface, method
	Status       string   `json:"status"`      // "added", "modified", "removed"
	ClaimRefs    []string `json:"claim_refs,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

// DriftReport describes drift for a single decision.
type DriftReport struct {
	DecisionID        string           `json:"decision_id"`
	DecisionTitle     string           `json:"decision_title"`
	HasBaseline       bool             `json:"has_baseline"`
	BaselineKind      BaselineKind     `json:"baseline_kind,omitempty"`
	BaselineProfile   *BaselineProfile `json:"baseline_profile,omitempty"`
	LikelyImplemented bool             `json:"likely_implemented,omitempty"` // no baseline but files changed in git since decision
	Files             []DriftItem      `json:"files,omitempty"`
	ImpactedModules   []ModuleImpact   `json:"impacted_modules,omitempty"` // Level C: impact propagation
}

// BaselineKind names the governance meaning of a baseline-like snapshot.
// Decision drift hashes are reference data until supporting evidence makes a
// verified-state projection honest; spec approval baselines live in
// project/specflow.
type BaselineKind string

const (
	BaselineKindUnknownLegacy         BaselineKind = "unknown_legacy_baseline"
	BaselineKindPreWorkReference      BaselineKind = "pre_work_reference_snapshot"
	BaselineKindObservedStateSnapshot BaselineKind = "observed_state_snapshot"
	BaselineKindSpecSectionApproval   BaselineKind = "spec_section_approval_baseline"
	BaselineKindVerifiedStateSnapshot BaselineKind = "verified_state_snapshot"
)

type BaselineProfile struct {
	Kind              BaselineKind `json:"kind"`
	Object            string       `json:"object"`
	AuthorityBoundary string       `json:"authority_boundary"`
	Diagnostic        string       `json:"diagnostic"`
}

func DecisionBaselineProfile(evidenceItems []EvidenceItem) BaselineProfile {
	if hasVerifiedStateEvidence(evidenceItems) {
		return VerifiedStateBaselineProfile()
	}
	return ObservedStateBaselineProfile()
}

func ObservedStateBaselineProfile() BaselineProfile {
	return BaselineProfile{
		Kind:              BaselineKindObservedStateSnapshot,
		Object:            "ObservedStateSnapshot",
		AuthorityBoundary: "drift_detection_reference_not_verification_or_approval",
		Diagnostic:        "affected_files hashes are observed reference data for drift detection; they do not prove verification without supporting evidence",
	}
}

func VerifiedStateBaselineProfile() BaselineProfile {
	return BaselineProfile{
		Kind:              BaselineKindVerifiedStateSnapshot,
		Object:            "VerifiedStateSnapshot",
		AuthorityBoundary: "drift_detection_snapshot_not_spec_approval_or_pre_work_reference",
		Diagnostic:        "affected_files hashes have supporting evidence and are projected as verified-state snapshots for decision drift detection",
	}
}

func hasVerifiedStateEvidence(items []EvidenceItem) bool {
	for _, item := range items {
		verdict := strings.ToLower(strings.TrimSpace(item.Verdict))
		if verdict != "supports" && verdict != "accepted" {
			continue
		}
		evidenceType := strings.ToLower(strings.TrimSpace(item.Type))
		switch evidenceType {
		case "verification", "measurement", "audit", "test", "focused_regression_tests", "focused_tests_and_public_behavior_check":
			return true
		}
		content := strings.ToLower(item.Content)
		if strings.Contains(content, "verification pass") || strings.Contains(content, "go test") || strings.Contains(content, "passed") {
			return true
		}
	}
	return false
}

// Symbol-level triage verdicts for a drift report. These partition session-start
// drift deterministically so benign additive drift can be auto-baselined while
// anything touching a governed symbol body is surfaced to the operator.
const (
	// SymbolVerdictAdditiveOnly: every modified file's drift is provably
	// additive (new symbols only) — safe to re-baseline without operator review.
	SymbolVerdictAdditiveOnly = "additive_only"
	// SymbolVerdictGovernedModified: a governed symbol body was modified or
	// removed (or a file deleted) — must reach the operator.
	SymbolVerdictGovernedModified = "governed_modified"
	// SymbolVerdictNeedsReview: the partition could not be proven benign
	// (no symbol baseline, unanalyzable language, or a change outside any
	// tracked symbol body) — fail-safe to the operator.
	SymbolVerdictNeedsReview = "needs_review"
)

// SymbolVerdict classifies this report's drift by symbol-level change. It is
// conservative by construction: a decision is SymbolVerdictAdditiveOnly only
// when EVERY modified file is provably additive; any governed-symbol
// modification/removal, deleted file, or unanalyzable change routes to the
// operator. This is the kernel-computed floor the agent triage keys off —
// never the agent's eyeball judgment.
func (r DriftReport) SymbolVerdict() string {
	sawAdditive := false
	needsReview := false
	for _, f := range r.Files {
		materiality := f.EffectiveMateriality()
		switch f.Status {
		case DriftMissing:
			if materiality == DriftMaterialityUnknownLegacyFileScope {
				needsReview = true
				continue
			}
			return SymbolVerdictGovernedModified
		case DriftModified:
			switch materiality {
			case DriftMaterialityMaterialSymbol, DriftMaterialityMaterialSemanticTarget:
				return SymbolVerdictGovernedModified
			case DriftMaterialityAdjacentFileChurn, DriftMaterialityCarrierOnly, DriftMaterialityGeneratedOrIgnored:
				sawAdditive = true
				continue
			case DriftMaterialityUnknownLegacyFileScope, DriftMaterialityNeedsBindingResolution:
				needsReview = true
				continue
			}
			if len(f.Symbols) == 0 {
				// File changed but no symbol evidence: extraction unavailable
				// or the change sits outside any tracked symbol body. Cannot
				// prove benign — fail-safe.
				needsReview = true
				continue
			}
			for _, s := range f.Symbols {
				if s.Status != "added" {
					return SymbolVerdictGovernedModified
				}
			}
			sawAdditive = true
		case DriftAdded:
			if materiality == DriftMaterialityUnknownLegacyFileScope || materiality == DriftMaterialityNeedsBindingResolution {
				needsReview = true
				continue
			}
			sawAdditive = true
		case DriftNoBaseline:
			needsReview = true
		}
	}
	if needsReview {
		return SymbolVerdictNeedsReview
	}
	if sawAdditive {
		return SymbolVerdictAdditiveOnly
	}
	return SymbolVerdictNeedsReview
}

func (f DriftItem) EffectiveMateriality() DriftMateriality {
	if f.Materiality != "" {
		return f.Materiality
	}
	switch f.Status {
	case DriftMissing:
		return DriftMaterialityMaterialSymbol
	case DriftNoBaseline:
		return DriftMaterialityUnknownLegacyFileScope
	case DriftAdded:
		return DriftMaterialityAdjacentFileChurn
	case DriftModified:
		if len(f.Symbols) == 0 {
			return DriftMaterialityUnknownLegacyFileScope
		}
		for _, symbol := range f.Symbols {
			if symbol.Status != "added" {
				return DriftMaterialityMaterialSymbol
			}
		}
		return DriftMaterialityAdjacentFileChurn
	default:
		return ""
	}
}

func (r DriftReport) EffectiveMateriality() DriftMateriality {
	sawUnknown := false
	sawNeedsResolution := false
	sawAdjacent := false
	sawCarrier := false
	sawGenerated := false
	for _, file := range r.Files {
		switch file.EffectiveMateriality() {
		case DriftMaterialityMaterialSymbol, DriftMaterialityMaterialSemanticTarget:
			return file.EffectiveMateriality()
		case DriftMaterialityUnknownLegacyFileScope:
			sawUnknown = true
		case DriftMaterialityNeedsBindingResolution:
			sawNeedsResolution = true
		case DriftMaterialityAdjacentFileChurn:
			sawAdjacent = true
		case DriftMaterialityCarrierOnly:
			sawCarrier = true
		case DriftMaterialityGeneratedOrIgnored:
			sawGenerated = true
		}
	}
	switch {
	case sawNeedsResolution:
		return DriftMaterialityNeedsBindingResolution
	case sawUnknown:
		return DriftMaterialityUnknownLegacyFileScope
	case sawAdjacent:
		return DriftMaterialityAdjacentFileChurn
	case sawCarrier:
		return DriftMaterialityCarrierOnly
	case sawGenerated:
		return DriftMaterialityGeneratedOrIgnored
	default:
		return ""
	}
}

// ModuleImpact describes a dependent module affected by drift propagation.
type ModuleImpact struct {
	ModuleID       string            `json:"module_id"`
	ModulePath     string            `json:"module_path"`
	DecisionIDs    []string          `json:"decision_ids,omitempty"`    // decisions governing this module
	DecisionTitles map[string]string `json:"decision_titles,omitempty"` // decision ID -> title, presentation support
	IsBlind        bool              `json:"is_blind"`                  // no decisions = unmonitored impact
}
