package profileauthority

import (
	"fmt"
	"slices"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
)

const (
	authorityResolutionDigestDomain = "haft.profile-authority.authority-resolution/v2\x00"
	actionEnvelopeDigestDomain      = "haft.profile-authority.action-envelope/v2\x00"
	projectBindingDigestDomain      = "haft.profile-authority.project-binding/v2\x00"
	roleStateRelationValue          = "A.2.5.EnactableStateAdmission"
	enactableStateValue             = "profile-declaration-permission-current"
	currentnessResultValue          = "current"
	predicateResultValue            = "satisfied"
	admissionResultValue            = "admitted"
)

// AuthorityResolutionRecord is one immutable, context-local evaluation of an
// existing ProfileDeclarationPermission at CheckedAt. It records an
// A.2.5-style enactable-state admission result; it is not another commitment,
// a U.Kind, performed Work, a capability, the four-ref basis, or the later
// profile admission.
//
// There is deliberately no exported record builder or wire decoder. A record
// is minted only as the New branch of EvaluateNewResolution before Work. A
// persistence adapter can reconstruct one from the exact source closure and
// stored checked_at, compare its digest and canonical bytes, then request a
// post-Work replay evaluation that may mint the consumable use.
type AuthorityResolutionRecord struct {
	state *authorityResolutionState
}

type authorityResolutionState struct {
	ref       ProfileDeclarationAuthorityResolutionRef
	closure   Closure
	checkedAt time.Time
	digest    authority.Digest
	canonical []byte
}

type authorityResolutionSnapshot struct {
	ref                      ProfileDeclarationAuthorityResolutionRef
	basisRef                 BasisRef
	basisDigest              authority.Digest
	speechActRef             authority.SpeechActRef
	speechActDigest          authority.Digest
	contentRef               authority.AuthorizationContentRef
	contentDigest            authority.Digest
	permissionRef            authority.PermissionRef
	permissionDigest         authority.Digest
	contextPolicyRef         authority.ContextPolicyRef
	contextPolicyDigest      authority.Digest
	actionKind               authority.ActionKind
	projectRoot              authority.ProjectRoot
	actionEnvelopeDigest     authority.Digest
	projectBindingDigest     authority.Digest
	profileAuthorRef         authority.RoleAssignmentRef
	profileAuthorDigest      authority.Digest
	claimScope               authority.ClaimScopeRef
	boundedContext           authority.BoundedContextRef
	methodDescription        authority.MethodDescriptionRef
	methodDescriptionDigest  authority.Digest
	methodContract           authority.MethodContractRef
	methodContractDigest     authority.Digest
	classifierVersion        authority.ClassifierVersion
	policyVersion            authority.PolicyVersion
	futureWorkSession        authority.SessionRef
	allowedWork              authority.TimeWindow
	allowedBasisObservation  authority.TimeWindow
	authorizationValidity    authority.TimeWindow
	permissionValidity       authority.TimeWindow
	singleUseKey             authority.SingleUseKey
	enactabilityPredicate    EnactabilityPredicateRef
	verifierIdentity         authority.VerifierIdentity
	verifierVersion          authority.VerifierVersion
	verificationPolicy       authority.VerificationPolicyRef
	verificationPolicyDigest authority.Digest
	checkedAt                time.Time
	digest                   authority.Digest
	canonical                []byte
}

type profileActionEnvelopeJSONV2 struct {
	AuthorizationContentRef    string `json:"authorization_content_ref"`
	AuthorizationContentDigest string `json:"authorization_content_digest"`
	ActionKind                 string `json:"action_kind"`
	ProjectRoot                string `json:"project_root"`
	ProfileAuthorRef           string `json:"profile_author_role_assignment_ref"`
	ProfileAuthorDigest        string `json:"profile_author_role_assignment_digest"`
	MethodDescriptionRef       string `json:"method_description_ref"`
	MethodDescriptionDigest    string `json:"method_description_digest"`
	MethodContractRef          string `json:"method_contract_ref"`
	MethodContractDigest       string `json:"method_contract_digest"`
	ClassifierVersion          string `json:"classifier_version"`
	PolicyVersion              string `json:"policy_version"`
	FutureWorkSessionRef       string `json:"future_work_session_ref"`
	AllowedWorkFrom            string `json:"allowed_work_from"`
	AllowedWorkUntil           string `json:"allowed_work_until"`
	BasisObservationFrom       string `json:"basis_observation_from"`
	BasisObservationUntil      string `json:"basis_observation_until"`
	AuthorizationValidFrom     string `json:"authorization_valid_from"`
	AuthorizationValidUntil    string `json:"authorization_valid_until"`
	SingleUseKey               string `json:"single_use_key"`
}

type profileProjectBindingJSONV2 struct {
	ProjectRoot                string `json:"project_root"`
	ActionKind                 string `json:"action_kind"`
	AuthorizationContentRef    string `json:"authorization_content_ref"`
	AuthorizationContentDigest string `json:"authorization_content_digest"`
}

type authorityResolutionJSONV2 struct {
	Schema                     string `json:"schema"`
	AuthorityResolutionRef     string `json:"authority_resolution_ref"`
	AuthorityBasisRef          string `json:"authority_basis_ref"`
	AuthorityBasisDigest       string `json:"authority_basis_digest"`
	SpeechActRef               string `json:"speech_act_ref"`
	SpeechActDigest            string `json:"speech_act_digest"`
	AuthorizationContentRef    string `json:"authorization_content_ref"`
	AuthorizationContentDigest string `json:"authorization_content_digest"`
	PermissionRef              string `json:"permission_ref"`
	PermissionDigest           string `json:"permission_digest"`
	ContextPolicyRef           string `json:"context_policy_ref"`
	ContextPolicyDigest        string `json:"context_policy_digest"`
	ActionKind                 string `json:"action_kind"`
	ProjectRoot                string `json:"project_root"`
	ActionEnvelopeDigest       string `json:"action_envelope_digest"`
	ProjectBindingDigest       string `json:"project_binding_digest"`
	ProfileAuthorRef           string `json:"profile_author_role_assignment_ref"`
	ProfileAuthorDigest        string `json:"profile_author_role_assignment_digest"`
	ClaimScopeRef              string `json:"claim_scope_ref"`
	BoundedContextRef          string `json:"bounded_context_ref"`
	MethodDescriptionRef       string `json:"method_description_ref"`
	MethodDescriptionDigest    string `json:"method_description_digest"`
	MethodContractRef          string `json:"method_contract_ref"`
	MethodContractDigest       string `json:"method_contract_digest"`
	ClassifierVersion          string `json:"classifier_version"`
	PolicyVersion              string `json:"policy_version"`
	FutureWorkSessionRef       string `json:"future_work_session_ref"`
	AllowedWorkFrom            string `json:"allowed_work_from"`
	AllowedWorkUntil           string `json:"allowed_work_until"`
	BasisObservationFrom       string `json:"basis_observation_from"`
	BasisObservationUntil      string `json:"basis_observation_until"`
	AuthorizationValidFrom     string `json:"authorization_valid_from"`
	AuthorizationValidUntil    string `json:"authorization_valid_until"`
	PermissionValidFrom        string `json:"permission_valid_from"`
	PermissionValidUntil       string `json:"permission_valid_until"`
	SingleUseKey               string `json:"single_use_key"`
	EnactabilityPredicateRef   string `json:"enactability_predicate_ref"`
	VerifierIdentity           string `json:"verifier_identity"`
	VerifierVersion            string `json:"verifier_version"`
	VerificationPolicyRef      string `json:"verification_policy_ref"`
	VerificationPolicyDigest   string `json:"verification_policy_digest"`
	CheckedAt                  string `json:"checked_at"`
	RoleStateRelation          string `json:"role_state_relation"`
	EnactableState             string `json:"enactable_state"`
	CurrentnessResult          string `json:"currentness_result"`
	PredicateResult            string `json:"predicate_result"`
	AdmissionResult            string `json:"admission_result"`
}

func canonicalAuthorityResolution(
	ref ProfileDeclarationAuthorityResolutionRef,
	closure Closure,
	checkedAt time.Time,
) (authorityResolutionSnapshot, error) {
	if !ref.valid() {
		return authorityResolutionSnapshot{}, fmt.Errorf(
			"profile authority resolution ref is invalid",
		)
	}
	if !closure.valid() {
		return authorityResolutionSnapshot{}, fmt.Errorf(
			"profile authority resolution requires an exact source closure",
		)
	}
	canonicalCheckedAt := canonicalTime(checkedAt)
	if canonicalCheckedAt.IsZero() {
		return authorityResolutionSnapshot{}, fmt.Errorf(
			"profile authority resolution checked_at is invalid",
		)
	}
	permission := closure.permission
	if !permission.state.validity.Contains(canonicalCheckedAt) {
		return authorityResolutionSnapshot{}, fmt.Errorf(
			"profile declaration MAY permission is not current at checked_at",
		)
	}
	content := permission.state.prepared.state.content
	basis := closure.basis
	basisRef, _ := basis.Ref()
	basisDigest, _ := basis.Digest()
	speechActRef, speechActDigest, _ := basis.SpeechAct()
	contentRef, contentDigest, _ := basis.AuthorizationContent()
	permissionRef, permissionDigest, _ := basis.Permission()
	contextPolicyRef, contextPolicyDigest, _ := basis.ContextPolicy()
	actionKind, err := ActionKind()
	if err != nil {
		return authorityResolutionSnapshot{}, err
	}
	actionEnvelopeDigest, err := resolutionActionEnvelopeDigest(content, actionKind)
	if err != nil {
		return authorityResolutionSnapshot{}, err
	}
	projectBindingDigest, err := resolutionProjectBindingDigest(
		content,
		actionKind,
	)
	if err != nil {
		return authorityResolutionSnapshot{}, err
	}
	dto := authorityResolutionJSONV2{
		Schema:                     "haft.profile-authority.authority-resolution/v2",
		AuthorityResolutionRef:     ref.String(),
		AuthorityBasisRef:          basisRef.String(),
		AuthorityBasisDigest:       basisDigest.String(),
		SpeechActRef:               speechActRef.String(),
		SpeechActDigest:            speechActDigest.String(),
		AuthorizationContentRef:    contentRef.String(),
		AuthorizationContentDigest: contentDigest.String(),
		PermissionRef:              permissionRef.String(),
		PermissionDigest:           permissionDigest.String(),
		ContextPolicyRef:           contextPolicyRef.String(),
		ContextPolicyDigest:        contextPolicyDigest.String(),
		ActionKind:                 actionKind.String(),
		ProjectRoot:                content.state.projectRoot.String(),
		ActionEnvelopeDigest:       actionEnvelopeDigest.String(),
		ProjectBindingDigest:       projectBindingDigest.String(),
		ProfileAuthorRef:           content.state.profileAuthor.String(),
		ProfileAuthorDigest:        content.state.profileAuthorDigest.String(),
		ClaimScopeRef:              permission.state.claimScope.String(),
		BoundedContextRef:          permission.state.boundedContext.String(),
		MethodDescriptionRef:       content.state.methodDescription.String(),
		MethodDescriptionDigest:    content.state.methodDescriptionDig.String(),
		MethodContractRef:          content.state.methodContract.String(),
		MethodContractDigest:       content.state.methodContractDigest.String(),
		ClassifierVersion:          content.state.classifierVersion.String(),
		PolicyVersion:              content.state.policyVersion.String(),
		FutureWorkSessionRef:       content.state.sessionRef.String(),
		AllowedWorkFrom:            formatTime(content.state.allowedWork.From()),
		AllowedWorkUntil:           formatTime(content.state.allowedWork.Until()),
		BasisObservationFrom:       formatTime(content.state.allowedBasisObservation.From()),
		BasisObservationUntil:      formatTime(content.state.allowedBasisObservation.Until()),
		AuthorizationValidFrom:     formatTime(content.state.authorizationValidity.From()),
		AuthorizationValidUntil:    formatTime(content.state.authorizationValidity.Until()),
		PermissionValidFrom:        formatTime(permission.state.validity.From()),
		PermissionValidUntil:       formatTime(permission.state.validity.Until()),
		SingleUseKey:               content.state.singleUseKey.String(),
		EnactabilityPredicateRef:   permission.state.enactabilityPredicate.String(),
		VerifierIdentity:           permission.state.prepared.state.verifierIdentity.String(),
		VerifierVersion:            permission.state.prepared.state.verifierVersion.String(),
		VerificationPolicyRef:      permission.state.prepared.state.verificationPolicy.String(),
		VerificationPolicyDigest:   permission.state.prepared.state.verificationPolicyDigest.String(),
		CheckedAt:                  formatTime(canonicalCheckedAt),
		RoleStateRelation:          roleStateRelationValue,
		EnactableState:             enactableStateValue,
		CurrentnessResult:          currentnessResultValue,
		PredicateResult:            predicateResultValue,
		AdmissionResult:            admissionResultValue,
	}
	digest, canonical, err := canonicalDigest(authorityResolutionDigestDomain, dto)
	if err != nil {
		return authorityResolutionSnapshot{}, err
	}
	return authorityResolutionSnapshot{
		ref:                      ref,
		basisRef:                 basisRef,
		basisDigest:              basisDigest,
		speechActRef:             speechActRef,
		speechActDigest:          speechActDigest,
		contentRef:               contentRef,
		contentDigest:            contentDigest,
		permissionRef:            permissionRef,
		permissionDigest:         permissionDigest,
		contextPolicyRef:         contextPolicyRef,
		contextPolicyDigest:      contextPolicyDigest,
		actionKind:               actionKind,
		projectRoot:              content.state.projectRoot,
		actionEnvelopeDigest:     actionEnvelopeDigest,
		projectBindingDigest:     projectBindingDigest,
		profileAuthorRef:         content.state.profileAuthor,
		profileAuthorDigest:      content.state.profileAuthorDigest,
		claimScope:               permission.state.claimScope,
		boundedContext:           permission.state.boundedContext,
		methodDescription:        content.state.methodDescription,
		methodDescriptionDigest:  content.state.methodDescriptionDig,
		methodContract:           content.state.methodContract,
		methodContractDigest:     content.state.methodContractDigest,
		classifierVersion:        content.state.classifierVersion,
		policyVersion:            content.state.policyVersion,
		futureWorkSession:        content.state.sessionRef,
		allowedWork:              content.state.allowedWork,
		allowedBasisObservation:  content.state.allowedBasisObservation,
		authorizationValidity:    content.state.authorizationValidity,
		permissionValidity:       permission.state.validity,
		singleUseKey:             content.state.singleUseKey,
		enactabilityPredicate:    permission.state.enactabilityPredicate,
		verifierIdentity:         permission.state.prepared.state.verifierIdentity,
		verifierVersion:          permission.state.prepared.state.verifierVersion,
		verificationPolicy:       permission.state.prepared.state.verificationPolicy,
		verificationPolicyDigest: permission.state.prepared.state.verificationPolicyDigest,
		checkedAt:                canonicalCheckedAt,
		digest:                   digest,
		canonical:                canonical,
	}, nil
}

func resolutionActionEnvelopeDigest(
	content AuthorizationContent,
	action authority.ActionKind,
) (authority.Digest, error) {
	contentRef, _ := content.Ref()
	contentDigest, _ := content.Digest()
	dto := profileActionEnvelopeJSONV2{
		AuthorizationContentRef:    contentRef.String(),
		AuthorizationContentDigest: contentDigest.String(),
		ActionKind:                 action.String(),
		ProjectRoot:                content.state.projectRoot.String(),
		ProfileAuthorRef:           content.state.profileAuthor.String(),
		ProfileAuthorDigest:        content.state.profileAuthorDigest.String(),
		MethodDescriptionRef:       content.state.methodDescription.String(),
		MethodDescriptionDigest:    content.state.methodDescriptionDig.String(),
		MethodContractRef:          content.state.methodContract.String(),
		MethodContractDigest:       content.state.methodContractDigest.String(),
		ClassifierVersion:          content.state.classifierVersion.String(),
		PolicyVersion:              content.state.policyVersion.String(),
		FutureWorkSessionRef:       content.state.sessionRef.String(),
		AllowedWorkFrom:            formatTime(content.state.allowedWork.From()),
		AllowedWorkUntil:           formatTime(content.state.allowedWork.Until()),
		BasisObservationFrom:       formatTime(content.state.allowedBasisObservation.From()),
		BasisObservationUntil:      formatTime(content.state.allowedBasisObservation.Until()),
		AuthorizationValidFrom:     formatTime(content.state.authorizationValidity.From()),
		AuthorizationValidUntil:    formatTime(content.state.authorizationValidity.Until()),
		SingleUseKey:               content.state.singleUseKey.String(),
	}
	digest, _, err := canonicalDigest(actionEnvelopeDigestDomain, dto)
	return digest, err
}

func resolutionProjectBindingDigest(
	content AuthorizationContent,
	action authority.ActionKind,
) (authority.Digest, error) {
	contentRef, _ := content.Ref()
	contentDigest, _ := content.Digest()
	dto := profileProjectBindingJSONV2{
		ProjectRoot:                content.state.projectRoot.String(),
		ActionKind:                 action.String(),
		AuthorizationContentRef:    contentRef.String(),
		AuthorizationContentDigest: contentDigest.String(),
	}
	digest, _, err := canonicalDigest(projectBindingDigestDomain, dto)
	return digest, err
}

func newAuthorityResolutionRecord(
	ref ProfileDeclarationAuthorityResolutionRef,
	closure Closure,
	checkedAt time.Time,
) (AuthorityResolutionRecord, error) {
	snapshot, err := canonicalAuthorityResolution(ref, closure, checkedAt)
	if err != nil {
		return AuthorityResolutionRecord{}, err
	}
	state := authorityResolutionState{
		ref:       ref,
		closure:   closure,
		checkedAt: snapshot.checkedAt,
		digest:    snapshot.digest,
		canonical: slices.Clone(snapshot.canonical),
	}
	return AuthorityResolutionRecord{state: &state}, nil
}

func (record AuthorityResolutionRecord) snapshot() (
	authorityResolutionSnapshot,
	bool,
) {
	if record.state == nil {
		return authorityResolutionSnapshot{}, false
	}
	snapshot, err := canonicalAuthorityResolution(
		record.state.ref,
		record.state.closure,
		record.state.checkedAt,
	)
	if err != nil {
		return authorityResolutionSnapshot{}, false
	}
	matches := snapshot.digest.String() == record.state.digest.String()
	matches = matches && slices.Equal(snapshot.canonical, record.state.canonical)
	return snapshot, matches
}

func (record AuthorityResolutionRecord) Ref() (
	ProfileDeclarationAuthorityResolutionRef,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.ref, ok
}

func (record AuthorityResolutionRecord) Digest() (authority.Digest, bool) {
	snapshot, ok := record.snapshot()
	return snapshot.digest, ok
}

func (record AuthorityResolutionRecord) CanonicalBytes() ([]byte, bool) {
	snapshot, ok := record.snapshot()
	if !ok {
		return nil, false
	}
	return slices.Clone(snapshot.canonical), true
}

func (record AuthorityResolutionRecord) Basis() (
	BasisRef,
	authority.Digest,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.basisRef, snapshot.basisDigest, ok
}

func (record AuthorityResolutionRecord) SpeechAct() (
	authority.SpeechActRef,
	authority.Digest,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.speechActRef, snapshot.speechActDigest, ok
}

func (record AuthorityResolutionRecord) AuthorizationContent() (
	authority.AuthorizationContentRef,
	authority.Digest,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.contentRef, snapshot.contentDigest, ok
}

func (record AuthorityResolutionRecord) Permission() (
	authority.PermissionRef,
	authority.Digest,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.permissionRef, snapshot.permissionDigest, ok
}

func (record AuthorityResolutionRecord) ContextPolicy() (
	authority.ContextPolicyRef,
	authority.Digest,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.contextPolicyRef, snapshot.contextPolicyDigest, ok
}

func (record AuthorityResolutionRecord) ProjectBinding() (
	authority.ProjectRoot,
	authority.ActionKind,
	authority.Digest,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.projectRoot,
		snapshot.actionKind,
		snapshot.projectBindingDigest,
		ok
}

func (record AuthorityResolutionRecord) ActionEnvelopeDigest() (
	authority.Digest,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.actionEnvelopeDigest, ok
}

func (record AuthorityResolutionRecord) ProfileAuthor() (
	authority.RoleAssignmentRef,
	authority.Digest,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.profileAuthorRef, snapshot.profileAuthorDigest, ok
}

func (record AuthorityResolutionRecord) ClaimScope() (
	authority.ClaimScopeRef,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.claimScope, ok
}

func (record AuthorityResolutionRecord) BoundedContext() (
	authority.BoundedContextRef,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.boundedContext, ok
}

func (record AuthorityResolutionRecord) MethodDescription() (
	authority.MethodDescriptionRef,
	authority.Digest,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.methodDescription, snapshot.methodDescriptionDigest, ok
}

func (record AuthorityResolutionRecord) MethodContract() (
	authority.MethodContractRef,
	authority.Digest,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.methodContract, snapshot.methodContractDigest, ok
}

func (record AuthorityResolutionRecord) Versions() (
	authority.ClassifierVersion,
	authority.PolicyVersion,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.classifierVersion, snapshot.policyVersion, ok
}

func (record AuthorityResolutionRecord) FutureWorkSession() (
	authority.SessionRef,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.futureWorkSession, ok
}

func (record AuthorityResolutionRecord) AllowedWorkWindow() (
	authority.TimeWindow,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.allowedWork, ok
}

func (record AuthorityResolutionRecord) AllowedBasisObservationWindow() (
	authority.TimeWindow,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.allowedBasisObservation, ok
}

func (record AuthorityResolutionRecord) AuthorizationValidity() (
	authority.TimeWindow,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.authorizationValidity, ok
}

func (record AuthorityResolutionRecord) PermissionValidity() (
	authority.TimeWindow,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.permissionValidity, ok
}

func (record AuthorityResolutionRecord) SingleUseKey() (
	authority.SingleUseKey,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.singleUseKey, ok
}

func (record AuthorityResolutionRecord) EnactabilityPredicate() (
	EnactabilityPredicateRef,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.enactabilityPredicate, ok
}

func (record AuthorityResolutionRecord) Verifier() (
	authority.VerifierIdentity,
	authority.VerifierVersion,
	authority.VerificationPolicyRef,
	authority.Digest,
	bool,
) {
	snapshot, ok := record.snapshot()
	return snapshot.verifierIdentity,
		snapshot.verifierVersion,
		snapshot.verificationPolicy,
		snapshot.verificationPolicyDigest,
		ok
}

func (record AuthorityResolutionRecord) CheckedAt() (time.Time, bool) {
	snapshot, ok := record.snapshot()
	return snapshot.checkedAt, ok
}

func (record AuthorityResolutionRecord) CurrentAtCheckedAt() bool {
	_, ok := record.snapshot()
	return ok
}

func (record AuthorityResolutionRecord) PredicateSatisfied() bool {
	_, ok := record.snapshot()
	return ok
}

func (record AuthorityResolutionRecord) Admitted() bool {
	_, ok := record.snapshot()
	return ok
}
