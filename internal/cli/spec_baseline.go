package cli

import (
	"errors"
	"os"
	"strings"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

// projectBaseline opens the project's SQLite store and returns a
// BaselineStore + canonical project_id + close function. When the
// project has no .haft/project.yaml or no DB yet, it returns nil
// stores and an empty project ID so callers can degrade to baseline-
// agnostic behavior without erroring.
//
// Callers MUST invoke the returned close function when done; passing
// nil close (returned alongside nil store) is safe and a no-op.
func projectBaseline(projectRoot string) (specflow.BaselineStore, string, func(), error) {
	projectRoot = strings.TrimSpace(projectRoot)
	if projectRoot == "" {
		return nil, "", noopClose, nil
	}

	haftDir := haftDirFor(projectRoot)
	cfg, err := project.Load(haftDir)
	if err != nil {
		return nil, "", noopClose, nil
	}
	if cfg == nil {
		return nil, "", noopClose, nil
	}

	dbPath, err := cfg.DBPath()
	if err != nil {
		return nil, "", noopClose, nil
	}

	if _, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", noopClose, nil
		}
		return nil, "", noopClose, err
	}

	store, err := db.NewStore(dbPath)
	if err != nil {
		return nil, "", noopClose, err
	}

	baselineStore := specflow.NewSQLiteBaselineStore(store.GetRawDB())

	return baselineStore, cfg.ID, func() { _ = store.Close() }, nil
}

func haftDirFor(projectRoot string) string {
	return strings.TrimRight(projectRoot, "/") + "/.haft"
}

func noopClose() {}

// appendSpecBaselineFindings runs SpecSection drift / missing-baseline
// detection over the project's baseline store and folds the findings
// into the existing SpecCheckReport. When the project has no DB yet
// (pre-init / post-onboard but pre-approve) the report is returned
// unchanged.
func appendSpecBaselineFindings(report project.SpecCheckReport, projectRoot string) project.SpecCheckReport {
	store, projectID, closeFn, err := projectBaseline(projectRoot)
	defer closeFn()
	if err != nil || store == nil {
		return report
	}

	specSet, err := project.LoadProjectSpecificationSet(projectRoot)
	if err != nil {
		return report
	}

	driftFindings := specflow.SectionBaselineFindings(specSet, store, projectID)
	if len(driftFindings) == 0 {
		return report
	}

	report.Findings = append(report.Findings, driftFindings...)
	report.Summary.TotalFindings = len(report.Findings)
	return report
}
