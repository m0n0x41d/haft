// Package legacyimporteffect defines the sealed transaction contract for
// preserving a verified legacyimport plan as opaque history. It depends on
// selected-head effect coordinates but cannot semantically admit legacy data
// or activate a ProjectTypeEnv.
package legacyimporteffect

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/legacyimport"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const ImportReceiptSchemaVersionV1 = "haft.legacy-import.receipt/v1"

const (
	importReceiptRefPrefix = "legacy-import-receipt:"
)

var (
	ErrImportReplayConflict              = errors.New("legacy import idempotency key conflicts with another plan")
	ErrImportReplayCorrupt               = errors.New("legacy import replay record is inconsistent")
	ErrSelectedProjectTypeEnvUnavailable = errors.New("legacy import requires an exact selected project TypeEnv")
	ErrOpaqueHistoryWrite                = errors.New("legacy opaque-history write failed")
)

type ImportIdempotencyKey struct {
	value string
}

func NewImportIdempotencyKey(raw string) (ImportIdempotencyKey, error) {
	if raw == "" {
		return ImportIdempotencyKey{}, fmt.Errorf("legacy import idempotency key is required")
	}
	if raw != strings.TrimSpace(raw) {
		return ImportIdempotencyKey{}, fmt.Errorf(
			"legacy import idempotency key must not have surrounding whitespace",
		)
	}
	if !utf8.ValidString(raw) {
		return ImportIdempotencyKey{}, fmt.Errorf(
			"legacy import idempotency key must be valid UTF-8",
		)
	}
	if strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return ImportIdempotencyKey{}, fmt.Errorf(
			"legacy import idempotency key must not contain control characters",
		)
	}
	return ImportIdempotencyKey{value: raw}, nil
}

func (key ImportIdempotencyKey) String() string { return key.value }

func (key ImportIdempotencyKey) valid() bool { return key.value != "" }

// SelectedProjectTypeEnvBasis is an exact coordinate envelope returned only by
// the storage-owned transaction port. It is not caller authority and its
// presence does not admit any legacy semantic claim. The concrete SQLite
// adapter must reconstruct and verify these coordinates from the selected
// ProjectTypeEnvHead, receipt, and closure inside the same transaction.
type SelectedProjectTypeEnvBasis struct {
	project             projectidentity.ProjectID
	headRef             projecttypeenvselection.ProjectTypeEnvHeadRef
	headRevision        projecttypeenvselection.HeadRevision
	typeEnvRef          typedmemory.TypeEnvRef
	graphRevision       typedmemory.GraphRevision
	selectionReceiptRef projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionReceiptRef
	selectionClosureRef projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureRef
}

type selectedProjectTypeEnvBasisInput struct {
	project             projectidentity.ProjectID
	headRef             projecttypeenvselection.ProjectTypeEnvHeadRef
	headRevision        projecttypeenvselection.HeadRevision
	typeEnvRef          typedmemory.TypeEnvRef
	graphRevision       typedmemory.GraphRevision
	selectionReceiptRef projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionReceiptRef
	selectionClosureRef projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureRef
}

func newSelectedProjectTypeEnvBasis(
	input selectedProjectTypeEnvBasisInput,
) (SelectedProjectTypeEnvBasis, error) {
	value := SelectedProjectTypeEnvBasis(input)
	if err := value.verifyForProject(input.project); err != nil {
		return SelectedProjectTypeEnvBasis{}, err
	}
	return value, nil
}

func (basis SelectedProjectTypeEnvBasis) ProjectID() projectidentity.ProjectID {
	return basis.project
}

func (basis SelectedProjectTypeEnvBasis) HeadRef() projecttypeenvselection.ProjectTypeEnvHeadRef {
	return basis.headRef
}

func (basis SelectedProjectTypeEnvBasis) HeadRevision() projecttypeenvselection.HeadRevision {
	return basis.headRevision
}

func (basis SelectedProjectTypeEnvBasis) TypeEnvRef() typedmemory.TypeEnvRef {
	return basis.typeEnvRef
}

func (basis SelectedProjectTypeEnvBasis) GraphRevision() typedmemory.GraphRevision {
	return basis.graphRevision
}

func (basis SelectedProjectTypeEnvBasis) SelectionReceiptRef() projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionReceiptRef {
	return basis.selectionReceiptRef
}

func (basis SelectedProjectTypeEnvBasis) SelectionClosureRef() projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureRef {
	return basis.selectionClosureRef
}

func (basis SelectedProjectTypeEnvBasis) verifyForProject(
	project projectidentity.ProjectID,
) error {
	if project.String() == "" || basis.project != project {
		return fmt.Errorf("selected project TypeEnv basis project does not match import project")
	}
	if basis.headRef.Project() != project {
		return fmt.Errorf(
			"selected project TypeEnv head project = %q, want %q",
			basis.headRef.Project().String(),
			project.String(),
		)
	}
	if basis.headRevision.Value() == 0 {
		return fmt.Errorf("selected project TypeEnv head revision must be greater than zero")
	}
	if _, err := typedmemory.ParseTypeEnvRef(basis.typeEnvRef.String()); err != nil {
		return fmt.Errorf("selected project TypeEnv reference: %w", err)
	}
	if basis.graphRevision.Value() == 0 {
		return fmt.Errorf("selected project TypeEnv graph revision must be greater than zero")
	}
	if basis.selectionReceiptRef.Digest().String() == "" {
		return fmt.Errorf("project TypeEnv selection receipt reference is required")
	}
	if basis.selectionClosureRef.Digest().String() == "" {
		return fmt.Errorf("project TypeEnv selection closure reference is required")
	}
	return nil
}

type ImportApplyRequest struct {
	plan legacyimport.ImportPlan
	key  ImportIdempotencyKey
}

func NewImportApplyRequest(
	plan legacyimport.ImportPlan,
	key ImportIdempotencyKey,
) (ImportApplyRequest, error) {
	if err := plan.Verify(); err != nil {
		return ImportApplyRequest{}, fmt.Errorf("legacy import apply request requires a valid import plan")
	}
	if !key.valid() {
		return ImportApplyRequest{}, fmt.Errorf("legacy import apply request requires an idempotency key")
	}
	return ImportApplyRequest{plan: plan, key: key}, nil
}

func (request ImportApplyRequest) Plan() legacyimport.ImportPlan { return request.plan }

func (request ImportApplyRequest) IdempotencyKey() ImportIdempotencyKey {
	return request.key
}

func (request ImportApplyRequest) valid() bool {
	return request.plan.Verify() == nil && request.key.valid()
}

type ImportApplyCoordinate struct {
	project    projectidentity.ProjectID
	key        ImportIdempotencyKey
	planDigest typedmemory.SHA256Digest
}

func coordinateOf(request ImportApplyRequest) ImportApplyCoordinate {
	return ImportApplyCoordinate{
		project:    request.plan.ProjectID(),
		key:        request.key,
		planDigest: request.plan.Digest(),
	}
}

func (coordinate ImportApplyCoordinate) ProjectID() projectidentity.ProjectID {
	return coordinate.project
}

func (coordinate ImportApplyCoordinate) IdempotencyKey() ImportIdempotencyKey {
	return coordinate.key
}

func (coordinate ImportApplyCoordinate) PlanDigest() typedmemory.SHA256Digest {
	return coordinate.planDigest
}

type ImportReceiptRef struct {
	digest typedmemory.SHA256Digest
}

func ParseImportReceiptRef(raw string) (ImportReceiptRef, error) {
	digestText, found := strings.CutPrefix(raw, importReceiptRefPrefix)
	if !found {
		return ImportReceiptRef{}, fmt.Errorf(
			"legacy import receipt reference must start with %q",
			importReceiptRefPrefix,
		)
	}
	digest, err := typedmemory.NewSHA256Digest(digestText)
	if err != nil {
		return ImportReceiptRef{}, fmt.Errorf("legacy import receipt reference: %w", err)
	}
	ref := ImportReceiptRef{digest: digest}
	if ref.String() != raw {
		return ImportReceiptRef{}, fmt.Errorf("legacy import receipt reference is not canonical")
	}
	return ref, nil
}

func (ref ImportReceiptRef) Digest() typedmemory.SHA256Digest { return ref.digest }

func (ref ImportReceiptRef) String() string {
	return importReceiptRefPrefix + ref.digest.String()
}

func digestImportReceiptBytes(value []byte) typedmemory.SHA256Digest {
	sum := sha256.Sum256(value)
	encoded := hex.EncodeToString(sum[:])
	digest, _ := typedmemory.NewSHA256Digest("sha256:" + encoded)
	return digest
}

type ImportReceipt struct {
	ref              ImportReceiptRef
	project          projectidentity.ProjectID
	key              ImportIdempotencyKey
	planDigest       typedmemory.SHA256Digest
	reportDigest     typedmemory.SHA256Digest
	sourceDigest     typedmemory.SHA256Digest
	selectedBasis    SelectedProjectTypeEnvBasis
	carrierCount     uint64
	dispositionCount uint64
	canonicalBytes   []byte
}

func newImportReceipt(
	request ImportApplyRequest,
	basis SelectedProjectTypeEnvBasis,
) (ImportReceipt, error) {
	if !request.valid() {
		return ImportReceipt{}, fmt.Errorf("legacy import receipt requires a valid request")
	}
	if err := basis.verifyForProject(request.plan.ProjectID()); err != nil {
		return ImportReceipt{}, err
	}
	carrierCount := uint64(len(request.plan.CarrierHistories()))
	dispositionCount := uint64(len(request.plan.SubjectDispositions()))
	body := importReceiptBodyDTO{
		SchemaVersion:           ImportReceiptSchemaVersionV1,
		Posture:                 legacyimport.ImportPlanPosture,
		ProjectID:               request.plan.ProjectID().String(),
		IdempotencyKey:          request.key.String(),
		ImportPlanDigest:        request.plan.Digest().String(),
		DryRunReportDigest:      request.plan.DryRunReportDigest().String(),
		SourceSnapshotDigest:    request.plan.SourceSnapshotDigest().String(),
		SelectedHeadRef:         basis.HeadRef().String(),
		SelectedHeadRevision:    basis.HeadRevision().Value(),
		SelectedTypeEnvRef:      basis.TypeEnvRef().String(),
		SelectedGraphRevision:   basis.GraphRevision().Value(),
		SelectionReceiptRef:     basis.SelectionReceiptRef().String(),
		SelectionClosureRef:     basis.SelectionClosureRef().String(),
		OpaqueCarrierCount:      carrierCount,
		SubjectDispositionCount: dispositionCount,
	}
	canonical, err := json.Marshal(body)
	if err != nil {
		return ImportReceipt{}, fmt.Errorf("encode legacy import receipt: %w", err)
	}
	digest := digestImportReceiptBytes(canonical)
	return ImportReceipt{
		ref:              ImportReceiptRef{digest: digest},
		project:          request.plan.ProjectID(),
		key:              request.key,
		planDigest:       request.plan.Digest(),
		reportDigest:     request.plan.DryRunReportDigest(),
		sourceDigest:     request.plan.SourceSnapshotDigest(),
		selectedBasis:    basis,
		carrierCount:     carrierCount,
		dispositionCount: dispositionCount,
		canonicalBytes:   canonical,
	}, nil
}

func (receipt ImportReceipt) Ref() ImportReceiptRef { return receipt.ref }

func (receipt ImportReceipt) Posture() string {
	return legacyimport.ImportPlanPosture
}

func (receipt ImportReceipt) ProjectID() projectidentity.ProjectID {
	return receipt.project
}

func (receipt ImportReceipt) IdempotencyKey() ImportIdempotencyKey {
	return receipt.key
}

func (receipt ImportReceipt) ImportPlanDigest() typedmemory.SHA256Digest {
	return receipt.planDigest
}

func (receipt ImportReceipt) DryRunReportDigest() typedmemory.SHA256Digest {
	return receipt.reportDigest
}

func (receipt ImportReceipt) SourceSnapshotDigest() typedmemory.SHA256Digest {
	return receipt.sourceDigest
}

func (receipt ImportReceipt) SelectedProjectTypeEnv() SelectedProjectTypeEnvBasis {
	return receipt.selectedBasis
}

func (receipt ImportReceipt) OpaqueCarrierCount() uint64 {
	return receipt.carrierCount
}

func (receipt ImportReceipt) SubjectDispositionCount() uint64 {
	return receipt.dispositionCount
}

func (receipt ImportReceipt) CanonicalBytes() []byte {
	return append([]byte(nil), receipt.canonicalBytes...)
}

func DecodeImportReceipt(canonical []byte) (ImportReceipt, error) {
	body, err := decodeImportReceiptBody(canonical)
	if err != nil {
		return ImportReceipt{}, err
	}
	if body.SchemaVersion != ImportReceiptSchemaVersionV1 {
		return ImportReceipt{}, fmt.Errorf(
			"legacy import receipt schema = %q, want %q",
			body.SchemaVersion,
			ImportReceiptSchemaVersionV1,
		)
	}
	if body.Posture != legacyimport.ImportPlanPosture {
		return ImportReceipt{}, fmt.Errorf(
			"legacy import receipt posture = %q, want %q",
			body.Posture,
			legacyimport.ImportPlanPosture,
		)
	}
	project, err := projectidentity.ParseProjectID(body.ProjectID)
	if err != nil {
		return ImportReceipt{}, fmt.Errorf("legacy import receipt project: %w", err)
	}
	key, err := NewImportIdempotencyKey(body.IdempotencyKey)
	if err != nil {
		return ImportReceipt{}, fmt.Errorf("legacy import receipt idempotency key: %w", err)
	}
	planDigest, err := typedmemory.NewSHA256Digest(body.ImportPlanDigest)
	if err != nil {
		return ImportReceipt{}, fmt.Errorf("legacy import receipt plan digest: %w", err)
	}
	reportDigest, err := typedmemory.NewSHA256Digest(body.DryRunReportDigest)
	if err != nil {
		return ImportReceipt{}, fmt.Errorf("legacy import receipt report digest: %w", err)
	}
	sourceDigest, err := typedmemory.NewSHA256Digest(body.SourceSnapshotDigest)
	if err != nil {
		return ImportReceipt{}, fmt.Errorf("legacy import receipt source digest: %w", err)
	}
	typeEnvRef, err := typedmemory.ParseTypeEnvRef(body.SelectedTypeEnvRef)
	if err != nil {
		return ImportReceipt{}, fmt.Errorf("legacy import receipt selected TypeEnv: %w", err)
	}
	headRef, err := projecttypeenvselection.ParseProjectTypeEnvHeadRef(
		body.SelectedHeadRef,
	)
	if err != nil {
		return ImportReceipt{}, fmt.Errorf("legacy import receipt selected head: %w", err)
	}
	headRevision, err := projecttypeenvselection.NewHeadRevision(
		body.SelectedHeadRevision,
	)
	if err != nil {
		return ImportReceipt{}, fmt.Errorf("legacy import receipt selected head revision: %w", err)
	}
	selectionReceiptRef, err := projecttypeenvselectioneffect.ParseProjectTypeEnvHeadSelectionReceiptRef(
		body.SelectionReceiptRef,
	)
	if err != nil {
		return ImportReceipt{}, fmt.Errorf("legacy import receipt selection receipt: %w", err)
	}
	selectionClosureRef, err := projecttypeenvselectioneffect.ParseProjectTypeEnvHeadSelectionClosureRef(
		body.SelectionClosureRef,
	)
	if err != nil {
		return ImportReceipt{}, fmt.Errorf("legacy import receipt selection closure: %w", err)
	}
	basis, err := newSelectedProjectTypeEnvBasis(
		selectedProjectTypeEnvBasisInput{
			project:             project,
			headRef:             headRef,
			headRevision:        headRevision,
			typeEnvRef:          typeEnvRef,
			graphRevision:       typedmemory.NewGraphRevision(body.SelectedGraphRevision),
			selectionReceiptRef: selectionReceiptRef,
			selectionClosureRef: selectionClosureRef,
		},
	)
	if err != nil {
		return ImportReceipt{}, fmt.Errorf("legacy import receipt selected basis: %w", err)
	}
	owned := append([]byte(nil), canonical...)
	digest := digestImportReceiptBytes(owned)
	receipt := ImportReceipt{
		ref:              ImportReceiptRef{digest: digest},
		project:          project,
		key:              key,
		planDigest:       planDigest,
		reportDigest:     reportDigest,
		sourceDigest:     sourceDigest,
		selectedBasis:    basis,
		carrierCount:     body.OpaqueCarrierCount,
		dispositionCount: body.SubjectDispositionCount,
		canonicalBytes:   owned,
	}
	if err := receipt.verifyCanonicalFields(); err != nil {
		return ImportReceipt{}, err
	}
	return receipt, nil
}

func (receipt ImportReceipt) verifyForRequest(request ImportApplyRequest) error {
	if err := receipt.verifyCanonicalFields(); err != nil {
		return fmt.Errorf("%w: %v", ErrImportReplayCorrupt, err)
	}
	expected, err := newImportReceipt(request, receipt.selectedBasis)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrImportReplayCorrupt, err)
	}
	if expected.ref != receipt.ref {
		return fmt.Errorf("%w: receipt reference differs from canonical request", ErrImportReplayCorrupt)
	}
	if string(expected.canonicalBytes) != string(receipt.canonicalBytes) {
		return fmt.Errorf("%w: receipt bytes differ from canonical request", ErrImportReplayCorrupt)
	}
	if expected.project != receipt.project ||
		expected.key != receipt.key ||
		expected.planDigest != receipt.planDigest ||
		expected.reportDigest != receipt.reportDigest ||
		expected.sourceDigest != receipt.sourceDigest ||
		expected.selectedBasis != receipt.selectedBasis ||
		expected.carrierCount != receipt.carrierCount ||
		expected.dispositionCount != receipt.dispositionCount {
		return fmt.Errorf("%w: receipt fields differ from canonical request", ErrImportReplayCorrupt)
	}
	return nil
}

func (receipt ImportReceipt) verifyCanonicalFields() error {
	if receipt.project.String() == "" || !receipt.key.valid() {
		return fmt.Errorf("legacy import receipt identity is incomplete")
	}
	if receipt.planDigest.String() == "" ||
		receipt.reportDigest.String() == "" ||
		receipt.sourceDigest.String() == "" {
		return fmt.Errorf("legacy import receipt provenance digests are incomplete")
	}
	if err := receipt.selectedBasis.verifyForProject(receipt.project); err != nil {
		return err
	}
	if receipt.carrierCount == 0 || receipt.dispositionCount == 0 {
		return fmt.Errorf("legacy import receipt must preserve carriers and dispositions")
	}
	body := importReceiptBodyDTO{
		SchemaVersion:           ImportReceiptSchemaVersionV1,
		Posture:                 legacyimport.ImportPlanPosture,
		ProjectID:               receipt.project.String(),
		IdempotencyKey:          receipt.key.String(),
		ImportPlanDigest:        receipt.planDigest.String(),
		DryRunReportDigest:      receipt.reportDigest.String(),
		SourceSnapshotDigest:    receipt.sourceDigest.String(),
		SelectedHeadRef:         receipt.selectedBasis.HeadRef().String(),
		SelectedHeadRevision:    receipt.selectedBasis.HeadRevision().Value(),
		SelectedTypeEnvRef:      receipt.selectedBasis.TypeEnvRef().String(),
		SelectedGraphRevision:   receipt.selectedBasis.GraphRevision().Value(),
		SelectionReceiptRef:     receipt.selectedBasis.SelectionReceiptRef().String(),
		SelectionClosureRef:     receipt.selectedBasis.SelectionClosureRef().String(),
		OpaqueCarrierCount:      receipt.carrierCount,
		SubjectDispositionCount: receipt.dispositionCount,
	}
	expected, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode legacy import receipt verification body: %w", err)
	}
	if !bytes.Equal(expected, receipt.canonicalBytes) {
		return fmt.Errorf("legacy import receipt is not canonical")
	}
	if receipt.ref.digest != digestImportReceiptBytes(receipt.canonicalBytes) {
		return fmt.Errorf("legacy import receipt reference differs from canonical bytes")
	}
	return nil
}

func decodeImportReceiptBody(
	canonical []byte,
) (importReceiptBodyDTO, error) {
	if len(canonical) == 0 {
		return importReceiptBodyDTO{}, fmt.Errorf("legacy import receipt bytes are required")
	}
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.DisallowUnknownFields()
	var body importReceiptBodyDTO
	if err := decoder.Decode(&body); err != nil {
		return importReceiptBodyDTO{}, fmt.Errorf("decode legacy import receipt: %w", err)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if !errors.Is(err, io.EOF) {
		return importReceiptBodyDTO{}, fmt.Errorf("legacy import receipt has trailing JSON")
	}
	reencoded, err := json.Marshal(body)
	if err != nil {
		return importReceiptBodyDTO{}, fmt.Errorf("re-encode legacy import receipt: %w", err)
	}
	if !bytes.Equal(reencoded, canonical) {
		return importReceiptBodyDTO{}, fmt.Errorf("legacy import receipt is not canonical")
	}
	return body, nil
}

type importReceiptBodyDTO struct {
	SchemaVersion           string `json:"schema_version"`
	Posture                 string `json:"posture"`
	ProjectID               string `json:"project_id"`
	IdempotencyKey          string `json:"idempotency_key"`
	ImportPlanDigest        string `json:"import_plan_digest"`
	DryRunReportDigest      string `json:"dry_run_report_digest"`
	SourceSnapshotDigest    string `json:"source_snapshot_digest"`
	SelectedHeadRef         string `json:"selected_head_ref"`
	SelectedHeadRevision    uint64 `json:"selected_head_revision"`
	SelectedTypeEnvRef      string `json:"selected_typeenv_ref"`
	SelectedGraphRevision   uint64 `json:"selected_graph_revision"`
	SelectionReceiptRef     string `json:"selection_receipt_ref"`
	SelectionClosureRef     string `json:"selection_closure_ref"`
	OpaqueCarrierCount      uint64 `json:"opaque_carrier_count"`
	SubjectDispositionCount uint64 `json:"subject_disposition_count"`
}

type OpaqueImportBatch struct {
	plan    legacyimport.ImportPlan
	basis   SelectedProjectTypeEnvBasis
	receipt ImportReceipt
}

func newOpaqueImportBatch(
	plan legacyimport.ImportPlan,
	basis SelectedProjectTypeEnvBasis,
	receipt ImportReceipt,
) (OpaqueImportBatch, error) {
	request, err := NewImportApplyRequest(plan, receipt.IdempotencyKey())
	if err != nil {
		return OpaqueImportBatch{}, err
	}
	if err := receipt.verifyForRequest(request); err != nil {
		return OpaqueImportBatch{}, err
	}
	if err := basis.verifyForProject(plan.ProjectID()); err != nil {
		return OpaqueImportBatch{}, err
	}
	if receipt.SelectedProjectTypeEnv() != basis {
		return OpaqueImportBatch{}, fmt.Errorf("legacy import batch selected TypeEnv differs from receipt")
	}
	return OpaqueImportBatch{
		plan:    plan,
		basis:   basis,
		receipt: receipt,
	}, nil
}

func (batch OpaqueImportBatch) Plan() legacyimport.ImportPlan {
	return batch.plan
}

func (batch OpaqueImportBatch) SelectedProjectTypeEnv() SelectedProjectTypeEnvBasis {
	return batch.basis
}

func (batch OpaqueImportBatch) Receipt() ImportReceipt { return batch.receipt }

// ImportReplayProbe is a closed transaction-local sum. The storage adapter
// probes it before resolving current-head state so exact historical replay does
// not depend on the current project head.
type ImportReplayProbe interface {
	importReplayProbeVariant()
}

type ImportReplayAbsent struct{}

func (ImportReplayAbsent) importReplayProbeVariant() {}

type ImportReplayExact struct {
	trustedReceiptRef ImportReceiptRef
	receipt           ImportReceipt
}

func newImportReplayExact(
	trustedReceiptRef ImportReceiptRef,
	receipt ImportReceipt,
) (ImportReplayExact, error) {
	replay := ImportReplayExact{
		trustedReceiptRef: trustedReceiptRef,
		receipt:           receipt,
	}
	if err := replay.verifyTrustedReceiptRef(); err != nil {
		return ImportReplayExact{}, err
	}
	return replay, nil
}

func (ImportReplayExact) importReplayProbeVariant() {}

func (replay ImportReplayExact) Receipt() ImportReceipt { return replay.receipt }

func (replay ImportReplayExact) verifyTrustedReceiptRef() error {
	if replay.trustedReceiptRef != replay.receipt.Ref() {
		return fmt.Errorf(
			"%w: persisted receipt reference differs from canonical receipt bytes",
			ErrImportReplayCorrupt,
		)
	}
	return nil
}

type ImportReplayConflict struct {
	existingPlanDigest typedmemory.SHA256Digest
}

func newImportReplayConflict(
	existingPlanDigest typedmemory.SHA256Digest,
) ImportReplayConflict {
	return ImportReplayConflict{existingPlanDigest: existingPlanDigest}
}

func (ImportReplayConflict) importReplayProbeVariant() {}

func (conflict ImportReplayConflict) ExistingPlanDigest() typedmemory.SHA256Digest {
	return conflict.existingPlanDigest
}

// ImportApplyStore owns the transaction boundary. It is sealed so product
// packages cannot supply an ad-hoc implementation that treats caller data as a
// selected-head proof. A future SQLite adapter must live in this package.
//
// RunImportTransaction has one observable commit boundary: a non-nil return
// guarantees that no opaque import batch or receipt from operation became
// durable. Once the transaction commits, it must return nil even if its context
// is cancelled after that commit. This keeps Apply's result aligned with
// durable state and makes retry semantics unambiguous.
type ImportApplyStore interface {
	RunImportTransaction(
		ctx context.Context,
		operation func(ImportApplyTransaction) error,
	) error
	legacyImportApplyStore()
}

// ImportApplyTransaction is intentionally narrower than typed-memory CommitPort.
// AppendOpaqueImport may preserve only the batch's exact opaque carrier history,
// dispositions, and receipt. It may not admit MemberOf, relation signatures,
// scope, authority, or Work.
type ImportApplyTransaction interface {
	ProbeImportReplay(
		ctx context.Context,
		coordinate ImportApplyCoordinate,
	) (ImportReplayProbe, error)
	// ResolveSelectedProjectTypeEnv may only read and verify a pre-existing
	// selected head, receipt, and closure in the current transaction. It must
	// not create, move, select, activate, or otherwise mutate a project head.
	ResolveSelectedProjectTypeEnv(
		ctx context.Context,
		project projectidentity.ProjectID,
	) (SelectedProjectTypeEnvBasis, error)
	AppendOpaqueImport(
		ctx context.Context,
		batch OpaqueImportBatch,
	) error
	legacyImportApplyTransaction()
}

type ImportApplyResult interface {
	Receipt() ImportReceipt
	importApplyResultVariant()
}

// ImportApplied means only that the exact opaque-history batch and its receipt
// were appended by the transaction port. It is not semantic admission.
type ImportApplied struct {
	receipt ImportReceipt
}

func (ImportApplied) importApplyResultVariant() {}

func (result ImportApplied) Receipt() ImportReceipt { return result.receipt }

// ImportReplayed returns an already committed opaque-history receipt without
// re-reading current head state or appending another batch.
type ImportReplayed struct {
	receipt ImportReceipt
}

func (ImportReplayed) importApplyResultVariant() {}

func (result ImportReplayed) Receipt() ImportReceipt { return result.receipt }

type ApplyService struct{}

func NewApplyService() ApplyService { return ApplyService{} }

func (ApplyService) Apply(
	ctx context.Context,
	store ImportApplyStore,
	request ImportApplyRequest,
) (ImportApplyResult, error) {
	if store == nil {
		return nil, fmt.Errorf("legacy import apply store is required")
	}
	if !request.valid() {
		return nil, fmt.Errorf("legacy import apply request is invalid")
	}
	var result ImportApplyResult
	err := store.RunImportTransaction(
		ctx,
		func(transaction ImportApplyTransaction) error {
			resolved, resolveErr := applyInTransaction(ctx, transaction, request)
			if resolveErr != nil {
				return resolveErr
			}
			result = resolved
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("legacy import transaction returned no result")
	}
	return result, nil
}

func applyInTransaction(
	ctx context.Context,
	transaction ImportApplyTransaction,
	request ImportApplyRequest,
) (ImportApplyResult, error) {
	if transaction == nil {
		return nil, fmt.Errorf("legacy import apply transaction is required")
	}
	probe, err := transaction.ProbeImportReplay(
		ctx,
		coordinateOf(request),
	)
	if err != nil {
		return nil, fmt.Errorf("probe legacy import replay: %w", err)
	}
	switch observed := probe.(type) {
	case ImportReplayExact:
		if err := observed.verifyTrustedReceiptRef(); err != nil {
			return nil, err
		}
		if err := observed.receipt.verifyForRequest(request); err != nil {
			return nil, err
		}
		return ImportReplayed{receipt: observed.receipt}, nil
	case ImportReplayConflict:
		return nil, fmt.Errorf(
			"%w: existing plan %s, requested plan %s",
			ErrImportReplayConflict,
			observed.existingPlanDigest.String(),
			request.plan.Digest().String(),
		)
	case ImportReplayAbsent:
		return applyAbsentImport(ctx, transaction, request)
	default:
		return nil, fmt.Errorf("%w: unsupported replay probe", ErrImportReplayCorrupt)
	}
}

func applyAbsentImport(
	ctx context.Context,
	transaction ImportApplyTransaction,
	request ImportApplyRequest,
) (ImportApplyResult, error) {
	basis, err := transaction.ResolveSelectedProjectTypeEnv(
		ctx,
		request.plan.ProjectID(),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSelectedProjectTypeEnvUnavailable, err)
	}
	if err := basis.verifyForProject(request.plan.ProjectID()); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSelectedProjectTypeEnvUnavailable, err)
	}
	receipt, err := newImportReceipt(request, basis)
	if err != nil {
		return nil, err
	}
	batch, err := newOpaqueImportBatch(request.plan, basis, receipt)
	if err != nil {
		return nil, err
	}
	if err := transaction.AppendOpaqueImport(ctx, batch); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOpaqueHistoryWrite, err)
	}
	return ImportApplied{receipt: receipt}, nil
}
