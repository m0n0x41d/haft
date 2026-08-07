package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func newStoreBeforeAuthoritySourceMigration(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "pre-v38.db")
	dsn, err := sqliteConnectionDSN(dbPath)
	if err != nil {
		t.Fatalf("build pre-v38 SQLite DSN: %v", err)
	}
	connection, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open pre-v38 SQLite store: %v", err)
	}
	if err := connection.Ping(); err != nil {
		_ = connection.Close()
		t.Fatalf("ping pre-v38 SQLite store: %v", err)
	}
	if _, err := connection.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = connection.Close()
		t.Fatalf("enable pre-v38 WAL mode: %v", err)
	}
	if _, err := connection.Exec(schema); err != nil {
		_ = connection.Close()
		t.Fatalf("install base schema for pre-v38 store: %v", err)
	}
	migrations := migrationsBeforeVersion(kernelMigrations, 38, 0, nil)
	if err := Migrate(connection, "schema_version", migrations); err != nil {
		_ = connection.Close()
		t.Fatalf("migrate pre-v38 store: %v", err)
	}
	return &Store{conn: connection, q: New()}
}

func TestRunMigrations_FreshDatabase(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// Check schema_version table exists and has entries
	var count int
	err = store.conn.QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query schema_version: %v", err)
	}
	if count != len(kernelMigrations) {
		t.Errorf("Expected %d kernelMigrations recorded, got %d", len(kernelMigrations), count)
	}
	for _, table := range []string{
		"artifact_symbol_bindings",
		"symbol_rebind_history",
		"authority_presentations",
		"authority_resolution_records",
		"authority_uses",
		"profile_onboarding_method_descriptions",
		"profile_onboarding_method_contracts",
		"profile_onboarding_executor_system_admissions",
		"profile_author_role_admissions",
		"profile_author_assignment_support_carriers",
		"profile_author_role_assignments",
		"observed_project_bases",
		"profile_onboarding_work_records",
		"profile_onboarding_effects",
		"profile_onboarding_outcome_assessments",
		"project_profile_admissions",
		"project_profile_revisions",
		"project_profile_projection_debt",
		"migration_review_speech_acts",
		"migration_review_admissions",
		"project_ledger_binding",
	} {
		var name string
		err := store.conn.QueryRow(
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
			table,
		).Scan(&name)
		if err != nil || name != table {
			t.Fatalf("migration missing table %s: name=%q err=%v", table, name, err)
		}
	}
	for _, forbidden := range []string{
		"profile_declaration_candidates",
		"profile_onboarding_candidates",
		"typed_memory_type_registry",
	} {
		var count int
		err := store.conn.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
			forbidden,
		).Scan(&count)
		if err != nil {
			t.Fatalf("inspect forbidden table %s: %v", forbidden, err)
		}
		if count != 0 {
			t.Fatalf("migration created forbidden generic/value-only table %s", forbidden)
		}
	}
	for _, trigger := range []string{
		"authority_presentations_no_replace",
		"authority_presentations_exact_assignment_method",
		"authority_presentations_no_update",
		"authority_presentations_no_delete",
		"authority_resolution_records_no_replace",
		"authority_resolution_records_exact_presentation",
		"authority_resolution_records_no_update",
		"authority_resolution_records_no_delete",
		"authority_uses_no_replace",
		"authority_uses_exact_tuple",
		"authority_uses_no_update",
		"authority_uses_no_delete",
		"profile_onboarding_method_descriptions_no_replace",
		"profile_onboarding_method_descriptions_no_update",
		"profile_onboarding_method_descriptions_no_delete",
		"profile_onboarding_method_contracts_no_replace",
		"profile_onboarding_method_contracts_exact_description",
		"profile_onboarding_method_contracts_no_update",
		"profile_onboarding_method_contracts_no_delete",
		"profile_onboarding_executor_system_admissions_no_replace",
		"profile_onboarding_executor_system_admissions_exact_method",
		"profile_onboarding_executor_system_admissions_no_update",
		"profile_onboarding_executor_system_admissions_no_delete",
		"profile_author_role_admissions_no_replace",
		"profile_author_role_admissions_exact_method",
		"profile_author_role_admissions_no_update",
		"profile_author_role_admissions_no_delete",
		"profile_author_assignment_support_carriers_no_replace",
		"profile_author_assignment_support_carriers_exact_admissions",
		"profile_author_assignment_support_carriers_no_update",
		"profile_author_assignment_support_carriers_no_delete",
		"profile_author_role_assignments_no_replace",
		"profile_author_role_assignments_exact_support",
		"profile_author_role_assignments_no_update",
		"profile_author_role_assignments_no_delete",
		"observed_project_bases_no_replace",
		"observed_project_bases_no_update",
		"observed_project_bases_no_delete",
		"profile_onboarding_work_records_no_replace",
		"profile_onboarding_work_records_exact_support",
		"profile_onboarding_work_records_no_update",
		"profile_onboarding_work_records_no_delete",
		"profile_onboarding_effects_no_replace",
		"profile_onboarding_effects_exact_work",
		"profile_onboarding_effects_no_update",
		"profile_onboarding_effects_no_delete",
		"profile_onboarding_outcome_assessments_no_replace",
		"profile_onboarding_outcome_assessments_exact_effect",
		"profile_onboarding_outcome_assessments_no_update",
		"profile_onboarding_outcome_assessments_no_delete",
		"project_profile_admissions_no_replace",
		"project_profile_admissions_revision_cas",
		"project_profile_admissions_exact_authority",
		"project_profile_admissions_no_update",
		"project_profile_admissions_no_delete",
		"project_profile_revisions_no_replace",
		"project_profile_revisions_exact_admission",
		"project_profile_revisions_no_update",
		"project_profile_revisions_no_delete",
		"project_profile_projection_debt_no_replace",
		"project_profile_projection_debt_no_update",
		"project_profile_projection_debt_no_delete",
		"migration_review_speech_acts_no_replace",
		"migration_review_speech_acts_no_update",
		"migration_review_speech_acts_no_delete",
		"migration_review_admissions_no_replace",
		"migration_review_admissions_exact_speech_act",
		"migration_review_admissions_exact_json_bindings",
		"migration_review_admissions_no_update",
		"migration_review_admissions_no_delete",
		"project_ledger_binding_no_replace",
		"project_ledger_binding_no_update",
		"project_ledger_binding_no_delete",
		"migration_review_speech_acts_project_ledger_root",
		"migration_review_admissions_project_ledger_root",
		"authority_presentations_project_ledger_root",
		"authority_uses_project_ledger_root",
		"observed_project_bases_project_ledger_root",
		"profile_onboarding_work_records_project_ledger_root",
		"project_profile_admissions_project_ledger_root",
		"project_profile_revisions_project_ledger_root",
		"project_profile_projection_debt_project_ledger_root",
	} {
		var name string
		err := store.conn.QueryRow(
			`SELECT name FROM sqlite_master WHERE type = 'trigger' AND name = ?`,
			trigger,
		).Scan(&name)
		if err != nil || name != trigger {
			t.Fatalf("migration missing trigger %s: name=%q err=%v", trigger, name, err)
		}
	}
}

func TestRunMigrations_ProjectLedgerBindingRejectsWeakIDAndForeignRoot(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()
	digest := "sha256:" + strings.Repeat("a", 64)
	boundAt := "2026-07-15T10:00:00Z"
	insertBinding := `INSERT INTO project_ledger_binding (
		binding_slot, project_id, project_root, binding_digest, binding_json, bound_at
	) VALUES (1, ?, ?, ?, ?, ?)`
	weakJSON := `{"schema":"haft.project-ledger-binding/v1","project_id":"qnt_test","project_root":"/tmp/project","bound_at":"2026-07-15T10:00:00Z"}`
	if _, err := store.conn.Exec(insertBinding, "qnt_test", "/tmp/project", digest, weakJSON, boundAt); err == nil {
		t.Fatal("project ledger binding accepted non-canonical project ID")
	}
	validJSON := `{"schema":"haft.project-ledger-binding/v1","project_id":"qnt_a7f3b2c1","project_root":"/tmp/project","bound_at":"2026-07-15T10:00:00Z"}`
	if _, err := store.conn.Exec(insertBinding, "qnt_a7f3b2c1", "/tmp/project", digest, validJSON, boundAt); err != nil {
		t.Fatalf("insert exact project ledger binding: %v", err)
	}
	packetDigest := "sha256:" + strings.Repeat("b", 64)
	carrierDigest := "sha256:" + strings.Repeat("c", 64)
	auditDigest := "sha256:" + strings.Repeat("d", 64)
	speechDigest := "sha256:" + strings.Repeat("e", 64)
	speechJSON := `{"schema":"haft.spec-migration-v2.semantic-review-speech-act/v1","speech_act_ref":"speech-act:foreign","project_root":"/tmp/foreign","packet_digest":"` + packetDigest + `","packet_carrier_digest":"` + carrierDigest + `","partition_audit_schema":"haft.spec-migration-v2.packet-partition-audit/v1","partition_audit_status":"verified","partition_audit_digest":"` + auditDigest + `","reviewer_role_ref":"role:operator","judgement_context_ref":"context:migration-review","session_ref":"session:test","canonical_utterance":"ACCEPT ` + carrierDigest + `","occurred_at":"2026-07-15T10:00:00Z","valid_from":"2026-07-15T09:55:00Z","valid_until":"2026-07-15T10:05:00Z"}`
	_, err = store.conn.Exec(
		`INSERT INTO migration_review_speech_acts (
			speech_act_ref, speech_act_digest, project_root,
			packet_digest, packet_carrier_digest,
			partition_audit_schema, partition_audit_status, partition_audit_digest,
			reviewer_role_ref, judgement_context_ref, session_ref,
			canonical_utterance, occurred_at, valid_from, valid_until,
			speech_act_json, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"speech-act:foreign", speechDigest, "/tmp/foreign",
		packetDigest, carrierDigest,
		"haft.spec-migration-v2.packet-partition-audit/v1", "verified", auditDigest,
		"role:operator", "context:migration-review", "session:test",
		"ACCEPT "+carrierDigest,
		"2026-07-15T10:00:00Z", "2026-07-15T09:55:00Z", "2026-07-15T10:05:00Z",
		speechJSON, "2026-07-15T10:00:01Z",
	)
	if err == nil || !strings.Contains(err.Error(), "bound project ledger root") {
		t.Fatalf("foreign-root semantic-review SpeechAct was not rejected: %v", err)
	}
}

func TestRunMigrations_SemanticReviewLedgerIsDistinctAppendOnlyAndExact(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	digest := func(character string) string {
		return "sha256:" + strings.Repeat(character, 64)
	}
	packetDigest := digest("a")
	carrierDigest := digest("b")
	speechDigest := digest("c")
	admissionDigest := digest("d")
	auditSchema := "haft.spec-migration-v2.packet-partition-audit/v1"
	auditStatus := "verified"
	auditDigest := digest("7")
	speechJSON := `{"schema":"haft.spec-migration-v2.semantic-review-speech-act/v1","speech_act_ref":"speech-act:test","project_root":"/tmp/project","packet_digest":"` + packetDigest + `","packet_carrier_digest":"` + carrierDigest + `","partition_audit_schema":"` + auditSchema + `","partition_audit_status":"` + auditStatus + `","partition_audit_digest":"` + auditDigest + `","reviewer_role_ref":"role:operator","judgement_context_ref":"context:migration-review","session_ref":"session:test","canonical_utterance":"ACCEPT ` + carrierDigest + `","occurred_at":"2026-07-15T10:00:00Z","valid_from":"2026-07-15T09:55:00Z","valid_until":"2026-07-15T10:05:00Z"}`
	_, err = store.conn.Exec(
		`INSERT INTO migration_review_speech_acts (
			speech_act_ref, speech_act_digest, project_root,
			packet_digest, packet_carrier_digest,
			partition_audit_schema, partition_audit_status, partition_audit_digest,
			reviewer_role_ref, judgement_context_ref, session_ref,
			canonical_utterance, occurred_at, valid_from, valid_until,
			speech_act_json, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"speech-act:test", speechDigest, "/tmp/project",
		packetDigest, carrierDigest,
		auditSchema, auditStatus, auditDigest,
		"role:operator", "context:migration-review", "session:test",
		"ACCEPT "+carrierDigest,
		"2026-07-15T10:00:00Z", "2026-07-15T09:55:00Z", "2026-07-15T10:05:00Z",
		speechJSON, "2026-07-15T10:00:01Z",
	)
	if err != nil {
		t.Fatalf("insert semantic-review SpeechAct: %v", err)
	}

	carrierBindings := `[{"role":"software_system","carrier":".context/software.md","digest":"` + digest("e") + `"},{"role":"target_system","carrier":".context/target.md","digest":"` + digest("f") + `"},{"role":"term_map","carrier":".context/terms.md","digest":"` + digest("1") + `"}]`
	lifecycleIntent := `[{"section_ref":"SS.role.001","operation":"activate"}]`
	admissionJSON := `{"schema":"haft.spec-migration-v2.semantic-review-admission/v1","admission_ref":"review-admission:test","speech_act_ref":"speech-act:test","speech_act_digest":"` + speechDigest + `","project_root":"/tmp/project","packet_digest":"` + packetDigest + `","packet_carrier_digest":"` + carrierDigest + `","partition_audit_schema":"` + auditSchema + `","partition_audit_status":"` + auditStatus + `","partition_audit_digest":"` + auditDigest + `","source_carrier":".haft/specs/enabling-system.md","source_digest":"` + digest("2") + `","target_carrier_digests":` + carrierBindings + `,"fpf_revision":"` + strings.Repeat("3", 40) + `","semantic_zero_pass_carrier":".context/semantic.md","semantic_zero_pass_digest":"` + digest("4") + `","lifecycle_intent":` + lifecycleIntent + `,"admitted_at":"2026-07-15T10:00:01Z"}`
	insertAdmission := `INSERT INTO migration_review_admissions (
		admission_ref, admission_digest, project_root,
		packet_digest, packet_carrier_digest,
		partition_audit_schema, partition_audit_status, partition_audit_digest,
		source_carrier, source_digest, target_carrier_digests_json,
		fpf_revision, semantic_zero_pass_carrier, semantic_zero_pass_digest,
		lifecycle_intent_json, speech_act_ref, speech_act_digest,
		admission_json, admitted_at, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = store.conn.Exec(
		insertAdmission,
		"review-admission:test", admissionDigest, "/tmp/project",
		packetDigest, carrierDigest,
		auditSchema, auditStatus, auditDigest,
		".haft/specs/enabling-system.md", digest("2"), carrierBindings,
		strings.Repeat("3", 40), ".context/semantic.md", digest("4"),
		lifecycleIntent, "speech-act:test", speechDigest,
		admissionJSON, "2026-07-15T10:00:01Z", "2026-07-15T10:00:01Z",
	)
	if err != nil {
		t.Fatalf("insert semantic-review admission: %v", err)
	}

	for _, table := range []string{
		"migration_review_speech_acts",
		"migration_review_admissions",
	} {
		if _, err := store.conn.Exec("UPDATE " + table + " SET recorded_at = recorded_at"); err == nil {
			t.Fatalf("append-only table %s accepted UPDATE", table)
		}
		if _, err := store.conn.Exec("DELETE FROM " + table); err == nil {
			t.Fatalf("append-only table %s accepted DELETE", table)
		}
		if _, err := store.conn.Exec("INSERT OR REPLACE INTO " + table + " SELECT * FROM " + table); err == nil {
			t.Fatalf("append-only table %s accepted INSERT OR REPLACE", table)
		}
	}

	badAdmissionJSON := strings.Replace(admissionJSON, "speech-act:test", "speech-act:missing", 1)
	_, err = store.conn.Exec(
		insertAdmission,
		"review-admission:bad", digest("5"), "/tmp/project",
		packetDigest, digest("6"),
		auditSchema, auditStatus, auditDigest,
		".haft/specs/enabling-system.md", digest("2"), carrierBindings,
		strings.Repeat("3", 40), ".context/semantic.md", digest("4"),
		lifecycleIntent, "speech-act:missing", speechDigest,
		badAdmissionJSON, "2026-07-15T10:00:01Z", "2026-07-15T10:00:01Z",
	)
	if err == nil || !strings.Contains(err.Error(), "exact human SpeechAct") {
		t.Fatalf("mismatched admission SpeechAct was not rejected exactly: %v", err)
	}
}

func TestRunMigrations_AuthorityLedgerIsAppendOnly(t *testing.T) {
	t.Parallel()

	store := newStoreBeforeAuthoritySourceMigration(t)
	defer store.Close()
	insertTypedMemoryDAGFixture(t, store.conn)

	insertPresentation := `INSERT INTO authority_presentations (
		presentation_id, speech_act_ref, speech_act_digest,
		authorization_content_ref, authorization_content_digest,
		permission_ref, permission_digest, permission_modality,
		permission_source_speech_act_ref,
		permission_subject_role_assignment_ref,
		permission_authorization_content_ref,
		permission_action_kind, permission_project_root,
		permission_method_description_ref,
		permission_valid_from, permission_valid_until,
		permission_single_use_key, permission_profile_admission_predicate_ref,
		permission_context_policy_ref,
		context_policy_ref,
		context_policy_digest, action_kind, project_root,
		profile_author_role_assignment_ref, profile_author_role_assignment_digest,
		method_description_ref, method_description_digest,
		method_contract_ref, method_contract_digest,
		classifier_version, policy_version, session_ref,
		allowed_work_from, allowed_work_until,
		basis_observation_from, basis_observation_until,
		valid_from, valid_until, single_use_key, project_binding_digest,
		envelope_digest,
		presentation_digest, recorded_at
	) VALUES (
		'presentation.test', 'work:speech', 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
		'content:test', 'sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
		'permission:test', 'sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc', 'MAY',
		'work:speech', 'assignment:test', 'content:test',
		'profile.declare', '/tmp/project', 'method-description:onboard',
		'2026-07-14T00:10:00Z', '2026-07-14T00:50:00Z', 'use.test',
		'predicate:profile-declaration-admission:v1', 'policy:test',
		'policy:test',
		'sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd',
		'profile.declare', '/tmp/project',
		'assignment:test', 'digest:assignment',
		'method-description:onboard', 'digest:method-description',
		'contract:onboard', 'digest:contract',
		'classifier:v1', 'policy:v1', 'session:test',
		'2026-07-14T00:20:00Z', '2026-07-14T00:30:00Z',
		'2026-07-14T00:15:00Z', '2026-07-14T00:20:00Z',
		'2026-07-14T00:10:00Z', '2026-07-14T00:50:00Z', 'use.test',
		'sha256:1111111111111111111111111111111111111111111111111111111111111111',
		'sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee',
		'sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff',
		'2026-07-14T00:00:00Z'
	)`
	_, err := store.conn.Exec(insertPresentation)
	if err != nil {
		t.Fatalf("insert authority presentation: %v", err)
	}

	if _, err := store.conn.Exec(`UPDATE authority_presentations SET action_kind = 'changed' WHERE presentation_id = 'presentation.test'`); err == nil {
		t.Fatal("append-only authority presentation accepted UPDATE")
	}
	if _, err := store.conn.Exec(`DELETE FROM authority_presentations WHERE presentation_id = 'presentation.test'`); err == nil {
		t.Fatal("append-only authority presentation accepted DELETE")
	}
	replacePresentation := strings.Replace(insertPresentation, "INSERT INTO", "INSERT OR REPLACE INTO", 1)
	replacePresentation = strings.Replace(replacePresentation, "'profile.declare'", "'profile.changed'", 1)
	if _, err := store.conn.Exec(replacePresentation); err == nil {
		t.Fatal("append-only authority presentation accepted INSERT OR REPLACE")
	}
	var action string
	if err := store.conn.QueryRow(
		"SELECT action_kind FROM authority_presentations WHERE presentation_id = 'presentation.test'",
	).Scan(&action); err != nil {
		t.Fatalf("reload presentation after rejected replace: %v", err)
	}
	if action != "profile.declare" {
		t.Fatalf("rejected replace changed immutable presentation: action=%q", action)
	}
}

func TestRunMigrations_ProfileOnboardingWorkHasOneCanonicalDigest(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	rows, err := store.conn.Query(`PRAGMA table_info(profile_onboarding_work_records)`)
	if err != nil {
		t.Fatalf("inspect profile-onboarding Work schema: %v", err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var columnID int
		var name string
		var dataType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(
			&columnID,
			&name,
			&dataType,
			&notNull,
			&defaultValue,
			&primaryKey,
		); err != nil {
			t.Fatalf("read profile-onboarding Work schema: %v", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate profile-onboarding Work schema: %v", err)
	}

	for _, required := range []string{
		"method_description_ref",
		"method_description_digest",
		"method_contract_ref",
		"method_contract_digest",
		"parameter_bindings_json",
		"performed_by_role_assignment_ref",
		"profile_author_role_assignment_ref",
		"profile_author_role_assignment_digest",
		"observed_project_basis_ref",
		"observed_project_basis_digest",
		"inputs_json",
		"outputs_json",
		"resources_json",
		"affected_refs_json",
		"work_record_json",
		"work_record_digest",
	} {
		if !columns[required] {
			t.Fatalf("profile-onboarding Work schema is missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"role_assignment_from",
		"role_assignment_until",
		"role_assignment_coverage_json",
		"role_assignment_coverage_digest",
		"parameter_bindings_digest",
		"inputs_digest",
		"outputs_digest",
		"resources_digest",
		"affected_refs_digest",
	} {
		if columns[forbidden] {
			t.Fatalf("denormalized projection %q must not define a competing digest", forbidden)
		}
	}
}

func TestRunMigrations_ProfileAdmissionBindsExactSemanticSupport(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	rows, err := store.conn.Query(`PRAGMA table_info(project_profile_admissions)`)
	if err != nil {
		t.Fatalf("inspect project-profile admission schema: %v", err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var columnID int
		var name string
		var dataType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(
			&columnID,
			&name,
			&dataType,
			&notNull,
			&defaultValue,
			&primaryKey,
		); err != nil {
			t.Fatalf("read project-profile admission schema: %v", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate project-profile admission schema: %v", err)
	}

	for _, required := range []string{
		"profile_author_role_assignment_ref",
		"profile_author_role_assignment_digest",
		"observed_project_basis_ref",
		"observed_project_basis_digest",
		"outcome_assessment_ref",
		"outcome_assessment_digest",
	} {
		if !columns[required] {
			t.Fatalf("project-profile admission schema is missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"role_assignment_coverage_ref",
		"role_assignment_coverage_digest",
	} {
		if columns[forbidden] {
			t.Fatalf("project-profile admission retains legacy coverage surrogate %q", forbidden)
		}
	}
}

func TestRunMigrations_TypedMemoryValueTablesAreAppendOnly(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	tables := insertTypedMemoryDAGFixture(t, store.conn)
	var foreignKeyTable string
	err = store.conn.QueryRow(`PRAGMA foreign_key_check`).Scan(&foreignKeyTable)
	if err != sql.ErrNoRows {
		t.Fatalf("typed-memory fixture violates a foreign key: table=%q err=%v", foreignKeyTable, err)
	}
	for _, table := range tables {
		updateSQL := "UPDATE " + table.name + " SET recorded_at = recorded_at WHERE " + table.primaryKey + " = ?"
		if _, err := store.conn.Exec(updateSQL, table.primaryValue); err == nil {
			t.Fatalf("append-only table %s accepted UPDATE", table.name)
		}
		deleteSQL := "DELETE FROM " + table.name + " WHERE " + table.primaryKey + " = ?"
		if _, err := store.conn.Exec(deleteSQL, table.primaryValue); err == nil {
			t.Fatalf("append-only table %s accepted DELETE", table.name)
		}
		replaceSQL := "INSERT OR REPLACE INTO " + table.name + " SELECT * FROM " + table.name + " WHERE " + table.primaryKey + " = ?"
		if _, err := store.conn.Exec(replaceSQL, table.primaryValue); err == nil {
			t.Fatalf("append-only table %s accepted INSERT OR REPLACE", table.name)
		}
	}
}

func TestRunMigrations_TypedMemoryRejectsMismatchedDigestPairs(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	insertTypedMemoryDAGFixture(t, store.conn)
	badContract := `INSERT INTO profile_onboarding_method_contracts (
		method_contract_ref, edition, method_description_ref,
		method_description_digest, bounded_context_ref,
		role_admission_policy_ref, system_admission_policy_ref,
		parameter_spec_set_digest, accepted_result_kinds_json,
		required_occurrence_slots_json, occurrence_coverage_rule_refs_json,
		effect_state_witness_rule_ref, acceptance_standard_ref,
		acceptance_standard_edition, holder_equals_executed_within_rule_ref,
		method_contract_json, method_contract_digest, recorded_at
	) VALUES (
		'contract:mismatch', 'v1', 'method-description:onboard',
		'digest:wrong-description', 'context:onboard',
		'policy:role', 'policy:system', 'digest:parameters',
		'["CandidatePayloadProduced"]', '["work_interval"]', '["rule:coverage"]',
		'rule:effect', 'acceptance:onboard', 'v1', 'rule:holder',
		'{}', 'digest:contract-mismatch', '2026-07-14T00:00:00Z'
	)`
	if _, err := store.conn.Exec(badContract); err == nil || !strings.Contains(err.Error(), "exact MethodDescription") {
		t.Fatalf("mismatched MethodDescription digest was not rejected exactly: %v", err)
	}

	badAssignment := `INSERT INTO profile_author_role_assignments (
		role_assignment_ref, holder_system_ref, admitted_role_ref,
		bounded_context_ref, valid_from, valid_until,
		system_admission_ref, system_admission_digest,
		role_admission_ref, role_admission_digest,
		assignment_justification_ref, assignment_justification_digest,
		assignment_provenance_ref, assignment_provenance_digest,
		role_assignment_json, role_assignment_digest, recorded_at
	) VALUES (
		'assignment:mismatch', 'system:haft', 'role:profile-author',
		'context:onboard', '2026-07-14T00:10:00Z', '2026-07-14T00:50:00Z',
		'system-admission:test', 'digest:wrong-system-admission',
		'role-admission:test', 'digest:role-admission',
		'justification:test', 'digest:justification',
		'provenance:test', 'digest:provenance',
		'{}', 'digest:assignment-mismatch', '2026-07-14T00:10:00Z'
	)`
	if _, err := store.conn.Exec(badAssignment); err == nil || !strings.Contains(err.Error(), "exact admission and provenance support") {
		t.Fatalf("mismatched RoleAssignment support digest was not rejected exactly: %v", err)
	}

	badEffect := `INSERT INTO profile_onboarding_effects (
		effect_ref, work_record_ref, work_ref, work_record_digest,
		result_kind, output_ref, profile_payload_digest,
		observed_project_basis_ref, observed_project_basis_digest,
		missing_basis_digest, affected_entity_refs_json, state_plane_ref,
		pre_state_ref, post_state_ref, delta_predicate_ref,
		evidence_provenance_path_refs_json, effect_json, effect_digest, recorded_at
	) VALUES (
		'effect:mismatch', 'work-record:test', 'work:test', 'digest:wrong-work',
		'CandidatePayloadProduced', 'output:test', 'digest:payload',
		'basis:test', 'digest:basis', '', '["entity:test"]', 'state:profile',
		'state:before', 'state:after', '', '[]', '{}', 'digest:effect-mismatch',
		'2026-07-14T00:31:00Z'
	)`
	if _, err := store.conn.Exec(badEffect); err == nil || !strings.Contains(err.Error(), "exact Work result") {
		t.Fatalf("mismatched Effect Work digest was not rejected exactly: %v", err)
	}
}

func TestRunMigrations_ExecutorAdmissionUsesClosedIdentityAndSessionWindow(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	columns := tableColumns(t, store.conn, "profile_onboarding_executor_system_admissions")
	for _, required := range []string{
		"identity_basis_kind",
		"identity_basis_system_ref",
		"identity_basis_kernel_identity",
		"identity_basis_kernel_version",
		"identity_basis_designation_ref",
		"identity_basis_designation_digest",
		"session_ref",
		"valid_from",
		"valid_until",
	} {
		if !columns[required] {
			t.Fatalf("executor-system admission schema is missing %q", required)
		}
	}
	for _, forbidden := range []string{"identity_basis_ref", "identity_basis_digest"} {
		if columns[forbidden] {
			t.Fatalf("executor-system admission retains open identity surrogate %q", forbidden)
		}
	}
}

func TestRunMigrations_AuthorityPersistenceCarriesExactAssignmentAndMethodTuple(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	required := []string{
		"profile_author_role_assignment_ref",
		"profile_author_role_assignment_digest",
		"method_description_ref",
		"method_description_digest",
		"method_contract_ref",
		"method_contract_digest",
	}
	for _, table := range []string{"authority_presentations", "authority_resolution_records"} {
		columns := tableColumns(t, store.conn, table)
		for _, column := range required {
			if !columns[column] {
				t.Fatalf("%s schema is missing exact authority field %q", table, column)
			}
		}
	}
}

func TestRunMigrations_ProfileAdmissionRequiresExactTypedMemoryAndAuthoritySession(t *testing.T) {
	t.Parallel()

	t.Run("exact tuple is admitted", func(t *testing.T) {
		store := newStoreBeforeAuthoritySourceMigration(t)
		defer store.Close()

		insertTypedMemoryDAGFixture(t, store.conn)
		if err := insertTypedMemoryAuthorityFixture(store.conn, "session:test"); err != nil {
			t.Fatalf("insert typed-memory authority fixture: %v", err)
		}
		if _, err := store.conn.Exec(typedMemoryAdmissionInsertSQL); err != nil {
			t.Fatalf("exact typed-memory admission was rejected: %v", err)
		}
	})

	t.Run("authority session mismatch is rejected", func(t *testing.T) {
		store := newStoreBeforeAuthoritySourceMigration(t)
		defer store.Close()

		insertTypedMemoryDAGFixture(t, store.conn)
		err := insertTypedMemoryAuthorityFixture(store.conn, "session:wrong")
		if err == nil || !strings.Contains(err.Error(), "exact pre-existing assignment") {
			t.Fatalf("authority/session mismatch was not rejected at presentation admission: %v", err)
		}
	})
}

func TestRunMigrations_AuthorityPresentationAndResolutionBindExactAssignmentAndMethod(t *testing.T) {
	t.Parallel()

	t.Run("presentation rejects assignment digest mismatch", func(t *testing.T) {
		store := newStoreBeforeAuthoritySourceMigration(t)
		defer store.Close()
		insertTypedMemoryDAGFixture(t, store.conn)

		err := insertTypedMemoryAuthorityPresentationFixture(
			store.conn,
			"session:test",
			"digest:wrong-assignment",
			"digest:method-description",
			"digest:contract",
		)
		if err == nil || !strings.Contains(err.Error(), "exact pre-existing assignment") {
			t.Fatalf("presentation accepted another assignment digest: %v", err)
		}
	})

	t.Run("presentation rejects method contract digest mismatch", func(t *testing.T) {
		store := newStoreBeforeAuthoritySourceMigration(t)
		defer store.Close()
		insertTypedMemoryDAGFixture(t, store.conn)

		err := insertTypedMemoryAuthorityPresentationFixture(
			store.conn,
			"session:test",
			"digest:assignment",
			"digest:method-description",
			"digest:wrong-contract",
		)
		if err == nil || !strings.Contains(err.Error(), "exact pre-existing assignment") {
			t.Fatalf("presentation accepted another MethodContract digest: %v", err)
		}
	})

	t.Run("resolution rejects presentation tuple mismatch", func(t *testing.T) {
		store := newStoreBeforeAuthoritySourceMigration(t)
		defer store.Close()
		insertTypedMemoryDAGFixture(t, store.conn)
		err := insertTypedMemoryAuthorityPresentationFixture(
			store.conn,
			"session:test",
			"digest:assignment",
			"digest:method-description",
			"digest:contract",
		)
		if err != nil {
			t.Fatalf("insert exact authority presentation: %v", err)
		}

		err = insertTypedMemoryAuthorityResolutionFixture(
			store.conn,
			"digest:assignment",
			"digest:wrong-method-description",
			"digest:contract",
		)
		if err == nil || !strings.Contains(err.Error(), "exact presentation assignment") {
			t.Fatalf("resolution accepted another method tuple: %v", err)
		}
	})
}

func TestRunMigrations_ProfileAdmissionRevisionRejectsMaxInt64(t *testing.T) {
	t.Parallel()

	store := newStoreBeforeAuthoritySourceMigration(t)
	defer store.Close()

	var tableSQL string
	err := store.conn.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'project_profile_admissions'`,
	).Scan(&tableSQL)
	if err != nil {
		t.Fatalf("read project-profile admission schema: %v", err)
	}
	if !strings.Contains(tableSQL, "expected_ledger_revision < 9223372036854775807") {
		t.Fatalf("project-profile admission schema has no signed MaxInt64 guard: %s", tableSQL)
	}
	insertTypedMemoryDAGFixture(t, store.conn)
	if err := insertTypedMemoryAuthorityFixture(store.conn, "session:test"); err != nil {
		t.Fatalf("insert typed-memory authority fixture: %v", err)
	}
	if _, err := store.conn.Exec(`DROP TRIGGER project_profile_admissions_revision_cas`); err != nil {
		t.Fatalf("isolate signed revision-domain constraint: %v", err)
	}
	maxRevisionAdmission := strings.Replace(
		typedMemoryAdmissionInsertSQL,
		"\n\t0, 1, 'use.typed'",
		"\n\t9223372036854775807, 9223372036854775807, 'use.typed'",
		1,
	)
	if _, err := store.conn.Exec(maxRevisionAdmission); err == nil {
		t.Fatal("project-profile admission accepted an unadvanceable MaxInt64 expected revision")
	}
}

type typedMemoryTableFixture struct {
	name         string
	primaryKey   string
	primaryValue string
}

const typedMemoryAdmissionInsertSQL = `INSERT INTO project_profile_admissions (
	admission_id, action_kind, project_root, project_binding_digest,
	profile_payload_json, candidate_provenance_json, candidate_provenance_digest,
	profile_author_role_assignment_ref, profile_author_role_assignment_digest,
	profile_payload_digest, observed_project_basis_ref,
	observed_project_basis_digest, work_record_ref, work_record_digest,
	outcome_assessment_ref, outcome_assessment_digest,
	authority_basis_ref, authority_basis_digest,
	authority_resolution_ref, authority_resolution_digest,
	receipt_json, receipt_digest, expected_ledger_revision, ledger_revision,
	single_use_key, admission_request_digest, admission_json,
	admission_digest, recorded_at
) VALUES (
	'admission:test', 'profile.declare.from_onboarding_candidate',
	'/tmp/project', 'digest:project-binding', '{}',
	'{"authority_basis_ref":"presentation.typed","work_record_ref":"work-record:test","work_record_digest":"digest:work","profile_author_role_assignment_ref":"assignment:test","profile_author_role_assignment_digest":"digest:assignment","observed_project_basis_ref":"basis:test","observed_project_basis_digest":"digest:basis","outcome_assessment_ref":"assessment:test","outcome_assessment_digest":"digest:assessment","project_root":"/tmp/project","classifier_version":"classifier:v1","policy_version":"policy:v1","session_ref":"session:test","payload_digest":"digest:payload","provenance_digest":"digest:candidate-provenance"}',
	'digest:candidate-provenance', 'assignment:test', 'digest:assignment',
	'digest:payload', 'basis:test', 'digest:basis',
	'work-record:test', 'digest:work', 'assessment:test', 'digest:assessment',
	'presentation.typed', 'digest:presentation',
	'resolution:typed', 'digest:resolution', '{}', 'digest:receipt',
	0, 1, 'use.typed', 'digest:admission-request', '{}',
	'digest:admission', '2026-07-14T00:40:00Z'
)`

func insertTypedMemoryAuthorityFixture(conn *sql.DB, sessionRef string) error {
	err := insertTypedMemoryAuthorityPresentationFixture(
		conn,
		sessionRef,
		"digest:assignment",
		"digest:method-description",
		"digest:contract",
	)
	if err != nil {
		return err
	}
	return insertTypedMemoryAuthorityResolutionFixture(
		conn,
		"digest:assignment",
		"digest:method-description",
		"digest:contract",
	)
}

func insertTypedMemoryAuthorityPresentationFixture(
	conn *sql.DB,
	sessionRef string,
	assignmentDigest string,
	methodDescriptionDigest string,
	methodContractDigest string,
) error {
	presentationSQL := `INSERT INTO authority_presentations (
		presentation_id, speech_act_ref, speech_act_digest,
		authorization_content_ref, authorization_content_digest,
		permission_ref, permission_digest, permission_modality,
		permission_source_speech_act_ref,
		permission_subject_role_assignment_ref,
		permission_authorization_content_ref,
		permission_action_kind, permission_project_root,
		permission_method_description_ref,
		permission_valid_from, permission_valid_until,
		permission_single_use_key, permission_profile_admission_predicate_ref,
		permission_context_policy_ref, context_policy_ref,
		context_policy_digest, action_kind, project_root,
		profile_author_role_assignment_ref, profile_author_role_assignment_digest,
		method_description_ref, method_description_digest,
		method_contract_ref, method_contract_digest,
		classifier_version, policy_version, session_ref,
		allowed_work_from, allowed_work_until,
		basis_observation_from, basis_observation_until,
		valid_from, valid_until, single_use_key, project_binding_digest,
		envelope_digest, presentation_digest, recorded_at
	) VALUES (
		'presentation.typed', 'speech:typed', 'digest:speech',
		'content:typed', 'digest:content',
		'permission:typed', 'digest:permission', 'MAY',
		'speech:typed', 'assignment:test', 'content:typed',
		'profile.declare.from_onboarding_candidate', '/tmp/project',
		'method-description:onboard',
		'2026-07-14T00:10:00Z', '2026-07-14T00:50:00Z',
		'use.typed', 'predicate:profile-admission', 'policy:context',
		'policy:context', 'digest:context-policy',
		'profile.declare.from_onboarding_candidate', '/tmp/project',
		'assignment:test', ?,
		'method-description:onboard', ?,
		'contract:onboard', ?,
		'classifier:v1', 'policy:v1', ?,
		'2026-07-14T00:20:00Z', '2026-07-14T00:30:00Z',
		'2026-07-14T00:15:00Z', '2026-07-14T00:20:00Z',
		'2026-07-14T00:10:00Z', '2026-07-14T00:50:00Z',
		'use.typed', 'digest:project-binding', 'digest:envelope',
		'digest:presentation', '2026-07-14T00:10:00Z'
	)`
	_, err := conn.Exec(
		presentationSQL,
		assignmentDigest,
		methodDescriptionDigest,
		methodContractDigest,
		sessionRef,
	)
	return err
}

func insertTypedMemoryAuthorityResolutionFixture(
	conn *sql.DB,
	assignmentDigest string,
	methodDescriptionDigest string,
	methodContractDigest string,
) error {
	resolutionSQL := `INSERT INTO authority_resolution_records (
		authority_resolution_id, presentation_id, presentation_digest,
		profile_author_role_assignment_ref, profile_author_role_assignment_digest,
		method_description_ref, method_description_digest,
		method_contract_ref, method_contract_digest,
		verifier_identity, verifier_version, verification_policy_ref,
		verification_policy_digest, resolved_at, valid_until,
		authority_resolution_digest, recorded_at
	) VALUES (
		'resolution:typed', 'presentation.typed', 'digest:presentation',
		'assignment:test', ?,
		'method-description:onboard', ?,
		'contract:onboard', ?,
		'verifier:test', 'v1', 'policy:verification',
		'digest:verification-policy', '2026-07-14T00:10:00Z',
		'2026-07-14T00:50:00Z', 'digest:resolution',
		'2026-07-14T00:10:00Z'
	)`
	_, err := conn.Exec(
		resolutionSQL,
		assignmentDigest,
		methodDescriptionDigest,
		methodContractDigest,
	)
	return err
}

func insertTypedMemoryDAGFixture(t *testing.T, conn *sql.DB) []typedMemoryTableFixture {
	t.Helper()
	statements := []string{
		`INSERT INTO profile_onboarding_method_descriptions (
			method_description_ref, described_method_ref, bounded_context_ref,
			source_revision, edition, required_role_ref, required_system_kind,
			state_plane_ref, affected_ref_kind, effect_witness_rule_ref,
			method_description_json, method_description_digest, recorded_at
		) VALUES (
			'method-description:onboard', 'method:onboard', 'context:onboard',
			'fpf:revision', 'v1', 'role:profile-author', 'U.System',
			'state:profile', 'ProfileClassificationEpistemeV1', 'rule:effect',
			'{}', 'digest:method-description', '2026-07-14T00:00:00Z'
		)`,
		`INSERT INTO profile_onboarding_method_contracts (
			method_contract_ref, edition, method_description_ref,
			method_description_digest, bounded_context_ref,
			role_admission_policy_ref, system_admission_policy_ref,
			parameter_spec_set_digest, accepted_result_kinds_json,
			required_occurrence_slots_json, occurrence_coverage_rule_refs_json,
			effect_state_witness_rule_ref, acceptance_standard_ref,
			acceptance_standard_edition, holder_equals_executed_within_rule_ref,
			method_contract_json, method_contract_digest, recorded_at
		) VALUES (
			'contract:onboard', 'v1', 'method-description:onboard',
			'digest:method-description', 'context:onboard',
			'policy:role', 'policy:system', 'digest:parameters',
			'["CandidatePayloadProduced","ClassificationUnderdetermined"]',
			'["work_interval","basis_observation_window"]', '["rule:coverage"]',
			'rule:effect', 'acceptance:onboard', 'v1', 'rule:holder',
			'{}', 'digest:contract', '2026-07-14T00:00:00Z'
		)`,
		`INSERT INTO profile_onboarding_executor_system_admissions (
			system_admission_ref, system_ref, admitted_system_kind,
			bounded_context_ref, governing_pattern_ref,
			identity_basis_kind, identity_basis_system_ref,
			identity_basis_kernel_identity, identity_basis_kernel_version,
			identity_basis_designation_ref, identity_basis_designation_digest,
			acting_eligibility_basis_ref, acting_eligibility_basis_digest,
			session_ref, valid_from, valid_until,
			method_description_ref, method_description_digest,
			method_contract_ref, method_contract_digest,
			system_admission_policy_ref, system_admission_json,
			system_admission_digest, recorded_at
		) VALUES (
			'system-admission:test', 'system:haft', 'U.System',
			'context:onboard', 'A.1', 'kernel_owned', 'system:haft',
			'haft-kernel', 'v9', '', '',
			'eligibility:test', 'digest:eligibility',
			'session:test', '2026-07-14T00:00:00Z', '2026-07-14T01:00:00Z',
			'method-description:onboard', 'digest:method-description',
			'contract:onboard', 'digest:contract', 'policy:system',
			'{}', 'digest:system-admission', '2026-07-14T00:00:00Z'
		)`,
		`INSERT INTO profile_author_role_admissions (
			role_admission_ref, role_ref, bounded_context_ref,
			governing_pattern_ref, method_description_ref,
			method_description_digest, method_contract_ref,
			method_contract_digest, role_admission_policy_ref,
			role_admission_json, role_admission_digest, recorded_at
		) VALUES (
			'role-admission:test', 'role:profile-author', 'context:onboard',
			'A.2.1', 'method-description:onboard', 'digest:method-description',
			'contract:onboard', 'digest:contract', 'policy:role',
			'{}', 'digest:role-admission', '2026-07-14T00:00:00Z'
		)`,
		`INSERT INTO profile_author_assignment_support_carriers (
			assignment_justification_ref, assignment_rule_ref,
			assignment_rule_statement, bounded_context_ref,
			system_admission_ref, system_admission_digest,
			role_admission_ref, role_admission_digest,
			assignment_from, assignment_until, method_contract_ref,
			method_contract_digest, assignment_justification_json,
			assignment_justification_digest, assignment_provenance_ref,
			provenance_justification_ref, provenance_justification_digest,
			session_ref, kernel_identity, kernel_version,
			runtime_identity, runtime_version, provenance_recorded_at,
			assignment_provenance_json, assignment_provenance_digest, recorded_at
		) VALUES (
			'justification:test', 'rule:assignment', 'exact assignment support',
			'context:onboard', 'system-admission:test', 'digest:system-admission',
			'role-admission:test', 'digest:role-admission',
			'2026-07-14T00:10:00Z', '2026-07-14T00:50:00Z',
			'contract:onboard', 'digest:contract', '{}', 'digest:justification',
			'provenance:test', 'justification:test', 'digest:justification',
			'session:test', 'haft-kernel', 'v9', 'codex', 'v1',
			'2026-07-14T00:10:00Z', '{}', 'digest:provenance',
			'2026-07-14T00:10:00Z'
		)`,
		`INSERT INTO profile_author_role_assignments (
			role_assignment_ref, holder_system_ref, admitted_role_ref,
			bounded_context_ref, valid_from, valid_until,
			system_admission_ref, system_admission_digest,
			role_admission_ref, role_admission_digest,
			assignment_justification_ref, assignment_justification_digest,
			assignment_provenance_ref, assignment_provenance_digest,
			role_assignment_json, role_assignment_digest, recorded_at
		) VALUES (
			'assignment:test', 'system:haft', 'role:profile-author',
			'context:onboard', '2026-07-14T00:10:00Z', '2026-07-14T00:50:00Z',
			'system-admission:test', 'digest:system-admission',
			'role-admission:test', 'digest:role-admission',
			'justification:test', 'digest:justification',
			'provenance:test', 'digest:provenance',
			'{}', 'digest:assignment', '2026-07-14T00:10:00Z'
		)`,
		`INSERT INTO observed_project_bases (
			observed_project_basis_ref, project_root, observation_from,
			observation_until, detector_version, classifier_version,
			observed_project_basis_json, observed_project_basis_digest, recorded_at
		) VALUES (
			'basis:test', '/tmp/project', '2026-07-14T00:15:00Z',
			'2026-07-14T00:20:00Z', 'detector:v1', 'classifier:v1',
			'{}', 'digest:basis', '2026-07-14T00:20:00Z'
		)`,
		`INSERT INTO profile_onboarding_work_records (
			work_record_ref, work_ref, project_root, enacts_method_ref,
			method_description_ref, method_description_digest,
			method_contract_ref, method_contract_digest, parameter_bindings_json,
			performed_by_role_assignment_ref, profile_author_role_assignment_ref,
			profile_author_role_assignment_digest, executed_within_ref,
			work_from, work_until, bounded_context_ref,
			basis_observation_from, basis_observation_until,
			observed_project_basis_ref, observed_project_basis_digest,
			inputs_json, outputs_json, resources_json, affected_ref_kind,
			affected_refs_json, state_plane_ref, pre_state_ref, post_state_ref,
			delta_predicate_ref, outcome_kind, profile_payload_digest,
			observed_basis_digest, missing_basis_digest, work_record_json,
			work_record_digest, recorded_at
		) VALUES (
			'work-record:test', 'work:test', '/tmp/project', 'method:onboard',
			'method-description:onboard', 'digest:method-description',
			'contract:onboard', 'digest:contract',
			'[{"name":"classifier_version","value":"classifier:v1"},{"name":"policy_version","value":"policy:v1"},{"name":"project_root","value":"/tmp/project"},{"name":"session_ref","value":"session:test"}]',
			'assignment:test', 'assignment:test', 'digest:assignment', 'system:haft',
			'2026-07-14T00:20:00Z', '2026-07-14T00:30:00Z', 'context:onboard',
			'2026-07-14T00:15:00Z', '2026-07-14T00:20:00Z',
			'basis:test', 'digest:basis', '["basis:test"]', '["output:test"]', '[]',
			'ProfileClassificationEpistemeV1', '["entity:test"]', 'state:profile',
			'state:before', 'state:after', '', 'CandidatePayloadProduced',
			'digest:payload', 'digest:basis', '', '{}', 'digest:work',
			'2026-07-14T00:30:00Z'
		)`,
		`INSERT INTO profile_onboarding_effects (
			effect_ref, work_record_ref, work_ref, work_record_digest,
			result_kind, output_ref, profile_payload_digest,
			observed_project_basis_ref, observed_project_basis_digest,
			missing_basis_digest, affected_entity_refs_json, state_plane_ref,
			pre_state_ref, post_state_ref, delta_predicate_ref,
			evidence_provenance_path_refs_json, effect_json, effect_digest, recorded_at
		) VALUES (
			'effect:test', 'work-record:test', 'work:test', 'digest:work',
			'CandidatePayloadProduced', 'output:test', 'digest:payload',
			'basis:test', 'digest:basis', '', '["entity:test"]', 'state:profile',
			'state:before', 'state:after', '', '[]', '{}', 'digest:effect',
			'2026-07-14T00:31:00Z'
		)`,
		`INSERT INTO profile_onboarding_outcome_assessments (
			outcome_assessment_ref, effect_ref, effect_digest,
			work_record_ref, work_ref, work_record_digest,
			acceptance_standard_ref, acceptance_standard_edition,
			comparator_ref, comparator_edition, verdict_kind,
			verdict_reason_ref, missing_basis_digest,
			evidence_provenance_path_refs_json, outcome_assessment_json,
			outcome_assessment_digest, recorded_at
		) VALUES (
			'assessment:test', 'effect:test', 'digest:effect',
			'work-record:test', 'work:test', 'digest:work',
			'acceptance:onboard', 'v1', 'comparator:test', 'v1', 'passed',
			'', '', '[]', '{}', 'digest:assessment', '2026-07-14T00:32:00Z'
		)`,
	}
	for _, statement := range statements {
		if _, err := conn.Exec(statement); err != nil {
			t.Fatalf("insert typed-memory DAG fixture: %v\nstatement: %s", err, statement)
		}
	}
	return []typedMemoryTableFixture{
		{name: "profile_onboarding_method_descriptions", primaryKey: "method_description_ref", primaryValue: "method-description:onboard"},
		{name: "profile_onboarding_method_contracts", primaryKey: "method_contract_ref", primaryValue: "contract:onboard"},
		{name: "profile_onboarding_executor_system_admissions", primaryKey: "system_admission_ref", primaryValue: "system-admission:test"},
		{name: "profile_author_role_admissions", primaryKey: "role_admission_ref", primaryValue: "role-admission:test"},
		{name: "profile_author_assignment_support_carriers", primaryKey: "assignment_justification_ref", primaryValue: "justification:test"},
		{name: "profile_author_role_assignments", primaryKey: "role_assignment_ref", primaryValue: "assignment:test"},
		{name: "observed_project_bases", primaryKey: "observed_project_basis_ref", primaryValue: "basis:test"},
		{name: "profile_onboarding_work_records", primaryKey: "work_record_ref", primaryValue: "work-record:test"},
		{name: "profile_onboarding_effects", primaryKey: "effect_ref", primaryValue: "effect:test"},
		{name: "profile_onboarding_outcome_assessments", primaryKey: "outcome_assessment_ref", primaryValue: "assessment:test"},
	}
}

func tableColumns(t *testing.T, conn *sql.DB, table string) map[string]bool {
	t.Helper()
	rows, err := conn.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatalf("inspect %s schema: %v", table, err)
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var columnID int
		var name string
		var dataType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		err := rows.Scan(&columnID, &name, &dataType, &notNull, &defaultValue, &primaryKey)
		if err != nil {
			t.Fatalf("read %s schema: %v", table, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s schema: %v", table, err)
	}
	return columns
}

func TestRunMigrations_ExistingDatabase(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create database with old schema (no parent_id, no cached_r_score)
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}

	oldSchema := `CREATE TABLE holons (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		kind TEXT,
		layer TEXT NOT NULL,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		context_id TEXT NOT NULL,
		scope TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
	if _, err := conn.Exec(oldSchema); err != nil {
		t.Fatalf("Failed to create old schema: %v", err)
	}
	conn.Close()

	// Now open with NewStore which runs kernelMigrations
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// Verify new columns exist by querying them
	var parentID sql.NullString
	var cachedRScore sql.NullFloat64
	err = store.conn.QueryRow("SELECT parent_id, cached_r_score FROM holons LIMIT 1").Scan(&parentID, &cachedRScore)
	// Will get sql.ErrNoRows since table is empty, but query should not fail due to missing columns
	if err != nil && err != sql.ErrNoRows {
		t.Errorf("New columns should exist: %v", err)
	}

	// Verify kernelMigrations are recorded
	var count int
	store.conn.QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&count)
	if count != len(kernelMigrations) {
		t.Errorf("Expected %d kernelMigrations recorded, got %d", len(kernelMigrations), count)
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Run kernelMigrations twice
	store1, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("First NewStore failed: %v", err)
	}
	store1.Close()

	store2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Second NewStore failed: %v", err)
	}
	defer store2.Close()

	// Should still have same number of migration records
	var count int
	store2.conn.QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&count)
	if count != len(kernelMigrations) {
		t.Errorf("Expected %d kernelMigrations, got %d (not idempotent)", len(kernelMigrations), count)
	}
}

func TestMigrateRejectsFutureSchemaBeforeApplyingOlderBinaryChanges(
	t *testing.T,
) {
	t.Parallel()
	database, err := sql.Open(
		"sqlite",
		filepath.Join(t.TempDir(), "future-schema.db"),
	)
	if err != nil {
		t.Fatalf("open future-schema fixture: %v", err)
	}
	defer database.Close()
	_, err = database.Exec(`
		CREATE TABLE schema_version (
			version INTEGER PRIMARY KEY,
			applied_at TEXT DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO schema_version(version) VALUES (51);
		CREATE TABLE future_schema_sentinel(value TEXT NOT NULL);
	`)
	if err != nil {
		t.Fatalf("install future-schema fixture: %v", err)
	}
	migrations := []Migration{{
		Version:     50,
		Description: "older binary mutation must not run",
		Statements: []string{
			`INSERT INTO future_schema_sentinel(value) VALUES ('mutated')`,
		},
	}}

	err = Migrate(database, "schema_version", migrations)
	if err == nil || !strings.Contains(err.Error(), "newer than this binary") {
		t.Fatalf(
			"Migrate() error = %v, want future-schema refusal",
			err,
		)
	}
	count := 0
	if queryErr := database.QueryRow(
		`SELECT COUNT(*) FROM future_schema_sentinel`,
	).Scan(&count); queryErr != nil {
		t.Fatalf("read future-schema sentinel: %v", queryErr)
	}
	if count != 0 {
		t.Fatalf("older migration mutated future schema: rows = %d", count)
	}
}

func TestRunMigrations_AddsEpistemicDebtBudget(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	var budget sql.NullFloat64
	err = store.conn.QueryRow(
		"SELECT epistemic_debt_budget FROM fpf_state LIMIT 1",
	).Scan(&budget)
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("query epistemic_debt_budget: %v", err)
	}
}
