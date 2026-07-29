package db

import (
	"fmt"
	"slices"
)

var authorityBasisMigration38 = Migration{
	Version:     38,
	Description: "Normalized manual SpeechAct authority basis graph",
	Apply:       applyAuthorityBasisMigration38,
}

var authorityBasisMigration38Tables = []string{
	"speech_act_method_descriptions",
	"speech_act_context_policies",
	"profile_declaration_authorization_contents",
	"terminal_capture_records",
	"speech_act_role_assignments",
	"speech_acts",
	"profile_declaration_permissions",
	"speech_act_instituted_effects",
	"authority_basis_presentations",
	"authority_basis_resolutions",
}

var preV38AuthorityBearingTables = []string{
	"authority_presentations",
	"authority_resolution_records",
	"authority_uses",
	"migration_review_speech_acts",
	"migration_review_admissions",
	"project_profile_admissions",
	"project_profile_revisions",
}

func applyAuthorityBasisMigration38(
	tx MigrationTransaction,
	_ []Migration,
) error {
	if err := requireAbsentAuthorityBasisV38Footprint(tx, 0); err != nil {
		return err
	}
	if err := requireEmptyPreV38AuthorityBearingTables(tx, 0); err != nil {
		return err
	}
	statements := authorityBasisMigration38Statements()
	if err := executeStatements(tx, statements, 0); err != nil {
		return fmt.Errorf("install canonical v38 authority source closure: %w", err)
	}
	if err := verifyForeignKeys(tx); err != nil {
		return fmt.Errorf("verify canonical v38 authority source closure: %w", err)
	}
	return nil
}

func requireAbsentAuthorityBasisV38Footprint(
	tx MigrationTransaction,
	index int,
) error {
	if index >= len(authorityBasisMigration38Tables) {
		return nil
	}
	table := authorityBasisMigration38Tables[index]
	var count int
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect v38 authority table %s: %w", table, err)
	}
	if count != 0 {
		return fmt.Errorf(
			"authority source migration refused: unversioned v38 table %s already exists; unknown partial or hybrid schema requires manual review",
			table,
		)
	}
	return requireAbsentAuthorityBasisV38Footprint(tx, index+1)
}

func requireEmptyPreV38AuthorityBearingTables(
	tx MigrationTransaction,
	index int,
) error {
	if index >= len(preV38AuthorityBearingTables) {
		return nil
	}
	table := preV38AuthorityBearingTables[index]
	query := "SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table)
	var count int64
	if err := tx.QueryRow(query).Scan(&count); err != nil {
		return fmt.Errorf("inspect pre-v38 authority table %s: %w", table, err)
	}
	if count != 0 {
		return fmt.Errorf(
			"authority source migration refused: pre-v38 table %s contains %d row(s); presentation-only authority cannot be upgraded automatically",
			table,
			count,
		)
	}
	return requireEmptyPreV38AuthorityBearingTables(tx, index+1)
}

func authorityBasisMigration38Statements() []string {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS speech_act_method_descriptions (
			method_description_ref TEXT PRIMARY KEY,
			method_description_digest TEXT NOT NULL UNIQUE CHECK(length(method_description_digest) = 71 AND substr(method_description_digest, 1, 7) = 'sha256:'),
			method_ref TEXT NOT NULL,
			procedure_ref TEXT NOT NULL,
			bounded_context_ref TEXT NOT NULL,
			procedure_semantics TEXT NOT NULL CHECK(length(procedure_semantics) BETWEEN 1 AND 8192 AND trim(procedure_semantics) = procedure_semantics),
			canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
			recorded_at TEXT NOT NULL,
			CHECK(json_extract(canonical_json, '$.schema') = 'haft.authority.speech-act-method-description/v1'),
			CHECK(json_extract(canonical_json, '$.method_description_ref') = method_description_ref),
			CHECK(json_extract(canonical_json, '$.method_ref') = method_ref),
			CHECK(json_extract(canonical_json, '$.procedure_ref') = procedure_ref),
			CHECK(json_extract(canonical_json, '$.bounded_context_ref') = bounded_context_ref),
			CHECK(json_extract(canonical_json, '$.procedure_semantics') = procedure_semantics)
		) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS speech_act_context_policies (
			context_policy_ref TEXT PRIMARY KEY,
			context_policy_digest TEXT NOT NULL UNIQUE CHECK(length(context_policy_digest) = 71 AND substr(context_policy_digest, 1, 7) = 'sha256:'),
			bounded_context_ref TEXT NOT NULL,
			recognized_act_type_ref TEXT NOT NULL,
			authorizer_role_ref TEXT NOT NULL,
			admitted_holder_kind TEXT NOT NULL,
			assignment_source_rule TEXT NOT NULL,
			institutional_effect_rule_ref TEXT NOT NULL,
			instituted_object_kind TEXT NOT NULL,
			institutional_modality TEXT NOT NULL CHECK(institutional_modality != ''),
			scoped_action TEXT NOT NULL,
			utterance_description_ref TEXT NOT NULL,
			utterance_verb TEXT NOT NULL,
			utterance_binding TEXT NOT NULL CHECK(utterance_binding != ''),
			canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
			recorded_at TEXT NOT NULL,
			CHECK(json_extract(canonical_json, '$.schema') = 'haft.authority.speech-act-context-policy/v1'),
			CHECK(json_extract(canonical_json, '$.ref') = context_policy_ref),
			CHECK(json_extract(canonical_json, '$.bounded_context_ref') = bounded_context_ref),
			CHECK(json_extract(canonical_json, '$.recognized_act_type_ref') = recognized_act_type_ref),
			CHECK(json_extract(canonical_json, '$.authorizer_role_ref') = authorizer_role_ref),
			CHECK(json_extract(canonical_json, '$.admitted_holder_kind') = admitted_holder_kind),
			CHECK(json_extract(canonical_json, '$.assignment_source_rule') = assignment_source_rule),
			CHECK(json_extract(canonical_json, '$.institutional_effect_rule_ref') = institutional_effect_rule_ref),
			CHECK(json_extract(canonical_json, '$.instituted_object_kind') = instituted_object_kind),
			CHECK(json_extract(canonical_json, '$.institutional_modality') = institutional_modality),
			CHECK(json_extract(canonical_json, '$.scoped_action') = scoped_action),
			CHECK(json_extract(canonical_json, '$.utterance_description_ref') = utterance_description_ref),
			CHECK(json_extract(canonical_json, '$.utterance_verb') = utterance_verb),
			CHECK(json_extract(canonical_json, '$.utterance_binding') = utterance_binding)
		) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS profile_declaration_authorization_contents (
			authorization_content_ref TEXT PRIMARY KEY,
			authorization_content_digest TEXT NOT NULL UNIQUE CHECK(length(authorization_content_digest) = 71 AND substr(authorization_content_digest, 1, 7) = 'sha256:'),
			project_root TEXT NOT NULL,
			action_kind TEXT NOT NULL CHECK(action_kind = 'profile.declare.from_onboarding_candidate'),
			profile_author_role_assignment_ref TEXT NOT NULL REFERENCES profile_author_role_assignments(role_assignment_ref),
			profile_author_role_assignment_digest TEXT NOT NULL CHECK(length(profile_author_role_assignment_digest) = 71 AND substr(profile_author_role_assignment_digest, 1, 7) = 'sha256:'),
			method_description_ref TEXT NOT NULL REFERENCES profile_onboarding_method_descriptions(method_description_ref),
			method_description_digest TEXT NOT NULL CHECK(length(method_description_digest) = 71 AND substr(method_description_digest, 1, 7) = 'sha256:'),
			method_contract_ref TEXT NOT NULL REFERENCES profile_onboarding_method_contracts(method_contract_ref),
			method_contract_digest TEXT NOT NULL CHECK(length(method_contract_digest) = 71 AND substr(method_contract_digest, 1, 7) = 'sha256:'),
			classifier_version TEXT NOT NULL,
			policy_version TEXT NOT NULL,
			session_ref TEXT NOT NULL,
			allowed_work_from TEXT NOT NULL,
			allowed_work_until TEXT NOT NULL,
			basis_observation_from TEXT NOT NULL,
			basis_observation_until TEXT NOT NULL,
			authorization_valid_from TEXT NOT NULL,
			authorization_valid_until TEXT NOT NULL,
			single_use_key TEXT NOT NULL UNIQUE,
			canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
			recorded_at TEXT NOT NULL,
			CHECK(json_extract(canonical_json, '$.schema') = 'haft.authority.profile-declaration-authorization-content/v1'),
			CHECK(json_extract(canonical_json, '$.ref') = authorization_content_ref),
			CHECK(json_extract(canonical_json, '$.project_root') = project_root),
			CHECK(json_extract(canonical_json, '$.action_kind') = action_kind),
			CHECK(json_extract(canonical_json, '$.profile_author_role_assignment_ref') = profile_author_role_assignment_ref),
			CHECK(json_extract(canonical_json, '$.profile_author_role_assignment_digest') = profile_author_role_assignment_digest),
			CHECK(json_extract(canonical_json, '$.method_description_ref') = method_description_ref),
			CHECK(json_extract(canonical_json, '$.method_description_digest') = method_description_digest),
			CHECK(json_extract(canonical_json, '$.method_contract_ref') = method_contract_ref),
			CHECK(json_extract(canonical_json, '$.method_contract_digest') = method_contract_digest),
			CHECK(json_extract(canonical_json, '$.classifier_version') = classifier_version),
			CHECK(json_extract(canonical_json, '$.policy_version') = policy_version),
			CHECK(json_extract(canonical_json, '$.session_ref') = session_ref),
			CHECK(json_extract(canonical_json, '$.allowed_work_from') = allowed_work_from),
			CHECK(json_extract(canonical_json, '$.allowed_work_until') = allowed_work_until),
			CHECK(json_extract(canonical_json, '$.basis_observation_from') = basis_observation_from),
			CHECK(json_extract(canonical_json, '$.basis_observation_until') = basis_observation_until),
			CHECK(json_extract(canonical_json, '$.authorization_valid_from') = authorization_valid_from),
			CHECK(json_extract(canonical_json, '$.authorization_valid_until') = authorization_valid_until),
			CHECK(json_extract(canonical_json, '$.single_use_key') = single_use_key),
			CHECK(json_type(canonical_json, '$.speech_act_digest') IS NULL),
			CHECK(json_type(canonical_json, '$.capture_carrier_digest') IS NULL),
			CHECK(json_type(canonical_json, '$.captured_at') IS NULL),
			CHECK(json_type(canonical_json, '$.terminal_role_assignment_ref') IS NULL),
			CHECK(json_type(canonical_json, '$.permission_digest') IS NULL),
			CHECK(json_type(canonical_json, '$.presentation_digest') IS NULL),
			CHECK(json_type(canonical_json, '$.work_record_ref') IS NULL),
			CHECK(json_type(canonical_json, '$.payload_digest') IS NULL),
			CHECK(json_type(canonical_json, '$.outcome') IS NULL),
			CHECK(json_type(canonical_json, '$.candidate') IS NULL),
			CHECK(json_type(canonical_json, '$.admission_result') IS NULL),
			CHECK(` + sqliteCanonicalUTCNanoShape("allowed_work_from") + `),
			CHECK(` + sqliteCanonicalUTCNanoShape("allowed_work_until") + `),
			CHECK(substr(allowed_work_from, -1) = 'Z'),
			CHECK(substr(allowed_work_until, -1) = 'Z'),
			CHECK(
				(CASE WHEN instr(allowed_work_from, '.') = 0
					THEN substr(allowed_work_from, 1, length(allowed_work_from) - 1) || '.000000000Z'
					ELSE substr(allowed_work_from, 1, instr(allowed_work_from, '.')) ||
						substr(substr(allowed_work_from, instr(allowed_work_from, '.') + 1, length(allowed_work_from) - instr(allowed_work_from, '.') - 1) || '000000000', 1, 9) || 'Z'
				END)
				<
				(CASE WHEN instr(allowed_work_until, '.') = 0
					THEN substr(allowed_work_until, 1, length(allowed_work_until) - 1) || '.000000000Z'
					ELSE substr(allowed_work_until, 1, instr(allowed_work_until, '.')) ||
						substr(substr(allowed_work_until, instr(allowed_work_until, '.') + 1, length(allowed_work_until) - instr(allowed_work_until, '.') - 1) || '000000000', 1, 9) || 'Z'
				END)
			),
			CHECK(` + sqliteCanonicalUTCNanoShape("basis_observation_from") + `),
			CHECK(` + sqliteCanonicalUTCNanoShape("basis_observation_until") + `),
			CHECK(substr(basis_observation_from, -1) = 'Z'),
			CHECK(substr(basis_observation_until, -1) = 'Z'),
			CHECK(
				(CASE WHEN instr(basis_observation_from, '.') = 0
					THEN substr(basis_observation_from, 1, length(basis_observation_from) - 1) || '.000000000Z'
					ELSE substr(basis_observation_from, 1, instr(basis_observation_from, '.')) ||
						substr(substr(basis_observation_from, instr(basis_observation_from, '.') + 1, length(basis_observation_from) - instr(basis_observation_from, '.') - 1) || '000000000', 1, 9) || 'Z'
				END)
				<
				(CASE WHEN instr(basis_observation_until, '.') = 0
					THEN substr(basis_observation_until, 1, length(basis_observation_until) - 1) || '.000000000Z'
					ELSE substr(basis_observation_until, 1, instr(basis_observation_until, '.')) ||
						substr(substr(basis_observation_until, instr(basis_observation_until, '.') + 1, length(basis_observation_until) - instr(basis_observation_until, '.') - 1) || '000000000', 1, 9) || 'Z'
				END)
			),
			CHECK(` + sqliteCanonicalUTCNanoShape("authorization_valid_from") + `),
			CHECK(` + sqliteCanonicalUTCNanoShape("authorization_valid_until") + `),
			CHECK(` + sqliteUTCNanoLess("authorization_valid_from", "authorization_valid_until") + `)
		) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS terminal_capture_records (
			capture_carrier_ref TEXT PRIMARY KEY,
			capture_carrier_digest TEXT NOT NULL UNIQUE CHECK(length(capture_carrier_digest) = 71 AND substr(capture_carrier_digest, 1, 7) = 'sha256:'),
			project_root TEXT NOT NULL,
			prepared_speech_act_intent_digest TEXT NOT NULL CHECK(length(prepared_speech_act_intent_digest) = 71 AND substr(prepared_speech_act_intent_digest, 1, 7) = 'sha256:'),
			review_text TEXT NOT NULL CHECK(review_text != ''),
			review_digest TEXT NOT NULL CHECK(length(review_digest) = 71 AND substr(review_digest, 1, 7) = 'sha256:'),
			canonical_utterance TEXT NOT NULL CHECK(canonical_utterance != ''),
			started_at TEXT NOT NULL,
			exact_utterance_observed_at TEXT NOT NULL,
			ended_at TEXT NOT NULL,
			intent_session_ref TEXT NOT NULL,
			observed_session_material TEXT NOT NULL CHECK(observed_session_material != ''),
			observation_nonce TEXT NOT NULL UNIQUE CHECK(length(observation_nonce) = 32 AND observation_nonce NOT GLOB '*[^0-9a-f]*'),
			observation_digest TEXT NOT NULL CHECK(length(observation_digest) = 71 AND substr(observation_digest, 1, 7) = 'sha256:'),
			observed_holder_system_ref TEXT NOT NULL UNIQUE,
			observed_role_assignment_ref TEXT NOT NULL UNIQUE,
			canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
			recorded_at TEXT NOT NULL,
			CHECK(json_extract(canonical_json, '$.schema') = 'haft.authority.terminal-capture/v1'),
			CHECK(json_extract(canonical_json, '$.carrier_ref') = capture_carrier_ref),
			CHECK(json_extract(canonical_json, '$.project_root') = project_root),
			CHECK(json_extract(canonical_json, '$.prepared_speech_act_intent_digest') = prepared_speech_act_intent_digest),
			CHECK(json_extract(canonical_json, '$.review_text') = review_text),
			CHECK(json_extract(canonical_json, '$.review_digest') = review_digest),
			CHECK(json_extract(canonical_json, '$.canonical_utterance') = canonical_utterance),
			CHECK(json_extract(canonical_json, '$.started_at') = started_at),
			CHECK(json_extract(canonical_json, '$.exact_utterance_observed_at') = exact_utterance_observed_at),
			CHECK(json_extract(canonical_json, '$.ended_at') = ended_at),
			CHECK(json_extract(canonical_json, '$.session_ref') = intent_session_ref),
			CHECK(json_extract(canonical_json, '$.observed_session_material') = observed_session_material),
			CHECK(json_extract(canonical_json, '$.observation_nonce') = observation_nonce),
			CHECK(json_extract(canonical_json, '$.observation_digest') = observation_digest),
			CHECK(json_extract(canonical_json, '$.observed_holder_system_ref') = observed_holder_system_ref),
			CHECK(json_extract(canonical_json, '$.observed_role_assignment_ref') = observed_role_assignment_ref),
			CHECK(` + sqliteCanonicalUTCNanoShape("started_at") + `),
			CHECK(` + sqliteCanonicalUTCNanoShape("exact_utterance_observed_at") + `),
			CHECK(` + sqliteCanonicalUTCNanoShape("ended_at") + `),
			CHECK(` + sqliteUTCNanoLess("started_at", "exact_utterance_observed_at") + `),
			CHECK(` + sqliteUTCNanoLess("exact_utterance_observed_at", "ended_at") + `)
		) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS speech_act_role_assignments (
			role_assignment_ref TEXT PRIMARY KEY,
			role_assignment_digest TEXT NOT NULL UNIQUE CHECK(length(role_assignment_digest) = 71 AND substr(role_assignment_digest, 1, 7) = 'sha256:'),
			project_root TEXT NOT NULL,
			holder_system_ref TEXT NOT NULL,
			admitted_holder_kind TEXT NOT NULL CHECK(admitted_holder_kind = 'U.System'),
			role_ref TEXT NOT NULL,
			bounded_context_ref TEXT NOT NULL,
			valid_from TEXT NOT NULL,
			valid_until TEXT NOT NULL,
			context_policy_ref TEXT NOT NULL REFERENCES speech_act_context_policies(context_policy_ref),
			context_policy_digest TEXT NOT NULL,
			provenance_carrier_ref TEXT NOT NULL REFERENCES terminal_capture_records(capture_carrier_ref),
			provenance_carrier_digest TEXT NOT NULL CHECK(length(provenance_carrier_digest) = 71 AND substr(provenance_carrier_digest, 1, 7) = 'sha256:'),
			identity_boundary TEXT NOT NULL CHECK(identity_boundary != ''),
			canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
			recorded_at TEXT NOT NULL,
			CHECK(json_extract(canonical_json, '$.schema') = 'haft.authority.context-policy-assigned-terminal-session/v1'),
			CHECK(json_extract(canonical_json, '$.role_assignment_ref') = role_assignment_ref),
			CHECK(json_extract(canonical_json, '$.project_root') = project_root),
			CHECK(json_extract(canonical_json, '$.holder_system_ref') = holder_system_ref),
			CHECK(json_extract(canonical_json, '$.admitted_holder_kind') = admitted_holder_kind),
			CHECK(json_extract(canonical_json, '$.role_ref') = role_ref),
			CHECK(json_extract(canonical_json, '$.bounded_context_ref') = bounded_context_ref),
			CHECK(json_extract(canonical_json, '$.valid_from') = valid_from),
			CHECK(json_extract(canonical_json, '$.valid_until') = valid_until),
			CHECK(json_extract(canonical_json, '$.justification_source_ref') = context_policy_ref),
			CHECK(json_extract(canonical_json, '$.justification_source_digest') = context_policy_digest),
			CHECK(json_extract(canonical_json, '$.assignment_provenance_carrier_ref') = provenance_carrier_ref),
			CHECK(json_extract(canonical_json, '$.assignment_provenance_carrier_digest') = provenance_carrier_digest),
			CHECK(json_extract(canonical_json, '$.identity_boundary') = identity_boundary),
			CHECK(` + sqliteCanonicalUTCNanoShape("valid_from") + `),
			CHECK(` + sqliteCanonicalUTCNanoShape("valid_until") + `),
			CHECK(` + sqliteUTCNanoLess("valid_from", "valid_until") + `)
		) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS speech_acts (
			speech_act_ref TEXT PRIMARY KEY,
			speech_act_digest TEXT NOT NULL UNIQUE CHECK(length(speech_act_digest) = 71 AND substr(speech_act_digest, 1, 7) = 'sha256:'),
			project_root TEXT NOT NULL,
			work_kind TEXT NOT NULL CHECK(work_kind = 'Communicative'),
			act_type_ref TEXT NOT NULL,
			performed_by_ref TEXT NOT NULL REFERENCES speech_act_role_assignments(role_assignment_ref),
			performed_by_digest TEXT NOT NULL CHECK(length(performed_by_digest) = 71 AND substr(performed_by_digest, 1, 7) = 'sha256:'),
			method_ref TEXT NOT NULL,
			method_description_ref TEXT NOT NULL REFERENCES speech_act_method_descriptions(method_description_ref),
			method_description_digest TEXT NOT NULL CHECK(length(method_description_digest) = 71 AND substr(method_description_digest, 1, 7) = 'sha256:'),
			executed_within_ref TEXT NOT NULL,
			bounded_context_ref TEXT NOT NULL,
			window_from TEXT NOT NULL,
			window_until TEXT NOT NULL,
			parameters_json TEXT NOT NULL CHECK(json_valid(parameters_json)),
			input_refs_json TEXT NOT NULL CHECK(json_valid(input_refs_json)),
			output_refs_json TEXT NOT NULL CHECK(json_valid(output_refs_json)),
			resource_refs_json TEXT NOT NULL CHECK(json_valid(resource_refs_json)),
			affected_refs_json TEXT NOT NULL CHECK(json_valid(affected_refs_json)),
			state_plane_ref TEXT NOT NULL,
			delta_predicate_ref TEXT NOT NULL,
			outcome_ref TEXT NOT NULL,
			utterance_ref TEXT NOT NULL,
			capture_carrier_ref TEXT NOT NULL REFERENCES terminal_capture_records(capture_carrier_ref),
			capture_carrier_digest TEXT NOT NULL CHECK(length(capture_carrier_digest) = 71 AND substr(capture_carrier_digest, 1, 7) = 'sha256:'),
			review_subject_ref TEXT NOT NULL,
			review_subject_digest TEXT NOT NULL CHECK(length(review_subject_digest) = 71 AND substr(review_subject_digest, 1, 7) = 'sha256:'),
			instituted_object_ref TEXT NOT NULL,
			context_policy_ref TEXT NOT NULL REFERENCES speech_act_context_policies(context_policy_ref),
			context_policy_digest TEXT NOT NULL,
			canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
			recorded_at TEXT NOT NULL,
			CHECK(json_extract(canonical_json, '$.schema') = 'haft.authority.speech-act/v1'),
			CHECK(json_extract(canonical_json, '$.speech_act_ref') = speech_act_ref),
			CHECK(json_extract(canonical_json, '$.project_root') = project_root),
			CHECK(json_extract(canonical_json, '$.work_kind') = work_kind),
			CHECK(json_extract(canonical_json, '$.act_type_ref') = act_type_ref),
			CHECK(json_extract(canonical_json, '$.performed_by_role_assignment_ref') = performed_by_ref),
			CHECK(json_extract(canonical_json, '$.performed_by_role_assignment_digest') = performed_by_digest),
			CHECK(json_extract(canonical_json, '$.method_ref') = method_ref),
			CHECK(json_extract(canonical_json, '$.method_description_ref') = method_description_ref),
			CHECK(json_extract(canonical_json, '$.method_description_digest') = method_description_digest),
			CHECK(json_extract(canonical_json, '$.executed_within_system_ref') = executed_within_ref),
			CHECK(json_extract(canonical_json, '$.bounded_context_ref') = bounded_context_ref),
			CHECK(json_extract(canonical_json, '$.window_from') = window_from),
			CHECK(json_extract(canonical_json, '$.window_until') = window_until),
			CHECK(json(json_extract(canonical_json, '$.parameters')) = json(parameters_json)),
			CHECK(json(json_extract(canonical_json, '$.input_refs')) = json(input_refs_json)),
			CHECK(json(json_extract(canonical_json, '$.output_refs')) = json(output_refs_json)),
			CHECK(json(json_extract(canonical_json, '$.resource_refs')) = json(resource_refs_json)),
			CHECK(json(json_extract(canonical_json, '$.affected_refs')) = json(affected_refs_json)),
			CHECK(json_type(parameters_json) = 'array'),
			CHECK(json_type(input_refs_json) = 'array' AND json_array_length(input_refs_json) > 0),
			CHECK(json_type(output_refs_json) = 'array' AND json_array_length(output_refs_json) > 0),
			CHECK(json_type(resource_refs_json) = 'array'),
			CHECK(json_type(affected_refs_json) = 'array' AND json_array_length(affected_refs_json) > 0),
			CHECK(json_extract(canonical_json, '$.state_plane_ref') = state_plane_ref),
			CHECK(json_extract(canonical_json, '$.delta_predicate_ref') = delta_predicate_ref),
			CHECK(json_extract(canonical_json, '$.outcome_ref') = outcome_ref),
			CHECK(json_extract(canonical_json, '$.utterance_ref') = utterance_ref),
			CHECK(json_extract(canonical_json, '$.capture_carrier_ref') = capture_carrier_ref),
			CHECK(json_extract(canonical_json, '$.capture_carrier_digest') = capture_carrier_digest),
			CHECK(json_extract(canonical_json, '$.review_subject_ref') = review_subject_ref),
			CHECK(json_extract(canonical_json, '$.review_subject_digest') = review_subject_digest),
			CHECK(json_extract(canonical_json, '$.instituted_object_ref') = instituted_object_ref),
			CHECK(` + sqliteCanonicalUTCNanoShape("window_from") + `),
			CHECK(` + sqliteCanonicalUTCNanoShape("window_until") + `),
			CHECK(` + sqliteUTCNanoLess("window_from", "window_until") + `)
		) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS profile_declaration_permissions (
			permission_ref TEXT PRIMARY KEY,
			permission_digest TEXT NOT NULL UNIQUE CHECK(length(permission_digest) = 71 AND substr(permission_digest, 1, 7) = 'sha256:'),
			project_root TEXT NOT NULL,
			subject_ref TEXT NOT NULL REFERENCES profile_author_role_assignments(role_assignment_ref),
			subject_digest TEXT NOT NULL CHECK(length(subject_digest) = 71 AND substr(subject_digest, 1, 7) = 'sha256:'),
			modality TEXT NOT NULL CHECK(modality = 'MAY'),
			claim_scope_ref TEXT NOT NULL,
			action_kind TEXT NOT NULL CHECK(action_kind = 'profile.declare.from_onboarding_candidate'),
			bounded_context_ref TEXT NOT NULL,
			valid_from TEXT NOT NULL,
			valid_until TEXT NOT NULL,
			authorization_content_ref TEXT NOT NULL REFERENCES profile_declaration_authorization_contents(authorization_content_ref),
			authorization_content_digest TEXT NOT NULL CHECK(length(authorization_content_digest) = 71 AND substr(authorization_content_digest, 1, 7) = 'sha256:'),
			method_description_ref TEXT NOT NULL REFERENCES profile_onboarding_method_descriptions(method_description_ref),
			method_description_digest TEXT NOT NULL CHECK(length(method_description_digest) = 71 AND substr(method_description_digest, 1, 7) = 'sha256:'),
			admission_predicate_ref TEXT NOT NULL,
			referents_json TEXT NOT NULL CHECK(json_valid(referents_json) AND json_type(referents_json) = 'array' AND json_array_length(referents_json) = 3),
			source_speech_act_ref TEXT NOT NULL REFERENCES speech_acts(speech_act_ref),
			context_policy_ref TEXT NOT NULL REFERENCES speech_act_context_policies(context_policy_ref),
			context_policy_digest TEXT NOT NULL CHECK(length(context_policy_digest) = 71 AND substr(context_policy_digest, 1, 7) = 'sha256:'),
			adjudication_verifier_identity TEXT NOT NULL,
			adjudication_verifier_version TEXT NOT NULL,
			adjudication_evidence_claim_refs_json TEXT NOT NULL CHECK(json_valid(adjudication_evidence_claim_refs_json) AND json_type(adjudication_evidence_claim_refs_json) = 'array' AND json_array_length(adjudication_evidence_claim_refs_json) > 0),
			adjudication_carrier_refs_json TEXT NOT NULL CHECK(json_valid(adjudication_carrier_refs_json) AND json_type(adjudication_carrier_refs_json) = 'array' AND json_array_length(adjudication_carrier_refs_json) > 0),
			adjudication_evaluation_policy_ref TEXT NOT NULL,
			adjudication_evaluation_policy_digest TEXT NOT NULL CHECK(length(adjudication_evaluation_policy_digest) = 71 AND substr(adjudication_evaluation_policy_digest, 1, 7) = 'sha256:'),
			capture_carrier_ref TEXT NOT NULL REFERENCES terminal_capture_records(capture_carrier_ref),
			capture_carrier_digest TEXT NOT NULL CHECK(length(capture_carrier_digest) = 71 AND substr(capture_carrier_digest, 1, 7) = 'sha256:'),
			canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
			recorded_at TEXT NOT NULL,
			CHECK(json_extract(canonical_json, '$.schema') = 'haft.authority.profile-declaration-permission/v1'),
			CHECK(json_extract(canonical_json, '$.permission_ref') = permission_ref),
			CHECK(json_extract(canonical_json, '$.project_root') = project_root),
			CHECK(json_extract(canonical_json, '$.subject_role_assignment_ref') = subject_ref),
			CHECK(json_extract(canonical_json, '$.subject_role_assignment_digest') = subject_digest),
			CHECK(json_extract(canonical_json, '$.modality') = modality),
			CHECK(json_extract(canonical_json, '$.claim_scope_ref') = claim_scope_ref),
			CHECK(json_extract(canonical_json, '$.action_kind') = action_kind),
			CHECK(json_extract(canonical_json, '$.claim_scope_bounded_context_ref') = bounded_context_ref),
			CHECK(json_extract(canonical_json, '$.valid_from') = valid_from),
			CHECK(json_extract(canonical_json, '$.valid_until') = valid_until),
			CHECK(json_extract(canonical_json, '$.authorization_content_ref') = authorization_content_ref),
			CHECK(json_extract(canonical_json, '$.authorization_content_digest') = authorization_content_digest),
			CHECK(json_extract(canonical_json, '$.method_description_ref') = method_description_ref),
			CHECK(json_extract(canonical_json, '$.method_description_digest') = method_description_digest),
			CHECK(json_extract(canonical_json, '$.profile_admission_predicate_ref') = admission_predicate_ref),
			CHECK(json(json_extract(canonical_json, '$.referents')) = json(referents_json)),
			CHECK(json_extract(canonical_json, '$.source_speech_act_ref') = source_speech_act_ref),
			CHECK(json_extract(canonical_json, '$.context_policy_ref') = context_policy_ref),
			CHECK(json_extract(canonical_json, '$.context_policy_digest') = context_policy_digest),
			CHECK(json_extract(canonical_json, '$.adjudication_verifier_identity') = adjudication_verifier_identity),
			CHECK(json_extract(canonical_json, '$.adjudication_verifier_version') = adjudication_verifier_version),
			CHECK(json(json_extract(canonical_json, '$.adjudication_evidence_claim_refs')) = json(adjudication_evidence_claim_refs_json)),
			CHECK(json(json_extract(canonical_json, '$.adjudication_carrier_refs')) = json(adjudication_carrier_refs_json)),
			CHECK(json_extract(canonical_json, '$.adjudication_evaluation_policy_ref') = adjudication_evaluation_policy_ref),
			CHECK(json_extract(canonical_json, '$.adjudication_evaluation_policy_digest') = adjudication_evaluation_policy_digest),
			CHECK(json_extract(canonical_json, '$.instituting_terminal_capture_carrier_ref') = capture_carrier_ref),
			CHECK(json_extract(canonical_json, '$.instituting_terminal_capture_carrier_digest') = capture_carrier_digest),
			CHECK(json_extract(referents_json, '$[0]') = authorization_content_ref),
			CHECK(json_extract(referents_json, '$[1]') = method_description_ref),
			CHECK(json_extract(referents_json, '$[2]') = admission_predicate_ref),
			CHECK(` + sqliteCanonicalUTCNanoShape("valid_from") + `),
			CHECK(` + sqliteCanonicalUTCNanoShape("valid_until") + `),
			CHECK(` + sqliteUTCNanoLess("valid_from", "valid_until") + `)
		) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS speech_act_instituted_effects (
			instituted_effect_digest TEXT PRIMARY KEY CHECK(length(instituted_effect_digest) = 71 AND substr(instituted_effect_digest, 1, 7) = 'sha256:'),
			project_root TEXT NOT NULL,
			speech_act_ref TEXT NOT NULL REFERENCES speech_acts(speech_act_ref),
			speech_act_digest TEXT NOT NULL CHECK(length(speech_act_digest) = 71 AND substr(speech_act_digest, 1, 7) = 'sha256:'),
			permission_ref TEXT NOT NULL REFERENCES profile_declaration_permissions(permission_ref),
			permission_digest TEXT NOT NULL CHECK(length(permission_digest) = 71 AND substr(permission_digest, 1, 7) = 'sha256:'),
			canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
			recorded_at TEXT NOT NULL,
			CHECK(json_extract(canonical_json, '$.schema') = 'haft.authority.instituted-permission-effect/v1'),
			CHECK(json_extract(canonical_json, '$.project_root') = project_root),
			CHECK(json_extract(canonical_json, '$.speech_act_ref') = speech_act_ref),
			CHECK(json_extract(canonical_json, '$.speech_act_digest') = speech_act_digest),
			CHECK(json_extract(canonical_json, '$.permission_ref') = permission_ref),
			CHECK(json_extract(canonical_json, '$.permission_digest') = permission_digest)
		) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS authority_basis_presentations (
			presentation_id TEXT PRIMARY KEY,
			presentation_digest TEXT NOT NULL UNIQUE CHECK(length(presentation_digest) = 71 AND substr(presentation_digest, 1, 7) = 'sha256:'),
			project_root TEXT NOT NULL,
			context_policy_ref TEXT NOT NULL REFERENCES speech_act_context_policies(context_policy_ref),
			context_policy_digest TEXT NOT NULL CHECK(length(context_policy_digest) = 71 AND substr(context_policy_digest, 1, 7) = 'sha256:'),
			authorization_content_ref TEXT NOT NULL REFERENCES profile_declaration_authorization_contents(authorization_content_ref),
			authorization_content_digest TEXT NOT NULL CHECK(length(authorization_content_digest) = 71 AND substr(authorization_content_digest, 1, 7) = 'sha256:'),
			capture_carrier_ref TEXT NOT NULL REFERENCES terminal_capture_records(capture_carrier_ref),
			capture_carrier_digest TEXT NOT NULL CHECK(length(capture_carrier_digest) = 71 AND substr(capture_carrier_digest, 1, 7) = 'sha256:'),
			authorizer_ref TEXT NOT NULL REFERENCES speech_act_role_assignments(role_assignment_ref),
			authorizer_digest TEXT NOT NULL CHECK(length(authorizer_digest) = 71 AND substr(authorizer_digest, 1, 7) = 'sha256:'),
			speech_act_ref TEXT NOT NULL REFERENCES speech_acts(speech_act_ref),
			speech_act_digest TEXT NOT NULL CHECK(length(speech_act_digest) = 71 AND substr(speech_act_digest, 1, 7) = 'sha256:'),
			permission_ref TEXT NOT NULL REFERENCES profile_declaration_permissions(permission_ref),
			permission_digest TEXT NOT NULL CHECK(length(permission_digest) = 71 AND substr(permission_digest, 1, 7) = 'sha256:'),
			instituted_effect_digest TEXT NOT NULL REFERENCES speech_act_instituted_effects(instituted_effect_digest),
			legacy_projection_digest TEXT NOT NULL UNIQUE CHECK(length(legacy_projection_digest) = 71 AND substr(legacy_projection_digest, 1, 7) = 'sha256:'),
			canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
			recorded_at TEXT NOT NULL,
			CHECK(json_extract(canonical_json, '$.schema') = 'haft.authority.basis-presentation/v2'),
			CHECK(json_extract(canonical_json, '$.presentation_id') = presentation_id),
			CHECK(json_extract(canonical_json, '$.project_root') = project_root),
			CHECK(json_extract(canonical_json, '$.context_policy_ref') = context_policy_ref),
			CHECK(json_extract(canonical_json, '$.context_policy_digest') = context_policy_digest),
			CHECK(json_extract(canonical_json, '$.authorization_content_ref') = authorization_content_ref),
			CHECK(json_extract(canonical_json, '$.authorization_content_digest') = authorization_content_digest),
			CHECK(json_extract(canonical_json, '$.terminal_capture_carrier_ref') = capture_carrier_ref),
			CHECK(json_extract(canonical_json, '$.terminal_capture_carrier_digest') = capture_carrier_digest),
			CHECK(json_extract(canonical_json, '$.authorizer_role_assignment_ref') = authorizer_ref),
			CHECK(json_extract(canonical_json, '$.authorizer_role_assignment_digest') = authorizer_digest),
			CHECK(json_extract(canonical_json, '$.speech_act_ref') = speech_act_ref),
			CHECK(json_extract(canonical_json, '$.speech_act_digest') = speech_act_digest),
			CHECK(json_extract(canonical_json, '$.permission_ref') = permission_ref),
			CHECK(json_extract(canonical_json, '$.permission_digest') = permission_digest),
			CHECK(json_extract(canonical_json, '$.instituted_effect_digest') = instituted_effect_digest),
			CHECK(json_extract(canonical_json, '$.legacy_projection_digest') = legacy_projection_digest)
		) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS authority_basis_resolutions (
			authority_resolution_id TEXT PRIMARY KEY,
			authority_resolution_digest TEXT NOT NULL UNIQUE CHECK(length(authority_resolution_digest) = 71 AND substr(authority_resolution_digest, 1, 7) = 'sha256:'),
			project_root TEXT NOT NULL,
			presentation_id TEXT NOT NULL UNIQUE REFERENCES authority_basis_presentations(presentation_id),
			presentation_digest TEXT NOT NULL UNIQUE CHECK(length(presentation_digest) = 71 AND substr(presentation_digest, 1, 7) = 'sha256:'),
			verifier_identity TEXT NOT NULL,
			verifier_version TEXT NOT NULL,
			verification_policy_ref TEXT NOT NULL,
			verification_policy_digest TEXT NOT NULL CHECK(length(verification_policy_digest) = 71 AND substr(verification_policy_digest, 1, 7) = 'sha256:'),
			authority_valid_from TEXT NOT NULL,
			resolved_at TEXT NOT NULL,
			valid_until TEXT NOT NULL,
			legacy_projection_digest TEXT NOT NULL UNIQUE CHECK(length(legacy_projection_digest) = 71 AND substr(legacy_projection_digest, 1, 7) = 'sha256:'),
			canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
			recorded_at TEXT NOT NULL,
			CHECK(json_extract(canonical_json, '$.schema') = 'haft.authority.basis-resolution/v2'),
			CHECK(json_extract(canonical_json, '$.authority_resolution_id') = authority_resolution_id),
			CHECK(json_extract(canonical_json, '$.project_root') = project_root),
			CHECK(json_extract(canonical_json, '$.presentation_id') = presentation_id),
			CHECK(json_extract(canonical_json, '$.presentation_digest') = presentation_digest),
			CHECK(json_extract(canonical_json, '$.verifier_identity') = verifier_identity),
			CHECK(json_extract(canonical_json, '$.verifier_version') = verifier_version),
			CHECK(json_extract(canonical_json, '$.verification_policy_ref') = verification_policy_ref),
			CHECK(json_extract(canonical_json, '$.verification_policy_digest') = verification_policy_digest),
			CHECK(json_extract(canonical_json, '$.authority_valid_from') = authority_valid_from),
			CHECK(json_extract(canonical_json, '$.resolved_at') = resolved_at),
			CHECK(json_extract(canonical_json, '$.valid_until') = valid_until),
			CHECK(json_extract(canonical_json, '$.legacy_projection_digest') = legacy_projection_digest),
			CHECK(` + sqliteCanonicalUTCNanoShape("authority_valid_from") + `),
			CHECK(` + sqliteCanonicalUTCNanoShape("resolved_at") + `),
			CHECK(` + sqliteCanonicalUTCNanoShape("valid_until") + `),
			CHECK(` + sqliteUTCNanoLess("resolved_at", "valid_until") + `)
		) WITHOUT ROWID`,
	}
	statements = appendAuthorityBasisExactBindingTriggers(statements)
	statements = appendAuthorityBasisImmutabilityTriggers(statements)
	return appendAuthorityBasisRootGuards(statements)
}

// sqliteCanonicalUTCNanoKey normalizes the two canonical RFC3339Nano UTC
// spellings emitted by the kernel (whole seconds and 1-9 fractional digits)
// to one fixed-width lexical key. SQLite julianday is intentionally not used
// for ordering: it collapses distinct sub-millisecond observations.
func sqliteCanonicalUTCNanoKey(column string) string {
	return `(CASE WHEN instr(` + column + `, '.') = 0
		THEN substr(` + column + `, 1, length(` + column + `) - 1) || '.000000000Z'
		ELSE substr(` + column + `, 1, instr(` + column + `, '.')) ||
			substr(substr(` + column + `, instr(` + column + `, '.') + 1, length(` + column + `) - instr(` + column + `, '.') - 1) || '000000000', 1, 9) || 'Z'
	END)`
}

func sqliteCanonicalUTCNanoShape(column string) string {
	return `julianday(` + column + `) IS NOT NULL
		AND length(` + column + `) BETWEEN 20 AND 30
		AND substr(` + column + `, 5, 1) = '-'
		AND substr(` + column + `, 8, 1) = '-'
		AND substr(` + column + `, 11, 1) = 'T'
		AND substr(` + column + `, 14, 1) = ':'
		AND substr(` + column + `, 17, 1) = ':'
		AND substr(` + column + `, -1) = 'Z'
		AND substr(` + column + `, 1, 4) NOT GLOB '*[^0-9]*'
		AND substr(` + column + `, 6, 2) NOT GLOB '*[^0-9]*'
		AND substr(` + column + `, 9, 2) NOT GLOB '*[^0-9]*'
		AND substr(` + column + `, 12, 2) NOT GLOB '*[^0-9]*'
		AND substr(` + column + `, 15, 2) NOT GLOB '*[^0-9]*'
		AND substr(` + column + `, 18, 2) NOT GLOB '*[^0-9]*'
		AND substr(` + column + `, 6, 2) BETWEEN '01' AND '12'
		AND date(substr(` + column + `, 1, 10), '+0 days') = substr(` + column + `, 1, 10)
		AND substr(` + column + `, 12, 2) BETWEEN '00' AND '23'
		AND substr(` + column + `, 15, 2) BETWEEN '00' AND '59'
		AND substr(` + column + `, 18, 2) BETWEEN '00' AND '59'
		AND (
			(length(` + column + `) = 20 AND substr(` + column + `, 20, 1) = 'Z')
			OR (
				length(` + column + `) BETWEEN 22 AND 30
				AND substr(` + column + `, 20, 1) = '.'
				AND substr(` + column + `, 21, length(` + column + `) - 21) NOT GLOB '*[^0-9]*'
				AND substr(` + column + `, -2, 1) BETWEEN '1' AND '9'
			)
		)`
}

func sqliteUTCNanoLess(left string, right string) string {
	return sqliteCanonicalUTCNanoKey(left) + ` < ` + sqliteCanonicalUTCNanoKey(right)
}

func sqliteUTCNanoLessOrEqual(left string, right string) string {
	return sqliteCanonicalUTCNanoKey(left) + ` <= ` + sqliteCanonicalUTCNanoKey(right)
}

func appendAuthorityBasisExactBindingTriggers(statements []string) []string {
	return append(statements,
		`DROP TRIGGER IF EXISTS authority_presentations_exact_assignment_method`,
		`CREATE TRIGGER authority_presentations_exact_assignment_method
		BEFORE INSERT ON authority_presentations
		WHEN NOT EXISTS (
			SELECT 1
			FROM profile_author_role_assignments assignment
			JOIN profile_author_assignment_support_carriers assignment_support
				ON assignment_support.assignment_justification_ref = assignment.assignment_justification_ref
			JOIN profile_onboarding_method_descriptions description
				ON description.method_description_ref = NEW.method_description_ref
			JOIN profile_onboarding_method_contracts contract
				ON contract.method_contract_ref = NEW.method_contract_ref
			WHERE assignment.role_assignment_ref = NEW.profile_author_role_assignment_ref
			AND assignment.role_assignment_digest = NEW.profile_author_role_assignment_digest
			AND NEW.permission_subject_role_assignment_ref = assignment.role_assignment_ref
			AND description.method_description_digest = NEW.method_description_digest
			AND contract.method_contract_digest = NEW.method_contract_digest
			AND contract.method_description_ref = description.method_description_ref
			AND contract.method_description_digest = description.method_description_digest
			AND assignment_support.method_contract_ref = contract.method_contract_ref
			AND assignment_support.method_contract_digest = contract.method_contract_digest
			AND NEW.permission_method_description_ref = description.method_description_ref
			AND NEW.permission_action_kind = NEW.action_kind
			AND NEW.permission_project_root = NEW.project_root
			AND `+sqliteUTCNanoLessOrEqual("NEW.valid_from", "NEW.permission_valid_from")+`
			AND `+sqliteUTCNanoLess("NEW.permission_valid_from", "NEW.permission_valid_until")+`
			AND NEW.permission_valid_until = NEW.valid_until
			AND NEW.permission_single_use_key = NEW.single_use_key
			AND assignment_support.session_ref = NEW.session_ref
			AND `+sqliteUTCNanoLessOrEqual("assignment.valid_from", "NEW.valid_from")+`
			AND `+sqliteUTCNanoLessOrEqual("NEW.valid_until", "assignment.valid_until")+`
			AND `+sqliteUTCNanoLessOrEqual("assignment.valid_from", "NEW.allowed_work_from")+`
			AND `+sqliteUTCNanoLessOrEqual("NEW.allowed_work_until", "assignment.valid_until")+`
		) BEGIN
			SELECT RAISE(ABORT, 'authority presentation does not consume the exact pre-existing assignment and method contract');
		END`,
		`CREATE TRIGGER IF NOT EXISTS profile_declaration_authorization_contents_exact_sources
		BEFORE INSERT ON profile_declaration_authorization_contents
		WHEN NOT EXISTS (
			SELECT 1
			FROM profile_author_role_assignments assignment
			JOIN profile_onboarding_method_descriptions method
				ON method.method_description_ref = NEW.method_description_ref
			JOIN profile_onboarding_method_contracts contract
				ON contract.method_contract_ref = NEW.method_contract_ref
			WHERE assignment.role_assignment_ref = NEW.profile_author_role_assignment_ref
			AND assignment.role_assignment_digest = NEW.profile_author_role_assignment_digest
			AND method.method_description_digest = NEW.method_description_digest
			AND contract.method_contract_digest = NEW.method_contract_digest
			AND contract.method_description_ref = method.method_description_ref
			AND contract.method_description_digest = method.method_description_digest
		) BEGIN
			SELECT RAISE(ABORT, 'profile-declaration authorization content does not bind exact RoleAssignment and onboarding method sources');
		END`,
		`CREATE TRIGGER IF NOT EXISTS speech_act_role_assignments_exact_sources
		BEFORE INSERT ON speech_act_role_assignments
		WHEN NOT EXISTS (
			SELECT 1
			FROM speech_act_context_policies policy
			JOIN terminal_capture_records capture
				ON capture.capture_carrier_ref = NEW.provenance_carrier_ref
			WHERE policy.context_policy_ref = NEW.context_policy_ref
			AND policy.context_policy_digest = NEW.context_policy_digest
			AND policy.bounded_context_ref = NEW.bounded_context_ref
			AND policy.authorizer_role_ref = NEW.role_ref
			AND policy.admitted_holder_kind = NEW.admitted_holder_kind
			AND capture.capture_carrier_digest = NEW.provenance_carrier_digest
			AND capture.project_root = NEW.project_root
			AND capture.observed_holder_system_ref = NEW.holder_system_ref
			AND capture.observed_role_assignment_ref = NEW.role_assignment_ref
			AND NEW.valid_from = capture.started_at
			AND NEW.valid_until = capture.ended_at
		) BEGIN
			SELECT RAISE(ABORT, 'SpeechAct RoleAssignment does not bind exact policy and capture provenance');
		END`,
		`CREATE TRIGGER IF NOT EXISTS speech_acts_exact_sources
		BEFORE INSERT ON speech_acts
		WHEN NOT EXISTS (
			SELECT 1
			FROM speech_act_role_assignments assignment
			JOIN terminal_capture_records capture
				ON capture.capture_carrier_ref = NEW.capture_carrier_ref
			JOIN speech_act_context_policies policy
				ON policy.context_policy_ref = NEW.context_policy_ref
			JOIN speech_act_method_descriptions method
				ON method.method_description_ref = NEW.method_description_ref
			WHERE assignment.role_assignment_ref = NEW.performed_by_ref
			AND assignment.role_assignment_digest = NEW.performed_by_digest
			AND assignment.project_root = NEW.project_root
			AND assignment.bounded_context_ref = NEW.bounded_context_ref
			AND assignment.valid_from = NEW.window_from
			AND assignment.valid_until = NEW.window_until
			AND capture.capture_carrier_digest = NEW.capture_carrier_digest
			AND capture.project_root = NEW.project_root
			AND capture.started_at = NEW.window_from
			AND capture.ended_at = NEW.window_until
			AND policy.context_policy_digest = NEW.context_policy_digest
			AND policy.recognized_act_type_ref = NEW.act_type_ref
			AND policy.bounded_context_ref = NEW.bounded_context_ref
			AND policy.utterance_description_ref = NEW.utterance_ref
			AND capture.canonical_utterance = policy.utterance_verb || ' ' ||
				CASE policy.utterance_binding
					WHEN 'review_digest' THEN capture.review_digest
					WHEN 'review_subject_digest' THEN NEW.review_subject_digest
				END
			AND method.method_description_digest = NEW.method_description_digest
			AND method.method_ref = NEW.method_ref
			AND method.bounded_context_ref = NEW.bounded_context_ref
			AND EXISTS (
				SELECT 1 FROM json_each(NEW.input_refs_json)
				WHERE value = NEW.review_subject_ref
			)
			AND EXISTS (
				SELECT 1 FROM json_each(NEW.output_refs_json)
				WHERE value = NEW.instituted_object_ref
			)
		) BEGIN
			SELECT RAISE(ABORT, 'SpeechAct does not bind exact capture, assignment, policy, and MethodDescription');
		END`,
		`CREATE TRIGGER IF NOT EXISTS profile_declaration_permissions_exact_instituting_sources
		BEFORE INSERT ON profile_declaration_permissions
		WHEN NOT EXISTS (
			SELECT 1
			FROM profile_declaration_authorization_contents content
			JOIN speech_acts act ON act.speech_act_ref = NEW.source_speech_act_ref
			JOIN speech_act_context_policies policy ON policy.context_policy_ref = NEW.context_policy_ref
			JOIN terminal_capture_records capture ON capture.capture_carrier_ref = NEW.capture_carrier_ref
			JOIN profile_author_role_assignments subject ON subject.role_assignment_ref = NEW.subject_ref
			WHERE content.authorization_content_ref = NEW.authorization_content_ref
			AND content.authorization_content_digest = NEW.authorization_content_digest
			AND content.project_root = NEW.project_root
			AND content.action_kind = NEW.action_kind
			AND content.profile_author_role_assignment_ref = NEW.subject_ref
			AND content.profile_author_role_assignment_digest = NEW.subject_digest
			AND content.method_description_ref = NEW.method_description_ref
			AND content.method_description_digest = NEW.method_description_digest
			AND subject.role_assignment_digest = NEW.subject_digest
			AND act.project_root = NEW.project_root
			AND act.instituted_object_ref = NEW.permission_ref
			AND act.act_type_ref = policy.recognized_act_type_ref
			AND act.context_policy_ref = policy.context_policy_ref
			AND act.context_policy_digest = policy.context_policy_digest
			AND policy.context_policy_digest = NEW.context_policy_digest
			AND policy.bounded_context_ref = NEW.bounded_context_ref
			AND policy.instituted_object_kind = 'U.Commitment'
			AND policy.institutional_modality = NEW.modality
			AND policy.scoped_action = NEW.action_kind
			AND capture.capture_carrier_digest = NEW.capture_carrier_digest
			AND capture.project_root = NEW.project_root
			AND act.capture_carrier_ref = capture.capture_carrier_ref
			AND act.capture_carrier_digest = capture.capture_carrier_digest
			AND `+sqliteUTCNanoLessOrEqual("capture.ended_at", "NEW.valid_from")+`
			AND `+sqliteUTCNanoLessOrEqual("NEW.valid_until", "content.authorization_valid_until")+`
			AND EXISTS (
				SELECT 1 FROM json_each(NEW.adjudication_evidence_claim_refs_json)
				WHERE value = act.review_subject_ref
			)
			AND EXISTS (
				SELECT 1 FROM json_each(NEW.adjudication_carrier_refs_json)
				WHERE value = capture.capture_carrier_ref
			)
		) BEGIN
			SELECT RAISE(ABORT, 'permission is not licensed by exact content, policy, SpeechAct, and capture');
		END`,
		`CREATE TRIGGER IF NOT EXISTS speech_act_instituted_effects_exact_sources
		BEFORE INSERT ON speech_act_instituted_effects
		WHEN NOT EXISTS (
			SELECT 1 FROM speech_acts act
			JOIN profile_declaration_permissions permission ON permission.permission_ref = NEW.permission_ref
			WHERE act.speech_act_ref = NEW.speech_act_ref
			AND act.speech_act_digest = NEW.speech_act_digest
			AND permission.permission_digest = NEW.permission_digest
			AND act.project_root = NEW.project_root
			AND permission.project_root = NEW.project_root
			AND permission.source_speech_act_ref = act.speech_act_ref
			AND act.instituted_object_ref = permission.permission_ref
		) BEGIN
			SELECT RAISE(ABORT, 'instituted effect does not bind exact SpeechAct and permission');
		END`,
		`CREATE TRIGGER IF NOT EXISTS authority_basis_presentations_exact_graph
		BEFORE INSERT ON authority_basis_presentations
		WHEN NOT EXISTS (
			SELECT 1
			FROM speech_act_context_policies policy
			JOIN profile_declaration_authorization_contents content ON content.authorization_content_ref = NEW.authorization_content_ref
			JOIN terminal_capture_records capture ON capture.capture_carrier_ref = NEW.capture_carrier_ref
			JOIN speech_act_role_assignments authorizer ON authorizer.role_assignment_ref = NEW.authorizer_ref
			JOIN speech_acts act ON act.speech_act_ref = NEW.speech_act_ref
			JOIN profile_declaration_permissions permission ON permission.permission_ref = NEW.permission_ref
			JOIN speech_act_instituted_effects effect ON effect.instituted_effect_digest = NEW.instituted_effect_digest
			WHERE policy.context_policy_ref = NEW.context_policy_ref
			AND policy.context_policy_digest = NEW.context_policy_digest
			AND content.authorization_content_digest = NEW.authorization_content_digest
			AND capture.capture_carrier_digest = NEW.capture_carrier_digest
			AND authorizer.role_assignment_digest = NEW.authorizer_digest
			AND act.speech_act_digest = NEW.speech_act_digest
			AND permission.permission_digest = NEW.permission_digest
			AND content.project_root = NEW.project_root
			AND capture.project_root = NEW.project_root
			AND authorizer.project_root = NEW.project_root
			AND act.project_root = NEW.project_root
			AND permission.project_root = NEW.project_root
			AND effect.project_root = NEW.project_root
			AND act.performed_by_ref = authorizer.role_assignment_ref
			AND act.capture_carrier_ref = capture.capture_carrier_ref
			AND act.context_policy_ref = policy.context_policy_ref
			AND content.authorization_content_ref = permission.authorization_content_ref
			AND content.authorization_content_digest = permission.authorization_content_digest
			AND permission.context_policy_ref = policy.context_policy_ref
			AND permission.context_policy_digest = policy.context_policy_digest
			AND permission.capture_carrier_ref = capture.capture_carrier_ref
			AND permission.source_speech_act_ref = act.speech_act_ref
			AND effect.speech_act_ref = act.speech_act_ref
			AND effect.permission_ref = permission.permission_ref
		) BEGIN
			SELECT RAISE(ABORT, 'basis presentation does not join the exact authority graph');
		END`,
		`CREATE TRIGGER IF NOT EXISTS authority_basis_resolutions_exact_presentation
		BEFORE INSERT ON authority_basis_resolutions
		WHEN NOT EXISTS (
			SELECT 1
			FROM authority_basis_presentations presentation
			JOIN terminal_capture_records capture
				ON capture.capture_carrier_ref = presentation.capture_carrier_ref
			JOIN profile_declaration_authorization_contents content
				ON content.authorization_content_ref = presentation.authorization_content_ref
			JOIN profile_declaration_permissions permission
				ON permission.permission_ref = presentation.permission_ref
			WHERE presentation.presentation_id = NEW.presentation_id
			AND presentation.presentation_digest = NEW.presentation_digest
			AND presentation.project_root = NEW.project_root
			AND `+sqliteUTCNanoLessOrEqual("NEW.authority_valid_from", "capture.started_at")+`
			AND `+sqliteUTCNanoLessOrEqual("capture.ended_at", "NEW.resolved_at")+`
			AND `+sqliteUTCNanoLess("NEW.resolved_at", "NEW.valid_until")+`
			AND `+sqliteUTCNanoLess("NEW.resolved_at", "content.authorization_valid_until")+`
			AND `+sqliteUTCNanoLess("NEW.resolved_at", "permission.valid_until")+`
			AND `+sqliteUTCNanoLessOrEqual("NEW.valid_until", "content.authorization_valid_until")+`
			AND `+sqliteUTCNanoLessOrEqual("NEW.valid_until", "permission.valid_until")+`
		) BEGIN
			SELECT RAISE(ABORT, 'basis resolution does not bind exact presentation');
		END`,
		`CREATE TRIGGER IF NOT EXISTS authority_presentations_require_v38_basis
		BEFORE INSERT ON authority_presentations
		WHEN NOT EXISTS (
			SELECT 1
			FROM authority_basis_presentations basis
			JOIN profile_declaration_authorization_contents content
				ON content.authorization_content_ref = basis.authorization_content_ref
			JOIN profile_declaration_permissions permission
				ON permission.permission_ref = basis.permission_ref
			WHERE basis.presentation_id = NEW.presentation_id
			AND basis.legacy_projection_digest = NEW.presentation_digest
			AND basis.project_root = NEW.project_root
			AND basis.speech_act_ref = NEW.speech_act_ref
			AND basis.speech_act_digest = NEW.speech_act_digest
			AND basis.authorization_content_ref = NEW.authorization_content_ref
			AND basis.authorization_content_digest = NEW.authorization_content_digest
			AND basis.permission_ref = NEW.permission_ref
			AND basis.permission_digest = NEW.permission_digest
			AND basis.context_policy_ref = NEW.context_policy_ref
			AND basis.context_policy_digest = NEW.context_policy_digest
			AND permission.modality = NEW.permission_modality
			AND permission.source_speech_act_ref = NEW.permission_source_speech_act_ref
			AND permission.subject_ref = NEW.permission_subject_role_assignment_ref
			AND permission.authorization_content_ref = NEW.permission_authorization_content_ref
			AND permission.action_kind = NEW.permission_action_kind
			AND permission.project_root = NEW.permission_project_root
			AND permission.method_description_ref = NEW.permission_method_description_ref
			AND permission.valid_from = NEW.permission_valid_from
			AND permission.valid_until = NEW.permission_valid_until
			AND content.single_use_key = NEW.permission_single_use_key
			AND permission.admission_predicate_ref = NEW.permission_profile_admission_predicate_ref
			AND permission.context_policy_ref = NEW.permission_context_policy_ref
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
			AND content.session_ref = NEW.session_ref
			AND content.allowed_work_from = NEW.allowed_work_from
			AND content.allowed_work_until = NEW.allowed_work_until
			AND content.basis_observation_from = NEW.basis_observation_from
			AND content.basis_observation_until = NEW.basis_observation_until
			AND content.authorization_valid_from = NEW.valid_from
			AND content.authorization_valid_until = NEW.valid_until
			AND content.single_use_key = NEW.single_use_key
		) BEGIN
			SELECT RAISE(ABORT, 'legacy authority presentation requires exact v38 authority basis closure');
		END`,
		`CREATE TRIGGER IF NOT EXISTS authority_resolution_records_require_v38_basis
		BEFORE INSERT ON authority_resolution_records
		WHEN NOT EXISTS (
			SELECT 1
			FROM authority_basis_resolutions resolution
			JOIN authority_basis_presentations presentation
				ON presentation.presentation_id = resolution.presentation_id
			JOIN profile_declaration_authorization_contents content
				ON content.authorization_content_ref = presentation.authorization_content_ref
			WHERE resolution.authority_resolution_id = NEW.authority_resolution_id
			AND resolution.legacy_projection_digest = NEW.authority_resolution_digest
			AND resolution.presentation_id = NEW.presentation_id
			AND presentation.legacy_projection_digest = NEW.presentation_digest
			AND resolution.verifier_identity = NEW.verifier_identity
			AND resolution.verifier_version = NEW.verifier_version
			AND resolution.verification_policy_ref = NEW.verification_policy_ref
			AND resolution.verification_policy_digest = NEW.verification_policy_digest
			AND resolution.resolved_at = NEW.resolved_at
			AND resolution.valid_until = NEW.valid_until
			AND content.profile_author_role_assignment_ref = NEW.profile_author_role_assignment_ref
			AND content.profile_author_role_assignment_digest = NEW.profile_author_role_assignment_digest
			AND content.method_description_ref = NEW.method_description_ref
			AND content.method_description_digest = NEW.method_description_digest
			AND content.method_contract_ref = NEW.method_contract_ref
			AND content.method_contract_digest = NEW.method_contract_digest
		) BEGIN
			SELECT RAISE(ABORT, 'legacy authority resolution requires exact v38 authority basis closure');
		END`,
	)
}

type immutableAuthorityBasisTable struct {
	name         string
	primaryKey   string
	digestColumn string
}

func appendAuthorityBasisImmutabilityTriggers(statements []string) []string {
	tables := []immutableAuthorityBasisTable{
		{name: "speech_act_method_descriptions", primaryKey: "method_description_ref", digestColumn: "method_description_digest"},
		{name: "speech_act_context_policies", primaryKey: "context_policy_ref", digestColumn: "context_policy_digest"},
		{name: "profile_declaration_authorization_contents", primaryKey: "authorization_content_ref", digestColumn: "authorization_content_digest"},
		{name: "terminal_capture_records", primaryKey: "capture_carrier_ref", digestColumn: "capture_carrier_digest"},
		{name: "speech_act_role_assignments", primaryKey: "role_assignment_ref", digestColumn: "role_assignment_digest"},
		{name: "speech_acts", primaryKey: "speech_act_ref", digestColumn: "speech_act_digest"},
		{name: "profile_declaration_permissions", primaryKey: "permission_ref", digestColumn: "permission_digest"},
		{name: "speech_act_instituted_effects", primaryKey: "instituted_effect_digest", digestColumn: "instituted_effect_digest"},
		{name: "authority_basis_presentations", primaryKey: "presentation_id", digestColumn: "presentation_digest"},
		{name: "authority_basis_resolutions", primaryKey: "authority_resolution_id", digestColumn: "authority_resolution_digest"},
	}
	return appendAuthorityBasisTableTriggers(statements, tables, 0)
}

func appendAuthorityBasisTableTriggers(
	statements []string,
	tables []immutableAuthorityBasisTable,
	index int,
) []string {
	if index >= len(tables) {
		return statements
	}
	table := tables[index]
	noReplace := "CREATE TRIGGER IF NOT EXISTS " + table.name + "_no_replace " +
		"BEFORE INSERT ON " + table.name + " WHEN EXISTS (SELECT 1 FROM " + table.name +
		" existing WHERE existing." + table.primaryKey + " = NEW." + table.primaryKey +
		" OR existing." + table.digestColumn + " = NEW." + table.digestColumn +
		") BEGIN SELECT RAISE(ABORT, '" + table.name + " is append-only'); END"
	noUpdate := "CREATE TRIGGER IF NOT EXISTS " + table.name + "_no_update " +
		"BEFORE UPDATE ON " + table.name + " BEGIN SELECT RAISE(ABORT, '" + table.name + " is append-only'); END"
	noDelete := "CREATE TRIGGER IF NOT EXISTS " + table.name + "_no_delete " +
		"BEFORE DELETE ON " + table.name + " BEGIN SELECT RAISE(ABORT, '" + table.name + " is append-only'); END"
	next := slices.Clone(statements)
	next = append(next, noReplace, noUpdate, noDelete)
	return appendAuthorityBasisTableTriggers(next, tables, index+1)
}

func appendAuthorityBasisRootGuards(statements []string) []string {
	tables := []string{
		"profile_declaration_authorization_contents",
		"terminal_capture_records",
		"speech_act_role_assignments",
		"speech_acts",
		"profile_declaration_permissions",
		"speech_act_instituted_effects",
		"authority_basis_presentations",
		"authority_basis_resolutions",
	}
	return appendAuthorityBasisRootGuard(statements, tables, 0)
}

func appendAuthorityBasisRootGuard(
	statements []string,
	tables []string,
	index int,
) []string {
	if index >= len(tables) {
		return statements
	}
	table := tables[index]
	statement := "CREATE TRIGGER IF NOT EXISTS " + table + "_project_ledger_root " +
		"BEFORE INSERT ON " + table + " WHEN EXISTS (SELECT 1 FROM project_ledger_binding) " +
		"AND NOT EXISTS (SELECT 1 FROM project_ledger_binding binding WHERE binding.project_root = NEW.project_root) " +
		"BEGIN SELECT RAISE(ABORT, '" + table + " does not match the bound project ledger root'); END"
	next := slices.Clone(statements)
	next = append(next, statement)
	return appendAuthorityBasisRootGuard(next, tables, index+1)
}
