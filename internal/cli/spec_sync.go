package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

const (
	specSyncAuthorityBoundary             = "typed_spec_section_import_not_approval_rebaseline_evidence_gate_claim_truth_global_truth_or_prose_authority"
	specSyncEditionAuditAuthorityBoundary = "source_publication_carrier_audit_not_approval_rebaseline_evidence_gate_claim_truth_or_global_truth"
	specSyncScopeFullProject              = "full_project"
	specSyncScopeExactSection             = "exact_section"
)

type specSyncResult struct {
	SchemaVersion     int                        `json:"schema_version"`
	AuthorityBoundary string                     `json:"authority_boundary"`
	SourceOfTruth     string                     `json:"source_of_truth"`
	Scope             string                     `json:"scope"`
	RequestedSection  string                     `json:"requested_section,omitempty"`
	Imported          []specSyncImportedEntry    `json:"imported"`
	BlockedFindings   []project.SpecCheckFinding `json:"blocked_findings,omitempty"`
}

type specSyncScope struct {
	kind      string
	sectionID string
}

func newSpecSyncScope(rawSectionID string) (specSyncScope, error) {
	sectionID := strings.TrimSpace(rawSectionID)
	if sectionID == "" {
		return specSyncScope{kind: specSyncScopeFullProject}, nil
	}
	if strings.ContainsAny(sectionID, " \t\r\n") {
		return specSyncScope{}, fmt.Errorf(
			"spec sync --section must be one exact SpecSection id",
		)
	}
	return specSyncScope{
		kind:      specSyncScopeExactSection,
		sectionID: sectionID,
	}, nil
}

func (scope specSyncScope) valid() bool {
	if scope.kind == specSyncScopeFullProject {
		return scope.sectionID == ""
	}
	if scope.kind == specSyncScopeExactSection {
		return scope.sectionID != ""
	}
	return false
}

func (scope specSyncScope) deletesMissingEditions() bool {
	return scope.kind == specSyncScopeFullProject
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
	scope, err := newSpecSyncScope(specSyncSection)
	if err != nil {
		return err
	}
	result, err := syncProjectSpecificationSetToSQLWithScope(
		projectID,
		specSet,
		store,
		scope,
	)
	if err != nil {
		return err
	}

	if specSyncJSON {
		return writeSpecSyncJSON(cmd.OutOrStdout(), result)
	}
	return writeSpecSyncText(cmd.OutOrStdout(), result)
}

func openSpecSectionEditionStore(projectRoot string, cfg *project.Config) (string, specflow.SpecSectionEditionStore, func(), error) {
	return openSpecSectionEditionStoreWithAccess(
		context.Background(),
		projectRoot,
		cfg,
		projectledger.ReadWrite,
		"SpecSection edition write",
	)
}

func openSpecSectionEditionReadStore(
	ctx context.Context,
	projectRoot string,
	cfg *project.Config,
) (string, specflow.SpecSectionEditionStore, func(), error) {
	return openSpecSectionEditionStoreWithAccess(
		ctx,
		projectRoot,
		cfg,
		projectledger.ReadOnly,
		"SpecSection edition read",
	)
}

func openSpecSectionEditionStoreWithAccess(
	ctx context.Context,
	projectRoot string,
	cfg *project.Config,
	access projectledger.Access,
	operation string,
) (string, specflow.SpecSectionEditionStore, func(), error) {
	if cfg == nil {
		return "", nil, noopClose, fmt.Errorf("project not initialized — run 'haft init' first")
	}

	ledger, err := openCurrentProjectLedger(
		ctx,
		projectRoot,
		access,
		operation,
	)
	if err != nil {
		return "", nil, noopClose, err
	}

	store := specflow.NewSQLiteSpecSectionEditionStore(ledger.Database())
	return ledger.ProjectID().String(), store, func() { _ = ledger.Close() }, nil
}

func syncProjectSpecificationSetToSQLWithScope(
	projectID string,
	specSet project.ProjectSpecificationSet,
	store specflow.SpecSectionEditionStore,
	scope specSyncScope,
) (specSyncResult, error) {
	result := specSyncResult{
		SchemaVersion:     1,
		AuthorityBoundary: specSyncAuthorityBoundary,
		SourceOfTruth:     "sql_project_graph",
		Scope:             scope.kind,
		RequestedSection:  scope.sectionID,
		Imported:          []specSyncImportedEntry{},
	}
	if !scope.valid() {
		return result, fmt.Errorf("spec sync scope is invalid")
	}

	if len(specSet.Findings) > 0 {
		result.BlockedFindings = append(result.BlockedFindings, specSet.Findings...)
		return result, fmt.Errorf("spec sync blocked by %d carrier finding(s)", len(specSet.Findings))
	}

	selectedSections, err := selectSpecSyncSections(specSet.Sections, scope)
	if err != nil {
		return result, err
	}

	now := time.Now().UTC()
	currentSectionIDs := make(map[string]bool, len(selectedSections))
	for _, section := range selectedSections {
		edition := specflow.NewSpecSectionEdition(projectID, section, specflow.SpecSectionSourceCarrierImport, now)
		if err := store.PutCurrent(edition); err != nil {
			return result, err
		}
		currentSectionIDs[edition.SectionID] = true
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
				AuthorityBoundary:        specSyncEditionAuditAuthorityBoundary,
			},
		})
	}

	if !scope.deletesMissingEditions() {
		return result, nil
	}
	currentEditions, err := store.ListCurrent(projectID)
	if err != nil {
		return result, err
	}
	for _, edition := range currentEditions {
		if currentSectionIDs[edition.SectionID] {
			continue
		}
		if err := store.DeleteCurrent(projectID, edition.SectionID); err != nil {
			return result, err
		}
	}

	return result, nil
}

func selectSpecSyncSections(
	sections []project.SpecSection,
	scope specSyncScope,
) ([]project.SpecSection, error) {
	if scope.kind == specSyncScopeFullProject {
		return append([]project.SpecSection{}, sections...), nil
	}
	for _, section := range sections {
		if section.ID == scope.sectionID {
			return []project.SpecSection{section}, nil
		}
	}
	return nil, fmt.Errorf(
		"spec sync section %s is not present in typed carriers",
		scope.sectionID,
	)
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
