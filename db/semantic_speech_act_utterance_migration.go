package db

import "fmt"

var semanticSpeechActUtteranceMigration40 = Migration{
	Version:     40,
	Description: "Support policy-owned human-readable SpeechAct utterances",
	Apply:       applySemanticSpeechActUtteranceMigration40,
}

func applySemanticSpeechActUtteranceMigration40(
	tx MigrationTransaction,
	_ []Migration,
) error {
	if err := requireSemanticSpeechActUtteranceSource(tx); err != nil {
		return err
	}
	statements := []string{
		`ALTER TABLE speech_act_context_policies
		 ADD COLUMN utterance_literal TEXT NOT NULL DEFAULT ''`,
		`DROP TRIGGER speech_acts_exact_sources`,
		speechActContextPolicyUtteranceShapeTriggerV40(),
		speechActsExactSourcesTriggerV40(),
	}
	if err := executeStatements(tx, statements, 0); err != nil {
		return fmt.Errorf("install semantic SpeechAct utterance support: %w", err)
	}
	if err := verifyForeignKeys(tx); err != nil {
		return fmt.Errorf("verify semantic SpeechAct utterance support: %w", err)
	}
	return nil
}

func requireSemanticSpeechActUtteranceSource(tx MigrationTransaction) error {
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version IN (38, 39)",
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect SpeechAct source migrations: %w", err)
	}
	if count != 2 {
		return fmt.Errorf("semantic SpeechAct utterances require schema versions 38 and 39")
	}
	return nil
}

func speechActContextPolicyUtteranceShapeTriggerV40() string {
	return `CREATE TRIGGER speech_act_context_policies_utterance_shape
	 BEFORE INSERT ON speech_act_context_policies
	 WHEN (
		NEW.utterance_binding = 'literal'
		AND (
			NEW.utterance_literal = ''
			OR json_extract(NEW.canonical_json, '$.utterance_literal') IS NOT NEW.utterance_literal
		)
	 ) OR (
		NEW.utterance_binding != 'literal'
		AND (
			NEW.utterance_literal != ''
			OR json_type(NEW.canonical_json, '$.utterance_literal') IS NOT NULL
		)
	 ) BEGIN
		SELECT RAISE(ABORT, 'SpeechAct context policy utterance binding and literal disagree');
	 END`
}

func speechActsExactSourcesTriggerV40() string {
	return `CREATE TRIGGER speech_acts_exact_sources
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
		AND capture.canonical_utterance = CASE policy.utterance_binding
			WHEN 'review_digest' THEN policy.utterance_verb || ' ' || capture.review_digest
			WHEN 'review_subject_digest' THEN policy.utterance_verb || ' ' || NEW.review_subject_digest
			WHEN 'literal' THEN policy.utterance_verb || ' ' || policy.utterance_literal
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
	 END`
}
