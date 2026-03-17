package artifact

import (
	"fmt"
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

// Dir returns the .quint/ subdirectory for this kind.
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

// Status represents artifact lifecycle status.
type Status string

const (
	StatusActive     Status = "active"
	StatusSuperseded Status = "superseded"
	StatusDeprecated Status = "deprecated"
	StatusRefreshDue Status = "refresh_due"
)

// Mode represents the decision depth mode.
type Mode string

const (
	ModeNote     Mode = "note"
	ModeTactical Mode = "tactical"
	ModeStandard Mode = "standard"
	ModeDeep     Mode = "deep"
)

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
	Meta Meta   `yaml:"meta" json:"meta"`
	Body string `yaml:"-" json:"body"` // markdown content after frontmatter
}

// GenerateID creates a deterministic artifact ID.
func GenerateID(kind Kind, seq int) string {
	date := time.Now().Format("20060102")
	return fmt.Sprintf("%s-%s-%03d", kind.IDPrefix(), date, seq)
}

// --- Domain-specific structured content ---
// These are parsed from the markdown body by tools, not stored in frontmatter.
// The Body field holds everything as markdown. These types exist for
// programmatic access when tools need structured data.

// Variant represents a solution option in a SolutionPortfolio.
type Variant struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Strengths        []string `json:"strengths,omitempty"`
	WeakestLink      string   `json:"weakest_link"`
	Risks            []string `json:"risks,omitempty"`
	SteppingStone    bool     `json:"stepping_stone,omitempty"`
	AssumptionNotes  string   `json:"assumption_notes,omitempty"`
	RollbackNotes    string   `json:"rollback_notes,omitempty"`
	EvidenceRefs     []string `json:"evidence_refs,omitempty"`
}

// ComparisonResult holds the outcome of comparing variants.
type ComparisonResult struct {
	Dimensions      []string            `json:"dimensions"`
	Scores          map[string]map[string]string `json:"scores"` // variant_id -> dimension -> value
	NonDominatedSet []string            `json:"non_dominated_set"`
	Incomparable    [][]string          `json:"incomparable,omitempty"`
	PolicyApplied   string              `json:"policy_applied,omitempty"`
	SelectedRef     string              `json:"selected_ref,omitempty"`
}

// EvidenceItem represents a single piece of evidence.
type EvidenceItem struct {
	ID              string `json:"id"`
	Type            string `json:"type"` // measurement, test, research, benchmark, audit
	Content         string `json:"content"`
	Verdict         string `json:"verdict,omitempty"` // supports, weakens, refutes
	CarrierRef      string `json:"carrier_ref,omitempty"`
	CongruenceLevel int    `json:"congruence_level,omitempty"` // 0-3
	FormalityLevel  int    `json:"formality_level,omitempty"` // 0-9
	ValidUntil      string `json:"valid_until,omitempty"`
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
	Hash string `json:"hash,omitempty"` // SHA256 at decision time
}
