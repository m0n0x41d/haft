package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvheadstore"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

// GenesisSelectionInput contains immutable proposal/source records only. It
// deliberately contains no Method, MethodDescription, StatePlane, resource,
// outcome, acceptance, audit, authority-resolution, live authority-use, or
// transaction read-set value. The service derives those after exact replay
// has been ruled out inside its own BEGIN IMMEDIATE transaction.
type GenesisSelectionInput struct {
	Request   projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	Content   projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent
	Authority GenesisAuthorityIngress
}

type GenesisService struct {
	database         *sql.DB
	projectRoot      projectprofile.ProjectRootV1
	stages           *projecttypeenvstage.Store
	heads            *projecttypeenvheadstore.Store
	activation       *typedmemorystore.ProjectTypeEnvActivationAdapter
	clock            typedmemorystore.Clock
	installedRuntime projecttypeenvruntime.InstalledRuntimeRegistryInput
	committer        genesisTransactionCommitter
}

// genesisTransactionCommitter is a private fault seam. Every implementation
// must finish the supplied transaction exactly once. Production reports the
// concrete COMMIT result; tests may physically commit or roll back and then
// hide that outcome to exercise the independent closure reread.
type genesisTransactionCommitter interface {
	commit(
		context.Context,
		*sqlitetransaction.Transaction,
	) bool
}

type sqliteGenesisTransactionCommitter struct{}

func (sqliteGenesisTransactionCommitter) commit(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
) bool {
	return transaction.Commit(ctx).Succeeded()
}

type genesisTransactionOutcome struct {
	result              projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionResult
	finalized           bool
	exactReplayRuledOut bool
}

func NewGenesisService(
	ctx context.Context,
	database *sql.DB,
	projectRoot string,
	installedRuntime projecttypeenvruntime.InstalledRuntimeRegistryInput,
	clock typedmemorystore.Clock,
) (*GenesisService, error) {
	if ctx == nil {
		return nil, sqlitetransaction.ErrContextRequired
	}
	if database == nil {
		return nil, sqlitetransaction.ErrDatabaseRequired
	}
	root, err := projectprofile.NewProjectRootV1(strings.TrimSpace(projectRoot))
	if err != nil {
		return nil, fmt.Errorf("new Genesis service project root: %w", err)
	}
	if clock == nil {
		return nil, fmt.Errorf("new Genesis service: clock is required")
	}
	stages, err := projecttypeenvstage.New(ctx, database)
	if err != nil {
		return nil, fmt.Errorf("new Genesis service Stage store: %w", err)
	}
	heads, err := projecttypeenvheadstore.New(ctx, database)
	if err != nil {
		return nil, fmt.Errorf("new Genesis service head store: %w", err)
	}
	activation, err := typedmemorystore.NewProjectTypeEnvActivationAdapter(
		clock,
		stages,
	)
	if err != nil {
		return nil, fmt.Errorf("new Genesis service activation adapter: %w", err)
	}
	return &GenesisService{
		database:         database,
		projectRoot:      root,
		stages:           stages,
		heads:            heads,
		activation:       activation,
		clock:            clock,
		installedRuntime: installedRuntime,
		committer:        sqliteGenesisTransactionCommitter{},
	}, nil
}

func (service *GenesisService) SelectGenesis(
	ctx context.Context,
	input GenesisSelectionInput,
) (projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionResult, error) {
	if err := service.verifyInput(ctx, input); err != nil {
		return nil, err
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, service.database)
	if err != nil {
		reason, selected := preCommitNotSelectedReason(err)
		if selected {
			return projecttypeenvselectioneffect.NewNotSelected(reason)
		}
		return nil, fmt.Errorf("begin Genesis head selection: %w", err)
	}
	outcome, effectErr := service.selectGenesisTx(ctx, transaction, input)
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
				"finalized Genesis transaction produced no result",
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
	return nil, fmt.Errorf("genesis head selection produced no result")
}

func (service *GenesisService) selectGenesisTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	input GenesisSelectionInput,
) (genesisTransactionOutcome, error) {
	// This call is intentionally the literal first semantic database
	// operation after BEGIN IMMEDIATE. Do not move current config, Stage,
	// profile, graph, head, or authority reads above it.
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
		result, resultErr :=
			projecttypeenvselectioneffect.NewReplayedExisting(exact.Closure())
		return genesisTransactionOutcome{result: result}, resultErr
	}
	if conflict, ok := probe.Conflict(); ok {
		return genesisTransactionOutcome{result: conflict.Conflict()}, nil
	}
	if _, absent := probe.Absent(); !absent {
		return genesisTransactionOutcome{},
			fmt.Errorf("genesis replay probe returned an invalid variant")
	}
	replayAbsent := genesisTransactionOutcome{exactReplayRuledOut: true}
	workStartedAt := service.clock.Now()
	frameResult, err := loadCurrentGenesisFrameTx(
		ctx,
		transaction,
		currentGenesisFrameDependencies{
			stages:           service.stages,
			heads:            service.heads,
			installedRuntime: service.installedRuntime,
			observedAt:       service.clock.Now(),
		},
		input.Request,
	)
	if err != nil {
		return replayAbsent,
			fmt.Errorf("load current Genesis frame: %w", err)
	}
	frameReady, ok := frameResult.(currentGenesisFrameReady)
	if rejected, rejectedOK :=
		frameResult.(currentGenesisFrameRejected); rejectedOK {
		return genesisNotSelectedOutcome(rejected.reason)
	}
	if !ok {
		return replayAbsent, fmt.Errorf(
			"current Genesis frame returned an invalid result variant: %T",
			frameResult,
		)
	}
	frame := frameReady.frame
	if frame.projectRoot != service.projectRoot {
		return genesisNotSelectedOutcome(
			projecttypeenvselectioneffect.NotSelectedCurrentAuthorityRejection(),
		)
	}
	authorityResult, err := service.resolveCurrentAuthority(
		ctx,
		transaction,
		frame,
		input,
	)
	if err != nil {
		return replayAbsent,
			fmt.Errorf("resolve current Genesis authority: %w", err)
	}
	authorityReady, ok := authorityResult.(currentGenesisAuthorityReady)
	if rejected, rejectedOK :=
		authorityResult.(currentGenesisAuthorityRejected); rejectedOK {
		return genesisNotSelectedOutcome(rejected.reason)
	}
	if !ok || authorityReady.use == nil {
		return replayAbsent, fmt.Errorf(
			"current Genesis authority returned an invalid result variant: %T",
			authorityResult,
		)
	}
	outcome, err := service.commitOriginalGenesis(
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

func (service *GenesisService) verifyInput(
	ctx context.Context,
	input GenesisSelectionInput,
) error {
	if ctx == nil {
		return sqlitetransaction.ErrContextRequired
	}
	if service == nil ||
		service.database == nil ||
		service.stages == nil ||
		service.heads == nil ||
		service.activation == nil ||
		service.clock == nil ||
		service.committer == nil {
		return fmt.Errorf("genesis service is invalid")
	}
	if err := input.Request.Verify(); err != nil {
		return fmt.Errorf("genesis selection request: %w", err)
	}
	if err := input.Content.ExactAgainst(input.Request); err != nil {
		return fmt.Errorf("genesis authorization content: %w", err)
	}
	return nil
}
