package recordatconcern

import "github.com/m0n0x41d/haft/internal/recordmembershipregistration"

// requireRegisteredMapping keeps historical MemberOf delivery fail-closed.
// Current direct KindClassification targets authenticate their mapping through
// the exact source carrier and selected C/X runtime instead of a historical
// registration artifact.
func requireRegisteredMapping(
	contract Contract,
	runtime ExactRuntimeBasis,
) (Result, bool) {
	if !runtime.sourceMode.IsHistoricalMembership() {
		return nil, true
	}
	policy, err := runtime.registration.EvaluateMappingPolicy(
		contract.manifest,
		contract.adapter,
	)
	if err != nil {
		return underdeterminedFor(
			"record_membership_registration",
			"repair:resolve-selected-record-membership-registration",
		), false
	}
	if policy.Kind() != recordmembershipregistration.MappingAccepted {
		return underdeterminedFor(
			contract.definition.mappingRegistration,
			contract.definition.registrationRepair,
		), false
	}
	return nil, true
}
