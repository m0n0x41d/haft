package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectTypeEnvCompatibleSuccessorMigration57InstallsClosedAuthorityGeneration(
	t *testing.T,
) {
	store, err := NewStore(filepath.Join(t.TempDir(), "compatible-successor-v57.db"))
	if err != nil {
		t.Fatalf("open current store: %v", err)
	}
	defer store.Close()

	assertMigrationVersionPresent(t, store.conn, 57)
	for _, table := range []string{
		"project_typeenv_head_selection_compatible_resolutions_v1",
		"project_typeenv_head_selection_compatible_uses_v1",
	} {
		assertSQLiteObjectExists(t, store.conn, "table", table)
	}
	for _, table := range []string{
		"project_typeenv_head_selection_authority_resolutions",
		"project_typeenv_head_selection_authority_uses",
	} {
		ddl := sqliteObjectSQL44(t, store.conn, "table", table)
		for _, current := range []string{
			hostRoutedAuthorityMode56,
			typeEnvCompatibleAuthorityGeneration57,
		} {
			if !strings.Contains(ddl, current) {
				t.Fatalf("current authority table %s omits %s", table, current)
			}
		}
	}
	graphDDL := sqliteObjectSQL44(t, store.conn, "table", "typed_memory_graph_events")
	for _, authorityClass := range []string{
		"manual_type_env_activation",
		hostRoutedAuthorityMode56,
		typeEnvCompatibleAuthorityGeneration57,
	} {
		if !strings.Contains(graphDDL, authorityClass) {
			t.Fatalf("v57 graph-event table omits authority class %s", authorityClass)
		}
	}
	for _, trigger := range []string{
		"typed_memory_type_env_activations_v47_exact_effect",
		"typed_memory_graph_commits_v47_activation_effect",
	} {
		ddl := sqliteObjectSQL44(t, store.conn, "trigger", trigger)
		for _, authorityClass := range []string{
			hostRoutedAuthorityMode56,
			typeEnvCompatibleAuthorityGeneration57,
		} {
			if !strings.Contains(ddl, authorityClass) {
				t.Fatalf("v57 trigger %s omits authority class %s", trigger, authorityClass)
			}
		}
	}
	assertNoForeignKeyViolationsV38(t, store.conn)
}

func TestProjectTypeEnvCompatibleSuccessorMigration57PreservesV56History(
	t *testing.T,
) {
	database, basisTypeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, basisTypeEnvRef)
	migrateProjectTypeEnvHeadSelection47(t, database)
	resultTypeEnvRef := insertSecondProjectTypeEnvExecutable47(t, database)
	fixture := newProjectTypeEnvHeadEffectFixture47(basisTypeEnvRef, resultTypeEnvRef)
	insertProjectTypeEnvCandidateStage47(t, database, fixture)
	commitCompleteGenesisHeadEffect47(t, database, fixture)

	through56 := migrationsBeforeVersion(kernelMigrations, 57, 0, nil)
	if err := Migrate(database, "schema_version", through56); err != nil {
		t.Fatalf("migrate TypeEnv fixture through v56: %v", err)
	}
	assertMigrationVersionPresent(t, database, 56)
	assertMigrationVersionAbsent(t, database, 57)

	tables := []string{
		"project_typeenv_heads",
		"project_typeenv_head_history",
		"project_typeenv_head_selection_authority_resolutions",
		"project_typeenv_head_selection_authority_uses",
	}
	before := make(map[string]int, len(tables))
	for _, table := range tables {
		before[table] = migration57TableCount(t, database, table)
	}

	if err := Migrate(
		database,
		"schema_version",
		[]Migration{projectTypeEnvCompatibleSuccessorMigration57},
	); err != nil {
		t.Fatalf("migrate TypeEnv fixture from v56 to v57: %v", err)
	}
	for _, table := range tables {
		if got := migration57TableCount(t, database, table); got != before[table] {
			t.Fatalf("v57 changed %s row count from %d to %d", table, before[table], got)
		}
	}
	var generation string
	if err := database.QueryRow(`SELECT authority_generation
		FROM project_typeenv_head_selection_authority_resolutions
		WHERE project_id = ?`, fixture.projectID).Scan(&generation); err != nil {
		t.Fatalf("read v57 migrated authority generation: %v", err)
	}
	if generation != "legacy_unreproducible" {
		t.Fatalf("v57 migrated authority generation = %q", generation)
	}
	var graphAuthorityClass string
	if err := database.QueryRow(`SELECT authority_class
		FROM typed_memory_graph_events
		WHERE project_id = ?`, fixture.projectID).Scan(&graphAuthorityClass); err != nil {
		t.Fatalf("read v57 migrated graph authority class: %v", err)
	}
	if graphAuthorityClass != "manual_type_env_activation" {
		t.Fatalf("v57 changed historical graph authority class to %q", graphAuthorityClass)
	}
	assertTypedMemoryForeignKeysClean45(t, database)
}

func migration57TableCount(t *testing.T, database *sql.DB, table string) int {
	t.Helper()
	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table)).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}
