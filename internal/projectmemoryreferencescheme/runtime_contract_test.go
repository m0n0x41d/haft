package projectmemoryreferencescheme

import (
	"slices"
	"testing"
)

func TestRuntimeContractRoleMappingIsClosedAndTotal(t *testing.T) {
	testCases := []struct {
		role     SemanticRole
		contract RuntimeContract
		text     string
	}{
		{
			role:     SemanticRoleDesignation,
			contract: RuntimeContractReferenceDesignationResolution,
			text:     "reference_designation_resolution",
		},
		{
			role:     SemanticRoleInterpretation,
			contract: RuntimeContractClaimInterpretation,
			text:     "claim_interpretation",
		},
		{
			role:     SemanticRoleMeasurement,
			contract: RuntimeContractClaimMeasurement,
			text:     "claim_measurement",
		},
		{
			role:     SemanticRoleEvaluation,
			contract: RuntimeContractClaimEvaluation,
			text:     "claim_evaluation",
		},
	}
	mappings := RoleRuntimeContracts()
	if len(mappings) != len(testCases) {
		t.Fatalf("role mappings = %d, want %d", len(mappings), len(testCases))
	}
	for index, testCase := range testCases {
		mapped, err := RuntimeContractForRole(testCase.role)
		if err != nil {
			t.Fatalf("RuntimeContractForRole(%q) error = %v", testCase.role, err)
		}
		if mapped != testCase.contract || mapped.String() != testCase.text {
			t.Fatalf(
				"RuntimeContractForRole(%q) = %q, want %q",
				testCase.role,
				mapped,
				testCase.text,
			)
		}
		parsed, err := ParseRuntimeContract(testCase.text)
		if err != nil || parsed != testCase.contract {
			t.Fatalf("ParseRuntimeContract(%q) = %q, %v", testCase.text, parsed, err)
		}
		if mappings[index].Role() != testCase.role ||
			mappings[index].Contract() != testCase.contract {
			t.Fatalf("role mapping %d = %#v, want %#v", index, mappings[index], testCase)
		}
	}
	if _, err := RuntimeContractForRole(SemanticRole("unknown")); err == nil {
		t.Fatal("RuntimeContractForRole accepted an unknown role")
	}
	if _, err := ParseRuntimeContract("claim_other"); err == nil {
		t.Fatal("ParseRuntimeContract accepted an unknown contract")
	}
}

func TestDeriveCallableRuntimeRequirementsUsesRulesBranchesOnly(t *testing.T) {
	designationRequired := fixtureRulePin(
		t,
		SemanticRoleDesignation,
		"designation:required",
		"designation-required",
		runtimePresent,
	)
	designationDeclarative := fixtureRulePin(
		t,
		SemanticRoleDesignation,
		"designation:declarative",
		"designation-declarative",
		runtimeAbsent,
	)
	interpretationRequired := fixtureRulePin(
		t,
		SemanticRoleInterpretation,
		"interpretation:required",
		"interpretation-required",
		runtimePresent,
	)
	measurementRequired := fixtureRulePin(
		t,
		SemanticRoleMeasurement,
		"measurement:required",
		"measurement-required",
		runtimePresent,
	)
	evaluationRequired := fixtureRulePin(
		t,
		SemanticRoleEvaluation,
		"evaluation:required",
		"evaluation-required",
		runtimePresent,
	)
	scheme := fixtureScheme(
		t,
		[]ExactRulePin{designationRequired, designationDeclarative},
		[]ExactRulePin{interpretationRequired},
		mustMeasurementRules(t, []ExactRulePin{measurementRequired}),
		mustEvaluationRules(t, []ExactRulePin{evaluationRequired}),
	)

	requirements, err := DeriveCallableRuntimeRequirements(scheme)
	if err != nil {
		t.Fatalf("DeriveCallableRuntimeRequirements() error = %v", err)
	}
	wantContracts := []RuntimeContract{
		RuntimeContractReferenceDesignationResolution,
		RuntimeContractClaimInterpretation,
		RuntimeContractClaimMeasurement,
		RuntimeContractClaimEvaluation,
	}
	wantRules := []ExactRulePin{
		designationRequired,
		interpretationRequired,
		measurementRequired,
		evaluationRequired,
	}
	if len(requirements) != len(wantContracts) {
		t.Fatalf("callable requirements = %d, want %d", len(requirements), len(wantContracts))
	}
	for index, requirement := range requirements {
		if !requirement.valid() {
			t.Fatalf("callable requirement %d is invalid", index)
		}
		if requirement.Contract() != wantContracts[index] {
			t.Fatalf(
				"callable requirement %d contract = %q, want %q",
				index,
				requirement.Contract(),
				wantContracts[index],
			)
		}
		if requirement.RuleRef() != wantRules[index].Rule() {
			t.Fatalf(
				"callable requirement %d RuleRef = %q, want %q",
				index,
				requirement.RuleRef(),
				wantRules[index].Rule(),
			)
		}
		if requirement.RulePin().Source() != wantRules[index].Source() {
			t.Fatalf("callable requirement %d source pin changed", index)
		}
		runtime := wantRules[index].Runtime().(RuntimeRequired)
		if requirement.Mechanism() != runtime.Mechanism() {
			t.Fatalf("callable requirement %d mechanism changed", index)
		}
	}
}

func TestDeriveCallableRuntimeRequirementsTreatsBothNotApplicableBranchesAsNonCallable(t *testing.T) {
	designation := fixtureRulePin(
		t,
		SemanticRoleDesignation,
		"designation:declarative",
		"designation",
		runtimeAbsent,
	)
	interpretation := fixtureRulePin(
		t,
		SemanticRoleInterpretation,
		"interpretation:declarative",
		"interpretation",
		runtimeAbsent,
	)
	measurement := fixtureRulePin(
		t,
		SemanticRoleMeasurement,
		"measurement:not-applicable",
		"measurement",
		runtimePresent,
	)
	evaluation := fixtureRulePin(
		t,
		SemanticRoleEvaluation,
		"evaluation:not-applicable",
		"evaluation",
		runtimePresent,
	)
	scheme := fixtureScheme(
		t,
		[]ExactRulePin{designation},
		[]ExactRulePin{interpretation},
		mustMeasurementNotApplicable(t, measurement),
		mustEvaluationNotApplicable(t, evaluation),
	)

	requirements, err := DeriveCallableRuntimeRequirements(scheme)
	if err != nil {
		t.Fatalf("DeriveCallableRuntimeRequirements() error = %v", err)
	}
	if len(requirements) != 0 {
		t.Fatalf("NotApplicable branches produced callable requirements: %#v", requirements)
	}
}

func TestEpistemeConstitutionContractCombinesRoleContractsWithoutReplacingEvaluation(t *testing.T) {
	aggregate := EpistemeConstitutionEvaluationRuntimeContract()
	if aggregate != RuntimeContractEpistemeConstitutionEvaluation {
		t.Fatalf("aggregate contract = %q", aggregate)
	}
	if aggregate == RuntimeContractClaimEvaluation {
		t.Fatal("episteme constitution contract collapsed into claim evaluation")
	}
	if aggregate.String() != "episteme_constitution_evaluation" {
		t.Fatalf("aggregate contract text = %q", aggregate.String())
	}
	inputs := EpistemeConstitutionEvaluationInputContracts()
	want := RoleRuntimeContracts()
	if !slices.Equal(inputs, want) {
		t.Fatalf("constitution inputs = %#v, want %#v", inputs, want)
	}
	if inputs[3].Contract() != RuntimeContractClaimEvaluation {
		t.Fatalf("evaluation role input = %q", inputs[3].Contract())
	}
	inputs[3] = RoleRuntimeContract{}
	if slices.Equal(inputs, EpistemeConstitutionEvaluationInputContracts()) {
		t.Fatal("constitution input accessor returned shared storage")
	}
}

func TestDeriveCallableRuntimeRequirementsRejectsZeroScheme(t *testing.T) {
	if _, err := DeriveCallableRuntimeRequirements(ProjectMemoryReferenceSchemeV1{}); err == nil {
		t.Fatal("DeriveCallableRuntimeRequirements accepted a zero scheme")
	}
}
