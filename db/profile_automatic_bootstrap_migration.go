package db

import "fmt"

const (
	profileAutomaticAuthorityBasisTable55      = "profile_initial_bootstrap_authority_bases_v1"
	profileAutomaticAuthorityResolutionTable55 = "profile_initial_bootstrap_authority_resolutions_v1"
	profileAutomaticAuthorityUseTable55        = "profile_declaration_authority_uses_v4"
	projectProfileAdmissionTable55             = "project_profile_admissions_v4"
	projectProfileRevisionTable55              = "project_profile_revisions_v4"
	projectProfileDebtTable55                  = "project_profile_projection_debt_v4"

	profileAutomaticMode55       = "automatic_supported_singleton_init"
	profileAutomaticAction55     = "profile.apply_supported_singleton_default"
	profileAutomaticResolution55 = "deterministic_policy_satisfaction"
	profileDetectorDefault55     = "detector_default"
)

var profileAutomaticBootstrapMigration55 = Migration{
	Version:     55,
	Description: "Add automatic supported-singleton profile authority and admission origin",
	Apply:       applyProfileAutomaticBootstrapMigration55,
}

var profileAutomaticBootstrapTables55 = []string{
	profileAutomaticAuthorityBasisTable55,
	profileAutomaticAuthorityResolutionTable55,
	profileAutomaticAuthorityUseTable55,
	projectProfileAdmissionTable55,
	projectProfileRevisionTable55,
	projectProfileDebtTable55,
}

func applyProfileAutomaticBootstrapMigration55(
	tx MigrationTransaction,
	_ []Migration,
) error {
	if err := requireProfileAutomaticBootstrapSource55(tx); err != nil {
		return err
	}
	if err := requireAbsentProfileAutomaticBootstrapFootprint55(tx, 0); err != nil {
		return err
	}
	statements := profileAutomaticBootstrapStatements55()
	if err := executeStatements(tx, statements, 0); err != nil {
		return fmt.Errorf("install automatic profile bootstrap authority: %w", err)
	}
	if err := verifyForeignKeys(tx); err != nil {
		return fmt.Errorf("verify automatic profile bootstrap authority: %w", err)
	}
	return nil
}

func requireProfileAutomaticBootstrapSource55(tx MigrationTransaction) error {
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = 54",
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect automatic profile bootstrap source migration: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("automatic profile bootstrap requires schema version 54")
	}
	return nil
}

func requireAbsentProfileAutomaticBootstrapFootprint55(
	tx MigrationTransaction,
	index int,
) error {
	if index >= len(profileAutomaticBootstrapTables55) {
		return nil
	}
	table := profileAutomaticBootstrapTables55[index]
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect automatic profile bootstrap table %s: %w", table, err)
	}
	if count != 0 {
		return fmt.Errorf(
			"automatic profile bootstrap refused: unversioned table %s already exists",
			table,
		)
	}
	return requireAbsentProfileAutomaticBootstrapFootprint55(tx, index+1)
}

func profileAutomaticBootstrapStatements55() []string {
	statements := []string{
		profileAutomaticAuthorityBasisTableSQL55(),
		profileAutomaticAuthorityResolutionTableSQL55(),
		projectProfileAdmissionTableSQL55(),
		profileAutomaticAuthorityUseTableSQL55(),
		projectProfileRevisionTableSQL55(),
		projectProfileDebtTableSQL55(),
		profileAutomaticAuthorityBasisExactSourcesTrigger55(),
		profileAutomaticAuthorityResolutionExactSourcesTrigger55(),
		projectProfileAdmissionRevisionCASTrigger55(),
		projectProfileAdmissionExactSourcesTrigger55(),
		profileAutomaticAuthorityUseExactSourcesTrigger55(),
		projectProfileRevisionExactAdmissionTrigger55(),
		projectProfileDebtExactAdmissionTrigger55(),
		"DROP TRIGGER project_profile_admissions_v3_revision_cas",
		projectProfileAdmissionRevisionCASTrigger55ForV3(),
		"DROP VIEW current_project_profiles",
		currentProjectProfilesView55(),
		"CREATE INDEX idx_profile_initial_bootstrap_basis_project_recorded ON profile_initial_bootstrap_authority_bases_v1(project_root, recorded_at)",
		"CREATE INDEX idx_profile_initial_bootstrap_resolution_project_checked ON profile_initial_bootstrap_authority_resolutions_v1(project_root, checked_at)",
		"CREATE INDEX idx_project_profile_admissions_v4_project_revision ON project_profile_admissions_v4(project_root, ledger_revision)",
		"CREATE INDEX idx_project_profile_revisions_v4_project_revision ON project_profile_revisions_v4(project_root, ledger_revision)",
		"CREATE INDEX idx_project_profile_projection_debt_v4_open ON project_profile_projection_debt_v4(project_root, ledger_revision, event_kind)",
	}
	for _, table := range profileAutomaticBootstrapTables55 {
		statements = append(statements, automaticProfileRootGuard55(table))
	}
	for _, table := range profileAutomaticBootstrapTables55 {
		statements = append(
			statements,
			"CREATE TRIGGER "+table+"_no_update BEFORE UPDATE ON "+table+" BEGIN SELECT RAISE(ABORT, '"+table+" is append-only'); END",
			"CREATE TRIGGER "+table+"_no_delete BEFORE DELETE ON "+table+" BEGIN SELECT RAISE(ABORT, '"+table+" is append-only'); END",
		)
	}
	return statements
}

func profileAutomaticAuthorityBasisTableSQL55() string {
	return `CREATE TABLE profile_initial_bootstrap_authority_bases_v1 (
		basis_ref TEXT PRIMARY KEY CHECK(basis_ref GLOB 'profile-authority-basis:?*'),
		basis_digest TEXT NOT NULL UNIQUE CHECK(length(basis_digest) = 71 AND substr(basis_digest, 1, 7) = 'sha256:'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		action_kind TEXT NOT NULL CHECK(action_kind = '` + profileAutomaticAction55 + `'),
		authority_mode TEXT NOT NULL CHECK(authority_mode = '` + profileAutomaticMode55 + `'),
		profile_origin TEXT NOT NULL CHECK(profile_origin = '` + profileDetectorDefault55 + `'),
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
		suggestion_ref TEXT NOT NULL CHECK(suggestion_ref GLOB 'profile-suggestion:sha256:*'),
		observation_digest TEXT NOT NULL CHECK(length(observation_digest) = 71 AND substr(observation_digest, 1, 7) = 'sha256:'),
		future_work_session_ref TEXT NOT NULL CHECK(future_work_session_ref != ''),
		allowed_work_from TEXT NOT NULL,
		allowed_work_until TEXT NOT NULL,
		basis_observation_from TEXT NOT NULL,
		basis_observation_until TEXT NOT NULL,
		single_use_key TEXT NOT NULL UNIQUE CHECK(single_use_key != ''),
		canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
		recorded_at TEXT NOT NULL,
		CHECK(COALESCE(json_extract(canonical_json, '$.schema') = 'haft.profile-authority.automatic-basis/v1', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.basis_ref') = basis_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.project_root') = project_root, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.action_kind') = action_kind, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_mode') = authority_mode, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.profile_origin') = profile_origin, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.work_input_ref') = work_input_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.work_input_digest') = work_input_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.classifier_version') = classifier_version, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.policy_version') = policy_version, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.suggestion_ref') = suggestion_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.observation_digest') = observation_digest, 0)),
		CHECK(` + sqliteCanonicalUTCNanoShape("allowed_work_from") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("allowed_work_until") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("basis_observation_from") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("basis_observation_until") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		CHECK(` + sqliteUTCNanoLess("allowed_work_from", "allowed_work_until") + `),
		CHECK(` + sqliteUTCNanoLess("basis_observation_from", "basis_observation_until") + `)
	) WITHOUT ROWID`
}

func profileAutomaticAuthorityResolutionTableSQL55() string {
	return `CREATE TABLE profile_initial_bootstrap_authority_resolutions_v1 (
		authority_resolution_ref TEXT PRIMARY KEY CHECK(authority_resolution_ref GLOB 'profile-authority-resolution:?*'),
		authority_resolution_digest TEXT NOT NULL UNIQUE CHECK(length(authority_resolution_digest) = 71 AND substr(authority_resolution_digest, 1, 7) = 'sha256:'),
		authority_basis_ref TEXT NOT NULL UNIQUE REFERENCES profile_initial_bootstrap_authority_bases_v1(basis_ref),
		authority_basis_digest TEXT NOT NULL CHECK(length(authority_basis_digest) = 71 AND substr(authority_basis_digest, 1, 7) = 'sha256:'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		action_kind TEXT NOT NULL CHECK(action_kind = '` + profileAutomaticAction55 + `'),
		authority_mode TEXT NOT NULL CHECK(authority_mode = '` + profileAutomaticMode55 + `'),
		resolution_kind TEXT NOT NULL CHECK(resolution_kind = '` + profileAutomaticResolution55 + `'),
		profile_origin TEXT NOT NULL CHECK(profile_origin = '` + profileDetectorDefault55 + `'),
		work_input_ref TEXT NOT NULL UNIQUE REFERENCES profile_onboarding_work_inputs_v1(work_input_ref),
		work_input_digest TEXT NOT NULL CHECK(length(work_input_digest) = 71 AND substr(work_input_digest, 1, 7) = 'sha256:'),
		project_binding_digest TEXT NOT NULL CHECK(length(project_binding_digest) = 71 AND substr(project_binding_digest, 1, 7) = 'sha256:'),
		detector_version TEXT NOT NULL CHECK(detector_version != ''),
		detector_policy_version TEXT NOT NULL CHECK(detector_policy_version != ''),
		suggestion_ref TEXT NOT NULL CHECK(suggestion_ref GLOB 'profile-suggestion:sha256:*'),
		observation_digest TEXT NOT NULL CHECK(length(observation_digest) = 71 AND substr(observation_digest, 1, 7) = 'sha256:'),
		verifier_identity TEXT NOT NULL CHECK(verifier_identity = 'haft-core'),
		verifier_version TEXT NOT NULL CHECK(verifier_version != ''),
		verification_policy_ref TEXT NOT NULL CHECK(verification_policy_ref != ''),
		verification_policy_digest TEXT NOT NULL CHECK(length(verification_policy_digest) = 71 AND substr(verification_policy_digest, 1, 7) = 'sha256:'),
		checked_at TEXT NOT NULL,
		currentness_result TEXT NOT NULL CHECK(currentness_result = 'current'),
		predicate_result TEXT NOT NULL CHECK(predicate_result = 'satisfied'),
		admission_result TEXT NOT NULL CHECK(admission_result = 'admitted'),
		canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
		recorded_at TEXT NOT NULL,
		CHECK(COALESCE(json_extract(canonical_json, '$.schema') = 'haft.profile-authority.automatic-resolution/v1', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_resolution_ref') = authority_resolution_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_basis_ref') = authority_basis_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_basis_digest') = authority_basis_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.project_root') = project_root, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.action_kind') = action_kind, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_mode') = authority_mode, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.resolution_kind') = resolution_kind, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.profile_origin') = profile_origin, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.observation_digest') = observation_digest, 0)),
		CHECK(recorded_at = checked_at),
		CHECK(` + sqliteCanonicalUTCNanoShape("checked_at") + `)
	) WITHOUT ROWID`
}

func projectProfileAdmissionTableSQL55() string {
	return `CREATE TABLE project_profile_admissions_v4 (
		admission_id TEXT PRIMARY KEY,
		action_kind TEXT NOT NULL CHECK(action_kind = '` + profileAutomaticAction55 + `'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		authority_mode TEXT NOT NULL CHECK(authority_mode = '` + profileAutomaticMode55 + `'),
		resolution_kind TEXT NOT NULL CHECK(resolution_kind = '` + profileAutomaticResolution55 + `'),
		profile_origin TEXT NOT NULL CHECK(profile_origin = '` + profileDetectorDefault55 + `'),
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
		authority_basis_ref TEXT NOT NULL REFERENCES profile_initial_bootstrap_authority_bases_v1(basis_ref),
		authority_basis_digest TEXT NOT NULL CHECK(length(authority_basis_digest) = 71 AND substr(authority_basis_digest, 1, 7) = 'sha256:'),
		authority_resolution_ref TEXT NOT NULL UNIQUE REFERENCES profile_initial_bootstrap_authority_resolutions_v1(authority_resolution_ref),
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
		CHECK(COALESCE(json_extract(receipt_json, '$.authority_resolution_record_ref') = authority_resolution_ref, 0)),
		CHECK(COALESCE(json_extract(receipt_json, '$.authority_resolution_record_digest') = authority_resolution_digest, 0)),
		CHECK(COALESCE(json_extract(admission_json, '$.admission_record_ref') = admission_id, 0)),
		CHECK(json(json_extract(admission_json, '$.payload')) = json(profile_payload_json)),
		CHECK(json(json_extract(admission_json, '$.candidate_provenance')) = json(candidate_provenance_json)),
		CHECK(json(json_extract(admission_json, '$.receipt')) = json(receipt_json)),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `)
	) WITHOUT ROWID`
}

func profileAutomaticAuthorityUseTableSQL55() string {
	return `CREATE TABLE profile_declaration_authority_uses_v4 (
		use_ref TEXT PRIMARY KEY CHECK(use_ref GLOB 'profile-authority-use:?*'),
		use_digest TEXT NOT NULL UNIQUE CHECK(length(use_digest) = 71 AND substr(use_digest, 1, 7) = 'sha256:'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		action_kind TEXT NOT NULL CHECK(action_kind = '` + profileAutomaticAction55 + `'),
		authority_mode TEXT NOT NULL CHECK(authority_mode = '` + profileAutomaticMode55 + `'),
		resolution_kind TEXT NOT NULL CHECK(resolution_kind = '` + profileAutomaticResolution55 + `'),
		profile_origin TEXT NOT NULL CHECK(profile_origin = '` + profileDetectorDefault55 + `'),
		project_binding_digest TEXT NOT NULL CHECK(length(project_binding_digest) = 71 AND substr(project_binding_digest, 1, 7) = 'sha256:'),
		authority_resolution_ref TEXT NOT NULL UNIQUE REFERENCES profile_initial_bootstrap_authority_resolutions_v1(authority_resolution_ref),
		authority_resolution_digest TEXT NOT NULL CHECK(length(authority_resolution_digest) = 71 AND substr(authority_resolution_digest, 1, 7) = 'sha256:'),
		authority_basis_ref TEXT NOT NULL UNIQUE REFERENCES profile_initial_bootstrap_authority_bases_v1(basis_ref),
		authority_basis_digest TEXT NOT NULL CHECK(length(authority_basis_digest) = 71 AND substr(authority_basis_digest, 1, 7) = 'sha256:'),
		work_input_ref TEXT NOT NULL UNIQUE REFERENCES profile_onboarding_work_inputs_v1(work_input_ref),
		work_input_digest TEXT NOT NULL CHECK(length(work_input_digest) = 71 AND substr(work_input_digest, 1, 7) = 'sha256:'),
		single_use_key TEXT NOT NULL UNIQUE CHECK(single_use_key != ''),
		admission_request_digest TEXT NOT NULL CHECK(length(admission_request_digest) = 71 AND substr(admission_request_digest, 1, 7) = 'sha256:'),
		committed_admission_ref TEXT NOT NULL UNIQUE REFERENCES project_profile_admissions_v4(admission_id),
		committed_admission_digest TEXT NOT NULL UNIQUE CHECK(length(committed_admission_digest) = 71 AND substr(committed_admission_digest, 1, 7) = 'sha256:'),
		canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
		consumed_at TEXT NOT NULL,
		recorded_at TEXT NOT NULL,
		CHECK(recorded_at = consumed_at),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_mode') = authority_mode, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.resolution_kind') = resolution_kind, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.profile_origin') = profile_origin, 0)),
		CHECK(` + sqliteCanonicalUTCNanoShape("consumed_at") + `)
	) WITHOUT ROWID`
}

func projectProfileRevisionTableSQL55() string {
	return `CREATE TABLE project_profile_revisions_v4 (
		project_root TEXT NOT NULL,
		ledger_revision INTEGER NOT NULL CHECK(ledger_revision > 0),
		configured_profile_kind TEXT NOT NULL CHECK(configured_profile_kind = 'Declared'),
		profile_origin TEXT NOT NULL CHECK(profile_origin = '` + profileDetectorDefault55 + `'),
		profile_payload_json TEXT NOT NULL CHECK(json_valid(profile_payload_json)),
		profile_payload_digest TEXT NOT NULL CHECK(length(profile_payload_digest) = 71 AND substr(profile_payload_digest, 1, 7) = 'sha256:'),
		receipt_json TEXT NOT NULL CHECK(json_valid(receipt_json)),
		receipt_digest TEXT NOT NULL CHECK(length(receipt_digest) = 71 AND substr(receipt_digest, 1, 7) = 'sha256:'),
		admission_id TEXT NOT NULL UNIQUE REFERENCES project_profile_admissions_v4(admission_id),
		admission_digest TEXT NOT NULL UNIQUE CHECK(length(admission_digest) = 71 AND substr(admission_digest, 1, 7) = 'sha256:'),
		recorded_at TEXT NOT NULL,
		PRIMARY KEY(project_root, ledger_revision),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `)
	) WITHOUT ROWID`
}

func projectProfileDebtTableSQL55() string {
	return `CREATE TABLE project_profile_projection_debt_v4 (
		event_id TEXT PRIMARY KEY CHECK(event_id != ''),
		debt_id TEXT NOT NULL CHECK(debt_id != ''),
		event_kind TEXT NOT NULL CHECK(event_kind IN ('opened', 'resolved')),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		ledger_revision INTEGER NOT NULL CHECK(ledger_revision > 0),
		profile_revision_generation TEXT NOT NULL CHECK(profile_revision_generation = 'v4'),
		admission_id TEXT NOT NULL REFERENCES project_profile_admissions_v4(admission_id),
		admission_digest TEXT NOT NULL CHECK(length(admission_digest) = 71 AND substr(admission_digest, 1, 7) = 'sha256:'),
		profile_payload_digest TEXT NOT NULL CHECK(length(profile_payload_digest) = 71 AND substr(profile_payload_digest, 1, 7) = 'sha256:'),
		projection_path TEXT NOT NULL CHECK(projection_path != ''),
		reason_code TEXT NOT NULL CHECK(reason_code != ''),
		detail TEXT NOT NULL CHECK(detail != ''),
		expected_projection_digest TEXT NOT NULL CHECK(length(expected_projection_digest) = 71 AND substr(expected_projection_digest, 1, 7) = 'sha256:'),
		observed_projection_digest TEXT NOT NULL DEFAULT '' CHECK(observed_projection_digest = '' OR (length(observed_projection_digest) = 71 AND substr(observed_projection_digest, 1, 7) = 'sha256:')),
		supersedes_event_generation TEXT CHECK(supersedes_event_generation IS NULL OR supersedes_event_generation IN ('v1', 'v2', 'v3', 'v4')),
		supersedes_event_id TEXT,
		recorded_at TEXT NOT NULL,
		CHECK((event_kind = 'opened' AND supersedes_event_generation IS NULL AND supersedes_event_id IS NULL) OR (event_kind = 'resolved' AND supersedes_event_generation IS NOT NULL AND supersedes_event_id IS NOT NULL)),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `)
	) WITHOUT ROWID`
}

func profileAutomaticAuthorityBasisExactSourcesTrigger55() string {
	return `CREATE TRIGGER profile_initial_bootstrap_authority_bases_v1_exact_sources
	 BEFORE INSERT ON profile_initial_bootstrap_authority_bases_v1
	 WHEN NOT EXISTS (
		SELECT 1 FROM profile_onboarding_work_inputs_v1 work_input
		JOIN profile_author_role_assignments assignment ON assignment.role_assignment_ref = NEW.profile_author_role_assignment_ref
		JOIN profile_onboarding_method_descriptions description ON description.method_description_ref = NEW.method_description_ref
		JOIN profile_onboarding_method_contracts contract ON contract.method_contract_ref = NEW.method_contract_ref
		WHERE work_input.work_input_ref = NEW.work_input_ref
		AND work_input.work_input_digest = NEW.work_input_digest
		AND work_input.project_root = NEW.project_root
		AND work_input.detector_version = NEW.classifier_version
		AND work_input.policy_version = NEW.policy_version
		AND work_input.suggestion_ref = NEW.suggestion_ref
		AND work_input.observation_digest = NEW.observation_digest
		AND assignment.role_assignment_digest = NEW.profile_author_role_assignment_digest
		AND description.method_description_digest = NEW.method_description_digest
		AND contract.method_contract_digest = NEW.method_contract_digest
		AND contract.method_description_ref = NEW.method_description_ref
	 ) BEGIN SELECT RAISE(ABORT, 'automatic profile basis does not bind exact detector WorkInput, Haft Core support, and policy provenance'); END`
}

func profileAutomaticAuthorityResolutionExactSourcesTrigger55() string {
	return `CREATE TRIGGER profile_initial_bootstrap_authority_resolutions_v1_exact_sources
	 BEFORE INSERT ON profile_initial_bootstrap_authority_resolutions_v1
	 WHEN NOT EXISTS (
		SELECT 1 FROM profile_initial_bootstrap_authority_bases_v1 basis
		JOIN project_ledger_binding binding ON binding.project_root = basis.project_root
		WHERE basis.basis_ref = NEW.authority_basis_ref
		AND basis.basis_digest = NEW.authority_basis_digest
		AND basis.project_root = NEW.project_root
		AND basis.work_input_ref = NEW.work_input_ref
		AND basis.work_input_digest = NEW.work_input_digest
		AND basis.classifier_version = NEW.detector_version
		AND basis.policy_version = NEW.detector_policy_version
		AND basis.suggestion_ref = NEW.suggestion_ref
		AND basis.observation_digest = NEW.observation_digest
		AND binding.binding_digest = NEW.project_binding_digest
		AND ` + sqliteUTCNanoLessOrEqual("basis.recorded_at", "NEW.checked_at") + `
		AND ` + sqliteUTCNanoLessOrEqual("basis.allowed_work_from", "NEW.checked_at") + `
		AND ` + sqliteUTCNanoLessOrEqual("NEW.checked_at", "basis.allowed_work_until") + `
	 ) BEGIN SELECT RAISE(ABORT, 'automatic profile resolution does not bind exact current detector policy and project basis'); END`
}

func projectProfileAdmissionRevisionCASTrigger55() string {
	return projectProfileAdmissionRevisionCAS55(
		"project_profile_admissions_v4_revision_cas",
		"project_profile_admissions_v4",
	)
}

func projectProfileAdmissionRevisionCASTrigger55ForV3() string {
	return projectProfileAdmissionRevisionCAS55(
		"project_profile_admissions_v3_revision_cas",
		"project_profile_admissions_v3",
	)
}

func projectProfileAdmissionRevisionCAS55(trigger string, table string) string {
	return `CREATE TRIGGER ` + trigger + ` BEFORE INSERT ON ` + table + `
	 WHEN NOT EXISTS (
		SELECT 1 FROM (
			SELECT COUNT(*) AS revision_count,
				COUNT(DISTINCT ledger_revision) AS distinct_count,
				COALESCE(MIN(ledger_revision), 0) AS minimum_revision,
				COALESCE(MAX(ledger_revision), 0) AS maximum_revision
			FROM (
				SELECT ledger_revision FROM project_profile_revisions legacy WHERE legacy.project_root = NEW.project_root
				UNION ALL SELECT ledger_revision FROM project_profile_revisions_v2 previous WHERE previous.project_root = NEW.project_root
				UNION ALL SELECT ledger_revision FROM project_profile_revisions_v3 current WHERE current.project_root = NEW.project_root
				UNION ALL SELECT ledger_revision FROM project_profile_revisions_v4 automatic WHERE automatic.project_root = NEW.project_root
			)
		) chain
		WHERE (NEW.expected_ledger_revision = 0 AND chain.revision_count = 0)
		OR (NEW.expected_ledger_revision > 0
			AND chain.revision_count = NEW.expected_ledger_revision
			AND chain.distinct_count = chain.revision_count
			AND chain.minimum_revision = 1
			AND chain.maximum_revision = NEW.expected_ledger_revision)
	 ) BEGIN SELECT RAISE(ABORT, 'project profile cross-generation ledger revision conflict'); END`
}

func projectProfileAdmissionExactSourcesTrigger55() string {
	return `CREATE TRIGGER project_profile_admissions_v4_exact_sources
	 BEFORE INSERT ON project_profile_admissions_v4
	 WHEN NOT EXISTS (
		SELECT 1 FROM profile_initial_bootstrap_authority_resolutions_v1 resolution
		JOIN profile_initial_bootstrap_authority_bases_v1 basis ON basis.basis_ref = resolution.authority_basis_ref
		JOIN profile_onboarding_work_inputs_v1 work_input ON work_input.work_input_ref = basis.work_input_ref
		JOIN profile_onboarding_work_records work_record ON work_record.work_record_ref = NEW.work_record_ref
		JOIN profile_author_role_assignments assignment ON assignment.role_assignment_ref = NEW.profile_author_role_assignment_ref
		JOIN observed_project_bases observed_basis ON observed_basis.observed_project_basis_ref = NEW.observed_project_basis_ref
		JOIN profile_onboarding_outcome_assessments assessment ON assessment.outcome_assessment_ref = NEW.outcome_assessment_ref
		JOIN profile_onboarding_effects work_effect ON work_effect.effect_ref = assessment.effect_ref
		WHERE resolution.authority_resolution_ref = NEW.authority_resolution_ref
		AND resolution.authority_resolution_digest = NEW.authority_resolution_digest
		AND resolution.authority_basis_ref = NEW.authority_basis_ref
		AND resolution.authority_basis_digest = NEW.authority_basis_digest
		AND resolution.project_root = NEW.project_root
		AND resolution.project_binding_digest = NEW.project_binding_digest
		AND resolution.work_input_ref = NEW.work_input_ref
		AND resolution.work_input_digest = NEW.work_input_digest
		AND basis.single_use_key = NEW.single_use_key
		AND work_input.work_input_digest = NEW.work_input_digest
		AND json(work_input.profile_payload_json) = json(NEW.profile_payload_json)
		AND work_input.profile_payload_digest = NEW.profile_payload_digest
		AND work_record.work_record_digest = NEW.work_record_digest
		AND work_record.project_root = NEW.project_root
		AND work_record.profile_payload_digest = NEW.profile_payload_digest
		AND work_record.profile_author_role_assignment_ref = NEW.profile_author_role_assignment_ref
		AND work_record.profile_author_role_assignment_digest = NEW.profile_author_role_assignment_digest
		AND observed_basis.observed_project_basis_digest = NEW.observed_project_basis_digest
		AND assessment.outcome_assessment_digest = NEW.outcome_assessment_digest
		AND work_effect.work_record_ref = NEW.work_record_ref
		AND work_effect.work_record_digest = NEW.work_record_digest
		AND work_effect.profile_payload_digest = NEW.profile_payload_digest
		AND ` + sqliteUTCNanoLessOrEqual("resolution.checked_at", "work_record.work_from") + `
		AND ` + sqliteUTCNanoLessOrEqual("work_record.work_until", "NEW.recorded_at") + `
	 ) BEGIN SELECT RAISE(ABORT, 'automatic profile admission does not bind exact detector authority, Work, payload, and Haft Core provenance'); END`
}

func profileAutomaticAuthorityUseExactSourcesTrigger55() string {
	return `CREATE TRIGGER profile_declaration_authority_uses_v4_exact_sources
	 BEFORE INSERT ON profile_declaration_authority_uses_v4
	 WHEN NOT EXISTS (
		SELECT 1 FROM profile_initial_bootstrap_authority_resolutions_v1 resolution
		JOIN profile_initial_bootstrap_authority_bases_v1 basis ON basis.basis_ref = resolution.authority_basis_ref
		JOIN project_profile_admissions_v4 admission ON admission.admission_id = NEW.committed_admission_ref
		WHERE resolution.authority_resolution_ref = NEW.authority_resolution_ref
		AND resolution.authority_resolution_digest = NEW.authority_resolution_digest
		AND resolution.authority_basis_ref = NEW.authority_basis_ref
		AND resolution.authority_basis_digest = NEW.authority_basis_digest
		AND basis.single_use_key = NEW.single_use_key
		AND admission.admission_digest = NEW.committed_admission_digest
		AND admission.admission_request_digest = NEW.admission_request_digest
		AND admission.authority_resolution_ref = NEW.authority_resolution_ref
		AND admission.authority_basis_ref = NEW.authority_basis_ref
		AND admission.work_input_ref = NEW.work_input_ref
		AND admission.work_input_digest = NEW.work_input_digest
		AND admission.recorded_at = NEW.consumed_at
	 ) BEGIN SELECT RAISE(ABORT, 'automatic profile authority use does not bind exact resolution and admission'); END`
}

func projectProfileRevisionExactAdmissionTrigger55() string {
	return `CREATE TRIGGER project_profile_revisions_v4_exact_admission
	 BEFORE INSERT ON project_profile_revisions_v4
	 WHEN NOT EXISTS (
		SELECT 1 FROM project_profile_admissions_v4 admission
		JOIN profile_declaration_authority_uses_v4 authority_use ON authority_use.committed_admission_ref = admission.admission_id
		WHERE admission.admission_id = NEW.admission_id
		AND admission.admission_digest = NEW.admission_digest
		AND admission.project_root = NEW.project_root
		AND admission.ledger_revision = NEW.ledger_revision
		AND admission.profile_payload_json = NEW.profile_payload_json
		AND admission.profile_payload_digest = NEW.profile_payload_digest
		AND admission.receipt_json = NEW.receipt_json
		AND admission.receipt_digest = NEW.receipt_digest
		AND admission.profile_origin = NEW.profile_origin
		AND authority_use.committed_admission_digest = NEW.admission_digest
	 ) BEGIN SELECT RAISE(ABORT, 'automatic project profile revision does not match exact admission and authority use'); END`
}

func projectProfileDebtExactAdmissionTrigger55() string {
	return `CREATE TRIGGER project_profile_projection_debt_v4_exact_admission
	 BEFORE INSERT ON project_profile_projection_debt_v4
	 WHEN NOT EXISTS (
		SELECT 1 FROM project_profile_admissions_v4 admission
		JOIN project_profile_revisions_v4 revision ON revision.admission_id = admission.admission_id
		WHERE admission.admission_id = NEW.admission_id
		AND admission.admission_digest = NEW.admission_digest
		AND admission.project_root = NEW.project_root
		AND admission.ledger_revision = NEW.ledger_revision
		AND admission.profile_payload_digest = NEW.profile_payload_digest
	 ) BEGIN SELECT RAISE(ABORT, 'automatic projection debt does not bind exact admission'); END`
}

func currentProjectProfilesView55() string {
	return `CREATE VIEW current_project_profiles AS
	WITH all_revisions (
		storage_generation, project_root, ledger_revision, configured_profile_kind,
		profile_payload_json, profile_payload_digest,
		receipt_json, receipt_digest, admission_id, admission_digest, recorded_at,
		profile_origin
	) AS (
		SELECT 'v1', project_root, ledger_revision, configured_profile_kind,
			profile_payload_json, profile_payload_digest,
			receipt_json, receipt_digest, admission_id, admission_digest, recorded_at,
			'legacy_unknown' FROM project_profile_revisions
		UNION ALL
		SELECT 'v2', project_root, ledger_revision, configured_profile_kind,
			profile_payload_json, profile_payload_digest,
			receipt_json, receipt_digest, admission_id, admission_digest, recorded_at,
			'legacy_unknown' FROM project_profile_revisions_v2
		UNION ALL
		SELECT 'v3', project_root, ledger_revision, configured_profile_kind,
			profile_payload_json, profile_payload_digest,
			receipt_json, receipt_digest, admission_id, admission_digest, recorded_at,
			'explicit_operator' FROM project_profile_revisions_v3
		UNION ALL
		SELECT 'v4', project_root, ledger_revision, configured_profile_kind,
			profile_payload_json, profile_payload_digest,
			receipt_json, receipt_digest, admission_id, admission_digest, recorded_at,
			profile_origin FROM project_profile_revisions_v4
	), valid_roots AS (
		SELECT project_root FROM all_revisions GROUP BY project_root
		HAVING MIN(ledger_revision) = 1
		AND COUNT(*) = MAX(ledger_revision)
		AND COUNT(DISTINCT ledger_revision) = COUNT(*)
	)
	SELECT revision.storage_generation,
		revision.project_root, revision.ledger_revision, revision.configured_profile_kind,
		revision.profile_payload_json, revision.profile_payload_digest,
		revision.receipt_json, revision.receipt_digest,
		revision.admission_id, revision.admission_digest, revision.recorded_at,
		revision.profile_origin
	FROM all_revisions revision
	JOIN valid_roots valid ON valid.project_root = revision.project_root
	WHERE NOT EXISTS (
		SELECT 1 FROM all_revisions newer
		WHERE newer.project_root = revision.project_root
		AND newer.ledger_revision > revision.ledger_revision
	)`
}

func automaticProfileRootGuard55(table string) string {
	return "CREATE TRIGGER " + table + "_project_ledger_root " +
		"BEFORE INSERT ON " + table + " WHEN NOT EXISTS " +
		"(SELECT 1 FROM project_ledger_binding binding WHERE binding.project_root = NEW.project_root) " +
		"BEGIN SELECT RAISE(ABORT, '" + table + " does not match the bound project ledger root'); END"
}
