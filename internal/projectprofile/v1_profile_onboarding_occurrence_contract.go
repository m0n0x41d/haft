package projectprofile

import (
	"fmt"
	"slices"
)

// ProfileOnboardingDeferredAuthorityCoverageRequirementV1 is one exact
// occurrence-coverage obligation that the pure method core can identify but
// cannot discharge. Only the binding admission boundary can compare the Work
// occurrence with a sealed authority presentation. This value is neither an
// authorization nor evidence that the rule was satisfied.
type ProfileOnboardingDeferredAuthorityCoverageRequirementV1 struct {
	ruleRef        ProfileOnboardingOccurrenceCoverageRuleRefV1
	occurrenceSlot ProfileOnboardingOccurrenceSlotV1
}

func (requirement ProfileOnboardingDeferredAuthorityCoverageRequirementV1) RuleRef() ProfileOnboardingOccurrenceCoverageRuleRefV1 {
	return requirement.ruleRef
}

func (requirement ProfileOnboardingDeferredAuthorityCoverageRequirementV1) OccurrenceSlot() ProfileOnboardingOccurrenceSlotV1 {
	return requirement.occurrenceSlot
}

// ProfileOnboardingOccurrenceContractEvaluationV1 is the closed result of
// consuming the final-v1 MethodDescription and MethodContract occurrence
// semantics. Local relations have been checked. Its deferred requirements
// still have to be discharged by the binding admission boundary.
type ProfileOnboardingOccurrenceContractEvaluationV1 interface {
	DeferredAuthorityCoverageRequirements() []ProfileOnboardingDeferredAuthorityCoverageRequirementV1
	profileOnboardingOccurrenceContractEvaluationV1()
}

type profileOnboardingOccurrenceContractEvaluationV1 struct {
	deferredAuthorityCoverage []ProfileOnboardingDeferredAuthorityCoverageRequirementV1
}

func (profileOnboardingOccurrenceContractEvaluationV1) profileOnboardingOccurrenceContractEvaluationV1() {
}

func (evaluation profileOnboardingOccurrenceContractEvaluationV1) DeferredAuthorityCoverageRequirements() []ProfileOnboardingDeferredAuthorityCoverageRequirementV1 {
	result := append(
		[]ProfileOnboardingDeferredAuthorityCoverageRequirementV1{},
		evaluation.deferredAuthorityCoverage...,
	)
	return result
}

type profileOnboardingOccurrenceContractContextV1 struct {
	work        ProfileOnboardingWorkRecord
	description profileOnboardingMethodDescriptionV1
	contract    profileOnboardingMethodContractV1
	assignment  ProfileAuthorRoleAssignmentV1
	basis       observedProjectBasisV1
}

type profileOnboardingOccurrenceSlotRuleV1 struct {
	slot     ProfileOnboardingOccurrenceSlotV1
	validate func(profileOnboardingOccurrenceContractContextV1) error
}

type profileOnboardingLocalCoverageRuleV1 struct {
	ref      ProfileOnboardingOccurrenceCoverageRuleRefV1
	validate func(profileOnboardingOccurrenceContractContextV1) error
}

type profileOnboardingDeferredCoverageRuleV1 struct {
	ref  ProfileOnboardingOccurrenceCoverageRuleRefV1
	slot ProfileOnboardingOccurrenceSlotV1
}

// EvaluateProfileOnboardingOccurrenceContractV1 consumes the occurrence
// semantics of the exact final-v1 MethodDescription and MethodContract. It
// proves the typed ObservedProjectBasis input, required occurrence slots, and
// local RoleAssignment coverage. Authority coverage remains explicit in the
// returned closed requirement set and must not be treated as satisfied here.
func EvaluateProfileOnboardingOccurrenceContractV1(
	record ProfileOnboardingWorkRecord,
	description ProfileOnboardingMethodDescriptionV1,
	contract ProfileOnboardingMethodContractV1,
	assignment ProfileAuthorRoleAssignmentV1,
	basis ObservedProjectBasisV1,
) (ProfileOnboardingOccurrenceContractEvaluationV1, error) {
	context, err := canonicalProfileOnboardingOccurrenceContractContextV1(
		record,
		description,
		contract,
		assignment,
		basis,
	)
	if err != nil {
		return nil, err
	}
	return evaluateProfileOnboardingOccurrenceContractV1(context)
}

func canonicalProfileOnboardingOccurrenceContractContextV1(
	record ProfileOnboardingWorkRecord,
	description ProfileOnboardingMethodDescriptionV1,
	contract ProfileOnboardingMethodContractV1,
	assignment ProfileAuthorRoleAssignmentV1,
	basis ObservedProjectBasisV1,
) (profileOnboardingOccurrenceContractContextV1, error) {
	work, err := canonicalizeProfileOnboardingWorkRecord(record)
	if err != nil {
		return profileOnboardingOccurrenceContractContextV1{}, err
	}
	exactDescription, err := exactProfileOnboardingMethodDescriptionV1(description)
	if err != nil {
		return profileOnboardingOccurrenceContractContextV1{}, err
	}
	exactContract, err := exactProfileOnboardingMethodContractV1(contract)
	if err != nil {
		return profileOnboardingOccurrenceContractContextV1{}, err
	}
	exactAssignment, err := canonicalProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		return profileOnboardingOccurrenceContractContextV1{}, err
	}
	exactBasis, err := exactObservedProjectBasisV1(basis)
	if err != nil {
		return profileOnboardingOccurrenceContractContextV1{}, err
	}
	return profileOnboardingOccurrenceContractContextV1{
		work:        work,
		description: exactDescription,
		contract:    exactContract,
		assignment:  exactAssignment,
		basis:       exactBasis,
	}, nil
}

func evaluateProfileOnboardingOccurrenceContractV1(
	context profileOnboardingOccurrenceContractContextV1,
) (ProfileOnboardingOccurrenceContractEvaluationV1, error) {
	err := validateProfileOnboardingMethodInputV1(context)
	if err != nil {
		return nil, err
	}
	slotRules := profileOnboardingOccurrenceSlotRulesV1()
	err = validateProfileOnboardingOccurrenceSlotSetV1(context.contract, slotRules)
	if err != nil {
		return nil, err
	}
	err = visitSliceV1(slotRules, func(_ int, rule profileOnboardingOccurrenceSlotRuleV1) error {
		return rule.validate(context)
	})
	if err != nil {
		return nil, err
	}
	localRules := profileOnboardingLocalCoverageRulesV1()
	deferredRules := profileOnboardingDeferredCoverageRulesV1()
	err = validateProfileOnboardingCoverageRuleSetV1(context.contract, localRules, deferredRules)
	if err != nil {
		return nil, err
	}
	err = visitSliceV1(localRules, func(_ int, rule profileOnboardingLocalCoverageRuleV1) error {
		return rule.validate(context)
	})
	if err != nil {
		return nil, err
	}
	requirements := mapSliceV1Pure(
		deferredRules,
		func(rule profileOnboardingDeferredCoverageRuleV1) ProfileOnboardingDeferredAuthorityCoverageRequirementV1 {
			return ProfileOnboardingDeferredAuthorityCoverageRequirementV1{
				ruleRef:        rule.ref,
				occurrenceSlot: rule.slot,
			}
		},
	)
	err = validateProfileOnboardingDeferredCoverageRequirementsV1(requirements)
	if err != nil {
		return nil, err
	}
	return profileOnboardingOccurrenceContractEvaluationV1{
		deferredAuthorityCoverage: requirements,
	}, nil
}

func validateProfileOnboardingMethodInputV1(
	context profileOnboardingOccurrenceContractContextV1,
) error {
	acceptedKinds := context.description.AcceptedInputKinds()
	actualKind := ProfileOnboardingInputKindV1{value: profileOnboardingObservedBasisKindV1Value}
	expectedKinds := []string{actualKind.String()}
	actualKinds := profileOnboardingInputKindsToStringsV1(acceptedKinds)
	err := validateExactProfileOnboardingNameSetV1(
		"MethodDescription accepted input kinds",
		actualKinds,
		expectedKinds,
	)
	if err != nil {
		return err
	}
	basisDigest, err := DigestObservedProjectBasisV1(context.basis)
	if err != nil {
		return err
	}
	basisRef := context.basis.Ref()
	inputRefs := workInputStrings(context.work.inputRefs)
	expectedRefs := []string{basisRef.String()}
	checks := []profileOnboardingWorkSupportCheckV1{
		{
			valid:  context.work.observedProjectBasisRef == basisRef,
			reason: "Work inputs do not reference the exact ObservedProjectBasis ref",
		},
		{
			valid:  context.work.observedProjectBasisDigest == basisDigest,
			reason: "Work ObservedProjectBasis digest does not match exact basis",
		},
		{
			valid:  slices.Equal(inputRefs, expectedRefs),
			reason: "Work inputs do not reference ObservedProjectBasis; expected exact typed ObservedProjectBasis input",
		},
	}
	err = visitSliceV1(checks, validateProfileOnboardingWorkSupportCheckV1)
	if err != nil {
		return err
	}
	return ValidateObservedProjectBasisV1AgainstWorkRecord(context.basis, context.work)
}

func profileOnboardingOccurrenceSlotRulesV1() []profileOnboardingOccurrenceSlotRuleV1 {
	return []profileOnboardingOccurrenceSlotRuleV1{
		{
			slot: ProfileOnboardingOccurrenceSlotV1{value: profileOnboardingWorkIntervalSlotV1Value},
			validate: func(context profileOnboardingOccurrenceContractContextV1) error {
				if !context.work.workInterval.valid() {
					return fmt.Errorf("MethodContract work_interval occurrence slot is not populated")
				}
				return nil
			},
		},
		{
			slot: ProfileOnboardingOccurrenceSlotV1{value: profileOnboardingBasisWindowSlotV1Value},
			validate: func(context profileOnboardingOccurrenceContractContextV1) error {
				if !context.work.basisObservationWindow.valid() {
					return fmt.Errorf("MethodContract basis_observation_window occurrence slot is not populated")
				}
				return nil
			},
		},
	}
}

func profileOnboardingLocalCoverageRulesV1() []profileOnboardingLocalCoverageRuleV1 {
	return []profileOnboardingLocalCoverageRuleV1{
		{
			ref: ProfileOnboardingOccurrenceCoverageRuleRefV1{value: roleAssignmentCoversWorkRuleRefV1Value},
			validate: func(context profileOnboardingOccurrenceContractContextV1) error {
				return ValidateProfileOnboardingWorkRecordAgainstProfileAuthorRoleAssignmentV1(
					context.work,
					context.assignment,
				)
			},
		},
	}
}

func profileOnboardingDeferredCoverageRulesV1() []profileOnboardingDeferredCoverageRuleV1 {
	return []profileOnboardingDeferredCoverageRuleV1{
		{
			ref:  ProfileOnboardingOccurrenceCoverageRuleRefV1{value: authorityCoversWorkRuleRefV1Value},
			slot: ProfileOnboardingOccurrenceSlotV1{value: profileOnboardingWorkIntervalSlotV1Value},
		},
		{
			ref:  ProfileOnboardingOccurrenceCoverageRuleRefV1{value: authorityCoversBasisRuleRefV1Value},
			slot: ProfileOnboardingOccurrenceSlotV1{value: profileOnboardingBasisWindowSlotV1Value},
		},
	}
}

func validateProfileOnboardingOccurrenceSlotSetV1(
	contract profileOnboardingMethodContractV1,
	rules []profileOnboardingOccurrenceSlotRuleV1,
) error {
	actual := profileOnboardingOccurrenceSlotsToStringsV1(contract.RequiredOccurrenceSlots())
	expected := mapSliceV1Pure(rules, func(rule profileOnboardingOccurrenceSlotRuleV1) string {
		return rule.slot.String()
	})
	return validateExactProfileOnboardingNameSetV1(
		"MethodContract required occurrence slots",
		actual,
		expected,
	)
}

func validateProfileOnboardingCoverageRuleSetV1(
	contract profileOnboardingMethodContractV1,
	local []profileOnboardingLocalCoverageRuleV1,
	deferred []profileOnboardingDeferredCoverageRuleV1,
) error {
	actual := profileOnboardingOccurrenceRulesToStringsV1(contract.OccurrenceCoverageRuleRefs())
	localNames := mapSliceV1Pure(local, func(rule profileOnboardingLocalCoverageRuleV1) string {
		return rule.ref.String()
	})
	deferredNames := mapSliceV1Pure(deferred, func(rule profileOnboardingDeferredCoverageRuleV1) string {
		return rule.ref.String()
	})
	expected := slices.Concat(localNames, deferredNames)
	return validateExactProfileOnboardingNameSetV1(
		"MethodContract occurrence coverage rules",
		actual,
		expected,
	)
}

func validateExactProfileOnboardingNameSetV1(
	name string,
	actual []string,
	expected []string,
) error {
	canonicalActual := append([]string{}, actual...)
	canonicalExpected := append([]string{}, expected...)
	slices.Sort(canonicalActual)
	slices.Sort(canonicalExpected)
	if slices.Equal(canonicalActual, canonicalExpected) {
		return nil
	}
	return fmt.Errorf("%s do not equal the closed final-v1 set", name)
}

func validateProfileOnboardingDeferredCoverageRequirementsV1(
	values []ProfileOnboardingDeferredAuthorityCoverageRequirementV1,
) error {
	expectedRules := profileOnboardingDeferredCoverageRulesV1()
	actualNames := mapSliceV1Pure(
		values,
		func(value ProfileOnboardingDeferredAuthorityCoverageRequirementV1) string {
			return value.RuleRef().String() + "@" + value.OccurrenceSlot().String()
		},
	)
	expectedNames := mapSliceV1Pure(
		expectedRules,
		func(value profileOnboardingDeferredCoverageRuleV1) string {
			return value.ref.String() + "@" + value.slot.String()
		},
	)
	return validateExactProfileOnboardingNameSetV1(
		"deferred authority-coverage requirements",
		actualNames,
		expectedNames,
	)
}
