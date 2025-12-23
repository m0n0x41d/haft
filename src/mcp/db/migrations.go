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
	{
		version:     4,
		description: "Add FTS5 tables for full-text search and populate from existing data",
		sql: `
			-- Create FTS5 virtual tables
			CREATE VIRTUAL TABLE IF NOT EXISTS holons_fts USING fts5(
				id,
				title,
				content,
				content='holons',
				content_rowid='rowid'
			);

			CREATE VIRTUAL TABLE IF NOT EXISTS evidence_fts USING fts5(
				id,
				content,
				content='evidence',
				content_rowid='rowid'
			);

			-- Populate FTS from existing holons
			INSERT INTO holons_fts(holons_fts) VALUES('rebuild');

			-- Populate FTS from existing evidence
			INSERT INTO evidence_fts(evidence_fts) VALUES('rebuild');

			-- Create triggers for holons (IF NOT EXISTS not supported for triggers in older SQLite)
			DROP TRIGGER IF EXISTS holons_ai;
			CREATE TRIGGER holons_ai AFTER INSERT ON holons BEGIN
				INSERT INTO holons_fts(rowid, id, title, content)
				VALUES (new.rowid, new.id, new.title, new.content);
			END;

			DROP TRIGGER IF EXISTS holons_ad;
			CREATE TRIGGER holons_ad AFTER DELETE ON holons BEGIN
				INSERT INTO holons_fts(holons_fts, rowid, id, title, content)
				VALUES('delete', old.rowid, old.id, old.title, old.content);
			END;

			DROP TRIGGER IF EXISTS holons_au;
			CREATE TRIGGER holons_au AFTER UPDATE ON holons BEGIN
				INSERT INTO holons_fts(holons_fts, rowid, id, title, content)
				VALUES('delete', old.rowid, old.id, old.title, old.content);
				INSERT INTO holons_fts(rowid, id, title, content)
				VALUES (new.rowid, new.id, new.title, new.content);
			END;

			-- Create triggers for evidence
			DROP TRIGGER IF EXISTS evidence_ai;
			CREATE TRIGGER evidence_ai AFTER INSERT ON evidence BEGIN
				INSERT INTO evidence_fts(rowid, id, content)
				VALUES (new.rowid, new.id, new.content);
			END;

			DROP TRIGGER IF EXISTS evidence_ad;
			CREATE TRIGGER evidence_ad AFTER DELETE ON evidence BEGIN
				INSERT INTO evidence_fts(evidence_fts, rowid, id, content)
				VALUES('delete', old.rowid, old.id, old.content);
			END;

			DROP TRIGGER IF EXISTS evidence_au;
			CREATE TRIGGER evidence_au AFTER UPDATE ON evidence BEGIN
				INSERT INTO evidence_fts(evidence_fts, rowid, id, content)
				VALUES('delete', old.rowid, old.id, old.content);
				INSERT INTO evidence_fts(rowid, id, content)
				VALUES (new.rowid, new.id, new.content);
			END;
		`,
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
