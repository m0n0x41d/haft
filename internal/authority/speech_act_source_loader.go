package authority

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

type RecordedSpeechActSource struct {
	state *verifiedSpeechActSourceState
}

func (source RecordedSpeechActSource) Valid() bool {
	return VerifiedSpeechActSource(source).valid()
}

func (source RecordedSpeechActSource) SpeechActRef() (SpeechActRef, bool) {
	if !source.Valid() {
		return SpeechActRef{}, false
	}
	return source.state.speechAct.state.ref, true
}

func (source RecordedSpeechActSource) SpeechActDigest() (Digest, bool) {
	if !source.Valid() {
		return Digest{}, false
	}
	return source.state.speechAct.state.digest, true
}

func (source RecordedSpeechActSource) ProjectRoot() (ProjectRoot, bool) {
	if !source.Valid() {
		return ProjectRoot{}, false
	}
	return source.state.speechAct.state.projectRoot, true
}

func (source RecordedSpeechActSource) PreparedIntentDigest() (Digest, bool) {
	return VerifiedSpeechActSource(source).PreparedIntentDigest()
}

func (source RecordedSpeechActSource) WorkWindow() (TimeWindow, bool) {
	return VerifiedSpeechActSource(source).WorkWindow()
}

func (source RecordedSpeechActSource) CompletedAt() (time.Time, bool) {
	return VerifiedSpeechActSource(source).CompletedAt()
}

func (source RecordedSpeechActSource) PerformedByRoleAssignmentRef() (RoleAssignmentRef, bool) {
	return VerifiedSpeechActSource(source).PerformedByRoleAssignmentRef()
}

func (source RecordedSpeechActSource) PerformedByRoleAssignmentDigest() (Digest, bool) {
	return VerifiedSpeechActSource(source).PerformedByRoleAssignmentDigest()
}

func (source RecordedSpeechActSource) TerminalCaptureDigest() (Digest, bool) {
	if !source.Valid() {
		return Digest{}, false
	}
	return source.state.capture.state.carrierDigest, true
}

func (source RecordedSpeechActSource) TerminalCaptureRef() (CarrierRef, bool) {
	if !source.Valid() {
		return CarrierRef{}, false
	}
	return source.state.capture.state.carrierRef, true
}

func (source RecordedSpeechActSource) ReviewDigest() (Digest, bool) {
	if !source.Valid() {
		return Digest{}, false
	}
	return source.state.reviewDigest, true
}

func (source RecordedSpeechActSource) ReviewText() (string, bool) {
	if !source.Valid() {
		return "", false
	}
	return source.state.capture.state.reviewText, true
}

func (source RecordedSpeechActSource) ReviewSubjectRef() (SpeechActReviewSubjectRef, bool) {
	if !source.Valid() {
		return SpeechActReviewSubjectRef{}, false
	}
	return source.state.speechAct.state.reviewSubjectRef, true
}

func (source RecordedSpeechActSource) ReviewSubjectDigest() (Digest, bool) {
	if !source.Valid() {
		return Digest{}, false
	}
	return source.state.speechAct.state.reviewSubjectDigest, true
}

// InstitutedObjectRef returns the object identity named by the recorded act;
// the presence of this source still does not imply that its later domain
// effect committed successfully.
func (source RecordedSpeechActSource) InstitutedObjectRef() (InstitutedObjectRef, bool) {
	if !source.Valid() {
		return InstitutedObjectRef{}, false
	}
	return source.state.speechAct.state.institutedObjectRef, true
}

func (source RecordedSpeechActSource) ContextPolicyRef() (ContextPolicyRef, bool) {
	if !source.Valid() {
		return ContextPolicyRef{}, false
	}
	return source.state.intent.state.contextPolicy.state.ref, true
}

func (source RecordedSpeechActSource) ContextPolicyDigest() (Digest, bool) {
	if !source.Valid() {
		return Digest{}, false
	}
	return source.state.intent.state.contextPolicy.state.digest, true
}

func (source RecordedSpeechActSource) OccurredAt() (time.Time, bool) {
	if !source.Valid() {
		return time.Time{}, false
	}
	return source.state.capture.state.exactUtteranceObservedAt, true
}

type speechActSourceCanonicalRows struct {
	methodDigest     string
	methodJSON       string
	policyDigest     string
	policyJSON       string
	captureDigest    string
	captureJSON      string
	authorizerDigest string
	authorizerJSON   string
	speechActDigest  string
	speechActJSON    string
}

func (rows *speechActSourceCanonicalRows) scanTargets() []any {
	return []any{
		&rows.methodDigest,
		&rows.methodJSON,
		&rows.policyDigest,
		&rows.policyJSON,
		&rows.captureDigest,
		&rows.captureJSON,
		&rows.authorizerDigest,
		&rows.authorizerJSON,
		&rows.speechActDigest,
		&rows.speechActJSON,
	}
}

const loadSpeechActSourceSQL = `
SELECT
	method.method_description_digest,
	method.canonical_json,
	policy.context_policy_digest,
	policy.canonical_json,
	capture.capture_carrier_digest,
	capture.canonical_json,
	authorizer.role_assignment_digest,
	authorizer.canonical_json,
	act.speech_act_digest,
	act.canonical_json
FROM speech_acts act
JOIN speech_act_method_descriptions method
	ON method.method_description_ref = act.method_description_ref
	AND method.method_description_digest = act.method_description_digest
JOIN speech_act_context_policies policy
	ON policy.context_policy_ref = act.context_policy_ref
	AND policy.context_policy_digest = act.context_policy_digest
JOIN terminal_capture_records capture
	ON capture.capture_carrier_ref = act.capture_carrier_ref
	AND capture.capture_carrier_digest = act.capture_carrier_digest
JOIN speech_act_role_assignments authorizer
	ON authorizer.role_assignment_ref = act.performed_by_ref
	AND authorizer.role_assignment_digest = act.performed_by_digest
	AND authorizer.provenance_carrier_ref = capture.capture_carrier_ref
	AND authorizer.provenance_carrier_digest = capture.capture_carrier_digest
	AND authorizer.holder_system_ref = capture.observed_holder_system_ref
	AND authorizer.role_assignment_ref = capture.observed_role_assignment_ref
WHERE act.speech_act_ref = ?
AND act.speech_act_digest = ?`

func LoadRecordedSpeechActSource(
	ctx context.Context,
	database *sql.DB,
	ref SpeechActRef,
	digest Digest,
) (RecordedSpeechActSource, error) {
	if ctx == nil || database == nil || !ref.valid() || !digest.valid() {
		return RecordedSpeechActSource{}, fmt.Errorf("SpeechAct source load requires canonical arguments")
	}
	rows := speechActSourceCanonicalRows{}
	err := database.QueryRowContext(
		ctx,
		loadSpeechActSourceSQL,
		ref.String(),
		digest.String(),
	).Scan(rows.scanTargets()...)
	if err != nil {
		return RecordedSpeechActSource{}, fmt.Errorf("load SpeechAct source: %w", err)
	}
	return reconstructRecordedSpeechActSource(rows)
}

// ResolveRecordedSpeechActSource recovers one durable generic SpeechAct source
// from its canonical stable ref. The stored primary-key row supplies only the
// digest address; the existing strict ref+digest loader must still reconstruct
// and verify the complete canonical source graph.
func ResolveRecordedSpeechActSource(
	ctx context.Context,
	database *sql.DB,
	ref SpeechActRef,
) (RecordedSpeechActSource, bool, error) {
	if ctx == nil || database == nil || !ref.valid() {
		return RecordedSpeechActSource{}, false, fmt.Errorf(
			"SpeechAct source resolution requires canonical arguments",
		)
	}
	var rawDigest string
	err := database.QueryRowContext(
		ctx,
		"SELECT speech_act_digest FROM speech_acts WHERE speech_act_ref = ?",
		ref.String(),
	).Scan(&rawDigest)
	if errors.Is(err, sql.ErrNoRows) {
		return RecordedSpeechActSource{}, false, nil
	}
	if err != nil {
		return RecordedSpeechActSource{}, false, fmt.Errorf(
			"resolve SpeechAct source digest: %w",
			err,
		)
	}
	digest, err := NewDigest(rawDigest)
	if err != nil {
		return RecordedSpeechActSource{}, false, fmt.Errorf(
			"resolve SpeechAct source with malformed stored digest: %w",
			err,
		)
	}
	recorded, err := LoadRecordedSpeechActSource(ctx, database, ref, digest)
	if err != nil {
		return RecordedSpeechActSource{}, false, fmt.Errorf(
			"resolve strict SpeechAct source: %w",
			err,
		)
	}
	resolvedRef, ok := recorded.SpeechActRef()
	if !ok || resolvedRef != ref {
		return RecordedSpeechActSource{}, false, fmt.Errorf(
			"resolved SpeechAct source does not preserve its canonical ref",
		)
	}
	return recorded, true, nil
}

// LoadSpeechActContextPolicy resolves the context-policy member of a
// source-native authority basis independently from a SpeechAct aggregate. The
// stored canonical JSON is reparsed and rehashed before the policy is exposed.
func LoadSpeechActContextPolicy(
	ctx context.Context,
	database *sql.DB,
	ref ContextPolicyRef,
	digest Digest,
) (SpeechActContextPolicy, error) {
	if ctx == nil || database == nil || !ref.valid() || !digest.valid() {
		return SpeechActContextPolicy{}, fmt.Errorf(
			"SpeechAct context-policy load requires canonical arguments",
		)
	}
	row := struct {
		canonical string
		digest    string
	}{}
	err := database.QueryRowContext(
		ctx,
		`SELECT canonical_json, context_policy_digest
		 FROM speech_act_context_policies
		 WHERE context_policy_ref = ? AND context_policy_digest = ?`,
		ref.String(),
		digest.String(),
	).Scan(&row.canonical, &row.digest)
	if err != nil {
		return SpeechActContextPolicy{}, fmt.Errorf(
			"load SpeechAct context policy: %w",
			err,
		)
	}
	policy, err := parseSpeechActContextPolicy(row.canonical, row.digest)
	if err != nil {
		return SpeechActContextPolicy{}, err
	}
	loadedRef, refOK := policy.Ref()
	loadedDigest, digestOK := policy.Digest()
	exact := refOK && digestOK && loadedRef == ref && loadedDigest == digest
	if !exact {
		return SpeechActContextPolicy{}, fmt.Errorf(
			"loaded SpeechAct context policy does not preserve its exact identity",
		)
	}
	return policy, nil
}

func loadRecordedSpeechActSourceInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref SpeechActRef,
	digest Digest,
) (RecordedSpeechActSource, bool, error) {
	rows := speechActSourceCanonicalRows{}
	err := transaction.ScanOne(
		ctx,
		loadSpeechActSourceSQL,
		[]any{ref.String(), digest.String()},
		rows.scanTargets(),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RecordedSpeechActSource{}, false, nil
	}
	if err != nil {
		return RecordedSpeechActSource{}, false, fmt.Errorf("load SpeechAct source in transaction: %w", err)
	}
	recorded, err := reconstructRecordedSpeechActSource(rows)
	return recorded, true, err
}

// LoadRecordedSpeechActSourceInTransaction strictly reconstructs one source
// inside a caller-owned SQLite snapshot. It neither commits nor rolls back.
func LoadRecordedSpeechActSourceInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref SpeechActRef,
	digest Digest,
) (RecordedSpeechActSource, error) {
	if ctx == nil || transaction == nil || !ref.valid() || !digest.valid() {
		return RecordedSpeechActSource{}, fmt.Errorf(
			"transactional SpeechAct source load requires canonical arguments",
		)
	}
	if err := transaction.RequireActive(); err != nil {
		return RecordedSpeechActSource{}, err
	}
	recorded, found, err := loadRecordedSpeechActSourceInTransaction(
		ctx,
		transaction,
		ref,
		digest,
	)
	if err != nil {
		return RecordedSpeechActSource{}, err
	}
	if !found {
		return RecordedSpeechActSource{}, sql.ErrNoRows
	}
	loadedRef, refOK := recorded.SpeechActRef()
	loadedDigest, digestOK := recorded.SpeechActDigest()
	if !refOK || !digestOK || loadedRef != ref || loadedDigest != digest {
		return RecordedSpeechActSource{}, fmt.Errorf(
			"transactional SpeechAct source did not preserve its exact identity",
		)
	}
	return recorded, nil
}

// LoadSpeechActContextPolicyInTransaction strictly reconstructs one policy
// inside a caller-owned SQLite snapshot. It neither commits nor rolls back.
func LoadSpeechActContextPolicyInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref ContextPolicyRef,
	digest Digest,
) (SpeechActContextPolicy, error) {
	if ctx == nil || transaction == nil || !ref.valid() || !digest.valid() {
		return SpeechActContextPolicy{}, fmt.Errorf(
			"transactional SpeechAct context-policy load requires canonical arguments",
		)
	}
	if err := transaction.RequireActive(); err != nil {
		return SpeechActContextPolicy{}, err
	}
	row := struct {
		canonical string
		digest    string
	}{}
	err := transaction.ScanOne(
		ctx,
		`SELECT canonical_json, context_policy_digest
		 FROM speech_act_context_policies
		 WHERE context_policy_ref = ? AND context_policy_digest = ?`,
		[]any{ref.String(), digest.String()},
		[]any{&row.canonical, &row.digest},
	)
	if err != nil {
		return SpeechActContextPolicy{}, fmt.Errorf(
			"load SpeechAct context policy in transaction: %w",
			err,
		)
	}
	policy, err := parseSpeechActContextPolicy(row.canonical, row.digest)
	if err != nil {
		return SpeechActContextPolicy{}, err
	}
	loadedRef, refOK := policy.Ref()
	loadedDigest, digestOK := policy.Digest()
	if !refOK || !digestOK || loadedRef != ref || loadedDigest != digest {
		return SpeechActContextPolicy{}, fmt.Errorf(
			"transactional SpeechAct context policy did not preserve its exact identity",
		)
	}
	return policy, nil
}

type speechActMethodProjection struct {
	Schema             string `json:"schema"`
	MethodRef          string `json:"method_ref"`
	Ref                string `json:"method_description_ref"`
	ProcedureRef       string `json:"procedure_ref"`
	BoundedContext     string `json:"bounded_context_ref"`
	ProcedureSemantics string `json:"procedure_semantics"`
}

type speechActPolicyProjection struct {
	Schema               string `json:"schema"`
	Ref                  string `json:"ref"`
	BoundedContext       string `json:"bounded_context_ref"`
	RecognizedActType    string `json:"recognized_act_type_ref"`
	EffectRule           string `json:"institutional_effect_rule_ref"`
	InstitutedKind       string `json:"instituted_object_kind"`
	Modality             string `json:"institutional_modality"`
	ScopedAction         string `json:"scoped_action"`
	UtteranceVerb        string `json:"utterance_verb"`
	UtteranceBinding     string `json:"utterance_binding"`
	UtteranceLiteral     string `json:"utterance_literal,omitempty"`
	UtteranceDescription string `json:"utterance_description_ref"`
}

type terminalCaptureProjection struct {
	CarrierRef               string `json:"carrier_ref"`
	ReviewDigest             string `json:"review_digest"`
	ReviewText               string `json:"review_text"`
	PreparedIntentDig        string `json:"prepared_speech_act_intent_digest"`
	CanonicalUtterance       string `json:"canonical_utterance"`
	StartedAt                string `json:"started_at"`
	ExactUtteranceObservedAt string `json:"exact_utterance_observed_at"`
	EndedAt                  string `json:"ended_at"`
	ProjectRoot              string `json:"project_root"`
	SessionRef               string `json:"session_ref"`
	ObservedMaterial         string `json:"observed_session_material"`
	ObservationNonce         string `json:"observation_nonce"`
	ObservationDigest        string `json:"observation_digest"`
	ObservedHolderRef        string `json:"observed_holder_system_ref"`
	ObservedRoleRef          string `json:"observed_role_assignment_ref"`
}

func reconstructRecordedSpeechActSource(
	rows speechActSourceCanonicalRows,
) (RecordedSpeechActSource, error) {
	method, err := parseSpeechActMethodDescription(rows.methodJSON, rows.methodDigest)
	if err != nil {
		return RecordedSpeechActSource{}, err
	}
	policy, err := parseSpeechActContextPolicy(rows.policyJSON, rows.policyDigest)
	if err != nil {
		return RecordedSpeechActSource{}, err
	}
	captureProjection := terminalCaptureProjection{}
	if err := json.Unmarshal([]byte(rows.captureJSON), &captureProjection); err != nil {
		return RecordedSpeechActSource{}, fmt.Errorf("decode terminal capture: %w", err)
	}
	speechProjection := authoritySpeechActProjection{}
	if err := json.Unmarshal([]byte(rows.speechActJSON), &speechProjection); err != nil {
		return RecordedSpeechActSource{}, fmt.Errorf("decode SpeechAct: %w", err)
	}
	source, err := rebuildSpeechActSource(
		method,
		policy,
		captureProjection,
		speechProjection,
	)
	if err != nil {
		return RecordedSpeechActSource{}, err
	}
	state := source.state
	exact := rows.methodDigest == state.intent.state.executionFrame.state.methodDescriptionDigest.String() &&
		rows.policyDigest == state.intent.state.contextPolicy.state.digest.String() &&
		rows.captureDigest == state.capture.state.carrierDigest.String() &&
		rows.authorizerDigest == state.authorizer.state.digest.String() &&
		rows.speechActDigest == state.speechAct.state.digest.String() &&
		rows.methodJSON == string(state.intent.state.executionFrame.state.methodDescription.state.canonicalJSON) &&
		rows.policyJSON == string(state.intent.state.contextPolicy.state.canonicalJSON) &&
		rows.captureJSON == string(state.capture.state.canonicalJSON) &&
		rows.authorizerJSON == string(state.authorizer.state.canonicalJSON) &&
		rows.speechActJSON == string(state.speechAct.state.canonicalJSON)
	if !exact {
		return RecordedSpeechActSource{}, fmt.Errorf("durable SpeechAct source is not exact canonical material")
	}
	return RecordedSpeechActSource{state: state}, nil
}

func parseSpeechActMethodDescription(
	rawJSON string,
	rawDigest string,
) (SpeechActMethodDescription, error) {
	projection := speechActMethodProjection{}
	if err := json.Unmarshal([]byte(rawJSON), &projection); err != nil {
		return SpeechActMethodDescription{}, fmt.Errorf("decode SpeechAct MethodDescription: %w", err)
	}
	methodRef, err := NewMethodRef(projection.MethodRef)
	if err != nil {
		return SpeechActMethodDescription{}, err
	}
	descriptionRef, err := NewMethodDescriptionRef(projection.Ref)
	if err != nil {
		return SpeechActMethodDescription{}, err
	}
	procedureRef, err := NewMethodProcedureRef(projection.ProcedureRef)
	if err != nil {
		return SpeechActMethodDescription{}, err
	}
	boundedContext, err := NewBoundedContextRef(projection.BoundedContext)
	if err != nil {
		return SpeechActMethodDescription{}, err
	}
	description, err := NewSpeechActMethodDescription(
		methodRef,
		descriptionRef,
		procedureRef,
		boundedContext,
		projection.ProcedureSemantics,
	)
	if err != nil {
		return SpeechActMethodDescription{}, err
	}
	if description.state.digest.String() != rawDigest ||
		!slices.Equal(description.state.canonicalJSON, []byte(rawJSON)) {
		return SpeechActMethodDescription{}, fmt.Errorf("SpeechAct MethodDescription digest or canonical JSON is invalid")
	}
	return description, nil
}

func parseSpeechActContextPolicy(
	rawJSON string,
	rawDigest string,
) (SpeechActContextPolicy, error) {
	projection := speechActPolicyProjection{}
	if err := json.Unmarshal([]byte(rawJSON), &projection); err != nil {
		return SpeechActContextPolicy{}, fmt.Errorf("decode SpeechAct context policy: %w", err)
	}
	utteranceRule := SpeechActUtteranceRule{
		verb:    projection.UtteranceVerb,
		binding: speechActUtteranceBinding(projection.UtteranceBinding),
		literal: projection.UtteranceLiteral,
	}
	effectRuleRef, err := NewInstitutionalEffectRuleRef(projection.EffectRule)
	if err != nil {
		return SpeechActContextPolicy{}, err
	}
	institutedKind, err := NewInstitutedObjectKind(projection.InstitutedKind)
	if err != nil {
		return SpeechActContextPolicy{}, err
	}
	modality, err := NewInstitutionalModality(projection.Modality)
	if err != nil {
		return SpeechActContextPolicy{}, err
	}
	scopedAction, err := NewActionKind(projection.ScopedAction)
	if err != nil {
		return SpeechActContextPolicy{}, err
	}
	utteranceDescription, err := NewUtteranceRef(projection.UtteranceDescription)
	if err != nil {
		return SpeechActContextPolicy{}, err
	}
	effectRule, err := NewInstitutionalEffectRule(
		effectRuleRef,
		institutedKind,
		modality,
		scopedAction,
		utteranceRule,
		utteranceDescription,
	)
	if err != nil {
		return SpeechActContextPolicy{}, err
	}
	policyRef, err := NewContextPolicyRef(projection.Ref)
	if err != nil {
		return SpeechActContextPolicy{}, err
	}
	boundedContext, err := NewBoundedContextRef(projection.BoundedContext)
	if err != nil {
		return SpeechActContextPolicy{}, err
	}
	recognizedActType, err := NewSpeechActTypeRef(projection.RecognizedActType)
	if err != nil {
		return SpeechActContextPolicy{}, err
	}
	policy, err := NewSpeechActContextPolicy(
		policyRef,
		boundedContext,
		recognizedActType,
		effectRule,
	)
	if err != nil {
		return SpeechActContextPolicy{}, err
	}
	if policy.state.digest.String() != rawDigest ||
		!slices.Equal(policy.state.canonicalJSON, []byte(rawJSON)) {
		return SpeechActContextPolicy{}, fmt.Errorf("SpeechAct context policy digest or canonical JSON is invalid")
	}
	return policy, nil
}

func rebuildSpeechActSource(
	method SpeechActMethodDescription,
	policy SpeechActContextPolicy,
	capture terminalCaptureProjection,
	speech authoritySpeechActProjection,
) (VerifiedSpeechActSource, error) {
	frame, err := rebuildSpeechActExecutionFrame(method, speech)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	speechActRef, err := NewSpeechActRef(speech.Ref)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	captureRef, err := NewCarrierRef(capture.CarrierRef)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	projectRoot, err := NewProjectRoot(speech.ProjectRoot)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	sessionRef, err := NewSessionRef(capture.SessionRef)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	reviewSubjectRef, err := NewSpeechActReviewSubjectRef(speech.ReviewSubjectRef)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	reviewSubjectDigest, err := NewDigest(speech.ReviewSubjectDigest)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	institutedObjectRef, err := NewInstitutedObjectRef(speech.InstitutedObjectRef)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	intent, err := NewPreparedSpeechActIntentBuilder(
		speechActRef,
		captureRef,
	).
		ForProject(projectRoot).
		InSession(sessionRef).
		Reviewing(reviewSubjectRef, reviewSubjectDigest).
		Institutes(institutedObjectRef).
		UnderContextPolicy(policy).
		WithExecutionFrame(frame).
		Build()
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	if intent.state.digest.String() != capture.PreparedIntentDig {
		return VerifiedSpeechActSource{}, fmt.Errorf("terminal capture does not bind the reconstructed prepared SpeechAct intent")
	}
	startedAt, err := parseAuthorityTime(capture.StartedAt)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	exactUtteranceObservedAt, err := parseAuthorityTime(capture.ExactUtteranceObservedAt)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	endedAt, err := parseAuthorityTime(capture.EndedAt)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	observation, err := newTerminalSessionObservation(
		capture.ObservedMaterial,
		capture.ObservationNonce,
		exactUtteranceObservedAt,
	)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	observationExact := observation.digest.String() == capture.ObservationDigest &&
		observation.holderSystemRef.String() == capture.ObservedHolderRef &&
		observation.assignmentRef.String() == capture.ObservedRoleRef
	if !observationExact {
		return VerifiedSpeechActSource{}, fmt.Errorf("terminal observation derivation does not match capture")
	}
	reviewDigest, err := NewDigest(capture.ReviewDigest)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	return newVerifiedSpeechActSource(
		intent,
		capture.ReviewText,
		reviewDigest,
		capture.CanonicalUtterance,
		startedAt,
		exactUtteranceObservedAt,
		endedAt,
		observation,
	)
}

func rebuildSpeechActExecutionFrame(
	method SpeechActMethodDescription,
	projection authoritySpeechActProjection,
) (SpeechActExecutionFrame, error) {
	executedWithin, err := NewSystemRef(projection.ExecutedWithinRef)
	if err != nil {
		return SpeechActExecutionFrame{}, err
	}
	statePlane, err := NewStatePlaneRef(projection.StatePlaneRef)
	if err != nil {
		return SpeechActExecutionFrame{}, err
	}
	deltaPredicate, err := NewDeltaPredicateRef(projection.DeltaPredicateRef)
	if err != nil {
		return SpeechActExecutionFrame{}, err
	}
	outcome, err := NewWorkOutcomeRef(projection.OutcomeRef)
	if err != nil {
		return SpeechActExecutionFrame{}, err
	}
	utterance, err := NewUtteranceRef(projection.UtteranceRef)
	if err != nil {
		return SpeechActExecutionFrame{}, err
	}
	builder := NewSpeechActExecutionFrameBuilder(method)
	builder = builder.ExecutedWithin(executedWithin)
	builder = builder.OnStatePlane(statePlane, deltaPredicate)
	builder = builder.WithOutcome(outcome)
	builder = builder.WithUtteranceDescription(utterance)
	builder, err = addLoadedWorkParameters(builder, projection.Parameters, 0)
	if err != nil {
		return SpeechActExecutionFrame{}, err
	}
	builder, err = addLoadedWorkResources(builder, projection.Resources, 0)
	if err != nil {
		return SpeechActExecutionFrame{}, err
	}
	builder, err = addLoadedAffectedRefs(builder, projection.AffectedRefs, 0)
	if err != nil {
		return SpeechActExecutionFrame{}, err
	}
	return builder.Build()
}

func addLoadedWorkParameters(
	builder SpeechActExecutionFrameBuilder,
	values []workParameterProjection,
	index int,
) (SpeechActExecutionFrameBuilder, error) {
	if index >= len(values) {
		return builder, nil
	}
	binding, err := NewWorkParameterBinding(values[index].Name, values[index].Value)
	if err != nil {
		return SpeechActExecutionFrameBuilder{}, err
	}
	next := builder.BindParameter(binding)
	return addLoadedWorkParameters(next, values, index+1)
}

func addLoadedWorkResources(
	builder SpeechActExecutionFrameBuilder,
	values []string,
	index int,
) (SpeechActExecutionFrameBuilder, error) {
	if index >= len(values) {
		return builder, nil
	}
	resource, err := NewWorkResourceRef(values[index])
	if err != nil {
		return SpeechActExecutionFrameBuilder{}, err
	}
	next := builder.UseResource(resource)
	return addLoadedWorkResources(next, values, index+1)
}

func addLoadedAffectedRefs(
	builder SpeechActExecutionFrameBuilder,
	values []string,
	index int,
) (SpeechActExecutionFrameBuilder, error) {
	if index >= len(values) {
		return builder, nil
	}
	affected, err := NewAffectedRef(values[index])
	if err != nil {
		return SpeechActExecutionFrameBuilder{}, err
	}
	next := builder.Affect(affected)
	return addLoadedAffectedRefs(next, values, index+1)
}
