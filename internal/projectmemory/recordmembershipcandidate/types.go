package recordmembershipcandidate

import "github.com/m0n0x41d/haft/internal/recordmembershipregistration"

const (
	RegistrationSchemaV1     = recordmembershipregistration.RegistrationSchemaV1
	MaximumRegistrationBytes = recordmembershipregistration.MaximumRegistrationBytes
	MaximumAcceptedMappings  = recordmembershipregistration.MaximumAcceptedMappings
)

type RegistrationRef = recordmembershipregistration.RegistrationRef

func ParseRegistrationRef(raw string) (RegistrationRef, error) {
	return recordmembershipregistration.ParseRegistrationRef(raw)
}

type MechanismRole = recordmembershipregistration.MechanismRole

const (
	EvaluatorMechanism              = recordmembershipregistration.EvaluatorMechanism
	SourceDeliveryBoundaryMechanism = recordmembershipregistration.SourceDeliveryBoundaryMechanism
)

type MechanismCoordinate = recordmembershipregistration.MechanismCoordinate

type MechanismCoordinateInput = recordmembershipregistration.MechanismCoordinateInput

func NewMechanismCoordinate(
	input MechanismCoordinateInput,
) (MechanismCoordinate, error) {
	return recordmembershipregistration.NewMechanismCoordinate(input)
}

type AcceptedMapping = recordmembershipregistration.AcceptedMapping

type AcceptedMappingInput = recordmembershipregistration.AcceptedMappingInput

func NewAcceptedMapping(input AcceptedMappingInput) (AcceptedMapping, error) {
	return recordmembershipregistration.NewAcceptedMapping(input)
}

type MappingPolicyDecisionKind = recordmembershipregistration.MappingPolicyDecisionKind

const (
	MappingAccepted            = recordmembershipregistration.MappingAccepted
	MappingManifestNotAccepted = recordmembershipregistration.MappingManifestNotAccepted
	MappingAdapterMismatch     = recordmembershipregistration.MappingAdapterMismatch
)

type MappingPolicyDecision = recordmembershipregistration.MappingPolicyDecision

type AcceptedMappingConflictKind = recordmembershipregistration.AcceptedMappingConflictKind

const (
	DuplicateAcceptedMapping      = recordmembershipregistration.DuplicateAcceptedMapping
	ConflictingAdapterForManifest = recordmembershipregistration.ConflictingAdapterForManifest
)

type AcceptedMappingConflict = recordmembershipregistration.AcceptedMappingConflict
