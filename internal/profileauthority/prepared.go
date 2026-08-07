package profileauthority

import (
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/authority"
)

const preparedAuthorizationDigestDomain = "haft.profile-authority.prepared-authorization/v1\x00"

type preparedAuthorizationState struct {
	content                  AuthorizationContent
	permissionRef            authority.PermissionRef
	speechActRef             authority.SpeechActRef
	captureRef               authority.CarrierRef
	speechActSession         authority.SessionRef
	claimScope               authority.ClaimScopeRef
	enactabilityPredicate    EnactabilityPredicateRef
	evidenceClaim            EvidenceClaimRef
	carrierClass             CarrierClassRef
	verifierIdentity         authority.VerifierIdentity
	verifierVersion          authority.VerifierVersion
	verificationPolicy       authority.VerificationPolicyRef
	verificationPolicyDigest authority.Digest
	basisRef                 BasisRef
	policy                   authority.SpeechActContextPolicy
	review                   ReviewCard
	speechActIntent          authority.PreparedSpeechActIntent
	manualSpeechAct          authority.PreparedManualSpeechAct
	digest                   authority.Digest
	canonical                []byte
}

// PreparedAuthorization is profile-owned design-time material. It may be
// presented to the generic terminal SpeechAct capture boundary, but it is not
// an act, permission, basis, or admission.
type PreparedAuthorization struct {
	state *preparedAuthorizationState
}

type PreparedAuthorizationBuilder struct {
	value preparedAuthorizationState
}

func NewPreparedAuthorizationBuilder(
	content AuthorizationContent,
	permissionRef authority.PermissionRef,
	speechActRef authority.SpeechActRef,
	captureRef authority.CarrierRef,
) PreparedAuthorizationBuilder {
	return PreparedAuthorizationBuilder{
		value: preparedAuthorizationState{
			content:       content,
			permissionRef: permissionRef,
			speechActRef:  speechActRef,
			captureRef:    captureRef,
		},
	}
}

func (builder PreparedAuthorizationBuilder) InSpeechActSession(
	ref authority.SessionRef,
) PreparedAuthorizationBuilder {
	builder.value.speechActSession = ref
	return builder
}

func (builder PreparedAuthorizationBuilder) WithinClaimScope(
	ref authority.ClaimScopeRef,
) PreparedAuthorizationBuilder {
	builder.value.claimScope = ref
	return builder
}

func (builder PreparedAuthorizationBuilder) UnderEnactabilityPredicate(
	ref EnactabilityPredicateRef,
) PreparedAuthorizationBuilder {
	builder.value.enactabilityPredicate = ref
	return builder
}

func (builder PreparedAuthorizationBuilder) WithAdjudication(
	evidence EvidenceClaimRef,
	carrierClass CarrierClassRef,
) PreparedAuthorizationBuilder {
	builder.value.evidenceClaim = evidence
	builder.value.carrierClass = carrierClass
	return builder
}

func (builder PreparedAuthorizationBuilder) VerifiedBy(
	identity authority.VerifierIdentity,
	version authority.VerifierVersion,
	policy authority.VerificationPolicyRef,
	policyDigest authority.Digest,
) PreparedAuthorizationBuilder {
	builder.value.verifierIdentity = identity
	builder.value.verifierVersion = version
	builder.value.verificationPolicy = policy
	builder.value.verificationPolicyDigest = policyDigest
	return builder
}

func (builder PreparedAuthorizationBuilder) AsBasis(
	ref BasisRef,
) PreparedAuthorizationBuilder {
	builder.value.basisRef = ref
	return builder
}

func (builder PreparedAuthorizationBuilder) Build() (PreparedAuthorization, error) {
	state, err := canonicalPreparedAuthorization(builder.value)
	if err != nil {
		return PreparedAuthorization{}, err
	}
	return PreparedAuthorization{state: &state}, nil
}

type preparedAuthorizationJSONV1 struct {
	Schema                   string `json:"schema"`
	ContentRef               string `json:"authorization_content_ref"`
	ContentDigest            string `json:"authorization_content_digest"`
	PermissionRef            string `json:"permission_ref"`
	SpeechActRef             string `json:"speech_act_ref"`
	CaptureRef               string `json:"capture_carrier_ref"`
	SpeechActSession         string `json:"speech_act_session_ref"`
	ClaimScopeRef            string `json:"claim_scope_ref"`
	EnactabilityPredicateRef string `json:"enactability_predicate_ref"`
	EvidenceClaimRef         string `json:"evidence_claim_ref"`
	CarrierClassRef          string `json:"carrier_class_ref"`
	VerifierIdentity         string `json:"verifier_identity"`
	VerifierVersion          string `json:"verifier_version"`
	VerificationPolicyRef    string `json:"verification_policy_ref"`
	VerificationPolicyDigest string `json:"verification_policy_digest"`
	BasisRef                 string `json:"basis_ref"`
	ContextPolicyRef         string `json:"context_policy_ref"`
	ContextPolicyDigest      string `json:"context_policy_digest"`
	SpeechActIntentDigest    string `json:"speech_act_intent_digest"`
}

func canonicalPreparedAuthorization(
	value preparedAuthorizationState,
) (preparedAuthorizationState, error) {
	if err := validatePreparedAuthorizationInputs(value); err != nil {
		return preparedAuthorizationState{}, err
	}
	policy, err := ContextPolicy()
	if err != nil {
		return preparedAuthorizationState{}, err
	}
	review, err := NewReviewCard(value.content)
	if err != nil {
		return preparedAuthorizationState{}, err
	}
	frame, err := profileSpeechActExecutionFrame(value.content, value.permissionRef)
	if err != nil {
		return preparedAuthorizationState{}, err
	}
	contentRef, _ := value.content.Ref()
	subjectRef, err := authority.NewSpeechActReviewSubjectRef(contentRef.String())
	if err != nil {
		return preparedAuthorizationState{}, err
	}
	contentDigest, _ := value.content.Digest()
	institutedRef, err := authority.NewInstitutedObjectRef(value.permissionRef.String())
	if err != nil {
		return preparedAuthorizationState{}, err
	}
	root, _ := value.content.ProjectRoot()
	intent, err := authority.NewPreparedSpeechActIntentBuilder(
		value.speechActRef,
		value.captureRef,
	).
		ForProject(root).
		InSession(value.speechActSession).
		Reviewing(subjectRef, contentDigest).
		Institutes(institutedRef).
		UnderContextPolicy(policy).
		WithExecutionFrame(frame).
		Build()
	if err != nil {
		return preparedAuthorizationState{}, err
	}
	reviewText, _ := review.Text()
	manual, err := authority.PrepareManualSpeechAct(intent, reviewText)
	if err != nil {
		return preparedAuthorizationState{}, err
	}
	policyRef, _ := policy.Ref()
	policyDigest, _ := policy.Digest()
	intentDigest, _ := intent.Digest()
	dto := preparedAuthorizationJSONV1{
		Schema:                   "haft.profile-authority.prepared-authorization/v1",
		ContentRef:               contentRef.String(),
		ContentDigest:            contentDigest.String(),
		PermissionRef:            value.permissionRef.String(),
		SpeechActRef:             value.speechActRef.String(),
		CaptureRef:               value.captureRef.String(),
		SpeechActSession:         value.speechActSession.String(),
		ClaimScopeRef:            value.claimScope.String(),
		EnactabilityPredicateRef: value.enactabilityPredicate.String(),
		EvidenceClaimRef:         value.evidenceClaim.String(),
		CarrierClassRef:          value.carrierClass.String(),
		VerifierIdentity:         value.verifierIdentity.String(),
		VerifierVersion:          value.verifierVersion.String(),
		VerificationPolicyRef:    value.verificationPolicy.String(),
		VerificationPolicyDigest: value.verificationPolicyDigest.String(),
		BasisRef:                 value.basisRef.String(),
		ContextPolicyRef:         policyRef.String(),
		ContextPolicyDigest:      policyDigest.String(),
		SpeechActIntentDigest:    intentDigest.String(),
	}
	digest, canonical, err := canonicalDigest(preparedAuthorizationDigestDomain, dto)
	if err != nil {
		return preparedAuthorizationState{}, err
	}
	value.policy = policy
	value.review = review
	value.speechActIntent = intent
	value.manualSpeechAct = manual
	value.digest = digest
	value.canonical = canonical
	return value, nil
}

func validatePreparedAuthorizationInputs(value preparedAuthorizationState) error {
	contentSession := authority.SessionRef{}
	if value.content.state != nil {
		contentSession = value.content.state.sessionRef
	}
	checks := []struct {
		valid  bool
		detail string
	}{
		{valid: value.content.valid(), detail: "authorization content is invalid"},
		{valid: value.permissionRef.String() != "", detail: "permission ref is missing"},
		{valid: value.speechActRef.String() != "", detail: "SpeechAct ref is missing"},
		{valid: value.captureRef.String() != "", detail: "capture ref is missing"},
		{valid: value.speechActSession.String() != "", detail: "SpeechAct session is missing"},
		{valid: value.speechActSession.String() != contentSession.String(), detail: "SpeechAct and future Work sessions must differ"},
		{valid: value.claimScope.String() != "", detail: "claim scope is missing"},
		{valid: value.enactabilityPredicate.valid(), detail: "A-* enactability predicate is missing"},
		{valid: value.evidenceClaim.valid(), detail: "E-* evidence claim is missing"},
		{valid: value.carrierClass.valid(), detail: "carrier class is missing"},
		{valid: value.verifierIdentity.String() != "", detail: "verifier identity is missing"},
		{valid: value.verifierVersion.String() != "", detail: "verifier version is missing"},
		{valid: value.verificationPolicy.String() != "", detail: "verification policy is missing"},
		{valid: validDigest(value.verificationPolicyDigest), detail: "verification policy digest is invalid"},
		{valid: value.basisRef.valid(), detail: "four-ref basis ref is missing"},
	}
	invalid := slices.IndexFunc(checks, func(check struct {
		valid  bool
		detail string
	}) bool {
		return !check.valid
	})
	if invalid >= 0 {
		return fmt.Errorf("%s", checks[invalid].detail)
	}
	return nil
}

func profileSpeechActExecutionFrame(
	content AuthorizationContent,
	permissionRef authority.PermissionRef,
) (authority.SpeechActExecutionFrame, error) {
	method, err := profileSpeechActMethodDescription()
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	system, err := authority.NewSystemRef("system:haft-profile-authority")
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	statePlane, err := authority.NewStatePlaneRef("state-plane:profile-declaration-permission")
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	delta, err := authority.NewDeltaPredicateRef("delta-predicate:profile-permission-instituted")
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	outcome, err := authority.NewWorkOutcomeRef("work-outcome:profile-permission-instituted")
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	utterance, err := authority.NewUtteranceRef(profileUtteranceValue)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	contentDigest, _ := content.Digest()
	parameter, err := authority.NewWorkParameterBinding(
		"parameter:authorization-content-digest",
		contentDigest.String(),
	)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	resource, err := authority.NewWorkResourceRef("resource:controlling-terminal")
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	permissionAffected, err := authority.NewAffectedRef(
		"affected:profile-permission:" + permissionRef.String(),
	)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	contentAffected, err := authority.NewAffectedRef(
		"affected:profile-authorization-content:" + contentDigest.String(),
	)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	return authority.NewSpeechActExecutionFrameBuilder(method).
		ExecutedWithin(system).
		OnStatePlane(statePlane, delta).
		WithOutcome(outcome).
		WithUtteranceDescription(utterance).
		BindParameter(parameter).
		UseResource(resource).
		Affect(permissionAffected).
		Affect(contentAffected).
		Build()
}

func profileSpeechActMethodDescription() (
	authority.SpeechActMethodDescription,
	error,
) {
	methodRef, err := authority.NewMethodRef("method:profile-declaration-authorization")
	if err != nil {
		return authority.SpeechActMethodDescription{}, err
	}
	descriptionRef, err := authority.NewMethodDescriptionRef(
		"method-description:profile-declaration-authorization:v2",
	)
	if err != nil {
		return authority.SpeechActMethodDescription{}, err
	}
	procedureRef, err := authority.NewMethodProcedureRef(
		"procedure:review-profile-intent-capture-controlling-terminal:v2",
	)
	if err != nil {
		return authority.SpeechActMethodDescription{}, err
	}
	boundedContext, err := authority.NewBoundedContextRef(profileBoundedContextValue)
	if err != nil {
		return authority.SpeechActMethodDescription{}, err
	}
	return authority.NewManualControllingTTYMethodDescription(
		methodRef,
		descriptionRef,
		procedureRef,
		boundedContext,
	)
}

func (prepared PreparedAuthorization) valid() bool {
	if prepared.state == nil {
		return false
	}
	rebuilt, err := canonicalPreparedAuthorization(*prepared.state)
	if err != nil {
		return false
	}
	return rebuilt.digest.String() == prepared.state.digest.String() &&
		slices.Equal(rebuilt.canonical, prepared.state.canonical)
}

func (prepared PreparedAuthorization) ManualSpeechAct() (
	authority.PreparedManualSpeechAct,
	bool,
) {
	if !prepared.valid() {
		return authority.PreparedManualSpeechAct{}, false
	}
	return prepared.state.manualSpeechAct, true
}

func (prepared PreparedAuthorization) SpeechActIntent() (
	authority.PreparedSpeechActIntent,
	bool,
) {
	if !prepared.valid() {
		return authority.PreparedSpeechActIntent{}, false
	}
	return prepared.state.speechActIntent, true
}

func (prepared PreparedAuthorization) ReviewCard() (ReviewCard, bool) {
	if !prepared.valid() {
		return ReviewCard{}, false
	}
	return prepared.state.review, true
}

func (prepared PreparedAuthorization) Digest() (authority.Digest, bool) {
	if !prepared.valid() {
		return authority.Digest{}, false
	}
	return prepared.state.digest, true
}

// CanonicalBytes returns the immutable preparation carrier used by durable
// adapters. It does not expose, recreate, or complete a SpeechAct.
func (prepared PreparedAuthorization) CanonicalBytes() ([]byte, bool) {
	if !prepared.valid() {
		return nil, false
	}
	return slices.Clone(prepared.state.canonical), true
}

func (prepared PreparedAuthorization) Content() (AuthorizationContent, bool) {
	if !prepared.valid() {
		return AuthorizationContent{}, false
	}
	return prepared.state.content, true
}

func (prepared PreparedAuthorization) PermissionRef() (authority.PermissionRef, bool) {
	if !prepared.valid() {
		return authority.PermissionRef{}, false
	}
	return prepared.state.permissionRef, true
}

func (prepared PreparedAuthorization) SpeechActRef() (authority.SpeechActRef, bool) {
	if !prepared.valid() {
		return authority.SpeechActRef{}, false
	}
	return prepared.state.speechActRef, true
}

func (prepared PreparedAuthorization) CaptureRef() (authority.CarrierRef, bool) {
	if !prepared.valid() {
		return authority.CarrierRef{}, false
	}
	return prepared.state.captureRef, true
}

func (prepared PreparedAuthorization) SpeechActSession() (authority.SessionRef, bool) {
	if !prepared.valid() {
		return authority.SessionRef{}, false
	}
	return prepared.state.speechActSession, true
}

func (prepared PreparedAuthorization) ClaimScope() (authority.ClaimScopeRef, bool) {
	if !prepared.valid() {
		return authority.ClaimScopeRef{}, false
	}
	return prepared.state.claimScope, true
}

func (prepared PreparedAuthorization) EnactabilityPredicate() (
	EnactabilityPredicateRef,
	bool,
) {
	if !prepared.valid() {
		return EnactabilityPredicateRef{}, false
	}
	return prepared.state.enactabilityPredicate, true
}

func (prepared PreparedAuthorization) EvidenceClaim() (EvidenceClaimRef, bool) {
	if !prepared.valid() {
		return EvidenceClaimRef{}, false
	}
	return prepared.state.evidenceClaim, true
}

func (prepared PreparedAuthorization) CarrierClass() (CarrierClassRef, bool) {
	if !prepared.valid() {
		return CarrierClassRef{}, false
	}
	return prepared.state.carrierClass, true
}

func (prepared PreparedAuthorization) Verifier() (
	authority.VerifierIdentity,
	authority.VerifierVersion,
	authority.VerificationPolicyRef,
	authority.Digest,
	bool,
) {
	if !prepared.valid() {
		return authority.VerifierIdentity{},
			authority.VerifierVersion{},
			authority.VerificationPolicyRef{},
			authority.Digest{},
			false
	}
	return prepared.state.verifierIdentity,
		prepared.state.verifierVersion,
		prepared.state.verificationPolicy,
		prepared.state.verificationPolicyDigest,
		true
}

func (prepared PreparedAuthorization) BasisRef() (BasisRef, bool) {
	if !prepared.valid() {
		return BasisRef{}, false
	}
	return prepared.state.basisRef, true
}

func (prepared PreparedAuthorization) ContextPolicy() (
	authority.SpeechActContextPolicy,
	bool,
) {
	if !prepared.valid() {
		return authority.SpeechActContextPolicy{}, false
	}
	return prepared.state.policy, true
}
