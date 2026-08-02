package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

// TransitionSelectionInput names one exact post-Genesis head transition. The
// prior head is carried only by Request.Predecessor; absence never degrades to
// Genesis and no caller-provided current-world observation is accepted.
type TransitionSelectionInput struct {
	Request   projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	Content   projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent
	Authority GenesisAuthorityIngress
}

// TransitionService reuses the same physical stores and authority adapters as
// Genesis while preserving a distinct public operation and predecessor
// algebra. The embedded service is an implementation dependency, not a
// semantic fallback.
type TransitionService struct {
	core *GenesisService
}

func NewTransitionService(
	ctx context.Context,
	database *sql.DB,
	projectRoot string,
	installedRuntime projecttypeenvruntime.InstalledRuntimeRegistryInput,
	clock typedmemorystore.Clock,
) (*TransitionService, error) {
	core, err := NewGenesisService(
		ctx,
		database,
		projectRoot,
		installedRuntime,
		clock,
	)
	if err != nil {
		return nil, fmt.Errorf("new Transition service: %w", err)
	}
	return &TransitionService{core: core}, nil
}

func (service *TransitionService) SelectTransition(
	ctx context.Context,
	input TransitionSelectionInput,
) (projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionResult, error) {
	if err := service.verifyInput(ctx, input); err != nil {
		return nil, err
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, service.core.database)
	if err != nil {
		reason, selected := preCommitNotSelectedReason(err)
		if selected {
			return projecttypeenvselectioneffect.NewNotSelected(reason)
		}
		return nil, fmt.Errorf("begin Transition head selection: %w", err)
	}
	outcome, effectErr := service.selectTransitionTx(ctx, transaction, input)
	if effectErr != nil {
		if outcome.finalized {
			return nil, effectErr
		}
		rollback := transaction.Rollback(context.WithoutCancel(ctx))
		if !rollback.Succeeded() {
			return nil, errors.Join(effectErr, rollback.Err())
		}
		reason, selected := preCommitNotSelectedReason(effectErr)
		if outcome.exactReplayRuledOut && selected {
			return projecttypeenvselectioneffect.NewNotSelected(reason)
		}
		return nil, effectErr
	}
	if outcome.finalized {
		if outcome.result == nil {
			return nil, fmt.Errorf(
				"finalized Transition transaction produced no result",
			)
		}
		return outcome.result, nil
	}
	if outcome.result != nil {
		rollback := transaction.Rollback(context.WithoutCancel(ctx))
		if !rollback.Succeeded() {
			return nil, rollback.Err()
		}
		return outcome.result, nil
	}
	return nil, fmt.Errorf("transition head selection produced no result")
}

func (service *TransitionService) selectTransitionTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	input TransitionSelectionInput,
) (genesisTransactionOutcome, error) {
	// Exact replay is intentionally the literal first semantic database read.
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
		return genesisTransactionOutcome{}, err
	}
	if exact, ok := probe.Exact(); ok {
		result, resultErr := projecttypeenvselectioneffect.NewReplayedExisting(
			exact.Closure(),
		)
		return genesisTransactionOutcome{result: result}, resultErr
	}
	if conflict, ok := probe.Conflict(); ok {
		return genesisTransactionOutcome{result: conflict.Conflict()}, nil
	}
	if _, absent := probe.Absent(); !absent {
		return genesisTransactionOutcome{},
			fmt.Errorf("transition replay probe returned an invalid variant")
	}
	replayAbsent := genesisTransactionOutcome{exactReplayRuledOut: true}
	workStartedAt := service.core.clock.Now()
	frameResult, err := loadCurrentTransitionFrameTx(
		ctx,
		transaction,
		currentGenesisFrameDependencies{
			stages:           service.core.stages,
			heads:            service.core.heads,
			installedRuntime: service.core.installedRuntime,
			observedAt:       service.core.clock.Now(),
		},
		input.Request,
	)
	if err != nil {
		return replayAbsent, fmt.Errorf("load current Transition frame: %w", err)
	}
	frameReady, ok := frameResult.(currentTransitionFrameReady)
	if rejected, rejectedOK := frameResult.(currentTransitionFrameRejected); rejectedOK {
		return genesisNotSelectedOutcome(rejected.reason)
	}
	if !ok {
		return replayAbsent, fmt.Errorf(
			"current Transition frame returned an invalid result variant: %T",
			frameResult,
		)
	}
	frame := frameReady.frame
	if frame.projectRoot != service.core.projectRoot {
		return genesisNotSelectedOutcome(
			projecttypeenvselectioneffect.NotSelectedCurrentAuthorityRejection(),
		)
	}
	authorityResult, err := resolveCurrentHeadSelectionAuthority(
		ctx,
		transaction,
		service.core.projectRoot,
		service.core.clock,
		currentHeadSelectionAuthorityInput{
			request:   input.Request,
			content:   input.Content,
			authority: input.Authority,
			profile:   frame.currentProfile,
		},
	)
	if err != nil {
		return replayAbsent, fmt.Errorf("resolve current Transition authority: %w", err)
	}
	authorityReady, ok := authorityResult.(currentGenesisAuthorityReady)
	if rejected, rejectedOK := authorityResult.(currentGenesisAuthorityRejected); rejectedOK {
		return genesisNotSelectedOutcome(rejected.reason)
	}
	if !ok || authorityReady.use == nil {
		return replayAbsent, fmt.Errorf(
			"current Transition authority returned an invalid result variant: %T",
			authorityResult,
		)
	}
	outcome, err := service.commitOriginalTransition(
		ctx,
		transaction,
		frame,
		input,
		authorityReady.use,
		workStartedAt,
	)
	outcome.exactReplayRuledOut = true
	return outcome, err
}

func (service *TransitionService) verifyInput(
	ctx context.Context,
	input TransitionSelectionInput,
) error {
	if ctx == nil {
		return sqlitetransaction.ErrContextRequired
	}
	if service == nil || service.core == nil {
		return fmt.Errorf("transition service is invalid")
	}
	if err := service.core.verifyInput(
		ctx,
		GenesisSelectionInput(input),
	); err != nil {
		return err
	}
	if _, ok := input.Request.Predecessor().(projecttypeenvselection.TransitionStagePredecessor); !ok {
		return fmt.Errorf("transition selection request requires an exact prior head")
	}
	return nil
}
