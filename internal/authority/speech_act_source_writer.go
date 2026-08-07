package authority

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

type SpeechActSourceWriteKind string

const (
	SpeechActSourceWriteStaged      SpeechActSourceWriteKind = "staged"
	SpeechActSourceWriteExactReplay SpeechActSourceWriteKind = "exact_replay"
	SpeechActSourceWriteRejected    SpeechActSourceWriteKind = "rejected"
)

type SpeechActSourceWriteResult struct {
	kind     SpeechActSourceWriteKind
	recorded RecordedSpeechActSource
	detail   string
}

func (result SpeechActSourceWriteResult) Kind() SpeechActSourceWriteKind {
	return result.kind
}

func (result SpeechActSourceWriteResult) RecordedSource() (RecordedSpeechActSource, bool) {
	if result.kind != SpeechActSourceWriteStaged && result.kind != SpeechActSourceWriteExactReplay {
		return RecordedSpeechActSource{}, false
	}
	return result.recorded, result.recorded.Valid()
}

func (result SpeechActSourceWriteResult) RejectionDetail() (string, bool) {
	return result.detail, result.kind == SpeechActSourceWriteRejected && result.detail != ""
}

type SpeechActSourceWriter struct {
	database *sql.DB
	now      func() time.Time
}

func OpenSpeechActSourceWriter(database *sql.DB) (*SpeechActSourceWriter, error) {
	if database == nil {
		return nil, fmt.Errorf("SpeechAct source writer requires a database")
	}
	var count int
	err := database.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = 38",
	).Scan(&count)
	if err != nil || count != 1 {
		return nil, errors.Join(fmt.Errorf("authority source migration 38 is unavailable"), err)
	}
	err = database.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = 40",
	).Scan(&count)
	if err != nil || count != 1 {
		return nil, errors.Join(
			fmt.Errorf("semantic SpeechAct utterance migration 40 is unavailable; migrate or rebuild the development ledger before writing manual SpeechActs"),
			err,
		)
	}
	err = database.QueryRow(`SELECT COUNT(*)
		FROM pragma_table_info('speech_act_context_policies')
		WHERE name = 'utterance_literal'`,
	).Scan(&count)
	if err != nil || count != 1 {
		return nil, errors.Join(
			fmt.Errorf("semantic SpeechAct utterance schema is stale; migrate or rebuild the development ledger before writing manual SpeechActs"),
			err,
		)
	}
	err = database.QueryRow(`SELECT COUNT(*)
		FROM pragma_table_info('terminal_capture_records')
		WHERE name IN ('started_at', 'exact_utterance_observed_at', 'ended_at')`,
	).Scan(&count)
	if err != nil || count != 3 {
		return nil, errors.Join(
			fmt.Errorf("authority source migration 38 has a stale terminal-capture schema; rebuild the fresh development ledger"),
			err,
		)
	}
	return &SpeechActSourceWriter{database: database, now: time.Now}, nil
}

// Record durably closes one already-performed generic SpeechAct source before
// any domain-specific instituted effect is attempted. An exact retry commits
// its read-only transaction and returns the same canonical durable source; it
// never recreates the terminal capture or SpeechAct rows.
func (writer *SpeechActSourceWriter) Record(
	ctx context.Context,
	source VerifiedSpeechActSource,
) (RecordedSpeechActSource, error) {
	if writer == nil || writer.database == nil || writer.now == nil || ctx == nil {
		return RecordedSpeechActSource{}, fmt.Errorf(
			"SpeechAct source write requires an open writer and context",
		)
	}
	if err := ctx.Err(); err != nil {
		return RecordedSpeechActSource{}, err
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, writer.database)
	if err != nil {
		return RecordedSpeechActSource{}, fmt.Errorf("begin SpeechAct source transaction: %w", err)
	}
	result, err := writer.RecordInTransaction(ctx, transaction, source)
	if err != nil {
		finish := transaction.Rollback(context.Background())
		return RecordedSpeechActSource{}, errors.Join(err, finish.Err())
	}
	if result.kind == SpeechActSourceWriteRejected {
		finish := transaction.Rollback(context.Background())
		detail, _ := result.RejectionDetail()
		return RecordedSpeechActSource{}, errors.Join(
			fmt.Errorf("SpeechAct source rejected: %s", detail),
			finish.Err(),
		)
	}
	recorded, ok := result.RecordedSource()
	if !ok {
		finish := transaction.Rollback(context.Background())
		return RecordedSpeechActSource{}, errors.Join(
			fmt.Errorf("SpeechAct source writer returned no canonical source"),
			finish.Err(),
		)
	}
	ref, refOK := recorded.SpeechActRef()
	digest, digestOK := recorded.SpeechActDigest()
	if !refOK || !digestOK {
		finish := transaction.Rollback(context.Background())
		return RecordedSpeechActSource{}, errors.Join(
			fmt.Errorf("SpeechAct source writer returned no canonical identity"),
			finish.Err(),
		)
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		durable, loadErr := LoadRecordedSpeechActSource(
			context.Background(),
			writer.database,
			ref,
			digest,
		)
		if loadErr == nil && durable.Valid() {
			return durable, nil
		}
		return RecordedSpeechActSource{}, errors.Join(
			fmt.Errorf("SpeechAct source commit outcome is unknown"),
			finish.Err(),
			loadErr,
		)
	}
	durable, err := LoadRecordedSpeechActSource(
		context.Background(),
		writer.database,
		ref,
		digest,
	)
	if err != nil {
		return RecordedSpeechActSource{}, fmt.Errorf(
			"committed SpeechAct source failed exact durable reread: %w",
			err,
		)
	}
	return durable, nil
}

func (writer *SpeechActSourceWriter) RecordInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	source VerifiedSpeechActSource,
) (SpeechActSourceWriteResult, error) {
	if writer == nil || writer.database == nil || writer.now == nil || ctx == nil || transaction == nil {
		return SpeechActSourceWriteResult{}, fmt.Errorf("SpeechAct source write requires an open writer, context, and transaction")
	}
	if err := transaction.RequireImmediate(); err != nil {
		return SpeechActSourceWriteResult{}, err
	}
	if !source.valid() {
		return SpeechActSourceWriteResult{
			kind:   SpeechActSourceWriteRejected,
			detail: "SpeechAct source is not package-verified canonical material",
		}, nil
	}
	act := source.state.speechAct.state
	existing, found, err := loadRecordedSpeechActSourceInTransaction(
		ctx,
		transaction,
		act.ref,
		act.digest,
	)
	if err != nil {
		return SpeechActSourceWriteResult{}, err
	}
	if found {
		return SpeechActSourceWriteResult{
			kind:     SpeechActSourceWriteExactReplay,
			recorded: existing,
		}, nil
	}
	recordedAt := canonicalAuthorityTime(writer.now())
	if err := insertSpeechActSourceRows(ctx, transaction, source, recordedAt); err != nil {
		var collision canonicalSourceIdentityCollision
		if errors.As(err, &collision) {
			return SpeechActSourceWriteResult{
				kind:   SpeechActSourceWriteRejected,
				detail: collision.Error(),
			}, nil
		}
		return SpeechActSourceWriteResult{}, err
	}
	recorded, found, err := loadRecordedSpeechActSourceInTransaction(
		ctx,
		transaction,
		act.ref,
		act.digest,
	)
	if err != nil {
		return SpeechActSourceWriteResult{}, err
	}
	if !found || !recorded.Valid() {
		return SpeechActSourceWriteResult{}, fmt.Errorf("staged SpeechAct source failed strict exact reread")
	}
	return SpeechActSourceWriteResult{
		kind:     SpeechActSourceWriteStaged,
		recorded: recorded,
	}, nil
}

type canonicalSourceIdentityCollision struct{ detail string }

func (collision canonicalSourceIdentityCollision) Error() string { return collision.detail }

func insertSpeechActSourceRows(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	source VerifiedSpeechActSource,
	recordedAt time.Time,
) error {
	state := source.state
	method := state.intent.state.executionFrame.state.methodDescription.state
	if err := insertSpeechActMethodDescription(ctx, transaction, method, recordedAt); err != nil {
		return err
	}
	policy := state.intent.state.contextPolicy.state
	if err := insertSpeechActContextPolicy(ctx, transaction, policy, recordedAt); err != nil {
		return err
	}
	if err := insertTerminalCapture(ctx, transaction, state.capture.state, recordedAt); err != nil {
		return err
	}
	if err := insertSpeechActRoleAssignment(ctx, transaction, state.authorizer.state, recordedAt); err != nil {
		return err
	}
	return insertSpeechAct(ctx, transaction, state, recordedAt)
}

func insertSpeechActMethodDescription(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	state *speechActMethodDescriptionState,
	recordedAt time.Time,
) error {
	statement := `INSERT INTO speech_act_method_descriptions (
		method_description_ref, method_description_digest, method_ref,
		procedure_ref, bounded_context_ref, procedure_semantics,
		canonical_json, recorded_at
	) SELECT ?, ?, ?, ?, ?, ?, ?, ?
	WHERE NOT EXISTS (
		SELECT 1 FROM speech_act_method_descriptions WHERE method_description_ref = ?
	)`
	arguments := []any{
		state.ref.String(),
		state.digest.String(),
		state.methodRef.String(),
		state.procedureRef.String(),
		state.boundedContext.String(),
		state.procedureSemantics,
		string(state.canonicalJSON),
		formatAuthorityTime(recordedAt),
		state.ref.String(),
	}
	if _, err := transaction.Execute(ctx, statement, arguments); err != nil {
		return fmt.Errorf("insert SpeechAct MethodDescription: %w", err)
	}
	return verifyStoredCanonicalNode(
		ctx,
		transaction,
		"speech_act_method_descriptions",
		"method_description_ref",
		"method_description_digest",
		state.ref.String(),
		state.digest.String(),
		string(state.canonicalJSON),
	)
}

func insertSpeechActContextPolicy(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	state *speechActContextPolicyState,
	recordedAt time.Time,
) error {
	rule := state.effectRule
	statement := `INSERT INTO speech_act_context_policies (
		context_policy_ref, context_policy_digest, bounded_context_ref,
		recognized_act_type_ref, authorizer_role_ref, admitted_holder_kind,
		assignment_source_rule, institutional_effect_rule_ref,
		instituted_object_kind, institutional_modality, scoped_action,
		utterance_verb, utterance_binding, utterance_literal,
		utterance_description_ref,
		canonical_json, recorded_at
	) SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
	WHERE NOT EXISTS (
		SELECT 1 FROM speech_act_context_policies WHERE context_policy_ref = ?
	)`
	arguments := []any{
		state.ref.String(),
		state.digest.String(),
		state.boundedContext.String(),
		state.recognizedActType.String(),
		terminalAuthorizerRoleRefValue,
		admittedHolderSystemKindValue,
		"observed-local-controlling-terminal-session/v1",
		rule.ref.String(),
		rule.institutedObjectKind.String(),
		rule.modality.String(),
		rule.scopedAction.String(),
		rule.utteranceRule.verb,
		string(rule.utteranceRule.binding),
		rule.utteranceRule.literal,
		rule.utteranceDescription.String(),
		string(state.canonicalJSON),
		formatAuthorityTime(recordedAt),
		state.ref.String(),
	}
	if _, err := transaction.Execute(ctx, statement, arguments); err != nil {
		return fmt.Errorf("insert SpeechAct context policy: %w", err)
	}
	return verifyStoredCanonicalNode(
		ctx,
		transaction,
		"speech_act_context_policies",
		"context_policy_ref",
		"context_policy_digest",
		state.ref.String(),
		state.digest.String(),
		string(state.canonicalJSON),
	)
}

func insertTerminalCapture(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	state *terminalCaptureRecordState,
	recordedAt time.Time,
) error {
	observation := state.observation
	statement := `INSERT INTO terminal_capture_records (
		capture_carrier_ref, capture_carrier_digest, project_root,
		prepared_speech_act_intent_digest, review_text, review_digest,
		canonical_utterance, started_at, exact_utterance_observed_at, ended_at,
		intent_session_ref,
		observed_session_material, observation_nonce, observation_digest,
		observed_holder_system_ref, observed_role_assignment_ref,
		canonical_json, recorded_at
	) SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
	WHERE NOT EXISTS (
		SELECT 1 FROM terminal_capture_records WHERE capture_carrier_ref = ?
	)`
	arguments := []any{
		state.carrierRef.String(),
		state.carrierDigest.String(),
		state.projectRoot.String(),
		state.preparedIntentDig.String(),
		state.reviewText,
		state.reviewDigest.String(),
		state.canonicalUtterance,
		formatAuthorityTime(state.startedAt),
		formatAuthorityTime(state.exactUtteranceObservedAt),
		formatAuthorityTime(state.endedAt),
		state.sessionRef.String(),
		observation.material,
		observation.nonce,
		observation.digest.String(),
		observation.holderSystemRef.String(),
		observation.assignmentRef.String(),
		string(state.canonicalJSON),
		formatAuthorityTime(recordedAt),
		state.carrierRef.String(),
	}
	if _, err := transaction.Execute(ctx, statement, arguments); err != nil {
		return fmt.Errorf("insert terminal capture: %w", err)
	}
	return verifyStoredCanonicalNode(
		ctx,
		transaction,
		"terminal_capture_records",
		"capture_carrier_ref",
		"capture_carrier_digest",
		state.carrierRef.String(),
		state.carrierDigest.String(),
		string(state.canonicalJSON),
	)
}

func insertSpeechActRoleAssignment(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	state *authorityRoleAssignmentState,
	recordedAt time.Time,
) error {
	statement := `INSERT INTO speech_act_role_assignments (
		role_assignment_ref, role_assignment_digest, project_root,
		holder_system_ref, admitted_holder_kind, role_ref,
		bounded_context_ref, valid_from, valid_until,
		context_policy_ref, context_policy_digest,
		provenance_carrier_ref, provenance_carrier_digest,
		identity_boundary, canonical_json, recorded_at
	) SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
	WHERE NOT EXISTS (
		SELECT 1 FROM speech_act_role_assignments WHERE role_assignment_ref = ?
	)`
	arguments := []any{
		state.ref.String(),
		state.digest.String(),
		state.projectRoot.String(),
		state.holderSystemRef.String(),
		state.admittedHolderKind,
		state.roleRef,
		state.boundedContextRef.String(),
		formatAuthorityTime(state.assignmentWindow.from),
		formatAuthorityTime(state.assignmentWindow.until),
		state.justificationSourceRef.String(),
		state.justificationSourceDigest.String(),
		state.provenanceCarrierRef.String(),
		state.provenanceCarrierDigest.String(),
		authorityIssueIdentityBoundary,
		string(state.canonicalJSON),
		formatAuthorityTime(recordedAt),
		state.ref.String(),
	}
	if _, err := transaction.Execute(ctx, statement, arguments); err != nil {
		return fmt.Errorf("insert SpeechAct RoleAssignment: %w", err)
	}
	return verifyStoredCanonicalNode(
		ctx,
		transaction,
		"speech_act_role_assignments",
		"role_assignment_ref",
		"role_assignment_digest",
		state.ref.String(),
		state.digest.String(),
		string(state.canonicalJSON),
	)
}

func insertSpeechAct(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	state *verifiedSpeechActSourceState,
	recordedAt time.Time,
) error {
	act := state.speechAct.state
	policy := state.intent.state.contextPolicy.state
	parameters, err := json.Marshal(projectWorkParameters(act.parameters))
	if err != nil {
		return err
	}
	inputs, err := json.Marshal(act.inputRefs)
	if err != nil {
		return err
	}
	outputs, err := json.Marshal(act.outputRefs)
	if err != nil {
		return err
	}
	resources, err := json.Marshal(projectWorkResources(act.resources))
	if err != nil {
		return err
	}
	affected, err := json.Marshal(projectAffectedRefs(act.affected))
	if err != nil {
		return err
	}
	statement := `INSERT INTO speech_acts (
		speech_act_ref, speech_act_digest, project_root, work_kind, act_type_ref,
		performed_by_ref, performed_by_digest, method_ref,
		method_description_ref, method_description_digest, executed_within_ref,
		bounded_context_ref, window_from, window_until, parameters_json,
		input_refs_json, output_refs_json, resource_refs_json, affected_refs_json,
		state_plane_ref, delta_predicate_ref, outcome_ref, utterance_ref,
		capture_carrier_ref, capture_carrier_digest,
		review_subject_ref, review_subject_digest, instituted_object_ref,
		context_policy_ref, context_policy_digest, canonical_json, recorded_at
	) SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
	WHERE NOT EXISTS (SELECT 1 FROM speech_acts WHERE speech_act_ref = ?)`
	arguments := []any{
		act.ref.String(), act.digest.String(), act.projectRoot.String(), act.workKind,
		act.actType.String(), act.performedByRef.String(), act.performedByDigest.String(),
		act.methodRef.String(), act.methodDescriptionRef.String(),
		act.methodDescriptionDigest.String(), act.executedWithin.String(),
		act.boundedContext.String(), formatAuthorityTime(act.window.from),
		formatAuthorityTime(act.window.until), string(parameters), string(inputs),
		string(outputs), string(resources), string(affected), act.statePlane.String(),
		act.deltaPredicate.String(), act.outcome.String(), act.utteranceRef.String(),
		act.captureCarrierRef.String(), act.captureCarrierDigest.String(),
		act.reviewSubjectRef.String(), act.reviewSubjectDigest.String(),
		act.institutedObjectRef.String(), policy.ref.String(), policy.digest.String(),
		string(act.canonicalJSON), formatAuthorityTime(recordedAt), act.ref.String(),
	}
	if _, err := transaction.Execute(ctx, statement, arguments); err != nil {
		return fmt.Errorf("insert SpeechAct: %w", err)
	}
	return verifyStoredCanonicalNode(
		ctx,
		transaction,
		"speech_acts",
		"speech_act_ref",
		"speech_act_digest",
		act.ref.String(),
		act.digest.String(),
		string(act.canonicalJSON),
	)
}

func verifyStoredCanonicalNode(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	table string,
	keyColumn string,
	digestColumn string,
	key string,
	digest string,
	canonicalJSON string,
) error {
	statement := "SELECT " + digestColumn + ", canonical_json FROM " + table + " WHERE " + keyColumn + " = ? OR " + digestColumn + " = ?"
	var storedDigest string
	var storedJSON string
	err := transaction.ScanOne(
		ctx,
		statement,
		[]any{key, digest},
		[]any{&storedDigest, &storedJSON},
	)
	if err != nil {
		return fmt.Errorf("reread %s canonical node: %w", table, err)
	}
	if storedDigest != digest || storedJSON != canonicalJSON {
		return canonicalSourceIdentityCollision{
			detail: table + " identity binds different canonical material",
		}
	}
	return nil
}
