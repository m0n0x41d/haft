package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestTypedMemoryIdentityReconciliationMigration52UpgradesV51Additively(
	t *testing.T,
) {
	database := openDatabaseBeforeIdentityReconciliation52(t)
	defer database.Close()
	legacyTrigger := sqliteObjectSQL44(
		t,
		database,
		"trigger",
		"typed_memory_graph_commits_exact_closure",
	)

	if err := Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryIdentityReconciliationMigration52},
	); err != nil {
		t.Fatalf("migrate v51 database through v52: %v", err)
	}
	assertMigrationVersionPresent(t, database, 52)
	for _, object := range mustTypedMemoryIdentityObjects52(t) {
		assertSQLiteObjectExists(t, database, object.kind, object.name)
	}
	currentTrigger := sqliteObjectSQL44(
		t,
		database,
		"trigger",
		"typed_memory_graph_commits_exact_closure",
	)
	if currentTrigger == legacyTrigger ||
		!strings.Contains(currentTrigger, "typed_memory_identity_reconciliations") {
		t.Fatal("v52 did not extend the exact graph-commit closure with reviewed identity events")
	}
	assertNoForeignKeyViolationsV38(t, database)
	assertSQLiteIntegrity52(t, database)
}

func TestTypedMemoryIdentityReconciliationMigration52RollsBackAtomicallyOnConflict(
	t *testing.T,
) {
	database := openDatabaseBeforeIdentityReconciliation52(t)
	defer database.Close()
	legacyTrigger := sqliteObjectSQL44(
		t,
		database,
		"trigger",
		"typed_memory_graph_commits_exact_closure",
	)
	_, err := database.Exec(
		`CREATE TABLE typed_memory_identity_redirects (sentinel TEXT PRIMARY KEY) WITHOUT ROWID`,
	)
	if err != nil {
		t.Fatalf("install conflicting v52 footprint: %v", err)
	}

	err = Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryIdentityReconciliationMigration52},
	)
	if err == nil || !strings.Contains(err.Error(), "footprint typed_memory_identity_redirects already exists") {
		t.Fatalf("conflicting v52 migration error = %v", err)
	}
	assertMigrationVersionAbsent(t, database, 52)
	for _, table := range []string{
		typedMemoryIdentityReconciliationsTable52,
		typedMemoryIdentityParticipantsTable52,
		typedMemoryIdentityClosuresTable52,
	} {
		assertSQLiteObjectAbsent49(t, database, "table", table)
	}
	assertSQLiteObjectExists(t, database, "table", typedMemoryIdentityRedirectsTable52)
	currentTrigger := sqliteObjectSQL44(
		t,
		database,
		"trigger",
		"typed_memory_graph_commits_exact_closure",
	)
	if currentTrigger != legacyTrigger {
		t.Fatal("failed v52 migration changed the v51 graph-commit trigger")
	}
	assertNoForeignKeyViolationsV38(t, database)
	assertSQLiteIntegrity52(t, database)
}

func TestTypedMemoryIdentityReconciliationMigration52FootprintIsImmutable(
	t *testing.T,
) {
	store, err := NewStore(filepath.Join(t.TempDir(), "identity-v52.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()
	for _, table := range []string{
		typedMemoryIdentityReconciliationsTable52,
		typedMemoryIdentityParticipantsTable52,
		typedMemoryIdentityRedirectsTable52,
		typedMemoryIdentityClosuresTable52,
	} {
		for _, action := range []string{"update", "delete"} {
			assertSQLiteObjectExists(t, store.conn, "trigger", table+"_v52_no_"+action)
		}
	}
	assertSQLiteIntegrity52(t, store.conn)
}

func openDatabaseBeforeIdentityReconciliation52(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pre-v52.db")
	dsn, err := sqliteConnectionDSN(path)
	if err != nil {
		t.Fatalf("build pre-v52 SQLite DSN: %v", err)
	}
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open pre-v52 SQLite database: %v", err)
	}
	if err := database.Ping(); err != nil {
		_ = database.Close()
		t.Fatalf("ping pre-v52 SQLite database: %v", err)
	}
	if _, err := database.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = database.Close()
		t.Fatalf("enable pre-v52 WAL: %v", err)
	}
	if _, err := database.Exec(schema); err != nil {
		_ = database.Close()
		t.Fatalf("install pre-v52 base schema: %v", err)
	}
	migrations := migrationsBeforeVersion(kernelMigrations, 52, 0, nil)
	if err := Migrate(database, "schema_version", migrations); err != nil {
		_ = database.Close()
		t.Fatalf("migrate database through v51: %v", err)
	}
	return database
}

func mustTypedMemoryIdentityObjects52(t *testing.T) []typedMemoryIdentityObject52 {
	t.Helper()
	objects, err := typedMemoryIdentityObjects52()
	if err != nil {
		t.Fatalf("typedMemoryIdentityObjects52: %v", err)
	}
	return objects
}

func assertSQLiteIntegrity52(t *testing.T, database *sql.DB) {
	t.Helper()
	var result string
	if err := database.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		t.Fatalf("run SQLite integrity_check: %v", err)
	}
	if result != "ok" {
		t.Fatalf("SQLite integrity_check = %q; want ok", result)
	}
}
