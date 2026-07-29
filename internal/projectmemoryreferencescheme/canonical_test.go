package projectmemoryreferencescheme

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestProjectMemoryReferenceSchemeV1NormalizesPinPermutation(t *testing.T) {
	designationA := fixtureRulePin(t, SemanticRoleDesignation, "designation:a", "source-a", runtimeAbsent)
	designationB := fixtureRulePin(t, SemanticRoleDesignation, "designation:b", "source-b", runtimePresent)
	interpretationA := fixtureRulePin(t, SemanticRoleInterpretation, "interpretation:a", "source-a", runtimePresent)
	interpretationB := fixtureRulePin(t, SemanticRoleInterpretation, "interpretation:b", "source-b", runtimeAbsent)
	measurementA := fixtureRulePin(t, SemanticRoleMeasurement, "measurement:a", "source-a", runtimePresent)
	measurementB := fixtureRulePin(t, SemanticRoleMeasurement, "measurement:b", "source-b", runtimeAbsent)
	evaluation := fixtureRulePin(t, SemanticRoleEvaluation, "evaluation:na", "source-a", runtimePresent)

	left := fixtureScheme(
		t,
		[]ExactRulePin{designationA, designationB},
		[]ExactRulePin{interpretationA, interpretationB},
		mustMeasurementRules(t, []ExactRulePin{measurementA, measurementB}),
		mustEvaluationNotApplicable(t, evaluation),
	)
	right := fixtureScheme(
		t,
		[]ExactRulePin{designationB, designationA},
		[]ExactRulePin{interpretationB, interpretationA},
		mustMeasurementRules(t, []ExactRulePin{measurementB, measurementA}),
		mustEvaluationNotApplicable(t, evaluation),
	)

	if left.Digest() != right.Digest() {
		t.Fatalf("permutation changed digest: %q != %q", left.Digest(), right.Digest())
	}
	if !bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes()) {
		t.Fatal("permutation changed canonical bytes")
	}
	if err := left.Verify(); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	decoded, err := VerifyProjectMemoryReferenceSchemeV1(
		left.Digest(),
		left.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("VerifyProjectMemoryReferenceSchemeV1() error = %v", err)
	}
	if decoded.Digest() != left.Digest() {
		t.Fatalf("decoded digest = %q, want %q", decoded.Digest(), left.Digest())
	}

	designationCopy := left.Designation().Pins()
	designationCopy[0] = designationB
	canonicalCopy := left.CanonicalBytes()
	canonicalCopy[0] ^= 0xff
	if !bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes()) {
		t.Fatal("accessor mutation changed sealed canonical bytes")
	}
}

func TestProjectMemoryReferenceSchemeV1DigestChangesWithEveryExactPinCoordinate(t *testing.T) {
	baseDesignation := rulePinFixtureInput{
		role:             SemanticRoleDesignation,
		rule:             "designation:resolve",
		sourceRevision:   "fpf-revision:a",
		sourceCarrier:    "carrier:fpf-spec",
		sourceEdition:    "edition:1",
		sourceDigestByte: '1',
		runtime:          runtimePresent,
		runtimeArtifact:  "mechanism:designation",
		runtimeEdition:   "build-20260720.1",
		runtimeDigest:    '2',
	}
	base := schemeWithDesignation(t, rulePinFromFixtureInput(t, baseDesignation))
	cases := []struct {
		name   string
		mutate func(rulePinFixtureInput) rulePinFixtureInput
	}{
		{
			name: "RuleRef",
			mutate: func(input rulePinFixtureInput) rulePinFixtureInput {
				input.rule = "designation:other"
				return input
			},
		},
		{
			name: "source revision",
			mutate: func(input rulePinFixtureInput) rulePinFixtureInput {
				input.sourceRevision = "fpf-revision:b"
				return input
			},
		},
		{
			name: "source carrier",
			mutate: func(input rulePinFixtureInput) rulePinFixtureInput {
				input.sourceCarrier = "carrier:other"
				return input
			},
		},
		{
			name: "source edition",
			mutate: func(input rulePinFixtureInput) rulePinFixtureInput {
				input.sourceEdition = "edition:2"
				return input
			},
		},
		{
			name: "source digest",
			mutate: func(input rulePinFixtureInput) rulePinFixtureInput {
				input.sourceDigestByte = '3'
				return input
			},
		},
		{
			name: "runtime branch",
			mutate: func(input rulePinFixtureInput) rulePinFixtureInput {
				input.runtime = runtimeAbsent
				return input
			},
		},
		{
			name: "runtime artifact",
			mutate: func(input rulePinFixtureInput) rulePinFixtureInput {
				input.runtimeArtifact = "mechanism:other"
				return input
			},
		},
		{
			name: "runtime edition",
			mutate: func(input rulePinFixtureInput) rulePinFixtureInput {
				input.runtimeEdition = "build-20260720.2"
				return input
			},
		},
		{
			name: "runtime digest",
			mutate: func(input rulePinFixtureInput) rulePinFixtureInput {
				input.runtimeDigest = '4'
				return input
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			changedInput := testCase.mutate(baseDesignation)
			changedPin := rulePinFromFixtureInput(t, changedInput)
			changed := schemeWithDesignation(t, changedPin)
			if changed.Digest() == base.Digest() {
				t.Fatalf("%s change preserved digest %q", testCase.name, base.Digest())
			}
		})
	}
}

func TestProjectMemoryReferenceSchemeV1RequiresEveryClosedGroup(t *testing.T) {
	designation := fixtureRulePin(t, SemanticRoleDesignation, "designation:one", "source", runtimeAbsent)
	interpretation := fixtureRulePin(t, SemanticRoleInterpretation, "interpretation:one", "source", runtimeAbsent)
	measurement := fixtureRulePin(t, SemanticRoleMeasurement, "measurement:one", "source", runtimeAbsent)
	evaluation := fixtureRulePin(t, SemanticRoleEvaluation, "evaluation:one", "source", runtimeAbsent)

	if _, err := NewDesignationRules(nil); err == nil || !strings.Contains(err.Error(), "at least one") {
		t.Fatalf("empty designation error = %v", err)
	}
	if _, err := NewInterpretationRules(nil); err == nil || !strings.Contains(err.Error(), "at least one") {
		t.Fatalf("empty interpretation error = %v", err)
	}
	if _, err := NewMeasurementRules(nil); err == nil || !strings.Contains(err.Error(), "at least one") {
		t.Fatalf("empty measurement error = %v", err)
	}
	if _, err := NewEvaluationRules(nil); err == nil || !strings.Contains(err.Error(), "at least one") {
		t.Fatalf("empty evaluation error = %v", err)
	}
	if _, err := NewDesignationRules([]ExactRulePin{designation, designation}); err == nil ||
		!strings.Contains(err.Error(), "duplicate RuleRef") {
		t.Fatalf("duplicate designation RuleRef error = %v", err)
	}
	if _, err := NewMeasurementNotApplicable(evaluation); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("wrong measurement NotApplicable role error = %v", err)
	}
	if _, err := NewEvaluationNotApplicable(measurement); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("wrong evaluation NotApplicable role error = %v", err)
	}

	designationRules := mustDesignationRules(t, []ExactRulePin{designation})
	interpretationRules := mustInterpretationRules(t, []ExactRulePin{interpretation})
	measurementRules := mustMeasurementRules(t, []ExactRulePin{measurement})
	evaluationRules := mustEvaluationRules(t, []ExactRulePin{evaluation})
	if _, err := NewProjectMemoryReferenceSchemeV1(
		designationRules,
		interpretationRules,
		nil,
		evaluationRules,
	); err == nil || !strings.Contains(err.Error(), "measurement") {
		t.Fatalf("missing measurement branch error = %v", err)
	}
	if _, err := NewProjectMemoryReferenceSchemeV1(
		designationRules,
		interpretationRules,
		measurementRules,
		nil,
	); err == nil || !strings.Contains(err.Error(), "evaluation") {
		t.Fatalf("missing evaluation branch error = %v", err)
	}
	if _, err := NewExactRulePin(ExactRulePinInput{
		Role:   SemanticRoleDesignation,
		Rule:   designation.Rule(),
		Source: designation.Source(),
	}); err == nil || !strings.Contains(err.Error(), "runtime requirement") {
		t.Fatalf("missing runtime branch error = %v", err)
	}
}

func TestProjectMemoryReferenceSchemeV1KeepsRuntimePinAsARealDistinction(t *testing.T) {
	withoutRuntime := schemeWithDesignation(
		t,
		fixtureRulePin(t, SemanticRoleDesignation, "designation:resolve", "source", runtimeAbsent),
	)
	withRuntime := schemeWithDesignation(
		t,
		fixtureRulePin(t, SemanticRoleDesignation, "designation:resolve", "source", runtimePresent),
	)
	if withoutRuntime.Digest() == withRuntime.Digest() {
		t.Fatal("runtime mechanism requirement did not change scheme identity")
	}

	decoded, err := DecodeProjectMemoryReferenceSchemeV1(withRuntime.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeProjectMemoryReferenceSchemeV1() error = %v", err)
	}
	runtime := decoded.Designation().Pins()[0].Runtime()
	required, ok := runtime.(RuntimeRequired)
	if !ok {
		t.Fatalf("decoded runtime requirement = %T, want RuntimeRequired", runtime)
	}
	if required.Mechanism().Artifact().String() != "mechanism:designation:resolve" {
		t.Fatalf("runtime artifact = %q", required.Mechanism().Artifact())
	}
}

func TestProjectMemoryReferenceSchemeV1KeepsRulesAndNotApplicableBranchesDistinct(t *testing.T) {
	designation := fixtureRulePin(
		t,
		SemanticRoleDesignation,
		"designation:resolve",
		"source",
		runtimeAbsent,
	)
	interpretation := fixtureRulePin(
		t,
		SemanticRoleInterpretation,
		"interpretation:claims",
		"source",
		runtimeAbsent,
	)
	measurement := fixtureRulePin(
		t,
		SemanticRoleMeasurement,
		"measurement:branch",
		"source",
		runtimeAbsent,
	)
	evaluation := fixtureRulePin(
		t,
		SemanticRoleEvaluation,
		"evaluation:branch",
		"source",
		runtimeAbsent,
	)
	rules := fixtureScheme(
		t,
		[]ExactRulePin{designation},
		[]ExactRulePin{interpretation},
		mustMeasurementRules(t, []ExactRulePin{measurement}),
		mustEvaluationRules(t, []ExactRulePin{evaluation}),
	)
	notApplicable := fixtureScheme(
		t,
		[]ExactRulePin{designation},
		[]ExactRulePin{interpretation},
		mustMeasurementNotApplicable(t, measurement),
		mustEvaluationNotApplicable(t, evaluation),
	)
	if rules.Digest() == notApplicable.Digest() {
		t.Fatal("Rules and NotApplicable branches collapsed to one identity")
	}
	if _, ok := rules.Measurement().(MeasurementRules); !ok {
		t.Fatalf("measurement branch = %T, want MeasurementRules", rules.Measurement())
	}
	if _, ok := notApplicable.Evaluation().(EvaluationNotApplicable); !ok {
		t.Fatalf(
			"evaluation branch = %T, want EvaluationNotApplicable",
			notApplicable.Evaluation(),
		)
	}
}

func TestProjectMemoryReferenceSchemeV1RejectsNonCanonicalTamperedAndLegacyBytes(t *testing.T) {
	designationA := fixtureRulePin(t, SemanticRoleDesignation, "designation:a", "source-a", runtimeAbsent)
	designationB := fixtureRulePin(t, SemanticRoleDesignation, "designation:b", "source-b", runtimeAbsent)
	interpretation := fixtureRulePin(t, SemanticRoleInterpretation, "interpretation:a", "source", runtimeAbsent)
	measurement := fixtureRulePin(t, SemanticRoleMeasurement, "measurement:a", "source", runtimeAbsent)
	evaluation := fixtureRulePin(t, SemanticRoleEvaluation, "evaluation:a", "source", runtimeAbsent)
	base := fixtureScheme(
		t,
		[]ExactRulePin{designationA, designationB},
		[]ExactRulePin{interpretation},
		mustMeasurementNotApplicable(t, measurement),
		mustEvaluationNotApplicable(t, evaluation),
	)

	trailing := append(base.CanonicalBytes(), 0)
	if _, err := DecodeProjectMemoryReferenceSchemeV1(trailing); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("trailing bytes error = %v", err)
	}

	nonCanonicalWriter := newSchemeWriter()
	nonCanonicalWriter.addRulePins([]ExactRulePin{designationB, designationA})
	nonCanonicalWriter.addRulePins([]ExactRulePin{interpretation})
	if err := nonCanonicalWriter.addMeasurementBranch(mustMeasurementNotApplicable(t, measurement)); err != nil {
		t.Fatalf("addMeasurementBranch() error = %v", err)
	}
	if err := nonCanonicalWriter.addEvaluationBranch(mustEvaluationNotApplicable(t, evaluation)); err != nil {
		t.Fatalf("addEvaluationBranch() error = %v", err)
	}
	if _, err := DecodeProjectMemoryReferenceSchemeV1(nonCanonicalWriter.bytes()); err == nil ||
		!strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("noncanonical order error = %v", err)
	}

	changed := schemeWithDesignation(
		t,
		fixtureRulePin(t, SemanticRoleDesignation, "designation:changed", "source", runtimeAbsent),
	)
	if _, err := VerifyProjectMemoryReferenceSchemeV1(
		base.Digest(),
		changed.CanonicalBytes(),
	); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("tampered canonical error = %v", err)
	}

	legacyArtifactReferenceScheme := []byte(`{"primary":"fpf","anchors":["A.6.0"]}`)
	if _, err := DecodeProjectMemoryReferenceSchemeV1(legacyArtifactReferenceScheme); err == nil ||
		!strings.Contains(err.Error(), "domain") {
		t.Fatalf("legacy ReferenceScheme decode error = %v", err)
	}
}

type runtimeFixture int

const (
	runtimeAbsent runtimeFixture = iota
	runtimePresent
)

type rulePinFixtureInput struct {
	role             SemanticRole
	rule             string
	sourceRevision   string
	sourceCarrier    string
	sourceEdition    string
	sourceDigestByte byte
	runtime          runtimeFixture
	runtimeArtifact  string
	runtimeEdition   string
	runtimeDigest    byte
}

func fixtureRulePin(
	t *testing.T,
	role SemanticRole,
	rule string,
	sourceSuffix string,
	runtime runtimeFixture,
) ExactRulePin {
	t.Helper()
	return rulePinFromFixtureInput(t, rulePinFixtureInput{
		role:             role,
		rule:             rule,
		sourceRevision:   "fpf-revision:" + sourceSuffix,
		sourceCarrier:    "carrier:fpf:" + sourceSuffix,
		sourceEdition:    "edition:1",
		sourceDigestByte: 'a',
		runtime:          runtime,
		runtimeArtifact:  "mechanism:" + rule,
		runtimeEdition:   "build-20260720.1",
		runtimeDigest:    'b',
	})
}

func rulePinFromFixtureInput(
	t *testing.T,
	input rulePinFixtureInput,
) ExactRulePin {
	t.Helper()
	revision := mustSourceRevision(t, input.sourceRevision)
	carrier := mustCarrierRef(t, input.sourceCarrier)
	edition := mustCarrierEdition(t, input.sourceEdition)
	digest := fixtureDigest(t, input.sourceDigestByte)
	source := mustSourceCarrierPin(t, ExactSourceCarrierPinInput{
		SourceRevision: revision,
		Carrier:        carrier,
		Edition:        edition,
		Digest:         digest,
	})
	rule := mustRuleRef(t, input.rule)
	runtime := fixtureRuntimeRequirement(t, input)
	return mustExactRulePin(t, ExactRulePinInput{
		Role:    input.role,
		Rule:    rule,
		Source:  source,
		Runtime: runtime,
	})
}

func fixtureRuntimeRequirement(
	t *testing.T,
	input rulePinFixtureInput,
) RuntimeRequirement {
	t.Helper()
	if input.runtime == runtimeAbsent {
		return NewRuntimeNotRequired()
	}
	artifact := mustCarrierRef(t, input.runtimeArtifact)
	edition := mustCarrierEdition(t, input.runtimeEdition)
	digest := fixtureDigest(t, input.runtimeDigest)
	mechanism := mustRuntimeMechanismPin(t, ExactRuntimeMechanismPinInput{
		Artifact: artifact,
		Edition:  edition,
		Digest:   digest,
	})
	return mustRuntimeRequired(t, mechanism)
}

func schemeWithDesignation(
	t *testing.T,
	designation ExactRulePin,
) ProjectMemoryReferenceSchemeV1 {
	t.Helper()
	interpretation := fixtureRulePin(
		t,
		SemanticRoleInterpretation,
		"interpretation:claims",
		"shared",
		runtimeAbsent,
	)
	measurement := fixtureRulePin(
		t,
		SemanticRoleMeasurement,
		"measurement:na",
		"shared",
		runtimeAbsent,
	)
	evaluation := fixtureRulePin(
		t,
		SemanticRoleEvaluation,
		"evaluation:na",
		"shared",
		runtimeAbsent,
	)
	return fixtureScheme(
		t,
		[]ExactRulePin{designation},
		[]ExactRulePin{interpretation},
		mustMeasurementNotApplicable(t, measurement),
		mustEvaluationNotApplicable(t, evaluation),
	)
}

func fixtureScheme(
	t *testing.T,
	designation []ExactRulePin,
	interpretation []ExactRulePin,
	measurement MeasurementBranch,
	evaluation EvaluationBranch,
) ProjectMemoryReferenceSchemeV1 {
	t.Helper()
	designationRules := mustDesignationRules(t, designation)
	interpretationRules := mustInterpretationRules(t, interpretation)
	scheme, err := NewProjectMemoryReferenceSchemeV1(
		designationRules,
		interpretationRules,
		measurement,
		evaluation,
	)
	if err != nil {
		t.Fatalf("NewProjectMemoryReferenceSchemeV1() error = %v", err)
	}
	return scheme
}

func mustDesignationRules(t *testing.T, pins []ExactRulePin) DesignationRules {
	t.Helper()
	value, err := NewDesignationRules(pins)
	return mustValue(t, value, err)
}

func mustInterpretationRules(t *testing.T, pins []ExactRulePin) InterpretationRules {
	t.Helper()
	value, err := NewInterpretationRules(pins)
	return mustValue(t, value, err)
}

func mustMeasurementRules(t *testing.T, pins []ExactRulePin) MeasurementRules {
	t.Helper()
	value, err := NewMeasurementRules(pins)
	return mustValue(t, value, err)
}

func mustEvaluationRules(t *testing.T, pins []ExactRulePin) EvaluationRules {
	t.Helper()
	value, err := NewEvaluationRules(pins)
	return mustValue(t, value, err)
}

func mustMeasurementNotApplicable(
	t *testing.T,
	pin ExactRulePin,
) MeasurementNotApplicable {
	t.Helper()
	value, err := NewMeasurementNotApplicable(pin)
	return mustValue(t, value, err)
}

func mustEvaluationNotApplicable(
	t *testing.T,
	pin ExactRulePin,
) EvaluationNotApplicable {
	t.Helper()
	value, err := NewEvaluationNotApplicable(pin)
	return mustValue(t, value, err)
}

func fixtureDigest(t *testing.T, fill byte) typedmemory.SHA256Digest {
	t.Helper()
	raw := "sha256:" + strings.Repeat(string(fill), 64)
	value, err := typedmemory.NewSHA256Digest(raw)
	return mustValue(t, value, err)
}

func mustSourceRevision(t *testing.T, raw string) typedmemory.SourceRevision {
	t.Helper()
	value, err := typedmemory.NewSourceRevision(raw)
	return mustValue(t, value, err)
}

func mustCarrierRef(t *testing.T, raw string) typedmemory.CarrierRef {
	t.Helper()
	value, err := typedmemory.NewCarrierRef(raw)
	return mustValue(t, value, err)
}

func mustCarrierEdition(t *testing.T, raw string) typedmemory.CarrierEdition {
	t.Helper()
	value, err := typedmemory.NewCarrierEdition(raw)
	return mustValue(t, value, err)
}

func mustRuleRef(t *testing.T, raw string) typedmemory.RuleRef {
	t.Helper()
	value, err := typedmemory.NewRuleRef(raw)
	return mustValue(t, value, err)
}

func mustSourceCarrierPin(
	t *testing.T,
	input ExactSourceCarrierPinInput,
) ExactSourceCarrierPin {
	t.Helper()
	value, err := NewExactSourceCarrierPin(input)
	return mustValue(t, value, err)
}

func mustRuntimeMechanismPin(
	t *testing.T,
	input ExactRuntimeMechanismPinInput,
) ExactRuntimeMechanismPin {
	t.Helper()
	value, err := NewExactRuntimeMechanismPin(input)
	return mustValue(t, value, err)
}

func mustRuntimeRequired(
	t *testing.T,
	pin ExactRuntimeMechanismPin,
) RuntimeRequired {
	t.Helper()
	value, err := NewRuntimeRequired(pin)
	return mustValue(t, value, err)
}

func mustExactRulePin(t *testing.T, input ExactRulePinInput) ExactRulePin {
	t.Helper()
	value, err := NewExactRulePin(input)
	return mustValue(t, value, err)
}

func mustValue[T any](t *testing.T, value T, err error) T {
	t.Helper()
	if err != nil {
		t.Fatalf("fixture constructor error = %v", err)
	}
	return value
}
