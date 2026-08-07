package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func loadProjectSpecificationSetSQLFirstFromCanonicalProfile(
	ctx context.Context,
	projectRoot string,
	request projectSpecificationScopeRequest,
) (
	project.ProjectSpecificationSet,
	projectSpecificationApplicabilityResolution,
	error,
) {
	resolution, err := resolveCanonicalProjectSpecificationApplicability(
		ctx,
		projectRoot,
		request,
	)
	if err != nil {
		return project.ProjectSpecificationSet{},
			projectSpecificationApplicabilityResolution{},
			err
	}
	applicability, _, resolved := resolution.Resolved()
	if !resolved {
		return project.ProjectSpecificationSet{}, resolution, nil
	}
	canonicalRoot := resolution.ProjectRoot().String()
	specSet, err := loadProjectSpecificationSetSQLFirstForScope(
		canonicalRoot,
		applicability,
	)
	return specSet, resolution, err
}

func loadProjectSpecificationSetSQLFirst(projectRoot string) (project.ProjectSpecificationSet, error) {
	sqlSpecSet, hasSQLSource, err := loadProjectSpecificationSetFromSQLEditions(projectRoot)
	if err != nil {
		return project.ProjectSpecificationSet{}, err
	}
	if hasSQLSource {
		carrierSpecSet, carrierErr := project.LoadProjectSpecificationSet(projectRoot)
		if carrierErr != nil {
			return sqlSpecSet, nil
		}

		return mergeSQLSpecSetWithCarrierSupport(sqlSpecSet, carrierSpecSet), nil
	}

	return project.LoadProjectSpecificationSet(projectRoot)
}

func loadProjectSpecificationSetSQLFirstForScope(
	projectRoot string,
	applicability project.ProjectSpecificationSetApplicability,
) (project.ProjectSpecificationSet, error) {
	sqlSpecSet, hasSQLSource, err := loadProjectSpecificationSetFromSQLEditionsForScope(
		projectRoot,
		applicability,
	)
	if err != nil {
		return project.ProjectSpecificationSet{}, err
	}
	if hasSQLSource {
		carrierSpecSet, carrierErr := project.LoadProjectSpecificationSetForScope(
			projectRoot,
			applicability,
		)
		if carrierErr != nil {
			return sqlSpecSet, nil
		}

		return mergeSQLSpecSetWithCarrierSupport(sqlSpecSet, carrierSpecSet), nil
	}

	return project.LoadProjectSpecificationSetForScope(
		projectRoot,
		applicability,
	)
}

func mergeSQLSpecSetWithCarrierSupport(
	sqlSpecSet project.ProjectSpecificationSet,
	carrierSpecSet project.ProjectSpecificationSet,
) project.ProjectSpecificationSet {
	termMapPaths := termMapDocumentPaths(carrierSpecSet.Documents)

	merged := sqlSpecSet
	merged.Documents = append(append([]project.SpecDocument{}, sqlSpecSet.Documents...), termMapDocuments(carrierSpecSet.Documents)...)
	merged.TermMapEntries = append([]project.TermMapEntry{}, carrierSpecSet.TermMapEntries...)
	merged.Findings = append(append([]project.SpecCheckFinding{}, sqlSpecSet.Findings...), termMapFindings(carrierSpecSet.Findings, termMapPaths)...)
	merged.Findings = append(merged.Findings, specMigrationRequiredFindings(carrierSpecSet.Findings)...)

	return merged
}

func specMigrationRequiredFindings(findings []project.SpecCheckFinding) []project.SpecCheckFinding {
	out := []project.SpecCheckFinding{}
	for _, finding := range findings {
		if finding.Code != project.SpecMigrationRequiredFindingCode {
			continue
		}
		out = append(out, finding)
	}
	return out
}

func termMapDocuments(documents []project.SpecDocument) []project.SpecDocument {
	out := []project.SpecDocument{}
	for _, document := range documents {
		if document.Kind != project.SpecDocumentKindTermMap {
			continue
		}
		out = append(out, document)
	}
	return out
}

func termMapDocumentPaths(documents []project.SpecDocument) map[string]bool {
	paths := map[string]bool{}
	for _, document := range termMapDocuments(documents) {
		path := strings.TrimSpace(document.Path)
		if path == "" {
			continue
		}
		paths[path] = true
	}
	return paths
}

func termMapFindings(findings []project.SpecCheckFinding, termMapPaths map[string]bool) []project.SpecCheckFinding {
	out := []project.SpecCheckFinding{}
	for _, finding := range findings {
		path := strings.TrimSpace(finding.Path)
		if path == "" || !termMapPaths[path] {
			continue
		}
		out = append(out, finding)
	}
	return out
}

func loadProjectSpecificationSetFromSQLEditions(projectRoot string) (project.ProjectSpecificationSet, bool, error) {
	return loadProjectSpecificationSetFromSQLEditionsWith(
		projectRoot,
		specflow.ProjectSpecificationSetFromEditions,
	)
}

func loadProjectSpecificationSetFromSQLEditionsForScope(
	projectRoot string,
	applicability project.ProjectSpecificationSetApplicability,
) (project.ProjectSpecificationSet, bool, error) {
	projector := func(
		editions []specflow.SpecSectionEdition,
	) (project.ProjectSpecificationSet, error) {
		return specflow.ProjectSpecificationSetFromEditionsForScope(
			editions,
			applicability,
		)
	}
	return loadProjectSpecificationSetFromSQLEditionsWith(
		projectRoot,
		projector,
	)
}

func loadProjectSpecificationSetFromSQLEditionsWith(
	projectRoot string,
	projector func([]specflow.SpecSectionEdition) (project.ProjectSpecificationSet, error),
) (project.ProjectSpecificationSet, bool, error) {
	cfg, err := project.Load(haftDirFor(projectRoot))
	if err != nil {
		return project.ProjectSpecificationSet{}, false, err
	}
	if cfg == nil {
		return project.ProjectSpecificationSet{}, false, nil
	}

	projectID, store, closeStore, err := openSpecSectionEditionReadStore(
		context.Background(),
		projectRoot,
		cfg,
	)
	if err != nil {
		return project.ProjectSpecificationSet{}, false, err
	}
	defer closeStore()

	editions, err := store.ListCurrent(projectID)
	if err != nil {
		if errors.Is(err, specflow.ErrSpecSectionEditionNotFound) {
			return project.ProjectSpecificationSet{}, false, nil
		}
		return project.ProjectSpecificationSet{}, false, fmt.Errorf("read SQL spec section editions: %w", err)
	}
	if len(editions) == 0 {
		return project.ProjectSpecificationSet{}, false, nil
	}

	specSet, err := projector(editions)
	if err != nil {
		return project.ProjectSpecificationSet{}, true, fmt.Errorf("project specification SQL edition projection: %w", err)
	}

	return specSet, true, nil
}
