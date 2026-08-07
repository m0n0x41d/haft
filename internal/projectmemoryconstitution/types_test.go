package projectmemoryconstitution

import (
	"bytes"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemoryreferencescheme"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type outcomePosture uint8

const (
	outcomeSatisfied outcomePosture = iota + 1
	outcomeContradicted
	outcomeUnderdetermined
)

type constitutionFixture struct {
	graph   CanonicalClaimGraph
	concern ResolvedEntityOfConcern
	scheme  projectmemoryreferencescheme.ProjectMemoryReferenceSchemeV1
	pins    map[projectmemoryreferencescheme.SemanticRole][]projectmemoryreferencescheme.ExactRulePin
}

func TestEvaluateProducesEqualCoordinateForEqualCanonicalTriple(t *testing.T) {
	first := newConstitutionFixture(t, []string{"claim-a", "claim-b"}, "entity:target", "scheme-a")
	second := newConstitutionFixture(t, []string{"claim-b", "claim-a"}, "entity:target", "scheme-a")
	firstCoordinate := evaluateSatisfiedCoordinate(t, first)
	secondCoordinate := evaluateSatisfiedCoordinate(t, second)

	if firstCoordinate != secondCoordinate {
		t.Fatal("equal canonical ClaimGraph, EntityID, and scheme digest yielded distinct coordinates")
	}
	if !bytes.Equal(
		firstCoordinate.ClaimGraphCanonicalBytes(),
		secondCoordinate.ClaimGraphCanonicalBytes(),
	) {
		t.Fatal("equal coordinate exposed different ClaimGraph canonical bytes")
	}
	if firstCoordinate.EntityID() != first.concern.EntityID() {
		t.Fatal("coordinate lost resolved EntityID")
	}
	if firstCoordinate.ReferenceSchemeDigest() != first.scheme.Digest() {
		t.Fatal("coordinate lost intrinsic ReferenceScheme digest")
	}
}

func TestEachC21DiscriminatorChangesEpistemeCoordinate(t *testing.T) {
	baseline := newConstitutionFixture(t, []string{"claim-a"}, "entity:target", "scheme-a")
	changedGraph := newConstitutionFixture(t, []string{"claim-b"}, "entity:target", "scheme-a")
	changedEntity := newConstitutionFixture(t, []string{"claim-a"}, "entity:other", "scheme-a")
	changedScheme := newConstitutionFixture(t, []string{"claim-a"}, "entity:target", "scheme-b")
	baselineCoordinate := evaluateSatisfiedCoordinate(t, baseline)
	candidates := []struct {
		name       string
		coordinate EpistemeCoordinate
	}{
		{name: "ClaimGraph", coordinate: evaluateSatisfiedCoordinate(t, changedGraph)},
		{name: "EntityOfConcern", coordinate: evaluateSatisfiedCoordinate(t, changedEntity)},
		{name: "ReferenceScheme", coordinate: evaluateSatisfiedCoordinate(t, changedScheme)},
	}

	for _, candidate := range candidates {
		candidate := candidate
		t.Run(candidate.name, func(t *testing.T) {
			if candidate.coordinate == baselineCoordinate {
				t.Fatalf("changing %s did not change episteme identity", candidate.name)
			}
		})
	}
}

func TestEpistemeCoordinateExcludesGroundingTimeTypeEnvAndResolutionBasis(t *testing.T) {
	coordinateType := reflect.TypeFor[EpistemeCoordinate]()
	fieldNames := make([]string, coordinateType.NumField())
	for index := range coordinateType.NumField() {
		fieldNames[index] = coordinateType.Field(index).Name
	}
	want := []string{
		"claimGraphCanonical",
		"entityID",
		"referenceSchemeDigest",
	}
	if !slices.Equal(fieldNames, want) {
		t.Fatalf("EpistemeCoordinate fields = %v; want only %v", fieldNames, want)
	}

	inputType := reflect.TypeFor[EvaluationInput]()
	forbidden := []string{
		"Grounding",
		"GammaTime",
		"TypeEnv",
		"GraphRevision",
		"ResolutionBasis",
		"Publication",
		"Carrier",
	}
	for _, name := range forbidden {
		if _, present := inputType.FieldByName(name); present {
			t.Fatalf("EvaluationInput leaks non-identity coordinate %s", name)
		}
	}
}

func TestCoordinateAndBasisOwnMutableInputs(t *testing.T) {
	fixture := newConstitutionFixture(t, []string{"claim-a"}, "entity:target", "scheme-a")
	outcomes := fixtureRoleOutcomes(t, fixture, nil)
	basis := NewRoleRuntimeEvaluationBasis(outcomes)
	outcomes[0] = mustRoleOutcome(
		t,
		fixture,
		projectmemoryreferencescheme.SemanticRoleDesignation,
		outcomeContradicted,
	)
	returned := basis.Outcomes()
	returned[0] = outcomes[0]
	input := EvaluationInput{
		ClaimGraph:      fixture.graph,
		EntityOfConcern: fixture.concern,
		ReferenceScheme: fixture.scheme,
		RuntimeBasis:    basis,
	}
	result := Evaluate(input)
	satisfied, ok := result.(Satisfied)
	if !ok {
		t.Fatalf("mutating caller slices changed sealed basis: %T", result)
	}
	coordinate := satisfied.Coordinate()
	canonical := coordinate.ClaimGraphCanonicalBytes()
	canonical[0] ^= 0xff
	if bytes.Equal(canonical, coordinate.ClaimGraphCanonicalBytes()) {
		t.Fatal("coordinate exposed mutable ClaimGraph identity bytes")
	}
}

func TestEvaluateKeepsMissingRuntimeBasisUnderdetermined(t *testing.T) {
	fixture := newConstitutionFixture(t, []string{"claim-a"}, "entity:target", "scheme-a")
	cases := []struct {
		name  string
		basis RuntimeEvaluationBasis
	}{
		{name: "absent basis", basis: NewMissingRuntimeEvaluationBasis()},
		{name: "nil basis", basis: nil},
		{name: "registry availability without outcomes", basis: NewRoleRuntimeEvaluationBasis(nil)},
		{
			name: "one required role outcome missing",
			basis: NewRoleRuntimeEvaluationBasis(
				fixtureRoleOutcomesWithout(
					t,
					fixture,
					projectmemoryreferencescheme.SemanticRoleEvaluation,
				),
			),
		},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			result := evaluateWithBasis(fixture, testCase.basis)
			assertUnderdeterminedReason(
				t,
				result,
				ReasonReferenceSchemeRuntimeBasisMissing,
			)
		})
	}
}

func TestEvaluateRejectsMalformedDuplicateAndMismatchedOutcomes(t *testing.T) {
	fixture := newConstitutionFixture(t, []string{"claim-a"}, "entity:target", "scheme-a")
	otherScheme := newConstitutionFixture(t, []string{"claim-a"}, "entity:target", "scheme-b")
	exact := fixtureRoleOutcomes(t, fixture, nil)
	duplicate := append(slices.Clone(exact), exact[0])
	mismatchedDigest := fixtureRoleOutcomes(t, otherScheme, nil)
	mismatchedPins := slices.Clone(exact)
	mismatchedPins[0] = mustRoleOutcomeWithPins(
		t,
		fixture.scheme.Digest(),
		projectmemoryreferencescheme.SemanticRoleDesignation,
		otherScheme.pins[projectmemoryreferencescheme.SemanticRoleDesignation],
		outcomeSatisfied,
	)
	malformed := slices.Clone(exact)
	malformed[0] = RoleSatisfied{}
	wrongContract := slices.Clone(exact)
	wrongContract[0] = RoleSatisfied{state: roleOutcomeState{
		schemeDigest: fixture.scheme.Digest(),
		role:         projectmemoryreferencescheme.SemanticRoleDesignation,
		contract:     projectmemoryreferencescheme.RuntimeContractClaimEvaluation,
		rulePins: slices.Clone(
			fixture.pins[projectmemoryreferencescheme.SemanticRoleDesignation],
		),
	}}
	cases := []struct {
		name     string
		outcomes []RoleOutcome
	}{
		{name: "duplicate role", outcomes: duplicate},
		{name: "mismatched scheme digest", outcomes: mismatchedDigest},
		{name: "mismatched exact rule-pin group", outcomes: mismatchedPins},
		{name: "malformed outcome", outcomes: malformed},
		{name: "role-contract mismatch", outcomes: wrongContract},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			basis := NewRoleRuntimeEvaluationBasis(testCase.outcomes)
			result := evaluateWithBasis(fixture, basis)
			assertInvalidReason(
				t,
				result,
				ReasonReferenceSchemeRuntimeBasisInvalid,
			)
		})
	}
}

func TestEvaluatePreservesUnderdeterminedAndContradictedPostures(t *testing.T) {
	fixture := newConstitutionFixture(t, []string{"claim-a"}, "entity:target", "scheme-a")
	cases := []struct {
		name    string
		posture outcomePosture
		assert  func(*testing.T, Result, Reason)
	}{
		{
			name:    "explicit underdetermined role",
			posture: outcomeUnderdetermined,
			assert:  assertUnderdeterminedReason,
		},
		{
			name:    "explicit contradicted role",
			posture: outcomeContradicted,
			assert:  assertInvalidReason,
		},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			overrides := map[projectmemoryreferencescheme.SemanticRole]outcomePosture{
				projectmemoryreferencescheme.SemanticRoleInterpretation: testCase.posture,
			}
			outcomes := fixtureRoleOutcomes(t, fixture, overrides)
			basis := NewRoleRuntimeEvaluationBasis(outcomes)
			result := evaluateWithBasis(fixture, basis)
			testCase.assert(t, result, ReasonEpistemeConstitutionNotSatisfied)
		})
	}
}

func TestRoleOutcomesRequireExactNormalizedRulePinGroups(t *testing.T) {
	fixture := newConstitutionFixture(t, []string{"claim-a"}, "entity:target", "scheme-a")
	designation := slices.Clone(
		fixture.pins[projectmemoryreferencescheme.SemanticRoleDesignation],
	)
	slices.Reverse(designation)
	outcome := mustRoleOutcomeWithPins(
		t,
		fixture.scheme.Digest(),
		projectmemoryreferencescheme.SemanticRoleDesignation,
		designation,
		outcomeSatisfied,
	)
	normalized := outcome.EvaluatedRulePins()
	if normalized[0].Rule().String() > normalized[1].Rule().String() {
		t.Fatal("role outcome retained caller rule-pin order")
	}

	duplicate := []projectmemoryreferencescheme.ExactRulePin{
		designation[0],
		designation[0],
	}
	contract := mustRuntimeContract(
		t,
		projectmemoryreferencescheme.SemanticRoleDesignation,
	)
	_, err := NewRoleSatisfied(RoleOutcomeInput{
		SchemeDigest:      fixture.scheme.Digest(),
		Role:              projectmemoryreferencescheme.SemanticRoleDesignation,
		Contract:          contract,
		EvaluatedRulePins: duplicate,
	})
	if err == nil {
		t.Fatal("duplicate outcome RuleRef was accepted")
	}
}

func TestEvaluateRejectsInvalidConstitutionParticipants(t *testing.T) {
	fixture := newConstitutionFixture(t, []string{"claim-a"}, "entity:target", "scheme-a")
	basis := NewRoleRuntimeEvaluationBasis(fixtureRoleOutcomes(t, fixture, nil))
	cases := []struct {
		name   string
		input  EvaluationInput
		reason Reason
	}{
		{
			name: "ClaimGraph",
			input: EvaluationInput{
				EntityOfConcern: fixture.concern,
				ReferenceScheme: fixture.scheme,
				RuntimeBasis:    basis,
			},
			reason: ReasonClaimGraphBasisInvalid,
		},
		{
			name: "EntityOfConcern",
			input: EvaluationInput{
				ClaimGraph:      fixture.graph,
				ReferenceScheme: fixture.scheme,
				RuntimeBasis:    basis,
			},
			reason: ReasonEntityOfConcernBasisInvalid,
		},
		{
			name: "ReferenceScheme",
			input: EvaluationInput{
				ClaimGraph:      fixture.graph,
				EntityOfConcern: fixture.concern,
				RuntimeBasis:    basis,
			},
			reason: ReasonReferenceSchemeBasisInvalid,
		},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			result := Evaluate(testCase.input)
			assertInvalidReason(t, result, testCase.reason)
		})
	}
}

func newConstitutionFixture(
	t *testing.T,
	claims []string,
	entityRaw string,
	schemeSuffix string,
) constitutionFixture {
	t.Helper()
	graph := fixtureCanonicalClaimGraph(t, claims)
	entity := mustResult(t, capture(typedmemory.NewEntityID(entityRaw)))
	concern := mustResult(t, capture(NewResolvedEntityOfConcern(entity)))
	designation := []projectmemoryreferencescheme.ExactRulePin{
		fixtureRulePin(
			t,
			projectmemoryreferencescheme.SemanticRoleDesignation,
			"designation:a:"+schemeSuffix,
			true,
		),
		fixtureRulePin(
			t,
			projectmemoryreferencescheme.SemanticRoleDesignation,
			"designation:b:"+schemeSuffix,
			true,
		),
	}
	interpretation := []projectmemoryreferencescheme.ExactRulePin{
		fixtureRulePin(
			t,
			projectmemoryreferencescheme.SemanticRoleInterpretation,
			"interpretation:"+schemeSuffix,
			true,
		),
	}
	measurement := []projectmemoryreferencescheme.ExactRulePin{
		fixtureRulePin(
			t,
			projectmemoryreferencescheme.SemanticRoleMeasurement,
			"measurement:"+schemeSuffix,
			true,
		),
	}
	evaluation := []projectmemoryreferencescheme.ExactRulePin{
		fixtureRulePin(
			t,
			projectmemoryreferencescheme.SemanticRoleEvaluation,
			"evaluation:not-applicable:"+schemeSuffix,
			false,
		),
	}
	designationRules := mustResult(
		t,
		capture(projectmemoryreferencescheme.NewDesignationRules(designation)),
	)
	interpretationRules := mustResult(
		t,
		capture(projectmemoryreferencescheme.NewInterpretationRules(interpretation)),
	)
	measurementRules := mustResult(
		t,
		capture(projectmemoryreferencescheme.NewMeasurementRules(measurement)),
	)
	evaluationBranch := mustResult(
		t,
		capture(projectmemoryreferencescheme.NewEvaluationNotApplicable(evaluation[0])),
	)
	scheme := mustResult(
		t,
		capture(projectmemoryreferencescheme.NewProjectMemoryReferenceSchemeV1(
			designationRules,
			interpretationRules,
			measurementRules,
			evaluationBranch,
		)),
	)
	pins := map[projectmemoryreferencescheme.SemanticRole][]projectmemoryreferencescheme.ExactRulePin{
		projectmemoryreferencescheme.SemanticRoleDesignation:    designation,
		projectmemoryreferencescheme.SemanticRoleInterpretation: interpretation,
		projectmemoryreferencescheme.SemanticRoleMeasurement:    measurement,
		projectmemoryreferencescheme.SemanticRoleEvaluation:     evaluation,
	}
	return constitutionFixture{
		graph:   graph,
		concern: concern,
		scheme:  scheme,
		pins:    pins,
	}
}

func fixtureCanonicalClaimGraph(
	t *testing.T,
	claims []string,
) CanonicalClaimGraph {
	t.Helper()
	digest := fixtureDigest(t, '1')
	typeEnv := mustResult(t, capture(typedmemory.NewTypeEnvRef(digest)))
	kindID := mustResult(t, capture(typedmemory.NewKindID("U.ProjectClaim")))
	valueKind := mustResult(t, capture(typedmemory.NewValueKindRef(typeEnv, kindID)))
	nodes := make([]typedmemory.ClaimNode, len(claims))
	for index, claim := range claims {
		identifier := mustResult(
			t,
			capture(typedmemory.NewClaimNodeID(claim)),
		)
		nodes[index] = mustResult(
			t,
			capture(typedmemory.NewClaimNode(
				identifier,
				valueKind,
				typedmemory.NewTextValue(claim),
			)),
		)
	}
	value := mustResult(t, capture(typedmemory.NewClaimGraphValue(nodes, nil)))
	shapeID := mustResult(t, capture(typedmemory.NewShapeID("U.ClaimGraphShape")))
	shape := mustResult(t, capture(typedmemory.NewValueShapeRef(shapeID, digest)))
	codec := mustResult(t, capture(typedmemory.NewClaimGraphCodecV1(shape)))
	return mustResult(t, capture(NewCanonicalClaimGraph(value, codec)))
}

func fixtureRulePin(
	t *testing.T,
	role projectmemoryreferencescheme.SemanticRole,
	ruleRaw string,
	runtimeRequired bool,
) projectmemoryreferencescheme.ExactRulePin {
	t.Helper()
	revision := mustResult(
		t,
		capture(typedmemory.NewSourceRevision("fpf:1d5c1ed:"+ruleRaw)),
	)
	carrier := mustResult(
		t,
		capture(typedmemory.NewCarrierRef("carrier:fpf:"+ruleRaw)),
	)
	edition := mustResult(t, capture(typedmemory.NewCarrierEdition("edition:1")))
	source := mustResult(
		t,
		capture(projectmemoryreferencescheme.NewExactSourceCarrierPin(
			projectmemoryreferencescheme.ExactSourceCarrierPinInput{
				SourceRevision: revision,
				Carrier:        carrier,
				Edition:        edition,
				Digest:         fixtureDigest(t, 'a'),
			},
		)),
	)
	rule := mustResult(t, capture(typedmemory.NewRuleRef(ruleRaw)))
	runtime := fixtureRuntimeRequirement(t, ruleRaw, runtimeRequired)
	return mustResult(
		t,
		capture(projectmemoryreferencescheme.NewExactRulePin(
			projectmemoryreferencescheme.ExactRulePinInput{
				Role:    role,
				Rule:    rule,
				Source:  source,
				Runtime: runtime,
			},
		)),
	)
}

func fixtureRuntimeRequirement(
	t *testing.T,
	ruleRaw string,
	required bool,
) projectmemoryreferencescheme.RuntimeRequirement {
	t.Helper()
	if !required {
		return projectmemoryreferencescheme.NewRuntimeNotRequired()
	}
	artifact := mustResult(
		t,
		capture(typedmemory.NewCarrierRef("mechanism:"+ruleRaw)),
	)
	edition := mustResult(t, capture(typedmemory.NewCarrierEdition("runtime:1")))
	mechanism := mustResult(
		t,
		capture(projectmemoryreferencescheme.NewExactRuntimeMechanismPin(
			projectmemoryreferencescheme.ExactRuntimeMechanismPinInput{
				Artifact: artifact,
				Edition:  edition,
				Digest:   fixtureDigest(t, 'b'),
			},
		)),
	)
	return mustResult(
		t,
		capture(projectmemoryreferencescheme.NewRuntimeRequired(mechanism)),
	)
}

func fixtureRoleOutcomes(
	t *testing.T,
	fixture constitutionFixture,
	overrides map[projectmemoryreferencescheme.SemanticRole]outcomePosture,
) []RoleOutcome {
	t.Helper()
	mappings := projectmemoryreferencescheme.RoleRuntimeContracts()
	outcomes := make([]RoleOutcome, len(mappings))
	for index, mapping := range mappings {
		posture := outcomeSatisfied
		if override, present := overrides[mapping.Role()]; present {
			posture = override
		}
		outcomes[index] = mustRoleOutcome(t, fixture, mapping.Role(), posture)
	}
	return outcomes
}

func fixtureRoleOutcomesWithout(
	t *testing.T,
	fixture constitutionFixture,
	omitted projectmemoryreferencescheme.SemanticRole,
) []RoleOutcome {
	t.Helper()
	outcomes := fixtureRoleOutcomes(t, fixture, nil)
	return slices.DeleteFunc(outcomes, func(outcome RoleOutcome) bool {
		return outcome.Role() == omitted
	})
}

func mustRoleOutcome(
	t *testing.T,
	fixture constitutionFixture,
	role projectmemoryreferencescheme.SemanticRole,
	posture outcomePosture,
) RoleOutcome {
	t.Helper()
	return mustRoleOutcomeWithPins(
		t,
		fixture.scheme.Digest(),
		role,
		fixture.pins[role],
		posture,
	)
}

func mustRoleOutcomeWithPins(
	t *testing.T,
	digest projectmemoryreferencescheme.ReferenceSchemeDigest,
	role projectmemoryreferencescheme.SemanticRole,
	pins []projectmemoryreferencescheme.ExactRulePin,
	posture outcomePosture,
) RoleOutcome {
	t.Helper()
	contract := mustRuntimeContract(t, role)
	input := RoleOutcomeInput{
		SchemeDigest:      digest,
		Role:              role,
		Contract:          contract,
		EvaluatedRulePins: pins,
	}
	switch posture {
	case outcomeSatisfied:
		return mustResult(t, capture(NewRoleSatisfied(input)))
	case outcomeContradicted:
		return mustResult(t, capture(NewRoleContradicted(input)))
	case outcomeUnderdetermined:
		return mustResult(t, capture(NewRoleUnderdetermined(input)))
	default:
		t.Fatalf("unknown outcome posture %d", posture)
		return nil
	}
}

func mustRuntimeContract(
	t *testing.T,
	role projectmemoryreferencescheme.SemanticRole,
) projectmemoryreferencescheme.RuntimeContract {
	t.Helper()
	return mustResult(
		t,
		capture(projectmemoryreferencescheme.RuntimeContractForRole(role)),
	)
}

func evaluateSatisfiedCoordinate(
	t *testing.T,
	fixture constitutionFixture,
) EpistemeCoordinate {
	t.Helper()
	basis := NewRoleRuntimeEvaluationBasis(fixtureRoleOutcomes(t, fixture, nil))
	result := evaluateWithBasis(fixture, basis)
	satisfied, ok := result.(Satisfied)
	if !ok {
		t.Fatalf("Evaluate() = %T; want Satisfied", result)
	}
	return satisfied.Coordinate()
}

func evaluateWithBasis(
	fixture constitutionFixture,
	basis RuntimeEvaluationBasis,
) Result {
	input := EvaluationInput{
		ClaimGraph:      fixture.graph,
		EntityOfConcern: fixture.concern,
		ReferenceScheme: fixture.scheme,
		RuntimeBasis:    basis,
	}
	return Evaluate(input)
}

func assertInvalidReason(t *testing.T, result Result, want Reason) {
	t.Helper()
	invalid, ok := result.(Invalid)
	if !ok {
		t.Fatalf("result = %T; want Invalid(%s)", result, want)
	}
	if invalid.Reason() != want {
		t.Fatalf("Invalid reason = %s; want %s", invalid.Reason(), want)
	}
}

func assertUnderdeterminedReason(t *testing.T, result Result, want Reason) {
	t.Helper()
	underdetermined, ok := result.(Underdetermined)
	if !ok {
		t.Fatalf("result = %T; want Underdetermined(%s)", result, want)
	}
	if underdetermined.Reason() != want {
		t.Fatalf(
			"Underdetermined reason = %s; want %s",
			underdetermined.Reason(),
			want,
		)
	}
}

func fixtureDigest(t *testing.T, fill byte) typedmemory.SHA256Digest {
	t.Helper()
	raw := "sha256:" + strings.Repeat(string(fill), 64)
	return mustResult(t, capture(typedmemory.NewSHA256Digest(raw)))
}

type capturedResult[T any] struct {
	value T
	err   error
}

func capture[T any](value T, err error) capturedResult[T] {
	return capturedResult[T]{value: value, err: err}
}

func mustResult[T any](t *testing.T, result capturedResult[T]) T {
	t.Helper()
	if result.err != nil {
		t.Fatalf("fixture constructor: %v", result.err)
	}
	return result.value
}
