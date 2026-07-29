package db

import (
	"fmt"
	"slices"
)

var migrationReviewProtocolV2Migration39 = Migration{
	Version:     39,
	Description: "Bind migration-review acceptance to the canonical generic SpeechAct source",
	Apply:       applyMigrationReviewProtocolV2Migration39,
}

var migrationReviewProtocolV2Tables = []string{
	"migration_review_acceptance_contents",
	"migration_review_admissions_v2",
	"migration_review_instituted_effects",
}

func applyMigrationReviewProtocolV2Migration39(
	tx MigrationTransaction,
	_ []Migration,
) error {
	if err := requireMigrationReviewProtocolV2AuthoritySource(tx); err != nil {
		return err
	}
	if err := requireAbsentMigrationReviewProtocolV2Footprint(tx, 0); err != nil {
		return err
	}
	if err := requireEmptyLegacyMigrationReviewTables(tx); err != nil {
		return err
	}
	statements := migrationReviewProtocolV2Statements()
	if err := executeStatements(tx, statements, 0); err != nil {
		return fmt.Errorf("install canonical migration-review protocol v2: %w", err)
	}
	if err := verifyForeignKeys(tx); err != nil {
		return fmt.Errorf("verify canonical migration-review protocol v2: %w", err)
	}
	return nil
}

func requireMigrationReviewProtocolV2AuthoritySource(
	tx MigrationTransaction,
) error {
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = 38",
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect canonical v38 SpeechAct source migration: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("migration-review protocol v2 requires canonical v38 SpeechAct source migration")
	}
	return nil
}

func requireAbsentMigrationReviewProtocolV2Footprint(
	tx MigrationTransaction,
	index int,
) error {
	if index >= len(migrationReviewProtocolV2Tables) {
		return nil
	}
	table := migrationReviewProtocolV2Tables[index]
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect migration-review v2 table %s: %w", table, err)
	}
	if count != 0 {
		return fmt.Errorf(
			"migration-review protocol v2 refused: unversioned table %s already exists; unknown partial schema requires manual review",
			table,
		)
	}
	return requireAbsentMigrationReviewProtocolV2Footprint(tx, index+1)
}

func requireEmptyLegacyMigrationReviewTables(tx MigrationTransaction) error {
	tables := []string{"migration_review_speech_acts", "migration_review_admissions"}
	return requireEmptyMigrationReviewTables(tx, tables, 0)
}

func requireEmptyMigrationReviewTables(
	tx MigrationTransaction,
	tables []string,
	index int,
) error {
	if index >= len(tables) {
		return nil
	}
	table := tables[index]
	count := int64(0)
	query := "SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table)
	if err := tx.QueryRow(query).Scan(&count); err != nil {
		return fmt.Errorf("inspect legacy migration-review table %s: %w", table, err)
	}
	if count != 0 {
		return fmt.Errorf(
			"migration-review protocol v2 refused: legacy table %s contains %d row(s); provisional SpeechAct semantics cannot be upgraded automatically",
			table,
			count,
		)
	}
	return requireEmptyMigrationReviewTables(tx, tables, index+1)
}

func migrationReviewProtocolV2Statements() []string {
	statements := []string{
		`CREATE TABLE migration_review_acceptance_contents (
			review_content_ref TEXT PRIMARY KEY,
			review_content_digest TEXT NOT NULL UNIQUE CHECK(length(review_content_digest) = 71 AND substr(review_content_digest, 1, 7) = 'sha256:'),
			project_root TEXT NOT NULL,
			packet_digest TEXT NOT NULL CHECK(length(packet_digest) = 71 AND substr(packet_digest, 1, 7) = 'sha256:'),
			packet_carrier_digest TEXT NOT NULL CHECK(length(packet_carrier_digest) = 71 AND substr(packet_carrier_digest, 1, 7) = 'sha256:'),
			partition_audit_schema TEXT NOT NULL CHECK(partition_audit_schema = 'haft.spec-migration-v2.packet-partition-audit/v1'),
			partition_audit_status TEXT NOT NULL CHECK(partition_audit_status = 'verified'),
			partition_audit_digest TEXT NOT NULL CHECK(length(partition_audit_digest) = 71 AND substr(partition_audit_digest, 1, 7) = 'sha256:'),
			source_carrier TEXT NOT NULL,
			source_digest TEXT NOT NULL CHECK(length(source_digest) = 71 AND substr(source_digest, 1, 7) = 'sha256:'),
			target_carrier_digests_json TEXT NOT NULL CHECK(json_valid(target_carrier_digests_json)),
			fpf_revision TEXT NOT NULL CHECK(length(fpf_revision) = 40 AND fpf_revision NOT GLOB '*[^0-9a-f]*'),
			semantic_zero_pass_carrier TEXT NOT NULL,
			semantic_zero_pass_digest TEXT NOT NULL CHECK(length(semantic_zero_pass_digest) = 71 AND substr(semantic_zero_pass_digest, 1, 7) = 'sha256:'),
			lifecycle_intent_json TEXT NOT NULL CHECK(json_valid(lifecycle_intent_json)),
			canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
			recorded_at TEXT NOT NULL,
			CHECK(json_extract(canonical_json, '$.schema') = 'haft.spec-migration-v2.review-acceptance-content/v2'),
			CHECK(json_extract(canonical_json, '$.review_content_ref') = review_content_ref),
			CHECK(json_extract(canonical_json, '$.project_root') = project_root),
			CHECK(json_extract(canonical_json, '$.packet_digest') = packet_digest),
			CHECK(json_extract(canonical_json, '$.packet_carrier_digest') = packet_carrier_digest),
			CHECK(json_extract(canonical_json, '$.partition_audit_schema') = partition_audit_schema),
			CHECK(json_extract(canonical_json, '$.partition_audit_status') = partition_audit_status),
			CHECK(json_extract(canonical_json, '$.partition_audit_digest') = partition_audit_digest),
			CHECK(json_extract(canonical_json, '$.source_carrier') = source_carrier),
			CHECK(json_extract(canonical_json, '$.source_digest') = source_digest),
			CHECK(json(json_extract(canonical_json, '$.target_carrier_digests')) = json(target_carrier_digests_json)),
			CHECK(json_extract(canonical_json, '$.fpf_revision') = fpf_revision),
			CHECK(json_extract(canonical_json, '$.semantic_zero_pass_carrier') = semantic_zero_pass_carrier),
			CHECK(json_extract(canonical_json, '$.semantic_zero_pass_digest') = semantic_zero_pass_digest),
			CHECK(json(json_extract(canonical_json, '$.lifecycle_intent')) = json(lifecycle_intent_json))
		) WITHOUT ROWID`,
		`CREATE TABLE migration_review_admissions_v2 (
			admission_ref TEXT PRIMARY KEY,
			admission_digest TEXT NOT NULL UNIQUE CHECK(length(admission_digest) = 71 AND substr(admission_digest, 1, 7) = 'sha256:'),
			project_root TEXT NOT NULL,
			packet_carrier_digest TEXT NOT NULL CHECK(length(packet_carrier_digest) = 71 AND substr(packet_carrier_digest, 1, 7) = 'sha256:'),
			review_content_ref TEXT NOT NULL UNIQUE REFERENCES migration_review_acceptance_contents(review_content_ref),
			review_content_digest TEXT NOT NULL,
			review_text TEXT NOT NULL CHECK(review_text != ''),
			review_digest TEXT NOT NULL CHECK(length(review_digest) = 71 AND substr(review_digest, 1, 7) = 'sha256:'),
			capture_carrier_ref TEXT NOT NULL UNIQUE REFERENCES terminal_capture_records(capture_carrier_ref),
			capture_carrier_digest TEXT NOT NULL,
			speech_act_ref TEXT NOT NULL UNIQUE REFERENCES speech_acts(speech_act_ref),
			speech_act_digest TEXT NOT NULL,
			context_policy_ref TEXT NOT NULL REFERENCES speech_act_context_policies(context_policy_ref),
			context_policy_digest TEXT NOT NULL CHECK(length(context_policy_digest) = 71 AND substr(context_policy_digest, 1, 7) = 'sha256:'),
			act_type_ref TEXT NOT NULL,
			method_ref TEXT NOT NULL,
			method_description_ref TEXT NOT NULL REFERENCES speech_act_method_descriptions(method_description_ref),
			method_description_digest TEXT NOT NULL CHECK(length(method_description_digest) = 71 AND substr(method_description_digest, 1, 7) = 'sha256:'),
			bounded_context_ref TEXT NOT NULL,
			institutional_effect_rule_ref TEXT NOT NULL,
			admission_json TEXT NOT NULL UNIQUE CHECK(json_valid(admission_json)),
			admitted_at TEXT NOT NULL,
			recorded_at TEXT NOT NULL,
			CHECK(json_extract(admission_json, '$.schema') = 'haft.spec-migration-v2.semantic-review-admission/v2'),
			CHECK(json_extract(admission_json, '$.admission_ref') = admission_ref),
			CHECK(json_extract(admission_json, '$.project_root') = project_root),
			CHECK(json_extract(admission_json, '$.packet_carrier_digest') = packet_carrier_digest),
			CHECK(json_extract(admission_json, '$.review_content_ref') = review_content_ref),
			CHECK(json_extract(admission_json, '$.review_content_digest') = review_content_digest),
			CHECK(json_extract(admission_json, '$.review_text') = review_text),
			CHECK(json_extract(admission_json, '$.review_digest') = review_digest),
			CHECK(json_extract(admission_json, '$.capture_carrier_ref') = capture_carrier_ref),
			CHECK(json_extract(admission_json, '$.capture_carrier_digest') = capture_carrier_digest),
			CHECK(json_extract(admission_json, '$.speech_act_ref') = speech_act_ref),
			CHECK(json_extract(admission_json, '$.speech_act_digest') = speech_act_digest),
			CHECK(json_extract(admission_json, '$.context_policy_ref') = context_policy_ref),
			CHECK(json_extract(admission_json, '$.context_policy_digest') = context_policy_digest),
			CHECK(json_extract(admission_json, '$.act_type_ref') = act_type_ref),
			CHECK(json_extract(admission_json, '$.method_ref') = method_ref),
			CHECK(json_extract(admission_json, '$.method_description_ref') = method_description_ref),
			CHECK(json_extract(admission_json, '$.method_description_digest') = method_description_digest),
			CHECK(json_extract(admission_json, '$.bounded_context_ref') = bounded_context_ref),
			CHECK(json_extract(admission_json, '$.institutional_effect_rule_ref') = institutional_effect_rule_ref),
			CHECK(json_extract(admission_json, '$.admitted_at') = admitted_at)
		) WITHOUT ROWID`,
		`CREATE TABLE migration_review_instituted_effects (
			effect_digest TEXT PRIMARY KEY CHECK(length(effect_digest) = 71 AND substr(effect_digest, 1, 7) = 'sha256:'),
			project_root TEXT NOT NULL,
			speech_act_ref TEXT NOT NULL UNIQUE REFERENCES speech_acts(speech_act_ref),
			speech_act_digest TEXT NOT NULL,
			admission_ref TEXT NOT NULL UNIQUE REFERENCES migration_review_admissions_v2(admission_ref),
			admission_digest TEXT NOT NULL,
			canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
			recorded_at TEXT NOT NULL,
			CHECK(json_extract(canonical_json, '$.schema') = 'haft.spec-migration-v2.review-instituted-effect/v1'),
			CHECK(json_extract(canonical_json, '$.effect_digest') = effect_digest),
			CHECK(json_extract(canonical_json, '$.project_root') = project_root),
			CHECK(json_extract(canonical_json, '$.speech_act_ref') = speech_act_ref),
			CHECK(json_extract(canonical_json, '$.speech_act_digest') = speech_act_digest),
			CHECK(json_extract(canonical_json, '$.admission_ref') = admission_ref),
			CHECK(json_extract(canonical_json, '$.admission_digest') = admission_digest)
		) WITHOUT ROWID`,
		`CREATE INDEX idx_migration_review_acceptance_current
		 ON migration_review_acceptance_contents(project_root, packet_carrier_digest, partition_audit_digest)`,
		`CREATE INDEX idx_migration_review_admissions_v2_current
		 ON migration_review_admissions_v2(project_root, packet_carrier_digest)`,
	}
	statements = appendMigrationReviewProtocolV2ExactBindingTriggers(statements)
	statements = appendMigrationReviewProtocolV2ImmutabilityTriggers(statements)
	return appendMigrationReviewProtocolV2RootGuards(statements)
}

func appendMigrationReviewProtocolV2ExactBindingTriggers(
	statements []string,
) []string {
	return append(
		statements,
		`CREATE TRIGGER migration_review_admissions_v2_exact_sources
		 BEFORE INSERT ON migration_review_admissions_v2
		 WHEN NOT EXISTS (
			SELECT 1
			FROM migration_review_acceptance_contents content
			JOIN terminal_capture_records capture
				ON capture.capture_carrier_ref = NEW.capture_carrier_ref
			JOIN speech_acts act
				ON act.speech_act_ref = NEW.speech_act_ref
			JOIN speech_act_context_policies policy
				ON policy.context_policy_ref = act.context_policy_ref
				AND policy.context_policy_digest = act.context_policy_digest
			JOIN speech_act_method_descriptions method
				ON method.method_description_ref = act.method_description_ref
				AND method.method_description_digest = act.method_description_digest
			WHERE content.review_content_ref = NEW.review_content_ref
			AND content.review_content_digest = NEW.review_content_digest
			AND content.project_root = NEW.project_root
			AND content.packet_carrier_digest = NEW.packet_carrier_digest
			AND capture.capture_carrier_digest = NEW.capture_carrier_digest
			AND capture.project_root = NEW.project_root
			AND capture.review_text = NEW.review_text
			AND capture.review_digest = NEW.review_digest
			AND capture.canonical_utterance = 'ACCEPT ' || content.review_content_digest
			AND act.speech_act_digest = NEW.speech_act_digest
			AND act.project_root = NEW.project_root
			AND act.capture_carrier_ref = capture.capture_carrier_ref
			AND act.capture_carrier_digest = capture.capture_carrier_digest
			AND act.review_subject_ref = content.review_content_ref
			AND act.review_subject_digest = content.review_content_digest
			AND act.instituted_object_ref = NEW.admission_ref
			AND act.act_type_ref = NEW.act_type_ref
			AND act.method_ref = NEW.method_ref
			AND act.method_description_ref = NEW.method_description_ref
			AND act.method_description_digest = NEW.method_description_digest
			AND act.bounded_context_ref = NEW.bounded_context_ref
			AND act.context_policy_ref = NEW.context_policy_ref
			AND act.context_policy_digest = NEW.context_policy_digest
			AND act.executed_within_ref = 'system:haft-spec-migration-review'
			AND act.state_plane_ref = 'state-plane:spec-migration-review-admission'
			AND act.delta_predicate_ref = 'delta-predicate:review-admission-instituted'
			AND act.outcome_ref = 'work-outcome:review-admission-instituted'
			AND act.utterance_ref = 'utterance:exact-migration-review-acceptance'
			AND NEW.context_policy_ref = 'context-policy:migration-review-acceptance:v1'
			AND NEW.act_type_ref = 'speech-act-type:accept'
			AND NEW.method_ref = 'method:migration-review-acceptance'
			AND NEW.method_description_ref = 'method-description:migration-review-acceptance:v1'
			AND NEW.bounded_context_ref = 'bounded-context:haft-spec-migration-v2'
			AND NEW.institutional_effect_rule_ref = 'institution-rule:accept-institutes-migration-review-admission:v1'
			AND policy.context_policy_ref = NEW.context_policy_ref
			AND policy.context_policy_digest = NEW.context_policy_digest
			AND policy.bounded_context_ref = NEW.bounded_context_ref
			AND policy.recognized_act_type_ref = NEW.act_type_ref
			AND policy.institutional_effect_rule_ref = NEW.institutional_effect_rule_ref
			AND policy.instituted_object_kind = 'haft.MigrationReviewAdmission'
			AND policy.institutional_modality = 'ADMITTED'
			AND policy.scoped_action = 'spec-migration-v2.review.admit'
			AND policy.utterance_description_ref = 'utterance:exact-migration-review-acceptance'
			AND policy.utterance_verb = 'ACCEPT'
			AND policy.utterance_binding = 'review_subject_digest'
			AND method.method_ref = NEW.method_ref
			AND method.method_description_ref = NEW.method_description_ref
			AND method.method_description_digest = NEW.method_description_digest
			AND method.procedure_ref = 'procedure:review-exact-intent-capture-controlling-terminal:v1'
			AND method.bounded_context_ref = 'bounded-context:haft-spec-migration-v2'
			AND method.procedure_semantics = 'display exact pre-act review bindings; require the policy-owned canonical utterance on the controlling terminal; observe terminal session and capture time; derive capture, authorizer assignment, and SpeechAct in that order'
		 ) BEGIN
			SELECT RAISE(ABORT, 'migration-review admission does not bind exact content, canonical review text, and sealed SpeechAct protocol');
		 END`,
		`CREATE TRIGGER migration_review_instituted_effects_exact_sources
		 BEFORE INSERT ON migration_review_instituted_effects
		 WHEN NOT EXISTS (
			SELECT 1
			FROM speech_acts act
			JOIN migration_review_admissions_v2 admission
				ON admission.admission_ref = NEW.admission_ref
			WHERE act.speech_act_ref = NEW.speech_act_ref
			AND act.speech_act_digest = NEW.speech_act_digest
			AND act.project_root = NEW.project_root
			AND act.instituted_object_ref = admission.admission_ref
			AND admission.admission_digest = NEW.admission_digest
			AND admission.project_root = NEW.project_root
			AND admission.speech_act_ref = act.speech_act_ref
			AND admission.speech_act_digest = act.speech_act_digest
		 ) BEGIN
			SELECT RAISE(ABORT, 'migration-review instituted effect does not bind exact SpeechAct and admission');
		 END`,
	)
}

func appendMigrationReviewProtocolV2ImmutabilityTriggers(
	statements []string,
) []string {
	tables := []immutableAuthorityBasisTable{
		{name: "migration_review_acceptance_contents", primaryKey: "review_content_ref", digestColumn: "review_content_digest"},
		{name: "migration_review_admissions_v2", primaryKey: "admission_ref", digestColumn: "admission_digest"},
		{name: "migration_review_instituted_effects", primaryKey: "effect_digest", digestColumn: "effect_digest"},
	}
	return appendAuthorityBasisTableTriggers(statements, tables, 0)
}

func appendMigrationReviewProtocolV2RootGuards(statements []string) []string {
	tables := []string{
		"migration_review_acceptance_contents",
		"migration_review_admissions_v2",
		"migration_review_instituted_effects",
	}
	return appendMigrationReviewProtocolV2RootGuard(statements, tables, 0)
}

func appendMigrationReviewProtocolV2RootGuard(
	statements []string,
	tables []string,
	index int,
) []string {
	if index >= len(tables) {
		return statements
	}
	table := tables[index]
	trigger := "CREATE TRIGGER " + table + "_project_ledger_root " +
		"BEFORE INSERT ON " + table + " WHEN EXISTS (SELECT 1 FROM project_ledger_binding) " +
		"AND NOT EXISTS (SELECT 1 FROM project_ledger_binding binding WHERE binding.project_root = NEW.project_root) " +
		"BEGIN SELECT RAISE(ABORT, '" + table + " does not match the bound project ledger root'); END"
	next := slices.Clone(statements)
	next = append(next, trigger)
	return appendMigrationReviewProtocolV2RootGuard(next, tables, index+1)
}
