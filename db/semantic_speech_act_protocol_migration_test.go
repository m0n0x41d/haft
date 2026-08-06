package db

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSemanticSpeechActProtocolMigration42PinsNewReviewLiteral(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "semantic-protocol-v42.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	var triggerSQL string
	err = store.conn.QueryRow(`SELECT sql FROM sqlite_master
		WHERE type = 'trigger' AND name = 'migration_review_admissions_v2_exact_sources'`).Scan(&triggerSQL)
	if err != nil {
		t.Fatalf("read v42 migration-review trigger: %v", err)
	}
	required := []string{
		"capture.canonical_utterance = 'ACCEPT REVIEWED MIGRATION'",
		"act.utterance_ref = 'utterance:accept-reviewed-migration:v1'",
		"NEW.context_policy_ref = 'context-policy:migration-review-acceptance:v2'",
		"NEW.institutional_effect_rule_ref = 'institution-rule:accept-institutes-migration-review-admission:v2'",
		"policy.context_policy_digest = NEW.context_policy_digest",
		"policy.utterance_binding = 'literal'",
		"policy.utterance_literal = 'REVIEWED MIGRATION'",
	}
	for _, fragment := range required {
		if !strings.Contains(triggerSQL, fragment) {
			t.Fatalf("v42 migration-review trigger omitted %q", fragment)
		}
	}
	forbidden := []string{
		"context-policy:migration-review-acceptance:v1",
		"utterance:exact-migration-review-acceptance",
		"'ACCEPT ' || content.review_content_digest",
		"policy.utterance_binding = 'review_subject_digest'",
	}
	for _, fragment := range forbidden {
		if strings.Contains(triggerSQL, fragment) {
			t.Fatalf("v42 migration-review trigger retained writable legacy protocol %q", fragment)
		}
	}
	assertMigrationVersionPresent(t, store.conn, 42)
}

func TestSemanticSpeechActProtocolMigration42AcceptsOnlyV2ReviewWrites(t *testing.T) {
	t.Parallel()

	t.Run("v2 semantic literal", func(t *testing.T) {
		store, err := NewStore(filepath.Join(t.TempDir(), "v2-write.db"))
		if err != nil {
			t.Fatalf("NewStore: %v", err)
		}
		defer store.Close()

		content := reviewV2ContentFixture("/project/review", "v42")
		if err := insertReviewV2Content(store.conn, content); err != nil {
			t.Fatalf("insert v42 review content: %v", err)
		}
		err = insertSemanticReviewAdmissionV42(
			store.conn,
			content,
			"v42",
			semanticReviewProtocolV42(),
		)
		if err != nil {
			t.Fatalf("insert v2 semantic-literal review admission: %v", err)
		}
	})

	t.Run("v1 digest-bound new write", func(t *testing.T) {
		store, err := NewStore(filepath.Join(t.TempDir(), "v1-write.db"))
		if err != nil {
			t.Fatalf("NewStore: %v", err)
		}
		defer store.Close()

		content := reviewV2ContentFixture("/project/review", "legacy-new")
		if err := insertReviewV2Content(store.conn, content); err != nil {
			t.Fatalf("insert legacy review content: %v", err)
		}
		err = insertSemanticReviewAdmissionV42(
			store.conn,
			content,
			"legacy-new",
			legacyDigestReviewProtocolV39(),
		)
		if err == nil || !strings.Contains(err.Error(), "sealed semantic-literal SpeechAct protocol") {
			t.Fatalf("new v1 digest-bound admission error = %v", err)
		}
	})
}

func TestSemanticSpeechActProtocolMigration42PreservesHistoricalV1Admission(t *testing.T) {
	t.Parallel()

	database := openDatabaseBeforeMigration42(t)
	defer database.Close()

	content := reviewV2ContentFixture("/project/review", "historical")
	if err := insertReviewV2Content(database, content); err != nil {
		t.Fatalf("insert historical review content: %v", err)
	}
	err := insertSemanticReviewAdmissionV42(
		database,
		content,
		"historical",
		legacyDigestReviewProtocolV39(),
	)
	if err != nil {
		t.Fatalf("insert historical v1 review admission: %v", err)
	}
	if err := Migrate(database, "schema_version", kernelMigrations); err != nil {
		t.Fatalf("migrate historical v1 admission through v42: %v", err)
	}

	var policyRef string
	err = database.QueryRow(
		"SELECT context_policy_ref FROM migration_review_admissions_v2 WHERE admission_ref = ?",
		"review-admission:v2:historical",
	).Scan(&policyRef)
	if err != nil {
		t.Fatalf("read historical v1 admission after v42: %v", err)
	}
	if policyRef != "context-policy:migration-review-acceptance:v1" {
		t.Fatalf("historical policy ref = %q", policyRef)
	}
	_, err = database.Exec(
		"UPDATE migration_review_admissions_v2 SET review_text = 'changed' WHERE admission_ref = ?",
		"review-admission:v2:historical",
	)
	if err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("historical v1 admission became writable: %v", err)
	}
}

type semanticReviewProtocolFixture struct {
	policyRef          string
	effectRuleRef      string
	policyBinding      string
	policyLiteral      string
	utteranceRef       string
	canonicalUtterance func(reviewV2ContentRow) string
	nonceCharacter     string
}

func semanticReviewProtocolV42() semanticReviewProtocolFixture {
	return semanticReviewProtocolFixture{
		policyRef:          migrationReviewSemanticLiteralPolicyRefV42,
		effectRuleRef:      "institution-rule:accept-institutes-migration-review-admission:v2",
		policyBinding:      "literal",
		policyLiteral:      migrationReviewSemanticUtteranceLiteralV42,
		utteranceRef:       migrationReviewSemanticUtteranceRefV42,
		canonicalUtterance: func(reviewV2ContentRow) string { return "ACCEPT REVIEWED MIGRATION" },
		nonceCharacter:     "b",
	}
}

func legacyDigestReviewProtocolV39() semanticReviewProtocolFixture {
	return semanticReviewProtocolFixture{
		policyRef:          "context-policy:migration-review-acceptance:v1",
		effectRuleRef:      "institution-rule:accept-institutes-migration-review-admission:v1",
		policyBinding:      "review_subject_digest",
		utteranceRef:       "utterance:exact-migration-review-acceptance",
		canonicalUtterance: func(content reviewV2ContentRow) string { return "ACCEPT " + content.digest },
		nonceCharacter:     "c",
	}
}

func insertSemanticReviewAdmissionV42(
	database *sql.DB,
	content reviewV2ContentRow,
	token string,
	protocol semanticReviewProtocolFixture,
) error {
	values := newSemanticReviewSourceValuesV42(content, token, protocol)
	steps := []func() error{
		func() error { return insertSemanticReviewMethodV42(database, values) },
		func() error { return insertSemanticReviewPolicyV42(database, values, protocol) },
		func() error { return insertSemanticReviewCaptureV42(database, values, protocol) },
		func() error { return insertSemanticReviewAssignmentV42(database, values) },
		func() error { return insertSemanticReviewSpeechActV42(database, values, protocol) },
		func() error { return insertSemanticReviewAdmissionRowV42(database, values) },
	}
	return executeSemanticReviewFixtureStepsV42(steps, 0)
}

type semanticReviewSourceValuesV42 struct {
	content            reviewV2ContentRow
	token              string
	methodDigest       string
	policyDigest       string
	effectRuleRef      string
	captureRef         string
	captureDigest      string
	reviewText         string
	reviewDigest       string
	assignmentRef      string
	assignmentDigest   string
	holderSystemRef    string
	speechActRef       string
	speechActDigest    string
	admissionRef       string
	admissionDigest    string
	startedAt          string
	utteranceObserved  string
	endedAt            string
	observationNonce   string
	observationDigest  string
	preparedIntentHash string
}

func newSemanticReviewSourceValuesV42(
	content reviewV2ContentRow,
	token string,
	protocol semanticReviewProtocolFixture,
) semanticReviewSourceValuesV42 {
	return semanticReviewSourceValuesV42{
		content:            content,
		token:              token,
		methodDigest:       reviewV2Digest("method-" + token),
		policyDigest:       reviewV2Digest("policy-" + token),
		effectRuleRef:      protocol.effectRuleRef,
		captureRef:         "carrier:terminal-capture:migration-review:" + token,
		captureDigest:      reviewV2Digest("capture-" + token),
		reviewText:         "review exact migration packet " + token,
		reviewDigest:       reviewV2Digest("review-" + token),
		assignmentRef:      "role-assignment:migration-review:" + token,
		assignmentDigest:   reviewV2Digest("assignment-" + token),
		holderSystemRef:    "system:local-terminal-session:" + token,
		speechActRef:       "speech-act:migration-review:" + token,
		speechActDigest:    reviewV2Digest("speech-" + token),
		admissionRef:       "review-admission:v2:" + token,
		admissionDigest:    reviewV2Digest("admission-" + token),
		startedAt:          "2026-07-15T08:00:00Z",
		utteranceObserved:  "2026-07-15T08:00:00.5Z",
		endedAt:            "2026-07-15T08:00:01Z",
		observationNonce:   strings.Repeat(protocol.nonceCharacter, 32),
		observationDigest:  reviewV2Digest("observation-" + token),
		preparedIntentHash: reviewV2Digest("prepared-" + token),
	}
}

func executeSemanticReviewFixtureStepsV42(steps []func() error, index int) error {
	if index >= len(steps) {
		return nil
	}
	if err := steps[index](); err != nil {
		return err
	}
	return executeSemanticReviewFixtureStepsV42(steps, index+1)
}

const (
	semanticReviewMethodRefV42      = "method:migration-review-acceptance"
	semanticReviewMethodDescRefV42  = "method-description:migration-review-acceptance:v1"
	semanticReviewProcedureRefV42   = "procedure:review-exact-intent-capture-controlling-terminal:v1"
	semanticReviewBoundedContextV42 = "bounded-context:haft-spec-migration-v2"
	semanticReviewProcedureV42      = "display exact pre-act review bindings; require the policy-owned canonical utterance on the controlling terminal; observe terminal session and capture time; derive capture, authorizer assignment, and SpeechAct in that order"
)

func insertSemanticReviewMethodV42(
	database *sql.DB,
	values semanticReviewSourceValuesV42,
) error {
	canonical, err := authorityBasisJSON(map[string]any{
		"schema":                 "haft.authority.speech-act-method-description/v1",
		"method_description_ref": semanticReviewMethodDescRefV42,
		"method_ref":             semanticReviewMethodRefV42,
		"procedure_ref":          semanticReviewProcedureRefV42,
		"bounded_context_ref":    semanticReviewBoundedContextV42,
		"procedure_semantics":    semanticReviewProcedureV42,
	})
	if err != nil {
		return err
	}
	_, err = database.Exec(`INSERT INTO speech_act_method_descriptions (
		method_description_ref, method_description_digest, method_ref,
		procedure_ref, bounded_context_ref, procedure_semantics,
		canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		semanticReviewMethodDescRefV42,
		values.methodDigest,
		semanticReviewMethodRefV42,
		semanticReviewProcedureRefV42,
		semanticReviewBoundedContextV42,
		semanticReviewProcedureV42,
		canonical,
		values.endedAt,
	)
	return err
}

func insertSemanticReviewPolicyV42(
	database *sql.DB,
	values semanticReviewSourceValuesV42,
	protocol semanticReviewProtocolFixture,
) error {
	projection := map[string]any{
		"schema":                        "haft.authority.speech-act-context-policy/v1",
		"ref":                           protocol.policyRef,
		"bounded_context_ref":           semanticReviewBoundedContextV42,
		"recognized_act_type_ref":       "speech-act-type:accept",
		"authorizer_role_ref":           "role:project-principal-authorizer",
		"admitted_holder_kind":          "U.System",
		"assignment_source_rule":        "observed-local-controlling-terminal-session/v1",
		"institutional_effect_rule_ref": values.effectRuleRef,
		"instituted_object_kind":        "haft.MigrationReviewAdmission",
		"institutional_modality":        "ADMITTED",
		"scoped_action":                 "spec-migration-v2.review.admit",
		"utterance_description_ref":     protocol.utteranceRef,
		"utterance_verb":                "ACCEPT",
		"utterance_binding":             protocol.policyBinding,
	}
	if protocol.policyLiteral != "" {
		projection["utterance_literal"] = protocol.policyLiteral
	}
	canonical, err := authorityBasisJSON(projection)
	if err != nil {
		return err
	}
	_, err = database.Exec(`INSERT INTO speech_act_context_policies (
		context_policy_ref, context_policy_digest, bounded_context_ref,
		recognized_act_type_ref, authorizer_role_ref, admitted_holder_kind,
		assignment_source_rule, institutional_effect_rule_ref,
		instituted_object_kind, institutional_modality, scoped_action,
		utterance_description_ref, utterance_verb, utterance_binding,
		utterance_literal, canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		protocol.policyRef,
		values.policyDigest,
		semanticReviewBoundedContextV42,
		"speech-act-type:accept",
		"role:project-principal-authorizer",
		"U.System",
		"observed-local-controlling-terminal-session/v1",
		values.effectRuleRef,
		"haft.MigrationReviewAdmission",
		"ADMITTED",
		"spec-migration-v2.review.admit",
		protocol.utteranceRef,
		"ACCEPT",
		protocol.policyBinding,
		protocol.policyLiteral,
		canonical,
		values.endedAt,
	)
	return err
}

func insertSemanticReviewCaptureV42(
	database *sql.DB,
	values semanticReviewSourceValuesV42,
	protocol semanticReviewProtocolFixture,
) error {
	canonicalUtterance := protocol.canonicalUtterance(values.content)
	canonical, err := authorityBasisJSON(map[string]any{
		"schema":                            "haft.authority.terminal-capture/v1",
		"carrier_ref":                       values.captureRef,
		"project_root":                      values.content.root,
		"prepared_speech_act_intent_digest": values.preparedIntentHash,
		"review_text":                       values.reviewText,
		"review_digest":                     values.reviewDigest,
		"canonical_utterance":               canonicalUtterance,
		"started_at":                        values.startedAt,
		"exact_utterance_observed_at":       values.utteranceObserved,
		"ended_at":                          values.endedAt,
		"session_ref":                       "session:migration-review:" + values.token,
		"observed_session_material":         "path=/dev/tty;mode=Dcrw--w----;pid=1;ppid=0",
		"observation_nonce":                 values.observationNonce,
		"observation_digest":                values.observationDigest,
		"observed_holder_system_ref":        values.holderSystemRef,
		"observed_role_assignment_ref":      values.assignmentRef,
	})
	if err != nil {
		return err
	}
	_, err = database.Exec(`INSERT INTO terminal_capture_records (
		capture_carrier_ref, capture_carrier_digest, project_root,
		prepared_speech_act_intent_digest, review_text, review_digest,
		canonical_utterance, started_at, exact_utterance_observed_at, ended_at,
		intent_session_ref, observed_session_material, observation_nonce,
		observation_digest, observed_holder_system_ref,
		observed_role_assignment_ref, canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		values.captureRef,
		values.captureDigest,
		values.content.root,
		values.preparedIntentHash,
		values.reviewText,
		values.reviewDigest,
		canonicalUtterance,
		values.startedAt,
		values.utteranceObserved,
		values.endedAt,
		"session:migration-review:"+values.token,
		"path=/dev/tty;mode=Dcrw--w----;pid=1;ppid=0",
		values.observationNonce,
		values.observationDigest,
		values.holderSystemRef,
		values.assignmentRef,
		canonical,
		values.endedAt,
	)
	return err
}

func insertSemanticReviewAssignmentV42(
	database *sql.DB,
	values semanticReviewSourceValuesV42,
) error {
	var policyRef string
	err := database.QueryRow(
		"SELECT context_policy_ref FROM speech_act_context_policies WHERE context_policy_digest = ?",
		values.policyDigest,
	).Scan(&policyRef)
	if err != nil {
		return err
	}
	canonical, err := authorityBasisJSON(map[string]any{
		"schema":                               "haft.authority.context-policy-assigned-terminal-session/v1",
		"role_assignment_ref":                  values.assignmentRef,
		"project_root":                         values.content.root,
		"holder_system_ref":                    values.holderSystemRef,
		"admitted_holder_kind":                 "U.System",
		"role_ref":                             "role:project-principal-authorizer",
		"bounded_context_ref":                  semanticReviewBoundedContextV42,
		"valid_from":                           values.startedAt,
		"valid_until":                          values.endedAt,
		"justification_source_ref":             policyRef,
		"justification_source_digest":          values.policyDigest,
		"assignment_provenance_carrier_ref":    values.captureRef,
		"assignment_provenance_carrier_digest": values.captureDigest,
		"identity_boundary":                    "anti-accident terminal-session identity only",
	})
	if err != nil {
		return err
	}
	_, err = database.Exec(`INSERT INTO speech_act_role_assignments (
		role_assignment_ref, role_assignment_digest, project_root,
		holder_system_ref, admitted_holder_kind, role_ref, bounded_context_ref,
		valid_from, valid_until, context_policy_ref, context_policy_digest,
		provenance_carrier_ref, provenance_carrier_digest, identity_boundary,
		canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		values.assignmentRef,
		values.assignmentDigest,
		values.content.root,
		values.holderSystemRef,
		"U.System",
		"role:project-principal-authorizer",
		semanticReviewBoundedContextV42,
		values.startedAt,
		values.endedAt,
		policyRef,
		values.policyDigest,
		values.captureRef,
		values.captureDigest,
		"anti-accident terminal-session identity only",
		canonical,
		values.endedAt,
	)
	return err
}

func insertSemanticReviewSpeechActV42(
	database *sql.DB,
	values semanticReviewSourceValuesV42,
	protocol semanticReviewProtocolFixture,
) error {
	parameters := []any{}
	inputs := []string{values.content.ref}
	outputs := []string{values.admissionRef}
	resources := []string{"resource:controlling-terminal"}
	affected := []string{"affected:migration-review:" + values.token}
	canonical, err := authorityBasisJSON(map[string]any{
		"schema":                              "haft.authority.speech-act/v1",
		"speech_act_ref":                      values.speechActRef,
		"project_root":                        values.content.root,
		"work_kind":                           "Communicative",
		"act_type_ref":                        "speech-act-type:accept",
		"performed_by_role_assignment_ref":    values.assignmentRef,
		"performed_by_role_assignment_digest": values.assignmentDigest,
		"method_ref":                          semanticReviewMethodRefV42,
		"method_description_ref":              semanticReviewMethodDescRefV42,
		"method_description_digest":           values.methodDigest,
		"executed_within_system_ref":          "system:haft-spec-migration-review",
		"bounded_context_ref":                 semanticReviewBoundedContextV42,
		"window_from":                         values.startedAt,
		"window_until":                        values.endedAt,
		"parameters":                          parameters,
		"input_refs":                          inputs,
		"output_refs":                         outputs,
		"resource_refs":                       resources,
		"affected_refs":                       affected,
		"state_plane_ref":                     "state-plane:spec-migration-review-admission",
		"delta_predicate_ref":                 "delta-predicate:review-admission-instituted",
		"outcome_ref":                         "work-outcome:review-admission-instituted",
		"utterance_ref":                       protocol.utteranceRef,
		"capture_carrier_ref":                 values.captureRef,
		"capture_carrier_digest":              values.captureDigest,
		"review_subject_ref":                  values.content.ref,
		"review_subject_digest":               values.content.digest,
		"instituted_object_ref":               values.admissionRef,
	})
	if err != nil {
		return err
	}
	parametersJSON, _ := json.Marshal(parameters)
	inputsJSON, _ := json.Marshal(inputs)
	outputsJSON, _ := json.Marshal(outputs)
	resourcesJSON, _ := json.Marshal(resources)
	affectedJSON, _ := json.Marshal(affected)
	_, err = database.Exec(`INSERT INTO speech_acts (
		speech_act_ref, speech_act_digest, project_root, work_kind, act_type_ref,
		performed_by_ref, performed_by_digest, method_ref,
		method_description_ref, method_description_digest,
		executed_within_ref, bounded_context_ref, window_from, window_until,
		parameters_json, input_refs_json, output_refs_json, resource_refs_json,
		affected_refs_json, state_plane_ref, delta_predicate_ref, outcome_ref,
		utterance_ref, capture_carrier_ref, capture_carrier_digest,
		review_subject_ref, review_subject_digest, instituted_object_ref,
		context_policy_ref, context_policy_digest, canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		values.speechActRef,
		values.speechActDigest,
		values.content.root,
		"Communicative",
		"speech-act-type:accept",
		values.assignmentRef,
		values.assignmentDigest,
		semanticReviewMethodRefV42,
		semanticReviewMethodDescRefV42,
		values.methodDigest,
		"system:haft-spec-migration-review",
		semanticReviewBoundedContextV42,
		values.startedAt,
		values.endedAt,
		string(parametersJSON),
		string(inputsJSON),
		string(outputsJSON),
		string(resourcesJSON),
		string(affectedJSON),
		"state-plane:spec-migration-review-admission",
		"delta-predicate:review-admission-instituted",
		"work-outcome:review-admission-instituted",
		protocol.utteranceRef,
		values.captureRef,
		values.captureDigest,
		values.content.ref,
		values.content.digest,
		values.admissionRef,
		protocol.policyRef,
		values.policyDigest,
		canonical,
		values.endedAt,
	)
	return err
}

func insertSemanticReviewAdmissionRowV42(
	database *sql.DB,
	values semanticReviewSourceValuesV42,
) error {
	var policyRef string
	err := database.QueryRow(
		"SELECT context_policy_ref FROM speech_act_context_policies WHERE context_policy_digest = ?",
		values.policyDigest,
	).Scan(&policyRef)
	if err != nil {
		return err
	}
	canonical, err := authorityBasisJSON(map[string]any{
		"schema":                        "haft.spec-migration-v2.semantic-review-admission/v2",
		"admission_ref":                 values.admissionRef,
		"project_root":                  values.content.root,
		"packet_carrier_digest":         values.content.packetCarrierDigest,
		"review_content_ref":            values.content.ref,
		"review_content_digest":         values.content.digest,
		"review_text":                   values.reviewText,
		"review_digest":                 values.reviewDigest,
		"capture_carrier_ref":           values.captureRef,
		"capture_carrier_digest":        values.captureDigest,
		"speech_act_ref":                values.speechActRef,
		"speech_act_digest":             values.speechActDigest,
		"context_policy_ref":            policyRef,
		"context_policy_digest":         values.policyDigest,
		"act_type_ref":                  "speech-act-type:accept",
		"method_ref":                    semanticReviewMethodRefV42,
		"method_description_ref":        semanticReviewMethodDescRefV42,
		"method_description_digest":     values.methodDigest,
		"bounded_context_ref":           semanticReviewBoundedContextV42,
		"institutional_effect_rule_ref": values.effectRuleRef,
		"admitted_at":                   values.endedAt,
	})
	if err != nil {
		return err
	}
	_, err = database.Exec(`INSERT INTO migration_review_admissions_v2 (
		admission_ref, admission_digest, project_root, packet_carrier_digest,
		review_content_ref, review_content_digest, review_text, review_digest,
		capture_carrier_ref, capture_carrier_digest,
		speech_act_ref, speech_act_digest,
		context_policy_ref, context_policy_digest, act_type_ref,
		method_ref, method_description_ref, method_description_digest,
		bounded_context_ref, institutional_effect_rule_ref,
		admission_json, admitted_at, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		values.admissionRef,
		values.admissionDigest,
		values.content.root,
		values.content.packetCarrierDigest,
		values.content.ref,
		values.content.digest,
		values.reviewText,
		values.reviewDigest,
		values.captureRef,
		values.captureDigest,
		values.speechActRef,
		values.speechActDigest,
		policyRef,
		values.policyDigest,
		"speech-act-type:accept",
		semanticReviewMethodRefV42,
		semanticReviewMethodDescRefV42,
		values.methodDigest,
		semanticReviewBoundedContextV42,
		values.effectRuleRef,
		canonical,
		values.endedAt,
		values.endedAt,
	)
	return err
}

func openDatabaseBeforeMigration42(t testing.TB) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pre-v42.db")
	dsn, err := sqliteConnectionDSN(path)
	if err != nil {
		t.Fatalf("build pre-v42 DSN: %v", err)
	}
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open pre-v42 database: %v", err)
	}
	if _, err := database.Exec(schema); err != nil {
		_ = database.Close()
		t.Fatalf("install base schema: %v", err)
	}
	migrations := migrationsBeforeVersion(kernelMigrations, 42, 0, nil)
	if err := Migrate(database, "schema_version", migrations); err != nil {
		_ = database.Close()
		t.Fatalf("migrate through v41: %v", err)
	}
	return database
}
