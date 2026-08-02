package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

func TestRunSpecSyncImportsTypedSectionsIntoSQLWithoutBaselines(t *testing.T) {
	root := setupSpecSyncProject(t)
	restoreCwd := chdirForTest(t, root)
	defer restoreCwd()
	restoreFlags := stubSpecSyncFlags(t, true)
	defer restoreFlags()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runSpecSync(cmd, nil); err != nil {
		t.Fatalf("runSpecSync: %v", err)
	}

	var result specSyncResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, output.String())
	}
	if result.AuthorityBoundary != specSyncAuthorityBoundary {
		t.Fatalf("authority boundary = %q", result.AuthorityBoundary)
	}
	if result.Scope != specSyncScopeFullProject ||
		result.RequestedSection != "" {
		t.Fatalf("full sync scope = %#v", result)
	}
	for _, want := range []string{"evidence", "gate", "claim_truth", "global_truth", "prose_authority"} {
		if !strings.Contains(result.AuthorityBoundary, want) {
			t.Fatalf("authority boundary missing %q: %q", want, result.AuthorityBoundary)
		}
	}
	if len(result.Imported) != 2 {
		t.Fatalf("imported = %#v, want two sections", result.Imported)
	}
	if result.Imported[0].Audit.SourceEpisteme != "sql_spec_section_edition" {
		t.Fatalf("source episteme = %#v", result.Imported[0].Audit)
	}
	if result.Imported[0].Audit.PublicationProjection != "typed_yaml_spec_section_projection" {
		t.Fatalf("publication projection = %#v", result.Imported[0].Audit)
	}
	if result.Imported[0].Audit.ImportedSemanticMutation != "carrier_import_to_sql_edition" {
		t.Fatalf("imported mutation = %#v", result.Imported[0].Audit)
	}

	database := openSpecSyncDB(t, root)
	defer database.Close()
	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	edition, err := store.GetCurrent("qnt_5eec5eec", "TS.sync.001")
	if err != nil {
		t.Fatalf("GetCurrent target section: %v", err)
	}
	if edition.Section.Title != "" {
		t.Fatalf("title = %q, fixture title should be empty from carrier block", edition.Section.Title)
	}
	if edition.SourceKind != specflow.SpecSectionSourceCarrierImport {
		t.Fatalf("source kind = %q", edition.SourceKind)
	}

	var baselineRows int
	if err := database.GetRawDB().QueryRow(`SELECT COUNT(*) FROM spec_section_baselines`).Scan(&baselineRows); err != nil {
		t.Fatalf("count baselines: %v", err)
	}
	if baselineRows != 0 {
		t.Fatalf("spec sync must not create baselines, got %d", baselineRows)
	}
}

func TestSyncProjectSpecificationSetToSQLWithScopeBlocksCarrierFindings(t *testing.T) {
	database := newTestCLIDB(t)
	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	specSet := project.ProjectSpecificationSet{
		Sections: []project.SpecSection{{ID: "TS.bad.001", Status: "active"}},
		Findings: []project.SpecCheckFinding{
			{Code: "spec_section_invalid_yaml", Message: "bad yaml"},
		},
	}

	scope, err := newSpecSyncScope("")
	if err != nil {
		t.Fatalf("construct full-project sync scope: %v", err)
	}
	result, err := syncProjectSpecificationSetToSQLWithScope(
		"proj-1",
		specSet,
		store,
		scope,
	)
	if err == nil {
		t.Fatal("expected sync to block on findings")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.BlockedFindings) != 1 {
		t.Fatalf("blocked findings = %#v", result.BlockedFindings)
	}
	if _, getErr := store.GetCurrent("proj-1", "TS.bad.001"); getErr == nil {
		t.Fatal("blocked sync wrote a spec section edition")
	}
}

func TestSyncProjectSpecificationSetToSQLWithScopeFullProjectDeletesMissingCarrierSections(t *testing.T) {
	database := newTestCLIDB(t)
	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())

	kept := project.SpecSection{
		ID:           "TS.kept.001",
		Spec:         "target-system",
		DocumentKind: string(project.SpecDocumentKindTargetSystem),
		Status:       "active",
		Path:         ".haft/specs/target-system.md",
	}
	removed := project.SpecSection{
		ID:           "TS.removed.001",
		Spec:         "target-system",
		DocumentKind: string(project.SpecDocumentKindTargetSystem),
		Status:       "active",
		Path:         ".haft/specs/target-system.md",
	}
	if err := store.PutCurrent(specflow.NewSpecSectionEdition("proj-1", kept, specflow.SpecSectionSourceCarrierImport, time.Time{})); err != nil {
		t.Fatalf("seed kept edition: %v", err)
	}
	if err := store.PutCurrent(specflow.NewSpecSectionEdition("proj-1", removed, specflow.SpecSectionSourceCarrierImport, time.Time{})); err != nil {
		t.Fatalf("seed removed edition: %v", err)
	}

	scope, err := newSpecSyncScope("")
	if err != nil {
		t.Fatalf("construct full-project sync scope: %v", err)
	}
	_, err = syncProjectSpecificationSetToSQLWithScope(
		"proj-1",
		project.ProjectSpecificationSet{
			Sections: []project.SpecSection{kept},
		},
		store,
		scope,
	)
	if err != nil {
		t.Fatalf("syncProjectSpecificationSetToSQLWithScope: %v", err)
	}

	if _, err := store.GetCurrent("proj-1", "TS.kept.001"); err != nil {
		t.Fatalf("kept edition missing: %v", err)
	}
	_, err = store.GetCurrent("proj-1", "TS.removed.001")
	if !errors.Is(err, specflow.ErrSpecSectionEditionNotFound) {
		t.Fatalf("removed edition err = %v, want ErrSpecSectionEditionNotFound", err)
	}
}

func TestSyncProjectSpecificationSetToSQLExactSectionLeavesOtherEditionsUntouched(
	t *testing.T,
) {
	database := newTestCLIDB(t)
	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	selected := project.SpecSection{
		ID:           "SS.selected.001",
		Spec:         "software-system",
		DocumentKind: string(project.SpecDocumentKindSoftwareSystem),
		Status:       "draft",
		Title:        "Selected carrier section",
		Path:         ".haft/specs/software-system.md",
	}
	unrelatedBefore := project.SpecSection{
		ID:           "TS.unrelated.001",
		Spec:         "target-system",
		DocumentKind: string(project.SpecDocumentKindTargetSystem),
		Status:       "active",
		Title:        "SQL truth remains",
		Path:         ".haft/specs/target-system.md",
	}
	unrelatedCarrier := unrelatedBefore
	unrelatedCarrier.Title = "Carrier drift must not sync"
	unrelatedEdition := specflow.NewSpecSectionEdition(
		"proj-1",
		unrelatedBefore,
		specflow.SpecSectionSourceCarrierImport,
		time.Time{},
	)
	if err := store.PutCurrent(unrelatedEdition); err != nil {
		t.Fatalf("seed unrelated edition: %v", err)
	}
	scope, err := newSpecSyncScope(selected.ID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := syncProjectSpecificationSetToSQLWithScope(
		"proj-1",
		project.ProjectSpecificationSet{
			Sections: []project.SpecSection{
				selected,
				unrelatedCarrier,
			},
		},
		store,
		scope,
	)
	if err != nil {
		t.Fatalf("scoped spec sync: %v", err)
	}
	if result.Scope != specSyncScopeExactSection ||
		result.RequestedSection != selected.ID ||
		len(result.Imported) != 1 ||
		result.Imported[0].SectionID != selected.ID {
		t.Fatalf("scoped sync result = %#v", result)
	}
	currentUnrelated, err := store.GetCurrent(
		"proj-1",
		unrelatedBefore.ID,
	)
	if err != nil {
		t.Fatalf("read unrelated edition: %v", err)
	}
	if currentUnrelated.SemanticHash != unrelatedEdition.SemanticHash ||
		currentUnrelated.Section.Title != unrelatedBefore.Title {
		t.Fatalf(
			"scoped sync mutated unrelated edition: before=%#v after=%#v",
			unrelatedEdition,
			currentUnrelated,
		)
	}
	if _, err := store.GetCurrent("proj-1", selected.ID); err != nil {
		t.Fatalf("selected edition missing: %v", err)
	}
}

func TestSyncProjectSpecificationSetToSQLExactSectionRejectsMissingCarrier(
	t *testing.T,
) {
	database := newTestCLIDB(t)
	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	scope, err := newSpecSyncScope("SS.missing.001")
	if err != nil {
		t.Fatal(err)
	}
	result, err := syncProjectSpecificationSetToSQLWithScope(
		"proj-1",
		project.ProjectSpecificationSet{
			Sections: []project.SpecSection{
				{ID: "SS.present.001", Status: "draft"},
			},
		},
		store,
		scope,
	)
	if err == nil || !strings.Contains(err.Error(), "not present") {
		t.Fatalf("missing exact section error = %v", err)
	}
	if len(result.Imported) != 0 {
		t.Fatalf("missing exact section wrote entries: %#v", result)
	}
}

func setupSpecSyncProject(t *testing.T) string {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	haftDir := filepath.Join(root, ".haft")
	specDir := filepath.Join(haftDir, "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSpecCheckCLIFile(t, filepath.Join(haftDir, "project.yaml"), "id: qnt_5eec5eec\nname: spec-sync-test\n")
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "target-system.md"), validCLISpecSectionCarrier("TS.sync.001", "acceptance"))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "software-system.md"), validCLISpecSectionCarrier("SS.sync.001", "software.role"))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "term-map.md"), validCLITermMapCarrier())
	database := openSpecSyncDB(t, root)
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	if err := projectledger.BindInitialized(
		context.Background(),
		root,
		time.Date(2026, time.July, 18, 8, 0, 0, 0, time.UTC),
	); err != nil {
		t.Fatalf("bind initialized spec fixture: %v", err)
	}
	return root
}

func openSpecSyncDB(t *testing.T, root string) *db.Store {
	t.Helper()
	cfg, err := project.Load(filepath.Join(root, ".haft"))
	if err != nil {
		t.Fatal(err)
	}
	dbPath, err := cfg.DBPath()
	if err != nil {
		t.Fatal(err)
	}
	database, err := openCurrentKernelTestStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	return database
}

func newTestCLIDB(t *testing.T) *db.Store {
	t.Helper()
	database, err := openCurrentKernelTestStore(
		filepath.Join(t.TempDir(), "haft.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func chdirForTest(t *testing.T, dir string) func() {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatal(err)
		}
	}
}

func stubSpecSyncFlags(t *testing.T, jsonFlag bool) func() {
	t.Helper()
	previousJSON := specSyncJSON
	previousSection := specSyncSection
	specSyncJSON = jsonFlag
	specSyncSection = ""
	return func() {
		specSyncJSON = previousJSON
		specSyncSection = previousSection
	}
}
