package sqlite

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

// ProfileOnboardingValueSetV1 is the exact cycle-free value set persisted for
// one final-v1 ProfileOnboarding Work occurrence. It is a storage-boundary
// aggregate, not a new FPF kind: every member remains separately addressed,
// encoded, digested, and validated.
type ProfileOnboardingValueSetV1 struct {
	methodDescription projectprofile.ProfileOnboardingMethodDescriptionEdition
	methodContract    projectprofile.ProfileOnboardingMethodContractEdition
	systemAdmission   projectprofile.ProfileOnboardingExecutorSystemAdmissionV1
	roleAdmission     projectprofile.ProfileAuthorRoleAdmissionV1
	justification     projectprofile.ProfileAuthorAssignmentJustificationV1
	provenance        projectprofile.ProfileAuthorAssignmentProvenanceV1
	roleAssignment    projectprofile.ProfileAuthorRoleAssignmentV1
	observedBasis     projectprofile.ObservedProjectBasisV1
	workRecord        projectprofile.ProfileOnboardingWorkRecord
	effect            projectprofile.ProfileOnboardingEffectV1
	assessment        projectprofile.ProfileOnboardingOutcomeAssessmentV1
}

// ProfileOnboardingValueSetV1Builder keeps construction linear while the
// Build gate validates every direct relation in the persisted DAG.
type ProfileOnboardingValueSetV1Builder struct {
	value ProfileOnboardingValueSetV1
}

func NewProfileOnboardingValueSetV1Builder(
	workRecord projectprofile.ProfileOnboardingWorkRecord,
) ProfileOnboardingValueSetV1Builder {
	return ProfileOnboardingValueSetV1Builder{
		value: ProfileOnboardingValueSetV1{workRecord: workRecord},
	}
}

func (builder ProfileOnboardingValueSetV1Builder) WithMethodDescription(
	description projectprofile.ProfileOnboardingMethodDescriptionV1,
) ProfileOnboardingValueSetV1Builder {
	builder.value.methodDescription = description
	return builder
}

func (builder ProfileOnboardingValueSetV1Builder) WithMethodDescriptionV2(
	description projectprofile.ProfileOnboardingMethodDescriptionV2,
) ProfileOnboardingValueSetV1Builder {
	builder.value.methodDescription = description
	return builder
}

func (builder ProfileOnboardingValueSetV1Builder) WithMethodContract(
	contract projectprofile.ProfileOnboardingMethodContractV1,
) ProfileOnboardingValueSetV1Builder {
	builder.value.methodContract = contract
	return builder
}

func (builder ProfileOnboardingValueSetV1Builder) WithMethodContractV2(
	contract projectprofile.ProfileOnboardingMethodContractV2,
) ProfileOnboardingValueSetV1Builder {
	builder.value.methodContract = contract
	return builder
}

func (builder ProfileOnboardingValueSetV1Builder) WithSystemAdmission(
	systemAdmission projectprofile.ProfileOnboardingExecutorSystemAdmissionV1,
) ProfileOnboardingValueSetV1Builder {
	builder.value.systemAdmission = systemAdmission
	return builder
}

func (builder ProfileOnboardingValueSetV1Builder) WithRoleAdmission(
	roleAdmission projectprofile.ProfileAuthorRoleAdmissionV1,
) ProfileOnboardingValueSetV1Builder {
	builder.value.roleAdmission = roleAdmission
	return builder
}

func (builder ProfileOnboardingValueSetV1Builder) WithAssignmentJustification(
	justification projectprofile.ProfileAuthorAssignmentJustificationV1,
) ProfileOnboardingValueSetV1Builder {
	builder.value.justification = justification
	return builder
}

func (builder ProfileOnboardingValueSetV1Builder) WithAssignmentProvenance(
	provenance projectprofile.ProfileAuthorAssignmentProvenanceV1,
) ProfileOnboardingValueSetV1Builder {
	builder.value.provenance = provenance
	return builder
}

func (builder ProfileOnboardingValueSetV1Builder) WithRoleAssignment(
	assignment projectprofile.ProfileAuthorRoleAssignmentV1,
) ProfileOnboardingValueSetV1Builder {
	builder.value.roleAssignment = assignment
	return builder
}

func (builder ProfileOnboardingValueSetV1Builder) WithObservedBasis(
	basis projectprofile.ObservedProjectBasisV1,
) ProfileOnboardingValueSetV1Builder {
	builder.value.observedBasis = basis
	return builder
}

func (builder ProfileOnboardingValueSetV1Builder) WithEffect(
	effect projectprofile.ProfileOnboardingEffectV1,
) ProfileOnboardingValueSetV1Builder {
	builder.value.effect = effect
	return builder
}

func (builder ProfileOnboardingValueSetV1Builder) WithAssessment(
	assessment projectprofile.ProfileOnboardingOutcomeAssessmentV1,
) ProfileOnboardingValueSetV1Builder {
	builder.value.assessment = assessment
	return builder
}

func (builder ProfileOnboardingValueSetV1Builder) Build() (
	ProfileOnboardingValueSetV1,
	error,
) {
	err := validateProfileOnboardingValueSetV1(builder.value)
	if err != nil {
		return ProfileOnboardingValueSetV1{}, err
	}
	return builder.value, nil
}

func (value ProfileOnboardingValueSetV1) MethodDescription() projectprofile.ProfileOnboardingMethodDescriptionV1 {
	description, _ := value.methodDescription.(projectprofile.ProfileOnboardingMethodDescriptionV1)
	return description
}

func (value ProfileOnboardingValueSetV1) MethodContract() projectprofile.ProfileOnboardingMethodContractV1 {
	contract, _ := value.methodContract.(projectprofile.ProfileOnboardingMethodContractV1)
	return contract
}

func (value ProfileOnboardingValueSetV1) MethodDescriptionEdition() projectprofile.ProfileOnboardingMethodDescriptionEdition {
	return value.methodDescription
}

func (value ProfileOnboardingValueSetV1) MethodContractEdition() projectprofile.ProfileOnboardingMethodContractEdition {
	return value.methodContract
}

func (value ProfileOnboardingValueSetV1) MethodDescriptionV2() (
	projectprofile.ProfileOnboardingMethodDescriptionV2,
	bool,
) {
	description, ok := value.methodDescription.(projectprofile.ProfileOnboardingMethodDescriptionV2)
	return description, ok
}

func (value ProfileOnboardingValueSetV1) MethodContractV2() (
	projectprofile.ProfileOnboardingMethodContractV2,
	bool,
) {
	contract, ok := value.methodContract.(projectprofile.ProfileOnboardingMethodContractV2)
	return contract, ok
}

func (value ProfileOnboardingValueSetV1) SystemAdmission() projectprofile.ProfileOnboardingExecutorSystemAdmissionV1 {
	return value.systemAdmission
}

func (value ProfileOnboardingValueSetV1) RoleAdmission() projectprofile.ProfileAuthorRoleAdmissionV1 {
	return value.roleAdmission
}

func (value ProfileOnboardingValueSetV1) AssignmentJustification() projectprofile.ProfileAuthorAssignmentJustificationV1 {
	return value.justification
}

func (value ProfileOnboardingValueSetV1) AssignmentProvenance() projectprofile.ProfileAuthorAssignmentProvenanceV1 {
	return value.provenance
}

func (value ProfileOnboardingValueSetV1) RoleAssignment() projectprofile.ProfileAuthorRoleAssignmentV1 {
	return value.roleAssignment
}

func (value ProfileOnboardingValueSetV1) ObservedBasis() projectprofile.ObservedProjectBasisV1 {
	return value.observedBasis
}

func (value ProfileOnboardingValueSetV1) WorkRecord() projectprofile.ProfileOnboardingWorkRecord {
	return value.workRecord
}

func (value ProfileOnboardingValueSetV1) Effect() projectprofile.ProfileOnboardingEffectV1 {
	return value.effect
}

func (value ProfileOnboardingValueSetV1) Assessment() projectprofile.ProfileOnboardingOutcomeAssessmentV1 {
	return value.assessment
}

func validateProfileOnboardingValueSetV1(value ProfileOnboardingValueSetV1) error {
	carrier, err := projectprofile.CarryProfileAuthorAssignmentSupportV1(
		value.systemAdmission,
		value.roleAdmission,
		value.justification,
		value.provenance,
	)
	if err != nil {
		return fmt.Errorf("carry ProfileAuthorRoleAssignment support: %w", err)
	}
	err = projectprofile.ValidateProfileAuthorRoleAssignmentV1Support(
		value.roleAssignment,
		carrier,
	)
	if err != nil {
		return fmt.Errorf("validate ProfileAuthorRoleAssignment support: %w", err)
	}
	err = validateProfileOnboardingWorkAgainstEdition(value, carrier)
	if err != nil {
		return fmt.Errorf("validate profile-onboarding Work support: %w", err)
	}
	err = projectprofile.ValidateProfileOnboardingEffectV1AgainstWorkRecord(
		value.effect,
		value.workRecord,
	)
	if err != nil {
		return fmt.Errorf("validate profile-onboarding effect against Work: %w", err)
	}
	err = projectprofile.ValidateProfileOnboardingEffectV1AgainstObservedProjectBasis(
		value.effect,
		value.observedBasis,
	)
	if err != nil {
		return fmt.Errorf("validate profile-onboarding effect against observed basis: %w", err)
	}
	err = projectprofile.ValidateProfileOnboardingOutcomeAssessmentV1AgainstEffect(
		value.assessment,
		value.effect,
	)
	if err != nil {
		return fmt.Errorf("validate profile-onboarding assessment against effect: %w", err)
	}
	return nil
}

func validateProfileOnboardingWorkAgainstEdition(
	value ProfileOnboardingValueSetV1,
	carrier projectprofile.ProfileAuthorAssignmentSupportCarrierV1,
) error {
	switch description := value.methodDescription.(type) {
	case projectprofile.ProfileOnboardingMethodDescriptionV1:
		contract, ok := value.methodContract.(projectprofile.ProfileOnboardingMethodContractV1)
		if !ok {
			return fmt.Errorf("profile-onboarding MethodDescription and MethodContract editions differ")
		}
		return projectprofile.ValidateProfileOnboardingWorkRecordAgainstSupportV1(
			value.workRecord,
			description,
			contract,
			value.roleAssignment,
			carrier,
			value.observedBasis,
		)
	case projectprofile.ProfileOnboardingMethodDescriptionV2:
		contract, ok := value.methodContract.(projectprofile.ProfileOnboardingMethodContractV2)
		if !ok {
			return fmt.Errorf("profile-onboarding MethodDescription and MethodContract editions differ")
		}
		workInputRef, ok := value.workRecord.ProfileOnboardingWorkInputRefV2()
		if !ok {
			return fmt.Errorf("profile-onboarding Work v2 does not expose its exact WorkInput ref")
		}
		return projectprofile.ValidateProfileOnboardingWorkRecordAgainstSupportV2(
			value.workRecord,
			description,
			contract,
			value.roleAssignment,
			carrier,
			value.observedBasis,
			workInputRef,
		)
	default:
		return fmt.Errorf("profile-onboarding MethodDescription edition is absent or unsupported")
	}
}
