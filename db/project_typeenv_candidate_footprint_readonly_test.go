package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequireCurrentProjectTypeEnvCandidateFootprintReadOnlyAcceptsExactDDL(
	t *testing.T,
) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "candidate-exact.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	err = RequireCurrentProjectTypeEnvCandidateFootprintReadOnly(
		context.Background(),
		store.GetRawDB(),
	)
	if err != nil {
		t.Fatalf(
			"RequireCurrentProjectTypeEnvCandidateFootprintReadOnly() error = %v",
			err,
		)
	}
}

func TestRequireCurrentProjectTypeEnvCandidateFootprintReadOnlyRejectsAlteredTriggerWithoutRepair(
	t *testing.T,
) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "candidate-drift.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	database := store.GetRawDB()
	const trigger = "project_typeenv_stages_v47_no_insert"
	const altered = `CREATE TRIGGER project_typeenv_stages_v47_no_insert
		BEFORE INSERT ON project_typeenv_stages
		BEGIN
			SELECT RAISE(
				ABORT,
				'project TypeEnv candidate store is immutable'
			) WHERE 0;
		END`
	if _, err := database.Exec("DROP TRIGGER " + trigger); err != nil {
		t.Fatalf("drop exact candidate trigger: %v", err)
	}
	if _, err := database.Exec(altered); err != nil {
		t.Fatalf("install altered candidate trigger: %v", err)
	}
	before := exactSQLiteObjectSQL(t, database, "trigger", trigger)

	err = RequireCurrentProjectTypeEnvCandidateFootprintReadOnly(
		context.Background(),
		database,
	)
	if err == nil || !strings.Contains(err.Error(), trigger) {
		t.Fatalf(
			"RequireCurrentProjectTypeEnvCandidateFootprintReadOnly() error = %v, want %q",
			err,
			trigger,
		)
	}

	after := exactSQLiteObjectSQL(t, database, "trigger", trigger)
	if after != before {
		t.Fatal("read-only candidate-footprint verification repaired altered DDL")
	}
}

func exactSQLiteObjectSQL(
	t *testing.T,
	database interface {
		QueryRow(query string, args ...any) *sql.Row
	},
	kind string,
	name string,
) string {
	t.Helper()
	var statement string
	err := database.
		QueryRow(
			`SELECT sql
			 FROM sqlite_schema
			 WHERE type = ? AND name = ?`,
			kind,
			name,
		).
		Scan(&statement)
	if err != nil {
		t.Fatal(err)
	}
	return statement
}
