package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestProfileAuthorityUnionMigration51InstallsClosedV3Contract(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "profile-v51.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	for _, table := range profileAuthorityUnionTables51 {
		assertSQLiteObjectExists(t, store.conn, "table", table)
	}
	for _, trigger := range []string{
		"profile_declaration_authority_bases_v3_exact_sources",
		"profile_declaration_authority_bases_v3_no_cross_generation_collision",
		"profile_declaration_authority_resolutions_v3_exact_sources",
		"profile_declaration_authority_resolutions_v3_no_cross_generation_collision",
		"project_profile_admissions_v3_revision_cas",
		"project_profile_admissions_v3_exact_sources",
		"project_profile_admissions_v3_no_cross_generation_collision",
		"profile_declaration_authority_uses_v3_exact_sources",
		"profile_declaration_authority_uses_v3_no_cross_generation_collision",
		"project_profile_revisions_v3_exact_admission",
		"project_profile_revisions_v3_no_cross_generation_collision",
		"project_profile_projection_debt_v3_exact_admission",
		"project_profile_projection_debt_v3_no_cross_generation_collision",
	} {
		assertSQLiteObjectExists(t, store.conn, "trigger", trigger)
	}
	for _, table := range profileAdmissionV2WriteTables51 {
		assertSQLiteObjectExists(t, store.conn, "trigger", table+"_v51_writes_sealed")
	}
	assertMigrationVersionPresent(t, store.conn, 51)

	assertExactColumnOrder44(t, store.conn, profileOnboardingWorkInputTable51, []string{
		"work_input_ref", "work_input_digest", "project_root", "suggestion_ref",
		"detector_version", "policy_version", "observation_digest",
		"profile_payload_json", "profile_payload_digest", "canonical_json", "recorded_at",
	})
	assertExactColumnOrder44(t, store.conn, profileAuthorityBasisTable51, []string{
		"basis_ref", "basis_digest", "project_root", "action_kind", "authority_mode",
		"work_input_ref", "work_input_digest",
		"profile_author_role_assignment_ref", "profile_author_role_assignment_digest",
		"method_description_ref", "method_description_digest",
		"method_contract_ref", "method_contract_digest",
		"classifier_version", "policy_version", "future_work_session_ref",
		"allowed_work_from", "allowed_work_until",
		"basis_observation_from", "basis_observation_until", "single_use_key",
		"config_carrier_ref", "config_carrier_digest",
		"strict_authority_basis_ref", "strict_authority_basis_digest",
		"canonical_json", "recorded_at",
	})
	assertExactColumnOrder44(t, store.conn, profileAuthorityResolutionTable51, []string{
		"authority_resolution_ref", "authority_resolution_digest",
		"authority_basis_ref", "authority_basis_digest",
		"project_root", "action_kind", "authority_mode", "resolution_kind",
		"work_input_ref", "work_input_digest", "project_binding_digest",
		"strict_permission_ref", "strict_permission_digest",
		"verifier_identity", "verifier_version",
		"verification_policy_ref", "verification_policy_digest",
		"checked_at", "currentness_result", "predicate_result", "admission_result",
		"canonical_json", "recorded_at",
	})
	assertExactColumnOrder44(t, store.conn, profileAuthorityUseTable51, []string{
		"use_ref", "use_digest", "project_root", "action_kind",
		"authority_mode", "resolution_kind", "project_binding_digest",
		"authority_resolution_ref", "authority_resolution_digest",
		"authority_basis_ref", "authority_basis_digest",
		"work_input_ref", "work_input_digest", "single_use_key",
		"admission_request_digest", "committed_admission_ref",
		"committed_admission_digest", "canonical_json", "consumed_at", "recorded_at",
	})

	basisDDL := sqliteObjectSQL44(t, store.conn, "table", profileAuthorityBasisTable51)
	for _, required := range []string{
		"authority_mode IN ('explicit_h_onboard', 'strict_cli_speech_act')",
		"config_carrier_ref IS NOT NULL",
		"strict_authority_basis_ref IS NULL",
		"config_carrier_ref IS NULL",
		"strict_authority_basis_ref IS NOT NULL",
	} {
		if !strings.Contains(basisDDL, required) {
			t.Fatalf("v3 authority-basis closed sum omitted %q", required)
		}
	}
	resolutionDDL := sqliteObjectSQL44(t, store.conn, "table", profileAuthorityResolutionTable51)
	for _, required := range []string{
		"resolution_kind IN ('explicit_policy_acceptance', 'strict_permission')",
		"authority_mode = 'explicit_h_onboard' AND resolution_kind = 'explicit_policy_acceptance'",
		"authority_mode = 'strict_cli_speech_act' AND resolution_kind = 'strict_permission'",
	} {
		if !strings.Contains(resolutionDDL, required) {
			t.Fatalf("v3 authority-resolution closed sum omitted %q", required)
		}
	}

	assertForeignKeyTarget44(
		t,
		store.conn,
		profileAuthorityBasisTable51,
		"work_input_ref",
		profileOnboardingWorkInputTable51,
		"work_input_ref",
	)
	assertForeignKeyTarget44(
		t,
		store.conn,
		projectProfileAdmissionTable51,
		"authority_resolution_ref",
		profileAuthorityResolutionTable51,
		"authority_resolution_ref",
	)
	assertForeignKeyTarget44(
		t,
		store.conn,
		profileAuthorityUseTable51,
		"committed_admission_ref",
		projectProfileAdmissionTable51,
		"admission_id",
	)
	assertNoForeignKeyViolationsV38(t, store.conn)
}

func TestProfileAuthorityUnionMigration51PinsExactSourcesCASAndUnionView(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "profile-v51-contract.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	basisTrigger := sqliteObjectSQL44(
		t,
		store.conn,
		"trigger",
		"profile_declaration_authority_bases_v3_exact_sources",
	)
	for _, required := range []string{
		"FROM profile_onboarding_work_inputs_v1 work_input",
		"JOIN profile_author_role_assignments assignment",
		"JOIN profile_onboarding_method_descriptions description",
		"JOIN profile_onboarding_method_contracts contract",
		"FROM profile_declaration_authority_bases_v2 strict_basis",
		"JOIN profile_declaration_permissions_v2 permission",
	} {
		if !strings.Contains(basisTrigger, required) {
			t.Fatalf("v3 authority-basis exact-source trigger omitted %q", required)
		}
	}

	admissionTrigger := sqliteObjectSQL44(
		t,
		store.conn,
		"trigger",
		"project_profile_admissions_v3_exact_sources",
	)
	for _, required := range []string{
		"JOIN profile_onboarding_work_inputs_v1 work_input",
		"json(work_input.profile_payload_json) = json(NEW.profile_payload_json)",
		"JOIN profile_onboarding_work_records work_record",
		"JOIN profile_onboarding_outcome_assessments assessment",
		"JOIN profile_onboarding_effects work_effect",
		"NEW.recorded_at",
		"authority_basis.allowed_work_until",
		"resolution.strict_permission_ref",
	} {
		if !strings.Contains(admissionTrigger, required) {
			t.Fatalf("v3 admission exact-source trigger omitted %q", required)
		}
	}

	useTrigger := sqliteObjectSQL44(
		t,
		store.conn,
		"trigger",
		"profile_declaration_authority_uses_v3_exact_sources",
	)
	for _, required := range []string{
		"JOIN profile_onboarding_work_inputs_v1 work_input",
		"JOIN project_profile_admissions_v3 admission",
		"NEW.consumed_at",
		"basis.allowed_work_until",
	} {
		if !strings.Contains(useTrigger, required) {
			t.Fatalf("v3 authority-use exact-source trigger omitted %q", required)
		}
	}

	resolutionTrigger := sqliteObjectSQL44(
		t,
		store.conn,
		"trigger",
		"profile_declaration_authority_resolutions_v3_exact_sources",
	)
	for _, required := range []string{
		"binding.binding_digest = NEW.project_binding_digest",
		sqliteUTCNanoLessOrEqual("basis.recorded_at", "NEW.checked_at"),
		sqliteUTCNanoLessOrEqual("basis.allowed_work_from", "NEW.checked_at"),
		sqliteUTCNanoLessOrEqual("NEW.checked_at", "basis.allowed_work_until"),
	} {
		if !strings.Contains(resolutionTrigger, required) {
			t.Fatalf("v3 resolution timing contract omitted %q", required)
		}
	}
	for _, forbidden := range []string{
		sqliteUTCNanoLessOrEqual("NEW.checked_at", "basis.allowed_work_from"),
	} {
		if strings.Contains(resolutionTrigger, forbidden) {
			t.Fatalf("v3 resolution timing retained reversed constraint %q", forbidden)
		}
	}

	casTrigger := sqliteObjectSQL44(
		t,
		store.conn,
		"trigger",
		"project_profile_admissions_v3_revision_cas",
	)
	for _, required := range []string{
		"FROM project_profile_revisions legacy",
		"FROM project_profile_revisions_v2 previous",
		"FROM project_profile_revisions_v3 historical",
		"FROM project_profile_revisions_v4 automatic",
		"FROM project_profile_revisions_v5 current",
		"COUNT(DISTINCT ledger_revision)",
		"minimum_revision = 1",
		"maximum_revision = NEW.expected_ledger_revision",
	} {
		if !strings.Contains(casTrigger, required) {
			t.Fatalf("v3 cross-generation CAS omitted %q", required)
		}
	}

	currentView := sqliteObjectSQL44(t, store.conn, "view", "current_project_profiles")
	for _, required := range []string{
		"SELECT 'v1'",
		"SELECT 'v2'",
		"SELECT 'v3'",
		"SELECT 'v4'",
		"SELECT 'v5'",
		"COUNT(DISTINCT ledger_revision) = COUNT(*)",
	} {
		if !strings.Contains(currentView, required) {
			t.Fatalf("current-profile v1/v2/v3 union view omitted %q", required)
		}
	}
}

func TestProfileAuthorityUnionMigration51UpgradesHistoryWithoutBackfill(t *testing.T) {
	database := openDatabaseBeforeProfileAuthorityAdmissionV2Migration44(t)
	defer database.Close()
	seedHistoricalV1ProfileRevision44(t, database)

	through50 := migrationsBeforeVersion(kernelMigrations, 51, 0, nil)
	if err := Migrate(database, "schema_version", through50); err != nil {
		t.Fatalf("migrate historical profile through v50: %v", err)
	}
	if err := Migrate(database, "schema_version", kernelMigrations); err != nil {
		t.Fatalf("upgrade historical profile through v51: %v", err)
	}

	var generation string
	var revision int
	var admissionID string
	err := database.QueryRow(`SELECT storage_generation, ledger_revision, admission_id
		FROM current_project_profiles WHERE project_root = '/tmp/project'`).Scan(
		&generation,
		&revision,
		&admissionID,
	)
	if err != nil {
		t.Fatalf("read preserved current profile: %v", err)
	}
	if generation != "v1" || revision != 1 || admissionID != "admission:test" {
		t.Fatalf("preserved current profile = %q rev %d %q", generation, revision, admissionID)
	}
	for _, table := range profileAuthorityUnionTables51 {
		var count int
		if err := database.QueryRow("SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table)).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("v51 migration backfilled %d row(s) into %s", count, table)
		}
	}

	if err := Migrate(database, "schema_version", kernelMigrations); err != nil {
		t.Fatalf("repeat v51 migration: %v", err)
	}
	assertMigrationVersionPresent(t, database, 51)
	assertNoForeignKeyViolationsV38(t, database)
}

func TestProfileAuthorityUnionMigration51RejectsUnknownPartialFootprint(t *testing.T) {
	database := openDatabaseBeforeProfileAuthorityUnionMigration51(t)
	defer database.Close()

	_, err := database.Exec(`CREATE TABLE profile_onboarding_work_inputs_v1 (unknown TEXT)`)
	if err != nil {
		t.Fatalf("seed unknown partial v51 footprint: %v", err)
	}
	err = Migrate(database, "schema_version", kernelMigrations)
	if err == nil || !strings.Contains(err.Error(), "unknown partial schema") {
		t.Fatalf("partial v51 footprint error = %v", err)
	}
	assertMigrationVersionAbsent(t, database, 51)
	assertSQLiteObjectAbsent(t, database, "table", profileAuthorityBasisTable51)
}

func openDatabaseBeforeProfileAuthorityUnionMigration51(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "pre-v51.db")
	dsn, err := sqliteConnectionDSN(dbPath)
	if err != nil {
		t.Fatalf("build pre-v51 DSN: %v", err)
	}
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open pre-v51 database: %v", err)
	}
	if err := database.Ping(); err != nil {
		_ = database.Close()
		t.Fatalf("ping pre-v51 database: %v", err)
	}
	if _, err := database.Exec(schema); err != nil {
		_ = database.Close()
		t.Fatalf("install base schema: %v", err)
	}
	migrations := migrationsBeforeVersion(kernelMigrations, 51, 0, nil)
	if err := Migrate(database, "schema_version", migrations); err != nil {
		_ = database.Close()
		t.Fatalf("migrate pre-v51 database: %v", err)
	}
	return database
}
