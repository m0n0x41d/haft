package profileonboarding

import (
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/projectprofile"
	projectprofilesqlite "github.com/m0n0x41d/haft/internal/projectprofile/sqlite"
)

type dogfoodRefs struct {
	token                string
	session              string
	authoritySession     string
	system               string
	systemAdmission      string
	actingEligibility    string
	roleAdmission        string
	assignmentReason     string
	assignmentProvenance string
	roleAssignment       string
	speechAct            string
	authorityCapture     string
	content              string
	permission           string
	claimScope           string
	singleUse            string
}

func newDogfoodRefs(token string) (dogfoodRefs, error) {
	if token == "" {
		return dogfoodRefs{}, fmt.Errorf("dogfood identity is required")
	}
	return dogfoodRefs{
		token:                token,
		session:              "session:profile-onboarding:" + token,
		authoritySession:     "session:manual-authority-issue:" + token,
		system:               "system:haft-kernel:" + token,
		systemAdmission:      "system-admission:profile-onboarding:" + token,
		actingEligibility:    "acting-eligibility:haft-profile-onboarding:" + token,
		roleAdmission:        "role-admission:profile-author:" + token,
		assignmentReason:     "assignment-justification:profile-author:" + token,
		assignmentProvenance: "assignment-provenance:profile-author:" + token,
		roleAssignment:       "role-assignment:profile-author:" + token,
		speechAct:            "speech-act:profile-onboarding:" + token,
		authorityCapture:     "carrier:authority-terminal-capture:" + token,
		content:              "authorization-content:profile-onboarding-v1:" + token,
		permission:           "permission:profile-onboarding:" + token,
		claimScope:           "claim-scope:profile-declaration:" + token,
		singleUse:            "single-use:profile-onboarding:" + token,
	}, nil
}

type dogfoodAuthoritySupport struct {
	values projectprofilesqlite.ProfileOnboardingAuthoritySupportV1
	actors dogfoodActors
}

func buildProfileAuthoritySupport(
	refs dogfoodRefs,
	preparedAt time.Time,
	validUntil time.Time,
	classifierVersion string,
	policyVersion string,
) (dogfoodAuthoritySupport, error) {
	description := projectprofile.ProfileOnboardingMethodDescriptionV1Value()
	contract, err := projectprofile.ProfileOnboardingMethodContractV1Value()
	if err != nil {
		return dogfoodAuthoritySupport{}, err
	}
	actors, err := buildProfileActors(
		refs,
		preparedAt,
		validUntil,
		classifierVersion,
		policyVersion,
		profileActorMethodEditionV1(),
	)
	if err != nil {
		return dogfoodAuthoritySupport{}, err
	}
	builder := projectprofilesqlite.NewProfileOnboardingAuthoritySupportV1Builder(
		actors.assignment,
	)
	builder = builder.WithMethodDescription(description)
	builder = builder.WithMethodContract(contract)
	builder = builder.WithSystemAdmission(actors.systemAdmission)
	builder = builder.WithRoleAdmission(actors.roleAdmission)
	builder = builder.WithAssignmentJustification(actors.justification)
	builder = builder.WithAssignmentProvenance(actors.provenance)
	values, err := builder.Build()
	if err != nil {
		return dogfoodAuthoritySupport{}, err
	}
	return dogfoodAuthoritySupport{values: values, actors: actors}, nil
}

type dogfoodActors struct {
	session         projectprofile.SessionRef
	classifier      projectprofile.ClassifierVersion
	policy          projectprofile.PolicyVersion
	systemAdmission projectprofile.ProfileOnboardingExecutorSystemAdmissionV1
	roleAdmission   projectprofile.ProfileAuthorRoleAdmissionV1
	justification   projectprofile.ProfileAuthorAssignmentJustificationV1
	provenance      projectprofile.ProfileAuthorAssignmentProvenanceV1
	assignment      projectprofile.ProfileAuthorRoleAssignmentV1
}

type profileActorMethodEdition struct {
	configureSystemAdmission func(
		projectprofile.ProfileOnboardingExecutorSystemAdmissionV1Builder,
	) projectprofile.ProfileOnboardingExecutorSystemAdmissionV1Builder
	newRoleAdmission func(
		projectprofile.RoleAdmissionRef,
	) (projectprofile.ProfileAuthorRoleAdmissionV1, error)
}

func profileActorMethodEditionV1() profileActorMethodEdition {
	return profileActorMethodEdition{
		configureSystemAdmission: func(
			builder projectprofile.ProfileOnboardingExecutorSystemAdmissionV1Builder,
		) projectprofile.ProfileOnboardingExecutorSystemAdmissionV1Builder {
			return builder
		},
		newRoleAdmission: projectprofile.NewProfileAuthorRoleAdmissionV1,
	}
}

func buildProfileActors(
	refs dogfoodRefs,
	validFrom time.Time,
	validUntil time.Time,
	classifierVersion string,
	policyVersion string,
	methodEdition profileActorMethodEdition,
) (dogfoodActors, error) {
	session, err := parseValue(refs.session, projectprofile.NewSessionRef)
	if err != nil {
		return dogfoodActors{}, err
	}
	system, err := parseValue(refs.system, projectprofile.NewSystemRef)
	if err != nil {
		return dogfoodActors{}, err
	}
	classifier, err := parseValue(classifierVersion, projectprofile.NewClassifierVersion)
	if err != nil {
		return dogfoodActors{}, err
	}
	policy, err := parseValue(policyVersion, projectprofile.NewPolicyVersion)
	if err != nil {
		return dogfoodActors{}, err
	}
	kernel, err := projectprofile.NewProfileOnboardingKernelIdentityV1(
		"haft-kernel",
		"v9",
	)
	if err != nil {
		return dogfoodActors{}, err
	}
	runtime, err := projectprofile.NewProfileOnboardingRuntimeIdentityV1(
		"haft-cli",
		"v9",
	)
	if err != nil {
		return dogfoodActors{}, err
	}
	identityBasis, err := projectprofile.NewProfileOnboardingKernelExecutorIdentityBasisV1(
		system,
		kernel,
	)
	if err != nil {
		return dogfoodActors{}, err
	}
	systemWindow, err := projectprofile.NewProfileOnboardingExecutorAdmissionWindowV1(
		validFrom,
		validUntil,
	)
	if err != nil {
		return dogfoodActors{}, err
	}
	actingRef, err := parseValue(
		refs.actingEligibility,
		projectprofile.NewProfileOnboardingSystemActingEligibilityBasisRefV1,
	)
	if err != nil {
		return dogfoodActors{}, err
	}
	actingDigest, err := profileContentDigest(
		"haft.profile-onboarding.acting-eligibility/v1",
		[]string{system.String(), session.String(), "local-haft-cli"},
	)
	if err != nil {
		return dogfoodActors{}, err
	}
	systemAdmissionRef, err := parseValue(
		refs.systemAdmission,
		projectprofile.NewSystemAdmissionRef,
	)
	if err != nil {
		return dogfoodActors{}, err
	}
	systemBuilder := projectprofile.NewProfileOnboardingExecutorSystemAdmissionV1Builder(
		systemAdmissionRef,
		system,
	)
	systemBuilder = systemBuilder.IdentifiedBy(identityBasis)
	systemBuilder = systemBuilder.AdmittedToActBy(actingRef, actingDigest)
	systemBuilder = systemBuilder.InSession(session)
	systemBuilder = systemBuilder.ValidDuring(systemWindow)
	systemBuilder = methodEdition.configureSystemAdmission(systemBuilder)
	systemAdmission, err := systemBuilder.Build()
	if err != nil {
		return dogfoodActors{}, err
	}
	roleAdmissionRef, err := parseValue(
		refs.roleAdmission,
		projectprofile.NewRoleAdmissionRef,
	)
	if err != nil {
		return dogfoodActors{}, err
	}
	roleAdmission, err := methodEdition.newRoleAdmission(roleAdmissionRef)
	if err != nil {
		return dogfoodActors{}, err
	}
	assignmentWindow, err := projectprofile.NewRoleAssignmentWindowV1(
		validFrom,
		validUntil,
	)
	if err != nil {
		return dogfoodActors{}, err
	}
	justificationRef, err := parseValue(
		refs.assignmentReason,
		projectprofile.NewRoleAssignmentJustificationRef,
	)
	if err != nil {
		return dogfoodActors{}, err
	}
	justificationBuilder := projectprofile.NewProfileAuthorAssignmentJustificationV1Builder(
		justificationRef,
	)
	justificationBuilder = justificationBuilder.ApplyingAdmissions(
		systemAdmission,
		roleAdmission,
	)
	justificationBuilder = justificationBuilder.ValidDuring(assignmentWindow)
	justification, err := justificationBuilder.Build()
	if err != nil {
		return dogfoodActors{}, err
	}
	provenanceRef, err := parseValue(
		refs.assignmentProvenance,
		projectprofile.NewRoleAssignmentProvenanceRef,
	)
	if err != nil {
		return dogfoodActors{}, err
	}
	provenanceBuilder := projectprofile.NewProfileAuthorAssignmentProvenanceV1Builder(
		provenanceRef,
		justification,
	)
	provenanceBuilder = provenanceBuilder.InSession(session)
	provenanceBuilder = provenanceBuilder.ProducedBy(kernel, runtime)
	provenanceBuilder = provenanceBuilder.RecordedAt(validFrom)
	provenance, err := provenanceBuilder.Build()
	if err != nil {
		return dogfoodActors{}, err
	}
	support, err := projectprofile.CarryProfileAuthorAssignmentSupportV1(
		systemAdmission,
		roleAdmission,
		justification,
		provenance,
	)
	if err != nil {
		return dogfoodActors{}, err
	}
	assignmentRef, err := parseValue(
		refs.roleAssignment,
		projectprofile.NewRoleAssignmentRef,
	)
	if err != nil {
		return dogfoodActors{}, err
	}
	assignmentBuilder := projectprofile.NewProfileAuthorRoleAssignmentV1Builder(
		assignmentRef,
	)
	assignmentBuilder = assignmentBuilder.HeldBy(system)
	assignmentBuilder = assignmentBuilder.Assigning(
		projectprofile.ProfileAuthorRoleRefV1(),
	)
	assignmentBuilder = assignmentBuilder.InContext(
		projectprofile.ProfileOnboardingBoundedContextRefV1(),
	)
	assignmentBuilder = assignmentBuilder.ValidDuring(assignmentWindow)
	assignmentBuilder = assignmentBuilder.WithSystemAdmission(
		systemAdmission.Ref(),
		support.SystemAdmissionDigest(),
	)
	assignmentBuilder = assignmentBuilder.WithRoleAdmission(
		roleAdmission.Ref(),
		support.RoleAdmissionDigest(),
	)
	assignmentBuilder = assignmentBuilder.JustifiedBy(
		justification.Ref(),
		support.JustificationDigest(),
	)
	assignmentBuilder = assignmentBuilder.WithProvenance(
		provenance.Ref(),
		support.ProvenanceDigest(),
	)
	assignment, err := assignmentBuilder.Build()
	if err != nil {
		return dogfoodActors{}, err
	}
	return dogfoodActors{
		session:         session,
		classifier:      classifier,
		policy:          policy,
		systemAdmission: systemAdmission,
		roleAdmission:   roleAdmission,
		justification:   justification,
		provenance:      provenance,
		assignment:      assignment,
	}, nil
}

func profileContentDigest(
	domain string,
	values []string,
) (projectprofile.ContentDigest, error) {
	writer := newContentDigestWriter(domain)
	addDigestValues(writer, values, 0)
	raw := "sha256:" + fmt.Sprintf("%x", writer.hash.Sum(nil))
	return projectprofile.NewContentDigest(raw)
}

func parseValue[T any](raw string, parser func(string) (T, error)) (T, error) {
	value, err := parser(raw)
	if err != nil {
		var zero T
		return zero, err
	}
	return value, nil
}
