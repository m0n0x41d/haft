package specflow

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/project"
)

// BaselineKind names the governance object a baseline-like record belongs to.
// It is a compatibility-layer discriminator, not a storage migration.
type BaselineKind string

const (
	BaselineKindUnknownLegacy       BaselineKind = "unknown_legacy_baseline"
	BaselineKindSpecSectionApproval BaselineKind = "spec_section_approval_baseline"
	BaselineKindPreWorkReference    BaselineKind = "pre_work_reference_snapshot"
	BaselineKindVerifiedState       BaselineKind = "verified_state_snapshot"
)

type BaselineKindProfile struct {
	Kind              BaselineKind `json:"kind"`
	Object            string       `json:"object"`
	AuthorityBoundary string       `json:"authority_boundary"`
	StorageSurface    string       `json:"storage_surface"`
	Diagnostic        string       `json:"diagnostic,omitempty"`
}

// ParseBaselineKind preserves legacy/unknown posture explicitly instead of
// guessing a concrete semantic kind from an absent or unrecognized carrier.
func ParseBaselineKind(raw string) BaselineKind {
	kind := BaselineKind(strings.TrimSpace(raw))
	switch kind {
	case BaselineKindSpecSectionApproval:
		return BaselineKindSpecSectionApproval
	case BaselineKindPreWorkReference:
		return BaselineKindPreWorkReference
	case BaselineKindVerifiedState:
		return BaselineKindVerifiedState
	case BaselineKindUnknownLegacy:
		return BaselineKindUnknownLegacy
	default:
		return BaselineKindUnknownLegacy
	}
}

func DescribeBaselineKind(kind BaselineKind) BaselineKindProfile {
	parsed := ParseBaselineKind(string(kind))
	switch parsed {
	case BaselineKindSpecSectionApproval:
		return BaselineKindProfile{
			Kind:              parsed,
			Object:            "SpecSectionApprovalBaseline",
			AuthorityBoundary: "spec_lifecycle_approval_baseline",
			StorageSurface:    "spec_section_baselines",
			Diagnostic:        "approved carrier hash for an active SpecSection",
		}
	case BaselineKindPreWorkReference:
		return BaselineKindProfile{
			Kind:              parsed,
			Object:            "PreWorkReferenceSnapshot",
			AuthorityBoundary: "work_reference_only",
			StorageSurface:    "work/evidence carrier, not spec_section_baselines",
			Diagnostic:        "planned or reference state before work; not a spec approval baseline",
		}
	case BaselineKindVerifiedState:
		return BaselineKindProfile{
			Kind:              parsed,
			Object:            "VerifiedStateSnapshot",
			AuthorityBoundary: "evidence_measurement_only",
			StorageSurface:    "evidence/measurement carrier, not spec_section_baselines",
			Diagnostic:        "observed post-work state; does not rewrite a plan or approve a SpecSection",
		}
	default:
		return BaselineKindProfile{
			Kind:              BaselineKindUnknownLegacy,
			Object:            "UnknownLegacyBaseline",
			AuthorityBoundary: "unknown_legacy_do_not_strengthen",
			StorageSurface:    "legacy carrier",
			Diagnostic:        "legacy baseline-like record has no declared semantic kind",
		}
	}
}

// SectionBaseline is the recorded canonical hash of an active SpecSection
// at the moment of approval. Drift detection compares the current carrier
// hash against this baseline; mismatch surfaces a typed finding so the
// operator can triage as valid evolution, error, or section-reopen.
type SectionBaseline struct {
	Kind       BaselineKind
	ProjectID  string
	SectionID  string
	Hash       string
	CapturedAt time.Time
	ApprovedBy string
}

// SpecSectionApprovalBaseline is the typed baseline that belongs to the spec
// lifecycle approval gate. It is the only snapshot kind this package may write
// to spec_section_baselines.
type SpecSectionApprovalBaseline struct {
	ProjectID  string
	SectionID  string
	Hash       string
	CapturedAt time.Time
	ApprovedBy string
}

// PreWorkReferenceSnapshot is a planned/reference snapshot cited by work before
// execution. It is not a spec approval baseline and must not be written to
// spec_section_baselines.
type PreWorkReferenceSnapshot struct {
	WorkRef    string
	TargetRef  string
	Hash       string
	CapturedAt time.Time
	CarrierRef string
}

// VerifiedStateSnapshot is an observed post-work state. It is evidence or
// measurement input, not a rewrite of the plan or a spec approval baseline.
type VerifiedStateSnapshot struct {
	EvidenceRef string
	TargetRef   string
	Hash        string
	CapturedAt  time.Time
	CarrierRef  string
}

func NewSpecSectionApprovalBaseline(projectID string, section project.SpecSection, approvedBy string, capturedAt time.Time) SpecSectionApprovalBaseline {
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}
	return SpecSectionApprovalBaseline{
		ProjectID:  strings.TrimSpace(projectID),
		SectionID:  strings.TrimSpace(section.ID),
		Hash:       HashSection(section),
		CapturedAt: capturedAt,
		ApprovedBy: strings.TrimSpace(approvedBy),
	}
}

func (baseline SpecSectionApprovalBaseline) SectionBaseline() SectionBaseline {
	return SectionBaseline{
		Kind:       BaselineKindSpecSectionApproval,
		ProjectID:  baseline.ProjectID,
		SectionID:  baseline.SectionID,
		Hash:       baseline.Hash,
		CapturedAt: baseline.CapturedAt,
		ApprovedBy: baseline.ApprovedBy,
	}
}

func (baseline SectionBaseline) SpecSectionApprovalBaseline() (SpecSectionApprovalBaseline, error) {
	normalized, err := normalizeSpecSectionApprovalBaseline(baseline)
	if err != nil {
		return SpecSectionApprovalBaseline{}, err
	}
	return SpecSectionApprovalBaseline{
		ProjectID:  normalized.ProjectID,
		SectionID:  normalized.SectionID,
		Hash:       normalized.Hash,
		CapturedAt: normalized.CapturedAt,
		ApprovedBy: normalized.ApprovedBy,
	}, nil
}

func normalizeStoredSectionBaseline(baseline SectionBaseline) SectionBaseline {
	baseline.Kind = BaselineKindSpecSectionApproval
	return baseline
}

func normalizeSpecSectionApprovalBaseline(baseline SectionBaseline) (SectionBaseline, error) {
	kind := ParseBaselineKind(string(baseline.Kind))
	if strings.TrimSpace(string(baseline.Kind)) != "" && kind != BaselineKindSpecSectionApproval {
		return SectionBaseline{}, fmt.Errorf("spec section baseline store accepts only %s, got %s", BaselineKindSpecSectionApproval, baseline.Kind)
	}

	baseline.Kind = BaselineKindSpecSectionApproval
	return baseline, nil
}

// ErrBaselineNotFound is returned by stores when no baseline exists for a
// (project_id, section_id) pair. Surfaces use this to distinguish "needs
// baseline" from "drifted" — both are blocking but require different
// operator next-actions.
var ErrBaselineNotFound = errors.New("spec section baseline not found")

// BaselineStore is the storage contract for SpecSection baselines. The
// SQLite implementation lives in this package; tests can substitute an
// in-memory implementation via the same interface.
type BaselineStore interface {
	Get(projectID, sectionID string) (SectionBaseline, error)
	Put(baseline SectionBaseline) error
	PutSpecSectionApproval(baseline SpecSectionApprovalBaseline) error
	Delete(projectID, sectionID string) error
	ListForProject(projectID string) ([]SectionBaseline, error)
}

// HashSection computes the canonical SHA-256 hex digest of a SpecSection.
// Hashing is deterministic over the load-bearing fields the YAML carrier
// declares; line numbers, file paths, and parser-internal flags are
// excluded so cosmetic edits to surrounding prose do not flip the hash.
//
// Drift detection trusts this function to be stable across releases; any
// change to the canonical form must bump the baseline schema (drop and
// re-baseline) or stay backwards-compatible.
func HashSection(section project.SpecSection) string {
	canonical := canonicalSectionString(section)
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func canonicalSectionString(section project.SpecSection) string {
	var b strings.Builder

	writeField(&b, "id", section.ID)
	writeField(&b, "spec", section.Spec)
	writeField(&b, "kind", section.Kind)
	writeField(&b, "title", section.Title)
	writeField(&b, "statement_type", section.StatementType)
	writeField(&b, "claim_layer", section.ClaimLayer)
	writeField(&b, "owner", section.Owner)
	writeField(&b, "status", section.Status)
	writeField(&b, "valid_until", section.ValidUntil)
	writeField(&b, "document_kind", section.DocumentKind)
	writeListField(&b, "terms", section.Terms)
	writeListField(&b, "depends_on", section.DependsOn)
	writeListField(&b, "target_refs", section.TargetRefs)

	for index, requirement := range section.EvidenceRequired {
		writeField(&b, fmt.Sprintf("evidence_required[%d].kind", index), requirement.Kind)
		writeField(&b, fmt.Sprintf("evidence_required[%d].description", index), requirement.Description)
	}

	return b.String()
}

func writeField(b *strings.Builder, key, value string) {
	b.WriteString(key)
	b.WriteString("=")
	b.WriteString(strings.TrimSpace(value))
	b.WriteString("\n")
}

func writeListField(b *strings.Builder, key string, values []string) {
	b.WriteString(key)
	b.WriteString("=")
	for index, raw := range values {
		if index > 0 {
			b.WriteString(",")
		}
		b.WriteString(strings.TrimSpace(raw))
	}
	b.WriteString("\n")
}

// SQLiteBaselineStore persists SectionBaseline rows in the project's
// SQLite database. The schema lives in db/migrations.go (version 28).
type SQLiteBaselineStore struct {
	db *sql.DB
}

func NewSQLiteBaselineStore(database *sql.DB) *SQLiteBaselineStore {
	return &SQLiteBaselineStore{db: database}
}

func (s *SQLiteBaselineStore) Get(projectID, sectionID string) (SectionBaseline, error) {
	row := s.db.QueryRow(
		`SELECT project_id, section_id, hash, captured_at, approved_by
		   FROM spec_section_baselines
		  WHERE project_id = ? AND section_id = ?`,
		projectID,
		sectionID,
	)

	var baseline SectionBaseline
	var captured time.Time
	if err := row.Scan(&baseline.ProjectID, &baseline.SectionID, &baseline.Hash, &captured, &baseline.ApprovedBy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SectionBaseline{}, ErrBaselineNotFound
		}
		return SectionBaseline{}, fmt.Errorf("read spec section baseline: %w", err)
	}

	baseline = normalizeStoredSectionBaseline(baseline)
	baseline.CapturedAt = captured
	return baseline, nil
}

func (s *SQLiteBaselineStore) Put(baseline SectionBaseline) error {
	baseline, err := normalizeSpecSectionApprovalBaseline(baseline)
	if err != nil {
		return err
	}
	if strings.TrimSpace(baseline.ProjectID) == "" {
		return fmt.Errorf("project_id is required")
	}
	if strings.TrimSpace(baseline.SectionID) == "" {
		return fmt.Errorf("section_id is required")
	}
	if strings.TrimSpace(baseline.Hash) == "" {
		return fmt.Errorf("hash is required")
	}

	captured := baseline.CapturedAt
	if captured.IsZero() {
		captured = time.Now().UTC()
	}

	_, err = s.db.Exec(
		`INSERT INTO spec_section_baselines (project_id, section_id, hash, captured_at, approved_by)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(project_id, section_id) DO UPDATE SET
		   hash = excluded.hash,
		   captured_at = excluded.captured_at,
		   approved_by = excluded.approved_by`,
		baseline.ProjectID,
		baseline.SectionID,
		baseline.Hash,
		captured,
		baseline.ApprovedBy,
	)
	if err != nil {
		return fmt.Errorf("write spec section baseline: %w", err)
	}

	return nil
}

func (s *SQLiteBaselineStore) PutSpecSectionApproval(baseline SpecSectionApprovalBaseline) error {
	return s.Put(baseline.SectionBaseline())
}

func (s *SQLiteBaselineStore) Delete(projectID, sectionID string) error {
	_, err := s.db.Exec(
		`DELETE FROM spec_section_baselines WHERE project_id = ? AND section_id = ?`,
		projectID,
		sectionID,
	)
	if err != nil {
		return fmt.Errorf("delete spec section baseline: %w", err)
	}

	return nil
}

func (s *SQLiteBaselineStore) ListForProject(projectID string) ([]SectionBaseline, error) {
	rows, err := s.db.Query(
		`SELECT project_id, section_id, hash, captured_at, approved_by
		   FROM spec_section_baselines
		  WHERE project_id = ?
		  ORDER BY section_id`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list spec section baselines: %w", err)
	}
	defer rows.Close()

	var baselines []SectionBaseline
	for rows.Next() {
		var baseline SectionBaseline
		var captured time.Time
		if err := rows.Scan(&baseline.ProjectID, &baseline.SectionID, &baseline.Hash, &captured, &baseline.ApprovedBy); err != nil {
			return nil, fmt.Errorf("scan spec section baseline: %w", err)
		}
		baseline = normalizeStoredSectionBaseline(baseline)
		baseline.CapturedAt = captured
		baselines = append(baselines, baseline)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate spec section baselines: %w", err)
	}

	return baselines, nil
}

// MemoryBaselineStore is a pure in-memory implementation used by tests
// that do not need a real SQLite connection.
type MemoryBaselineStore struct {
	rows map[string]SectionBaseline
}

func NewMemoryBaselineStore() *MemoryBaselineStore {
	return &MemoryBaselineStore{rows: make(map[string]SectionBaseline)}
}

func memoryBaselineKey(projectID, sectionID string) string {
	return projectID + "\x00" + sectionID
}

func (s *MemoryBaselineStore) Get(projectID, sectionID string) (SectionBaseline, error) {
	baseline, ok := s.rows[memoryBaselineKey(projectID, sectionID)]
	if !ok {
		return SectionBaseline{}, ErrBaselineNotFound
	}
	return normalizeStoredSectionBaseline(baseline), nil
}

func (s *MemoryBaselineStore) Put(baseline SectionBaseline) error {
	baseline, err := normalizeSpecSectionApprovalBaseline(baseline)
	if err != nil {
		return err
	}
	if strings.TrimSpace(baseline.ProjectID) == "" {
		return fmt.Errorf("project_id is required")
	}
	if strings.TrimSpace(baseline.SectionID) == "" {
		return fmt.Errorf("section_id is required")
	}
	if strings.TrimSpace(baseline.Hash) == "" {
		return fmt.Errorf("hash is required")
	}

	captured := baseline.CapturedAt
	if captured.IsZero() {
		captured = time.Now().UTC()
	}

	s.rows[memoryBaselineKey(baseline.ProjectID, baseline.SectionID)] = SectionBaseline{
		Kind:       baseline.Kind,
		ProjectID:  baseline.ProjectID,
		SectionID:  baseline.SectionID,
		Hash:       baseline.Hash,
		CapturedAt: captured,
		ApprovedBy: baseline.ApprovedBy,
	}
	return nil
}

func (s *MemoryBaselineStore) PutSpecSectionApproval(baseline SpecSectionApprovalBaseline) error {
	return s.Put(baseline.SectionBaseline())
}

func (s *MemoryBaselineStore) Delete(projectID, sectionID string) error {
	delete(s.rows, memoryBaselineKey(projectID, sectionID))
	return nil
}

func (s *MemoryBaselineStore) ListForProject(projectID string) ([]SectionBaseline, error) {
	var baselines []SectionBaseline
	for _, baseline := range s.rows {
		if baseline.ProjectID == projectID {
			baselines = append(baselines, normalizeStoredSectionBaseline(baseline))
		}
	}
	return baselines, nil
}
