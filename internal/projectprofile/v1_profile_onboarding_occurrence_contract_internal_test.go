package projectprofile

import (
	"strings"
	"testing"
)

func TestProfileOnboardingOccurrenceContractEvaluationConsumesExactSemantics(t *testing.T) {
	fixture := exactProfileOnboardingWorkSupportFixtureV1(t)
	evaluation, err := EvaluateProfileOnboardingOccurrenceContractV1(
		fixture.record,
		fixture.description,
		fixture.contract,
		fixture.assignment,
		fixture.basis,
	)
	if err != nil {
		t.Fatalf("EvaluateProfileOnboardingOccurrenceContractV1: %v", err)
	}
	requirements := evaluation.DeferredAuthorityCoverageRequirements()
	actual := mapSliceV1Pure(
		requirements,
		func(value ProfileOnboardingDeferredAuthorityCoverageRequirementV1) string {
			return value.RuleRef().String() + "@" + value.OccurrenceSlot().String()
		},
	)
	expected := []string{
		authorityCoversWorkRuleRefV1Value + "@" + profileOnboardingWorkIntervalSlotV1Value,
		authorityCoversBasisRuleRefV1Value + "@" + profileOnboardingBasisWindowSlotV1Value,
	}
	err = validateExactProfileOnboardingNameSetV1("test requirements", actual, expected)
	if err != nil {
		t.Fatalf("deferred authority requirements: %v", err)
	}
	requirements[0] = ProfileOnboardingDeferredAuthorityCoverageRequirementV1{}
	secondRead := evaluation.DeferredAuthorityCoverageRequirements()
	if secondRead[0].RuleRef().String() == "" {
		t.Fatal("mutating returned requirements changed the evaluation")
	}
}

func TestProfileOnboardingOccurrenceContractRejectsUntypedOrAdditionalInput(t *testing.T) {
	fixture := exactProfileOnboardingWorkSupportFixtureV1(t)
	mutated := fixture.record
	mutated.inputRefs = append(
		mutated.inputRefs,
		WorkInputRef{v1Reference: v1Reference{value: "input:untyped-extra"}},
	)
	_, err := EvaluateProfileOnboardingOccurrenceContractV1(
		mutated,
		fixture.description,
		fixture.contract,
		fixture.assignment,
		fixture.basis,
	)
	assertOccurrenceContractErrorV1(t, err, "exact typed ObservedProjectBasis input")
}

func TestProfileOnboardingOccurrenceContractConsumesDeclaredInputKinds(t *testing.T) {
	fixture := exactProfileOnboardingWorkSupportFixtureV1(t)
	context := exactOccurrenceContractContextV1(t, fixture)
	context.description.acceptedInputKinds = []ProfileOnboardingInputKindV1{{value: "UnknownInputV1"}}
	_, err := evaluateProfileOnboardingOccurrenceContractV1(context)
	assertOccurrenceContractErrorV1(t, err, "accepted input kinds")
}

func TestProfileOnboardingOccurrenceContractConsumesSlotsAndCoverageRules(t *testing.T) {
	fixture := exactProfileOnboardingWorkSupportFixtureV1(t)
	cases := []struct {
		name   string
		mutate func(profileOnboardingOccurrenceContractContextV1) profileOnboardingOccurrenceContractContextV1
		want   string
	}{
		{
			name: "missing basis slot",
			mutate: func(value profileOnboardingOccurrenceContractContextV1) profileOnboardingOccurrenceContractContextV1 {
				value.contract.requiredOccurrenceSlots = []ProfileOnboardingOccurrenceSlotV1{
					{value: profileOnboardingWorkIntervalSlotV1Value},
				}
				return value
			},
			want: "required occurrence slots",
		},
		{
			name: "unknown slot",
			mutate: func(value profileOnboardingOccurrenceContractContextV1) profileOnboardingOccurrenceContractContextV1 {
				value.contract.requiredOccurrenceSlots = []ProfileOnboardingOccurrenceSlotV1{
					{value: profileOnboardingWorkIntervalSlotV1Value},
					{value: "unknown_slot"},
				}
				return value
			},
			want: "required occurrence slots",
		},
		{
			name: "missing authority basis coverage",
			mutate: func(value profileOnboardingOccurrenceContractContextV1) profileOnboardingOccurrenceContractContextV1 {
				value.contract.occurrenceCoverageRuleRefs = []ProfileOnboardingOccurrenceCoverageRuleRefV1{
					{value: roleAssignmentCoversWorkRuleRefV1Value},
					{value: authorityCoversWorkRuleRefV1Value},
				}
				return value
			},
			want: "occurrence coverage rules",
		},
		{
			name: "unknown coverage rule",
			mutate: func(value profileOnboardingOccurrenceContractContextV1) profileOnboardingOccurrenceContractContextV1 {
				value.contract.occurrenceCoverageRuleRefs = []ProfileOnboardingOccurrenceCoverageRuleRefV1{
					{value: roleAssignmentCoversWorkRuleRefV1Value},
					{value: authorityCoversWorkRuleRefV1Value},
					{value: "haft:rule:profile-onboarding/unknown/v1"},
				}
				return value
			},
			want: "occurrence coverage rules",
		},
	}
	visitSliceV1Pure(cases, func(testCase struct {
		name   string
		mutate func(profileOnboardingOccurrenceContractContextV1) profileOnboardingOccurrenceContractContextV1
		want   string
	}) {
		t.Run(testCase.name, func(t *testing.T) {
			context := exactOccurrenceContractContextV1(t, fixture)
			context = testCase.mutate(context)
			_, err := evaluateProfileOnboardingOccurrenceContractV1(context)
			assertOccurrenceContractErrorV1(t, err, testCase.want)
		})
	})
}

func TestProfileOnboardingOccurrenceContractEvaluatesRoleAssignmentWindowOverWork(t *testing.T) {
	fixture := exactProfileOnboardingWorkSupportFixtureV1(t)
	context := exactOccurrenceContractContextV1(t, fixture)
	context.assignment.validityWindow = RoleAssignmentWindowV1{
		closedIntervalV1: context.basis.observationWindow.closedIntervalV1,
	}
	_, err := evaluateProfileOnboardingOccurrenceContractV1(context)
	assertOccurrenceContractErrorV1(t, err, "assignment window must cover")
}

func TestProfileOnboardingDeferredCoverageRequirementsRejectZeroOrUnknown(t *testing.T) {
	cases := []struct {
		name  string
		value ProfileOnboardingDeferredAuthorityCoverageRequirementV1
	}{
		{name: "zero", value: ProfileOnboardingDeferredAuthorityCoverageRequirementV1{}},
		{
			name: "unknown",
			value: ProfileOnboardingDeferredAuthorityCoverageRequirementV1{
				ruleRef: ProfileOnboardingOccurrenceCoverageRuleRefV1{
					value: "haft:rule:profile-onboarding/unknown/v1",
				},
				occurrenceSlot: ProfileOnboardingOccurrenceSlotV1{
					value: profileOnboardingWorkIntervalSlotV1Value,
				},
			},
		},
	}
	visitSliceV1Pure(cases, func(testCase struct {
		name  string
		value ProfileOnboardingDeferredAuthorityCoverageRequirementV1
	}) {
		t.Run(testCase.name, func(t *testing.T) {
			err := validateProfileOnboardingDeferredCoverageRequirementsV1(
				[]ProfileOnboardingDeferredAuthorityCoverageRequirementV1{testCase.value},
			)
			assertOccurrenceContractErrorV1(t, err, "deferred authority-coverage requirements")
		})
	})
}

func TestProfileOnboardingWorkOutcomeUsesOneCanonicalOperation(t *testing.T) {
	fixture := exactProfileOnboardingWorkSupportFixtureV1(t)
	basisDigest, err := DigestObservedProjectBasisV1(fixture.basis)
	if err != nil {
		t.Fatalf("DigestObservedProjectBasisV1: %v", err)
	}
	candidate, err := NewCandidatePayloadProduced(workSupportDigestV1(t, "candidate"), basisDigest)
	if err != nil {
		t.Fatalf("NewCandidatePayloadProduced: %v", err)
	}
	missing, err := NewClassificationUnderdetermined(workSupportDigestV1(t, "missing"))
	if err != nil {
		t.Fatalf("NewClassificationUnderdetermined: %v", err)
	}
	cases := []struct {
		name       string
		value      ProfileOnboardingWorkOutcomeV1
		resultKind string
		jsonKind   string
	}{
		{
			name:       "candidate",
			value:      candidate,
			resultKind: profileOnboardingCandidateResultKindV1Value,
			jsonKind:   "candidate_payload_produced",
		},
		{
			name:       "underdetermined",
			value:      missing,
			resultKind: profileOnboardingUnderdeterminedKindV1Value,
			jsonKind:   "classification_underdetermined",
		},
	}
	visitSliceV1Pure(cases, func(testCase struct {
		name       string
		value      ProfileOnboardingWorkOutcomeV1
		resultKind string
		jsonKind   string
	}) {
		t.Run(testCase.name, func(t *testing.T) {
			operation, err := exactProfileOnboardingWorkOutcomeOperationV1(testCase.value)
			if err != nil {
				t.Fatalf("exactProfileOnboardingWorkOutcomeOperationV1: %v", err)
			}
			if operation.resultKind.String() != testCase.resultKind {
				t.Fatalf("result kind = %q", operation.resultKind.String())
			}
			dto, err := profileOnboardingWorkOutcomeToJSONV1(testCase.value)
			if err != nil {
				t.Fatalf("profileOnboardingWorkOutcomeToJSONV1: %v", err)
			}
			if dto.Kind != testCase.jsonKind || dto.Kind != operation.canonicalKind {
				t.Fatalf("JSON kind = %q; operation kind = %q", dto.Kind, operation.canonicalKind)
			}
			if operation.digestFields[0] != dto.Kind {
				t.Fatalf("digest discriminant = %q; JSON kind = %q", operation.digestFields[0], dto.Kind)
			}
		})
	})
}

func exactOccurrenceContractContextV1(
	t *testing.T,
	fixture exactWorkSupportFixtureV1,
) profileOnboardingOccurrenceContractContextV1 {
	t.Helper()
	context, err := canonicalProfileOnboardingOccurrenceContractContextV1(
		fixture.record,
		fixture.description,
		fixture.contract,
		fixture.assignment,
		fixture.basis,
	)
	if err != nil {
		t.Fatalf("canonicalProfileOnboardingOccurrenceContractContextV1: %v", err)
	}
	return context
}

func assertOccurrenceContractErrorV1(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want substring %q", err, want)
	}
}
