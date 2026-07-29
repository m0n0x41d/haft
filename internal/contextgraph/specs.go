package contextgraph

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

type SpecResolution string

const (
	SpecResolutionResolved    SpecResolution = "resolved"
	SpecResolutionMissing     SpecResolution = "missing"
	SpecResolutionAmbiguous   SpecResolution = "ambiguous"
	SpecResolutionUnavailable SpecResolution = "unavailable"
	SpecResolutionCorrupt     SpecResolution = "corrupt"
)

type SpecBaselineState string

const (
	SpecBaselineCurrent     SpecBaselineState = "current"
	SpecBaselineDrifted     SpecBaselineState = "drifted"
	SpecBaselineMissing     SpecBaselineState = "missing"
	SpecBaselineUnavailable SpecBaselineState = "unavailable"
)

// SpecClaimContext is the compact typed claim projection needed while reading
// code. It preserves authority-bearing identifiers and freshness without
// embedding the whole ProjectSpecificationSet in every graph response.
type SpecClaimContext struct {
	ID         string
	Class      string
	Statement  string
	ValidUntil string
}

// SpecSectionContext is a read-only join from a governing DecisionRecord's
// section_refs to the current SQL-backed SpecSection edition and its approval
// baseline. It is governance context, never a code traversal edge or approval.
type SpecSectionContext struct {
	ID                 string
	Title              string
	Spec               string
	Kind               string
	StatementType      string
	ClaimLayer         string
	Owner              string
	Status             string
	LifecycleState     project.SpecSectionState
	ValidUntil         string
	DocumentKind       string
	Path               string
	Claims             []SpecClaimContext
	DecisionRefs       []string
	Resolution         SpecResolution
	ResolutionDetail   string
	ProjectID          string
	SemanticHash       string
	SourceKind         string
	CarrierPath        string
	EditionUpdatedAt   string
	BaselineState      SpecBaselineState
	BaselineDetail     string
	BaselineHash       string
	BaselineCapturedAt string
	BaselineApprovedBy string
}

type specRefBinding struct {
	id           string
	decisionRefs []string
}

type storedSpecEdition struct {
	projectID    string
	sectionID    string
	semanticHash string
	sectionJSON  string
	sourceKind   string
	carrierPath  string
	updatedAt    string
}

type storedSpecBaseline struct {
	projectID  string
	sectionID  string
	hash       string
	capturedAt string
	approvedBy string
}

func fetchGoverningSpecSections(
	ctx context.Context,
	database *sql.DB,
	decisions []*artifact.Artifact,
	now time.Time,
) []SpecSectionContext {
	bindings := specRefBindings(decisions)
	if len(bindings) == 0 {
		return nil
	}
	if database == nil {
		return unavailableSpecSections(bindings, "project database is unavailable")
	}

	tx, err := database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		detail := fmt.Sprintf("begin SpecSection read snapshot: %v", err)
		return unavailableSpecSections(bindings, detail)
	}
	defer func() { _ = tx.Rollback() }()

	sectionIDs := specBindingIDs(bindings)
	editions, err := loadSpecEditions(ctx, tx, sectionIDs)
	if err != nil {
		return unavailableSpecSections(bindings, err.Error())
	}
	baselines, baselineErr := loadSpecBaselines(ctx, tx, sectionIDs)
	if baselineErr == nil {
		if err := tx.Commit(); err != nil {
			detail := fmt.Sprintf("commit SpecSection read snapshot: %v", err)
			return unavailableSpecSections(bindings, detail)
		}
	}

	return buildSpecSectionContexts(bindings, editions, baselines, baselineErr, now)
}

func specRefBindings(decisions []*artifact.Artifact) []specRefBinding {
	bindings := make([]specRefBinding, 0)
	positions := map[string]int{}
	for _, decision := range decisions {
		if decision == nil {
			continue
		}
		fields := decision.UnmarshalDecisionFields()
		for _, rawRef := range fields.SectionRefs {
			ref := strings.TrimSpace(rawRef)
			if ref == "" {
				continue
			}
			position, exists := positions[ref]
			if !exists {
				position = len(bindings)
				positions[ref] = position
				bindings = append(bindings, specRefBinding{id: ref})
			}
			bindings[position].decisionRefs = appendUnique(bindings[position].decisionRefs, decision.Meta.ID)
		}
	}
	return bindings
}

func specBindingIDs(bindings []specRefBinding) []string {
	ids := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		ids = append(ids, binding.id)
	}
	return ids
}

func loadSpecEditions(
	ctx context.Context,
	tx *sql.Tx,
	sectionIDs []string,
) (map[string][]storedSpecEdition, error) {
	// #nosec G202 -- the only interpolated fragment is a count-derived list of SQL placeholders; section IDs remain bound parameters.
	query := `SELECT project_id, section_id, semantic_hash, section_json, source_kind, carrier_path, CAST(updated_at AS TEXT)
		FROM spec_section_editions
		WHERE section_id IN (` + sqlPlaceholders(len(sectionIDs)) + `)
		ORDER BY section_id, project_id`
	rows, err := tx.QueryContext(ctx, query, stringArgs(sectionIDs)...)
	if err != nil {
		return nil, fmt.Errorf("read current SpecSection editions: %w", err)
	}
	defer rows.Close()

	editions := make(map[string][]storedSpecEdition, len(sectionIDs))
	for rows.Next() {
		var edition storedSpecEdition
		err := rows.Scan(
			&edition.projectID,
			&edition.sectionID,
			&edition.semanticHash,
			&edition.sectionJSON,
			&edition.sourceKind,
			&edition.carrierPath,
			&edition.updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan current SpecSection edition: %w", err)
		}
		editions[edition.sectionID] = append(editions[edition.sectionID], edition)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate current SpecSection editions: %w", err)
	}
	return editions, nil
}

func loadSpecBaselines(
	ctx context.Context,
	tx *sql.Tx,
	sectionIDs []string,
) (map[string]storedSpecBaseline, error) {
	// #nosec G202 -- the only interpolated fragment is a count-derived list of SQL placeholders; section IDs remain bound parameters.
	query := `SELECT project_id, section_id, hash, CAST(captured_at AS TEXT), approved_by
		FROM spec_section_baselines
		WHERE section_id IN (` + sqlPlaceholders(len(sectionIDs)) + `)
		ORDER BY section_id, project_id`
	rows, err := tx.QueryContext(ctx, query, stringArgs(sectionIDs)...)
	if err != nil {
		return nil, fmt.Errorf("read SpecSection approval baselines: %w", err)
	}
	defer rows.Close()

	baselines := make(map[string]storedSpecBaseline, len(sectionIDs))
	for rows.Next() {
		var baseline storedSpecBaseline
		err := rows.Scan(
			&baseline.projectID,
			&baseline.sectionID,
			&baseline.hash,
			&baseline.capturedAt,
			&baseline.approvedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("scan SpecSection approval baseline: %w", err)
		}
		baselines[specBaselineKey(baseline.projectID, baseline.sectionID)] = baseline
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate SpecSection approval baselines: %w", err)
	}
	return baselines, nil
}

func buildSpecSectionContexts(
	bindings []specRefBinding,
	editions map[string][]storedSpecEdition,
	baselines map[string]storedSpecBaseline,
	baselineErr error,
	now time.Time,
) []SpecSectionContext {
	sections := make([]SpecSectionContext, 0, len(bindings))
	for _, binding := range bindings {
		section := buildSpecSectionContext(binding, editions[binding.id], baselines, baselineErr, now)
		sections = append(sections, section)
	}
	return sections
}

func buildSpecSectionContext(
	binding specRefBinding,
	editions []storedSpecEdition,
	baselines map[string]storedSpecBaseline,
	baselineErr error,
	now time.Time,
) SpecSectionContext {
	base := SpecSectionContext{
		ID:            binding.id,
		DecisionRefs:  binding.decisionRefs,
		Resolution:    SpecResolutionMissing,
		BaselineState: SpecBaselineMissing,
	}
	if len(editions) == 0 {
		return base
	}
	if len(editions) != 1 {
		base.Resolution = SpecResolutionAmbiguous
		base.ResolutionDetail = fmt.Sprintf("%d current editions share this section id", len(editions))
		base.BaselineState = SpecBaselineUnavailable
		return base
	}

	edition := editions[0]
	section, err := decodeStoredSpecSection(edition.sectionJSON)
	if err != nil {
		return corruptSpecSection(base, err)
	}
	if strings.TrimSpace(section.ID) != binding.id {
		detail := fmt.Sprintf("edition section id %q does not match ref %q", section.ID, binding.id)
		return corruptSpecSection(base, fmt.Errorf("%s", detail))
	}
	computedHash := specflow.HashSection(section)
	if strings.TrimSpace(edition.semanticHash) != computedHash {
		detail := fmt.Sprintf(
			"stored semantic hash %s does not match computed hash %s",
			edition.semanticHash,
			computedHash,
		)
		return corruptSpecSection(base, fmt.Errorf("%s", detail))
	}

	resolved := specContextFromStored(binding, edition, section, now)
	return attachSpecBaseline(resolved, baselines, baselineErr)
}

func decodeStoredSpecSection(raw string) (project.SpecSection, error) {
	var section project.SpecSection
	if err := json.Unmarshal([]byte(raw), &section); err != nil {
		return project.SpecSection{}, fmt.Errorf("decode current SpecSection edition: %w", err)
	}
	if strings.TrimSpace(section.ID) == "" {
		return project.SpecSection{}, fmt.Errorf("current SpecSection edition has no id")
	}
	return section, nil
}

func specContextFromStored(
	binding specRefBinding,
	edition storedSpecEdition,
	section project.SpecSection,
	now time.Time,
) SpecSectionContext {
	claims := make([]SpecClaimContext, 0, len(section.Claims))
	for _, claim := range section.Claims {
		claims = append(claims, SpecClaimContext{
			ID:         claim.ID,
			Class:      claim.Class,
			Statement:  claim.Statement,
			ValidUntil: claim.ValidUntil,
		})
	}
	path := strings.TrimSpace(section.Path)
	if path == "" {
		path = strings.TrimSpace(edition.carrierPath)
	}
	return SpecSectionContext{
		ID:               section.ID,
		Title:            section.Title,
		Spec:             section.Spec,
		Kind:             section.Kind,
		StatementType:    section.StatementType,
		ClaimLayer:       section.ClaimLayer,
		Owner:            section.Owner,
		Status:           section.Status,
		LifecycleState:   section.LifecycleState(now),
		ValidUntil:       section.ValidUntil,
		DocumentKind:     section.DocumentKind,
		Path:             path,
		Claims:           claims,
		DecisionRefs:     binding.decisionRefs,
		Resolution:       SpecResolutionResolved,
		ProjectID:        edition.projectID,
		SemanticHash:     edition.semanticHash,
		SourceKind:       edition.sourceKind,
		CarrierPath:      edition.carrierPath,
		EditionUpdatedAt: edition.updatedAt,
		BaselineState:    SpecBaselineMissing,
	}
}

func attachSpecBaseline(
	section SpecSectionContext,
	baselines map[string]storedSpecBaseline,
	baselineErr error,
) SpecSectionContext {
	if baselineErr != nil {
		section.BaselineState = SpecBaselineUnavailable
		section.BaselineDetail = baselineErr.Error()
		return section
	}
	baseline, ok := baselines[specBaselineKey(section.ProjectID, section.ID)]
	if !ok {
		section.BaselineState = SpecBaselineMissing
		return section
	}
	section.BaselineHash = baseline.hash
	section.BaselineCapturedAt = baseline.capturedAt
	section.BaselineApprovedBy = baseline.approvedBy
	section.BaselineState = SpecBaselineCurrent
	if baseline.hash != section.SemanticHash {
		section.BaselineState = SpecBaselineDrifted
	}
	return section
}

func corruptSpecSection(base SpecSectionContext, err error) SpecSectionContext {
	base.Resolution = SpecResolutionCorrupt
	base.ResolutionDetail = err.Error()
	base.BaselineState = SpecBaselineUnavailable
	return base
}

func unavailableSpecSections(bindings []specRefBinding, detail string) []SpecSectionContext {
	sections := make([]SpecSectionContext, 0, len(bindings))
	for _, binding := range bindings {
		sections = append(sections, SpecSectionContext{
			ID:               binding.id,
			DecisionRefs:     binding.decisionRefs,
			Resolution:       SpecResolutionUnavailable,
			ResolutionDetail: detail,
			BaselineState:    SpecBaselineUnavailable,
		})
	}
	return sections
}

func specBaselineKey(projectID, sectionID string) string {
	return strings.TrimSpace(projectID) + "\x00" + strings.TrimSpace(sectionID)
}

func sqlPlaceholders(count int) string {
	placeholders := make([]string, count)
	for index := range placeholders {
		placeholders[index] = "?"
	}
	return strings.Join(placeholders, ",")
}

func stringArgs(values []string) []any {
	args := make([]any, 0, len(values))
	for _, value := range values {
		args = append(args, value)
	}
	return args
}

func appendUnique(values []string, candidate string) []string {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}
