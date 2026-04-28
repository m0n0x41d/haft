package cli

import (
	"errors"
	"os"
	"strings"
	"time"

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

// appendSpecHealthFindings runs SpecSection drift / missing-baseline
// detection AND time-based staleness detection over the project's
// baseline store + carriers, folding both finding kinds into the
// existing SpecCheckReport. When the project has no DB yet, drift
// findings are skipped but staleness findings still run because
// staleness only needs the carrier set + current time.
//
// Per dec-20260428-spec-enforcement-hardening-219a58b5: this is the
// single source of truth for spec-health rollups consumed by both
// `haft spec check` (CLI) and `haft_query(action="check")` (MCP).
func appendSpecHealthFindings(report project.SpecCheckReport, projectRoot string) project.SpecCheckReport {
	specSet, specErr := project.LoadProjectSpecificationSet(projectRoot)
	if specErr != nil {
		return report
	}

	store, projectID, closeFn, _ := projectBaseline(projectRoot)
	defer closeFn()

	driftFindings := specflow.SectionBaselineFindings(specSet, store, projectID)
	staleFindings := specflow.SectionStalenessFindings(specSet, time.Now().UTC())

	if len(driftFindings) == 0 && len(staleFindings) == 0 {
		return report
	}

	report.Findings = append(report.Findings, driftFindings...)
	report.Findings = append(report.Findings, staleFindings...)
	report.Summary.TotalFindings = len(report.Findings)
	return report
}

// appendSpecBaselineFindings is preserved as the previous slice's name
// for backwards compatibility within this package; new callers should
// use appendSpecHealthFindings to get drift + stale together.
func appendSpecBaselineFindings(report project.SpecCheckReport, projectRoot string) project.SpecCheckReport {
	return appendSpecHealthFindings(report, projectRoot)
}
