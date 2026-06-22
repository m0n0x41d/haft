package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
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
	if result.AuthorityBoundary != "typed_spec_section_import_not_approval_rebaseline_or_prose_authority" {
		t.Fatalf("authority boundary = %q", result.AuthorityBoundary)
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
	edition, err := store.GetCurrent("qnt_spec_sync_test", "TS.sync.001")
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

func TestSyncProjectSpecificationSetToSQLBlocksCarrierFindings(t *testing.T) {
	database := newTestCLIDB(t)
	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	specSet := project.ProjectSpecificationSet{
		Sections: []project.SpecSection{{ID: "TS.bad.001", Status: "active"}},
		Findings: []project.SpecCheckFinding{
			{Code: "spec_section_invalid_yaml", Message: "bad yaml"},
		},
	}

	result, err := syncProjectSpecificationSetToSQL("proj-1", specSet, store)
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

func setupSpecSyncProject(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	specDir := filepath.Join(haftDir, "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSpecCheckCLIFile(t, filepath.Join(haftDir, "project.yaml"), "id: qnt_spec_sync_test\nname: spec-sync-test\n")
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "target-system.md"), validCLISpecSectionCarrier("TS.sync.001", "acceptance"))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "enabling-system.md"), validCLISpecSectionCarrier("ES.sync.001", "creator-role"))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "term-map.md"), validCLITermMapCarrier())
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
	database, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	return database
}

func newTestCLIDB(t *testing.T) *db.Store {
	t.Helper()
	database, err := db.NewStore(filepath.Join(t.TempDir(), "haft.db"))
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
	specSyncJSON = jsonFlag
	return func() {
		specSyncJSON = previousJSON
	}
}
