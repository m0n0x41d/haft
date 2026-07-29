package authority

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const loadHistoricalProfileAdmissionUseSQL = `SELECT
	u.use_id,
	u.authority_resolution_ref,
	u.authority_resolution_digest,
	u.single_use_key,
	u.action_kind,
	u.project_root,
	u.project_binding_digest,
	u.envelope_digest,
	u.authority_record_ref,
	u.authority_record_digest,
	u.admission_request_digest,
	u.verifier_identity,
	u.verifier_version,
	u.committed_result_ref,
	u.committed_result_digest,
	u.consumed_at,
	(
		SELECT COUNT(*)
		FROM authority_uses candidate
		WHERE candidate.committed_result_ref = ?
	)
FROM authority_uses u
WHERE u.committed_result_ref = ?
ORDER BY u.use_id
LIMIT 1`

type historicalProfileAdmissionAuthorityProofV1State struct {
	presentation canonicalPresentation
	resolution   canonicalAuthorityResolution
	recordedUse  ProfileAdmissionRecordedUse
}

// HistoricalProfileAdmissionAuthorityProofV1 is a request-free, read-only proof
// that one committed admission is linked to an exact canonical authority use,
// resolution, presentation, basis, and envelope. It is historical provenance,
// not reusable authority and not a current permission judgement.
type HistoricalProfileAdmissionAuthorityProofV1 struct {
	state *historicalProfileAdmissionAuthorityProofV1State
}

func (value HistoricalProfileAdmissionAuthorityProofV1) RecordedUse() (
	ProfileAdmissionRecordedUse,
	bool,
) {
	if !value.valid() {
		return ProfileAdmissionRecordedUse{}, false
	}
	return value.state.recordedUse, true
}

func (value HistoricalProfileAdmissionAuthorityProofV1) Presentation() (
	Presentation,
	bool,
) {
	if !value.valid() {
		return Presentation{}, false
	}
	return Presentation{value: value.state.presentation}, true
}

func (value HistoricalProfileAdmissionAuthorityProofV1) Envelope() (
	AuthorizationEnvelope,
	bool,
) {
	presentation, ok := value.Presentation()
	if !ok {
		return AuthorizationEnvelope{}, false
	}
	return presentation.Envelope(), true
}

func (value HistoricalProfileAdmissionAuthorityProofV1) AuthorityUseRecordRef() (
	AuthorityUseRecordRef,
	bool,
) {
	use, ok := value.RecordedUse()
	return use.useRef, ok
}

func (value HistoricalProfileAdmissionAuthorityProofV1) AuthorityResolutionID() (
	AuthorityResolutionID,
	bool,
) {
	if !value.valid() {
		return AuthorityResolutionID{}, false
	}
	return value.state.resolution.id, true
}

func (value HistoricalProfileAdmissionAuthorityProofV1) AuthorityResolutionDigest() (
	Digest,
	bool,
) {
	if !value.valid() {
		return Digest{}, false
	}
	return value.state.resolution.digest, true
}

func (value HistoricalProfileAdmissionAuthorityProofV1) AuthorityBasisRef() (
	PresentationID,
	bool,
) {
	if !value.valid() {
		return PresentationID{}, false
	}
	return value.state.presentation.id, true
}

func (value HistoricalProfileAdmissionAuthorityProofV1) AuthorityBasisDigest() (
	Digest,
	bool,
) {
	if !value.valid() {
		return Digest{}, false
	}
	return value.state.presentation.digest, true
}

func (value HistoricalProfileAdmissionAuthorityProofV1) VerificationPolicyRef() (
	VerificationPolicyRef,
	bool,
) {
	if !value.valid() {
		return VerificationPolicyRef{}, false
	}
	return value.state.resolution.verificationPolicyRef, true
}

func (value HistoricalProfileAdmissionAuthorityProofV1) VerificationPolicyDigest() (
	Digest,
	bool,
) {
	if !value.valid() {
		return Digest{}, false
	}
	return value.state.resolution.verificationPolicyDigest, true
}

func (value HistoricalProfileAdmissionAuthorityProofV1) ResolutionWindow() (
	TimeWindow,
	bool,
) {
	if !value.valid() {
		return TimeWindow{}, false
	}
	window := TimeWindow{
		from:  value.state.resolution.resolvedAt,
		until: value.state.resolution.validUntil,
	}
	if !window.valid() {
		return TimeWindow{}, false
	}
	return window, true
}

func (value HistoricalProfileAdmissionAuthorityProofV1) ActionKind() (ActionKind, bool) {
	use, ok := value.RecordedUse()
	return use.actionKind, ok
}

func (value HistoricalProfileAdmissionAuthorityProofV1) ProjectRoot() (ProjectRoot, bool) {
	use, ok := value.RecordedUse()
	return use.projectRoot, ok
}

func (value HistoricalProfileAdmissionAuthorityProofV1) ProjectBindingDigest() (Digest, bool) {
	use, ok := value.RecordedUse()
	return use.projectBindingDigest, ok
}

func (value HistoricalProfileAdmissionAuthorityProofV1) EnvelopeDigest() (Digest, bool) {
	use, ok := value.RecordedUse()
	return use.envelopeDigest, ok
}

func (value HistoricalProfileAdmissionAuthorityProofV1) SingleUseKey() (SingleUseKey, bool) {
	use, ok := value.RecordedUse()
	return use.singleUseKey, ok
}

func (value HistoricalProfileAdmissionAuthorityProofV1) AdmissionRequestDigest() (Digest, bool) {
	use, ok := value.RecordedUse()
	return use.admissionRequestDigest, ok
}

func (value HistoricalProfileAdmissionAuthorityProofV1) VerifierIdentity() (
	VerifierIdentity,
	bool,
) {
	if !value.valid() {
		return VerifierIdentity{}, false
	}
	return value.state.resolution.verifierIdentity, true
}

func (value HistoricalProfileAdmissionAuthorityProofV1) VerifierVersion() (
	VerifierVersion,
	bool,
) {
	if !value.valid() {
		return VerifierVersion{}, false
	}
	return value.state.resolution.verifierVersion, true
}

func (value HistoricalProfileAdmissionAuthorityProofV1) CommittedResultRef() (
	ProfileDeclarationAdmissionRecordRef,
	bool,
) {
	use, ok := value.RecordedUse()
	return use.committedResultRef, ok
}

func (value HistoricalProfileAdmissionAuthorityProofV1) CommittedResultDigest() (Digest, bool) {
	use, ok := value.RecordedUse()
	return use.committedResultDigest, ok
}

func (value HistoricalProfileAdmissionAuthorityProofV1) ConsumedAt() (time.Time, bool) {
	use, ok := value.RecordedUse()
	return use.consumedAt, ok
}

func (value HistoricalProfileAdmissionAuthorityProofV1) valid() bool {
	if value.state == nil {
		return false
	}
	presentation := value.state.presentation
	resolution := value.state.resolution
	recordedUse := value.state.recordedUse
	if presentation.envelope.actionKind.String() != profileDeclarationActionKind {
		return false
	}
	err := validateCanonicalPresentation(presentation)
	if err != nil {
		return false
	}
	err = validateCanonicalAuthorityResolution(resolution, presentation)
	if err != nil {
		return false
	}
	err = validateHistoricalProfileDeclarationRecordedUse(
		recordedUse,
		presentation,
		resolution,
	)
	return err == nil
}

// ResolveHistoricalProfileAdmissionAuthorityV1 resolves historical authority
// solely from the committed admission ref on one caller-owned SQLite snapshot.
// It validates the canonical stored chain and the original use-time bounds but
// deliberately does not re-judge permission validity at the current time.
func ResolveHistoricalProfileAdmissionAuthorityV1(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	admissionRef ProfileDeclarationAdmissionRecordRef,
) (HistoricalProfileAdmissionAuthorityProofV1, error) {
	if ctx == nil {
		return HistoricalProfileAdmissionAuthorityProofV1{}, fmt.Errorf(
			"historical profile-admission authority context is required",
		)
	}
	if err := transaction.RequireActive(); err != nil {
		return HistoricalProfileAdmissionAuthorityProofV1{}, fmt.Errorf(
			"historical profile-admission authority transaction is invalid: %w",
			err,
		)
	}
	if !admissionRef.valid() {
		return HistoricalProfileAdmissionAuthorityProofV1{}, fmt.Errorf(
			"historical profile-admission ref is invalid",
		)
	}
	recordedUse, err := loadHistoricalProfileAdmissionRecordedUse(
		ctx,
		transaction,
		admissionRef,
	)
	if err != nil {
		return HistoricalProfileAdmissionAuthorityProofV1{}, err
	}
	record, err := loadHistoricalAuthorityRecord(
		ctx,
		transaction,
		recordedUse,
	)
	if err != nil {
		return HistoricalProfileAdmissionAuthorityProofV1{}, err
	}
	presentation, resolution, err := parseAuthorityRecord(record)
	if err != nil {
		return HistoricalProfileAdmissionAuthorityProofV1{}, fmt.Errorf(
			"validate historical canonical authority chain: %w",
			err,
		)
	}
	state := historicalProfileAdmissionAuthorityProofV1State{
		presentation: presentation,
		resolution:   resolution,
		recordedUse:  recordedUse,
	}
	proof := HistoricalProfileAdmissionAuthorityProofV1{state: &state}
	if !proof.valid() {
		return HistoricalProfileAdmissionAuthorityProofV1{}, fmt.Errorf(
			"historical profile-admission authority chain is invalid",
		)
	}
	return proof, nil
}

func loadHistoricalProfileAdmissionRecordedUse(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	admissionRef ProfileDeclarationAdmissionRecordRef,
) (ProfileAdmissionRecordedUse, error) {
	raw := profileAdmissionRecordedUseRaw{}
	arguments := []any{
		admissionRef.String(),
		admissionRef.String(),
	}
	destinations := raw.scanTargets()
	err := transaction.ScanOne(
		ctx,
		loadHistoricalProfileAdmissionUseSQL,
		arguments,
		destinations,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ProfileAdmissionRecordedUse{}, fmt.Errorf(
			"historical authority use for admission %q: %w",
			admissionRef.String(),
			sql.ErrNoRows,
		)
	}
	if err != nil {
		return ProfileAdmissionRecordedUse{}, fmt.Errorf(
			"load historical authority use: %w",
			err,
		)
	}
	if raw.matchCount != 1 {
		return ProfileAdmissionRecordedUse{}, fmt.Errorf(
			"historical admission ref matches %d authority-use rows; expected exactly one",
			raw.matchCount,
		)
	}
	recordedUse, err := raw.parse()
	if err != nil {
		return ProfileAdmissionRecordedUse{}, fmt.Errorf(
			"parse historical authority use: %w",
			err,
		)
	}
	if recordedUse.committedResultRef != admissionRef {
		return ProfileAdmissionRecordedUse{}, fmt.Errorf(
			"historical authority use points to another committed admission",
		)
	}
	return recordedUse, nil
}

func loadHistoricalAuthorityRecord(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	recordedUse ProfileAdmissionRecordedUse,
) (authorityRecordRow, error) {
	record := authorityRecordRow{}
	arguments := []any{
		recordedUse.authorityRecordRef.String(),
		recordedUse.authorityResolutionRef.String(),
	}
	destinations := record.scanTargets()
	err := transaction.ScanOne(
		ctx,
		loadAuthorityRecordSQL,
		arguments,
		destinations,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return authorityRecordRow{}, fmt.Errorf(
			"historical authority resolution or presentation is missing: %w",
			sql.ErrNoRows,
		)
	}
	if err != nil {
		return authorityRecordRow{}, fmt.Errorf(
			"load historical authority resolution and presentation: %w",
			err,
		)
	}
	return record, nil
}
