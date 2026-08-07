package typedmemoryevaluation

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectmemoryconstitution"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// EpistemeConstitutionEvaluationRequest carries the exact constitution
// participants and explicit role outcomes consumed by the pure constitution
// core. Registry availability is intentionally absent: finding this callable
// cannot fabricate any role outcome or a positive constitution result.
type EpistemeConstitutionEvaluationRequest struct {
	input       projectmemoryconstitution.EvaluationInput
	initialized bool
}

func NewEpistemeConstitutionEvaluationRequest(
	input projectmemoryconstitution.EvaluationInput,
) EpistemeConstitutionEvaluationRequest {
	return EpistemeConstitutionEvaluationRequest{
		input:       input,
		initialized: true,
	}
}

func (request EpistemeConstitutionEvaluationRequest) valid() bool {
	return request.initialized
}

// EpistemeConstitutionEvaluationResult reuses the constitution core's one
// canonical Invalid | Underdetermined | Satisfied algebra. The registry adds
// lookup and mechanism identity, not a second semantic result representation.
type EpistemeConstitutionEvaluationResult = projectmemoryconstitution.Result

type EpistemeConstitutionEvaluationRegistry = Registry[
	EpistemeConstitutionEvaluationRequest,
	EpistemeConstitutionEvaluationResult,
]

func NewEpistemeConstitutionEvaluationRegistry(
	rule typedmemory.RuleRef,
	identity MechanismIdentity,
) (EpistemeConstitutionEvaluationRegistry, error) {
	return newSingleEvaluatorRegistry(
		rule,
		identity,
		evaluateEpistemeConstitution,
	)
}

func evaluateEpistemeConstitution(
	request EpistemeConstitutionEvaluationRequest,
) (EpistemeConstitutionEvaluationResult, error) {
	if !request.valid() {
		return nil, fmt.Errorf("episteme-constitution-evaluation request is invalid")
	}
	result := projectmemoryconstitution.Evaluate(request.input)
	return result, nil
}
