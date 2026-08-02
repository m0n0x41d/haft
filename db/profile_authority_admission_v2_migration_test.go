package db

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestProfileAuthorityAdmissionV2Migration44InstallsExactAdditiveContract(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "profile-v44.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	for _, table := range profileAuthorityAdmissionV2Tables {
		assertSQLiteObjectExists(t, store.conn, "table", table)
	}
	for _, trigger := range []string{
		"profile_declaration_authority_resolutions_v2_exact_sources",
		"profile_declaration_authority_resolutions_v2_no_cross_generation_collision",
		"profile_declaration_authority_uses_v2_exact_sources",
		"profile_declaration_authority_uses_v2_no_cross_generation_collision",
		"project_profile_admissions_v2_revision_cas",
		"project_profile_admissions_v2_exact_sources",
		"project_profile_admissions_v2_no_cross_generation_collision",
		"project_profile_revisions_v2_exact_admission",
		"project_profile_revisions_v2_no_cross_generation_collision",
		"project_profile_projection_debt_v2_exact_admission",
		"project_profile_projection_debt_v2_no_cross_generation_collision",
	} {
		assertSQLiteObjectExists(t, store.conn, "trigger", trigger)
	}
	for _, table := range legacyProfileWriteTables44 {
		assertSQLiteObjectExists(t, store.conn, "trigger", table+"_v44_writes_sealed")
	}
	assertSQLiteObjectAbsent(t, store.conn, "trigger", "speech_acts_v44_writes_sealed")
	assertSQLiteObjectAbsent(t, store.conn, "trigger", "terminal_capture_records_v44_writes_sealed")
	assertMigrationVersionPresent(t, store.conn, 44)

	assertExactColumnOrder44(t, store.conn, profileAuthorityV2ResolutionTable, []string{
		"authority_resolution_ref", "authority_resolution_digest",
		"authority_basis_ref", "authority_basis_digest",
		"speech_act_ref", "speech_act_digest",
		"authorization_content_ref", "authorization_content_digest",
		"permission_ref", "permission_digest",
		"context_policy_ref", "context_policy_digest",
		"action_kind", "project_root", "action_envelope_digest", "project_binding_digest",
		"profile_author_role_assignment_ref", "profile_author_role_assignment_digest",
		"claim_scope_ref", "bounded_context_ref",
		"method_description_ref", "method_description_digest",
		"method_contract_ref", "method_contract_digest",
		"classifier_version", "policy_version", "future_work_session_ref",
		"allowed_work_from", "allowed_work_until",
		"basis_observation_from", "basis_observation_until",
		"authorization_valid_from", "authorization_valid_until",
		"permission_valid_from", "permission_valid_until",
		"single_use_key", "enactability_predicate_ref",
		"verifier_identity", "verifier_version",
		"verification_policy_ref", "verification_policy_digest",
		"checked_at", "role_state_relation", "enactable_state",
		"currentness_result", "predicate_result", "admission_result",
		"canonical_json", "recorded_at",
	})
	assertExactColumnOrder44(t, store.conn, profileAuthorityV2UseTable, []string{
		"use_ref", "use_digest", "project_root", "action_kind", "project_binding_digest",
		"authority_resolution_ref", "authority_resolution_digest",
		"authority_basis_ref", "authority_basis_digest",
		"permission_ref", "permission_digest",
		"authorization_content_ref", "authorization_content_digest",
		"single_use_key", "admission_request_digest",
		"committed_admission_ref", "committed_admission_digest",
		"canonical_json", "consumed_at", "recorded_at",
	})
	assertExactColumnOrder44(t, store.conn, "current_project_profiles", []string{
		"storage_generation", "project_root", "ledger_revision", "configured_profile_kind",
		"profile_payload_json", "profile_payload_digest",
		"receipt_json", "receipt_digest", "admission_id", "admission_digest", "recorded_at",
		"profile_origin",
	})
	err = insertGenericSpeechActSourceFixture(
		store.conn,
		"/tmp/generic-speech-act",
		"bounded-context:generic-manual-authority",
		"bounded-context:generic-manual-authority",
	)
	if err != nil {
		t.Fatalf("v44 sealed a generic SpeechAct source write: %v", err)
	}
	assertTableRowCountV38(t, store.conn, "speech_acts", 1)
	assertNoForeignKeyViolationsV38(t, store.conn)
}

func TestProfileAuthorityAdmissionV2Migration44RejectsUnknownPartialFootprint(t *testing.T) {
	database := openDatabaseBeforeProfileAuthorityAdmissionV2Migration44(t)
	defer database.Close()

	_, err := database.Exec(`CREATE TABLE project_profile_admissions_v2 (unknown TEXT)`)
	if err != nil {
		t.Fatalf("seed unknown partial v44 footprint: %v", err)
	}
	err = Migrate(database, "schema_version", kernelMigrations)
	if err == nil || !strings.Contains(err.Error(), "unknown partial schema") {
		t.Fatalf("partial v44 footprint error = %v", err)
	}
	assertMigrationVersionAbsent(t, database, 44)
	assertSQLiteObjectAbsent(t, database, "table", profileAuthorityV2ResolutionTable)
	assertSQLiteObjectAbsent(t, database, "table", profileAuthorityV2UseTable)
}

func TestProfileAuthorityAdmissionV2Migration44RejectsMissingV43Guard(t *testing.T) {
	database := openDatabaseBeforeProfileAuthorityAdmissionV2Migration44(t)
	defer database.Close()

	_, err := database.Exec("DROP TRIGGER profile_declaration_authority_bases_v2_exact_sources")
	if err != nil {
		t.Fatalf("drop v43 basis guard: %v", err)
	}
	err = Migrate(database, "schema_version", kernelMigrations)
	if err == nil || !strings.Contains(
		err.Error(),
		"requires exact dependency trigger profile_declaration_authority_bases_v2_exact_sources",
	) {
		t.Fatalf("missing v43 guard error = %v", err)
	}
	assertMigrationVersionAbsent(t, database, 44)
}

func TestProfileAuthorityAdmissionV2Migration44PreservesV1HistoryWithoutBackfill(t *testing.T) {
	database := openDatabaseBeforeProfileAuthorityAdmissionV2Migration44(t)
	defer database.Close()
	seedHistoricalV1ProfileRevision44(t, database)

	through44 := migrationsBeforeVersion(kernelMigrations, 45, 0, nil)
	if err := Migrate(database, "schema_version", through44); err != nil {
		t.Fatalf("migrate v1 profile history through v44: %v", err)
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
		t.Fatalf("read preserved v1 current profile: %v", err)
	}
	if generation != "v1" || revision != 1 || admissionID != "admission:test" {
		t.Fatalf("preserved current profile = %q rev %d %q", generation, revision, admissionID)
	}
	for _, table := range profileAuthorityAdmissionV2Tables {
		var count int
		if err := database.QueryRow("SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table)).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("migration backfilled %d row(s) into %s", count, table)
		}
	}

	var historicalDigest string
	err = database.QueryRow(
		"SELECT admission_digest FROM project_profile_admissions WHERE admission_id = 'admission:test'",
	).Scan(&historicalDigest)
	if err != nil || historicalDigest != "digest:admission" {
		t.Fatalf("read historical v1 admission digest = %q err=%v", historicalDigest, err)
	}
	_, err = database.Exec(`INSERT INTO project_profile_revisions (
		project_root, ledger_revision, configured_profile_kind,
		profile_payload_json, profile_payload_digest,
		receipt_json, receipt_digest, admission_id, admission_digest, recorded_at
	) VALUES ('/tmp/another', 1, 'Declared', '{}', 'digest:new-payload',
		'{}', 'digest:new-receipt', 'admission:new', 'digest:new-admission', '2026-07-15T00:00:00Z')`)
	if err == nil || !strings.Contains(err.Error(), "legacy profile writes are sealed") {
		t.Fatalf("legacy v1 profile write error = %v", err)
	}

	insertProjectLedgerBinding44(t, database, "/tmp/project")
	if _, err := database.Exec("DROP TRIGGER project_profile_admissions_v2_exact_sources"); err != nil {
		t.Fatalf("isolate v44 cross-generation admission guards: %v", err)
	}
	err = insertSyntacticV2Admission44(t, database, 1, "use.typed", "collision")
	if err == nil || !strings.Contains(err.Error(), "cross-generation profile history") {
		t.Fatalf("cross-generation single-use collision error = %v", err)
	}
	err = insertSyntacticV2Admission44(t, database, 2, "single-use:v2:stale", "stale")
	if err == nil || !strings.Contains(err.Error(), "cross-generation ledger revision conflict") {
		t.Fatalf("cross-generation stale CAS error = %v", err)
	}
}

func TestProfileAuthorityAdmissionV2Migration44PinsCrossGenerationAndTypedJoins(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "profile-v44-contract.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	resolutionTable := sqliteObjectSQL44(
		t,
		store.conn,
		"table",
		profileAuthorityV2ResolutionTable,
	)
	if !strings.Contains(
		resolutionTable,
		"json_extract(canonical_json, '$.enactability_predicate_ref') = enactability_predicate_ref",
	) {
		t.Fatal("resolution table omitted canonical Permission-enactability predicate binding")
	}
	if strings.Contains(resolutionTable, "$.admission_predicate_ref") {
		t.Fatal("resolution table retained the later candidate-admission predicate alias")
	}

	resolutionTrigger := sqliteObjectSQL44(
		t,
		store.conn,
		"trigger",
		"profile_declaration_authority_resolutions_v2_exact_sources",
	)
	for _, required := range []string{
		"FROM profile_declaration_authority_bases_v2 basis",
		"JOIN profile_declaration_authorization_contents_v2 content",
		"JOIN profile_declaration_permissions_v2 permission",
		"JOIN profile_declaration_instituted_effects_v2 effect",
		"JOIN speech_acts act",
		"JOIN speech_act_context_policies policy",
		"JOIN profile_author_role_assignments assignment",
		"NEW.role_state_relation = 'A.2.5.EnactableStateAdmission'",
		"permission.enactability_predicate_ref = NEW.enactability_predicate_ref",
		"act.window_until",
		"NEW.checked_at",
	} {
		if !strings.Contains(resolutionTrigger, required) {
			t.Fatalf("resolution exact-source trigger omitted %q", required)
		}
	}
	for _, forbidden := range []string{
		"binding.binding_digest = NEW.project_binding_digest",
		"NEW.admission_predicate_ref",
		"CAST(",
		"authority_presentations",
		"authority_resolution_records",
	} {
		if strings.Contains(resolutionTrigger, forbidden) {
			t.Fatalf("resolution exact-source trigger retained forbidden category bridge %q", forbidden)
		}
	}

	admissionTrigger := sqliteObjectSQL44(
		t,
		store.conn,
		"trigger",
		"project_profile_admissions_v2_exact_sources",
	)
	for _, required := range []string{
		"JOIN profile_onboarding_work_records work_record",
		"JOIN profile_author_role_assignments assignment",
		"JOIN observed_project_bases observed_basis",
		"JOIN profile_onboarding_outcome_assessments assessment",
		"JOIN profile_onboarding_effects work_effect",
		"resolution.checked_at",
		"work_record.work_from",
		"work_record.work_until",
		"NEW.recorded_at",
		"resolution.permission_valid_until",
	} {
		if !strings.Contains(admissionTrigger, required) {
			t.Fatalf("admission exact-source trigger omitted %q", required)
		}
	}

	casTrigger := sqliteObjectSQL44(
		t,
		store.conn,
		"trigger",
		"project_profile_admissions_v2_revision_cas",
	)
	for _, required := range []string{
		"FROM project_profile_revisions legacy",
		"FROM project_profile_revisions_v2 current",
		"COUNT(DISTINCT ledger_revision)",
		"minimum_revision = 1",
		"maximum_revision = NEW.expected_ledger_revision",
	} {
		if !strings.Contains(casTrigger, required) {
			t.Fatalf("cross-generation CAS omitted %q", required)
		}
	}

	debtTrigger := sqliteObjectSQL44(
		t,
		store.conn,
		"trigger",
		"project_profile_projection_debt_v2_exact_admission",
	)
	for _, required := range []string{
		"NEW.profile_revision_generation = 'v1'",
		"NEW.profile_revision_generation = 'v2'",
		"NEW.supersedes_event_generation = 'v1'",
		"NEW.supersedes_event_generation = 'v2'",
	} {
		if !strings.Contains(debtTrigger, required) {
			t.Fatalf("tagged projection-debt trigger omitted %q", required)
		}
	}

	currentView := sqliteObjectSQL44(t, store.conn, "view", "current_project_profiles")
	for _, required := range []string{
		"SELECT 'v1'",
		"UNION ALL",
		"SELECT 'v2'",
		"MIN(ledger_revision) = 1",
		"COUNT(*) = MAX(ledger_revision)",
		"COUNT(DISTINCT ledger_revision) = COUNT(*)",
	} {
		if !strings.Contains(currentView, required) {
			t.Fatalf("current-profile union view omitted %q", required)
		}
	}

	assertForeignKeyTarget44(
		t,
		store.conn,
		profileAuthorityV2ResolutionTable,
		"authority_basis_ref",
		"profile_declaration_authority_bases_v2",
		"basis_ref",
	)
	assertForeignKeyTarget44(
		t,
		store.conn,
		projectProfileV2AdmissionTable,
		"authority_resolution_ref",
		profileAuthorityV2ResolutionTable,
		"authority_resolution_ref",
	)
	assertForeignKeyTarget44(
		t,
		store.conn,
		profileAuthorityV2UseTable,
		"committed_admission_ref",
		projectProfileV2AdmissionTable,
		"admission_id",
	)
}

func openDatabaseBeforeProfileAuthorityAdmissionV2Migration44(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "pre-v44.db")
	dsn, err := sqliteConnectionDSN(dbPath)
	if err != nil {
		t.Fatalf("build pre-v44 DSN: %v", err)
	}
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open pre-v44 database: %v", err)
	}
	if err := database.Ping(); err != nil {
		_ = database.Close()
		t.Fatalf("ping pre-v44 database: %v", err)
	}
	if _, err := database.Exec(schema); err != nil {
		_ = database.Close()
		t.Fatalf("install base schema: %v", err)
	}
	migrations := migrationsBeforeVersion(kernelMigrations, 44, 0, nil)
	if err := Migrate(database, "schema_version", migrations); err != nil {
		_ = database.Close()
		t.Fatalf("migrate pre-v44 database: %v", err)
	}
	return database
}

func seedHistoricalV1ProfileRevision44(t *testing.T, database *sql.DB) {
	t.Helper()
	insertTypedMemoryDAGFixture(t, database)
	for _, trigger := range []string{
		"authority_presentations_require_v38_basis",
		"authority_resolution_records_require_v38_basis",
	} {
		if _, err := database.Exec("DROP TRIGGER " + trigger); err != nil {
			t.Fatalf("temporarily remove %s to seed frozen v1 history: %v", trigger, err)
		}
	}
	if err := insertTypedMemoryAuthorityFixture(database, "session:test"); err != nil {
		t.Fatalf("insert historical authority fixture: %v", err)
	}
	legacyBindingTriggers := appendAuthorityBasisExactBindingTriggers(nil)
	for _, trigger := range []string{
		"authority_presentations_require_v38_basis",
		"authority_resolution_records_require_v38_basis",
	} {
		statement := migrationStatementContaining(
			t,
			legacyBindingTriggers,
			"CREATE TRIGGER IF NOT EXISTS "+trigger,
		)
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("restore %s after v1 history seed: %v", trigger, err)
		}
	}
	if _, err := database.Exec(typedMemoryAdmissionInsertSQL); err != nil {
		t.Fatalf("insert historical v1 admission: %v", err)
	}
	_, err := database.Exec(`INSERT INTO authority_uses (
		use_id, authority_resolution_ref, authority_resolution_digest,
		single_use_key, action_kind, project_root, project_binding_digest,
		envelope_digest, authority_record_ref, authority_record_digest,
		admission_request_digest, verifier_identity, verifier_version,
		committed_result_ref, committed_result_digest, consumed_at
	) VALUES (
		'authority-use:test', 'resolution:typed', 'digest:resolution',
		'use.typed', 'profile.declare.from_onboarding_candidate', '/tmp/project',
		'digest:project-binding', 'digest:envelope', 'presentation.typed',
		'digest:presentation', 'digest:admission-request', 'verifier:test', 'v1',
		'admission:test', 'digest:admission', '2026-07-14T00:40:00Z'
	)`)
	if err != nil {
		t.Fatalf("insert historical v1 authority use: %v", err)
	}
	_, err = database.Exec(`INSERT INTO project_profile_revisions (
		project_root, ledger_revision, configured_profile_kind,
		profile_payload_json, profile_payload_digest,
		receipt_json, receipt_digest, admission_id, admission_digest, recorded_at
	) VALUES (
		'/tmp/project', 1, 'Declared', '{}', 'digest:payload',
		'{}', 'digest:receipt', 'admission:test', 'digest:admission',
		'2026-07-14T00:40:00Z'
	)`)
	if err != nil {
		t.Fatalf("insert historical v1 revision: %v", err)
	}
}

func insertProjectLedgerBinding44(t *testing.T, database *sql.DB, root string) {
	t.Helper()
	boundAt := "2026-07-15T00:00:00Z"
	bindingJSON := mustAuthorityBasisJSON(t, map[string]any{
		"schema":       "haft.project-ledger-binding/v1",
		"project_id":   "qnt_a7f3b2c1",
		"project_root": root,
		"bound_at":     boundAt,
	})
	_, err := database.Exec(`INSERT INTO project_ledger_binding (
		binding_slot, project_id, project_root, binding_digest, binding_json, bound_at
	) VALUES (1, ?, ?, ?, ?, ?)`,
		"qnt_a7f3b2c1",
		root,
		authorityBasisTestDigest("a"),
		bindingJSON,
		boundAt,
	)
	if err != nil {
		t.Fatalf("insert v44 project-ledger binding: %v", err)
	}
}

func insertSyntacticV2Admission44(
	t *testing.T,
	database *sql.DB,
	expectedRevision int,
	singleUseKey string,
	suffix string,
) error {
	t.Helper()
	digest := func(character string) string {
		return authorityBasisTestDigest(character)
	}
	resolutionRef := "profile-authority-resolution:" + suffix
	basisRef := "profile-authority-basis:" + suffix
	workRef := "work-record:v2:" + suffix
	admissionRef := "profile-admission.v2." + suffix
	committedRevision := expectedRevision + 1
	recordedAt := "2026-07-15T00:45:00Z"
	payloadJSON := mustAuthorityBasisJSON(t, map[string]any{"profile": suffix})
	provenanceJSON := mustAuthorityBasisJSON(t, map[string]any{"provenance": suffix})
	receiptJSON := mustAuthorityBasisJSON(t, map[string]any{
		"schema":                             "haft.project-profile.declaration-receipt/v1",
		"authority_resolution_record_ref":    resolutionRef,
		"authority_resolution_record_digest": digest("1"),
		"authority_basis_ref":                basisRef,
		"work_record_ref":                    workRef,
		"candidate_provenance_digest":        digest("2"),
		"payload_digest":                     digest("3"),
		"observed_basis_digest":              digest("4"),
		"ledger_revision":                    committedRevision,
		"recorded_at":                        recordedAt,
	})
	var receiptValue map[string]any
	if err := decodeAuthorityBasisJSON44(receiptJSON, &receiptValue); err != nil {
		t.Fatalf("decode v44 receipt fixture: %v", err)
	}
	var payloadValue map[string]any
	if err := decodeAuthorityBasisJSON44(payloadJSON, &payloadValue); err != nil {
		t.Fatalf("decode v44 payload fixture: %v", err)
	}
	var provenanceValue map[string]any
	if err := decodeAuthorityBasisJSON44(provenanceJSON, &provenanceValue); err != nil {
		t.Fatalf("decode v44 provenance fixture: %v", err)
	}
	admissionJSON := mustAuthorityBasisJSON(t, map[string]any{
		"schema":                             "haft.project-profile.admission-record/v1",
		"admission_record_ref":               admissionRef,
		"payload":                            payloadValue,
		"candidate_provenance":               provenanceValue,
		"classification_work_record_ref":     workRef,
		"authority_basis_ref":                basisRef,
		"authority_resolution_record_ref":    resolutionRef,
		"authority_resolution_record_digest": digest("1"),
		"receipt":                            receiptValue,
		"expected_ledger_revision":           expectedRevision,
		"committed_ledger_revision":          committedRevision,
		"single_use_key":                     singleUseKey,
		"committed_at":                       recordedAt,
	})
	_, err := database.Exec(`INSERT INTO project_profile_admissions_v2 (
		admission_id, action_kind, project_root, project_binding_digest,
		profile_payload_json, candidate_provenance_json, candidate_provenance_digest,
		profile_author_role_assignment_ref, profile_author_role_assignment_digest,
		profile_payload_digest, observed_project_basis_ref, observed_project_basis_digest,
		work_record_ref, work_record_digest, outcome_assessment_ref, outcome_assessment_digest,
		authority_basis_ref, authority_basis_digest,
		authority_resolution_ref, authority_resolution_digest,
		receipt_json, receipt_digest, expected_ledger_revision, ledger_revision,
		single_use_key, admission_request_digest, admission_json, admission_digest, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		admissionRef,
		profileAuthorityV2Action,
		"/tmp/project",
		digest("5"),
		payloadJSON,
		provenanceJSON,
		digest("2"),
		"assignment:test",
		digest("6"),
		digest("3"),
		"basis:test",
		digest("4"),
		workRef,
		digest("7"),
		"assessment:v2:"+suffix,
		digest("8"),
		basisRef,
		digest("9"),
		resolutionRef,
		digest("1"),
		receiptJSON,
		digest("a"),
		expectedRevision,
		committedRevision,
		singleUseKey,
		digest("b"),
		admissionJSON,
		digest("c"),
		recordedAt,
	)
	return err
}

func decodeAuthorityBasisJSON44(raw string, destination any) error {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	return decoder.Decode(destination)
}

func assertExactColumnOrder44(
	t *testing.T,
	database *sql.DB,
	table string,
	want []string,
) {
	t.Helper()
	rows, err := database.Query("SELECT name FROM pragma_table_info(?) ORDER BY cid", table)
	if err != nil {
		t.Fatalf("inspect %s columns: %v", table, err)
	}
	defer rows.Close()
	got := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan %s column: %v", table, err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s columns: %v", table, err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("%s columns = %#v, want %#v", table, got, want)
	}
}

func sqliteObjectSQL44(
	t *testing.T,
	database *sql.DB,
	kind string,
	name string,
) string {
	t.Helper()
	var statement string
	err := database.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type = ? AND name = ?",
		kind,
		name,
	).Scan(&statement)
	if err != nil {
		t.Fatalf("read %s %s SQL: %v", kind, name, err)
	}
	return statement
}

func assertForeignKeyTarget44(
	t *testing.T,
	database *sql.DB,
	table string,
	from string,
	targetTable string,
	targetColumn string,
) {
	t.Helper()
	rows, err := database.Query("PRAGMA foreign_key_list(" + quoteSQLiteIdentifier(table) + ")")
	if err != nil {
		t.Fatalf("inspect %s foreign keys: %v", table, err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var id int
		var sequence int
		var referencedTable string
		var sourceColumn string
		var referencedColumn string
		var onUpdate string
		var onDelete string
		var match string
		err := rows.Scan(
			&id,
			&sequence,
			&referencedTable,
			&sourceColumn,
			&referencedColumn,
			&onUpdate,
			&onDelete,
			&match,
		)
		if err != nil {
			t.Fatalf("scan %s foreign key: %v", table, err)
		}
		if sourceColumn == from && referencedTable == targetTable && referencedColumn == targetColumn {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s foreign keys: %v", table, err)
	}
	if !found {
		t.Fatalf("%s.%s does not reference %s.%s", table, from, targetTable, targetColumn)
	}
}
