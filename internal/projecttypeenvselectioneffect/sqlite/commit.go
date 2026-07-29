package sqlite

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

func (service *GenesisService) commitOriginalGenesis(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	frame currentGenesisFrame,
	input GenesisSelectionInput,
	authorityUse *admittedGenesisAuthorityUse,
	workStartedAt time.Time,
) (genesisTransactionOutcome, error) {
	prepared, err := prepareOriginalGenesisEffect(
		transaction,
		frame,
		input,
		authorityUse,
		workStartedAt,
	)
	if err != nil {
		return genesisTransactionOutcome{}, fmt.Errorf(
			"prepare original Genesis effect: %w",
			err,
		)
	}
	var sealed sealedGenesisEffect
	_, err = service.activation.WritePreparedProjectTypeEnvActivationGraphTx(
		ctx,
		transaction,
		prepared.graph,
		func(
			callbackContext context.Context,
			callbackTransaction *sqlitetransaction.Transaction,
			writeContext typedmemorystore.ProjectTypeEnvActivationWriteContext,
		) error {
			candidate, sealErr := prepared.seal(writeContext)
			if sealErr != nil {
				return sealErr
			}
			if writeErr := service.writeGenesisEffectTx(
				callbackContext,
				callbackTransaction,
				candidate,
			); writeErr != nil {
				return writeErr
			}
			sealed = candidate
			return nil
		},
	)
	if err != nil {
		return genesisTransactionOutcome{}, err
	}
	if err := sealed.closure.Verify(); err != nil {
		return genesisTransactionOutcome{}, fmt.Errorf(
			"verify sealed Genesis closure: %w",
			err,
		)
	}
	precommit, err := ProbeReplayTx(
		ctx,
		transaction,
		ReplayProbeInput{
			Project:        input.Request.Project(),
			IdempotencyKey: input.Request.IdempotencyKey(),
			RequestDigest:  input.Request.Ref().Digest(),
			ContentDigest:  input.Content.Digest(),
		},
	)
	if err != nil {
		return genesisTransactionOutcome{}, fmt.Errorf(
			"precommit exact Genesis reread: %w",
			err,
		)
	}
	exact, exactOK := precommit.Exact()
	if !exactOK ||
		!bytes.Equal(
			exact.Closure().CanonicalBytes(),
			sealed.closure.CanonicalBytes(),
		) {
		return genesisTransactionOutcome{},
			fmt.Errorf("precommit Genesis closure differs from sealed effect")
	}
	commitSucceeded := service.committer.commit(ctx, transaction)
	result := service.observeCommittedGenesis(
		ctx,
		input,
		sealed.closure,
		commitSucceeded,
	)
	if result != nil {
		return genesisTransactionOutcome{
			result:    result,
			finalized: true,
		}, nil
	}
	unknown, unknownErr :=
		projecttypeenvselectioneffect.NewCommitOutcomeUnknown(
			projecttypeenvselectioneffect.CommitOutcomeUnknownInput{
				RetryKey:      input.Request.IdempotencyKey(),
				RequestDigest: input.Request.Ref().Digest(),
				ContentDigest: input.Content.Digest(),
			},
		)
	return genesisTransactionOutcome{
		result:    unknown,
		finalized: true,
	}, unknownErr
}

func (service *GenesisService) observeCommittedGenesis(
	ctx context.Context,
	input GenesisSelectionInput,
	expected projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
	commitSucceeded bool,
) projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionResult {
	transaction, err := sqlitetransaction.BeginImmediate(ctx, service.database)
	if err != nil {
		return nil
	}
	defer func() {
		_ = transaction.Rollback(ctx)
	}()
	probe, err := ProbeReplayTx(
		ctx,
		transaction,
		ReplayProbeInput{
			Project:        input.Request.Project(),
			IdempotencyKey: input.Request.IdempotencyKey(),
			RequestDigest:  input.Request.Ref().Digest(),
			ContentDigest:  input.Content.Digest(),
		},
	)
	if err != nil {
		return nil
	}
	exact, ok := probe.Exact()
	if !ok ||
		!bytes.Equal(
			exact.Closure().CanonicalBytes(),
			expected.CanonicalBytes(),
		) {
		return nil
	}
	delivery := projecttypeenvselectioneffect.SuccessfulProjectTypeEnvHeadSelectionDeliveryPosture(
		projecttypeenvselectioneffect.CommitRecoveredByExactClosureReread{},
	)
	if commitSucceeded {
		delivery = projecttypeenvselectioneffect.CommittedAndObserved{}
	}
	result, err := projecttypeenvselectioneffect.NewFreshlyCommitted(
		exact.Closure(),
		delivery,
	)
	if err != nil {
		return nil
	}
	return result
}
