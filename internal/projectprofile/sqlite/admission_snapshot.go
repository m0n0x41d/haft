package sqlite

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

type profileAdmissionValueSnapshotV1State struct {
	values ProfileOnboardingValueSetV1
}

// ProfileAdmissionValueSnapshotV1 is the exact reliance-bearing support for
// one admission-bound candidate, resolved through one caller-owned SQLite
// snapshot. Its package-owned state cannot be supplied by model output.
type ProfileAdmissionValueSnapshotV1 struct {
	state *profileAdmissionValueSnapshotV1State
}

func (value ProfileAdmissionValueSnapshotV1) Values() (
	ProfileOnboardingValueSetV1,
	bool,
) {
	if !value.valid() {
		return ProfileOnboardingValueSetV1{}, false
	}
	return value.state.values, true
}

func (value ProfileAdmissionValueSnapshotV1) valid() bool {
	if value.state == nil {
		return false
	}
	err := validateProfileOnboardingValueSetV1(value.state.values)
	return err == nil
}

// ResolveProfileAdmissionValueSnapshotV1 resolves every exact support object
// named by the candidate, validates the passed assessment and all transitive
// relations through one opaque transaction. Profile-ledger integrity and the
// expected admission revision remain the binding admission adapter's concern.
func ResolveProfileAdmissionValueSnapshotV1(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	candidate projectprofile.ProfileDeclarationCandidateV1,
) (ProfileAdmissionValueSnapshotV1, error) {
	if ctx == nil {
		return ProfileAdmissionValueSnapshotV1{}, fmt.Errorf("profile-admission value context is required")
	}
	if err := transaction.RequireActive(); err != nil {
		return ProfileAdmissionValueSnapshotV1{}, fmt.Errorf("profile-admission value transaction is invalid: %w", err)
	}
	canonicalCandidate, err := projectprofile.NewProfileDeclarationCandidateV1(
		candidate.Payload(),
		candidate.Provenance(),
	)
	if err != nil {
		return ProfileAdmissionValueSnapshotV1{}, fmt.Errorf("validate profile declaration candidate: %w", err)
	}
	provenance := canonicalCandidate.Provenance()
	projectRoot := provenance.ProjectRoot()
	workRef := provenance.WorkRecordRef()
	workDigest := provenance.WorkRecordDigest()
	assessmentRef := provenance.OutcomeAssessmentRef()
	assessmentDigest := provenance.OutcomeAssessmentDigest()
	identityBuilder := NewProfileOnboardingValueIdentityV1Builder(
		projectRoot,
	)
	identityBuilder = identityBuilder.WithWork(
		workRef,
		workDigest,
	)
	identityBuilder = identityBuilder.WithAssessment(
		assessmentRef,
		assessmentDigest,
	)
	identity, err := identityBuilder.Build()
	if err != nil {
		return ProfileAdmissionValueSnapshotV1{}, fmt.Errorf("build durable support identity: %w", err)
	}
	durable, err := ResolveProfileOnboardingValueSetV1(
		ctx,
		transaction,
		identity,
	)
	if err != nil {
		return ProfileAdmissionValueSnapshotV1{}, err
	}
	values, ok := durable.Values()
	if !ok {
		return ProfileAdmissionValueSnapshotV1{}, fmt.Errorf("durable profile-onboarding values are unusable")
	}
	assignmentSupport, err := projectprofile.CarryProfileAuthorAssignmentSupportV1(
		values.systemAdmission,
		values.roleAdmission,
		values.justification,
		values.provenance,
	)
	if err != nil {
		return ProfileAdmissionValueSnapshotV1{}, fmt.Errorf("reconstruct assignment support carrier: %w", err)
	}
	err = validateCandidateAgainstMethodEdition(canonicalCandidate, values, assignmentSupport)
	if err != nil {
		return ProfileAdmissionValueSnapshotV1{}, fmt.Errorf("validate candidate against durable supports: %w", err)
	}
	return sealProfileAdmissionValueSnapshotV1(values)
}

func validateCandidateAgainstMethodEdition(
	candidate projectprofile.ProfileDeclarationCandidateV1,
	values ProfileOnboardingValueSetV1,
	assignmentSupport projectprofile.ProfileAuthorAssignmentSupportCarrierV1,
) error {
	switch description := values.methodDescription.(type) {
	case projectprofile.ProfileOnboardingMethodDescriptionV1:
		contract, ok := values.methodContract.(projectprofile.ProfileOnboardingMethodContractV1)
		if !ok {
			return fmt.Errorf("durable profile-onboarding method editions differ")
		}
		return projectprofile.ValidateProfileDeclarationCandidateV1AgainstSupports(
			candidate,
			values.workRecord,
			description,
			contract,
			values.roleAssignment,
			assignmentSupport,
			values.observedBasis,
			values.effect,
			values.assessment,
		)
	case projectprofile.ProfileOnboardingMethodDescriptionV2:
		contract, ok := values.methodContract.(projectprofile.ProfileOnboardingMethodContractV2)
		if !ok {
			return fmt.Errorf("durable profile-onboarding method editions differ")
		}
		workInputRef, ok := values.workRecord.ProfileOnboardingWorkInputRefV2()
		if !ok {
			return fmt.Errorf("durable profile-onboarding Work v2 lacks its exact WorkInput ref")
		}
		return projectprofile.ValidateProfileDeclarationCandidateV1AgainstSupportsV2(
			candidate,
			values.workRecord,
			description,
			contract,
			values.roleAssignment,
			assignmentSupport,
			values.observedBasis,
			values.effect,
			values.assessment,
			workInputRef,
		)
	default:
		return fmt.Errorf("durable profile-onboarding method edition is unsupported")
	}
}

func sealProfileAdmissionValueSnapshotV1(
	values ProfileOnboardingValueSetV1,
) (ProfileAdmissionValueSnapshotV1, error) {
	state := profileAdmissionValueSnapshotV1State{
		values: values,
	}
	snapshot := ProfileAdmissionValueSnapshotV1{state: &state}
	if !snapshot.valid() {
		return ProfileAdmissionValueSnapshotV1{}, fmt.Errorf("resolved profile-admission value snapshot is invalid")
	}
	return snapshot, nil
}
