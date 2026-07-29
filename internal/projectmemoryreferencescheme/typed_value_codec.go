package projectmemoryreferencescheme

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const projectMemoryReferenceSchemeShapeID = "Haft.Shape.ProjectMemoryReferenceSchemeV1"

// TypedValueCodecV1 adapts the intrinsic ReferenceScheme canonical form to
// typedmemory's closed value algebra. Canonical storage remains the intrinsic
// scheme bytes; the projected TypedValue mirrors the Local-Practice record,
// sum, and unordered-set shape instead of hiding the scheme in an opaque byte
// scalar.
type TypedValueCodecV1 struct {
	shape typedmemory.ValueShapeRef
	codec ProjectMemoryReferenceSchemeV1Codec
}

func NewTypedValueCodecV1(
	shape typedmemory.ValueShapeRef,
) (TypedValueCodecV1, error) {
	rebuilt, err := typedmemory.NewValueShapeRef(shape.ID(), shape.Digest())
	if err != nil || rebuilt != shape {
		return TypedValueCodecV1{}, fmt.Errorf(
			"project-memory ReferenceScheme ValueShapeRef is invalid",
		)
	}
	if shape.ID().String() != projectMemoryReferenceSchemeShapeID {
		return TypedValueCodecV1{}, fmt.Errorf(
			"project-memory ReferenceScheme ValueShape ID %q is not %q",
			shape.ID().String(),
			projectMemoryReferenceSchemeShapeID,
		)
	}
	return TypedValueCodecV1{
		shape: shape,
		codec: NewProjectMemoryReferenceSchemeV1Codec(),
	}, nil
}

func (codec TypedValueCodecV1) Shape() typedmemory.ValueShapeRef {
	return codec.shape
}

func (codec TypedValueCodecV1) Canonicalize(
	expectedShape typedmemory.ValueShapeRef,
	inputBytes []byte,
) typedmemory.CodecCanonicalization {
	if expectedShape != codec.shape {
		return rejectTypedValueCodec(
			typedmemory.DiagnosticValueShapeMismatch,
			"project-memory ReferenceScheme codec was invoked for another ValueShapeRef",
			"typed_value.value_shape_ref",
		)
	}
	scheme, err := codec.codec.Decode(inputBytes)
	if err != nil {
		return rejectTypedValueCodec(
			typedmemory.DiagnosticMalformedValue,
			err.Error(),
			"typed_value.project_memory_reference_scheme",
		)
	}
	return codec.canonicalized(scheme)
}

func (codec TypedValueCodecV1) EncodeInput(
	scheme ProjectMemoryReferenceSchemeV1,
) typedmemory.CodecCanonicalization {
	if err := scheme.Verify(); err != nil {
		return rejectTypedValueCodec(
			typedmemory.DiagnosticMalformedValue,
			err.Error(),
			"typed_value.project_memory_reference_scheme",
		)
	}
	return codec.canonicalized(scheme)
}

func (codec TypedValueCodecV1) canonicalized(
	scheme ProjectMemoryReferenceSchemeV1,
) typedmemory.CodecCanonicalization {
	value, err := projectMemoryReferenceSchemeTypedValue(scheme)
	if err != nil {
		return rejectTypedValueCodec(
			typedmemory.DiagnosticMalformedValue,
			err.Error(),
			"typed_value.project_memory_reference_scheme",
		)
	}
	canonical, err := codec.codec.Encode(scheme)
	if err != nil {
		return rejectTypedValueCodec(
			typedmemory.DiagnosticMalformedValue,
			err.Error(),
			"typed_value.project_memory_reference_scheme",
		)
	}
	result, err := typedmemory.NewCanonicalizedCodecValue(value, canonical)
	if err != nil {
		return rejectTypedValueCodec(
			typedmemory.DiagnosticMalformedValue,
			err.Error(),
			"typed_value.project_memory_reference_scheme",
		)
	}
	return result
}

func projectMemoryReferenceSchemeTypedValue(
	scheme ProjectMemoryReferenceSchemeV1,
) (typedmemory.TypedValue, error) {
	designation, err := exactRulePinSetTypedValue(scheme.Designation().Pins())
	if err != nil {
		return nil, err
	}
	interpretation, err := exactRulePinSetTypedValue(scheme.Interpretation().Pins())
	if err != nil {
		return nil, err
	}
	measurement, err := measurementBranchTypedValue(scheme.Measurement())
	if err != nil {
		return nil, err
	}
	evaluation, err := evaluationBranchTypedValue(scheme.Evaluation())
	if err != nil {
		return nil, err
	}
	return recordTypedValue([]namedTypedValue{
		{name: "designation", value: designation},
		{name: "interpretation", value: interpretation},
		{name: "measurement", value: measurement},
		{name: "evaluation", value: evaluation},
	})
}

func measurementBranchTypedValue(
	branch MeasurementBranch,
) (typedmemory.TypedValue, error) {
	switch value := branch.(type) {
	case MeasurementRules:
		pins, err := exactRulePinSetTypedValue(value.Pins())
		return sumTypedValue("Rules", pins, err)
	case MeasurementNotApplicable:
		pin, err := exactRulePinTypedValue(value.Rule())
		return sumTypedValue("NotApplicable", pin, err)
	default:
		return nil, fmt.Errorf("measurement branch is invalid")
	}
}

func evaluationBranchTypedValue(
	branch EvaluationBranch,
) (typedmemory.TypedValue, error) {
	switch value := branch.(type) {
	case EvaluationRules:
		pins, err := exactRulePinSetTypedValue(value.Pins())
		return sumTypedValue("Rules", pins, err)
	case EvaluationNotApplicable:
		pin, err := exactRulePinTypedValue(value.Rule())
		return sumTypedValue("NotApplicable", pin, err)
	default:
		return nil, fmt.Errorf("evaluation branch is invalid")
	}
}

func exactRulePinSetTypedValue(
	pins []ExactRulePin,
) (typedmemory.TypedValue, error) {
	items := make([]typedmemory.TypedValue, 0, len(pins))
	for _, pin := range pins {
		value, err := exactRulePinTypedValue(pin)
		if err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	return typedmemory.NewUnorderedSetValue(items)
}

func exactRulePinTypedValue(
	pin ExactRulePin,
) (typedmemory.TypedValue, error) {
	source, err := sourceCarrierPinTypedValue(pin.Source())
	if err != nil {
		return nil, err
	}
	common := []namedTypedValue{
		{name: "semantic_role", value: typedmemory.NewTextValue(string(pin.Role()))},
		{name: "rule_ref", value: typedmemory.NewTextValue(pin.Rule().String())},
		{name: "source", value: source},
	}
	switch runtime := pin.Runtime().(type) {
	case RuntimeNotRequired:
		value, recordErr := recordTypedValue(common)
		return sumTypedValue("RuntimeNotRequired", value, recordErr)
	case RuntimeRequired:
		mechanism, mechanismErr := runtimeMechanismPinTypedValue(runtime.Mechanism())
		if mechanismErr != nil {
			return nil, mechanismErr
		}
		fields := append(
			append([]namedTypedValue(nil), common...),
			namedTypedValue{name: "runtime_mechanism", value: mechanism},
		)
		value, recordErr := recordTypedValue(fields)
		return sumTypedValue("RuntimeRequired", value, recordErr)
	default:
		return nil, fmt.Errorf("exact rule runtime branch is invalid")
	}
}

func sourceCarrierPinTypedValue(
	pin ExactSourceCarrierPin,
) (typedmemory.TypedValue, error) {
	return recordTypedValue([]namedTypedValue{
		{name: "revision", value: typedmemory.NewTextValue(pin.SourceRevision().String())},
		{name: "carrier", value: typedmemory.NewTextValue(pin.Carrier().String())},
		{name: "edition", value: typedmemory.NewTextValue(pin.Edition().String())},
		{name: "digest", value: typedmemory.NewTextValue(pin.Digest().String())},
	})
}

func runtimeMechanismPinTypedValue(
	pin ExactRuntimeMechanismPin,
) (typedmemory.TypedValue, error) {
	return recordTypedValue([]namedTypedValue{
		{name: "artifact", value: typedmemory.NewTextValue(pin.Artifact().String())},
		{name: "edition", value: typedmemory.NewTextValue(pin.Edition().String())},
		{name: "digest", value: typedmemory.NewTextValue(pin.Digest().String())},
	})
}

type namedTypedValue struct {
	name  string
	value typedmemory.TypedValue
}

func recordTypedValue(
	fields []namedTypedValue,
) (typedmemory.TypedValue, error) {
	result := make([]typedmemory.RecordFieldValue, 0, len(fields))
	for _, field := range fields {
		name, err := typedmemory.NewValueMemberName(field.name)
		if err != nil {
			return nil, err
		}
		value, err := typedmemory.NewRecordFieldValue(name, field.value)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return typedmemory.NewRecordValue(result)
}

func sumTypedValue(
	variant string,
	value typedmemory.TypedValue,
	prior error,
) (typedmemory.TypedValue, error) {
	if prior != nil {
		return nil, prior
	}
	name, err := typedmemory.NewValueMemberName(variant)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewSumValue(name, value)
}

func rejectTypedValueCodec(
	code typedmemory.DiagnosticCode,
	message string,
	rawPath string,
) typedmemory.CodecCanonicalization {
	path, _ := typedmemory.NewDiagnosticPath(rawPath)
	issue, _ := typedmemory.NewCodecIssue(code, message, path)
	result, _ := typedmemory.NewRejectedCodecValue([]typedmemory.CodecIssue{issue})
	return result
}

var _ typedmemory.CodecImplementation = TypedValueCodecV1{}
