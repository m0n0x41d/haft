package specflow

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/project"
)

var (
	ErrSpecSectionEditionNotFound             = errors.New("spec section edition not found")
	ErrSpecSectionEditionSemanticHashMismatch = errors.New("spec section edition semantic_hash mismatch")
)

type SpecSectionSourceKind string

const (
	SpecSectionSourceCarrierImport SpecSectionSourceKind = "carrier_import"
	SpecSectionSourceSyncBack      SpecSectionSourceKind = "markdown_sync_back"
	SpecSectionSourceSQL           SpecSectionSourceKind = "sql_source_of_truth"
)

type SpecSectionEdition struct {
	ProjectID    string                `json:"project_id"`
	SectionID    string                `json:"section_id"`
	SemanticHash string                `json:"semantic_hash"`
	Section      project.SpecSection   `json:"section"`
	SourceKind   SpecSectionSourceKind `json:"source_kind"`
	CarrierPath  string                `json:"carrier_path,omitempty"`
	UpdatedAt    time.Time             `json:"updated_at"`
}

type SpecSectionEditionHashMismatch struct {
	ProjectID    string `json:"project_id"`
	SectionID    string `json:"section_id"`
	StoredHash   string `json:"stored_hash"`
	ComputedHash string `json:"computed_hash"`
	SourceKind   string `json:"source_kind,omitempty"`
	CarrierPath  string `json:"carrier_path,omitempty"`
}

func (err *SpecSectionEditionHashMismatch) Error() string {
	if err == nil {
		return ErrSpecSectionEditionSemanticHashMismatch.Error()
	}
	return fmt.Sprintf(
		"spec section %q semantic_hash mismatch: stored %s, computed %s",
		err.SectionID,
		err.StoredHash,
		err.ComputedHash,
	)
}

func (err *SpecSectionEditionHashMismatch) Is(target error) bool {
	return target == ErrSpecSectionEditionSemanticHashMismatch
}

type SpecSectionEditionRepairPlan struct {
	ProjectID   string                           `json:"project_id"`
	Mismatches  []SpecSectionEditionHashMismatch `json:"mismatches"`
	Repaired    []SpecSectionEditionHashMismatch `json:"repaired,omitempty"`
	RepairScope string                           `json:"repair_scope"`
}

type SpecSectionEditionStore interface {
	PutCurrent(section SpecSectionEdition) error
	GetCurrent(projectID string, sectionID string) (SpecSectionEdition, error)
	ListCurrent(projectID string) ([]SpecSectionEdition, error)
	DeleteCurrent(projectID string, sectionID string) error
	ListSemanticHashMismatches(projectID string) (SpecSectionEditionRepairPlan, error)
	RepairSemanticHashMismatches(projectID string) (SpecSectionEditionRepairPlan, error)
}

func ProjectSpecificationSetFromEditions(editions []SpecSectionEdition) (project.ProjectSpecificationSet, error) {
	documents, err := specDocumentInputsFromEditions(editions)
	if err != nil {
		return project.ProjectSpecificationSet{}, err
	}
	return project.ProjectSpecificationSetFromDocuments(documents), nil
}

// ProjectSpecificationSetFromEditionsForScope applies the same central
// applicability projection used by carrier reads before parsing SQL editions.
// SQL-first is a source-selection rule; it does not bypass profile
// applicability.
func ProjectSpecificationSetFromEditionsForScope(
	editions []SpecSectionEdition,
	applicability project.ProjectSpecificationSetApplicability,
) (project.ProjectSpecificationSet, error) {
	documents, err := specDocumentInputsFromEditions(editions)
	if err != nil {
		return project.ProjectSpecificationSet{}, err
	}
	return project.ProjectSpecificationSetFromDocumentsForScope(
		documents,
		applicability,
	)
}

func specDocumentInputsFromEditions(
	editions []SpecSectionEdition,
) ([]project.SpecDocumentInput, error) {
	documents := make([]project.SpecDocumentInput, 0, len(editions))
	for _, edition := range editions {
		publication, err := RenderSpecSectionEditionMarkdown(edition)
		if err != nil {
			return nil, err
		}

		documents = append(documents, project.SpecDocumentInput{
			Path:    publication.CarrierPath,
			Kind:    specSectionEditionDocumentKind(edition.Section),
			Content: publication.Markdown,
		})
	}

	return documents, nil
}

func specSectionEditionDocumentKind(section project.SpecSection) string {
	if strings.TrimSpace(section.DocumentKind) != "" {
		return strings.TrimSpace(section.DocumentKind)
	}
	return strings.TrimSpace(section.Spec)
}

func NewSpecSectionEdition(projectID string, section project.SpecSection, sourceKind SpecSectionSourceKind, updatedAt time.Time) SpecSectionEdition {
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	sectionID := strings.TrimSpace(section.ID)
	return SpecSectionEdition{
		ProjectID:    strings.TrimSpace(projectID),
		SectionID:    sectionID,
		SemanticHash: HashSection(section),
		Section:      section,
		SourceKind:   normalizeSpecSectionSourceKind(sourceKind),
		CarrierPath:  strings.TrimSpace(section.Path),
		UpdatedAt:    updatedAt,
	}
}

type SQLiteSpecSectionEditionStore struct {
	db *sql.DB
}

func NewSQLiteSpecSectionEditionStore(database *sql.DB) *SQLiteSpecSectionEditionStore {
	return &SQLiteSpecSectionEditionStore{db: database}
}

func (s *SQLiteSpecSectionEditionStore) PutCurrent(edition SpecSectionEdition) error {
	normalized, err := normalizeSpecSectionEdition(edition)
	if err != nil {
		return err
	}

	sectionJSON, err := json.Marshal(normalized.Section)
	if err != nil {
		return fmt.Errorf("marshal spec section edition: %w", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO spec_section_editions
		   (project_id, section_id, semantic_hash, section_json, source_kind, carrier_path, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(project_id, section_id) DO UPDATE SET
		   semantic_hash = excluded.semantic_hash,
		   section_json = excluded.section_json,
		   source_kind = excluded.source_kind,
		   carrier_path = excluded.carrier_path,
		   updated_at = excluded.updated_at`,
		normalized.ProjectID,
		normalized.SectionID,
		normalized.SemanticHash,
		string(sectionJSON),
		string(normalized.SourceKind),
		normalized.CarrierPath,
		normalized.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("write spec section edition: %w", err)
	}

	return nil
}

func (s *SQLiteSpecSectionEditionStore) GetCurrent(projectID string, sectionID string) (SpecSectionEdition, error) {
	row := s.db.QueryRow(
		`SELECT project_id, section_id, semantic_hash, section_json, source_kind, carrier_path, updated_at
		   FROM spec_section_editions
		  WHERE project_id = ? AND section_id = ?`,
		strings.TrimSpace(projectID),
		strings.TrimSpace(sectionID),
	)

	edition, err := scanSpecSectionEdition(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SpecSectionEdition{}, ErrSpecSectionEditionNotFound
		}
		return SpecSectionEdition{}, fmt.Errorf("read spec section edition: %w", err)
	}
	return edition, nil
}

func (s *SQLiteSpecSectionEditionStore) ListCurrent(projectID string) ([]SpecSectionEdition, error) {
	rows, err := s.db.Query(
		`SELECT project_id, section_id, semantic_hash, section_json, source_kind, carrier_path, updated_at
		   FROM spec_section_editions
		  WHERE project_id = ?
		  ORDER BY section_id`,
		strings.TrimSpace(projectID),
	)
	if err != nil {
		return nil, fmt.Errorf("list spec section editions: %w", err)
	}
	defer rows.Close()

	var editions []SpecSectionEdition
	for rows.Next() {
		edition, err := scanSpecSectionEdition(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan spec section edition: %w", err)
		}
		editions = append(editions, edition)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate spec section editions: %w", err)
	}
	return editions, nil
}

func (s *SQLiteSpecSectionEditionStore) DeleteCurrent(projectID string, sectionID string) error {
	_, err := s.db.Exec(
		`DELETE FROM spec_section_editions
		  WHERE project_id = ? AND section_id = ?`,
		strings.TrimSpace(projectID),
		strings.TrimSpace(sectionID),
	)
	if err != nil {
		return fmt.Errorf("delete spec section edition: %w", err)
	}
	return nil
}

func (s *SQLiteSpecSectionEditionStore) ListSemanticHashMismatches(projectID string) (SpecSectionEditionRepairPlan, error) {
	normalizedProjectID := strings.TrimSpace(projectID)
	plan := SpecSectionEditionRepairPlan{
		ProjectID:   normalizedProjectID,
		Mismatches:  []SpecSectionEditionHashMismatch{},
		RepairScope: "semantic_hash_cache_only",
	}

	rows, err := s.db.Query(
		`SELECT project_id, section_id, semantic_hash, section_json, source_kind, carrier_path, updated_at
		   FROM spec_section_editions
		  WHERE project_id = ?
		  ORDER BY section_id`,
		normalizedProjectID,
	)
	if err != nil {
		return SpecSectionEditionRepairPlan{}, fmt.Errorf("list spec section editions for hash repair: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		edition, err := scanSpecSectionEditionUnchecked(rows.Scan)
		if err != nil {
			return SpecSectionEditionRepairPlan{}, fmt.Errorf("scan spec section edition for hash repair: %w", err)
		}
		mismatch := specSectionEditionHashMismatch(edition)
		if mismatch == nil {
			continue
		}
		plan.Mismatches = append(plan.Mismatches, *mismatch)
	}
	if err := rows.Err(); err != nil {
		return SpecSectionEditionRepairPlan{}, fmt.Errorf("iterate spec section editions for hash repair: %w", err)
	}

	return plan, nil
}

func (s *SQLiteSpecSectionEditionStore) RepairSemanticHashMismatches(projectID string) (SpecSectionEditionRepairPlan, error) {
	plan, err := s.ListSemanticHashMismatches(projectID)
	if err != nil {
		return SpecSectionEditionRepairPlan{}, err
	}
	plan.Repaired = []SpecSectionEditionHashMismatch{}

	for _, mismatch := range plan.Mismatches {
		result, err := s.db.Exec(
			`UPDATE spec_section_editions
			    SET semantic_hash = ?
			  WHERE project_id = ?
			    AND section_id = ?
			    AND semantic_hash = ?`,
			mismatch.ComputedHash,
			mismatch.ProjectID,
			mismatch.SectionID,
			mismatch.StoredHash,
		)
		if err != nil {
			return SpecSectionEditionRepairPlan{}, fmt.Errorf("repair spec section edition semantic_hash for %s: %w", mismatch.SectionID, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return SpecSectionEditionRepairPlan{}, fmt.Errorf("read semantic_hash repair result for %s: %w", mismatch.SectionID, err)
		}
		if affected == 0 {
			return SpecSectionEditionRepairPlan{}, fmt.Errorf("repair spec section edition semantic_hash for %s: row changed before repair", mismatch.SectionID)
		}
		plan.Repaired = append(plan.Repaired, mismatch)
	}

	return plan, nil
}

func scanSpecSectionEdition(scan func(dest ...any) error) (SpecSectionEdition, error) {
	edition, err := scanSpecSectionEditionUnchecked(scan)
	if err != nil {
		return SpecSectionEdition{}, err
	}
	return normalizeSpecSectionEdition(edition)
}

func scanSpecSectionEditionUnchecked(scan func(dest ...any) error) (SpecSectionEdition, error) {
	var edition SpecSectionEdition
	var sectionJSON string
	var sourceKind string
	var updatedAt time.Time
	if err := scan(
		&edition.ProjectID,
		&edition.SectionID,
		&edition.SemanticHash,
		&sectionJSON,
		&sourceKind,
		&edition.CarrierPath,
		&updatedAt,
	); err != nil {
		return SpecSectionEdition{}, err
	}
	if err := json.Unmarshal([]byte(sectionJSON), &edition.Section); err != nil {
		return SpecSectionEdition{}, fmt.Errorf("unmarshal spec section edition: %w", err)
	}
	edition.SourceKind = normalizeSpecSectionSourceKind(SpecSectionSourceKind(sourceKind))
	edition.UpdatedAt = updatedAt
	return edition, nil
}

func normalizeSpecSectionEdition(edition SpecSectionEdition) (SpecSectionEdition, error) {
	edition.ProjectID = strings.TrimSpace(edition.ProjectID)
	edition.SectionID = strings.TrimSpace(edition.SectionID)
	edition.Section.ID = strings.TrimSpace(edition.Section.ID)
	edition.SemanticHash = strings.TrimSpace(edition.SemanticHash)
	edition.SourceKind = normalizeSpecSectionSourceKind(edition.SourceKind)
	edition.CarrierPath = strings.TrimSpace(edition.CarrierPath)
	if edition.UpdatedAt.IsZero() {
		edition.UpdatedAt = time.Now().UTC()
	}

	if edition.ProjectID == "" {
		return SpecSectionEdition{}, fmt.Errorf("project_id is required")
	}
	if edition.SectionID == "" {
		edition.SectionID = edition.Section.ID
	}
	if edition.SectionID == "" {
		return SpecSectionEdition{}, fmt.Errorf("section_id is required")
	}
	if edition.Section.ID == "" {
		edition.Section.ID = edition.SectionID
	}
	if edition.Section.ID != edition.SectionID {
		return SpecSectionEdition{}, fmt.Errorf("section_id %q does not match section.id %q", edition.SectionID, edition.Section.ID)
	}
	computedHash := HashSection(edition.Section)
	if edition.SemanticHash == "" {
		edition.SemanticHash = computedHash
		return edition, nil
	}
	if edition.SemanticHash != computedHash {
		return SpecSectionEdition{}, specSectionEditionHashMismatch(edition)
	}

	return edition, nil
}

func specSectionEditionHashMismatch(edition SpecSectionEdition) *SpecSectionEditionHashMismatch {
	sectionID := strings.TrimSpace(edition.SectionID)
	if sectionID == "" {
		sectionID = strings.TrimSpace(edition.Section.ID)
	}
	storedHash := strings.TrimSpace(edition.SemanticHash)
	computedHash := HashSection(edition.Section)
	if storedHash == computedHash {
		return nil
	}
	return &SpecSectionEditionHashMismatch{
		ProjectID:    strings.TrimSpace(edition.ProjectID),
		SectionID:    sectionID,
		StoredHash:   storedHash,
		ComputedHash: computedHash,
		SourceKind:   string(normalizeSpecSectionSourceKind(edition.SourceKind)),
		CarrierPath:  strings.TrimSpace(edition.CarrierPath),
	}
}

func normalizeSpecSectionSourceKind(kind SpecSectionSourceKind) SpecSectionSourceKind {
	switch kind {
	case SpecSectionSourceCarrierImport:
		return SpecSectionSourceCarrierImport
	case SpecSectionSourceSyncBack:
		return SpecSectionSourceSyncBack
	case SpecSectionSourceSQL:
		return SpecSectionSourceSQL
	default:
		return SpecSectionSourceCarrierImport
	}
}
