package projectprofile

import (
	"bytes"
	"fmt"
)

const (
	profileDeclarationReceiptJSONSchemaV1         = "haft.project-profile.declaration-receipt/v1"
	profileDeclarationAdmissionRecordJSONSchemaV1 = "haft.project-profile.admission-record/v1"
)

type profileDeclarationReceiptJSONV1 struct {
	Schema                          string `json:"schema"`
	AuthorityResolutionRecordRef    string `json:"authority_resolution_record_ref"`
	AuthorityResolutionRecordDigest string `json:"authority_resolution_record_digest"`
	AuthorityBasisRef               string `json:"authority_basis_ref"`
	WorkRecordRef                   string `json:"work_record_ref"`
	CandidateProvenanceDigest       string `json:"candidate_provenance_digest"`
	PayloadDigest                   string `json:"payload_digest"`
	ObservedBasisDigest             string `json:"observed_basis_digest"`
	LedgerRevision                  uint64 `json:"ledger_revision"`
	RecordedAt                      string `json:"recorded_at"`
}

type profileDeclarationAdmissionRecordJSONV1 struct {
	Schema                          string                          `json:"schema"`
	AdmissionRecordRef              string                          `json:"admission_record_ref"`
	Payload                         profileDeclarationPayloadJSONV1 `json:"payload"`
	CandidateProvenance             candidateProvenanceJSONV1       `json:"candidate_provenance"`
	ClassificationWorkRecordRef     string                          `json:"classification_work_record_ref"`
	AuthorityBasisRef               string                          `json:"authority_basis_ref"`
	AuthorityResolutionRecordRef    string                          `json:"authority_resolution_record_ref"`
	AuthorityResolutionRecordDigest string                          `json:"authority_resolution_record_digest"`
	Receipt                         profileDeclarationReceiptJSONV1 `json:"receipt"`
	ExpectedLedgerRevision          uint64                          `json:"expected_ledger_revision"`
	CommittedLedgerRevision         uint64                          `json:"committed_ledger_revision"`
	SingleUseKey                    string                          `json:"single_use_key"`
	CommittedAt                     string                          `json:"committed_at"`
}

// Final-record codecs remain package-private. They reconstruct already
// committed ledger descriptions; they are not an admission or finalization
// surface and cannot be reached through MCP/CLI inputs.
func encodeProfileDeclarationReceiptV1CanonicalJSON(
	receipt ProfileDeclarationReceiptV1,
) ([]byte, error) {
	dto, err := profileDeclarationReceiptToJSONV1(receipt)
	if err != nil {
		return nil, err
	}
	return marshalCanonicalJSONV1(dto)
}

func decodeProfileDeclarationReceiptV1CanonicalJSON(
	data []byte,
) (ProfileDeclarationReceiptV1, error) {
	var dto profileDeclarationReceiptJSONV1
	err := decodeJSONV1(data, &dto)
	if err != nil {
		return nil, err
	}
	receipt, err := profileDeclarationReceiptFromJSONV1(dto)
	if err != nil {
		return nil, err
	}
	canonical, err := encodeProfileDeclarationReceiptV1CanonicalJSON(receipt)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, canonical) {
		return nil, fmt.Errorf("profile declaration receipt JSON is not canonical")
	}
	return receipt, nil
}

func encodeProfileDeclarationAdmissionRecordCanonicalJSON(
	record ProfileDeclarationAdmissionRecord,
) ([]byte, error) {
	dto, err := profileDeclarationAdmissionRecordToJSONV1(record)
	if err != nil {
		return nil, err
	}
	return marshalCanonicalJSONV1(dto)
}

func decodeProfileDeclarationAdmissionRecordCanonicalJSON(
	data []byte,
) (ProfileDeclarationAdmissionRecord, error) {
	var dto profileDeclarationAdmissionRecordJSONV1
	err := decodeJSONV1(data, &dto)
	if err != nil {
		return nil, err
	}
	record, err := profileDeclarationAdmissionRecordFromJSONV1(dto)
	if err != nil {
		return nil, err
	}
	canonical, err := encodeProfileDeclarationAdmissionRecordCanonicalJSON(record)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, canonical) {
		return nil, fmt.Errorf("profile declaration admission-record JSON is not canonical")
	}
	return record, nil
}

func profileDeclarationReceiptToJSONV1(
	receipt ProfileDeclarationReceiptV1,
) (profileDeclarationReceiptJSONV1, error) {
	err := validateProfileDeclarationReceiptV1(receipt)
	if err != nil {
		return profileDeclarationReceiptJSONV1{}, err
	}
	value := receipt.(profileDeclarationReceiptV1)
	return profileDeclarationReceiptJSONV1{
		Schema:                          profileDeclarationReceiptJSONSchemaV1,
		AuthorityResolutionRecordRef:    value.authorityResolutionRecordRef.String(),
		AuthorityResolutionRecordDigest: value.authorityResolutionRecordDigest.String(),
		AuthorityBasisRef:               value.authorityBasisRef.String(),
		WorkRecordRef:                   value.workRecordRef.String(),
		CandidateProvenanceDigest:       value.candidateProvenanceDigest.String(),
		PayloadDigest:                   value.payloadDigest.String(),
		ObservedBasisDigest:             value.observedBasisDigest.String(),
		LedgerRevision:                  value.ledgerRevision.Value(),
		RecordedAt:                      canonicalTime(value.recordedAt),
	}, nil
}

func profileDeclarationReceiptFromJSONV1(
	dto profileDeclarationReceiptJSONV1,
) (ProfileDeclarationReceiptV1, error) {
	if dto.Schema != profileDeclarationReceiptJSONSchemaV1 {
		return nil, fmt.Errorf("unsupported receipt JSON schema %q", dto.Schema)
	}
	resolutionRef, err := NewAuthorityResolutionRecordRef(dto.AuthorityResolutionRecordRef)
	if err != nil {
		return nil, err
	}
	resolutionDigest, err := NewContentDigest(dto.AuthorityResolutionRecordDigest)
	if err != nil {
		return nil, err
	}
	authorityBasisRef, err := NewProfileDeclarationAuthorityBasisRef(dto.AuthorityBasisRef)
	if err != nil {
		return nil, err
	}
	workRecordRef, err := NewProfileOnboardingWorkRecordRef(dto.WorkRecordRef)
	if err != nil {
		return nil, err
	}
	provenanceDigest, err := NewContentDigest(dto.CandidateProvenanceDigest)
	if err != nil {
		return nil, err
	}
	payloadDigest, err := NewContentDigest(dto.PayloadDigest)
	if err != nil {
		return nil, err
	}
	observedBasisDigest, err := NewContentDigest(dto.ObservedBasisDigest)
	if err != nil {
		return nil, err
	}
	recordedAt, err := parseCanonicalTimeV1("receipt recorded_at", dto.RecordedAt)
	if err != nil {
		return nil, err
	}
	value := profileDeclarationReceiptV1{
		authorityResolutionRecordRef:    resolutionRef,
		authorityResolutionRecordDigest: resolutionDigest,
		authorityBasisRef:               authorityBasisRef,
		workRecordRef:                   workRecordRef,
		candidateProvenanceDigest:       provenanceDigest,
		payloadDigest:                   payloadDigest,
		observedBasisDigest:             observedBasisDigest,
		ledgerRevision:                  NewLedgerRevision(dto.LedgerRevision),
		recordedAt:                      recordedAt,
	}
	err = validateProfileDeclarationReceiptV1(value)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func profileDeclarationAdmissionRecordToJSONV1(
	record ProfileDeclarationAdmissionRecord,
) (profileDeclarationAdmissionRecordJSONV1, error) {
	err := validateProfileDeclarationAdmissionRecord(record)
	if err != nil {
		return profileDeclarationAdmissionRecordJSONV1{}, err
	}
	value := record.(profileDeclarationAdmissionRecord)
	payload, err := profileDeclarationPayloadToJSONV1(value.payload)
	if err != nil {
		return profileDeclarationAdmissionRecordJSONV1{}, err
	}
	receipt, err := profileDeclarationReceiptToJSONV1(value.receipt)
	if err != nil {
		return profileDeclarationAdmissionRecordJSONV1{}, err
	}
	return profileDeclarationAdmissionRecordJSONV1{
		Schema:                          profileDeclarationAdmissionRecordJSONSchemaV1,
		AdmissionRecordRef:              value.admissionRecordRef.String(),
		Payload:                         payload,
		CandidateProvenance:             candidateProvenanceToJSONV1(value.candidateProvenance),
		ClassificationWorkRecordRef:     value.classificationWorkRecordRef.String(),
		AuthorityBasisRef:               value.authorityBasisRef.String(),
		AuthorityResolutionRecordRef:    value.authorityResolutionRecordRef.String(),
		AuthorityResolutionRecordDigest: value.authorityResolutionRecordDigest.String(),
		Receipt:                         receipt,
		ExpectedLedgerRevision:          value.expectedLedgerRevision.Value(),
		CommittedLedgerRevision:         value.committedLedgerRevision.Value(),
		SingleUseKey:                    value.singleUseKey.String(),
		CommittedAt:                     canonicalTime(value.committedAt),
	}, nil
}

func profileDeclarationAdmissionRecordFromJSONV1(
	dto profileDeclarationAdmissionRecordJSONV1,
) (ProfileDeclarationAdmissionRecord, error) {
	if dto.Schema != profileDeclarationAdmissionRecordJSONSchemaV1 {
		return nil, fmt.Errorf("unsupported admission-record JSON schema %q", dto.Schema)
	}
	admissionRecordRef, err := NewProfileDeclarationAdmissionRecordRef(dto.AdmissionRecordRef)
	if err != nil {
		return nil, err
	}
	payload, err := profileDeclarationPayloadFromJSONV1(dto.Payload)
	if err != nil {
		return nil, err
	}
	provenance, err := candidateProvenanceFromJSONV1(dto.CandidateProvenance)
	if err != nil {
		return nil, err
	}
	workRecordRef, err := NewProfileOnboardingWorkRecordRef(dto.ClassificationWorkRecordRef)
	if err != nil {
		return nil, err
	}
	authorityBasisRef, err := NewProfileDeclarationAuthorityBasisRef(dto.AuthorityBasisRef)
	if err != nil {
		return nil, err
	}
	resolutionRef, err := NewAuthorityResolutionRecordRef(dto.AuthorityResolutionRecordRef)
	if err != nil {
		return nil, err
	}
	resolutionDigest, err := NewContentDigest(dto.AuthorityResolutionRecordDigest)
	if err != nil {
		return nil, err
	}
	receipt, err := profileDeclarationReceiptFromJSONV1(dto.Receipt)
	if err != nil {
		return nil, err
	}
	singleUseKey, err := NewSingleUseKey(dto.SingleUseKey)
	if err != nil {
		return nil, err
	}
	committedAt, err := parseCanonicalTimeV1("admission-record committed_at", dto.CommittedAt)
	if err != nil {
		return nil, err
	}
	value := profileDeclarationAdmissionRecord{
		admissionRecordRef:              admissionRecordRef,
		payload:                         payload,
		candidateProvenance:             provenance,
		classificationWorkRecordRef:     workRecordRef,
		authorityBasisRef:               authorityBasisRef,
		authorityResolutionRecordRef:    resolutionRef,
		authorityResolutionRecordDigest: resolutionDigest,
		receipt:                         receipt,
		expectedLedgerRevision:          NewLedgerRevision(dto.ExpectedLedgerRevision),
		committedLedgerRevision:         NewLedgerRevision(dto.CommittedLedgerRevision),
		singleUseKey:                    singleUseKey,
		committedAt:                     committedAt,
	}
	err = validateProfileDeclarationAdmissionRecord(value)
	if err != nil {
		return nil, err
	}
	return value, nil
}
