package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileAuthorityV2Migration43InstallsSourceNativeClosure(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "profile-authority-v2.db"))
	if err != nil {
		t.Fatalf("open v43 store: %v", err)
	}
	defer store.Close()

	for _, table := range profileAuthorityV2Tables {
		assertSQLiteObjectExists(t, store.conn, "table", table)
		assertSQLiteObjectExists(t, store.conn, "trigger", table+"_no_replace")
		assertSQLiteObjectExists(t, store.conn, "trigger", table+"_no_update")
		assertSQLiteObjectExists(t, store.conn, "trigger", table+"_no_delete")
		assertSQLiteObjectExists(t, store.conn, "trigger", table+"_project_ledger_root")
	}
	for _, trigger := range []string{
		"profile_declaration_authorization_contents_v2_exact_sources",
		"profile_declaration_authorization_preparations_v2_exact_sources",
		"profile_declaration_permissions_v2_exact_sources",
		"profile_declaration_instituted_effects_v2_exact_sources",
		"profile_declaration_authority_bases_v2_exact_sources",
	} {
		assertSQLiteObjectExists(t, store.conn, "trigger", trigger)
	}
	assertMigrationVersionPresent(t, store.conn, 43)
	assertProfileAuthorityV2ForeignKeysClean(t, store.conn)
}

func TestProfileAuthorityV2Migration43PinsCorrectedTypedProtocol(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "profile-authority-protocol.db"))
	if err != nil {
		t.Fatalf("open v43 store: %v", err)
	}
	defer store.Close()

	permissionTrigger := profileAuthorityV2SQLiteSQL(
		t,
		store.conn,
		"trigger",
		"profile_declaration_permissions_v2_exact_sources",
	)
	for _, required := range []string{
		"act.review_subject_ref = content.authorization_content_ref",
		"act.review_subject_digest = content.authorization_content_digest",
		"item.value NOT GLOB 'E-?*'",
		"item.value NOT GLOB 'carrier-class:?*'",
		"json_extract(NEW.referents_json, '$[0]') = NEW.method_description_ref",
		"json_extract(NEW.referents_json, '$[1]') = NEW.enactability_predicate_ref",
		"capture.prepared_speech_act_intent_digest = prepared.speech_act_intent_digest",
		"policy.institutional_effect_rule_ref = '" + profileAuthorityV2EffectRuleRef + "'",
		"method.procedure_semantics = '" + profileAuthorityV2ProcedureText + "'",
	} {
		if !strings.Contains(permissionTrigger, required) {
			t.Fatalf("v43 Permission trigger is missing %q: %s", required, permissionTrigger)
		}
	}
	if strings.Contains(permissionTrigger, "act.review_subject_ref = prepared.evidence_claim_ref") {
		t.Fatalf("v43 Permission trigger collapses review subject into E-* evidence: %s", permissionTrigger)
	}

	schemas := map[string]string{
		profileAuthorityV2ContentTable:     "haft.profile-authority.authorization-content/v1",
		profileAuthorityV2PreparationTable: "haft.profile-authority.prepared-authorization/v1",
		profileAuthorityV2PermissionTable:  "haft.profile-authority.permission/v2",
		profileAuthorityV2EffectTable:      "haft.profile-authority.instituted-permission-effect/v2",
		profileAuthorityV2BasisTable:       "haft.profile-authority.four-ref-basis/v1",
	}
	for table, schemaID := range schemas {
		tableSQL := profileAuthorityV2SQLiteSQL(t, store.conn, "table", table)
		if !strings.Contains(tableSQL, schemaID) {
			t.Fatalf("v43 table %s does not pin canonical schema %s: %s", table, schemaID, tableSQL)
		}
	}
	for _, table := range []string{
		profileAuthorityV2PreparationTable,
		profileAuthorityV2PermissionTable,
	} {
		tableSQL := profileAuthorityV2SQLiteSQL(t, store.conn, "table", table)
		if !strings.Contains(tableSQL, "enactability_predicate_ref") {
			t.Fatalf("v43 table %s lacks the Permission-enactability predicate: %s", table, tableSQL)
		}
		if strings.Contains(tableSQL, "admission_predicate_ref") ||
			strings.Contains(tableSQL, "profile_admission_predicate_ref") {
			t.Fatalf("v43 table %s retains the misleading admission-predicate label: %s", table, tableSQL)
		}
	}
	basisTable := profileAuthorityV2SQLiteSQL(t, store.conn, "table", profileAuthorityV2BasisTable)
	if !strings.Contains(basisTable, "json_type(canonical_json, '$.instituted_effect_digest') IS NULL") {
		t.Fatalf("four-ref basis must keep effect digest relational-only: %s", basisTable)
	}
	basisTrigger := profileAuthorityV2SQLiteSQL(
		t,
		store.conn,
		"trigger",
		"profile_declaration_authority_bases_v2_exact_sources",
	)
	for _, forbidden := range []string{"authority_basis_presentations", "authority_basis_resolutions"} {
		if strings.Contains(basisTrigger, forbidden) {
			t.Fatalf("source-native four-ref basis depends on legacy %s: %s", forbidden, basisTrigger)
		}
	}
}

func TestProfileAuthorityV2Migration43RejectsNonCanonicalRecordedAtShape(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "profile-authority-time-shape.db"))
	if err != nil {
		t.Fatalf("open v43 store: %v", err)
	}
	defer store.Close()

	shape := sqliteCanonicalUTCNanoShape("recorded_at")
	for _, table := range profileAuthorityV2Tables {
		tableSQL := profileAuthorityV2SQLiteSQL(t, store.conn, "table", table)
		if !strings.Contains(tableSQL, shape) {
			t.Fatalf("v43 table %s does not constrain recorded_at to canonical UTC-nano: %s", table, tableSQL)
		}
	}
	_, err = store.conn.Exec(
		"CREATE TEMP TABLE recorded_at_shape_probe (recorded_at TEXT NOT NULL, CHECK(" + shape + "))",
	)
	if err != nil {
		t.Fatalf("create recorded_at shape probe: %v", err)
	}
	_, err = store.conn.Exec(
		"INSERT INTO recorded_at_shape_probe(recorded_at) VALUES (?)",
		"2026-07-15T08:09:10.123456789Z",
	)
	if err != nil {
		t.Fatalf("canonical UTC-nano timestamp was rejected: %v", err)
	}
	for _, invalid := range []string{
		"2026-07-15 08:09:10Z",
		"2026-07-15T08:09:10+00:00",
		"2026-07-15T08:09:10.123456780Z",
		"not-a-time",
	} {
		_, insertErr := store.conn.Exec(
			"INSERT INTO recorded_at_shape_probe(recorded_at) VALUES (?)",
			invalid,
		)
		if insertErr == nil {
			t.Fatalf("non-canonical recorded_at %q crossed the v43 shape", invalid)
		}
	}
}

func TestProfileAuthorityV2Migration43RejectsUnknownPartialFootprintAtomically(t *testing.T) {
	database := openDatabaseBeforeMigration43(t)
	defer database.Close()
	if _, err := database.Exec(`CREATE TABLE profile_declaration_authorization_contents_v2 (project_root TEXT)`); err != nil {
		t.Fatalf("create unknown partial v43 table: %v", err)
	}

	err := Migrate(database, "schema_version", []Migration{profileAuthorityV2Migration43})
	if err == nil || !strings.Contains(err.Error(), "unknown partial schema") {
		t.Fatalf("expected unknown partial-schema refusal, got %v", err)
	}
	assertMigrationVersionAbsent(t, database, 43)
	for _, table := range profileAuthorityV2Tables[1:] {
		assertSQLiteObjectAbsent(t, database, "table", table)
	}
}

func TestProfileAuthorityV2Migration43PreparationSurvivesRestartBeforeTTY(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile-preparation-restart.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("open v43 store: %v", err)
	}
	insertProfileAuthorityV2SourceFixture(t, store.conn)
	fixture := insertProfileAuthorityV2PreparationFixture(t, store.conn)
	assertProfileAuthorityV2PreTTYState(t, store.conn)
	if err := store.Close(); err != nil {
		t.Fatalf("close pre-TTY store: %v", err)
	}

	reopened, err := NewStore(path)
	if err != nil {
		t.Fatalf("reopen pre-TTY store: %v", err)
	}
	defer reopened.Close()
	var contentDigest string
	var contentJSON string
	if err := reopened.conn.QueryRow(
		`SELECT authorization_content_digest, canonical_json
		 FROM profile_declaration_authorization_contents_v2
		 WHERE authorization_content_ref = ?`,
		fixture.contentRef,
	).Scan(&contentDigest, &contentJSON); err != nil {
		t.Fatalf("reload authorization content: %v", err)
	}
	if contentDigest != fixture.contentDigest || contentJSON != fixture.contentJSON {
		t.Fatalf("authorization content changed across restart")
	}
	var preparedDigest string
	var preparedJSON string
	if err := reopened.conn.QueryRow(
		`SELECT prepared_authorization_digest, canonical_json
		 FROM profile_declaration_authorization_preparations_v2
		 WHERE authorization_content_ref = ?`,
		fixture.contentRef,
	).Scan(&preparedDigest, &preparedJSON); err != nil {
		t.Fatalf("reload prepared authorization: %v", err)
	}
	if preparedDigest != fixture.preparedDigest || preparedJSON != fixture.preparedJSON {
		t.Fatalf("prepared authorization changed across restart")
	}
	assertProfileAuthorityV2PreTTYState(t, reopened.conn)
}

func TestProfileAuthorityV2Migration43PreservesHistoricalV38WithoutBackfill(t *testing.T) {
	database := openDatabaseBeforeMigration43(t)
	defer database.Close()
	insertProfileAuthorityV2SourceFixture(t, database)
	legacyDigest, legacyJSON := insertHistoricalProfileAuthorityContent(t, database)

	if err := Migrate(database, "schema_version", []Migration{profileAuthorityV2Migration43}); err != nil {
		t.Fatalf("apply v43 over historical v38 content: %v", err)
	}
	var gotDigest string
	var gotJSON string
	if err := database.QueryRow(
		`SELECT authorization_content_digest, canonical_json
		 FROM profile_declaration_authorization_contents
		 WHERE authorization_content_ref = 'authorization-content:historical-v38'`,
	).Scan(&gotDigest, &gotJSON); err != nil {
		t.Fatalf("read historical v38 content after v43: %v", err)
	}
	if gotDigest != legacyDigest || gotJSON != legacyJSON {
		t.Fatalf("v43 rewrote historical v38 content")
	}
	for _, table := range profileAuthorityV2Tables {
		var count int
		if err := database.QueryRow("SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table)).Scan(&count); err != nil {
			t.Fatalf("count v43 table %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("v43 backfilled %s with %d row(s)", table, count)
		}
	}
}

type profileAuthorityV2PreparationFixture struct {
	contentRef     string
	contentDigest  string
	contentJSON    string
	preparedDigest string
	preparedJSON   string
}

func insertProfileAuthorityV2SourceFixture(t testing.TB, database *sql.DB) {
	t.Helper()
	methodDigest := authorityBasisTestDigest("1")
	contractDigest := authorityBasisTestDigest("2")
	assignmentDigest := authorityBasisTestDigest("6")
	mustExecProfileAuthorityV2(t, database, `INSERT INTO profile_onboarding_method_descriptions (
		method_description_ref, described_method_ref, bounded_context_ref,
		source_revision, edition, required_role_ref, required_system_kind,
		state_plane_ref, affected_ref_kind, effect_witness_rule_ref,
		method_description_json, method_description_digest, recorded_at
	) VALUES (?, 'method:onboard:v43', 'context:onboard:v43', 'fpf:v43', 'v43',
		'role:profile-author', 'U.System', 'state:profile',
		'ProfileClassificationEpistemeV1', 'rule:effect', '{}', ?, '2026-07-15T00:00:00Z')`,
		"method-description:onboard:v43", methodDigest)
	mustExecProfileAuthorityV2(t, database, `INSERT INTO profile_onboarding_method_contracts (
		method_contract_ref, edition, method_description_ref,
		method_description_digest, bounded_context_ref,
		role_admission_policy_ref, system_admission_policy_ref,
		parameter_spec_set_digest, accepted_result_kinds_json,
		required_occurrence_slots_json, occurrence_coverage_rule_refs_json,
		effect_state_witness_rule_ref, acceptance_standard_ref,
		acceptance_standard_edition, holder_equals_executed_within_rule_ref,
		method_contract_json, method_contract_digest, recorded_at
	) VALUES ('contract:onboard:v43', 'v43', 'method-description:onboard:v43', ?,
		'context:onboard:v43', 'policy:role:v43', 'policy:system:v43',
		'digest:parameters:v43', '["CandidatePayloadProduced"]', '["work_interval"]',
		'["rule:coverage"]', 'rule:effect', 'acceptance:onboard:v43', 'v43',
		'rule:holder', '{}', ?, '2026-07-15T00:00:00Z')`, methodDigest, contractDigest)
	mustExecProfileAuthorityV2(t, database, `INSERT INTO profile_onboarding_executor_system_admissions (
		system_admission_ref, system_ref, admitted_system_kind, bounded_context_ref,
		governing_pattern_ref, identity_basis_kind, identity_basis_system_ref,
		identity_basis_kernel_identity, identity_basis_kernel_version,
		identity_basis_designation_ref, identity_basis_designation_digest,
		acting_eligibility_basis_ref, acting_eligibility_basis_digest,
		session_ref, valid_from, valid_until, method_description_ref,
		method_description_digest, method_contract_ref, method_contract_digest,
		system_admission_policy_ref, system_admission_json,
		system_admission_digest, recorded_at
	) VALUES ('system-admission:v43', 'system:haft:v43', 'U.System',
		'context:onboard:v43', 'A.1', 'kernel_owned', 'system:haft:v43',
		'haft-kernel', 'v9', '', '', 'eligibility:v43', 'digest:eligibility:v43',
		'session:future-work:v43', '2026-07-15T00:00:00Z', '2026-07-15T05:00:00Z',
		'method-description:onboard:v43', ?, 'contract:onboard:v43', ?,
		'policy:system:v43', '{}', 'digest:system-admission:v43', '2026-07-15T00:00:00Z')`,
		methodDigest, contractDigest)
	mustExecProfileAuthorityV2(t, database, `INSERT INTO profile_author_role_admissions (
		role_admission_ref, role_ref, bounded_context_ref, governing_pattern_ref,
		method_description_ref, method_description_digest, method_contract_ref,
		method_contract_digest, role_admission_policy_ref, role_admission_json,
		role_admission_digest, recorded_at
	) VALUES ('role-admission:v43', 'role:profile-author', 'context:onboard:v43', 'A.2.1',
		'method-description:onboard:v43', ?, 'contract:onboard:v43', ?,
		'policy:role:v43', '{}', 'digest:role-admission:v43', '2026-07-15T00:00:00Z')`,
		methodDigest, contractDigest)
	mustExecProfileAuthorityV2(t, database, `INSERT INTO profile_author_assignment_support_carriers (
		assignment_justification_ref, assignment_rule_ref, assignment_rule_statement,
		bounded_context_ref, system_admission_ref, system_admission_digest,
		role_admission_ref, role_admission_digest, assignment_from, assignment_until,
		method_contract_ref, method_contract_digest, assignment_justification_json,
		assignment_justification_digest, assignment_provenance_ref,
		provenance_justification_ref, provenance_justification_digest,
		session_ref, kernel_identity, kernel_version, runtime_identity, runtime_version,
		provenance_recorded_at, assignment_provenance_json,
		assignment_provenance_digest, recorded_at
	) VALUES ('justification:v43', 'rule:assignment:v43', 'exact v43 assignment',
		'context:onboard:v43', 'system-admission:v43', 'digest:system-admission:v43',
		'role-admission:v43', 'digest:role-admission:v43',
		'2026-07-15T00:00:00Z', '2026-07-15T05:00:00Z',
		'contract:onboard:v43', ?, '{}', 'digest:justification:v43',
		'provenance:v43', 'justification:v43', 'digest:justification:v43',
		'session:future-work:v43', 'haft-kernel', 'v9', 'codex', 'v1',
		'2026-07-15T00:00:00Z', '{}', 'digest:provenance:v43', '2026-07-15T00:00:00Z')`,
		contractDigest)
	mustExecProfileAuthorityV2(t, database, `INSERT INTO profile_author_role_assignments (
		role_assignment_ref, holder_system_ref, admitted_role_ref, bounded_context_ref,
		valid_from, valid_until, system_admission_ref, system_admission_digest,
		role_admission_ref, role_admission_digest, assignment_justification_ref,
		assignment_justification_digest, assignment_provenance_ref,
		assignment_provenance_digest, role_assignment_json,
		role_assignment_digest, recorded_at
	) VALUES ('assignment:v43', 'system:haft:v43', 'role:profile-author',
		'context:onboard:v43', '2026-07-15T00:00:00Z', '2026-07-15T05:00:00Z',
		'system-admission:v43', 'digest:system-admission:v43',
		'role-admission:v43', 'digest:role-admission:v43',
		'justification:v43', 'digest:justification:v43',
		'provenance:v43', 'digest:provenance:v43', '{}', ?, '2026-07-15T00:00:00Z')`,
		assignmentDigest)
}

func insertProfileAuthorityV2PreparationFixture(
	t *testing.T,
	database *sql.DB,
) profileAuthorityV2PreparationFixture {
	t.Helper()
	fixture := profileAuthorityV2PreparationFixture{
		contentRef:     "authorization-content:profile-v43",
		contentDigest:  authorityBasisTestDigest("7"),
		preparedDigest: authorityBasisTestDigest("8"),
	}
	fixture.contentJSON = mustAuthorityBasisJSON(t, map[string]any{
		"schema":                                "haft.profile-authority.authorization-content/v1",
		"authorization_content_ref":             fixture.contentRef,
		"project_root":                          "/tmp/project-v43",
		"action_kind":                           profileAuthorityV2Action,
		"profile_author_role_assignment_ref":    "assignment:v43",
		"profile_author_role_assignment_digest": authorityBasisTestDigest("6"),
		"method_description_ref":                "method-description:onboard:v43",
		"method_description_digest":             authorityBasisTestDigest("1"),
		"method_contract_ref":                   "contract:onboard:v43",
		"method_contract_digest":                authorityBasisTestDigest("2"),
		"classifier_version":                    "classifier:v43",
		"policy_version":                        "profile-policy:v43",
		"session_ref":                           "session:future-work:v43",
		"allowed_work_from":                     "2026-07-15T01:00:00Z",
		"allowed_work_until":                    "2026-07-15T04:00:00Z",
		"basis_observation_from":                "2026-07-15T00:30:00Z",
		"basis_observation_until":               "2026-07-15T03:30:00Z",
		"authorization_valid_from":              "2026-07-15T00:00:00Z",
		"authorization_valid_until":             "2026-07-15T05:00:00Z",
		"single_use_key":                        "single-use:profile-v43",
	})
	mustExecProfileAuthorityV2(t, database, `INSERT INTO profile_declaration_authorization_contents_v2 (
		authorization_content_ref, authorization_content_digest, project_root,
		action_kind, profile_author_role_assignment_ref,
		profile_author_role_assignment_digest, method_description_ref,
		method_description_digest, method_contract_ref, method_contract_digest,
		classifier_version, policy_version, session_ref, allowed_work_from,
		allowed_work_until, basis_observation_from, basis_observation_until,
		authorization_valid_from, authorization_valid_until, single_use_key,
		canonical_json, recorded_at
	) VALUES (?, ?, '/tmp/project-v43', ?, 'assignment:v43', ?,
		'method-description:onboard:v43', ?, 'contract:onboard:v43', ?,
		'classifier:v43', 'profile-policy:v43', 'session:future-work:v43',
		'2026-07-15T01:00:00Z', '2026-07-15T04:00:00Z',
		'2026-07-15T00:30:00Z', '2026-07-15T03:30:00Z',
		'2026-07-15T00:00:00Z', '2026-07-15T05:00:00Z',
		'single-use:profile-v43', ?, '2026-07-15T00:00:00Z')`,
		fixture.contentRef, fixture.contentDigest, profileAuthorityV2Action,
		authorityBasisTestDigest("6"), authorityBasisTestDigest("1"),
		authorityBasisTestDigest("2"), fixture.contentJSON)
	fixture.preparedJSON = mustAuthorityBasisJSON(t, map[string]any{
		"schema":                       "haft.profile-authority.prepared-authorization/v1",
		"authorization_content_ref":    fixture.contentRef,
		"authorization_content_digest": fixture.contentDigest,
		"permission_ref":               "permission:profile-v43",
		"speech_act_ref":               "speech-act:profile-v43",
		"capture_carrier_ref":          "carrier:terminal-capture:profile-v43",
		"speech_act_session_ref":       "session:speech-act:v43",
		"claim_scope_ref":              "claim-scope:profile-v43",
		"enactability_predicate_ref":   "A-permission-enactability:v43",
		"evidence_claim_ref":           "E-profile-authority:v43",
		"carrier_class_ref":            "carrier-class:controlling-terminal-capture:v43",
		"verifier_identity":            "verifier:profile-authority:v43",
		"verifier_version":             "verifier-version:v43",
		"verification_policy_ref":      "verification-policy:profile-authority:v43",
		"verification_policy_digest":   authorityBasisTestDigest("9"),
		"basis_ref":                    "profile-authority-basis:v43",
		"context_policy_ref":           profileAuthorityV2PolicyRef,
		"context_policy_digest":        authorityBasisTestDigest("a"),
		"speech_act_intent_digest":     authorityBasisTestDigest("b"),
	})
	mustExecProfileAuthorityV2(t, database, `INSERT INTO profile_declaration_authorization_preparations_v2 (
		prepared_authorization_digest, project_root, authorization_content_ref,
		authorization_content_digest, permission_ref, speech_act_ref,
		capture_carrier_ref, speech_act_session_ref, claim_scope_ref,
		enactability_predicate_ref, evidence_claim_ref, carrier_class_ref,
		verifier_identity, verifier_version, verification_policy_ref,
		verification_policy_digest, basis_ref, context_policy_ref,
		context_policy_digest, speech_act_intent_digest, canonical_json, recorded_at
	) VALUES (?, '/tmp/project-v43', ?, ?, 'permission:profile-v43',
		'speech-act:profile-v43', 'carrier:terminal-capture:profile-v43',
		'session:speech-act:v43', 'claim-scope:profile-v43',
		'A-permission-enactability:v43', 'E-profile-authority:v43',
		'carrier-class:controlling-terminal-capture:v43',
		'verifier:profile-authority:v43', 'verifier-version:v43',
		'verification-policy:profile-authority:v43', ?,
		'profile-authority-basis:v43', ?, ?, ?, ?, '2026-07-15T00:00:00Z')`,
		fixture.preparedDigest, fixture.contentRef, fixture.contentDigest,
		authorityBasisTestDigest("9"), profileAuthorityV2PolicyRef,
		authorityBasisTestDigest("a"), authorityBasisTestDigest("b"),
		fixture.preparedJSON)
	return fixture
}

func insertHistoricalProfileAuthorityContent(
	t *testing.T,
	database *sql.DB,
) (string, string) {
	t.Helper()
	digest := authorityBasisTestDigest("c")
	canonical := mustAuthorityBasisJSON(t, map[string]any{
		"schema":                                "haft.authority.profile-declaration-authorization-content/v1",
		"ref":                                   "authorization-content:historical-v38",
		"project_root":                          "/tmp/project-v43",
		"action_kind":                           profileAuthorityV2Action,
		"profile_author_role_assignment_ref":    "assignment:v43",
		"profile_author_role_assignment_digest": authorityBasisTestDigest("6"),
		"method_description_ref":                "method-description:onboard:v43",
		"method_description_digest":             authorityBasisTestDigest("1"),
		"method_contract_ref":                   "contract:onboard:v43",
		"method_contract_digest":                authorityBasisTestDigest("2"),
		"classifier_version":                    "classifier:v38",
		"policy_version":                        "profile-policy:v38",
		"session_ref":                           "session:future-work:v43",
		"allowed_work_from":                     "2026-07-15T01:00:00Z",
		"allowed_work_until":                    "2026-07-15T04:00:00Z",
		"basis_observation_from":                "2026-07-15T00:30:00Z",
		"basis_observation_until":               "2026-07-15T03:30:00Z",
		"authorization_valid_from":              "2026-07-15T00:00:00Z",
		"authorization_valid_until":             "2026-07-15T05:00:00Z",
		"single_use_key":                        "single-use:historical-v38",
	})
	mustExecProfileAuthorityV2(t, database, `INSERT INTO profile_declaration_authorization_contents (
		authorization_content_ref, authorization_content_digest, project_root,
		action_kind, profile_author_role_assignment_ref,
		profile_author_role_assignment_digest, method_description_ref,
		method_description_digest, method_contract_ref, method_contract_digest,
		classifier_version, policy_version, session_ref, allowed_work_from,
		allowed_work_until, basis_observation_from, basis_observation_until,
		authorization_valid_from, authorization_valid_until, single_use_key,
		canonical_json, recorded_at
	) VALUES ('authorization-content:historical-v38', ?, '/tmp/project-v43', ?,
		'assignment:v43', ?, 'method-description:onboard:v43', ?,
		'contract:onboard:v43', ?, 'classifier:v38', 'profile-policy:v38',
		'session:future-work:v43', '2026-07-15T01:00:00Z',
		'2026-07-15T04:00:00Z', '2026-07-15T00:30:00Z',
		'2026-07-15T03:30:00Z', '2026-07-15T00:00:00Z',
		'2026-07-15T05:00:00Z', 'single-use:historical-v38', ?,
		'2026-07-15T00:00:00Z')`, digest, profileAuthorityV2Action,
		authorityBasisTestDigest("6"), authorityBasisTestDigest("1"),
		authorityBasisTestDigest("2"), canonical)
	return digest, canonical
}

func assertProfileAuthorityV2PreTTYState(t testing.TB, database *sql.DB) {
	t.Helper()
	tables := []string{
		"terminal_capture_records",
		"speech_acts",
		profileAuthorityV2PermissionTable,
		profileAuthorityV2EffectTable,
		profileAuthorityV2BasisTable,
	}
	for _, table := range tables {
		var count int
		if err := database.QueryRow("SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table)).Scan(&count); err != nil {
			t.Fatalf("count pre-TTY table %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("pre-TTY preparation prematurely created %d row(s) in %s", count, table)
		}
	}
}

func openDatabaseBeforeMigration43(t testing.TB) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pre-v43.db")
	dsn, err := sqliteConnectionDSN(path)
	if err != nil {
		t.Fatalf("build pre-v43 DSN: %v", err)
	}
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open pre-v43 database: %v", err)
	}
	if _, err := database.Exec(schema); err != nil {
		_ = database.Close()
		t.Fatalf("install base schema: %v", err)
	}
	migrations := migrationsBeforeVersion(kernelMigrations, 43, 0, nil)
	if err := Migrate(database, "schema_version", migrations); err != nil {
		_ = database.Close()
		t.Fatalf("migrate through v42: %v", err)
	}
	return database
}

func profileAuthorityV2SQLiteSQL(
	t testing.TB,
	database *sql.DB,
	kind string,
	name string,
) string {
	t.Helper()
	var statement string
	if err := database.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type = ? AND name = ?",
		kind,
		name,
	).Scan(&statement); err != nil {
		t.Fatalf("read SQLite %s %s: %v", kind, name, err)
	}
	return statement
}

func mustExecProfileAuthorityV2(
	t testing.TB,
	database *sql.DB,
	statement string,
	arguments ...any,
) {
	t.Helper()
	if _, err := database.Exec(statement, arguments...); err != nil {
		t.Fatalf("execute profile-authority v2 fixture: %v", err)
	}
}

func assertProfileAuthorityV2ForeignKeysClean(t testing.TB, database *sql.DB) {
	t.Helper()
	rows, err := database.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatalf("run v43 foreign_key_check: %v", err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("v43 migration left a foreign-key violation")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read v43 foreign_key_check: %v", err)
	}
}
