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

func (service *TransitionService) commitOriginalTransition(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	frame currentTransitionFrame,
	input TransitionSelectionInput,
	authorityUse *admittedGenesisAuthorityUse,
	workStartedAt time.Time,
) (genesisTransactionOutcome, error) {
	prepared, err := prepareOriginalTransitionEffect(
		transaction,
		frame,
		input,
		authorityUse,
		workStartedAt,
	)
	if err != nil {
		return genesisTransactionOutcome{}, fmt.Errorf(
			"prepare original Transition effect: %w",
			err,
		)
	}
	var sealed sealedGenesisEffect
	_, err = service.core.activation.WritePreparedProjectTypeEnvActivationGraphTx(
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
			if writeErr := service.writeTransitionEffectTx(
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
			"verify sealed Transition closure: %w",
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
			"precommit exact Transition reread: %w",
			err,
		)
	}
	exact, exactOK := precommit.Exact()
	if !exactOK || !bytes.Equal(
		exact.Closure().CanonicalBytes(),
		sealed.closure.CanonicalBytes(),
	) {
		return genesisTransactionOutcome{},
			fmt.Errorf("precommit Transition closure differs from sealed effect")
	}
	commitSucceeded := service.core.committer.commit(ctx, transaction)
	result := service.core.observeCommittedGenesis(
		ctx,
		GenesisSelectionInput{
			Request: input.Request,
			Content: input.Content,
		},
		sealed.closure,
		commitSucceeded,
	)
	if result != nil {
		return genesisTransactionOutcome{
			result:    result,
			finalized: true,
		}, nil
	}
	unknown, unknownErr := projecttypeenvselectioneffect.NewCommitOutcomeUnknown(
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
