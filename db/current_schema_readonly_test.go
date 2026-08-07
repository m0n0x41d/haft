package db

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRequireCurrentSchemaReadOnlyAcceptsExactKernelEdition(
	t *testing.T,
) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "current.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	before := currentSchemaTestVersions(t, store.GetRawDB())

	err = RequireCurrentSchemaReadOnly(context.Background(), store.GetRawDB())
	if err != nil {
		t.Fatalf("RequireCurrentSchemaReadOnly() error = %v", err)
	}

	after := currentSchemaTestVersions(t, store.GetRawDB())
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("schema versions changed from %v to %v", before, after)
	}
}

func TestRequireCurrentSchemaReadOnlyRejectsDriftWithoutRepair(
	t *testing.T,
) {
	t.Parallel()

	currentVersion := kernelMigrations[len(kernelMigrations)-1].Version
	tests := []struct {
		name      string
		statement string
		wantError string
	}{
		{
			name: "missing current version",
			statement: fmt.Sprintf(
				"DELETE FROM schema_version WHERE version = %d",
				currentVersion,
			),
			wantError: "kernel schema is not current",
		},
		{
			name: "foreign future version",
			statement: fmt.Sprintf(
				"INSERT INTO schema_version (version) VALUES (%d)",
				currentVersion+1,
			),
			wantError: "kernel schema is not current",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, err := NewStore(filepath.Join(t.TempDir(), "drift.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			database := store.GetRawDB()
			if _, err := database.Exec(test.statement); err != nil {
				t.Fatal(err)
			}
			before := currentSchemaTestVersions(t, database)

			err = RequireCurrentSchemaReadOnly(context.Background(), database)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf(
					"RequireCurrentSchemaReadOnly() error = %v, want %q",
					err,
					test.wantError,
				)
			}

			after := currentSchemaTestVersions(t, database)
			if !reflect.DeepEqual(after, before) {
				t.Fatalf("failed schema check changed versions from %v to %v", before, after)
			}
		})
	}
}

func TestRequireCurrentSchemaReadOnlyDoesNotCreateVersionTable(
	t *testing.T,
) {
	t.Parallel()

	database, err := sql.Open(
		"sqlite",
		filepath.Join(t.TempDir(), "missing-version-table.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	err = RequireCurrentSchemaReadOnly(context.Background(), database)
	if err == nil ||
		!strings.Contains(err.Error(), "read kernel schema versions without migration") {
		t.Fatalf("RequireCurrentSchemaReadOnly() error = %v", err)
	}
	var count int
	err = database.
		QueryRow(
			`SELECT COUNT(*)
			 FROM sqlite_schema
			 WHERE type = 'table' AND name = 'schema_version'`,
		).
		Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("current-schema check created schema_version")
	}
}

func TestRequireSchemaPrefixReadOnlyRequiresExactContiguousPrefix(
	t *testing.T,
) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "prefix.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	database := store.GetRawDB()
	if _, err := database.Exec(
		"DELETE FROM schema_version WHERE version > 35",
	); err != nil {
		t.Fatal(err)
	}
	before := currentSchemaTestVersions(t, database)
	if err := RequireSchemaPrefixReadOnly(
		context.Background(),
		database,
		35,
	); err != nil {
		t.Fatalf("RequireSchemaPrefixReadOnly() error = %v", err)
	}

	if _, err := database.Exec(
		"DELETE FROM schema_version WHERE version = 17",
	); err != nil {
		t.Fatal(err)
	}
	err = RequireSchemaPrefixReadOnly(
		context.Background(),
		database,
		35,
	)
	if err == nil || !strings.Contains(
		err.Error(),
		"not an exact prefix through version 35",
	) {
		t.Fatalf("gapped schema prefix error = %v", err)
	}
	after := currentSchemaTestVersions(t, database)
	if len(after) != len(before)-1 {
		t.Fatalf(
			"failed prefix check changed schema versions from %v to %v",
			before,
			after,
		)
	}
}

func currentSchemaTestVersions(
	t *testing.T,
	database *sql.DB,
) []int {
	t.Helper()
	rows, err := database.Query(
		`SELECT version
		 FROM schema_version
		 ORDER BY version`,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	result := []int{}
	for rows.Next() {
		version := 0
		if err := rows.Scan(&version); err != nil {
			t.Fatal(err)
		}
		result = append(result, version)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}
