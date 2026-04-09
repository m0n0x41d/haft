package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
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
	a.dbConn = database
	a.store = artifact.NewStore(database.GetRawDB())
	a.tasks = newTaskRunner(a, newDesktopTaskStore(database.GetRawDB()))

	if err := a.tasks.restore(a.ctx, a.projectRoot); err != nil {
		fmt.Fprintf(os.Stderr, "haft desktop: failed to restore desktop tasks: %v\n", err)
	}
}

func (a *App) shutdown(_ context.Context) {
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
	art, err := a.store.Get(a.ctx, id)
	if err != nil {
		return nil, err
	}
	v := toDecisionDetail(art)
	return &v, nil
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

	dec, err := a.store.Get(a.ctx, decisionID)
	if err != nil {
		return nil, fmt.Errorf("decision not found: %w", err)
	}

	df := dec.UnmarshalDecisionFields()

	// Build implementation brief from decision record
	var brief strings.Builder
	brief.WriteString(fmt.Sprintf("## Implement Decision: %s\n\n", dec.Meta.Title))
	brief.WriteString(fmt.Sprintf("Selected: %s\n\n", df.SelectedTitle))

	// Load linked problem for context
	for _, pRef := range df.ProblemRefs {
		prob, err := a.store.Get(a.ctx, pRef)
		if err != nil {
			continue
		}
		pf := prob.UnmarshalProblemFields()
		brief.WriteString("## Problem\n")
		brief.WriteString(fmt.Sprintf("Signal: %s\n", pf.Signal))
		if len(pf.Constraints) > 0 {
			brief.WriteString("Constraints:\n")
			for _, c := range pf.Constraints {
				brief.WriteString(fmt.Sprintf("- %s\n", c))
			}
		}
		brief.WriteString("\n")
	}

	brief.WriteString("## Why Selected\n")
	brief.WriteString(df.WhySelected + "\n\n")

	if len(df.Invariants) > 0 {
		brief.WriteString("## Invariants (MUST hold at all times)\n")
		for _, inv := range df.Invariants {
			brief.WriteString(fmt.Sprintf("- %s\n", inv))
		}
		brief.WriteString("\n")
	}

	if len(df.Admissibility) > 0 {
		brief.WriteString("## Not Acceptable\n")
		for _, adm := range df.Admissibility {
			brief.WriteString(fmt.Sprintf("- %s\n", adm))
		}
		brief.WriteString("\n")
	}

	if len(df.PostConds) > 0 {
		brief.WriteString("## Post-conditions (verify after implementation)\n")
		for _, pc := range df.PostConds {
			brief.WriteString(fmt.Sprintf("- [ ] %s\n", pc))
		}
		brief.WriteString("\n")
	}

	if len(df.Claims) > 0 {
		brief.WriteString("## Claims to verify\n")
		for _, c := range df.Claims {
			brief.WriteString(fmt.Sprintf("- %s (threshold: %s)\n", c.Claim, c.Threshold))
		}
		brief.WriteString("\n")
	}

	brief.WriteString("## Instructions\n")
	brief.WriteString("Implement the selected approach. Follow all invariants strictly.\n")
	brief.WriteString("After implementation, verify each post-condition.\n")
	brief.WriteString("You have access to haft MCP tools for recording progress.\n")

	prompt := brief.String()

	if branchName == "" {
		branchName = fmt.Sprintf("implement-%s", decisionID)
	}

	return a.SpawnTask(agentKind, prompt, useWorktree, branchName)
}

// VerifyDecision spawns an agent to verify a decision's claims.
func (a *App) VerifyDecision(decisionID string, agentKind string) (*TaskState, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}

	dec, err := a.store.Get(a.ctx, decisionID)
	if err != nil {
		return nil, fmt.Errorf("decision not found: %w", err)
	}

	df := dec.UnmarshalDecisionFields()

	var prompt strings.Builder
	prompt.WriteString(fmt.Sprintf("## Verify Decision: %s\n\n", dec.Meta.Title))
	prompt.WriteString("Check each claim below. For each:\n")
	prompt.WriteString("1. Gather evidence (run commands, read files, check metrics)\n")
	prompt.WriteString("2. Assess: supported / weakened / refuted\n")
	prompt.WriteString("3. Call haft_decision(action=\"measure\") with your findings\n\n")

	prompt.WriteString("## Claims\n")
	for _, c := range df.Claims {
		prompt.WriteString(fmt.Sprintf("- **%s**: %s\n", c.ID, c.Claim))
		prompt.WriteString(fmt.Sprintf("  Observable: %s\n", c.Observable))
		prompt.WriteString(fmt.Sprintf("  Threshold: %s\n", c.Threshold))
		prompt.WriteString(fmt.Sprintf("  Current status: %s\n\n", c.Status))
	}

	prompt.WriteString("Do NOT skip claims. Do NOT fabricate evidence.\n")

	return a.SpawnTask(agentKind, prompt.String(), false, "")
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

// staleQuery is a read-only WAL-compatible query helper
var _ *sql.DB // suppress unused import if needed

func (a *App) emitAppError(scope string, err error) {
	if err == nil || a.ctx == nil {
		return
	}

	runtime.EventsEmit(a.ctx, "app.error", map[string]string{
		"scope":   scope,
		"message": err.Error(),
	})
}
