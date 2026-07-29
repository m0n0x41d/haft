package specmigrationv2

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
)

type ApplyProjectRoot struct {
	value string
}

func NewApplyProjectRoot(raw string) (ApplyProjectRoot, error) {
	trimmed := strings.TrimSpace(raw)
	absolute := filepath.IsAbs(raw)
	cleaned := filepath.Clean(raw)
	if raw != trimmed || !absolute || cleaned != raw {
		return ApplyProjectRoot{}, fmt.Errorf("migration apply project root must be a canonical absolute path")
	}
	return ApplyProjectRoot{value: raw}, nil
}

func (root ApplyProjectRoot) String() string {
	return root.value
}

func (root ApplyProjectRoot) valid() bool {
	nonempty := root.value != ""
	absolute := filepath.IsAbs(root.value)
	cleaned := filepath.Clean(root.value)
	return nonempty && absolute && cleaned == root.value
}

type ApplyRequestInput struct {
	ProjectRoot          ApplyProjectRoot
	Structural           StructuralRequest
	ProfileApplicability profileadmissionsqlite.SoftwareSystemSpecMigrationRequired
	Review               AdmittedMigrationReview
	RequestedAt          time.Time
}

// ApplyRequest is mintable only by combining an exact canonical SQLite-origin
// Required capability, a fresh request-free ledger reread, and a separately
// package-admitted semantic review. Its fields remain private.
type ApplyRequest struct {
	projectRoot          ApplyProjectRoot
	structural           StructuralRequest
	analysis             structuralAnalysis
	profileApplicability profileadmissionsqlite.SoftwareSystemSpecMigrationRequired
	profileBinding       opaqueProfileBinding
	review               admittedMigrationReview
	requestedAt          time.Time
}

type RecoveryRequestInput struct {
	ProjectRoot ApplyProjectRoot
	Structural  StructuralRequest
}

// RecoveryRequest deliberately carries no profile token or raw profile
// binding. RecoverMigration rereads the journal, then asks the concrete SQLite
// service to reconstruct the journal-bound historical Required capability.
type RecoveryRequest struct {
	projectRoot ApplyProjectRoot
	structural  StructuralRequest
	analysis    structuralAnalysis
}

func NewRecoveryRequest(input RecoveryRequestInput) (RecoveryRequest, error) {
	if !input.ProjectRoot.valid() {
		return RecoveryRequest{}, fmt.Errorf("migration recovery project root is invalid")
	}
	structural, analysis, err := revalidateApplyStructure(input.Structural)
	if err != nil {
		return RecoveryRequest{}, err
	}
	if structural.projectRoot.String() != input.ProjectRoot.String() {
		return RecoveryRequest{}, fmt.Errorf("migration structural project root does not match the recovery root")
	}
	return RecoveryRequest{
		projectRoot: input.ProjectRoot,
		structural:  structural,
		analysis:    analysis,
	}, nil
}

// opaqueProfileBinding is mechanics-only journal material derived from the
// sealed canonical SQLite-origin applicability proof. Raw journal fields are
// replay metadata and cannot reopen the effect boundary by themselves.
type opaqueProfileBinding struct {
	ref            string
	digest         string
	ledgerRevision uint64
}

type ProfileApplicabilityPrecondition string

const (
	ProfileApplicabilityProofInvalid    ProfileApplicabilityPrecondition = "proof_invalid"
	ProfileApplicabilityNotCurrent      ProfileApplicabilityPrecondition = "not_current"
	ProfileApplicabilityNotApplicable   ProfileApplicabilityPrecondition = "not_applicable"
	ProfileApplicabilityUnderdetermined ProfileApplicabilityPrecondition = "underdetermined"
)

// ProfileApplicabilityPreconditionError keeps every fail-closed profile gate
// machine-detectable. It does not turn a retrieval or configuration value into
// an effect capability.
type ProfileApplicabilityPreconditionError struct {
	precondition ProfileApplicabilityPrecondition
}

func (failure ProfileApplicabilityPreconditionError) Error() string {
	return "migration apply profile-applicability precondition failed: " + string(failure.precondition)
}

func (failure ProfileApplicabilityPreconditionError) Precondition() ProfileApplicabilityPrecondition {
	return failure.precondition
}

// NewApplyRequest re-resolves the current canonical admission through the
// concrete SQLite service before consuming the supplied Required capability.
// No generic port can attest freshness or mint the proof.
func NewApplyRequest(
	ctx context.Context,
	profileService profileadmissionsqlite.Service,
	input ApplyRequestInput,
) (ApplyRequest, error) {
	request, err := newApplyRequestShape(input)
	if err != nil {
		return ApplyRequest{}, err
	}
	validation := profileService.ValidateCurrentSoftwareSystemSpecMigrationRequired(
		ctx,
		request.profileApplicability,
	)
	if err := profileValidationError(validation); err != nil {
		return ApplyRequest{}, err
	}
	return request, nil
}

func newApplyRequestShape(input ApplyRequestInput) (ApplyRequest, error) {
	if !input.ProjectRoot.valid() {
		return ApplyRequest{}, fmt.Errorf("migration apply project root is invalid")
	}
	structural, analysis, err := revalidateApplyStructure(input.Structural)
	if err != nil {
		return ApplyRequest{}, err
	}
	structuralRoot := structural.projectRoot.String()
	effectRoot := input.ProjectRoot.String()
	if structuralRoot != effectRoot {
		return ApplyRequest{}, fmt.Errorf("migration structural project root does not match the effect root")
	}
	profileBinding, err := profileBindingFromRequired(
		input.ProfileApplicability,
		input.ProjectRoot,
	)
	if err != nil {
		return ApplyRequest{}, err
	}
	review, err := exactAdmittedMigrationReview(input.Review)
	if err != nil {
		return ApplyRequest{}, err
	}
	if err := validateReviewAgainstAnalysis(review, analysis); err != nil {
		return ApplyRequest{}, err
	}
	if input.RequestedAt.IsZero() {
		return ApplyRequest{}, fmt.Errorf("migration apply request time is required")
	}
	return ApplyRequest{
		projectRoot:          input.ProjectRoot,
		structural:           structural,
		analysis:             analysis,
		profileApplicability: input.ProfileApplicability,
		profileBinding:       profileBinding,
		review:               review,
		requestedAt:          input.RequestedAt.UTC(),
	}, nil
}

func revalidateApplyStructure(
	request StructuralRequest,
) (StructuralRequest, structuralAnalysis, error) {
	validated, err := NewStructuralRequest(StructuralRequestInput{
		Packet:           request.packet,
		ProjectRoot:      request.projectRoot,
		Source:           request.source,
		Target:           request.target,
		TargetClaims:     request.targetClaims,
		OutsideSnapshots: request.outsideSnapshots,
	})
	if err != nil {
		return StructuralRequest{}, structuralAnalysis{}, err
	}
	result := AnalyzeStructure(validated)
	valid, ok := result.(validAnalysis)
	if !ok {
		return StructuralRequest{}, structuralAnalysis{}, fmt.Errorf("migration structural analysis is not valid")
	}
	analysis, ok := valid.analysis.(structuralAnalysis)
	if !ok {
		return StructuralRequest{}, structuralAnalysis{}, fmt.Errorf("migration structural analysis is not package-owned")
	}
	return validated, analysis, nil
}

func profileBindingFromRequired(
	required profileadmissionsqlite.SoftwareSystemSpecMigrationRequired,
	projectRoot ApplyProjectRoot,
) (opaqueProfileBinding, error) {
	if !required.Valid() {
		return opaqueProfileBinding{}, ProfileApplicabilityPreconditionError{
			precondition: ProfileApplicabilityProofInvalid,
		}
	}
	if required.ProjectRoot().String() != projectRoot.String() {
		return opaqueProfileBinding{}, ProfileApplicabilityPreconditionError{
			precondition: ProfileApplicabilityProofInvalid,
		}
	}
	return opaqueProfileBinding{
		ref:            required.AdmissionRecordRef().String(),
		digest:         required.AdmissionRecordDigest().String(),
		ledgerRevision: required.LedgerRevision().Value(),
	}, nil
}

func profileValidationError(
	validation profileadmissionsqlite.SoftwareSystemSpecMigrationProofValidationKind,
) error {
	switch validation {
	case profileadmissionsqlite.SoftwareSystemSpecMigrationProofValid:
		return nil
	case profileadmissionsqlite.SoftwareSystemSpecMigrationProofNotCurrent:
		return ProfileApplicabilityPreconditionError{
			precondition: ProfileApplicabilityNotCurrent,
		}
	case profileadmissionsqlite.SoftwareSystemSpecMigrationProofNotApplicable:
		return ProfileApplicabilityPreconditionError{
			precondition: ProfileApplicabilityNotApplicable,
		}
	case profileadmissionsqlite.SoftwareSystemSpecMigrationProofUnderdetermined:
		return ProfileApplicabilityPreconditionError{
			precondition: ProfileApplicabilityUnderdetermined,
		}
	default:
		return ProfileApplicabilityPreconditionError{
			precondition: ProfileApplicabilityProofInvalid,
		}
	}
}

func exactAdmittedMigrationReview(
	review AdmittedMigrationReview,
) (admittedMigrationReview, error) {
	value, ok := review.(admittedMigrationReview)
	if !ok {
		return admittedMigrationReview{}, fmt.Errorf("migration apply requires a package-admitted semantic-review result")
	}
	if err := validateAdmittedMigrationReview(value); err != nil {
		return admittedMigrationReview{}, err
	}
	return value, nil
}

func validateReviewAgainstAnalysis(
	review admittedMigrationReview,
	analysis structuralAnalysis,
) error {
	diagnostics := reviewAnalysisDiagnostics(review, analysis).Values()
	if len(diagnostics) == 0 {
		return nil
	}
	first := diagnostics[0]
	return fmt.Errorf("semantic review mismatch %s: %s", first.Code(), first.Detail())
}

type MigrationApplyResult interface {
	migrationApplyResultVariant()
}

type Applied interface {
	MigrationApplyResult
	Receipt() MigrationEffectReceipt
	ReceiptCarrier() MigrationEffectReceiptCarrier
	appliedVariant()
}

type applied struct {
	receipt        MigrationEffectReceipt
	receiptCarrier MigrationEffectReceiptCarrier
}

func (applied) migrationApplyResultVariant() {}
func (applied) appliedVariant()              {}

func (result applied) Receipt() MigrationEffectReceipt {
	return result.receipt
}

func (result applied) ReceiptCarrier() MigrationEffectReceiptCarrier {
	return result.receiptCarrier
}

type Replayed interface {
	MigrationApplyResult
	Receipt() MigrationEffectReceipt
	ReceiptCarrier() MigrationEffectReceiptCarrier
	replayedVariant()
}

type replayed struct {
	receipt        MigrationEffectReceipt
	receiptCarrier MigrationEffectReceiptCarrier
}

func (replayed) migrationApplyResultVariant() {}
func (replayed) replayedVariant()             {}

func (result replayed) Receipt() MigrationEffectReceipt {
	return result.receipt
}

func (result replayed) ReceiptCarrier() MigrationEffectReceiptCarrier {
	return result.receiptCarrier
}

type RecoveryRequired interface {
	MigrationApplyResult
	MigrationID() MigrationPacketID
	PacketDigest() PacketDigest
	Phase() JournalPhase
	Reason() string
	recoveryRequiredVariant()
}

type recoveryRequired struct {
	migrationID MigrationPacketID
	packet      PacketDigest
	phase       JournalPhase
	reason      string
}

func (recoveryRequired) migrationApplyResultVariant() {}
func (recoveryRequired) recoveryRequiredVariant()     {}

func (result recoveryRequired) MigrationID() MigrationPacketID {
	return result.migrationID
}

func (result recoveryRequired) PacketDigest() PacketDigest {
	return result.packet
}

func (result recoveryRequired) Phase() JournalPhase {
	return result.phase
}

func (result recoveryRequired) Reason() string {
	return result.reason
}

type ApplyRejected interface {
	MigrationApplyResult
	Code() ApplyRejectionCode
	Reason() string
	applyRejectedVariant()
}

type ApplyRejectionCode string

const (
	ApplyRejectionInvalidRequest                   ApplyRejectionCode = "invalid_request"
	ApplyRejectionProfileProofInvalid              ApplyRejectionCode = "profile_proof_invalid"
	ApplyRejectionProfileApplicabilityNotCurrent   ApplyRejectionCode = "profile_applicability_not_current"
	ApplyRejectionProfileNotApplicable             ApplyRejectionCode = "profile_not_applicable"
	ApplyRejectionProfileApplicabilityUndetermined ApplyRejectionCode = "profile_applicability_underdetermined"
)

type applyRejected struct {
	code   ApplyRejectionCode
	reason string
}

func (applyRejected) migrationApplyResultVariant() {}
func (applyRejected) applyRejectedVariant()        {}

func (result applyRejected) Code() ApplyRejectionCode {
	if result.code == "" {
		return ApplyRejectionInvalidRequest
	}
	return result.code
}

func (result applyRejected) Reason() string {
	return result.reason
}

type MigrationEffectReceipt struct {
	migrationID                   MigrationPacketID
	packetDigest                  PacketDigest
	sourceDigest                  SourceDigest
	targetDigest                  TargetDigest
	lineageDigest                 LineagePolicyDigest
	profileAdmissionRef           string
	profileAdmissionHash          string
	profileLedgerRevision         uint64
	semanticReviewRef             ReviewRef
	semanticReviewAdmissionDigest SHA256
	semanticReviewDigest          SHA256
	gitWitnessDigest              SHA256
	appliedAt                     time.Time
}

// MigrationEffectReceiptCarrierRef identifies the project-relative carrier
// that stores one verified MigrationEffectReceipt. It is deliberately distinct
// from the receipt object and is minted only by the effect shell after it has
// derived and confined the carrier beneath the exact project root.
type MigrationEffectReceiptCarrierRef struct {
	value string
}

func (ref MigrationEffectReceiptCarrierRef) String() string {
	return ref.value
}

func (ref MigrationEffectReceiptCarrierRef) valid() bool {
	return validCarrierID(ref.value)
}

// MigrationEffectReceiptCarrier binds the address of the durable receipt
// carrier to the digest of its exact canonical bytes. It is effect evidence,
// not the semantic receipt object itself.
type MigrationEffectReceiptCarrier struct {
	ref    MigrationEffectReceiptCarrierRef
	digest SHA256
}

func (carrier MigrationEffectReceiptCarrier) Ref() MigrationEffectReceiptCarrierRef {
	return carrier.ref
}

func (carrier MigrationEffectReceiptCarrier) Digest() SHA256 {
	return carrier.digest
}

func (carrier MigrationEffectReceiptCarrier) valid() bool {
	return carrier.ref.valid() && carrier.digest.valid()
}

func newMigrationEffectReceiptCarrier(
	root ApplyProjectRoot,
	absolutePath string,
	digest SHA256,
) (MigrationEffectReceiptCarrier, error) {
	if !root.valid() {
		return MigrationEffectReceiptCarrier{}, fmt.Errorf("migration receipt carrier project root is invalid")
	}
	relative, err := confinedRelativePath(root.String(), absolutePath)
	if err != nil {
		return MigrationEffectReceiptCarrier{}, err
	}
	refValue := filepath.ToSlash(relative)
	ref := MigrationEffectReceiptCarrierRef{value: refValue}
	carrier := MigrationEffectReceiptCarrier{ref: ref, digest: digest}
	if !carrier.valid() {
		return MigrationEffectReceiptCarrier{}, fmt.Errorf("migration receipt carrier binding is invalid")
	}
	return carrier, nil
}

func (receipt MigrationEffectReceipt) MigrationID() MigrationPacketID {
	return receipt.migrationID
}

func (receipt MigrationEffectReceipt) PacketDigest() PacketDigest {
	return receipt.packetDigest
}

func (receipt MigrationEffectReceipt) SourceDigest() SourceDigest {
	return receipt.sourceDigest
}

func (receipt MigrationEffectReceipt) TargetDigest() TargetDigest {
	return receipt.targetDigest
}

func (receipt MigrationEffectReceipt) LineageDigest() LineagePolicyDigest {
	return receipt.lineageDigest
}

func (receipt MigrationEffectReceipt) AppliedAt() time.Time {
	return receipt.appliedAt
}

// OpaqueProfileBindingRef is replay metadata, not independent proof of P0PA
// admission, SQLite origin, COMMIT, applicability, or authority.
func (receipt MigrationEffectReceipt) OpaqueProfileBindingRef() string {
	return receipt.profileAdmissionRef
}

func (receipt MigrationEffectReceipt) OpaqueProfileBindingDigest() string {
	return receipt.profileAdmissionHash
}

func (receipt MigrationEffectReceipt) OpaqueProfileLedgerRevision() uint64 {
	return receipt.profileLedgerRevision
}

func (receipt MigrationEffectReceipt) SemanticReviewRef() ReviewRef {
	return receipt.semanticReviewRef
}

func (receipt MigrationEffectReceipt) SemanticReviewDigest() SHA256 {
	return receipt.semanticReviewDigest
}

func (receipt MigrationEffectReceipt) SemanticReviewAdmissionDigest() SHA256 {
	return receipt.semanticReviewAdmissionDigest
}

func (receipt MigrationEffectReceipt) GitWitnessDigest() SHA256 {
	return receipt.gitWitnessDigest
}
