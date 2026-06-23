package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func TestRunSpecApplyChangeAppliesRecognizedRelationshipUpdate(t *testing.T) {
	root := setupSpecSyncProject(t)
	restoreCwd := chdirForTest(t, root)
	defer restoreCwd()
	before := writeSpecClassifyChangeFile(t, "target-system.md", specClassifyChangeCarrier("TS.sync.001", "acceptance", ""))
	after := writeSpecClassifyChangeFile(t, "target-system-after.md", specClassifyChangeCarrier("TS.sync.001", "acceptance", "depends_on:\n  - TS.boundary.001\n"))
	restoreFlags := stubSpecApplyChangeFlags(t, before, after, "TS.sync.001", "target-system", true)
	defer restoreFlags()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runSpecApplyChange(cmd, nil); err != nil {
		t.Fatalf("runSpecApplyChange: %v\n%s", err, output.String())
	}

	var result specApplyChangeResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, output.String())
	}
	if !result.Applied || result.Noop {
		t.Fatalf("apply result = %+v", result)
	}
	if result.Change.Kind != project.SpecCarrierChangeRelationshipUpdate {
		t.Fatalf("change kind = %q", result.Change.Kind)
	}
	if result.Audit.SourceEpisteme != "sql_spec_section_edition" {
		t.Fatalf("source episteme = %#v", result.Audit)
	}
	if result.Audit.ImportedSemanticMutation != "relationship_update" {
		t.Fatalf("imported mutation = %#v", result.Audit)
	}

	database := openSpecSyncDB(t, root)
	defer database.Close()
	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	edition, err := store.GetCurrent("qnt_spec_sync_test", "TS.sync.001")
	if err != nil {
		t.Fatalf("GetCurrent: %v", err)
	}
	if !strings.Contains(strings.Join(edition.Section.DependsOn, ","), "TS.boundary.001") {
		t.Fatalf("depends_on = %#v", edition.Section.DependsOn)
	}
	if edition.SourceKind != specflow.SpecSectionSourceSyncBack {
		t.Fatalf("source_kind = %q", edition.SourceKind)
	}
}

func TestRunSpecApplyChangeBlocksUnknownHighRisk(t *testing.T) {
	root := setupSpecSyncProject(t)
	restoreCwd := chdirForTest(t, root)
	defer restoreCwd()
	before := writeSpecClassifyChangeFile(t, "target-system.md", specClassifyChangeCarrier("TS.sync.001", "acceptance", ""))
	after := writeSpecClassifyChangeFile(t, "enabling-system.md", specClassifyChangeCarrier("TS.sync.001", "acceptance", ""))
	restoreFlags := stubSpecApplyChangeFlags(t, before, after, "TS.sync.001", "", true)
	defer restoreFlags()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	err := runSpecApplyChange(cmd, nil)
	if err == nil {
		t.Fatal("expected high-risk apply to block")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("unexpected error: %v", err)
	}

	var result specApplyChangeResult
	if jsonErr := json.Unmarshal(output.Bytes(), &result); jsonErr != nil {
		t.Fatalf("decode blocked result: %v\n%s", jsonErr, output.String())
	}
	if result.Applied {
		t.Fatalf("blocked result applied: %+v", result)
	}
	if result.Change.Kind != project.SpecCarrierChangeUnknownHighRisk {
		t.Fatalf("change kind = %q", result.Change.Kind)
	}
}

func TestApplySpecCarrierChangeToSQLBlockedHighRiskPreservesCurrentEdition(t *testing.T) {
	database := newTestCLIDB(t)
	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	seed := project.SpecSection{
		ID:            "TS.sync.001",
		Spec:          "target-system",
		Kind:          "acceptance",
		Title:         "Current SQL truth",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "human",
		Status:        "active",
		ValidUntil:    "2026-08-01",
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
	seedEdition := specflow.NewSpecSectionEdition("proj-1", seed, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(seedEdition); err != nil {
		t.Fatalf("seed current SQL edition: %v", err)
	}

	before := writeSpecClassifyChangeFile(t, "target-system.md", specClassifyChangeCarrier("TS.sync.001", "acceptance", ""))
	after := writeSpecClassifyChangeFile(t, "enabling-system.md", specClassifyChangeCarrier("TS.sync.001", "acceptance", ""))

	result, err := applySpecCarrierChangeToSQL("proj-1", specCarrierChangeInput{
		BeforePath: before,
		AfterPath:  after,
		SectionID:  "TS.sync.001",
	}, store)
	if err == nil {
		t.Fatal("expected high-risk apply to block")
	}
	if result.Applied || result.Noop {
		t.Fatalf("blocked high-risk result should not apply or noop: %+v", result)
	}
	if result.Change.Kind != project.SpecCarrierChangeUnknownHighRisk {
		t.Fatalf("change kind = %q", result.Change.Kind)
	}

	current, getErr := store.GetCurrent("proj-1", "TS.sync.001")
	if getErr != nil {
		t.Fatalf("GetCurrent after blocked apply: %v", getErr)
	}
	if current.SemanticHash != seedEdition.SemanticHash {
		t.Fatalf("blocked apply changed semantic hash: got %s want %s", current.SemanticHash, seedEdition.SemanticHash)
	}
	if current.Section.Title != "Current SQL truth" {
		t.Fatalf("blocked apply changed current section title: %q", current.Section.Title)
	}
	if current.SourceKind != specflow.SpecSectionSourceSQL {
		t.Fatalf("blocked apply changed source kind: %q", current.SourceKind)
	}
}

func TestApplySpecCarrierChangeToSQLBlocksStaleBeforeCarrier(t *testing.T) {
	database := newTestCLIDB(t)
	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	before := writeSpecClassifyChangeFile(t, "target-system.md", specClassifyChangeCarrier("TS.sync.001", "acceptance", ""))
	after := writeSpecClassifyChangeFile(t, "target-system-after.md", specClassifyChangeCarrier("TS.sync.001", "acceptance", "depends_on:\n  - TS.boundary.001\n"))
	currentSQL := writeSpecClassifyChangeFile(t, "target-system-current.md", specClassifyChangeCarrier("TS.sync.001", "acceptance", "depends_on:\n  - TS.current.001\n"))
	currentSection, loadErr := loadSpecCarrierChangeSection(currentSQL, "target-system", "TS.sync.001")
	if loadErr != nil {
		t.Fatalf("load current SQL section: %v", loadErr)
	}
	currentEdition := specflow.NewSpecSectionEdition("proj-1", currentSection, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(currentEdition); err != nil {
		t.Fatalf("seed current SQL edition: %v", err)
	}

	result, err := applySpecCarrierChangeToSQL("proj-1", specCarrierChangeInput{
		BeforePath: before,
		AfterPath:  after,
		SectionID:  "TS.sync.001",
		Kind:       "target-system",
	}, store)
	if err == nil {
		t.Fatal("expected stale before carrier to block")
	}
	if !strings.Contains(err.Error(), "current SQL edition") {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Applied || result.Noop {
		t.Fatalf("conflict result should not apply or noop: %+v", result)
	}
	if result.Conflict == nil {
		t.Fatalf("conflict missing: %+v", result)
	}
	if result.Conflict.Reason != "current_sql_edition_differs_from_before_carrier" {
		t.Fatalf("conflict reason = %q", result.Conflict.Reason)
	}
	if result.Conflict.CurrentSemanticHash != currentEdition.SemanticHash {
		t.Fatalf("current semantic hash = %q, want %q", result.Conflict.CurrentSemanticHash, currentEdition.SemanticHash)
	}
	if result.Conflict.BeforeSemanticHash == currentEdition.SemanticHash {
		t.Fatalf("before semantic hash unexpectedly matched current: %+v", result.Conflict)
	}

	current, getErr := store.GetCurrent("proj-1", "TS.sync.001")
	if getErr != nil {
		t.Fatalf("GetCurrent after blocked conflict: %v", getErr)
	}
	if current.SemanticHash != currentEdition.SemanticHash {
		t.Fatalf("conflict changed current semantic hash: got %s want %s", current.SemanticHash, currentEdition.SemanticHash)
	}
	if !strings.Contains(strings.Join(current.Section.DependsOn, ","), "TS.current.001") {
		t.Fatalf("current depends_on overwritten: %#v", current.Section.DependsOn)
	}
}

func TestApplySpecCarrierChangeToSQLCarrierOnlyNoop(t *testing.T) {
	database := newTestCLIDB(t)
	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	before := writeSpecClassifyChangeFile(t, "target-system.md", specClassifyChangeCarrier("TS.sync.001", "acceptance", ""))
	after := writeSpecClassifyChangeFile(t, "target-system-moved.md", specClassifyChangeCarrier("TS.sync.001", "acceptance", ""))

	result, err := applySpecCarrierChangeToSQL("proj-1", specCarrierChangeInput{
		BeforePath: before,
		AfterPath:  after,
		SectionID:  "TS.sync.001",
		Kind:       "target-system",
	}, store)
	if err != nil {
		t.Fatalf("applySpecCarrierChangeToSQL: %v", err)
	}
	if !result.Noop || result.Applied {
		t.Fatalf("carrier-only result = %+v", result)
	}
	if result.Audit.CarrierOnlyDisposition != "carrier_only_no_semantic_edition_created" {
		t.Fatalf("carrier-only audit = %#v", result.Audit)
	}
	if _, getErr := store.GetCurrent("proj-1", "TS.sync.001"); getErr == nil {
		t.Fatal("carrier-only no-op wrote an edition")
	}
}

func stubSpecApplyChangeFlags(t *testing.T, before string, after string, section string, kind string, jsonFlag bool) func() {
	t.Helper()
	previousBefore := specApplyBefore
	previousAfter := specApplyAfter
	previousSection := specApplySection
	previousKind := specApplyKind
	previousJSON := specApplyChangeJSON
	specApplyBefore = before
	specApplyAfter = after
	specApplySection = section
	specApplyKind = kind
	specApplyChangeJSON = jsonFlag
	return func() {
		specApplyBefore = previousBefore
		specApplyAfter = previousAfter
		specApplySection = previousSection
		specApplyKind = previousKind
		specApplyChangeJSON = previousJSON
	}
}
