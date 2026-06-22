package specflow

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/project"
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
