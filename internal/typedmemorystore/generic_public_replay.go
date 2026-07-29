package typedmemorystore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// ReplayMemoryChangeSet verifies and returns only an already-committed exact
// admission. Absence returns found=false. Any occupied idempotency coordinate
// that differs from the supplied request returns ErrIdempotencyConflict.
//
// The method has no AdmissionBatch and performs no write. A caller must proceed
// through ordinary validation and CommitMemoryChangeSet when found=false.
func (adapter *SQLiteAdapter) ReplayMemoryChangeSet(
	ctx context.Context,
	request ReplayRequest,
) (CommitReceipt, bool, error) {
	if ctx == nil {
		return CommitReceipt{}, false, fmt.Errorf(
			"replay typed-memory change set: context is required",
		)
	}
	if adapter == nil || adapter.database == nil {
		return CommitReceipt{}, false, ErrDatabaseRequired
	}
	candidateBytes, err := request.candidate.CanonicalBytes()
	if err != nil {
		return CommitReceipt{}, false, fmt.Errorf(
			"canonicalize typed-memory replay candidate: %w",
			err,
		)
	}
	candidateDigest, err := request.candidate.Digest()
	if err != nil {
		return CommitReceipt{}, false, fmt.Errorf(
			"digest typed-memory replay candidate: %w",
			err,
		)
	}
	transaction, err := sqlitetransaction.BeginRead(ctx, adapter.database)
	if err != nil {
		return CommitReceipt{}, false, fmt.Errorf(
			"begin typed-memory replay probe: %w",
			err,
		)
	}
	receipt, found, replayErr := replayExactGenericAdmission(
		ctx,
		transaction,
		request,
		candidateBytes,
		candidateDigest,
	)
	finish := transaction.Rollback(ctx)
	if replayErr != nil {
		return CommitReceipt{}, found, joinReplayFinishError(replayErr, finish.Err())
	}
	if !finish.Succeeded() {
		return CommitReceipt{}, found, fmt.Errorf(
			"finish typed-memory replay probe: %w",
			finish.Err(),
		)
	}
	return receipt, found, nil
}

func replayExactGenericAdmission(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	request ReplayRequest,
	candidateBytes []byte,
	candidateDigest typedmemory.SHA256Digest,
) (CommitReceipt, bool, error) {
	if err := requireGenericAdmissionStorageCapability(
		ctx,
		transaction,
		request.ContractVersion(),
	); err != nil {
		return CommitReceipt{}, false, err
	}
	common, found, err := loadDurableGenericCommonRow(
		ctx,
		transaction,
		request.project,
		request.idempotencyKey,
	)
	if err != nil || !found {
		return CommitReceipt{}, found, err
	}
	if err := verifyStoredGenericEventIdentity(request.project, common); err != nil {
		return CommitReceipt{}, true, err
	}
	admission, admissionFound, err := loadDurableV46AdmissionRow(
		ctx,
		transaction,
		request.project,
		common.idempotencyEventRef,
	)
	if err != nil {
		return CommitReceipt{}, true, err
	}
	closure, closureFound, err := loadDurableV46ClosureRow(
		ctx,
		transaction,
		request.project,
		common.idempotencyEventRef,
	)
	if err != nil {
		return CommitReceipt{}, true, err
	}
	writer, writerFound, err := loadEventWriterGeneration(
		ctx,
		transaction,
		request.project,
		common.idempotencyEventRef,
	)
	if err != nil {
		return CommitReceipt{}, true, err
	}
	if !admissionFound || !closureFound {
		return CommitReceipt{}, true, storedAdmissionIntegrity(
			"event writer generation companion completeness",
			nil,
		)
	}
	basisKind, err := typedmemory.ParseAdmissionBasisKind(admission.basisKind)
	if err != nil {
		return CommitReceipt{}, true, storedAdmissionIntegrity(
			"public replay admission-basis kind",
			err,
		)
	}
	if err := requireExpectedStorageAvailability(
		ctx,
		transaction,
		request.ContractVersion(),
		basisKind,
	); err != nil {
		return CommitReceipt{}, true, err
	}
	if err := verifyExpectedReplayWriter(
		request.ContractVersion(),
		basisKind,
		writer,
		writerFound,
	); err != nil {
		return CommitReceipt{}, true, err
	}
	if err := verifyStoredGenericAdmissionCarriers(admission); err != nil {
		return CommitReceipt{}, true, err
	}
	if err := verifyStoredGenericAdmissionLinks(common, admission, closure); err != nil {
		return CommitReceipt{}, true, err
	}
	if err := comparePublicReplayRequest(
		request,
		candidateBytes,
		candidateDigest,
		common,
		admission,
	); err != nil {
		return CommitReceipt{}, true, err
	}
	if _, err := verifySnapshotExactV46Materialization(
		ctx,
		transaction,
		request.project,
		common,
		admission,
		closure,
		request.ContractVersion(),
	); err != nil {
		return CommitReceipt{}, true, err
	}
	receipt, err := replayReceiptFromStoredAdmission(
		request,
		common,
		admission,
		closure,
	)
	if err != nil {
		return CommitReceipt{}, true, err
	}
	return receipt, true, nil
}

func comparePublicReplayRequest(
	request ReplayRequest,
	candidateBytes []byte,
	candidateDigest typedmemory.SHA256Digest,
	common durableGenericCommonRow,
	admission durableV46AdmissionRow,
) error {
	expectedRevision, exact := sqliteIntegerFromUint64(request.expectedRevision.Value())
	if !exact {
		return ErrRevisionOverflow
	}
	checks := []struct {
		matches bool
		detail  string
	}{
		{
			admission.typeEnvRef == request.expectedTypeEnv.String(),
			"basis TypeEnv",
		},
		{
			admission.basisRevision == expectedRevision,
			"basis graph revision",
		},
		{
			admission.requestDigest == candidateDigest.String(),
			"request digest",
		},
		{
			bytes.Equal(admission.requestBytes, candidateBytes),
			"request bytes",
		},
		{
			common.eventProvenance == request.requestProvenance.String(),
			"request provenance",
		},
		{
			common.eventAuthorityClass ==
				"non_binding_semantic_assertion",
			"authority class",
		},
		{
			common.eventChangeCount ==
				int64(len(request.candidate.Changes())),
			"change count",
		},
	}
	for _, check := range checks {
		if check.matches {
			continue
		}
		return genericReplayConflict(check.detail, nil)
	}
	return nil
}

func replayReceiptFromStoredAdmission(
	request ReplayRequest,
	common durableGenericCommonRow,
	admission durableV46AdmissionRow,
	closure durableV46ClosureRow,
) (CommitReceipt, error) {
	expectedRevision, err := graphRevisionFromSQLite(common.eventExpectedRevision)
	if err != nil {
		return CommitReceipt{}, storedAdmissionIntegrity(
			"replay event expected revision",
			err,
		)
	}
	graphRevision, err := graphRevisionFromSQLite(common.eventRevision)
	if err != nil {
		return CommitReceipt{}, storedAdmissionIntegrity(
			"replay event graph revision",
			err,
		)
	}
	basisTypeEnv, err := parseTypeEnvRef(common.eventBasisTypeEnv)
	if err != nil {
		return CommitReceipt{}, storedAdmissionIntegrity(
			"replay event basis TypeEnv",
			err,
		)
	}
	semanticDigest, err := typedmemory.NewSHA256Digest(
		admission.semanticDigest,
	)
	if err != nil {
		return CommitReceipt{}, storedAdmissionIntegrity(
			"replay semantic digest",
			err,
		)
	}
	expectedCommitRef := derivedRef(
		"typed-memory-commit",
		request.project.String(),
		request.idempotencyKey.String(),
		semanticDigest.String(),
		strconv.FormatUint(graphRevision.Value(), 10),
	)
	eventDigest, err := digestFields(
		"typed-memory-graph-event.v1",
		request.project.String(),
		expectedCommitRef,
		strconv.FormatUint(expectedRevision.Value(), 10),
		strconv.FormatUint(graphRevision.Value(), 10),
		basisTypeEnv.String(),
		semanticDigest.String(),
		string(admission.semanticBytes),
		common.eventKind,
		common.eventAuthorityClass,
		common.eventProvenance,
	)
	if err != nil {
		return CommitReceipt{}, storedAdmissionIntegrity(
			"replay event digest",
			err,
		)
	}
	expectedEventRef := derivedRef(
		"typed-memory-event",
		eventDigest.String(),
	)
	checks := []struct {
		matches bool
		detail  string
	}{
		{
			graphRevision.Value() == expectedRevision.Value()+1,
			"event revision continuity",
		},
		{
			common.eventDigest == eventDigest.String(),
			"event digest",
		},
		{
			common.idempotencyResultDigest == eventDigest.String(),
			"idempotency result digest",
		},
		{
			common.commitEventDigest == eventDigest.String(),
			"commit event digest",
		},
		{
			common.idempotencyEventRef == expectedEventRef,
			"idempotency event ref",
		},
		{
			common.commitEventRef == expectedEventRef,
			"commit event ref",
		},
		{
			common.eventCommitRef == expectedCommitRef,
			"event commit ref",
		},
		{
			common.commitRef == expectedCommitRef,
			"commit ref",
		},
		{
			closure.commitRef == expectedCommitRef,
			"closure commit ref",
		},
		{
			closure.eventDigest == eventDigest.String(),
			"closure event digest",
		},
	}
	for _, check := range checks {
		if check.matches {
			continue
		}
		return CommitReceipt{}, storedAdmissionIntegrity(
			"public replay "+check.detail,
			nil,
		)
	}
	return CommitReceipt{
		disposition: CommitReplay,
		eventRef:    expectedEventRef,
		commitRef:   expectedCommitRef,
		revision:    graphRevision,
		digest:      eventDigest,
	}, nil
}

func joinReplayFinishError(primary error, finish error) error {
	return errors.Join(primary, finish)
}
