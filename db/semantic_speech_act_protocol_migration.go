package db

import "fmt"

const (
	migrationReviewSemanticLiteralPolicyRefV42 = "context-policy:migration-review-acceptance:v2"
	migrationReviewSemanticUtteranceRefV42     = "utterance:accept-reviewed-migration:v1"
	migrationReviewSemanticUtteranceVerbV42    = "ACCEPT"
	migrationReviewSemanticUtteranceLiteralV42 = "REVIEWED MIGRATION"
)

var semanticSpeechActProtocolMigration42 = Migration{
	Version:     42,
	Description: "Complete semantic-literal compatibility for versioned migration-review acts",
	Apply:       applySemanticSpeechActProtocolMigration42,
}

func applySemanticSpeechActProtocolMigration42(
	tx MigrationTransaction,
	_ []Migration,
) error {
	if err := requireSemanticSpeechActProtocolSourceV42(tx); err != nil {
		return err
	}
	statements := []string{
		`DROP TRIGGER migration_review_admissions_v2_exact_sources`,
		migrationReviewAdmissionsExactSemanticLiteralTriggerV42(),
	}
	if err := executeStatements(tx, statements, 0); err != nil {
		return fmt.Errorf("install semantic SpeechAct protocol v42: %w", err)
	}
	if err := verifyForeignKeys(tx); err != nil {
		return fmt.Errorf("verify semantic SpeechAct protocol v42: %w", err)
	}
	return nil
}

func requireSemanticSpeechActProtocolSourceV42(tx MigrationTransaction) error {
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version IN (38, 39, 40, 41)",
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect semantic SpeechAct protocol source migrations: %w", err)
	}
	if count != 4 {
		return fmt.Errorf("semantic SpeechAct protocol v42 requires schema versions 38 through 41")
	}
	return nil
}

func migrationReviewAdmissionsExactSemanticLiteralTriggerV42() string {
	return `CREATE TRIGGER migration_review_admissions_v2_exact_sources
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
		AND capture.canonical_utterance = '` + migrationReviewSemanticUtteranceVerbV42 + ` ` + migrationReviewSemanticUtteranceLiteralV42 + `'
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
		AND act.utterance_ref = '` + migrationReviewSemanticUtteranceRefV42 + `'
		AND NEW.context_policy_ref = '` + migrationReviewSemanticLiteralPolicyRefV42 + `'
		AND NEW.act_type_ref = 'speech-act-type:accept'
		AND NEW.method_ref = 'method:migration-review-acceptance'
		AND NEW.method_description_ref = 'method-description:migration-review-acceptance:v1'
		AND NEW.bounded_context_ref = 'bounded-context:haft-spec-migration-v2'
		AND NEW.institutional_effect_rule_ref = 'institution-rule:accept-institutes-migration-review-admission:v2'
		AND policy.context_policy_ref = NEW.context_policy_ref
		AND policy.context_policy_digest = NEW.context_policy_digest
		AND policy.bounded_context_ref = NEW.bounded_context_ref
		AND policy.recognized_act_type_ref = NEW.act_type_ref
		AND policy.institutional_effect_rule_ref = NEW.institutional_effect_rule_ref
		AND policy.instituted_object_kind = 'haft.MigrationReviewAdmission'
		AND policy.institutional_modality = 'ADMITTED'
		AND policy.scoped_action = 'spec-migration-v2.review.admit'
		AND policy.utterance_description_ref = '` + migrationReviewSemanticUtteranceRefV42 + `'
		AND policy.utterance_verb = '` + migrationReviewSemanticUtteranceVerbV42 + `'
		AND policy.utterance_binding = 'literal'
		AND policy.utterance_literal = '` + migrationReviewSemanticUtteranceLiteralV42 + `'
		AND method.method_ref = NEW.method_ref
		AND method.method_description_ref = NEW.method_description_ref
		AND method.method_description_digest = NEW.method_description_digest
		AND method.procedure_ref = 'procedure:review-exact-intent-capture-controlling-terminal:v1'
		AND method.bounded_context_ref = 'bounded-context:haft-spec-migration-v2'
		AND method.procedure_semantics = 'display exact pre-act review bindings; require the policy-owned canonical utterance on the controlling terminal; observe terminal session and capture time; derive capture, authorizer assignment, and SpeechAct in that order'
	 ) BEGIN
		SELECT RAISE(ABORT, 'migration-review admission does not bind exact content, canonical review text, and sealed semantic-literal SpeechAct protocol');
	 END`
}
