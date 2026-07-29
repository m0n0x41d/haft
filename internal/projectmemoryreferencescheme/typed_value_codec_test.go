package projectmemoryreferencescheme

import (
	"bytes"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestTypedValueCodecV1ProjectsExactSchemeShapeAndPreservesCanonicalBytes(
	t *testing.T,
) {
	t.Parallel()
	designation := fixtureRulePin(
		t,
		SemanticRoleDesignation,
		"designation:resolve",
		"typed-codec-designation",
		runtimePresent,
	)
	interpretation := fixtureRulePin(
		t,
		SemanticRoleInterpretation,
		"interpretation:claims",
		"typed-codec-interpretation",
		runtimeAbsent,
	)
	measurement := fixtureRulePin(
		t,
		SemanticRoleMeasurement,
		"measurement:not-applicable",
		"typed-codec-measurement",
		runtimeAbsent,
	)
	evaluation := fixtureRulePin(
		t,
		SemanticRoleEvaluation,
		"evaluation:claims",
		"typed-codec-evaluation",
		runtimePresent,
	)
	scheme := fixtureScheme(
		t,
		[]ExactRulePin{designation},
		[]ExactRulePin{interpretation},
		mustMeasurementNotApplicable(t, measurement),
		mustEvaluationRules(t, []ExactRulePin{evaluation}),
	)
	codec := mustTypedValueCodec(t, projectMemoryReferenceSchemeShapeID, 'a')

	encoded := codec.EncodeInput(scheme)
	canonicalized, exact := encoded.(typedmemory.CanonicalizedCodecValue)
	if !exact {
		t.Fatalf("EncodeInput() = %T, want CanonicalizedCodecValue", encoded)
	}
	if !bytes.Equal(canonicalized.CanonicalBytes(), scheme.CanonicalBytes()) {
		t.Fatal("typed codec changed intrinsic scheme canonical bytes")
	}
	record, exact := canonicalized.Value().(typedmemory.RecordTypedValue)
	if !exact {
		t.Fatalf("typed value = %T, want RecordTypedValue", canonicalized.Value())
	}
	fields := record.Fields()
	wantNames := []string{"designation", "interpretation", "measurement", "evaluation"}
	if len(fields) != len(wantNames) {
		t.Fatalf("root field count = %d, want %d", len(fields), len(wantNames))
	}
	fieldsByName := make(map[string]typedmemory.TypedValue, len(fields))
	for _, field := range fields {
		fieldsByName[field.Name().String()] = field.Value()
	}
	for _, want := range wantNames {
		if _, present := fieldsByName[want]; !present {
			t.Fatalf("root record has no %q field", want)
		}
	}
	designationSet, exact := fieldsByName["designation"].(typedmemory.UnorderedSetTypedValue)
	if !exact {
		t.Fatalf("designation = %T, want unordered set", fieldsByName["designation"])
	}
	if len(designationSet.Items()) != 1 {
		t.Fatalf("designation length = %d, want 1", len(designationSet.Items()))
	}
	designationPin, exact := designationSet.Items()[0].(typedmemory.SumTypedValue)
	if !exact || designationPin.Variant().String() != "RuntimeRequired" {
		t.Fatalf("designation pin = %T/%q, want RuntimeRequired sum", designationSet.Items()[0], designationPin.Variant())
	}
	measurementBranch, exact := fieldsByName["measurement"].(typedmemory.SumTypedValue)
	if !exact || measurementBranch.Variant().String() != "NotApplicable" {
		t.Fatalf("measurement = %T/%q, want NotApplicable sum", fieldsByName["measurement"], measurementBranch.Variant())
	}
	evaluationBranch, exact := fieldsByName["evaluation"].(typedmemory.SumTypedValue)
	if !exact || evaluationBranch.Variant().String() != "Rules" {
		t.Fatalf("evaluation = %T/%q, want Rules sum", fieldsByName["evaluation"], evaluationBranch.Variant())
	}

	roundTrip := codec.Canonicalize(codec.Shape(), scheme.CanonicalBytes())
	roundTripValue, exact := roundTrip.(typedmemory.CanonicalizedCodecValue)
	if !exact || !bytes.Equal(roundTripValue.CanonicalBytes(), scheme.CanonicalBytes()) {
		t.Fatalf("Canonicalize() = %T, want byte-identical canonical value", roundTrip)
	}
	copyBytes := roundTripValue.CanonicalBytes()
	copyBytes[0] ^= 0xff
	if bytes.Equal(copyBytes, roundTripValue.CanonicalBytes()) {
		t.Fatal("CanonicalizedCodecValue leaked mutable canonical storage")
	}
}

func TestTypedValueCodecV1RejectsWrongShapeAndLegacyCarrierMetadata(t *testing.T) {
	t.Parallel()
	codec := mustTypedValueCodec(t, projectMemoryReferenceSchemeShapeID, 'b')
	wrong := mustValueShapeRef(t, "Haft.Shape.Other", 'c')

	shapeResult := codec.Canonicalize(wrong, []byte("not-read"))
	if _, exact := shapeResult.(typedmemory.RejectedCodecValue); !exact {
		t.Fatalf("wrong-shape result = %T, want RejectedCodecValue", shapeResult)
	}
	legacy := []byte(`{"Primary":"fpf","Anchors":["A.6.0"]}`)
	legacyResult := codec.Canonicalize(codec.Shape(), legacy)
	if _, exact := legacyResult.(typedmemory.RejectedCodecValue); !exact {
		t.Fatalf("legacy result = %T, want RejectedCodecValue", legacyResult)
	}
	if _, err := NewTypedValueCodecV1(wrong); err == nil {
		t.Fatal("NewTypedValueCodecV1 accepted another shape ID")
	}
}

func mustTypedValueCodec(
	t *testing.T,
	shapeID string,
	digestFill byte,
) TypedValueCodecV1 {
	t.Helper()
	shape := mustValueShapeRef(t, shapeID, digestFill)
	codec, err := NewTypedValueCodecV1(shape)
	if err != nil {
		t.Fatalf("NewTypedValueCodecV1() error = %v", err)
	}
	return codec
}

func mustValueShapeRef(
	t *testing.T,
	shapeID string,
	digestFill byte,
) typedmemory.ValueShapeRef {
	t.Helper()
	id, err := typedmemory.NewShapeID(shapeID)
	if err != nil {
		t.Fatalf("NewShapeID() error = %v", err)
	}
	shape, err := typedmemory.NewValueShapeRef(id, fixtureDigest(t, digestFill))
	if err != nil {
		t.Fatalf("NewValueShapeRef() error = %v", err)
	}
	return shape
}
