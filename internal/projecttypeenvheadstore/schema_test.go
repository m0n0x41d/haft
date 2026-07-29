package projecttypeenvheadstore

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestNewRejectsMissingVersionedSchemaObject(t *testing.T) {
	fixture := newHeadStoreFixture(t)
	_, err := fixture.database.Exec(
		`DROP TRIGGER project_typeenv_heads_revision_cas`,
	)
	if err != nil {
		t.Fatalf("drop required trigger: %v", err)
	}
	_, err = New(context.Background(), fixture.database)
	if err == nil ||
		!strings.Contains(err.Error(), "project_typeenv_heads_revision_cas") {
		t.Fatalf("New() with missing trigger = %v", err)
	}
}

func TestNewRejectsUnversionedPartialHeadFootprint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial-head.db")
	database, err := sql.Open(
		"sqlite",
		"file:"+filepath.ToSlash(path)+"?_pragma=foreign_keys(1)",
	)
	if err != nil {
		t.Fatalf("open partial database: %v", err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	_, err = database.Exec(
		`CREATE TABLE project_typeenv_heads (project_id TEXT PRIMARY KEY)`,
	)
	if err != nil {
		t.Fatalf("create partial head table: %v", err)
	}
	_, err = New(context.Background(), database)
	if err == nil || !strings.Contains(err.Error(), "unversioned") {
		t.Fatalf("New() with partial unversioned footprint = %v", err)
	}
	var versionTables int
	err = database.QueryRow(
		`SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'table'
			AND name = 'project_typeenv_head_store_schema'`,
	).Scan(&versionTables)
	if err != nil {
		t.Fatalf("count schema-version tables: %v", err)
	}
	if versionTables != 0 {
		t.Fatalf("failed schema adoption left %d version tables", versionTables)
	}
}
