package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestProjectRootRelocationMigration60RewritesBindingGuards(
	t *testing.T,
) {
	databasePath := filepath.Join(t.TempDir(), "schema59.db")
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Exec(schema); err != nil {
		t.Fatalf("install base schema: %v", err)
	}
	through59 := migrationsBeforeVersion(
		kernelMigrations,
		ProjectRootRelocationSchemaVersion,
		0,
		nil,
	)
	if err := Migrate(database, "schema_version", through59); err != nil {
		t.Fatalf("migrate through schema 59: %v", err)
	}

	var predecessorGuards int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM sqlite_schema
		 WHERE type = 'trigger'
		 AND sql LIKE '%project_ledger_binding%'
		 AND name NOT LIKE 'project_ledger_binding_no_%'
		 AND tbl_name NOT IN (
			SELECT DISTINCT tbl_name
			FROM sqlite_schema
			WHERE type = 'trigger'
			AND sql LIKE '%sealed historical%'
		 )`,
	).Scan(&predecessorGuards); err != nil {
		t.Fatal(err)
	}
	if predecessorGuards == 0 {
		t.Fatal("schema 59 fixture has no project-root guards to rewrite")
	}

	if err := Migrate(database, "schema_version", kernelMigrations); err != nil {
		t.Fatalf("apply schema 60: %v", err)
	}
	if err := Migrate(database, "schema_version", kernelMigrations); err != nil {
		t.Fatalf("repeat schema 60: %v", err)
	}
	for objectType, objectName := range map[string]string{
		"table": "project_ledger_root_relocations",
		"view":  "project_ledger_current_binding",
	} {
		var count int
		if err := database.QueryRow(
			`SELECT COUNT(*) FROM sqlite_schema WHERE type = ? AND name = ?`,
			objectType,
			objectName,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("%s %s count = %d, want 1", objectType, objectName, count)
		}
	}

	var relocationAwareGuards int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM sqlite_schema
		 WHERE type = 'trigger'
		 AND sql LIKE '%project_ledger_current_binding%'`,
	).Scan(&relocationAwareGuards); err != nil {
		t.Fatal(err)
	}
	if relocationAwareGuards != predecessorGuards {
		t.Fatalf(
			"relocation-aware guard count = %d, want predecessor count %d",
			relocationAwareGuards,
			predecessorGuards,
		)
	}

	var staleGuards int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM sqlite_schema
		 WHERE type = 'trigger'
		 AND sql LIKE '%project_ledger_binding%'
		 AND name NOT IN (
			'project_ledger_binding_no_replace',
			'project_ledger_binding_no_update',
			'project_ledger_binding_no_delete',
			'project_ledger_root_relocations_exact_chain',
			'project_ledger_root_relocations_no_cycle'
		 )
		 AND tbl_name NOT IN (
			SELECT DISTINCT tbl_name
			FROM sqlite_schema
			WHERE type = 'trigger'
			AND sql LIKE '%sealed historical%'
		 )`,
	).Scan(&staleGuards); err != nil {
		t.Fatal(err)
	}
	if staleGuards != 0 {
		t.Fatalf("stale genesis-only root guards = %d, want 0", staleGuards)
	}
	if _, err := database.Exec(
		"DELETE FROM schema_version WHERE version = ?",
		ProjectRootRelocationSchemaVersion,
	); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(database, "schema_version", kernelMigrations); err != nil {
		t.Fatalf("repair missing schema-60 receipt without duplicating objects: %v", err)
	}
}
