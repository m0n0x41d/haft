package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func TestRunSpecStatusSummaryShowsLifecycleAction(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".haft", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	restore := enterTestProjectRoot(t, root)
	defer restore()

	restoreJSON := stubSpecStatusJSON(t, false)
	defer restoreJSON()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runSpecStatus(cmd, nil); err != nil {
		t.Fatalf("runSpecStatus returned error: %v", err)
	}

	got := output.String()
	for _, want := range []string{
		"Spec status: needs_action",
		"Next action: draft",
		"Carrier:     .haft/specs/target-system.md",
		"Allowed next steps:",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestRunSpecNextJSONReturnsLifecycleProjection(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".haft", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	restore := enterTestProjectRoot(t, root)
	defer restore()

	restoreJSON := stubSpecNextJSON(t, true)
	defer restoreJSON()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runSpecNext(cmd, nil); err != nil {
		t.Fatalf("runSpecNext returned error: %v", err)
	}

	var projection specflow.SpecLifecycleProjection
	if err := json.Unmarshal(output.Bytes(), &projection); err != nil {
		t.Fatalf("decode JSON: %v\nraw: %s", err, output.String())
	}
	if projection.Action != specflow.LifecycleActionDraft {
		t.Fatalf("Action = %q, want %q", projection.Action, specflow.LifecycleActionDraft)
	}
	if projection.WorkflowIntent.Phase != specflow.PhaseTargetEnvironmentDraft {
		t.Fatalf("WorkflowIntent.Phase = %q", projection.WorkflowIntent.Phase)
	}
}

func TestBuildSpecLifecycleProjectionReadsCurrentSQLEditionsBeforeCarriers(t *testing.T) {
	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	defer database.Close()
	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := project.SpecSection{
		ID:            "TS.sql.status.001",
		Spec:          "target-system",
		SystemFrame:   project.SystemReferenceFrame{ID: "target_system", Kind: "target_system", Source: "declared"},
		Kind:          "target.environment",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
		Status:        "active",
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
	edition := specflow.NewSpecSectionEdition("qnt_spec_sync_test", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	projection, err := buildSpecLifecycleProjection(root)
	if err != nil {
		t.Fatalf("buildSpecLifecycleProjection: %v", err)
	}
	if projection.SectionID != "TS.sql.status.001" {
		t.Fatalf("SectionID = %q, want SQL edition section", projection.SectionID)
	}
}

func TestLoadProjectSpecificationSetSQLFirstPreservesCarrierTermMapEntries(t *testing.T) {
	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := project.SpecSection{
		ID:            "TS.sql.term-map.001",
		Spec:          "target-system",
		SystemFrame:   project.SystemReferenceFrame{ID: "target_system", Kind: "target_system", Source: "declared"},
		Kind:          "target.environment",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
		Status:        "active",
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
	edition := specflow.NewSpecSectionEdition("qnt_spec_sync_test", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	specSet, err := loadProjectSpecificationSetSQLFirst(root)
	if err != nil {
		t.Fatalf("loadProjectSpecificationSetSQLFirst: %v", err)
	}
	if len(specSet.Sections) != 1 || specSet.Sections[0].ID != "TS.sql.term-map.001" {
		t.Fatalf("sections should come from SQL editions only: %#v", specSet.Sections)
	}
	if len(specSet.TermMapEntries) != 1 || specSet.TermMapEntries[0].Term != "HarnessableProject" {
		t.Fatalf("term-map entries should be preserved from typed carrier: %#v", specSet.TermMapEntries)
	}
	for _, document := range specSet.Documents {
		if document.Kind == project.SpecDocumentKindTermMap {
			return
		}
	}
	t.Fatalf("SQL-first spec set should retain typed term-map document: %#v", specSet.Documents)
}

func TestBuildSpecLifecycleProjectionPropagatesBaselineStoreError(t *testing.T) {
	root, _ := newBaselineTestProject(t)
	makeBaselineDBUnopenable(t)

	_, err := buildSpecLifecycleProjection(root)
	if err == nil {
		t.Fatal("buildSpecLifecycleProjection ignored baseline store error")
	}
}

func stubSpecStatusJSON(t *testing.T, value bool) func() {
	t.Helper()
	prev := specStatusJSON
	specStatusJSON = value
	return func() { specStatusJSON = prev }
}

func stubSpecNextJSON(t *testing.T, value bool) func() {
	t.Helper()
	prev := specNextJSON
	specNextJSON = value
	return func() { specNextJSON = prev }
}
