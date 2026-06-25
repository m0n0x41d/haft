package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

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

func checkProjectSpecificationSetSQLFirst(projectRoot string) (project.SpecCheckReport, error) {
	specSet, err := loadProjectSpecificationSetSQLFirst(projectRoot)
	if err != nil {
		return project.SpecCheckReport{}, err
	}

	return project.SpecCheckReportFromSpecificationSet(specSet), nil
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

	return merged
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
	cfg, err := project.Load(haftDirFor(projectRoot))
	if err != nil {
		return project.ProjectSpecificationSet{}, false, err
	}
	if cfg == nil {
		return project.ProjectSpecificationSet{}, false, nil
	}

	projectID, store, closeStore, err := openSpecSectionEditionStore(projectRoot, cfg)
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

	specSet, err := specflow.ProjectSpecificationSetFromEditions(editions)
	if err != nil {
		return project.ProjectSpecificationSet{}, true, fmt.Errorf("project specification SQL edition projection: %w", err)
	}

	return specSet, true, nil
}
