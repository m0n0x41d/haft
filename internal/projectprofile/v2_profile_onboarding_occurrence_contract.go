package projectprofile

import (
	"fmt"
	"slices"
)

// ProfileOnboardingOccurrenceContractEvaluationV2 proves local v2 occurrence
// conformance. Authority coverage remains a separate deferred obligation.
type ProfileOnboardingOccurrenceContractEvaluationV2 interface {
	DeferredAuthorityCoverageRequirements() []ProfileOnboardingDeferredAuthorityCoverageRequirementV1
	profileOnboardingOccurrenceContractEvaluationV2()
}

type profileOnboardingOccurrenceContractEvaluationV2 struct {
	deferredAuthorityCoverage []ProfileOnboardingDeferredAuthorityCoverageRequirementV1
}

func (profileOnboardingOccurrenceContractEvaluationV2) profileOnboardingOccurrenceContractEvaluationV2() {
}

func (evaluation profileOnboardingOccurrenceContractEvaluationV2) DeferredAuthorityCoverageRequirements() []ProfileOnboardingDeferredAuthorityCoverageRequirementV1 {
	return append(
		[]ProfileOnboardingDeferredAuthorityCoverageRequirementV1{},
		evaluation.deferredAuthorityCoverage...,
	)
}

// EvaluateProfileOnboardingOccurrenceContractV2 requires both exact input
// refs. The WorkInput ref is supplied by the authority-bound orchestration
// layer; this pure core neither loads nor fabricates that input record.
func EvaluateProfileOnboardingOccurrenceContractV2(
	record ProfileOnboardingWorkRecord,
	description ProfileOnboardingMethodDescriptionV2,
	contract ProfileOnboardingMethodContractV2,
	assignment ProfileAuthorRoleAssignmentV1,
	basis ObservedProjectBasisV1,
	workInputRef WorkInputRef,
) (ProfileOnboardingOccurrenceContractEvaluationV2, error) {
	work, err := canonicalizeProfileOnboardingWorkRecord(record)
	if err != nil {
		return nil, err
	}
	exactDescription, err := exactProfileOnboardingMethodDescriptionV2(description)
	if err != nil {
		return nil, err
	}
	exactContract, err := exactProfileOnboardingMethodContractV2(contract)
	if err != nil {
		return nil, err
	}
	exactAssignment, err := canonicalProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		return nil, err
	}
	exactBasis, err := exactObservedProjectBasisV1(basis)
	if err != nil {
		return nil, err
	}
	if !workInputRef.valid() {
		return nil, fmt.Errorf("ProfileOnboardingWorkInputV1 ref is required")
	}
	err = validateProfileOnboardingOccurrenceV2(
		work,
		exactDescription,
		exactContract,
		exactAssignment,
		exactBasis,
		workInputRef,
	)
	if err != nil {
		return nil, err
	}
	requirements := mapSliceV1Pure(
		profileOnboardingDeferredCoverageRulesV1(),
		func(rule profileOnboardingDeferredCoverageRuleV1) ProfileOnboardingDeferredAuthorityCoverageRequirementV1 {
			return ProfileOnboardingDeferredAuthorityCoverageRequirementV1{
				ruleRef:        rule.ref,
				occurrenceSlot: rule.slot,
			}
		},
	)
	if err := validateProfileOnboardingDeferredCoverageRequirementsV1(requirements); err != nil {
		return nil, err
	}
	return profileOnboardingOccurrenceContractEvaluationV2{
		deferredAuthorityCoverage: requirements,
	}, nil
}

func validateProfileOnboardingOccurrenceV2(
	work ProfileOnboardingWorkRecord,
	description profileOnboardingMethodDescriptionV2,
	contract profileOnboardingMethodContractV2,
	assignment ProfileAuthorRoleAssignmentV1,
	basis observedProjectBasisV1,
	workInputRef WorkInputRef,
) error {
	descriptionDigest, err := DigestProfileOnboardingMethodDescriptionV2(description)
	if err != nil {
		return err
	}
	contractDigest, err := DigestProfileOnboardingMethodContractV2(contract)
	if err != nil {
		return err
	}
	basisDigest, err := DigestObservedProjectBasisV1(basis)
	if err != nil {
		return err
	}
	acceptedKinds := profileOnboardingInputKindsToStringsV1(description.AcceptedInputKinds())
	expectedKinds := []string{
		profileOnboardingObservedBasisKindV1Value,
		profileOnboardingWorkInputKindV1Value,
	}
	if err := validateExactProfileOnboardingNameSetV1(
		"MethodDescription accepted input kinds",
		acceptedKinds,
		expectedKinds,
	); err != nil {
		return err
	}
	inputRefs := workInputStrings(work.inputRefs)
	expectedRefs := []string{basis.Ref().String(), workInputRef.String()}
	slices.Sort(expectedRefs)
	resultKind, err := profileOnboardingWorkOutcomeResultKindV1(work.outcome)
	if err != nil {
		return err
	}
	acceptedResult := slices.ContainsFunc(
		contract.AcceptedResultKinds(),
		func(value ProfileOnboardingResultKindV1) bool {
			return value.String() == resultKind
		},
	)
	checks := []profileOnboardingWorkSupportCheckV1{
		{valid: work.enactsMethodRef == description.DescribedMethodRef(), reason: "Work method ref does not match MethodDescription v2"},
		{valid: work.methodDescriptionRef == description.Ref(), reason: "Work MethodDescription ref does not match exact v2 description"},
		{valid: work.methodDescriptionDigest == descriptionDigest, reason: "Work MethodDescription digest does not match exact v2 description"},
		{valid: work.methodContractRef != nil && work.methodContractRef.String() == contract.Ref().String(), reason: "Work MethodContract ref does not match exact v2 contract"},
		{valid: work.methodContractDigest == contractDigest, reason: "Work MethodContract digest does not match exact v2 contract"},
		{valid: contract.MethodDescriptionRef() == description.Ref(), reason: "MethodContract v2 does not bind the exact MethodDescription ref"},
		{valid: contract.MethodDescriptionDigest() == descriptionDigest, reason: "MethodContract v2 does not bind the exact MethodDescription digest"},
		{valid: work.profileAuthorRoleAssignmentRef == assignment.RoleAssignmentRef(), reason: "Work ProfileAuthorRoleAssignment ref does not match exact assignment"},
		{valid: work.actualPerformerSystem == assignment.HolderSystemRef(), reason: "Work actual performer does not match assignment holder"},
		{valid: work.boundedContextRef == description.BoundedContextRef(), reason: "Work context does not match MethodDescription v2"},
		{valid: work.boundedContextRef == contract.BoundedContextRef(), reason: "Work context does not match MethodContract v2"},
		{valid: work.boundedContextRef == assignment.BoundedContextRef(), reason: "Work context does not match ProfileAuthorRoleAssignment"},
		{valid: work.observedProjectBasisRef == basis.Ref(), reason: "Work ObservedProjectBasis ref does not match exact basis"},
		{valid: work.observedProjectBasisDigest == basisDigest, reason: "Work ObservedProjectBasis digest does not match exact basis"},
		{valid: slices.Equal(inputRefs, expectedRefs), reason: "Work v2 inputs must be exactly ObservedProjectBasisV1 and ProfileOnboardingWorkInputV1 refs"},
		{valid: work.workInterval.valid(), reason: "MethodContract work_interval occurrence slot is not populated"},
		{valid: work.basisObservationWindow.valid(), reason: "MethodContract basis_observation_window occurrence slot is not populated"},
		{valid: work.statePlaneRef == description.StatePlaneRef(), reason: "Work StatePlane does not match MethodDescription v2"},
		{valid: work.affectedRefKind == description.AffectedRefKind(), reason: "Work affected kind does not match MethodDescription v2"},
		{valid: acceptedResult, reason: "Work result kind is not accepted by MethodContract v2"},
	}
	if err := visitSliceV1(checks, validateProfileOnboardingWorkSupportCheckV1); err != nil {
		return err
	}
	if err := ValidateProfileOnboardingWorkRecordAgainstProfileAuthorRoleAssignmentV1(work, assignment); err != nil {
		return err
	}
	if err := ValidateHolderEqualsExecutedWithinV1(
		contract.HolderEqualsExecutedWithinRule(),
		assignment,
		work.actualPerformerSystem,
	); err != nil {
		return err
	}
	if err := ValidateObservedProjectBasisV1AgainstWorkRecord(basis, work); err != nil {
		return err
	}
	return validateProfileOnboardingOutcomeBasisDigestV1(work.outcome, basisDigest)
}

// ValidateProfileOnboardingWorkRecordAgainstSupportV2 closes the full local
// v2 support graph. Every actor admission must carry the same v2 method pins;
// a v1 authority closure cannot authorize v2 Work by structural similarity.
func ValidateProfileOnboardingWorkRecordAgainstSupportV2(
	record ProfileOnboardingWorkRecord,
	description ProfileOnboardingMethodDescriptionV2,
	contract ProfileOnboardingMethodContractV2,
	assignment ProfileAuthorRoleAssignmentV1,
	assignmentSupport ProfileAuthorAssignmentSupportCarrierV1,
	basis ObservedProjectBasisV1,
	workInputRef WorkInputRef,
) error {
	if err := ValidateProfileAuthorRoleAssignmentV1Support(assignment, assignmentSupport); err != nil {
		return err
	}
	evaluation, err := EvaluateProfileOnboardingOccurrenceContractV2(
		record,
		description,
		contract,
		assignment,
		basis,
		workInputRef,
	)
	if err != nil {
		return err
	}
	if err := validateProfileOnboardingDeferredCoverageRequirementsV1(
		evaluation.DeferredAuthorityCoverageRequirements(),
	); err != nil {
		return err
	}
	descriptionDigest, err := DigestProfileOnboardingMethodDescriptionV2(description)
	if err != nil {
		return err
	}
	contractDigest, err := DigestProfileOnboardingMethodContractV2(contract)
	if err != nil {
		return err
	}
	systemAdmission := assignmentSupport.SystemAdmission()
	roleAdmission := assignmentSupport.RoleAdmission()
	justification := assignmentSupport.Justification()
	contractRef := contract.Ref().String()
	checks := []profileOnboardingWorkSupportCheckV1{
		{valid: systemAdmission.MethodDescriptionRef() == description.Ref(), reason: "executor-system admission does not bind MethodDescription v2"},
		{valid: systemAdmission.MethodDescriptionDigest() == descriptionDigest, reason: "executor-system admission does not bind MethodDescription v2 digest"},
		{valid: systemAdmission.MethodContractRef() != nil && systemAdmission.MethodContractRef().String() == contractRef, reason: "executor-system admission does not bind MethodContract v2"},
		{valid: systemAdmission.MethodContractDigest() == contractDigest, reason: "executor-system admission does not bind MethodContract v2 digest"},
		{valid: roleAdmission.MethodDescriptionRef() == description.Ref(), reason: "ProfileAuthor role admission does not bind MethodDescription v2"},
		{valid: roleAdmission.MethodDescriptionDigest() == descriptionDigest, reason: "ProfileAuthor role admission does not bind MethodDescription v2 digest"},
		{valid: roleAdmission.MethodContractRef() != nil && roleAdmission.MethodContractRef().String() == contractRef, reason: "ProfileAuthor role admission does not bind MethodContract v2"},
		{valid: roleAdmission.MethodContractDigest() == contractDigest, reason: "ProfileAuthor role admission does not bind MethodContract v2 digest"},
		{valid: justification.MethodContractRef() != nil && justification.MethodContractRef().String() == contractRef, reason: "assignment justification does not bind MethodContract v2"},
		{valid: justification.MethodContractDigest() == contractDigest, reason: "assignment justification does not bind MethodContract v2 digest"},
	}
	return visitSliceV1(checks, validateProfileOnboardingWorkSupportCheckV1)
}
