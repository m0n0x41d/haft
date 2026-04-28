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

// SectionBaseline is the recorded canonical hash of an active SpecSection
// at the moment of approval. Drift detection compares the current carrier
// hash against this baseline; mismatch surfaces a typed finding so the
// operator can triage as valid evolution, error, or section-reopen.
type SectionBaseline struct {
	ProjectID  string
	SectionID  string
	Hash       string
	CapturedAt time.Time
	ApprovedBy string
}

// BaselineNotFound is returned by stores when no baseline exists for a
// (project_id, section_id) pair. Surfaces use this to distinguish "needs
// baseline" from "drifted" — both are blocking but require different
// operator next-actions.
var BaselineNotFound = errors.New("spec section baseline not found")

// BaselineStore is the storage contract for SpecSection baselines. The
// SQLite implementation lives in this package; tests can substitute an
// in-memory implementation via the same interface.
type BaselineStore interface {
	Get(projectID, sectionID string) (SectionBaseline, error)
	Put(baseline SectionBaseline) error
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
			return SectionBaseline{}, BaselineNotFound
		}
		return SectionBaseline{}, fmt.Errorf("read spec section baseline: %w", err)
	}

	baseline.CapturedAt = captured
	return baseline, nil
}

func (s *SQLiteBaselineStore) Put(baseline SectionBaseline) error {
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

	_, err := s.db.Exec(
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
		return SectionBaseline{}, BaselineNotFound
	}
	return baseline, nil
}

func (s *MemoryBaselineStore) Put(baseline SectionBaseline) error {
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
		ProjectID:  baseline.ProjectID,
		SectionID:  baseline.SectionID,
		Hash:       baseline.Hash,
		CapturedAt: captured,
		ApprovedBy: baseline.ApprovedBy,
	}
	return nil
}

func (s *MemoryBaselineStore) Delete(projectID, sectionID string) error {
	delete(s.rows, memoryBaselineKey(projectID, sectionID))
	return nil
}

func (s *MemoryBaselineStore) ListForProject(projectID string) ([]SectionBaseline, error) {
	var baselines []SectionBaseline
	for _, baseline := range s.rows {
		if baseline.ProjectID == projectID {
			baselines = append(baselines, baseline)
		}
	}
	return baselines, nil
}
