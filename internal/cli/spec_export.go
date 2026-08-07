package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

type specExportResult struct {
	SchemaVersion     int                                    `json:"schema_version"`
	AuthorityBoundary string                                 `json:"authority_boundary"`
	SourceOfTruth     string                                 `json:"source_of_truth"`
	Edition           specExportEdition                      `json:"edition"`
	Publication       specflow.SpecSectionEditionPublication `json:"publication"`
	Audit             specSyncEditionAudit                   `json:"audit"`
}

type specExportEdition struct {
	ProjectID    string `json:"project_id"`
	SectionID    string `json:"section_id"`
	SemanticHash string `json:"semantic_hash"`
	SourceKind   string `json:"source_kind"`
	CarrierPath  string `json:"carrier_path,omitempty"`
}

func runSpecExport(cmd *cobra.Command, args []string) error {
	if specExportJSON && specExportMarkdown {
		return fmt.Errorf("spec export --json and --markdown are mutually exclusive")
	}

	result, err := buildSpecExportResult(args[0])
	if err != nil {
		return err
	}

	if specExportJSON {
		return writeSpecExportJSON(cmd.OutOrStdout(), result)
	}
	if specExportMarkdown {
		_, writeErr := io.WriteString(cmd.OutOrStdout(), result.Publication.Markdown)
		return writeErr
	}
	return writeSpecExportText(cmd.OutOrStdout(), result)
}

func buildSpecExportResult(sectionID string) (specExportResult, error) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return specExportResult{}, fmt.Errorf("not a haft project: %w", err)
	}

	cfg, err := project.Load(haftDirFor(projectRoot))
	if err != nil {
		return specExportResult{}, err
	}
	projectID, store, closeStore, err := openSpecSectionEditionReadStore(
		context.Background(),
		projectRoot,
		cfg,
	)
	if err != nil {
		return specExportResult{}, err
	}
	defer closeStore()

	edition, err := store.GetCurrent(projectID, sectionID)
	if errors.Is(err, specflow.ErrSpecSectionEditionNotFound) {
		return specExportResult{}, fmt.Errorf("spec export requires a current SQL edition for %s; run `haft spec sync` first", sectionID)
	}
	if err != nil {
		return specExportResult{}, err
	}

	publication, err := specflow.RenderSpecSectionEditionMarkdown(edition)
	if err != nil {
		return specExportResult{}, err
	}

	return specExportResult{
		SchemaVersion:     1,
		AuthorityBoundary: specflow.SpecSectionPublicationProjectionAuthorityBoundary,
		SourceOfTruth:     "sql_project_graph",
		Edition: specExportEdition{
			ProjectID:    edition.ProjectID,
			SectionID:    edition.SectionID,
			SemanticHash: edition.SemanticHash,
			SourceKind:   string(edition.SourceKind),
			CarrierPath:  edition.CarrierPath,
		},
		Publication: publication,
		Audit: specSyncEditionAudit{
			SourceEpisteme:        "sql_spec_section_edition",
			PublicationProjection: publication.PublicationProjection,
			CarrierBytes:          publication.CarrierPath,
			AuthorityBoundary:     specSyncEditionAuditAuthorityBoundary,
		},
	}, nil
}

func writeSpecExportJSON(writer io.Writer, result specExportResult) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func writeSpecExportText(writer io.Writer, result specExportResult) error {
	if _, err := fmt.Fprintf(writer, "spec export: %s\n", result.Edition.SectionID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "source_of_truth: %s\n", result.SourceOfTruth); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "source_edition_hash: %s\n", result.Publication.SourceEditionHash); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "publication_hash: %s\n", result.Publication.PublicationHash); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "carrier_path: %s\n", result.Publication.CarrierPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "publication_projection: %s\n", result.Publication.PublicationProjection); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "authority_boundary: %s\n", result.AuthorityBoundary); err != nil {
		return err
	}
	_, err := fmt.Fprintln(writer, "markdown: use --markdown for carrier bytes")
	return err
}
