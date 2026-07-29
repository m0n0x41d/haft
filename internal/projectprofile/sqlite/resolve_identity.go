package sqlite

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

// ProfileOnboardingValueIdentityV1 is the minimal durable identity needed to
// reconstruct one Work-to-assessment value DAG. Its fields remain private so a
// partially specified or unvalidated lookup cannot cross the storage boundary.
type ProfileOnboardingValueIdentityV1 struct {
	projectRoot      projectprofile.ProjectRootV1
	workRef          projectprofile.ProfileOnboardingWorkRecordRef
	workDigest       projectprofile.ContentDigest
	assessmentRef    projectprofile.ProfileOnboardingOutcomeAssessmentRefV1
	assessmentDigest projectprofile.ContentDigest
}

// ProfileOnboardingValueIdentityV1Builder keeps lookup identity construction
// linear and validates all addresses together at Build.
type ProfileOnboardingValueIdentityV1Builder struct {
	value ProfileOnboardingValueIdentityV1
}

func NewProfileOnboardingValueIdentityV1Builder(
	projectRoot projectprofile.ProjectRootV1,
) ProfileOnboardingValueIdentityV1Builder {
	return ProfileOnboardingValueIdentityV1Builder{
		value: ProfileOnboardingValueIdentityV1{projectRoot: projectRoot},
	}
}

func (builder ProfileOnboardingValueIdentityV1Builder) WithWork(
	ref projectprofile.ProfileOnboardingWorkRecordRef,
	digest projectprofile.ContentDigest,
) ProfileOnboardingValueIdentityV1Builder {
	builder.value.workRef = ref
	builder.value.workDigest = digest
	return builder
}

func (builder ProfileOnboardingValueIdentityV1Builder) WithAssessment(
	ref projectprofile.ProfileOnboardingOutcomeAssessmentRefV1,
	digest projectprofile.ContentDigest,
) ProfileOnboardingValueIdentityV1Builder {
	builder.value.assessmentRef = ref
	builder.value.assessmentDigest = digest
	return builder
}

func (builder ProfileOnboardingValueIdentityV1Builder) Build() (
	ProfileOnboardingValueIdentityV1,
	error,
) {
	return validateProfileOnboardingValueIdentityV1(builder.value)
}

func validateProfileOnboardingValueIdentityV1(
	value ProfileOnboardingValueIdentityV1,
) (ProfileOnboardingValueIdentityV1, error) {
	root, err := validateProjectRoot(value.projectRoot)
	if err != nil {
		return ProfileOnboardingValueIdentityV1{}, err
	}
	workRef, err := validateWorkRecordRef(value.workRef)
	if err != nil {
		return ProfileOnboardingValueIdentityV1{}, err
	}
	workDigest, err := validateContentDigest("Work-record", value.workDigest)
	if err != nil {
		return ProfileOnboardingValueIdentityV1{}, err
	}
	assessmentRefRaw := value.assessmentRef.String()
	assessmentRef, err := projectprofile.NewProfileOnboardingOutcomeAssessmentRefV1(
		assessmentRefRaw,
	)
	if err != nil {
		return ProfileOnboardingValueIdentityV1{}, fmt.Errorf("validate outcome-assessment ref: %w", err)
	}
	assessmentDigest, err := validateContentDigest(
		"outcome-assessment",
		value.assessmentDigest,
	)
	if err != nil {
		return ProfileOnboardingValueIdentityV1{}, err
	}
	return ProfileOnboardingValueIdentityV1{
		projectRoot:      root,
		workRef:          workRef,
		workDigest:       workDigest,
		assessmentRef:    assessmentRef,
		assessmentDigest: assessmentDigest,
	}, nil
}
