package typedmemorystore

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvactivation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// ProjectTypeEnvActivationHistoryCoordinate is the exact committed coordinate
// an effect replay expects to find. The activation delta is included so a
// storage key cannot select a different, internally valid activation event.
type ProjectTypeEnvActivationHistoryCoordinate struct {
	Project               projectidentity.ProjectID
	StorageIdempotencyKey string
	Delta                 projecttypeenvactivation.Delta
	Event                 projecttypeenvselection.GraphEventRef
	EventDigest           typedmemory.SHA256Digest
	Commit                projecttypeenvselection.GraphCommitRef
	GraphRevision         typedmemory.GraphRevision
	MaterializationDigest typedmemory.SHA256Digest
}

// VerifiedProjectTypeEnvActivationHistory is transaction-bound read evidence,
// not a current-head or mutation capability. Receipt and selection-closure
// carriers remain effect-layer values; storage returns their exact immutable
// bytes so that layer can decode and compare them without re-querying.
type VerifiedProjectTypeEnvActivationHistory struct {
	project                projectidentity.ProjectID
	delta                  projecttypeenvactivation.Delta
	event                  projecttypeenvselection.GraphEventRef
	eventDigest            typedmemory.SHA256Digest
	commit                 projecttypeenvselection.GraphCommitRef
	graphRevision          typedmemory.GraphRevision
	materializationDigest  typedmemory.SHA256Digest
	receiptRef             string
	receiptDigest          typedmemory.SHA256Digest
	receiptBytes           []byte
	selectionClosureRef    string
	selectionClosureDigest typedmemory.SHA256Digest
	selectionClosureBytes  []byte
}

func (value VerifiedProjectTypeEnvActivationHistory) Project() projectidentity.ProjectID {
	return value.project
}

func (value VerifiedProjectTypeEnvActivationHistory) Delta() projecttypeenvactivation.Delta {
	decoded, _ := projecttypeenvactivation.DecodeDelta(value.delta.CanonicalBytes())
	return decoded
}

func (value VerifiedProjectTypeEnvActivationHistory) EventRef() projecttypeenvselection.GraphEventRef {
	return value.event
}

func (value VerifiedProjectTypeEnvActivationHistory) EventDigest() typedmemory.SHA256Digest {
	return value.eventDigest
}

func (value VerifiedProjectTypeEnvActivationHistory) CommitRef() projecttypeenvselection.GraphCommitRef {
	return value.commit
}

func (value VerifiedProjectTypeEnvActivationHistory) GraphRevision() typedmemory.GraphRevision {
	return value.graphRevision
}

func (value VerifiedProjectTypeEnvActivationHistory) MaterializationDigest() typedmemory.SHA256Digest {
	return value.materializationDigest
}

func (value VerifiedProjectTypeEnvActivationHistory) ReceiptRef() string {
	return value.receiptRef
}

func (value VerifiedProjectTypeEnvActivationHistory) ReceiptDigest() typedmemory.SHA256Digest {
	return value.receiptDigest
}

func (value VerifiedProjectTypeEnvActivationHistory) ReceiptCanonicalBytes() []byte {
	return append([]byte(nil), value.receiptBytes...)
}

func (value VerifiedProjectTypeEnvActivationHistory) SelectionClosureRef() string {
	return value.selectionClosureRef
}

func (value VerifiedProjectTypeEnvActivationHistory) SelectionClosureDigest() typedmemory.SHA256Digest {
	return value.selectionClosureDigest
}

func (value VerifiedProjectTypeEnvActivationHistory) SelectionClosureCanonicalBytes() []byte {
	return append([]byte(nil), value.selectionClosureBytes...)
}

// VerifyCommittedProjectTypeEnvActivationHistoryTx replays the complete
// storage-owned proof for one historical activation inside the caller's
// transaction. It does not require the event to remain the current graph head.
func VerifyCommittedProjectTypeEnvActivationHistoryTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	expected ProjectTypeEnvActivationHistoryCoordinate,
) (VerifiedProjectTypeEnvActivationHistory, error) {
	coordinate, key, err := normalizeProjectTypeEnvActivationHistoryCoordinate(
		expected,
	)
	if err != nil {
		return VerifiedProjectTypeEnvActivationHistory{}, err
	}
	sqliteGraphRevision, err := exactSQLiteCoordinate(
		coordinate.GraphRevision.Value(),
		"historical activation graph revision",
	)
	if err != nil {
		return VerifiedProjectTypeEnvActivationHistory{}, err
	}
	if ctx == nil {
		return VerifiedProjectTypeEnvActivationHistory{},
			fmt.Errorf("verify ProjectTypeEnv activation history: context is required")
	}
	if err := transaction.RequireActive(); err != nil {
		return VerifiedProjectTypeEnvActivationHistory{}, err
	}
	if err := requireGenericStorageCapability(ctx, transaction); err != nil {
		return VerifiedProjectTypeEnvActivationHistory{}, err
	}
	common, found, err := loadDurableGenericCommonRow(
		ctx,
		transaction,
		coordinate.Project,
		key,
	)
	if err != nil {
		return VerifiedProjectTypeEnvActivationHistory{}, err
	}
	if !found || common.idempotencyEventRef != coordinate.Event.String() {
		return VerifiedProjectTypeEnvActivationHistory{},
			storedAdmissionIntegrity("historical activation idempotency/event coordinate", nil)
	}
	writer, found, err := loadEventWriterGeneration(
		ctx,
		transaction,
		coordinate.Project,
		coordinate.Event.String(),
	)
	if err != nil {
		return VerifiedProjectTypeEnvActivationHistory{}, err
	}
	if !found ||
		writer.generation != genericStorageWriterGeneration ||
		writer.provenance != "writer_v46" {
		return VerifiedProjectTypeEnvActivationHistory{},
			storedAdmissionIntegrity("historical activation writer generation", nil)
	}
	admission, found, err := loadDurableV46AdmissionRow(
		ctx,
		transaction,
		coordinate.Project,
		coordinate.Event.String(),
	)
	if err != nil {
		return VerifiedProjectTypeEnvActivationHistory{}, err
	}
	if !found {
		return VerifiedProjectTypeEnvActivationHistory{},
			storedAdmissionIntegrity("historical activation admission is missing", nil)
	}
	closure, found, err := loadDurableV46ClosureRow(
		ctx,
		transaction,
		coordinate.Project,
		coordinate.Event.String(),
	)
	if err != nil {
		return VerifiedProjectTypeEnvActivationHistory{}, err
	}
	if !found {
		return VerifiedProjectTypeEnvActivationHistory{},
			storedAdmissionIntegrity("historical activation materialization is missing", nil)
	}
	storedCoordinate := storedAdmissionCoordinate{
		EventRef:             coordinate.Event.String(),
		IdempotencyKey:       key.String(),
		WriterGeneration:     writer.generation,
		GenerationProvenance: writer.provenance,
	}
	verified, err := verifyExactProjectTypeEnvActivationCoordinate(
		ctx,
		transaction,
		coordinate.Project,
		storedCoordinate,
		common,
		admission,
		closure,
	)
	if err != nil {
		return VerifiedProjectTypeEnvActivationHistory{}, err
	}
	if verified.eventRef != coordinate.Event.String() ||
		verified.commit != coordinate.Commit.String() ||
		verified.digest != coordinate.MaterializationDigest ||
		common.eventDigest != coordinate.EventDigest.String() ||
		common.eventRevision != sqliteGraphRevision ||
		!bytes.Equal(common.eventCanonicalBytes, coordinate.Delta.CanonicalBytes()) ||
		common.eventChangeDigest != coordinate.Delta.Digest().String() {
		return VerifiedProjectTypeEnvActivationHistory{},
			storedAdmissionIntegrity("historical activation expected coordinate", nil)
	}
	effect, err := loadAndVerifyProjectTypeEnvActivationEffectHistory(
		ctx,
		transaction,
		common,
		coordinate.Delta,
	)
	if err != nil {
		return VerifiedProjectTypeEnvActivationHistory{}, err
	}
	return VerifiedProjectTypeEnvActivationHistory{
		project:                coordinate.Project,
		delta:                  coordinate.Delta,
		event:                  coordinate.Event,
		eventDigest:            coordinate.EventDigest,
		commit:                 coordinate.Commit,
		graphRevision:          coordinate.GraphRevision,
		materializationDigest:  coordinate.MaterializationDigest,
		receiptRef:             effect.receiptRef,
		receiptDigest:          effect.receiptDigest,
		receiptBytes:           effect.receiptBytes,
		selectionClosureRef:    effect.selectionClosureRef,
		selectionClosureDigest: effect.selectionClosureDigest,
		selectionClosureBytes:  effect.selectionClosureBytes,
	}, nil
}

func normalizeProjectTypeEnvActivationHistoryCoordinate(
	input ProjectTypeEnvActivationHistoryCoordinate,
) (
	ProjectTypeEnvActivationHistoryCoordinate,
	IdempotencyKey,
	error,
) {
	project, err := projectidentity.ParseProjectID(input.Project.String())
	if err != nil || project != input.Project {
		return ProjectTypeEnvActivationHistoryCoordinate{}, IdempotencyKey{},
			fmt.Errorf("historical activation project is required")
	}
	key, err := NewIdempotencyKey(input.StorageIdempotencyKey)
	if err != nil {
		return ProjectTypeEnvActivationHistoryCoordinate{}, IdempotencyKey{}, err
	}
	if err := input.Delta.Verify(); err != nil {
		return ProjectTypeEnvActivationHistoryCoordinate{}, IdempotencyKey{}, err
	}
	event, err := projecttypeenvselection.ParseGraphEventRef(input.Event.String())
	if err != nil || event != input.Event {
		return ProjectTypeEnvActivationHistoryCoordinate{}, IdempotencyKey{},
			fmt.Errorf("historical activation event is required")
	}
	commit, err := projecttypeenvselection.ParseGraphCommitRef(input.Commit.String())
	if err != nil || commit != input.Commit {
		return ProjectTypeEnvActivationHistoryCoordinate{}, IdempotencyKey{},
			fmt.Errorf("historical activation commit is required")
	}
	eventDigest, err := typedmemory.NewSHA256Digest(input.EventDigest.String())
	if err != nil || eventDigest != input.EventDigest {
		return ProjectTypeEnvActivationHistoryCoordinate{}, IdempotencyKey{},
			fmt.Errorf("historical activation event digest is required")
	}
	materializationDigest, err := typedmemory.NewSHA256Digest(
		input.MaterializationDigest.String(),
	)
	if err != nil || materializationDigest != input.MaterializationDigest {
		return ProjectTypeEnvActivationHistoryCoordinate{}, IdempotencyKey{},
			fmt.Errorf("historical activation materialization digest is required")
	}
	matches := input.Delta.Project() == project &&
		input.Delta.CommittedGraphRevision() == input.GraphRevision &&
		input.GraphRevision.Value() > 0 &&
		input.GraphRevision.Value() <= mathMaxSQLiteRevision &&
		input.Delta.SuccessorHeadRevision().Value() <= mathMaxSQLiteRevision
	if !matches {
		return ProjectTypeEnvActivationHistoryCoordinate{}, IdempotencyKey{},
			fmt.Errorf("historical activation delta coordinate differs from expectation")
	}
	return ProjectTypeEnvActivationHistoryCoordinate{
		Project:               project,
		StorageIdempotencyKey: key.String(),
		Delta:                 input.Delta,
		Event:                 event,
		EventDigest:           eventDigest,
		Commit:                commit,
		GraphRevision:         input.GraphRevision,
		MaterializationDigest: materializationDigest,
	}, key, nil
}

type storedProjectTypeEnvActivationEffectHistory struct {
	headStateDigest        typedmemory.SHA256Digest
	headStateBytes         []byte
	receiptRef             string
	receiptDigest          typedmemory.SHA256Digest
	receiptBytes           []byte
	selectionClosureRef    string
	selectionClosureDigest typedmemory.SHA256Digest
	selectionClosureBytes  []byte
}

func loadAndVerifyProjectTypeEnvActivationEffectHistory(
	ctx context.Context,
	source scanner,
	common durableGenericCommonRow,
	delta projecttypeenvactivation.Delta,
) (storedProjectTypeEnvActivationEffectHistory, error) {
	sqliteHeadRevision, err := exactSQLiteCoordinate(
		delta.SuccessorHeadRevision().Value(),
		"historical activation successor head revision",
	)
	if err != nil {
		return storedProjectTypeEnvActivationEffectHistory{}, err
	}
	sqliteGraphRevision, err := exactSQLiteCoordinate(
		delta.CommittedGraphRevision().Value(),
		"historical activation committed graph revision",
	)
	if err != nil {
		return storedProjectTypeEnvActivationEffectHistory{}, err
	}
	row := struct {
		headStateDigestText        string
		headStateBytes             []byte
		historyRecordedAt          string
		receiptRef                 string
		receiptDigestText          string
		receiptBytes               []byte
		receiptRecordedAt          string
		selectionClosureRef        string
		selectionClosureDigestText string
		selectionClosureBytes      []byte
		selectionClosureRecordedAt string
	}{}
	err = source.ScanOne(
		ctx,
		`SELECT history.head_state_digest, history.canonical_head_state_bytes,
			history.recorded_at,
			receipt.receipt_ref, receipt.receipt_digest, receipt.canonical_bytes,
			receipt.recorded_at,
			selection_closure.closure_ref, selection_closure.closure_digest,
			selection_closure.canonical_bytes, selection_closure.recorded_at
		FROM typed_memory_type_env_activations activation
		JOIN project_typeenv_head_history history
			ON history.project_id = activation.project_id
			AND history.activation_ref = activation.activation_ref
			AND history.activation_digest = activation.activation_digest
			AND history.graph_event_ref = activation.event_ref
			AND history.request_ref = activation.request_ref
			AND history.request_digest = activation.request_digest
			AND history.authority_use_ref = activation.authority_use_ref
			AND history.authority_use_digest = activation.authority_use_digest
			AND history.work_ref = activation.work_ref
		JOIN project_typeenv_head_states head_state
			ON head_state.project_id = history.project_id
			AND head_state.head_ref = history.head_ref
			AND head_state.head_revision = history.head_revision
			AND head_state.selected_composite_ref = history.selected_composite_ref
			AND head_state.state_digest = history.head_state_digest
			AND head_state.canonical_bytes = history.canonical_head_state_bytes
		JOIN project_typeenv_head_selection_authority_uses authority_use
			ON authority_use.authority_use_ref = history.authority_use_ref
			AND authority_use.authority_use_digest = history.authority_use_digest
			AND authority_use.request_ref = history.request_ref
			AND authority_use.request_digest = history.request_digest
			AND authority_use.work_ref = history.work_ref
			AND authority_use.receipt_ref = history.receipt_ref
		JOIN project_typeenv_head_cas_work_records work_record
			ON work_record.work_ref = history.work_ref
			AND work_record.authority_use_ref = authority_use.authority_use_ref
			AND work_record.authority_use_digest = authority_use.authority_use_digest
			AND work_record.receipt_ref = history.receipt_ref
			AND work_record.activation_ref = history.activation_ref
		JOIN project_typeenv_head_selection_receipts receipt
			ON receipt.receipt_ref = history.receipt_ref
			AND receipt.project_id = history.project_id
			AND receipt.authority_use_ref = history.authority_use_ref
			AND receipt.authority_use_digest = history.authority_use_digest
			AND receipt.cas_work_record_ref = work_record.cas_work_record_ref
			AND receipt.cas_work_record_digest = work_record.cas_work_record_digest
			AND receipt.work_ref = history.work_ref
			AND receipt.activation_ref = history.activation_ref
			AND receipt.activation_digest = history.activation_digest
			AND receipt.authority_resolution_ref =
				authority_use.authority_resolution_ref
			AND receipt.authority_resolution_digest =
				authority_use.authority_resolution_digest
			AND receipt.content_ref = authority_use.content_ref
			AND receipt.content_digest = authority_use.content_digest
			AND receipt.request_ref = history.request_ref
			AND receipt.request_digest = history.request_digest
		JOIN project_typeenv_head_selection_closures selection_closure
			ON selection_closure.receipt_ref = receipt.receipt_ref
			AND selection_closure.receipt_digest = receipt.receipt_digest
			AND selection_closure.project_id = receipt.project_id
			AND selection_closure.authority_use_ref = receipt.authority_use_ref
			AND selection_closure.authority_use_digest = receipt.authority_use_digest
			AND selection_closure.cas_work_record_ref =
				receipt.cas_work_record_ref
			AND selection_closure.cas_work_record_digest =
				receipt.cas_work_record_digest
			AND selection_closure.activation_ref = receipt.activation_ref
			AND selection_closure.activation_digest = receipt.activation_digest
			AND selection_closure.authority_resolution_ref =
				receipt.authority_resolution_ref
			AND selection_closure.authority_resolution_digest =
				receipt.authority_resolution_digest
			AND selection_closure.content_ref = receipt.content_ref
			AND selection_closure.content_digest = receipt.content_digest
			AND selection_closure.request_ref = receipt.request_ref
			AND selection_closure.request_digest = receipt.request_digest
			AND selection_closure.head_ref = receipt.head_ref
			AND selection_closure.head_revision = receipt.head_revision
			AND selection_closure.head_state_digest = history.head_state_digest
			AND selection_closure.graph_revision = receipt.graph_revision
			AND selection_closure.graph_event_ref = receipt.graph_event_ref
			AND selection_closure.graph_commit_ref = receipt.graph_commit_ref
		WHERE activation.project_id = ? AND activation.event_ref = ?
			AND history.head_ref = ?
			AND history.head_revision = ?
			AND history.selected_composite_ref = ?
			AND history.graph_revision = ?
			AND history.graph_event_ref = ?
			AND history.graph_commit_ref = ?
			AND history.request_ref = ?
			AND history.request_digest = ?
			AND history.authority_use_ref = ?
			AND history.work_ref = ?
			AND authority_use.work_ref = ?
			AND work_record.cas_work_record_ref = ?
			AND work_record.activation_ref = ?
			AND receipt.activation_ref = ?
			AND receipt.activation_digest = ?
			AND receipt.request_ref = ?
			AND receipt.request_digest = ?
			AND receipt.head_ref = ?
			AND receipt.head_revision = ?
			AND receipt.selected_composite_ref = ?
			AND receipt.graph_revision = ?
			AND receipt.graph_event_ref = ?
			AND receipt.graph_commit_ref = ?
			AND selection_closure.activation_ref = ?
			AND selection_closure.activation_digest = ?
			AND selection_closure.request_ref = ?
			AND selection_closure.request_digest = ?
			AND selection_closure.head_ref = ?
			AND selection_closure.head_revision = ?
			AND selection_closure.graph_revision = ?
			AND selection_closure.graph_event_ref = ?
			AND selection_closure.graph_event_digest = ?
			AND selection_closure.graph_commit_ref = ?`,
		[]any{
			delta.Project().String(),
			common.idempotencyEventRef,
			delta.Head().String(),
			sqliteHeadRevision,
			delta.Target().Composite().String(),
			sqliteGraphRevision,
			common.idempotencyEventRef,
			common.eventCommitRef,
			delta.RequestRef().String(),
			delta.RequestDigest().String(),
			delta.AuthorityUseRef(),
			delta.WorkRef().String(),
			delta.WorkRef().String(),
			delta.WorkRecordRef(),
			delta.Ref().String(),
			delta.Ref().String(),
			delta.Digest().String(),
			delta.RequestRef().String(),
			delta.RequestDigest().String(),
			delta.Head().String(),
			sqliteHeadRevision,
			delta.Target().Composite().String(),
			sqliteGraphRevision,
			common.idempotencyEventRef,
			common.eventCommitRef,
			delta.Ref().String(),
			delta.Digest().String(),
			delta.RequestRef().String(),
			delta.RequestDigest().String(),
			delta.Head().String(),
			sqliteHeadRevision,
			sqliteGraphRevision,
			common.idempotencyEventRef,
			common.eventDigest,
			common.eventCommitRef,
		},
		[]any{
			&row.headStateDigestText,
			&row.headStateBytes,
			&row.historyRecordedAt,
			&row.receiptRef,
			&row.receiptDigestText,
			&row.receiptBytes,
			&row.receiptRecordedAt,
			&row.selectionClosureRef,
			&row.selectionClosureDigestText,
			&row.selectionClosureBytes,
			&row.selectionClosureRecordedAt,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return storedProjectTypeEnvActivationEffectHistory{},
			storedAdmissionIntegrity("historical activation effect closure is missing", nil)
	}
	if err != nil {
		return storedProjectTypeEnvActivationEffectHistory{},
			fmt.Errorf("load historical ProjectTypeEnv activation effect closure: %w", err)
	}
	headState, err := projecttypeenvselection.DecodeProjectTypeEnvHeadState(
		row.headStateBytes,
	)
	if err != nil {
		return storedProjectTypeEnvActivationEffectHistory{},
			storedAdmissionIntegrity("historical activation head state", err)
	}
	headStateDigest, err := typedmemory.NewSHA256Digest(row.headStateDigestText)
	if err != nil {
		return storedProjectTypeEnvActivationEffectHistory{},
			storedAdmissionIntegrity("historical activation head-state digest", err)
	}
	recomputedHeadStateDigest, err := digestBytes(row.headStateBytes)
	if err != nil {
		return storedProjectTypeEnvActivationEffectHistory{}, err
	}
	receiptDigest, err := typedmemory.NewSHA256Digest(row.receiptDigestText)
	if err != nil {
		return storedProjectTypeEnvActivationEffectHistory{},
			storedAdmissionIntegrity("historical activation receipt digest", err)
	}
	selectionClosureDigest, err := typedmemory.NewSHA256Digest(
		row.selectionClosureDigestText,
	)
	if err != nil {
		return storedProjectTypeEnvActivationEffectHistory{},
			storedAdmissionIntegrity("historical activation selection-closure digest", err)
	}
	recordedAtCoordinates := []struct {
		label string
		value string
	}{
		{label: "head history", value: row.historyRecordedAt},
		{label: "selection receipt", value: row.receiptRecordedAt},
		{label: "selection closure", value: row.selectionClosureRecordedAt},
	}
	for _, coordinate := range recordedAtCoordinates {
		if _, err := parseCanonicalGenericRecordedAt(coordinate.value); err != nil {
			return storedProjectTypeEnvActivationEffectHistory{},
				storedAdmissionIntegrity(
					"historical activation "+coordinate.label+" recorded_at",
					err,
				)
		}
		if coordinate.value != common.eventRecordedAt {
			return storedProjectTypeEnvActivationEffectHistory{},
				storedAdmissionIntegrity(
					"historical activation "+coordinate.label+" recorded_at link",
					nil,
				)
		}
	}
	matches := headState.Project() == delta.Project() &&
		headState.Ref() == delta.Head() &&
		headState.Revision() == delta.SuccessorHeadRevision() &&
		headState.SelectedComposite() == delta.Target().Composite() &&
		headStateDigest == recomputedHeadStateDigest &&
		row.receiptRef != "" &&
		len(row.receiptBytes) > 0 &&
		row.selectionClosureRef != "" &&
		len(row.selectionClosureBytes) > 0
	if !matches {
		return storedProjectTypeEnvActivationEffectHistory{},
			storedAdmissionIntegrity("historical activation exact effect carriers", nil)
	}
	return storedProjectTypeEnvActivationEffectHistory{
		headStateDigest:        headStateDigest,
		headStateBytes:         append([]byte(nil), row.headStateBytes...),
		receiptRef:             row.receiptRef,
		receiptDigest:          receiptDigest,
		receiptBytes:           append([]byte(nil), row.receiptBytes...),
		selectionClosureRef:    row.selectionClosureRef,
		selectionClosureDigest: selectionClosureDigest,
		selectionClosureBytes:  append([]byte(nil), row.selectionClosureBytes...),
	}, nil
}
