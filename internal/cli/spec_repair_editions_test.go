package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func TestRunSpecRepairEditionsDryRunReportsMismatches(t *testing.T) {
	root := setupSpecSyncProject(t)
	restoreCwd := chdirForTest(t, root)
	defer restoreCwd()

	database := openSpecSyncDB(t, root)
	defer database.Close()
	section := specRepairEditionsTestSection("TS.stale.001")
	putRawCLISpecSectionEdition(t, database, "qnt_5eec5eec", "stale-hash", section)

	restoreFlags := stubSpecRepairEditionsFlags(t, true, false)
	defer restoreFlags()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := runSpecRepairEditions(cmd, nil); err != nil {
		t.Fatalf("runSpecRepairEditions: %v", err)
	}

	var result specRepairEditionsResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, output.String())
	}
	if result.Mode != "dry_run" {
		t.Fatalf("mode = %q, want dry_run", result.Mode)
	}
	if result.AuthorityBoundary != specRepairEditionsAuthorityBoundary {
		t.Fatalf("authority_boundary = %q", result.AuthorityBoundary)
	}
	if len(result.Mismatches) != 1 {
		t.Fatalf("mismatches = %#v, want one", result.Mismatches)
	}
	if len(result.Repaired) != 0 {
		t.Fatalf("repaired = %#v, want none for dry-run", result.Repaired)
	}

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	_, err := store.GetCurrent("qnt_5eec5eec", "TS.stale.001")
	if !errors.Is(err, specflow.ErrSpecSectionEditionSemanticHashMismatch) {
		t.Fatalf("dry-run should leave mismatch in place, err = %v", err)
	}
}

func TestRunSpecRepairEditionsApplyUnblocksSpecExport(t *testing.T) {
	root := setupSpecSyncProject(t)
	restoreCwd := chdirForTest(t, root)
	defer restoreCwd()

	database := openSpecSyncDB(t, root)
	defer database.Close()
	section := specRepairEditionsTestSection("TS.stale.001")
	putRawCLISpecSectionEdition(t, database, "qnt_5eec5eec", "stale-hash", section)

	restoreRepairFlags := stubSpecRepairEditionsFlags(t, true, true)
	defer restoreRepairFlags()

	var repairOutput bytes.Buffer
	repairCmd := &cobra.Command{}
	repairCmd.SetOut(&repairOutput)
	if err := runSpecRepairEditions(repairCmd, nil); err != nil {
		t.Fatalf("runSpecRepairEditions --apply: %v", err)
	}
	var repairResult specRepairEditionsResult
	if err := json.Unmarshal(repairOutput.Bytes(), &repairResult); err != nil {
		t.Fatalf("decode repair result: %v\n%s", err, repairOutput.String())
	}
	if repairResult.Mode != "apply" || len(repairResult.Repaired) != 1 {
		t.Fatalf("repair result = %#v, want one applied repair", repairResult)
	}

	restoreExportFlags := stubSpecExportFlags(t, true, false)
	defer restoreExportFlags()

	var exportOutput bytes.Buffer
	exportCmd := &cobra.Command{}
	exportCmd.SetOut(&exportOutput)
	if err := runSpecExport(exportCmd, []string{"TS.stale.001"}); err != nil {
		t.Fatalf("runSpecExport after repair: %v\n%s", err, exportOutput.String())
	}
	var exportResult specExportResult
	if err := json.Unmarshal(exportOutput.Bytes(), &exportResult); err != nil {
		t.Fatalf("decode export result: %v\n%s", err, exportOutput.String())
	}
	if exportResult.Edition.SemanticHash != specflow.HashSection(section) {
		t.Fatalf("export semantic_hash = %q, want repaired HashSection", exportResult.Edition.SemanticHash)
	}
}

func TestRunSpecRepairEditionsDryRunReturnsEmptyMismatchList(t *testing.T) {
	root := setupSpecSyncProject(t)
	restoreCwd := chdirForTest(t, root)
	defer restoreCwd()

	restoreFlags := stubSpecRepairEditionsFlags(t, true, false)
	defer restoreFlags()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := runSpecRepairEditions(cmd, nil); err != nil {
		t.Fatalf("runSpecRepairEditions: %v", err)
	}

	var result specRepairEditionsResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, output.String())
	}
	if result.Mismatches == nil {
		t.Fatalf("mismatches should marshal as an empty list, got nil in result: %s", output.String())
	}
	if len(result.Mismatches) != 0 {
		t.Fatalf("mismatches = %#v, want none", result.Mismatches)
	}
	if !strings.Contains(output.String(), `"mismatches": []`) {
		t.Fatalf("JSON should expose empty mismatches list:\n%s", output.String())
	}
}

func stubSpecRepairEditionsFlags(t *testing.T, jsonOutput bool, apply bool) func() {
	t.Helper()

	previousJSON := specRepairEditionsJSON
	previousApply := specRepairEditionsApply
	specRepairEditionsJSON = jsonOutput
	specRepairEditionsApply = apply

	return func() {
		specRepairEditionsJSON = previousJSON
		specRepairEditionsApply = previousApply
	}
}

func specRepairEditionsTestSection(id string) project.SpecSection {
	return project.SpecSection{
		ID:            id,
		Spec:          "target-system",
		SystemFrame:   project.SystemReferenceFrame{ID: "target_system", Kind: "target_system", Source: "declared"},
		Kind:          "target.environment",
		Title:         "Stale SQL section",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
		Status:        "active",
		ValidUntil:    "2026-12-31",
		DependsOn:     []string{"TS.sync.001"},
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
}

func putRawCLISpecSectionEdition(
	t *testing.T,
	database *db.Store,
	projectID string,
	semanticHash string,
	section project.SpecSection,
) {
	t.Helper()

	sectionJSON, err := json.Marshal(section)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.GetRawDB().Exec(
		`INSERT INTO spec_section_editions
		   (project_id, section_id, semantic_hash, section_json, source_kind, carrier_path, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		projectID,
		section.ID,
		strings.TrimSpace(semanticHash),
		string(sectionJSON),
		string(specflow.SpecSectionSourceSQL),
		section.Path,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("insert raw spec section edition: %v", err)
	}
}
