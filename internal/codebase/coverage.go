package codebase

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/reff"
)

// CoverageStatus represents how well a module is governed by decisions.
type CoverageStatus string

const (
	CoverageCovered CoverageStatus = "covered" // ≥1 active decision covers files in this module
	CoveragePartial CoverageStatus = "partial" // has decisions but they're stale or low R_eff
	CoverageBlind   CoverageStatus = "blind"   // no decisions reference files in this module
)

// ModuleCoverage describes the decision coverage for a single module.
type ModuleCoverage struct {
	Module        Module
	Status        CoverageStatus
	DecisionCount int
	DecisionIDs   []string

	// ImpactScore is the number of governed (Covered or Partial) modules
	// that depend on this module — i.e., how many decision-covered places
	// would be at risk if this module changes. Computed in ComputeCoverage
	// via Scanner.GetDependents reverse-graph traversal (dec-20260527-e4b86938).
	// Zero means no governed dependents (isolated utility, low priority).
	ImpactScore int
}

// CoverageReport is the full coverage report for a project.
type CoverageReport struct {
	TotalModules int
	CoveredCount int
	PartialCount int
	BlindCount   int
	Modules      []ModuleCoverage
}

// ModuleGovernanceGap reports that a module touched by a new decision has no
// prior active decision coverage.
type ModuleGovernanceGap struct {
	Module Module
	Files  []string
}

// ComputeCoverage calculates decision coverage for all modules.
// It joins codebase_modules with affected_files via path prefix matching.
func ComputeCoverage(ctx context.Context, db *sql.DB) (*CoverageReport, error) {
	// Get all modules
	scanner := NewScanner(db)
	modules, err := scanner.GetModules(ctx)
	if err != nil {
		return nil, fmt.Errorf("get modules: %w", err)
	}

	if len(modules) == 0 {
		return &CoverageReport{}, nil
	}

	// Get all affected_files from active decisions and notes
	// Only DecisionRecords count as governance — Notes are descriptive, not architectural contracts
	rows, err := db.QueryContext(ctx, `
		SELECT af.file_path, a.id
		FROM affected_files af
		JOIN artifacts a ON a.id = af.artifact_id
		WHERE a.status = 'active'
		  AND a.kind = 'DecisionRecord'
		ORDER BY af.file_path`)
	if err != nil {
		return nil, fmt.Errorf("query affected files: %w", err)
	}
	defer rows.Close()

	// Build map: file_path -> list of decision IDs
	fileDecisions := make(map[string][]string)
	for rows.Next() {
		var filePath, decID string
		if err := rows.Scan(&filePath, &decID); err != nil {
			continue
		}
		fileDecisions[filePath] = append(fileDecisions[filePath], decID)
	}

	// For each module, check if any affected_file falls within its path
	report := &CoverageReport{TotalModules: len(modules)}

	for _, m := range modules {
		mc := ModuleCoverage{Module: m}
		decisionSet := make(map[string]bool)

		for filePath, decIDs := range fileDecisions {
			if isFileInModule(filePath, m.Path) {
				for _, id := range decIDs {
					decisionSet[id] = true
				}
			}
		}

		mc.DecisionCount = len(decisionSet)
		for id := range decisionSet {
			mc.DecisionIDs = append(mc.DecisionIDs, id)
		}

		if mc.DecisionCount == 0 {
			mc.Status = CoverageBlind
			report.BlindCount++
		} else {
			// Check R_eff for each decision — module status = best among its decisions
			bestREff := -1.0 // -1 means "no evidence on any decision"
			for id := range decisionSet {
				rEff, hasEvidence := computeDecisionREff(ctx, db, id)
				if !hasEvidence {
					// Fresh decision without evidence — counts as healthy
					bestREff = 1.0
					break
				}
				bestREff = math.Max(bestREff, rEff)
			}

			if bestREff < 0 || bestREff >= 0.5 {
				mc.Status = CoverageCovered
				report.CoveredCount++
			} else {
				mc.Status = CoveragePartial
				report.PartialCount++
			}
		}

		report.Modules = append(report.Modules, mc)
	}

	// Impact-ranking pass (dec-20260527-e4b86938): for each module count
	// governed dependents — how many decision-covered modules would be
	// affected if this module changes. Reverse-graph query via
	// Scanner.GetDependents (same data source EnrichDriftWithImpact uses
	// in forward direction). Time-bounded fallback: if compute exceeds
	// the deadline, leave remaining ImpactScore at zero and continue;
	// callers see partial ranking, not a stall.
	governedModuleIDs := make(map[string]bool, len(report.Modules))
	for _, mc := range report.Modules {
		if mc.Status == CoverageCovered || mc.Status == CoveragePartial {
			governedModuleIDs[mc.Module.ID] = true
		}
	}
	impactDeadline := time.Now().Add(500 * time.Millisecond)
	for i := range report.Modules {
		if time.Now().After(impactDeadline) {
			// Time guard per DRR weakest_link mitigation — abandon
			// remaining impact compute, return what we have.
			break
		}
		deps, err := scanner.GetDependents(ctx, report.Modules[i].Module.ID)
		if err != nil {
			continue
		}
		for _, dep := range deps {
			if governedModuleIDs[dep] {
				report.Modules[i].ImpactScore++
			}
		}
	}

	return report, nil
}

// FormatCoverageResponse formats the coverage report for MCP output.
func FormatCoverageResponse(report *CoverageReport) string {
	if report.TotalModules == 0 {
		return "No modules detected. Run module scan first.\n"
	}

	var sb strings.Builder

	pct := 0
	if report.TotalModules > 0 {
		pct = (report.CoveredCount + report.PartialCount) * 100 / report.TotalModules
	}
	header := fmt.Sprintf("## Module Coverage (%d modules, %d%% governed", report.TotalModules, pct)
	if report.PartialCount > 0 {
		header += fmt.Sprintf(", %d degraded", report.PartialCount)
	}
	header += ")\n\n"
	sb.WriteString(header)

	// Covered first, then partial, then blind. Within each tier sort by
	// ImpactScore descending then path ascending (dec-20260527-e4b86938
	// V1 — impact-ranked spec coverage): hottest items at top so operator
	// reading the list sees governance-critical modules first.
	for _, status := range []CoverageStatus{CoverageCovered, CoveragePartial, CoverageBlind} {
		// Collect modules in this tier and sort.
		var tier []ModuleCoverage
		for _, mc := range report.Modules {
			if mc.Status == status {
				tier = append(tier, mc)
			}
		}
		sort.Slice(tier, func(i, j int) bool {
			if tier[i].ImpactScore != tier[j].ImpactScore {
				return tier[i].ImpactScore > tier[j].ImpactScore
			}
			return tier[i].Module.Path < tier[j].Module.Path
		})

		for _, mc := range tier {
			path := mc.Module.Path
			if path == "" {
				path = "(root)"
			}
			impactTag := ""
			if mc.ImpactScore > 0 {
				impactTag = fmt.Sprintf(", impact: %d", mc.ImpactScore)
			}

			switch mc.Status {
			case CoverageCovered:
				sb.WriteString(fmt.Sprintf("  ✓ %-30s — %d decision(s)%s [%s]\n",
					path, mc.DecisionCount, impactTag, mc.Module.Lang))
			case CoveragePartial:
				sb.WriteString(fmt.Sprintf("  ~ %-30s — %d decision(s), stale%s [%s]\n",
					path, mc.DecisionCount, impactTag, mc.Module.Lang))
			case CoverageBlind:
				sb.WriteString(fmt.Sprintf("  ✗ %-30s — no decisions (blind)%s [%s]\n",
					path, impactTag, mc.Module.Lang))
			}
		}
	}

	return sb.String()
}

// FormatCoverageSummary formats a compact module coverage projection for
// default status output. The full module list remains available through
// FormatCoverageResponse and haft_query(action="status", full=true).
func FormatCoverageSummary(report *CoverageReport) string {
	if report.TotalModules == 0 {
		return "No modules detected. Run module scan first.\n"
	}

	var sb strings.Builder

	pct := 0
	if report.TotalModules > 0 {
		pct = (report.CoveredCount + report.PartialCount) * 100 / report.TotalModules
	}

	header := fmt.Sprintf("## Module Coverage Summary (%d modules, %d%% governed", report.TotalModules, pct)
	if report.PartialCount > 0 {
		header += fmt.Sprintf(", %d degraded", report.PartialCount)
	}
	header += ") — orientation cue, not a target\n\n"
	sb.WriteString(header)

	const topModulesPerTier = 5
	for _, status := range []CoverageStatus{CoverageBlind, CoveragePartial, CoverageCovered} {
		tier := sortedCoverageTier(report, status)
		if len(tier) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("### %s (%d)\n", coverageStatusLabel(status), len(tier)))
		for i, mc := range tier {
			if i >= topModulesPerTier {
				break
			}
			sb.WriteString(formatCoverageModuleLine(mc))
		}
		if omitted := len(tier) - topModulesPerTier; omitted > 0 {
			sb.WriteString(fmt.Sprintf("  ... and %d more module(s)\n", omitted))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Full module list: `haft_query(action=\"status\", full=true)` or `haft_query(action=\"coverage\")`.\n")

	return sb.String()
}

func sortedCoverageTier(report *CoverageReport, status CoverageStatus) []ModuleCoverage {
	var tier []ModuleCoverage
	for _, mc := range report.Modules {
		if mc.Status == status {
			tier = append(tier, mc)
		}
	}

	sort.Slice(tier, func(i, j int) bool {
		if tier[i].ImpactScore != tier[j].ImpactScore {
			return tier[i].ImpactScore > tier[j].ImpactScore
		}
		return tier[i].Module.Path < tier[j].Module.Path
	})

	return tier
}

func coverageStatusLabel(status CoverageStatus) string {
	switch status {
	case CoverageCovered:
		return "Covered"
	case CoveragePartial:
		return "Degraded"
	case CoverageBlind:
		return "Blind"
	default:
		return string(status)
	}
}

func formatCoverageModuleLine(mc ModuleCoverage) string {
	path := mc.Module.Path
	if path == "" {
		path = "(root)"
	}

	impactTag := ""
	if mc.ImpactScore > 0 {
		impactTag = fmt.Sprintf(", impact: %d", mc.ImpactScore)
	}

	switch mc.Status {
	case CoverageCovered:
		return fmt.Sprintf("  ✓ %-30s — %d decision(s)%s [%s]\n", path, mc.DecisionCount, impactTag, mc.Module.Lang)
	case CoveragePartial:
		return fmt.Sprintf("  ~ %-30s — %d decision(s), stale%s [%s]\n", path, mc.DecisionCount, impactTag, mc.Module.Lang)
	case CoverageBlind:
		return fmt.Sprintf("  ✗ %-30s — no decisions (blind)%s [%s]\n", path, impactTag, mc.Module.Lang)
	default:
		return fmt.Sprintf("  ? %-30s — %s%s [%s]\n", path, mc.Status, impactTag, mc.Module.Lang)
	}
}

// FindFirstDecisionModules returns touched modules that currently have no
// active decision coverage. The caller can use this to warn that a decision is
// establishing the first explicit architectural context for a module.
func FindFirstDecisionModules(ctx context.Context, db *sql.DB, affectedFiles []string) ([]ModuleGovernanceGap, error) {
	if len(affectedFiles) == 0 {
		return nil, nil
	}

	report, err := ComputeCoverage(ctx, db)
	if err != nil {
		return nil, err
	}
	if report == nil || report.TotalModules == 0 {
		return nil, nil
	}

	scanner := NewScanner(db)
	moduleFiles := make(map[string]map[string]struct{})
	moduleCoverage := make(map[string]ModuleCoverage, len(report.Modules))
	moduleIndex := make(map[string]Module, len(report.Modules))

	for _, coverage := range report.Modules {
		moduleCoverage[coverage.Module.ID] = coverage
		moduleIndex[coverage.Module.ID] = coverage.Module
	}

	for _, filePath := range affectedFiles {
		moduleID, err := scanner.ResolveFileToModule(ctx, filePath)
		if err != nil || moduleID == "" {
			continue
		}

		if _, ok := moduleFiles[moduleID]; !ok {
			moduleFiles[moduleID] = make(map[string]struct{})
		}
		moduleFiles[moduleID][filePath] = struct{}{}
	}

	gaps := make([]ModuleGovernanceGap, 0, len(moduleFiles))

	for moduleID, files := range moduleFiles {
		coverage, ok := moduleCoverage[moduleID]
		if ok && coverage.Status != CoverageBlind && coverage.DecisionCount > 0 {
			continue
		}

		module := moduleIndex[moduleID]
		fileList := make([]string, 0, len(files))
		for filePath := range files {
			fileList = append(fileList, filePath)
		}
		sort.Strings(fileList)

		gaps = append(gaps, ModuleGovernanceGap{
			Module: module,
			Files:  fileList,
		})
	}

	sort.Slice(gaps, func(i, j int) bool {
		return gaps[i].Module.Path < gaps[j].Module.Path
	})

	return gaps, nil
}

// EnrichDriftWithImpact adds dependency propagation to drift reports.
// For each drifted file, resolves to a module, finds dependents, and looks up their decisions.
func EnrichDriftWithImpact(ctx context.Context, db *sql.DB, driftFiles []string) ([]ModuleImpactInfo, error) {
	scanner := NewScanner(db)
	modules, err := scanner.GetModules(ctx)
	if err != nil || len(modules) == 0 {
		return nil, nil
	}

	// Resolve drifted files to modules
	driftedModuleIDs := make(map[string]bool)
	for _, filePath := range driftFiles {
		modID, _ := scanner.ResolveFileToModule(ctx, filePath)
		if modID != "" {
			driftedModuleIDs[modID] = true
		}
	}

	if len(driftedModuleIDs) == 0 {
		return nil, nil
	}

	// For each drifted module, find dependents (1-hop)
	impactedModuleIDs := make(map[string]bool)
	for modID := range driftedModuleIDs {
		deps, _ := scanner.GetDependents(ctx, modID)
		for _, dep := range deps {
			if !driftedModuleIDs[dep] {
				impactedModuleIDs[dep] = true
			}
		}
	}

	if len(impactedModuleIDs) == 0 {
		return nil, nil
	}

	// Build module path lookup
	modPaths := make(map[string]string)
	for _, m := range modules {
		modPaths[m.ID] = m.Path
	}

	// Get coverage to find which impacted modules have decisions
	report, _ := ComputeCoverage(ctx, db)
	coverageMap := make(map[string]*ModuleCoverage)
	if report != nil {
		for i := range report.Modules {
			coverageMap[report.Modules[i].Module.ID] = &report.Modules[i]
		}
	}

	var impacts []ModuleImpactInfo
	for modID := range impactedModuleIDs {
		impact := ModuleImpactInfo{
			ModuleID:   modID,
			ModulePath: modPaths[modID],
		}
		if mc, ok := coverageMap[modID]; ok {
			impact.DecisionIDs = mc.DecisionIDs
			impact.IsBlind = mc.Status == CoverageBlind
		} else {
			impact.IsBlind = true
		}
		impacts = append(impacts, impact)
	}

	return impacts, nil
}

// ModuleImpactInfo describes a module affected by dependency propagation.
type ModuleImpactInfo struct {
	ModuleID    string
	ModulePath  string
	DecisionIDs []string
	IsBlind     bool
}

// computeDecisionREff computes R_eff for a decision by querying evidence_items directly.
// Returns (rEff, hasEvidence). Same algorithm as artifact.ComputeWLNKSummary but without
// cross-package dependency — queries the DB directly.
// R_eff = min(effective_score) where:
//   - base: supports=1.0, weakens=0.5, refutes=0.0
//   - CL penalty: CL3=0, CL2=0.1, CL1=0.4, CL0=0.9
//   - expired evidence scores 0.1
func computeDecisionREff(ctx context.Context, db *sql.DB, decisionID string) (float64, bool) {
	rows, err := db.QueryContext(ctx,
		`SELECT verdict, congruence_level, valid_until FROM evidence_items WHERE artifact_ref = ? AND verdict != 'superseded'`,
		decisionID)
	if err != nil {
		return 0, false
	}
	defer rows.Close()

	now := time.Now().UTC()
	minScore := 2.0 // sentinel > 1.0
	count := 0

	for rows.Next() {
		var verdict string
		var cl int
		var validUntil sql.NullString

		if err := rows.Scan(&verdict, &cl, &validUntil); err != nil {
			continue
		}
		count++

		vu := ""
		if validUntil.Valid {
			vu = validUntil.String
		}
		score := reff.ScoreEvidence(verdict, cl, vu, now)
		if score < minScore {
			minScore = score
		}
	}

	if count == 0 {
		return 0, false
	}
	if minScore > 1.0 {
		minScore = 1.0
	}

	return minScore, true
}

// isFileInModule checks if a file path belongs to a module's directory.
func isFileInModule(filePath, modulePath string) bool {
	if modulePath == "" {
		// Root module covers all files in the project
		return true
	}
	return strings.HasPrefix(filePath, modulePath+"/") || filePath == modulePath
}
