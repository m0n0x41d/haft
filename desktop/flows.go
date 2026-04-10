package main

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type FlowInput struct {
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

type DesktopFlow struct {
	ID          string `json:"id"`
	ProjectName string `json:"project_name"`
	ProjectPath string `json:"project_path"`
	Title       string `json:"title"`
	Description string `json:"description"`
	TemplateID  string `json:"template_id"`
	Agent       string `json:"agent"`
	Prompt      string `json:"prompt"`
	Schedule    string `json:"schedule"`
	Branch      string `json:"branch"`
	UseWorktree bool   `json:"use_worktree"`
	Enabled     bool   `json:"enabled"`
	LastTaskID  string `json:"last_task_id"`
	LastRunAt   string `json:"last_run_at"`
	NextRunAt   string `json:"next_run_at"`
	LastError   string `json:"last_error"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type FlowTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Agent       string `json:"agent"`
	Schedule    string `json:"schedule"`
	Prompt      string `json:"prompt"`
	Branch      string `json:"branch"`
	UseWorktree bool   `json:"use_worktree"`
}

type flowController struct {
	app   *App
	store *desktopFlowStore

	mu   sync.Mutex
	cron *cron.Cron
}

type desktopFlowStore struct {
	db *sql.DB
}

type flowRowScanner interface {
	Scan(dest ...any) error
}

func newFlowController(app *App, store *desktopFlowStore) *flowController {
	return &flowController{
		app:   app,
		store: store,
	}
}

func newDesktopFlowStore(db *sql.DB) *desktopFlowStore {
	return &desktopFlowStore{db: db}
}

func defaultFlowTemplates() []FlowTemplate {
	return []FlowTemplate{
		{
			ID:          "decision-refresh",
			Name:        "Decision Refresh",
			Description: "Verify due decisions and turn stale reasoning into scheduled operator work.",
			Agent:       string(AgentClaude),
			Schedule:    "0 9 * * 1",
			Branch:      "flows/decision-refresh",
			UseWorktree: true,
			Prompt: strings.TrimSpace(`
Review active decisions with expired or near-expired validity windows.

Instructions:
- List decisions that need refresh or measurement.
- Spawn or update the appropriate verification follow-up.
- Record clear next actions for any decision that remains stale after inspection.
`),
		},
		{
			ID:          "drift-scan",
			Name:        "Drift Detection",
			Description: "Run a recurring drift-focused review against baselined files and decisions.",
			Agent:       string(AgentCodex),
			Schedule:    "0 10 * * 1-5",
			Branch:      "flows/drift-scan",
			UseWorktree: true,
			Prompt: strings.TrimSpace(`
Scan the current project for drift against decision baselines and recently affected files.

Instructions:
- Review recorded baselines and affected files.
- Surface files or modules that have drifted since the last baseline.
- Summarize the highest-priority follow-up problems or verification tasks.
`),
		},
		{
			ID:          "dependency-audit",
			Name:        "Dependency Audit",
			Description: "Check the project for outdated dependencies and risky upgrade pressure.",
			Agent:       string(AgentCodex),
			Schedule:    "0 11 * * 1",
			Branch:      "flows/dependency-audit",
			UseWorktree: true,
			Prompt: strings.TrimSpace(`
Audit dependencies for outdated or risky versions.

Instructions:
- Inspect the project's package and module manifests.
- Highlight outdated or vulnerable dependencies.
- Recommend the smallest safe remediation steps and note anything that should become a tracked problem.
`),
		},
		{
			ID:          "evidence-health",
			Name:        "Evidence Health",
			Description: "Look for weak evidence and decisions that should be refreshed before they decay further.",
			Agent:       string(AgentClaude),
			Schedule:    "0 14 * * 5",
			Branch:      "flows/evidence-health",
			UseWorktree: true,
			Prompt: strings.TrimSpace(`
Review decision evidence health across the active project.

Instructions:
- Identify decisions whose evidence is weak, stale, or missing.
- Summarize which claims or measurements should be refreshed next.
- Record explicit operator follow-up recommendations instead of vague warnings.
`),
		},
		{
			ID:          "coverage-report",
			Name:        "Coverage Report",
			Description: "Generate a weekly governance coverage snapshot for the current project.",
			Agent:       string(AgentHaft),
			Schedule:    "0 15 * * 1",
			Branch:      "flows/coverage-report",
			UseWorktree: false,
			Prompt: strings.TrimSpace(`
Summarize module governance coverage for the current project.

Instructions:
- Inspect governed, partial, and blind modules.
- Call out any critical modules that still lack decision coverage.
- End with a short ranked list of follow-up tasks.
`),
		},
	}
}

func parseFlowSchedule(value string) (cron.Schedule, error) {
	schedule := strings.TrimSpace(value)
	if schedule == "" {
		return nil, fmt.Errorf("schedule is required")
	}

	return cron.ParseStandard(schedule)
}

func computeFlowNextRun(value string, from time.Time) (string, error) {
	schedule, err := parseFlowSchedule(value)
	if err != nil {
		return "", err
	}

	nextRun := schedule.Next(from)
	if nextRun.IsZero() {
		return "", nil
	}

	return nextRun.UTC().Format(time.RFC3339), nil
}

func normalizeFlowInput(input FlowInput, cfg DesktopConfig) (FlowInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.TemplateID = strings.TrimSpace(input.TemplateID)
	input.Agent = normalizeAgentKind(input.Agent, cfg.DefaultAgent)
	input.Prompt = strings.TrimSpace(input.Prompt)
	input.Schedule = strings.TrimSpace(input.Schedule)
	input.Branch = strings.TrimSpace(input.Branch)

	if input.Title == "" {
		return FlowInput{}, fmt.Errorf("flow title is required")
	}

	if input.Prompt == "" {
		return FlowInput{}, fmt.Errorf("flow prompt is required")
	}

	if _, err := parseFlowSchedule(input.Schedule); err != nil {
		return FlowInput{}, fmt.Errorf("invalid flow schedule: %w", err)
	}

	return input, nil
}

func buildFlowFromInput(input FlowInput, projectName string, projectPath string) (DesktopFlow, error) {
	cfg := defaultDesktopConfig()
	loadedConfig, err := loadDesktopConfig()
	if err == nil && loadedConfig != nil {
		cfg = *loadedConfig
	}

	normalized, err := normalizeFlowInput(input, cfg)
	if err != nil {
		return DesktopFlow{}, err
	}

	now := nowRFC3339()
	flowID := firstNonEmpty(strings.TrimSpace(normalized.ID), fmt.Sprintf("flow-%d", time.Now().UnixNano()))
	nextRunAt := ""
	if normalized.Enabled {
		nextRunAt, err = computeFlowNextRun(normalized.Schedule, time.Now())
		if err != nil {
			return DesktopFlow{}, err
		}
	}

	return DesktopFlow{
		ID:          flowID,
		ProjectName: projectName,
		ProjectPath: projectPath,
		Title:       normalized.Title,
		Description: normalized.Description,
		TemplateID:  normalized.TemplateID,
		Agent:       normalized.Agent,
		Prompt:      normalized.Prompt,
		Schedule:    normalized.Schedule,
		Branch:      normalized.Branch,
		UseWorktree: normalized.UseWorktree,
		Enabled:     normalized.Enabled,
		NextRunAt:   nextRunAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (s *desktopFlowStore) UpsertFlow(ctx context.Context, flow DesktopFlow) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("desktop flow store is not initialized")
	}

	createdAt := firstNonEmpty(flow.CreatedAt, nowRFC3339())
	updatedAt := firstNonEmpty(flow.UpdatedAt, nowRFC3339())

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO desktop_flows (
			id,
			project_name,
			project_path,
			title,
			description,
			template_id,
			agent,
			prompt,
			schedule,
			branch,
			use_worktree,
			enabled,
			last_task_id,
			last_run_at,
			next_run_at,
			last_error,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			project_name = excluded.project_name,
			project_path = excluded.project_path,
			title = excluded.title,
			description = excluded.description,
			template_id = excluded.template_id,
			agent = excluded.agent,
			prompt = excluded.prompt,
			schedule = excluded.schedule,
			branch = excluded.branch,
			use_worktree = excluded.use_worktree,
			enabled = excluded.enabled,
			last_task_id = excluded.last_task_id,
			last_run_at = excluded.last_run_at,
			next_run_at = excluded.next_run_at,
			last_error = excluded.last_error,
			updated_at = excluded.updated_at`,
		flow.ID,
		flow.ProjectName,
		flow.ProjectPath,
		flow.Title,
		flow.Description,
		flow.TemplateID,
		flow.Agent,
		flow.Prompt,
		flow.Schedule,
		flow.Branch,
		boolToInt(flow.UseWorktree),
		boolToInt(flow.Enabled),
		flow.LastTaskID,
		nullString(flow.LastRunAt),
		nullString(flow.NextRunAt),
		flow.LastError,
		createdAt,
		updatedAt,
	)

	return err
}

func (s *desktopFlowStore) ListFlows(ctx context.Context, projectPath string) ([]DesktopFlow, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("desktop flow store is not initialized")
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT
			id,
			project_name,
			project_path,
			title,
			description,
			template_id,
			agent,
			prompt,
			schedule,
			branch,
			use_worktree,
			enabled,
			last_task_id,
			COALESCE(last_run_at, ''),
			COALESCE(next_run_at, ''),
			last_error,
			created_at,
			updated_at
		FROM desktop_flows
		WHERE project_path = ?
		ORDER BY updated_at DESC, title ASC`,
		projectPath,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]DesktopFlow, 0)
	for rows.Next() {
		flow, err := scanDesktopFlow(rows)
		if err != nil {
			return nil, err
		}

		result = append(result, flow)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *desktopFlowStore) GetFlow(ctx context.Context, id string) (*DesktopFlow, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("desktop flow store is not initialized")
	}

	row := s.db.QueryRowContext(
		ctx,
		`SELECT
			id,
			project_name,
			project_path,
			title,
			description,
			template_id,
			agent,
			prompt,
			schedule,
			branch,
			use_worktree,
			enabled,
			last_task_id,
			COALESCE(last_run_at, ''),
			COALESCE(next_run_at, ''),
			last_error,
			created_at,
			updated_at
		FROM desktop_flows
		WHERE id = ?`,
		id,
	)

	flow, err := scanDesktopFlow(row)
	if err != nil {
		return nil, err
	}

	return &flow, nil
}

func (s *desktopFlowStore) DeleteFlow(ctx context.Context, id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("desktop flow store is not initialized")
	}

	_, err := s.db.ExecContext(ctx, `DELETE FROM desktop_flows WHERE id = ?`, id)
	return err
}

func scanDesktopFlow(scanner flowRowScanner) (DesktopFlow, error) {
	var flow DesktopFlow
	var useWorktree int
	var enabled int

	err := scanner.Scan(
		&flow.ID,
		&flow.ProjectName,
		&flow.ProjectPath,
		&flow.Title,
		&flow.Description,
		&flow.TemplateID,
		&flow.Agent,
		&flow.Prompt,
		&flow.Schedule,
		&flow.Branch,
		&useWorktree,
		&enabled,
		&flow.LastTaskID,
		&flow.LastRunAt,
		&flow.NextRunAt,
		&flow.LastError,
		&flow.CreatedAt,
		&flow.UpdatedAt,
	)
	if err != nil {
		return DesktopFlow{}, err
	}

	flow.UseWorktree = useWorktree == 1
	flow.Enabled = enabled == 1

	return flow, nil
}

func (c *flowController) list(ctx context.Context, projectPath string) ([]DesktopFlow, error) {
	if c == nil || c.store == nil {
		return []DesktopFlow{}, nil
	}

	return c.store.ListFlows(ctx, projectPath)
}

func (c *flowController) shutdown() {
	if c == nil {
		return
	}

	c.mu.Lock()
	current := c.cron
	c.cron = nil
	c.mu.Unlock()

	if current == nil {
		return
	}

	stopCtx := current.Stop()
	<-stopCtx.Done()
}

func (c *flowController) reload(ctx context.Context) error {
	if c == nil || c.store == nil || c.app == nil || c.app.projectRoot == "" {
		return nil
	}

	c.shutdown()

	flows, err := c.store.ListFlows(ctx, c.app.projectRoot)
	if err != nil {
		return err
	}

	engine := cron.New()
	now := time.Now()

	for _, flow := range flows {
		flow.UpdatedAt = nowRFC3339()
		flow.LastError = ""
		flow.NextRunAt = ""

		if flow.Enabled {
			nextRunAt, nextErr := computeFlowNextRun(flow.Schedule, now)
			if nextErr != nil {
				flow.LastError = nextErr.Error()
			} else {
				flow.NextRunAt = nextRunAt

				flowID := flow.ID
				_, addErr := engine.AddFunc(flow.Schedule, func() {
					if _, runErr := c.runNow(context.Background(), flowID); runErr != nil && c.app != nil {
						c.app.emitAppError("scheduled flow", runErr)
					}
				})
				if addErr != nil {
					flow.NextRunAt = ""
					flow.LastError = addErr.Error()
				}
			}
		}

		if err := c.store.UpsertFlow(ctx, flow); err != nil {
			return err
		}
	}

	engine.Start()

	c.mu.Lock()
	c.cron = engine
	c.mu.Unlock()

	return nil
}

func (c *flowController) runNow(ctx context.Context, id string) (*TaskState, error) {
	if c == nil || c.store == nil || c.app == nil {
		return nil, fmt.Errorf("flow controller is not initialized")
	}

	flow, err := c.store.GetFlow(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load flow %s: %w", id, err)
	}

	task, err := c.app.spawnTaskWithTitle(
		flow.Agent,
		flow.Prompt,
		flow.UseWorktree,
		flow.Branch,
		fmt.Sprintf("Flow: %s", flow.Title),
	)

	now := time.Now()
	flow.UpdatedAt = nowRFC3339()
	// Don't clear LastTaskID here — only update it after successful spawn (line below).
	// Clearing before checking err loses the reference to the previous successful task.
	flow.LastError = ""

	if flow.Enabled {
		flow.NextRunAt, _ = computeFlowNextRun(flow.Schedule, now)
	}

	if err != nil {
		flow.LastError = err.Error()
		storeErr := c.store.UpsertFlow(ctx, *flow)
		if storeErr != nil && c.app != nil {
			c.app.emitAppError("flow persistence", storeErr)
		}
		return nil, fmt.Errorf("run flow %s: %w", flow.Title, err)
	}

	flow.LastRunAt = now.UTC().Format(time.RFC3339)
	flow.LastTaskID = task.ID

	if err := c.store.UpsertFlow(ctx, *flow); err != nil {
		return nil, err
	}

	return task, nil
}

func (a *App) ensureFlowController() {
	if a == nil || a.dbConn == nil {
		return
	}

	if a.flows != nil {
		return
	}

	a.flows = newFlowController(a, newDesktopFlowStore(a.dbConn.GetRawDB()))
}

func (a *App) ListFlows() ([]DesktopFlow, error) {
	if a.projectRoot == "" {
		return []DesktopFlow{}, nil
	}

	a.ensureFlowController()
	return a.flows.list(a.ctx, a.projectRoot)
}

func (a *App) ListFlowTemplates() ([]FlowTemplate, error) {
	templates := defaultFlowTemplates()

	sort.Slice(templates, func(i int, j int) bool {
		return templates[i].Name < templates[j].Name
	})

	return templates, nil
}

func (a *App) CreateFlow(input FlowInput) (*DesktopFlow, error) {
	if a.projectRoot == "" {
		return nil, fmt.Errorf("no active project")
	}

	a.ensureFlowController()

	flow, err := buildFlowFromInput(input, a.projectName, a.projectRoot)
	if err != nil {
		return nil, err
	}

	if err := a.flows.store.UpsertFlow(a.ctx, flow); err != nil {
		return nil, err
	}

	if err := a.flows.reload(a.ctx); err != nil {
		return nil, err
	}

	return a.flows.store.GetFlow(a.ctx, flow.ID)
}

func (a *App) UpdateFlow(input FlowInput) (*DesktopFlow, error) {
	if strings.TrimSpace(input.ID) == "" {
		return nil, fmt.Errorf("flow id is required")
	}

	a.ensureFlowController()

	existing, err := a.flows.store.GetFlow(a.ctx, input.ID)
	if err != nil {
		return nil, err
	}

	flow, err := buildFlowFromInput(input, existing.ProjectName, existing.ProjectPath)
	if err != nil {
		return nil, err
	}

	flow.CreatedAt = existing.CreatedAt
	flow.LastTaskID = existing.LastTaskID
	flow.LastRunAt = existing.LastRunAt
	flow.LastError = existing.LastError

	if err := a.flows.store.UpsertFlow(a.ctx, flow); err != nil {
		return nil, err
	}

	if err := a.flows.reload(a.ctx); err != nil {
		return nil, err
	}

	return a.flows.store.GetFlow(a.ctx, flow.ID)
}

func (a *App) ToggleFlow(id string, enabled bool) (*DesktopFlow, error) {
	a.ensureFlowController()

	flow, err := a.flows.store.GetFlow(a.ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}

	flow.Enabled = enabled
	flow.UpdatedAt = nowRFC3339()
	if !enabled {
		flow.NextRunAt = ""
	}
	if enabled {
		flow.NextRunAt, err = computeFlowNextRun(flow.Schedule, time.Now())
		if err != nil {
			return nil, err
		}
	}

	if err := a.flows.store.UpsertFlow(a.ctx, *flow); err != nil {
		return nil, err
	}

	if err := a.flows.reload(a.ctx); err != nil {
		return nil, err
	}

	return a.flows.store.GetFlow(a.ctx, flow.ID)
}

func (a *App) DeleteFlow(id string) error {
	a.ensureFlowController()

	if err := a.flows.store.DeleteFlow(a.ctx, strings.TrimSpace(id)); err != nil {
		return err
	}

	return a.flows.reload(a.ctx)
}

func (a *App) RunFlowNow(id string) (*TaskState, error) {
	a.ensureFlowController()
	return a.flows.runNow(a.ctx, strings.TrimSpace(id))
}
