package projectprofile

import (
	"bytes"
	"fmt"
)

// ValidateDurableProfileAdmissionRecordV1 proves only the semantic integrity
// of one already-read final-v1 durable record: canonical bytes, recomputed
// digests, embedded payload/receipt/provenance equality, and revision equality.
// It does not prove SQLite origin, transaction ownership, COMMIT, authority,
// freshness, or evidence in the world.
func ValidateDurableProfileAdmissionRecordV1(
	durable DurableProfileAdmissionTupleV1,
	candidateProvenanceCanonicalJSON []byte,
	candidateProvenanceDigest ContentDigest,
) error {
	value, ok := durable.(durableProfileAdmissionTupleV1)
	if !ok {
		return fmt.Errorf("unknown or externally supplied DurableProfileAdmissionTupleV1")
	}
	err := validateDurableProfileAdmissionTupleV1Fields(value)
	if err != nil {
		return fmt.Errorf("durable profile-admission tuple is invalid: %w", err)
	}
	admission, err := decodeProfileDeclarationAdmissionRecordCanonicalJSON(
		value.admissionRecordCanonicalJSON,
	)
	if err != nil {
		return fmt.Errorf("decode durable admission record: %w", err)
	}
	receipt, err := decodeProfileDeclarationReceiptV1CanonicalJSON(
		value.receiptCanonicalJSON,
	)
	if err != nil {
		return fmt.Errorf("decode durable receipt: %w", err)
	}
	payload, err := DecodeProfileDeclarationPayloadCanonicalJSON(
		value.payloadCanonicalJSON,
	)
	if err != nil {
		return fmt.Errorf("decode durable payload: %w", err)
	}
	admissionDigest, err := DigestProfileDeclarationAdmissionRecord(admission)
	if err != nil {
		return err
	}
	receiptDigest, err := DigestProfileDeclarationReceiptV1(receipt)
	if err != nil {
		return err
	}
	payloadDigest, err := DigestProfileDeclarationPayload(payload)
	if err != nil {
		return err
	}
	recordReceiptJSON, err := encodeProfileDeclarationReceiptV1CanonicalJSON(admission.Receipt())
	if err != nil {
		return err
	}
	recordPayloadJSON, err := EncodeProfileDeclarationPayloadCanonicalJSON(admission.Payload())
	if err != nil {
		return err
	}
	recordProvenance := admission.CandidateProvenance()
	recordProvenanceJSON, err := encodeCandidateProvenanceV1CanonicalJSON(recordProvenance)
	if err != nil {
		return err
	}
	recordProvenanceDigest, err := DigestCandidateProvenanceV1(recordProvenance)
	if err != nil {
		return err
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: admissionDigest == value.admissionRecordDigest, name: "admission-record digest"},
		{matches: receiptDigest == value.receiptDigest, name: "receipt digest"},
		{matches: payloadDigest == value.payloadDigest, name: "payload digest"},
		{matches: bytes.Equal(recordReceiptJSON, value.receiptCanonicalJSON), name: "embedded receipt"},
		{matches: bytes.Equal(recordPayloadJSON, value.payloadCanonicalJSON), name: "embedded payload"},
		{matches: bytes.Equal(recordProvenanceJSON, candidateProvenanceCanonicalJSON), name: "candidate provenance JSON"},
		{matches: recordProvenanceDigest == candidateProvenanceDigest, name: "candidate provenance digest"},
		{matches: admission.CommittedLedgerRevision() == value.ledgerRevision, name: "admission ledger revision"},
		{matches: receipt.LedgerRevision() == value.ledgerRevision, name: "receipt ledger revision"},
	}
	return validateDurableProfileAdmissionRecordChecksV1(checks, 0)
}

func validateDurableProfileAdmissionRecordChecksV1(
	checks []struct {
		matches bool
		name    string
	},
	index int,
) error {
	if index >= len(checks) {
		return nil
	}
	check := checks[index]
	if !check.matches {
		return fmt.Errorf("durable profile admission has a mismatched %s", check.name)
	}
	return validateDurableProfileAdmissionRecordChecksV1(checks, index+1)
}
