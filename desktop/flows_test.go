package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/project"
)

func TestFlowCRUDAndSchedulingMetadata(t *testing.T) {
	app := newAutomationTestApp(t, "automation-main")
	defer app.shutdown(context.Background())

	created, err := app.CreateFlow(FlowInput{
		Title:       "Weekly decision refresh",
		Description: "Verify due decisions every Monday.",
		TemplateID:  "decision-refresh",
		Agent:       "claude",
		Prompt:      "Review active decisions with expired or near-expired validity windows.",
		Schedule:    "0 9 * * 1",
		Branch:      "flows/decision-refresh",
		UseWorktree: true,
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}

	if created.NextRunAt == "" {
		t.Fatalf("expected next_run_at to be populated")
	}

	updated, err := app.UpdateFlow(FlowInput{
		ID:          created.ID,
		Title:       "Weekly decision refresh",
		Description: "Updated description",
		TemplateID:  created.TemplateID,
		Agent:       "codex",
		Prompt:      created.Prompt,
		Schedule:    created.Schedule,
		Branch:      created.Branch,
		UseWorktree: created.UseWorktree,
		Enabled:     created.Enabled,
	})
	if err != nil {
		t.Fatalf("UpdateFlow: %v", err)
	}

	if updated.Agent != "codex" {
		t.Fatalf("expected updated agent codex, got %q", updated.Agent)
	}

	paused, err := app.ToggleFlow(created.ID, false)
	if err != nil {
		t.Fatalf("ToggleFlow: %v", err)
	}

	if paused.Enabled {
		t.Fatalf("expected flow to be paused")
	}

	if paused.NextRunAt != "" {
		t.Fatalf("expected paused flow to clear next_run_at, got %q", paused.NextRunAt)
	}

	flows, err := app.ListFlows()
	if err != nil {
		t.Fatalf("ListFlows: %v", err)
	}

	if len(flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(flows))
	}
}

func TestListAllTasksAggregatesRegisteredProjects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	setup := NewApp()
	firstProjectRoot := filepath.Join(t.TempDir(), "project-a")
	secondProjectRoot := filepath.Join(t.TempDir(), "project-b")

	if err := os.MkdirAll(firstProjectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll project-a: %v", err)
	}

	if err := os.MkdirAll(secondProjectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll project-b: %v", err)
	}

	if _, err := setup.InitProject(firstProjectRoot); err != nil {
		t.Fatalf("InitProject project-a: %v", err)
	}

	if _, err := setup.InitProject(secondProjectRoot); err != nil {
		t.Fatalf("InitProject project-b: %v", err)
	}

	app := NewApp()
	app.projectRoot = firstProjectRoot
	app.startup(context.Background())
	defer app.shutdown(context.Background())

	firstStore := openAutomationTaskStore(t, firstProjectRoot)
	secondStore := openAutomationTaskStore(t, secondProjectRoot)
	ctx := context.Background()

	firstTask := TaskState{
		ID:          "task-a",
		Title:       "Project A task",
		Agent:       "claude",
		Project:     "project-a",
		ProjectPath: firstProjectRoot,
		Status:      "running",
		Prompt:      "Inspect project A",
		StartedAt:   nowRFC3339(),
	}
	secondTask := TaskState{
		ID:          "task-b",
		Title:       "Project B task",
		Agent:       "codex",
		Project:     "project-b",
		ProjectPath: secondProjectRoot,
		Status:      "completed",
		Prompt:      "Inspect project B",
		StartedAt:   nowRFC3339(),
	}

	if err := firstStore.UpsertTask(ctx, firstTask); err != nil {
		t.Fatalf("UpsertTask project-a: %v", err)
	}

	if err := secondStore.UpsertTask(ctx, secondTask); err != nil {
		t.Fatalf("UpsertTask project-b: %v", err)
	}

	allTasks, err := app.ListAllTasks()
	if err != nil {
		t.Fatalf("ListAllTasks: %v", err)
	}

	if len(allTasks) != 2 {
		t.Fatalf("expected 2 aggregated tasks, got %d", len(allTasks))
	}
}

func newAutomationTestApp(t *testing.T, name string) *App {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	setup := NewApp()
	if _, err := setup.InitProject(projectRoot); err != nil {
		t.Fatalf("InitProject: %v", err)
	}

	app := NewApp()
	app.projectRoot = projectRoot
	app.startup(context.Background())

	if app.dbConn == nil {
		t.Fatal("expected database connection after startup")
	}

	return app
}

func openAutomationTaskStore(t *testing.T, projectRoot string) *desktopTaskStore {
	t.Helper()

	cfg, err := project.Load(filepath.Join(projectRoot, ".haft"))
	if err != nil {
		t.Fatalf("project.Load(%s): %v", projectRoot, err)
	}

	dbPath, err := cfg.DBPath()
	if err != nil {
		t.Fatalf("DBPath(%s): %v", projectRoot, err)
	}

	database, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("db.NewStore(%s): %v", projectRoot, err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	return newDesktopTaskStore(database.GetRawDB())
}
