package recordcarrier

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// NewTrustedRecordMembershipSourceDeliveryV1 is the narrow production bridge
// from a previously matched registration policy to the evaluator-only trusted
// source delivery variant. It does not evaluate membership and does not trust
// caller bytes directly: the source is decoded, correlated to the exact
// observable input, and checked against the accepted mapping policy first.
func NewTrustedRecordMembershipSourceDeliveryV1(
	policy recordmembershipregistration.RegistrationArtifactV1,
	expected typedmemory.MemberOfObservableInput,
	canonical []byte,
) (RecordMembershipSourceDeliveryV1, error) {
	if err := policy.Verify(); err != nil {
		return nil, fmt.Errorf("trusted record-membership delivery policy: %w", err)
	}
	source, err := VerifyRecordMembershipSourceV1(expected, canonical)
	if err != nil {
		return nil, fmt.Errorf("trusted record-membership source: %w", err)
	}
	decision, err := policy.EvaluateMappingPolicy(
		source.Binding().MappingManifestRef(),
		source.Binding().AdapterVersion(),
	)
	if err != nil {
		return nil, fmt.Errorf("trusted record-membership mapping policy: %w", err)
	}
	if decision.Kind() != recordmembershipregistration.MappingAccepted {
		return nil, fmt.Errorf(
			"trusted record-membership mapping policy returned %q",
			decision.Kind().String(),
		)
	}
	return newTrustedRecordMembershipSourceDeliveryV1(expected, canonical)
}
