package codebase

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/m0n0x41d/quint-code/internal/reff"
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
}

// CoverageReport is the full coverage report for a project.
type CoverageReport struct {
	TotalModules   int
	CoveredCount   int
	PartialCount   int
	BlindCount     int
	Modules        []ModuleCoverage
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

	// Covered first, then partial, then blind
	for _, status := range []CoverageStatus{CoverageCovered, CoveragePartial, CoverageBlind} {
		for _, mc := range report.Modules {
			if mc.Status != status {
				continue
			}
			path := mc.Module.Path
			if path == "" {
				path = "(root)"
			}

			switch mc.Status {
			case CoverageCovered:
				sb.WriteString(fmt.Sprintf("  ✓ %-30s — %d decision(s) [%s]\n",
					path, mc.DecisionCount, mc.Module.Lang))
			case CoveragePartial:
				sb.WriteString(fmt.Sprintf("  ~ %-30s — %d decision(s), stale [%s]\n",
					path, mc.DecisionCount, mc.Module.Lang))
			case CoverageBlind:
				sb.WriteString(fmt.Sprintf("  ✗ %-30s — no decisions (blind) [%s]\n",
					path, mc.Module.Lang))
			}
		}
	}

	return sb.String()
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
		// Root module — check if file is directly in root (no subdirectory)
		return !strings.Contains(filePath, "/")
	}
	return strings.HasPrefix(filePath, modulePath+"/") || filePath == modulePath
}
