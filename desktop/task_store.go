package main

import (
	"context"
	"database/sql"
	"fmt"
)

type desktopTaskStore struct {
	db *sql.DB
}

func newDesktopTaskStore(db *sql.DB) *desktopTaskStore {
	return &desktopTaskStore{db: db}
}

func (s *desktopTaskStore) UpsertTask(ctx context.Context, state TaskState) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("desktop task store is not initialized")
	}

	_, err := s.db.ExecContext(
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
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)
		ON CONFLICT(id) DO UPDATE SET
			project_name = excluded.project_name,
			project_path = excluded.project_path,
			title = excluded.title,
			agent = excluded.agent,
			status = excluded.status,
			prompt = excluded.prompt,
			branch = excluded.branch,
			worktree = excluded.worktree,
			worktree_path = excluded.worktree_path,
			reused_worktree = excluded.reused_worktree,
			error_message = excluded.error_message,
			output_tail = excluded.output_tail,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			updated_at = excluded.updated_at`,
		state.ID,
		state.Project,
		state.ProjectPath,
		state.Title,
		state.Agent,
		state.Status,
		state.Prompt,
		state.Branch,
		boolToInt(state.Worktree),
		state.WorktreePath,
		boolToInt(state.ReusedWorktree),
		state.ErrorMessage,
		state.Output,
		state.StartedAt,
		nullString(state.CompletedAt),
		nowRFC3339(),
	)

	return err
}

func (s *desktopTaskStore) UpdateOutput(ctx context.Context, id string, output string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("desktop task store is not initialized")
	}

	_, err := s.db.ExecContext(
		ctx,
		`UPDATE desktop_tasks
		SET output_tail = ?, updated_at = ?
		WHERE id = ?`,
		output,
		nowRFC3339(),
		id,
	)

	return err
}

func (s *desktopTaskStore) ListTasks(ctx context.Context, projectPath string) ([]TaskState, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("desktop task store is not initialized")
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT
			id,
			title,
			agent,
			project_name,
			project_path,
			status,
			prompt,
			branch,
			worktree,
			worktree_path,
			reused_worktree,
			error_message,
			output_tail,
			started_at,
			COALESCE(completed_at, ''),
			COALESCE(auto_run, 0)
		FROM desktop_tasks
		WHERE archived_at IS NULL
			AND project_path = ?
		ORDER BY started_at DESC, id DESC`,
		projectPath,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]TaskState, 0)

	for rows.Next() {
		state, err := scanTaskState(rows)
		if err != nil {
			return nil, err
		}

		result = append(result, state)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *desktopTaskStore) GetTask(ctx context.Context, id string) (*TaskState, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("desktop task store is not initialized")
	}

	row := s.db.QueryRowContext(
		ctx,
		`SELECT
			id,
			title,
			agent,
			project_name,
			project_path,
			status,
			prompt,
			branch,
			worktree,
			worktree_path,
			reused_worktree,
			error_message,
			output_tail,
			started_at,
			COALESCE(completed_at, ''),
			COALESCE(auto_run, 0)
		FROM desktop_tasks
		WHERE id = ?
			AND archived_at IS NULL`,
		id,
	)

	state, err := scanTaskState(row)
	if err != nil {
		return nil, err
	}

	return &state, nil
}

func (s *desktopTaskStore) GetTaskOutput(ctx context.Context, id string) (string, error) {
	if s == nil || s.db == nil {
		return "", fmt.Errorf("desktop task store is not initialized")
	}

	var output string

	err := s.db.QueryRowContext(
		ctx,
		`SELECT COALESCE(output_tail, '')
		FROM desktop_tasks
		WHERE id = ?
			AND archived_at IS NULL`,
		id,
	).Scan(&output)
	if err != nil {
		return "", err
	}

	return output, nil
}

func (s *desktopTaskStore) MarkRunningTasksInterrupted(ctx context.Context, projectPath string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("desktop task store is not initialized")
	}

	_, err := s.db.ExecContext(
		ctx,
		`UPDATE desktop_tasks
		SET status = 'interrupted',
			error_message = CASE
				WHEN error_message = '' THEN 'Desktop session ended before the task completed.'
				ELSE error_message
			END,
			completed_at = COALESCE(completed_at, ?),
			updated_at = ?
		WHERE project_path = ?
			AND archived_at IS NULL
			AND status = 'running'`,
		nowRFC3339(),
		nowRFC3339(),
		projectPath,
	)

	return err
}

func (s *desktopTaskStore) ArchiveTask(ctx context.Context, id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("desktop task store is not initialized")
	}

	_, err := s.db.ExecContext(
		ctx,
		`UPDATE desktop_tasks
		SET archived_at = ?, updated_at = ?
		WHERE id = ?`,
		nowRFC3339(),
		nowRFC3339(),
		id,
	)

	return err
}

func (s *desktopTaskStore) CountTaskRefs(ctx context.Context, worktreePath string, ignoreTaskID string) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("desktop task store is not initialized")
	}

	var count int

	err := s.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		FROM desktop_tasks
		WHERE archived_at IS NULL
			AND worktree_path = ?
			AND id != ?
			AND status IN ('pending', 'running', 'completed', 'failed', 'cancelled', 'interrupted')`,
		worktreePath,
		ignoreTaskID,
	).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s *desktopTaskStore) SetAutoRun(ctx context.Context, id string, autoRun bool) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("desktop task store is not initialized")
	}

	val := 0
	if autoRun {
		val = 1
	}

	_, err := s.db.ExecContext(
		ctx,
		`UPDATE desktop_tasks SET auto_run = ? WHERE id = ?`,
		val,
		id,
	)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTaskState(scanner rowScanner) (TaskState, error) {
	var state TaskState
	var worktree int
	var reusedWorktree int
	var autoRun int

	err := scanner.Scan(
		&state.ID,
		&state.Title,
		&state.Agent,
		&state.Project,
		&state.ProjectPath,
		&state.Status,
		&state.Prompt,
		&state.Branch,
		&worktree,
		&state.WorktreePath,
		&reusedWorktree,
		&state.ErrorMessage,
		&state.Output,
		&state.StartedAt,
		&state.CompletedAt,
		&autoRun,
	)
	if err != nil {
		return TaskState{}, err
	}

	state.Worktree = worktree == 1
	state.ReusedWorktree = reusedWorktree == 1
	state.AutoRun = autoRun == 1

	return state, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}

	return 0
}

func nullString(value string) any {
	if value == "" {
		return nil
	}

	return value
}
