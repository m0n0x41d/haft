package projecttypeenv

import "github.com/m0n0x41d/haft/internal/recordmembershipregistration"

// Registration policy is owned below projecttypeenv. These aliases expose the
// exact strong artifact to X without importing projectmemory or duplicating its
// canonical grammar.
type RegistrationPolicyRef = recordmembershipregistration.RegistrationRef

type RegistrationPolicyMechanismRole = recordmembershipregistration.MechanismRole

const (
	RegistrationPolicyRoleEvaluator      = recordmembershipregistration.EvaluatorMechanism
	RegistrationPolicyRoleSourceDelivery = recordmembershipregistration.SourceDeliveryBoundaryMechanism
)

type RegistrationPolicyMechanismCoordinate = recordmembershipregistration.MechanismCoordinate

type RegistrationPolicyAcceptedMapping = recordmembershipregistration.AcceptedMapping

type RegistrationPolicyArtifact = recordmembershipregistration.RegistrationArtifactV1

func ParseRegistrationPolicyRef(raw string) (RegistrationPolicyRef, error) {
	return recordmembershipregistration.ParseRegistrationRef(raw)
}

func DecodeRegistrationPolicyArtifact(
	canonical []byte,
) (RegistrationPolicyArtifact, error) {
	return recordmembershipregistration.DecodeRegistrationArtifactV1(canonical)
}

func VerifyRegistrationPolicyArtifact(
	expected RegistrationPolicyRef,
	canonical []byte,
) (RegistrationPolicyArtifact, error) {
	return recordmembershipregistration.VerifyRegistrationArtifactV1(
		expected,
		canonical,
	)
}
