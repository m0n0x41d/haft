package recordmembershipcandidate

import "github.com/m0n0x41d/haft/internal/recordmembershipregistration"

type RegistrationArtifactV1 = recordmembershipregistration.RegistrationArtifactV1

type RegistrationArtifactInputV1 = recordmembershipregistration.RegistrationArtifactInputV1

func SealRegistrationArtifactV1(
	input RegistrationArtifactInputV1,
) (RegistrationArtifactV1, error) {
	return recordmembershipregistration.SealRegistrationArtifactV1(input)
}

func DecodeRegistrationArtifactV1(
	canonical []byte,
) (RegistrationArtifactV1, error) {
	return recordmembershipregistration.DecodeRegistrationArtifactV1(canonical)
}

func VerifyRegistrationArtifactV1(
	expected RegistrationRef,
	canonical []byte,
) (RegistrationArtifactV1, error) {
	return recordmembershipregistration.VerifyRegistrationArtifactV1(
		expected,
		canonical,
	)
}
