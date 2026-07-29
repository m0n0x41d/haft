package projectprofile

import (
	"fmt"
	"strconv"
	"time"
)

const (
	profileAdmissionInputsDigestDomainV1 = "haft.project-profile.admission-inputs/v1"
	profileCommitPlanDigestDomainV1      = "haft.project-profile.commit-plan/v1"
	profileReceiptDigestDomainV1         = "haft.project-profile.declaration-receipt/v1"
	profileAdmissionRecordDigestDomainV1 = "haft.project-profile.admission-record/v1"
)

// ProfileDeclarationAdmissionInputs are non-binding pure inputs. They contain
// neither Declared state nor a receipt and grant no authority or persistence.
type ProfileDeclarationAdmissionInputs struct {
	candidate              ProfileDeclarationCandidateV1
	expectedLedgerRevision LedgerRevision
	digest                 ContentDigest
}

func NewProfileDeclarationAdmissionInputs(
	candidate ProfileDeclarationCandidateV1,
	expectedLedgerRevision LedgerRevision,
) (ProfileDeclarationAdmissionInputs, error) {
	err := validateProfileDeclarationCandidateV1(candidate)
	if err != nil {
		return ProfileDeclarationAdmissionInputs{}, err
	}
	_, err = expectedLedgerRevision.Next()
	if err != nil {
		return ProfileDeclarationAdmissionInputs{}, fmt.Errorf("expected ledger revision has no successor: %w", err)
	}
	value := ProfileDeclarationAdmissionInputs{
		candidate:              candidate,
		expectedLedgerRevision: expectedLedgerRevision,
	}
	value.digest = digestProfileDeclarationAdmissionInputsFields(value)
	return value, nil
}

func (inputs ProfileDeclarationAdmissionInputs) Candidate() ProfileDeclarationCandidateV1 {
	return inputs.candidate
}

func (inputs ProfileDeclarationAdmissionInputs) ExpectedLedgerRevision() LedgerRevision {
	return inputs.expectedLedgerRevision
}

func (inputs ProfileDeclarationAdmissionInputs) Digest() ContentDigest {
	return inputs.digest
}

func validateProfileDeclarationAdmissionInputs(
	inputs ProfileDeclarationAdmissionInputs,
) error {
	err := validateProfileDeclarationCandidateV1(inputs.candidate)
	if err != nil {
		return err
	}
	_, err = inputs.expectedLedgerRevision.Next()
	if err != nil {
		return fmt.Errorf("expected ledger revision has no successor: %w", err)
	}
	expected := digestProfileDeclarationAdmissionInputsFields(inputs)
	if inputs.digest != expected {
		return fmt.Errorf("profile declaration admission-input digest is not canonical")
	}
	return nil
}

func digestProfileDeclarationAdmissionInputsFields(
	inputs ProfileDeclarationAdmissionInputs,
) ContentDigest {
	writer := newCanonicalDigestWriter(profileAdmissionInputsDigestDomainV1)
	payloadDigest := inputs.candidate.provenance.payloadDigest
	provenanceDigest := inputs.candidate.provenance.candidateProvenanceHash
	payloadDigestText := payloadDigest.String()
	provenanceDigestText := provenanceDigest.String()
	expectedRevisionValue := inputs.expectedLedgerRevision.Value()
	expectedRevisionText := strconv.FormatUint(expectedRevisionValue, 10)
	writer.add(payloadDigestText)
	writer.add(provenanceDigestText)
	writer.add(expectedRevisionText)
	return writer.digest()
}

// ProfileDeclarationCommitPlan is still non-binding. It freezes the values a
// future admission transaction must revalidate, but carries no sealed
// capability, final revision, receipt, Declared profile, or commit method.
type ProfileDeclarationCommitPlan struct {
	inputs                          ProfileDeclarationAdmissionInputs
	authorityResolutionRecordRef    AuthorityResolutionRecordRef
	authorityResolutionRecordDigest ContentDigest
	singleUseKey                    SingleUseKey
	digest                          ContentDigest
}

func NewProfileDeclarationCommitPlan(
	inputs ProfileDeclarationAdmissionInputs,
	authorityResolutionRecordRef AuthorityResolutionRecordRef,
	authorityResolutionRecordDigest ContentDigest,
	singleUseKey SingleUseKey,
) (ProfileDeclarationCommitPlan, error) {
	err := validateProfileDeclarationAdmissionInputs(inputs)
	if err != nil {
		return ProfileDeclarationCommitPlan{}, err
	}
	if !authorityResolutionRecordRef.valid() || !authorityResolutionRecordDigest.valid() {
		return ProfileDeclarationCommitPlan{}, fmt.Errorf("commit plan authority-resolution record binding is invalid")
	}
	if !singleUseKey.valid() {
		return ProfileDeclarationCommitPlan{}, fmt.Errorf("commit plan single-use key is invalid")
	}
	value := ProfileDeclarationCommitPlan{
		inputs:                          inputs,
		authorityResolutionRecordRef:    authorityResolutionRecordRef,
		authorityResolutionRecordDigest: authorityResolutionRecordDigest,
		singleUseKey:                    singleUseKey,
	}
	value.digest = digestProfileDeclarationCommitPlanFields(value)
	return value, nil
}

func (plan ProfileDeclarationCommitPlan) Inputs() ProfileDeclarationAdmissionInputs {
	return plan.inputs
}

func (plan ProfileDeclarationCommitPlan) AuthorityResolutionRecordRef() AuthorityResolutionRecordRef {
	return plan.authorityResolutionRecordRef
}

func (plan ProfileDeclarationCommitPlan) AuthorityResolutionRecordDigest() ContentDigest {
	return plan.authorityResolutionRecordDigest
}

func (plan ProfileDeclarationCommitPlan) SingleUseKey() SingleUseKey {
	return plan.singleUseKey
}

func (plan ProfileDeclarationCommitPlan) Digest() ContentDigest {
	return plan.digest
}

func validateProfileDeclarationCommitPlan(plan ProfileDeclarationCommitPlan) error {
	err := validateProfileDeclarationAdmissionInputs(plan.inputs)
	if err != nil {
		return err
	}
	if !plan.authorityResolutionRecordRef.valid() || !plan.authorityResolutionRecordDigest.valid() {
		return fmt.Errorf("commit plan authority-resolution record binding is invalid")
	}
	if !plan.singleUseKey.valid() {
		return fmt.Errorf("commit plan single-use key is invalid")
	}
	expected := digestProfileDeclarationCommitPlanFields(plan)
	if plan.digest != expected {
		return fmt.Errorf("profile declaration commit-plan digest is not canonical")
	}
	return nil
}

func digestProfileDeclarationCommitPlanFields(plan ProfileDeclarationCommitPlan) ContentDigest {
	writer := newCanonicalDigestWriter(profileCommitPlanDigestDomainV1)
	inputsDigest := plan.inputs.digest.String()
	resolutionRef := plan.authorityResolutionRecordRef.String()
	resolutionDigest := plan.authorityResolutionRecordDigest.String()
	singleUseKey := plan.singleUseKey.String()
	writer.add(inputsDigest)
	writer.add(resolutionRef)
	writer.add(resolutionDigest)
	writer.add(singleUseKey)
	return writer.digest()
}

type ConfiguredProjectProfileV1 interface {
	configuredProjectProfileV1Variant()
}

type AutoProfileV1 struct{}

func (AutoProfileV1) configuredProjectProfileV1Variant() {}

type DeclaredProjectProfileV1 interface {
	ConfiguredProjectProfileV1
	Payload() ProfileDeclarationPayload
	Receipt() ProfileDeclarationReceiptV1
	declaredProjectProfileV1Variant()
}

type declaredProjectProfileV1 struct {
	payload ProfileDeclarationPayload
	receipt ProfileDeclarationReceiptV1
}

func (declaredProjectProfileV1) configuredProjectProfileV1Variant() {}
func (declaredProjectProfileV1) declaredProjectProfileV1Variant()   {}

func (profile declaredProjectProfileV1) Payload() ProfileDeclarationPayload {
	return profile.payload
}

func (profile declaredProjectProfileV1) Receipt() ProfileDeclarationReceiptV1 {
	return profile.receipt
}

type ProfileDeclarationReceiptV1 interface {
	AuthorityResolutionRecordRef() AuthorityResolutionRecordRef
	AuthorityResolutionRecordDigest() ContentDigest
	AuthorityBasisRef() ProfileDeclarationAuthorityBasisRef
	WorkRecordRef() ProfileOnboardingWorkRecordRef
	CandidateProvenanceDigest() ContentDigest
	PayloadDigest() ContentDigest
	ObservedBasisDigest() ContentDigest
	LedgerRevision() LedgerRevision
	RecordedAt() time.Time
	profileDeclarationReceiptV1Variant()
}

type profileDeclarationReceiptV1 struct {
	authorityResolutionRecordRef    AuthorityResolutionRecordRef
	authorityResolutionRecordDigest ContentDigest
	authorityBasisRef               ProfileDeclarationAuthorityBasisRef
	workRecordRef                   ProfileOnboardingWorkRecordRef
	candidateProvenanceDigest       ContentDigest
	payloadDigest                   ContentDigest
	observedBasisDigest             ContentDigest
	ledgerRevision                  LedgerRevision
	recordedAt                      time.Time
}

func (profileDeclarationReceiptV1) profileDeclarationReceiptV1Variant() {}

func (receipt profileDeclarationReceiptV1) AuthorityResolutionRecordRef() AuthorityResolutionRecordRef {
	return receipt.authorityResolutionRecordRef
}

func (receipt profileDeclarationReceiptV1) AuthorityResolutionRecordDigest() ContentDigest {
	return receipt.authorityResolutionRecordDigest
}

func (receipt profileDeclarationReceiptV1) AuthorityBasisRef() ProfileDeclarationAuthorityBasisRef {
	return receipt.authorityBasisRef
}

func (receipt profileDeclarationReceiptV1) WorkRecordRef() ProfileOnboardingWorkRecordRef {
	return receipt.workRecordRef
}

func (receipt profileDeclarationReceiptV1) CandidateProvenanceDigest() ContentDigest {
	return receipt.candidateProvenanceDigest
}

func (receipt profileDeclarationReceiptV1) PayloadDigest() ContentDigest {
	return receipt.payloadDigest
}

func (receipt profileDeclarationReceiptV1) ObservedBasisDigest() ContentDigest {
	return receipt.observedBasisDigest
}

func (receipt profileDeclarationReceiptV1) LedgerRevision() LedgerRevision {
	return receipt.ledgerRevision
}

func (receipt profileDeclarationReceiptV1) RecordedAt() time.Time {
	return receipt.recordedAt
}

type ProfileDeclarationAdmissionRecord interface {
	AdmissionRecordRef() ProfileDeclarationAdmissionRecordRef
	Payload() ProfileDeclarationPayload
	CandidateProvenance() CandidateProvenanceV1
	ClassificationWorkRecordRef() ProfileOnboardingWorkRecordRef
	AuthorityBasisRef() ProfileDeclarationAuthorityBasisRef
	AuthorityResolutionRecordRef() AuthorityResolutionRecordRef
	AuthorityResolutionRecordDigest() ContentDigest
	Receipt() ProfileDeclarationReceiptV1
	ExpectedLedgerRevision() LedgerRevision
	CommittedLedgerRevision() LedgerRevision
	SingleUseKey() SingleUseKey
	CommittedAt() time.Time
	profileDeclarationAdmissionRecordVariant()
}

type profileDeclarationAdmissionRecord struct {
	admissionRecordRef              ProfileDeclarationAdmissionRecordRef
	payload                         ProfileDeclarationPayload
	candidateProvenance             CandidateProvenanceV1
	classificationWorkRecordRef     ProfileOnboardingWorkRecordRef
	authorityBasisRef               ProfileDeclarationAuthorityBasisRef
	authorityResolutionRecordRef    AuthorityResolutionRecordRef
	authorityResolutionRecordDigest ContentDigest
	receipt                         ProfileDeclarationReceiptV1
	expectedLedgerRevision          LedgerRevision
	committedLedgerRevision         LedgerRevision
	singleUseKey                    SingleUseKey
	committedAt                     time.Time
}

func (profileDeclarationAdmissionRecord) profileDeclarationAdmissionRecordVariant() {}

func (record profileDeclarationAdmissionRecord) AdmissionRecordRef() ProfileDeclarationAdmissionRecordRef {
	return record.admissionRecordRef
}

func (record profileDeclarationAdmissionRecord) Payload() ProfileDeclarationPayload {
	return record.payload
}

func (record profileDeclarationAdmissionRecord) CandidateProvenance() CandidateProvenanceV1 {
	return record.candidateProvenance
}

func (record profileDeclarationAdmissionRecord) ClassificationWorkRecordRef() ProfileOnboardingWorkRecordRef {
	return record.classificationWorkRecordRef
}

func (record profileDeclarationAdmissionRecord) AuthorityBasisRef() ProfileDeclarationAuthorityBasisRef {
	return record.authorityBasisRef
}

func (record profileDeclarationAdmissionRecord) AuthorityResolutionRecordRef() AuthorityResolutionRecordRef {
	return record.authorityResolutionRecordRef
}

func (record profileDeclarationAdmissionRecord) AuthorityResolutionRecordDigest() ContentDigest {
	return record.authorityResolutionRecordDigest
}

func (record profileDeclarationAdmissionRecord) Receipt() ProfileDeclarationReceiptV1 {
	return record.receipt
}

func (record profileDeclarationAdmissionRecord) ExpectedLedgerRevision() LedgerRevision {
	return record.expectedLedgerRevision
}

func (record profileDeclarationAdmissionRecord) CommittedLedgerRevision() LedgerRevision {
	return record.committedLedgerRevision
}

func (record profileDeclarationAdmissionRecord) SingleUseKey() SingleUseKey {
	return record.singleUseKey
}

func (record profileDeclarationAdmissionRecord) CommittedAt() time.Time {
	return record.committedAt
}

func validateProfileDeclarationReceiptV1(receipt ProfileDeclarationReceiptV1) error {
	value, ok := receipt.(profileDeclarationReceiptV1)
	if !ok {
		return fmt.Errorf("unknown or externally supplied ProfileDeclarationReceiptV1")
	}
	if !value.authorityResolutionRecordRef.valid() || !value.authorityResolutionRecordDigest.valid() {
		return fmt.Errorf("receipt authority-resolution binding is invalid")
	}
	if !value.authorityBasisRef.valid() || !value.workRecordRef.valid() {
		return fmt.Errorf("receipt authority-basis and Work-record refs are invalid")
	}
	if !value.candidateProvenanceDigest.valid() || !value.payloadDigest.valid() || !value.observedBasisDigest.valid() {
		return fmt.Errorf("receipt provenance, payload, and observed-basis digests are invalid")
	}
	if value.ledgerRevision.Value() == 0 || value.recordedAt.IsZero() {
		return fmt.Errorf("receipt committed revision and recording time are required")
	}
	return nil
}

func validateProfileDeclarationAdmissionRecord(
	record ProfileDeclarationAdmissionRecord,
) error {
	value, ok := record.(profileDeclarationAdmissionRecord)
	if !ok {
		return fmt.Errorf("unknown or externally supplied ProfileDeclarationAdmissionRecord")
	}
	if !value.admissionRecordRef.valid() {
		return fmt.Errorf("admission-record ref is invalid")
	}
	if !value.payload.valid() {
		return fmt.Errorf("admission-record payload is invalid")
	}
	err := validateCandidateProvenanceV1(value.candidateProvenance)
	if err != nil {
		return err
	}
	payloadDigest, err := DigestProfileDeclarationPayload(value.payload)
	if err != nil {
		return err
	}
	if payloadDigest != value.candidateProvenance.payloadDigest {
		return fmt.Errorf("admission-record payload does not match candidate provenance")
	}
	if value.classificationWorkRecordRef != value.candidateProvenance.workRecordRef {
		return fmt.Errorf("admission record Work-record ref does not match provenance")
	}
	if value.authorityBasisRef != value.candidateProvenance.authorityBasisRef {
		return fmt.Errorf("admission record authority-basis ref does not match provenance")
	}
	if !value.authorityResolutionRecordRef.valid() || !value.authorityResolutionRecordDigest.valid() {
		return fmt.Errorf("admission record authority-resolution binding is invalid")
	}
	err = validateProfileDeclarationReceiptV1(value.receipt)
	if err != nil {
		return err
	}
	receipt := value.receipt.(profileDeclarationReceiptV1)
	if receipt.authorityBasisRef != value.authorityBasisRef {
		return fmt.Errorf("admission record and receipt authority-basis refs differ")
	}
	if receipt.workRecordRef != value.classificationWorkRecordRef {
		return fmt.Errorf("admission record and receipt Work-record refs differ")
	}
	if receipt.candidateProvenanceDigest != value.candidateProvenance.candidateProvenanceHash {
		return fmt.Errorf("admission record and receipt candidate-provenance digests differ")
	}
	if receipt.payloadDigest != payloadDigest {
		return fmt.Errorf("admission record and receipt payload digests differ")
	}
	if receipt.observedBasisDigest != value.candidateProvenance.observedBasisDigest {
		return fmt.Errorf("admission record and receipt observed-basis digests differ")
	}
	if receipt.authorityResolutionRecordRef != value.authorityResolutionRecordRef ||
		receipt.authorityResolutionRecordDigest != value.authorityResolutionRecordDigest {
		return fmt.Errorf("admission record and receipt authority-resolution bindings differ")
	}
	if receipt.ledgerRevision != value.committedLedgerRevision {
		return fmt.Errorf("admission record and receipt revisions differ")
	}
	expectedCommittedRevision, err := value.expectedLedgerRevision.Next()
	if err != nil {
		return err
	}
	if value.committedLedgerRevision != expectedCommittedRevision {
		return fmt.Errorf("admission record committed revision is not the expected next revision")
	}
	if !value.singleUseKey.valid() || value.committedAt.IsZero() {
		return fmt.Errorf("admission record single-use key and commit time are required")
	}
	if !value.committedAt.Equal(receipt.recordedAt) {
		return fmt.Errorf("admission record and receipt commit times differ")
	}
	return nil
}

func DigestProfileDeclarationReceiptV1(
	receipt ProfileDeclarationReceiptV1,
) (ContentDigest, error) {
	err := validateProfileDeclarationReceiptV1(receipt)
	if err != nil {
		return ContentDigest{}, err
	}
	value := receipt.(profileDeclarationReceiptV1)
	writer := newCanonicalDigestWriter(profileReceiptDigestDomainV1)
	addProfileDeclarationReceiptV1Fields(writer, value)
	return writer.digest(), nil
}

func DigestProfileDeclarationAdmissionRecord(
	record ProfileDeclarationAdmissionRecord,
) (ContentDigest, error) {
	err := validateProfileDeclarationAdmissionRecord(record)
	if err != nil {
		return ContentDigest{}, err
	}
	value := record.(profileDeclarationAdmissionRecord)
	receiptDigest, err := DigestProfileDeclarationReceiptV1(value.receipt)
	if err != nil {
		return ContentDigest{}, err
	}
	writer := newCanonicalDigestWriter(profileAdmissionRecordDigestDomainV1)
	admissionRecordRef := value.admissionRecordRef.String()
	writer.add(admissionRecordRef)
	payloadDigest, err := DigestProfileDeclarationPayload(value.payload)
	if err != nil {
		return ContentDigest{}, err
	}
	payloadDigestText := payloadDigest.String()
	provenanceDigest := value.candidateProvenance.candidateProvenanceHash.String()
	workRecordRef := value.classificationWorkRecordRef.String()
	authorityBasisRef := value.authorityBasisRef.String()
	resolutionRecordRef := value.authorityResolutionRecordRef.String()
	resolutionRecordDigest := value.authorityResolutionRecordDigest.String()
	receiptDigestText := receiptDigest.String()
	expectedRevisionValue := value.expectedLedgerRevision.Value()
	expectedRevision := strconv.FormatUint(expectedRevisionValue, 10)
	committedRevisionValue := value.committedLedgerRevision.Value()
	committedRevision := strconv.FormatUint(committedRevisionValue, 10)
	singleUseKey := value.singleUseKey.String()
	committedAt := canonicalTime(value.committedAt)
	writer.add(payloadDigestText)
	writer.add(provenanceDigest)
	writer.add(workRecordRef)
	writer.add(authorityBasisRef)
	writer.add(resolutionRecordRef)
	writer.add(resolutionRecordDigest)
	writer.add(receiptDigestText)
	writer.add(expectedRevision)
	writer.add(committedRevision)
	writer.add(singleUseKey)
	writer.add(committedAt)
	return writer.digest(), nil
}

func addProfileDeclarationReceiptV1Fields(
	writer canonicalDigestWriter,
	receipt profileDeclarationReceiptV1,
) {
	resolutionRecordRef := receipt.authorityResolutionRecordRef.String()
	resolutionRecordDigest := receipt.authorityResolutionRecordDigest.String()
	authorityBasisRef := receipt.authorityBasisRef.String()
	workRecordRef := receipt.workRecordRef.String()
	provenanceDigest := receipt.candidateProvenanceDigest.String()
	payloadDigest := receipt.payloadDigest.String()
	observedBasisDigest := receipt.observedBasisDigest.String()
	revisionValue := receipt.ledgerRevision.Value()
	revision := strconv.FormatUint(revisionValue, 10)
	recordedAt := canonicalTime(receipt.recordedAt)
	writer.add(resolutionRecordRef)
	writer.add(resolutionRecordDigest)
	writer.add(authorityBasisRef)
	writer.add(workRecordRef)
	writer.add(provenanceDigest)
	writer.add(payloadDigest)
	writer.add(observedBasisDigest)
	writer.add(revision)
	writer.add(recordedAt)
}
