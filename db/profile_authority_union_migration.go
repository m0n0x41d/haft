package db

import (
	"fmt"
	"slices"
)

const (
	profileOnboardingWorkInputTable51 = "profile_onboarding_work_inputs_v1"
	profileAuthorityBasisTable51      = "profile_declaration_authority_bases_v3"
	profileAuthorityResolutionTable51 = "profile_declaration_authority_resolutions_v3"
	profileAuthorityUseTable51        = "profile_declaration_authority_uses_v3"
	projectProfileAdmissionTable51    = "project_profile_admissions_v3"
	projectProfileRevisionTable51     = "project_profile_revisions_v3"
	projectProfileDebtTable51         = "project_profile_projection_debt_v3"

	profileAuthorityExplicitMode51       = "explicit_h_onboard"
	profileAuthorityStrictMode51         = "strict_cli_speech_act"
	profileAuthorityExplicitResolution51 = "explicit_policy_acceptance"
	profileAuthorityStrictResolution51   = "strict_permission"

	profileWorkInputSchema51           = "haft.profile-onboarding.work-input/v1"
	profileAuthorityBasisSchema51      = "haft.profile-authority.authority-basis/v3"
	profileAuthorityResolutionSchema51 = "haft.profile-authority.authority-resolution/v3"
	profileAuthorityUseSchema51        = "haft.profile-authority.authority-use/v3"
)

var profileAuthorityUnionMigration51 = Migration{
	Version:     51,
	Description: "Add closed profile authority union and v3 profile admission ledger",
	Apply:       applyProfileAuthorityUnionMigration51,
}

var profileAuthorityUnionTables51 = []string{
	profileOnboardingWorkInputTable51,
	profileAuthorityBasisTable51,
	profileAuthorityResolutionTable51,
	profileAuthorityUseTable51,
	projectProfileAdmissionTable51,
	projectProfileRevisionTable51,
	projectProfileDebtTable51,
}

var profileAdmissionV2WriteTables51 = []string{
	profileAuthorityV2ResolutionTable,
	profileAuthorityV2UseTable,
	projectProfileV2AdmissionTable,
	projectProfileV2RevisionTable,
	projectProfileV2DebtTable,
}

func applyProfileAuthorityUnionMigration51(
	tx MigrationTransaction,
	_ []Migration,
) error {
	if err := requireProfileAuthorityUnionSource51(tx); err != nil {
		return err
	}
	if err := requireAbsentProfileAuthorityUnionFootprint51(tx, 0); err != nil {
		return err
	}
	if err := requireProfileAuthorityUnionDependencies51(tx, 0); err != nil {
		return err
	}
	if err := executeStatements(tx, profileAuthorityUnionStatements51(), 0); err != nil {
		return fmt.Errorf("install profile authority union: %w", err)
	}
	if err := verifyForeignKeys(tx); err != nil {
		return fmt.Errorf("verify profile authority union: %w", err)
	}
	return nil
}

func requireProfileAuthorityUnionSource51(tx MigrationTransaction) error {
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = 50",
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect profile authority union source migration: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("profile authority union requires schema version 50")
	}
	return nil
}

var profileAuthorityUnionDependencies51 = []struct {
	kind string
	name string
}{
	{kind: "table", name: "project_ledger_binding"},
	{kind: "table", name: "profile_onboarding_method_descriptions"},
	{kind: "table", name: "profile_onboarding_method_contracts"},
	{kind: "table", name: "profile_author_role_assignments"},
	{kind: "table", name: "observed_project_bases"},
	{kind: "table", name: "profile_onboarding_work_records"},
	{kind: "table", name: "profile_onboarding_effects"},
	{kind: "table", name: "profile_onboarding_outcome_assessments"},
	{kind: "table", name: "profile_declaration_authority_bases_v2"},
	{kind: "table", name: "profile_declaration_permissions_v2"},
	{kind: "table", name: "profile_declaration_authorization_contents_v2"},
	{kind: "table", name: profileAuthorityV2ResolutionTable},
	{kind: "table", name: profileAuthorityV2UseTable},
	{kind: "table", name: projectProfileV2AdmissionTable},
	{kind: "table", name: projectProfileV2RevisionTable},
	{kind: "table", name: projectProfileV2DebtTable},
	{kind: "view", name: "current_project_profiles"},
}

func requireProfileAuthorityUnionDependencies51(
	tx MigrationTransaction,
	index int,
) error {
	if index >= len(profileAuthorityUnionDependencies51) {
		return nil
	}
	dependency := profileAuthorityUnionDependencies51[index]
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ? AND sql IS NOT NULL",
		dependency.kind,
		dependency.name,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect v51 dependency %s %s: %w", dependency.kind, dependency.name, err)
	}
	if count != 1 {
		return fmt.Errorf(
			"profile authority union requires exact dependency %s %s",
			dependency.kind,
			dependency.name,
		)
	}
	return requireProfileAuthorityUnionDependencies51(tx, index+1)
}

func requireAbsentProfileAuthorityUnionFootprint51(
	tx MigrationTransaction,
	index int,
) error {
	if index >= len(profileAuthorityUnionTables51) {
		return nil
	}
	table := profileAuthorityUnionTables51[index]
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect profile authority union table %s: %w", table, err)
	}
	if count != 0 {
		return fmt.Errorf(
			"profile authority union refused: unversioned table %s already exists; unknown partial schema requires manual review",
			table,
		)
	}
	return requireAbsentProfileAuthorityUnionFootprint51(tx, index+1)
}

func profileAuthorityUnionStatements51() []string {
	statements := []string{
		profileOnboardingWorkInputTableSQL51(),
		profileAuthorityBasisTableSQL51(),
		profileAuthorityResolutionTableSQL51(),
		projectProfileAdmissionTableSQL51(),
		profileAuthorityUseTableSQL51(),
		projectProfileRevisionTableSQL51(),
		projectProfileDebtTableSQL51(),
		profileAuthorityBasisExactSourcesTrigger51(),
		profileAuthorityBasisCrossGenerationTrigger51(),
		profileAuthorityResolutionExactSourcesTrigger51(),
		profileAuthorityResolutionCrossGenerationTrigger51(),
		projectProfileAdmissionRevisionCASTrigger51(),
		projectProfileAdmissionExactSourcesTrigger51(),
		projectProfileAdmissionCrossGenerationTrigger51(),
		profileAuthorityUseExactSourcesTrigger51(),
		profileAuthorityUseCrossGenerationTrigger51(),
		projectProfileRevisionExactAdmissionTrigger51(),
		projectProfileRevisionCrossGenerationTrigger51(),
		projectProfileDebtExactAdmissionTrigger51(),
		projectProfileDebtCrossGenerationTrigger51(),
		"DROP VIEW current_project_profiles",
		currentProjectProfilesView51(),
		"CREATE INDEX idx_profile_work_inputs_v1_project_recorded ON profile_onboarding_work_inputs_v1(project_root, recorded_at)",
		"CREATE INDEX idx_profile_authority_bases_v3_project_recorded ON profile_declaration_authority_bases_v3(project_root, recorded_at)",
		"CREATE INDEX idx_profile_authority_resolutions_v3_project_checked ON profile_declaration_authority_resolutions_v3(project_root, checked_at)",
		"CREATE INDEX idx_profile_authority_uses_v3_project_consumed ON profile_declaration_authority_uses_v3(project_root, consumed_at)",
		"CREATE INDEX idx_project_profile_admissions_v3_project_revision ON project_profile_admissions_v3(project_root, ledger_revision)",
		"CREATE INDEX idx_project_profile_revisions_v3_current ON project_profile_revisions_v3(project_root, ledger_revision DESC)",
		"CREATE INDEX idx_project_profile_projection_debt_v3_open ON project_profile_projection_debt_v3(debt_id, event_kind, recorded_at)",
	}
	immutable := []immutableAuthorityBasisTable{
		{name: profileOnboardingWorkInputTable51, primaryKey: "work_input_ref", digestColumn: "work_input_digest"},
		{name: profileAuthorityBasisTable51, primaryKey: "basis_ref", digestColumn: "basis_digest"},
		{name: profileAuthorityResolutionTable51, primaryKey: "authority_resolution_ref", digestColumn: "authority_resolution_digest"},
		{name: profileAuthorityUseTable51, primaryKey: "use_ref", digestColumn: "use_digest"},
		{name: projectProfileAdmissionTable51, primaryKey: "admission_id", digestColumn: "admission_digest"},
	}
	statements = appendAuthorityBasisTableTriggers(statements, immutable, 0)
	statements = appendProjectProfileV3AppendOnlyTriggers51(statements)
	statements = appendProfileAuthorityUnionRootGuards51(statements, 0)
	return appendProfileAdmissionV2WriteSeals51(statements, 0)
}

func profileOnboardingWorkInputTableSQL51() string {
	return `CREATE TABLE profile_onboarding_work_inputs_v1 (
		work_input_ref TEXT PRIMARY KEY CHECK(work_input_ref GLOB 'profile-onboarding-work-input:?*'),
		work_input_digest TEXT NOT NULL UNIQUE CHECK(length(work_input_digest) = 71 AND substr(work_input_digest, 1, 7) = 'sha256:'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		suggestion_ref TEXT NOT NULL CHECK(suggestion_ref != ''),
		detector_version TEXT NOT NULL CHECK(detector_version != ''),
		policy_version TEXT NOT NULL CHECK(policy_version != ''),
		observation_digest TEXT NOT NULL CHECK(length(observation_digest) = 71 AND substr(observation_digest, 1, 7) = 'sha256:'),
		profile_payload_json TEXT NOT NULL CHECK(json_valid(profile_payload_json)),
		profile_payload_digest TEXT NOT NULL CHECK(length(profile_payload_digest) = 71 AND substr(profile_payload_digest, 1, 7) = 'sha256:'),
		canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
		recorded_at TEXT NOT NULL,
		CHECK(COALESCE(json_extract(canonical_json, '$.schema') = '` + profileWorkInputSchema51 + `', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.project_root') = project_root, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.suggestion_ref') = suggestion_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.detector_version') = detector_version, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.policy_version') = policy_version, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.observation_digest') = observation_digest, 0)),
		CHECK(json_type(canonical_json, '$.scopes') = 'array'),
		CHECK(json_array_length(canonical_json, '$.scopes') > 0),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `)
	) WITHOUT ROWID`
}

func profileAuthorityBasisTableSQL51() string {
	return `CREATE TABLE profile_declaration_authority_bases_v3 (
		basis_ref TEXT PRIMARY KEY CHECK(basis_ref GLOB 'profile-authority-basis:?*'),
		basis_digest TEXT NOT NULL UNIQUE CHECK(length(basis_digest) = 71 AND substr(basis_digest, 1, 7) = 'sha256:'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		action_kind TEXT NOT NULL CHECK(action_kind = '` + profileAuthorityV2Action + `'),
		authority_mode TEXT NOT NULL CHECK(authority_mode IN ('` + profileAuthorityExplicitMode51 + `', '` + profileAuthorityStrictMode51 + `')),
		work_input_ref TEXT NOT NULL UNIQUE REFERENCES profile_onboarding_work_inputs_v1(work_input_ref),
		work_input_digest TEXT NOT NULL CHECK(length(work_input_digest) = 71 AND substr(work_input_digest, 1, 7) = 'sha256:'),
		profile_author_role_assignment_ref TEXT NOT NULL REFERENCES profile_author_role_assignments(role_assignment_ref),
		profile_author_role_assignment_digest TEXT NOT NULL CHECK(length(profile_author_role_assignment_digest) = 71 AND substr(profile_author_role_assignment_digest, 1, 7) = 'sha256:'),
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
		single_use_key TEXT NOT NULL UNIQUE CHECK(single_use_key != ''),
		config_carrier_ref TEXT,
		config_carrier_digest TEXT CHECK(config_carrier_digest IS NULL OR (length(config_carrier_digest) = 71 AND substr(config_carrier_digest, 1, 7) = 'sha256:')),
		strict_authority_basis_ref TEXT UNIQUE REFERENCES profile_declaration_authority_bases_v2(basis_ref),
		strict_authority_basis_digest TEXT CHECK(strict_authority_basis_digest IS NULL OR (length(strict_authority_basis_digest) = 71 AND substr(strict_authority_basis_digest, 1, 7) = 'sha256:')),
		canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
		recorded_at TEXT NOT NULL,
		CHECK((authority_mode = '` + profileAuthorityExplicitMode51 + `' AND config_carrier_ref IS NOT NULL AND config_carrier_ref != '' AND config_carrier_digest IS NOT NULL AND strict_authority_basis_ref IS NULL AND strict_authority_basis_digest IS NULL) OR (authority_mode = '` + profileAuthorityStrictMode51 + `' AND config_carrier_ref IS NULL AND config_carrier_digest IS NULL AND strict_authority_basis_ref IS NOT NULL AND strict_authority_basis_digest IS NOT NULL)),
		CHECK(COALESCE(json_extract(canonical_json, '$.schema') = '` + profileAuthorityBasisSchema51 + `', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.basis_ref') = basis_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.project_root') = project_root, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.action_kind') = action_kind, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_mode') = authority_mode, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.work_input_ref') = work_input_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.work_input_digest') = work_input_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.profile_author_role_assignment_ref') = profile_author_role_assignment_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.profile_author_role_assignment_digest') = profile_author_role_assignment_digest, 0)),
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
		CHECK(COALESCE(json_extract(canonical_json, '$.single_use_key') = single_use_key, 0)),
		CHECK((authority_mode = '` + profileAuthorityExplicitMode51 + `' AND COALESCE(json_extract(canonical_json, '$.config_carrier_ref') = config_carrier_ref, 0) AND COALESCE(json_extract(canonical_json, '$.config_carrier_digest') = config_carrier_digest, 0) AND json_type(canonical_json, '$.strict_authority_basis_ref') IS NULL AND json_type(canonical_json, '$.strict_authority_basis_digest') IS NULL) OR (authority_mode = '` + profileAuthorityStrictMode51 + `' AND json_type(canonical_json, '$.config_carrier_ref') IS NULL AND json_type(canonical_json, '$.config_carrier_digest') IS NULL AND COALESCE(json_extract(canonical_json, '$.strict_authority_basis_ref') = strict_authority_basis_ref, 0) AND COALESCE(json_extract(canonical_json, '$.strict_authority_basis_digest') = strict_authority_basis_digest, 0))),
		CHECK(` + sqliteCanonicalUTCNanoShape("allowed_work_from") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("allowed_work_until") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("basis_observation_from") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("basis_observation_until") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		CHECK(` + sqliteUTCNanoLess("allowed_work_from", "allowed_work_until") + `),
		CHECK(` + sqliteUTCNanoLess("basis_observation_from", "basis_observation_until") + `),
		CHECK(` + sqliteUTCNanoLessOrEqual("recorded_at", "allowed_work_from") + `)
	) WITHOUT ROWID`
}

func profileAuthorityResolutionTableSQL51() string {
	return `CREATE TABLE profile_declaration_authority_resolutions_v3 (
		authority_resolution_ref TEXT PRIMARY KEY CHECK(authority_resolution_ref GLOB 'profile-authority-resolution:?*'),
		authority_resolution_digest TEXT NOT NULL UNIQUE CHECK(length(authority_resolution_digest) = 71 AND substr(authority_resolution_digest, 1, 7) = 'sha256:'),
		authority_basis_ref TEXT NOT NULL UNIQUE REFERENCES profile_declaration_authority_bases_v3(basis_ref),
		authority_basis_digest TEXT NOT NULL CHECK(length(authority_basis_digest) = 71 AND substr(authority_basis_digest, 1, 7) = 'sha256:'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		action_kind TEXT NOT NULL CHECK(action_kind = '` + profileAuthorityV2Action + `'),
		authority_mode TEXT NOT NULL CHECK(authority_mode IN ('` + profileAuthorityExplicitMode51 + `', '` + profileAuthorityStrictMode51 + `')),
		resolution_kind TEXT NOT NULL CHECK(resolution_kind IN ('` + profileAuthorityExplicitResolution51 + `', '` + profileAuthorityStrictResolution51 + `')),
		work_input_ref TEXT NOT NULL UNIQUE REFERENCES profile_onboarding_work_inputs_v1(work_input_ref),
		work_input_digest TEXT NOT NULL CHECK(length(work_input_digest) = 71 AND substr(work_input_digest, 1, 7) = 'sha256:'),
		project_binding_digest TEXT NOT NULL CHECK(length(project_binding_digest) = 71 AND substr(project_binding_digest, 1, 7) = 'sha256:'),
		strict_permission_ref TEXT UNIQUE REFERENCES profile_declaration_permissions_v2(permission_ref),
		strict_permission_digest TEXT CHECK(strict_permission_digest IS NULL OR (length(strict_permission_digest) = 71 AND substr(strict_permission_digest, 1, 7) = 'sha256:')),
		verifier_identity TEXT NOT NULL CHECK(verifier_identity != ''),
		verifier_version TEXT NOT NULL CHECK(verifier_version != ''),
		verification_policy_ref TEXT NOT NULL CHECK(verification_policy_ref != ''),
		verification_policy_digest TEXT NOT NULL CHECK(length(verification_policy_digest) = 71 AND substr(verification_policy_digest, 1, 7) = 'sha256:'),
		checked_at TEXT NOT NULL,
		currentness_result TEXT NOT NULL CHECK(currentness_result = 'current'),
		predicate_result TEXT NOT NULL CHECK(predicate_result = 'satisfied'),
		admission_result TEXT NOT NULL CHECK(admission_result = 'admitted'),
		canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
		recorded_at TEXT NOT NULL,
		CHECK((authority_mode = '` + profileAuthorityExplicitMode51 + `' AND resolution_kind = '` + profileAuthorityExplicitResolution51 + `' AND strict_permission_ref IS NULL AND strict_permission_digest IS NULL) OR (authority_mode = '` + profileAuthorityStrictMode51 + `' AND resolution_kind = '` + profileAuthorityStrictResolution51 + `' AND strict_permission_ref IS NOT NULL AND strict_permission_digest IS NOT NULL)),
		CHECK(COALESCE(json_extract(canonical_json, '$.schema') = '` + profileAuthorityResolutionSchema51 + `', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_resolution_ref') = authority_resolution_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_basis_ref') = authority_basis_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_basis_digest') = authority_basis_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.project_root') = project_root, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.action_kind') = action_kind, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_mode') = authority_mode, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.resolution_kind') = resolution_kind, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.work_input_ref') = work_input_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.work_input_digest') = work_input_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.project_binding_digest') = project_binding_digest, 0)),
		CHECK((authority_mode = '` + profileAuthorityExplicitMode51 + `' AND json_type(canonical_json, '$.strict_permission_ref') IS NULL AND json_type(canonical_json, '$.strict_permission_digest') IS NULL) OR (authority_mode = '` + profileAuthorityStrictMode51 + `' AND COALESCE(json_extract(canonical_json, '$.strict_permission_ref') = strict_permission_ref, 0) AND COALESCE(json_extract(canonical_json, '$.strict_permission_digest') = strict_permission_digest, 0))),
		CHECK(COALESCE(json_extract(canonical_json, '$.verifier_identity') = verifier_identity, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.verifier_version') = verifier_version, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.verification_policy_ref') = verification_policy_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.verification_policy_digest') = verification_policy_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.checked_at') = checked_at, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.currentness_result') = currentness_result, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.predicate_result') = predicate_result, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.admission_result') = admission_result, 0)),
		CHECK(recorded_at = checked_at),
		CHECK(` + sqliteCanonicalUTCNanoShape("checked_at") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `)
	) WITHOUT ROWID`
}

func projectProfileAdmissionTableSQL51() string {
	return `CREATE TABLE project_profile_admissions_v3 (
		admission_id TEXT PRIMARY KEY,
		action_kind TEXT NOT NULL CHECK(action_kind = '` + profileAuthorityV2Action + `'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		authority_mode TEXT NOT NULL CHECK(authority_mode IN ('` + profileAuthorityExplicitMode51 + `', '` + profileAuthorityStrictMode51 + `')),
		resolution_kind TEXT NOT NULL CHECK(resolution_kind IN ('` + profileAuthorityExplicitResolution51 + `', '` + profileAuthorityStrictResolution51 + `')),
		project_binding_digest TEXT NOT NULL CHECK(length(project_binding_digest) = 71 AND substr(project_binding_digest, 1, 7) = 'sha256:'),
		work_input_ref TEXT NOT NULL REFERENCES profile_onboarding_work_inputs_v1(work_input_ref),
		work_input_digest TEXT NOT NULL CHECK(length(work_input_digest) = 71 AND substr(work_input_digest, 1, 7) = 'sha256:'),
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
		authority_basis_ref TEXT NOT NULL REFERENCES profile_declaration_authority_bases_v3(basis_ref),
		authority_basis_digest TEXT NOT NULL CHECK(length(authority_basis_digest) = 71 AND substr(authority_basis_digest, 1, 7) = 'sha256:'),
		authority_resolution_ref TEXT NOT NULL UNIQUE REFERENCES profile_declaration_authority_resolutions_v3(authority_resolution_ref),
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
		CHECK((authority_mode = '` + profileAuthorityExplicitMode51 + `' AND resolution_kind = '` + profileAuthorityExplicitResolution51 + `') OR (authority_mode = '` + profileAuthorityStrictMode51 + `' AND resolution_kind = '` + profileAuthorityStrictResolution51 + `')),
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

func profileAuthorityUseTableSQL51() string {
	return `CREATE TABLE profile_declaration_authority_uses_v3 (
		use_ref TEXT PRIMARY KEY CHECK(use_ref GLOB 'profile-authority-use:?*'),
		use_digest TEXT NOT NULL UNIQUE CHECK(length(use_digest) = 71 AND substr(use_digest, 1, 7) = 'sha256:'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		action_kind TEXT NOT NULL CHECK(action_kind = '` + profileAuthorityV2Action + `'),
		authority_mode TEXT NOT NULL CHECK(authority_mode IN ('` + profileAuthorityExplicitMode51 + `', '` + profileAuthorityStrictMode51 + `')),
		resolution_kind TEXT NOT NULL CHECK(resolution_kind IN ('` + profileAuthorityExplicitResolution51 + `', '` + profileAuthorityStrictResolution51 + `')),
		project_binding_digest TEXT NOT NULL CHECK(length(project_binding_digest) = 71 AND substr(project_binding_digest, 1, 7) = 'sha256:'),
		authority_resolution_ref TEXT NOT NULL UNIQUE REFERENCES profile_declaration_authority_resolutions_v3(authority_resolution_ref),
		authority_resolution_digest TEXT NOT NULL CHECK(length(authority_resolution_digest) = 71 AND substr(authority_resolution_digest, 1, 7) = 'sha256:'),
		authority_basis_ref TEXT NOT NULL UNIQUE REFERENCES profile_declaration_authority_bases_v3(basis_ref),
		authority_basis_digest TEXT NOT NULL CHECK(length(authority_basis_digest) = 71 AND substr(authority_basis_digest, 1, 7) = 'sha256:'),
		work_input_ref TEXT NOT NULL UNIQUE REFERENCES profile_onboarding_work_inputs_v1(work_input_ref),
		work_input_digest TEXT NOT NULL CHECK(length(work_input_digest) = 71 AND substr(work_input_digest, 1, 7) = 'sha256:'),
		single_use_key TEXT NOT NULL UNIQUE CHECK(single_use_key != ''),
		admission_request_digest TEXT NOT NULL CHECK(length(admission_request_digest) = 71 AND substr(admission_request_digest, 1, 7) = 'sha256:'),
		committed_admission_ref TEXT NOT NULL UNIQUE REFERENCES project_profile_admissions_v3(admission_id),
		committed_admission_digest TEXT NOT NULL CHECK(length(committed_admission_digest) = 71 AND substr(committed_admission_digest, 1, 7) = 'sha256:'),
		canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
		consumed_at TEXT NOT NULL,
		recorded_at TEXT NOT NULL,
		CHECK((authority_mode = '` + profileAuthorityExplicitMode51 + `' AND resolution_kind = '` + profileAuthorityExplicitResolution51 + `') OR (authority_mode = '` + profileAuthorityStrictMode51 + `' AND resolution_kind = '` + profileAuthorityStrictResolution51 + `')),
		CHECK(COALESCE(json_extract(canonical_json, '$.schema') = '` + profileAuthorityUseSchema51 + `', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.use_ref') = use_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.project_root') = project_root, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.action_kind') = action_kind, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_mode') = authority_mode, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.resolution_kind') = resolution_kind, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.project_binding_digest') = project_binding_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_resolution_ref') = authority_resolution_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_resolution_digest') = authority_resolution_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_basis_ref') = authority_basis_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_basis_digest') = authority_basis_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.work_input_ref') = work_input_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.work_input_digest') = work_input_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.single_use_key') = single_use_key, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.admission_request_digest') = admission_request_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.committed_admission_ref') = committed_admission_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.committed_admission_digest') = committed_admission_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.consumed_at') = consumed_at, 0)),
		CHECK(recorded_at = consumed_at),
		CHECK(` + sqliteCanonicalUTCNanoShape("consumed_at") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `)
	) WITHOUT ROWID`
}

func projectProfileRevisionTableSQL51() string {
	return `CREATE TABLE project_profile_revisions_v3 (
		project_root TEXT NOT NULL CHECK(project_root != ''),
		ledger_revision INTEGER NOT NULL CHECK(ledger_revision > 0),
		configured_profile_kind TEXT NOT NULL CHECK(configured_profile_kind = 'Declared'),
		profile_payload_json TEXT NOT NULL CHECK(json_valid(profile_payload_json)),
		profile_payload_digest TEXT NOT NULL CHECK(length(profile_payload_digest) = 71 AND substr(profile_payload_digest, 1, 7) = 'sha256:'),
		receipt_json TEXT NOT NULL CHECK(json_valid(receipt_json)),
		receipt_digest TEXT NOT NULL UNIQUE CHECK(length(receipt_digest) = 71 AND substr(receipt_digest, 1, 7) = 'sha256:'),
		admission_id TEXT NOT NULL UNIQUE REFERENCES project_profile_admissions_v3(admission_id),
		admission_digest TEXT NOT NULL UNIQUE CHECK(length(admission_digest) = 71 AND substr(admission_digest, 1, 7) = 'sha256:'),
		recorded_at TEXT NOT NULL,
		PRIMARY KEY(project_root, ledger_revision),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `)
	) WITHOUT ROWID`
}

func projectProfileDebtTableSQL51() string {
	return `CREATE TABLE project_profile_projection_debt_v3 (
		event_id TEXT PRIMARY KEY,
		debt_id TEXT NOT NULL CHECK(debt_id != ''),
		profile_revision_generation TEXT NOT NULL CHECK(profile_revision_generation IN ('v1', 'v2', 'v3')),
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
		supersedes_event_generation TEXT CHECK(supersedes_event_generation IS NULL OR supersedes_event_generation IN ('v1', 'v2', 'v3')),
		supersedes_event_id TEXT,
		recorded_at TEXT NOT NULL,
		CHECK((event_kind = 'opened' AND supersedes_event_generation IS NULL AND supersedes_event_id IS NULL) OR (event_kind = 'resolved' AND supersedes_event_generation IS NOT NULL AND supersedes_event_id IS NOT NULL)),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `)
	) WITHOUT ROWID`
}

func profileAuthorityBasisExactSourcesTrigger51() string {
	return `CREATE TRIGGER profile_declaration_authority_bases_v3_exact_sources
	 BEFORE INSERT ON profile_declaration_authority_bases_v3
	 WHEN NOT EXISTS (
		SELECT 1
		FROM profile_onboarding_work_inputs_v1 work_input
		JOIN profile_author_role_assignments assignment
			ON assignment.role_assignment_ref = NEW.profile_author_role_assignment_ref
		JOIN profile_onboarding_method_descriptions description
			ON description.method_description_ref = NEW.method_description_ref
		JOIN profile_onboarding_method_contracts contract
			ON contract.method_contract_ref = NEW.method_contract_ref
		JOIN project_ledger_binding binding ON binding.project_root = NEW.project_root
		WHERE work_input.work_input_ref = NEW.work_input_ref
		AND work_input.work_input_digest = NEW.work_input_digest
		AND work_input.project_root = NEW.project_root
		AND work_input.detector_version = NEW.classifier_version
		AND work_input.policy_version = NEW.policy_version
		AND assignment.role_assignment_digest = NEW.profile_author_role_assignment_digest
		AND description.method_description_digest = NEW.method_description_digest
		AND contract.method_contract_digest = NEW.method_contract_digest
		AND contract.method_description_ref = NEW.method_description_ref
		AND contract.method_description_digest = NEW.method_description_digest
		AND ` + sqliteUTCNanoLessOrEqual("work_input.recorded_at", "NEW.recorded_at") + `
		AND (
			(NEW.authority_mode = '` + profileAuthorityExplicitMode51 + `'
			 AND NEW.config_carrier_ref != ''
			 AND NEW.config_carrier_digest IS NOT NULL)
			OR
			(NEW.authority_mode = '` + profileAuthorityStrictMode51 + `'
			 AND EXISTS (
				SELECT 1
				FROM profile_declaration_authority_bases_v2 strict_basis
				JOIN profile_declaration_authorization_contents_v2 content
					ON content.authorization_content_ref = strict_basis.authorization_content_ref
				JOIN profile_declaration_permissions_v2 permission
					ON permission.permission_ref = strict_basis.permission_ref
				WHERE strict_basis.basis_ref = NEW.strict_authority_basis_ref
				AND strict_basis.basis_digest = NEW.strict_authority_basis_digest
				AND strict_basis.project_root = NEW.project_root
				AND content.authorization_content_digest = strict_basis.authorization_content_digest
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
				AND content.single_use_key = NEW.single_use_key
				AND permission.permission_digest = strict_basis.permission_digest
				AND permission.subject_ref = NEW.profile_author_role_assignment_ref
				AND permission.subject_digest = NEW.profile_author_role_assignment_digest
				AND permission.method_description_ref = NEW.method_description_ref
				AND permission.method_description_digest = NEW.method_description_digest
			 )
			)
		)
	 ) BEGIN
		SELECT RAISE(ABORT, 'v3 profile authority basis does not bind exact WorkInput, method, ProfileAuthor, project, and selected authority branch');
	 END`
}

func profileAuthorityBasisCrossGenerationTrigger51() string {
	return `CREATE TRIGGER profile_declaration_authority_bases_v3_no_cross_generation_collision
	 BEFORE INSERT ON profile_declaration_authority_bases_v3
	 WHEN EXISTS (
		SELECT 1 FROM profile_declaration_authority_bases_v2 previous
		WHERE previous.basis_ref = NEW.basis_ref
		OR previous.basis_digest = NEW.basis_digest
	 ) OR EXISTS (
		SELECT 1 FROM authority_uses legacy WHERE legacy.single_use_key = NEW.single_use_key
	 ) OR EXISTS (
		SELECT 1 FROM profile_declaration_authority_uses_v2 previous WHERE previous.single_use_key = NEW.single_use_key
	 ) BEGIN
		SELECT RAISE(ABORT, 'v3 profile authority basis collides with earlier authority history or a consumed single-use key');
	 END`
}

func profileAuthorityResolutionExactSourcesTrigger51() string {
	return `CREATE TRIGGER profile_declaration_authority_resolutions_v3_exact_sources
	 BEFORE INSERT ON profile_declaration_authority_resolutions_v3
	 WHEN NOT EXISTS (
		SELECT 1
		FROM profile_declaration_authority_bases_v3 basis
		JOIN profile_onboarding_work_inputs_v1 work_input
			ON work_input.work_input_ref = basis.work_input_ref
		JOIN project_ledger_binding binding ON binding.project_root = basis.project_root
		WHERE basis.basis_ref = NEW.authority_basis_ref
		AND basis.basis_digest = NEW.authority_basis_digest
		AND basis.project_root = NEW.project_root
		AND basis.action_kind = NEW.action_kind
		AND basis.authority_mode = NEW.authority_mode
		AND basis.work_input_ref = NEW.work_input_ref
		AND basis.work_input_digest = NEW.work_input_digest
		AND work_input.work_input_digest = NEW.work_input_digest
		AND binding.project_root = NEW.project_root
		AND binding.binding_digest = NEW.project_binding_digest
		AND ` + sqliteUTCNanoLessOrEqual("basis.recorded_at", "NEW.checked_at") + `
		AND ` + sqliteUTCNanoLessOrEqual("basis.allowed_work_from", "NEW.checked_at") + `
		AND ` + sqliteUTCNanoLessOrEqual("NEW.checked_at", "basis.allowed_work_until") + `
		AND NEW.currentness_result = 'current'
		AND NEW.predicate_result = 'satisfied'
		AND NEW.admission_result = 'admitted'
		AND (
			(NEW.authority_mode = '` + profileAuthorityExplicitMode51 + `'
			 AND NEW.resolution_kind = '` + profileAuthorityExplicitResolution51 + `'
			 AND basis.config_carrier_ref IS NOT NULL)
			OR
			(NEW.authority_mode = '` + profileAuthorityStrictMode51 + `'
			 AND NEW.resolution_kind = '` + profileAuthorityStrictResolution51 + `'
			 AND EXISTS (
				SELECT 1
				FROM profile_declaration_authority_bases_v2 strict_basis
				JOIN profile_declaration_permissions_v2 permission
					ON permission.permission_ref = strict_basis.permission_ref
				WHERE strict_basis.basis_ref = basis.strict_authority_basis_ref
				AND strict_basis.basis_digest = basis.strict_authority_basis_digest
				AND permission.permission_ref = NEW.strict_permission_ref
				AND permission.permission_digest = NEW.strict_permission_digest
				AND permission.project_root = NEW.project_root
				AND ` + sqliteUTCNanoLessOrEqual("permission.valid_from", "NEW.checked_at") + `
				AND ` + sqliteUTCNanoLess("NEW.checked_at", "permission.valid_until") + `
			 )
			)
		)
	 ) BEGIN
		SELECT RAISE(ABORT, 'v3 profile authority resolution does not bind the exact selected authority branch and current project basis');
	 END`
}

func profileAuthorityResolutionCrossGenerationTrigger51() string {
	return `CREATE TRIGGER profile_declaration_authority_resolutions_v3_no_cross_generation_collision
	 BEFORE INSERT ON profile_declaration_authority_resolutions_v3
	 WHEN EXISTS (
		SELECT 1 FROM authority_resolution_records legacy
		WHERE legacy.authority_resolution_id = NEW.authority_resolution_ref
		OR legacy.authority_resolution_digest = NEW.authority_resolution_digest
	 ) OR EXISTS (
		SELECT 1 FROM authority_basis_resolutions legacy
		WHERE legacy.authority_resolution_id = NEW.authority_resolution_ref
		OR legacy.authority_resolution_digest = NEW.authority_resolution_digest
	 ) OR EXISTS (
		SELECT 1 FROM profile_declaration_authority_resolutions_v2 previous
		WHERE previous.authority_resolution_ref = NEW.authority_resolution_ref
		OR previous.authority_resolution_digest = NEW.authority_resolution_digest
	 ) OR EXISTS (
		SELECT 1 FROM authority_uses legacy
		WHERE legacy.single_use_key = (
			SELECT basis.single_use_key FROM profile_declaration_authority_bases_v3 basis
			WHERE basis.basis_ref = NEW.authority_basis_ref
		)
	 ) OR EXISTS (
		SELECT 1 FROM profile_declaration_authority_uses_v2 previous
		WHERE previous.single_use_key = (
			SELECT basis.single_use_key FROM profile_declaration_authority_bases_v3 basis
			WHERE basis.basis_ref = NEW.authority_basis_ref
		)
	 ) BEGIN
		SELECT RAISE(ABORT, 'v3 profile authority resolution collides with earlier authority history or a consumed single-use key');
	 END`
}

func projectProfileAdmissionRevisionCASTrigger51() string {
	return `CREATE TRIGGER project_profile_admissions_v3_revision_cas
	 BEFORE INSERT ON project_profile_admissions_v3
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
				SELECT ledger_revision FROM project_profile_revisions_v2 previous WHERE previous.project_root = NEW.project_root
				UNION ALL
				SELECT ledger_revision FROM project_profile_revisions_v3 current WHERE current.project_root = NEW.project_root
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

func projectProfileAdmissionExactSourcesTrigger51() string {
	return `CREATE TRIGGER project_profile_admissions_v3_exact_sources
	 BEFORE INSERT ON project_profile_admissions_v3
	 WHEN NOT EXISTS (
		SELECT 1
		FROM profile_declaration_authority_resolutions_v3 resolution
		JOIN profile_declaration_authority_bases_v3 authority_basis
			ON authority_basis.basis_ref = resolution.authority_basis_ref
		JOIN profile_onboarding_work_inputs_v1 work_input
			ON work_input.work_input_ref = authority_basis.work_input_ref
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
		AND resolution.authority_mode = NEW.authority_mode
		AND resolution.resolution_kind = NEW.resolution_kind
		AND resolution.project_binding_digest = NEW.project_binding_digest
		AND resolution.work_input_ref = NEW.work_input_ref
		AND resolution.work_input_digest = NEW.work_input_digest
		AND resolution.currentness_result = 'current'
		AND resolution.predicate_result = 'satisfied'
		AND resolution.admission_result = 'admitted'
		AND authority_basis.basis_digest = NEW.authority_basis_digest
		AND authority_basis.project_root = NEW.project_root
		AND authority_basis.authority_mode = NEW.authority_mode
		AND authority_basis.single_use_key = NEW.single_use_key
		AND work_input.work_input_digest = NEW.work_input_digest
		AND work_input.project_root = NEW.project_root
		AND json(work_input.profile_payload_json) = json(NEW.profile_payload_json)
		AND work_input.profile_payload_digest = NEW.profile_payload_digest
		AND work_record.work_record_digest = NEW.work_record_digest
		AND work_record.project_root = NEW.project_root
		AND work_record.method_description_ref = authority_basis.method_description_ref
		AND work_record.method_description_digest = authority_basis.method_description_digest
		AND work_record.method_contract_ref = authority_basis.method_contract_ref
		AND work_record.method_contract_digest = authority_basis.method_contract_digest
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
		AND ` + sqliteUTCNanoLessOrEqual("authority_basis.allowed_work_from", "work_record.work_from") + `
		AND ` + sqliteUTCNanoLessOrEqual("work_record.work_until", "authority_basis.allowed_work_until") + `
		AND ` + sqliteUTCNanoLessOrEqual("authority_basis.basis_observation_from", "work_record.basis_observation_from") + `
		AND ` + sqliteUTCNanoLessOrEqual("work_record.basis_observation_until", "authority_basis.basis_observation_until") + `
		AND ` + sqliteUTCNanoLessOrEqual("assignment.valid_from", "work_record.work_from") + `
		AND ` + sqliteUTCNanoLessOrEqual("work_record.work_until", "assignment.valid_until") + `
		AND ` + sqliteUTCNanoLessOrEqual("work_record.work_until", "NEW.recorded_at") + `
		AND ` + sqliteUTCNanoLessOrEqual("NEW.recorded_at", "authority_basis.allowed_work_until") + `
		AND (NEW.authority_mode = '` + profileAuthorityExplicitMode51 + `' OR EXISTS (
			SELECT 1 FROM profile_declaration_permissions_v2 permission
			WHERE permission.permission_ref = resolution.strict_permission_ref
			AND permission.permission_digest = resolution.strict_permission_digest
			AND ` + sqliteUTCNanoLess("NEW.recorded_at", "permission.valid_until") + `
		))
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
		AND json_extract(NEW.candidate_provenance_json, '$.classifier_version') = authority_basis.classifier_version
		AND json_extract(NEW.candidate_provenance_json, '$.policy_version') = authority_basis.policy_version
		AND json_extract(NEW.candidate_provenance_json, '$.session_ref') = authority_basis.future_work_session_ref
		AND json_extract(NEW.candidate_provenance_json, '$.payload_digest') = NEW.profile_payload_digest
		AND json_extract(NEW.candidate_provenance_json, '$.provenance_digest') = NEW.candidate_provenance_digest
	 ) BEGIN
		SELECT RAISE(ABORT, 'v3 profile admission does not bind exact WorkInput, authority branch, Work, RoleAssignment, observed basis, assessment, and provenance');
	 END`
}

func projectProfileAdmissionCrossGenerationTrigger51() string {
	return `CREATE TRIGGER project_profile_admissions_v3_no_cross_generation_collision
	 BEFORE INSERT ON project_profile_admissions_v3
	 WHEN EXISTS (
		SELECT 1 FROM project_profile_admissions legacy
		WHERE legacy.admission_id = NEW.admission_id
		OR legacy.admission_digest = NEW.admission_digest
		OR legacy.receipt_digest = NEW.receipt_digest
		OR legacy.authority_resolution_ref = NEW.authority_resolution_ref
		OR legacy.single_use_key = NEW.single_use_key
		OR (legacy.project_root = NEW.project_root AND legacy.ledger_revision = NEW.ledger_revision)
	 ) OR EXISTS (
		SELECT 1 FROM project_profile_admissions_v2 previous
		WHERE previous.admission_id = NEW.admission_id
		OR previous.admission_digest = NEW.admission_digest
		OR previous.receipt_digest = NEW.receipt_digest
		OR previous.authority_resolution_ref = NEW.authority_resolution_ref
		OR previous.single_use_key = NEW.single_use_key
		OR (previous.project_root = NEW.project_root AND previous.ledger_revision = NEW.ledger_revision)
	 ) OR EXISTS (
		SELECT 1 FROM authority_uses legacy WHERE legacy.single_use_key = NEW.single_use_key
	 ) OR EXISTS (
		SELECT 1 FROM profile_declaration_authority_uses_v2 previous WHERE previous.single_use_key = NEW.single_use_key
	 ) BEGIN
		SELECT RAISE(ABORT, 'v3 profile admission collides with cross-generation profile history or consumed authority');
	 END`
}

func profileAuthorityUseExactSourcesTrigger51() string {
	return `CREATE TRIGGER profile_declaration_authority_uses_v3_exact_sources
	 BEFORE INSERT ON profile_declaration_authority_uses_v3
	 WHEN NOT EXISTS (
		SELECT 1
		FROM profile_declaration_authority_resolutions_v3 resolution
		JOIN profile_declaration_authority_bases_v3 basis
			ON basis.basis_ref = resolution.authority_basis_ref
		JOIN profile_onboarding_work_inputs_v1 work_input
			ON work_input.work_input_ref = basis.work_input_ref
		JOIN project_profile_admissions_v3 admission
			ON admission.admission_id = NEW.committed_admission_ref
		JOIN profile_onboarding_work_records work_record
			ON work_record.work_record_ref = admission.work_record_ref
		WHERE resolution.authority_resolution_ref = NEW.authority_resolution_ref
		AND resolution.authority_resolution_digest = NEW.authority_resolution_digest
		AND resolution.project_root = NEW.project_root
		AND resolution.action_kind = NEW.action_kind
		AND resolution.authority_mode = NEW.authority_mode
		AND resolution.resolution_kind = NEW.resolution_kind
		AND resolution.project_binding_digest = NEW.project_binding_digest
		AND resolution.authority_basis_ref = NEW.authority_basis_ref
		AND resolution.authority_basis_digest = NEW.authority_basis_digest
		AND resolution.work_input_ref = NEW.work_input_ref
		AND resolution.work_input_digest = NEW.work_input_digest
		AND basis.basis_digest = NEW.authority_basis_digest
		AND basis.single_use_key = NEW.single_use_key
		AND work_input.work_input_digest = NEW.work_input_digest
		AND admission.admission_digest = NEW.committed_admission_digest
		AND admission.authority_resolution_ref = NEW.authority_resolution_ref
		AND admission.authority_resolution_digest = NEW.authority_resolution_digest
		AND admission.authority_basis_ref = NEW.authority_basis_ref
		AND admission.authority_basis_digest = NEW.authority_basis_digest
		AND admission.work_input_ref = NEW.work_input_ref
		AND admission.work_input_digest = NEW.work_input_digest
		AND admission.authority_mode = NEW.authority_mode
		AND admission.resolution_kind = NEW.resolution_kind
		AND admission.single_use_key = NEW.single_use_key
		AND admission.action_kind = NEW.action_kind
		AND admission.project_root = NEW.project_root
		AND admission.project_binding_digest = NEW.project_binding_digest
		AND admission.admission_request_digest = NEW.admission_request_digest
		AND admission.recorded_at = NEW.consumed_at
		AND work_record.work_record_digest = admission.work_record_digest
		AND ` + sqliteUTCNanoLessOrEqual("resolution.checked_at", "work_record.work_from") + `
		AND ` + sqliteUTCNanoLessOrEqual("work_record.work_until", "NEW.consumed_at") + `
		AND ` + sqliteUTCNanoLessOrEqual("NEW.consumed_at", "basis.allowed_work_until") + `
		AND (NEW.authority_mode = '` + profileAuthorityExplicitMode51 + `' OR EXISTS (
			SELECT 1 FROM profile_declaration_permissions_v2 permission
			WHERE permission.permission_ref = resolution.strict_permission_ref
			AND permission.permission_digest = resolution.strict_permission_digest
			AND ` + sqliteUTCNanoLess("NEW.consumed_at", "permission.valid_until") + `
		))
	 ) BEGIN
		SELECT RAISE(ABORT, 'v3 profile authority use does not bind exact resolution, WorkInput, authority branch, and committed admission');
	 END`
}

func profileAuthorityUseCrossGenerationTrigger51() string {
	return `CREATE TRIGGER profile_declaration_authority_uses_v3_no_cross_generation_collision
	 BEFORE INSERT ON profile_declaration_authority_uses_v3
	 WHEN EXISTS (
		SELECT 1 FROM authority_uses legacy
		WHERE legacy.use_id = NEW.use_ref
		OR legacy.authority_resolution_ref = NEW.authority_resolution_ref
		OR legacy.single_use_key = NEW.single_use_key
		OR legacy.committed_result_ref = NEW.committed_admission_ref
	 ) OR EXISTS (
		SELECT 1 FROM profile_declaration_authority_uses_v2 previous
		WHERE previous.use_ref = NEW.use_ref
		OR previous.authority_resolution_ref = NEW.authority_resolution_ref
		OR previous.single_use_key = NEW.single_use_key
		OR previous.committed_admission_ref = NEW.committed_admission_ref
	 ) OR EXISTS (
		SELECT 1 FROM project_profile_admissions legacy
		WHERE legacy.admission_id = NEW.committed_admission_ref
		OR legacy.single_use_key = NEW.single_use_key
	 ) OR EXISTS (
		SELECT 1 FROM project_profile_admissions_v2 previous
		WHERE previous.admission_id = NEW.committed_admission_ref
		OR previous.single_use_key = NEW.single_use_key
	 ) BEGIN
		SELECT RAISE(ABORT, 'v3 profile authority use collides with earlier authority or admission history');
	 END`
}

func projectProfileRevisionExactAdmissionTrigger51() string {
	return `CREATE TRIGGER project_profile_revisions_v3_exact_admission
	 BEFORE INSERT ON project_profile_revisions_v3
	 WHEN NOT EXISTS (
		SELECT 1
		FROM project_profile_admissions_v3 admission
		JOIN profile_declaration_authority_uses_v3 authority_use
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
		AND authority_use.authority_basis_ref = admission.authority_basis_ref
		AND authority_use.authority_basis_digest = admission.authority_basis_digest
		AND authority_use.work_input_ref = admission.work_input_ref
		AND authority_use.work_input_digest = admission.work_input_digest
		AND authority_use.authority_mode = admission.authority_mode
		AND authority_use.resolution_kind = admission.resolution_kind
		AND authority_use.single_use_key = admission.single_use_key
		AND authority_use.action_kind = admission.action_kind
		AND authority_use.project_root = admission.project_root
		AND authority_use.project_binding_digest = admission.project_binding_digest
		AND authority_use.admission_request_digest = admission.admission_request_digest
		AND authority_use.consumed_at = admission.recorded_at
	 ) BEGIN
		SELECT RAISE(ABORT, 'v3 project profile revision does not match canonical admission and authority use');
	 END`
}

func projectProfileRevisionCrossGenerationTrigger51() string {
	return `CREATE TRIGGER project_profile_revisions_v3_no_cross_generation_collision
	 BEFORE INSERT ON project_profile_revisions_v3
	 WHEN EXISTS (
		SELECT 1 FROM project_profile_revisions legacy
		WHERE (legacy.project_root = NEW.project_root AND legacy.ledger_revision = NEW.ledger_revision)
		OR legacy.admission_id = NEW.admission_id
		OR legacy.admission_digest = NEW.admission_digest
		OR legacy.receipt_digest = NEW.receipt_digest
	 ) OR EXISTS (
		SELECT 1 FROM project_profile_revisions_v2 previous
		WHERE (previous.project_root = NEW.project_root AND previous.ledger_revision = NEW.ledger_revision)
		OR previous.admission_id = NEW.admission_id
		OR previous.admission_digest = NEW.admission_digest
		OR previous.receipt_digest = NEW.receipt_digest
	 ) BEGIN
		SELECT RAISE(ABORT, 'v3 project profile revision collides with earlier history');
	 END`
}

func projectProfileDebtExactAdmissionTrigger51() string {
	return `CREATE TRIGGER project_profile_projection_debt_v3_exact_admission
	 BEFORE INSERT ON project_profile_projection_debt_v3
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
			AND authority_use.committed_admission_digest = NEW.admission_digest
		))
		OR (NEW.profile_revision_generation = 'v3' AND EXISTS (
			SELECT 1
			FROM project_profile_admissions_v3 admission
			JOIN project_profile_revisions_v3 revision ON revision.admission_id = admission.admission_id
			JOIN profile_declaration_authority_uses_v3 authority_use ON authority_use.committed_admission_ref = admission.admission_id
			WHERE admission.admission_id = NEW.admission_id
			AND admission.admission_digest = NEW.admission_digest
			AND admission.project_root = NEW.project_root
			AND admission.ledger_revision = NEW.ledger_revision
			AND admission.profile_payload_digest = NEW.profile_payload_digest
			AND revision.admission_digest = NEW.admission_digest
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
		OR (NEW.supersedes_event_generation = 'v3' AND EXISTS (
			SELECT 1 FROM project_profile_projection_debt_v3 opened
			WHERE opened.event_id = NEW.supersedes_event_id
			AND opened.debt_id = NEW.debt_id
			AND opened.admission_id = NEW.admission_id
			AND opened.admission_digest = NEW.admission_digest
			AND opened.event_kind = 'opened'
		))
	 )) BEGIN
		SELECT RAISE(ABORT, 'v3 projection debt does not bind the exact admission or opened debt event');
	 END`
}

func projectProfileDebtCrossGenerationTrigger51() string {
	return `CREATE TRIGGER project_profile_projection_debt_v3_no_cross_generation_collision
	 BEFORE INSERT ON project_profile_projection_debt_v3
	 WHEN EXISTS (
		SELECT 1 FROM project_profile_projection_debt legacy
		WHERE legacy.event_id = NEW.event_id
		OR (NEW.event_kind = 'opened' AND legacy.debt_id = NEW.debt_id)
	 ) OR EXISTS (
		SELECT 1 FROM project_profile_projection_debt_v2 previous
		WHERE previous.event_id = NEW.event_id
		OR (NEW.event_kind = 'opened' AND previous.debt_id = NEW.debt_id)
	 ) BEGIN
		SELECT RAISE(ABORT, 'v3 projection debt identity collides with earlier history');
	 END`
}

func currentProjectProfilesView51() string {
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
		UNION ALL
		SELECT 'v3', project_root, ledger_revision, configured_profile_kind,
			profile_payload_json, profile_payload_digest,
			receipt_json, receipt_digest, admission_id, admission_digest, recorded_at
		FROM project_profile_revisions_v3
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

func appendProjectProfileV3AppendOnlyTriggers51(statements []string) []string {
	return append(statements,
		`CREATE TRIGGER project_profile_revisions_v3_no_replace
		 BEFORE INSERT ON project_profile_revisions_v3
		 WHEN EXISTS (
			SELECT 1 FROM project_profile_revisions_v3 existing
			WHERE (existing.project_root = NEW.project_root AND existing.ledger_revision = NEW.ledger_revision)
			OR existing.admission_id = NEW.admission_id
			OR existing.admission_digest = NEW.admission_digest
			OR existing.receipt_digest = NEW.receipt_digest
		 ) BEGIN SELECT RAISE(ABORT, 'project_profile_revisions_v3 is append-only'); END`,
		`CREATE TRIGGER project_profile_revisions_v3_no_update
		 BEFORE UPDATE ON project_profile_revisions_v3 BEGIN
			SELECT RAISE(ABORT, 'project_profile_revisions_v3 is append-only');
		 END`,
		`CREATE TRIGGER project_profile_revisions_v3_no_delete
		 BEFORE DELETE ON project_profile_revisions_v3 BEGIN
			SELECT RAISE(ABORT, 'project_profile_revisions_v3 is append-only');
		 END`,
		`CREATE TRIGGER project_profile_projection_debt_v3_no_replace
		 BEFORE INSERT ON project_profile_projection_debt_v3
		 WHEN EXISTS (SELECT 1 FROM project_profile_projection_debt_v3 existing WHERE existing.event_id = NEW.event_id)
		 BEGIN SELECT RAISE(ABORT, 'project_profile_projection_debt_v3 is append-only'); END`,
		`CREATE TRIGGER project_profile_projection_debt_v3_no_update
		 BEFORE UPDATE ON project_profile_projection_debt_v3 BEGIN
			SELECT RAISE(ABORT, 'project_profile_projection_debt_v3 is append-only');
		 END`,
		`CREATE TRIGGER project_profile_projection_debt_v3_no_delete
		 BEFORE DELETE ON project_profile_projection_debt_v3 BEGIN
			SELECT RAISE(ABORT, 'project_profile_projection_debt_v3 is append-only');
		 END`,
	)
}

func appendProfileAuthorityUnionRootGuards51(
	statements []string,
	index int,
) []string {
	if index >= len(profileAuthorityUnionTables51) {
		return statements
	}
	table := profileAuthorityUnionTables51[index]
	trigger := "CREATE TRIGGER " + table + "_project_ledger_root " +
		"BEFORE INSERT ON " + table + " WHEN NOT EXISTS " +
		"(SELECT 1 FROM project_ledger_binding binding WHERE binding.project_root = NEW.project_root) " +
		"BEGIN SELECT RAISE(ABORT, '" + table + " does not match the bound project ledger root'); END"
	next := slices.Clone(statements)
	next = append(next, trigger)
	return appendProfileAuthorityUnionRootGuards51(next, index+1)
}

func appendProfileAdmissionV2WriteSeals51(
	statements []string,
	index int,
) []string {
	if index >= len(profileAdmissionV2WriteTables51) {
		return statements
	}
	table := profileAdmissionV2WriteTables51[index]
	trigger := "CREATE TRIGGER " + table + "_v51_writes_sealed " +
		"BEFORE INSERT ON " + table + " BEGIN " +
		"SELECT RAISE(ABORT, 'v2 profile admission writes are sealed after schema v51'); END"
	next := slices.Clone(statements)
	next = append(next, trigger)
	return appendProfileAdmissionV2WriteSeals51(next, index+1)
}
