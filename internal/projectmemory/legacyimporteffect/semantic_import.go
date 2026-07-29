package legacyimporteffect

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/legacydualread"
	"github.com/m0n0x41d/haft/internal/projectmemory/legacyimport"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemoryvalidation"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

const SemanticImportRecordSchemaVersionV1 = "haft.legacy-semantic-import/v1"

var (
	ErrSemanticImportRequestInvalid = errors.New(
		"legacy semantic-import request is invalid",
	)
	ErrSemanticImportConflict = errors.New(
		"legacy semantic-import coordinate conflicts with stored state",
	)
	ErrSemanticImportStore = errors.New(
		"legacy semantic-import durable marker failed",
	)
)

type SemanticImportRequest struct {
	opaqueRequest ImportApplyRequest
	selector      typedmemorywire.BasisSelector
	candidate     typedmemory.MemoryChangeSet
	bridges       []legacydualread.IdentityBridge
	typedKey      typedmemorystore.IdempotencyKey
	provenance    typedmemory.ProvenanceRef
}

type SemanticImportRequestInput struct {
	OpaqueRequest ImportApplyRequest
	Selector      typedmemorywire.BasisSelector
	Candidate     typedmemory.MemoryChangeSet
	Bridges       []legacydualread.IdentityBridge
	TypedKey      typedmemorystore.IdempotencyKey
	Provenance    typedmemory.ProvenanceRef
}

func NewSemanticImportRequest(
	input SemanticImportRequestInput,
) (SemanticImportRequest, error) {
	if !input.OpaqueRequest.valid() {
		return SemanticImportRequest{}, ErrSemanticImportRequestInvalid
	}
	switch input.Selector.(type) {
	case typedmemorywire.ProjectCurrentSelector:
	case typedmemorywire.ExactProjectSelector:
	default:
		return SemanticImportRequest{}, fmt.Errorf(
			"%w: project-current or exact-project basis is required",
			ErrSemanticImportRequestInvalid,
		)
	}
	if len(input.Candidate.Changes()) == 0 {
		return SemanticImportRequest{}, fmt.Errorf(
			"%w: typed candidate is required",
			ErrSemanticImportRequestInvalid,
		)
	}
	if _, err := input.Candidate.Digest(); err != nil {
		return SemanticImportRequest{}, fmt.Errorf(
			"%w: candidate: %v",
			ErrSemanticImportRequestInvalid,
			err,
		)
	}
	if _, err := typedmemorystore.NewIdempotencyKey(
		input.TypedKey.String(),
	); err != nil {
		return SemanticImportRequest{}, fmt.Errorf(
			"%w: typed idempotency key",
			ErrSemanticImportRequestInvalid,
		)
	}
	if _, err := typedmemory.NewProvenanceRef(
		input.Provenance.String(),
	); err != nil {
		return SemanticImportRequest{}, fmt.Errorf(
			"%w: request provenance",
			ErrSemanticImportRequestInvalid,
		)
	}
	bridges, err := validateSemanticImportBridges(
		input.OpaqueRequest.Plan(),
		input.Bridges,
	)
	if err != nil {
		return SemanticImportRequest{}, err
	}
	return SemanticImportRequest{
		opaqueRequest: input.OpaqueRequest,
		selector:      input.Selector,
		candidate:     input.Candidate,
		bridges:       bridges,
		typedKey:      input.TypedKey,
		provenance:    input.Provenance,
	}, nil
}

func (request SemanticImportRequest) OpaqueRequest() ImportApplyRequest {
	return request.opaqueRequest
}

func (request SemanticImportRequest) Bridges() []legacydualread.IdentityBridge {
	return append([]legacydualread.IdentityBridge(nil), request.bridges...)
}

type PreparedSemanticImport struct {
	request SemanticImportRequest
	valid   typedmemoryvalidation.ValidOutcome
}

func PrepareSemanticImport(
	ctx context.Context,
	admission projectmemory.AdmissionRuntime,
	request SemanticImportRequest,
) (PreparedSemanticImport, error) {
	if err := verifySemanticImportRequest(request); err != nil {
		return PreparedSemanticImport{}, err
	}
	if admission.ProjectID().String() !=
		request.opaqueRequest.Plan().ProjectID().String() {
		return PreparedSemanticImport{}, fmt.Errorf(
			"%w: admission runtime belongs to another project",
			ErrSemanticImportRequestInvalid,
		)
	}
	valid, err := admission.PrepareCandidate(
		ctx,
		request.selector,
		request.candidate,
	)
	if err != nil {
		return PreparedSemanticImport{}, err
	}
	prepared := PreparedSemanticImport{
		request: request,
		valid:   valid,
	}
	if err := verifyPreparedSemanticImport(prepared); err != nil {
		return PreparedSemanticImport{}, err
	}
	return prepared, nil
}

type SemanticImportResult struct {
	opaque       ImportApplyResult
	typedReceipt typedmemorystore.CommitReceipt
	record       SemanticImportRecord
}

func (result SemanticImportResult) OpaqueImport() ImportApplyResult {
	return result.opaque
}

func (result SemanticImportResult) TypedReceipt() typedmemorystore.CommitReceipt {
	return result.typedReceipt
}

func (result SemanticImportResult) Record() SemanticImportRecord {
	return result.record
}

type SemanticImportService struct{}

func NewSemanticImportService() SemanticImportService {
	return SemanticImportService{}
}

// Apply composes the existing opaque-history effect and the existing
// projectmemory.AdmissionRuntime. It does not implement a second semantic
// validator or commit path. The three monotonic effects are exact-replayable:
// a retry can finish the durable semantic marker after an already-committed
// typed admission without duplicating either prior effect.
func (SemanticImportService) Apply(
	ctx context.Context,
	store *SQLiteStore,
	admission projectmemory.AdmissionRuntime,
	prepared PreparedSemanticImport,
) (SemanticImportResult, error) {
	if store == nil {
		return SemanticImportResult{}, fmt.Errorf(
			"%w: SQLite store is required",
			ErrSemanticImportStore,
		)
	}
	if err := verifyPreparedSemanticImport(prepared); err != nil {
		return SemanticImportResult{}, err
	}
	if admission.ProjectID().String() !=
		prepared.request.opaqueRequest.Plan().ProjectID().String() {
		return SemanticImportResult{}, fmt.Errorf(
			"%w: admission runtime belongs to another project",
			ErrSemanticImportRequestInvalid,
		)
	}
	opaque, err := NewApplyService().Apply(
		ctx,
		store,
		prepared.request.opaqueRequest,
	)
	if err != nil {
		return SemanticImportResult{}, err
	}
	if err := correlateOpaqueAndSemanticBasis(
		opaque.Receipt(),
		prepared.valid,
	); err != nil {
		return SemanticImportResult{}, err
	}
	typedReceipt, err := admission.AdmitValidated(
		ctx,
		prepared.valid,
		prepared.request.typedKey,
		prepared.request.provenance,
	)
	if err != nil {
		return SemanticImportResult{}, err
	}
	record, err := newSemanticImportRecord(
		opaque.Receipt(),
		prepared,
		typedReceipt,
	)
	if err != nil {
		return SemanticImportResult{}, err
	}
	durable, err := store.appendSemanticImport(ctx, record)
	if err != nil {
		return SemanticImportResult{}, fmt.Errorf(
			"%w: %v",
			ErrSemanticImportStore,
			err,
		)
	}
	return SemanticImportResult{
		opaque:       opaque,
		typedReceipt: typedReceipt,
		record:       durable,
	}, nil
}

type SemanticImportRecord struct {
	ref            string
	projectID      string
	receipt        ImportReceiptRef
	candidate      typedmemory.SHA256Digest
	semantic       typedmemory.SHA256Digest
	typedKey       typedmemorystore.IdempotencyKey
	provenance     typedmemory.ProvenanceRef
	graphEventRef  string
	graphCommitRef string
	graphRevision  typedmemory.GraphRevision
	resultDigest   typedmemory.SHA256Digest
	bridges        []legacydualread.IdentityBridge
	canonical      []byte
}

func (record SemanticImportRecord) Ref() string { return record.ref }

func (record SemanticImportRecord) ImportReceiptRef() ImportReceiptRef {
	return record.receipt
}

func (record SemanticImportRecord) GraphEventRef() string { return record.graphEventRef }

func (record SemanticImportRecord) GraphCommitRef() string { return record.graphCommitRef }

func (record SemanticImportRecord) GraphRevision() typedmemory.GraphRevision {
	return record.graphRevision
}

func (record SemanticImportRecord) ResultDigest() typedmemory.SHA256Digest {
	return record.resultDigest
}

func (record SemanticImportRecord) IdentityBridges() []legacydualread.IdentityBridge {
	return append([]legacydualread.IdentityBridge(nil), record.bridges...)
}

type semanticImportRecordDTO struct {
	SchemaVersion        string   `json:"schema_version"`
	ProjectID            string   `json:"project_id"`
	ImportReceiptRef     string   `json:"import_receipt_ref"`
	CandidateDigest      string   `json:"candidate_digest"`
	SemanticChangeDigest string   `json:"semantic_change_digest"`
	TypedIdempotencyKey  string   `json:"typed_idempotency_key"`
	RequestProvenanceRef string   `json:"request_provenance_ref"`
	GraphEventRef        string   `json:"graph_event_ref"`
	GraphCommitRef       string   `json:"graph_commit_ref"`
	ResultGraphRevision  uint64   `json:"result_graph_revision"`
	ResultDigest         string   `json:"result_digest"`
	BridgeDigests        []string `json:"bridge_digests"`
}

func newSemanticImportRecord(
	opaque ImportReceipt,
	prepared PreparedSemanticImport,
	typedReceipt typedmemorystore.CommitReceipt,
) (SemanticImportRecord, error) {
	candidateDigest, err := prepared.request.candidate.Digest()
	if err != nil {
		return SemanticImportRecord{}, err
	}
	bridges := canonicalSemanticBridges(prepared.request.bridges)
	bridgeDigests := make([]string, 0, len(bridges))
	for _, bridge := range bridges {
		bridgeDigests = append(bridgeDigests, bridge.Digest().String())
	}
	dto := semanticImportRecordDTO{
		SchemaVersion:        SemanticImportRecordSchemaVersionV1,
		ProjectID:            opaque.ProjectID().String(),
		ImportReceiptRef:     opaque.Ref().String(),
		CandidateDigest:      candidateDigest.String(),
		SemanticChangeDigest: prepared.valid.SemanticChangeDigest().String(),
		TypedIdempotencyKey:  prepared.request.typedKey.String(),
		RequestProvenanceRef: prepared.request.provenance.String(),
		GraphEventRef:        typedReceipt.EventRef(),
		GraphCommitRef:       typedReceipt.CommitRef(),
		ResultGraphRevision:  typedReceipt.GraphRevision().Value(),
		ResultDigest:         typedReceipt.ResultDigest().String(),
		BridgeDigests:        bridgeDigests,
	}
	canonical, err := json.Marshal(dto)
	if err != nil {
		return SemanticImportRecord{}, fmt.Errorf(
			"encode semantic import record: %w",
			err,
		)
	}
	digest := digestSQLiteBytes(canonical)
	record := SemanticImportRecord{
		ref:            "legacy-semantic-import:" + digest.String(),
		projectID:      opaque.ProjectID().String(),
		receipt:        opaque.Ref(),
		candidate:      candidateDigest,
		semantic:       prepared.valid.SemanticChangeDigest(),
		typedKey:       prepared.request.typedKey,
		provenance:     prepared.request.provenance,
		graphEventRef:  typedReceipt.EventRef(),
		graphCommitRef: typedReceipt.CommitRef(),
		graphRevision:  typedReceipt.GraphRevision(),
		resultDigest:   typedReceipt.ResultDigest(),
		bridges:        bridges,
		canonical:      canonical,
	}
	if err := verifySemanticImportRecord(record); err != nil {
		return SemanticImportRecord{}, err
	}
	return record, nil
}

func (store *SQLiteStore) appendSemanticImport(
	ctx context.Context,
	record SemanticImportRecord,
) (SemanticImportRecord, error) {
	if err := verifySemanticImportRecord(record); err != nil {
		return SemanticImportRecord{}, err
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, store.database)
	if err != nil {
		return SemanticImportRecord{}, err
	}
	finished := false
	defer func() {
		if !finished {
			_ = transaction.Rollback(context.Background()).Err()
		}
	}()
	existing, found, err := loadSemanticImportByCoordinate(
		ctx,
		transaction,
		record.projectID,
		record.receipt.String(),
		record.typedKey.String(),
	)
	if err != nil {
		return SemanticImportRecord{}, err
	}
	if found {
		if !sameSemanticImportRecord(existing, record) {
			return SemanticImportRecord{}, ErrSemanticImportConflict
		}
		if err := verifySemanticTypedCommit(
			ctx,
			transaction,
			existing,
		); err != nil {
			return SemanticImportRecord{}, err
		}
		finished = true
		if finish := transaction.Rollback(context.Background()); finish.Err() != nil {
			return SemanticImportRecord{}, finish.Err()
		}
		return existing, nil
	}
	if err := insertSemanticImportRecord(ctx, transaction, record); err != nil {
		return SemanticImportRecord{}, err
	}
	staged, found, err := loadSemanticImportByCoordinate(
		ctx,
		transaction,
		record.projectID,
		record.receipt.String(),
		record.typedKey.String(),
	)
	if err != nil {
		return SemanticImportRecord{}, err
	}
	if !found || !sameSemanticImportRecord(staged, record) {
		return SemanticImportRecord{}, fmt.Errorf(
			"staged semantic import failed exact reread",
		)
	}
	if err := ctx.Err(); err != nil {
		return SemanticImportRecord{}, err
	}
	finished = true
	finish := transaction.Commit(context.Background())
	if finish.StatementError() != nil {
		return SemanticImportRecord{}, finish.Err()
	}
	return record, nil
}

func insertSemanticImportRecord(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	record SemanticImportRecord,
) error {
	graphRevision, err := sqliteSemanticGraphRevision(record.graphRevision)
	if err != nil {
		return err
	}
	if err := verifySemanticTypedCommit(
		ctx,
		transaction,
		record,
	); err != nil {
		return err
	}
	_, err = transaction.Execute(
		ctx,
		`INSERT INTO legacy_semantic_imports (
			project_id,
			semantic_import_ref,
			import_receipt_ref,
			candidate_digest,
			semantic_change_digest,
			typed_idempotency_key,
			request_provenance_ref,
			graph_event_ref,
			graph_commit_ref,
			result_graph_revision,
			result_digest,
			bridge_count,
			canonical_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		[]any{
			record.projectID,
			record.ref,
			record.receipt.String(),
			record.candidate.String(),
			record.semantic.String(),
			record.typedKey.String(),
			record.provenance.String(),
			record.graphEventRef,
			record.graphCommitRef,
			graphRevision,
			record.resultDigest.String(),
			int64(len(record.bridges)),
			record.canonical,
		},
	)
	if err != nil {
		return fmt.Errorf("insert semantic import: %w", err)
	}
	for index, bridge := range record.bridges {
		basis := bridge.MappingCarrier()
		_, err = transaction.Execute(
			ctx,
			`INSERT INTO legacy_identity_bridges (
				project_id,
				semantic_import_ref,
				legacy_identity_ref,
				entity_id,
				bounded_context_ref,
				mapping_carrier_ref,
				mapping_carrier_edition,
				mapping_carrier_digest,
				bridge_digest,
				canonical_bytes
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{
				record.projectID,
				record.ref,
				bridge.LegacyIdentity().String(),
				bridge.EntityID().String(),
				bridge.BoundedContext().String(),
				basis.Ref().String(),
				basis.Edition().String(),
				basis.Digest().String(),
				bridge.Digest().String(),
				bridge.CanonicalBytes(),
			},
		)
		if err != nil {
			return fmt.Errorf("insert semantic identity bridge %d: %w", index, err)
		}
	}
	return nil
}

func verifySemanticTypedCommit(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	record SemanticImportRecord,
) error {
	expectedRevision, err := sqliteSemanticGraphRevision(record.graphRevision)
	if err != nil {
		return err
	}
	var semanticDigest string
	var eventRef string
	var revision int64
	var resultDigest string
	var commitRef string
	err = transaction.ScanOne(
		ctx,
		`SELECT
			idempotency.change_set_digest,
			idempotency.event_ref,
			idempotency.graph_revision,
			idempotency.result_digest,
			event.commit_ref
		 FROM typed_memory_idempotency_history idempotency
		 JOIN typed_memory_graph_events event
			ON event.project_id = idempotency.project_id
			AND event.event_ref = idempotency.event_ref
		 WHERE idempotency.project_id = ?
			AND idempotency.idempotency_key = ?`,
		[]any{record.projectID, record.typedKey.String()},
		[]any{
			&semanticDigest,
			&eventRef,
			&revision,
			&resultDigest,
			&commitRef,
		},
	)
	if err != nil {
		return fmt.Errorf("load exact semantic typed commit: %w", err)
	}
	matches := semanticDigest == record.semantic.String() &&
		eventRef == record.graphEventRef &&
		revision == expectedRevision &&
		resultDigest == record.resultDigest.String() &&
		commitRef == record.graphCommitRef
	if !matches {
		return fmt.Errorf(
			"%w: typed idempotency history differs from semantic import marker",
			ErrSemanticImportConflict,
		)
	}
	return nil
}

func loadSemanticImportByCoordinate(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	projectID string,
	receiptRef string,
	typedKey string,
) (SemanticImportRecord, bool, error) {
	var matches int
	err := transaction.ScanOne(
		ctx,
		`SELECT COUNT(*)
		 FROM legacy_semantic_imports
		 WHERE project_id = ?
			AND (
				import_receipt_ref = ?
				OR typed_idempotency_key = ?
			)`,
		[]any{projectID, receiptRef, typedKey},
		[]any{&matches},
	)
	if err != nil {
		return SemanticImportRecord{}, false, err
	}
	if matches == 0 {
		return SemanticImportRecord{}, false, nil
	}
	if matches != 1 {
		return SemanticImportRecord{}, false, ErrSemanticImportConflict
	}
	var ref string
	var candidate string
	var semantic string
	var provenance string
	var eventRef string
	var commitRef string
	var revision int64
	var resultDigest string
	var bridgeCount int64
	var canonical []byte
	err = transaction.ScanOne(
		ctx,
		`SELECT
			semantic_import_ref,
			candidate_digest,
			semantic_change_digest,
			request_provenance_ref,
			graph_event_ref,
			graph_commit_ref,
			result_graph_revision,
			result_digest,
			bridge_count,
			canonical_bytes
		 FROM legacy_semantic_imports
		 WHERE project_id = ?
			AND (
				import_receipt_ref = ?
				OR typed_idempotency_key = ?
			)`,
		[]any{projectID, receiptRef, typedKey},
		[]any{
			&ref,
			&candidate,
			&semantic,
			&provenance,
			&eventRef,
			&commitRef,
			&revision,
			&resultDigest,
			&bridgeCount,
			&canonical,
		},
	)
	if err != nil {
		return SemanticImportRecord{}, false, err
	}
	if revision <= 0 || bridgeCount <= 0 {
		return SemanticImportRecord{}, false, ErrSemanticImportConflict
	}
	candidateDigest, err := typedmemory.NewSHA256Digest(candidate)
	if err != nil {
		return SemanticImportRecord{}, false, err
	}
	semanticDigest, err := typedmemory.NewSHA256Digest(semantic)
	if err != nil {
		return SemanticImportRecord{}, false, err
	}
	provenanceRef, err := typedmemory.NewProvenanceRef(provenance)
	if err != nil {
		return SemanticImportRecord{}, false, err
	}
	result, err := typedmemory.NewSHA256Digest(resultDigest)
	if err != nil {
		return SemanticImportRecord{}, false, err
	}
	key, err := typedmemorystore.NewIdempotencyKey(typedKey)
	if err != nil {
		return SemanticImportRecord{}, false, err
	}
	receipt, err := ParseImportReceiptRef(receiptRef)
	if err != nil {
		return SemanticImportRecord{}, false, err
	}
	bridges, err := loadSemanticImportBridges(
		ctx,
		transaction,
		projectID,
		ref,
		int(bridgeCount),
	)
	if err != nil {
		return SemanticImportRecord{}, false, err
	}
	record := SemanticImportRecord{
		ref:            ref,
		projectID:      projectID,
		receipt:        receipt,
		candidate:      candidateDigest,
		semantic:       semanticDigest,
		typedKey:       key,
		provenance:     provenanceRef,
		graphEventRef:  eventRef,
		graphCommitRef: commitRef,
		graphRevision:  typedmemory.NewGraphRevision(uint64(revision)),
		resultDigest:   result,
		bridges:        bridges,
		canonical:      append([]byte(nil), canonical...),
	}
	if err := verifySemanticImportRecord(record); err != nil {
		return SemanticImportRecord{}, false, err
	}
	return record, true, nil
}

func loadSemanticImportBridges(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	projectID string,
	semanticRef string,
	expected int,
) ([]legacydualread.IdentityBridge, error) {
	// sqlitetransaction deliberately exposes ScanOne but not raw rows. Bridge
	// digests are already ordered in the record envelope, so recover each exact
	// bridge through its canonical bytes with bounded indexed lookups.
	var canonical string
	var count int
	err := transaction.ScanOne(
		ctx,
		`SELECT
			COALESCE(
				json_group_array(
					json_object(
						'digest',
						bridge_digest,
						'canonical_hex',
						hex(canonical_bytes)
					)
				),
				'[]'
			),
			COUNT(*)
		 FROM (
			SELECT bridge_digest, canonical_bytes
			FROM legacy_identity_bridges
			WHERE project_id = ? AND semantic_import_ref = ?
			ORDER BY bridge_digest
		 )`,
		[]any{projectID, semanticRef},
		[]any{&canonical, &count},
	)
	if err != nil {
		return nil, err
	}
	if count != expected {
		return nil, ErrSemanticImportConflict
	}
	var encoded []struct {
		Digest       string `json:"digest"`
		CanonicalHex string `json:"canonical_hex"`
	}
	if err := json.Unmarshal([]byte(canonical), &encoded); err != nil {
		return nil, err
	}
	result := make([]legacydualread.IdentityBridge, 0, len(encoded))
	for _, item := range encoded {
		decoded, err := hex.DecodeString(item.CanonicalHex)
		if err != nil {
			return nil, err
		}
		bridge, err := decodeIdentityBridge(decoded)
		if err != nil {
			return nil, err
		}
		if bridge.Digest().String() != item.Digest {
			return nil, ErrSemanticImportConflict
		}
		result = append(result, bridge)
	}
	return canonicalSemanticBridges(result), nil
}

func decodeIdentityBridge(
	canonical []byte,
) (legacydualread.IdentityBridge, error) {
	var dto struct {
		SchemaVersion  string `json:"schema_version"`
		ProjectID      string `json:"project_id"`
		LegacyIdentity string `json:"legacy_identity"`
		EntityID       string `json:"entity_id"`
		BoundedContext string `json:"bounded_context"`
		MappingCarrier struct {
			Ref     string `json:"ref"`
			Edition string `json:"edition"`
			Digest  string `json:"digest"`
		} `json:"mapping_carrier"`
	}
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dto); err != nil {
		return legacydualread.IdentityBridge{}, err
	}
	if dto.SchemaVersion != legacydualread.IdentityBridgeSchemaVersionV1 {
		return legacydualread.IdentityBridge{}, fmt.Errorf(
			"unsupported identity bridge schema %q",
			dto.SchemaVersion,
		)
	}
	project, err := projectidentity.ParseProjectID(dto.ProjectID)
	if err != nil {
		return legacydualread.IdentityBridge{}, err
	}
	legacyRef, err := legacyimport.NewLegacyIdentityRef(dto.LegacyIdentity)
	if err != nil {
		return legacydualread.IdentityBridge{}, err
	}
	entity, err := typedmemory.NewEntityID(dto.EntityID)
	if err != nil {
		return legacydualread.IdentityBridge{}, err
	}
	contextRef, err := typedmemory.NewBoundedContextRef(dto.BoundedContext)
	if err != nil {
		return legacydualread.IdentityBridge{}, err
	}
	carrierRef, err := typedmemory.NewCarrierRef(dto.MappingCarrier.Ref)
	if err != nil {
		return legacydualread.IdentityBridge{}, err
	}
	edition, err := typedmemory.NewCarrierEdition(dto.MappingCarrier.Edition)
	if err != nil {
		return legacydualread.IdentityBridge{}, err
	}
	digest, err := typedmemory.NewSHA256Digest(dto.MappingCarrier.Digest)
	if err != nil {
		return legacydualread.IdentityBridge{}, err
	}
	basis, err := legacydualread.NewMappingCarrierBasis(
		carrierRef,
		edition,
		digest,
	)
	if err != nil {
		return legacydualread.IdentityBridge{}, err
	}
	bridge, err := legacydualread.NewIdentityBridge(
		legacydualread.IdentityBridgeInput{
			Project: project,
			Legacy:  legacyRef,
			Entity:  entity,
			Context: contextRef,
			Basis:   basis,
		},
	)
	if err != nil {
		return legacydualread.IdentityBridge{}, err
	}
	if !bytes.Equal(bridge.CanonicalBytes(), canonical) {
		return legacydualread.IdentityBridge{}, ErrSemanticImportConflict
	}
	return bridge, nil
}

func validateSemanticImportBridges(
	plan legacyimport.ImportPlan,
	values []legacydualread.IdentityBridge,
) ([]legacydualread.IdentityBridge, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf(
			"%w: at least one explicit identity bridge is required",
			ErrSemanticImportRequestInvalid,
		)
	}
	knownLegacy := map[string]struct{}{}
	knownMappingCarriers := map[string]struct{}{}
	for _, carrier := range plan.CarrierHistories() {
		mappingKey := carrier.CarrierRef().String() + "\x00" +
			carrier.CarrierEdition().String() + "\x00" +
			carrier.CarrierDigest().String()
		knownMappingCarriers[mappingKey] = struct{}{}
		identity, present :=
			carrier.LegacyIdentity().(legacyimport.IdentifiedLegacyCarrier)
		if present {
			knownLegacy[identity.Ref().String()] = struct{}{}
		}
	}
	bridges := canonicalSemanticBridges(values)
	targetByLegacy := map[string]string{}
	for index, bridge := range bridges {
		rebuilt, err := legacydualread.NewIdentityBridge(
			legacydualread.IdentityBridgeInput{
				Project: bridge.ProjectID(),
				Legacy:  bridge.LegacyIdentity(),
				Entity:  bridge.EntityID(),
				Context: bridge.BoundedContext(),
				Basis:   bridge.MappingCarrier(),
			},
		)
		if err != nil ||
			!bytes.Equal(rebuilt.CanonicalBytes(), bridge.CanonicalBytes()) ||
			rebuilt.Digest() != bridge.Digest() {
			return nil, fmt.Errorf(
				"%w: identity bridge %d is invalid",
				ErrSemanticImportRequestInvalid,
				index,
			)
		}
		if bridge.ProjectID() != plan.ProjectID() {
			return nil, fmt.Errorf(
				"%w: identity bridge %d belongs to another project",
				ErrSemanticImportRequestInvalid,
				index,
			)
		}
		legacyRef := bridge.LegacyIdentity().String()
		if _, present := knownLegacy[legacyRef]; !present {
			return nil, fmt.Errorf(
				"%w: identity bridge %d has no exact legacy source carrier",
				ErrSemanticImportRequestInvalid,
				index,
			)
		}
		basis := bridge.MappingCarrier()
		mappingKey := basis.Ref().String() + "\x00" +
			basis.Edition().String() + "\x00" +
			basis.Digest().String()
		if _, present := knownMappingCarriers[mappingKey]; !present {
			return nil, fmt.Errorf(
				"%w: identity bridge %d mapping carrier is not preserved by the import plan",
				ErrSemanticImportRequestInvalid,
				index,
			)
		}
		target := bridge.EntityID().String() + "\x00" +
			bridge.BoundedContext().String()
		prior, present := targetByLegacy[legacyRef]
		if present && prior != target {
			return nil, fmt.Errorf(
				"%w: identity bridge collision for %s",
				ErrSemanticImportRequestInvalid,
				legacyRef,
			)
		}
		targetByLegacy[legacyRef] = target
	}
	return bridges, nil
}

func canonicalSemanticBridges(
	values []legacydualread.IdentityBridge,
) []legacydualread.IdentityBridge {
	owned := append([]legacydualread.IdentityBridge(nil), values...)
	sort.Slice(
		owned,
		func(left, right int) bool {
			return owned[left].Digest().String() <
				owned[right].Digest().String()
		},
	)
	result := make([]legacydualread.IdentityBridge, 0, len(owned))
	for _, bridge := range owned {
		if len(result) > 0 &&
			result[len(result)-1].Digest() == bridge.Digest() {
			continue
		}
		result = append(result, bridge)
	}
	return result
}

func verifySemanticImportRequest(request SemanticImportRequest) error {
	rebuilt, err := NewSemanticImportRequest(
		SemanticImportRequestInput{
			OpaqueRequest: request.opaqueRequest,
			Selector:      request.selector,
			Candidate:     request.candidate,
			Bridges:       request.bridges,
			TypedKey:      request.typedKey,
			Provenance:    request.provenance,
		},
	)
	if err != nil {
		return err
	}
	if len(rebuilt.bridges) != len(request.bridges) {
		return ErrSemanticImportRequestInvalid
	}
	return nil
}

func verifyPreparedSemanticImport(
	prepared PreparedSemanticImport,
) error {
	if err := verifySemanticImportRequest(prepared.request); err != nil {
		return err
	}
	if prepared.valid == nil {
		return projectmemory.ErrProjectAdmissionNotValid
	}
	requestDigest, err := prepared.request.candidate.Digest()
	if err != nil {
		return err
	}
	validDigest, err := prepared.valid.Candidate().Digest()
	if err != nil {
		return err
	}
	if requestDigest != validDigest {
		return fmt.Errorf(
			"%w: prepared candidate differs from request",
			ErrSemanticImportRequestInvalid,
		)
	}
	return nil
}

func correlateOpaqueAndSemanticBasis(
	receipt ImportReceipt,
	valid typedmemoryvalidation.ValidOutcome,
) error {
	basis := valid.AdmissionBasis()
	if basis == nil {
		return projectmemory.ErrProjectAdmissionNotValid
	}
	opaque := receipt.SelectedProjectTypeEnv()
	if basis.TypeEnv() != opaque.TypeEnvRef() ||
		basis.GraphRevision() != opaque.GraphRevision() {
		return fmt.Errorf(
			"%w: opaque import and semantic admission bases differ",
			ErrSemanticImportRequestInvalid,
		)
	}
	return nil
}

func verifySemanticImportRecord(record SemanticImportRecord) error {
	if _, err := sqliteSemanticGraphRevision(record.graphRevision); err != nil {
		return err
	}
	if record.projectID == "" ||
		record.ref == "" ||
		record.receipt.String() == "" ||
		record.candidate.String() == "" ||
		record.semantic.String() == "" ||
		record.typedKey.String() == "" ||
		record.provenance.String() == "" ||
		record.graphEventRef == "" ||
		record.graphCommitRef == "" ||
		record.graphRevision.Value() == 0 ||
		record.resultDigest.String() == "" ||
		len(record.bridges) == 0 ||
		len(record.canonical) == 0 {
		return ErrSemanticImportConflict
	}
	bridgeDigests := make([]string, 0, len(record.bridges))
	for _, bridge := range canonicalSemanticBridges(record.bridges) {
		bridgeDigests = append(bridgeDigests, bridge.Digest().String())
	}
	dto := semanticImportRecordDTO{
		SchemaVersion:        SemanticImportRecordSchemaVersionV1,
		ProjectID:            record.projectID,
		ImportReceiptRef:     record.receipt.String(),
		CandidateDigest:      record.candidate.String(),
		SemanticChangeDigest: record.semantic.String(),
		TypedIdempotencyKey:  record.typedKey.String(),
		RequestProvenanceRef: record.provenance.String(),
		GraphEventRef:        record.graphEventRef,
		GraphCommitRef:       record.graphCommitRef,
		ResultGraphRevision:  record.graphRevision.Value(),
		ResultDigest:         record.resultDigest.String(),
		BridgeDigests:        bridgeDigests,
	}
	canonical, err := json.Marshal(dto)
	if err != nil || !bytes.Equal(canonical, record.canonical) {
		return ErrSemanticImportConflict
	}
	expectedRef := "legacy-semantic-import:" +
		digestSQLiteBytes(canonical).String()
	if record.ref != expectedRef {
		return ErrSemanticImportConflict
	}
	return nil
}

func sqliteSemanticGraphRevision(
	revision typedmemory.GraphRevision,
) (int64, error) {
	value := revision.Value()
	if value > math.MaxInt64 {
		return 0, fmt.Errorf(
			"%w: graph revision exceeds SQLite integer range",
			ErrSemanticImportConflict,
		)
	}
	return int64(value), nil // #nosec G115 -- value is bounded by math.MaxInt64 above.
}

func sameSemanticImportRecord(
	left SemanticImportRecord,
	right SemanticImportRecord,
) bool {
	if left.ref != right.ref ||
		left.projectID != right.projectID ||
		left.receipt != right.receipt ||
		left.candidate != right.candidate ||
		left.semantic != right.semantic ||
		left.typedKey != right.typedKey ||
		left.provenance != right.provenance ||
		left.graphEventRef != right.graphEventRef ||
		left.graphCommitRef != right.graphCommitRef ||
		left.graphRevision != right.graphRevision ||
		left.resultDigest != right.resultDigest ||
		!bytes.Equal(left.canonical, right.canonical) ||
		len(left.bridges) != len(right.bridges) {
		return false
	}
	for index := range left.bridges {
		if left.bridges[index].Digest() != right.bridges[index].Digest() {
			return false
		}
	}
	return true
}
