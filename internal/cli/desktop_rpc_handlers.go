package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/graph"
	"github.com/m0n0x41d/haft/internal/project"
)

// ── Artifact authoring ──────────────────────────────────────────────

func handleCreateProblem(env *rpcEnv, w io.Writer) error {
	var input artifact.ProblemFrameInput
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	art, mdPath, err := artifact.FrameProblem(env.ctx, env.store, env.haftDir, input)
	if err != nil {
		return fmt.Errorf("frame problem: %w", err)
	}

	return writeResult(w, map[string]any{
		"id":         art.Meta.ID,
		"title":      art.Meta.Title,
		"kind":       string(art.Meta.Kind),
		"status":     string(art.Meta.Status),
		"md_path":    mdPath,
		"created_at": art.Meta.CreatedAt,
	})
}

func handleCreateDecision(env *rpcEnv, w io.Writer) error {
	var input artifact.DecideInput
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	art, mdPath, err := artifact.Decide(env.ctx, env.store, env.haftDir, input)
	if err != nil {
		return fmt.Errorf("decide: %w", err)
	}

	return writeResult(w, map[string]any{
		"id":         art.Meta.ID,
		"title":      art.Meta.Title,
		"kind":       string(art.Meta.Kind),
		"status":     string(art.Meta.Status),
		"md_path":    mdPath,
		"created_at": art.Meta.CreatedAt,
	})
}

func handleCreatePortfolio(env *rpcEnv, w io.Writer) error {
	var input artifact.ExploreInput
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	art, mdPath, err := artifact.ExploreSolutions(env.ctx, env.store, env.haftDir, input)
	if err != nil {
		return fmt.Errorf("explore solutions: %w", err)
	}

	return writeResult(w, map[string]any{
		"id":         art.Meta.ID,
		"title":      art.Meta.Title,
		"kind":       string(art.Meta.Kind),
		"status":     string(art.Meta.Status),
		"md_path":    mdPath,
		"created_at": art.Meta.CreatedAt,
	})
}

func handleCharacterize(env *rpcEnv, w io.Writer) error {
	var input artifact.CharacterizeInput
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	art, mdPath, err := artifact.CharacterizeProblem(env.ctx, env.store, env.haftDir, input)
	if err != nil {
		return fmt.Errorf("characterize: %w", err)
	}

	return writeResult(w, map[string]any{
		"id":         art.Meta.ID,
		"title":      art.Meta.Title,
		"kind":       string(art.Meta.Kind),
		"status":     string(art.Meta.Status),
		"md_path":    mdPath,
		"created_at": art.Meta.CreatedAt,
	})
}

func handleComparePortfolio(env *rpcEnv, w io.Writer) error {
	var input artifact.CompareInput
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	art, mdPath, err := artifact.CompareSolutions(env.ctx, env.store, env.haftDir, input)
	if err != nil {
		return fmt.Errorf("compare: %w", err)
	}

	return writeResult(w, map[string]any{
		"id":         art.Meta.ID,
		"title":      art.Meta.Title,
		"kind":       string(art.Meta.Kind),
		"status":     string(art.Meta.Status),
		"md_path":    mdPath,
		"created_at": art.Meta.CreatedAt,
	})
}

// ── Decision lifecycle ──────────────────────────────────────────────

func handleImplementDecision(env *rpcEnv, w io.Writer) error {
	var input struct {
		DecisionRef string `json:"decision_ref"`
	}
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	brief, err := artifact.Apply(env.ctx, env.store, input.DecisionRef)
	if err != nil {
		return fmt.Errorf("generate implementation brief: %w", err)
	}

	return writeResult(w, map[string]any{
		"decision_ref": input.DecisionRef,
		"brief":        brief,
	})
}

func handleVerifyDecision(env *rpcEnv, w io.Writer) error {
	var input struct {
		DecisionRef string `json:"decision_ref"`
	}
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	gs := graph.NewStore(env.rawDB)
	results, err := graph.VerifyInvariants(env.ctx, gs, env.rawDB, input.DecisionRef)
	if err != nil {
		return fmt.Errorf("verify invariants: %w", err)
	}

	return writeResult(w, map[string]any{
		"decision_ref": input.DecisionRef,
		"invariants":   results,
	})
}

func handleBaseline(env *rpcEnv, w io.Writer) error {
	var input artifact.BaselineInput
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	files, err := artifact.Baseline(env.ctx, env.store, env.projectRoot, input)
	if err != nil {
		return fmt.Errorf("baseline: %w", err)
	}

	return writeResult(w, map[string]any{
		"decision_ref":   input.DecisionRef,
		"affected_files": files,
	})
}

func handleMeasure(env *rpcEnv, w io.Writer) error {
	var input artifact.MeasureInput
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	art, err := artifact.Measure(env.ctx, env.store, env.haftDir, input)
	if err != nil {
		return fmt.Errorf("measure: %w", err)
	}

	return writeResult(w, map[string]any{
		"id":           art.Meta.ID,
		"decision_ref": input.DecisionRef,
		"verdict":      input.Verdict,
	})
}

// ── Artifact lifecycle ──────────────────────────────────────────────

func handleWaive(env *rpcEnv, w io.Writer) error {
	var input struct {
		ArtifactRef   string `json:"artifact_ref"`
		Reason        string `json:"reason"`
		NewValidUntil string `json:"new_valid_until"`
		Evidence      string `json:"evidence"`
	}
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	art, err := artifact.WaiveArtifact(env.ctx, env.store, env.haftDir, input.ArtifactRef, input.Reason, input.NewValidUntil, input.Evidence)
	if err != nil {
		return fmt.Errorf("waive: %w", err)
	}

	return writeResult(w, map[string]any{
		"id":     art.Meta.ID,
		"status": string(art.Meta.Status),
	})
}

func handleDeprecate(env *rpcEnv, w io.Writer) error {
	var input struct {
		ArtifactRef string `json:"artifact_ref"`
		Reason      string `json:"reason"`
	}
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	art, err := artifact.DeprecateArtifact(env.ctx, env.store, env.haftDir, input.ArtifactRef, input.Reason)
	if err != nil {
		return fmt.Errorf("deprecate: %w", err)
	}

	return writeResult(w, map[string]any{
		"id":     art.Meta.ID,
		"status": string(art.Meta.Status),
	})
}

func handleReopen(env *rpcEnv, w io.Writer) error {
	var input struct {
		DecisionRef string `json:"decision_ref"`
		Reason      string `json:"reason"`
	}
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	decision, newProblem, err := artifact.ReopenDecision(env.ctx, env.store, env.haftDir, input.DecisionRef, input.Reason)
	if err != nil {
		return fmt.Errorf("reopen: %w", err)
	}

	return writeResult(w, map[string]any{
		"decision_id":     decision.Meta.ID,
		"decision_status": string(decision.Meta.Status),
		"new_problem_id":  newProblem.Meta.ID,
	})
}

// ── Problem candidates ──────────────────────────────────────────────

func handleAdoptCandidate(env *rpcEnv, w io.Writer) error {
	// Accept both shapes on stdin:
	//   { "id": "..." }                                     — minimal (desktop frontend sends this)
	//   { "id": "...", "title": "...", "signal": "...", ... } — full (CLI / test callers)
	//
	// Missing fields are looked up from the desktop_problem_candidates table
	// by id. The row is populated when the governance scanner surfaces the
	// candidate, so the full payload is always recoverable server-side.
	var input struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		Signal     string `json:"signal"`
		Acceptance string `json:"acceptance"`
		Context    string `json:"context"`
	}
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	id := strings.TrimSpace(input.ID)
	if id == "" {
		return fmt.Errorf("adopt-candidate requires id")
	}

	// Fill in any fields the caller omitted by querying the candidate row.
	if input.Title == "" || input.Signal == "" || input.Acceptance == "" || input.Context == "" {
		var title, signal, acceptance, context sql.NullString
		err := env.rawDB.QueryRowContext(env.ctx,
			`SELECT title, signal, acceptance, context
			 FROM desktop_problem_candidates
			 WHERE id = ? AND status = 'active'`,
			id,
		).Scan(&title, &signal, &acceptance, &context)
		if err != nil {
			return fmt.Errorf("lookup candidate %s: %w", id, err)
		}
		if input.Title == "" {
			input.Title = title.String
		}
		if input.Signal == "" {
			input.Signal = signal.String
		}
		if input.Acceptance == "" {
			input.Acceptance = acceptance.String
		}
		if input.Context == "" {
			input.Context = context.String
		}
	}

	if input.Title == "" || input.Signal == "" || input.Acceptance == "" {
		return fmt.Errorf("candidate %s missing title, signal, or acceptance after DB lookup", id)
	}

	art, _, err := artifact.FrameProblem(env.ctx, env.store, env.haftDir, artifact.ProblemFrameInput{
		Title:               input.Title,
		Signal:              input.Signal,
		Acceptance:          input.Acceptance,
		Context:             input.Context,
		Mode:                "tactical",
		BlastRadius:         "Governance follow-up from the desktop decision loop",
		Reversibility:       "high",
		Constraints:         []string{"Validate the surfaced governance finding with fresh evidence before making irreversible changes."},
		OptimizationTargets: []string{"Close the surfaced governance gap quickly"},
	})
	if err != nil {
		return fmt.Errorf("adopt candidate: %w", err)
	}

	// Mark the candidate as adopted so it stops appearing in the active list
	// and links back to the framed problem.
	_, _ = env.rawDB.ExecContext(env.ctx,
		`UPDATE desktop_problem_candidates
		 SET status = 'adopted', problem_ref = ?, updated_at = ?
		 WHERE id = ?`,
		art.Meta.ID, time.Now().UTC().Format(time.RFC3339), id,
	)

	return writeResult(w, map[string]any{
		"candidate_id":  id,
		"problem_id":    art.Meta.ID,
		"problem_title": art.Meta.Title,
	})
}

func handleDismissCandidate(env *rpcEnv, w io.Writer) error {
	var input struct {
		ID string `json:"id"`
	}
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	// The candidate store is in the desktop DB (desktop_problem_candidates table).
	// For the stateless RPC, we set status = 'dismissed' directly.
	_, err := env.rawDB.ExecContext(env.ctx,
		`UPDATE desktop_problem_candidates SET status = 'dismissed', updated_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), strings.TrimSpace(input.ID),
	)
	if err != nil {
		return fmt.Errorf("dismiss candidate: %w", err)
	}

	return writeResult(w, map[string]any{
		"id":     input.ID,
		"status": "dismissed",
	})
}

// ── Flow management ─────────────────────────────────────────────────

type rpcFlowInput struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	TemplateID  string `json:"template_id"`
	Agent       string `json:"agent"`
	Prompt      string `json:"prompt"`
	Schedule    string `json:"schedule"`
	Branch      string `json:"branch"`
	UseWorktree bool   `json:"use_worktree"`
	Enabled     bool   `json:"enabled"`
}

type rpcFlowResult struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Agent       string `json:"agent"`
	Prompt      string `json:"prompt"`
	Schedule    string `json:"schedule"`
	Branch      string `json:"branch"`
	UseWorktree bool   `json:"use_worktree"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func handleCreateFlow(env *rpcEnv, w io.Writer) error {
	var input rpcFlowInput
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	if strings.TrimSpace(input.Title) == "" {
		return fmt.Errorf("flow title is required")
	}
	if strings.TrimSpace(input.Prompt) == "" {
		return fmt.Errorf("flow prompt is required")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	flowID := strings.TrimSpace(input.ID)
	if flowID == "" {
		flowID = fmt.Sprintf("flow-%d", time.Now().UnixNano())
	}

	_, err := env.rawDB.ExecContext(env.ctx,
		`INSERT INTO desktop_flows (
			id, project_name, project_path, title, description, template_id,
			agent, prompt, schedule, branch, use_worktree, enabled,
			last_task_id, last_run_at, next_run_at, last_error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', NULL, NULL, '', ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title=excluded.title, description=excluded.description,
			agent=excluded.agent, prompt=excluded.prompt, schedule=excluded.schedule,
			branch=excluded.branch, use_worktree=excluded.use_worktree,
			enabled=excluded.enabled, updated_at=excluded.updated_at`,
		flowID, filepath.Base(env.projectRoot), env.projectRoot,
		strings.TrimSpace(input.Title), strings.TrimSpace(input.Description),
		strings.TrimSpace(input.TemplateID), strings.TrimSpace(input.Agent),
		strings.TrimSpace(input.Prompt), strings.TrimSpace(input.Schedule),
		strings.TrimSpace(input.Branch), rpcBoolToInt(input.UseWorktree),
		rpcBoolToInt(input.Enabled), now, now,
	)
	if err != nil {
		return fmt.Errorf("create flow: %w", err)
	}

	return writeResult(w, rpcFlowResult{
		ID:          flowID,
		Title:       input.Title,
		Description: input.Description,
		Agent:       input.Agent,
		Prompt:      input.Prompt,
		Schedule:    input.Schedule,
		Branch:      input.Branch,
		UseWorktree: input.UseWorktree,
		Enabled:     input.Enabled,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
}

func handleUpdateFlow(env *rpcEnv, w io.Writer) error {
	var input rpcFlowInput
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	flowID := strings.TrimSpace(input.ID)
	if flowID == "" {
		return fmt.Errorf("flow id is required")
	}

	now := time.Now().UTC().Format(time.RFC3339)

	res, err := env.rawDB.ExecContext(env.ctx,
		`UPDATE desktop_flows SET
			title=?, description=?, template_id=?, agent=?, prompt=?,
			schedule=?, branch=?, use_worktree=?, enabled=?, updated_at=?
		WHERE id=?`,
		strings.TrimSpace(input.Title), strings.TrimSpace(input.Description),
		strings.TrimSpace(input.TemplateID), strings.TrimSpace(input.Agent),
		strings.TrimSpace(input.Prompt), strings.TrimSpace(input.Schedule),
		strings.TrimSpace(input.Branch), rpcBoolToInt(input.UseWorktree),
		rpcBoolToInt(input.Enabled), now, flowID,
	)
	if err != nil {
		return fmt.Errorf("update flow: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("flow %s not found", flowID)
	}

	return writeResult(w, rpcFlowResult{
		ID:          flowID,
		Title:       input.Title,
		Description: input.Description,
		Agent:       input.Agent,
		Prompt:      input.Prompt,
		Schedule:    input.Schedule,
		Branch:      input.Branch,
		UseWorktree: input.UseWorktree,
		Enabled:     input.Enabled,
		UpdatedAt:   now,
	})
}

func handleToggleFlow(env *rpcEnv, w io.Writer) error {
	var input struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	flowID := strings.TrimSpace(input.ID)
	if flowID == "" {
		return fmt.Errorf("flow id is required")
	}

	now := time.Now().UTC().Format(time.RFC3339)

	res, err := env.rawDB.ExecContext(env.ctx,
		`UPDATE desktop_flows SET enabled=?, updated_at=? WHERE id=?`,
		rpcBoolToInt(input.Enabled), now, flowID,
	)
	if err != nil {
		return fmt.Errorf("toggle flow: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("flow %s not found", flowID)
	}

	return writeResult(w, map[string]any{
		"id":      flowID,
		"enabled": input.Enabled,
	})
}

func handleDeleteFlow(env *rpcEnv, w io.Writer) error {
	var input struct {
		ID string `json:"id"`
	}
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	flowID := strings.TrimSpace(input.ID)
	if flowID == "" {
		return fmt.Errorf("flow id is required")
	}

	res, err := env.rawDB.ExecContext(env.ctx,
		`DELETE FROM desktop_flows WHERE id=?`, flowID,
	)
	if err != nil {
		return fmt.Errorf("delete flow: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("flow %s not found", flowID)
	}

	return writeResult(w, map[string]any{
		"id":      flowID,
		"deleted": true,
	})
}

func handleRunFlowNow(env *rpcEnv, w io.Writer) error {
	var input struct {
		ID string `json:"id"`
	}
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	flowID := strings.TrimSpace(input.ID)
	if flowID == "" {
		return fmt.Errorf("flow id is required")
	}

	// Mark the flow as triggered — the desktop app's flow controller
	// picks up the execution. The RPC bridge cannot spawn PTY tasks.
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := env.rawDB.ExecContext(env.ctx,
		`UPDATE desktop_flows SET last_run_at=?, updated_at=? WHERE id=?`,
		now, now, flowID,
	)
	if err != nil {
		return fmt.Errorf("run flow: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("flow %s not found", flowID)
	}

	return writeResult(w, map[string]any{
		"id":           flowID,
		"triggered_at": now,
	})
}

// ── Project management ──────────────────────────────────────────────

type rpcProjectInfo struct {
	Path string `json:"path"`
	Name string `json:"name"`
	ID   string `json:"id"`
}

type rpcProjectRegistry struct {
	Projects   []rpcRegisteredProject `json:"projects"`
	ActivePath string                 `json:"active_path,omitempty"`
}

type rpcRegisteredProject struct {
	Path string `json:"path"`
	Name string `json:"name"`
	ID   string `json:"id"`
}

func handleSwitchProject(env *rpcEnv, w io.Writer) error {
	var input struct {
		Path string `json:"path"`
	}
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	path := strings.TrimSpace(input.Path)
	if path == "" {
		return fmt.Errorf("path is required")
	}

	haftDir := filepath.Join(path, ".haft")
	if _, err := os.Stat(haftDir); os.IsNotExist(err) {
		return fmt.Errorf("no .haft/ directory in %s", path)
	}

	cfg, err := project.Load(haftDir)
	if err != nil || cfg == nil {
		return fmt.Errorf("cannot load project config from %s: %w", haftDir, err)
	}

	dbPath, err := cfg.DBPath()
	if err != nil {
		return fmt.Errorf("cannot resolve DB path: %w", err)
	}

	testDB, err := db.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("cannot open DB for %s: %w", path, err)
	}
	_ = testDB.Close()

	// Persist the switch in the registry so `list_projects` returns the
	// right `is_active` flag and a subsequent `haft desktop` launch opens
	// this project. Also add the project if it wasn't already registered —
	// switch-project doubles as register-and-activate for recently-opened
	// directories.
	reg, err := rpcLoadRegistry()
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}
	registered := false
	for _, p := range reg.Projects {
		if p.Path == path {
			registered = true
			break
		}
	}
	if !registered {
		reg.Projects = append(reg.Projects, rpcRegisteredProject{
			Path: path,
			Name: cfg.Name,
			ID:   cfg.ID,
		})
	}
	reg.ActivePath = path
	if err := rpcSaveRegistry(reg); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	return writeResult(w, rpcProjectInfo{
		Path: path,
		Name: cfg.Name,
		ID:   cfg.ID,
	})
}

func handleAddProject(env *rpcEnv, w io.Writer) error {
	var input struct {
		Path string `json:"path"`
	}
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	path := strings.TrimSpace(input.Path)
	if path == "" {
		return fmt.Errorf("path is required")
	}

	haftDir := filepath.Join(path, ".haft")
	if _, err := os.Stat(haftDir); os.IsNotExist(err) {
		return fmt.Errorf("no .haft/ directory found in %s", path)
	}

	cfg, err := project.Load(haftDir)
	if err != nil || cfg == nil {
		return fmt.Errorf("load project config: %w", err)
	}

	reg, err := rpcLoadRegistry()
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	// Add if not already registered
	found := false
	for _, p := range reg.Projects {
		if p.Path == path {
			found = true
			break
		}
	}
	if !found {
		reg.Projects = append(reg.Projects, rpcRegisteredProject{
			Path: path,
			Name: cfg.Name,
			ID:   cfg.ID,
		})
		if err := rpcSaveRegistry(reg); err != nil {
			return fmt.Errorf("save registry: %w", err)
		}
	}

	return writeResult(w, rpcProjectInfo{
		Path: path,
		Name: cfg.Name,
		ID:   cfg.ID,
	})
}

func handleInitProject(env *rpcEnv, w io.Writer) error {
	var input struct {
		Path string `json:"path"`
	}
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	path := strings.TrimSpace(input.Path)
	if path == "" {
		return fmt.Errorf("path is required")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("access path %s: %w", absPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absPath)
	}

	haftDir := filepath.Join(absPath, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		return fmt.Errorf("create .haft/: %w", err)
	}

	cfg, err := project.Create(haftDir, absPath)
	if err != nil {
		return fmt.Errorf("create project config: %w", err)
	}

	dbPath, err := cfg.DBPath()
	if err != nil {
		return fmt.Errorf("resolve DB path: %w", err)
	}

	database, err := db.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	_ = database.Close()

	// Register in project registry
	reg, err := rpcLoadRegistry()
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}
	found := false
	for _, p := range reg.Projects {
		if p.Path == absPath {
			found = true
			break
		}
	}
	if !found {
		reg.Projects = append(reg.Projects, rpcRegisteredProject{
			Path: absPath,
			Name: cfg.Name,
			ID:   cfg.ID,
		})
		_ = rpcSaveRegistry(reg)
	}

	return writeResult(w, rpcProjectInfo{
		Path: absPath,
		Name: cfg.Name,
		ID:   cfg.ID,
	})
}

// ── Governance & analysis ───────────────────────────────────────────

func handleRefreshGovernance(env *rpcEnv, w io.Writer) error {
	staleItems, err := artifact.ScanStale(env.ctx, env.store, env.projectRoot)
	if err != nil {
		return fmt.Errorf("scan stale: %w", err)
	}

	driftReports, err := artifact.CheckDrift(env.ctx, env.store, env.projectRoot)
	if err != nil {
		return fmt.Errorf("check drift: %w", err)
	}

	return writeResult(w, map[string]any{
		"stale_count": len(staleItems),
		"stale":       staleItems,
		"drift_count": len(driftReports),
		"drift":       driftReports,
		"scanned_at":  time.Now().UTC().Format(time.RFC3339),
	})
}

func handleGetGovernanceOverview(env *rpcEnv, w io.Writer) error {
	staleItems, err := artifact.ScanStale(env.ctx, env.store, env.projectRoot)
	if err != nil {
		return fmt.Errorf("scan stale: %w", err)
	}

	driftReports, err := artifact.CheckDrift(env.ctx, env.store, env.projectRoot)
	if err != nil {
		return fmt.Errorf("check drift: %w", err)
	}

	coverage, _ := codebase.ComputeCoverage(env.ctx, env.rawDB)

	decisions, _ := env.store.ListActiveByKind(env.ctx, artifact.KindDecisionRecord, 0)
	problems, _ := env.store.ListActiveByKind(env.ctx, artifact.KindProblemCard, 0)

	return writeResult(w, map[string]any{
		"stale_count":    len(staleItems),
		"drift_count":    len(driftReports),
		"decision_count": len(decisions),
		"problem_count":  len(problems),
		"coverage":       coverage,
		"scanned_at":     time.Now().UTC().Format(time.RFC3339),
	})
}

func handleGetCoverage(env *rpcEnv, w io.Writer) error {
	report, err := codebase.ComputeCoverage(env.ctx, env.rawDB)
	if err != nil {
		return fmt.Errorf("compute coverage: %w", err)
	}

	return writeResult(w, report)
}

func handleAssessReadiness(env *rpcEnv, w io.Writer) error {
	var input struct {
		PortfolioID string `json:"portfolio_id"`
	}
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	report, err := graph.AssessReadiness(env.ctx, env.rawDB, input.PortfolioID)
	if err != nil {
		return fmt.Errorf("assess readiness: %w", err)
	}

	return writeResult(w, report)
}

// ── Agents & external ───────────────────────────────────────────────

type rpcInstalledAgent struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
}

func handleDetectAgents(_ *rpcEnv, w io.Writer) error {
	var agents []rpcInstalledAgent

	for _, spec := range []struct {
		kind, name, binary, versionFlag string
	}{
		{"claude", "Claude Code", "claude", "--version"},
		{"codex", "Codex", "codex", "--version"},
		{"haft", "Haft Agent", "haft", "version"},
	} {
		path, err := exec.LookPath(spec.binary)
		if err != nil {
			continue
		}
		version := rpcGetVersion(path, spec.versionFlag)
		agents = append(agents, rpcInstalledAgent{
			Kind:    spec.kind,
			Name:    spec.name,
			Path:    path,
			Version: version,
		})
	}

	if agents == nil {
		agents = []rpcInstalledAgent{}
	}

	return writeResult(w, agents)
}

func handleCreatePullRequest(env *rpcEnv, w io.Writer) error {
	var input struct {
		DecisionRef string `json:"decision_ref"`
		Branch      string `json:"branch"`
		RepoPath    string `json:"repo_path"`
	}
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	branch := strings.TrimSpace(input.Branch)
	if branch == "" {
		return fmt.Errorf("branch is required")
	}

	repoPath := strings.TrimSpace(input.RepoPath)
	if repoPath == "" {
		repoPath = env.projectRoot
	}

	decisionRef := strings.TrimSpace(input.DecisionRef)
	if decisionRef == "" {
		return fmt.Errorf("decision_ref is required")
	}

	art, err := env.store.Get(env.ctx, decisionRef)
	if err != nil {
		return fmt.Errorf("load decision %s: %w", decisionRef, err)
	}

	title := fmt.Sprintf("impl(%s): %s", decisionRef, art.Meta.Title)
	if len(title) > 70 {
		title = title[:67] + "..."
	}

	body := fmt.Sprintf("## Decision\n\n**%s** (`%s`)\n\n%s",
		art.Meta.Title, decisionRef, art.Meta.Context)

	result := map[string]any{
		"decision_ref":  decisionRef,
		"branch":        branch,
		"title":         title,
		"body":          body,
		"pushed":        false,
		"draft_created": false,
		"url":           "",
		"warnings":      []string{},
	}

	warnings := []string{}

	// Push
	pushCmd := exec.Command("git", "-C", repoPath, "push", "-u", "origin", branch)
	if pushErr := pushCmd.Run(); pushErr != nil {
		warnings = append(warnings, fmt.Sprintf("branch push failed: %v", pushErr))
	} else {
		result["pushed"] = true
	}

	// Create draft PR
	if result["pushed"].(bool) {
		ghCmd := exec.Command("gh", "pr", "create",
			"--draft",
			"--title", title,
			"--body", body,
			"--head", branch,
		)
		ghCmd.Dir = repoPath
		out, ghErr := ghCmd.Output()
		if ghErr != nil {
			warnings = append(warnings, fmt.Sprintf("draft PR creation failed: %v", ghErr))
		} else {
			result["draft_created"] = true
			result["url"] = strings.TrimSpace(string(out))
		}
	}

	result["warnings"] = warnings
	return writeResult(w, result)
}

// ── Helpers ─────────────────────────────────────────────────────────

func rpcBoolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func rpcGetVersion(path, flag string) string {
	out, err := exec.Command(path, flag).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(strings.Split(string(out), "\n")[0])
}

func rpcRegistryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".haft")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "desktop-projects.json"), nil
}

func rpcLoadRegistry() (*rpcProjectRegistry, error) {
	path, err := rpcRegistryPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &rpcProjectRegistry{}, nil
		}
		return nil, err
	}
	var reg rpcProjectRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return &rpcProjectRegistry{}, nil
	}
	return &reg, nil
}

func rpcSaveRegistry(reg *rpcProjectRegistry) error {
	path, err := rpcRegistryPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// handlePersistTask UPSERTs a RunningTask snapshot from the Rust desktop
// agent into `desktop_tasks`. Called fire-and-forget by the Tauri side on
// every status change / output flush; failures on this RPC are non-fatal
// on the caller side (state stays in-memory) but do drop persistence, so
// surface any DB error to stderr for diagnosis.
func handlePersistTask(env *rpcEnv, w io.Writer) error {
	var input struct {
		ID             string `json:"id"`
		ProjectName    string `json:"project_name"`
		ProjectPath    string `json:"project_path"`
		Title          string `json:"title"`
		Agent          string `json:"agent"`
		Status         string `json:"status"`
		Prompt         string `json:"prompt"`
		Branch         string `json:"branch"`
		Worktree       bool   `json:"worktree"`
		WorktreePath   string `json:"worktree_path"`
		ReusedWorktree bool   `json:"reused_worktree"`
		ErrorMessage   string `json:"error_message"`
		OutputTail     string `json:"output_tail"`
		ChatBlocksJSON string `json:"chat_blocks_json"`
		RawOutput      string `json:"raw_output"`
		StartedAt      string `json:"started_at"`
		CompletedAt    string `json:"completed_at"`
		UpdatedAt      string `json:"updated_at"`
	}
	if err := readInput(&input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}
	if strings.TrimSpace(input.ID) == "" {
		return fmt.Errorf("task id is required")
	}

	var completedAt any
	if strings.TrimSpace(input.CompletedAt) == "" {
		completedAt = nil
	} else {
		completedAt = input.CompletedAt
	}
	if strings.TrimSpace(input.ChatBlocksJSON) == "" {
		input.ChatBlocksJSON = "[]"
	}

	_, err := env.rawDB.ExecContext(env.ctx,
		`INSERT INTO desktop_tasks (
			id, project_name, project_path, title, agent, status, prompt,
			branch, worktree, worktree_path, reused_worktree, error_message,
			output_tail, chat_blocks_json, raw_output,
			started_at, completed_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			project_name=excluded.project_name,
			project_path=excluded.project_path,
			title=excluded.title,
			agent=excluded.agent,
			status=excluded.status,
			prompt=excluded.prompt,
			branch=excluded.branch,
			worktree=excluded.worktree,
			worktree_path=excluded.worktree_path,
			reused_worktree=excluded.reused_worktree,
			error_message=excluded.error_message,
			output_tail=excluded.output_tail,
			chat_blocks_json=excluded.chat_blocks_json,
			raw_output=excluded.raw_output,
			completed_at=excluded.completed_at,
			updated_at=excluded.updated_at`,
		input.ID, input.ProjectName, input.ProjectPath, input.Title,
		input.Agent, input.Status, input.Prompt,
		input.Branch, rpcBoolToInt(input.Worktree), input.WorktreePath,
		rpcBoolToInt(input.ReusedWorktree), input.ErrorMessage,
		input.OutputTail, input.ChatBlocksJSON, input.RawOutput,
		input.StartedAt, completedAt, input.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("persist task: %w", err)
	}

	return writeResult(w, map[string]any{
		"id":     input.ID,
		"status": input.Status,
	})
}
