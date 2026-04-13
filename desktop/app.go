package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/graph"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails binding layer. Exported methods become callable from the React frontend.
// This is a thin adapter — all domain logic lives in internal/artifact.
type App struct {
	ctx         context.Context
	store       *artifact.Store
	dbConn      *db.Store
	projectName string
	projectRoot string
	tasks       *taskRunner
	flows       *flowController
	governance  *governanceController
	terminals   *terminalManager
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	root := strings.TrimSpace(a.projectRoot)
	if root == "" {
		detectedRoot, err := findProjectRoot()
		if err != nil {
			fmt.Fprintf(os.Stderr, "haft desktop: no .haft/ directory found: %v\n", err)
			return
		}

		root = detectedRoot
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "haft desktop: failed to resolve project root: %v\n", err)
		return
	}
	a.projectRoot = absRoot

	haftDir := filepath.Join(a.projectRoot, ".haft")
	projCfg, err := project.Load(haftDir)
	if err != nil || projCfg == nil {
		fmt.Fprintf(os.Stderr, "haft desktop: failed to load project config: %v\n", err)
		return
	}
	a.projectName = projCfg.Name

	dbPath, err := projCfg.DBPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "haft desktop: failed to get DB path: %v\n", err)
		return
	}

	database, err := db.NewStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "haft desktop: failed to open DB: %v\n", err)
		return
	}
	// Enable WAL mode + busy timeout to prevent SQLITE_BUSY when
	// governance scanner and UI queries run concurrently.
	rawDB := database.GetRawDB()
	_, _ = rawDB.Exec("PRAGMA journal_mode=WAL")
	_, _ = rawDB.Exec("PRAGMA busy_timeout=5000")

	a.dbConn = database
	a.store = artifact.NewStore(rawDB)
	a.tasks = newTaskRunner(a, newDesktopTaskStore(database.GetRawDB()))
	a.flows = newFlowController(a, newDesktopFlowStore(database.GetRawDB()))
	a.governance = newGovernanceController(a, a.store, database.GetRawDB(), a.projectRoot)
	a.terminals = newTerminalManager(a)

	if err := a.tasks.restore(a.ctx, a.projectRoot); err != nil {
		fmt.Fprintf(os.Stderr, "haft desktop: failed to restore desktop tasks: %v\n", err)
	}

	if err := a.flows.reload(a.ctx); err != nil {
		fmt.Fprintf(os.Stderr, "haft desktop: failed to start flow scheduler: %v\n", err)
	}

	if a.canUseNotifications() {
		if err := runtime.InitializeNotifications(a.ctx); err != nil {
			a.emitAppError("notifications", err)
		}
	}

	if a.governance != nil && (a.canEmitEvents() || a.canUseNotifications()) {
		a.governance.start(a.ctx)
	}
}

func (a *App) shutdown(_ context.Context) {
	if a.governance != nil {
		a.governance.shutdown()
	}

	if a.flows != nil {
		a.flows.shutdown()
	}

	if a.terminals != nil {
		a.terminals.shutdown()
	}

	if a.canUseNotifications() {
		runtime.CleanupNotifications(a.ctx)
	}

	if a.tasks != nil {
		a.tasks.shutdown()
	}

	if a.dbConn != nil {
		a.dbConn.Close()
	}
}

// --- Binding methods: read-only views for the frontend ---

func (a *App) GetDashboard() (*DashboardView, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}

	problems, _ := a.store.ListActiveByKind(a.ctx, artifact.KindProblemCard, 100)
	decisions, _ := a.store.ListActiveByKind(a.ctx, artifact.KindDecisionRecord, 100)
	stale, _ := a.store.FindStaleArtifacts(a.ctx)
	notes, _ := a.store.ListActiveByKind(a.ctx, artifact.KindNote, 50)
	portfolios, _ := a.store.ListActiveByKind(a.ctx, artifact.KindSolutionPortfolio, 100)

	return &DashboardView{
		ProjectName:     a.projectName,
		ProblemCount:    len(problems),
		DecisionCount:   len(decisions),
		PortfolioCount:  len(portfolios),
		NoteCount:       len(notes),
		StaleCount:      len(stale),
		RecentProblems:  mapArtifacts(problems, toProblemView, 8),
		RecentDecisions: mapArtifacts(decisions, toDecisionView, 8),
		StaleItems:      mapArtifacts(stale, toArtifactView, 10),
	}, nil
}

func (a *App) ListProblems() ([]ProblemView, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}
	arts, err := a.store.ListActiveByKind(a.ctx, artifact.KindProblemCard, 200)
	if err != nil {
		return nil, err
	}
	return mapArtifacts(arts, toProblemView, 0), nil
}

func (a *App) ListDecisions() ([]DecisionView, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}
	arts, err := a.store.ListActiveByKind(a.ctx, artifact.KindDecisionRecord, 200)
	if err != nil {
		return nil, err
	}
	return mapArtifacts(arts, toDecisionView, 0), nil
}

func (a *App) GetProblem(id string) (*ProblemDetailView, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}
	art, err := a.store.Get(a.ctx, id)
	if err != nil {
		return nil, err
	}
	v := toProblemDetail(a.ctx, art, a.store)
	return &v, nil
}

func (a *App) GetDecision(id string) (*DecisionDetailView, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}

	_, view, err := a.loadDecisionDetail(id)
	if err != nil {
		return nil, err
	}

	return &view, nil
}

func (a *App) GetPortfolio(id string) (*PortfolioDetailView, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}
	art, err := a.store.Get(a.ctx, id)
	if err != nil {
		return nil, err
	}
	v := toPortfolioDetail(art)
	return &v, nil
}

func (a *App) ListPortfolios() ([]PortfolioSummaryView, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}
	arts, err := a.store.ListActiveByKind(a.ctx, artifact.KindSolutionPortfolio, 200)
	if err != nil {
		return nil, err
	}
	return mapArtifacts(arts, toPortfolioSummary, 0), nil
}

func (a *App) OpenDirectoryPicker() (string, error) {
	defaultDirectory := a.projectRoot
	if defaultDirectory == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			defaultDirectory = home
		}
	}

	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:                "Choose project directory",
		DefaultDirectory:     defaultDirectory,
		CanCreateDirectories: true,
	})
}

func (a *App) OpenPathInIDE(path string) error {
	targetPath := strings.TrimSpace(path)
	if targetPath == "" {
		return fmt.Errorf("path is required")
	}

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("open path %s: %w", absPath, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absPath)
	}

	cfg := defaultDesktopConfig()
	loadedConfig, err := loadDesktopConfig()
	if err == nil && loadedConfig != nil {
		cfg = *loadedConfig
	}

	command := buildIDECommand(cfg.DefaultIDE, absPath)
	commandPath, err := exec.LookPath(command[0])
	if err != nil {
		return fmt.Errorf("%s not found in PATH", command[0])
	}

	openCommand := exec.Command(commandPath, command[1:]...)

	if err := openCommand.Start(); err != nil {
		return fmt.Errorf("start %s: %w", command[0], err)
	}

	return nil
}

// ImplementDecision spawns an agent with the full decision context as prompt.
// This is the Decision-Anchored Implementation flow — the AIEE differentiator.
func (a *App) ImplementDecision(decisionID string, agentKind string, useWorktree bool, branchName string) (*TaskState, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}

	dec, detail, err := a.loadDecisionDetail(decisionID)
	if err != nil {
		return nil, fmt.Errorf("decision not found: %w", err)
	}

	problems := a.loadDecisionProblems(detail.ProblemRefs)

	// Enrich with invariants from ALL decisions governing the affected files,
	// not just this decision's own invariants. This is the knowledge graph value:
	// agents see the full architectural context, not just one decision's view.
	detail = a.enrichWithGraphInvariants(detail)

	prompt := buildImplementationPrompt(dec, detail, problems)

	if branchName == "" {
		branchName = fmt.Sprintf("implement-%s", decisionID)
	}

	return a.spawnTaskWithTitle(
		agentKind,
		prompt,
		useWorktree,
		branchName,
		decisionTaskTitle("Implement", detail),
	)
}

// VerifyDecision spawns an agent to verify a decision's claims.
func (a *App) VerifyDecision(decisionID string, agentKind string) (*TaskState, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}

	dec, detail, err := a.loadDecisionDetail(decisionID)
	if err != nil {
		return nil, fmt.Errorf("decision not found: %w", err)
	}

	prompt := buildVerificationPrompt(dec, detail)

	return a.spawnTaskWithTitle(
		agentKind,
		prompt,
		false,
		"",
		decisionTaskTitle("Verify", detail),
	)
}

func (a *App) SearchArtifacts(query string) ([]ArtifactView, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}
	arts, err := a.store.Search(a.ctx, query, 50)
	if err != nil {
		return nil, err
	}
	return mapArtifacts(arts, toArtifactView, 0), nil
}

// AssessComparisonReadiness evaluates whether a portfolio is ready for fair comparison.
// This is the probe-or-commit gate: commit (ready), probe (need data), widen (need variants), reroute (wrong framing).
func (a *App) AssessComparisonReadiness(portfolioID string) (*graph.ReadinessReport, error) {
	if a.dbConn == nil {
		return nil, fmt.Errorf("no database connection")
	}
	return graph.AssessReadiness(a.ctx, a.dbConn.GetRawDB(), portfolioID)
}

func (a *App) GetCoverage() (*CoverageView, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}

	coverage, err := buildCoverageView(a.ctx, a.store.DB(), a.projectRoot, nil)
	if err != nil {
		return nil, err
	}

	return &coverage, nil
}

func (a *App) GetGovernanceOverview() (*GovernanceOverviewView, error) {
	if a.governance == nil {
		return &GovernanceOverviewView{}, nil // not yet initialized (e.g. during project switch)
	}

	overview, err := a.governance.snapshotOrScan(a.ctx)
	if err != nil {
		return nil, err
	}

	return &overview, nil
}

func (a *App) RefreshGovernance() (*GovernanceOverviewView, error) {
	if a.governance == nil {
		return &GovernanceOverviewView{}, nil
	}

	overview, err := a.governance.scan(a.ctx, true)
	if err != nil {
		return nil, err
	}

	return &overview, nil
}

func (a *App) ListProblemCandidates() ([]ProblemCandidateView, error) {
	overview, err := a.GetGovernanceOverview()
	if err != nil {
		return nil, err
	}

	return overview.ProblemCandidates, nil
}

func (a *App) DismissProblemCandidate(id string) error {
	if a.governance == nil || a.governance.state == nil {
		return fmt.Errorf("governance controller is not initialized")
	}

	if _, err := a.governance.state.GetCandidate(a.ctx, strings.TrimSpace(id)); err != nil {
		return fmt.Errorf("load problem candidate %s: %w", id, err)
	}

	if err := a.governance.state.SetCandidateStatus(a.ctx, id, candidateStatusDismissed, ""); err != nil {
		return fmt.Errorf("dismiss problem candidate %s: %w", id, err)
	}

	if _, err := a.governance.scan(a.ctx, false); err != nil {
		return fmt.Errorf("refresh governance after dismissal: %w", err)
	}

	return nil
}

func (a *App) AdoptProblemCandidate(id string) (*ProblemDetailView, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}
	if a.governance == nil || a.governance.state == nil {
		return nil, fmt.Errorf("governance controller is not initialized")
	}

	candidate, err := a.governance.state.GetCandidate(a.ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, fmt.Errorf("load problem candidate %s: %w", id, err)
	}

	if candidate.Status == candidateStatusDismissed {
		return nil, fmt.Errorf("problem candidate %s has already been dismissed", id)
	}

	if candidate.Status == candidateStatusAdopted && candidate.ProblemRef != "" {
		problem, err := a.store.Get(a.ctx, candidate.ProblemRef)
		if err != nil {
			return nil, fmt.Errorf("load adopted problem %s: %w", candidate.ProblemRef, err)
		}
		view := toProblemDetail(a.ctx, problem, a.store)
		return &view, nil
	}

	created, _, err := artifact.FrameProblem(a.ctx, a.store, a.haftDir(), artifact.ProblemFrameInput{
		Title:         candidate.Title,
		Signal:        candidate.Signal,
		Acceptance:    candidate.Acceptance,
		Context:       candidate.Context,
		Mode:          "tactical",
		BlastRadius:   "Governance follow-up from the desktop decision loop",
		Reversibility: "high",
		Constraints: []string{
			"Validate the surfaced governance finding with fresh evidence before making irreversible changes.",
		},
		OptimizationTargets: []string{
			"Close the surfaced governance gap quickly",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("frame problem for candidate %s: %w", id, err)
	}

	if candidate.SourceArtifactRef != "" {
		if _, err := a.store.Get(a.ctx, candidate.SourceArtifactRef); err == nil {
			_ = a.store.AddLink(a.ctx, created.Meta.ID, candidate.SourceArtifactRef, "based_on")
		}
	}

	if err := a.governance.state.SetCandidateStatus(a.ctx, id, candidateStatusAdopted, created.Meta.ID); err != nil {
		return nil, fmt.Errorf("mark problem candidate %s adopted: %w", id, err)
	}

	if _, err := a.governance.scan(a.ctx, false); err != nil {
		return nil, fmt.Errorf("refresh governance after adoption: %w", err)
	}

	view := toProblemDetail(a.ctx, created, a.store)
	return &view, nil
}

func (a *App) loadDecisionDetail(id string) (*artifact.Artifact, DecisionDetailView, error) {
	art, err := a.store.Get(a.ctx, id)
	if err != nil {
		return nil, DecisionDetailView{}, err
	}

	affectedFiles, coverageModules, coverageWarnings := a.loadDecisionGovernance(art.Meta.ID)
	evidence := a.loadDecisionEvidence(art)
	view := toDecisionDetail(art, affectedFiles, coverageModules, coverageWarnings, evidence)

	return art, view, nil
}

func (a *App) loadDecisionGovernance(id string) ([]string, []CoverageModuleView, []string) {
	warnings := make([]string, 0)

	if a.store == nil {
		return nil, nil, []string{"Decision governance context is unavailable because no database is connected."}
	}

	affectedFileRows, err := a.store.GetAffectedFiles(a.ctx, id)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("Load affected files: %v", err))
	}

	affectedFiles := make([]string, 0, len(affectedFileRows))
	for _, file := range affectedFileRows {
		affectedFiles = append(affectedFiles, file.Path)
	}
	sort.Strings(affectedFiles)

	if len(affectedFiles) == 0 {
		warnings = append(warnings, "No affected files are recorded for this decision yet.")
		return affectedFiles, nil, warnings
	}

	coverage, err := buildCoverageView(a.ctx, a.store.DB(), a.projectRoot, affectedFiles)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("Coverage context is unavailable: %v", err))
		return affectedFiles, nil, warnings
	}

	impacted := make([]CoverageModuleView, 0)
	for _, module := range coverage.Modules {
		if !module.Impacted {
			continue
		}
		impacted = append(impacted, module)
	}

	if len(impacted) == 0 {
		warnings = append(warnings, "Affected files do not map to any scanned module yet.")
	}

	return affectedFiles, impacted, warnings
}

func (a *App) loadDecisionEvidence(art *artifact.Artifact) EvidenceSummaryView {
	summary := EvidenceSummaryView{
		Items:        []EvidenceItemView{},
		CoverageGaps: []string{},
	}

	if a.store == nil || art == nil {
		return summary
	}

	df := art.UnmarshalDecisionFields()
	summary.TotalClaims = len(df.Claims)

	items, err := a.store.GetEvidenceItems(a.ctx, art.Meta.ID)
	if err != nil {
		return summary
	}

	now := time.Now().UTC()
	coveredClaims := make(map[string]bool)

	for _, item := range items {
		isExpired := false
		if item.ValidUntil != "" {
			if t, err := time.Parse(time.RFC3339, item.ValidUntil); err == nil {
				isExpired = now.After(t)
			} else if t, err := time.Parse("2006-01-02", item.ValidUntil); err == nil {
				isExpired = now.After(t)
			}
		}

		for _, ref := range item.ClaimRefs {
			coveredClaims[ref] = true
		}
		for _, scope := range item.ClaimScope {
			coveredClaims[scope] = true
		}

		summary.Items = append(summary.Items, EvidenceItemView{
			ID:              item.ID,
			Type:            item.Type,
			Content:         item.Content,
			Verdict:         item.Verdict,
			FormalityLevel:  item.FormalityLevel,
			CongruenceLevel: item.CongruenceLevel,
			ClaimRefs:       safeStrings(item.ClaimRefs),
			ValidUntil:      item.ValidUntil,
			IsExpired:       isExpired,
		})
	}

	summary.CoveredClaims = len(coveredClaims)

	// Find coverage gaps: claims that have no evidence
	for _, claim := range df.Claims {
		if !coveredClaims[claim.ID] {
			summary.CoverageGaps = append(summary.CoverageGaps, claim.ID+": "+claim.Claim)
		}
	}

	return summary
}

func (a *App) loadDecisionProblems(problemRefs []string) []*artifact.Artifact {
	if a.store == nil || len(problemRefs) == 0 {
		return nil
	}

	problems := make([]*artifact.Artifact, 0, len(problemRefs))
	for _, problemRef := range problemRefs {
		problem, err := a.store.Get(a.ctx, problemRef)
		if err != nil {
			continue
		}
		problems = append(problems, problem)
	}

	return problems
}

// enrichWithGraphInvariants queries the knowledge graph for invariants
// from OTHER decisions that govern the same affected files. Deduplicates
// against the decision's own invariants and appends with source attribution.
func (a *App) enrichWithGraphInvariants(detail DecisionDetailView) DecisionDetailView {
	if a.dbConn == nil || len(detail.AffectedFiles) == 0 {
		return detail
	}

	gs := graph.NewStore(a.dbConn.GetRawDB())

	// Collect existing invariant texts for dedup
	existing := make(map[string]bool, len(detail.Invariants))
	for _, inv := range detail.Invariants {
		existing[inv] = true
	}

	var extra []string
	for _, filePath := range detail.AffectedFiles {
		invariants, err := gs.FindInvariantsForFile(a.ctx, filePath)
		if err != nil {
			continue
		}
		for _, inv := range invariants {
			// Skip invariants from this decision (already included)
			if inv.DecisionID == detail.ID {
				continue
			}
			tagged := fmt.Sprintf("[%s] %s", inv.DecisionID, inv.Text)
			if !existing[tagged] && !existing[inv.Text] {
				existing[tagged] = true
				extra = append(extra, tagged)
			}
		}
	}

	if len(extra) > 0 {
		// Append graph-sourced invariants after the decision's own
		enriched := make([]string, 0, len(detail.Invariants)+len(extra))
		enriched = append(enriched, detail.Invariants...)
		enriched = append(enriched, extra...)
		detail.Invariants = enriched
	}

	return detail
}

// --- Helpers ---

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".haft")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no .haft/ found")
		}
		dir = parent
	}
}

func mapArtifacts[T any](arts []*artifact.Artifact, fn func(*artifact.Artifact) T, limit int) []T {
	if limit <= 0 || limit > len(arts) {
		limit = len(arts)
	}
	result := make([]T, 0, limit)
	for i := range limit {
		result = append(result, fn(arts[i]))
	}
	return result
}

func (a *App) emitAppError(scope string, err error) {
	if err == nil {
		return
	}

	a.emitEvent("app.error", map[string]string{
		"scope":   scope,
		"message": err.Error(),
	})
}
