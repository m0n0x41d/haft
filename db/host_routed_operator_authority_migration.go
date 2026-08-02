package db

import (
	"fmt"
	"strings"
)

const (
	hostRoutedAuthorityMode56 = "host_routed_operator_request"
	hostRoutedProfileAction56 = "profile.declare.from_onboarding_candidate"
	hostRoutedResolution56    = "host_routed_request_acceptance"
	hostRoutedProfileOrigin56 = "host_routed_operator_request"
)

var hostRoutedOperatorAuthorityMigration56 = Migration{
	Version:            56,
	Description:        "Replace product-local strict authority modes with host-routed operator requests",
	Apply:              applyHostRoutedOperatorAuthorityMigration56,
	ApplyBoundary:      ForeignKeyTableRebuildBoundary,
	ForeignKeyVerifier: verifyForeignKeys,
}

var hostRoutedProfileTables56 = []string{
	"profile_declaration_authority_bases_v5",
	"profile_declaration_authority_resolutions_v5",
	"project_profile_admissions_v5",
	"profile_declaration_authority_uses_v5",
	"project_profile_revisions_v5",
	"project_profile_projection_debt_v5",
}

func applyHostRoutedOperatorAuthorityMigration56(
	tx MigrationTransaction,
	_ []Migration,
) error {
	statements := []string{
		hostRoutedProfileAuthorityBasisTableSQL56(),
		hostRoutedProfileAuthorityResolutionTableSQL56(),
		hostRoutedProfileAdmissionTableSQL56(),
		hostRoutedProfileAuthorityUseTableSQL56(),
		hostRoutedProfileRevisionTableSQL56(),
		hostRoutedProfileDebtTableSQL56(),
		hostRoutedProfileBasisExactSourcesTrigger56(),
		hostRoutedProfileResolutionExactSourcesTrigger56(),
		hostRoutedProfileAdmissionExactSourcesTrigger56(),
		hostRoutedProfileAuthorityUseExactSourcesTrigger56(),
		hostRoutedProfileRevisionExactAdmissionTrigger56(),
		"DROP TRIGGER project_profile_admissions_v3_revision_cas",
		"DROP TRIGGER project_profile_admissions_v4_revision_cas",
		hostRoutedProfileRevisionCASTrigger56("project_profile_admissions_v3_revision_cas", "project_profile_admissions_v3"),
		hostRoutedProfileRevisionCASTrigger56("project_profile_admissions_v4_revision_cas", "project_profile_admissions_v4"),
		hostRoutedProfileRevisionCASTrigger56("project_profile_admissions_v5_revision_cas", "project_profile_admissions_v5"),
		"DROP VIEW current_project_profiles",
		currentProjectProfilesView56(),
	}
	statements = append(statements, hostRoutedTypeEnvAuthorityRebuildStatements56()...)
	for _, table := range hostRoutedProfileTables56 {
		statements = append(
			statements,
			hostRoutedRootGuard56(table),
			"CREATE TRIGGER "+table+"_no_update BEFORE UPDATE ON "+table+" BEGIN SELECT RAISE(ABORT, '"+table+" is append-only'); END",
			"CREATE TRIGGER "+table+"_no_delete BEFORE DELETE ON "+table+" BEGIN SELECT RAISE(ABORT, '"+table+" is append-only'); END",
		)
	}
	for _, table := range []string{
		"profile_declaration_authority_bases_v3",
		"profile_declaration_authority_resolutions_v3",
		"profile_declaration_authority_uses_v3",
		"project_typeenv_head_selection_config_authority_bases",
		"project_typeenv_head_selection_mode_policies",
		"project_typeenv_head_selection_trusted_cli_sources",
		"project_typeenv_head_selection_speech_act_records",
		"project_typeenv_head_selection_permissions_v3",
		"project_typeenv_head_selection_authority_resolution_bases",
		"project_typeenv_head_selection_explicit_policy_acceptance_resolutions",
		"project_typeenv_head_selection_strict_permission_resolutions",
		"project_typeenv_head_selection_authority_resolutions_legacy_v47",
		"project_typeenv_head_selection_authority_uses_legacy_v47",
	} {
		statements = append(statements, sealHistoricalAuthorityTable56(table))
	}
	for _, table := range []string{
		"project_typeenv_head_selection_host_requests_v1",
		"project_typeenv_head_selection_host_resolutions_v1",
		"project_typeenv_head_selection_host_uses_v1",
		"project_typeenv_head_selection_authority_resolutions",
		"project_typeenv_head_selection_authority_uses",
		"project_typeenv_head_selection_authority_resolutions_legacy_v47",
		"project_typeenv_head_selection_authority_uses_legacy_v47",
	} {
		statements = append(
			statements,
			"CREATE TRIGGER "+table+"_no_update BEFORE UPDATE ON "+table+" BEGIN SELECT RAISE(ABORT, '"+table+" is append-only'); END",
			"CREATE TRIGGER "+table+"_no_delete BEFORE DELETE ON "+table+" BEGIN SELECT RAISE(ABORT, '"+table+" is append-only'); END",
		)
	}
	if err := executeStatements(tx, statements, 0); err != nil {
		return fmt.Errorf("install host-routed operator authority generation: %w", err)
	}
	return verifyForeignKeys(tx)
}

func hostRoutedProfileAuthorityBasisTableSQL56() string {
	return `CREATE TABLE profile_declaration_authority_bases_v5 (
		basis_ref TEXT PRIMARY KEY CHECK(basis_ref GLOB 'profile-authority-basis:?*'),
		basis_digest TEXT NOT NULL UNIQUE CHECK(length(basis_digest) = 71 AND substr(basis_digest, 1, 7) = 'sha256:'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		action_kind TEXT NOT NULL CHECK(action_kind = '` + hostRoutedProfileAction56 + `'),
		authority_mode TEXT NOT NULL CHECK(authority_mode = '` + hostRoutedAuthorityMode56 + `'),
		profile_origin TEXT NOT NULL CHECK(profile_origin = '` + hostRoutedProfileOrigin56 + `'),
		operator_request_ref TEXT NOT NULL UNIQUE CHECK(operator_request_ref GLOB 'operator-request:sha256:*'),
		operator_request_digest TEXT NOT NULL UNIQUE CHECK(length(operator_request_digest) = 71 AND substr(operator_request_digest, 1, 7) = 'sha256:'),
		operator_request_subject_ref TEXT NOT NULL CHECK(operator_request_subject_ref != ''),
		operator_request_payload_digest TEXT NOT NULL CHECK(length(operator_request_payload_digest) = 71 AND substr(operator_request_payload_digest, 1, 7) = 'sha256:'),
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
		suggestion_ref TEXT NOT NULL CHECK(suggestion_ref != ''),
		observation_digest TEXT NOT NULL CHECK(length(observation_digest) = 71 AND substr(observation_digest, 1, 7) = 'sha256:'),
		future_work_session_ref TEXT NOT NULL CHECK(future_work_session_ref != ''),
		allowed_work_from TEXT NOT NULL,
		allowed_work_until TEXT NOT NULL,
		basis_observation_from TEXT NOT NULL,
		basis_observation_until TEXT NOT NULL,
		single_use_key TEXT NOT NULL UNIQUE CHECK(single_use_key != ''),
		canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
		recorded_at TEXT NOT NULL,
		CHECK(COALESCE(json_extract(canonical_json, '$.schema') = 'haft.profile-authority.host-routed-basis/v1', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.authority_mode') = authority_mode, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.operator_request_ref') = operator_request_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.operator_request_digest') = operator_request_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.operator_request_payload_digest') = operator_request_payload_digest, 0)),
		CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("allowed_work_from") + `),
		CHECK(` + sqliteCanonicalUTCNanoShape("allowed_work_until") + `),
		CHECK(` + sqliteUTCNanoLess("allowed_work_from", "allowed_work_until") + `)
	) WITHOUT ROWID`
}

func hostRoutedProfileAuthorityResolutionTableSQL56() string {
	return `CREATE TABLE profile_declaration_authority_resolutions_v5 (
		authority_resolution_ref TEXT PRIMARY KEY CHECK(authority_resolution_ref GLOB 'profile-authority-resolution:?*'),
		authority_resolution_digest TEXT NOT NULL UNIQUE CHECK(length(authority_resolution_digest) = 71 AND substr(authority_resolution_digest, 1, 7) = 'sha256:'),
		authority_basis_ref TEXT NOT NULL UNIQUE REFERENCES profile_declaration_authority_bases_v5(basis_ref),
		authority_basis_digest TEXT NOT NULL CHECK(length(authority_basis_digest) = 71 AND substr(authority_basis_digest, 1, 7) = 'sha256:'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		action_kind TEXT NOT NULL CHECK(action_kind = '` + hostRoutedProfileAction56 + `'),
		authority_mode TEXT NOT NULL CHECK(authority_mode = '` + hostRoutedAuthorityMode56 + `'),
		resolution_kind TEXT NOT NULL CHECK(resolution_kind = '` + hostRoutedResolution56 + `'),
		profile_origin TEXT NOT NULL CHECK(profile_origin = '` + hostRoutedProfileOrigin56 + `'),
		operator_request_ref TEXT NOT NULL UNIQUE,
		operator_request_digest TEXT NOT NULL UNIQUE CHECK(length(operator_request_digest) = 71 AND substr(operator_request_digest, 1, 7) = 'sha256:'),
		operator_request_subject_ref TEXT NOT NULL CHECK(operator_request_subject_ref != ''),
		operator_request_payload_digest TEXT NOT NULL CHECK(length(operator_request_payload_digest) = 71 AND substr(operator_request_payload_digest, 1, 7) = 'sha256:'),
		work_input_ref TEXT NOT NULL UNIQUE REFERENCES profile_onboarding_work_inputs_v1(work_input_ref),
		work_input_digest TEXT NOT NULL CHECK(length(work_input_digest) = 71 AND substr(work_input_digest, 1, 7) = 'sha256:'),
		project_binding_digest TEXT NOT NULL CHECK(length(project_binding_digest) = 71 AND substr(project_binding_digest, 1, 7) = 'sha256:'),
		detector_version TEXT NOT NULL CHECK(detector_version != ''),
		detector_policy_version TEXT NOT NULL CHECK(detector_policy_version != ''),
		suggestion_ref TEXT NOT NULL CHECK(suggestion_ref != ''),
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
		recorded_at TEXT NOT NULL CHECK(recorded_at = checked_at),
		CHECK(COALESCE(json_extract(canonical_json, '$.schema') = 'haft.profile-authority.host-routed-resolution/v1', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.operator_request_ref') = operator_request_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.operator_request_digest') = operator_request_digest, 0)),
		CHECK(` + sqliteCanonicalUTCNanoShape("checked_at") + `)
	) WITHOUT ROWID`
}

func hostRoutedProfileAdmissionTableSQL56() string {
	value := projectProfileAdmissionTableSQL55()
	return applyHostRoutedProfileReplacements56(value)
}

func hostRoutedProfileAuthorityUseTableSQL56() string {
	value := profileAutomaticAuthorityUseTableSQL55()
	return applyHostRoutedProfileReplacements56(value)
}

func hostRoutedProfileRevisionTableSQL56() string {
	value := projectProfileRevisionTableSQL55()
	return applyHostRoutedProfileReplacements56(value)
}

func hostRoutedProfileDebtTableSQL56() string {
	value := projectProfileDebtTableSQL55()
	value = applyHostRoutedProfileReplacements56(value)
	value = strings.ReplaceAll(value, "profile_revision_generation = 'v4'", "profile_revision_generation = 'v5'")
	return strings.ReplaceAll(
		value,
		"IN ('v1', 'v2', 'v3', 'v4')",
		"IN ('v1', 'v2', 'v3', 'v4', 'v5')",
	)
}

func applyHostRoutedProfileReplacements56(value string) string {
	replacer := strings.NewReplacer(
		"project_profile_admissions_v4", "project_profile_admissions_v5",
		"profile_declaration_authority_uses_v4", "profile_declaration_authority_uses_v5",
		"project_profile_revisions_v4", "project_profile_revisions_v5",
		"project_profile_projection_debt_v4", "project_profile_projection_debt_v5",
		"profile_initial_bootstrap_authority_bases_v1", "profile_declaration_authority_bases_v5",
		"profile_initial_bootstrap_authority_resolutions_v1", "profile_declaration_authority_resolutions_v5",
		profileAutomaticAction55, hostRoutedProfileAction56,
		profileAutomaticMode55, hostRoutedAuthorityMode56,
		profileAutomaticResolution55, hostRoutedResolution56,
		profileDetectorDefault55, hostRoutedProfileOrigin56,
	)
	return replacer.Replace(value)
}

func hostRoutedProfileBasisExactSourcesTrigger56() string {
	return `CREATE TRIGGER profile_declaration_authority_bases_v5_exact_sources
	 BEFORE INSERT ON profile_declaration_authority_bases_v5
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
	 ) BEGIN SELECT RAISE(ABORT, 'host-routed profile basis does not bind exact WorkInput and Haft Core support'); END`
}

func hostRoutedProfileResolutionExactSourcesTrigger56() string {
	return `CREATE TRIGGER profile_declaration_authority_resolutions_v5_exact_sources
	 BEFORE INSERT ON profile_declaration_authority_resolutions_v5
	 WHEN NOT EXISTS (
		SELECT 1 FROM profile_declaration_authority_bases_v5 basis
		JOIN project_ledger_binding binding ON binding.project_root = basis.project_root
		WHERE basis.basis_ref = NEW.authority_basis_ref
		AND basis.basis_digest = NEW.authority_basis_digest
		AND basis.project_root = NEW.project_root
		AND basis.operator_request_ref = NEW.operator_request_ref
		AND basis.operator_request_digest = NEW.operator_request_digest
		AND basis.operator_request_subject_ref = NEW.operator_request_subject_ref
		AND basis.operator_request_payload_digest = NEW.operator_request_payload_digest
		AND basis.work_input_ref = NEW.work_input_ref
		AND basis.work_input_digest = NEW.work_input_digest
		AND binding.binding_digest = NEW.project_binding_digest
	 ) BEGIN SELECT RAISE(ABORT, 'host-routed profile resolution does not bind exact request, basis, and project'); END`
}

func hostRoutedProfileAdmissionExactSourcesTrigger56() string {
	value := projectProfileAdmissionExactSourcesTrigger55()
	value = applyHostRoutedProfileReplacements56(value)
	value = strings.Replace(value, "CREATE TRIGGER project_profile_admissions_v5_exact_sources", "CREATE TRIGGER project_profile_admissions_v5_exact_sources", 1)
	return value
}

func hostRoutedProfileAuthorityUseExactSourcesTrigger56() string {
	return applyHostRoutedProfileReplacements56(profileAutomaticAuthorityUseExactSourcesTrigger55())
}

func hostRoutedProfileRevisionExactAdmissionTrigger56() string {
	return applyHostRoutedProfileReplacements56(projectProfileRevisionExactAdmissionTrigger55())
}

func hostRoutedProfileRevisionCASTrigger56(trigger string, table string) string {
	return `CREATE TRIGGER ` + trigger + ` BEFORE INSERT ON ` + table + `
	 WHEN NOT EXISTS (
		SELECT 1 FROM (
			SELECT COUNT(*) AS revision_count, COUNT(DISTINCT ledger_revision) AS distinct_count,
				COALESCE(MIN(ledger_revision), 0) AS minimum_revision,
				COALESCE(MAX(ledger_revision), 0) AS maximum_revision
			FROM (
				SELECT ledger_revision FROM project_profile_revisions legacy WHERE legacy.project_root = NEW.project_root
				UNION ALL SELECT ledger_revision FROM project_profile_revisions_v2 previous WHERE previous.project_root = NEW.project_root
				UNION ALL SELECT ledger_revision FROM project_profile_revisions_v3 historical WHERE historical.project_root = NEW.project_root
				UNION ALL SELECT ledger_revision FROM project_profile_revisions_v4 automatic WHERE automatic.project_root = NEW.project_root
				UNION ALL SELECT ledger_revision FROM project_profile_revisions_v5 current WHERE current.project_root = NEW.project_root
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

func currentProjectProfilesView56() string {
	value := currentProjectProfilesView55()
	needle := "\t), valid_roots AS ("
	branch := `
		UNION ALL
		SELECT 'v5', project_root, ledger_revision, configured_profile_kind,
			profile_payload_json, profile_payload_digest,
			receipt_json, receipt_digest, admission_id, admission_digest, recorded_at,
			profile_origin FROM project_profile_revisions_v5`
	return strings.Replace(value, needle, branch+needle, 1)
}

func hostRoutedRootGuard56(table string) string {
	return "CREATE TRIGGER " + table + "_project_ledger_root BEFORE INSERT ON " + table +
		" WHEN NOT EXISTS (SELECT 1 FROM project_ledger_binding binding WHERE binding.project_root = NEW.project_root)" +
		" BEGIN SELECT RAISE(ABORT, '" + table + " does not match the bound project ledger root'); END"
}

func sealHistoricalAuthorityTable56(table string) string {
	return "CREATE TRIGGER " + table + "_sealed_after_56 BEFORE INSERT ON " + table +
		" BEGIN SELECT RAISE(ABORT, '" + table + " is sealed historical authority; use the host-routed generation'); END"
}

func hostRoutedTypeEnvAuthorityRebuildStatements56() []string {
	legacyResolutionTable := strings.Replace(
		projectTypeEnvAuthorityResolutionsTable47(),
		"CREATE TABLE project_typeenv_head_selection_authority_resolutions",
		"CREATE TABLE project_typeenv_head_selection_authority_resolutions_legacy_v47",
		1,
	)
	legacyUseTable := strings.Replace(
		projectTypeEnvAuthorityUsesTable47(),
		"CREATE TABLE project_typeenv_head_selection_authority_uses",
		"CREATE TABLE project_typeenv_head_selection_authority_uses_legacy_v47",
		1,
	)
	return []string{
		hostRoutedTypeEnvRequestTableSQL56(),
		hostRoutedTypeEnvResolutionTableSQL56(),
		legacyResolutionTable,
		`INSERT INTO project_typeenv_head_selection_authority_resolutions_legacy_v47
		 SELECT * FROM project_typeenv_head_selection_authority_resolutions`,
		`DROP TABLE project_typeenv_head_selection_authority_resolutions`,
		hostRoutedTypeEnvAuthorityResolutionCatalogSQL56(),
		`INSERT INTO project_typeenv_head_selection_authority_resolutions (
			authority_resolution_ref, authority_resolution_digest,
			authority_generation, project_id, authority_resolution_kind,
			content_ref, content_digest, request_ref, request_digest,
			trusted_cli_source_ref, trusted_cli_source_digest,
			strict_basis_ref, strict_basis_digest,
			explicit_resolution_ref, explicit_resolution_digest,
			strict_resolution_ref, strict_resolution_digest,
			host_resolution_ref, host_resolution_digest,
			evaluated_at, canonical_bytes, recorded_at
		) SELECT authority_resolution_ref, authority_resolution_digest,
			'legacy_unreproducible', project_id, authority_resolution_kind,
			content_ref, content_digest, request_ref, request_digest,
			trusted_cli_source_ref, trusted_cli_source_digest,
			strict_basis_ref, strict_basis_digest,
			explicit_resolution_ref, explicit_resolution_digest,
			strict_resolution_ref, strict_resolution_digest,
			NULL, NULL, evaluated_at, canonical_bytes, recorded_at
		 FROM project_typeenv_head_selection_authority_resolutions_legacy_v47`,
		legacyUseTable,
		`INSERT INTO project_typeenv_head_selection_authority_uses_legacy_v47
		 SELECT * FROM project_typeenv_head_selection_authority_uses`,
		`DROP TABLE project_typeenv_head_selection_authority_uses`,
		hostRoutedTypeEnvAuthorityUseCatalogSQL56(),
		`INSERT INTO project_typeenv_head_selection_authority_uses (
			authority_use_ref, authority_use_digest, authority_generation,
			project_id, original_idempotency_key, authority_resolution_kind,
			authority_resolution_ref, authority_resolution_digest,
			content_ref, content_digest, request_ref, request_digest,
			work_ref, receipt_ref, predecessor_kind, predecessor_head_ref,
			predecessor_head_revision, predecessor_selected_composite_ref,
			base_type_env_ref, ordered_extension_refs_digest,
			canonical_ordered_extension_refs, runtime_evaluation_basis_ref,
			selected_composite_ref, stage_ref, stage_digest,
			expected_graph_revision, committed_head_revision,
			committed_graph_revision, verifier_ref, verifier_edition,
			canonical_bytes, recorded_at
		) SELECT authority_use_ref, authority_use_digest,
			'legacy_unreproducible', project_id, original_idempotency_key,
			authority_resolution_kind, authority_resolution_ref,
			authority_resolution_digest, content_ref, content_digest,
			request_ref, request_digest, work_ref, receipt_ref,
			predecessor_kind, predecessor_head_ref, predecessor_head_revision,
			predecessor_selected_composite_ref, base_type_env_ref,
			ordered_extension_refs_digest, canonical_ordered_extension_refs,
			runtime_evaluation_basis_ref, selected_composite_ref, stage_ref,
			stage_digest, expected_graph_revision, committed_head_revision,
			committed_graph_revision, verifier_ref, verifier_edition,
			canonical_bytes, recorded_at
		 FROM project_typeenv_head_selection_authority_uses_legacy_v47`,
		hostRoutedTypeEnvUseTableSQL56(),
		hostRoutedTypeEnvRequestExactProjectTrigger56(),
		hostRoutedTypeEnvResolutionExactRequestTrigger56(),
		hostRoutedTypeEnvCatalogResolutionExactHostTrigger56(),
		hostRoutedTypeEnvCatalogUseExactHostTrigger56(),
		hostRoutedTypeEnvUseExactCatalogTrigger56(),
		currentGenerationOnlyTrigger56(
			"project_typeenv_head_selection_authority_resolutions",
		),
		currentGenerationOnlyTrigger56(
			"project_typeenv_head_selection_authority_uses",
		),
	}
}

func hostRoutedTypeEnvRequestTableSQL56() string {
	return `CREATE TABLE project_typeenv_head_selection_host_requests_v1 (
		request_ref TEXT PRIMARY KEY CHECK(request_ref GLOB 'operator-request:sha256:*'),
		request_digest TEXT NOT NULL UNIQUE CHECK(length(request_digest) = 71 AND substr(request_digest, 1, 7) = 'sha256:'),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		effect_kind TEXT NOT NULL CHECK(effect_kind = 'project_typeenv_head.select'),
		subject_ref TEXT NOT NULL CHECK(subject_ref != ''),
		payload_digest TEXT NOT NULL CHECK(length(payload_digest) = 71 AND substr(payload_digest, 1, 7) = 'sha256:'),
		provenance TEXT NOT NULL CHECK(provenance = '` + hostRoutedAuthorityMode56 + `'),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		UNIQUE(request_ref, request_digest)
	) WITHOUT ROWID`
}

func hostRoutedTypeEnvResolutionTableSQL56() string {
	return `CREATE TABLE project_typeenv_head_selection_host_resolutions_v1 (
		resolution_ref TEXT PRIMARY KEY CHECK(resolution_ref != ''),
		resolution_digest TEXT NOT NULL UNIQUE CHECK(length(resolution_digest) = 71 AND substr(resolution_digest, 1, 7) = 'sha256:'),
		request_ref TEXT NOT NULL UNIQUE,
		request_digest TEXT NOT NULL CHECK(length(request_digest) = 71 AND substr(request_digest, 1, 7) = 'sha256:'),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		project_binding_digest TEXT NOT NULL CHECK(length(project_binding_digest) = 71 AND substr(project_binding_digest, 1, 7) = 'sha256:'),
		selection_request_ref TEXT NOT NULL UNIQUE,
		selection_request_digest TEXT NOT NULL CHECK(length(selection_request_digest) = 71 AND substr(selection_request_digest, 1, 7) = 'sha256:'),
		content_ref TEXT NOT NULL UNIQUE,
		content_digest TEXT NOT NULL CHECK(length(content_digest) = 71 AND substr(content_digest, 1, 7) = 'sha256:'),
		resolution_kind TEXT NOT NULL CHECK(resolution_kind = '` + hostRoutedResolution56 + `'),
		canonical_bytes BLOB NOT NULL UNIQUE,
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		UNIQUE(resolution_ref, resolution_digest),
		FOREIGN KEY(request_ref, request_digest)
			REFERENCES project_typeenv_head_selection_host_requests_v1(request_ref, request_digest),
		FOREIGN KEY(selection_request_ref, selection_request_digest)
			REFERENCES project_typeenv_head_selection_requests(request_ref, request_digest),
		FOREIGN KEY(content_ref, content_digest)
			REFERENCES project_typeenv_head_selection_authorization_contents(content_ref, content_digest)
	) WITHOUT ROWID`
}

func hostRoutedTypeEnvUseTableSQL56() string {
	return `CREATE TABLE project_typeenv_head_selection_host_uses_v1 (
		use_ref TEXT PRIMARY KEY CHECK(use_ref != ''),
		use_digest TEXT NOT NULL UNIQUE CHECK(length(use_digest) = 71 AND substr(use_digest, 1, 7) = 'sha256:'),
		resolution_ref TEXT NOT NULL UNIQUE,
		resolution_digest TEXT NOT NULL CHECK(length(resolution_digest) = 71 AND substr(resolution_digest, 1, 7) = 'sha256:'),
		request_ref TEXT NOT NULL UNIQUE,
		request_digest TEXT NOT NULL CHECK(length(request_digest) = 71 AND substr(request_digest, 1, 7) = 'sha256:'),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		selected_composite_ref TEXT NOT NULL CHECK(selected_composite_ref != ''),
		head_revision INTEGER NOT NULL CHECK(head_revision > 0),
		canonical_bytes BLOB NOT NULL UNIQUE,
		consumed_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("consumed_at") + `),
		UNIQUE(use_ref, use_digest),
		FOREIGN KEY(use_ref, use_digest)
			REFERENCES project_typeenv_head_selection_authority_uses(authority_use_ref, authority_use_digest),
		FOREIGN KEY(resolution_ref, resolution_digest)
			REFERENCES project_typeenv_head_selection_host_resolutions_v1(resolution_ref, resolution_digest),
		FOREIGN KEY(request_ref, request_digest)
			REFERENCES project_typeenv_head_selection_host_requests_v1(request_ref, request_digest)
	) WITHOUT ROWID`
}

func hostRoutedTypeEnvAuthorityResolutionCatalogSQL56() string {
	return `CREATE TABLE project_typeenv_head_selection_authority_resolutions (
		authority_resolution_ref TEXT PRIMARY KEY CHECK(` +
		typedMemoryNonBlankShape46("authority_resolution_ref") + `),
		authority_resolution_digest TEXT NOT NULL UNIQUE CHECK(` +
		typedMemorySHA256Shape46("authority_resolution_digest") + `),
		authority_generation TEXT NOT NULL CHECK(
			authority_generation IN ('legacy_unreproducible', '` + hostRoutedAuthorityMode56 + `')
		),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		authority_resolution_kind TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("authority_resolution_kind") + `),
		content_ref TEXT NOT NULL,
		content_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("content_digest") + `),
		request_ref TEXT NOT NULL,
		request_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("request_digest") + `),
		trusted_cli_source_ref TEXT,
		trusted_cli_source_digest TEXT,
		strict_basis_ref TEXT,
		strict_basis_digest TEXT,
		explicit_resolution_ref TEXT,
		explicit_resolution_digest TEXT,
		strict_resolution_ref TEXT,
		strict_resolution_digest TEXT,
		host_resolution_ref TEXT,
		host_resolution_digest TEXT,
		evaluated_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("evaluated_at") + `),
		canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		UNIQUE(authority_resolution_ref, authority_resolution_digest),
		FOREIGN KEY(content_ref, content_digest)
			REFERENCES project_typeenv_head_selection_authorization_contents(content_ref, content_digest),
		FOREIGN KEY(request_ref, request_digest)
			REFERENCES project_typeenv_head_selection_requests(request_ref, request_digest),
		FOREIGN KEY(trusted_cli_source_ref, trusted_cli_source_digest)
			REFERENCES project_typeenv_head_selection_trusted_cli_sources(trusted_cli_source_ref, trusted_cli_source_digest),
		FOREIGN KEY(strict_basis_ref, strict_basis_digest)
			REFERENCES project_typeenv_head_selection_authority_resolution_bases(basis_ref, basis_digest),
		FOREIGN KEY(explicit_resolution_ref, explicit_resolution_digest)
			REFERENCES project_typeenv_head_selection_explicit_policy_acceptance_resolutions(authority_resolution_ref, authority_resolution_digest)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(strict_resolution_ref, strict_resolution_digest)
			REFERENCES project_typeenv_head_selection_strict_permission_resolutions(authority_resolution_ref, authority_resolution_digest)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(host_resolution_ref, host_resolution_digest)
			REFERENCES project_typeenv_head_selection_host_resolutions_v1(resolution_ref, resolution_digest),
		CHECK(
			(authority_generation = 'legacy_unreproducible'
				AND host_resolution_ref IS NULL
				AND host_resolution_digest IS NULL)
			OR
			(authority_generation = '` + hostRoutedAuthorityMode56 + `'
				AND authority_resolution_kind = '` + hostRoutedResolution56 + `'
				AND trusted_cli_source_ref IS NULL
				AND trusted_cli_source_digest IS NULL
				AND strict_basis_ref IS NULL
				AND strict_basis_digest IS NULL
				AND explicit_resolution_ref IS NULL
				AND explicit_resolution_digest IS NULL
				AND strict_resolution_ref IS NULL
				AND strict_resolution_digest IS NULL
				AND host_resolution_ref = authority_resolution_ref
				AND host_resolution_digest = authority_resolution_digest)
		)
	) WITHOUT ROWID`
}

func hostRoutedTypeEnvAuthorityUseCatalogSQL56() string {
	value := projectTypeEnvAuthorityUsesTable47()
	value = strings.Replace(
		value,
		"\t\tproject_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),",
		"\t\tauthority_generation TEXT NOT NULL CHECK(authority_generation IN ('legacy_unreproducible', '"+hostRoutedAuthorityMode56+"')),\n\t\tproject_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),",
		1,
	)
	oldKind := `authority_resolution_kind TEXT NOT NULL CHECK(
			authority_resolution_kind IN (
				'explicit_policy_acceptance',
				'strict_permission'
			)
		)`
	value = strings.Replace(
		value,
		oldKind,
		"authority_resolution_kind TEXT NOT NULL CHECK("+
			typedMemoryNonBlankShape46("authority_resolution_kind")+")",
		1,
	)
	needle := "\t\tCHECK(\n\t\t\t(predecessor_kind = 'genesis'"
	replacement := "\t\tCHECK((authority_generation = 'legacy_unreproducible') OR " +
		"(authority_generation = '" + hostRoutedAuthorityMode56 + "' AND " +
		"authority_resolution_kind = '" + hostRoutedResolution56 + "')),\n" + needle
	return strings.Replace(value, needle, replacement, 1)
}

func hostRoutedTypeEnvRequestExactProjectTrigger56() string {
	return `CREATE TRIGGER project_typeenv_head_selection_host_requests_v1_exact_project
	 BEFORE INSERT ON project_typeenv_head_selection_host_requests_v1
	 WHEN NOT EXISTS (
		SELECT 1 FROM project_ledger_binding binding
		WHERE binding.project_id = NEW.project_id
		AND binding.project_root = NEW.project_root
	 ) BEGIN SELECT RAISE(ABORT, 'host-routed TypeEnv request does not match the bound project'); END`
}

func hostRoutedTypeEnvResolutionExactRequestTrigger56() string {
	return `CREATE TRIGGER project_typeenv_head_selection_host_resolutions_v1_exact_request
	 BEFORE INSERT ON project_typeenv_head_selection_host_resolutions_v1
	 WHEN NOT EXISTS (
		SELECT 1 FROM project_typeenv_head_selection_host_requests_v1 host_request
		JOIN project_ledger_binding binding
			ON binding.project_id = host_request.project_id
			AND binding.project_root = host_request.project_root
		JOIN project_typeenv_head_selection_requests selection_request
			ON selection_request.request_ref = NEW.selection_request_ref
			AND selection_request.request_digest = NEW.selection_request_digest
		JOIN project_typeenv_head_selection_authorization_contents content
			ON content.content_ref = NEW.content_ref
			AND content.content_digest = NEW.content_digest
		WHERE host_request.request_ref = NEW.request_ref
		AND host_request.request_digest = NEW.request_digest
		AND host_request.project_id = NEW.project_id
		AND host_request.project_root = NEW.project_root
		AND host_request.subject_ref = NEW.selection_request_ref
		AND selection_request.project_id = NEW.project_id
		AND content.project_id = NEW.project_id
		AND content.request_ref = NEW.selection_request_ref
		AND content.request_digest = NEW.selection_request_digest
		AND NEW.recorded_at >= content.valid_from
		AND NEW.recorded_at < content.valid_until
	 ) BEGIN SELECT RAISE(ABORT, 'host-routed TypeEnv resolution lacks its exact request, content, and project binding'); END`
}

func hostRoutedTypeEnvCatalogResolutionExactHostTrigger56() string {
	return `CREATE TRIGGER project_typeenv_head_selection_authority_resolutions_v56_exact_host
	 BEFORE INSERT ON project_typeenv_head_selection_authority_resolutions
	 WHEN NEW.authority_generation = '` + hostRoutedAuthorityMode56 + `'
	 AND NOT EXISTS (
		SELECT 1 FROM project_typeenv_head_selection_host_resolutions_v1 host
		WHERE host.resolution_ref = NEW.authority_resolution_ref
		AND host.resolution_digest = NEW.authority_resolution_digest
		AND host.project_id = NEW.project_id
		AND host.selection_request_ref = NEW.request_ref
		AND host.selection_request_digest = NEW.request_digest
		AND host.content_ref = NEW.content_ref
		AND host.content_digest = NEW.content_digest
		AND host.recorded_at = NEW.evaluated_at
		AND host.canonical_bytes = NEW.canonical_bytes
	 ) BEGIN SELECT RAISE(ABORT, 'current TypeEnv authority resolution lacks its exact host-routed resolution'); END`
}

func hostRoutedTypeEnvCatalogUseExactHostTrigger56() string {
	return `CREATE TRIGGER project_typeenv_head_selection_authority_uses_v56_exact_host
	 BEFORE INSERT ON project_typeenv_head_selection_authority_uses
	 WHEN NEW.authority_generation = '` + hostRoutedAuthorityMode56 + `'
	 AND NOT EXISTS (
		SELECT 1 FROM project_typeenv_head_selection_authority_resolutions resolution
		JOIN project_typeenv_head_selection_requests request
			ON request.request_ref = NEW.request_ref
			AND request.request_digest = NEW.request_digest
		JOIN project_typeenv_head_selection_authorization_contents content
			ON content.content_ref = NEW.content_ref
			AND content.content_digest = NEW.content_digest
		JOIN project_typeenv_stages stage
			ON stage.stage_ref = NEW.stage_ref
			AND stage.stage_digest = NEW.stage_digest
		WHERE resolution.authority_resolution_ref = NEW.authority_resolution_ref
		AND resolution.authority_resolution_digest = NEW.authority_resolution_digest
		AND resolution.authority_generation = NEW.authority_generation
		AND resolution.authority_resolution_kind = NEW.authority_resolution_kind
		AND resolution.project_id = NEW.project_id
		AND resolution.content_ref = NEW.content_ref
		AND resolution.content_digest = NEW.content_digest
		AND resolution.request_ref = NEW.request_ref
		AND resolution.request_digest = NEW.request_digest
		AND request.project_id = NEW.project_id
		AND request.original_idempotency_key = NEW.original_idempotency_key
		AND request.predecessor_kind = NEW.predecessor_kind
		AND request.base_type_env_ref = NEW.base_type_env_ref
		AND request.ordered_extension_refs_digest = NEW.ordered_extension_refs_digest
		AND request.canonical_ordered_extension_refs = NEW.canonical_ordered_extension_refs
		AND request.runtime_evaluation_basis_ref = NEW.runtime_evaluation_basis_ref
		AND request.selected_composite_ref = NEW.selected_composite_ref
		AND request.stage_ref = NEW.stage_ref
		AND request.stage_digest = NEW.stage_digest
		AND request.expected_graph_revision = NEW.expected_graph_revision
		AND content.project_id = NEW.project_id
		AND content.request_ref = NEW.request_ref
		AND content.request_digest = NEW.request_digest
		AND stage.project_id = NEW.project_id
		AND stage.executable_type_env_ref = NEW.selected_composite_ref
		AND (
			(NEW.predecessor_kind = 'genesis'
				AND NEW.predecessor_head_ref IS NULL
				AND NEW.predecessor_head_revision IS NULL
				AND NEW.predecessor_selected_composite_ref IS NULL)
			OR
			(NEW.predecessor_kind = 'transition'
				AND request.prior_head_ref = NEW.predecessor_head_ref
				AND request.prior_head_revision = NEW.predecessor_head_revision
				AND request.prior_selected_composite_ref = NEW.predecessor_selected_composite_ref)
		)
	 ) BEGIN SELECT RAISE(ABORT, 'current TypeEnv authority use lacks its exact host-routed resolution and selection request'); END`
}

func hostRoutedTypeEnvUseExactCatalogTrigger56() string {
	return `CREATE TRIGGER project_typeenv_head_selection_host_uses_v1_exact_catalog
	 BEFORE INSERT ON project_typeenv_head_selection_host_uses_v1
	 WHEN NOT EXISTS (
		SELECT 1 FROM project_typeenv_head_selection_authority_uses authority_use
		JOIN project_typeenv_head_selection_host_resolutions_v1 resolution
			ON resolution.resolution_ref = NEW.resolution_ref
			AND resolution.resolution_digest = NEW.resolution_digest
		JOIN project_typeenv_head_selection_host_requests_v1 request
			ON request.request_ref = NEW.request_ref
			AND request.request_digest = NEW.request_digest
		WHERE authority_use.authority_use_ref = NEW.use_ref
		AND authority_use.authority_use_digest = NEW.use_digest
		AND authority_use.authority_generation = '` + hostRoutedAuthorityMode56 + `'
		AND authority_use.project_id = NEW.project_id
		AND authority_use.authority_resolution_ref = NEW.resolution_ref
		AND authority_use.authority_resolution_digest = NEW.resolution_digest
		AND authority_use.selected_composite_ref = NEW.selected_composite_ref
		AND authority_use.committed_head_revision = NEW.head_revision
		AND authority_use.canonical_bytes = NEW.canonical_bytes
		AND resolution.request_ref = NEW.request_ref
		AND resolution.request_digest = NEW.request_digest
		AND resolution.project_id = NEW.project_id
		AND resolution.project_root = NEW.project_root
		AND request.project_id = NEW.project_id
		AND request.project_root = NEW.project_root
	 ) BEGIN SELECT RAISE(ABORT, 'host-routed TypeEnv authority use lacks its exact committed authority-use record'); END`
}

func currentGenerationOnlyTrigger56(table string) string {
	return `CREATE TRIGGER ` + table + `_v56_current_generation_only
	 BEFORE INSERT ON ` + table + `
	 WHEN NEW.authority_generation != '` + hostRoutedAuthorityMode56 + `'
	 BEGIN SELECT RAISE(ABORT, 'legacy TypeEnv authority is historical and cannot be reproduced'); END`
}
