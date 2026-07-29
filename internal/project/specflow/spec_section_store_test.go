package specflow

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func TestSQLiteSpecSectionEditionStoreRoundTripWithMigration(t *testing.T) {
	dbStore := newTestBaselineDB(t)
	store := NewSQLiteSpecSectionEditionStore(dbStore.GetRawDB())
	section := specSectionEditionTestSection("TS.sync.001")
	updatedAt := time.Now().UTC().Truncate(time.Second)

	edition := NewSpecSectionEdition("proj-1", section, SpecSectionSourceSyncBack, updatedAt)
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("PutCurrent: %v", err)
	}

	got, err := store.GetCurrent("proj-1", "TS.sync.001")
	if err != nil {
		t.Fatalf("GetCurrent: %v", err)
	}
	if got.ProjectID != "proj-1" {
		t.Fatalf("project_id = %q", got.ProjectID)
	}
	if got.SectionID != "TS.sync.001" {
		t.Fatalf("section_id = %q", got.SectionID)
	}
	if got.SemanticHash != HashSection(section) {
		t.Fatalf("semantic_hash = %q, want HashSection", got.SemanticHash)
	}
	if got.Section.Title != "Sync back" {
		t.Fatalf("section title = %q", got.Section.Title)
	}
	if got.SourceKind != SpecSectionSourceSyncBack {
		t.Fatalf("source_kind = %q", got.SourceKind)
	}
	if got.CarrierPath != ".haft/specs/target-system.md" {
		t.Fatalf("carrier_path = %q", got.CarrierPath)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("updated_at should be stored")
	}
}

func TestSQLiteSpecSectionEditionStoreUpsertReplacesCurrentEdition(t *testing.T) {
	dbStore := newTestBaselineDB(t)
	store := NewSQLiteSpecSectionEditionStore(dbStore.GetRawDB())
	section := specSectionEditionTestSection("TS.sync.001")

	if err := store.PutCurrent(NewSpecSectionEdition("proj-1", section, SpecSectionSourceCarrierImport, time.Time{})); err != nil {
		t.Fatalf("PutCurrent initial: %v", err)
	}
	section.Title = "Updated sync back"
	section.DependsOn = []string{"TS.boundary.001"}
	if err := store.PutCurrent(NewSpecSectionEdition("proj-1", section, SpecSectionSourceSyncBack, time.Time{})); err != nil {
		t.Fatalf("PutCurrent updated: %v", err)
	}

	got, err := store.GetCurrent("proj-1", "TS.sync.001")
	if err != nil {
		t.Fatalf("GetCurrent: %v", err)
	}
	if got.Section.Title != "Updated sync back" {
		t.Fatalf("title = %q", got.Section.Title)
	}
	if !strings.Contains(strings.Join(got.Section.DependsOn, ","), "TS.boundary.001") {
		t.Fatalf("depends_on = %#v", got.Section.DependsOn)
	}
	if got.SourceKind != SpecSectionSourceSyncBack {
		t.Fatalf("source_kind = %q", got.SourceKind)
	}
}

func TestSQLiteSpecSectionEditionStoreListScopesByProject(t *testing.T) {
	dbStore := newTestBaselineDB(t)
	store := NewSQLiteSpecSectionEditionStore(dbStore.GetRawDB())
	_ = store.PutCurrent(NewSpecSectionEdition("p1", specSectionEditionTestSection("TS.one.001"), SpecSectionSourceCarrierImport, time.Time{}))
	_ = store.PutCurrent(NewSpecSectionEdition("p1", specSectionEditionTestSection("TS.two.001"), SpecSectionSourceCarrierImport, time.Time{}))
	_ = store.PutCurrent(NewSpecSectionEdition("p2", specSectionEditionTestSection("TS.other.001"), SpecSectionSourceCarrierImport, time.Time{}))

	rows, err := store.ListCurrent("p1")
	if err != nil {
		t.Fatalf("ListCurrent: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0].SectionID != "TS.one.001" || rows[1].SectionID != "TS.two.001" {
		t.Fatalf("rows sorted by section_id: %#v", rows)
	}
}

func TestSQLiteSpecSectionEditionStoreReportsNotFound(t *testing.T) {
	dbStore := newTestBaselineDB(t)
	store := NewSQLiteSpecSectionEditionStore(dbStore.GetRawDB())

	_, err := store.GetCurrent("proj-1", "missing")
	if !errors.Is(err, ErrSpecSectionEditionNotFound) {
		t.Fatalf("err = %v, want ErrSpecSectionEditionNotFound", err)
	}
}

func TestSpecSectionEditionRejectsMismatchedSectionID(t *testing.T) {
	store := NewSQLiteSpecSectionEditionStore(newTestBaselineDB(t).GetRawDB())
	section := specSectionEditionTestSection("TS.sync.001")
	edition := NewSpecSectionEdition("proj-1", section, SpecSectionSourceCarrierImport, time.Time{})
	edition.SectionID = "TS.other.001"

	err := store.PutCurrent(edition)
	if err == nil {
		t.Fatal("expected mismatched section id error")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSpecSectionEditionRejectsMismatchedSemanticHashOnWrite(t *testing.T) {
	store := NewSQLiteSpecSectionEditionStore(newTestBaselineDB(t).GetRawDB())
	section := specSectionEditionTestSection("TS.sync.001")
	edition := NewSpecSectionEdition("proj-1", section, SpecSectionSourceCarrierImport, time.Time{})
	edition.SemanticHash = "stale-hash"

	err := store.PutCurrent(edition)
	var mismatch *SpecSectionEditionHashMismatch
	if !errors.As(err, &mismatch) {
		t.Fatalf("err = %v, want SpecSectionEditionHashMismatch", err)
	}
	if mismatch.StoredHash != "stale-hash" || mismatch.ComputedHash != HashSection(section) {
		t.Fatalf("mismatch = %#v", mismatch)
	}
	if !errors.Is(err, ErrSpecSectionEditionSemanticHashMismatch) {
		t.Fatalf("err = %v, want ErrSpecSectionEditionSemanticHashMismatch", err)
	}
}

func TestSQLiteSpecSectionEditionStoreDetectsSemanticHashMismatchOnRead(t *testing.T) {
	dbStore := newTestBaselineDB(t)
	store := NewSQLiteSpecSectionEditionStore(dbStore.GetRawDB())
	section := specSectionEditionTestSection("TS.sync.001")
	putRawSpecSectionEdition(t, dbStore.GetRawDB(), "proj-1", "stale-hash", section)

	_, err := store.GetCurrent("proj-1", "TS.sync.001")
	var mismatch *SpecSectionEditionHashMismatch
	if !errors.As(err, &mismatch) {
		t.Fatalf("err = %v, want SpecSectionEditionHashMismatch", err)
	}
	if mismatch.SectionID != "TS.sync.001" {
		t.Fatalf("section id = %q", mismatch.SectionID)
	}
	if mismatch.ComputedHash != HashSection(section) {
		t.Fatalf("computed hash = %q, want HashSection", mismatch.ComputedHash)
	}
}

func TestSQLiteSpecSectionEditionStoreRepairsSemanticHashMismatches(t *testing.T) {
	dbStore := newTestBaselineDB(t)
	store := NewSQLiteSpecSectionEditionStore(dbStore.GetRawDB())
	section := specSectionEditionTestSection("TS.sync.001")
	putRawSpecSectionEdition(t, dbStore.GetRawDB(), "proj-1", "stale-hash", section)

	plan, err := store.ListSemanticHashMismatches("proj-1")
	if err != nil {
		t.Fatalf("ListSemanticHashMismatches: %v", err)
	}
	if len(plan.Mismatches) != 1 {
		t.Fatalf("mismatches = %#v, want one", plan.Mismatches)
	}
	if len(plan.Repaired) != 0 {
		t.Fatalf("dry-run plan repaired = %#v, want none", plan.Repaired)
	}

	applied, err := store.RepairSemanticHashMismatches("proj-1")
	if err != nil {
		t.Fatalf("RepairSemanticHashMismatches: %v", err)
	}
	if len(applied.Repaired) != 1 {
		t.Fatalf("repaired = %#v, want one", applied.Repaired)
	}
	got, err := store.GetCurrent("proj-1", "TS.sync.001")
	if err != nil {
		t.Fatalf("GetCurrent after repair: %v", err)
	}
	if got.SemanticHash != HashSection(section) {
		t.Fatalf("semantic_hash = %q, want HashSection", got.SemanticHash)
	}
}

func TestProjectSpecificationSetFromEditionsPreservesSemanticSections(t *testing.T) {
	target := NewSpecSectionEdition("proj-1", specSectionEditionTestSection("TS.sync.001"), SpecSectionSourceSQL, time.Time{})
	enablingSection := specSectionEditionTestSection("ES.sync.001")
	enablingSection.Spec = "enabling-system"
	enablingSection.SystemFrame = project.SystemReferenceFrame{ID: "enabling_system", Kind: "enabling_system", Source: "declared"}
	enablingSection.DocumentKind = "enabling-system"
	enablingSection.Path = ".haft/specs/enabling-system.md"
	enabling := NewSpecSectionEdition("proj-1", enablingSection, SpecSectionSourceSQL, time.Time{})

	specSet, err := ProjectSpecificationSetFromEditions([]SpecSectionEdition{target, enabling})
	if err != nil {
		t.Fatalf("ProjectSpecificationSetFromEditions: %v", err)
	}
	if len(specSet.Findings) != 0 {
		t.Fatalf("findings = %#v, want none", specSet.Findings)
	}
	if len(specSet.Sections) != 2 {
		t.Fatalf("sections = %#v, want two sections", specSet.Sections)
	}
	if HashSection(specSet.Sections[0]) != target.SemanticHash {
		t.Fatalf("target semantic hash changed")
	}
	if HashSection(specSet.Sections[1]) != enabling.SemanticHash {
		t.Fatalf("enabling semantic hash changed")
	}
}

func TestProjectSpecificationSetFromEditionsForScopeFiltersBeforeParsing(
	t *testing.T,
) {
	target := NewSpecSectionEdition(
		"proj-1",
		specSectionEditionTestSection("TS.scope.001"),
		SpecSectionSourceSQL,
		time.Time{},
	)
	softwareSection := specSectionEditionTestSection("SS.scope.001")
	softwareSection.Spec = string(project.SpecDocumentKindSoftwareSystem)
	softwareSection.DocumentKind = string(project.SpecDocumentKindSoftwareSystem)
	softwareSection.Path = ".haft/specs/software-system.md"
	software := NewSpecSectionEdition(
		"proj-1",
		softwareSection,
		SpecSectionSourceSQL,
		time.Time{},
	)
	legacySection := specSectionEditionTestSection("ES.scope.001")
	legacySection.Spec = string(project.SpecDocumentKindEnablingSystem)
	legacySection.DocumentKind = string(project.SpecDocumentKindEnablingSystem)
	legacySection.Path = ".haft/specs/enabling-system.md"
	legacy := NewSpecSectionEdition(
		"proj-1",
		legacySection,
		SpecSectionSourceSQL,
		time.Time{},
	)
	applicability := mustSpecflowNonSoftwareApplicability(t)

	specSet, err := ProjectSpecificationSetFromEditionsForScope(
		[]SpecSectionEdition{target, software, legacy},
		applicability,
	)
	if err != nil {
		t.Fatalf("ProjectSpecificationSetFromEditionsForScope: %v", err)
	}
	if len(specSet.Sections) != 0 {
		t.Fatalf("non-software SQL sections = %#v, want none", specSet.Sections)
	}
	if len(specSet.Documents) != 0 {
		t.Fatalf("non-software SQL documents = %#v, want none", specSet.Documents)
	}
	if !specflowHasFinding(
		specSet.Findings,
		"profile_capability_applicability_underdetermined",
	) {
		t.Fatalf("scoped SQL projection hid target-relation uncertainty: %#v", specSet.Findings)
	}
}

func mustSpecflowNonSoftwareApplicability(
	t *testing.T,
) project.ProjectSpecificationSetApplicability {
	t.Helper()
	scopeID, err := projectprofile.NewScopeID("documents")
	if err != nil {
		t.Fatalf("NewScopeID: %v", err)
	}
	scope, err := projectprofile.NewNonSoftwareRealization(
		scopeID,
		projectprofile.NoEntityReference{},
		projectprofile.UnspecifiedKindOrientation{},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("NewNonSoftwareRealization: %v", err)
	}
	scopes, err := projectprofile.NewScopeSet(
		[]projectprofile.RealizationScope{scope},
	)
	if err != nil {
		t.Fatalf("NewScopeSet: %v", err)
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(scopes)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload: %v", err)
	}
	matrix, err := projectprofile.ResolveCapabilityApplicabilityMatrix(payload)
	if err != nil {
		t.Fatalf("ResolveCapabilityApplicabilityMatrix: %v", err)
	}
	applicability, err := project.DeriveProjectSpecificationSetApplicability(
		matrix,
		scopeID,
	)
	if err != nil {
		t.Fatalf("DeriveProjectSpecificationSetApplicability: %v", err)
	}
	return applicability
}

func specflowHasFinding(
	findings []project.SpecCheckFinding,
	code string,
) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

func putRawSpecSectionEdition(
	t *testing.T,
	database *sql.DB,
	projectID string,
	semanticHash string,
	section project.SpecSection,
) {
	t.Helper()

	sectionJSON, err := json.Marshal(section)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(
		`INSERT INTO spec_section_editions
		   (project_id, section_id, semantic_hash, section_json, source_kind, carrier_path, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		projectID,
		section.ID,
		semanticHash,
		string(sectionJSON),
		string(SpecSectionSourceSQL),
		section.Path,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("insert raw spec section edition: %v", err)
	}
}

func specSectionEditionTestSection(id string) project.SpecSection {
	return project.SpecSection{
		ID:            id,
		Spec:          "target-system",
		SystemFrame:   project.SystemReferenceFrame{ID: "target_system", Kind: "target_system", Source: "declared"},
		Kind:          "acceptance",
		Title:         "Sync back",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
		Status:        "active",
		ValidUntil:    "2026-08-01",
		Terms:         []string{"carrier"},
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
		Line:          10,
	}
}
