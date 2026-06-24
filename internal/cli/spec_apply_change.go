package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

type specApplyChangeResult struct {
	SchemaVersion     int                             `json:"schema_version"`
	AuthorityBoundary string                          `json:"authority_boundary"`
	Applied           bool                            `json:"applied"`
	DryRun            bool                            `json:"dry_run"`
	WouldApply        bool                            `json:"would_apply"`
	Noop              bool                            `json:"noop"`
	Change            project.SpecCarrierChangeReport `json:"change"`
	Audit             specSyncEditionAudit            `json:"audit"`
	Edition           *specSyncImportedEntry          `json:"edition,omitempty"`
	PlannedEdition    *specSyncImportedEntry          `json:"planned_edition,omitempty"`
	Conflict          *specApplyChangeConflict        `json:"conflict,omitempty"`
}

type specApplyChangeConflict struct {
	Reason              string `json:"reason"`
	SectionID           string `json:"section_id"`
	CurrentSemanticHash string `json:"current_semantic_hash"`
	BeforeSemanticHash  string `json:"before_semantic_hash"`
	CurrentSourceKind   string `json:"current_source_kind"`
	Resolution          string `json:"resolution"`
}

func runSpecApplyChange(cmd *cobra.Command, _ []string) error {
	input, err := specApplyChangeInputFromFlags()
	if err != nil {
		return err
	}

	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}
	cfg, err := project.Load(haftDirFor(projectRoot))
	if err != nil {
		return err
	}
	projectID, store, closeStore, err := openSpecSectionEditionStore(projectRoot, cfg)
	if err != nil {
		return err
	}
	defer closeStore()

	result, err := applySpecCarrierChangeToSQL(projectID, input, store)
	if specApplyChangeJSON {
		writeErr := writeSpecApplyChangeJSON(cmd.OutOrStdout(), result)
		if writeErr != nil {
			return writeErr
		}
		return err
	}
	writeErr := writeSpecApplyChangeText(cmd.OutOrStdout(), result)
	if writeErr != nil {
		return writeErr
	}
	return err
}

func specApplyChangeInputFromFlags() (specCarrierChangeInput, error) {
	input := specCarrierChangeInput{
		BeforePath: strings.TrimSpace(specApplyBefore),
		AfterPath:  strings.TrimSpace(specApplyAfter),
		SectionID:  strings.TrimSpace(specApplySection),
		Kind:       strings.TrimSpace(specApplyKind),
		DryRun:     specApplyDryRun,
	}
	if input.BeforePath == "" {
		return specCarrierChangeInput{}, fmt.Errorf("spec apply-change requires --before")
	}
	if input.AfterPath == "" {
		return specCarrierChangeInput{}, fmt.Errorf("spec apply-change requires --after")
	}
	if input.SectionID == "" {
		return specCarrierChangeInput{}, fmt.Errorf("spec apply-change requires --section")
	}
	return input, nil
}

func applySpecCarrierChangeToSQL(
	projectID string,
	input specCarrierChangeInput,
	store specflow.SpecSectionEditionStore,
) (specApplyChangeResult, error) {
	result := specApplyChangeResult{
		SchemaVersion:     1,
		AuthorityBoundary: "sql_edition_update_not_approval_rebaseline_or_prose_authority",
		DryRun:            input.DryRun,
		Audit: specSyncEditionAudit{
			SourceEpisteme:        "sql_spec_section_edition",
			PublicationProjection: "typed_yaml_spec_section_projection",
			CarrierBytes:          input.AfterPath,
			AuthorityBoundary:     specSyncEditionAuditAuthorityBoundary,
		},
	}

	before, err := loadSpecCarrierChangeSection(input.BeforePath, input.Kind, input.SectionID)
	if err != nil {
		return result, err
	}
	after, err := loadSpecCarrierChangeSection(input.AfterPath, input.Kind, input.SectionID)
	if err != nil {
		return result, err
	}

	change := project.ClassifySpecSectionCarrierChange(before, after)
	result.Change = change
	switch change.ImportPosture {
	case project.SpecCarrierImportPostureNoSemanticMutation:
		result.Noop = true
		result.Audit.CarrierOnlyDisposition = "carrier_only_no_semantic_edition_created"
		return result, nil
	case project.SpecCarrierImportPostureRecognizedUpdate:
		if err := blockConflictingSpecCarrierSyncBack(projectID, before, store, &result); err != nil {
			return result, err
		}
		edition := specflow.NewSpecSectionEdition(projectID, after, specflow.SpecSectionSourceSyncBack, time.Now().UTC())
		entry := specApplyChangeEditionEntry(edition, change.Kind)
		if input.DryRun {
			result.WouldApply = true
			result.PlannedEdition = &entry
			return result, nil
		}
		if err := store.PutCurrent(edition); err != nil {
			return result, err
		}
		result.Applied = true
		result.Audit.ImportedSemanticMutation = string(change.Kind)
		result.WouldApply = true
		result.Edition = &entry
		return result, nil
	default:
		return result, fmt.Errorf("spec apply-change blocked: %s", change.Kind)
	}
}

func specApplyChangeEditionEntry(
	edition specflow.SpecSectionEdition,
	changeKind project.SpecCarrierChangeKind,
) specSyncImportedEntry {
	return specSyncImportedEntry{
		SectionID:    edition.SectionID,
		SemanticHash: edition.SemanticHash,
		SourceKind:   string(edition.SourceKind),
		CarrierPath:  edition.CarrierPath,
		Audit: specSyncEditionAudit{
			SourceEpisteme:           "sql_spec_section_edition",
			PublicationProjection:    "typed_yaml_spec_section_projection",
			CarrierBytes:             edition.CarrierPath,
			ImportedSemanticMutation: string(changeKind),
			AuthorityBoundary:        specSyncEditionAuditAuthorityBoundary,
		},
	}
}

func blockConflictingSpecCarrierSyncBack(
	projectID string,
	before project.SpecSection,
	store specflow.SpecSectionEditionStore,
	result *specApplyChangeResult,
) error {
	current, err := store.GetCurrent(projectID, before.ID)
	if errors.Is(err, specflow.ErrSpecSectionEditionNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	beforeHash := specflow.HashSection(before)
	if current.SemanticHash == beforeHash {
		return nil
	}

	result.Conflict = &specApplyChangeConflict{
		Reason:              "current_sql_edition_differs_from_before_carrier",
		SectionID:           before.ID,
		CurrentSemanticHash: current.SemanticHash,
		BeforeSemanticHash:  beforeHash,
		CurrentSourceKind:   string(current.SourceKind),
		Resolution:          "refresh the carrier from current SQL truth, re-review the edit, then rerun apply-change",
	}
	return fmt.Errorf("spec apply-change blocked: current SQL edition for %s does not match --before carrier", before.ID)
}

func writeSpecApplyChangeJSON(writer io.Writer, result specApplyChangeResult) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func writeSpecApplyChangeText(writer io.Writer, result specApplyChangeResult) error {
	if _, err := fmt.Fprintf(writer, "spec apply-change: %s\n", result.Change.Kind); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "applied: %t\n", result.Applied); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "dry_run: %t\n", result.DryRun); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "would_apply: %t\n", result.WouldApply); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "noop: %t\n", result.Noop); err != nil {
		return err
	}
	if result.Conflict != nil {
		if _, err := fmt.Fprintf(writer, "conflict: %s\n", result.Conflict.Reason); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(writer, "resolution: %s\n", result.Conflict.Resolution); err != nil {
			return err
		}
	}
	if _, err := writeSpecApplyChangeAuditText(writer, result.Audit); err != nil {
		return err
	}
	_, err := fmt.Fprintf(writer, "authority_boundary: %s\n", result.AuthorityBoundary)
	return err
}

func writeSpecApplyChangeAuditText(writer io.Writer, audit specSyncEditionAudit) (int, error) {
	written, err := fmt.Fprintln(writer, "audit:")
	if err != nil {
		return written, err
	}
	lines := []string{
		"  source_episteme: " + audit.SourceEpisteme,
		"  publication_projection: " + audit.PublicationProjection,
		"  carrier_bytes: " + audit.CarrierBytes,
	}
	if audit.ImportedSemanticMutation != "" {
		lines = append(lines, "  imported_semantic_mutation: "+audit.ImportedSemanticMutation)
	}
	if audit.CarrierOnlyDisposition != "" {
		lines = append(lines, "  carrier_only_disposition: "+audit.CarrierOnlyDisposition)
	}
	lines = append(lines, "  authority_boundary: "+audit.AuthorityBoundary)

	for _, line := range lines {
		count, err := fmt.Fprintln(writer, line)
		written += count
		if err != nil {
			return written, err
		}
	}
	return written, nil
}
