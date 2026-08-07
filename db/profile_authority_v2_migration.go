package db

import (
	"fmt"
	"slices"
)

const (
	profileAuthorityV2ContentTable     = "profile_declaration_authorization_contents_v2"
	profileAuthorityV2PreparationTable = "profile_declaration_authorization_preparations_v2"
	profileAuthorityV2PermissionTable  = "profile_declaration_permissions_v2"
	profileAuthorityV2EffectTable      = "profile_declaration_instituted_effects_v2"
	profileAuthorityV2BasisTable       = "profile_declaration_authority_bases_v2"

	profileAuthorityV2PolicyRef      = "context-policy:profile-declaration-authorization:v2"
	profileAuthorityV2EffectRuleRef  = "institution-rule:authorize-institutes-profile-permission-may:v2"
	profileAuthorityV2UtteranceRef   = "utterance:profile-declaration-authorization:v2"
	profileAuthorityV2UtteranceVerb  = "AUTHORIZE"
	profileAuthorityV2UtteranceText  = "REVIEWED PROJECT PROFILE"
	profileAuthorityV2ContextRef     = "bounded-context:profile-declaration-authority"
	profileAuthorityV2ActType        = "speech-act-type:authorize-profile-declaration"
	profileAuthorityV2Action         = "profile.declare.from_onboarding_candidate"
	profileAuthorityV2MethodRef      = "method:profile-declaration-authorization"
	profileAuthorityV2MethodDescRef  = "method-description:profile-declaration-authorization:v2"
	profileAuthorityV2ProcedureRef   = "procedure:review-profile-intent-capture-controlling-terminal:v2"
	profileAuthorityV2ProcedureText  = "display exact pre-act review bindings; require the policy-owned canonical utterance on the controlling terminal; observe terminal session and capture time; derive capture, authorizer assignment, and SpeechAct in that order"
	profileAuthorityV2SystemRef      = "system:haft-profile-authority"
	profileAuthorityV2StatePlaneRef  = "state-plane:profile-declaration-permission"
	profileAuthorityV2DeltaRef       = "delta-predicate:profile-permission-instituted"
	profileAuthorityV2OutcomeRef     = "work-outcome:profile-permission-instituted"
	profileAuthorityV2PermissionKind = "U.Commitment"
)

var profileAuthorityV2Migration43 = Migration{
	Version:     43,
	Description: "Add source-native typed profile-declaration authority closure",
	Apply:       applyProfileAuthorityV2Migration43,
}

var profileAuthorityV2Tables = []string{
	profileAuthorityV2ContentTable,
	profileAuthorityV2PreparationTable,
	profileAuthorityV2PermissionTable,
	profileAuthorityV2EffectTable,
	profileAuthorityV2BasisTable,
}

func applyProfileAuthorityV2Migration43(
	tx MigrationTransaction,
	_ []Migration,
) error {
	if err := requireProfileAuthorityV2Source43(tx); err != nil {
		return err
	}
	if err := requireAbsentProfileAuthorityV2Footprint43(tx, 0); err != nil {
		return err
	}
	statements := profileAuthorityV2Statements43()
	if err := executeStatements(tx, statements, 0); err != nil {
		return fmt.Errorf("install profile authority v2: %w", err)
	}
	if err := verifyForeignKeys(tx); err != nil {
		return fmt.Errorf("verify profile authority v2: %w", err)
	}
	return nil
}

func requireProfileAuthorityV2Source43(tx MigrationTransaction) error {
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version IN (38, 39, 40, 41, 42)",
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect profile authority v2 source migrations: %w", err)
	}
	if count != 5 {
		return fmt.Errorf("profile authority v2 requires schema versions 38 through 42")
	}
	return nil
}

func requireAbsentProfileAuthorityV2Footprint43(
	tx MigrationTransaction,
	index int,
) error {
	if index >= len(profileAuthorityV2Tables) {
		return nil
	}
	table := profileAuthorityV2Tables[index]
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect profile authority v2 table %s: %w", table, err)
	}
	if count != 0 {
		return fmt.Errorf(
			"profile authority v2 refused: unversioned table %s already exists; unknown partial schema requires manual review",
			table,
		)
	}
	return requireAbsentProfileAuthorityV2Footprint43(tx, index+1)
}

func profileAuthorityV2Statements43() []string {
	statements := []string{
		profileAuthorityContentTable43(),
		profileAuthorityPreparationTable43(),
		profileAuthorityPermissionTable43(),
		profileAuthorityEffectTable43(),
		profileAuthorityBasisTable43(),
		profileAuthorityContentExactSourcesTrigger43(),
		profileAuthorityPreparationExactSourcesTrigger43(),
		profileAuthorityPermissionExactSourcesTrigger43(),
		profileAuthorityEffectExactSourcesTrigger43(),
		profileAuthorityBasisExactSourcesTrigger43(),
		profileAuthorityContentLegacyCollisionTrigger43(),
		profileAuthorityPreparationLegacyCollisionTrigger43(),
		profileAuthorityPermissionLegacyCollisionTrigger43(),
		profileAuthorityEffectLegacyCollisionTrigger43(),
		profileAuthorityBasisLegacyCollisionTrigger43(),
	}
	immutable := []immutableAuthorityBasisTable{
		{name: profileAuthorityV2ContentTable, primaryKey: "authorization_content_ref", digestColumn: "authorization_content_digest"},
		{name: profileAuthorityV2PreparationTable, primaryKey: "prepared_authorization_digest", digestColumn: "prepared_authorization_digest"},
		{name: profileAuthorityV2PermissionTable, primaryKey: "permission_ref", digestColumn: "permission_digest"},
		{name: profileAuthorityV2EffectTable, primaryKey: "effect_digest", digestColumn: "effect_digest"},
		{name: profileAuthorityV2BasisTable, primaryKey: "basis_ref", digestColumn: "basis_digest"},
	}
	statements = appendAuthorityBasisTableTriggers(statements, immutable, 0)
	return appendProfileAuthorityV2RootGuards43(statements, 0)
}

func profileAuthorityContentTable43() string {
	return `CREATE TABLE profile_declaration_authorization_contents_v2 (
		authorization_content_ref TEXT PRIMARY KEY,
		authorization_content_digest TEXT NOT NULL UNIQUE CHECK(length(authorization_content_digest) = 71 AND substr(authorization_content_digest, 1, 7) = 'sha256:'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		action_kind TEXT NOT NULL CHECK(action_kind = '` + profileAuthorityV2Action + `'),
		profile_author_role_assignment_ref TEXT NOT NULL REFERENCES profile_author_role_assignments(role_assignment_ref),
		profile_author_role_assignment_digest TEXT NOT NULL CHECK(length(profile_author_role_assignment_digest) = 71 AND substr(profile_author_role_assignment_digest, 1, 7) = 'sha256:'),
		method_description_ref TEXT NOT NULL REFERENCES profile_onboarding_method_descriptions(method_description_ref),
		method_description_digest TEXT NOT NULL CHECK(length(method_description_digest) = 71 AND substr(method_description_digest, 1, 7) = 'sha256:'),
		method_contract_ref TEXT NOT NULL REFERENCES profile_onboarding_method_contracts(method_contract_ref),
		method_contract_digest TEXT NOT NULL CHECK(length(method_contract_digest) = 71 AND substr(method_contract_digest, 1, 7) = 'sha256:'),
		classifier_version TEXT NOT NULL CHECK(classifier_version != ''),
		policy_version TEXT NOT NULL CHECK(policy_version != ''),
		session_ref TEXT NOT NULL CHECK(session_ref != ''),
		allowed_work_from TEXT NOT NULL,
		allowed_work_until TEXT NOT NULL,
		basis_observation_from TEXT NOT NULL,
		basis_observation_until TEXT NOT NULL,
		authorization_valid_from TEXT NOT NULL,
		authorization_valid_until TEXT NOT NULL,
		single_use_key TEXT NOT NULL UNIQUE CHECK(single_use_key != ''),
		canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
		recorded_at TEXT NOT NULL CHECK(recorded_at != ''),
		CHECK(COALESCE(json_extract(canonical_json, '$.schema') = 'haft.profile-authority.authorization-content/v1', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authorization_content_ref') = authorization_content_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.project_root') = project_root, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.action_kind') = action_kind, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.profile_author_role_assignment_ref') = profile_author_role_assignment_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.profile_author_role_assignment_digest') = profile_author_role_assignment_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.method_description_ref') = method_description_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.method_description_digest') = method_description_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.method_contract_ref') = method_contract_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.method_contract_digest') = method_contract_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.classifier_version') = classifier_version, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.policy_version') = policy_version, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.session_ref') = session_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.allowed_work_from') = allowed_work_from, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.allowed_work_until') = allowed_work_until, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.basis_observation_from') = basis_observation_from, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.basis_observation_until') = basis_observation_until, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authorization_valid_from') = authorization_valid_from, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authorization_valid_until') = authorization_valid_until, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.single_use_key') = single_use_key, 0)),
		CHECK(` + sqliteCanonicalUTCNanoShape("allowed_work_from") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("allowed_work_until") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("basis_observation_from") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("basis_observation_until") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("authorization_valid_from") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("authorization_valid_until") + `),
		CHECK(` + sqliteUTCNanoLess("allowed_work_from", "allowed_work_until") + `),
		CHECK(` + sqliteUTCNanoLess("basis_observation_from", "basis_observation_until") + `),
		CHECK(` + sqliteUTCNanoLess("authorization_valid_from", "authorization_valid_until") + `),
		CHECK(` + sqliteUTCNanoLessOrEqual("authorization_valid_from", "allowed_work_from") + `),
		CHECK(` + sqliteUTCNanoLessOrEqual("allowed_work_until", "authorization_valid_until") + `),
		CHECK(` + sqliteUTCNanoLessOrEqual("authorization_valid_from", "basis_observation_from") + `),
		CHECK(` + sqliteUTCNanoLessOrEqual("basis_observation_until", "authorization_valid_until") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `)
	) WITHOUT ROWID`
}

func profileAuthorityPreparationTable43() string {
	return `CREATE TABLE profile_declaration_authorization_preparations_v2 (
		prepared_authorization_digest TEXT PRIMARY KEY CHECK(length(prepared_authorization_digest) = 71 AND substr(prepared_authorization_digest, 1, 7) = 'sha256:'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		authorization_content_ref TEXT NOT NULL UNIQUE REFERENCES profile_declaration_authorization_contents_v2(authorization_content_ref),
		authorization_content_digest TEXT NOT NULL UNIQUE CHECK(length(authorization_content_digest) = 71 AND substr(authorization_content_digest, 1, 7) = 'sha256:'),
		permission_ref TEXT NOT NULL UNIQUE CHECK(permission_ref != ''),
		speech_act_ref TEXT NOT NULL UNIQUE CHECK(speech_act_ref != ''),
		capture_carrier_ref TEXT NOT NULL UNIQUE CHECK(capture_carrier_ref != ''),
		speech_act_session_ref TEXT NOT NULL CHECK(speech_act_session_ref != ''),
		claim_scope_ref TEXT NOT NULL CHECK(claim_scope_ref != ''),
		enactability_predicate_ref TEXT NOT NULL CHECK(enactability_predicate_ref GLOB 'A-?*'),
		evidence_claim_ref TEXT NOT NULL CHECK(evidence_claim_ref GLOB 'E-?*'),
		carrier_class_ref TEXT NOT NULL CHECK(carrier_class_ref GLOB 'carrier-class:?*'),
		verifier_identity TEXT NOT NULL CHECK(verifier_identity != ''),
		verifier_version TEXT NOT NULL CHECK(verifier_version != ''),
		verification_policy_ref TEXT NOT NULL CHECK(verification_policy_ref != ''),
		verification_policy_digest TEXT NOT NULL CHECK(length(verification_policy_digest) = 71 AND substr(verification_policy_digest, 1, 7) = 'sha256:'),
		basis_ref TEXT NOT NULL UNIQUE CHECK(basis_ref GLOB 'profile-authority-basis:?*'),
		context_policy_ref TEXT NOT NULL CHECK(context_policy_ref = '` + profileAuthorityV2PolicyRef + `'),
		context_policy_digest TEXT NOT NULL CHECK(length(context_policy_digest) = 71 AND substr(context_policy_digest, 1, 7) = 'sha256:'),
		speech_act_intent_digest TEXT NOT NULL UNIQUE CHECK(length(speech_act_intent_digest) = 71 AND substr(speech_act_intent_digest, 1, 7) = 'sha256:'),
		canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
		recorded_at TEXT NOT NULL CHECK(recorded_at != ''),
		CHECK(COALESCE(json_extract(canonical_json, '$.schema') = 'haft.profile-authority.prepared-authorization/v1', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authorization_content_ref') = authorization_content_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authorization_content_digest') = authorization_content_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.permission_ref') = permission_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.speech_act_ref') = speech_act_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.capture_carrier_ref') = capture_carrier_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.speech_act_session_ref') = speech_act_session_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.claim_scope_ref') = claim_scope_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.enactability_predicate_ref') = enactability_predicate_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.evidence_claim_ref') = evidence_claim_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.carrier_class_ref') = carrier_class_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.verifier_identity') = verifier_identity, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.verifier_version') = verifier_version, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.verification_policy_ref') = verification_policy_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.verification_policy_digest') = verification_policy_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.basis_ref') = basis_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.context_policy_ref') = context_policy_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.context_policy_digest') = context_policy_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.speech_act_intent_digest') = speech_act_intent_digest, 0)),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `)
	) WITHOUT ROWID`
}

func profileAuthorityPermissionTable43() string {
	return `CREATE TABLE profile_declaration_permissions_v2 (
		permission_ref TEXT PRIMARY KEY,
		permission_digest TEXT NOT NULL UNIQUE CHECK(length(permission_digest) = 71 AND substr(permission_digest, 1, 7) = 'sha256:'),
		prepared_authorization_digest TEXT NOT NULL UNIQUE REFERENCES profile_declaration_authorization_preparations_v2(prepared_authorization_digest),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		permission_kind TEXT NOT NULL CHECK(permission_kind = '` + profileAuthorityV2PermissionKind + `'),
		subject_ref TEXT NOT NULL REFERENCES profile_author_role_assignments(role_assignment_ref),
		subject_digest TEXT NOT NULL CHECK(length(subject_digest) = 71 AND substr(subject_digest, 1, 7) = 'sha256:'),
		modality TEXT NOT NULL CHECK(modality = 'MAY'),
		action_kind TEXT NOT NULL CHECK(action_kind = '` + profileAuthorityV2Action + `'),
		claim_scope_ref TEXT NOT NULL CHECK(claim_scope_ref != ''),
		bounded_context_ref TEXT NOT NULL CHECK(bounded_context_ref = '` + profileAuthorityV2ContextRef + `'),
		valid_from TEXT NOT NULL,
		valid_until TEXT NOT NULL,
		referents_json TEXT NOT NULL CHECK(json_valid(referents_json) AND json_type(referents_json) = 'array' AND json_array_length(referents_json) = 2),
		authorization_content_ref TEXT NOT NULL UNIQUE REFERENCES profile_declaration_authorization_contents_v2(authorization_content_ref),
		authorization_content_digest TEXT NOT NULL CHECK(length(authorization_content_digest) = 71 AND substr(authorization_content_digest, 1, 7) = 'sha256:'),
		method_description_ref TEXT NOT NULL REFERENCES profile_onboarding_method_descriptions(method_description_ref),
		method_description_digest TEXT NOT NULL CHECK(length(method_description_digest) = 71 AND substr(method_description_digest, 1, 7) = 'sha256:'),
		enactability_predicate_ref TEXT NOT NULL CHECK(enactability_predicate_ref GLOB 'A-?*'),
		adjudication_evidence_claim_refs_json TEXT NOT NULL CHECK(json_valid(adjudication_evidence_claim_refs_json) AND json_type(adjudication_evidence_claim_refs_json) = 'array' AND json_array_length(adjudication_evidence_claim_refs_json) > 0),
		adjudication_carrier_class_refs_json TEXT NOT NULL CHECK(json_valid(adjudication_carrier_class_refs_json) AND json_type(adjudication_carrier_class_refs_json) = 'array' AND json_array_length(adjudication_carrier_class_refs_json) > 0),
		adjudication_verifier_identity TEXT NOT NULL CHECK(adjudication_verifier_identity != ''),
		adjudication_verifier_version TEXT NOT NULL CHECK(adjudication_verifier_version != ''),
		adjudication_evaluation_policy_ref TEXT NOT NULL CHECK(adjudication_evaluation_policy_ref != ''),
		adjudication_evaluation_policy_digest TEXT NOT NULL CHECK(length(adjudication_evaluation_policy_digest) = 71 AND substr(adjudication_evaluation_policy_digest, 1, 7) = 'sha256:'),
		source_speech_act_ref TEXT NOT NULL UNIQUE REFERENCES speech_acts(speech_act_ref),
		source_speech_act_digest TEXT NOT NULL CHECK(length(source_speech_act_digest) = 71 AND substr(source_speech_act_digest, 1, 7) = 'sha256:'),
		context_policy_ref TEXT NOT NULL REFERENCES speech_act_context_policies(context_policy_ref) CHECK(context_policy_ref = '` + profileAuthorityV2PolicyRef + `'),
		context_policy_digest TEXT NOT NULL CHECK(length(context_policy_digest) = 71 AND substr(context_policy_digest, 1, 7) = 'sha256:'),
		capture_carrier_ref TEXT NOT NULL UNIQUE REFERENCES terminal_capture_records(capture_carrier_ref),
		capture_carrier_digest TEXT NOT NULL CHECK(length(capture_carrier_digest) = 71 AND substr(capture_carrier_digest, 1, 7) = 'sha256:'),
		canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
		recorded_at TEXT NOT NULL CHECK(recorded_at != ''),
		CHECK(COALESCE(json_extract(canonical_json, '$.schema') = 'haft.profile-authority.permission/v2', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.permission_ref') = permission_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.project_root') = project_root, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.subject_role_assignment_ref') = subject_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.subject_role_assignment_digest') = subject_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.modality') = modality, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.action_kind') = action_kind, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.claim_scope_ref') = claim_scope_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.bounded_context_ref') = bounded_context_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.valid_from') = valid_from, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.valid_until') = valid_until, 0)),
		CHECK(json(json_extract(canonical_json, '$.referents')) = json(referents_json)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authorization_content_ref') = authorization_content_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authorization_content_digest') = authorization_content_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.method_description_ref') = method_description_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.method_description_digest') = method_description_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.enactability_predicate_ref') = enactability_predicate_ref, 0)),
		CHECK(json(json_extract(canonical_json, '$.adjudication_evidence_claim_refs')) = json(adjudication_evidence_claim_refs_json)),
		CHECK(json(json_extract(canonical_json, '$.adjudication_carrier_class_refs')) = json(adjudication_carrier_class_refs_json)),
		CHECK(COALESCE(json_extract(canonical_json, '$.adjudication_verifier_identity') = adjudication_verifier_identity, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.adjudication_verifier_version') = adjudication_verifier_version, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.adjudication_evaluation_policy_ref') = adjudication_evaluation_policy_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.adjudication_evaluation_policy_digest') = adjudication_evaluation_policy_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.source_speech_act_ref') = source_speech_act_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.source_speech_act_digest') = source_speech_act_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.context_policy_ref') = context_policy_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.context_policy_digest') = context_policy_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.instituting_terminal_capture_carrier_ref') = capture_carrier_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.instituting_terminal_capture_carrier_digest') = capture_carrier_digest, 0)),
		CHECK(COALESCE(json_extract(referents_json, '$[0]') = method_description_ref, 0)),
		CHECK(COALESCE(json_extract(referents_json, '$[1]') = enactability_predicate_ref, 0)),
		CHECK(` + sqliteCanonicalUTCNanoShape("valid_from") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("valid_until") + `),
		CHECK(` + sqliteUTCNanoLess("valid_from", "valid_until") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `)
	) WITHOUT ROWID`
}

func profileAuthorityEffectTable43() string {
	return `CREATE TABLE profile_declaration_instituted_effects_v2 (
		effect_digest TEXT PRIMARY KEY CHECK(length(effect_digest) = 71 AND substr(effect_digest, 1, 7) = 'sha256:'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		speech_act_ref TEXT NOT NULL UNIQUE REFERENCES speech_acts(speech_act_ref),
		speech_act_digest TEXT NOT NULL CHECK(length(speech_act_digest) = 71 AND substr(speech_act_digest, 1, 7) = 'sha256:'),
		permission_ref TEXT NOT NULL UNIQUE REFERENCES profile_declaration_permissions_v2(permission_ref),
		permission_digest TEXT NOT NULL CHECK(length(permission_digest) = 71 AND substr(permission_digest, 1, 7) = 'sha256:'),
		canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
		recorded_at TEXT NOT NULL CHECK(recorded_at != ''),
		CHECK(COALESCE(json_extract(canonical_json, '$.schema') = 'haft.profile-authority.instituted-permission-effect/v2', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.project_root') = project_root, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.speech_act_ref') = speech_act_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.speech_act_digest') = speech_act_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.permission_ref') = permission_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.permission_digest') = permission_digest, 0)),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `)
	) WITHOUT ROWID`
}

func profileAuthorityBasisTable43() string {
	return `CREATE TABLE profile_declaration_authority_bases_v2 (
		basis_ref TEXT PRIMARY KEY CHECK(basis_ref GLOB 'profile-authority-basis:?*'),
		basis_digest TEXT NOT NULL UNIQUE CHECK(length(basis_digest) = 71 AND substr(basis_digest, 1, 7) = 'sha256:'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		speech_act_ref TEXT NOT NULL UNIQUE REFERENCES speech_acts(speech_act_ref),
		speech_act_digest TEXT NOT NULL CHECK(length(speech_act_digest) = 71 AND substr(speech_act_digest, 1, 7) = 'sha256:'),
		authorization_content_ref TEXT NOT NULL UNIQUE REFERENCES profile_declaration_authorization_contents_v2(authorization_content_ref),
		authorization_content_digest TEXT NOT NULL CHECK(length(authorization_content_digest) = 71 AND substr(authorization_content_digest, 1, 7) = 'sha256:'),
		permission_ref TEXT NOT NULL UNIQUE REFERENCES profile_declaration_permissions_v2(permission_ref),
		permission_digest TEXT NOT NULL CHECK(length(permission_digest) = 71 AND substr(permission_digest, 1, 7) = 'sha256:'),
		context_policy_ref TEXT NOT NULL REFERENCES speech_act_context_policies(context_policy_ref),
		context_policy_digest TEXT NOT NULL CHECK(length(context_policy_digest) = 71 AND substr(context_policy_digest, 1, 7) = 'sha256:'),
		instituted_effect_digest TEXT NOT NULL UNIQUE REFERENCES profile_declaration_instituted_effects_v2(effect_digest),
		canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
		recorded_at TEXT NOT NULL CHECK(recorded_at != ''),
		CHECK(COALESCE(json_extract(canonical_json, '$.schema') = 'haft.profile-authority.four-ref-basis/v1', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.basis_ref') = basis_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.project_root') = project_root, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.speech_act_ref') = speech_act_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.speech_act_digest') = speech_act_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authorization_content_ref') = authorization_content_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authorization_content_digest') = authorization_content_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.permission_ref') = permission_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.permission_digest') = permission_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.context_policy_ref') = context_policy_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.context_policy_digest') = context_policy_digest, 0)),
		CHECK(json_type(canonical_json, '$.instituted_effect_digest') IS NULL),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `)
	) WITHOUT ROWID`
}

func profileAuthorityContentExactSourcesTrigger43() string {
	return `CREATE TRIGGER profile_declaration_authorization_contents_v2_exact_sources
	 BEFORE INSERT ON profile_declaration_authorization_contents_v2
	 WHEN NOT EXISTS (
		SELECT 1
		FROM profile_author_role_assignments assignment
		JOIN profile_author_assignment_support_carriers support
			ON support.assignment_justification_ref = assignment.assignment_justification_ref
		JOIN profile_onboarding_method_descriptions description
			ON description.method_description_ref = NEW.method_description_ref
		JOIN profile_onboarding_method_contracts contract
			ON contract.method_contract_ref = NEW.method_contract_ref
		WHERE assignment.role_assignment_ref = NEW.profile_author_role_assignment_ref
		AND assignment.role_assignment_digest = NEW.profile_author_role_assignment_digest
		AND support.assignment_justification_digest = assignment.assignment_justification_digest
		AND support.method_contract_ref = contract.method_contract_ref
		AND support.method_contract_digest = contract.method_contract_digest
		AND support.session_ref = NEW.session_ref
		AND description.method_description_digest = NEW.method_description_digest
		AND contract.method_contract_digest = NEW.method_contract_digest
		AND contract.method_description_ref = description.method_description_ref
		AND contract.method_description_digest = description.method_description_digest
		AND ` + sqliteUTCNanoLessOrEqual("assignment.valid_from", "NEW.authorization_valid_from") + `
		AND ` + sqliteUTCNanoLessOrEqual("NEW.authorization_valid_until", "assignment.valid_until") + `
	 ) BEGIN
		SELECT RAISE(ABORT, 'profile authority content does not bind the exact ProfileAuthor and onboarding method sources');
	 END`
}

func profileAuthorityPreparationExactSourcesTrigger43() string {
	return `CREATE TRIGGER profile_declaration_authorization_preparations_v2_exact_sources
	 BEFORE INSERT ON profile_declaration_authorization_preparations_v2
	 WHEN NOT EXISTS (
		SELECT 1 FROM profile_declaration_authorization_contents_v2 content
		WHERE content.authorization_content_ref = NEW.authorization_content_ref
		AND content.authorization_content_digest = NEW.authorization_content_digest
		AND content.project_root = NEW.project_root
		AND content.session_ref != NEW.speech_act_session_ref
		AND NEW.context_policy_ref = '` + profileAuthorityV2PolicyRef + `'
	 ) BEGIN
		SELECT RAISE(ABORT, 'prepared profile authorization does not bind exact durable content and separate SpeechAct session');
	 END`
}

func profileAuthorityPermissionExactSourcesTrigger43() string {
	return `CREATE TRIGGER profile_declaration_permissions_v2_exact_sources
	 BEFORE INSERT ON profile_declaration_permissions_v2
	 WHEN NOT EXISTS (
		SELECT 1
		FROM profile_declaration_authorization_preparations_v2 prepared
		JOIN profile_declaration_authorization_contents_v2 content
			ON content.authorization_content_ref = prepared.authorization_content_ref
		JOIN speech_acts act ON act.speech_act_ref = NEW.source_speech_act_ref
		JOIN terminal_capture_records capture ON capture.capture_carrier_ref = NEW.capture_carrier_ref
		JOIN speech_act_context_policies policy ON policy.context_policy_ref = NEW.context_policy_ref
		JOIN speech_act_method_descriptions method ON method.method_description_ref = act.method_description_ref
		JOIN speech_act_role_assignments authorizer ON authorizer.role_assignment_ref = act.performed_by_ref
		JOIN profile_author_role_assignments subject ON subject.role_assignment_ref = NEW.subject_ref
		WHERE prepared.prepared_authorization_digest = NEW.prepared_authorization_digest
		AND prepared.project_root = NEW.project_root
		AND prepared.permission_ref = NEW.permission_ref
		AND prepared.speech_act_ref = NEW.source_speech_act_ref
		AND prepared.capture_carrier_ref = NEW.capture_carrier_ref
		AND prepared.claim_scope_ref = NEW.claim_scope_ref
		AND prepared.enactability_predicate_ref = NEW.enactability_predicate_ref
		AND prepared.verifier_identity = NEW.adjudication_verifier_identity
		AND prepared.verifier_version = NEW.adjudication_verifier_version
		AND prepared.verification_policy_ref = NEW.adjudication_evaluation_policy_ref
		AND prepared.verification_policy_digest = NEW.adjudication_evaluation_policy_digest
		AND prepared.context_policy_ref = NEW.context_policy_ref
		AND prepared.context_policy_digest = NEW.context_policy_digest
		AND content.authorization_content_digest = prepared.authorization_content_digest
		AND content.authorization_content_ref = NEW.authorization_content_ref
		AND content.authorization_content_digest = NEW.authorization_content_digest
		AND content.project_root = NEW.project_root
		AND content.action_kind = NEW.action_kind
		AND content.profile_author_role_assignment_ref = NEW.subject_ref
		AND content.profile_author_role_assignment_digest = NEW.subject_digest
		AND content.method_description_ref = NEW.method_description_ref
		AND content.method_description_digest = NEW.method_description_digest
		AND subject.role_assignment_digest = NEW.subject_digest
		AND NEW.permission_kind = '` + profileAuthorityV2PermissionKind + `'
		AND NEW.modality = 'MAY'
		AND NEW.bounded_context_ref = '` + profileAuthorityV2ContextRef + `'
		AND NEW.valid_from = capture.ended_at
		AND NEW.valid_until = content.authorization_valid_until
		AND ` + sqliteUTCNanoLessOrEqual("content.authorization_valid_from", "capture.started_at") + `
		AND ` + sqliteUTCNanoLessOrEqual("capture.ended_at", "content.authorization_valid_until") + `
		AND json_array_length(NEW.referents_json) = 2
		AND json_extract(NEW.referents_json, '$[0]') = NEW.method_description_ref
		AND json_extract(NEW.referents_json, '$[1]') = NEW.enactability_predicate_ref
		AND json_array_length(NEW.adjudication_evidence_claim_refs_json) = 1
		AND json_extract(NEW.adjudication_evidence_claim_refs_json, '$[0]') = prepared.evidence_claim_ref
		AND json_array_length(NEW.adjudication_carrier_class_refs_json) = 1
		AND json_extract(NEW.adjudication_carrier_class_refs_json, '$[0]') = prepared.carrier_class_ref
		AND act.speech_act_digest = NEW.source_speech_act_digest
		AND act.project_root = NEW.project_root
		AND act.work_kind = 'Communicative'
		AND act.act_type_ref = '` + profileAuthorityV2ActType + `'
		AND act.review_subject_ref = content.authorization_content_ref
		AND act.review_subject_digest = content.authorization_content_digest
		AND act.instituted_object_ref = NEW.permission_ref
		AND act.context_policy_ref = NEW.context_policy_ref
		AND act.context_policy_digest = NEW.context_policy_digest
		AND act.method_ref = '` + profileAuthorityV2MethodRef + `'
		AND act.method_description_ref = '` + profileAuthorityV2MethodDescRef + `'
		AND act.executed_within_ref = '` + profileAuthorityV2SystemRef + `'
		AND act.bounded_context_ref = '` + profileAuthorityV2ContextRef + `'
		AND act.window_from = capture.started_at
		AND act.window_until = capture.ended_at
		AND act.state_plane_ref = '` + profileAuthorityV2StatePlaneRef + `'
		AND act.delta_predicate_ref = '` + profileAuthorityV2DeltaRef + `'
		AND act.outcome_ref = '` + profileAuthorityV2OutcomeRef + `'
		AND act.utterance_ref = '` + profileAuthorityV2UtteranceRef + `'
		AND act.capture_carrier_ref = capture.capture_carrier_ref
		AND act.capture_carrier_digest = capture.capture_carrier_digest
		AND capture.capture_carrier_digest = NEW.capture_carrier_digest
		AND capture.project_root = NEW.project_root
		AND capture.prepared_speech_act_intent_digest = prepared.speech_act_intent_digest
		AND capture.intent_session_ref = prepared.speech_act_session_ref
		AND capture.canonical_utterance = '` + profileAuthorityV2UtteranceVerb + ` ` + profileAuthorityV2UtteranceText + `'
		AND authorizer.role_assignment_digest = act.performed_by_digest
		AND authorizer.project_root = NEW.project_root
		AND authorizer.bounded_context_ref = '` + profileAuthorityV2ContextRef + `'
		AND authorizer.valid_from = capture.started_at
		AND authorizer.valid_until = capture.ended_at
		AND authorizer.context_policy_ref = NEW.context_policy_ref
		AND authorizer.context_policy_digest = NEW.context_policy_digest
		AND authorizer.provenance_carrier_ref = capture.capture_carrier_ref
		AND authorizer.provenance_carrier_digest = capture.capture_carrier_digest
		AND policy.context_policy_digest = NEW.context_policy_digest
		AND policy.bounded_context_ref = '` + profileAuthorityV2ContextRef + `'
		AND policy.recognized_act_type_ref = '` + profileAuthorityV2ActType + `'
		AND policy.authorizer_role_ref = 'role:project-principal-authorizer'
		AND policy.admitted_holder_kind = 'U.System'
		AND policy.assignment_source_rule = 'observed-local-controlling-terminal-session/v1'
		AND policy.institutional_effect_rule_ref = '` + profileAuthorityV2EffectRuleRef + `'
		AND policy.instituted_object_kind = '` + profileAuthorityV2PermissionKind + `'
		AND policy.institutional_modality = 'MAY'
		AND policy.scoped_action = '` + profileAuthorityV2Action + `'
		AND policy.utterance_description_ref = '` + profileAuthorityV2UtteranceRef + `'
		AND policy.utterance_verb = '` + profileAuthorityV2UtteranceVerb + `'
		AND policy.utterance_binding = 'literal'
		AND policy.utterance_literal = '` + profileAuthorityV2UtteranceText + `'
		AND method.method_description_digest = act.method_description_digest
		AND method.method_ref = '` + profileAuthorityV2MethodRef + `'
		AND method.procedure_ref = '` + profileAuthorityV2ProcedureRef + `'
		AND method.bounded_context_ref = '` + profileAuthorityV2ContextRef + `'
		AND method.procedure_semantics = '` + profileAuthorityV2ProcedureText + `'
		AND json_array_length(act.parameters_json) = 1
		AND json_extract(act.parameters_json, '$[0].name') = 'parameter:authorization-content-digest'
		AND json_extract(act.parameters_json, '$[0].value') = content.authorization_content_digest
		AND json_array_length(act.input_refs_json) = 1
		AND json_extract(act.input_refs_json, '$[0]') = content.authorization_content_ref
		AND json_array_length(act.output_refs_json) = 1
		AND json_extract(act.output_refs_json, '$[0]') = NEW.permission_ref
		AND json_array_length(act.resource_refs_json) = 1
		AND json_extract(act.resource_refs_json, '$[0]') = 'resource:controlling-terminal'
		AND json_array_length(act.affected_refs_json) = 2
		AND EXISTS (SELECT 1 FROM json_each(act.affected_refs_json) item WHERE item.value = 'affected:profile-permission:' || NEW.permission_ref)
		AND EXISTS (SELECT 1 FROM json_each(act.affected_refs_json) item WHERE item.value = 'affected:profile-authorization-content:' || content.authorization_content_digest)
	 ) OR EXISTS (
		SELECT 1 FROM json_each(NEW.adjudication_evidence_claim_refs_json) item
		WHERE item.type != 'text' OR item.value NOT GLOB 'E-?*'
	 ) OR EXISTS (
		SELECT 1 FROM json_each(NEW.adjudication_carrier_class_refs_json) item
		WHERE item.type != 'text' OR item.value NOT GLOB 'carrier-class:?*'
	 ) OR json_array_length(NEW.adjudication_evidence_claim_refs_json) != (
		SELECT COUNT(*) FROM (SELECT 1 FROM json_each(NEW.adjudication_evidence_claim_refs_json) GROUP BY value)
	 ) OR json_array_length(NEW.adjudication_carrier_class_refs_json) != (
		SELECT COUNT(*) FROM (SELECT 1 FROM json_each(NEW.adjudication_carrier_class_refs_json) GROUP BY value)
	 ) BEGIN
		SELECT RAISE(ABORT, 'profile Permission does not bind exact content, typed adjudication, policy, SpeechAct, and capture sources');
	 END`
}

func profileAuthorityEffectExactSourcesTrigger43() string {
	return `CREATE TRIGGER profile_declaration_instituted_effects_v2_exact_sources
	 BEFORE INSERT ON profile_declaration_instituted_effects_v2
	 WHEN NOT EXISTS (
		SELECT 1
		FROM profile_declaration_permissions_v2 permission
		JOIN speech_acts act ON act.speech_act_ref = NEW.speech_act_ref
		JOIN speech_act_context_policies policy ON policy.context_policy_ref = permission.context_policy_ref
		WHERE permission.permission_ref = NEW.permission_ref
		AND permission.permission_digest = NEW.permission_digest
		AND permission.project_root = NEW.project_root
		AND permission.source_speech_act_ref = NEW.speech_act_ref
		AND permission.source_speech_act_digest = NEW.speech_act_digest
		AND act.speech_act_digest = NEW.speech_act_digest
		AND act.project_root = NEW.project_root
		AND act.instituted_object_ref = NEW.permission_ref
		AND act.context_policy_ref = permission.context_policy_ref
		AND act.context_policy_digest = permission.context_policy_digest
		AND policy.context_policy_digest = permission.context_policy_digest
		AND policy.context_policy_ref = '` + profileAuthorityV2PolicyRef + `'
		AND policy.institutional_effect_rule_ref = '` + profileAuthorityV2EffectRuleRef + `'
		AND policy.instituted_object_kind = '` + profileAuthorityV2PermissionKind + `'
		AND policy.institutional_modality = 'MAY'
		AND policy.scoped_action = '` + profileAuthorityV2Action + `'
	 ) BEGIN
		SELECT RAISE(ABORT, 'profile instituted effect does not bind the exact profile SpeechAct and MAY Permission');
	 END`
}

func profileAuthorityBasisExactSourcesTrigger43() string {
	return `CREATE TRIGGER profile_declaration_authority_bases_v2_exact_sources
	 BEFORE INSERT ON profile_declaration_authority_bases_v2
	 WHEN NOT EXISTS (
		SELECT 1
		FROM profile_declaration_authorization_preparations_v2 prepared
		JOIN profile_declaration_authorization_contents_v2 content
			ON content.authorization_content_ref = NEW.authorization_content_ref
		JOIN profile_declaration_permissions_v2 permission
			ON permission.permission_ref = NEW.permission_ref
		JOIN speech_acts act ON act.speech_act_ref = NEW.speech_act_ref
		JOIN speech_act_context_policies policy ON policy.context_policy_ref = NEW.context_policy_ref
		JOIN profile_declaration_instituted_effects_v2 effect
			ON effect.effect_digest = NEW.instituted_effect_digest
		WHERE prepared.basis_ref = NEW.basis_ref
		AND prepared.authorization_content_ref = NEW.authorization_content_ref
		AND prepared.authorization_content_digest = NEW.authorization_content_digest
		AND prepared.permission_ref = NEW.permission_ref
		AND prepared.speech_act_ref = NEW.speech_act_ref
		AND prepared.context_policy_ref = NEW.context_policy_ref
		AND prepared.context_policy_digest = NEW.context_policy_digest
		AND prepared.project_root = NEW.project_root
		AND content.authorization_content_digest = NEW.authorization_content_digest
		AND content.project_root = NEW.project_root
		AND permission.permission_digest = NEW.permission_digest
		AND permission.project_root = NEW.project_root
		AND permission.prepared_authorization_digest = prepared.prepared_authorization_digest
		AND permission.authorization_content_ref = content.authorization_content_ref
		AND permission.authorization_content_digest = content.authorization_content_digest
		AND permission.source_speech_act_ref = NEW.speech_act_ref
		AND permission.source_speech_act_digest = NEW.speech_act_digest
		AND permission.context_policy_ref = NEW.context_policy_ref
		AND permission.context_policy_digest = NEW.context_policy_digest
		AND act.speech_act_digest = NEW.speech_act_digest
		AND act.project_root = NEW.project_root
		AND policy.context_policy_digest = NEW.context_policy_digest
		AND policy.context_policy_ref = '` + profileAuthorityV2PolicyRef + `'
		AND effect.project_root = NEW.project_root
		AND effect.speech_act_ref = NEW.speech_act_ref
		AND effect.speech_act_digest = NEW.speech_act_digest
		AND effect.permission_ref = NEW.permission_ref
		AND effect.permission_digest = NEW.permission_digest
	 ) BEGIN
		SELECT RAISE(ABORT, 'profile four-ref basis does not join exact SpeechAct, content, Permission, policy, and matching effect');
	 END`
}

func profileAuthorityContentLegacyCollisionTrigger43() string {
	return `CREATE TRIGGER profile_declaration_authorization_contents_v2_no_legacy_collision
	 BEFORE INSERT ON profile_declaration_authorization_contents_v2
	 WHEN EXISTS (
		SELECT 1 FROM profile_declaration_authorization_contents legacy
		WHERE legacy.authorization_content_ref = NEW.authorization_content_ref
		OR legacy.authorization_content_digest = NEW.authorization_content_digest
		OR legacy.single_use_key = NEW.single_use_key
	 ) OR EXISTS (
		SELECT 1 FROM migration_review_acceptance_contents review
		WHERE review.review_content_ref = NEW.authorization_content_ref
		OR review.review_content_digest = NEW.authorization_content_digest
	 ) OR EXISTS (
		SELECT 1 FROM decision_binding_contents decision
		WHERE decision.decision_content_ref = NEW.authorization_content_ref
		OR decision.decision_content_digest = NEW.authorization_content_digest
	 ) BEGIN
		SELECT RAISE(ABORT, 'profile authority content collides with an existing authority-domain identity');
	 END`
}

func profileAuthorityPreparationLegacyCollisionTrigger43() string {
	return `CREATE TRIGGER profile_declaration_authorization_preparations_v2_no_consumed_identity
	 BEFORE INSERT ON profile_declaration_authorization_preparations_v2
	 WHEN EXISTS (SELECT 1 FROM speech_acts act WHERE act.speech_act_ref = NEW.speech_act_ref)
	 OR EXISTS (SELECT 1 FROM terminal_capture_records capture WHERE capture.capture_carrier_ref = NEW.capture_carrier_ref OR capture.prepared_speech_act_intent_digest = NEW.speech_act_intent_digest)
	 OR EXISTS (SELECT 1 FROM profile_declaration_permissions permission WHERE permission.permission_ref = NEW.permission_ref)
	 OR EXISTS (SELECT 1 FROM authority_basis_presentations basis WHERE basis.presentation_id = NEW.basis_ref)
	 BEGIN
		SELECT RAISE(ABORT, 'prepared profile authorization reuses an already consumed authority identity');
	 END`
}

func profileAuthorityPermissionLegacyCollisionTrigger43() string {
	return `CREATE TRIGGER profile_declaration_permissions_v2_no_legacy_collision
	 BEFORE INSERT ON profile_declaration_permissions_v2
	 WHEN EXISTS (
		SELECT 1 FROM profile_declaration_permissions legacy
		WHERE legacy.permission_ref = NEW.permission_ref
		OR legacy.permission_digest = NEW.permission_digest
		OR legacy.source_speech_act_ref = NEW.source_speech_act_ref
		OR legacy.capture_carrier_ref = NEW.capture_carrier_ref
	 ) OR EXISTS (
		SELECT 1 FROM migration_review_admissions_v2 review WHERE review.speech_act_ref = NEW.source_speech_act_ref OR review.capture_carrier_ref = NEW.capture_carrier_ref
	 ) OR EXISTS (
		SELECT 1 FROM decision_record_instituted_effects decision WHERE decision.speech_act_ref = NEW.source_speech_act_ref
	 ) BEGIN
		SELECT RAISE(ABORT, 'profile Permission collides with an existing authority-domain use');
	 END`
}

func profileAuthorityEffectLegacyCollisionTrigger43() string {
	return `CREATE TRIGGER profile_declaration_instituted_effects_v2_no_legacy_collision
	 BEFORE INSERT ON profile_declaration_instituted_effects_v2
	 WHEN EXISTS (
		SELECT 1 FROM speech_act_instituted_effects legacy
		WHERE legacy.instituted_effect_digest = NEW.effect_digest
		OR legacy.speech_act_ref = NEW.speech_act_ref
		OR legacy.permission_ref = NEW.permission_ref
	 ) OR EXISTS (
		SELECT 1 FROM migration_review_instituted_effects review
		WHERE review.effect_digest = NEW.effect_digest OR review.speech_act_ref = NEW.speech_act_ref
	 ) OR EXISTS (
		SELECT 1 FROM decision_record_instituted_effects decision
		WHERE decision.effect_digest = NEW.effect_digest OR decision.speech_act_ref = NEW.speech_act_ref
	 ) BEGIN
		SELECT RAISE(ABORT, 'profile instituted effect collides with an existing authority-domain effect');
	 END`
}

func profileAuthorityBasisLegacyCollisionTrigger43() string {
	return `CREATE TRIGGER profile_declaration_authority_bases_v2_no_legacy_collision
	 BEFORE INSERT ON profile_declaration_authority_bases_v2
	 WHEN EXISTS (
		SELECT 1 FROM authority_basis_presentations legacy
		WHERE legacy.presentation_id = NEW.basis_ref OR legacy.presentation_digest = NEW.basis_digest
	 ) OR EXISTS (
		SELECT 1 FROM authority_basis_resolutions legacy
		WHERE legacy.authority_resolution_id = NEW.basis_ref OR legacy.authority_resolution_digest = NEW.basis_digest
	 ) OR EXISTS (
		SELECT 1 FROM authority_presentations legacy
		WHERE legacy.presentation_id = NEW.basis_ref OR legacy.presentation_digest = NEW.basis_digest
	 ) BEGIN
		SELECT RAISE(ABORT, 'profile four-ref basis collides with a legacy presentation or resolution identity');
	 END`
}

func appendProfileAuthorityV2RootGuards43(statements []string, index int) []string {
	if index >= len(profileAuthorityV2Tables) {
		return statements
	}
	table := profileAuthorityV2Tables[index]
	trigger := "CREATE TRIGGER " + table + "_project_ledger_root " +
		"BEFORE INSERT ON " + table + " WHEN EXISTS (SELECT 1 FROM project_ledger_binding) " +
		"AND NOT EXISTS (SELECT 1 FROM project_ledger_binding binding WHERE binding.project_root = NEW.project_root) " +
		"BEGIN SELECT RAISE(ABORT, '" + table + " does not match the bound project ledger root'); END"
	next := slices.Clone(statements)
	next = append(next, trigger)
	return appendProfileAuthorityV2RootGuards43(next, index+1)
}
