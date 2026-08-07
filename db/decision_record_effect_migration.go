package db

import (
	"fmt"
	"slices"
)

var decisionRecordEffectMigration41 = Migration{
	Version:     41,
	Description: "Bind DecisionRecord effects to exact generic SpeechAct sources",
	Apply:       applyDecisionRecordEffectMigration41,
}

var decisionRecordEffectMigration41Tables = []string{
	"decision_binding_contents",
	"decision_record_instituted_effects",
}

const (
	decisionBindingContentSchemaV1  = "haft.decision-binding-content/v1"
	decisionBindingPreparedSchemaV1 = "haft.artifact.prepared-decision/v1"
	decisionBindingPolicyRefV1      = "context-policy:decision-binding:v1"
	decisionBindingContextRefV1     = "bounded-context:haft-decision-binding"
	decisionBindingActTypeRefV1     = "speech-act-type:decide"
	decisionBindingEffectRuleRefV1  = "institution-rule:decide-institutes-decision-record:v1"
	decisionBindingObjectKindV1     = "haft.DecisionRecord"
	decisionBindingModalityV1       = "BOUND"
	decisionBindingActionV1         = "decision.bind"
	decisionBindingUtteranceRefV1   = "utterance:decide-this-reviewed-choice:v1"
	decisionBindingUtteranceVerbV1  = "DECIDE"
	decisionBindingUtteranceTextV1  = "THIS REVIEWED CHOICE"
	decisionBindingMethodRefV1      = "method:decision-binding-manual-speech-act"
	decisionBindingMethodDescRefV1  = "method-description:decision-binding-manual-speech-act:v1"
	decisionBindingProcedureRefV1   = "procedure:review-exact-intent-capture-controlling-terminal:v1"
	decisionBindingSystemRefV1      = "system:haft-decision-binding"
	decisionBindingStatePlaneRefV1  = "state-plane:decision-record-binding"
	decisionBindingDeltaRefV1       = "delta-predicate:decision-record-instituted"
	decisionBindingOutcomeRefV1     = "work-outcome:decision-record-instituted"
)

func applyDecisionRecordEffectMigration41(
	tx MigrationTransaction,
	_ []Migration,
) error {
	if err := requireDecisionRecordEffectSource(tx); err != nil {
		return err
	}
	if err := requireAbsentDecisionRecordEffectFootprint(tx, 0); err != nil {
		return err
	}
	if err := executeStatements(tx, decisionRecordEffectMigration41Statements(), 0); err != nil {
		return fmt.Errorf("install DecisionRecord institutional effect closure: %w", err)
	}
	if err := verifyForeignKeys(tx); err != nil {
		return fmt.Errorf("verify DecisionRecord institutional effect closure: %w", err)
	}
	return nil
}

func requireDecisionRecordEffectSource(tx MigrationTransaction) error {
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version IN (38, 40)",
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect generic SpeechAct source migrations: %w", err)
	}
	if count != 2 {
		return fmt.Errorf("DecisionRecord effects require generic SpeechAct migrations 38 and 40")
	}
	return nil
}

func requireAbsentDecisionRecordEffectFootprint(
	tx MigrationTransaction,
	index int,
) error {
	if index >= len(decisionRecordEffectMigration41Tables) {
		return nil
	}
	table := decisionRecordEffectMigration41Tables[index]
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect DecisionRecord effect table %s: %w", table, err)
	}
	if count != 0 {
		return fmt.Errorf(
			"DecisionRecord effect migration refused: unversioned table %s already exists; unknown partial schema requires manual review",
			table,
		)
	}
	return requireAbsentDecisionRecordEffectFootprint(tx, index+1)
}

func decisionRecordEffectMigration41Statements() []string {
	statements := []string{
		decisionBindingContentsTableV41(),
		decisionRecordInstitutedEffectsTableV41(),
		decisionBindingContentsShapeTriggerV41(),
		decisionBindingContentsNoRetrobindTriggerV41(),
		decisionRecordEffectsExactSourceTriggerV41(),
		decisionRecordEffectsExactArtifactTriggerV41(),
		`CREATE INDEX idx_decision_binding_contents_project_recorded
		 ON decision_binding_contents(project_root, recorded_at)`,
		`CREATE INDEX idx_decision_record_effects_project_recorded
		 ON decision_record_instituted_effects(project_root, recorded_at)`,
	}
	statements = appendDecisionRecordEffectImmutabilityTriggers(statements)
	return appendDecisionRecordEffectRootGuards(statements)
}

func decisionBindingContentsTableV41() string {
	return `CREATE TABLE decision_binding_contents (
		decision_content_ref TEXT PRIMARY KEY,
		decision_content_digest TEXT NOT NULL UNIQUE
			CHECK(length(decision_content_digest) = 71 AND substr(decision_content_digest, 1, 7) = 'sha256:'),
		prepared_decision_digest TEXT NOT NULL UNIQUE
			CHECK(length(prepared_decision_digest) = 71 AND substr(prepared_decision_digest, 1, 7) = 'sha256:'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		decision_ref TEXT NOT NULL UNIQUE CHECK(decision_ref != ''),
		canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
		recorded_at TEXT NOT NULL,
		CHECK(decision_content_ref = 'review-subject:decision-binding:' || substr(decision_content_digest, 8)),
		CHECK(COALESCE(json_extract(canonical_json, '$.schema') = '` + decisionBindingContentSchemaV1 + `', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.prepared_decision_digest') = prepared_decision_digest, 0)),
		CHECK(COALESCE(json_type(canonical_json, '$.prepared_decision') = 'object', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.prepared_decision.schema') = '` + decisionBindingPreparedSchemaV1 + `', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.prepared_decision.project_root') = project_root, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.prepared_decision.decision_ref') = decision_ref, 0)),
		CHECK(COALESCE(json_type(canonical_json, '$.prepared_decision.proposal_input') = 'object', 0)),
		CHECK(COALESCE(json_type(canonical_json, '$.prepared_decision.resolved_input') = 'object', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.prepared_decision.artifact.id') = decision_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.prepared_decision.artifact.kind') = 'DecisionRecord', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.prepared_decision.artifact.version') = 1, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.prepared_decision.artifact.status') = 'active', 0)),
		CHECK(COALESCE(json_type(canonical_json, '$.prepared_decision.artifact.context') = 'text', 0)),
		CHECK(COALESCE(json_type(canonical_json, '$.prepared_decision.artifact.mode') = 'text', 0)),
		CHECK(COALESCE(json_type(canonical_json, '$.prepared_decision.artifact.title') = 'text', 0)),
		CHECK(COALESCE(json_type(canonical_json, '$.prepared_decision.artifact.body') = 'text', 0)),
		CHECK(COALESCE(json_type(canonical_json, '$.prepared_decision.artifact.valid_until') = 'text', 0)),
		CHECK(COALESCE(json_type(canonical_json, '$.prepared_decision.artifact.search_keywords') = 'text', 0)),
		CHECK(COALESCE(json_type(canonical_json, '$.prepared_decision.artifact.structured_data') = 'object', 0)),
		CHECK(COALESCE(json_type(canonical_json, '$.prepared_decision.links') = 'array', 0)),
		CHECK(COALESCE(json_type(canonical_json, '$.prepared_decision.affected_files') = 'array', 0)),
		CHECK(COALESCE(json_type(canonical_json, '$.prepared_decision.source_pins') = 'array', 0))
	) WITHOUT ROWID`
}

func decisionRecordInstitutedEffectsTableV41() string {
	return `CREATE TABLE decision_record_instituted_effects (
		effect_digest TEXT PRIMARY KEY
			CHECK(length(effect_digest) = 71 AND substr(effect_digest, 1, 7) = 'sha256:'),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		decision_ref TEXT NOT NULL UNIQUE CHECK(decision_ref != '') REFERENCES artifacts(id),
		decision_content_ref TEXT NOT NULL UNIQUE REFERENCES decision_binding_contents(decision_content_ref),
		decision_content_digest TEXT NOT NULL
			CHECK(length(decision_content_digest) = 71 AND substr(decision_content_digest, 1, 7) = 'sha256:'),
		speech_act_ref TEXT NOT NULL UNIQUE REFERENCES speech_acts(speech_act_ref),
		speech_act_digest TEXT NOT NULL
			CHECK(length(speech_act_digest) = 71 AND substr(speech_act_digest, 1, 7) = 'sha256:'),
		context_policy_ref TEXT NOT NULL REFERENCES speech_act_context_policies(context_policy_ref),
		context_policy_digest TEXT NOT NULL
			CHECK(length(context_policy_digest) = 71 AND substr(context_policy_digest, 1, 7) = 'sha256:'),
		institutional_effect_rule_ref TEXT NOT NULL,
		canonical_json TEXT NOT NULL UNIQUE CHECK(json_valid(canonical_json)),
		recorded_at TEXT NOT NULL,
		CHECK(COALESCE(json_extract(canonical_json, '$.schema') = 'haft.decision-record-instituted-effect/v1', 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.effect_digest') = effect_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.project_root') = project_root, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.decision_ref') = decision_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.decision_content_ref') = decision_content_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.decision_content_digest') = decision_content_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.speech_act_ref') = speech_act_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.speech_act_digest') = speech_act_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.context_policy_ref') = context_policy_ref, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.context_policy_digest') = context_policy_digest, 0)),
		CHECK(COALESCE(json_extract(canonical_json, '$.institutional_effect_rule_ref') = institutional_effect_rule_ref, 0))
	) WITHOUT ROWID`
}

func decisionBindingContentsShapeTriggerV41() string {
	return `CREATE TRIGGER decision_binding_contents_exact_collection_shape
	 BEFORE INSERT ON decision_binding_contents
	 WHEN EXISTS (
		SELECT 1 FROM json_each(NEW.canonical_json, '$.prepared_decision.links') item
		WHERE COALESCE(json_type(item.value, '$.ref') = 'text', 0) = 0
		OR COALESCE(json_extract(item.value, '$.ref') != '', 0) = 0
		OR COALESCE(json_type(item.value, '$.type') = 'text', 0) = 0
		OR COALESCE(json_extract(item.value, '$.type') != '', 0) = 0
	 ) OR EXISTS (
		SELECT 1 FROM json_each(NEW.canonical_json, '$.prepared_decision.affected_files') item
		WHERE COALESCE(json_type(item.value, '$.path') = 'text', 0) = 0
		OR COALESCE(json_extract(item.value, '$.path') != '', 0) = 0
		OR json_type(item.value, '$.hash') IS NOT NULL
	 ) OR EXISTS (
		SELECT 1 FROM json_each(NEW.canonical_json, '$.prepared_decision.source_pins') item
		WHERE COALESCE(json_type(item.value, '$.operation') = 'text', 0) = 0
		OR COALESCE(json_type(item.value, '$.ref') = 'text', 0) = 0
		OR COALESCE(json_extract(item.value, '$.ref') != '', 0) = 0
		OR json_extract(item.value, '$.operation') NOT IN ('get', 'list_by_kind')
		OR (
			json_extract(item.value, '$.operation') = 'get'
			AND COALESCE(json_extract(item.value, '$.outcome') IN ('found', 'unavailable'), 0) = 0
		)
		OR (
			json_extract(item.value, '$.operation') = 'get'
			AND json_extract(item.value, '$.outcome') = 'found'
			AND (
				COALESCE(json_type(item.value, '$.version') = 'integer', 0) = 0
				OR COALESCE(json_extract(item.value, '$.version') >= 1, 0) = 0
				OR COALESCE(json_type(item.value, '$.digest') = 'text', 0) = 0
				OR COALESCE(length(json_extract(item.value, '$.digest')) = 71, 0) = 0
				OR COALESCE(substr(json_extract(item.value, '$.digest'), 1, 7) = 'sha256:', 0) = 0
				OR json_type(item.value, '$.members') IS NOT NULL
			)
		)
		OR (
			json_extract(item.value, '$.operation') = 'get'
			AND json_extract(item.value, '$.outcome') = 'unavailable'
			AND (
				json_type(item.value, '$.version') IS NOT NULL
				OR json_type(item.value, '$.digest') IS NOT NULL
				OR json_type(item.value, '$.members') IS NOT NULL
			)
		)
		OR (
			json_extract(item.value, '$.operation') = 'list_by_kind'
			AND (
				COALESCE(json_extract(item.value, '$.outcome') = 'observed', 0) = 0
				OR json_type(item.value, '$.version') IS NOT NULL
				OR COALESCE(json_type(item.value, '$.digest') = 'text', 0) = 0
				OR COALESCE(length(json_extract(item.value, '$.digest')) = 71, 0) = 0
				OR COALESCE(substr(json_extract(item.value, '$.digest'), 1, 7) = 'sha256:', 0) = 0
				OR (
					json_type(item.value, '$.members') IS NOT NULL
					AND json_type(item.value, '$.members') != 'array'
				)
			)
		)
	 ) OR EXISTS (
		SELECT 1
		FROM json_each(NEW.canonical_json, '$.prepared_decision.source_pins') pin
		JOIN json_each(pin.value, '$.members') member
		WHERE COALESCE(json_type(member.value, '$.ref') = 'text', 0) = 0
		OR COALESCE(json_extract(member.value, '$.ref') != '', 0) = 0
		OR COALESCE(json_type(member.value, '$.version') = 'integer', 0) = 0
		OR COALESCE(json_extract(member.value, '$.version') >= 1, 0) = 0
		OR COALESCE(json_type(member.value, '$.digest') = 'text', 0) = 0
		OR COALESCE(length(json_extract(member.value, '$.digest')) = 71, 0) = 0
		OR COALESCE(substr(json_extract(member.value, '$.digest'), 1, 7) = 'sha256:', 0) = 0
	 ) OR json_array_length(NEW.canonical_json, '$.prepared_decision.links') != (
		SELECT COUNT(*) FROM (
			SELECT 1
			FROM json_each(NEW.canonical_json, '$.prepared_decision.links') item
			GROUP BY json_extract(item.value, '$.type'), json_extract(item.value, '$.ref')
		)
	 ) OR json_array_length(NEW.canonical_json, '$.prepared_decision.affected_files') != (
		SELECT COUNT(*) FROM (
			SELECT 1
			FROM json_each(NEW.canonical_json, '$.prepared_decision.affected_files') item
			GROUP BY json_extract(item.value, '$.path')
		)
	 ) OR json_array_length(NEW.canonical_json, '$.prepared_decision.source_pins') != (
		SELECT COUNT(*) FROM (
			SELECT 1
			FROM json_each(NEW.canonical_json, '$.prepared_decision.source_pins') item
			GROUP BY json_extract(item.value, '$.operation'), json_extract(item.value, '$.ref')
		)
	 ) OR EXISTS (
		SELECT 1
		FROM json_each(NEW.canonical_json, '$.prepared_decision.source_pins') pin
		WHERE json_extract(pin.value, '$.operation') = 'list_by_kind'
		AND json_array_length(pin.value, '$.members') != (
			SELECT COUNT(*) FROM (
				SELECT 1
				FROM json_each(pin.value, '$.members') member
				GROUP BY json_extract(member.value, '$.ref')
			)
		)
	 ) BEGIN
		SELECT RAISE(ABORT, 'PreparedDecision links, affected files, and source pins must be canonical non-duplicated sets');
	 END`
}

func decisionBindingContentsNoRetrobindTriggerV41() string {
	return `CREATE TRIGGER decision_binding_contents_no_retrobind
	 BEFORE INSERT ON decision_binding_contents
	 WHEN EXISTS (
		SELECT 1 FROM artifacts artifact WHERE artifact.id = NEW.decision_ref
	 ) BEGIN
		SELECT RAISE(ABORT, 'DecisionBindingContent must precede its DecisionRecord and cannot authorize a legacy artifact retroactively');
	 END`
}

func decisionRecordEffectsExactSourceTriggerV41() string {
	return `CREATE TRIGGER decision_record_instituted_effects_exact_source
	 BEFORE INSERT ON decision_record_instituted_effects
	 WHEN NOT EXISTS (
		SELECT 1
		FROM decision_binding_contents content
		JOIN speech_acts act ON act.speech_act_ref = NEW.speech_act_ref
		JOIN terminal_capture_records capture ON capture.capture_carrier_ref = act.capture_carrier_ref
		JOIN speech_act_context_policies policy ON policy.context_policy_ref = NEW.context_policy_ref
		JOIN speech_act_method_descriptions method ON method.method_description_ref = act.method_description_ref
		WHERE content.decision_content_ref = NEW.decision_content_ref
		AND content.decision_content_digest = NEW.decision_content_digest
		AND content.project_root = NEW.project_root
		AND content.decision_ref = NEW.decision_ref
		AND content.decision_content_ref = 'review-subject:decision-binding:' || substr(content.decision_content_digest, 8)
		AND act.speech_act_digest = NEW.speech_act_digest
		AND act.speech_act_ref = 'speech-act:decision-binding:' || substr(content.decision_content_digest, 8)
		AND act.project_root = NEW.project_root
		AND act.work_kind = 'Communicative'
		AND act.review_subject_ref = content.decision_content_ref
		AND act.review_subject_digest = content.decision_content_digest
		AND act.instituted_object_ref = NEW.decision_ref
		AND act.context_policy_ref = NEW.context_policy_ref
		AND act.context_policy_digest = NEW.context_policy_digest
		AND capture.capture_carrier_digest = act.capture_carrier_digest
		AND capture.capture_carrier_ref = 'carrier:terminal-capture:decision-binding:' || substr(content.decision_content_digest, 8)
		AND capture.intent_session_ref = 'session:decision-binding:' || substr(content.decision_content_digest, 8)
		AND capture.project_root = NEW.project_root
		AND policy.context_policy_digest = NEW.context_policy_digest
		AND policy.context_policy_ref = '` + decisionBindingPolicyRefV1 + `'
		AND policy.bounded_context_ref = '` + decisionBindingContextRefV1 + `'
		AND policy.recognized_act_type_ref = '` + decisionBindingActTypeRefV1 + `'
		AND policy.institutional_effect_rule_ref = NEW.institutional_effect_rule_ref
		AND NEW.institutional_effect_rule_ref = '` + decisionBindingEffectRuleRefV1 + `'
		AND policy.instituted_object_kind = '` + decisionBindingObjectKindV1 + `'
		AND policy.institutional_modality = '` + decisionBindingModalityV1 + `'
		AND policy.scoped_action = '` + decisionBindingActionV1 + `'
		AND policy.utterance_description_ref = '` + decisionBindingUtteranceRefV1 + `'
		AND policy.utterance_verb = '` + decisionBindingUtteranceVerbV1 + `'
		AND policy.utterance_binding = 'literal'
		AND policy.utterance_literal = '` + decisionBindingUtteranceTextV1 + `'
		AND act.act_type_ref = policy.recognized_act_type_ref
		AND act.bounded_context_ref = policy.bounded_context_ref
		AND act.method_ref = '` + decisionBindingMethodRefV1 + `'
		AND act.method_description_ref = '` + decisionBindingMethodDescRefV1 + `'
		AND method.method_description_digest = act.method_description_digest
		AND method.method_ref = act.method_ref
		AND method.procedure_ref = '` + decisionBindingProcedureRefV1 + `'
		AND method.bounded_context_ref = policy.bounded_context_ref
		AND act.executed_within_ref = '` + decisionBindingSystemRefV1 + `'
		AND act.state_plane_ref = '` + decisionBindingStatePlaneRefV1 + `'
		AND act.delta_predicate_ref = '` + decisionBindingDeltaRefV1 + `'
		AND act.outcome_ref = '` + decisionBindingOutcomeRefV1 + `'
		AND act.utterance_ref = policy.utterance_description_ref
		AND json_array_length(act.parameters_json) = 1
		AND EXISTS (
			SELECT 1 FROM json_each(act.parameters_json) parameter
			WHERE json_extract(parameter.value, '$.name') = 'parameter:decision-binding-content-digest'
			AND json_extract(parameter.value, '$.value') = content.decision_content_digest
		)
		AND json_array_length(act.input_refs_json) = 1
		AND json_extract(act.input_refs_json, '$[0]') = content.decision_content_ref
		AND json_array_length(act.output_refs_json) = 1
		AND json_extract(act.output_refs_json, '$[0]') = NEW.decision_ref
		AND json_array_length(act.resource_refs_json) = 1
		AND json_extract(act.resource_refs_json, '$[0]') = 'resource:controlling-terminal'
		AND json_array_length(act.affected_refs_json) = 2
		AND EXISTS (
			SELECT 1 FROM json_each(act.affected_refs_json) affected
			WHERE affected.value = 'affected:decision-record:' || NEW.decision_ref
		)
		AND EXISTS (
			SELECT 1 FROM json_each(act.affected_refs_json) affected
			WHERE affected.value = 'affected:decision-binding-content:' || content.decision_content_digest
		)
	 ) BEGIN
		SELECT RAISE(ABORT, 'DecisionRecord effect does not bind exact decision content, context policy, and SpeechAct source');
	 END`
}

func decisionRecordEffectsExactArtifactTriggerV41() string {
	// The trigger proves the instituted state at effect time. The immutable
	// historical snapshot remains in decision_binding_contents, while later
	// baseline, evidence, lifecycle, and graph-projection Work may evolve the
	// current artifact rows through their own bounded operations.
	return `CREATE TRIGGER decision_record_instituted_effects_exact_artifact
	 BEFORE INSERT ON decision_record_instituted_effects
	 WHEN NOT EXISTS (
		SELECT 1
		FROM decision_binding_contents content
		JOIN artifacts artifact ON artifact.id = NEW.decision_ref
		JOIN speech_acts act ON act.speech_act_ref = NEW.speech_act_ref
		JOIN terminal_capture_records capture ON capture.capture_carrier_ref = act.capture_carrier_ref
		WHERE content.decision_content_ref = NEW.decision_content_ref
		AND content.decision_content_digest = NEW.decision_content_digest
		AND content.decision_ref = artifact.id
		AND act.speech_act_digest = NEW.speech_act_digest
		AND capture.capture_carrier_digest = act.capture_carrier_digest
		AND artifact.created_at = capture.exact_utterance_observed_at
		AND artifact.updated_at = capture.exact_utterance_observed_at
		AND artifact.kind = json_extract(content.canonical_json, '$.prepared_decision.artifact.kind')
		AND artifact.version = json_extract(content.canonical_json, '$.prepared_decision.artifact.version')
		AND artifact.status = json_extract(content.canonical_json, '$.prepared_decision.artifact.status')
		AND artifact.context IS json_extract(content.canonical_json, '$.prepared_decision.artifact.context')
		AND artifact.mode IS json_extract(content.canonical_json, '$.prepared_decision.artifact.mode')
		AND artifact.title = json_extract(content.canonical_json, '$.prepared_decision.artifact.title')
		AND artifact.content = json_extract(content.canonical_json, '$.prepared_decision.artifact.body')
		AND artifact.valid_until IS json_extract(content.canonical_json, '$.prepared_decision.artifact.valid_until')
		AND COALESCE(artifact.search_keywords, '') = json_extract(content.canonical_json, '$.prepared_decision.artifact.search_keywords')
		AND json(artifact.structured_data) = json(json_extract(content.canonical_json, '$.prepared_decision.artifact.structured_data'))
		AND (SELECT COUNT(*) FROM artifact_links link WHERE link.source_id = artifact.id)
			= json_array_length(content.canonical_json, '$.prepared_decision.links')
		AND NOT EXISTS (
			SELECT 1 FROM json_each(content.canonical_json, '$.prepared_decision.links') item
			WHERE NOT EXISTS (
				SELECT 1 FROM artifact_links link
				WHERE link.source_id = artifact.id
				AND link.target_id = json_extract(item.value, '$.ref')
				AND link.link_type = json_extract(item.value, '$.type')
			)
		)
		AND (SELECT COUNT(*) FROM affected_files file WHERE file.artifact_id = artifact.id)
			= json_array_length(content.canonical_json, '$.prepared_decision.affected_files')
		AND NOT EXISTS (
			SELECT 1 FROM json_each(content.canonical_json, '$.prepared_decision.affected_files') item
			WHERE NOT EXISTS (
				SELECT 1 FROM affected_files file
				WHERE file.artifact_id = artifact.id
				AND file.file_path = json_extract(item.value, '$.path')
				AND COALESCE(file.file_hash, '') = ''
			)
		)
	 ) BEGIN
		SELECT RAISE(ABORT, 'DecisionRecord effect does not match the exact staged artifact, links, and affected files');
	 END`
}

func appendDecisionRecordEffectImmutabilityTriggers(statements []string) []string {
	tables := []immutableAuthorityBasisTable{
		{name: "decision_binding_contents", primaryKey: "decision_content_ref", digestColumn: "decision_content_digest"},
		{name: "decision_record_instituted_effects", primaryKey: "effect_digest", digestColumn: "effect_digest"},
	}
	return appendAuthorityBasisTableTriggers(statements, tables, 0)
}

func appendDecisionRecordEffectRootGuards(statements []string) []string {
	tables := []string{
		"decision_binding_contents",
		"decision_record_instituted_effects",
	}
	return appendDecisionRecordEffectRootGuard(statements, tables, 0)
}

func appendDecisionRecordEffectRootGuard(
	statements []string,
	tables []string,
	index int,
) []string {
	if index >= len(tables) {
		return statements
	}
	table := tables[index]
	statement := "CREATE TRIGGER " + table + "_project_ledger_root " +
		"BEFORE INSERT ON " + table + " WHEN EXISTS (SELECT 1 FROM project_ledger_binding) " +
		"AND NOT EXISTS (SELECT 1 FROM project_ledger_binding binding WHERE binding.project_root = NEW.project_root) " +
		"BEGIN SELECT RAISE(ABORT, '" + table + " does not match the bound project ledger root'); END"
	next := slices.Clone(statements)
	next = append(next, statement)
	return appendDecisionRecordEffectRootGuard(next, tables, index+1)
}
