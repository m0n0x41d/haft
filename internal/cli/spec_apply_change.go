package cli

import (
	"encoding/json"
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
	Noop              bool                            `json:"noop"`
	Change            project.SpecCarrierChangeReport `json:"change"`
	Edition           *specSyncImportedEntry          `json:"edition,omitempty"`
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
		return result, nil
	case project.SpecCarrierImportPostureRecognizedUpdate:
		edition := specflow.NewSpecSectionEdition(projectID, after, specflow.SpecSectionSourceSyncBack, time.Now().UTC())
		if err := store.PutCurrent(edition); err != nil {
			return result, err
		}
		result.Applied = true
		result.Edition = &specSyncImportedEntry{
			SectionID:    edition.SectionID,
			SemanticHash: edition.SemanticHash,
			SourceKind:   string(edition.SourceKind),
			CarrierPath:  edition.CarrierPath,
		}
		return result, nil
	default:
		return result, fmt.Errorf("spec apply-change blocked: %s", change.Kind)
	}
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
	if _, err := fmt.Fprintf(writer, "noop: %t\n", result.Noop); err != nil {
		return err
	}
	_, err := fmt.Fprintf(writer, "authority_boundary: %s\n", result.AuthorityBoundary)
	return err
}
