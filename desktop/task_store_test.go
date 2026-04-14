package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/db"
)

func TestDesktopTaskStoreRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "haft.db")

	database, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer database.Close()

	store := newDesktopTaskStore(database.GetRawDB())
	ctx := context.Background()

	state := TaskState{
		ID:             "task-1",
		Title:          "Runtime foundation",
		Agent:          "claude",
		Project:        "haft",
		ProjectPath:    "/tmp/haft",
		Status:         "running",
		Prompt:         "Implement runtime foundation",
		Branch:         "feat/runtime-foundation",
		Worktree:       true,
		WorktreePath:   "/tmp/haft/.haft/worktrees/feat/runtime-foundation",
		ReusedWorktree: false,
		StartedAt:      nowRFC3339(),
		Output:         "line one\nline two",
	}

	if err := store.UpsertTask(ctx, state); err != nil {
		t.Fatalf("UpsertTask: %v", err)
	}

	tasks, err := store.ListTasks(ctx, state.ProjectPath)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	if tasks[0].WorktreePath != state.WorktreePath {
		t.Fatalf("expected worktree path %q, got %q", state.WorktreePath, tasks[0].WorktreePath)
	}

	if err := store.MarkRunningTasksInterrupted(ctx, state.ProjectPath); err != nil {
		t.Fatalf("MarkRunningTasksInterrupted: %v", err)
	}

	interrupted, err := store.GetTask(ctx, state.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}

	if interrupted.Status != "interrupted" {
		t.Fatalf("expected interrupted status, got %q", interrupted.Status)
	}

	if interrupted.CompletedAt == "" {
		t.Fatalf("expected completed_at to be populated")
	}

	refs, err := store.CountTaskRefs(ctx, state.WorktreePath, "other-task")
	if err != nil {
		t.Fatalf("CountTaskRefs: %v", err)
	}

	if refs != 1 {
		t.Fatalf("expected 1 worktree ref, got %d", refs)
	}

	if err := store.ArchiveTask(ctx, state.ID); err != nil {
		t.Fatalf("ArchiveTask: %v", err)
	}

	remaining, err := store.ListTasks(ctx, state.ProjectPath)
	if err != nil {
		t.Fatalf("ListTasks after archive: %v", err)
	}

	if len(remaining) != 0 {
		t.Fatalf("expected archived task to be hidden, got %d task(s)", len(remaining))
	}
}

func TestDesktopTaskStoreNormalizesLegacyOutputOnRead(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "haft.db")

	database, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer database.Close()

	store := newDesktopTaskStore(database.GetRawDB())
	ctx := context.Background()
	rawOutput := strings.Repeat("legacy-output-", 7000) + "ENDMARKER"

	_, err = database.GetRawDB().ExecContext(
		ctx,
		`INSERT INTO desktop_tasks (
			id,
			project_name,
			project_path,
			title,
			agent,
			status,
			prompt,
			branch,
			worktree,
			worktree_path,
			reused_worktree,
			error_message,
			output_tail,
			started_at,
			completed_at,
			updated_at,
			archived_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, NULL)`,
		"legacy-task",
		"haft",
		"/tmp/haft",
		"Legacy output",
		"codex",
		"completed",
		"Inspect legacy output handling",
		"",
		0,
		"",
		0,
		"",
		rawOutput,
		nowRFC3339(),
		nowRFC3339(),
	)
	if err != nil {
		t.Fatalf("insert legacy task: %v", err)
	}

	tasks, err := store.ListTasks(ctx, "/tmp/haft")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	if utf8.RuneCountInString(tasks[0].Output) > taskOutputMaxChars {
		t.Fatalf("expected listed task output <= %d runes, got %d", taskOutputMaxChars, utf8.RuneCountInString(tasks[0].Output))
	}

	if !strings.HasSuffix(tasks[0].Output, "ENDMARKER") {
		t.Fatalf("expected listed task output to preserve newest tail")
	}

	output, err := store.GetTaskOutput(ctx, "legacy-task")
	if err != nil {
		t.Fatalf("GetTaskOutput: %v", err)
	}

	if utf8.RuneCountInString(output) > taskOutputMaxChars {
		t.Fatalf("expected fetched task output <= %d runes, got %d", taskOutputMaxChars, utf8.RuneCountInString(output))
	}

	if !strings.HasSuffix(output, "ENDMARKER") {
		t.Fatalf("expected fetched task output to preserve newest tail")
	}
}
