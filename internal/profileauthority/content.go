package profileauthority

import (
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/authority"
)

const authorizationContentDigestDomain = "haft.profile-authority.authorization-content/v1\x00"

type authorizationContentState struct {
	ref                     authority.AuthorizationContentRef
	projectRoot             authority.ProjectRoot
	profileAuthor           authority.RoleAssignmentRef
	profileAuthorDigest     authority.Digest
	methodDescription       authority.MethodDescriptionRef
	methodDescriptionDig    authority.Digest
	methodContract          authority.MethodContractRef
	methodContractDigest    authority.Digest
	classifierVersion       authority.ClassifierVersion
	policyVersion           authority.PolicyVersion
	sessionRef              authority.SessionRef
	allowedWork             authority.TimeWindow
	allowedBasisObservation authority.TimeWindow
	authorizationValidity   authority.TimeWindow
	singleUseKey            authority.SingleUseKey
	digest                  authority.Digest
	canonical               []byte
}

// AuthorizationContent is the exact pre-action description of one bounded
// profile-declaration permission. It contains no SpeechAct occurrence,
// terminal capture, performed onboarding Work, or admission result.
type AuthorizationContent struct {
	state *authorizationContentState
}

type AuthorizationContentBuilder struct {
	value authorizationContentState
}

func NewAuthorizationContentBuilder(
	ref authority.AuthorizationContentRef,
	projectRoot authority.ProjectRoot,
) AuthorizationContentBuilder {
	return AuthorizationContentBuilder{
		value: authorizationContentState{
			ref:         ref,
			projectRoot: projectRoot,
		},
	}
}

func (builder AuthorizationContentBuilder) ForProfileAuthor(
	ref authority.RoleAssignmentRef,
	digest authority.Digest,
) AuthorizationContentBuilder {
	builder.value.profileAuthor = ref
	builder.value.profileAuthorDigest = digest
	return builder
}

func (builder AuthorizationContentBuilder) ForMethod(
	description authority.MethodDescriptionRef,
	descriptionDigest authority.Digest,
	contract authority.MethodContractRef,
	contractDigest authority.Digest,
) AuthorizationContentBuilder {
	builder.value.methodDescription = description
	builder.value.methodDescriptionDig = descriptionDigest
	builder.value.methodContract = contract
	builder.value.methodContractDigest = contractDigest
	return builder
}

func (builder AuthorizationContentBuilder) WithVersions(
	classifier authority.ClassifierVersion,
	policy authority.PolicyVersion,
) AuthorizationContentBuilder {
	builder.value.classifierVersion = classifier
	builder.value.policyVersion = policy
	return builder
}

func (builder AuthorizationContentBuilder) InSession(
	ref authority.SessionRef,
) AuthorizationContentBuilder {
	builder.value.sessionRef = ref
	return builder
}

func (builder AuthorizationContentBuilder) AllowWorkWithin(
	window authority.TimeWindow,
) AuthorizationContentBuilder {
	builder.value.allowedWork = window
	return builder
}

func (builder AuthorizationContentBuilder) AllowBasisObservationWithin(
	window authority.TimeWindow,
) AuthorizationContentBuilder {
	builder.value.allowedBasisObservation = window
	return builder
}

func (builder AuthorizationContentBuilder) ValidWithin(
	window authority.TimeWindow,
) AuthorizationContentBuilder {
	builder.value.authorizationValidity = window
	return builder
}

func (builder AuthorizationContentBuilder) SingleUse(
	key authority.SingleUseKey,
) AuthorizationContentBuilder {
	builder.value.singleUseKey = key
	return builder
}

func (builder AuthorizationContentBuilder) Build() (AuthorizationContent, error) {
	state, err := canonicalAuthorizationContent(builder.value)
	if err != nil {
		return AuthorizationContent{}, err
	}
	return AuthorizationContent{state: &state}, nil
}

type authorizationContentJSONV1 struct {
	Schema                  string `json:"schema"`
	Ref                     string `json:"authorization_content_ref"`
	ProjectRoot             string `json:"project_root"`
	ActionKind              string `json:"action_kind"`
	ProfileAuthorRef        string `json:"profile_author_role_assignment_ref"`
	ProfileAuthorDigest     string `json:"profile_author_role_assignment_digest"`
	MethodDescriptionRef    string `json:"method_description_ref"`
	MethodDescriptionDigest string `json:"method_description_digest"`
	MethodContractRef       string `json:"method_contract_ref"`
	MethodContractDigest    string `json:"method_contract_digest"`
	ClassifierVersion       string `json:"classifier_version"`
	PolicyVersion           string `json:"policy_version"`
	SessionRef              string `json:"session_ref"`
	AllowedWorkFrom         string `json:"allowed_work_from"`
	AllowedWorkUntil        string `json:"allowed_work_until"`
	BasisObservationFrom    string `json:"basis_observation_from"`
	BasisObservationUntil   string `json:"basis_observation_until"`
	AuthorizationValidFrom  string `json:"authorization_valid_from"`
	AuthorizationValidUntil string `json:"authorization_valid_until"`
	SingleUseKey            string `json:"single_use_key"`
}

func canonicalAuthorizationContent(
	value authorizationContentState,
) (authorizationContentState, error) {
	if err := validateAuthorizationContentInputs(value); err != nil {
		return authorizationContentState{}, err
	}
	action, err := ActionKind()
	if err != nil {
		return authorizationContentState{}, err
	}
	dto := authorizationContentJSONV1{
		Schema:                  "haft.profile-authority.authorization-content/v1",
		Ref:                     value.ref.String(),
		ProjectRoot:             value.projectRoot.String(),
		ActionKind:              action.String(),
		ProfileAuthorRef:        value.profileAuthor.String(),
		ProfileAuthorDigest:     value.profileAuthorDigest.String(),
		MethodDescriptionRef:    value.methodDescription.String(),
		MethodDescriptionDigest: value.methodDescriptionDig.String(),
		MethodContractRef:       value.methodContract.String(),
		MethodContractDigest:    value.methodContractDigest.String(),
		ClassifierVersion:       value.classifierVersion.String(),
		PolicyVersion:           value.policyVersion.String(),
		SessionRef:              value.sessionRef.String(),
		AllowedWorkFrom:         formatTime(value.allowedWork.From()),
		AllowedWorkUntil:        formatTime(value.allowedWork.Until()),
		BasisObservationFrom:    formatTime(value.allowedBasisObservation.From()),
		BasisObservationUntil:   formatTime(value.allowedBasisObservation.Until()),
		AuthorizationValidFrom:  formatTime(value.authorizationValidity.From()),
		AuthorizationValidUntil: formatTime(value.authorizationValidity.Until()),
		SingleUseKey:            value.singleUseKey.String(),
	}
	digest, canonical, err := canonicalDigest(authorizationContentDigestDomain, dto)
	if err != nil {
		return authorizationContentState{}, err
	}
	value.digest = digest
	value.canonical = canonical
	return value, nil
}

func validateAuthorizationContentInputs(value authorizationContentState) error {
	checks := []struct {
		valid  bool
		detail string
	}{
		{valid: value.ref.String() != "", detail: "authorization-content ref is missing"},
		{valid: value.projectRoot.String() != "", detail: "project root is missing"},
		{valid: value.profileAuthor.String() != "", detail: "ProfileAuthor RoleAssignment is missing"},
		{valid: validDigest(value.profileAuthorDigest), detail: "ProfileAuthor RoleAssignment digest is invalid"},
		{valid: value.methodDescription.String() != "", detail: "MethodDescription ref is missing"},
		{valid: validDigest(value.methodDescriptionDig), detail: "MethodDescription digest is invalid"},
		{valid: value.methodContract.String() != "", detail: "Method contract ref is missing"},
		{valid: validDigest(value.methodContractDigest), detail: "Method contract digest is invalid"},
		{valid: value.classifierVersion.String() != "", detail: "classifier version is missing"},
		{valid: value.policyVersion.String() != "", detail: "policy version is missing"},
		{valid: value.sessionRef.String() != "", detail: "future Work session is missing"},
		{valid: !value.allowedWork.From().IsZero(), detail: "allowed Work window is invalid"},
		{valid: !value.allowedBasisObservation.From().IsZero(), detail: "basis-observation window is invalid"},
		{valid: !value.authorizationValidity.From().IsZero(), detail: "authorization validity is invalid"},
		{valid: value.singleUseKey.String() != "", detail: "single-use key is missing"},
		{valid: coveredBy(value.authorizationValidity, value.allowedWork), detail: "authorization validity must cover allowed Work"},
		{valid: coveredBy(value.authorizationValidity, value.allowedBasisObservation), detail: "authorization validity must cover basis observation"},
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

func (content AuthorizationContent) valid() bool {
	if content.state == nil {
		return false
	}
	rebuilt, err := canonicalAuthorizationContent(*content.state)
	if err != nil {
		return false
	}
	return rebuilt.digest.String() == content.state.digest.String() &&
		slices.Equal(rebuilt.canonical, content.state.canonical)
}

func (content AuthorizationContent) Ref() (authority.AuthorizationContentRef, bool) {
	if !content.valid() {
		return authority.AuthorizationContentRef{}, false
	}
	return content.state.ref, true
}

func (content AuthorizationContent) Digest() (authority.Digest, bool) {
	if !content.valid() {
		return authority.Digest{}, false
	}
	return content.state.digest, true
}

func (content AuthorizationContent) CanonicalBytes() ([]byte, bool) {
	if !content.valid() {
		return nil, false
	}
	return slices.Clone(content.state.canonical), true
}

func (content AuthorizationContent) ProjectRoot() (authority.ProjectRoot, bool) {
	if !content.valid() {
		return authority.ProjectRoot{}, false
	}
	return content.state.projectRoot, true
}

func (content AuthorizationContent) ProfileAuthor() (
	authority.RoleAssignmentRef,
	authority.Digest,
	bool,
) {
	if !content.valid() {
		return authority.RoleAssignmentRef{}, authority.Digest{}, false
	}
	return content.state.profileAuthor, content.state.profileAuthorDigest, true
}

func (content AuthorizationContent) MethodDescription() (
	authority.MethodDescriptionRef,
	authority.Digest,
	bool,
) {
	if !content.valid() {
		return authority.MethodDescriptionRef{}, authority.Digest{}, false
	}
	return content.state.methodDescription, content.state.methodDescriptionDig, true
}

func (content AuthorizationContent) MethodContract() (
	authority.MethodContractRef,
	authority.Digest,
	bool,
) {
	if !content.valid() {
		return authority.MethodContractRef{}, authority.Digest{}, false
	}
	return content.state.methodContract, content.state.methodContractDigest, true
}

func (content AuthorizationContent) ClassifierVersion() (
	authority.ClassifierVersion,
	bool,
) {
	if !content.valid() {
		return authority.ClassifierVersion{}, false
	}
	return content.state.classifierVersion, true
}

func (content AuthorizationContent) PolicyVersion() (authority.PolicyVersion, bool) {
	if !content.valid() {
		return authority.PolicyVersion{}, false
	}
	return content.state.policyVersion, true
}

func (content AuthorizationContent) FutureWorkSession() (authority.SessionRef, bool) {
	if !content.valid() {
		return authority.SessionRef{}, false
	}
	return content.state.sessionRef, true
}

func (content AuthorizationContent) AllowedWorkWindow() (authority.TimeWindow, bool) {
	if !content.valid() {
		return authority.TimeWindow{}, false
	}
	return content.state.allowedWork, true
}

func (content AuthorizationContent) AllowedBasisObservationWindow() (
	authority.TimeWindow,
	bool,
) {
	if !content.valid() {
		return authority.TimeWindow{}, false
	}
	return content.state.allowedBasisObservation, true
}

func (content AuthorizationContent) AuthorizationValidity() (authority.TimeWindow, bool) {
	if !content.valid() {
		return authority.TimeWindow{}, false
	}
	return content.state.authorizationValidity, true
}

func (content AuthorizationContent) SingleUseKey() (authority.SingleUseKey, bool) {
	if !content.valid() {
		return authority.SingleUseKey{}, false
	}
	return content.state.singleUseKey, true
}
