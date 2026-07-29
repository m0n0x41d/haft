package typedmemoryevaluation

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// ConservativeEvaluationResultKind is the closed posture currently shared by
// the four role-level ReferenceScheme callable boundaries. There is
// deliberately no Satisfied member: exact registry presence proves that a
// callable is installed, not that the ReferenceScheme's semantic inputs were
// supplied or evaluated successfully.
type ConservativeEvaluationResultKind uint8

const (
	ConservativeEvaluationUnderdetermined ConservativeEvaluationResultKind = iota + 1
)

func (kind ConservativeEvaluationResultKind) String() string {
	switch kind {
	case ConservativeEvaluationUnderdetermined:
		return "underdetermined"
	default:
		return ""
	}
}

// ConservativeEvaluationReason names the missing basis at the temporary
// callable boundary. It is not a semantic verdict about a claim or episteme.
type ConservativeEvaluationReason string

const (
	ConservativeEvaluationSemanticContractUnavailable ConservativeEvaluationReason = "semantic_contract_unavailable"
)

func (reason ConservativeEvaluationReason) String() string {
	return string(reason)
}

// ReferenceDesignationResolutionRequest is a sealed placeholder request. Its
// exact designation inputs are intentionally inexpressible until the semantic
// contract is implemented; invoking the callable can therefore return only an
// explicit Underdetermined result.
type ReferenceDesignationResolutionRequest struct {
	initialized bool
}

func NewReferenceDesignationResolutionRequest() ReferenceDesignationResolutionRequest {
	return ReferenceDesignationResolutionRequest{initialized: true}
}

func (request ReferenceDesignationResolutionRequest) valid() bool {
	return request.initialized
}

type ReferenceDesignationResolutionResult interface {
	Kind() ConservativeEvaluationResultKind
	Reason() ConservativeEvaluationReason
	referenceDesignationResolutionResultVariant()
}

type ReferenceDesignationResolutionUnderdetermined interface {
	ReferenceDesignationResolutionResult
	referenceDesignationResolutionUnderdeterminedVariant()
}

type referenceDesignationResolutionUnderdetermined struct{}

func (referenceDesignationResolutionUnderdetermined) Kind() ConservativeEvaluationResultKind {
	return ConservativeEvaluationUnderdetermined
}

func (referenceDesignationResolutionUnderdetermined) Reason() ConservativeEvaluationReason {
	return ConservativeEvaluationSemanticContractUnavailable
}

func (referenceDesignationResolutionUnderdetermined) referenceDesignationResolutionResultVariant() {
}

func (referenceDesignationResolutionUnderdetermined) referenceDesignationResolutionUnderdeterminedVariant() {
}

// ClaimInterpretationRequest is a sealed placeholder request. It carries no
// claim input, so it cannot express or fabricate an interpretation result.
type ClaimInterpretationRequest struct {
	initialized bool
}

func NewClaimInterpretationRequest() ClaimInterpretationRequest {
	return ClaimInterpretationRequest{initialized: true}
}

func (request ClaimInterpretationRequest) valid() bool {
	return request.initialized
}

type ClaimInterpretationResult interface {
	Kind() ConservativeEvaluationResultKind
	Reason() ConservativeEvaluationReason
	claimInterpretationResultVariant()
}

type ClaimInterpretationUnderdetermined interface {
	ClaimInterpretationResult
	claimInterpretationUnderdeterminedVariant()
}

type claimInterpretationUnderdetermined struct{}

func (claimInterpretationUnderdetermined) Kind() ConservativeEvaluationResultKind {
	return ConservativeEvaluationUnderdetermined
}

func (claimInterpretationUnderdetermined) Reason() ConservativeEvaluationReason {
	return ConservativeEvaluationSemanticContractUnavailable
}

func (claimInterpretationUnderdetermined) claimInterpretationResultVariant() {}

func (claimInterpretationUnderdetermined) claimInterpretationUnderdeterminedVariant() {
}

// ClaimMeasurementRequest is a sealed placeholder request. It carries no
// measurement basis, so invocation cannot be interpreted as performed
// measurement.
type ClaimMeasurementRequest struct {
	initialized bool
}

func NewClaimMeasurementRequest() ClaimMeasurementRequest {
	return ClaimMeasurementRequest{initialized: true}
}

func (request ClaimMeasurementRequest) valid() bool {
	return request.initialized
}

type ClaimMeasurementResult interface {
	Kind() ConservativeEvaluationResultKind
	Reason() ConservativeEvaluationReason
	claimMeasurementResultVariant()
}

type ClaimMeasurementUnderdetermined interface {
	ClaimMeasurementResult
	claimMeasurementUnderdeterminedVariant()
}

type claimMeasurementUnderdetermined struct{}

func (claimMeasurementUnderdetermined) Kind() ConservativeEvaluationResultKind {
	return ConservativeEvaluationUnderdetermined
}

func (claimMeasurementUnderdetermined) Reason() ConservativeEvaluationReason {
	return ConservativeEvaluationSemanticContractUnavailable
}

func (claimMeasurementUnderdetermined) claimMeasurementResultVariant() {}

func (claimMeasurementUnderdetermined) claimMeasurementUnderdeterminedVariant() {}

// ClaimEvaluationRequest is a sealed placeholder request. It carries no
// interpreted or measured claim basis, so invocation cannot satisfy a claim.
type ClaimEvaluationRequest struct {
	initialized bool
}

func NewClaimEvaluationRequest() ClaimEvaluationRequest {
	return ClaimEvaluationRequest{initialized: true}
}

func (request ClaimEvaluationRequest) valid() bool {
	return request.initialized
}

type ClaimEvaluationResult interface {
	Kind() ConservativeEvaluationResultKind
	Reason() ConservativeEvaluationReason
	claimEvaluationResultVariant()
}

type ClaimEvaluationUnderdetermined interface {
	ClaimEvaluationResult
	claimEvaluationUnderdeterminedVariant()
}

type claimEvaluationUnderdetermined struct{}

func (claimEvaluationUnderdetermined) Kind() ConservativeEvaluationResultKind {
	return ConservativeEvaluationUnderdetermined
}

func (claimEvaluationUnderdetermined) Reason() ConservativeEvaluationReason {
	return ConservativeEvaluationSemanticContractUnavailable
}

func (claimEvaluationUnderdetermined) claimEvaluationResultVariant() {}

func (claimEvaluationUnderdetermined) claimEvaluationUnderdeterminedVariant() {}

// Each invocation contract owns a distinct typed registry. These aliases are
// intentionally not collapsed into an untyped ReferenceScheme evaluator map.
type ReferenceDesignationResolutionRegistry = Registry[
	ReferenceDesignationResolutionRequest,
	ReferenceDesignationResolutionResult,
]

type ClaimInterpretationRegistry = Registry[
	ClaimInterpretationRequest,
	ClaimInterpretationResult,
]

type ClaimMeasurementRegistry = Registry[
	ClaimMeasurementRequest,
	ClaimMeasurementResult,
]

type ClaimEvaluationRegistry = Registry[
	ClaimEvaluationRequest,
	ClaimEvaluationResult,
]

func NewReferenceDesignationResolutionRegistry(
	rule typedmemory.RuleRef,
	identity MechanismIdentity,
) (ReferenceDesignationResolutionRegistry, error) {
	return newSingleEvaluatorRegistry(
		rule,
		identity,
		evaluateReferenceDesignationResolution,
	)
}

func NewClaimInterpretationRegistry(
	rule typedmemory.RuleRef,
	identity MechanismIdentity,
) (ClaimInterpretationRegistry, error) {
	return newSingleEvaluatorRegistry(
		rule,
		identity,
		evaluateClaimInterpretation,
	)
}

func NewClaimMeasurementRegistry(
	rule typedmemory.RuleRef,
	identity MechanismIdentity,
) (ClaimMeasurementRegistry, error) {
	return newSingleEvaluatorRegistry(
		rule,
		identity,
		evaluateClaimMeasurement,
	)
}

func NewClaimEvaluationRegistry(
	rule typedmemory.RuleRef,
	identity MechanismIdentity,
) (ClaimEvaluationRegistry, error) {
	return newSingleEvaluatorRegistry(
		rule,
		identity,
		evaluateClaimEvaluation,
	)
}

func evaluateReferenceDesignationResolution(
	request ReferenceDesignationResolutionRequest,
) (ReferenceDesignationResolutionResult, error) {
	if !request.valid() {
		return nil, fmt.Errorf("reference-designation-resolution request is invalid")
	}
	return referenceDesignationResolutionUnderdetermined{}, nil
}

func evaluateClaimInterpretation(
	request ClaimInterpretationRequest,
) (ClaimInterpretationResult, error) {
	if !request.valid() {
		return nil, fmt.Errorf("claim-interpretation request is invalid")
	}
	return claimInterpretationUnderdetermined{}, nil
}

func evaluateClaimMeasurement(
	request ClaimMeasurementRequest,
) (ClaimMeasurementResult, error) {
	if !request.valid() {
		return nil, fmt.Errorf("claim-measurement request is invalid")
	}
	return claimMeasurementUnderdetermined{}, nil
}

func evaluateClaimEvaluation(
	request ClaimEvaluationRequest,
) (ClaimEvaluationResult, error) {
	if !request.valid() {
		return nil, fmt.Errorf("claim-evaluation request is invalid")
	}
	return claimEvaluationUnderdetermined{}, nil
}
