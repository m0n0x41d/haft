package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

const specRepairEditionsAuthorityBoundary = "sql_spec_section_edition_hash_repair_not_approval_rebaseline_evidence_gate_claim_truth_global_truth_or_publication"

type specRepairEditionsResult struct {
	SchemaVersion     int                                       `json:"schema_version"`
	AuthorityBoundary string                                    `json:"authority_boundary"`
	SourceOfTruth     string                                    `json:"source_of_truth"`
	Mode              string                                    `json:"mode"`
	ProjectID         string                                    `json:"project_id"`
	RepairScope       string                                    `json:"repair_scope"`
	Mismatches        []specflow.SpecSectionEditionHashMismatch `json:"mismatches"`
	Repaired          []specflow.SpecSectionEditionHashMismatch `json:"repaired,omitempty"`
}

func runSpecRepairEditions(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	result, err := buildSpecRepairEditionsResult(projectRoot, specRepairEditionsApply)
	if err != nil {
		return err
	}
	if specRepairEditionsJSON {
		return writeSpecRepairEditionsJSON(cmd.OutOrStdout(), result)
	}
	return writeSpecRepairEditionsText(cmd.OutOrStdout(), result)
}

func buildSpecRepairEditionsResult(projectRoot string, apply bool) (specRepairEditionsResult, error) {
	cfg, err := project.Load(haftDirFor(projectRoot))
	if err != nil {
		return specRepairEditionsResult{}, err
	}
	projectID, store, closeStore, err := openSpecSectionEditionStore(projectRoot, cfg)
	if err != nil {
		return specRepairEditionsResult{}, err
	}
	defer closeStore()

	plan, err := store.ListSemanticHashMismatches(projectID)
	if err != nil {
		return specRepairEditionsResult{}, err
	}
	if apply {
		plan, err = store.RepairSemanticHashMismatches(projectID)
		if err != nil {
			return specRepairEditionsResult{}, err
		}
	}

	return specRepairEditionsResult{
		SchemaVersion:     1,
		AuthorityBoundary: specRepairEditionsAuthorityBoundary,
		SourceOfTruth:     "sql_project_graph",
		Mode:              specRepairEditionsMode(apply),
		ProjectID:         plan.ProjectID,
		RepairScope:       plan.RepairScope,
		Mismatches:        copySpecRepairEditionsMismatches(plan.Mismatches),
		Repaired:          copySpecRepairEditionsMismatches(plan.Repaired),
	}, nil
}

func copySpecRepairEditionsMismatches(
	values []specflow.SpecSectionEditionHashMismatch,
) []specflow.SpecSectionEditionHashMismatch {
	out := make([]specflow.SpecSectionEditionHashMismatch, len(values))
	copy(out, values)
	return out
}

func specRepairEditionsMode(apply bool) string {
	if apply {
		return "apply"
	}
	return "dry_run"
}

func writeSpecRepairEditionsJSON(writer io.Writer, result specRepairEditionsResult) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func writeSpecRepairEditionsText(writer io.Writer, result specRepairEditionsResult) error {
	if _, err := fmt.Fprintf(writer, "spec repair-editions: %d mismatch(es)\n", len(result.Mismatches)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "mode: %s\n", result.Mode); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "authority_boundary: %s\n", result.AuthorityBoundary); err != nil {
		return err
	}
	if len(result.Mismatches) == 0 {
		return nil
	}

	ids := make([]string, 0, len(result.Mismatches))
	for _, mismatch := range result.Mismatches {
		ids = append(ids, mismatch.SectionID)
	}
	if _, err := fmt.Fprintf(writer, "sections: %s\n", strings.Join(ids, ", ")); err != nil {
		return err
	}
	if result.Mode == "dry_run" {
		_, err := fmt.Fprintln(writer, "next_action: rerun with --apply to update semantic_hash cache values")
		return err
	}
	_, err := fmt.Fprintf(writer, "repaired: %d\n", len(result.Repaired))
	return err
}
