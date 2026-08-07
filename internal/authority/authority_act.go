package authority

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

const (
	authorityTerminalCaptureDigestDomain = "haft.authority.terminal-capture/v1"
	authorityRoleAssignmentDigestDomain  = "haft.authority.role-assignment/v1"
	authoritySpeechActDigestDomain       = "haft.authority.speech-act/v1"
	authorityPermissionDigestDomain      = "haft.authority.profile-declaration-permission/v1"
	institutedEffectDigestDomain         = "haft.authority.instituted-effect/v1"
	terminalAuthorizerRoleRefValue       = "role:project-principal-authorizer"
	communicativeWorkKindValue           = "Communicative"
	admittedHolderSystemKindValue        = "U.System"
)

type TerminalCaptureRecord struct {
	state *terminalCaptureRecordState
}

type terminalCaptureRecordState struct {
	carrierRef               CarrierRef
	carrierDigest            Digest
	reviewDigest             Digest
	reviewText               string
	preparedIntentDig        Digest
	canonicalUtterance       string
	startedAt                time.Time
	exactUtteranceObservedAt time.Time
	endedAt                  time.Time
	projectRoot              ProjectRoot
	sessionRef               SessionRef
	observation              terminalSessionObservation
	canonicalJSON            []byte
}

type AuthorityRoleAssignment struct {
	state *authorityRoleAssignmentState
}

type authorityRoleAssignmentState struct {
	ref                       RoleAssignmentRef
	digest                    Digest
	projectRoot               ProjectRoot
	holderSystemRef           SystemRef
	admittedHolderKind        string
	roleRef                   string
	boundedContextRef         BoundedContextRef
	assignmentWindow          TimeWindow
	justificationSourceRef    ContextPolicyRef
	justificationSourceDigest Digest
	provenanceCarrierRef      CarrierRef
	provenanceCarrierDigest   Digest
	canonicalJSON             []byte
}

type AuthoritySpeechAct struct {
	state *authoritySpeechActState
}

type authoritySpeechActState struct {
	ref                     SpeechActRef
	digest                  Digest
	projectRoot             ProjectRoot
	workKind                string
	actType                 SpeechActTypeRef
	performedByRef          RoleAssignmentRef
	performedByDigest       Digest
	methodRef               MethodRef
	methodDescriptionRef    MethodDescriptionRef
	methodDescriptionDigest Digest
	executedWithin          SystemRef
	boundedContext          BoundedContextRef
	window                  TimeWindow
	parameters              []WorkParameterBinding
	inputRefs               []string
	outputRefs              []string
	resources               []WorkResourceRef
	affected                []AffectedRef
	statePlane              StatePlaneRef
	deltaPredicate          DeltaPredicateRef
	outcome                 WorkOutcomeRef
	utteranceRef            UtteranceRef
	captureCarrierRef       CarrierRef
	captureCarrierDigest    Digest
	reviewSubjectRef        SpeechActReviewSubjectRef
	reviewSubjectDigest     Digest
	institutedObjectRef     InstitutedObjectRef
	canonicalJSON           []byte
}

type verifiedSpeechActSourceState struct {
	intent       PreparedSpeechActIntent
	capture      TerminalCaptureRecord
	authorizer   AuthorityRoleAssignment
	speechAct    AuthoritySpeechAct
	reviewDigest Digest
}

// VerifiedSpeechActSource is the generic immutable result of a real terminal
// SpeechAct occurrence. It is independent of profile permissions and may be
// consumed by any higher-level instituted-effect protocol.
type VerifiedSpeechActSource struct {
	state *verifiedSpeechActSourceState
}

type ProfileDeclarationPermission struct {
	state *profileDeclarationPermissionState
}

type profileDeclarationPermissionState struct {
	ref                     PermissionRef
	digest                  Digest
	subjectRef              RoleAssignmentRef
	subjectDigest           Digest
	modality                string
	actionKind              ActionKind
	projectRoot             ProjectRoot
	validityWindow          TimeWindow
	authorizationContentRef AuthorizationContentRef
	authorizationContentDig Digest
	methodDescriptionRef    MethodDescriptionRef
	methodDescriptionDigest Digest
	predicateRef            ProfileAdmissionPredicateRef
	sourceSpeechActRef      SpeechActRef
	contextPolicyRef        ContextPolicyRef
	contextPolicyDigest     Digest
	boundedContextRef       BoundedContextRef
	claimScopeRef           ClaimScopeRef
	referents               []string
	evidenceClaimRefs       []string
	carrierRefs             []string
	verifierIdentity        VerifierIdentity
	verifierVersion         VerifierVersion
	verificationPolicyRef   VerificationPolicyRef
	verificationPolicyDig   Digest
	evidenceRelationRef     VerificationEvidenceRelationRef
	carrierExpectationRef   VerificationCarrierExpectationRef
	captureCarrierRef       CarrierRef
	captureCarrierDigest    Digest
	canonicalJSON           []byte
}

type InstitutedPermissionEffect struct {
	state *institutedPermissionEffectState
}

type institutedPermissionEffectState struct {
	speechActRef     SpeechActRef
	speechActDigest  Digest
	permissionRef    PermissionRef
	permissionDigest Digest
	projectRoot      ProjectRoot
	digest           Digest
	canonicalJSON    []byte
}

type terminalSessionObservation struct {
	material        string
	nonce           string
	digest          Digest
	holderSystemRef SystemRef
	assignmentRef   RoleAssignmentRef
}

func (observation terminalSessionObservation) validAt(capturedAt time.Time) bool {
	rebuilt, err := newTerminalSessionObservation(
		observation.material,
		observation.nonce,
		capturedAt,
	)
	return err == nil && rebuilt == observation
}

type verifiedAuthorityActState struct {
	intent       PreparedAuthorityIntent
	source       VerifiedSpeechActSource
	capture      TerminalCaptureRecord
	authorizer   AuthorityRoleAssignment
	speechAct    AuthoritySpeechAct
	permission   ProfileDeclarationPermission
	effect       InstitutedPermissionEffect
	basis        AuthorityBasisExpectation
	reviewDigest Digest
}

// VerifiedAuthorityAct is the profile-declaration-specific completion of one
// VerifiedSpeechActSource. It includes the legacy MAY permission and profile
// authority DAG; it is not a reusable decision or commission authorization.
// No public builder, JSON decoder, boolean, or raw-field constructor can mint
// it.
type VerifiedAuthorityAct struct {
	state *verifiedAuthorityActState
}

func newVerifiedSpeechActSource(
	intent PreparedSpeechActIntent,
	reviewText string,
	reviewDigest Digest,
	canonicalUtterance string,
	startedAt time.Time,
	exactUtteranceObservedAt time.Time,
	endedAt time.Time,
	terminalSession terminalSessionObservation,
) (VerifiedSpeechActSource, error) {
	canonicalReviewDigest, reviewErr := SpeechActIntentReviewDigest(intent, reviewText)
	if !intent.valid() || !reviewDigest.valid() || reviewErr != nil || canonicalReviewDigest != reviewDigest {
		return VerifiedSpeechActSource{}, fmt.Errorf("verified SpeechAct source requires canonical pre-act material")
	}
	workWindow, workWindowErr := NewTimeWindow(startedAt, endedAt)
	canonicalUtteranceObservedAt := canonicalAuthorityTime(exactUtteranceObservedAt)
	if workWindowErr != nil || !canonicalUtteranceObservedAt.After(workWindow.from) ||
		!canonicalUtteranceObservedAt.Before(workWindow.until) {
		return VerifiedSpeechActSource{}, fmt.Errorf("verified SpeechAct source requires actual ordered Work observations")
	}
	if !terminalSession.validAt(canonicalUtteranceObservedAt) {
		return VerifiedSpeechActSource{}, fmt.Errorf("verified SpeechAct source terminal observation is invalid")
	}
	expectedUtterance, err := intent.state.utteranceRule.expected(
		reviewDigest,
		intent.state.reviewSubjectDig,
	)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	if canonicalUtterance != expectedUtterance {
		return VerifiedSpeechActSource{}, fmt.Errorf("verified SpeechAct source utterance does not match review digest")
	}
	capture, err := newTerminalCaptureRecord(
		intent,
		reviewText,
		reviewDigest,
		canonicalUtterance,
		workWindow,
		canonicalUtteranceObservedAt,
		terminalSession,
	)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	authorizer, err := newContextPolicyAssignedTerminalSession(
		intent,
		capture,
		workWindow,
	)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	speechAct, err := newAuthoritySpeechAct(intent, capture, authorizer)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	state := verifiedSpeechActSourceState{
		intent:       intent,
		capture:      capture,
		authorizer:   authorizer,
		speechAct:    speechAct,
		reviewDigest: reviewDigest,
	}
	result := VerifiedSpeechActSource{state: &state}
	if !result.valid() {
		return VerifiedSpeechActSource{}, fmt.Errorf("verified SpeechAct source graph is invalid")
	}
	return result, nil
}

// completeVerifiedAuthorityAct composes a verified generic source with the
// exact profile authority intent that supplied its review subject. It does not
// perform another act and rejects sources minted for another intent.
func completeVerifiedAuthorityAct(
	intent PreparedAuthorityIntent,
	source VerifiedSpeechActSource,
) (VerifiedAuthorityAct, error) {
	if !intent.valid() || !source.valid() {
		return VerifiedAuthorityAct{}, fmt.Errorf("authority completion requires canonical intent and SpeechAct source")
	}
	if source.state.intent.state.digest != intent.state.sourceIntent.state.digest {
		return VerifiedAuthorityAct{}, fmt.Errorf("SpeechAct source does not bind the profile authority intent")
	}
	workWindow := source.state.speechAct.state.window
	if !authorityIntentCoversWork(intent, workWindow) {
		return VerifiedAuthorityAct{}, fmt.Errorf("SpeechAct source occurred outside the profile authority windows")
	}
	capture := source.state.capture
	authorizer := source.state.authorizer
	speechAct := source.state.speechAct
	permission, err := newProfileDeclarationPermission(intent, speechAct)
	if err != nil {
		return VerifiedAuthorityAct{}, err
	}
	effect, err := newInstitutedPermissionEffect(speechAct, permission)
	if err != nil {
		return VerifiedAuthorityAct{}, err
	}
	basis, err := authorityBasisFromAct(intent, speechAct, permission)
	if err != nil {
		return VerifiedAuthorityAct{}, err
	}
	state := verifiedAuthorityActState{
		intent:       intent,
		source:       source,
		capture:      capture,
		authorizer:   authorizer,
		speechAct:    speechAct,
		permission:   permission,
		effect:       effect,
		basis:        basis,
		reviewDigest: source.state.reviewDigest,
	}
	result := VerifiedAuthorityAct{state: &state}
	if !result.valid() {
		return VerifiedAuthorityAct{}, fmt.Errorf("verified authority act DAG is invalid")
	}
	return result, nil
}

func authorityIntentCoversWork(intent PreparedAuthorityIntent, work TimeWindow) bool {
	envelope := intent.state.content.state.envelope
	return envelope.authorizationValidityWindow.Contains(work.from) &&
		!work.until.After(envelope.authorizationValidityWindow.until) &&
		intent.state.resolutionWindow.Contains(work.from) &&
		!work.until.After(intent.state.resolutionWindow.until)
}

func newTerminalCaptureRecord(
	intent PreparedSpeechActIntent,
	reviewText string,
	reviewDigest Digest,
	canonicalUtterance string,
	workWindow TimeWindow,
	exactUtteranceObservedAt time.Time,
	observation terminalSessionObservation,
) (TerminalCaptureRecord, error) {
	canonicalUtteranceObservedAt := canonicalAuthorityTime(exactUtteranceObservedAt)
	if !observation.validAt(canonicalUtteranceObservedAt) ||
		!canonicalUtteranceObservedAt.After(workWindow.from) ||
		!canonicalUtteranceObservedAt.Before(workWindow.until) {
		return TerminalCaptureRecord{}, fmt.Errorf("terminal capture requires replayable terminal-session observation")
	}
	projection := struct {
		Schema                   string `json:"schema"`
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
		IdentityBoundary         string `json:"identity_boundary"`
	}{
		Schema:                   "haft.authority.terminal-capture/v1",
		CarrierRef:               intent.state.captureCarrierRef.String(),
		ReviewDigest:             reviewDigest.String(),
		ReviewText:               reviewText,
		PreparedIntentDig:        intent.state.digest.String(),
		CanonicalUtterance:       canonicalUtterance,
		StartedAt:                formatAuthorityTime(workWindow.from),
		ExactUtteranceObservedAt: formatAuthorityTime(canonicalUtteranceObservedAt),
		EndedAt:                  formatAuthorityTime(workWindow.until),
		ProjectRoot:              intent.state.projectRoot.String(),
		SessionRef:               intent.state.sessionRef.String(),
		ObservedMaterial:         observation.material,
		ObservationNonce:         observation.nonce,
		ObservationDigest:        observation.digest.String(),
		ObservedHolderRef:        observation.holderSystemRef.String(),
		ObservedRoleRef:          observation.assignmentRef.String(),
		IdentityBoundary:         authorityIssueIdentityBoundary,
	}
	canonicalJSON, err := json.Marshal(projection)
	if err != nil {
		return TerminalCaptureRecord{}, fmt.Errorf("encode terminal capture: %w", err)
	}
	writer := newAuthorityDigestWriter(authorityTerminalCaptureDigestDomain)
	writer.add(string(canonicalJSON))
	state := terminalCaptureRecordState{
		carrierRef:               intent.state.captureCarrierRef,
		carrierDigest:            writer.digest(),
		reviewDigest:             reviewDigest,
		reviewText:               reviewText,
		preparedIntentDig:        intent.state.digest,
		canonicalUtterance:       canonicalUtterance,
		startedAt:                workWindow.from,
		exactUtteranceObservedAt: canonicalUtteranceObservedAt,
		endedAt:                  workWindow.until,
		projectRoot:              intent.state.projectRoot,
		sessionRef:               intent.state.sessionRef,
		observation:              observation,
		canonicalJSON:            canonicalJSON,
	}
	return TerminalCaptureRecord{state: &state}, nil
}

func newContextPolicyAssignedTerminalSession(
	intent PreparedSpeechActIntent,
	capture TerminalCaptureRecord,
	workWindow TimeWindow,
) (AuthorityRoleAssignment, error) {
	if capture.state == nil || capture.state.startedAt != workWindow.from ||
		capture.state.endedAt != workWindow.until {
		return AuthorityRoleAssignment{}, fmt.Errorf("terminal-session RoleAssignment requires its exact capture provenance")
	}
	terminalSession := capture.state.observation
	if !terminalSession.validAt(capture.state.exactUtteranceObservedAt) {
		return AuthorityRoleAssignment{}, fmt.Errorf("terminal-session RoleAssignment does not match capture observation")
	}
	window := workWindow
	policy := intent.state.contextPolicy.state
	projection := struct {
		Schema                    string `json:"schema"`
		Ref                       string `json:"role_assignment_ref"`
		ProjectRoot               string `json:"project_root"`
		HolderSystemRef           string `json:"holder_system_ref"`
		AdmittedHolderKind        string `json:"admitted_holder_kind"`
		RoleRef                   string `json:"role_ref"`
		BoundedContextRef         string `json:"bounded_context_ref"`
		ValidFrom                 string `json:"valid_from"`
		ValidUntil                string `json:"valid_until"`
		JustificationSourceRef    string `json:"justification_source_ref"`
		JustificationSourceDigest string `json:"justification_source_digest"`
		ProvenanceCarrierRef      string `json:"assignment_provenance_carrier_ref"`
		ProvenanceCarrierDigest   string `json:"assignment_provenance_carrier_digest"`
		IdentityBoundary          string `json:"identity_boundary"`
	}{
		Schema:                    "haft.authority.context-policy-assigned-terminal-session/v1",
		Ref:                       terminalSession.assignmentRef.String(),
		ProjectRoot:               intent.state.projectRoot.String(),
		HolderSystemRef:           terminalSession.holderSystemRef.String(),
		AdmittedHolderKind:        admittedHolderSystemKindValue,
		RoleRef:                   terminalAuthorizerRoleRefValue,
		BoundedContextRef:         policy.boundedContext.String(),
		ValidFrom:                 formatAuthorityTime(window.from),
		ValidUntil:                formatAuthorityTime(window.until),
		JustificationSourceRef:    policy.ref.String(),
		JustificationSourceDigest: policy.digest.String(),
		ProvenanceCarrierRef:      capture.state.carrierRef.String(),
		ProvenanceCarrierDigest:   capture.state.carrierDigest.String(),
		IdentityBoundary:          authorityIssueIdentityBoundary,
	}
	canonicalJSON, err := json.Marshal(projection)
	if err != nil {
		return AuthorityRoleAssignment{}, fmt.Errorf("encode terminal-session RoleAssignment: %w", err)
	}
	writer := newAuthorityDigestWriter(authorityRoleAssignmentDigestDomain)
	writer.add(string(canonicalJSON))
	state := authorityRoleAssignmentState{
		ref:                       terminalSession.assignmentRef,
		digest:                    writer.digest(),
		projectRoot:               intent.state.projectRoot,
		holderSystemRef:           terminalSession.holderSystemRef,
		admittedHolderKind:        admittedHolderSystemKindValue,
		roleRef:                   terminalAuthorizerRoleRefValue,
		boundedContextRef:         policy.boundedContext,
		assignmentWindow:          window,
		justificationSourceRef:    policy.ref,
		justificationSourceDigest: policy.digest,
		provenanceCarrierRef:      capture.state.carrierRef,
		provenanceCarrierDigest:   capture.state.carrierDigest,
		canonicalJSON:             canonicalJSON,
	}
	return AuthorityRoleAssignment{state: &state}, nil
}

func newAuthoritySpeechAct(
	intent PreparedSpeechActIntent,
	capture TerminalCaptureRecord,
	authorizer AuthorityRoleAssignment,
) (AuthoritySpeechAct, error) {
	frame := intent.state.executionFrame.state
	policy := intent.state.contextPolicy.state
	projection := authoritySpeechActProjection{
		Schema:                  "haft.authority.speech-act/v1",
		Ref:                     intent.state.speechActRef.String(),
		ProjectRoot:             intent.state.projectRoot.String(),
		WorkKind:                communicativeWorkKindValue,
		ActTypeRef:              policy.recognizedActType.String(),
		PerformedByRef:          authorizer.state.ref.String(),
		PerformedByDigest:       authorizer.state.digest.String(),
		MethodRef:               frame.methodRef.String(),
		MethodDescriptionRef:    frame.methodDescriptionRef.String(),
		MethodDescriptionDigest: frame.methodDescriptionDigest.String(),
		ExecutedWithinRef:       frame.executedWithin.String(),
		BoundedContextRef:       policy.boundedContext.String(),
		WindowFrom:              formatAuthorityTime(authorizer.state.assignmentWindow.from),
		WindowUntil:             formatAuthorityTime(authorizer.state.assignmentWindow.until),
		Parameters:              projectWorkParameters(frame.parameters),
		InputRefs:               []string{intent.state.reviewSubjectRef.String()},
		OutputRefs:              []string{intent.state.institutedObject.String()},
		Resources:               projectWorkResources(frame.resources),
		AffectedRefs:            projectAffectedRefs(frame.affected),
		StatePlaneRef:           frame.statePlane.String(),
		DeltaPredicateRef:       frame.deltaPredicate.String(),
		OutcomeRef:              frame.outcome.String(),
		UtteranceRef:            frame.utterance.String(),
		CaptureCarrierRef:       capture.state.carrierRef.String(),
		CaptureCarrierDigest:    capture.state.carrierDigest.String(),
		ReviewSubjectRef:        intent.state.reviewSubjectRef.String(),
		ReviewSubjectDigest:     intent.state.reviewSubjectDig.String(),
		InstitutedObjectRef:     intent.state.institutedObject.String(),
	}
	canonicalJSON, err := json.Marshal(projection)
	if err != nil {
		return AuthoritySpeechAct{}, fmt.Errorf("encode authority SpeechAct: %w", err)
	}
	writer := newAuthorityDigestWriter(authoritySpeechActDigestDomain)
	writer.add(string(canonicalJSON))
	state := authoritySpeechActState{
		ref:                     intent.state.speechActRef,
		digest:                  writer.digest(),
		projectRoot:             intent.state.projectRoot,
		workKind:                communicativeWorkKindValue,
		actType:                 policy.recognizedActType,
		performedByRef:          authorizer.state.ref,
		performedByDigest:       authorizer.state.digest,
		methodRef:               frame.methodRef,
		methodDescriptionRef:    frame.methodDescriptionRef,
		methodDescriptionDigest: frame.methodDescriptionDigest,
		executedWithin:          frame.executedWithin,
		boundedContext:          policy.boundedContext,
		window:                  authorizer.state.assignmentWindow,
		parameters:              append([]WorkParameterBinding{}, frame.parameters...),
		inputRefs:               []string{intent.state.reviewSubjectRef.String()},
		outputRefs:              []string{intent.state.institutedObject.String()},
		resources:               append([]WorkResourceRef{}, frame.resources...),
		affected:                append([]AffectedRef{}, frame.affected...),
		statePlane:              frame.statePlane,
		deltaPredicate:          frame.deltaPredicate,
		outcome:                 frame.outcome,
		utteranceRef:            frame.utterance,
		captureCarrierRef:       capture.state.carrierRef,
		captureCarrierDigest:    capture.state.carrierDigest,
		reviewSubjectRef:        intent.state.reviewSubjectRef,
		reviewSubjectDigest:     intent.state.reviewSubjectDig,
		institutedObjectRef:     intent.state.institutedObject,
		canonicalJSON:           canonicalJSON,
	}
	return AuthoritySpeechAct{state: &state}, nil
}

type authoritySpeechActProjection struct {
	Schema                  string                    `json:"schema"`
	Ref                     string                    `json:"speech_act_ref"`
	ProjectRoot             string                    `json:"project_root"`
	WorkKind                string                    `json:"work_kind"`
	ActTypeRef              string                    `json:"act_type_ref"`
	PerformedByRef          string                    `json:"performed_by_role_assignment_ref"`
	PerformedByDigest       string                    `json:"performed_by_role_assignment_digest"`
	MethodRef               string                    `json:"method_ref"`
	MethodDescriptionRef    string                    `json:"method_description_ref"`
	MethodDescriptionDigest string                    `json:"method_description_digest"`
	ExecutedWithinRef       string                    `json:"executed_within_system_ref"`
	BoundedContextRef       string                    `json:"bounded_context_ref"`
	WindowFrom              string                    `json:"window_from"`
	WindowUntil             string                    `json:"window_until"`
	Parameters              []workParameterProjection `json:"parameters"`
	InputRefs               []string                  `json:"input_refs"`
	OutputRefs              []string                  `json:"output_refs"`
	Resources               []string                  `json:"resource_refs"`
	AffectedRefs            []string                  `json:"affected_refs"`
	StatePlaneRef           string                    `json:"state_plane_ref"`
	DeltaPredicateRef       string                    `json:"delta_predicate_ref"`
	OutcomeRef              string                    `json:"outcome_ref"`
	UtteranceRef            string                    `json:"utterance_ref"`
	CaptureCarrierRef       string                    `json:"capture_carrier_ref"`
	CaptureCarrierDigest    string                    `json:"capture_carrier_digest"`
	ReviewSubjectRef        string                    `json:"review_subject_ref"`
	ReviewSubjectDigest     string                    `json:"review_subject_digest"`
	InstitutedObjectRef     string                    `json:"instituted_object_ref"`
}

type workParameterProjection struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func projectWorkParameters(values []WorkParameterBinding) []workParameterProjection {
	result := make([]workParameterProjection, 0, len(values))
	for _, value := range values {
		result = append(result, workParameterProjection{Name: value.name, Value: value.value})
	}
	return result
}

func projectWorkResources(values []WorkResourceRef) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}

func projectAffectedRefs(values []AffectedRef) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}

func newProfileDeclarationPermission(
	intent PreparedAuthorityIntent,
	speechAct AuthoritySpeechAct,
) (ProfileDeclarationPermission, error) {
	institutedPermissionRef, err := NewInstitutedObjectRef(intent.state.permissionRef.String())
	if err != nil {
		return ProfileDeclarationPermission{}, err
	}
	if speechAct.state == nil || speechAct.state.institutedObjectRef != institutedPermissionRef {
		return ProfileDeclarationPermission{}, fmt.Errorf("SpeechAct does not institute the intended permission ref")
	}
	content := intent.state.content.state
	policy := intent.state.contextPolicy.state
	envelope := content.envelope
	permissionWindow, err := NewTimeWindow(
		speechAct.state.window.until,
		envelope.authorizationValidityWindow.until,
	)
	if err != nil {
		return ProfileDeclarationPermission{}, fmt.Errorf("permission validity cannot begin before its instituting SpeechAct: %w", err)
	}
	projection := struct {
		Schema                  string   `json:"schema"`
		Ref                     string   `json:"permission_ref"`
		SubjectRef              string   `json:"subject_role_assignment_ref"`
		SubjectDigest           string   `json:"subject_role_assignment_digest"`
		Modality                string   `json:"modality"`
		ActionKind              string   `json:"action_kind"`
		ProjectRoot             string   `json:"project_root"`
		ValidFrom               string   `json:"valid_from"`
		ValidUntil              string   `json:"valid_until"`
		AuthorizationContentRef string   `json:"authorization_content_ref"`
		AuthorizationContentDig string   `json:"authorization_content_digest"`
		MethodDescriptionRef    string   `json:"method_description_ref"`
		MethodDescriptionDigest string   `json:"method_description_digest"`
		AdmissionPredicateRef   string   `json:"profile_admission_predicate_ref"`
		SourceSpeechActRef      string   `json:"source_speech_act_ref"`
		ContextPolicyRef        string   `json:"context_policy_ref"`
		ContextPolicyDigest     string   `json:"context_policy_digest"`
		BoundedContextRef       string   `json:"claim_scope_bounded_context_ref"`
		ClaimScopeRef           string   `json:"claim_scope_ref"`
		Referents               []string `json:"referents"`
		EvidenceClaimRefs       []string `json:"adjudication_evidence_claim_refs"`
		CarrierRefs             []string `json:"adjudication_carrier_refs"`
		VerifierIdentity        string   `json:"adjudication_verifier_identity"`
		VerifierVersion         string   `json:"adjudication_verifier_version"`
		VerificationPolicyRef   string   `json:"adjudication_verification_policy_ref"`
		VerificationPolicyDig   string   `json:"adjudication_verification_policy_digest"`
		EvaluationPolicyRef     string   `json:"adjudication_evaluation_policy_ref"`
		EvaluationPolicyDigest  string   `json:"adjudication_evaluation_policy_digest"`
		EvidenceRelationRef     string   `json:"adjudication_evidence_relation_ref"`
		CarrierExpectationRef   string   `json:"adjudication_carrier_expectation_ref"`
		CaptureCarrierRef       string   `json:"instituting_terminal_capture_carrier_ref"`
		CaptureCarrierDigest    string   `json:"instituting_terminal_capture_carrier_digest"`
	}{
		Schema:                  "haft.authority.profile-declaration-permission/v1",
		Ref:                     intent.state.permissionRef.String(),
		SubjectRef:              envelope.profileAuthor.String(),
		SubjectDigest:           envelope.profileAuthorDigest.String(),
		Modality:                permissionModalityMay,
		ActionKind:              envelope.actionKind.String(),
		ProjectRoot:             envelope.projectRoot.String(),
		ValidFrom:               formatAuthorityTime(permissionWindow.from),
		ValidUntil:              formatAuthorityTime(permissionWindow.until),
		AuthorizationContentRef: content.ref.String(),
		AuthorizationContentDig: content.digest.String(),
		MethodDescriptionRef:    envelope.methodDescription.String(),
		MethodDescriptionDigest: envelope.methodDescriptionDigest.String(),
		AdmissionPredicateRef:   intent.state.permissionPredicateRef.String(),
		SourceSpeechActRef:      speechAct.state.ref.String(),
		ContextPolicyRef:        policy.ref.String(),
		ContextPolicyDigest:     policy.digest.String(),
		BoundedContextRef:       policy.boundedContext.String(),
		ClaimScopeRef:           intent.state.claimScopeRef.String(),
		Referents: []string{
			content.ref.String(),
			envelope.methodDescription.String(),
			intent.state.permissionPredicateRef.String(),
		},
		EvidenceClaimRefs: []string{speechAct.state.reviewSubjectRef.String()},
		CarrierRefs: []string{
			intent.state.carrierExpectationRef.String(),
			speechAct.state.captureCarrierRef.String(),
			speechAct.state.ref.String(),
		},
		VerifierIdentity:       intent.state.verifierIdentity.String(),
		VerifierVersion:        intent.state.verifierVersion.String(),
		VerificationPolicyRef:  intent.state.verificationPolicyRef.String(),
		VerificationPolicyDig:  intent.state.verificationPolicyDigest.String(),
		EvaluationPolicyRef:    intent.state.verificationPolicyRef.String(),
		EvaluationPolicyDigest: intent.state.verificationPolicyDigest.String(),
		EvidenceRelationRef:    intent.state.evidenceRelationRef.String(),
		CarrierExpectationRef:  intent.state.carrierExpectationRef.String(),
		CaptureCarrierRef:      speechAct.state.captureCarrierRef.String(),
		CaptureCarrierDigest:   speechAct.state.captureCarrierDigest.String(),
	}
	canonicalJSON, err := json.Marshal(projection)
	if err != nil {
		return ProfileDeclarationPermission{}, fmt.Errorf("encode profile-declaration permission: %w", err)
	}
	writer := newAuthorityDigestWriter(authorityPermissionDigestDomain)
	writer.add(string(canonicalJSON))
	state := profileDeclarationPermissionState{
		ref:                     intent.state.permissionRef,
		digest:                  writer.digest(),
		subjectRef:              envelope.profileAuthor,
		subjectDigest:           envelope.profileAuthorDigest,
		modality:                permissionModalityMay,
		actionKind:              envelope.actionKind,
		projectRoot:             envelope.projectRoot,
		validityWindow:          permissionWindow,
		authorizationContentRef: content.ref,
		authorizationContentDig: content.digest,
		methodDescriptionRef:    envelope.methodDescription,
		methodDescriptionDigest: envelope.methodDescriptionDigest,
		predicateRef:            intent.state.permissionPredicateRef,
		sourceSpeechActRef:      speechAct.state.ref,
		contextPolicyRef:        policy.ref,
		contextPolicyDigest:     policy.digest,
		boundedContextRef:       policy.boundedContext,
		claimScopeRef:           intent.state.claimScopeRef,
		referents: []string{
			content.ref.String(),
			envelope.methodDescription.String(),
			intent.state.permissionPredicateRef.String(),
		},
		evidenceClaimRefs: []string{speechAct.state.reviewSubjectRef.String()},
		carrierRefs: []string{
			intent.state.carrierExpectationRef.String(),
			speechAct.state.captureCarrierRef.String(),
			speechAct.state.ref.String(),
		},
		verifierIdentity:      intent.state.verifierIdentity,
		verifierVersion:       intent.state.verifierVersion,
		verificationPolicyRef: intent.state.verificationPolicyRef,
		verificationPolicyDig: intent.state.verificationPolicyDigest,
		evidenceRelationRef:   intent.state.evidenceRelationRef,
		carrierExpectationRef: intent.state.carrierExpectationRef,
		captureCarrierRef:     speechAct.state.captureCarrierRef,
		captureCarrierDigest:  speechAct.state.captureCarrierDigest,
		canonicalJSON:         canonicalJSON,
	}
	return ProfileDeclarationPermission{state: &state}, nil
}

func newInstitutedPermissionEffect(
	speechAct AuthoritySpeechAct,
	permission ProfileDeclarationPermission,
) (InstitutedPermissionEffect, error) {
	if speechAct.state == nil || permission.state == nil {
		return InstitutedPermissionEffect{}, fmt.Errorf("instituted effect requires SpeechAct and permission")
	}
	if speechAct.state.projectRoot != permission.state.projectRoot {
		return InstitutedPermissionEffect{}, fmt.Errorf("instituted effect crosses project roots")
	}
	projection := struct {
		Schema           string `json:"schema"`
		ProjectRoot      string `json:"project_root"`
		SpeechActRef     string `json:"speech_act_ref"`
		SpeechActDigest  string `json:"speech_act_digest"`
		PermissionRef    string `json:"permission_ref"`
		PermissionDigest string `json:"permission_digest"`
	}{
		Schema:           "haft.authority.instituted-permission-effect/v1",
		ProjectRoot:      speechAct.state.projectRoot.String(),
		SpeechActRef:     speechAct.state.ref.String(),
		SpeechActDigest:  speechAct.state.digest.String(),
		PermissionRef:    permission.state.ref.String(),
		PermissionDigest: permission.state.digest.String(),
	}
	canonicalJSON, err := json.Marshal(projection)
	if err != nil {
		return InstitutedPermissionEffect{}, fmt.Errorf("encode instituted permission effect: %w", err)
	}
	writer := newAuthorityDigestWriter(institutedEffectDigestDomain)
	writer.add(string(canonicalJSON))
	state := institutedPermissionEffectState{
		speechActRef:     speechAct.state.ref,
		speechActDigest:  speechAct.state.digest,
		permissionRef:    permission.state.ref,
		permissionDigest: permission.state.digest,
		projectRoot:      speechAct.state.projectRoot,
		digest:           writer.digest(),
		canonicalJSON:    canonicalJSON,
	}
	return InstitutedPermissionEffect{state: &state}, nil
}

func (source VerifiedSpeechActSource) valid() bool {
	if source.state == nil || !source.state.intent.valid() || !source.state.reviewDigest.valid() {
		return false
	}
	state := source.state
	if state.capture.state == nil || state.authorizer.state == nil || state.speechAct.state == nil {
		return false
	}
	rebuiltCapture, err := newTerminalCaptureRecord(
		state.intent,
		state.capture.state.reviewText,
		state.reviewDigest,
		state.capture.state.canonicalUtterance,
		state.speechAct.state.window,
		state.capture.state.exactUtteranceObservedAt,
		state.capture.state.observation,
	)
	if err != nil || rebuiltCapture.state.carrierDigest != state.capture.state.carrierDigest ||
		!slices.Equal(rebuiltCapture.state.canonicalJSON, state.capture.state.canonicalJSON) {
		return false
	}
	rebuiltAuthorizer, err := newContextPolicyAssignedTerminalSession(
		state.intent,
		state.capture,
		state.speechAct.state.window,
	)
	if err != nil || !authorityRoleAssignmentStatesEqual(
		rebuiltAuthorizer.state,
		state.authorizer.state,
	) {
		return false
	}
	rebuiltSpeechAct, err := newAuthoritySpeechAct(
		state.intent,
		state.capture,
		state.authorizer,
	)
	if err != nil || rebuiltSpeechAct.state.digest != state.speechAct.state.digest ||
		!slices.Equal(rebuiltSpeechAct.state.canonicalJSON, state.speechAct.state.canonicalJSON) {
		return false
	}
	return state.speechAct.state.performedByRef == state.authorizer.state.ref &&
		state.speechAct.state.captureCarrierRef == state.capture.state.carrierRef
}

func (source VerifiedSpeechActSource) Valid() bool {
	return source.valid()
}

func authorityRoleAssignmentStatesEqual(
	left *authorityRoleAssignmentState,
	right *authorityRoleAssignmentState,
) bool {
	if left == nil || right == nil {
		return false
	}
	return left.ref == right.ref &&
		left.digest == right.digest &&
		left.projectRoot == right.projectRoot &&
		left.holderSystemRef == right.holderSystemRef &&
		left.admittedHolderKind == right.admittedHolderKind &&
		left.roleRef == right.roleRef &&
		left.boundedContextRef == right.boundedContextRef &&
		left.assignmentWindow == right.assignmentWindow &&
		left.justificationSourceRef == right.justificationSourceRef &&
		left.justificationSourceDigest == right.justificationSourceDigest &&
		left.provenanceCarrierRef == right.provenanceCarrierRef &&
		left.provenanceCarrierDigest == right.provenanceCarrierDigest &&
		slices.Equal(left.canonicalJSON, right.canonicalJSON)
}

func (source VerifiedSpeechActSource) TerminalCaptureDigest() (Digest, bool) {
	if !source.valid() {
		return Digest{}, false
	}
	return source.state.capture.state.carrierDigest, true
}

func (source VerifiedSpeechActSource) TerminalCaptureRef() (CarrierRef, bool) {
	if !source.valid() {
		return CarrierRef{}, false
	}
	return source.state.capture.state.carrierRef, true
}

func (source VerifiedSpeechActSource) ReviewDigest() (Digest, bool) {
	if !source.valid() {
		return Digest{}, false
	}
	return source.state.reviewDigest, true
}

func (source VerifiedSpeechActSource) ReviewText() (string, bool) {
	if !source.valid() {
		return "", false
	}
	return source.state.capture.state.reviewText, true
}

func (source VerifiedSpeechActSource) ReviewSubjectRef() (SpeechActReviewSubjectRef, bool) {
	if !source.valid() {
		return SpeechActReviewSubjectRef{}, false
	}
	return source.state.speechAct.state.reviewSubjectRef, true
}

func (source VerifiedSpeechActSource) ReviewSubjectDigest() (Digest, bool) {
	if !source.valid() {
		return Digest{}, false
	}
	return source.state.speechAct.state.reviewSubjectDigest, true
}

func (source VerifiedSpeechActSource) SpeechActDigest() (Digest, bool) {
	if !source.valid() {
		return Digest{}, false
	}
	return source.state.speechAct.state.digest, true
}

func (source VerifiedSpeechActSource) SpeechActRef() (SpeechActRef, bool) {
	if !source.valid() {
		return SpeechActRef{}, false
	}
	return source.state.speechAct.state.ref, true
}

func (source VerifiedSpeechActSource) ProjectRoot() (ProjectRoot, bool) {
	if !source.valid() {
		return ProjectRoot{}, false
	}
	return source.state.speechAct.state.projectRoot, true
}

func (source VerifiedSpeechActSource) PreparedIntentDigest() (Digest, bool) {
	if !source.valid() {
		return Digest{}, false
	}
	return source.state.intent.state.digest, true
}

func (source VerifiedSpeechActSource) WorkWindow() (TimeWindow, bool) {
	if !source.valid() {
		return TimeWindow{}, false
	}
	return source.state.speechAct.state.window, true
}

func (source VerifiedSpeechActSource) CompletedAt() (time.Time, bool) {
	window, ok := source.WorkWindow()
	if !ok {
		return time.Time{}, false
	}
	return window.Until(), true
}

func (source VerifiedSpeechActSource) PerformedByRoleAssignmentRef() (RoleAssignmentRef, bool) {
	if !source.valid() {
		return RoleAssignmentRef{}, false
	}
	return source.state.speechAct.state.performedByRef, true
}

func (source VerifiedSpeechActSource) PerformedByRoleAssignmentDigest() (Digest, bool) {
	if !source.valid() {
		return Digest{}, false
	}
	return source.state.speechAct.state.performedByDigest, true
}

// InstitutedObjectRef returns the exact object identity named by the performed
// act. It does not prove that the domain-specific institutional effect was
// successfully applied.
func (source VerifiedSpeechActSource) InstitutedObjectRef() (InstitutedObjectRef, bool) {
	if !source.valid() {
		return InstitutedObjectRef{}, false
	}
	return source.state.speechAct.state.institutedObjectRef, true
}

func (source VerifiedSpeechActSource) ContextPolicyRef() (ContextPolicyRef, bool) {
	if !source.valid() {
		return ContextPolicyRef{}, false
	}
	return source.state.intent.state.contextPolicy.state.ref, true
}

func (source VerifiedSpeechActSource) ContextPolicyDigest() (Digest, bool) {
	if !source.valid() {
		return Digest{}, false
	}
	return source.state.intent.state.contextPolicy.state.digest, true
}

func (source VerifiedSpeechActSource) OccurredAt() (time.Time, bool) {
	if !source.valid() {
		return time.Time{}, false
	}
	return source.state.capture.state.exactUtteranceObservedAt, true
}

func authorityBasisFromAct(
	intent PreparedAuthorityIntent,
	speechAct AuthoritySpeechAct,
	permission ProfileDeclarationPermission,
) (AuthorityBasisExpectation, error) {
	content := intent.state.content.state
	policy := intent.state.contextPolicy.state
	builder := NewAuthorityBasisExpectationBuilder()
	builder = builder.FromSpeechAct(speechAct.state.ref, speechAct.state.digest)
	builder = builder.DescribedBy(content.ref, content.digest)
	builder = builder.InstitutesPermission(permission.state.ref, permission.state.digest)
	builder = builder.ScopedBy(intent.state.permissionPredicateRef)
	builder = builder.UnderContextPolicy(policy.ref, policy.digest)
	return builder.Build()
}

func authorityPairFromCheckedAct(
	intent PreparedAuthorityIntent,
	basis AuthorityBasisExpectation,
	workEndedAt time.Time,
	checkedAt time.Time,
) (canonicalPresentation, canonicalAuthorityResolution, error) {
	canonicalWorkEndedAt := canonicalAuthorityTime(workEndedAt)
	canonicalCheckedAt := canonicalAuthorityTime(checkedAt)
	if canonicalCheckedAt.Before(canonicalWorkEndedAt) ||
		!intent.state.resolutionWindow.Contains(canonicalCheckedAt) ||
		!intent.state.content.state.envelope.authorizationValidityWindow.Contains(canonicalCheckedAt) {
		return canonicalPresentation{}, canonicalAuthorityResolution{}, fmt.Errorf("authority check time is outside capture and validity bounds")
	}
	envelope := intent.state.content.state.envelope
	presentationDigest, err := presentationDigest(intent.state.presentationID, basis, envelope)
	if err != nil {
		return canonicalPresentation{}, canonicalAuthorityResolution{}, err
	}
	presentation := canonicalPresentation{
		id:       intent.state.presentationID,
		basis:    basis,
		envelope: envelope,
		digest:   presentationDigest,
	}
	resolution := canonicalAuthorityResolution{
		id:                       intent.state.authorityResolutionID,
		presentationID:           presentation.id,
		presentationDigest:       presentation.digest,
		profileAuthorRef:         envelope.profileAuthor,
		profileAuthorDigest:      envelope.profileAuthorDigest,
		methodDescriptionRef:     envelope.methodDescription,
		methodDescriptionDigest:  envelope.methodDescriptionDigest,
		methodContractRef:        envelope.methodContract,
		methodContractDigest:     envelope.methodContractDigest,
		verifierIdentity:         intent.state.verifierIdentity,
		verifierVersion:          intent.state.verifierVersion,
		verificationPolicyRef:    intent.state.verificationPolicyRef,
		verificationPolicyDigest: intent.state.verificationPolicyDigest,
		resolvedAt:               canonicalCheckedAt,
		validUntil:               intent.state.resolutionWindow.until,
	}
	resolution.digest = authorityResolutionDigest(resolution)
	if err := validateCanonicalAuthorityResolution(resolution, presentation); err != nil {
		return canonicalPresentation{}, canonicalAuthorityResolution{}, err
	}
	return presentation, resolution, nil
}

func (act VerifiedAuthorityAct) valid() bool {
	if act.state == nil || !act.state.intent.valid() || !act.state.source.valid() {
		return false
	}
	state := act.state
	if state.capture.state == nil || state.authorizer.state == nil ||
		state.speechAct.state == nil || state.permission.state == nil || state.effect.state == nil {
		return false
	}
	institutedPermissionRef, err := NewInstitutedObjectRef(state.permission.state.ref.String())
	if err != nil {
		return false
	}
	refsAlign := state.source.state.speechAct.state.digest == state.speechAct.state.digest &&
		state.speechAct.state.performedByRef == state.authorizer.state.ref &&
		state.speechAct.state.performedByDigest == state.authorizer.state.digest &&
		state.speechAct.state.captureCarrierRef == state.capture.state.carrierRef &&
		state.speechAct.state.captureCarrierDigest == state.capture.state.carrierDigest &&
		state.speechAct.state.institutedObjectRef == institutedPermissionRef &&
		state.permission.state.sourceSpeechActRef == state.speechAct.state.ref &&
		state.effect.state.speechActDigest == state.speechAct.state.digest &&
		state.effect.state.permissionDigest == state.permission.state.digest
	if !refsAlign {
		return false
	}
	rebuiltPermission, err := newProfileDeclarationPermission(state.intent, state.speechAct)
	if err != nil || !profileDeclarationPermissionStatesEqual(
		rebuiltPermission.state,
		state.permission.state,
	) {
		return false
	}
	rebuiltEffect, err := newInstitutedPermissionEffect(state.speechAct, state.permission)
	if err != nil || rebuiltEffect.state.digest != state.effect.state.digest {
		return false
	}
	rebuiltBasis, err := authorityBasisFromAct(state.intent, state.speechAct, state.permission)
	if err != nil || rebuiltBasis != state.basis {
		return false
	}
	return true
}

func profileDeclarationPermissionStatesEqual(
	left *profileDeclarationPermissionState,
	right *profileDeclarationPermissionState,
) bool {
	if left == nil || right == nil {
		return false
	}
	return left.ref == right.ref &&
		left.digest == right.digest &&
		left.subjectRef == right.subjectRef &&
		left.subjectDigest == right.subjectDigest &&
		left.modality == right.modality &&
		left.actionKind == right.actionKind &&
		left.projectRoot == right.projectRoot &&
		left.validityWindow == right.validityWindow &&
		left.authorizationContentRef == right.authorizationContentRef &&
		left.authorizationContentDig == right.authorizationContentDig &&
		left.methodDescriptionRef == right.methodDescriptionRef &&
		left.methodDescriptionDigest == right.methodDescriptionDigest &&
		left.predicateRef == right.predicateRef &&
		left.sourceSpeechActRef == right.sourceSpeechActRef &&
		left.contextPolicyRef == right.contextPolicyRef &&
		left.contextPolicyDigest == right.contextPolicyDigest &&
		left.boundedContextRef == right.boundedContextRef &&
		left.claimScopeRef == right.claimScopeRef &&
		slices.Equal(left.referents, right.referents) &&
		slices.Equal(left.evidenceClaimRefs, right.evidenceClaimRefs) &&
		slices.Equal(left.carrierRefs, right.carrierRefs) &&
		left.verifierIdentity == right.verifierIdentity &&
		left.verifierVersion == right.verifierVersion &&
		left.verificationPolicyRef == right.verificationPolicyRef &&
		left.verificationPolicyDig == right.verificationPolicyDig &&
		left.evidenceRelationRef == right.evidenceRelationRef &&
		left.carrierExpectationRef == right.carrierExpectationRef &&
		left.captureCarrierRef == right.captureCarrierRef &&
		left.captureCarrierDigest == right.captureCarrierDigest &&
		slices.Equal(left.canonicalJSON, right.canonicalJSON)
}

func (act VerifiedAuthorityAct) TerminalCaptureDigest() (Digest, bool) {
	if !act.valid() {
		return Digest{}, false
	}
	return act.state.capture.state.carrierDigest, true
}

func (act VerifiedAuthorityAct) SpeechActDigest() (Digest, bool) {
	if !act.valid() {
		return Digest{}, false
	}
	return act.state.speechAct.state.digest, true
}

func (act VerifiedAuthorityAct) PermissionDigest() (Digest, bool) {
	if !act.valid() {
		return Digest{}, false
	}
	return act.state.permission.state.digest, true
}

func (act VerifiedAuthorityAct) PresentationID() (PresentationID, bool) {
	if !act.valid() {
		return PresentationID{}, false
	}
	return act.state.intent.state.presentationID, true
}

func (act VerifiedAuthorityAct) PresentationDigest() (Digest, bool) {
	if !act.valid() {
		return Digest{}, false
	}
	return Digest{}, false
}

func (act VerifiedAuthorityAct) AuthorityResolutionID() (AuthorityResolutionID, bool) {
	if !act.valid() {
		return AuthorityResolutionID{}, false
	}
	return act.state.intent.state.authorityResolutionID, true
}

func (act VerifiedAuthorityAct) AuthorityResolutionDigest() (Digest, bool) {
	if !act.valid() {
		return Digest{}, false
	}
	return Digest{}, false
}

func (act VerifiedAuthorityAct) SpeechActOccurredAt() (time.Time, bool) {
	if !act.valid() {
		return time.Time{}, false
	}
	return act.state.capture.state.exactUtteranceObservedAt, true
}
