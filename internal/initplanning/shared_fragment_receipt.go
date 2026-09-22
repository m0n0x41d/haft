package initplanning

import "fmt"

// WithSharedReceiptPredecessors opts exact generated predecessors into receipt
// reconciliation when another host has already published the desired fragment.
// Both the recorded predecessor and the observed desired bytes must belong to
// this registry. It never permits replacement of differing observed bytes.
func (registry ManagedFragmentLegacyRegistry) WithSharedReceiptPredecessors(
	predecessors []ManagedFragmentRecord,
) (ManagedFragmentLegacyRegistry, error) {
	validated, err := canonicalManagedFragmentLegacyRecords(predecessors)
	if err != nil {
		return ManagedFragmentLegacyRegistry{}, err
	}
	if err := validateSharedReceiptPredecessors(registry.records, validated); err != nil {
		return ManagedFragmentLegacyRegistry{}, err
	}
	next := cloneManagedFragmentLegacyRegistry(registry)
	next.sharedReceiptPredecessors = validated
	return next, nil
}

func validateSharedReceiptPredecessors(
	records []ManagedFragmentRecord,
	predecessors []ManagedFragmentRecord,
) error {
	byKey := managedFragmentRecordSetsByKey(records)
	for _, predecessor := range predecessors {
		key := managedFragmentCoordinateKey(predecessor.coordinate)
		matches := managedFragmentRecordsContainDigest(byKey[key], predecessor.digest)
		if !matches {
			return fmt.Errorf("shared receipt predecessor is absent from the exact legacy registry")
		}
	}
	return nil
}

func reconcileSharedManagedFragmentReceipt(
	state ManagedFragmentCurrentness,
	legacy []ManagedFragmentRecord,
	predecessors []ManagedFragmentRecord,
	basis OwnershipBasis,
) ManagedFragmentCurrentness {
	if state.kind != ManagedFragmentLocallyModifiedOwned || !state.hasDesired {
		return state
	}
	if state.observedDigest != state.desiredDigest {
		return state
	}
	if !managedFragmentRecordsContainDigest(predecessors, state.manifestDigest) {
		return state
	}
	if !managedFragmentRecordsContainDigest(legacy, state.observedDigest) {
		return state
	}
	// Keep the original manifest digest and carrier observation. Publication
	// still compares the real predecessor receipt and complete carrier bytes;
	// only this host's receipt is adopted, without rewriting the shared carrier.
	next := state
	next.kind = ManagedFragmentKnownLegacyExact
	next.basis = basis
	return next
}
