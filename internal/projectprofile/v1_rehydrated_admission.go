package projectprofile

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// DurableProfileAdmissionTupleV1 is a package-owned container for the exact
// row material returned by a durable admission reread. The type proves only
// that its fields are structurally present. It does not prove that the bytes
// came from SQLite, that a transaction committed, or that the tuple is
// canonical. Only AdmissionService may construct it from a strict durable
// reread and pass it to RehydrateProfileAdmissionV1.
type DurableProfileAdmissionTupleV1 interface {
	AdmissionRecordCanonicalJSON() []byte
	AdmissionRecordDigest() ContentDigest
	ReceiptCanonicalJSON() []byte
	ReceiptDigest() ContentDigest
	PayloadCanonicalJSON() []byte
	PayloadDigest() ContentDigest
	LedgerRevision() LedgerRevision
	durableProfileAdmissionTupleV1Variant()
}

type durableProfileAdmissionTupleV1 struct {
	admissionRecordCanonicalJSON []byte
	admissionRecordDigest        ContentDigest
	receiptCanonicalJSON         []byte
	receiptDigest                ContentDigest
	payloadCanonicalJSON         []byte
	payloadDigest                ContentDigest
	ledgerRevision               LedgerRevision
}

func (durableProfileAdmissionTupleV1) durableProfileAdmissionTupleV1Variant() {}

// DurableProfileAdmissionTupleV1Builder packages transaction-adapter reread
// material without interpreting it as committed semantics. Canonical decoding
// is deliberately deferred until after exact Prepared/Tentative comparison.
type DurableProfileAdmissionTupleV1Builder struct {
	value durableProfileAdmissionTupleV1
}

func NewDurableProfileAdmissionTupleV1Builder(
	admissionRecordCanonicalJSON []byte,
	admissionRecordDigest ContentDigest,
) DurableProfileAdmissionTupleV1Builder {
	return DurableProfileAdmissionTupleV1Builder{value: durableProfileAdmissionTupleV1{
		admissionRecordCanonicalJSON: clonePreparedBytesV1(admissionRecordCanonicalJSON),
		admissionRecordDigest:        admissionRecordDigest,
	}}
}

func (builder DurableProfileAdmissionTupleV1Builder) WithReceipt(
	receiptCanonicalJSON []byte,
	receiptDigest ContentDigest,
) DurableProfileAdmissionTupleV1Builder {
	builder.value.receiptCanonicalJSON = clonePreparedBytesV1(receiptCanonicalJSON)
	builder.value.receiptDigest = receiptDigest
	return builder
}

func (builder DurableProfileAdmissionTupleV1Builder) WithPayload(
	payloadCanonicalJSON []byte,
	payloadDigest ContentDigest,
) DurableProfileAdmissionTupleV1Builder {
	builder.value.payloadCanonicalJSON = clonePreparedBytesV1(payloadCanonicalJSON)
	builder.value.payloadDigest = payloadDigest
	return builder
}

func (builder DurableProfileAdmissionTupleV1Builder) AtLedgerRevision(
	ledgerRevision LedgerRevision,
) DurableProfileAdmissionTupleV1Builder {
	builder.value.ledgerRevision = ledgerRevision
	return builder
}

func (builder DurableProfileAdmissionTupleV1Builder) Build() (DurableProfileAdmissionTupleV1, error) {
	value := builder.value
	err := validateDurableProfileAdmissionTupleV1Fields(value)
	if err != nil {
		return nil, err
	}
	value.admissionRecordCanonicalJSON = clonePreparedBytesV1(value.admissionRecordCanonicalJSON)
	value.receiptCanonicalJSON = clonePreparedBytesV1(value.receiptCanonicalJSON)
	value.payloadCanonicalJSON = clonePreparedBytesV1(value.payloadCanonicalJSON)
	return value, nil
}

func validateDurableProfileAdmissionTupleV1Fields(
	value durableProfileAdmissionTupleV1,
) error {
	checks := []struct {
		valid  bool
		reason string
	}{
		{valid: json.Valid(value.admissionRecordCanonicalJSON), reason: "durable admission-record JSON is required"},
		{valid: value.admissionRecordDigest.valid(), reason: "durable admission-record digest is invalid"},
		{valid: json.Valid(value.receiptCanonicalJSON), reason: "durable receipt JSON is required"},
		{valid: value.receiptDigest.valid(), reason: "durable receipt digest is invalid"},
		{valid: json.Valid(value.payloadCanonicalJSON), reason: "durable payload JSON is required"},
		{valid: value.payloadDigest.valid(), reason: "durable payload digest is invalid"},
		{valid: value.ledgerRevision.Value() > 0, reason: "durable ledger revision is required"},
	}
	return visitSliceV1(checks, func(_ int, check struct {
		valid  bool
		reason string
	}) error {
		if !check.valid {
			return fmt.Errorf("%s", check.reason)
		}
		return nil
	})
}

// RehydratedProfileAdmissionV1 is the sealed semantic result of matching a
// strict durable reread to the exact pre-COMMIT Prepared and Tentative values,
// then canonically decoding and redigesting every final carrier. The type does
// not attest database origin by itself; AdmissionService is responsible for
// calling this boundary only after a verified post-COMMIT durable reread.
type RehydratedProfileAdmissionV1 interface {
	DeclaredProfile() DeclaredProjectProfileV1
	Receipt() ProfileDeclarationReceiptV1
	AdmissionRecord() ProfileDeclarationAdmissionRecord
	AdmissionRecordDigest() ContentDigest
	LedgerRevision() LedgerRevision
	rehydratedProfileAdmissionV1Variant()
}

type rehydratedProfileAdmissionV1 struct {
	declaredProfile       DeclaredProjectProfileV1
	receipt               ProfileDeclarationReceiptV1
	admissionRecord       ProfileDeclarationAdmissionRecord
	admissionRecordDigest ContentDigest
	ledgerRevision        LedgerRevision
}

func (rehydratedProfileAdmissionV1) rehydratedProfileAdmissionV1Variant() {}

// ValidateRehydratedProfileAdmissionV1 verifies that a value is the exact
// package-owned semantic result produced by RehydrateProfileAdmissionV1 and
// that all of its mutually-redundant final bindings still agree. This is the
// boundary an admission orchestrator must use before projecting a transaction
// adapter result into the public Admitted variant. In particular, embedding a
// nil or real RehydratedProfileAdmissionV1 in a foreign struct does not pass.
//
// The check validates semantic consistency only. It does not independently
// prove SQLite origin or COMMIT; the transaction-owning adapter remains
// responsible for strict post-COMMIT durable reread before rehydration.
func ValidateRehydratedProfileAdmissionV1(
	admission RehydratedProfileAdmissionV1,
) error {
	value, ok := admission.(rehydratedProfileAdmissionV1)
	if !ok {
		return fmt.Errorf("unknown or externally supplied RehydratedProfileAdmissionV1")
	}
	profile, ok := value.declaredProfile.(declaredProjectProfileV1)
	if !ok {
		return fmt.Errorf("rehydrated Declared profile is not the package-owned value")
	}
	err := validateProfileDeclarationReceiptV1(value.receipt)
	if err != nil {
		return err
	}
	err = validateProfileDeclarationReceiptV1(profile.receipt)
	if err != nil {
		return err
	}
	err = validateProfileDeclarationAdmissionRecord(value.admissionRecord)
	if err != nil {
		return err
	}
	recordDigest, err := DigestProfileDeclarationAdmissionRecord(value.admissionRecord)
	if err != nil {
		return err
	}
	recordPayloadJSON, err := EncodeProfileDeclarationPayloadCanonicalJSON(
		value.admissionRecord.Payload(),
	)
	if err != nil {
		return err
	}
	profilePayloadJSON, err := EncodeProfileDeclarationPayloadCanonicalJSON(profile.payload)
	if err != nil {
		return err
	}
	recordReceiptJSON, err := encodeProfileDeclarationReceiptV1CanonicalJSON(
		value.admissionRecord.Receipt(),
	)
	if err != nil {
		return err
	}
	resultReceiptJSON, err := encodeProfileDeclarationReceiptV1CanonicalJSON(value.receipt)
	if err != nil {
		return err
	}
	profileReceiptJSON, err := encodeProfileDeclarationReceiptV1CanonicalJSON(profile.receipt)
	if err != nil {
		return err
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: value.admissionRecordDigest.valid(), name: "admission-record digest validity"},
		{matches: recordDigest == value.admissionRecordDigest, name: "admission-record digest"},
		{matches: value.ledgerRevision.Value() > 0, name: "ledger revision validity"},
		{matches: value.admissionRecord.CommittedLedgerRevision() == value.ledgerRevision, name: "admission-record revision"},
		{matches: value.receipt.LedgerRevision() == value.ledgerRevision, name: "receipt revision"},
		{matches: profile.receipt.LedgerRevision() == value.ledgerRevision, name: "Declared-profile receipt revision"},
		{matches: bytes.Equal(recordPayloadJSON, profilePayloadJSON), name: "Declared-profile payload"},
		{matches: bytes.Equal(recordReceiptJSON, resultReceiptJSON), name: "result receipt"},
		{matches: bytes.Equal(recordReceiptJSON, profileReceiptJSON), name: "Declared-profile receipt"},
	}
	return visitSliceV1(checks, func(_ int, check struct {
		matches bool
		name    string
	}) error {
		if !check.matches {
			return fmt.Errorf("rehydrated profile admission has a mismatched %s", check.name)
		}
		return nil
	})
}

// RehydrateProfileAdmissionV1 is the only public final-v1 rehydration path.
// Raw durable JSON cannot bypass exact Prepared/Tentative comparison, and the
// function never invokes the pre-COMMIT finalizer.
func RehydrateProfileAdmissionV1(
	prepared PreparedProfileAdmissionV1,
	tentative TentativeProfileAdmissionTransactionMaterialV1,
	durable DurableProfileAdmissionTupleV1,
) (RehydratedProfileAdmissionV1, error) {
	preparedValue, ok := prepared.(preparedProfileAdmissionV1)
	if !ok {
		return nil, fmt.Errorf("unknown or externally supplied PreparedProfileAdmissionV1")
	}
	tentativeValue, ok := tentative.(tentativeProfileAdmissionTransactionMaterialV1)
	if !ok {
		return nil, fmt.Errorf("unknown or externally supplied TentativeProfileAdmissionTransactionMaterialV1")
	}
	durableValue, ok := durable.(durableProfileAdmissionTupleV1)
	if !ok {
		return nil, fmt.Errorf("unknown or externally supplied DurableProfileAdmissionTupleV1")
	}
	err := ValidatePreparedProfileAdmissionV1(preparedValue)
	if err != nil {
		return nil, err
	}
	err = ValidateTentativeProfileAdmissionTransactionMaterialV1(tentativeValue)
	if err != nil {
		return nil, err
	}
	err = comparePreparedProfileAdmissionV1(tentativeValue.prepared, preparedValue)
	if err != nil {
		return nil, fmt.Errorf("tentative material belongs to another prepared admission: %w", err)
	}
	err = validateDurableProfileAdmissionTupleV1Fields(durableValue)
	if err != nil {
		return nil, err
	}
	err = compareDurableProfileAdmissionTupleV1(preparedValue, tentativeValue, durableValue)
	if err != nil {
		return nil, err
	}
	return decodeRehydratedProfileAdmissionV1(preparedValue, durableValue)
}

func compareDurableProfileAdmissionTupleV1(
	prepared preparedProfileAdmissionV1,
	tentative tentativeProfileAdmissionTransactionMaterialV1,
	durable durableProfileAdmissionTupleV1,
) error {
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: bytes.Equal(durable.admissionRecordCanonicalJSON, tentative.admissionRecordCanonicalJSON), name: "admission-record JSON"},
		{matches: durable.admissionRecordDigest == tentative.admissionRecordDigest, name: "admission-record digest"},
		{matches: bytes.Equal(durable.receiptCanonicalJSON, tentative.receiptCanonicalJSON), name: "receipt JSON"},
		{matches: durable.receiptDigest == tentative.receiptDigest, name: "receipt digest"},
		{matches: bytes.Equal(durable.payloadCanonicalJSON, prepared.profilePayloadCanonicalJSON), name: "payload JSON"},
		{matches: durable.payloadDigest == prepared.profilePayloadDigest, name: "payload digest"},
		{matches: durable.ledgerRevision == tentative.committedLedgerRevision, name: "ledger revision"},
	}
	return visitSliceV1(checks, func(_ int, check struct {
		matches bool
		name    string
	}) error {
		if !check.matches {
			return fmt.Errorf("durable profile admission has a mismatched %s", check.name)
		}
		return nil
	})
}

func decodeRehydratedProfileAdmissionV1(
	prepared preparedProfileAdmissionV1,
	durable durableProfileAdmissionTupleV1,
) (RehydratedProfileAdmissionV1, error) {
	admissionRecord, err := decodeProfileDeclarationAdmissionRecordCanonicalJSON(
		durable.admissionRecordCanonicalJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("decode durable admission record: %w", err)
	}
	receipt, err := decodeProfileDeclarationReceiptV1CanonicalJSON(durable.receiptCanonicalJSON)
	if err != nil {
		return nil, fmt.Errorf("decode durable receipt: %w", err)
	}
	payload, err := DecodeProfileDeclarationPayloadCanonicalJSON(durable.payloadCanonicalJSON)
	if err != nil {
		return nil, fmt.Errorf("decode durable profile payload: %w", err)
	}
	err = validateDecodedDurableProfileAdmissionV1(
		prepared,
		durable,
		admissionRecord,
		receipt,
		payload,
	)
	if err != nil {
		return nil, err
	}
	profile := declaredProjectProfileV1{payload: payload, receipt: receipt}
	return rehydratedProfileAdmissionV1{
		declaredProfile:       profile,
		receipt:               receipt,
		admissionRecord:       admissionRecord,
		admissionRecordDigest: durable.admissionRecordDigest,
		ledgerRevision:        durable.ledgerRevision,
	}, nil
}

func validateDecodedDurableProfileAdmissionV1(
	prepared preparedProfileAdmissionV1,
	durable durableProfileAdmissionTupleV1,
	admissionRecord ProfileDeclarationAdmissionRecord,
	receipt ProfileDeclarationReceiptV1,
	payload ProfileDeclarationPayload,
) error {
	admissionDigest, err := DigestProfileDeclarationAdmissionRecord(admissionRecord)
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
	recordReceiptJSON, err := encodeProfileDeclarationReceiptV1CanonicalJSON(admissionRecord.Receipt())
	if err != nil {
		return err
	}
	recordPayloadJSON, err := EncodeProfileDeclarationPayloadCanonicalJSON(admissionRecord.Payload())
	if err != nil {
		return err
	}
	recordProvenanceDigest, err := DigestCandidateProvenanceV1(admissionRecord.CandidateProvenance())
	if err != nil {
		return err
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: admissionDigest == durable.admissionRecordDigest, name: "recomputed admission-record digest"},
		{matches: receiptDigest == durable.receiptDigest, name: "recomputed receipt digest"},
		{matches: payloadDigest == durable.payloadDigest, name: "recomputed payload digest"},
		{matches: bytes.Equal(recordReceiptJSON, durable.receiptCanonicalJSON), name: "admission-record receipt"},
		{matches: bytes.Equal(recordPayloadJSON, durable.payloadCanonicalJSON), name: "admission-record payload"},
		{matches: recordProvenanceDigest == prepared.candidateProvenanceDigest, name: "candidate provenance"},
		{matches: admissionRecord.ExpectedLedgerRevision() == prepared.ExpectedLedgerRevision(), name: "expected ledger revision"},
		{matches: admissionRecord.CommittedLedgerRevision() == durable.ledgerRevision, name: "committed ledger revision"},
		{matches: receipt.LedgerRevision() == durable.ledgerRevision, name: "receipt ledger revision"},
		{matches: admissionRecord.AuthorityResolutionRecordRef() == prepared.plan.authorityResolutionRecordRef, name: "authority-resolution ref"},
		{matches: admissionRecord.AuthorityResolutionRecordDigest() == prepared.plan.authorityResolutionRecordDigest, name: "authority-resolution digest"},
		{matches: admissionRecord.SingleUseKey() == prepared.plan.singleUseKey, name: "single-use key"},
	}
	return visitSliceV1(checks, func(_ int, check struct {
		matches bool
		name    string
	}) error {
		if !check.matches {
			return fmt.Errorf("rehydrated profile admission has a mismatched %s", check.name)
		}
		return nil
	})
}

func (value durableProfileAdmissionTupleV1) AdmissionRecordCanonicalJSON() []byte {
	return clonePreparedBytesV1(value.admissionRecordCanonicalJSON)
}

func (value durableProfileAdmissionTupleV1) AdmissionRecordDigest() ContentDigest {
	return value.admissionRecordDigest
}

func (value durableProfileAdmissionTupleV1) ReceiptCanonicalJSON() []byte {
	return clonePreparedBytesV1(value.receiptCanonicalJSON)
}

func (value durableProfileAdmissionTupleV1) ReceiptDigest() ContentDigest {
	return value.receiptDigest
}

func (value durableProfileAdmissionTupleV1) PayloadCanonicalJSON() []byte {
	return clonePreparedBytesV1(value.payloadCanonicalJSON)
}

func (value durableProfileAdmissionTupleV1) PayloadDigest() ContentDigest {
	return value.payloadDigest
}

func (value durableProfileAdmissionTupleV1) LedgerRevision() LedgerRevision {
	return value.ledgerRevision
}

func (value rehydratedProfileAdmissionV1) DeclaredProfile() DeclaredProjectProfileV1 {
	return value.declaredProfile
}

func (value rehydratedProfileAdmissionV1) Receipt() ProfileDeclarationReceiptV1 {
	return value.receipt
}

func (value rehydratedProfileAdmissionV1) AdmissionRecord() ProfileDeclarationAdmissionRecord {
	return value.admissionRecord
}

func (value rehydratedProfileAdmissionV1) AdmissionRecordDigest() ContentDigest {
	return value.admissionRecordDigest
}

func (value rehydratedProfileAdmissionV1) LedgerRevision() LedgerRevision {
	return value.ledgerRevision
}
