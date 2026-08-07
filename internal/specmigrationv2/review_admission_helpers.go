package specmigrationv2

import (
	"bytes"
	"fmt"
	"sort"
)

func validateMigrationReviewPreparationBasis(
	carrier FinalCandidatePacketCarrier,
	audit PacketPartitionAudit,
) (ApplyProjectRoot, error) {
	if err := validatePacketCarrierForReviewAdmission(carrier); err != nil {
		return ApplyProjectRoot{}, err
	}
	root, err := reviewCarrierProjectRoot(carrier)
	if err != nil {
		return ApplyProjectRoot{}, err
	}
	if len(audit.CanonicalBytes()) == 0 {
		return ApplyProjectRoot{}, fmt.Errorf("migration-review admission requires a canonical partition audit")
	}
	if audit.Status() != PacketPartitionAuditVerified {
		return ApplyProjectRoot{}, fmt.Errorf("migration-review admission requires a verified partition audit")
	}
	if !audit.PacketDigest().Equal(carrier.PacketDigest()) ||
		!audit.PacketCarrierDigest().Equal(carrier.CarrierDigest()) {
		return ApplyProjectRoot{}, fmt.Errorf("partition audit does not bind the exact final-candidate packet carrier")
	}
	observedAuditDigest := PacketPartitionAuditDigest{
		value: DigestBytes(audit.CanonicalBytes()),
	}
	if !observedAuditDigest.Equal(audit.Digest()) {
		return ApplyProjectRoot{}, fmt.Errorf("partition audit digest does not bind its canonical record")
	}
	return root, nil
}

func validatePacketCarrierForReviewAdmission(
	carrier FinalCandidatePacketCarrier,
) error {
	canonical := carrier.CanonicalBytes()
	decoded, err := DecodePacketCarrier(canonical)
	if err != nil {
		return fmt.Errorf("final-candidate packet carrier is invalid: %w", err)
	}
	if !decoded.PacketDigest().Equal(carrier.PacketDigest()) ||
		!decoded.CarrierDigest().Equal(carrier.CarrierDigest()) ||
		!bytes.Equal(decoded.CanonicalBytes(), canonical) {
		return fmt.Errorf("final-candidate packet carrier does not match its canonical digests")
	}
	return validateReviewBasisAgainstPacket(decoded)
}

func validateReviewBasisAgainstPacket(
	carrier FinalCandidatePacketCarrier,
) error {
	packet := carrier.Packet()
	software, found := reviewCarrierByRole(
		carrier.ReviewBasis().CarrierDigests(),
		ReviewSoftwareSystemCarrier,
	)
	if !found {
		return fmt.Errorf("final-candidate review basis has no software-system carrier")
	}
	if software.digest.String() != packet.Target().Digest().String() {
		return fmt.Errorf("software-system review carrier digest does not match the packet target digest")
	}
	return nil
}

func reviewCarrierProjectRoot(
	carrier FinalCandidatePacketCarrier,
) (ApplyProjectRoot, error) {
	provenance := carrier.Packet().Source().Provenance()
	if !provenance.valid() {
		return ApplyProjectRoot{}, fmt.Errorf("final-candidate packet source provenance is invalid")
	}
	rootRef := provenance.origin.ProjectRoot()
	root, err := NewApplyProjectRoot(rootRef.String())
	if err != nil {
		return ApplyProjectRoot{}, fmt.Errorf("final-candidate packet project root is invalid: %w", err)
	}
	return root, nil
}

func reviewCarrierByRole(
	set ReviewCarrierDigestSet,
	role ReviewCarrierRole,
) (ReviewCarrierDigest, bool) {
	values := set.Values()
	for _, value := range values {
		if value.role == role {
			return value, true
		}
	}
	return ReviewCarrierDigest{}, false
}

func canonicalReviewCarrierDTOs(
	set ReviewCarrierDigestSet,
) []reviewCarrierDigestJSONV1 {
	values := set.Values()
	sort.Slice(values, func(left int, right int) bool {
		return values[left].role < values[right].role
	})
	result := make([]reviewCarrierDigestJSONV1, 0, len(values))
	for _, value := range values {
		result = append(result, reviewCarrierDigestJSONV1{
			Role:    string(value.role),
			Carrier: value.carrier.String(),
			Digest:  value.digest.String(),
		})
	}
	return result
}

func canonicalLifecycleIntentDTOs(
	intent LifecycleIntent,
) []lifecycleIntentJSONV1 {
	values := intent.Values()
	sort.Slice(values, func(left int, right int) bool {
		leftKey := values[left].sectionRef + "\x00" + string(values[left].operation)
		rightKey := values[right].sectionRef + "\x00" + string(values[right].operation)
		return leftKey < rightKey
	})
	result := make([]lifecycleIntentJSONV1, 0, len(values))
	for _, value := range values {
		result = append(result, lifecycleIntentJSONV1{
			SectionRef: value.sectionRef,
			Operation:  string(value.operation),
		})
	}
	return result
}

func decodeReviewCarrierDigests(
	values []reviewCarrierDigestJSONV1,
) (ReviewCarrierDigestSet, error) {
	result := make([]ReviewCarrierDigest, 0, len(values))
	for index, value := range values {
		carrier, err := NewTargetCarrierID(value.Carrier)
		if err != nil {
			return ReviewCarrierDigestSet{}, fmt.Errorf("decode review carrier %d: %w", index, err)
		}
		digest, err := NewSHA256(value.Digest)
		if err != nil {
			return ReviewCarrierDigestSet{}, fmt.Errorf("decode review carrier %d: %w", index, err)
		}
		result = append(result, ReviewCarrierDigest{
			role:    ReviewCarrierRole(value.Role),
			carrier: carrier,
			digest:  digest,
		})
	}
	set := ReviewCarrierDigestSet{values: result}
	if err := validateReviewCarrierDigestSet(set); err != nil {
		return ReviewCarrierDigestSet{}, err
	}
	return set, nil
}

func decodeLifecycleIntent(
	values []lifecycleIntentJSONV1,
) (LifecycleIntent, error) {
	result := make([]LifecycleIntentItem, 0, len(values))
	for _, value := range values {
		result = append(result, LifecycleIntentItem{
			sectionRef: value.SectionRef,
			operation:  LifecycleOperation(value.Operation),
		})
	}
	intent := LifecycleIntent{values: result}
	if err := validateLifecycleIntent(intent); err != nil {
		return LifecycleIntent{}, err
	}
	return intent, nil
}
