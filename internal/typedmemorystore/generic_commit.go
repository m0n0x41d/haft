package typedmemorystore

import (
	"context"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

type pendingGenericAdmissionRefs struct {
	eventRef  string
	commitRef string
}

func (adapter *SQLiteAdapter) CommitMemoryChangeSet(
	ctx context.Context,
	request CommitRequest,
) (CommitReceipt, error) {
	if ctx == nil {
		return CommitReceipt{}, fmt.Errorf("commit typed-memory change set: context is required")
	}
	if adapter == nil || adapter.referenceEngine == nil || adapter.observableInputs == nil {
		return CommitReceipt{}, ErrStorageGenerationUnavailable
	}
	prepared, err := prepareGenericAdmission(request)
	if err != nil {
		return CommitReceipt{}, err
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, adapter.database)
	if err != nil {
		return CommitReceipt{}, fmt.Errorf("begin generic typed-memory transaction: %w", err)
	}
	if err := requireGenericAdmissionStorageCapability(
		ctx,
		transaction,
		request.ContractVersion(),
	); err != nil {
		return CommitReceipt{}, rollbackError(ctx, transaction, err)
	}
	if err := requireAdmissionBasisStorageCapability(
		ctx,
		transaction,
		prepared.basis.Kind(),
	); err != nil {
		return CommitReceipt{}, rollbackError(ctx, transaction, err)
	}
	replay, found, err := loadGenericReplay(
		ctx,
		transaction,
		request,
		prepared,
	)
	if err != nil {
		return CommitReceipt{}, rollbackError(ctx, transaction, err)
	}
	if found {
		if err := rollbackSuccess(ctx, transaction); err != nil {
			return CommitReceipt{}, err
		}
		return replay, nil
	}
	if request.ContractVersion().IsV1() {
		return CommitReceipt{}, rollbackError(
			ctx,
			transaction,
			ErrLegacyAdmissionReplayOnly,
		)
	}

	head, err := loadHeadWithScanner(ctx, transaction, request.project)
	if err != nil {
		return CommitReceipt{}, rollbackError(ctx, transaction, err)
	}
	if head.Revision() != request.expectedRevision {
		return CommitReceipt{}, rollbackError(ctx, transaction, ErrStaleGraphRevision)
	}
	if head.ActiveTypeEnv() != request.expectedTypeEnv {
		return CommitReceipt{}, rollbackError(ctx, transaction, ErrActiveTypeEnvMismatch)
	}
	runtime, err := adapter.resolveTypeEnvRuntimeTx(
		ctx,
		transaction,
		request.project,
		head.Revision(),
		request.expectedTypeEnv,
	)
	if err != nil {
		return CommitReceipt{}, rollbackError(ctx, transaction, err)
	}
	revalidated, err := adapter.revalidateGenericAdmission(
		ctx,
		transaction,
		request,
		prepared,
		runtime.environment,
		runtime.codecs,
		runtime.memberOf,
		runtime.classification,
	)
	if err != nil {
		return CommitReceipt{}, rollbackError(ctx, transaction, err)
	}
	pending, err := adapter.persistGenericAdmission(
		ctx,
		transaction,
		request,
		revalidated,
		runtime.environment,
	)
	if err != nil {
		return CommitReceipt{}, rollbackError(ctx, transaction, err)
	}
	if err := verifyPendingGenericAdmission(
		ctx,
		transaction,
		request,
		revalidated.prepared,
		pending,
	); err != nil {
		return CommitReceipt{}, rollbackError(ctx, transaction, err)
	}
	if ctx.Err() != nil {
		return CommitReceipt{}, rollbackError(ctx, transaction, ctx.Err())
	}

	finish := adapter.finisher.Commit(ctx, transaction)
	disposition := CommitApplied
	if !finish.Succeeded() {
		disposition = CommitRecovered
	}
	receipt, durableErr := adapter.resolveDurableGeneric(
		request,
		revalidated.prepared,
		pending.eventRef,
		pending.commitRef,
		disposition,
	)
	if durableErr == nil {
		return receipt, nil
	}
	if !finish.Succeeded() {
		return CommitReceipt{}, fmt.Errorf(
			"%w for project %s and idempotency key %q; retry the same exact request to resolve: %v",
			ErrCommitOutcomeUnknown,
			request.project.String(),
			request.idempotencyKey.String(),
			errors.Join(finish.Err(), durableErr),
		)
	}
	return CommitReceipt{}, fmt.Errorf(
		"%w for project %s and idempotency key %q; retry the same exact request to resolve durable reread failure: %v",
		ErrCommitOutcomeUnknown,
		request.project.String(),
		request.idempotencyKey.String(),
		durableErr,
	)
}

func verifyPendingGenericAdmission(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	request CommitRequest,
	prepared preparedAdmission,
	pending pendingGenericAdmissionRefs,
) error {
	expectation, err := newDurableGenericAdmissionExpectation(request, prepared)
	if err != nil {
		return err
	}
	expectation.requiredEventRef = pending.eventRef
	expectation.requiredCommitRef = pending.commitRef
	_, found, err := resolveGenericIdempotencyReplay(
		ctx,
		transaction,
		expectation,
		CommitApplied,
	)
	if err != nil {
		return fmt.Errorf("verify pending generic typed-memory materialization: %w", err)
	}
	if !found {
		return fmt.Errorf("verify pending generic typed-memory materialization: durable rows are missing")
	}
	return nil
}
