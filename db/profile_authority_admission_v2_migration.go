package db

import (
	"fmt"
	"slices"
)

const (
	profileAuthorityV2ResolutionTable = "profile_declaration_authority_resolutions_v2"
	profileAuthorityV2UseTable        = "profile_declaration_authority_uses_v2"
	projectProfileV2AdmissionTable    = "project_profile_admissions_v2"
	projectProfileV2RevisionTable     = "project_profile_revisions_v2"
	projectProfileV2DebtTable         = "project_profile_projection_debt_v2"

	profileAuthorityV2ResolutionSchema = "haft.profile-authority.authority-resolution/v2"
	profileAuthorityV2UseSchema        = "haft.profile-authority.authority-use/v2"
	profileAuthorityV2RoleState        = "A.2.5.EnactableStateAdmission"
	profileAuthorityV2EnactableState   = "profile-declaration-permission-current"
)

var profileAuthorityAdmissionV2Migration44 = Migration{
	Version:     44,
	Description: "Add typed profile authority resolution and v2 profile admission ledger",
	Apply:       applyProfileAuthorityAdmissionV2Migration44,
}

var profileAuthorityAdmissionV2Tables = []string{
	profileAuthorityV2ResolutionTable,
	profileAuthorityV2UseTable,
	projectProfileV2AdmissionTable,
	projectProfileV2RevisionTable,
	projectProfileV2DebtTable,
}

var legacyProfileWriteTables44 = []string{
	"profile_declaration_authorization_contents",
	"profile_declaration_permissions",
	"speech_act_instituted_effects",
	"authority_basis_presentations",
	"authority_basis_resolutions",
	"authority_presentations",
	"authority_resolution_records",
	"authority_uses",
	"project_profile_admissions",
	"project_profile_revisions",
	"project_profile_projection_debt",
}

func applyProfileAuthorityAdmissionV2Migration44(
	tx MigrationTransaction,
	_ []Migration,
) error {
	if err := requireProfileAuthorityAdmissionV2Source44(tx); err != nil {
		return err
	}
	if err := requireAbsentProfileAuthorityAdmissionV2Footprint44(tx, 0); err != nil {
		return err
	}
	if err := requireProfileAuthorityAdmissionV2Dependencies44(tx, 0); err != nil {
		return err
	}
	statements := profileAuthorityAdmissionV2Statements44()
	if err := executeStatements(tx, statements, 0); err != nil {
		return fmt.Errorf("install profile authority admission v2: %w", err)
	}
	if err := verifyForeignKeys(tx); err != nil {
		return fmt.Errorf("verify profile authority admission v2: %w", err)
	}
	return nil
}

var profileAuthorityAdmissionV2Dependencies44 = []struct {
	kind string
	name string
}{
	{kind: "table", name: "profile_declaration_authorization_contents_v2"},
	{kind: "table", name: "profile_declaration_authorization_preparations_v2"},
	{kind: "table", name: "profile_declaration_permissions_v2"},
	{kind: "table", name: "profile_declaration_instituted_effects_v2"},
	{kind: "table", name: "profile_declaration_authority_bases_v2"},
	{kind: "trigger", name: "profile_declaration_permissions_v2_exact_sources"},
	{kind: "trigger", name: "profile_declaration_instituted_effects_v2_exact_sources"},
	{kind: "trigger", name: "profile_declaration_authority_bases_v2_exact_sources"},
	{kind: "table", name: "profile_onboarding_work_records"},
	{kind: "table", name: "profile_author_role_assignments"},
	{kind: "table", name: "observed_project_bases"},
	{kind: "table", name: "profile_onboarding_outcome_assessments"},
	{kind: "view", name: "current_project_profiles"},
}

func requireProfileAuthorityAdmissionV2Dependencies44(
	tx MigrationTransaction,
	index int,
) error {
	if index >= len(profileAuthorityAdmissionV2Dependencies44) {
		return nil
	}
	dependency := profileAuthorityAdmissionV2Dependencies44[index]
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ? AND sql IS NOT NULL",
		dependency.kind,
		dependency.name,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect v44 dependency %s %s: %w", dependency.kind, dependency.name, err)
	}
	if count != 1 {
		return fmt.Errorf(
			"profile authority admission v2 requires exact dependency %s %s",
			dependency.kind,
			dependency.name,
		)
	}
	return requireProfileAuthorityAdmissionV2Dependencies44(tx, index+1)
}

func requireProfileAuthorityAdmissionV2Source44(tx MigrationTransaction) error {
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = 43",
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect profile authority admission v2 source migration: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("profile authority admission v2 requires schema version 43")
	}
	return nil
}

func requireAbsentProfileAuthorityAdmissionV2Footprint44(
	tx MigrationTransaction,
	index int,
) error {
	if index >= len(profileAuthorityAdmissionV2Tables) {
		return nil
	}
	table := profileAuthorityAdmissionV2Tables[index]
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect profile authority admission v2 table %s: %w", table, err)
	}
	if count != 0 {
		return fmt.Errorf(
			"profile authority admission v2 refused: unversioned table %s already exists; unknown partial schema requires manual review",
			table,
		)
	}
	return requireAbsentProfileAuthorityAdmissionV2Footprint44(tx, index+1)
}

func profileAuthorityAdmissionV2Statements44() []string {
	statements := []string{
		profileAuthorityResolutionTable44(),
		projectProfileAdmissionTable44(),
		profileAuthorityUseTable44(),
		projectProfileRevisionTable44(),
		projectProfileProjectionDebtTable44(),
		profileAuthorityResolutionExactSourcesTrigger44(),
		profileAuthorityResolutionCrossGenerationTrigger44(),
		projectProfileAdmissionRevisionCASTrigger44(),
		projectProfileAdmissionExactSourcesTrigger44(),
		projectProfileAdmissionCrossGenerationTrigger44(),
		profileAuthorityUseExactSourcesTrigger44(),
		profileAuthorityUseCrossGenerationTrigger44(),
		projectProfileRevisionExactAdmissionTrigger44(),
		projectProfileRevisionCrossGenerationTrigger44(),
		projectProfileProjectionDebtExactAdmissionTrigger44(),
		projectProfileProjectionDebtCrossGenerationTrigger44(),
		"DROP VIEW current_project_profiles",
		currentProjectProfilesView44(),
		"CREATE INDEX idx_profile_authority_resolutions_v2_project_checked ON profile_declaration_authority_resolutions_v2(project_root, checked_at)",
		"CREATE INDEX idx_profile_authority_uses_v2_project_consumed ON profile_declaration_authority_uses_v2(project_root, consumed_at)",
		"CREATE INDEX idx_project_profile_admissions_v2_project_revision ON project_profile_admissions_v2(project_root, ledger_revision)",
		"CREATE INDEX idx_project_profile_revisions_v2_current ON project_profile_revisions_v2(project_root, ledger_revision DESC)",
		"CREATE INDEX idx_project_profile_projection_debt_v2_open ON project_profile_projection_debt_v2(debt_id, event_kind, recorded_at)",
	}
	immutable := []immutableAuthorityBasisTable{
		{name: profileAuthorityV2ResolutionTable, primaryKey: "authority_resolution_ref", digestColumn: "authority_resolution_digest"},
		{name: profileAuthorityV2UseTable, primaryKey: "use_ref", digestColumn: "use_digest"},
		{name: projectProfileV2AdmissionTable, primaryKey: "admission_id", digestColumn: "admission_digest"},
	}
	statements = appendAuthorityBasisTableTriggers(statements, immutable, 0)
	statements = appendProjectProfileV2AppendOnlyTriggers44(statements)
	statements = appendProfileAuthorityAdmissionV2RootGuards44(statements, 0)
	return appendLegacyProfileWriteSeals44(statements, 0)
}

func profileAuthorityResolutionTable44() string {
	return `CREATE TABLE profile_declaration_authority_resolutions_v2 (
		authority_resolution_ref TEXT PRIMARY KEY CHECK(authority_resolution_ref GLOB 'profile-authority-resolution:?*'),
		authority_resolution_digest TEXT NOT NULL UNIQUE CHECK(length(authority_resolution_digest) = 71 AND substr(authority_resolution_digest, 1, 7) = 'sha256:'),
		authority_basis_ref TEXT NOT NULL UNIQUE REFERENCES profile_declaration_authority_bases_v2(basis_ref),
		authority_basis_digest TEXT NOT NULL CHECK(length(authority_basis_digest) = 71 AND substr(authority_basis_digest, 1, 7) = 'sha256:'),
		speech_act_ref TEXT NOT NULL UNIQUE REFERENCES speech_acts(speech_act_ref),
		speech_act_digest TEXT NOT NULL CHECK(length(speech_act_digest) = 71 AND substr(speech_act_digest, 1, 7) = 'sha256:'),
		authorization_content_ref TEXT NOT NULL UNIQUE REFERENCES profile_declaration_authorization_contents_v2(authorization_content_ref),
		authorization_content_digest TEXT NOT NULL CHECK(length(authorization_content_digest) = 71 AND substr(authorization_content_digest, 1, 7) = 'sha256:'),
		permission_ref TEXT NOT NULL UNIQUE REFERENCES profile_declaration_permissions_v2(permission_ref),
		permission_digest TEXT NOT NULL CHECK(length(permission_digest) = 71 AND substr(permission_digest, 1, 7) = 'sha256:'),
		context_policy_ref TEXT NOT NULL REFERENCES speech_act_context_policies(context_policy_ref),
		context_policy_digest TEXT NOT NULL CHECK(length(context_policy_digest) = 71 AND substr(context_policy_digest, 1, 7) = 'sha256:'),
		action_kind TEXT NOT NULL CHECK(action_kind = '` + profileAuthorityV2Action + `'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		action_envelope_digest TEXT NOT NULL CHECK(length(action_envelope_digest) = 71 AND substr(action_envelope_digest, 1, 7) = 'sha256:'),
		project_binding_digest TEXT NOT NULL CHECK(length(project_binding_digest) = 71 AND substr(project_binding_digest, 1, 7) = 'sha256:'),
		profile_author_role_assignment_ref TEXT NOT NULL REFERENCES profile_author_role_assignments(role_assignment_ref),
		profile_author_role_assignment_digest TEXT NOT NULL CHECK(length(profile_author_role_assignment_digest) = 71 AND substr(profile_author_role_assignment_digest, 1, 7) = 'sha256:'),
		claim_scope_ref TEXT NOT NULL CHECK(claim_scope_ref != ''),
		bounded_context_ref TEXT NOT NULL CHECK(bounded_context_ref = '` + profileAuthorityV2ContextRef + `'),
		method_description_ref TEXT NOT NULL REFERENCES profile_onboarding_method_descriptions(method_description_ref),
		method_description_digest TEXT NOT NULL CHECK(length(method_description_digest) = 71 AND substr(method_description_digest, 1, 7) = 'sha256:'),
		method_contract_ref TEXT NOT NULL REFERENCES profile_onboarding_method_contracts(method_contract_ref),
		method_contract_digest TEXT NOT NULL CHECK(length(method_contract_digest) = 71 AND substr(method_contract_digest, 1, 7) = 'sha256:'),
		classifier_version TEXT NOT NULL CHECK(classifier_version != ''),
		policy_version TEXT NOT NULL CHECK(policy_version != ''),
		future_work_session_ref TEXT NOT NULL CHECK(future_work_session_ref != ''),
		allowed_work_from TEXT NOT NULL,
		allowed_work_until TEXT NOT NULL,
		basis_observation_from TEXT NOT NULL,
		basis_observation_until TEXT NOT NULL,
		authorization_valid_from TEXT NOT NULL,
		authorization_valid_until TEXT NOT NULL,
		permission_valid_from TEXT NOT NULL,
		permission_valid_until TEXT NOT NULL,
		single_use_key TEXT NOT NULL UNIQUE CHECK(single_use_key != ''),
		enactability_predicate_ref TEXT NOT NULL CHECK(enactability_predicate_ref GLOB 'A-?*'),
		verifier_identity TEXT NOT NULL CHECK(verifier_identity != ''),
		verifier_version TEXT NOT NULL CHECK(verifier_version != ''),
		verification_policy_ref TEXT NOT NULL CHECK(verification_policy_ref != ''),
		verification_policy_digest TEXT NOT NULL CHECK(length(verification_policy_digest) = 71 AND substr(verification_policy_digest, 1, 7) = 'sha256:'),
		checked_at TEXT NOT NULL,
		role_state_relation TEXT NOT NULL CHECK(role_state_relation = '` + profileAuthorityV2RoleState + `'),
		enactable_state TEXT NOT NULL CHECK(enactable_state = '` + profileAuthorityV2EnactableState + `'),
		currentness_result TEXT NOT NULL CHECK(currentness_result = 'current'),
		predicate_result TEXT NOT NULL CHECK(predicate_result = 'satisfied'),
		admission_result TEXT NOT NULL CHECK(admission_result = 'admitted'),
		canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
		recorded_at TEXT NOT NULL,
		CHECK(COALESCE(json_extract(canonical_json, '$.schema') = '` + profileAuthorityV2ResolutionSchema + `', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_resolution_ref') = authority_resolution_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_basis_ref') = authority_basis_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_basis_digest') = authority_basis_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.speech_act_ref') = speech_act_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.speech_act_digest') = speech_act_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authorization_content_ref') = authorization_content_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authorization_content_digest') = authorization_content_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.permission_ref') = permission_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.permission_digest') = permission_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.context_policy_ref') = context_policy_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.context_policy_digest') = context_policy_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.action_kind') = action_kind, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.project_root') = project_root, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.action_envelope_digest') = action_envelope_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.project_binding_digest') = project_binding_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.profile_author_role_assignment_ref') = profile_author_role_assignment_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.profile_author_role_assignment_digest') = profile_author_role_assignment_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.claim_scope_ref') = claim_scope_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.bounded_context_ref') = bounded_context_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.method_description_ref') = method_description_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.method_description_digest') = method_description_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.method_contract_ref') = method_contract_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.method_contract_digest') = method_contract_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.classifier_version') = classifier_version, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.policy_version') = policy_version, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.future_work_session_ref') = future_work_session_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.allowed_work_from') = allowed_work_from, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.allowed_work_until') = allowed_work_until, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.basis_observation_from') = basis_observation_from, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.basis_observation_until') = basis_observation_until, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authorization_valid_from') = authorization_valid_from, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authorization_valid_until') = authorization_valid_until, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.permission_valid_from') = permission_valid_from, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.permission_valid_until') = permission_valid_until, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.single_use_key') = single_use_key, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.enactability_predicate_ref') = enactability_predicate_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.verifier_identity') = verifier_identity, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.verifier_version') = verifier_version, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.verification_policy_ref') = verification_policy_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.verification_policy_digest') = verification_policy_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.checked_at') = checked_at, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.role_state_relation') = role_state_relation, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.enactable_state') = enactable_state, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.currentness_result') = currentness_result, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.predicate_result') = predicate_result, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.admission_result') = admission_result, 0)),
		CHECK(recorded_at = checked_at),
		CHECK(` + sqliteCanonicalUTCNanoShape("allowed_work_from") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("allowed_work_until") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("basis_observation_from") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("basis_observation_until") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("authorization_valid_from") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("authorization_valid_until") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("permission_valid_from") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("permission_valid_until") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("checked_at") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		CHECK(` + sqliteUTCNanoLess("allowed_work_from", "allowed_work_until") + `),
		CHECK(` + sqliteUTCNanoLess("basis_observation_from", "basis_observation_until") + `),
		CHECK(` + sqliteUTCNanoLess("authorization_valid_from", "authorization_valid_until") + `),
		CHECK(` + sqliteUTCNanoLess("permission_valid_from", "permission_valid_until") + `),
		CHECK(` + sqliteUTCNanoLessOrEqual("permission_valid_from", "checked_at") + `),
		CHECK(` + sqliteUTCNanoLess("checked_at", "permission_valid_until") + `)
	) WITHOUT ROWID`
}

func profileAuthorityUseTable44() string {
	return `CREATE TABLE profile_declaration_authority_uses_v2 (
		use_ref TEXT PRIMARY KEY CHECK(use_ref GLOB 'profile-authority-use:?*'),
		use_digest TEXT NOT NULL UNIQUE CHECK(length(use_digest) = 71 AND substr(use_digest, 1, 7) = 'sha256:'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		action_kind TEXT NOT NULL CHECK(action_kind = '` + profileAuthorityV2Action + `'),
		project_binding_digest TEXT NOT NULL CHECK(length(project_binding_digest) = 71 AND substr(project_binding_digest, 1, 7) = 'sha256:'),
		authority_resolution_ref TEXT NOT NULL UNIQUE REFERENCES profile_declaration_authority_resolutions_v2(authority_resolution_ref),
		authority_resolution_digest TEXT NOT NULL CHECK(length(authority_resolution_digest) = 71 AND substr(authority_resolution_digest, 1, 7) = 'sha256:'),
		authority_basis_ref TEXT NOT NULL UNIQUE REFERENCES profile_declaration_authority_bases_v2(basis_ref),
		authority_basis_digest TEXT NOT NULL CHECK(length(authority_basis_digest) = 71 AND substr(authority_basis_digest, 1, 7) = 'sha256:'),
		permission_ref TEXT NOT NULL UNIQUE REFERENCES profile_declaration_permissions_v2(permission_ref),
		permission_digest TEXT NOT NULL CHECK(length(permission_digest) = 71 AND substr(permission_digest, 1, 7) = 'sha256:'),
		authorization_content_ref TEXT NOT NULL UNIQUE REFERENCES profile_declaration_authorization_contents_v2(authorization_content_ref),
		authorization_content_digest TEXT NOT NULL CHECK(length(authorization_content_digest) = 71 AND substr(authorization_content_digest, 1, 7) = 'sha256:'),
		single_use_key TEXT NOT NULL UNIQUE CHECK(single_use_key != ''),
		admission_request_digest TEXT NOT NULL CHECK(length(admission_request_digest) = 71 AND substr(admission_request_digest, 1, 7) = 'sha256:'),
		committed_admission_ref TEXT NOT NULL UNIQUE REFERENCES project_profile_admissions_v2(admission_id),
		committed_admission_digest TEXT NOT NULL CHECK(length(committed_admission_digest) = 71 AND substr(committed_admission_digest, 1, 7) = 'sha256:'),
		canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
		consumed_at TEXT NOT NULL,
		recorded_at TEXT NOT NULL,
		CHECK(COALESCE(json_extract(canonical_json, '$.schema') = '` + profileAuthorityV2UseSchema + `', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.use_ref') = use_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.project_root') = project_root, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.action_kind') = action_kind, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.project_binding_digest') = project_binding_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_resolution_ref') = authority_resolution_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_resolution_digest') = authority_resolution_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_basis_ref') = authority_basis_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_basis_digest') = authority_basis_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.permission_ref') = permission_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.permission_digest') = permission_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authorization_content_ref') = authorization_content_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authorization_content_digest') = authorization_content_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.single_use_key') = single_use_key, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.admission_request_digest') = admission_request_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.committed_admission_ref') = committed_admission_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.committed_admission_digest') = committed_admission_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.consumed_at') = consumed_at, 0)),
		CHECK(recorded_at = consumed_at),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("consumed_at") + `)
	) WITHOUT ROWID`
}

func projectProfileAdmissionTable44() string {
	return `CREATE TABLE project_profile_admissions_v2 (
		admission_id TEXT PRIMARY KEY,
		action_kind TEXT NOT NULL CHECK(action_kind = '` + profileAuthorityV2Action + `'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		project_binding_digest TEXT NOT NULL CHECK(length(project_binding_digest) = 71 AND substr(project_binding_digest, 1, 7) = 'sha256:'),
		profile_payload_json TEXT NOT NULL CHECK(json_valid(profile_payload_json)),
		candidate_provenance_json TEXT NOT NULL CHECK(json_valid(candidate_provenance_json)),
		candidate_provenance_digest TEXT NOT NULL CHECK(length(candidate_provenance_digest) = 71 AND substr(candidate_provenance_digest, 1, 7) = 'sha256:'),
		profile_author_role_assignment_ref TEXT NOT NULL REFERENCES profile_author_role_assignments(role_assignment_ref),
		profile_author_role_assignment_digest TEXT NOT NULL CHECK(length(profile_author_role_assignment_digest) = 71 AND substr(profile_author_role_assignment_digest, 1, 7) = 'sha256:'),
		profile_payload_digest TEXT NOT NULL CHECK(length(profile_payload_digest) = 71 AND substr(profile_payload_digest, 1, 7) = 'sha256:'),
		observed_project_basis_ref TEXT NOT NULL REFERENCES observed_project_bases(observed_project_basis_ref),
		observed_project_basis_digest TEXT NOT NULL CHECK(length(observed_project_basis_digest) = 71 AND substr(observed_project_basis_digest, 1, 7) = 'sha256:'),
		work_record_ref TEXT NOT NULL REFERENCES profile_onboarding_work_records(work_record_ref),
		work_record_digest TEXT NOT NULL CHECK(length(work_record_digest) = 71 AND substr(work_record_digest, 1, 7) = 'sha256:'),
		outcome_assessment_ref TEXT NOT NULL REFERENCES profile_onboarding_outcome_assessments(outcome_assessment_ref),
		outcome_assessment_digest TEXT NOT NULL CHECK(length(outcome_assessment_digest) = 71 AND substr(outcome_assessment_digest, 1, 7) = 'sha256:'),
		authority_basis_ref TEXT NOT NULL REFERENCES profile_declaration_authority_bases_v2(basis_ref),
		authority_basis_digest TEXT NOT NULL CHECK(length(authority_basis_digest) = 71 AND substr(authority_basis_digest, 1, 7) = 'sha256:'),
		authority_resolution_ref TEXT NOT NULL UNIQUE REFERENCES profile_declaration_authority_resolutions_v2(authority_resolution_ref),
		authority_resolution_digest TEXT NOT NULL CHECK(length(authority_resolution_digest) = 71 AND substr(authority_resolution_digest, 1, 7) = 'sha256:'),
		receipt_json TEXT NOT NULL UNIQUE CHECK(json_valid(receipt_json)),
		receipt_digest TEXT NOT NULL UNIQUE CHECK(length(receipt_digest) = 71 AND substr(receipt_digest, 1, 7) = 'sha256:'),
		expected_ledger_revision INTEGER NOT NULL CHECK(expected_ledger_revision >= 0 AND expected_ledger_revision < 9223372036854775807),
		ledger_revision INTEGER NOT NULL CHECK(ledger_revision = expected_ledger_revision + 1),
		single_use_key TEXT NOT NULL UNIQUE CHECK(single_use_key != ''),
		admission_request_digest TEXT NOT NULL CHECK(length(admission_request_digest) = 71 AND substr(admission_request_digest, 1, 7) = 'sha256:'),
		admission_json TEXT NOT NULL UNIQUE CHECK(json_valid(admission_json)),
		admission_digest TEXT NOT NULL UNIQUE CHECK(length(admission_digest) = 71 AND substr(admission_digest, 1, 7) = 'sha256:'),
		recorded_at TEXT NOT NULL,
		UNIQUE(project_root, ledger_revision),
		CHECK(COALESCE(json_extract(receipt_json, '$.schema') = 'haft.project-profile.declaration-receipt/v1', 0)),
		CHECK(COALESCE(json_extract(receipt_json, '$.authority_resolution_record_ref') = authority_resolution_ref, 0)),
		CHECK(COALESCE(json_extract(receipt_json, '$.authority_resolution_record_digest') = authority_resolution_digest, 0)),
		CHECK(COALESCE(json_extract(receipt_json, '$.authority_basis_ref') = authority_basis_ref, 0)),
		CHECK(COALESCE(json_extract(receipt_json, '$.work_record_ref') = work_record_ref, 0)),
		CHECK(COALESCE(json_extract(receipt_json, '$.candidate_provenance_digest') = candidate_provenance_digest, 0)),
		CHECK(COALESCE(json_extract(receipt_json, '$.payload_digest') = profile_payload_digest, 0)),
		CHECK(COALESCE(json_extract(receipt_json, '$.observed_basis_digest') = observed_project_basis_digest, 0)),
		CHECK(COALESCE(json_extract(receipt_json, '$.ledger_revision') = ledger_revision, 0)),
		CHECK(COALESCE(json_extract(receipt_json, '$.recorded_at') = recorded_at, 0)),
		CHECK(COALESCE(json_extract(admission_json, '$.schema') = 'haft.project-profile.admission-record/v1', 0)),
		CHECK(COALESCE(json_extract(admission_json, '$.admission_record_ref') = admission_id, 0)),
		CHECK(COALESCE(json_extract(admission_json, '$.classification_work_record_ref') = work_record_ref, 0)),
		CHECK(COALESCE(json_extract(admission_json, '$.authority_basis_ref') = authority_basis_ref, 0)),
		CHECK(COALESCE(json_extract(admission_json, '$.authority_resolution_record_ref') = authority_resolution_ref, 0)),
		CHECK(COALESCE(json_extract(admission_json, '$.authority_resolution_record_digest') = authority_resolution_digest, 0)),
		CHECK(COALESCE(json_extract(admission_json, '$.expected_ledger_revision') = expected_ledger_revision, 0)),
		CHECK(COALESCE(json_extract(admission_json, '$.committed_ledger_revision') = ledger_revision, 0)),
		CHECK(COALESCE(json_extract(admission_json, '$.single_use_key') = single_use_key, 0)),
		CHECK(COALESCE(json_extract(admission_json, '$.committed_at') = recorded_at, 0)),
		CHECK(json(json_extract(admission_json, '$.payload')) = json(profile_payload_json)),
		CHECK(json(json_extract(admission_json, '$.candidate_provenance')) = json(candidate_provenance_json)),
		CHECK(json(json_extract(admission_json, '$.receipt')) = json(receipt_json)),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `)
	) WITHOUT ROWID`
}

func projectProfileRevisionTable44() string {
	return `CREATE TABLE project_profile_revisions_v2 (
		project_root TEXT NOT NULL CHECK(project_root != ''),
		ledger_revision INTEGER NOT NULL CHECK(ledger_revision > 0),
		configured_profile_kind TEXT NOT NULL CHECK(configured_profile_kind = 'Declared'),
		profile_payload_json TEXT NOT NULL CHECK(json_valid(profile_payload_json)),
		profile_payload_digest TEXT NOT NULL CHECK(length(profile_payload_digest) = 71 AND substr(profile_payload_digest, 1, 7) = 'sha256:'),
		receipt_json TEXT NOT NULL CHECK(json_valid(receipt_json)),
		receipt_digest TEXT NOT NULL UNIQUE CHECK(length(receipt_digest) = 71 AND substr(receipt_digest, 1, 7) = 'sha256:'),
		admission_id TEXT NOT NULL UNIQUE REFERENCES project_profile_admissions_v2(admission_id),
		admission_digest TEXT NOT NULL UNIQUE CHECK(length(admission_digest) = 71 AND substr(admission_digest, 1, 7) = 'sha256:'),
		recorded_at TEXT NOT NULL,
		PRIMARY KEY(project_root, ledger_revision),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `)
	) WITHOUT ROWID`
}

func projectProfileProjectionDebtTable44() string {
	return `CREATE TABLE project_profile_projection_debt_v2 (
		event_id TEXT PRIMARY KEY,
		debt_id TEXT NOT NULL CHECK(debt_id != ''),
		profile_revision_generation TEXT NOT NULL CHECK(profile_revision_generation IN ('v1', 'v2')),
		admission_id TEXT NOT NULL,
		admission_digest TEXT NOT NULL CHECK(length(admission_digest) = 71 AND substr(admission_digest, 1, 7) = 'sha256:'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		ledger_revision INTEGER NOT NULL CHECK(ledger_revision > 0),
		profile_payload_digest TEXT NOT NULL CHECK(length(profile_payload_digest) = 71 AND substr(profile_payload_digest, 1, 7) = 'sha256:'),
		projection_path TEXT NOT NULL CHECK(projection_path != ''),
		event_kind TEXT NOT NULL CHECK(event_kind IN ('opened', 'resolved')),
		reason_code TEXT NOT NULL CHECK(reason_code != ''),
		detail TEXT NOT NULL,
		expected_projection_digest TEXT NOT NULL CHECK(length(expected_projection_digest) = 71 AND substr(expected_projection_digest, 1, 7) = 'sha256:'),
		observed_projection_digest TEXT NOT NULL DEFAULT '' CHECK(observed_projection_digest = '' OR (length(observed_projection_digest) = 71 AND substr(observed_projection_digest, 1, 7) = 'sha256:')),
		supersedes_event_generation TEXT CHECK(supersedes_event_generation IS NULL OR supersedes_event_generation IN ('v1', 'v2')),
		supersedes_event_id TEXT,
		recorded_at TEXT NOT NULL,
		CHECK((event_kind = 'opened' AND supersedes_event_generation IS NULL AND supersedes_event_id IS NULL) OR (event_kind = 'resolved' AND supersedes_event_generation IS NOT NULL AND supersedes_event_id IS NOT NULL)),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `)
	) WITHOUT ROWID`
}

func profileAuthorityResolutionExactSourcesTrigger44() string {
	return `CREATE TRIGGER profile_declaration_authority_resolutions_v2_exact_sources
	 BEFORE INSERT ON profile_declaration_authority_resolutions_v2
	 WHEN NOT EXISTS (
		SELECT 1
		FROM profile_declaration_authority_bases_v2 basis
		JOIN profile_declaration_authorization_contents_v2 content
			ON content.authorization_content_ref = basis.authorization_content_ref
		JOIN profile_declaration_permissions_v2 permission
			ON permission.permission_ref = basis.permission_ref
		JOIN profile_declaration_instituted_effects_v2 effect
			ON effect.effect_digest = basis.instituted_effect_digest
		JOIN speech_acts act ON act.speech_act_ref = basis.speech_act_ref
		JOIN speech_act_context_policies policy ON policy.context_policy_ref = basis.context_policy_ref
		JOIN profile_author_role_assignments assignment
			ON assignment.role_assignment_ref = content.profile_author_role_assignment_ref
		JOIN profile_onboarding_method_descriptions description
			ON description.method_description_ref = content.method_description_ref
		JOIN profile_onboarding_method_contracts contract
			ON contract.method_contract_ref = content.method_contract_ref
		JOIN project_ledger_binding binding ON binding.project_root = content.project_root
		WHERE basis.basis_ref = NEW.authority_basis_ref
		AND basis.basis_digest = NEW.authority_basis_digest
		AND basis.project_root = NEW.project_root
		AND basis.speech_act_ref = NEW.speech_act_ref
		AND basis.speech_act_digest = NEW.speech_act_digest
		AND basis.authorization_content_ref = NEW.authorization_content_ref
		AND basis.authorization_content_digest = NEW.authorization_content_digest
		AND basis.permission_ref = NEW.permission_ref
		AND basis.permission_digest = NEW.permission_digest
		AND basis.context_policy_ref = NEW.context_policy_ref
		AND basis.context_policy_digest = NEW.context_policy_digest
		AND content.authorization_content_digest = NEW.authorization_content_digest
		AND content.action_kind = NEW.action_kind
		AND content.project_root = NEW.project_root
		AND content.profile_author_role_assignment_ref = NEW.profile_author_role_assignment_ref
		AND content.profile_author_role_assignment_digest = NEW.profile_author_role_assignment_digest
		AND content.method_description_ref = NEW.method_description_ref
		AND content.method_description_digest = NEW.method_description_digest
		AND content.method_contract_ref = NEW.method_contract_ref
		AND content.method_contract_digest = NEW.method_contract_digest
		AND content.classifier_version = NEW.classifier_version
		AND content.policy_version = NEW.policy_version
		AND content.session_ref = NEW.future_work_session_ref
		AND content.allowed_work_from = NEW.allowed_work_from
		AND content.allowed_work_until = NEW.allowed_work_until
		AND content.basis_observation_from = NEW.basis_observation_from
		AND content.basis_observation_until = NEW.basis_observation_until
		AND content.authorization_valid_from = NEW.authorization_valid_from
		AND content.authorization_valid_until = NEW.authorization_valid_until
		AND content.single_use_key = NEW.single_use_key
		AND permission.permission_digest = NEW.permission_digest
		AND permission.project_root = NEW.project_root
		AND permission.action_kind = NEW.action_kind
		AND permission.subject_ref = NEW.profile_author_role_assignment_ref
		AND permission.subject_digest = NEW.profile_author_role_assignment_digest
		AND permission.claim_scope_ref = NEW.claim_scope_ref
		AND permission.bounded_context_ref = NEW.bounded_context_ref
		AND permission.method_description_ref = NEW.method_description_ref
		AND permission.method_description_digest = NEW.method_description_digest
		AND permission.authorization_content_ref = NEW.authorization_content_ref
		AND permission.authorization_content_digest = NEW.authorization_content_digest
		AND permission.source_speech_act_ref = NEW.speech_act_ref
		AND permission.source_speech_act_digest = NEW.speech_act_digest
		AND permission.context_policy_ref = NEW.context_policy_ref
		AND permission.context_policy_digest = NEW.context_policy_digest
		AND permission.valid_from = NEW.permission_valid_from
		AND permission.valid_until = NEW.permission_valid_until
		AND permission.enactability_predicate_ref = NEW.enactability_predicate_ref
		AND permission.adjudication_verifier_identity = NEW.verifier_identity
		AND permission.adjudication_verifier_version = NEW.verifier_version
		AND permission.adjudication_evaluation_policy_ref = NEW.verification_policy_ref
		AND permission.adjudication_evaluation_policy_digest = NEW.verification_policy_digest
		AND effect.project_root = NEW.project_root
		AND effect.speech_act_ref = NEW.speech_act_ref
		AND effect.speech_act_digest = NEW.speech_act_digest
		AND effect.permission_ref = NEW.permission_ref
		AND effect.permission_digest = NEW.permission_digest
		AND act.speech_act_digest = NEW.speech_act_digest
		AND act.project_root = NEW.project_root
		AND policy.context_policy_digest = NEW.context_policy_digest
		AND assignment.role_assignment_digest = NEW.profile_author_role_assignment_digest
		AND description.method_description_digest = NEW.method_description_digest
		AND contract.method_contract_digest = NEW.method_contract_digest
		AND contract.method_description_ref = NEW.method_description_ref
		AND contract.method_description_digest = NEW.method_description_digest
		AND binding.project_root = NEW.project_root
		AND NEW.role_state_relation = '` + profileAuthorityV2RoleState + `'
		AND NEW.enactable_state = '` + profileAuthorityV2EnactableState + `'
		AND NEW.currentness_result = 'current'
		AND NEW.predicate_result = 'satisfied'
		AND NEW.admission_result = 'admitted'
		AND ` + sqliteUTCNanoLessOrEqual("act.window_until", "NEW.checked_at") + `
		AND ` + sqliteUTCNanoLessOrEqual("permission.valid_from", "NEW.checked_at") + `
		AND ` + sqliteUTCNanoLess("NEW.checked_at", "permission.valid_until") + `
	 ) BEGIN
		SELECT RAISE(ABORT, 'profile authority resolution does not bind the exact v43 closure, project binding, and pre-Work A.2.5 Permission-enactability evaluation');
	 END`
}

func profileAuthorityResolutionCrossGenerationTrigger44() string {
	return `CREATE TRIGGER profile_declaration_authority_resolutions_v2_no_cross_generation_collision
	 BEFORE INSERT ON profile_declaration_authority_resolutions_v2
	 WHEN EXISTS (
		SELECT 1 FROM authority_resolution_records legacy
		WHERE legacy.authority_resolution_id = NEW.authority_resolution_ref
		OR legacy.authority_resolution_digest = NEW.authority_resolution_digest
	 ) OR EXISTS (
		SELECT 1 FROM authority_basis_resolutions legacy
		WHERE legacy.authority_resolution_id = NEW.authority_resolution_ref
		OR legacy.authority_resolution_digest = NEW.authority_resolution_digest
	 ) OR EXISTS (
		SELECT 1 FROM authority_uses legacy WHERE legacy.single_use_key = NEW.single_use_key
	 ) BEGIN
		SELECT RAISE(ABORT, 'profile authority resolution collides with legacy authority history or a consumed single-use key');
	 END`
}

func projectProfileAdmissionRevisionCASTrigger44() string {
	return `CREATE TRIGGER project_profile_admissions_v2_revision_cas
	 BEFORE INSERT ON project_profile_admissions_v2
	 WHEN NOT EXISTS (
		SELECT 1
		FROM (
			SELECT COUNT(*) AS revision_count,
				COUNT(DISTINCT ledger_revision) AS distinct_count,
				COALESCE(MIN(ledger_revision), 0) AS minimum_revision,
				COALESCE(MAX(ledger_revision), 0) AS maximum_revision
			FROM (
				SELECT ledger_revision FROM project_profile_revisions legacy WHERE legacy.project_root = NEW.project_root
				UNION ALL
				SELECT ledger_revision FROM project_profile_revisions_v2 current WHERE current.project_root = NEW.project_root
			)
		) chain
		WHERE (NEW.expected_ledger_revision = 0 AND chain.revision_count = 0)
		OR (
			NEW.expected_ledger_revision > 0
			AND chain.revision_count = NEW.expected_ledger_revision
			AND chain.distinct_count = chain.revision_count
			AND chain.minimum_revision = 1
			AND chain.maximum_revision = NEW.expected_ledger_revision
		)
	 ) BEGIN
		SELECT RAISE(ABORT, 'project profile cross-generation ledger revision conflict');
	 END`
}

func projectProfileAdmissionExactSourcesTrigger44() string {
	return `CREATE TRIGGER project_profile_admissions_v2_exact_sources
	 BEFORE INSERT ON project_profile_admissions_v2
	 WHEN NOT EXISTS (
		SELECT 1
		FROM profile_declaration_authority_resolutions_v2 resolution
		JOIN profile_declaration_authority_bases_v2 authority_basis
			ON authority_basis.basis_ref = resolution.authority_basis_ref
		JOIN profile_declaration_authorization_contents_v2 content
			ON content.authorization_content_ref = authority_basis.authorization_content_ref
		JOIN profile_declaration_permissions_v2 permission
			ON permission.permission_ref = authority_basis.permission_ref
		JOIN profile_onboarding_work_records work_record
			ON work_record.work_record_ref = NEW.work_record_ref
		JOIN profile_author_role_assignments assignment
			ON assignment.role_assignment_ref = NEW.profile_author_role_assignment_ref
		JOIN observed_project_bases observed_basis
			ON observed_basis.observed_project_basis_ref = NEW.observed_project_basis_ref
		JOIN profile_onboarding_outcome_assessments assessment
			ON assessment.outcome_assessment_ref = NEW.outcome_assessment_ref
		JOIN profile_onboarding_effects work_effect ON work_effect.effect_ref = assessment.effect_ref
		WHERE resolution.authority_resolution_ref = NEW.authority_resolution_ref
		AND resolution.authority_resolution_digest = NEW.authority_resolution_digest
		AND resolution.authority_basis_ref = NEW.authority_basis_ref
		AND resolution.authority_basis_digest = NEW.authority_basis_digest
		AND resolution.project_root = NEW.project_root
		AND resolution.action_kind = NEW.action_kind
		AND resolution.project_binding_digest = NEW.project_binding_digest
		AND resolution.profile_author_role_assignment_ref = NEW.profile_author_role_assignment_ref
		AND resolution.profile_author_role_assignment_digest = NEW.profile_author_role_assignment_digest
		AND resolution.single_use_key = NEW.single_use_key
		AND resolution.currentness_result = 'current'
		AND resolution.predicate_result = 'satisfied'
		AND resolution.admission_result = 'admitted'
		AND authority_basis.basis_digest = NEW.authority_basis_digest
		AND authority_basis.project_root = NEW.project_root
		AND content.authorization_content_digest = resolution.authorization_content_digest
		AND content.project_root = NEW.project_root
		AND permission.permission_digest = resolution.permission_digest
		AND permission.project_root = NEW.project_root
		AND permission.valid_until = resolution.permission_valid_until
		AND work_record.work_record_digest = NEW.work_record_digest
		AND work_record.project_root = NEW.project_root
		AND work_record.method_description_ref = resolution.method_description_ref
		AND work_record.method_description_digest = resolution.method_description_digest
		AND work_record.method_contract_ref = resolution.method_contract_ref
		AND work_record.method_contract_digest = resolution.method_contract_digest
		AND work_record.performed_by_role_assignment_ref = NEW.profile_author_role_assignment_ref
		AND work_record.profile_author_role_assignment_ref = NEW.profile_author_role_assignment_ref
		AND work_record.profile_author_role_assignment_digest = NEW.profile_author_role_assignment_digest
		AND work_record.executed_within_ref = assignment.holder_system_ref
		AND work_record.bounded_context_ref = assignment.bounded_context_ref
		AND work_record.observed_project_basis_ref = NEW.observed_project_basis_ref
		AND work_record.observed_project_basis_digest = NEW.observed_project_basis_digest
		AND work_record.outcome_kind = 'CandidatePayloadProduced'
		AND work_record.profile_payload_digest = NEW.profile_payload_digest
		AND work_record.observed_basis_digest = NEW.observed_project_basis_digest
		AND assignment.role_assignment_digest = NEW.profile_author_role_assignment_digest
		AND observed_basis.observed_project_basis_digest = NEW.observed_project_basis_digest
		AND observed_basis.project_root = NEW.project_root
		AND observed_basis.observation_from = work_record.basis_observation_from
		AND observed_basis.observation_until = work_record.basis_observation_until
		AND assessment.outcome_assessment_digest = NEW.outcome_assessment_digest
		AND assessment.work_record_ref = work_record.work_record_ref
		AND assessment.work_record_digest = work_record.work_record_digest
		AND assessment.verdict_kind = 'passed'
		AND work_effect.effect_digest = assessment.effect_digest
		AND work_effect.work_record_ref = work_record.work_record_ref
		AND work_effect.work_record_digest = work_record.work_record_digest
		AND work_effect.result_kind = 'CandidatePayloadProduced'
		AND work_effect.profile_payload_digest = NEW.profile_payload_digest
		AND work_effect.observed_project_basis_ref = NEW.observed_project_basis_ref
		AND work_effect.observed_project_basis_digest = NEW.observed_project_basis_digest
		AND ` + sqliteUTCNanoLessOrEqual("resolution.checked_at", "work_record.work_from") + `
		AND ` + sqliteUTCNanoLessOrEqual("resolution.allowed_work_from", "work_record.work_from") + `
		AND ` + sqliteUTCNanoLessOrEqual("work_record.work_until", "resolution.allowed_work_until") + `
		AND ` + sqliteUTCNanoLessOrEqual("resolution.basis_observation_from", "work_record.basis_observation_from") + `
		AND ` + sqliteUTCNanoLessOrEqual("work_record.basis_observation_until", "resolution.basis_observation_until") + `
		AND ` + sqliteUTCNanoLessOrEqual("assignment.valid_from", "work_record.work_from") + `
		AND ` + sqliteUTCNanoLessOrEqual("work_record.work_until", "assignment.valid_until") + `
		AND ` + sqliteUTCNanoLessOrEqual("work_record.work_until", "NEW.recorded_at") + `
		AND ` + sqliteUTCNanoLess("NEW.recorded_at", "resolution.permission_valid_until") + `
		AND json_extract(NEW.candidate_provenance_json, '$.authority_basis_ref') = NEW.authority_basis_ref
		AND json_extract(NEW.candidate_provenance_json, '$.work_record_ref') = NEW.work_record_ref
		AND json_extract(NEW.candidate_provenance_json, '$.work_record_digest') = NEW.work_record_digest
		AND json_extract(NEW.candidate_provenance_json, '$.profile_author_role_assignment_ref') = NEW.profile_author_role_assignment_ref
		AND json_extract(NEW.candidate_provenance_json, '$.profile_author_role_assignment_digest') = NEW.profile_author_role_assignment_digest
		AND json_extract(NEW.candidate_provenance_json, '$.observed_project_basis_ref') = NEW.observed_project_basis_ref
		AND json_extract(NEW.candidate_provenance_json, '$.observed_project_basis_digest') = NEW.observed_project_basis_digest
		AND json_extract(NEW.candidate_provenance_json, '$.outcome_assessment_ref') = NEW.outcome_assessment_ref
		AND json_extract(NEW.candidate_provenance_json, '$.outcome_assessment_digest') = NEW.outcome_assessment_digest
		AND json_extract(NEW.candidate_provenance_json, '$.project_root') = NEW.project_root
		AND json_extract(NEW.candidate_provenance_json, '$.classifier_version') = resolution.classifier_version
		AND json_extract(NEW.candidate_provenance_json, '$.policy_version') = resolution.policy_version
		AND json_extract(NEW.candidate_provenance_json, '$.session_ref') = resolution.future_work_session_ref
		AND json_extract(NEW.candidate_provenance_json, '$.payload_digest') = NEW.profile_payload_digest
		AND json_extract(NEW.candidate_provenance_json, '$.provenance_digest') = NEW.candidate_provenance_digest
	 ) BEGIN
		SELECT RAISE(ABORT, 'v2 profile admission does not bind exact authority, Work, RoleAssignment, observed basis, assessment, and provenance');
	 END`
}

func projectProfileAdmissionCrossGenerationTrigger44() string {
	return `CREATE TRIGGER project_profile_admissions_v2_no_cross_generation_collision
	 BEFORE INSERT ON project_profile_admissions_v2
	 WHEN EXISTS (
		SELECT 1 FROM project_profile_admissions legacy
		WHERE legacy.admission_id = NEW.admission_id
		OR legacy.admission_digest = NEW.admission_digest
		OR legacy.receipt_digest = NEW.receipt_digest
		OR legacy.authority_resolution_ref = NEW.authority_resolution_ref
		OR legacy.single_use_key = NEW.single_use_key
		OR (legacy.project_root = NEW.project_root AND legacy.ledger_revision = NEW.ledger_revision)
	 ) OR EXISTS (
		SELECT 1 FROM authority_uses legacy WHERE legacy.single_use_key = NEW.single_use_key
	 ) OR EXISTS (
		SELECT 1 FROM profile_declaration_authority_uses_v2 current WHERE current.single_use_key = NEW.single_use_key
	 ) BEGIN
		SELECT RAISE(ABORT, 'v2 profile admission collides with cross-generation profile history or consumed authority');
	 END`
}

func profileAuthorityUseExactSourcesTrigger44() string {
	return `CREATE TRIGGER profile_declaration_authority_uses_v2_exact_sources
	 BEFORE INSERT ON profile_declaration_authority_uses_v2
	 WHEN NOT EXISTS (
		SELECT 1
		FROM profile_declaration_authority_resolutions_v2 resolution
		JOIN profile_declaration_authority_bases_v2 basis ON basis.basis_ref = resolution.authority_basis_ref
		JOIN profile_declaration_permissions_v2 permission ON permission.permission_ref = resolution.permission_ref
		JOIN profile_declaration_authorization_contents_v2 content ON content.authorization_content_ref = resolution.authorization_content_ref
		JOIN project_profile_admissions_v2 admission ON admission.admission_id = NEW.committed_admission_ref
		JOIN profile_onboarding_work_records work_record ON work_record.work_record_ref = admission.work_record_ref
		WHERE resolution.authority_resolution_ref = NEW.authority_resolution_ref
		AND resolution.authority_resolution_digest = NEW.authority_resolution_digest
		AND resolution.project_root = NEW.project_root
		AND resolution.action_kind = NEW.action_kind
		AND resolution.project_binding_digest = NEW.project_binding_digest
		AND resolution.authority_basis_ref = NEW.authority_basis_ref
		AND resolution.authority_basis_digest = NEW.authority_basis_digest
		AND resolution.permission_ref = NEW.permission_ref
		AND resolution.permission_digest = NEW.permission_digest
		AND resolution.authorization_content_ref = NEW.authorization_content_ref
		AND resolution.authorization_content_digest = NEW.authorization_content_digest
		AND resolution.single_use_key = NEW.single_use_key
		AND basis.basis_digest = NEW.authority_basis_digest
		AND permission.permission_digest = NEW.permission_digest
		AND content.authorization_content_digest = NEW.authorization_content_digest
		AND admission.admission_digest = NEW.committed_admission_digest
		AND admission.authority_resolution_ref = NEW.authority_resolution_ref
		AND admission.authority_resolution_digest = NEW.authority_resolution_digest
		AND admission.authority_basis_ref = NEW.authority_basis_ref
		AND admission.authority_basis_digest = NEW.authority_basis_digest
		AND admission.single_use_key = NEW.single_use_key
		AND admission.action_kind = NEW.action_kind
		AND admission.project_root = NEW.project_root
		AND admission.project_binding_digest = NEW.project_binding_digest
		AND admission.admission_request_digest = NEW.admission_request_digest
		AND admission.recorded_at = NEW.consumed_at
		AND work_record.work_record_digest = admission.work_record_digest
		AND ` + sqliteUTCNanoLessOrEqual("resolution.checked_at", "work_record.work_from") + `
		AND ` + sqliteUTCNanoLessOrEqual("work_record.work_until", "NEW.consumed_at") + `
		AND ` + sqliteUTCNanoLess("NEW.consumed_at", "resolution.permission_valid_until") + `
	 ) BEGIN
		SELECT RAISE(ABORT, 'profile authority use does not bind exact v2 resolution, closure, and committed admission');
	 END`
}

func profileAuthorityUseCrossGenerationTrigger44() string {
	return `CREATE TRIGGER profile_declaration_authority_uses_v2_no_cross_generation_collision
	 BEFORE INSERT ON profile_declaration_authority_uses_v2
	 WHEN EXISTS (
		SELECT 1 FROM authority_uses legacy
		WHERE legacy.use_id = NEW.use_ref
		OR legacy.authority_resolution_ref = NEW.authority_resolution_ref
		OR legacy.single_use_key = NEW.single_use_key
		OR legacy.committed_result_ref = NEW.committed_admission_ref
	 ) OR EXISTS (
		SELECT 1 FROM project_profile_admissions legacy
		WHERE legacy.admission_id = NEW.committed_admission_ref
		OR legacy.single_use_key = NEW.single_use_key
	 ) BEGIN
		SELECT RAISE(ABORT, 'profile authority use collides with legacy authority or admission history');
	 END`
}

func projectProfileRevisionExactAdmissionTrigger44() string {
	return `CREATE TRIGGER project_profile_revisions_v2_exact_admission
	 BEFORE INSERT ON project_profile_revisions_v2
	 WHEN NOT EXISTS (
		SELECT 1
		FROM project_profile_admissions_v2 admission
		JOIN profile_declaration_authority_uses_v2 authority_use
			ON authority_use.committed_admission_ref = admission.admission_id
		WHERE admission.admission_id = NEW.admission_id
		AND admission.admission_digest = NEW.admission_digest
		AND admission.project_root = NEW.project_root
		AND admission.ledger_revision = NEW.ledger_revision
		AND admission.profile_payload_json = NEW.profile_payload_json
		AND admission.profile_payload_digest = NEW.profile_payload_digest
		AND admission.receipt_json = NEW.receipt_json
		AND admission.receipt_digest = NEW.receipt_digest
		AND admission.recorded_at = NEW.recorded_at
		AND authority_use.committed_admission_digest = admission.admission_digest
		AND authority_use.authority_resolution_ref = admission.authority_resolution_ref
		AND authority_use.authority_resolution_digest = admission.authority_resolution_digest
		AND authority_use.single_use_key = admission.single_use_key
		AND authority_use.action_kind = admission.action_kind
		AND authority_use.project_root = admission.project_root
		AND authority_use.project_binding_digest = admission.project_binding_digest
		AND authority_use.admission_request_digest = admission.admission_request_digest
		AND authority_use.consumed_at = admission.recorded_at
	 ) BEGIN
		SELECT RAISE(ABORT, 'v2 project profile revision does not match canonical admission and authority use');
	 END`
}

func projectProfileRevisionCrossGenerationTrigger44() string {
	return `CREATE TRIGGER project_profile_revisions_v2_no_cross_generation_collision
	 BEFORE INSERT ON project_profile_revisions_v2
	 WHEN EXISTS (
		SELECT 1 FROM project_profile_revisions legacy
		WHERE (legacy.project_root = NEW.project_root AND legacy.ledger_revision = NEW.ledger_revision)
		OR legacy.admission_id = NEW.admission_id
		OR legacy.admission_digest = NEW.admission_digest
		OR legacy.receipt_digest = NEW.receipt_digest
	 ) BEGIN
		SELECT RAISE(ABORT, 'v2 project profile revision collides with legacy history');
	 END`
}

func projectProfileProjectionDebtExactAdmissionTrigger44() string {
	return `CREATE TRIGGER project_profile_projection_debt_v2_exact_admission
	 BEFORE INSERT ON project_profile_projection_debt_v2
	 WHEN NOT (
		(NEW.profile_revision_generation = 'v1' AND EXISTS (
			SELECT 1
			FROM project_profile_admissions admission
			JOIN project_profile_revisions revision ON revision.admission_id = admission.admission_id
			JOIN authority_uses authority_use ON authority_use.committed_result_ref = admission.admission_id
			WHERE admission.admission_id = NEW.admission_id
			AND admission.admission_digest = NEW.admission_digest
			AND admission.project_root = NEW.project_root
			AND admission.ledger_revision = NEW.ledger_revision
			AND admission.profile_payload_digest = NEW.profile_payload_digest
			AND revision.admission_digest = NEW.admission_digest
			AND revision.project_root = NEW.project_root
			AND revision.ledger_revision = NEW.ledger_revision
			AND authority_use.committed_result_digest = NEW.admission_digest
		))
		OR (NEW.profile_revision_generation = 'v2' AND EXISTS (
			SELECT 1
			FROM project_profile_admissions_v2 admission
			JOIN project_profile_revisions_v2 revision ON revision.admission_id = admission.admission_id
			JOIN profile_declaration_authority_uses_v2 authority_use ON authority_use.committed_admission_ref = admission.admission_id
			WHERE admission.admission_id = NEW.admission_id
			AND admission.admission_digest = NEW.admission_digest
			AND admission.project_root = NEW.project_root
			AND admission.ledger_revision = NEW.ledger_revision
			AND admission.profile_payload_digest = NEW.profile_payload_digest
			AND revision.admission_digest = NEW.admission_digest
			AND revision.project_root = NEW.project_root
			AND revision.ledger_revision = NEW.ledger_revision
			AND authority_use.committed_admission_digest = NEW.admission_digest
		))
	 ) OR (NEW.event_kind = 'resolved' AND NOT (
		(NEW.supersedes_event_generation = 'v1' AND EXISTS (
			SELECT 1 FROM project_profile_projection_debt opened
			WHERE opened.event_id = NEW.supersedes_event_id
			AND opened.debt_id = NEW.debt_id
			AND opened.admission_id = NEW.admission_id
			AND opened.admission_digest = NEW.admission_digest
			AND opened.event_kind = 'opened'
		))
		OR (NEW.supersedes_event_generation = 'v2' AND EXISTS (
			SELECT 1 FROM project_profile_projection_debt_v2 opened
			WHERE opened.event_id = NEW.supersedes_event_id
			AND opened.debt_id = NEW.debt_id
			AND opened.admission_id = NEW.admission_id
			AND opened.admission_digest = NEW.admission_digest
			AND opened.event_kind = 'opened'
		))
	 )) BEGIN
		SELECT RAISE(ABORT, 'v2 projection debt does not bind the exact admission or opened debt event');
	 END`
}

func projectProfileProjectionDebtCrossGenerationTrigger44() string {
	return `CREATE TRIGGER project_profile_projection_debt_v2_no_cross_generation_collision
	 BEFORE INSERT ON project_profile_projection_debt_v2
	 WHEN EXISTS (
		SELECT 1 FROM project_profile_projection_debt legacy
		WHERE legacy.event_id = NEW.event_id
		OR (NEW.event_kind = 'opened' AND legacy.debt_id = NEW.debt_id)
	 ) BEGIN
		SELECT RAISE(ABORT, 'v2 projection debt identity collides with legacy history');
	 END`
}

func currentProjectProfilesView44() string {
	return `CREATE VIEW current_project_profiles AS
	WITH all_revisions (
		storage_generation, project_root, ledger_revision, configured_profile_kind,
		profile_payload_json, profile_payload_digest,
		receipt_json, receipt_digest, admission_id, admission_digest, recorded_at
	) AS (
		SELECT 'v1', project_root, ledger_revision, configured_profile_kind,
			profile_payload_json, profile_payload_digest,
			receipt_json, receipt_digest, admission_id, admission_digest, recorded_at
		FROM project_profile_revisions
		UNION ALL
		SELECT 'v2', project_root, ledger_revision, configured_profile_kind,
			profile_payload_json, profile_payload_digest,
			receipt_json, receipt_digest, admission_id, admission_digest, recorded_at
		FROM project_profile_revisions_v2
	), valid_roots AS (
		SELECT project_root
		FROM all_revisions
		GROUP BY project_root
		HAVING MIN(ledger_revision) = 1
		AND COUNT(*) = MAX(ledger_revision)
		AND COUNT(DISTINCT ledger_revision) = COUNT(*)
	)
	SELECT revision.storage_generation,
		revision.project_root, revision.ledger_revision, revision.configured_profile_kind,
		revision.profile_payload_json, revision.profile_payload_digest,
		revision.receipt_json, revision.receipt_digest,
		revision.admission_id, revision.admission_digest, revision.recorded_at
	FROM all_revisions revision
	JOIN valid_roots valid ON valid.project_root = revision.project_root
	WHERE NOT EXISTS (
		SELECT 1 FROM all_revisions newer
		WHERE newer.project_root = revision.project_root
		AND newer.ledger_revision > revision.ledger_revision
	)`
}

func appendProjectProfileV2AppendOnlyTriggers44(statements []string) []string {
	statements = append(statements,
		`CREATE TRIGGER project_profile_revisions_v2_no_replace
		 BEFORE INSERT ON project_profile_revisions_v2
		 WHEN EXISTS (
			SELECT 1 FROM project_profile_revisions_v2 existing
			WHERE (existing.project_root = NEW.project_root AND existing.ledger_revision = NEW.ledger_revision)
			OR existing.admission_id = NEW.admission_id
			OR existing.admission_digest = NEW.admission_digest
			OR existing.receipt_digest = NEW.receipt_digest
		 ) BEGIN SELECT RAISE(ABORT, 'project_profile_revisions_v2 is append-only'); END`,
		`CREATE TRIGGER project_profile_revisions_v2_no_update
		 BEFORE UPDATE ON project_profile_revisions_v2 BEGIN
			SELECT RAISE(ABORT, 'project_profile_revisions_v2 is append-only');
		 END`,
		`CREATE TRIGGER project_profile_revisions_v2_no_delete
		 BEFORE DELETE ON project_profile_revisions_v2 BEGIN
			SELECT RAISE(ABORT, 'project_profile_revisions_v2 is append-only');
		 END`,
		`CREATE TRIGGER project_profile_projection_debt_v2_no_replace
		 BEFORE INSERT ON project_profile_projection_debt_v2
		 WHEN EXISTS (SELECT 1 FROM project_profile_projection_debt_v2 existing WHERE existing.event_id = NEW.event_id)
		 BEGIN SELECT RAISE(ABORT, 'project_profile_projection_debt_v2 is append-only'); END`,
		`CREATE TRIGGER project_profile_projection_debt_v2_no_update
		 BEFORE UPDATE ON project_profile_projection_debt_v2 BEGIN
			SELECT RAISE(ABORT, 'project_profile_projection_debt_v2 is append-only');
		 END`,
		`CREATE TRIGGER project_profile_projection_debt_v2_no_delete
		 BEFORE DELETE ON project_profile_projection_debt_v2 BEGIN
			SELECT RAISE(ABORT, 'project_profile_projection_debt_v2 is append-only');
		 END`,
	)
	return statements
}

func appendProfileAuthorityAdmissionV2RootGuards44(
	statements []string,
	index int,
) []string {
	if index >= len(profileAuthorityAdmissionV2Tables) {
		return statements
	}
	table := profileAuthorityAdmissionV2Tables[index]
	trigger := "CREATE TRIGGER " + table + "_project_ledger_root " +
		"BEFORE INSERT ON " + table + " WHEN NOT EXISTS " +
		"(SELECT 1 FROM project_ledger_binding binding WHERE binding.project_root = NEW.project_root) " +
		"BEGIN SELECT RAISE(ABORT, '" + table + " does not match the bound project ledger root'); END"
	next := slices.Clone(statements)
	next = append(next, trigger)
	return appendProfileAuthorityAdmissionV2RootGuards44(next, index+1)
}

func appendLegacyProfileWriteSeals44(statements []string, index int) []string {
	if index >= len(legacyProfileWriteTables44) {
		return statements
	}
	table := legacyProfileWriteTables44[index]
	trigger := "CREATE TRIGGER " + table + "_v44_writes_sealed " +
		"BEFORE INSERT ON " + table + " BEGIN " +
		"SELECT RAISE(ABORT, 'legacy profile writes are sealed after schema v44'); END"
	next := slices.Clone(statements)
	next = append(next, trigger)
	return appendLegacyProfileWriteSeals44(next, index+1)
}
