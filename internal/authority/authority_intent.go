package authority

import (
	"encoding/json"
	"fmt"
	"slices"
)

const (
	authorityContextPolicyDigestDomain = "haft.authority.context-policy/v1"
	authorizationContentDigestDomain   = "haft.authority.profile-declaration-content/v1"
	authorityWorkIntentDigestDomain    = "haft.authority.communicative-work-intent/v1"
	preparedSpeechActIntentDomain      = "haft.authority.prepared-speech-act-intent/v1"
	profileAuthoritySubjectDomain      = "haft.authority.profile-declaration-review-subject/v1"
	preparedAuthorityIntentDomain      = "haft.authority.prepared-intent/v1"
	profileDeclarationEffectRuleValue  = "institution-rule:authorize-institutes-profile-declaration-permission-may:v1"
)

type speechActContextPolicyState struct {
	ref               ContextPolicyRef
	digest            Digest
	boundedContext    BoundedContextRef
	recognizedActType SpeechActTypeRef
	effectRule        InstitutionalEffectRule
	canonicalJSON     []byte
}

// SpeechActContextPolicy is a reusable policy target describing which local
// terminal session may fill the authorizer role for a bounded SpeechAct. It is
// design-time policy, not a RoleAssignment or performed Work.
type SpeechActContextPolicy struct {
	state *speechActContextPolicyState
}

func NewSpeechActContextPolicy(
	ref ContextPolicyRef,
	boundedContext BoundedContextRef,
	recognizedActType SpeechActTypeRef,
	effectRule InstitutionalEffectRule,
) (SpeechActContextPolicy, error) {
	if !ref.valid() || !boundedContext.valid() || !recognizedActType.valid() || !effectRule.valid() {
		return SpeechActContextPolicy{}, fmt.Errorf("SpeechAct context policy requires canonical identities")
	}
	projection := struct {
		Schema               string `json:"schema"`
		Ref                  string `json:"ref"`
		BoundedContext       string `json:"bounded_context_ref"`
		RecognizedActType    string `json:"recognized_act_type_ref"`
		AuthorizerRole       string `json:"authorizer_role_ref"`
		HolderKind           string `json:"admitted_holder_kind"`
		AssignmentSource     string `json:"assignment_source_rule"`
		EffectRule           string `json:"institutional_effect_rule_ref"`
		InstitutedKind       string `json:"instituted_object_kind"`
		Modality             string `json:"institutional_modality"`
		ScopedAction         string `json:"scoped_action"`
		UtteranceVerb        string `json:"utterance_verb"`
		UtteranceBinding     string `json:"utterance_binding"`
		UtteranceLiteral     string `json:"utterance_literal,omitempty"`
		UtteranceDescription string `json:"utterance_description_ref"`
	}{
		Schema:               "haft.authority.speech-act-context-policy/v1",
		Ref:                  ref.String(),
		BoundedContext:       boundedContext.String(),
		RecognizedActType:    recognizedActType.String(),
		AuthorizerRole:       terminalAuthorizerRoleRefValue,
		HolderKind:           admittedHolderSystemKindValue,
		AssignmentSource:     "observed-local-controlling-terminal-session/v1",
		EffectRule:           effectRule.ref.String(),
		InstitutedKind:       effectRule.institutedObjectKind.String(),
		Modality:             effectRule.modality.String(),
		ScopedAction:         effectRule.scopedAction.String(),
		UtteranceVerb:        effectRule.utteranceRule.verb,
		UtteranceBinding:     string(effectRule.utteranceRule.binding),
		UtteranceLiteral:     effectRule.utteranceRule.literal,
		UtteranceDescription: effectRule.utteranceDescription.String(),
	}
	canonicalJSON, err := json.Marshal(projection)
	if err != nil {
		return SpeechActContextPolicy{}, fmt.Errorf("encode SpeechAct context policy: %w", err)
	}
	writer := newAuthorityDigestWriter(authorityContextPolicyDigestDomain)
	writer.add(string(canonicalJSON))
	state := speechActContextPolicyState{
		ref:               ref,
		digest:            writer.digest(),
		boundedContext:    boundedContext,
		recognizedActType: recognizedActType,
		effectRule:        effectRule,
		canonicalJSON:     canonicalJSON,
	}
	return SpeechActContextPolicy{state: &state}, nil
}

func (policy SpeechActContextPolicy) Ref() (ContextPolicyRef, bool) {
	if !policy.valid() {
		return ContextPolicyRef{}, false
	}
	return policy.state.ref, true
}

func (policy SpeechActContextPolicy) Digest() (Digest, bool) {
	if !policy.valid() {
		return Digest{}, false
	}
	return policy.state.digest, true
}

func (policy SpeechActContextPolicy) BoundedContext() (BoundedContextRef, bool) {
	if !policy.valid() {
		return BoundedContextRef{}, false
	}
	return policy.state.boundedContext, true
}

func (policy SpeechActContextPolicy) RecognizedActType() (SpeechActTypeRef, bool) {
	if !policy.valid() {
		return SpeechActTypeRef{}, false
	}
	return policy.state.recognizedActType, true
}

// InstitutedObjectKind exposes the exact institutional-effect kind carried by
// this policy so domain-specific consumers can verify category semantics
// without reconstructing or string-parsing canonical JSON.
func (policy SpeechActContextPolicy) InstitutedObjectKind() (InstitutedObjectKind, bool) {
	if !policy.valid() {
		return InstitutedObjectKind{}, false
	}
	return policy.state.effectRule.institutedObjectKind, true
}

// InstitutionalModality exposes the exact modality of the policy-owned
// institutional effect.
func (policy SpeechActContextPolicy) InstitutionalModality() (InstitutionalModality, bool) {
	if !policy.valid() {
		return InstitutionalModality{}, false
	}
	return policy.state.effectRule.modality, true
}

// ScopedAction exposes the exact action constrained by the policy-owned
// institutional effect.
func (policy SpeechActContextPolicy) ScopedAction() (ActionKind, bool) {
	if !policy.valid() {
		return ActionKind{}, false
	}
	return policy.state.effectRule.scopedAction, true
}

func (policy SpeechActContextPolicy) valid() bool {
	if policy.state == nil {
		return false
	}
	rebuilt, err := NewSpeechActContextPolicy(
		policy.state.ref,
		policy.state.boundedContext,
		policy.state.recognizedActType,
		policy.state.effectRule,
	)
	if err != nil {
		return false
	}
	return rebuilt.state.digest == policy.state.digest &&
		slices.Equal(rebuilt.state.canonicalJSON, policy.state.canonicalJSON)
}

// ProfileDeclarationContextPolicy remains a source-compatible name for the
// generic policy. Profile-specific instituted effects live in the composed
// permission layer, not in this source policy.
type ProfileDeclarationContextPolicy = SpeechActContextPolicy

func NewProfileDeclarationContextPolicy(
	ref ContextPolicyRef,
	boundedContext BoundedContextRef,
	recognizedActType SpeechActTypeRef,
) (ProfileDeclarationContextPolicy, error) {
	effectRuleRef, err := NewInstitutionalEffectRuleRef(profileDeclarationEffectRuleValue)
	if err != nil {
		return ProfileDeclarationContextPolicy{}, err
	}
	objectKind, err := NewInstitutedObjectKind("U.Commitment")
	if err != nil {
		return ProfileDeclarationContextPolicy{}, err
	}
	modality, err := NewInstitutionalModality(permissionModalityMay)
	if err != nil {
		return ProfileDeclarationContextPolicy{}, err
	}
	effectRule, err := NewInstitutionalEffectRule(
		effectRuleRef,
		objectKind,
		modality,
		ProfileDeclarationActionKind(),
		AuthorizeReviewedIntentUtteranceRule(),
		profileAuthorityUtteranceDescription(),
	)
	if err != nil {
		return ProfileDeclarationContextPolicy{}, err
	}
	return NewSpeechActContextPolicy(ref, boundedContext, recognizedActType, effectRule)
}

func profileAuthorityUtteranceDescription() UtteranceRef {
	value, _ := NewUtteranceRef("utterance:exact-terminal-authorize")
	return value
}

type profileDeclarationAuthorizationContentState struct {
	ref           AuthorizationContentRef
	digest        Digest
	envelope      AuthorizationEnvelope
	canonicalJSON []byte
}

// ProfileDeclarationAuthorizationContent is the exact bounded action
// description. It intentionally contains no candidate, Work, payload, or
// observed-basis digest: those values do not exist at authorization time.
type ProfileDeclarationAuthorizationContent struct {
	state *profileDeclarationAuthorizationContentState
}

func NewProfileDeclarationAuthorizationContent(
	ref AuthorizationContentRef,
	envelope AuthorizationEnvelope,
) (ProfileDeclarationAuthorizationContent, error) {
	if !ref.valid() {
		return ProfileDeclarationAuthorizationContent{}, fmt.Errorf("authorization-content ref is invalid")
	}
	if err := validateAuthorizationEnvelope(envelope); err != nil {
		return ProfileDeclarationAuthorizationContent{}, err
	}
	if envelope.actionKind != ProfileDeclarationActionKind() {
		return ProfileDeclarationAuthorizationContent{}, fmt.Errorf("authorization content must describe profile declaration")
	}
	projection := authorizationContentProjection{
		Schema:                 "haft.authority.profile-declaration-authorization-content/v1",
		Ref:                    ref.String(),
		ActionKind:             envelope.actionKind.String(),
		ProjectRoot:            envelope.projectRoot.String(),
		ProfileAuthor:          envelope.profileAuthor.String(),
		ProfileAuthorDigest:    envelope.profileAuthorDigest.String(),
		MethodDescription:      envelope.methodDescription.String(),
		MethodDescriptionHash:  envelope.methodDescriptionDigest.String(),
		MethodContract:         envelope.methodContract.String(),
		MethodContractHash:     envelope.methodContractDigest.String(),
		ClassifierVersion:      envelope.classifierVersion.String(),
		PolicyVersion:          envelope.policyVersion.String(),
		SessionRef:             envelope.sessionRef.String(),
		AllowedWorkFrom:        formatAuthorityTime(envelope.allowedWorkWindow.from),
		AllowedWorkUntil:       formatAuthorityTime(envelope.allowedWorkWindow.until),
		BasisObservationFrom:   formatAuthorityTime(envelope.allowedBasisObservation.from),
		BasisObservationUntil:  formatAuthorityTime(envelope.allowedBasisObservation.until),
		AuthorizationValidFrom: formatAuthorityTime(envelope.authorizationValidityWindow.from),
		AuthorizationValidTo:   formatAuthorityTime(envelope.authorizationValidityWindow.until),
		SingleUseKey:           envelope.singleUseKey.String(),
	}
	canonicalJSON, err := json.Marshal(projection)
	if err != nil {
		return ProfileDeclarationAuthorizationContent{}, fmt.Errorf("encode authorization content: %w", err)
	}
	writer := newAuthorityDigestWriter(authorizationContentDigestDomain)
	writer.add(string(canonicalJSON))
	state := profileDeclarationAuthorizationContentState{
		ref:           ref,
		digest:        writer.digest(),
		envelope:      envelope,
		canonicalJSON: canonicalJSON,
	}
	return ProfileDeclarationAuthorizationContent{state: &state}, nil
}

type authorizationContentProjection struct {
	Schema                 string `json:"schema"`
	Ref                    string `json:"ref"`
	ActionKind             string `json:"action_kind"`
	ProjectRoot            string `json:"project_root"`
	ProfileAuthor          string `json:"profile_author_role_assignment_ref"`
	ProfileAuthorDigest    string `json:"profile_author_role_assignment_digest"`
	MethodDescription      string `json:"method_description_ref"`
	MethodDescriptionHash  string `json:"method_description_digest"`
	MethodContract         string `json:"method_contract_ref"`
	MethodContractHash     string `json:"method_contract_digest"`
	ClassifierVersion      string `json:"classifier_version"`
	PolicyVersion          string `json:"policy_version"`
	SessionRef             string `json:"session_ref"`
	AllowedWorkFrom        string `json:"allowed_work_from"`
	AllowedWorkUntil       string `json:"allowed_work_until"`
	BasisObservationFrom   string `json:"basis_observation_from"`
	BasisObservationUntil  string `json:"basis_observation_until"`
	AuthorizationValidFrom string `json:"authorization_valid_from"`
	AuthorizationValidTo   string `json:"authorization_valid_until"`
	SingleUseKey           string `json:"single_use_key"`
}

func (content ProfileDeclarationAuthorizationContent) Ref() (AuthorizationContentRef, bool) {
	if !content.valid() {
		return AuthorizationContentRef{}, false
	}
	return content.state.ref, true
}

func (content ProfileDeclarationAuthorizationContent) Digest() (Digest, bool) {
	if !content.valid() {
		return Digest{}, false
	}
	return content.state.digest, true
}

func (content ProfileDeclarationAuthorizationContent) Envelope() (AuthorizationEnvelope, bool) {
	if !content.valid() {
		return AuthorizationEnvelope{}, false
	}
	return content.state.envelope, true
}

func (content ProfileDeclarationAuthorizationContent) valid() bool {
	if content.state == nil {
		return false
	}
	rebuilt, err := NewProfileDeclarationAuthorizationContent(
		content.state.ref,
		content.state.envelope,
	)
	if err != nil {
		return false
	}
	return rebuilt.state.digest == content.state.digest &&
		slices.Equal(rebuilt.state.canonicalJSON, content.state.canonicalJSON)
}

// SpeechActExecutionFrame describes the anchors a later manual authority act
// must satisfy. It is design-time intent, not performed Work or a SpeechAct.
type SpeechActExecutionFrame struct {
	state *speechActExecutionFrameState
}

type speechActExecutionFrameState struct {
	methodDescription       SpeechActMethodDescription
	methodRef               MethodRef
	methodDescriptionRef    MethodDescriptionRef
	methodDescriptionDigest Digest
	executedWithin          SystemRef
	statePlane              StatePlaneRef
	deltaPredicate          DeltaPredicateRef
	outcome                 WorkOutcomeRef
	utterance               UtteranceRef
	parameters              []WorkParameterBinding
	resources               []WorkResourceRef
	affected                []AffectedRef
	digest                  Digest
}

type SpeechActExecutionFrameBuilder struct {
	value speechActExecutionFrameState
}

func NewSpeechActExecutionFrameBuilder(
	methodDescription SpeechActMethodDescription,
) SpeechActExecutionFrameBuilder {
	methodRef := MethodRef{}
	methodDescriptionRef := MethodDescriptionRef{}
	methodDescriptionDigest := Digest{}
	if methodDescription.state != nil {
		methodRef = methodDescription.state.methodRef
		methodDescriptionRef = methodDescription.state.ref
		methodDescriptionDigest = methodDescription.state.digest
	}
	return SpeechActExecutionFrameBuilder{value: speechActExecutionFrameState{
		methodDescription:       methodDescription,
		methodRef:               methodRef,
		methodDescriptionRef:    methodDescriptionRef,
		methodDescriptionDigest: methodDescriptionDigest,
	}}
}

func (builder SpeechActExecutionFrameBuilder) ExecutedWithin(
	systemRef SystemRef,
) SpeechActExecutionFrameBuilder {
	builder.value.executedWithin = systemRef
	return builder
}

func (builder SpeechActExecutionFrameBuilder) OnStatePlane(
	statePlane StatePlaneRef,
	deltaPredicate DeltaPredicateRef,
) SpeechActExecutionFrameBuilder {
	builder.value.statePlane = statePlane
	builder.value.deltaPredicate = deltaPredicate
	return builder
}

func (builder SpeechActExecutionFrameBuilder) WithOutcome(
	outcome WorkOutcomeRef,
) SpeechActExecutionFrameBuilder {
	builder.value.outcome = outcome
	return builder
}

func (builder SpeechActExecutionFrameBuilder) WithUtteranceDescription(
	utterance UtteranceRef,
) SpeechActExecutionFrameBuilder {
	builder.value.utterance = utterance
	return builder
}

func (builder SpeechActExecutionFrameBuilder) BindParameter(
	binding WorkParameterBinding,
) SpeechActExecutionFrameBuilder {
	builder.value.parameters = append(builder.value.parameters, binding)
	return builder
}

func (builder SpeechActExecutionFrameBuilder) UseResource(
	ref WorkResourceRef,
) SpeechActExecutionFrameBuilder {
	builder.value.resources = append(builder.value.resources, ref)
	return builder
}

func (builder SpeechActExecutionFrameBuilder) Affect(
	ref AffectedRef,
) SpeechActExecutionFrameBuilder {
	builder.value.affected = append(builder.value.affected, ref)
	return builder
}

func (builder SpeechActExecutionFrameBuilder) Build() (SpeechActExecutionFrame, error) {
	value, err := canonicalSpeechActExecutionFrame(builder.value)
	if err != nil {
		return SpeechActExecutionFrame{}, err
	}
	return SpeechActExecutionFrame{state: &value}, nil
}

func canonicalSpeechActExecutionFrame(
	value speechActExecutionFrameState,
) (speechActExecutionFrameState, error) {
	identityValid := value.methodDescription.valid() &&
		value.methodRef.valid() &&
		value.methodDescriptionRef.valid() &&
		value.methodDescriptionDigest.valid() &&
		value.executedWithin.valid() &&
		value.statePlane.valid() &&
		value.deltaPredicate.valid() &&
		value.outcome.valid() &&
		value.utterance.valid()
	if !identityValid {
		return speechActExecutionFrameState{}, fmt.Errorf("SpeechAct execution-frame anchors are invalid")
	}
	description := value.methodDescription.state
	descriptionMatches := description.methodRef == value.methodRef &&
		description.ref == value.methodDescriptionRef &&
		description.digest == value.methodDescriptionDigest
	if !descriptionMatches {
		return speechActExecutionFrameState{}, fmt.Errorf("SpeechAct execution frame does not bind its exact MethodDescription source")
	}
	parameters, err := canonicalWorkParameters(value.parameters)
	if err != nil {
		return speechActExecutionFrameState{}, err
	}
	resources, err := canonicalWorkResources(value.resources)
	if err != nil {
		return speechActExecutionFrameState{}, err
	}
	affected, err := canonicalAffectedRefs(value.affected)
	if err != nil {
		return speechActExecutionFrameState{}, err
	}
	value.parameters = parameters
	value.resources = resources
	value.affected = affected
	writer := newAuthorityDigestWriter(authorityWorkIntentDigestDomain)
	writer.add(string(description.canonicalJSON))
	writer.add(value.methodRef.String())
	writer.add(value.methodDescriptionRef.String())
	writer.add(value.methodDescriptionDigest.String())
	writer.add(value.executedWithin.String())
	writer.add(value.statePlane.String())
	writer.add(value.deltaPredicate.String())
	writer.add(value.outcome.String())
	writer.add(value.utterance.String())
	addWorkParameterDigestValues(writer, value.parameters)
	addWorkResourceDigestValues(writer, value.resources)
	addAffectedDigestValues(writer, value.affected)
	value.digest = writer.digest()
	return value, nil
}

func (frame SpeechActExecutionFrame) valid() bool {
	if frame.state == nil {
		return false
	}
	rebuilt, err := canonicalSpeechActExecutionFrame(*frame.state)
	return err == nil && rebuilt.digest == frame.state.digest
}

func canonicalWorkParameters(values []WorkParameterBinding) ([]WorkParameterBinding, error) {
	result := append([]WorkParameterBinding{}, values...)
	if len(result) == 0 || slices.ContainsFunc(result, func(value WorkParameterBinding) bool {
		return !value.valid()
	}) {
		return nil, fmt.Errorf("communicative authority Work requires non-empty canonical parameter bindings")
	}
	slices.SortFunc(result, func(left WorkParameterBinding, right WorkParameterBinding) int {
		return compareStrings(left.name, right.name)
	})
	if hasAdjacentWorkParameterDuplicate(result, 1) {
		return nil, fmt.Errorf("communicative authority Work parameter names must be unique")
	}
	return result, nil
}

func hasAdjacentWorkParameterDuplicate(
	values []WorkParameterBinding,
	index int,
) bool {
	if index >= len(values) {
		return false
	}
	if values[index-1].name == values[index].name {
		return true
	}
	return hasAdjacentWorkParameterDuplicate(values, index+1)
}

func canonicalWorkResources(values []WorkResourceRef) ([]WorkResourceRef, error) {
	result := append([]WorkResourceRef{}, values...)
	if len(result) == 0 || slices.ContainsFunc(result, func(value WorkResourceRef) bool {
		return !value.valid()
	}) {
		return nil, fmt.Errorf("communicative authority Work requires non-empty canonical resources")
	}
	slices.SortFunc(result, func(left WorkResourceRef, right WorkResourceRef) int {
		return compareStrings(left.String(), right.String())
	})
	result = slices.Compact(result)
	return result, nil
}

func canonicalAffectedRefs(values []AffectedRef) ([]AffectedRef, error) {
	result := append([]AffectedRef{}, values...)
	if len(result) == 0 || slices.ContainsFunc(result, func(value AffectedRef) bool {
		return !value.valid()
	}) {
		return nil, fmt.Errorf("communicative authority Work requires non-empty canonical affected refs")
	}
	slices.SortFunc(result, func(left AffectedRef, right AffectedRef) int {
		return compareStrings(left.String(), right.String())
	})
	result = slices.Compact(result)
	return result, nil
}

func compareStrings(left string, right string) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func addWorkParameterDigestValues(
	writer authorityDigestWriter,
	values []WorkParameterBinding,
) {
	for _, value := range values {
		writer.add(value.name)
		writer.add(value.value)
	}
}

func addWorkResourceDigestValues(writer authorityDigestWriter, values []WorkResourceRef) {
	for _, value := range values {
		writer.add(value.String())
	}
}

func addAffectedDigestValues(writer authorityDigestWriter, values []AffectedRef) {
	for _, value := range values {
		writer.add(value.String())
	}
}

// PreparedAuthorityIntent is the legacy profile-declaration adapter layered on
// PreparedSpeechActIntent. Despite its historical name, it is not the reusable
// contract for decisions, commissions, specification lifecycle acts, or other
// institutional effects. New domains compose PreparedSpeechActIntent directly.
type PreparedAuthorityIntent struct {
	state *preparedAuthorityIntentState
}

type preparedAuthorityIntentState struct {
	presentationID           PresentationID
	authorityResolutionID    AuthorityResolutionID
	speechActRef             SpeechActRef
	permissionRef            PermissionRef
	captureCarrierRef        CarrierRef
	speechActSessionRef      SessionRef
	content                  ProfileDeclarationAuthorizationContent
	contextPolicy            SpeechActContextPolicy
	executionFrame           SpeechActExecutionFrame
	permissionPredicateRef   ProfileAdmissionPredicateRef
	claimScopeRef            ClaimScopeRef
	verifierIdentity         VerifierIdentity
	verifierVersion          VerifierVersion
	verificationPolicyRef    VerificationPolicyRef
	verificationPolicyDigest Digest
	evidenceRelationRef      VerificationEvidenceRelationRef
	carrierExpectationRef    VerificationCarrierExpectationRef
	resolutionWindow         TimeWindow
	reviewSubjectDigest      Digest
	sourceIntent             PreparedSpeechActIntent
	digest                   Digest
}

type PreparedAuthorityIntentBuilder struct {
	value preparedAuthorityIntentState
}

func NewPreparedAuthorityIntentBuilder(
	presentationID PresentationID,
	authorityResolutionID AuthorityResolutionID,
	speechActRef SpeechActRef,
	permissionRef PermissionRef,
	captureCarrierRef CarrierRef,
) PreparedAuthorityIntentBuilder {
	return PreparedAuthorityIntentBuilder{value: preparedAuthorityIntentState{
		presentationID:        presentationID,
		authorityResolutionID: authorityResolutionID,
		speechActRef:          speechActRef,
		permissionRef:         permissionRef,
		captureCarrierRef:     captureCarrierRef,
	}}
}

func (builder PreparedAuthorityIntentBuilder) WithAuthorizationContent(
	content ProfileDeclarationAuthorizationContent,
) PreparedAuthorityIntentBuilder {
	builder.value.content = content
	return builder
}

func (builder PreparedAuthorityIntentBuilder) InSpeechActSession(
	ref SessionRef,
) PreparedAuthorityIntentBuilder {
	builder.value.speechActSessionRef = ref
	return builder
}

func (builder PreparedAuthorityIntentBuilder) UnderContextPolicy(
	policy SpeechActContextPolicy,
) PreparedAuthorityIntentBuilder {
	builder.value.contextPolicy = policy
	return builder
}

func (builder PreparedAuthorityIntentBuilder) WithSpeechActExecutionFrame(
	frame SpeechActExecutionFrame,
) PreparedAuthorityIntentBuilder {
	builder.value.executionFrame = frame
	return builder
}

func (builder PreparedAuthorityIntentBuilder) ScopedBy(
	ref ProfileAdmissionPredicateRef,
) PreparedAuthorityIntentBuilder {
	builder.value.permissionPredicateRef = ref
	return builder
}

func (builder PreparedAuthorityIntentBuilder) WithinClaimScope(
	ref ClaimScopeRef,
) PreparedAuthorityIntentBuilder {
	builder.value.claimScopeRef = ref
	return builder
}

func (builder PreparedAuthorityIntentBuilder) VerifiedBy(
	identity VerifierIdentity,
	version VerifierVersion,
) PreparedAuthorityIntentBuilder {
	builder.value.verifierIdentity = identity
	builder.value.verifierVersion = version
	return builder
}

func (builder PreparedAuthorityIntentBuilder) UnderVerificationPolicy(
	ref VerificationPolicyRef,
	digest Digest,
) PreparedAuthorityIntentBuilder {
	builder.value.verificationPolicyRef = ref
	builder.value.verificationPolicyDigest = digest
	return builder
}

func (builder PreparedAuthorityIntentBuilder) WithAdjudicationEvidence(
	relationRef VerificationEvidenceRelationRef,
	carrierExpectationRef VerificationCarrierExpectationRef,
) PreparedAuthorityIntentBuilder {
	builder.value.evidenceRelationRef = relationRef
	builder.value.carrierExpectationRef = carrierExpectationRef
	return builder
}

func (builder PreparedAuthorityIntentBuilder) ResolutionEffectiveWithin(
	window TimeWindow,
) PreparedAuthorityIntentBuilder {
	builder.value.resolutionWindow = window
	return builder
}

func (builder PreparedAuthorityIntentBuilder) Build() (PreparedAuthorityIntent, error) {
	value, err := canonicalPreparedAuthorityIntent(builder.value)
	if err != nil {
		return PreparedAuthorityIntent{}, err
	}
	return PreparedAuthorityIntent{state: &value}, nil
}

func canonicalPreparedAuthorityIntent(
	value preparedAuthorityIntentState,
) (preparedAuthorityIntentState, error) {
	identitiesValid := value.presentationID.valid() &&
		value.authorityResolutionID.valid() &&
		value.speechActRef.valid() &&
		value.permissionRef.valid() &&
		value.captureCarrierRef.valid() &&
		value.speechActSessionRef.valid() &&
		value.permissionPredicateRef.valid() &&
		value.claimScopeRef.valid() &&
		value.verifierIdentity.valid() &&
		value.verifierVersion.valid() &&
		value.verificationPolicyRef.valid() &&
		value.verificationPolicyDigest.valid() &&
		value.evidenceRelationRef.valid() &&
		value.carrierExpectationRef.valid()
	if !identitiesValid {
		return preparedAuthorityIntentState{}, fmt.Errorf("prepared authority intent identities are invalid")
	}
	if !value.content.valid() || !value.contextPolicy.valid() || !value.executionFrame.valid() {
		return preparedAuthorityIntentState{}, fmt.Errorf("prepared authority intent typed targets are invalid")
	}
	manualDescription := ManualAuthorityIssueMethodDescription()
	frameDescription := value.executionFrame.state.methodDescription
	manualMethodMatches := manualDescription.valid() &&
		frameDescription.valid() &&
		manualDescription.state.digest == frameDescription.state.digest &&
		frameDescription.state.boundedContext == value.contextPolicy.state.boundedContext
	if !manualMethodMatches {
		return preparedAuthorityIntentState{}, fmt.Errorf("profile authority intent requires the sealed ManualAuthorityIssue MethodDescription in the same bounded context")
	}
	if !value.resolutionWindow.valid() {
		return preparedAuthorityIntentState{}, fmt.Errorf("prepared authority intent resolution window is invalid")
	}
	envelope := value.content.state.envelope
	effectRule := value.contextPolicy.state.effectRule
	legacyRule := AuthorizeReviewedIntentUtteranceRule()
	humanReadableAuthorization := effectRule.utteranceRule.binding == utteranceBindsLiteral &&
		effectRule.utteranceRule.verb == "AUTHORIZE"
	policyLicensesPermission := effectRule.institutedObjectKind.String() == "U.Commitment" &&
		effectRule.modality.String() == permissionModalityMay &&
		effectRule.scopedAction == envelope.actionKind &&
		(effectRule.utteranceRule == legacyRule || humanReadableAuthorization)
	if !policyLicensesPermission {
		return preparedAuthorityIntentState{}, fmt.Errorf("context policy does not license the exact profile-declaration MAY permission")
	}
	if value.speechActSessionRef == envelope.sessionRef {
		return preparedAuthorityIntentState{}, fmt.Errorf("manual SpeechAct session must be distinct from future profile-onboarding Work session")
	}
	resolutionWithinAuthorization := envelope.authorizationValidityWindow.Contains(value.resolutionWindow.from) &&
		!value.resolutionWindow.until.After(envelope.authorizationValidityWindow.until)
	if !resolutionWithinAuthorization {
		return preparedAuthorityIntentState{}, fmt.Errorf("authority resolution window must be inside authorization content validity")
	}
	reviewSubjectDigest := profileAuthorityReviewSubjectDigest(value)
	reviewSubjectRef, err := NewSpeechActReviewSubjectRef(
		"profile-authority-review-subject:" + value.presentationID.String(),
	)
	if err != nil {
		return preparedAuthorityIntentState{}, err
	}
	institutedObjectRef, err := NewInstitutedObjectRef(value.permissionRef.String())
	if err != nil {
		return preparedAuthorityIntentState{}, err
	}
	sourceIntent, err := NewPreparedSpeechActIntentBuilder(
		value.speechActRef,
		value.captureCarrierRef,
	).
		ForProject(envelope.projectRoot).
		InSession(value.speechActSessionRef).
		Reviewing(reviewSubjectRef, reviewSubjectDigest).
		Institutes(institutedObjectRef).
		UnderContextPolicy(value.contextPolicy).
		WithExecutionFrame(value.executionFrame).
		Build()
	if err != nil {
		return preparedAuthorityIntentState{}, err
	}
	value.reviewSubjectDigest = reviewSubjectDigest
	value.sourceIntent = sourceIntent
	writer := newAuthorityDigestWriter(preparedAuthorityIntentDomain)
	writer.add(reviewSubjectDigest.String())
	writer.add(sourceIntent.state.digest.String())
	value.digest = writer.digest()
	return value, nil
}

func profileAuthorityReviewSubjectDigest(value preparedAuthorityIntentState) Digest {
	writer := newAuthorityDigestWriter(profileAuthoritySubjectDomain)
	writer.add(value.presentationID.String())
	writer.add(value.authorityResolutionID.String())
	writer.add(value.permissionRef.String())
	writer.add(value.speechActSessionRef.String())
	writer.add(value.content.state.ref.String())
	writer.add(value.content.state.digest.String())
	writer.add(value.permissionPredicateRef.String())
	writer.add(value.claimScopeRef.String())
	writer.add(value.verifierIdentity.String())
	writer.add(value.verifierVersion.String())
	writer.add(value.verificationPolicyRef.String())
	writer.add(value.verificationPolicyDigest.String())
	writer.add(value.evidenceRelationRef.String())
	writer.add(value.carrierExpectationRef.String())
	writer.add(formatAuthorityTime(value.resolutionWindow.from))
	writer.add(formatAuthorityTime(value.resolutionWindow.until))
	return writer.digest()
}

func (intent PreparedAuthorityIntent) Digest() (Digest, bool) {
	if !intent.valid() {
		return Digest{}, false
	}
	return intent.state.digest, true
}

func (intent PreparedAuthorityIntent) AuthorizationContent() (
	ProfileDeclarationAuthorizationContent,
	bool,
) {
	if !intent.valid() {
		return ProfileDeclarationAuthorizationContent{}, false
	}
	return intent.state.content, true
}

func (intent PreparedAuthorityIntent) SpeechActIntent() (PreparedSpeechActIntent, bool) {
	if !intent.valid() {
		return PreparedSpeechActIntent{}, false
	}
	return intent.state.sourceIntent, true
}

func (intent PreparedAuthorityIntent) valid() bool {
	if intent.state == nil {
		return false
	}
	rebuilt, err := canonicalPreparedAuthorityIntent(*intent.state)
	return err == nil && rebuilt.digest == intent.state.digest
}

// AuthorityIntentReviewDigest binds human-readable review text to the exact
// pre-act intent. It intentionally does not claim that a SpeechAct already
// exists; the real TTY capture creates that later Work occurrence.
func AuthorityIntentReviewDigest(
	intent PreparedAuthorityIntent,
	reviewText string,
) (Digest, error) {
	if !intent.valid() {
		return Digest{}, fmt.Errorf("prepared authority intent is invalid")
	}
	if err := validateAuthorityIssueReviewText(reviewText); err != nil {
		return Digest{}, err
	}
	return SpeechActIntentReviewDigest(intent.state.sourceIntent, reviewText)
}
