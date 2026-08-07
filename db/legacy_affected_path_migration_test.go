package db

import (
	"testing"
)

// A database predating the project-relative path invariant can hold absolute
// or traversing affected_files rows. Migration 58 drops exactly those and
// leaves every expressible row untouched.
func TestLegacyAffectedPathMigration58DropsOnlyUnexpressibleRows(t *testing.T) {
	t.Parallel()

	store, err := NewStore(t.TempDir() + "/v58.db")
	if err != nil {
		t.Fatalf("open v58 store: %v", err)
	}
	defer store.Close()

	const artifactID = "note-20260629-preinvariant"
	if _, err := store.conn.Exec(
		`INSERT INTO artifacts (id, kind, title, content, created_at, updated_at)
		 VALUES (?, 'Note', 'Pre-invariant note', 'fixture',
		         '2026-06-29T17:30:01Z', '2026-06-29T17:30:01Z')`,
		artifactID,
	); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}

	expressible := "internal/cli/interface.go"
	unexpressible := []string{
		"/Users/someone/.agent/attachments/pasted-text.txt",
		"../outside-the-project.go",
	}
	for _, path := range append([]string{expressible}, unexpressible...) {
		if _, err := store.conn.Exec(
			`INSERT INTO affected_files (artifact_id, file_path, file_hash)
			 VALUES (?, ?, '')`,
			artifactID,
			path,
		); err != nil {
			t.Fatalf("seed affected file %q: %v", path, err)
		}
	}

	tx, err := store.conn.Begin()
	if err != nil {
		t.Fatalf("begin migration transaction: %v", err)
	}
	if err := applyLegacyAffectedPathMigration58(tx, nil); err != nil {
		_ = tx.Rollback()
		t.Fatalf("apply migration 58: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit migration 58: %v", err)
	}

	rows, err := store.conn.Query(
		`SELECT file_path FROM affected_files WHERE artifact_id = ?
		 ORDER BY file_path`,
		artifactID,
	)
	if err != nil {
		t.Fatalf("read surviving affected files: %v", err)
	}
	defer rows.Close()
	surviving := make([]string, 0)
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			t.Fatalf("scan surviving affected file: %v", err)
		}
		surviving = append(surviving, path)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("enumerate surviving affected files: %v", err)
	}
	if len(surviving) != 1 || surviving[0] != expressible {
		t.Fatalf("surviving affected files = %v", surviving)
	}
}

// The migration is a no-op on a database that never carried a pre-invariant
// row, so a fresh install pays nothing and reports nothing.
func TestLegacyAffectedPathMigration58IsANoOpWithoutLegacyRows(t *testing.T) {
	t.Parallel()

	store, err := NewStore(t.TempDir() + "/v58-clean.db")
	if err != nil {
		t.Fatalf("open v58 store: %v", err)
	}
	defer store.Close()

	tx, err := store.conn.Begin()
	if err != nil {
		t.Fatalf("begin migration transaction: %v", err)
	}
	if err := applyLegacyAffectedPathMigration58(tx, nil); err != nil {
		_ = tx.Rollback()
		t.Fatalf("apply migration 58 on a clean database: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit migration 58: %v", err)
	}
}
