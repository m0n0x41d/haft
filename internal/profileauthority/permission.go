package profileauthority

import (
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/authority"
)

const profilePermissionDigestDomain = "haft.profile-authority.permission/v2\x00"

type permissionState struct {
	prepared              PreparedAuthorization
	source                authority.RecordedSpeechActSource
	ref                   authority.PermissionRef
	digest                authority.Digest
	projectRoot           authority.ProjectRoot
	subjectRef            authority.RoleAssignmentRef
	subjectDigest         authority.Digest
	claimScope            authority.ClaimScopeRef
	boundedContext        authority.BoundedContextRef
	validity              authority.TimeWindow
	contentRef            authority.AuthorizationContentRef
	contentDigest         authority.Digest
	methodDescription     authority.MethodDescriptionRef
	methodDigest          authority.Digest
	enactabilityPredicate EnactabilityPredicateRef
	evidenceClaims        []EvidenceClaimRef
	carrierClasses        []CarrierClassRef
	speechActRef          authority.SpeechActRef
	speechActDigest       authority.Digest
	captureRef            authority.CarrierRef
	captureDigest         authority.Digest
	contextPolicyRef      authority.ContextPolicyRef
	contextPolicyDig      authority.Digest
	canonical             []byte
}

// Permission is the profile-domain U.Commitment(MAY) instituted by one exact
// recorded SpeechAct. Evidence ClaimIdRefs and carrier-class refs remain typed;
// the actual terminal capture is separate source provenance.
type Permission struct {
	state *permissionState
}

type profilePermissionJSONV2 struct {
	Schema                   string   `json:"schema"`
	PermissionRef            string   `json:"permission_ref"`
	ProjectRoot              string   `json:"project_root"`
	SubjectRef               string   `json:"subject_role_assignment_ref"`
	SubjectDigest            string   `json:"subject_role_assignment_digest"`
	Modality                 string   `json:"modality"`
	ActionKind               string   `json:"action_kind"`
	ClaimScopeRef            string   `json:"claim_scope_ref"`
	BoundedContextRef        string   `json:"bounded_context_ref"`
	ValidFrom                string   `json:"valid_from"`
	ValidUntil               string   `json:"valid_until"`
	Referents                []string `json:"referents"`
	AuthorizationContentRef  string   `json:"authorization_content_ref"`
	AuthorizationContentDig  string   `json:"authorization_content_digest"`
	MethodDescriptionRef     string   `json:"method_description_ref"`
	MethodDescriptionDigest  string   `json:"method_description_digest"`
	EnactabilityPredicateRef string   `json:"enactability_predicate_ref"`
	EvidenceClaimRefs        []string `json:"adjudication_evidence_claim_refs"`
	CarrierClassRefs         []string `json:"adjudication_carrier_class_refs"`
	VerifierIdentity         string   `json:"adjudication_verifier_identity"`
	VerifierVersion          string   `json:"adjudication_verifier_version"`
	EvaluationPolicyRef      string   `json:"adjudication_evaluation_policy_ref"`
	EvaluationPolicyDigest   string   `json:"adjudication_evaluation_policy_digest"`
	SourceSpeechActRef       string   `json:"source_speech_act_ref"`
	SourceSpeechActDigest    string   `json:"source_speech_act_digest"`
	ContextPolicyRef         string   `json:"context_policy_ref"`
	ContextPolicyDigest      string   `json:"context_policy_digest"`
	CaptureCarrierRef        string   `json:"instituting_terminal_capture_carrier_ref"`
	CaptureCarrierDigest     string   `json:"instituting_terminal_capture_carrier_digest"`
}

func NewPermission(
	prepared PreparedAuthorization,
	source authority.RecordedSpeechActSource,
) (Permission, error) {
	state, err := canonicalPermission(prepared, source)
	if err != nil {
		return Permission{}, err
	}
	return Permission{state: &state}, nil
}

func canonicalPermission(
	prepared PreparedAuthorization,
	source authority.RecordedSpeechActSource,
) (permissionState, error) {
	bindings, err := exactRecordedSourceBindings(prepared, source)
	if err != nil {
		return permissionState{}, err
	}
	content := prepared.state.content
	validity, _ := content.AuthorizationValidity()
	permissionWindow, err := authority.NewTimeWindow(
		bindings.completedAt,
		validity.Until(),
	)
	if err != nil {
		return permissionState{}, fmt.Errorf(
			"profile permission has no valid post-SpeechAct window: %w",
			err,
		)
	}
	contentRef, _ := content.Ref()
	contentDigest, _ := content.Digest()
	subjectRef, subjectDigest, _ := content.ProfileAuthor()
	methodRef, methodDigest, _ := content.MethodDescription()
	boundedContext, _ := prepared.state.policy.BoundedContext()
	action, err := ActionKind()
	if err != nil {
		return permissionState{}, err
	}
	referents := []string{
		methodRef.String(),
		prepared.state.enactabilityPredicate.String(),
	}
	evidenceClaims := []string{prepared.state.evidenceClaim.String()}
	carrierClasses := []string{prepared.state.carrierClass.String()}
	dto := profilePermissionJSONV2{
		Schema:                   "haft.profile-authority.permission/v2",
		PermissionRef:            prepared.state.permissionRef.String(),
		ProjectRoot:              bindings.projectRoot.String(),
		SubjectRef:               subjectRef.String(),
		SubjectDigest:            subjectDigest.String(),
		Modality:                 "MAY",
		ActionKind:               action.String(),
		ClaimScopeRef:            prepared.state.claimScope.String(),
		BoundedContextRef:        boundedContext.String(),
		ValidFrom:                formatTime(permissionWindow.From()),
		ValidUntil:               formatTime(permissionWindow.Until()),
		Referents:                referents,
		AuthorizationContentRef:  contentRef.String(),
		AuthorizationContentDig:  contentDigest.String(),
		MethodDescriptionRef:     methodRef.String(),
		MethodDescriptionDigest:  methodDigest.String(),
		EnactabilityPredicateRef: prepared.state.enactabilityPredicate.String(),
		EvidenceClaimRefs:        evidenceClaims,
		CarrierClassRefs:         carrierClasses,
		VerifierIdentity:         prepared.state.verifierIdentity.String(),
		VerifierVersion:          prepared.state.verifierVersion.String(),
		EvaluationPolicyRef:      prepared.state.verificationPolicy.String(),
		EvaluationPolicyDigest:   prepared.state.verificationPolicyDigest.String(),
		SourceSpeechActRef:       bindings.speechActRef.String(),
		SourceSpeechActDigest:    bindings.speechActDigest.String(),
		ContextPolicyRef:         bindings.contextPolicyRef.String(),
		ContextPolicyDigest:      bindings.contextPolicyDig.String(),
		CaptureCarrierRef:        bindings.captureRef.String(),
		CaptureCarrierDigest:     bindings.captureDigest.String(),
	}
	digest, canonical, err := canonicalDigest(profilePermissionDigestDomain, dto)
	if err != nil {
		return permissionState{}, err
	}
	state := permissionState{
		prepared:              prepared,
		source:                source,
		ref:                   prepared.state.permissionRef,
		digest:                digest,
		projectRoot:           bindings.projectRoot,
		subjectRef:            subjectRef,
		subjectDigest:         subjectDigest,
		claimScope:            prepared.state.claimScope,
		boundedContext:        boundedContext,
		validity:              permissionWindow,
		contentRef:            contentRef,
		contentDigest:         contentDigest,
		methodDescription:     methodRef,
		methodDigest:          methodDigest,
		enactabilityPredicate: prepared.state.enactabilityPredicate,
		evidenceClaims:        []EvidenceClaimRef{prepared.state.evidenceClaim},
		carrierClasses:        []CarrierClassRef{prepared.state.carrierClass},
		speechActRef:          bindings.speechActRef,
		speechActDigest:       bindings.speechActDigest,
		captureRef:            bindings.captureRef,
		captureDigest:         bindings.captureDigest,
		contextPolicyRef:      bindings.contextPolicyRef,
		contextPolicyDig:      bindings.contextPolicyDig,
		canonical:             canonical,
	}
	return state, nil
}

func (permission Permission) valid() bool {
	if permission.state == nil {
		return false
	}
	rebuilt, err := canonicalPermission(
		permission.state.prepared,
		permission.state.source,
	)
	if err != nil {
		return false
	}
	return rebuilt.digest.String() == permission.state.digest.String() &&
		slices.Equal(rebuilt.canonical, permission.state.canonical)
}

func (permission Permission) Ref() (authority.PermissionRef, bool) {
	if !permission.valid() {
		return authority.PermissionRef{}, false
	}
	return permission.state.ref, true
}

func (permission Permission) Digest() (authority.Digest, bool) {
	if !permission.valid() {
		return authority.Digest{}, false
	}
	return permission.state.digest, true
}

func (permission Permission) CanonicalBytes() ([]byte, bool) {
	if !permission.valid() {
		return nil, false
	}
	return slices.Clone(permission.state.canonical), true
}

func (permission Permission) EvidenceClaims() ([]EvidenceClaimRef, bool) {
	if !permission.valid() {
		return nil, false
	}
	return slices.Clone(permission.state.evidenceClaims), true
}

func (permission Permission) CarrierClasses() ([]CarrierClassRef, bool) {
	if !permission.valid() {
		return nil, false
	}
	return slices.Clone(permission.state.carrierClasses), true
}

func (permission Permission) CaptureCarrier() (
	authority.CarrierRef,
	authority.Digest,
	bool,
) {
	if !permission.valid() {
		return authority.CarrierRef{}, authority.Digest{}, false
	}
	return permission.state.captureRef, permission.state.captureDigest, true
}

func (permission Permission) SourceSpeechAct() (
	authority.SpeechActRef,
	authority.Digest,
	bool,
) {
	if !permission.valid() {
		return authority.SpeechActRef{}, authority.Digest{}, false
	}
	return permission.state.speechActRef, permission.state.speechActDigest, true
}
