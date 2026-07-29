package profileonboarding

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

const (
	profileEnactabilityPredicateValue = "A-profile-permission-enactable:v2"
	profileEvidenceClaimPrefix        = "E-profile-authorization:"
	profileCarrierClassPrefix         = "carrier-class:controlling-terminal-capture:"
	profileAuthorityBasisPrefix       = "profile-authority-basis:"
)

// ProfileAuthorization returns the source-native v43 profile proposal. It is
// design-time material only: no terminal SpeechAct, Permission, authority
// resolution, onboarding Work, or project profile exists yet.
func (prepared PreparedHaftSoftwareOnboarding) ProfileAuthorization() (
	profileauthority.PreparedAuthorization,
	bool,
) {
	if !prepared.valid() {
		return profileauthority.PreparedAuthorization{}, false
	}
	digest, ok := prepared.state.profileAuthorization.Digest()
	return prepared.state.profileAuthorization, ok && digest.String() != ""
}

func buildProfileAuthorization(
	refs dogfoodRefs,
	root projectprofile.ProjectRootV1,
	support dogfoodAuthoritySupport,
	allowedWork authority.TimeWindow,
	allowedBasis authority.TimeWindow,
	validity authority.TimeWindow,
	verifierIdentity authority.VerifierIdentity,
	verifierVersion authority.VerifierVersion,
	verificationPolicy authority.VerificationPolicyRef,
	verificationPolicyDigest authority.Digest,
) (profileauthority.PreparedAuthorization, error) {
	content, err := buildProfileAuthorizationContent(
		refs,
		root,
		support,
		allowedWork,
		allowedBasis,
		validity,
	)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, err
	}
	permissionRef, err := authority.NewPermissionRef(refs.permission)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, err
	}
	speechActRef, err := authority.NewSpeechActRef(refs.speechAct)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, err
	}
	captureRef, err := authority.NewCarrierRef(refs.authorityCapture)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, err
	}
	speechActSession, err := authority.NewSessionRef(refs.authoritySession)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, err
	}
	claimScope, err := authority.NewClaimScopeRef(refs.claimScope)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, err
	}
	predicate, err := profileauthority.NewEnactabilityPredicateRef(
		profileEnactabilityPredicateValue,
	)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, err
	}
	evidence, err := profileauthority.NewEvidenceClaimRef(
		profileEvidenceClaimPrefix + refs.token,
	)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, err
	}
	carrierClass, err := profileauthority.NewCarrierClassRef(
		profileCarrierClassPrefix + refs.token,
	)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, err
	}
	basisRef, err := profileauthority.NewBasisRef(
		profileAuthorityBasisPrefix + refs.token,
	)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, err
	}
	prepared, err := profileauthority.NewPreparedAuthorizationBuilder(
		content,
		permissionRef,
		speechActRef,
		captureRef,
	).
		InSpeechActSession(speechActSession).
		WithinClaimScope(claimScope).
		UnderEnactabilityPredicate(predicate).
		WithAdjudication(evidence, carrierClass).
		VerifiedBy(
			verifierIdentity,
			verifierVersion,
			verificationPolicy,
			verificationPolicyDigest,
		).
		AsBasis(basisRef).
		Build()
	if err != nil {
		return profileauthority.PreparedAuthorization{}, fmt.Errorf(
			"build source-native profile authorization: %w",
			err,
		)
	}
	return prepared, nil
}

func buildProfileAuthorizationContent(
	refs dogfoodRefs,
	root projectprofile.ProjectRootV1,
	support dogfoodAuthoritySupport,
	allowedWork authority.TimeWindow,
	allowedBasis authority.TimeWindow,
	validity authority.TimeWindow,
) (profileauthority.AuthorizationContent, error) {
	contentRef, err := authority.NewAuthorizationContentRef(refs.content)
	if err != nil {
		return profileauthority.AuthorizationContent{}, err
	}
	authorityRoot, err := authority.NewProjectRoot(root.String())
	if err != nil {
		return profileauthority.AuthorizationContent{}, err
	}
	assignment := support.values.RoleAssignment()
	assignmentDigest, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		return profileauthority.AuthorizationContent{}, err
	}
	authorRef, err := authority.NewRoleAssignmentRef(assignment.RoleAssignmentRef().String())
	if err != nil {
		return profileauthority.AuthorizationContent{}, err
	}
	authorDigest, err := authority.NewDigest(assignmentDigest.String())
	if err != nil {
		return profileauthority.AuthorizationContent{}, err
	}
	description := support.values.MethodDescription()
	descriptionDigest, err := projectprofile.DigestProfileOnboardingMethodDescriptionV1(description)
	if err != nil {
		return profileauthority.AuthorizationContent{}, err
	}
	descriptionRef, err := authority.NewMethodDescriptionRef(description.Ref().String())
	if err != nil {
		return profileauthority.AuthorizationContent{}, err
	}
	authorityDescriptionDigest, err := authority.NewDigest(descriptionDigest.String())
	if err != nil {
		return profileauthority.AuthorizationContent{}, err
	}
	contract := support.values.MethodContract()
	contractDigest, err := projectprofile.DigestProfileOnboardingMethodContractV1(contract)
	if err != nil {
		return profileauthority.AuthorizationContent{}, err
	}
	contractRef, err := authority.NewMethodContractRef(contract.Ref().String())
	if err != nil {
		return profileauthority.AuthorizationContent{}, err
	}
	authorityContractDigest, err := authority.NewDigest(contractDigest.String())
	if err != nil {
		return profileauthority.AuthorizationContent{}, err
	}
	classifier, err := authority.NewClassifierVersion(support.actors.classifier.String())
	if err != nil {
		return profileauthority.AuthorizationContent{}, err
	}
	policy, err := authority.NewPolicyVersion(support.actors.policy.String())
	if err != nil {
		return profileauthority.AuthorizationContent{}, err
	}
	session, err := authority.NewSessionRef(support.actors.session.String())
	if err != nil {
		return profileauthority.AuthorizationContent{}, err
	}
	singleUse, err := authority.NewSingleUseKey(refs.singleUse)
	if err != nil {
		return profileauthority.AuthorizationContent{}, err
	}
	content, err := profileauthority.NewAuthorizationContentBuilder(
		contentRef,
		authorityRoot,
	).
		ForProfileAuthor(authorRef, authorDigest).
		ForMethod(
			descriptionRef,
			authorityDescriptionDigest,
			contractRef,
			authorityContractDigest,
		).
		WithVersions(classifier, policy).
		InSession(session).
		AllowWorkWithin(allowedWork).
		AllowBasisObservationWithin(allowedBasis).
		ValidWithin(validity).
		SingleUse(singleUse).
		Build()
	if err != nil {
		return profileauthority.AuthorizationContent{}, fmt.Errorf(
			"build source-native profile authorization content: %w",
			err,
		)
	}
	return content, nil
}
