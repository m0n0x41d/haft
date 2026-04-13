package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// Migration defines a single versioned schema change.
type Migration struct {
	Version     int
	Description string
	Statements  []string // executed sequentially within the version
}

// Migrate applies all pending migrations to the database.
// Tracks applied versions in the given table name (e.g., "schema_version").
// Skips already-applied versions. Idempotent for ALTER TABLE / CREATE TABLE
// statements (catches "duplicate column" and "already exists" errors).
//
// Portable: uses only standard SQL (CREATE TABLE IF NOT EXISTS, INSERT, SELECT).
func Migrate(conn *sql.DB, versionTable string, migrations []Migration) error {
	// Ensure version tracking table exists
	_, err := conn.Exec(fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s (version INTEGER PRIMARY KEY, applied_at TEXT DEFAULT CURRENT_TIMESTAMP)`,
		versionTable,
	))
	if err != nil {
		return fmt.Errorf("create %s table: %w", versionTable, err)
	}

	for _, m := range migrations {
		// Skip already-applied migrations
		var exists int
		row := conn.QueryRow(
			fmt.Sprintf("SELECT 1 FROM %s WHERE version = ?", versionTable),
			m.Version,
		)
		if row.Scan(&exists) == nil && exists == 1 {
			continue
		}

		// Execute all statements for this migration
		for _, stmt := range m.Statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, execErr := conn.Exec(stmt); execErr != nil {
				if !isIdempotentError(execErr) {
					return fmt.Errorf("migration %d (%s) failed: %w", m.Version, m.Description, execErr)
				}
			}
		}

		// Record applied version
		if _, err := conn.Exec(
			fmt.Sprintf("INSERT INTO %s (version) VALUES (?)", versionTable),
			m.Version,
		); err != nil {
			return fmt.Errorf("record migration %d: %w", m.Version, err)
		}
	}

	return nil
}

// isIdempotentError returns true for errors that mean "already done" —
// safe to ignore when re-running migrations on existing databases.
func isIdempotentError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate column") ||
		strings.Contains(msg, "already exists")
}
