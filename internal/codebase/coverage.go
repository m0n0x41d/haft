package codebase

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/governance"
	"github.com/m0n0x41d/haft/internal/projectpath"
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
	FileGaps     FileGapProjection
}

// FileGapIndexState reports whether the derived code index is safe to use for
// exact file-link gap claims. An unavailable projection is never equivalent to
// an empty gap set.
type FileGapIndexState string

const (
	FileGapIndexUninitialized FileGapIndexState = "uninitialized"
	FileGapIndexCurrent       FileGapIndexState = "current"
	FileGapIndexStale         FileGapIndexState = "stale"
	FileGapIndexDegraded      FileGapIndexState = "degraded"
	FileGapIndexPartial       FileGapIndexState = "partial"
)

// FileDecisionLinkGap is an indexed source file inside a module that already
// has at least one active DecisionRecord, but has no exact active
// affected_files link of its own. It is an orientation cue, not proof that the
// file is undocumented, unconstrained, or incorrect.
type FileDecisionLinkGap struct {
	FilePath   string
	ModuleID   string
	ModulePath string
}

// FileGapProjection is a bounded read-only projection over the current code
// index. TotalGaps counts the full result; Gaps contains at most the requested
// projection limit.
type FileGapProjection struct {
	IndexState      FileGapIndexState
	Reason          string
	IndexEpoch      int64
	IndexBasisRef   string
	CoveragePosture string
	IndexedFiles    int
	TotalGaps       int
	OmittedGaps     int
	ProjectionLimit int
	Gaps            []FileDecisionLinkGap
}

const defaultFileGapProjectionLimit = 50
const maxFileGapProjectionLimit = 200

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

	// Only current DecisionRecords with module-scoped authority count as module
	// governance. Exact decisions stay exact; implementation footprints stay
	// descriptive. Typed module targets are authority in their own right and do
	// not depend on a redundant affected_files backlink.
	rows, err := db.QueryContext(ctx, `
		SELECT a.id, COALESCE(a.structured_data, '{}'),
		       COALESCE(af.file_path, '')
		FROM artifacts a
		LEFT JOIN affected_files af ON a.id = af.artifact_id
		WHERE a.status IN ('active', 'refresh_due')
		  AND a.kind = 'DecisionRecord'
		ORDER BY a.id, af.file_path`)
	if err != nil {
		return nil, fmt.Errorf("query affected files: %w", err)
	}
	defer rows.Close()

	type coverageDecision struct {
		structuredData string
		affectedPaths  []string
	}
	decisions := make(map[string]coverageDecision)
	for rows.Next() {
		var decID, structuredData, filePath string
		if err := rows.Scan(&decID, &structuredData, &filePath); err != nil {
			return nil, err
		}
		decision := decisions[decID]
		decision.structuredData = structuredData
		if strings.TrimSpace(filePath) != "" {
			decision.affectedPaths = append(
				decision.affectedPaths,
				filePath,
			)
		}
		decisions[decID] = decision
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	indexedFiles, _, err := coverageIndexedSourcePaths(
		ctx,
		db,
	)
	if err != nil {
		return nil, err
	}
	moduleDecisions := make(map[string]map[string]bool)
	addModuleDecision := func(moduleID string, decisionID string) {
		decisionSet := moduleDecisions[moduleID]
		if decisionSet == nil {
			decisionSet = make(map[string]bool)
			moduleDecisions[moduleID] = decisionSet
		}
		decisionSet[decisionID] = true
	}
	for decID, decision := range decisions {
		policy, err := governance.ParseDecisionPathPolicy(
			decision.structuredData,
		)
		if err != nil {
			continue
		}
		for _, target := range policy.ModuleTargets() {
			for _, module := range modules {
				modulePath, err := projectpath.ParseModule(module.Path)
				if err != nil ||
					modulePath.String() != target.String() {
					continue
				}
				addModuleDecision(module.ID, decID)
			}
		}
		for _, rawPath := range decision.affectedPaths {
			canonical, err := projectpath.Parse(rawPath)
			if err != nil {
				continue
			}
			module, ok := mostSpecificModuleForPath(
				modules,
				canonical.String(),
			)
			if !ok {
				continue
			}
			modulePath, err := projectpath.ParseModule(module.Path)
			if err != nil {
				continue
			}
			if !policy.AllowsAffectedPathModuleContext(
				rawPath,
				canonical,
				modulePath,
				indexedFiles[canonical.String()],
			) {
				continue
			}
			addModuleDecision(module.ID, decID)
		}
	}

	report := &CoverageReport{TotalModules: len(modules)}

	for _, m := range modules {
		mc := ModuleCoverage{Module: m}
		decisionSet := moduleDecisions[m.ID]

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

// ComputeCoverageWithFileGaps adds a bounded exact-file projection only when
// the existing derived code index matches the current source tree. It performs
// no scans and no writes; callers must run an explicit refresh to publish a new
// index before relying on stale or uninitialized results.
func ComputeCoverageWithFileGaps(
	ctx context.Context,
	db *sql.DB,
	projectRoot string,
	requestedLimit int,
) (*CoverageReport, error) {
	report, err := ComputeCoverage(ctx, db)
	if err != nil {
		return nil, err
	}

	projection, err := projectCurrentFileDecisionLinkGaps(
		ctx,
		db,
		projectRoot,
		report.Modules,
		normalizeFileGapProjectionLimit(requestedLimit),
	)
	if err != nil {
		return nil, err
	}
	report.FileGaps = projection
	return report, nil
}

type coverageIndexMeta struct {
	Fingerprint    string
	Epoch          int64
	Degraded       bool
	DegradedReason string
}

func projectCurrentFileDecisionLinkGaps(
	ctx context.Context,
	db *sql.DB,
	projectRoot string,
	modules []ModuleCoverage,
	limit int,
) (FileGapProjection, error) {
	projection := FileGapProjection{
		IndexState:      FileGapIndexUninitialized,
		ProjectionLimit: limit,
	}

	present, err := coverageTableExists(ctx, db, "code_files")
	if err != nil {
		return projection, err
	}
	if !present {
		projection.Reason = "derived code index has not been initialized"
		return projection, nil
	}

	meta, found, err := readCoverageIndexMeta(ctx, db)
	if err != nil {
		projection.Reason = "derived code index metadata is unreadable: " + err.Error()
		return projection, nil
	}
	if !found || meta.Epoch <= 0 || strings.TrimSpace(meta.Fingerprint) == "" {
		projection.Reason = "derived code index has no published epoch"
		return projection, nil
	}
	projection.IndexEpoch = meta.Epoch
	if meta.Degraded {
		projection.IndexState = FileGapIndexDegraded
		projection.Reason = strings.TrimSpace(meta.DegradedReason)
		if projection.Reason == "" {
			projection.Reason = "the last code-index refresh degraded"
		}
		return projection, nil
	}
	indexState, err := NewScanner(db).CurrentIndexState(ctx)
	if err != nil {
		projection.IndexState = FileGapIndexDegraded
		projection.Reason = "exact code-index basis is unreadable: " +
			err.Error()
		return projection, nil
	}
	projection.IndexBasisRef = indexState.Basis.CoverageRef()
	projection.CoveragePosture = indexState.Basis.Coverage.Posture
	if !indexState.SupportsKnownAbsence() {
		projection.IndexState = FileGapIndexPartial
		projection.Reason = fmt.Sprintf(
			"code-index coverage is %s under %s",
			indexState.Basis.Coverage.Posture,
			indexState.Basis.CoverageRef(),
		)
		return projection, nil
	}

	currentFingerprint, err := NewScanner(db).SourceFingerprint(projectRoot)
	if err != nil {
		projection.IndexState = FileGapIndexStale
		projection.Reason = "current source fingerprint is unavailable: " + err.Error()
		return projection, nil
	}
	if currentFingerprint != meta.Fingerprint {
		projection.IndexState = FileGapIndexStale
		projection.Reason = "current source tree differs from the published code index"
		return projection, nil
	}
	if len(modules) == 0 {
		projection.Reason = "module index is unavailable for file-to-module projection"
		return projection, nil
	}

	files, err := currentIndexedFiles(ctx, db)
	if err != nil {
		return projection, err
	}
	projection.IndexedFiles = len(files)
	if len(files) == 0 {
		projection.Reason = "published code index contains no source files"
		return projection, nil
	}

	linkedFiles, err := activeDecisionLinkedFiles(ctx, db)
	if err != nil {
		return projection, err
	}
	gaps := make([]FileDecisionLinkGap, 0)
	for _, filePath := range files {
		module, ok := mostSpecificCoverageModule(modules, filePath)
		if !ok || module.DecisionCount == 0 || module.Status == CoverageBlind {
			continue
		}
		if linkedFiles[filePath] {
			continue
		}
		projection.TotalGaps++
		if len(gaps) >= limit {
			continue
		}
		gaps = append(gaps, FileDecisionLinkGap{
			FilePath:   filePath,
			ModuleID:   module.Module.ID,
			ModulePath: module.Module.Path,
		})
	}

	afterIndexState, err := NewScanner(db).CurrentIndexState(ctx)
	if err != nil {
		return projection, err
	}
	if !indexState.SameCurrentBasis(afterIndexState) {
		projection.IndexState = FileGapIndexStale
		projection.Reason = "code-index basis changed during the file-gap query; retry"
		projection.IndexedFiles = 0
		projection.TotalGaps = 0
		projection.OmittedGaps = 0
		projection.Gaps = nil
		return projection, nil
	}
	projection.IndexState = FileGapIndexCurrent
	projection.Gaps = gaps
	projection.OmittedGaps = projection.TotalGaps - len(gaps)
	return projection, nil
}

func normalizeFileGapProjectionLimit(requested int) int {
	if requested <= 0 {
		return defaultFileGapProjectionLimit
	}
	return min(requested, maxFileGapProjectionLimit)
}

func coverageTableExists(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var count int
	err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
		name,
	).Scan(&count)
	return count > 0, err
}

func coverageIndexedSourcePaths(
	ctx context.Context,
	db *sql.DB,
) (map[string]bool, bool, error) {
	present, err := coverageTableExists(ctx, db, "code_files")
	if err != nil || !present {
		return nil, false, err
	}
	rows, err := db.QueryContext(
		ctx,
		`SELECT file_path FROM code_files ORDER BY file_path`,
	)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var rawPath string
		if err := rows.Scan(&rawPath); err != nil {
			return nil, false, err
		}
		canonical, err := projectpath.Parse(rawPath)
		if err != nil || rawPath != canonical.String() {
			continue
		}
		result[canonical.String()] = true
	}
	return result, len(result) > 0, rows.Err()
}

func readCoverageIndexMeta(ctx context.Context, db *sql.DB) (coverageIndexMeta, bool, error) {
	present, err := coverageTableExists(ctx, db, "code_index_meta")
	if err != nil || !present {
		return coverageIndexMeta{}, false, err
	}

	var meta coverageIndexMeta
	var degraded int
	err = db.QueryRowContext(ctx, `
		SELECT fingerprint, current_epoch, degraded, degraded_reason
		FROM code_index_meta
		WHERE id = 1`).Scan(
		&meta.Fingerprint,
		&meta.Epoch,
		&degraded,
		&meta.DegradedReason,
	)
	if err == sql.ErrNoRows {
		return coverageIndexMeta{}, false, nil
	}
	meta.Degraded = degraded != 0
	return meta, err == nil, err
}

func currentIndexedFiles(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT file_path
		FROM code_files
		WHERE parse_status IN ('indexed', 'empty')
		ORDER BY file_path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make([]string, 0)
	for rows.Next() {
		var filePath string
		if err := rows.Scan(&filePath); err != nil {
			return nil, err
		}
		canonical, err := projectpath.Parse(filePath)
		if err != nil {
			return nil, fmt.Errorf(
				"indexed file has invalid project path %q: %w",
				filePath,
				err,
			)
		}
		files = append(files, canonical.String())
	}
	return files, rows.Err()
}

func activeDecisionLinkedFiles(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT af.file_path, COALESCE(a.structured_data, '{}')
		FROM affected_files af
		JOIN artifacts a ON a.id = af.artifact_id
		WHERE a.status IN ('active', 'refresh_due')
		  AND a.kind = 'DecisionRecord'
		ORDER BY af.file_path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	linked := make(map[string]bool)
	for rows.Next() {
		var filePath, structuredData string
		if err := rows.Scan(&filePath, &structuredData); err != nil {
			return nil, err
		}
		_, err := governance.ParseDecisionPathPolicy(structuredData)
		if err != nil {
			continue
		}
		canonical, err := projectpath.Parse(filePath)
		if err != nil {
			continue
		}
		linked[canonical.String()] = true
	}
	return linked, rows.Err()
}

func mostSpecificCoverageModule(modules []ModuleCoverage, filePath string) (ModuleCoverage, bool) {
	rawModules := make([]Module, 0, len(modules))
	for _, module := range modules {
		rawModules = append(rawModules, module.Module)
	}
	resolved, ok := mostSpecificModuleForPath(rawModules, filePath)
	if !ok {
		return ModuleCoverage{}, false
	}
	for _, module := range modules {
		if module.Module.ID == resolved.ID {
			return module, true
		}
	}
	return ModuleCoverage{}, false
}

func mostSpecificModuleForPath(modules []Module, filePath string) (Module, bool) {
	candidate, err := projectpath.Parse(filePath)
	if err != nil {
		return Module{}, false
	}

	refs := make([]projectpath.ModuleRef, 0, len(modules))
	for _, module := range modules {
		moduleRef, err := projectpath.NewModuleRef(
			module.ID,
			module.Path,
		)
		if err != nil {
			return Module{}, false
		}
		refs = append(refs, moduleRef)
	}
	resolved, ok, err := projectpath.ResolveMostSpecificModule(
		refs,
		candidate,
	)
	if err != nil || !ok {
		return Module{}, false
	}
	for _, module := range modules {
		if module.ID == resolved.ID() {
			return module, true
		}
	}
	return Module{}, false
}

// FormatCoverageResponse formats the coverage report for MCP output.
func FormatCoverageResponse(report *CoverageReport) string {
	if report.TotalModules == 0 {
		result := "No stored module index. Run explicit `haft_refresh(action=\"scan\")` before relying on module or file coverage. This is unavailable, not an empty-clean result.\n"
		if report.FileGaps.IndexState != "" {
			result += formatFileGapProjection(report.FileGaps)
		}
		return result
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

	if report.FileGaps.IndexState != "" {
		sb.WriteString(formatFileGapProjection(report.FileGaps))
	}

	return sb.String()
}

func formatFileGapProjection(projection FileGapProjection) string {
	var sb strings.Builder
	sb.WriteString("\n## Exact File Decision-Link Gaps\n\n")
	if projection.IndexState != FileGapIndexCurrent {
		sb.WriteString(fmt.Sprintf(
			"- Unavailable: derived code index is `%s`",
			projection.IndexState,
		))
		if projection.Reason != "" {
			sb.WriteString(" — " + projection.Reason)
		}
		sb.WriteString(".\n")
		sb.WriteString("- This is not an empty or clean result. Run explicit `haft_refresh(action=\"scan\")` before relying on file-gap claims.\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf(
		"- Current code index epoch %d contains %d indexed source file(s).\n",
		projection.IndexEpoch,
		projection.IndexedFiles,
	))
	sb.WriteString(fmt.Sprintf(
		"- Exact basis: `%s` (coverage `%s`).\n",
		projection.IndexBasisRef,
		projection.CoveragePosture,
	))
	sb.WriteString(fmt.Sprintf(
		"- %d file(s) inside modules with active DecisionRecords have no exact active `affected_files` link. This is an orientation cue, not proof that a file is undocumented, unconstrained, or incorrect.\n",
		projection.TotalGaps,
	))
	for _, gap := range projection.Gaps {
		modulePath := gap.ModulePath
		if modulePath == "" {
			modulePath = "(root)"
		}
		sb.WriteString(fmt.Sprintf("  ✗ %s — module %s\n", gap.FilePath, modulePath))
	}
	if projection.OmittedGaps > 0 {
		sb.WriteString(fmt.Sprintf(
			"  ... %d more gap(s) omitted; repeat `haft_query(action=\"coverage\", limit=N)` with N up to %d.\n",
			projection.OmittedGaps,
			maxFileGapProjectionLimit,
		))
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

// FormatCoverageCockpitSummary formats a one-cue coverage projection for the
// default status cockpit. Full coverage stays behind explicit drill-down calls.
func FormatCoverageCockpitSummary(report *CoverageReport) string {
	if report.TotalModules == 0 {
		return "## Coverage Cue\n\n- No modules detected. Run module scan first.\n"
	}

	pct := 0
	if report.TotalModules > 0 {
		pct = (report.CoveredCount + report.PartialCount) * 100 / report.TotalModules
	}

	var sb strings.Builder
	sb.WriteString("## Coverage Cue\n\n")
	sb.WriteString(fmt.Sprintf(
		"- %d module(s), %d%% governed; %d blind, %d degraded. Detailed coverage stays behind the Drill-down commands.\n",
		report.TotalModules,
		pct,
		report.BlindCount,
		report.PartialCount,
	))

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
