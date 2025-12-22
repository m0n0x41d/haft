package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// Migrations are applied sequentially to existing databases.
// New migrations should be appended to the end of this list.
// Never modify or reorder existing migrations.
var migrations = []struct {
	version     int
	description string
	sql         string
}{
	{
		version:     1,
		description: "Add parent_id to holons for L0->L1->L2 chain tracking",
		sql:         `ALTER TABLE holons ADD COLUMN parent_id TEXT REFERENCES holons(id)`,
	},
	{
		version:     2,
		description: "Add cached_r_score to holons for trust calculus",
		sql:         `ALTER TABLE holons ADD COLUMN cached_r_score REAL DEFAULT 0.0`,
	},
	{
		version:     3,
		description: "Add fpf_state table for FSM state (replaces state.json)",
		sql: `CREATE TABLE IF NOT EXISTS fpf_state (
			context_id TEXT PRIMARY KEY,
			active_role TEXT,
			active_session_id TEXT,
			active_role_context TEXT,
			last_commit TEXT,
			assurance_threshold REAL DEFAULT 0.8 CHECK(assurance_threshold BETWEEN 0.0 AND 1.0),
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	},
}

// RunMigrations applies all pending migrations to the database.
// Tracks applied migrations in schema_version table.
// Returns error if any migration fails (except "duplicate column" for ALTER TABLE).
func RunMigrations(conn *sql.DB) error {
	_, err := conn.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	for _, m := range migrations {
		var exists int
		err := conn.QueryRow("SELECT 1 FROM schema_version WHERE version = ?", m.version).Scan(&exists)
		if err == nil && exists == 1 {
			continue
		}

		_, execErr := conn.Exec(m.sql)
		if execErr != nil && !isDuplicateColumnError(execErr) {
			return fmt.Errorf("migration %d (%s) failed: %w", m.version, m.description, execErr)
		}

		if _, err := conn.Exec("INSERT INTO schema_version (version) VALUES (?)", m.version); err != nil {
			return fmt.Errorf("failed to record migration %d: %w", m.version, err)
		}
	}

	return nil
}

// isDuplicateColumnError checks if error is SQLite "duplicate column" error.
// This happens when schema already has the column (fresh install).
func isDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "duplicate column")
}
