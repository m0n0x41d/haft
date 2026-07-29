package projecttypeenv

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	compositeRuntimeEnumerationRule  = "test:entity-set/enumerate/v1"
	compositeRuntimePriorBatchRule   = "test:entity-set/prior-batch/v1"
	compositeRuntimeDefinednessRule  = "test:kind-signature/definedness/v1"
	compositeRuntimeEvaluatorRule    = "test:kind-signature/evaluator/v1"
	compositeRuntimeAdapterRule      = "haft.member-of.project-record-carrier/v1"
	compositeRuntimeSourceMemberRule = "haft.rule.project-concern-member/v1"
	noMembershipDeclarationFixture   = ""
)

func TestCompositeRuntimeRequirementsAcceptExactNecessaryAndSufficientBasis(t *testing.T) {
	fixture := newCompositeRuntimeClosureFixture(
		t,
		carrierFirstMembershipBasisFixture,
		nil,
	)
	resolution := ResolveProjectTypeEnvCompositeRuntimeRequirements(
		fixture.composite,
		fixture.candidate,
		fixture.linked,
		fixture.basis,
	)
	if resolution.Rejected() {
		t.Fatalf("resolution rejected: %#v", resolution.Issues())
	}
	if len(resolution.Issues()) != 0 {
		t.Fatalf("accepted issues = %#v", resolution.Issues())
	}
	required := resolution.RequiredSet()
	if len(required.Requirements()) != 5 {
		t.Fatalf("required coordinates = %d, want 5", len(required.Requirements()))
	}
	if len(required.CanonicalBytes()) == 0 {
		t.Fatal("required set canonical bytes are empty")
	}
	if pins := fixture.basis.RegistrationPolicyPins(); len(pins) != 1 {
		t.Fatalf("registration-policy pins = %d, want exactly one", len(pins))
	}
	assertCompositeRuntimeRequirement(
		t,
		required,
		RuntimeMechanismRoleCodec,
		RuntimeMechanismContractCodecCanonicalization,
		fixture.codec.String(),
	)
	assertCompositeRuntimeRequirement(
		t,
		required,
		RuntimeMechanismRoleEvaluator,
		RuntimeMechanismContractEntitySetEnumeration,
		compositeRuntimeEnumerationRule,
	)
	assertCompositeRuntimeRequirement(
		t,
		required,
		RuntimeMechanismRoleEvaluator,
		RuntimeMechanismContractCandidateVisibility,
		compositeRuntimePriorBatchRule,
	)
	assertCompositeRuntimeRequirement(
		t,
		required,
		RuntimeMechanismRoleCarrierMembership,
		RuntimeMechanismContractCarrierMembershipDelivery,
		compositeRuntimeAdapterRule,
	)

	requirements := required.Requirements()
	requirements[0] = CompositeRuntimeRequirement{}
	if resolution.RequiredSet().Requirements()[0] == (CompositeRuntimeRequirement{}) {
		t.Fatal("Requirements returned shared storage")
	}
	canonical := required.CanonicalBytes()
	canonical[0] ^= 0xff
	if bytes.Equal(canonical, resolution.RequiredSet().CanonicalBytes()) {
		t.Fatal("CanonicalBytes returned shared storage")
	}
}

func TestCompositeRuntimeRequirementsRequireExactRegistrationPolicyForMembership(
	t *testing.T,
) {
	fixture := newCompositeRuntimeClosureFixtureWithRegistrationPolicy(
		t,
		carrierFirstMembershipBasisFixture,
		nil,
		false,
	)
	resolution := ResolveProjectTypeEnvCompositeRuntimeRequirements(
		fixture.composite,
		fixture.candidate,
		fixture.linked,
		fixture.basis,
	)
	if !resolution.Rejected() {
		t.Fatal("membership-capable composite accepted X without registration policy")
	}
	issues := resolution.Issues()
	if len(issues) != 1 ||
		issues[0].Code() != CompositeRuntimeIssueRegistrationMissing {
		t.Fatalf("issues = %#v, want one registration-policy-missing issue", issues)
	}
}

func TestCompositeRuntimeRequirementsAllowMechanismOnlyXWithoutMembershipCapability(
	t *testing.T,
) {
	fixture := newCompositeRuntimeClosureFixture(
		t,
		noMembershipDeclarationFixture,
		nil,
	)
	resolution := ResolveProjectTypeEnvCompositeRuntimeRequirements(
		fixture.composite,
		fixture.candidate,
		fixture.linked,
		fixture.basis,
	)
	if resolution.Rejected() {
		t.Fatalf("mechanism-only non-membership X rejected: %#v", resolution.Issues())
	}
	if pins := fixture.basis.RegistrationPolicyPins(); len(pins) != 0 {
		t.Fatalf("registration-policy pins = %d, want none", len(pins))
	}
}

func TestCompositeRuntimeRequirementsRejectRegistrationPolicyWithoutMembershipCapability(
	t *testing.T,
) {
	fixture := newCompositeRuntimeClosureFixtureWithRegistrationPolicy(
		t,
		noMembershipDeclarationFixture,
		nil,
		true,
	)
	resolution := ResolveProjectTypeEnvCompositeRuntimeRequirements(
		fixture.composite,
		fixture.candidate,
		fixture.linked,
		fixture.basis,
	)
	if !resolution.Rejected() {
		t.Fatal("non-membership composite accepted unnecessary registration policy")
	}
	issues := resolution.Issues()
	if len(issues) != 1 ||
		issues[0].Code() != CompositeRuntimeIssueRegistrationExtra {
		t.Fatalf("issues = %#v, want one registration-policy-extra issue", issues)
	}
}

func TestCompositeRegistrationPoliciesExactMatchEveryMemberOfRule(t *testing.T) {
	ruleA := compositeRuntimeRuleRef(t, "haft.member-of.test-family-a/v1")
	ruleB := compositeRuntimeRuleRef(t, "haft.member-of.test-family-b/v1")
	ruleC := compositeRuntimeRuleRef(t, "haft.member-of.test-family-c/v1")
	policyA := compositeRegistrationPolicyForRule(t, ruleA, "a", 0xd1)
	policyB := compositeRegistrationPolicyForRule(t, ruleB, "b", 0xd2)
	policyC := compositeRegistrationPolicyForRule(t, ruleC, "c", 0xd3)
	policyA2 := compositeRegistrationPolicyForRule(t, ruleA, "a2", 0xd4)

	exact := compositeRegistrationPolicyBasis(t, policyB, policyA)
	if issues := compareCompositeRegistrationPolicyRequirements(
		[]typedmemory.RuleRef{ruleA, ruleB},
		exact,
	); len(issues) != 0 {
		t.Fatalf("exact heterogeneous policy closure issues = %#v", issues)
	}

	tests := []struct {
		name     string
		required []typedmemory.RuleRef
		basis    RuntimeEvaluationBasisArtifact
		codes    []CompositeRuntimeRequirementIssueCode
	}{
		{
			name:     "missing",
			required: []typedmemory.RuleRef{ruleA, ruleB},
			basis:    compositeRegistrationPolicyBasis(t, policyA),
			codes:    []CompositeRuntimeRequirementIssueCode{CompositeRuntimeIssueRegistrationMissing},
		},
		{
			name:     "substituted",
			required: []typedmemory.RuleRef{ruleA, ruleB},
			basis:    compositeRegistrationPolicyBasis(t, policyA, policyC),
			codes: []CompositeRuntimeRequirementIssueCode{
				CompositeRuntimeIssueRegistrationMissing,
				CompositeRuntimeIssueRegistrationExtra,
			},
		},
		{
			name:     "duplicate evaluator rule",
			required: []typedmemory.RuleRef{ruleA},
			basis:    compositeRegistrationPolicyBasis(t, policyA, policyA2),
			codes:    []CompositeRuntimeRequirementIssueCode{CompositeRuntimeIssueRegistrationDuplicate},
		},
		{
			name:     "zero-policy posture",
			required: nil,
			basis:    compositeRegistrationPolicyBasis(t),
			codes:    nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			issues := compareCompositeRegistrationPolicyRequirements(
				test.required,
				test.basis,
			)
			if len(issues) != len(test.codes) {
				t.Fatalf("issues = %#v, want codes %v", issues, test.codes)
			}
			for index, code := range test.codes {
				if issues[index].Code() != code {
					t.Fatalf("issue[%d] = %s, want %s", index, issues[index].Code(), code)
				}
			}
		})
	}
}

func TestCompositeRuntimeRequirementsAreCallerOrderInvariant(t *testing.T) {
	left := newCompositeRuntimeClosureFixture(
		t,
		carrierFirstMembershipBasisFixture,
		nil,
	)
	right := newCompositeRuntimeClosureFixture(
		t,
		carrierFirstMembershipBasisFixture,
		func(pins []RuntimeEvaluationMechanismPin) []RuntimeEvaluationMechanismPin {
			result := append([]RuntimeEvaluationMechanismPin(nil), pins...)
			for leftIndex, rightIndex := 0, len(result)-1; leftIndex < rightIndex; leftIndex, rightIndex = leftIndex+1, rightIndex-1 {
				result[leftIndex], result[rightIndex] = result[rightIndex], result[leftIndex]
			}
			return result
		},
	)
	leftResolution := ResolveProjectTypeEnvCompositeRuntimeRequirements(
		left.composite,
		left.candidate,
		left.linked,
		left.basis,
	)
	rightResolution := ResolveProjectTypeEnvCompositeRuntimeRequirements(
		right.composite,
		right.candidate,
		right.linked,
		right.basis,
	)
	if leftResolution.Rejected() || rightResolution.Rejected() {
		t.Fatalf("permuted resolutions rejected: left=%#v right=%#v", leftResolution.Issues(), rightResolution.Issues())
	}
	if left.composite.Ref() != right.composite.Ref() {
		t.Fatal("X caller order changed composite C")
	}
	if !bytes.Equal(
		leftResolution.RequiredSet().CanonicalBytes(),
		rightResolution.RequiredSet().CanonicalBytes(),
	) {
		t.Fatal("caller order changed canonical runtime requirements")
	}
}

func TestCompositeRuntimeRequirementsDistinguishMissingExtraWrongContractAndWrongRole(t *testing.T) {
	tests := []struct {
		name           string
		mutate         func([]RuntimeEvaluationMechanismPin) []RuntimeEvaluationMechanismPin
		wantCode       CompositeRuntimeRequirementIssueCode
		wantRole       RuntimeMechanismRole
		actualRole     RuntimeMechanismRole
		wantContract   RuntimeMechanismInvocationContract
		actualContract RuntimeMechanismInvocationContract
	}{
		{
			name: "missing",
			mutate: func(pins []RuntimeEvaluationMechanismPin) []RuntimeEvaluationMechanismPin {
				return compositeRuntimePinsWithout(
					pins,
					RuntimeMechanismRoleEvaluator,
					compositeRuntimeEnumerationRule,
				)
			},
			wantCode:     CompositeRuntimeIssueMissing,
			wantRole:     RuntimeMechanismRoleEvaluator,
			wantContract: RuntimeMechanismContractEntitySetEnumeration,
		},
		{
			name: "extra",
			mutate: func(pins []RuntimeEvaluationMechanismPin) []RuntimeEvaluationMechanismPin {
				extra := runtimeEvaluatorMechanismPin(
					t,
					"test:unrequired/evaluator/v1",
					"artifact:unrequired-evaluator",
					"1.0.0",
					0x7f,
				)
				return append(pins, extra)
			},
			wantCode:       CompositeRuntimeIssueExtra,
			actualRole:     RuntimeMechanismRoleEvaluator,
			actualContract: RuntimeMechanismContractMemberOf,
		},
		{
			name: "wrong-contract",
			mutate: func(pins []RuntimeEvaluationMechanismPin) []RuntimeEvaluationMechanismPin {
				withoutEnumeration := compositeRuntimePinsWithout(
					pins,
					RuntimeMechanismRoleEvaluator,
					compositeRuntimeEnumerationRule,
				)
				wrong := runtimeEvaluatorMechanismPin(
					t,
					compositeRuntimeEnumerationRule,
					"artifact:enumerator-wrong-contract",
					"1.0.0",
					0x7d,
				)
				return append(withoutEnumeration, wrong)
			},
			wantCode:       CompositeRuntimeIssueWrongContract,
			wantRole:       RuntimeMechanismRoleEvaluator,
			actualRole:     RuntimeMechanismRoleEvaluator,
			wantContract:   RuntimeMechanismContractEntitySetEnumeration,
			actualContract: RuntimeMechanismContractMemberOf,
		},
		{
			name: "wrong-role",
			mutate: func(pins []RuntimeEvaluationMechanismPin) []RuntimeEvaluationMechanismPin {
				withoutCarrier := compositeRuntimePinsWithout(
					pins,
					RuntimeMechanismRoleCarrierMembership,
					compositeRuntimeAdapterRule,
				)
				wrong := runtimeEvaluatorMechanismPin(
					t,
					compositeRuntimeAdapterRule,
					"artifact:adapter-wrong-role",
					"1.0.0",
					0x7e,
				)
				return append(withoutCarrier, wrong)
			},
			wantCode:       CompositeRuntimeIssueWrongRole,
			wantRole:       RuntimeMechanismRoleCarrierMembership,
			actualRole:     RuntimeMechanismRoleEvaluator,
			wantContract:   RuntimeMechanismContractCarrierMembershipDelivery,
			actualContract: RuntimeMechanismContractMemberOf,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newCompositeRuntimeClosureFixture(
				t,
				carrierFirstMembershipBasisFixture,
				test.mutate,
			)
			resolution := ResolveProjectTypeEnvCompositeRuntimeRequirements(
				fixture.composite,
				fixture.candidate,
				fixture.linked,
				fixture.basis,
			)
			if !resolution.Rejected() {
				t.Fatal("inexact X was accepted")
			}
			issues := resolution.Issues()
			if len(issues) != 1 {
				t.Fatalf("issues = %#v, want exactly one", issues)
			}
			if issues[0].Code() != test.wantCode ||
				issues[0].ExpectedRole() != test.wantRole ||
				issues[0].ActualRole() != test.actualRole ||
				issues[0].ExpectedContract() != test.wantContract ||
				issues[0].ActualContract() != test.actualContract {
				t.Fatalf(
					"issue = code %q expected %q/%q actual %q/%q",
					issues[0].Code(),
					issues[0].ExpectedRole(),
					issues[0].ExpectedContract(),
					issues[0].ActualRole(),
					issues[0].ActualContract(),
				)
			}
		})
	}
}

func TestCompositeRuntimeRequirementsKeepEvaluatorAndCarrierRolesForSameRule(t *testing.T) {
	rule := compositeRuntimeRuleRef(t, compositeRuntimeAdapterRule)
	required, err := newCompositeRuntimeRequirementSet([]CompositeRuntimeRequirement{
		newCompositeRuleRuntimeRequirement(
			RuntimeMechanismRoleEvaluator,
			RuntimeMechanismContractMemberOf,
			rule,
		),
		newCompositeRuleRuntimeRequirement(
			RuntimeMechanismRoleCarrierMembership,
			RuntimeMechanismContractCarrierMembershipDelivery,
			rule,
		),
	})
	if err != nil {
		t.Fatalf("newCompositeRuntimeRequirementSet(): %v", err)
	}
	pins := []RuntimeEvaluationMechanismPin{
		runtimeEvaluatorMechanismPin(
			t,
			compositeRuntimeAdapterRule,
			"artifact:shared-rule-evaluator",
			"1.0.0",
			0x71,
		),
		runtimeCarrierMembershipMechanismPin(
			t,
			compositeRuntimeAdapterRule,
			"artifact:shared-rule-carrier",
			"1.0.0",
			0x72,
		),
	}
	basis, err := SealRuntimeEvaluationBasis(pins)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(): %v", err)
	}
	issues := compareCompositeRuntimeRequirements(required, basis.Pins())
	if len(issues) != 0 {
		t.Fatalf("shared RuleRef across roles produced issues: %#v", issues)
	}
	assertCompositeRuntimeRequirement(
		t,
		required,
		RuntimeMechanismRoleEvaluator,
		RuntimeMechanismContractMemberOf,
		compositeRuntimeAdapterRule,
	)
	assertCompositeRuntimeRequirement(
		t,
		required,
		RuntimeMechanismRoleCarrierMembership,
		RuntimeMechanismContractCarrierMembershipDelivery,
		compositeRuntimeAdapterRule,
	)
}

func TestCompositeRuntimeRequirementsDeriveBothKindSignatureRules(t *testing.T) {
	fixture := newCompositeRuntimeClosureFixture(
		t,
		carrierFirstMembershipBasisFixture,
		nil,
	)
	bindings := fixture.candidate.ValueBindings()
	entitySets := fixture.candidate.EntitySetDefinitions()
	if len(entitySets) != 1 {
		t.Fatalf("candidate fixture bindings/entity sets = %d/%d", len(bindings), len(entitySets))
	}
	var binding typedmemory.ValueBinding
	foundBinding := false
	for _, candidateBinding := range bindings {
		if candidateBinding.Codec() != fixture.codec {
			continue
		}
		binding = candidateBinding
		foundBinding = true
	}
	if !foundBinding {
		t.Fatal("candidate fixture custom ValueBinding is missing")
	}
	provenance := entitySets[0].Provenance()
	signature, err := typedmemory.NewKindSignatureDefinition(
		typedmemory.KindSignatureDefinitionInput{
			ValueKind:       binding.ValueKind(),
			Formality:       typedmemory.SignatureF3,
			DefinednessRule: compositeRuntimeRuleRef(t, compositeRuntimeDefinednessRule),
			Evaluator:       compositeRuntimeRuleRef(t, compositeRuntimeEvaluatorRule),
			EntitySet:       entitySets[0].Ref(),
			Provenance:      provenance,
		},
	)
	if err != nil {
		t.Fatalf("NewKindSignatureDefinition(): %v", err)
	}
	requirements := compositeKindSignatureRuntimeRequirements(
		[]typedmemory.KindSignatureDefinition{signature},
	)
	set, err := newCompositeRuntimeRequirementSet(requirements)
	if err != nil {
		t.Fatalf("newCompositeRuntimeRequirementSet(): %v", err)
	}
	assertCompositeRuntimeRequirement(
		t,
		set,
		RuntimeMechanismRoleEvaluator,
		RuntimeMechanismContractKindDefinedness,
		compositeRuntimeDefinednessRule,
	)
	assertCompositeRuntimeRequirement(
		t,
		set,
		RuntimeMechanismRoleEvaluator,
		RuntimeMechanismContractMemberOf,
		compositeRuntimeEvaluatorRule,
	)
}

func TestCompositeRuntimeRequirementsDirectObservableInputsAddsNoCarrierPin(t *testing.T) {
	fixture := newCompositeRuntimeClosureFixture(
		t,
		directObservableMembershipBasisFixture,
		nil,
	)
	resolution := ResolveProjectTypeEnvCompositeRuntimeRequirements(
		fixture.composite,
		fixture.candidate,
		fixture.linked,
		fixture.basis,
	)
	if resolution.Rejected() {
		t.Fatalf("direct-observable closure rejected: %#v", resolution.Issues())
	}
	for _, requirement := range resolution.RequiredSet().Requirements() {
		if requirement.Role() == RuntimeMechanismRoleCarrierMembership {
			t.Fatalf("direct-observable source created carrier requirement %#v", requirement)
		}
	}
	if len(resolution.RequiredSet().Requirements()) != 4 {
		t.Fatalf("direct-observable required coordinates = %d, want 4", len(resolution.RequiredSet().Requirements()))
	}
}

func TestCompositeRuntimeRequirementsReverifyAllBoundInputs(t *testing.T) {
	fixture := newCompositeRuntimeClosureFixture(
		t,
		carrierFirstMembershipBasisFixture,
		nil,
	)

	t.Run("composite", func(t *testing.T) {
		forged := fixture.composite
		forged.canonicalBytes = append([]byte(nil), forged.canonicalBytes...)
		forged.canonicalBytes[0] ^= 0xff
		assertCompositeRuntimeInputIssue(
			t,
			ResolveProjectTypeEnvCompositeRuntimeRequirements(
				forged,
				fixture.candidate,
				fixture.linked,
				fixture.basis,
			),
			CompositeRuntimeIssueArtifactInvalid,
		)
	})

	t.Run("linked-proof", func(t *testing.T) {
		forged := fixture.linked
		forged.canonical = append([]byte(nil), forged.canonical...)
		forged.canonical[0] ^= 0xff
		assertCompositeRuntimeInputIssue(
			t,
			ResolveProjectTypeEnvCompositeRuntimeRequirements(
				fixture.composite,
				fixture.candidate,
				forged,
				fixture.basis,
			),
			CompositeRuntimeIssueLinkedIRInvalid,
		)
	})

	t.Run("linked-recipe", func(t *testing.T) {
		base := loadBaseArtifact(t)
		extension := sealMembershipBasisFixture(
			t,
			base,
			directObservableMembershipBasisFixture,
		)
		other := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
			base,
			[]ProjectTypeEnvExtensionArtifact{extension},
		))
		assertCompositeRuntimeInputIssue(
			t,
			ResolveProjectTypeEnvCompositeRuntimeRequirements(
				fixture.composite,
				fixture.candidate,
				other,
				fixture.basis,
			),
			CompositeRuntimeIssueLinkedIRMismatch,
		)
	})

	t.Run("runtime-basis", func(t *testing.T) {
		forged := fixture.basis
		forged.canonical = append([]byte(nil), forged.canonical...)
		forged.canonical[0] ^= 0xff
		assertCompositeRuntimeInputIssue(
			t,
			ResolveProjectTypeEnvCompositeRuntimeRequirements(
				fixture.composite,
				fixture.candidate,
				fixture.linked,
				forged,
			),
			CompositeRuntimeIssueRuntimeBasisInvalid,
		)
	})

	t.Run("runtime-mechanism-closure", func(t *testing.T) {
		unresolved, err := DecodeRuntimeEvaluationBasisArtifact(
			fixture.basis.CanonicalBytes(),
		)
		if err != nil {
			t.Fatalf("DecodeRuntimeEvaluationBasisArtifact(): %v", err)
		}
		assertCompositeRuntimeInputIssue(
			t,
			ResolveProjectTypeEnvCompositeRuntimeRequirements(
				fixture.composite,
				fixture.candidate,
				fixture.linked,
				unresolved,
			),
			CompositeRuntimeIssueWrongArtifact,
		)
	})

	t.Run("candidate-ref", func(t *testing.T) {
		assertCompositeRuntimeInputIssue(
			t,
			ResolveProjectTypeEnvCompositeRuntimeRequirements(
				fixture.composite,
				typedmemory.TypeEnv{},
				fixture.linked,
				fixture.basis,
			),
			CompositeRuntimeIssueCandidateRefMismatch,
		)
	})

	t.Run("basis-binding", func(t *testing.T) {
		other, err := SealRuntimeEvaluationBasis(nil)
		if err != nil {
			t.Fatalf("SealRuntimeEvaluationBasis(nil): %v", err)
		}
		assertCompositeRuntimeInputIssue(
			t,
			ResolveProjectTypeEnvCompositeRuntimeRequirements(
				fixture.composite,
				fixture.candidate,
				fixture.linked,
				other,
			),
			CompositeRuntimeIssueRuntimeBasisMismatch,
		)
	})
}

type compositeRuntimeClosureFixture struct {
	composite ProjectTypeEnvCompositeArtifact
	candidate typedmemory.TypeEnv
	linked    LinkedProjectTypeEnvCompositeIR
	basis     RuntimeEvaluationBasisArtifact
	codec     typedmemory.CodecRef
}

func newCompositeRuntimeClosureFixture(
	t *testing.T,
	membershipBasisLine string,
	mutatePins func([]RuntimeEvaluationMechanismPin) []RuntimeEvaluationMechanismPin,
) compositeRuntimeClosureFixture {
	t.Helper()
	includeRegistrationPolicy := membershipBasisLine != noMembershipDeclarationFixture
	return newCompositeRuntimeClosureFixtureWithRegistrationPolicy(
		t,
		membershipBasisLine,
		mutatePins,
		includeRegistrationPolicy,
	)
}

func newCompositeRuntimeClosureFixtureWithRegistrationPolicy(
	t *testing.T,
	membershipBasisLine string,
	mutatePins func([]RuntimeEvaluationMechanismPin) []RuntimeEvaluationMechanismPin,
	includeRegistrationPolicy bool,
) compositeRuntimeClosureFixture {
	t.Helper()
	base := loadBaseArtifact(t)
	extensions := make([]ProjectTypeEnvExtensionArtifact, 0, 1)
	if membershipBasisLine != noMembershipDeclarationFixture {
		extension := sealMembershipBasisFixture(t, base, membershipBasisLine)
		extensions = append(extensions, extension)
	}
	linked := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		extensions,
	))
	codecPin := runtimeCodecMechanismPin(
		t,
		"Haft.RuntimeFixtureCodec",
		"v1",
		0x61,
		"artifact:runtime-fixture-codec",
		"1.0.0",
		0x62,
	)
	baseRef, exists := base.TypeEnvRef()
	if !exists {
		t.Fatal("compiled base fixture has no TypeEnvRef")
	}
	baseEnvironment, _, err := typeenv.LowerBaseTypeEnvArtifactWithCodecsAtRef(base, baseRef)
	if err != nil {
		t.Fatalf("LowerBaseTypeEnvArtifactWithCodecsAtRef(base): %v", err)
	}
	pins := []RuntimeEvaluationMechanismPin{
		codecPin,
		runtimeEvaluatorMechanismPinWithContract(
			t,
			compositeRuntimeEnumerationRule,
			RuntimeMechanismContractEntitySetEnumeration,
			"artifact:runtime-fixture-enumerator",
			"1.0.0",
			0x63,
		),
		runtimeEvaluatorMechanismPinWithContract(
			t,
			compositeRuntimePriorBatchRule,
			RuntimeMechanismContractCandidateVisibility,
			"artifact:runtime-fixture-prior-batch",
			"1.0.0",
			0x64,
		),
	}
	for index, binding := range baseEnvironment.ValueBindings() {
		pin := runtimeCodecMechanismPinForRef(
			t,
			binding.Codec(),
			fmt.Sprintf("artifact:runtime-fixture-base-codec-%d", index),
			"1.0.0",
			byte(0x70+index),
		)
		pins = append(pins, pin)
	}
	if membershipBasisLine == carrierFirstMembershipBasisFixture {
		pins = append(pins, runtimeCarrierMembershipMechanismPin(
			t,
			compositeRuntimeAdapterRule,
			"artifact:runtime-fixture-carrier-membership",
			"1.0.0",
			0x67,
		))
	}
	if mutatePins != nil {
		pins = mutatePins(append([]RuntimeEvaluationMechanismPin(nil), pins...))
	}
	basisPins := make([]RuntimeEvaluationBasisPin, 0, len(pins)+1)
	for _, pin := range pins {
		basisPins = append(basisPins, pin)
	}
	if includeRegistrationPolicy {
		policySpec := defaultRegistrationPolicySpec()
		policySpec.evaluatorRule = compositeRuntimeSourceMemberRule
		policy := registrationPolicyArtifactFixture(t, policySpec)
		policyPin, pinErr := NewRegistrationPolicyPin(policy)
		if pinErr != nil {
			t.Fatalf("NewRegistrationPolicyPin(): %v", pinErr)
		}
		basisPins = append(basisPins, policyPin)
	}
	basis, err := SealRuntimeEvaluationBasisWithPins(basisPins, nil, nil)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasisWithPins(): %v", err)
	}
	composite, err := SealProjectTypeEnvComposite(linked, basis)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(): %v", err)
	}
	candidate := buildCompositeRuntimeCandidate(
		t,
		base,
		composite.Ref(),
		codecPin.Codec(),
	)
	return compositeRuntimeClosureFixture{
		composite: composite,
		candidate: candidate,
		linked:    linked,
		basis:     basis,
		codec:     codecPin.Codec(),
	}
}

func buildCompositeRuntimeCandidate(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	compositeRef typedmemory.TypeEnvRef,
	codec typedmemory.CodecRef,
) typedmemory.TypeEnv {
	t.Helper()
	baseEnvironment, _, err := typeenv.LowerBaseTypeEnvArtifactWithCodecsAtRef(
		base,
		compositeRef,
	)
	if err != nil {
		t.Fatalf("LowerBaseTypeEnvArtifactWithCodecsAtRef(C): %v", err)
	}
	contextRef, err := typedmemory.NewBoundedContextRef("fpf:publication")
	if err != nil {
		t.Fatalf("NewBoundedContextRef(): %v", err)
	}
	kindID, err := typedmemory.NewKindID("U.Entity")
	if err != nil {
		t.Fatalf("NewKindID(): %v", err)
	}
	kind, exists := baseEnvironment.KindDefinition(kindID)
	if !exists {
		t.Fatal("base fixture has no U.Entity kind")
	}
	provenance := kind.Provenance()
	valueKind, err := typedmemory.NewValueKindRef(compositeRef, kindID)
	if err != nil {
		t.Fatalf("NewValueKindRef(): %v", err)
	}
	shapeID, err := typedmemory.NewShapeID("RuntimeFixtureShape")
	if err != nil {
		t.Fatalf("NewShapeID(): %v", err)
	}
	shapeValue := typedmemory.NewClaimGraphShape()
	shapeRef, err := typedmemory.DeriveValueShapeRef(shapeID, shapeValue)
	if err != nil {
		t.Fatalf("DeriveValueShapeRef(): %v", err)
	}
	shape, err := typedmemory.NewValueShapeDeclaration(
		shapeRef,
		shapeValue,
		provenance,
	)
	if err != nil {
		t.Fatalf("NewValueShapeDeclaration(): %v", err)
	}
	binding, err := typedmemory.NewValueBinding(valueKind, shapeRef, codec, provenance)
	if err != nil {
		t.Fatalf("NewValueBinding(): %v", err)
	}
	enumerationRule := compositeRuntimeRuleRef(t, compositeRuntimeEnumerationRule)
	priorRule := compositeRuntimeRuleRef(t, compositeRuntimePriorBatchRule)
	policy, err := typedmemory.NewPriorBatchDeclarationsVisible(priorRule)
	if err != nil {
		t.Fatalf("NewPriorBatchDeclarationsVisible(): %v", err)
	}
	entitySet, err := typedmemory.NewEntitySetDefinition(typedmemory.EntitySetDefinitionInput{
		TypeEnv:         compositeRef,
		Context:         contextRef,
		EnumerationRule: enumerationRule,
		CandidatePolicy: policy,
		Provenance:      provenance,
	})
	if err != nil {
		t.Fatalf("NewEntitySetDefinition(): %v", err)
	}
	builder := typedmemory.NewTypeEnvBuilder(compositeRef)
	builder.SetSourceRevision(baseEnvironment.SourceRevision())
	builder.SetCompilerSchemaVersion(baseEnvironment.CompilerSchemaVersion())
	builder.SetCoverageManifest(baseEnvironment.CoverageManifest())
	for _, context := range baseEnvironment.BoundedContexts() {
		builder.AddBoundedContext(context)
	}
	for _, definition := range baseEnvironment.KindDefinitions() {
		builder.AddKindDefinition(definition)
	}
	for _, definition := range baseEnvironment.EntitySetDefinitions() {
		builder.AddEntitySetDefinition(definition)
	}
	for _, definition := range baseEnvironment.KindSignatureDefinitions() {
		builder.AddKindSignatureDefinition(definition)
	}
	for _, definition := range baseEnvironment.RefKindDefinitions() {
		builder.AddRefKindDefinition(definition)
	}
	for _, availability := range baseEnvironment.ContextKindAvailabilities() {
		builder.AddContextKindAvailability(availability)
	}
	for _, relation := range baseEnvironment.SubkindRelations() {
		builder.AddSubkindRelation(relation)
	}
	for _, bridge := range baseEnvironment.ContextBridges() {
		builder.AddContextBridge(bridge)
	}
	for _, fragment := range baseEnvironment.TypedRelationDeclarationFragments() {
		builder.AddTypedRelationDeclarationFragment(fragment)
	}
	for _, valueShape := range baseEnvironment.ValueShapes() {
		builder.AddValueShape(valueShape)
	}
	for _, valueBinding := range baseEnvironment.ValueBindings() {
		builder.AddValueBinding(valueBinding)
	}
	for _, constraint := range baseEnvironment.Constraints() {
		builder.AddConstraint(constraint)
	}
	builder.AddValueShape(shape)
	builder.AddValueBinding(binding)
	builder.AddEntitySetDefinition(entitySet)
	candidate, err := builder.Build()
	if err != nil {
		t.Fatalf("Build candidate TypeEnv: %v", err)
	}
	return candidate
}

func compositeRuntimeRuleRef(t *testing.T, raw string) typedmemory.RuleRef {
	t.Helper()
	ref, err := typedmemory.NewRuleRef(raw)
	if err != nil {
		t.Fatalf("NewRuleRef(%q): %v", raw, err)
	}
	return ref
}

func compositeRegistrationPolicyForRule(
	t *testing.T,
	rule typedmemory.RuleRef,
	suffix string,
	digest byte,
) RegistrationPolicyArtifact {
	t.Helper()
	spec := defaultRegistrationPolicySpec()
	spec.evaluatorRule = rule.String()
	spec.evaluatorArtifact = "haft.runtime.test-membership-evaluator-" + suffix
	spec.deliveryRule = rule.String()
	spec.deliveryArtifact = "haft.runtime.test-membership-delivery-" + suffix
	spec.evaluatorDigest = digest
	spec.deliveryDigest = digest + 1
	spec.manifestID = "mapping.test-family-" + suffix
	spec.manifestDigest = digest + 2
	spec.adapter = "haft-test-adapter-" + suffix + "/1.0.0"
	return registrationPolicyArtifactFixture(t, spec)
}

func compositeRegistrationPolicyBasis(
	t *testing.T,
	policies ...RegistrationPolicyArtifact,
) RuntimeEvaluationBasisArtifact {
	t.Helper()
	pins := make([]RuntimeEvaluationBasisPin, 0, len(policies))
	for _, policy := range policies {
		pin, err := NewRegistrationPolicyPin(policy)
		if err != nil {
			t.Fatalf("NewRegistrationPolicyPin(): %v", err)
		}
		pins = append(pins, pin)
	}
	basis, err := SealRuntimeEvaluationBasisWithPins(pins, nil, nil)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasisWithPins(): %v", err)
	}
	return basis
}

func compositeRuntimePinsWithout(
	pins []RuntimeEvaluationMechanismPin,
	role RuntimeMechanismRole,
	semantic string,
) []RuntimeEvaluationMechanismPin {
	result := make([]RuntimeEvaluationMechanismPin, 0, len(pins))
	for _, pin := range pins {
		requirement := compositeRuntimeRequirementForPin(pin)
		if requirement.Role() == role && requirement.SemanticReference() == semantic {
			continue
		}
		result = append(result, pin)
	}
	return result
}

func assertCompositeRuntimeRequirement(
	t *testing.T,
	set CompositeRuntimeRequirementSet,
	role RuntimeMechanismRole,
	contract RuntimeMechanismInvocationContract,
	semantic string,
) {
	t.Helper()
	for _, requirement := range set.Requirements() {
		if requirement.Role() == role &&
			requirement.InvocationContract() == contract &&
			requirement.SemanticReference() == semantic {
			return
		}
	}
	t.Fatalf(
		"missing runtime requirement role=%q contract=%q semantic=%q",
		role,
		contract,
		semantic,
	)
}

func assertCompositeRuntimeInputIssue(
	t *testing.T,
	resolution CompositeRuntimeRequirementsResolution,
	code CompositeRuntimeRequirementIssueCode,
) {
	t.Helper()
	if !resolution.Rejected() {
		t.Fatalf("input issue %q was accepted", code)
	}
	issues := resolution.Issues()
	if len(issues) != 1 || issues[0].Code() != code {
		t.Fatalf("input issues = %#v, want one %q", issues, code)
	}
	if strings.TrimSpace(issues[0].Detail()) == "" || strings.TrimSpace(issues[0].Repair()) == "" {
		t.Fatalf("input issue lacks detail or repair: %#v", issues[0])
	}
}
