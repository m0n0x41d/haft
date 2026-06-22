package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

type specSyncResult struct {
	SchemaVersion     int                        `json:"schema_version"`
	AuthorityBoundary string                     `json:"authority_boundary"`
	SourceOfTruth     string                     `json:"source_of_truth"`
	Imported          []specSyncImportedEntry    `json:"imported"`
	BlockedFindings   []project.SpecCheckFinding `json:"blocked_findings,omitempty"`
}

type specSyncImportedEntry struct {
	SectionID    string               `json:"section_id"`
	SemanticHash string               `json:"semantic_hash"`
	SourceKind   string               `json:"source_kind"`
	CarrierPath  string               `json:"carrier_path,omitempty"`
	Audit        specSyncEditionAudit `json:"audit"`
}

type specSyncEditionAudit struct {
	SourceEpisteme           string `json:"source_episteme"`
	PublicationProjection    string `json:"publication_projection"`
	CarrierBytes             string `json:"carrier_bytes"`
	ImportedSemanticMutation string `json:"imported_semantic_mutation,omitempty"`
	CarrierOnlyDisposition   string `json:"carrier_only_disposition,omitempty"`
	AuthorityBoundary        string `json:"authority_boundary"`
}

func runSpecSync(cmd *cobra.Command, _ []string) error {
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

	specSet, err := project.LoadProjectSpecificationSet(projectRoot)
	if err != nil {
		return err
	}
	result, err := syncProjectSpecificationSetToSQL(projectID, specSet, store)
	if err != nil {
		return err
	}

	if specSyncJSON {
		return writeSpecSyncJSON(cmd.OutOrStdout(), result)
	}
	return writeSpecSyncText(cmd.OutOrStdout(), result)
}

func openSpecSectionEditionStore(projectRoot string, cfg *project.Config) (string, specflow.SpecSectionEditionStore, func(), error) {
	if cfg == nil {
		return "", nil, noopClose, fmt.Errorf("project not initialized — run 'haft init' first")
	}

	dbPath, err := cfg.DBPath()
	if err != nil {
		return "", nil, noopClose, err
	}
	database, err := db.NewStore(dbPath)
	if err != nil {
		return "", nil, noopClose, err
	}

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	return cfg.ID, store, func() { _ = database.Close() }, nil
}

func syncProjectSpecificationSetToSQL(
	projectID string,
	specSet project.ProjectSpecificationSet,
	store specflow.SpecSectionEditionStore,
) (specSyncResult, error) {
	result := specSyncResult{
		SchemaVersion:     1,
		AuthorityBoundary: "typed_spec_section_import_not_approval_rebaseline_or_prose_authority",
		SourceOfTruth:     "sql_project_graph",
		Imported:          []specSyncImportedEntry{},
	}

	if len(specSet.Findings) > 0 {
		result.BlockedFindings = append(result.BlockedFindings, specSet.Findings...)
		return result, fmt.Errorf("spec sync blocked by %d carrier finding(s)", len(specSet.Findings))
	}

	now := time.Now().UTC()
	for _, section := range specSet.Sections {
		edition := specflow.NewSpecSectionEdition(projectID, section, specflow.SpecSectionSourceCarrierImport, now)
		if err := store.PutCurrent(edition); err != nil {
			return result, err
		}
		result.Imported = append(result.Imported, specSyncImportedEntry{
			SectionID:    edition.SectionID,
			SemanticHash: edition.SemanticHash,
			SourceKind:   string(edition.SourceKind),
			CarrierPath:  edition.CarrierPath,
			Audit: specSyncEditionAudit{
				SourceEpisteme:           "sql_spec_section_edition",
				PublicationProjection:    "typed_yaml_spec_section_projection",
				CarrierBytes:             edition.CarrierPath,
				ImportedSemanticMutation: "carrier_import_to_sql_edition",
				AuthorityBoundary:        "not_approval_not_rebaseline_not_evidence",
			},
		})
	}

	return result, nil
}

func writeSpecSyncJSON(writer io.Writer, result specSyncResult) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func writeSpecSyncText(writer io.Writer, result specSyncResult) error {
	if _, err := fmt.Fprintf(writer, "spec sync: %d imported\n", len(result.Imported)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "authority_boundary: %s\n", result.AuthorityBoundary); err != nil {
		return err
	}
	if len(result.Imported) == 0 {
		return nil
	}
	ids := make([]string, 0, len(result.Imported))
	for _, entry := range result.Imported {
		ids = append(ids, entry.SectionID)
	}
	_, err := fmt.Fprintf(writer, "sections: %s\n", strings.Join(ids, ", "))
	return err
}
