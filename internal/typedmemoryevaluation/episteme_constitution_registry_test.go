package typedmemoryevaluation_test

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemoryconstitution"
	"github.com/m0n0x41d/haft/internal/projectmemoryreferencescheme"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
)

type constitutionRegistryFixture struct {
	graph        projectmemoryconstitution.CanonicalClaimGraph
	concern      projectmemoryconstitution.ResolvedEntityOfConcern
	scheme       projectmemoryreferencescheme.ProjectMemoryReferenceSchemeV1
	pins         map[projectmemoryreferencescheme.SemanticRole][]projectmemoryreferencescheme.ExactRulePin
	registryRule typedmemory.RuleRef
	identity     typedmemoryevaluation.MechanismIdentity
}

type constitutionOutcomePosture uint8

const (
	constitutionOutcomeSatisfied constitutionOutcomePosture = iota + 1
	constitutionOutcomeContradicted
	constitutionOutcomeUnderdetermined
)

func TestEpistemeConstitutionRegistryLooksUpAndInvokesCanonicalCore(t *testing.T) {
	t.Parallel()
	fixture := newConstitutionRegistryFixture(t)
	evaluator := constitutionRegistryEvaluator(t, fixture)
	outcomes := constitutionRegistryRoleOutcomes(
		t,
		fixture,
		nil,
	)
	basis := projectmemoryconstitution.NewRoleRuntimeEvaluationBasis(outcomes)
	input := constitutionRegistryEvaluationInput(fixture, basis)
	request := typedmemoryevaluation.NewEpistemeConstitutionEvaluationRequest(input)
	result, err := evaluator.Evaluate(request)
	if err != nil {
		t.Fatal(err)
	}
	satisfied, ok := result.(projectmemoryconstitution.Satisfied)
	if !ok {
		t.Fatalf("Evaluate() = %T, want projectmemoryconstitution.Satisfied", result)
	}
	coordinate := satisfied.Coordinate()
	if !bytes.Equal(
		coordinate.ClaimGraphCanonicalBytes(),
		fixture.graph.CanonicalBytes(),
	) {
		t.Fatal("registry evaluation lost the exact canonical ClaimGraph coordinate")
	}
	if coordinate.EntityID() != fixture.concern.EntityID() {
		t.Fatal("registry evaluation lost the exact EntityOfConcern coordinate")
	}
	if coordinate.ReferenceSchemeDigest() != fixture.scheme.Digest() {
		t.Fatal("registry evaluation lost the intrinsic ReferenceScheme digest")
	}
}

func TestEpistemeConstitutionRegistryDoesNotInferSuccessFromAvailability(t *testing.T) {
	t.Parallel()
	fixture := newConstitutionRegistryFixture(t)
	evaluator := constitutionRegistryEvaluator(t, fixture)
	complete := constitutionRegistryRoleOutcomes(t, fixture, nil)
	partial := slices.Clone(complete)
	partial = partial[:len(partial)-1]
	cases := []struct {
		name  string
		basis projectmemoryconstitution.RuntimeEvaluationBasis
	}{
		{
			name:  "explicitly missing runtime basis",
			basis: projectmemoryconstitution.NewMissingRuntimeEvaluationBasis(),
		},
		{
			name:  "installed registry without role outcomes",
			basis: projectmemoryconstitution.NewRoleRuntimeEvaluationBasis(nil),
		},
		{
			name:  "partial role outcomes",
			basis: projectmemoryconstitution.NewRoleRuntimeEvaluationBasis(partial),
		},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			input := constitutionRegistryEvaluationInput(fixture, testCase.basis)
			request := typedmemoryevaluation.NewEpistemeConstitutionEvaluationRequest(input)
			result, err := evaluator.Evaluate(request)
			if err != nil {
				t.Fatal(err)
			}
			constitutionRegistryAssertUnderdetermined(
				t,
				result,
				projectmemoryconstitution.ReasonReferenceSchemeRuntimeBasisMissing,
			)
		})
	}
}

func TestEpistemeConstitutionRegistryPreservesNonPositivePostures(t *testing.T) {
	t.Parallel()
	fixture := newConstitutionRegistryFixture(t)
	evaluator := constitutionRegistryEvaluator(t, fixture)
	cases := []struct {
		name    string
		posture constitutionOutcomePosture
		assert  func(
			*testing.T,
			projectmemoryconstitution.Result,
			projectmemoryconstitution.Reason,
		)
	}{
		{
			name:    "underdetermined role outcome",
			posture: constitutionOutcomeUnderdetermined,
			assert:  constitutionRegistryAssertUnderdetermined,
		},
		{
			name:    "contradicted role outcome",
			posture: constitutionOutcomeContradicted,
			assert:  constitutionRegistryAssertInvalid,
		},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			overrides := map[projectmemoryreferencescheme.SemanticRole]constitutionOutcomePosture{
				projectmemoryreferencescheme.SemanticRoleInterpretation: testCase.posture,
			}
			outcomes := constitutionRegistryRoleOutcomes(t, fixture, overrides)
			basis := projectmemoryconstitution.NewRoleRuntimeEvaluationBasis(outcomes)
			input := constitutionRegistryEvaluationInput(fixture, basis)
			request := typedmemoryevaluation.NewEpistemeConstitutionEvaluationRequest(input)
			result, err := evaluator.Evaluate(request)
			if err != nil {
				t.Fatal(err)
			}
			testCase.assert(
				t,
				result,
				projectmemoryconstitution.ReasonEpistemeConstitutionNotSatisfied,
			)
		})
	}
}

func TestEpistemeConstitutionRegistrySeparatesMalformedRequestFromInvalidBasis(t *testing.T) {
	t.Parallel()
	fixture := newConstitutionRegistryFixture(t)
	evaluator := constitutionRegistryEvaluator(t, fixture)
	result, err := evaluator.Evaluate(
		typedmemoryevaluation.EpistemeConstitutionEvaluationRequest{},
	)
	if err == nil || result != nil {
		t.Fatalf("zero request = (%T, %v), want nil protocol error", result, err)
	}

	outcomes := constitutionRegistryRoleOutcomes(t, fixture, nil)
	basis := projectmemoryconstitution.NewRoleRuntimeEvaluationBasis(outcomes)
	input := constitutionRegistryEvaluationInput(fixture, basis)
	input.ClaimGraph = projectmemoryconstitution.CanonicalClaimGraph{}
	request := typedmemoryevaluation.NewEpistemeConstitutionEvaluationRequest(input)
	result, err = evaluator.Evaluate(request)
	if err != nil {
		t.Fatal(err)
	}
	constitutionRegistryAssertInvalid(
		t,
		result,
		projectmemoryconstitution.ReasonClaimGraphBasisInvalid,
	)
}

func constitutionRegistryEvaluator(
	t *testing.T,
	fixture constitutionRegistryFixture,
) typedmemoryevaluation.PureEvaluator[
	typedmemoryevaluation.EpistemeConstitutionEvaluationRequest,
	typedmemoryevaluation.EpistemeConstitutionEvaluationResult,
] {
	t.Helper()
	registry := constitutionRegistryMust(
		typedmemoryevaluation.NewEpistemeConstitutionEvaluationRegistry(
			fixture.registryRule,
			fixture.identity,
		),
	)
	lookup := constitutionRegistryMust(
		registry.Lookup(fixture.registryRule, fixture.identity),
	)
	found, ok := lookup.(typedmemoryevaluation.Found[
		typedmemoryevaluation.EpistemeConstitutionEvaluationRequest,
		typedmemoryevaluation.EpistemeConstitutionEvaluationResult,
	])
	if !ok {
		t.Fatalf("Lookup() = %T, want exact EpistemeConstitution Found", lookup)
	}
	return found.Registration().Evaluator()
}

func newConstitutionRegistryFixture(t *testing.T) constitutionRegistryFixture {
	t.Helper()
	graph := constitutionRegistryCanonicalClaimGraph(t)
	entity := constitutionRegistryMust(
		typedmemory.NewEntityID("entity:project-memory-target"),
	)
	concern := constitutionRegistryMust(
		projectmemoryconstitution.NewResolvedEntityOfConcern(entity),
	)
	designation := constitutionRegistryRulePin(
		t,
		projectmemoryreferencescheme.SemanticRoleDesignation,
		"reference.designation.registry-test/v1",
		true,
	)
	interpretation := constitutionRegistryRulePin(
		t,
		projectmemoryreferencescheme.SemanticRoleInterpretation,
		"claim.interpretation.registry-test/v1",
		true,
	)
	measurement := constitutionRegistryRulePin(
		t,
		projectmemoryreferencescheme.SemanticRoleMeasurement,
		"claim.measurement.registry-test/v1",
		true,
	)
	evaluation := constitutionRegistryRulePin(
		t,
		projectmemoryreferencescheme.SemanticRoleEvaluation,
		"claim.evaluation.not-applicable.registry-test/v1",
		false,
	)
	designationRules := constitutionRegistryMust(
		projectmemoryreferencescheme.NewDesignationRules(
			[]projectmemoryreferencescheme.ExactRulePin{designation},
		),
	)
	interpretationRules := constitutionRegistryMust(
		projectmemoryreferencescheme.NewInterpretationRules(
			[]projectmemoryreferencescheme.ExactRulePin{interpretation},
		),
	)
	measurementRules := constitutionRegistryMust(
		projectmemoryreferencescheme.NewMeasurementRules(
			[]projectmemoryreferencescheme.ExactRulePin{measurement},
		),
	)
	evaluationBranch := constitutionRegistryMust(
		projectmemoryreferencescheme.NewEvaluationNotApplicable(evaluation),
	)
	scheme := constitutionRegistryMust(
		projectmemoryreferencescheme.NewProjectMemoryReferenceSchemeV1(
			designationRules,
			interpretationRules,
			measurementRules,
			evaluationBranch,
		),
	)
	registryRule := constitutionRegistryMust(
		typedmemory.NewRuleRef("episteme.constitution.registry-test/v1"),
	)
	identity := constitutionRegistryIdentity(t)
	pins := map[projectmemoryreferencescheme.SemanticRole][]projectmemoryreferencescheme.ExactRulePin{
		projectmemoryreferencescheme.SemanticRoleDesignation:    {designation},
		projectmemoryreferencescheme.SemanticRoleInterpretation: {interpretation},
		projectmemoryreferencescheme.SemanticRoleMeasurement:    {measurement},
		projectmemoryreferencescheme.SemanticRoleEvaluation:     {evaluation},
	}
	return constitutionRegistryFixture{
		graph:        graph,
		concern:      concern,
		scheme:       scheme,
		pins:         pins,
		registryRule: registryRule,
		identity:     identity,
	}
}

func constitutionRegistryCanonicalClaimGraph(
	t *testing.T,
) projectmemoryconstitution.CanonicalClaimGraph {
	t.Helper()
	digest := constitutionRegistryDigest(t, '1')
	typeEnv := constitutionRegistryMust(typedmemory.NewTypeEnvRef(digest))
	kindID := constitutionRegistryMust(typedmemory.NewKindID("U.ProjectClaim"))
	valueKind := constitutionRegistryMust(
		typedmemory.NewValueKindRef(typeEnv, kindID),
	)
	nodeID := constitutionRegistryMust(
		typedmemory.NewClaimNodeID("claim:registry-e2e"),
	)
	node := constitutionRegistryMust(
		typedmemory.NewClaimNode(
			nodeID,
			valueKind,
			typedmemory.NewTextValue("registry evaluation reaches constitution core"),
		),
	)
	value := constitutionRegistryMust(
		typedmemory.NewClaimGraphValue([]typedmemory.ClaimNode{node}, nil),
	)
	shapeID := constitutionRegistryMust(
		typedmemory.NewShapeID("U.ClaimGraphShape"),
	)
	shape := constitutionRegistryMust(
		typedmemory.NewValueShapeRef(shapeID, digest),
	)
	codec := constitutionRegistryMust(
		typedmemory.NewClaimGraphCodecV1(shape),
	)
	return constitutionRegistryMust(
		projectmemoryconstitution.NewCanonicalClaimGraph(value, codec),
	)
}

func constitutionRegistryRulePin(
	t *testing.T,
	role projectmemoryreferencescheme.SemanticRole,
	ruleRaw string,
	runtimeRequired bool,
) projectmemoryreferencescheme.ExactRulePin {
	t.Helper()
	revision := constitutionRegistryMust(
		typedmemory.NewSourceRevision("fpf:1d5c1ed:" + ruleRaw),
	)
	carrier := constitutionRegistryMust(
		typedmemory.NewCarrierRef("carrier:fpf:" + ruleRaw),
	)
	edition := constitutionRegistryMust(
		typedmemory.NewCarrierEdition("edition:1"),
	)
	source := constitutionRegistryMust(
		projectmemoryreferencescheme.NewExactSourceCarrierPin(
			projectmemoryreferencescheme.ExactSourceCarrierPinInput{
				SourceRevision: revision,
				Carrier:        carrier,
				Edition:        edition,
				Digest:         constitutionRegistryDigest(t, 'a'),
			},
		),
	)
	rule := constitutionRegistryMust(typedmemory.NewRuleRef(ruleRaw))
	runtime := constitutionRegistryRuntimeRequirement(
		t,
		ruleRaw,
		runtimeRequired,
	)
	input := projectmemoryreferencescheme.ExactRulePinInput{
		Role:    role,
		Rule:    rule,
		Source:  source,
		Runtime: runtime,
	}
	return constitutionRegistryMust(
		projectmemoryreferencescheme.NewExactRulePin(input),
	)
}

func constitutionRegistryRuntimeRequirement(
	t *testing.T,
	ruleRaw string,
	required bool,
) projectmemoryreferencescheme.RuntimeRequirement {
	t.Helper()
	if !required {
		return projectmemoryreferencescheme.NewRuntimeNotRequired()
	}
	artifact := constitutionRegistryMust(
		typedmemory.NewCarrierRef("mechanism:" + ruleRaw),
	)
	edition := constitutionRegistryMust(
		typedmemory.NewCarrierEdition("runtime:1"),
	)
	mechanism := constitutionRegistryMust(
		projectmemoryreferencescheme.NewExactRuntimeMechanismPin(
			projectmemoryreferencescheme.ExactRuntimeMechanismPinInput{
				Artifact: artifact,
				Edition:  edition,
				Digest:   constitutionRegistryDigest(t, 'b'),
			},
		),
	)
	return constitutionRegistryMust(
		projectmemoryreferencescheme.NewRuntimeRequired(mechanism),
	)
}

func constitutionRegistryIdentity(
	t *testing.T,
) typedmemoryevaluation.MechanismIdentity {
	t.Helper()
	artifact := constitutionRegistryMust(
		typedmemory.NewCarrierRef("artifact:episteme-constitution-registry-test"),
	)
	edition := constitutionRegistryMust(
		typedmemory.NewCarrierEdition("1.0.0"),
	)
	digest := constitutionRegistryDigest(t, 'c')
	return constitutionRegistryMust(
		typedmemoryevaluation.NewMechanismIdentity(
			artifact,
			edition,
			digest,
			typedmemoryevaluation.EvaluatorRole,
		),
	)
}

func constitutionRegistryRoleOutcomes(
	t *testing.T,
	fixture constitutionRegistryFixture,
	overrides map[projectmemoryreferencescheme.SemanticRole]constitutionOutcomePosture,
) []projectmemoryconstitution.RoleOutcome {
	t.Helper()
	mappings := projectmemoryreferencescheme.RoleRuntimeContracts()
	outcomes := make([]projectmemoryconstitution.RoleOutcome, len(mappings))
	for index, mapping := range mappings {
		posture := constitutionOutcomeSatisfied
		override, present := overrides[mapping.Role()]
		if present {
			posture = override
		}
		outcomes[index] = constitutionRegistryRoleOutcome(
			t,
			fixture,
			mapping,
			posture,
		)
	}
	return outcomes
}

func constitutionRegistryRoleOutcome(
	t *testing.T,
	fixture constitutionRegistryFixture,
	mapping projectmemoryreferencescheme.RoleRuntimeContract,
	posture constitutionOutcomePosture,
) projectmemoryconstitution.RoleOutcome {
	t.Helper()
	input := projectmemoryconstitution.RoleOutcomeInput{
		SchemeDigest:      fixture.scheme.Digest(),
		Role:              mapping.Role(),
		Contract:          mapping.Contract(),
		EvaluatedRulePins: fixture.pins[mapping.Role()],
	}
	switch posture {
	case constitutionOutcomeSatisfied:
		return constitutionRegistryMust(
			projectmemoryconstitution.NewRoleSatisfied(input),
		)
	case constitutionOutcomeContradicted:
		return constitutionRegistryMust(
			projectmemoryconstitution.NewRoleContradicted(input),
		)
	case constitutionOutcomeUnderdetermined:
		return constitutionRegistryMust(
			projectmemoryconstitution.NewRoleUnderdetermined(input),
		)
	default:
		t.Fatalf("unknown constitution outcome posture %d", posture)
		return nil
	}
}

func constitutionRegistryEvaluationInput(
	fixture constitutionRegistryFixture,
	basis projectmemoryconstitution.RuntimeEvaluationBasis,
) projectmemoryconstitution.EvaluationInput {
	return projectmemoryconstitution.EvaluationInput{
		ClaimGraph:      fixture.graph,
		EntityOfConcern: fixture.concern,
		ReferenceScheme: fixture.scheme,
		RuntimeBasis:    basis,
	}
}

func constitutionRegistryAssertInvalid(
	t *testing.T,
	result projectmemoryconstitution.Result,
	want projectmemoryconstitution.Reason,
) {
	t.Helper()
	invalid, ok := result.(projectmemoryconstitution.Invalid)
	if !ok {
		t.Fatalf("result = %T, want Invalid(%s)", result, want)
	}
	if invalid.Reason() != want {
		t.Fatalf("Invalid reason = %s, want %s", invalid.Reason(), want)
	}
}

func constitutionRegistryAssertUnderdetermined(
	t *testing.T,
	result projectmemoryconstitution.Result,
	want projectmemoryconstitution.Reason,
) {
	t.Helper()
	underdetermined, ok := result.(projectmemoryconstitution.Underdetermined)
	if !ok {
		t.Fatalf("result = %T, want Underdetermined(%s)", result, want)
	}
	if underdetermined.Reason() != want {
		t.Fatalf(
			"Underdetermined reason = %s, want %s",
			underdetermined.Reason(),
			want,
		)
	}
}

func constitutionRegistryDigest(
	t *testing.T,
	fill byte,
) typedmemory.SHA256Digest {
	t.Helper()
	raw := "sha256:" + strings.Repeat(string(fill), 64)
	return constitutionRegistryMust(typedmemory.NewSHA256Digest(raw))
}

func constitutionRegistryMust[T any](
	value T,
	err error,
) T {
	if err != nil {
		panic(err)
	}
	return value
}
