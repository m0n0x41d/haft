package main

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/graph"
	"github.com/m0n0x41d/haft/internal/reff"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	governanceScanInterval    = 2 * time.Minute
	governanceStateLastScanAt = "last_scan_at"
	candidateStatusActive     = "active"
	candidateStatusDismissed  = "dismissed"
	candidateStatusAdopted    = "adopted"
	adoptDiffMaxLines         = 200
	adoptDiffMaxChars         = 12000
)

type CoverageView struct {
	TotalModules    int                  `json:"total_modules"`
	CoveredCount    int                  `json:"covered_count"`
	PartialCount    int                  `json:"partial_count"`
	BlindCount      int                  `json:"blind_count"`
	GovernedPercent int                  `json:"governed_percent"`
	LastScanned     string               `json:"last_scanned"`
	Modules         []CoverageModuleView `json:"modules"`
}

type CoverageModuleView struct {
	ID            string   `json:"id"`
	Path          string   `json:"path"`
	Name          string   `json:"name"`
	Lang          string   `json:"lang"`
	Status        string   `json:"status"`
	DecisionCount int      `json:"decision_count"`
	DecisionIDs   []string `json:"decision_ids"`
	Impacted      bool     `json:"impacted"`
	Files         []string `json:"files"`
}

type GovernanceFindingView struct {
	ID          string  `json:"id"`
	ArtifactRef string  `json:"artifact_ref"`
	Title       string  `json:"title"`
	Kind        string  `json:"kind"`
	Category    string  `json:"category"`
	Reason      string  `json:"reason"`
	ValidUntil  string  `json:"valid_until"`
	DaysStale   int     `json:"days_stale"`
	REff        float64 `json:"r_eff"`
	DriftCount  int     `json:"drift_count"`
}

type ProblemCandidateView struct {
	ID                string `json:"id"`
	Status            string `json:"status"`
	Title             string `json:"title"`
	Signal            string `json:"signal"`
	Acceptance        string `json:"acceptance"`
	Context           string `json:"context"`
	Category          string `json:"category"`
	SourceArtifactRef string `json:"source_artifact_ref"`
	SourceTitle       string `json:"source_title"`
	ProblemRef        string `json:"problem_ref"`
}

type GovernanceOverviewView struct {
	LastScanAt        string                  `json:"last_scan_at"`
	Coverage          CoverageView            `json:"coverage"`
	Findings          []GovernanceFindingView `json:"findings"`
	ProblemCandidates []ProblemCandidateView  `json:"problem_candidates"`
}

type GovernanceScanEvent struct {
	LastScanAt               string `json:"last_scan_at"`
	FindingCount             int    `json:"finding_count"`
	DriftCount               int    `json:"drift_count"`
	PendingVerificationCount int    `json:"pending_verification_count"`
	CandidateCount           int    `json:"candidate_count"`
	CoveragePercent          int    `json:"coverage_percent"`
}

type DesktopNotification struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Tone   string `json:"tone"`
	Source string `json:"source"`
}

type governanceDriftAdoptionContext struct {
	Finding        GovernanceFindingView
	Decision       *artifact.Artifact
	Detail         DecisionDetailView
	Report         artifact.DriftReport
	FileDiffs      []governanceFileDiff
	DirectModules  []CoverageModuleView
	ImpactedModule []artifact.ModuleImpact
}

type governanceStaleAdoptionContext struct {
	Finding          GovernanceFindingView
	Decision         *artifact.Artifact
	Detail           DecisionDetailView
	Assurance        artifact.WLNKSummary
	EvidenceTimeline []governanceEvidenceTimelineItem
	ExpiredItems     []string
}

type governanceEvidenceTimelineItem struct {
	ID              string
	Type            string
	Verdict         string
	Content         string
	CarrierRef      string
	CongruenceLevel int
	FormalityLevel  int
	Claims          []string
	ValidUntil      string
	IsExpired       bool
	CountsForREff   bool
	Score           float64
}

type governanceFileDiff struct {
	Path   string
	Status artifact.DriftStatus
	Diff   string
}

type desktopGovernanceStore struct {
	db *sql.DB
}

type governanceController struct {
	mu          sync.RWMutex
	scanMu      sync.Mutex
	app         *App
	store       *artifact.Store
	db          *sql.DB
	projectRoot string
	state       *desktopGovernanceStore
	interval    time.Duration
	stop        chan struct{}
	done        chan struct{}
	ctx         context.Context
	cancel      context.CancelFunc
	known       map[string]struct{}
	snapshot    GovernanceOverviewView
	started     bool
	stopOnce    sync.Once
}

func newDesktopGovernanceStore(db *sql.DB) *desktopGovernanceStore {
	return &desktopGovernanceStore{db: db}
}

func newGovernanceController(app *App, store *artifact.Store, db *sql.DB, projectRoot string) *governanceController {
	return &governanceController{
		app:         app,
		store:       store,
		db:          db,
		projectRoot: projectRoot,
		state:       newDesktopGovernanceStore(db),
		interval:    governanceScanInterval,
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
		known:       make(map[string]struct{}),
	}
}

func (g *governanceController) start(ctx context.Context) {
	if g == nil || g.store == nil || g.db == nil || g.projectRoot == "" {
		return
	}

	g.mu.Lock()
	if g.started {
		g.mu.Unlock()
		return
	}
	g.started = true
	g.ctx, g.cancel = context.WithCancel(ctx)
	g.mu.Unlock()

	go func() {
		ticker := time.NewTicker(g.interval)

		defer close(g.done)
		defer ticker.Stop()

		if _, err := g.scan(g.ctx, false); err != nil && g.ctx.Err() == nil {
			g.app.emitAppError("governance scan", err)
		}

		for {
			select {
			case <-ticker.C:
				if _, err := g.scan(g.ctx, true); err != nil && g.ctx.Err() == nil {
					g.app.emitAppError("governance scan", err)
				}
			case <-g.stop:
				return
			}
		}
	}()
}

func (g *governanceController) shutdown() {
	if g == nil {
		return
	}

	g.mu.RLock()
	started := g.started
	g.mu.RUnlock()

	if !started {
		return
	}

	g.stopOnce.Do(func() {
		// Cancel context first — aborts any in-flight DB operations from
		// both the background goroutine and concurrent frontend scan calls.
		if g.cancel != nil {
			g.cancel()
		}
		close(g.stop)
	})
	<-g.done
}

func (g *governanceController) snapshotOrScan(_ context.Context) (GovernanceOverviewView, error) {
	g.mu.RLock()
	snapshot := g.snapshot
	scanCtx := g.ctx
	g.mu.RUnlock()

	if snapshot.LastScanAt != "" || len(snapshot.Findings) > 0 || len(snapshot.ProblemCandidates) > 0 {
		return snapshot, nil
	}

	// Use the controller's context if available; fall back to a background
	// context for synchronous scans when the background goroutine isn't started
	// (e.g., tests, or environments without Wails events).
	if scanCtx != nil && scanCtx.Err() != nil {
		return GovernanceOverviewView{}, fmt.Errorf("governance controller is shutting down")
	}
	if scanCtx == nil {
		scanCtx = context.Background()
	}

	return g.scan(scanCtx, false)
}

func (g *governanceController) scan(callerCtx context.Context, notify bool) (GovernanceOverviewView, error) {
	// Serialize scans — prevents SQLITE_BUSY when the background goroutine
	// and a frontend GetDashboard() call race into buildGovernanceOverview.
	g.scanMu.Lock()
	defer g.scanMu.Unlock()

	// Prefer the controller's own context (cancelled on shutdown) when available.
	// Fall back to the caller's context for synchronous scans (tests, no Wails runtime).
	ctx := g.ctx
	if ctx != nil && ctx.Err() != nil {
		// Shutting down — return empty data, not an error.
		// Errors during shutdown are not actionable and just produce toast noise.
		return GovernanceOverviewView{}, nil
	}
	if ctx == nil {
		ctx = callerCtx
	}
	if ctx == nil {
		ctx = context.Background()
	}

	overview, err := buildGovernanceOverview(ctx, g.store, g.db, g.state, g.projectRoot)
	if err != nil {
		return GovernanceOverviewView{}, err
	}

	newFindings := g.updateSnapshot(overview)

	g.app.emitEvent("scan.stale", governanceScanSummary(overview))
	g.app.emitEvent("scan.drift", governanceScanSummary(overview))

	if notify && len(newFindings) > 0 {
		g.app.pushNotification(newGovernanceNotification(overview, newFindings))
	}

	return overview, nil
}

func (g *governanceController) updateSnapshot(overview GovernanceOverviewView) []GovernanceFindingView {
	g.mu.Lock()
	defer g.mu.Unlock()

	current := make(map[string]GovernanceFindingView, len(overview.Findings))
	newFindings := make([]GovernanceFindingView, 0)

	for _, finding := range overview.Findings {
		current[finding.ID] = finding
		if _, exists := g.known[finding.ID]; exists {
			continue
		}
		newFindings = append(newFindings, finding)
	}

	g.known = make(map[string]struct{}, len(current))
	for id := range current {
		g.known[id] = struct{}{}
	}

	g.snapshot = overview

	return newFindings
}

func buildGovernanceOverview(
	ctx context.Context,
	store *artifact.Store,
	db *sql.DB,
	state *desktopGovernanceStore,
	projectRoot string,
) (GovernanceOverviewView, error) {
	// Check context before expensive operations — if shutting down, return empty.
	if ctx.Err() != nil {
		return GovernanceOverviewView{}, nil
	}

	coverage, coverageErr := buildCoverageView(ctx, db, projectRoot, nil)
	findings, err := artifact.ScanStale(ctx, store, projectRoot)
	if err != nil {
		// "database is closed" during project switch is expected, not an error.
		if strings.Contains(err.Error(), "database is closed") || ctx.Err() != nil {
			return GovernanceOverviewView{}, nil
		}
		return GovernanceOverviewView{}, fmt.Errorf("scan stale artifacts: %w", err)
	}

	// Check invariants for decisions with drift findings
	invariantFindings := checkDriftInvariants(ctx, store, db, findings)
	findings = append(findings, invariantFindings...)

	findingViews := toFindingViews(findings)
	candidateDrafts := buildProblemCandidates(findings)

	if err := state.UpsertCandidates(ctx, candidateDrafts); err != nil {
		if strings.Contains(err.Error(), "database is closed") || ctx.Err() != nil {
			return GovernanceOverviewView{}, nil
		}
		return GovernanceOverviewView{}, fmt.Errorf("sync problem candidates: %w", err)
	}

	candidates, err := state.ListActiveCandidates(ctx, candidateDrafts)
	if err != nil {
		if strings.Contains(err.Error(), "database is closed") || ctx.Err() != nil {
			return GovernanceOverviewView{}, nil
		}
		return GovernanceOverviewView{}, fmt.Errorf("list problem candidates: %w", err)
	}

	lastScanAt := nowRFC3339()
	if err := state.SetState(ctx, governanceStateLastScanAt, lastScanAt); err != nil {
		return GovernanceOverviewView{}, fmt.Errorf("persist governance scan state: %w", err)
	}

	if coverageErr != nil {
		candidates = appendCoverageCandidate(candidates, coverageErr)
	}

	return GovernanceOverviewView{
		LastScanAt:        lastScanAt,
		Coverage:          coverage,
		Findings:          findingViews,
		ProblemCandidates: candidates,
	}, nil
}

func buildCoverageView(
	ctx context.Context,
	db *sql.DB,
	projectRoot string,
	impactedFiles []string,
) (CoverageView, error) {
	if db == nil || projectRoot == "" {
		return CoverageView{}, nil
	}

	lastScanned, err := ensureCodebaseIndexed(ctx, db, projectRoot)
	if err != nil {
		return CoverageView{}, err
	}

	report, err := codebase.ComputeCoverage(ctx, db)
	if err != nil {
		return CoverageView{}, fmt.Errorf("compute coverage: %w", err)
	}

	return toCoverageView(ctx, db, report, lastScanned, impactedFiles), nil
}

func ensureCodebaseIndexed(ctx context.Context, db *sql.DB, projectRoot string) (time.Time, error) {
	if db == nil || projectRoot == "" {
		return time.Time{}, nil
	}

	scanner := codebase.NewScanner(db)
	modules, err := scanner.ScanModules(ctx, projectRoot)
	if err != nil {
		return time.Time{}, fmt.Errorf("scan modules: %w", err)
	}

	if len(modules) > 0 {
		_, _ = scanner.ScanDependencies(ctx, projectRoot)
	}

	return scanner.ModulesLastScanned(ctx), nil
}

func toCoverageView(
	ctx context.Context,
	db *sql.DB,
	report *codebase.CoverageReport,
	lastScanned time.Time,
	impactedFiles []string,
) CoverageView {
	if report == nil {
		return CoverageView{}
	}

	impactedByModule := impactedFilesByModule(ctx, db, impactedFiles)
	modules := make([]CoverageModuleView, 0, len(report.Modules))

	for _, module := range report.Modules {
		files := impactedByModule[module.Module.ID]
		view := CoverageModuleView{
			ID:            module.Module.ID,
			Path:          module.Module.Path,
			Name:          module.Module.Name,
			Lang:          module.Module.Lang,
			Status:        string(module.Status),
			DecisionCount: module.DecisionCount,
			DecisionIDs:   append([]string(nil), module.DecisionIDs...),
			Impacted:      len(files) > 0,
			Files:         append([]string(nil), files...),
		}

		modules = append(modules, view)
	}

	sort.Slice(modules, func(i int, j int) bool {
		if modules[i].Impacted != modules[j].Impacted {
			return modules[i].Impacted
		}
		if modules[i].Status != modules[j].Status {
			return modules[i].Status < modules[j].Status
		}
		return modules[i].Path < modules[j].Path
	})

	governedModules := report.CoveredCount + report.PartialCount
	governedPercent := 0
	if report.TotalModules > 0 {
		governedPercent = governedModules * 100 / report.TotalModules
	}

	lastScannedValue := ""
	if !lastScanned.IsZero() {
		lastScannedValue = lastScanned.Format(time.RFC3339)
	}

	return CoverageView{
		TotalModules:    report.TotalModules,
		CoveredCount:    report.CoveredCount,
		PartialCount:    report.PartialCount,
		BlindCount:      report.BlindCount,
		GovernedPercent: governedPercent,
		LastScanned:     lastScannedValue,
		Modules:         modules,
	}
}

func impactedFilesByModule(ctx context.Context, db *sql.DB, files []string) map[string][]string {
	if db == nil || len(files) == 0 {
		return map[string][]string{}
	}

	scanner := codebase.NewScanner(db)
	grouped := make(map[string][]string)

	for _, filePath := range files {
		moduleID, err := scanner.ResolveFileToModule(ctx, filePath)
		if err != nil || moduleID == "" {
			continue
		}

		grouped[moduleID] = append(grouped[moduleID], filePath)
	}

	for moduleID := range grouped {
		sort.Strings(grouped[moduleID])
	}

	return grouped
}

// checkDriftInvariants runs invariant verification for decisions that have drift findings.
// Returns additional StaleItems for invariant violations.
func checkDriftInvariants(
	ctx context.Context,
	store *artifact.Store,
	db *sql.DB,
	existing []artifact.StaleItem,
) []artifact.StaleItem {
	if db == nil || store == nil {
		return nil
	}

	gs := graph.NewStore(db)

	// Collect decision IDs that have drift
	driftDecisions := make(map[string]bool)
	for _, item := range existing {
		if item.Kind == "DecisionRecord" && len(item.DriftItems) > 0 {
			driftDecisions[item.ID] = true
		}
	}

	var violations []artifact.StaleItem
	for decID := range driftDecisions {
		results, err := graph.VerifyInvariants(ctx, gs, db, decID)
		if err != nil {
			continue
		}
		for _, r := range results {
			if r.Status != graph.InvariantViolated {
				continue
			}
			violations = append(violations, artifact.StaleItem{
				ID:       decID,
				Title:    "Invariant violation: " + r.Invariant.Text,
				Kind:     "DecisionRecord",
				Category: "invariant_violated",
				Reason:   r.Reason,
			})
		}
	}

	return violations
}

func toFindingViews(items []artifact.StaleItem) []GovernanceFindingView {
	views := make([]GovernanceFindingView, 0, len(items))

	for _, item := range items {
		views = append(views, GovernanceFindingView{
			ID:          findingID(item),
			ArtifactRef: item.ID,
			Title:       item.Title,
			Kind:        item.Kind,
			Category:    string(item.Category),
			Reason:      item.Reason,
			ValidUntil:  item.ValidUntil,
			DaysStale:   item.DaysStale,
			REff:        item.REff,
			DriftCount:  len(item.DriftItems),
		})
	}

	return views
}

func buildProblemCandidates(items []artifact.StaleItem) []ProblemCandidateView {
	candidates := make([]ProblemCandidateView, 0, len(items))

	for _, item := range items {
		candidate, ok := problemCandidateForItem(item)
		if !ok {
			continue
		}

		candidates = append(candidates, candidate)
	}

	sort.Slice(candidates, func(i int, j int) bool {
		if candidates[i].Category != candidates[j].Category {
			return candidates[i].Category < candidates[j].Category
		}
		return candidates[i].Title < candidates[j].Title
	})

	return candidates
}

func problemCandidateForItem(item artifact.StaleItem) (ProblemCandidateView, bool) {
	contextName := "desktop-governance"

	switch item.Category {
	case artifact.StaleCategoryDecisionStale:
		return ProblemCandidateView{
			ID:                candidateID(item),
			Status:            candidateStatusActive,
			Title:             fmt.Sprintf("Investigate drift against %s", item.Title),
			Signal:            item.Reason,
			Acceptance:        "The drifted decision has either a fresh baseline, a refresh action, or an explicit supersession path.",
			Context:           contextName,
			Category:          string(item.Category),
			SourceArtifactRef: item.ID,
			SourceTitle:       item.Title,
		}, true
	case artifact.StaleCategoryPendingVerification:
		return ProblemCandidateView{
			ID:                candidateID(item),
			Status:            candidateStatusActive,
			Title:             fmt.Sprintf("Verify due claims for %s", item.Title),
			Signal:            item.Reason,
			Acceptance:        "Due claims have evidence attached and the decision measurement reflects the latest verdict.",
			Context:           contextName,
			Category:          string(item.Category),
			SourceArtifactRef: item.ID,
			SourceTitle:       item.Title,
		}, true
	case artifact.StaleCategoryREffDegraded:
		return ProblemCandidateView{
			ID:                candidateID(item),
			Status:            candidateStatusActive,
			Title:             fmt.Sprintf("Refresh degraded evidence for %s", item.Title),
			Signal:            item.Reason,
			Acceptance:        "The affected decision has enough fresh evidence to recover or the degradation is explicitly superseded.",
			Context:           contextName,
			Category:          string(item.Category),
			SourceArtifactRef: item.ID,
			SourceTitle:       item.Title,
		}, true
	case artifact.StaleCategoryEvidenceExpired:
		return ProblemCandidateView{
			ID:                candidateID(item),
			Status:            candidateStatusActive,
			Title:             fmt.Sprintf("Refresh stale artifact %s", item.Title),
			Signal:            item.Reason,
			Acceptance:        "The stale artifact is either refreshed with current evidence or explicitly deprecated.",
			Context:           contextName,
			Category:          string(item.Category),
			SourceArtifactRef: item.ID,
			SourceTitle:       item.Title,
		}, true
	case artifact.StaleCategoryEpistemicDebtExceeded:
		return ProblemCandidateView{
			ID:                candidateID(item),
			Status:            candidateStatusActive,
			Title:             "Reduce project epistemic debt",
			Signal:            item.Reason,
			Acceptance:        "The project debt budget is back under the configured threshold or the budget is explicitly revised with evidence.",
			Context:           contextName,
			Category:          string(item.Category),
			SourceArtifactRef: item.ID,
			SourceTitle:       item.Title,
		}, true
	case "invariant_violated":
		return ProblemCandidateView{
			ID:                candidateID(item),
			Status:            candidateStatusActive,
			Title:             item.Title,
			Signal:            item.Reason,
			Acceptance:        "The violated invariant is either restored, the offending code is reverted, or the invariant is explicitly revised.",
			Context:           contextName,
			Category:          string(item.Category),
			SourceArtifactRef: item.ID,
			SourceTitle:       item.Title,
		}, true
	default:
		return ProblemCandidateView{}, false
	}
}

func governanceScanSummary(overview GovernanceOverviewView) GovernanceScanEvent {
	driftCount := 0
	pendingVerificationCount := 0

	for _, finding := range overview.Findings {
		switch finding.Category {
		case string(artifact.StaleCategoryDecisionStale):
			driftCount++
		case string(artifact.StaleCategoryPendingVerification):
			pendingVerificationCount++
		}
	}

	return GovernanceScanEvent{
		LastScanAt:               overview.LastScanAt,
		FindingCount:             len(overview.Findings),
		DriftCount:               driftCount,
		PendingVerificationCount: pendingVerificationCount,
		CandidateCount:           len(overview.ProblemCandidates),
		CoveragePercent:          overview.Coverage.GovernedPercent,
	}
}

func newGovernanceNotification(
	overview GovernanceOverviewView,
	newFindings []GovernanceFindingView,
) DesktopNotification {
	bodyParts := make([]string, 0, minInt(len(newFindings), 3))

	for _, finding := range newFindings[:minInt(len(newFindings), 3)] {
		bodyParts = append(bodyParts, finding.Title)
	}

	body := strings.Join(bodyParts, "; ")
	if len(newFindings) > 3 {
		body = fmt.Sprintf("%s; +%d more", body, len(newFindings)-3)
	}
	if body == "" {
		body = "New governance findings are available."
	}

	return DesktopNotification{
		ID:     fmt.Sprintf("governance-%s", shortHash(overview.LastScanAt+body)),
		Title:  fmt.Sprintf("Governance scan found %d new item(s)", len(newFindings)),
		Body:   body,
		Tone:   "warning",
		Source: "governance",
	}
}

func appendCoverageCandidate(candidates []ProblemCandidateView, err error) []ProblemCandidateView {
	if err == nil {
		return candidates
	}

	return append(candidates, ProblemCandidateView{
		ID:                fmt.Sprintf("cand-%s", shortHash(err.Error())),
		Status:            candidateStatusActive,
		Title:             "Repair module coverage scan",
		Signal:            err.Error(),
		Acceptance:        "Module detection runs successfully and coverage is visible in the desktop dashboard again.",
		Context:           "desktop-governance",
		Category:          string(artifact.StaleCategoryScanFailed),
		SourceArtifactRef: "system/coverage-scan",
		SourceTitle:       "Coverage scan",
	})
}

func findingID(item artifact.StaleItem) string {
	key := strings.Join([]string{item.ID, string(item.Category), item.Reason, item.ValidUntil}, "|")
	return fmt.Sprintf("finding-%s", shortHash(key))
}

func candidateID(item artifact.StaleItem) string {
	key := strings.Join([]string{item.ID, string(item.Category), item.Reason}, "|")
	return fmt.Sprintf("cand-%s", shortHash(key))
}

func shortHash(value string) string {
	sum := sha1.Sum([]byte(value))
	encoded := hex.EncodeToString(sum[:])
	if len(encoded) < 12 {
		return encoded
	}
	return encoded[:12]
}

func (a *App) loadDriftAdoptionContext(findingRef string) (*governanceDriftAdoptionContext, error) {
	if a == nil || a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}

	trimmed := strings.TrimSpace(findingRef)
	if trimmed == "" {
		return nil, fmt.Errorf("finding ref is required")
	}

	finding, item, err := a.resolveDriftFinding(trimmed)
	if err != nil {
		return nil, err
	}

	decision, detail, err := a.loadDecisionDetail(item.ID)
	if err != nil {
		return nil, fmt.Errorf("load decision %s: %w", item.ID, err)
	}

	report, err := a.loadDecisionDriftReport(item.ID)
	if err != nil {
		return nil, err
	}

	driftedFiles := driftedFilePaths(report.Files)
	directModules, err := a.loadDriftAffectedModules(driftedFiles)
	if err != nil {
		detail.CoverageWarnings = append(
			detail.CoverageWarnings,
			fmt.Sprintf("Affected module lookup unavailable: %v", err),
		)
	}

	impactedModules, err := a.loadDriftImpactedModules(driftedFiles)
	if err != nil {
		detail.CoverageWarnings = append(
			detail.CoverageWarnings,
			fmt.Sprintf("Dependency impact lookup unavailable: %v", err),
		)
	}

	return &governanceDriftAdoptionContext{
		Finding:        finding,
		Decision:       decision,
		Detail:         detail,
		Report:         report,
		FileDiffs:      loadGovernanceFileDiffs(a.projectRoot, report.Files),
		DirectModules:  directModules,
		ImpactedModule: impactedModules,
	}, nil
}

func (a *App) resolveGovernanceFinding(findingRef string) (GovernanceFindingView, artifact.StaleItem, error) {
	items, err := artifact.ScanStale(a.ctx, a.store, a.projectRoot)
	if err != nil {
		return GovernanceFindingView{}, artifact.StaleItem{}, fmt.Errorf("scan governance findings: %w", err)
	}

	for _, item := range items {
		if findingID(item) != findingRef {
			continue
		}

		view := toFindingViews([]artifact.StaleItem{item})[0]
		return view, item, nil
	}

	return GovernanceFindingView{}, artifact.StaleItem{}, fmt.Errorf("finding %s not found", findingRef)
}

func (a *App) resolveDriftFinding(findingRef string) (GovernanceFindingView, artifact.StaleItem, error) {
	view, item, err := a.resolveGovernanceFinding(findingRef)
	if err != nil {
		return GovernanceFindingView{}, artifact.StaleItem{}, err
	}

	if item.Category != artifact.StaleCategoryDecisionStale {
		return GovernanceFindingView{}, artifact.StaleItem{}, fmt.Errorf("finding %s is not a drift finding", findingRef)
	}
	if item.Kind != string(artifact.KindDecisionRecord) {
		return GovernanceFindingView{}, artifact.StaleItem{}, fmt.Errorf("finding %s does not point to a DecisionRecord", findingRef)
	}
	if len(item.DriftItems) == 0 {
		return GovernanceFindingView{}, artifact.StaleItem{}, fmt.Errorf("finding %s is not a drift finding", findingRef)
	}

	return view, item, nil
}

func (a *App) resolveStaleFinding(findingRef string) (GovernanceFindingView, artifact.StaleItem, error) {
	view, item, err := a.resolveGovernanceFinding(findingRef)
	if err != nil {
		return GovernanceFindingView{}, artifact.StaleItem{}, err
	}

	if item.Kind != string(artifact.KindDecisionRecord) {
		return GovernanceFindingView{}, artifact.StaleItem{}, fmt.Errorf("finding %s does not point to a DecisionRecord", findingRef)
	}
	if !isStaleAdoptionCategory(item.Category) {
		return GovernanceFindingView{}, artifact.StaleItem{}, fmt.Errorf("finding %s is not a stale decision finding", findingRef)
	}

	return view, item, nil
}

func isStaleAdoptionCategory(category artifact.StaleCategory) bool {
	switch category {
	case artifact.StaleCategoryEvidenceExpired, artifact.StaleCategoryREffDegraded:
		return true
	default:
		return false
	}
}

func (a *App) loadStaleAdoptionContext(findingRef string) (*governanceStaleAdoptionContext, error) {
	if a == nil || a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}

	trimmed := strings.TrimSpace(findingRef)
	if trimmed == "" {
		return nil, fmt.Errorf("finding ref is required")
	}

	finding, item, err := a.resolveStaleFinding(trimmed)
	if err != nil {
		return nil, err
	}

	decision, detail, err := a.loadDecisionDetail(item.ID)
	if err != nil {
		return nil, fmt.Errorf("load decision %s: %w", item.ID, err)
	}

	assurance := artifact.ComputeWLNKSummary(a.ctx, a.store, item.ID)
	timeline, expiredItems, err := a.loadStaleEvidenceTimeline(item.ID, detail.ValidUntil)
	if err != nil {
		return nil, fmt.Errorf("load evidence timeline for %s: %w", item.ID, err)
	}

	return &governanceStaleAdoptionContext{
		Finding:          finding,
		Decision:         decision,
		Detail:           detail,
		Assurance:        assurance,
		EvidenceTimeline: timeline,
		ExpiredItems:     expiredItems,
	}, nil
}

func (a *App) loadStaleEvidenceTimeline(
	decisionID string,
	decisionValidUntil string,
) ([]governanceEvidenceTimelineItem, []string, error) {
	items, err := a.store.GetEvidenceItems(a.ctx, decisionID)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now().UTC()
	timeline := make([]governanceEvidenceTimelineItem, 0, len(items))
	expiredItems := governanceDecisionExpiredItems(decisionValidUntil, now)

	for _, item := range items {
		timelineItem := governanceEvidenceTimelineItem{
			ID:              item.ID,
			Type:            item.Type,
			Verdict:         item.Verdict,
			Content:         strings.TrimSpace(item.Content),
			CarrierRef:      strings.TrimSpace(item.CarrierRef),
			CongruenceLevel: item.CongruenceLevel,
			FormalityLevel:  item.FormalityLevel,
			Claims:          governanceEvidenceClaims(item),
			ValidUntil:      item.ValidUntil,
			IsExpired:       governanceEvidenceExpired(item.ValidUntil, now),
			CountsForREff:   item.Verdict != "superseded",
		}
		if timelineItem.CountsForREff {
			timelineItem.Score = reff.ScoreEvidence(
				item.Verdict,
				item.CongruenceLevel,
				item.ValidUntil,
				now,
			)
		}

		timeline = append(timeline, timelineItem)

		if timelineItem.IsExpired {
			expiredItems = append(expiredItems, governanceExpiredEvidenceLabel(timelineItem))
		}
	}

	return timeline, expiredItems, nil
}

func governanceDecisionExpiredItems(validUntil string, now time.Time) []string {
	expiry, ok := reff.ParseValidUntil(validUntil)
	if !ok || !expiry.Before(now) {
		return []string{}
	}

	daysExpired := int(now.Sub(expiry).Hours() / 24)
	label := fmt.Sprintf(
		"DecisionRecord valid_until expired %d day(s) ago (%s).",
		daysExpired,
		validUntil,
	)

	return []string{label}
}

func governanceEvidenceClaims(item artifact.EvidenceItem) []string {
	seen := make(map[string]struct{})
	claims := make([]string, 0, len(item.ClaimRefs)+len(item.ClaimScope))

	for _, claim := range item.ClaimRefs {
		trimmed := strings.TrimSpace(claim)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		claims = append(claims, trimmed)
	}

	for _, claim := range item.ClaimScope {
		trimmed := strings.TrimSpace(claim)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		claims = append(claims, trimmed)
	}

	return claims
}

func governanceEvidenceExpired(validUntil string, now time.Time) bool {
	expiry, ok := reff.ParseValidUntil(validUntil)
	if !ok {
		return false
	}

	return expiry.Before(now)
}

func governanceExpiredEvidenceLabel(item governanceEvidenceTimelineItem) string {
	status := "counts in R_eff"
	if !item.CountsForREff {
		status = "excluded from R_eff"
	}

	return fmt.Sprintf(
		"%s [%s] verdict=%s valid_until=%s %s",
		item.ID,
		item.Type,
		item.Verdict,
		firstNonEmpty(item.ValidUntil, "n/a"),
		status,
	)
}

func buildAdoptStalePrompt(context governanceStaleAdoptionContext) string {
	var prompt strings.Builder

	writeSectionTitle(
		&prompt,
		"Adopt Stale Finding",
		firstNonEmpty(context.Detail.SelectedTitle, context.Detail.Title, context.Decision.Meta.Title),
	)
	writeMetaLine(&prompt, "Finding ID", context.Finding.ID)
	writeMetaLine(&prompt, "Finding category", context.Finding.Category)
	writeMetaLine(&prompt, "Decision ID", context.Detail.ID)
	writeMetaLine(&prompt, "Selected", firstNonEmpty(context.Detail.SelectedTitle, context.Detail.Title))
	writeMetaLine(&prompt, "Reason", context.Finding.Reason)
	writeMetaLine(&prompt, "Decision valid until", context.Detail.ValidUntil)
	if context.Finding.DaysStale > 0 {
		writeMetaLine(&prompt, "Days stale", fmt.Sprintf("%d", context.Finding.DaysStale))
	}
	writeBlankLine(&prompt)

	writeParagraphSection(&prompt, "Decision Record Body", strings.TrimSpace(context.Detail.Body))
	writeParagraphSection(&prompt, "Why Selected", context.Detail.WhySelected)
	writeParagraphSection(&prompt, "Counterargument", context.Detail.CounterArgument)
	writeParagraphSection(&prompt, "Weakest Link", context.Detail.WeakestLink)
	writeStringListSection(&prompt, "Decision Invariants", context.Detail.Invariants, "- ")
	writeStringListSection(&prompt, "Admissibility", context.Detail.Admissibility, "- ")
	writeStringListSection(&prompt, "Affected Files", context.Detail.AffectedFiles, "- ")
	writeClaimsSection(&prompt, context.Detail.Claims)
	writeEvidenceAssuranceSection(&prompt, context.Assurance)
	writeEvidenceTimelineSection(&prompt, context.EvidenceTimeline)
	writeAlwaysStringListSection(
		&prompt,
		"Expired Items",
		context.ExpiredItems,
		"- ",
		"No expired decision or evidence items detected.",
	)
	writeStringListSection(&prompt, "Evidence Coverage Gaps", context.Detail.Evidence.CoverageGaps, "- ")
	writeCoverageSection(&prompt, context.Detail.CoverageModules)
	writeStringListSection(&prompt, "Coverage Warnings", context.Detail.CoverageWarnings, "- ")
	writeInstructionSection(
		&prompt,
		[]string{
			"Treat the DecisionRecord body and evidence timeline as audit history. Explain the stale condition before proposing a resolution.",
			"Use the weakest-link rule honestly: Decision R_eff is the minimum active evidence score, never an average.",
			"Present the available resolution options explicitly: Measure, Waive, Deprecate, or Reopen.",
			"Do not execute measure, waive, deprecate, reopen, or any other lifecycle action without explicit user confirmation.",
			"Preserve the original DecisionRecord body and evidence history for audit.",
		},
	)

	return prompt.String()
}

func writeEvidenceAssuranceSection(builder *strings.Builder, assurance artifact.WLNKSummary) {
	builder.WriteString("## R_eff Computation\n")
	builder.WriteString(fmt.Sprintf("- Decision R_eff: %.2f\n", assurance.REff))
	builder.WriteString("- Decision score uses the weakest-link rule: min(active evidence scores), never average.\n")
	builder.WriteString(fmt.Sprintf("- Active evidence counted: %d\n", assurance.EvidenceCount))
	builder.WriteString(fmt.Sprintf("- Supporting: %d\n", assurance.Supporting))
	builder.WriteString(fmt.Sprintf("- Weakening: %d\n", assurance.Weakening))
	builder.WriteString(fmt.Sprintf("- Refuting: %d\n", assurance.Refuting))
	if assurance.HasEvidence {
		builder.WriteString(fmt.Sprintf("- Weakest congruence level: CL%d\n", assurance.WeakestCL))
	}
	builder.WriteString(fmt.Sprintf("- Summary: %s\n", assurance.Summary))
	writeBlankLine(builder)
}

func writeEvidenceTimelineSection(
	builder *strings.Builder,
	timeline []governanceEvidenceTimelineItem,
) {
	builder.WriteString("## Evidence Timeline\n")
	if len(timeline) == 0 {
		builder.WriteString("- No evidence items attached.\n\n")
		return
	}

	builder.WriteString("- Ordered latest-first from the stored evidence history.\n")
	for _, item := range timeline {
		status := "active"
		if item.IsExpired {
			status = "expired"
		}
		scoreText := "excluded_from_r_eff"
		if item.CountsForREff {
			scoreText = fmt.Sprintf("score=%.2f", item.Score)
		}

		line := fmt.Sprintf(
			"- %s [%s] verdict=%s CL%d F%d %s %s",
			item.ID,
			firstNonEmpty(item.Type, "unknown"),
			firstNonEmpty(item.Verdict, "unknown"),
			item.CongruenceLevel,
			item.FormalityLevel,
			status,
			scoreText,
		)
		if strings.TrimSpace(item.ValidUntil) != "" {
			line += " valid_until=" + item.ValidUntil
		}
		builder.WriteString(line + "\n")

		if len(item.Claims) > 0 {
			builder.WriteString(fmt.Sprintf("  Claims: %s\n", strings.Join(item.Claims, ", ")))
		}
		if strings.TrimSpace(item.CarrierRef) != "" {
			builder.WriteString(fmt.Sprintf("  Carrier: %s\n", item.CarrierRef))
		}
		if strings.TrimSpace(item.Content) != "" {
			builder.WriteString(fmt.Sprintf("  Content: %s\n", item.Content))
		}
	}
	writeBlankLine(builder)
}

func writeAlwaysStringListSection(
	builder *strings.Builder,
	title string,
	values []string,
	prefix string,
	emptyValue string,
) {
	builder.WriteString(fmt.Sprintf("## %s\n", title))
	if len(values) == 0 {
		builder.WriteString(prefix)
		builder.WriteString(emptyValue)
		builder.WriteString("\n\n")
		return
	}

	for _, value := range values {
		builder.WriteString(prefix)
		builder.WriteString(value)
		builder.WriteString("\n")
	}
	writeBlankLine(builder)
}

func (a *App) loadDecisionDriftReport(decisionID string) (artifact.DriftReport, error) {
	reports, err := artifact.CheckDrift(a.ctx, a.store, a.projectRoot)
	if err != nil {
		return artifact.DriftReport{}, fmt.Errorf("load drift report for %s: %w", decisionID, err)
	}

	for _, report := range reports {
		if report.DecisionID != decisionID {
			continue
		}
		return report, nil
	}

	return artifact.DriftReport{}, fmt.Errorf("drift report for %s not found", decisionID)
}

func (a *App) loadDriftAffectedModules(driftedFiles []string) ([]CoverageModuleView, error) {
	if len(driftedFiles) == 0 || a == nil || a.store == nil {
		return []CoverageModuleView{}, nil
	}

	coverage, err := buildCoverageView(a.ctx, a.store.DB(), a.projectRoot, driftedFiles)
	if err != nil {
		return nil, err
	}

	modules := make([]CoverageModuleView, 0, len(coverage.Modules))
	for _, module := range coverage.Modules {
		if !module.Impacted {
			continue
		}
		modules = append(modules, module)
	}

	return modules, nil
}

func (a *App) loadDriftImpactedModules(driftedFiles []string) ([]artifact.ModuleImpact, error) {
	if len(driftedFiles) == 0 || a == nil || a.store == nil {
		return []artifact.ModuleImpact{}, nil
	}

	impacts, err := codebase.EnrichDriftWithImpact(a.ctx, a.store.DB(), driftedFiles)
	if err != nil {
		return nil, err
	}

	modules := make([]artifact.ModuleImpact, 0, len(impacts))
	for _, impact := range impacts {
		modules = append(modules, artifact.ModuleImpact{
			ModuleID:    impact.ModuleID,
			ModulePath:  impact.ModulePath,
			DecisionIDs: append([]string(nil), impact.DecisionIDs...),
			IsBlind:     impact.IsBlind,
		})
	}

	sort.Slice(modules, func(i int, j int) bool {
		return modules[i].ModulePath < modules[j].ModulePath
	})

	return modules, nil
}

func buildAdoptDriftPrompt(context governanceDriftAdoptionContext) string {
	var prompt strings.Builder

	writeSectionTitle(
		&prompt,
		"Adopt Drift Finding",
		firstNonEmpty(context.Detail.SelectedTitle, context.Detail.Title, context.Decision.Meta.Title),
	)
	writeMetaLine(&prompt, "Finding ID", context.Finding.ID)
	writeMetaLine(&prompt, "Finding category", context.Finding.Category)
	writeMetaLine(&prompt, "Decision ID", context.Detail.ID)
	writeMetaLine(&prompt, "Selected", firstNonEmpty(context.Detail.SelectedTitle, context.Detail.Title))
	writeMetaLine(&prompt, "Reason", context.Finding.Reason)
	writeBlankLine(&prompt)

	writeParagraphSection(&prompt, "Decision Record Body", strings.TrimSpace(context.Detail.Body))
	writeParagraphSection(&prompt, "Why Selected", context.Detail.WhySelected)
	writeParagraphSection(&prompt, "Counterargument", context.Detail.CounterArgument)
	writeParagraphSection(&prompt, "Weakest Link", context.Detail.WeakestLink)
	writeStringListSection(&prompt, "Decision Invariants", context.Detail.Invariants, "- ")
	writeStringListSection(&prompt, "Admissibility", context.Detail.Admissibility, "- ")
	writeStringListSection(&prompt, "Affected Files", context.Detail.AffectedFiles, "- ")
	writeDriftReportSection(&prompt, context.Report)
	writeGovernanceDiffSection(&prompt, context.FileDiffs)
	writeCoverageSection(&prompt, context.DirectModules)
	writeImpactedModuleSection(&prompt, context.ImpactedModule)
	writeStringListSection(&prompt, "Coverage Warnings", context.Detail.CoverageWarnings, "- ")
	writeInstructionSection(
		&prompt,
		[]string{
			"Treat the drift report and diffs as runtime evidence. Read them before proposing a resolution.",
			"Preserve every DecisionRecord invariant and admissibility boundary while investigating.",
			"Present the available resolution options explicitly: Re-baseline, Reopen, or Waive.",
			"Do not execute re-baseline, reopen, waive, or any other lifecycle action without explicit user confirmation.",
			"Preserve the original DecisionRecord body and evidence history for audit.",
		},
	)

	return prompt.String()
}

func writeDriftReportSection(builder *strings.Builder, report artifact.DriftReport) {
	if len(report.Files) == 0 {
		return
	}

	builder.WriteString("## Drift Report\n")
	for _, item := range report.Files {
		line := fmt.Sprintf("- %s status=%s", item.Path, item.Status)
		if item.LinesChanged != "" {
			line += " " + item.LinesChanged
		}
		builder.WriteString(line + "\n")
		for _, invariant := range item.Invariants {
			builder.WriteString(fmt.Sprintf("  Invariant: %s\n", invariant))
		}
	}
	writeBlankLine(builder)
}

func writeGovernanceDiffSection(builder *strings.Builder, diffs []governanceFileDiff) {
	if len(diffs) == 0 {
		return
	}

	builder.WriteString("## Diffs\n")
	for _, fileDiff := range diffs {
		builder.WriteString(fmt.Sprintf("### %s (%s)\n", fileDiff.Path, fileDiff.Status))
		builder.WriteString("```diff\n")
		builder.WriteString(strings.TrimSpace(firstNonEmpty(fileDiff.Diff, "No diff content available.")))
		builder.WriteString("\n```\n\n")
	}
}

func writeImpactedModuleSection(builder *strings.Builder, modules []artifact.ModuleImpact) {
	if len(modules) == 0 {
		return
	}

	builder.WriteString("## Dependency Impact Modules\n")
	for _, module := range modules {
		status := "governed"
		if module.IsBlind {
			status = "blind"
		}

		builder.WriteString(
			fmt.Sprintf(
				"- %s status=%s decisions=%d\n",
				firstNonEmpty(module.ModulePath, module.ModuleID),
				status,
				len(module.DecisionIDs),
			),
		)
		if len(module.DecisionIDs) > 0 {
			builder.WriteString(fmt.Sprintf("  Decision refs: %s\n", strings.Join(module.DecisionIDs, ", ")))
		}
	}
	writeBlankLine(builder)
}

func driftedFilePaths(items []artifact.DriftItem) []string {
	paths := make([]string, 0, len(items))
	seen := make(map[string]bool, len(items))

	for _, item := range items {
		path := strings.TrimSpace(item.Path)
		if path == "" || seen[path] {
			continue
		}

		seen[path] = true
		paths = append(paths, path)
	}

	sort.Strings(paths)
	return paths
}

func loadGovernanceFileDiffs(projectRoot string, items []artifact.DriftItem) []governanceFileDiff {
	diffs := make([]governanceFileDiff, 0, len(items))
	for _, item := range items {
		diffs = append(diffs, governanceFileDiff{
			Path:   item.Path,
			Status: item.Status,
			Diff:   loadGovernanceFileDiff(projectRoot, item),
		})
	}

	return diffs
}

func loadGovernanceFileDiff(projectRoot string, item artifact.DriftItem) string {
	if strings.TrimSpace(projectRoot) == "" || strings.TrimSpace(item.Path) == "" {
		return ""
	}

	// Prefer a real git diff when the repository can render one.
	diff := gitCommandOutput(projectRoot, "diff", "--no-color", "--", item.Path)
	if strings.TrimSpace(diff) != "" {
		return truncateGovernanceDiff(diff)
	}

	absPath := filepath.Join(projectRoot, filepath.FromSlash(item.Path))

	switch item.Status {
	case artifact.DriftAdded:
		diff = gitCommandOutput(projectRoot, "diff", "--no-color", "--no-index", "--", "/dev/null", absPath)
		if strings.TrimSpace(diff) != "" {
			return truncateGovernanceDiff(diff)
		}

		content, err := os.ReadFile(absPath)
		if err != nil {
			return "New file detected, but the current content could not be read."
		}

		return truncateGovernanceDiff("new file content\n" + string(content))
	case artifact.DriftMissing:
		return "Current file is missing from the worktree."
	case artifact.DriftNoBaseline:
		content, err := os.ReadFile(absPath)
		if err != nil {
			return "No baseline recorded for this file."
		}

		return truncateGovernanceDiff("current unbaselined file content\n" + string(content))
	case artifact.DriftModified:
		content, err := os.ReadFile(absPath)
		if err != nil {
			return "Modified file detected, but the current content could not be read."
		}

		return truncateGovernanceDiff("current file content\n" + string(content))
	default:
		return ""
	}
}

func gitCommandOutput(projectRoot string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil && trimmed == "" {
		return ""
	}

	return trimmed
}

func truncateGovernanceDiff(diff string) string {
	trimmed := strings.TrimSpace(diff)
	if trimmed == "" {
		return ""
	}

	lines := strings.Split(trimmed, "\n")
	if len(lines) > adoptDiffMaxLines {
		lines = append(
			lines[:adoptDiffMaxLines],
			fmt.Sprintf("... diff truncated after %d lines", adoptDiffMaxLines),
		)
		trimmed = strings.Join(lines, "\n")
	}

	if len(trimmed) > adoptDiffMaxChars {
		trimmed = truncate(trimmed, adoptDiffMaxChars)
	}

	return trimmed
}

func (s *desktopGovernanceStore) UpsertCandidates(ctx context.Context, candidates []ProblemCandidateView) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("desktop governance store is not initialized")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, candidate := range candidates {
		_, err := tx.ExecContext(
			ctx,
			`INSERT INTO desktop_problem_candidates (
				id,
				title,
				signal,
				acceptance,
				context,
				category,
				source_artifact_ref,
				source_title,
				status,
				problem_ref,
				created_at,
				updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				title = excluded.title,
				signal = excluded.signal,
				acceptance = excluded.acceptance,
				context = excluded.context,
				category = excluded.category,
				source_artifact_ref = excluded.source_artifact_ref,
				source_title = excluded.source_title,
				updated_at = excluded.updated_at`,
			candidate.ID,
			candidate.Title,
			candidate.Signal,
			candidate.Acceptance,
			candidate.Context,
			candidate.Category,
			candidate.SourceArtifactRef,
			candidate.SourceTitle,
			candidateStatusActive,
			"",
			nowRFC3339(),
			nowRFC3339(),
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *desktopGovernanceStore) ListActiveCandidates(
	ctx context.Context,
	candidates []ProblemCandidateView,
) ([]ProblemCandidateView, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("desktop governance store is not initialized")
	}

	stored, err := s.ListCandidates(ctx)
	if err != nil {
		return nil, err
	}

	statusByID := make(map[string]ProblemCandidateView, len(stored))
	for _, candidate := range stored {
		statusByID[candidate.ID] = candidate
	}

	active := make([]ProblemCandidateView, 0, len(candidates))
	for _, candidate := range candidates {
		storedCandidate, ok := statusByID[candidate.ID]
		if ok {
			candidate.Status = storedCandidate.Status
			candidate.ProblemRef = storedCandidate.ProblemRef
		}
		if candidate.Status == "" {
			candidate.Status = candidateStatusActive
		}
		if candidate.Status != candidateStatusActive {
			continue
		}
		active = append(active, candidate)
	}

	return active, nil
}

func (s *desktopGovernanceStore) ListCandidates(ctx context.Context) ([]ProblemCandidateView, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("desktop governance store is not initialized")
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT
			id,
			status,
			title,
			signal,
			acceptance,
			context,
			category,
			source_artifact_ref,
			source_title,
			COALESCE(problem_ref, '')
		FROM desktop_problem_candidates
		ORDER BY updated_at DESC, id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make([]ProblemCandidateView, 0)
	for rows.Next() {
		var candidate ProblemCandidateView
		err := rows.Scan(
			&candidate.ID,
			&candidate.Status,
			&candidate.Title,
			&candidate.Signal,
			&candidate.Acceptance,
			&candidate.Context,
			&candidate.Category,
			&candidate.SourceArtifactRef,
			&candidate.SourceTitle,
			&candidate.ProblemRef,
		)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}

	return candidates, rows.Err()
}

func (s *desktopGovernanceStore) GetCandidate(ctx context.Context, id string) (*ProblemCandidateView, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("desktop governance store is not initialized")
	}

	row := s.db.QueryRowContext(
		ctx,
		`SELECT
			id,
			status,
			title,
			signal,
			acceptance,
			context,
			category,
			source_artifact_ref,
			source_title,
			COALESCE(problem_ref, '')
		FROM desktop_problem_candidates
		WHERE id = ?`,
		id,
	)

	var candidate ProblemCandidateView
	err := row.Scan(
		&candidate.ID,
		&candidate.Status,
		&candidate.Title,
		&candidate.Signal,
		&candidate.Acceptance,
		&candidate.Context,
		&candidate.Category,
		&candidate.SourceArtifactRef,
		&candidate.SourceTitle,
		&candidate.ProblemRef,
	)
	if err != nil {
		return nil, err
	}

	return &candidate, nil
}

func (s *desktopGovernanceStore) SetCandidateStatus(
	ctx context.Context,
	id string,
	status string,
	problemRef string,
) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("desktop governance store is not initialized")
	}

	_, err := s.db.ExecContext(
		ctx,
		`UPDATE desktop_problem_candidates
		SET status = ?, problem_ref = ?, updated_at = ?
		WHERE id = ?`,
		status,
		nullString(problemRef),
		nowRFC3339(),
		id,
	)

	return err
}

func (s *desktopGovernanceStore) SetState(ctx context.Context, key string, value string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("desktop governance store is not initialized")
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO desktop_governance_state (state_key, state_value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(state_key) DO UPDATE SET
			state_value = excluded.state_value,
			updated_at = excluded.updated_at`,
		key,
		value,
		nowRFC3339(),
	)

	return err
}

func (s *desktopGovernanceStore) GetState(ctx context.Context, key string) (string, error) {
	if s == nil || s.db == nil {
		return "", fmt.Errorf("desktop governance store is not initialized")
	}

	var value string
	err := s.db.QueryRowContext(
		ctx,
		`SELECT COALESCE(state_value, '') FROM desktop_governance_state WHERE state_key = ?`,
		key,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	return value, nil
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func (a *App) canEmitEvents() bool {
	return a != nil && a.ctx != nil && a.ctx.Value("events") != nil
}

func (a *App) canUseNotifications() bool {
	return a != nil && a.ctx != nil && a.ctx.Value("frontend") != nil
}

func (a *App) emitEvent(name string, payload any) {
	if !a.canEmitEvents() {
		return
	}

	runtime.EventsEmit(a.ctx, name, payload)
}

func (a *App) pushNotification(notification DesktopNotification) {
	a.emitEvent("notification.push", notification)

	if !a.canUseNotifications() {
		return
	}

	cfg, err := a.GetConfig()
	if err != nil || cfg == nil || !cfg.NotifyEnabled {
		return
	}

	if err := runtime.SendNotification(a.ctx, runtime.NotificationOptions{
		ID:       notification.ID,
		Title:    notification.Title,
		Subtitle: notificationSubtitle(notification.Source),
		Body:     notification.Body,
	}); err != nil {
		a.emitAppError("notifications", err)
	}
}

func (a *App) notifyTaskState(state TaskState) {
	var title string
	var body string
	var tone string

	switch state.Status {
	case "completed":
		title = "Task completed"
		body = state.Title
		tone = "success"
	case "failed":
		title = "Task failed"
		body = firstNonEmpty(state.ErrorMessage, state.Title)
		tone = "warning"
	case "cancelled", "interrupted":
		title = "Task stopped"
		body = state.Title
		tone = "info"
	default:
		return
	}

	a.pushNotification(DesktopNotification{
		ID:     fmt.Sprintf("task-%s-%s", state.ID, state.Status),
		Title:  title,
		Body:   body,
		Tone:   tone,
		Source: "tasks",
	})
}

func notificationSubtitle(source string) string {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return "Haft"
	}

	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for index := range parts {
		if parts[index] == "" {
			continue
		}
		parts[index] = strings.ToUpper(parts[index][:1]) + parts[index][1:]
	}

	return strings.Join(parts, " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		return value
	}
	return ""
}
