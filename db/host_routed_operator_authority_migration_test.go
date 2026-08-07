package db

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestHostRoutedOperatorAuthorityMigration56InstallsClosedCurrentGeneration(
	t *testing.T,
) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "host-routed-v56.db"))
	if err != nil {
		t.Fatalf("open current store: %v", err)
	}
	defer store.Close()

	assertMigrationVersionPresent(t, store.conn, 56)
	for _, table := range append(
		append([]string(nil), hostRoutedProfileTables56...),
		"project_typeenv_head_selection_host_requests_v1",
		"project_typeenv_head_selection_host_resolutions_v1",
		"project_typeenv_head_selection_host_uses_v1",
	) {
		assertSQLiteObjectExists(t, store.conn, "table", table)
	}

	for _, table := range []string{
		"profile_declaration_authority_bases_v5",
		"profile_declaration_authority_resolutions_v5",
		"project_profile_admissions_v5",
		"profile_declaration_authority_uses_v5",
		"project_typeenv_head_selection_authority_resolutions",
		"project_typeenv_head_selection_authority_uses",
	} {
		ddl := sqliteObjectSQL44(t, store.conn, "table", table)
		if !strings.Contains(ddl, hostRoutedAuthorityMode56) {
			t.Fatalf("current authority table %s omits host-routed generation", table)
		}
		for _, retired := range []string{
			"'explicit_h_decide'",
			"'explicit_h_onboard'",
			"'strict_cli_speech_act'",
			"'strict_permission'",
		} {
			if strings.Contains(ddl, retired) {
				t.Fatalf("current authority table %s admits retired value %s", table, retired)
			}
		}
	}

	view := sqliteObjectSQL44(t, store.conn, "view", "current_project_profiles")
	for _, generation := range []string{"SELECT 'v4'", "SELECT 'v5'"} {
		if !strings.Contains(view, generation) {
			t.Fatalf("current profile view omits %q", generation)
		}
	}
	assertNoForeignKeyViolationsV38(t, store.conn)
}

func TestHostRoutedOperatorAuthorityMigration56SealsHistoricalWriters(
	t *testing.T,
) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "sealed-authority-v56.db"))
	if err != nil {
		t.Fatalf("open current store: %v", err)
	}
	defer store.Close()

	for _, table := range []string{
		"profile_declaration_authority_bases_v3",
		"profile_declaration_authority_resolutions_v3",
		"project_typeenv_head_selection_config_authority_bases",
		"project_typeenv_head_selection_mode_policies",
		"project_typeenv_head_selection_strict_permission_resolutions",
	} {
		_, err := store.conn.Exec("INSERT INTO " + quoteSQLiteIdentifier(table) + " DEFAULT VALUES")
		if err == nil || !strings.Contains(err.Error(), "sealed historical authority") {
			t.Fatalf("historical authority writer %s error = %v", table, err)
		}
	}
}

func TestHostRoutedOperatorAuthorityMigration56PreservesTypeEnvHeadAndHistory(
	t *testing.T,
) {
	t.Parallel()

	database, basisTypeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, basisTypeEnvRef)
	migrateProjectTypeEnvHeadSelection47(t, database)
	resultTypeEnvRef := insertSecondProjectTypeEnvExecutable47(t, database)
	fixture := newProjectTypeEnvHeadEffectFixture47(
		basisTypeEnvRef,
		resultTypeEnvRef,
	)
	insertProjectTypeEnvCandidateStage47(t, database, fixture)
	commitCompleteGenesisHeadEffect47(t, database, fixture)

	through55 := migrationsBeforeVersion(kernelMigrations, 56, 0, nil)
	if err := Migrate(database, "schema_version", through55); err != nil {
		t.Fatalf("migrate TypeEnv fixture through v55: %v", err)
	}
	assertMigrationVersionPresent(t, database, 55)
	assertMigrationVersionAbsent(t, database, 56)

	var headRefBefore string
	var selectedBefore string
	var revisionBefore int
	if err := database.QueryRow(`SELECT head_ref, selected_composite_ref, head_revision
		FROM project_typeenv_heads WHERE project_id = ?`, fixture.projectID).Scan(
		&headRefBefore,
		&selectedBefore,
		&revisionBefore,
	); err != nil {
		t.Fatalf("read v55 TypeEnv head: %v", err)
	}
	var historyBefore int
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM project_typeenv_head_history",
	).Scan(&historyBefore); err != nil {
		t.Fatalf("count v55 TypeEnv history: %v", err)
	}

	if err := Migrate(
		database,
		"schema_version",
		[]Migration{hostRoutedOperatorAuthorityMigration56},
	); err != nil {
		t.Fatalf("migrate TypeEnv fixture from v55 to v56: %v", err)
	}

	var headRefAfter string
	var selectedAfter string
	var revisionAfter int
	if err := database.QueryRow(`SELECT head_ref, selected_composite_ref, head_revision
		FROM project_typeenv_heads WHERE project_id = ?`, fixture.projectID).Scan(
		&headRefAfter,
		&selectedAfter,
		&revisionAfter,
	); err != nil {
		t.Fatalf("read v56 TypeEnv head: %v", err)
	}
	if headRefAfter != headRefBefore ||
		selectedAfter != selectedBefore ||
		revisionAfter != revisionBefore {
		t.Fatalf(
			"v56 changed TypeEnv head: before=(%s,%s,%d) after=(%s,%s,%d)",
			headRefBefore,
			selectedBefore,
			revisionBefore,
			headRefAfter,
			selectedAfter,
			revisionAfter,
		)
	}
	var historyAfter int
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM project_typeenv_head_history",
	).Scan(&historyAfter); err != nil {
		t.Fatalf("count v56 TypeEnv history: %v", err)
	}
	if historyAfter != historyBefore {
		t.Fatalf("v56 TypeEnv history rows = %d, want %d", historyAfter, historyBefore)
	}

	var generation string
	if err := database.QueryRow(`SELECT authority_generation
		FROM project_typeenv_head_selection_authority_resolutions
		WHERE project_id = ?`, fixture.projectID).Scan(&generation); err != nil {
		t.Fatalf("read migrated authority generation: %v", err)
	}
	if generation != "legacy_unreproducible" {
		t.Fatalf("migrated authority generation = %q", generation)
	}
	assertTypedMemoryTableRowCount45(
		t,
		database,
		"project_typeenv_head_selection_authority_resolutions_legacy_v47",
		1,
	)
	assertTypedMemoryForeignKeysClean45(t, database)
}
