package typedmemorycandidatecodec

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func rejectShape(
	codecName string,
	expected typedmemory.ValueShapeRef,
	actual typedmemory.ValueShapeRef,
) typedmemory.CodecCanonicalization {
	message := codecName + " was invoked for a different ValueShapeRef"
	return rejectWithWitness(
		typedmemory.DiagnosticValueShapeMismatch,
		message,
		"typed_value.value_shape_ref",
		expected.String(),
		actual.String(),
	)
}

func rejectMalformed(
	codecName string,
	path string,
	err error,
) typedmemory.CodecCanonicalization {
	message := codecName + " rejected malformed candidate bytes"
	return rejectWithWitness(
		typedmemory.DiagnosticMalformedValue,
		message,
		path,
		"canonical "+codecName+" value",
		err.Error(),
	)
}

func rejectWithWitness(
	code typedmemory.DiagnosticCode,
	message string,
	pathRaw string,
	expectedRaw string,
	actualRaw string,
) typedmemory.CodecCanonicalization {
	path, pathErr := typedmemory.NewDiagnosticPath(pathRaw)
	expected, expectedErr := typedmemory.NewDiagnosticStateDatum(expectedRaw)
	actual, actualErr := typedmemory.NewDiagnosticTextDatum(actualRaw)
	witness, witnessErr := typedmemory.NewExpectedActualWitness(expected, actual)
	pointer, pointerErr := typedmemory.NewRepairPointer("change-candidate-at:" + pathRaw)
	repair, repairErr := typedmemory.NewRepairCandidate(
		typedmemory.RepairChangeInput,
		pointer,
		expected,
		typedmemory.HumanChoiceNotClaimed,
	)
	issue, issueErr := typedmemory.NewCodecIssueWithDetails(
		code,
		message,
		path,
		witness,
		[]typedmemory.RepairCandidate{repair},
	)
	result, resultErr := typedmemory.NewRejectedCodecValue([]typedmemory.CodecIssue{issue})
	errors := []error{
		pathErr,
		expectedErr,
		actualErr,
		witnessErr,
		pointerErr,
		repairErr,
		issueErr,
		resultErr,
	}
	for _, err := range errors {
		if err != nil {
			return nil
		}
	}
	return result
}

func acceptCanonical(
	codecName string,
	value typedmemory.TypedValue,
	canonicalBytes []byte,
) typedmemory.CodecCanonicalization {
	result, err := typedmemory.NewCanonicalizedCodecValue(value, canonicalBytes)
	if err != nil {
		return rejectMalformed(codecName, "typed_value.codec_result", err)
	}
	return result
}

func rejectionError(result typedmemory.CodecCanonicalization) error {
	rejected, ok := result.(typedmemory.RejectedCodecValue)
	if !ok {
		return fmt.Errorf("codec returned %T", result)
	}
	issues := rejected.Issues()
	if len(issues) == 0 {
		return fmt.Errorf("codec rejected without an issue")
	}
	return fmt.Errorf("%s: %s", issues[0].Code(), issues[0].Message())
}
