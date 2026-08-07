package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestProfileAutomaticBootstrapMigration55PreservesExplicitAdmissionStorage(
	t *testing.T,
) {
	t.Parallel()

	database := openDatabaseBeforeProfileAutomaticBootstrapMigration55(t)
	defer database.Close()
	explicitTables := []string{
		"profile_declaration_authority_bases_v3",
		"profile_declaration_authority_resolutions_v3",
		"profile_declaration_authority_uses_v3",
		"project_profile_admissions_v3",
		"project_profile_revisions_v3",
		"project_profile_projection_debt_v3",
	}
	before := make(map[string]string, len(explicitTables))
	for _, table := range explicitTables {
		before[table] = sqliteObjectSQL44(t, database, "table", table)
	}
	if err := Migrate(database, "schema_version", kernelMigrations); err != nil {
		t.Fatalf("migrate v54 to v55: %v", err)
	}
	for _, table := range explicitTables {
		after := sqliteObjectSQL44(t, database, "table", table)
		if after != before[table] {
			t.Fatalf("v55 rewrote explicit admission table %s", table)
		}
	}
	for _, table := range profileAutomaticBootstrapTables55 {
		assertSQLiteObjectExists(t, database, "table", table)
		var count int
		if err := database.QueryRow(
			"SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table),
		).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("v55 backfilled %d row(s) into %s", count, table)
		}
	}
	currentView := sqliteObjectSQL44(t, database, "view", "current_project_profiles")
	for _, required := range []string{
		"'legacy_unknown' FROM project_profile_revisions",
		"'explicit_operator' FROM project_profile_revisions_v3",
		"profile_origin FROM project_profile_revisions_v4",
		"SELECT 'v3', project_root",
		"SELECT 'v4', project_root",
	} {
		if !strings.Contains(currentView, required) {
			t.Fatalf("v55 current profile view omitted %q", required)
		}
	}
	cas := sqliteObjectSQL44(
		t,
		database,
		"trigger",
		"project_profile_admissions_v3_revision_cas",
	)
	if !strings.Contains(cas, "FROM project_profile_revisions_v4 automatic") {
		t.Fatal("v55 explicit admission CAS cannot supersede automatic origin")
	}
	assertMigrationVersionPresent(t, database, 55)
	assertNoForeignKeyViolationsV38(t, database)
}

func TestProfileAutomaticBootstrapMigration55RejectsUnknownPartialFootprint(
	t *testing.T,
) {
	t.Parallel()

	database := openDatabaseBeforeProfileAutomaticBootstrapMigration55(t)
	defer database.Close()
	_, err := database.Exec(
		"CREATE TABLE profile_initial_bootstrap_authority_bases_v1 (unknown TEXT)",
	)
	if err != nil {
		t.Fatalf("seed unknown v55 footprint: %v", err)
	}
	err = Migrate(database, "schema_version", kernelMigrations)
	if err == nil || !strings.Contains(err.Error(), "unversioned table") {
		t.Fatalf("partial v55 footprint error = %v", err)
	}
	assertMigrationVersionAbsent(t, database, 55)
	assertSQLiteObjectAbsent(
		t,
		database,
		"table",
		profileAutomaticAuthorityResolutionTable55,
	)
}

func openDatabaseBeforeProfileAutomaticBootstrapMigration55(
	t *testing.T,
) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "pre-v55.db")
	dsn, err := sqliteConnectionDSN(dbPath)
	if err != nil {
		t.Fatalf("build pre-v55 DSN: %v", err)
	}
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open pre-v55 database: %v", err)
	}
	if err := database.Ping(); err != nil {
		_ = database.Close()
		t.Fatalf("ping pre-v55 database: %v", err)
	}
	if _, err := database.Exec(schema); err != nil {
		_ = database.Close()
		t.Fatalf("install base schema: %v", err)
	}
	migrations := migrationsBeforeVersion(kernelMigrations, 55, 0, nil)
	if err := Migrate(database, "schema_version", migrations); err != nil {
		_ = database.Close()
		t.Fatalf("migrate through v54: %v", err)
	}
	return database
}
