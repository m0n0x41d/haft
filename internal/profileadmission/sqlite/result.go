package sqlite

import (
	"bytes"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

// AdmissionResultKind is the closed effect outcome returned by Service.
// AdmissionResult itself is concrete and package-sealed: callers can inspect
// a variant but cannot construct one.
type AdmissionResultKind string

const (
	AdmissionResultAdmitted    AdmissionResultKind = "admitted"
	AdmissionResultNotAdmitted AdmissionResultKind = "not_admitted"
	AdmissionResultWriteFailed AdmissionResultKind = "write_failed"
)

// CanonicalAdmissionDelivery says how the already-validated durable admission
// was reached. It never says that a write occurred in the current call.
type CanonicalAdmissionDelivery string

const (
	CanonicalAdmissionFresh                CanonicalAdmissionDelivery = "fresh"
	CanonicalAdmissionReplayed             CanonicalAdmissionDelivery = "replayed"
	CanonicalAdmissionRecovered            CanonicalAdmissionDelivery = "recovered"
	CanonicalAdmissionResolvedAfterRestart CanonicalAdmissionDelivery = "resolved_after_restart"
)

// AdmissionCommitPosture is the strongest effect claim available when Service
// cannot return a canonical admission.
type AdmissionCommitPosture string

const (
	AdmissionDefinitelyNotCommitted AdmissionCommitPosture = "definitely_not_committed"
	AdmissionCommitOutcomeUnknown   AdmissionCommitPosture = "commit_outcome_unknown"
)

// AdmissionDenial is a non-binding reason why no canonical effect was made.
// Its fields are private so only the SQLite admission boundary can produce it.
type AdmissionDenial struct {
	code   string
	detail string
}

func (denial AdmissionDenial) Code() string {
	return denial.code
}

func (denial AdmissionDenial) Detail() string {
	return denial.detail
}

// AdmissionFailure identifies an effect-stage failure without exposing an
// error string as transaction evidence.
type AdmissionFailure struct {
	commitPosture AdmissionCommitPosture
	failureRef    string
}

func (failure AdmissionFailure) CommitPosture() AdmissionCommitPosture {
	return failure.commitPosture
}

func (failure AdmissionFailure) FailureRef() string {
	return failure.failureRef
}

type canonicalProfileAdmissionState struct {
	projectRoot                       projectprofile.ProjectRootV1
	payload                           projectprofile.ProfileDeclarationPayload
	admissionRecordRef                projectprofile.ProfileDeclarationAdmissionRecordRef
	admissionRecordDigest             projectprofile.ContentDigest
	admissionRecordCanonicalJSON      []byte
	receiptCanonicalJSON              []byte
	receiptDigest                     projectprofile.ContentDigest
	candidateProvenanceCanonicalJSON  []byte
	candidateProvenanceDigest         projectprofile.ContentDigest
	workRecordRef                     projectprofile.ProfileOnboardingWorkRecordRef
	workRecordDigest                  projectprofile.ContentDigest
	authorityBasisRef                 projectprofile.ProfileDeclarationAuthorityBasisRef
	authorityBasisDigest              projectprofile.ContentDigest
	authorityResolutionRef            projectprofile.AuthorityResolutionRecordRef
	authorityResolutionDigest         projectprofile.ContentDigest
	profileAuthorRoleAssignmentRef    projectprofile.RoleAssignmentRef
	profileAuthorRoleAssignmentDigest projectprofile.ContentDigest
	observedProjectBasisRef           projectprofile.ObservedProjectBasisRefV1
	observedProjectBasisDigest        projectprofile.ContentDigest
	outcomeAssessmentRef              projectprofile.ProfileOnboardingOutcomeAssessmentRefV1
	outcomeAssessmentDigest           projectprofile.ContentDigest
	expectedLedgerRevision            projectprofile.LedgerRevision
	ledgerRevision                    projectprofile.LedgerRevision
	recordedAt                        time.Time
	delivery                          CanonicalAdmissionDelivery
}

// CanonicalProfileAdmission is the sole effect-usable profile-admission token.
// A zero value is invalid. There is deliberately no builder, constructor, or
// unmarshal path: Service mints it only after one request-free strict durable
// reread has validated the admission, authority use, and support DAG.
type CanonicalProfileAdmission struct {
	state *canonicalProfileAdmissionState
}

func (admission CanonicalProfileAdmission) Valid() bool {
	return validateCanonicalProfileAdmission(admission) == nil
}

func (admission CanonicalProfileAdmission) ProjectRoot() projectprofile.ProjectRootV1 {
	if !admission.Valid() {
		return projectprofile.ProjectRootV1{}
	}
	return admission.state.projectRoot
}

func (admission CanonicalProfileAdmission) Payload() projectprofile.ProfileDeclarationPayload {
	if !admission.Valid() {
		return projectprofile.ProfileDeclarationPayload{}
	}
	return admission.state.payload
}

func (admission CanonicalProfileAdmission) PayloadDigest() projectprofile.ContentDigest {
	if !admission.Valid() {
		return projectprofile.ContentDigest{}
	}
	digest, err := projectprofile.DigestProfileDeclarationPayload(admission.state.payload)
	if err != nil {
		return projectprofile.ContentDigest{}
	}
	return digest
}

func (admission CanonicalProfileAdmission) AdmissionRecordRef() projectprofile.ProfileDeclarationAdmissionRecordRef {
	if !admission.Valid() {
		return projectprofile.ProfileDeclarationAdmissionRecordRef{}
	}
	return admission.state.admissionRecordRef
}

func (admission CanonicalProfileAdmission) AdmissionRecordDigest() projectprofile.ContentDigest {
	if !admission.Valid() {
		return projectprofile.ContentDigest{}
	}
	return admission.state.admissionRecordDigest
}

func (admission CanonicalProfileAdmission) AdmissionRecordCanonicalJSON() []byte {
	if !admission.Valid() {
		return nil
	}
	return append([]byte{}, admission.state.admissionRecordCanonicalJSON...)
}

func (admission CanonicalProfileAdmission) ReceiptCanonicalJSON() []byte {
	if !admission.Valid() {
		return nil
	}
	return append([]byte{}, admission.state.receiptCanonicalJSON...)
}

func (admission CanonicalProfileAdmission) ReceiptDigest() projectprofile.ContentDigest {
	if !admission.Valid() {
		return projectprofile.ContentDigest{}
	}
	return admission.state.receiptDigest
}

func (admission CanonicalProfileAdmission) CandidateProvenanceCanonicalJSON() []byte {
	if !admission.Valid() {
		return nil
	}
	return append([]byte{}, admission.state.candidateProvenanceCanonicalJSON...)
}

func (admission CanonicalProfileAdmission) CandidateProvenanceDigest() projectprofile.ContentDigest {
	if !admission.Valid() {
		return projectprofile.ContentDigest{}
	}
	return admission.state.candidateProvenanceDigest
}

func (admission CanonicalProfileAdmission) CandidateProvenance() projectprofile.CandidateProvenanceV1 {
	if !admission.Valid() {
		return projectprofile.CandidateProvenanceV1{}
	}
	candidate, err := newCandidateFromCanonicalParts(
		admission.state.payload,
		admission.state.candidateProvenanceCanonicalJSON,
	)
	if err != nil {
		return projectprofile.CandidateProvenanceV1{}
	}
	return candidate.Provenance()
}

func (admission CanonicalProfileAdmission) WorkRecordRef() projectprofile.ProfileOnboardingWorkRecordRef {
	if !admission.Valid() {
		return projectprofile.ProfileOnboardingWorkRecordRef{}
	}
	return admission.state.workRecordRef
}

func (admission CanonicalProfileAdmission) WorkRecordDigest() projectprofile.ContentDigest {
	if !admission.Valid() {
		return projectprofile.ContentDigest{}
	}
	return admission.state.workRecordDigest
}

func (admission CanonicalProfileAdmission) AuthorityBasisRef() projectprofile.ProfileDeclarationAuthorityBasisRef {
	if !admission.Valid() {
		return projectprofile.ProfileDeclarationAuthorityBasisRef{}
	}
	return admission.state.authorityBasisRef
}

func (admission CanonicalProfileAdmission) AuthorityBasisDigest() projectprofile.ContentDigest {
	if !admission.Valid() {
		return projectprofile.ContentDigest{}
	}
	return admission.state.authorityBasisDigest
}

func (admission CanonicalProfileAdmission) AuthorityResolutionRef() projectprofile.AuthorityResolutionRecordRef {
	if !admission.Valid() {
		return projectprofile.AuthorityResolutionRecordRef{}
	}
	return admission.state.authorityResolutionRef
}

func (admission CanonicalProfileAdmission) AuthorityResolutionDigest() projectprofile.ContentDigest {
	if !admission.Valid() {
		return projectprofile.ContentDigest{}
	}
	return admission.state.authorityResolutionDigest
}

func (admission CanonicalProfileAdmission) ProfileAuthorRoleAssignmentRef() projectprofile.RoleAssignmentRef {
	if !admission.Valid() {
		return projectprofile.RoleAssignmentRef{}
	}
	return admission.state.profileAuthorRoleAssignmentRef
}

func (admission CanonicalProfileAdmission) ProfileAuthorRoleAssignmentDigest() projectprofile.ContentDigest {
	if !admission.Valid() {
		return projectprofile.ContentDigest{}
	}
	return admission.state.profileAuthorRoleAssignmentDigest
}

func (admission CanonicalProfileAdmission) ObservedProjectBasisRef() projectprofile.ObservedProjectBasisRefV1 {
	if !admission.Valid() {
		return projectprofile.ObservedProjectBasisRefV1{}
	}
	return admission.state.observedProjectBasisRef
}

func (admission CanonicalProfileAdmission) ObservedProjectBasisDigest() projectprofile.ContentDigest {
	if !admission.Valid() {
		return projectprofile.ContentDigest{}
	}
	return admission.state.observedProjectBasisDigest
}

func (admission CanonicalProfileAdmission) OutcomeAssessmentRef() projectprofile.ProfileOnboardingOutcomeAssessmentRefV1 {
	if !admission.Valid() {
		return projectprofile.ProfileOnboardingOutcomeAssessmentRefV1{}
	}
	return admission.state.outcomeAssessmentRef
}

func (admission CanonicalProfileAdmission) OutcomeAssessmentDigest() projectprofile.ContentDigest {
	if !admission.Valid() {
		return projectprofile.ContentDigest{}
	}
	return admission.state.outcomeAssessmentDigest
}

func (admission CanonicalProfileAdmission) ExpectedLedgerRevision() projectprofile.LedgerRevision {
	if !admission.Valid() {
		return projectprofile.LedgerRevision{}
	}
	return admission.state.expectedLedgerRevision
}

func (admission CanonicalProfileAdmission) LedgerRevision() projectprofile.LedgerRevision {
	if !admission.Valid() {
		return projectprofile.LedgerRevision{}
	}
	return admission.state.ledgerRevision
}

func (admission CanonicalProfileAdmission) RecordedAt() time.Time {
	if !admission.Valid() {
		return time.Time{}
	}
	return admission.state.recordedAt
}

func (admission CanonicalProfileAdmission) Delivery() CanonicalAdmissionDelivery {
	if !admission.Valid() {
		return ""
	}
	return admission.state.delivery
}

// AdmissionResult is an opaque concrete sum. Exactly one accessor succeeds.
type AdmissionResult struct {
	kind      AdmissionResultKind
	admission CanonicalProfileAdmission
	denials   []AdmissionDenial
	failure   AdmissionFailure
}

func (result AdmissionResult) Kind() AdmissionResultKind {
	return result.kind
}

func (result AdmissionResult) Admission() (CanonicalProfileAdmission, bool) {
	valid := result.kind == AdmissionResultAdmitted && result.admission.Valid()
	if !valid {
		return CanonicalProfileAdmission{}, false
	}
	return result.admission, true
}

func (result AdmissionResult) Denials() ([]AdmissionDenial, bool) {
	valid := result.kind == AdmissionResultNotAdmitted && len(result.denials) > 0
	if !valid {
		return nil, false
	}
	return append([]AdmissionDenial{}, result.denials...), true
}

func (result AdmissionResult) Failure() (AdmissionFailure, bool) {
	valid := result.kind == AdmissionResultWriteFailed && result.failure.valid()
	if !valid {
		return AdmissionFailure{}, false
	}
	return result.failure, true
}

func newCanonicalProfileAdmission(
	material canonicalAdmissionMaterial,
	delivery CanonicalAdmissionDelivery,
) (CanonicalProfileAdmission, error) {
	if !delivery.valid() {
		return CanonicalProfileAdmission{}, fmt.Errorf("canonical admission delivery is invalid")
	}
	payload := material.candidate.Payload()
	state := canonicalProfileAdmissionState{
		projectRoot:                       material.projectRoot,
		payload:                           payload,
		admissionRecordRef:                material.admissionRef,
		admissionRecordDigest:             material.admissionDigest,
		admissionRecordCanonicalJSON:      append([]byte{}, material.admissionJSON...),
		receiptCanonicalJSON:              append([]byte{}, material.receiptJSON...),
		receiptDigest:                     material.receiptDigest,
		candidateProvenanceCanonicalJSON:  append([]byte{}, material.provenanceJSON...),
		candidateProvenanceDigest:         material.provenanceDigest,
		workRecordRef:                     material.workRecordRef,
		workRecordDigest:                  material.workRecordDigest,
		authorityBasisRef:                 material.authorityBasisRef,
		authorityBasisDigest:              material.authorityBasisDigest,
		authorityResolutionRef:            material.authorityResolutionRef,
		authorityResolutionDigest:         material.authorityResolutionDigest,
		profileAuthorRoleAssignmentRef:    material.profileAuthorRoleAssignmentRef,
		profileAuthorRoleAssignmentDigest: material.profileAuthorRoleAssignmentDigest,
		observedProjectBasisRef:           material.observedProjectBasisRef,
		observedProjectBasisDigest:        material.observedProjectBasisDigest,
		outcomeAssessmentRef:              material.outcomeAssessmentRef,
		outcomeAssessmentDigest:           material.outcomeAssessmentDigest,
		expectedLedgerRevision:            material.expectedLedgerRevision,
		ledgerRevision:                    material.ledgerRevision,
		recordedAt:                        material.recordedAt,
		delivery:                          delivery,
	}
	result := CanonicalProfileAdmission{state: &state}
	if err := validateCanonicalProfileAdmission(result); err != nil {
		return CanonicalProfileAdmission{}, err
	}
	return result, nil
}

func validateCanonicalProfileAdmission(admission CanonicalProfileAdmission) error {
	if admission.state == nil {
		return fmt.Errorf("canonical profile admission is absent")
	}
	state := admission.state
	candidate, err := newCandidateFromCanonicalParts(
		state.payload,
		state.candidateProvenanceCanonicalJSON,
	)
	if err != nil {
		return err
	}
	provenance := candidate.Provenance()
	projectRoot := provenance.ProjectRoot()
	if projectRoot != state.projectRoot {
		return fmt.Errorf("canonical profile admission has a mismatched project root")
	}
	payloadJSON, err := projectprofile.EncodeProfileDeclarationPayloadCanonicalJSON(state.payload)
	if err != nil {
		return err
	}
	material := canonicalAdmissionMaterial{
		projectRoot:                       state.projectRoot,
		candidate:                         candidate,
		payloadJSON:                       payloadJSON,
		provenanceJSON:                    state.candidateProvenanceCanonicalJSON,
		provenanceDigest:                  state.candidateProvenanceDigest,
		admissionRef:                      state.admissionRecordRef,
		admissionDigest:                   state.admissionRecordDigest,
		admissionJSON:                     state.admissionRecordCanonicalJSON,
		receiptJSON:                       state.receiptCanonicalJSON,
		receiptDigest:                     state.receiptDigest,
		workRecordRef:                     state.workRecordRef,
		workRecordDigest:                  state.workRecordDigest,
		authorityBasisRef:                 state.authorityBasisRef,
		authorityBasisDigest:              state.authorityBasisDigest,
		authorityResolutionRef:            state.authorityResolutionRef,
		authorityResolutionDigest:         state.authorityResolutionDigest,
		profileAuthorRoleAssignmentRef:    state.profileAuthorRoleAssignmentRef,
		profileAuthorRoleAssignmentDigest: state.profileAuthorRoleAssignmentDigest,
		observedProjectBasisRef:           state.observedProjectBasisRef,
		observedProjectBasisDigest:        state.observedProjectBasisDigest,
		outcomeAssessmentRef:              state.outcomeAssessmentRef,
		outcomeAssessmentDigest:           state.outcomeAssessmentDigest,
		expectedLedgerRevision:            state.expectedLedgerRevision,
		ledgerRevision:                    state.ledgerRevision,
		recordedAt:                        state.recordedAt,
	}
	if err := validateCanonicalAdmissionMaterial(material); err != nil {
		return err
	}
	if !state.delivery.valid() {
		return fmt.Errorf("canonical admission delivery is invalid")
	}
	return nil
}

func newCandidateFromCanonicalParts(
	payload projectprofile.ProfileDeclarationPayload,
	provenanceJSON []byte,
) (projectprofile.ProfileDeclarationCandidateV1, error) {
	payloadJSON, err := projectprofile.EncodeProfileDeclarationPayloadCanonicalJSON(payload)
	if err != nil {
		return projectprofile.ProfileDeclarationCandidateV1{}, err
	}
	return decodeCandidateFromCanonicalParts(payloadJSON, provenanceJSON)
}

func (delivery CanonicalAdmissionDelivery) valid() bool {
	return delivery == CanonicalAdmissionFresh ||
		delivery == CanonicalAdmissionReplayed ||
		delivery == CanonicalAdmissionRecovered ||
		delivery == CanonicalAdmissionResolvedAfterRestart
}

func (failure AdmissionFailure) valid() bool {
	postureValid := failure.commitPosture == AdmissionDefinitelyNotCommitted ||
		failure.commitPosture == AdmissionCommitOutcomeUnknown
	return postureValid && failure.failureRef != ""
}

func admissionSucceeded(admission CanonicalProfileAdmission) AdmissionResult {
	return AdmissionResult{
		kind:      AdmissionResultAdmitted,
		admission: admission,
	}
}

func admissionDenied(denials []AdmissionDenial) AdmissionResult {
	return AdmissionResult{
		kind:    AdmissionResultNotAdmitted,
		denials: append([]AdmissionDenial{}, denials...),
	}
}

func admissionFailed(
	posture AdmissionCommitPosture,
	stage effectFailureStage,
) AdmissionResult {
	return AdmissionResult{
		kind: AdmissionResultWriteFailed,
		failure: AdmissionFailure{
			commitPosture: posture,
			failureRef:    stage.failureRef(),
		},
	}
}

func equalCanonicalBytes(left []byte, right []byte) bool {
	return bytes.Equal(left, right)
}
