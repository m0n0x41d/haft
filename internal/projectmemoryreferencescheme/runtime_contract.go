package projectmemoryreferencescheme

import (
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// RuntimeContract is the closed calling convention used by a project-memory
// ReferenceScheme runtime mechanism. It describes an invocation shape only;
// it neither identifies an implementation nor attests execution.
type RuntimeContract uint8

const (
	RuntimeContractReferenceDesignationResolution RuntimeContract = iota + 1
	RuntimeContractClaimInterpretation
	RuntimeContractClaimMeasurement
	RuntimeContractClaimEvaluation
	RuntimeContractEpistemeConstitutionEvaluation
)

func (contract RuntimeContract) String() string {
	switch contract {
	case RuntimeContractReferenceDesignationResolution:
		return "reference_designation_resolution"
	case RuntimeContractClaimInterpretation:
		return "claim_interpretation"
	case RuntimeContractClaimMeasurement:
		return "claim_measurement"
	case RuntimeContractClaimEvaluation:
		return "claim_evaluation"
	case RuntimeContractEpistemeConstitutionEvaluation:
		return "episteme_constitution_evaluation"
	default:
		return ""
	}
}

func ParseRuntimeContract(raw string) (RuntimeContract, error) {
	contracts := runtimeContracts()
	index := slices.IndexFunc(contracts, func(contract RuntimeContract) bool {
		return contract.String() == raw
	})
	if index < 0 {
		return 0, fmt.Errorf("project-memory reference-scheme runtime contract %q is not defined", raw)
	}
	return contracts[index], nil
}

// RoleRuntimeContract is one member of the total role-to-contract mapping.
// Its fields are private so a role cannot be paired with another role's
// calling convention.
type RoleRuntimeContract struct {
	role     SemanticRole
	contract RuntimeContract
}

func (mapping RoleRuntimeContract) Role() SemanticRole {
	return mapping.role
}

func (mapping RoleRuntimeContract) Contract() RuntimeContract {
	return mapping.contract
}

// RuntimeContractForRole is total over the closed SemanticRole set.
func RuntimeContractForRole(role SemanticRole) (RuntimeContract, error) {
	mappings := RoleRuntimeContracts()
	index := slices.IndexFunc(mappings, func(mapping RoleRuntimeContract) bool {
		return mapping.role == role
	})
	if index < 0 {
		return 0, fmt.Errorf("project-memory reference-scheme semantic role %q is not defined", role)
	}
	return mappings[index].contract, nil
}

// RoleRuntimeContracts returns the complete mapping in semantic role order.
func RoleRuntimeContracts() []RoleRuntimeContract {
	return []RoleRuntimeContract{
		{
			role:     SemanticRoleDesignation,
			contract: RuntimeContractReferenceDesignationResolution,
		},
		{
			role:     SemanticRoleInterpretation,
			contract: RuntimeContractClaimInterpretation,
		},
		{
			role:     SemanticRoleMeasurement,
			contract: RuntimeContractClaimMeasurement,
		},
		{
			role:     SemanticRoleEvaluation,
			contract: RuntimeContractClaimEvaluation,
		},
	}
}

func runtimeContracts() []RuntimeContract {
	return []RuntimeContract{
		RuntimeContractReferenceDesignationResolution,
		RuntimeContractClaimInterpretation,
		RuntimeContractClaimMeasurement,
		RuntimeContractClaimEvaluation,
		RuntimeContractEpistemeConstitutionEvaluation,
	}
}

// EpistemeConstitutionEvaluationInputContracts declares the role-outcome
// families consumed by the separate aggregate constitution evaluator. It does
// not define an evaluator result algebra or claim that any mechanism ran.
func EpistemeConstitutionEvaluationInputContracts() []RoleRuntimeContract {
	return RoleRuntimeContracts()
}

// EpistemeConstitutionEvaluationRuntimeContract returns the aggregate
// contract. It is deliberately distinct from the claim-evaluation contract
// selected for EvaluationRules.
func EpistemeConstitutionEvaluationRuntimeContract() RuntimeContract {
	return RuntimeContractEpistemeConstitutionEvaluation
}

// CallableRuntimeRequirement is one exact rule pin whose RuntimeRequired
// branch makes a call necessary. The contract is determined only by the pin's
// semantic role; the exact mechanism remains the pin supplied by the scheme.
type CallableRuntimeRequirement struct {
	role      SemanticRole
	contract  RuntimeContract
	pin       ExactRulePin
	mechanism ExactRuntimeMechanismPin
}

func (requirement CallableRuntimeRequirement) Role() SemanticRole {
	return requirement.role
}

func (requirement CallableRuntimeRequirement) Contract() RuntimeContract {
	return requirement.contract
}

func (requirement CallableRuntimeRequirement) RuleRef() typedmemory.RuleRef {
	return requirement.pin.Rule()
}

func (requirement CallableRuntimeRequirement) RulePin() ExactRulePin {
	return requirement.pin
}

func (requirement CallableRuntimeRequirement) Mechanism() ExactRuntimeMechanismPin {
	return requirement.mechanism
}

func (requirement CallableRuntimeRequirement) valid() bool {
	if !requirement.pin.valid() || !requirement.mechanism.valid() {
		return false
	}
	if requirement.role != requirement.pin.Role() {
		return false
	}
	contract, err := RuntimeContractForRole(requirement.role)
	if err != nil || contract != requirement.contract {
		return false
	}
	runtime, ok := requirement.pin.Runtime().(RuntimeRequired)
	return ok && runtime.Mechanism() == requirement.mechanism
}

// DeriveCallableRuntimeRequirements verifies the sealed scheme and projects
// only its executable rule pins. A Rules branch can therefore yield zero or
// more requirements. A NotApplicable branch always yields none, even though
// its exact rule pin remains part of the unchanged intrinsic V1 identity.
func DeriveCallableRuntimeRequirements(
	scheme ProjectMemoryReferenceSchemeV1,
) ([]CallableRuntimeRequirement, error) {
	if err := scheme.Verify(); err != nil {
		return nil, fmt.Errorf("derive reference-scheme runtime requirements: %w", err)
	}
	designation, err := deriveCallableRequirementsForPins(
		SemanticRoleDesignation,
		scheme.Designation().Pins(),
	)
	if err != nil {
		return nil, err
	}
	interpretation, err := deriveCallableRequirementsForPins(
		SemanticRoleInterpretation,
		scheme.Interpretation().Pins(),
	)
	if err != nil {
		return nil, err
	}
	measurement, err := deriveMeasurementCallableRequirements(
		scheme.Measurement(),
	)
	if err != nil {
		return nil, err
	}
	evaluation, err := deriveEvaluationCallableRequirements(
		scheme.Evaluation(),
	)
	if err != nil {
		return nil, err
	}
	combined := slices.Concat(
		designation,
		interpretation,
		measurement,
		evaluation,
	)
	return slices.Clone(combined), nil
}

func deriveMeasurementCallableRequirements(
	branch MeasurementBranch,
) ([]CallableRuntimeRequirement, error) {
	switch value := branch.(type) {
	case MeasurementRules:
		return deriveCallableRequirementsForPins(
			SemanticRoleMeasurement,
			value.Pins(),
		)
	case MeasurementNotApplicable:
		return []CallableRuntimeRequirement{}, nil
	default:
		return nil, fmt.Errorf("derive callable requirements: measurement branch is invalid")
	}
}

func deriveEvaluationCallableRequirements(
	branch EvaluationBranch,
) ([]CallableRuntimeRequirement, error) {
	switch value := branch.(type) {
	case EvaluationRules:
		return deriveCallableRequirementsForPins(
			SemanticRoleEvaluation,
			value.Pins(),
		)
	case EvaluationNotApplicable:
		return []CallableRuntimeRequirement{}, nil
	default:
		return nil, fmt.Errorf("derive callable requirements: evaluation branch is invalid")
	}
}

func deriveCallableRequirementsForPins(
	role SemanticRole,
	pins []ExactRulePin,
) ([]CallableRuntimeRequirement, error) {
	requirements := make([]CallableRuntimeRequirement, 0, len(pins))
	for index, pin := range pins {
		requirement, callable, err := deriveCallableRequirement(role, pin)
		if err != nil {
			return nil, fmt.Errorf(
				"derive callable requirement for %q pin %d: %w",
				role,
				index,
				err,
			)
		}
		if callable {
			requirements = append(requirements, requirement)
		}
	}
	return requirements, nil
}

func deriveCallableRequirement(
	role SemanticRole,
	pin ExactRulePin,
) (CallableRuntimeRequirement, bool, error) {
	if err := validateRuleForRole("callable runtime rule", role, pin); err != nil {
		return CallableRuntimeRequirement{}, false, err
	}
	contract, err := RuntimeContractForRole(role)
	if err != nil {
		return CallableRuntimeRequirement{}, false, err
	}
	switch runtime := pin.Runtime().(type) {
	case RuntimeNotRequired:
		return CallableRuntimeRequirement{}, false, nil
	case RuntimeRequired:
		requirement := CallableRuntimeRequirement{
			role:      role,
			contract:  contract,
			pin:       pin,
			mechanism: runtime.Mechanism(),
		}
		if !requirement.valid() {
			return CallableRuntimeRequirement{}, false, fmt.Errorf(
				"callable runtime requirement is invalid",
			)
		}
		return requirement, true, nil
	default:
		return CallableRuntimeRequirement{}, false, fmt.Errorf(
			"runtime requirement branch is invalid",
		)
	}
}
