// Package sqlite owns the dormant SQLite effect shell that prepares and
// persists one exact Genesis project-TypeEnv candidate. It never selects a
// project-TypeEnv head or creates authority, Work, activation, or receipts.
package sqlite

import (
	"bytes"
	"context"
	"fmt"

	typedmemorycandidates "github.com/m0n0x41d/haft/data/haft/local-practice/typed-memory/candidates"
	kerneldb "github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvpreparation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	profilebasissqlite "github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis/sqlite"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

// PreparationResult is a closed effect outcome. None of its variants is a
// project-TypeEnv head, a selection act, or an authority receipt.
type PreparationResult interface {
	preparationResultVariant()
}

type graphInitializationPosture uint8

const (
	graphInitializedAtBase graphInitializationPosture = iota + 1
	graphAlreadyExactAtBase
)

type preparedAtBase struct {
	candidate projecttypeenvpreparation.GenesisCandidate
}

func (result preparedAtBase) Candidate() projecttypeenvpreparation.GenesisCandidate {
	return result.candidate
}

// PreparedAtNewBase proves that this invocation initialized revision zero and
// then persisted and reloaded exact B/E/X/C and Stage bytes.
type PreparedAtNewBase struct{ preparedAtBase }

func (PreparedAtNewBase) preparationResultVariant() {}

// PreparedAtExistingExactBase proves exact replay over an already-existing
// revision-zero B followed by exact B/E/X/C and Stage persistence/reload.
type PreparedAtExistingExactBase struct{ preparedAtBase }

func (PreparedAtExistingExactBase) preparationResultVariant() {}

// GraphAlreadyActive preserves the exact initializer observation and performs
// no B/E/X/C or Stage persistence.
type GraphAlreadyActive struct {
	observation typedmemorystore.AlreadyActive
}

func (GraphAlreadyActive) preparationResultVariant() {}

func (result GraphAlreadyActive) Observation() typedmemorystore.AlreadyActive {
	return result.observation
}

type graphAdvancedBeforePreparation struct {
	observation typedmemorystore.CurrentProjectGraphObservation
}

func (result graphAdvancedBeforePreparation) Observation() typedmemorystore.CurrentProjectGraphObservation {
	return result.observation
}

// GraphAdvancedAfterNewBase means this invocation initialized revision zero,
// but another writer advanced the graph before the preparation snapshot. No
// B/E/X/C or Stage record is persisted by this invocation.
type GraphAdvancedAfterNewBase struct {
	graphAdvancedBeforePreparation
}

func (GraphAdvancedAfterNewBase) preparationResultVariant() {}

// GraphAdvancedAfterExistingExactBase means revision zero already existed and
// another writer advanced the graph before the preparation snapshot. No
// B/E/X/C or Stage record is persisted by this invocation.
type GraphAdvancedAfterExistingExactBase struct {
	graphAdvancedBeforePreparation
}

func (GraphAdvancedAfterExistingExactBase) preparationResultVariant() {}

// BaseSnapshotConflict preserves the exact existing/presented coordinate and
// performs no B/E/X/C or Stage persistence.
type BaseSnapshotConflict struct {
	observation typedmemorystore.Conflict
}

func (BaseSnapshotConflict) preparationResultVariant() {}

func (result BaseSnapshotConflict) Observation() typedmemorystore.Conflict {
	return result.observation
}

// Service retains only an already-anchored project ledger and an explicit
// clock. The caller owns the ledger handle lifecycle.
type Service struct {
	ledger *projectledger.Handle
	clock  typedmemorystore.Clock
}

func NewService(
	ctx context.Context,
	ledger *projectledger.Handle,
	clock typedmemorystore.Clock,
) (*Service, error) {
	if ctx == nil {
		return nil, fmt.Errorf("open Genesis preparation service: context is required")
	}
	if ledger == nil {
		return nil, fmt.Errorf("open Genesis preparation service: project ledger is required")
	}
	if clock == nil {
		return nil, fmt.Errorf("open Genesis preparation service: clock is required")
	}
	if err := ledger.Revalidate(ctx); err != nil {
		return nil, fmt.Errorf("open Genesis preparation service: %w", err)
	}
	return &Service{
		ledger: ledger,
		clock:  clock,
	}, nil
}

// PrepareAtBase compiles the package-owned Local-Practice source before any
// write, verifies the existing storage footprint without repair, initializes
// only the revision-zero base graph, rereads graph+profile in one snapshot,
// persists immutable preparation records, and rereads them as an exact storage
// proof.
//
// The graph initialization, artifact closure, and Stage are intentionally
// separate monotonic transactions. An error after initialization can leave
// exact revision-zero B, and an error after closure persistence can leave exact
// immutable B/E/X/C without a Stage. Both are non-binding and retry-safe; a nil
// result must not be interpreted as proof that no write occurred.
//
// Any non-nil result returned together with an error means the semantic outcome
// was observed against the anchored database but final topology revalidation
// failed; callers must retain the result and reconcile project topology.
func (service *Service) PrepareAtBase(
	ctx context.Context,
	base typeenv.BaseTypeEnvArtifact,
) (PreparationResult, error) {
	if ctx == nil {
		return nil, fmt.Errorf("prepare Genesis project TypeEnv: context is required")
	}
	if service == nil || service.ledger == nil || service.clock == nil {
		return nil, fmt.Errorf("prepare Genesis project TypeEnv: service is unavailable")
	}
	if err := service.ledger.Revalidate(ctx); err != nil {
		return nil, fmt.Errorf("prepare Genesis project TypeEnv: %w", err)
	}

	target, err := localpracticeruntime.Build(
		base,
		typedmemorycandidates.SourceV1_4(),
	)
	if err != nil {
		return nil, fmt.Errorf("prepare Genesis project TypeEnv target: %w", err)
	}
	baseSnapshot, err := projectmemory.NewBaseTypeEnvSnapshot(target.Base())
	if err != nil {
		return nil, err
	}
	if err := kerneldb.RequireCurrentProjectTypeEnvCandidateFootprintReadOnly(
		ctx,
		service.ledger.Database(),
	); err != nil {
		return nil, fmt.Errorf(
			"verify Genesis project-TypeEnv candidate storage: %w",
			err,
		)
	}
	stageStore, err := projecttypeenvstage.OpenExisting(
		ctx,
		service.ledger.Database(),
	)
	if err != nil {
		return nil, fmt.Errorf("open existing Genesis Stage store: %w", err)
	}
	initializer, err := typedmemorystore.NewSQLiteProjectGraphInitializer(
		service.ledger.Database(),
		projectmemory.NewBaseTypeEnvLoader(),
		service.clock,
	)
	if err != nil {
		return nil, fmt.Errorf("open Genesis graph initializer: %w", err)
	}
	initialization, err := initializer.InitializeProjectGraphAtBaseTypeEnv(
		ctx,
		service.ledger.ProjectID(),
		baseSnapshot,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize Genesis project graph: %w", err)
	}
	posture, terminal, err := classifyInitialization(initialization)
	if err != nil {
		return nil, err
	}
	if terminal != nil {
		return service.finish(ctx, terminal)
	}

	graph, profile, err := service.loadCurrentBasis(ctx)
	if err != nil {
		return nil, err
	}
	if graph.GraphSnapshotBasis().GraphRevision().Value() != 0 {
		advanced, err := advancedResult(posture, graph)
		if err != nil {
			return nil, err
		}
		return service.finish(ctx, advanced)
	}
	candidate, err := projecttypeenvpreparation.PrepareGenesisCandidate(
		projecttypeenvpreparation.GenesisCandidateInput{
			Project:        service.ledger.ProjectID(),
			ProjectRoot:    profile.ProjectRoot(),
			Target:         target,
			CurrentGraph:   graph,
			CurrentProfile: profile,
		},
	)
	if err != nil {
		return nil, err
	}
	if err := stageStore.PutArtifactClosure(
		ctx,
		candidate.ArtifactClosure(),
	); err != nil {
		return nil, fmt.Errorf("persist Genesis B/E/X/C closure: %w", err)
	}
	if err := stageStore.Put(
		ctx,
		candidate.Stage(),
		candidate.Verification().Record(),
		candidate.ExecutableSnapshot().Record(),
	); err != nil {
		return nil, fmt.Errorf("persist Genesis Stage: %w", err)
	}
	if err := requireExactReload(ctx, stageStore, candidate); err != nil {
		return nil, err
	}
	result, err := preparedResult(posture, candidate)
	if err != nil {
		return nil, err
	}
	return service.finish(ctx, result)
}

func classifyInitialization(
	result typedmemorystore.ProjectGraphInitializationResult,
) (graphInitializationPosture, PreparationResult, error) {
	switch observation := result.(type) {
	case typedmemorystore.InitializedAtBase:
		return graphInitializedAtBase, nil, nil
	case typedmemorystore.AlreadyExactAtBase:
		return graphAlreadyExactAtBase, nil, nil
	case typedmemorystore.AlreadyActive:
		return 0, GraphAlreadyActive{observation: observation}, nil
	case typedmemorystore.Conflict:
		return 0, BaseSnapshotConflict{observation: observation}, nil
	default:
		return 0, nil, fmt.Errorf(
			"initialize Genesis project graph: unsupported outcome %T",
			result,
		)
	}
}

func preparedResult(
	posture graphInitializationPosture,
	candidate projecttypeenvpreparation.GenesisCandidate,
) (PreparationResult, error) {
	value := preparedAtBase{candidate: candidate}
	switch posture {
	case graphInitializedAtBase:
		return PreparedAtNewBase{preparedAtBase: value}, nil
	case graphAlreadyExactAtBase:
		return PreparedAtExistingExactBase{preparedAtBase: value}, nil
	default:
		return nil, fmt.Errorf(
			"prepare Genesis project TypeEnv: invalid initialization posture %d",
			posture,
		)
	}
}

func advancedResult(
	posture graphInitializationPosture,
	observation typedmemorystore.CurrentProjectGraphObservation,
) (PreparationResult, error) {
	value := graphAdvancedBeforePreparation{observation: observation}
	switch posture {
	case graphInitializedAtBase:
		return GraphAdvancedAfterNewBase{
			graphAdvancedBeforePreparation: value,
		}, nil
	case graphAlreadyExactAtBase:
		return GraphAdvancedAfterExistingExactBase{
			graphAdvancedBeforePreparation: value,
		}, nil
	default:
		return nil, fmt.Errorf(
			"prepare Genesis project TypeEnv: invalid advanced-graph posture %d",
			posture,
		)
	}
}

func (service *Service) finish(
	ctx context.Context,
	result PreparationResult,
) (PreparationResult, error) {
	if result == nil {
		return nil, fmt.Errorf(
			"finish Genesis preparation: semantic result is required",
		)
	}
	if err := service.ledger.Revalidate(ctx); err != nil {
		return result, fmt.Errorf(
			"revalidate project topology after Genesis preparation: %w",
			err,
		)
	}
	return result, nil
}

func (service *Service) loadCurrentBasis(
	ctx context.Context,
) (
	typedmemorystore.CurrentProjectGraphObservation,
	projecttypeenvprofilebasis.CurrentProjectProfileBasis,
	error,
) {
	transaction, err := sqlitetransaction.BeginRead(
		ctx,
		service.ledger.Database(),
	)
	if err != nil {
		return typedmemorystore.CurrentProjectGraphObservation{},
			nil,
			fmt.Errorf("begin Genesis observation snapshot: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if err := projectledger.RequireExactPersistedBinding(
		ctx,
		transaction,
		service.ledger.ProjectID(),
	); err != nil {
		return typedmemorystore.CurrentProjectGraphObservation{},
			nil,
			fmt.Errorf("verify Genesis project binding in snapshot: %w", err)
	}
	graph, err := typedmemorystore.LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		service.ledger.ProjectID(),
	)
	if err != nil {
		return typedmemorystore.CurrentProjectGraphObservation{}, nil, err
	}
	root, err := projectprofile.NewProjectRootV1(
		service.ledger.ProjectRoot().String(),
	)
	if err != nil {
		return typedmemorystore.CurrentProjectGraphObservation{},
			nil,
			fmt.Errorf("bind Genesis project root: %w", err)
	}
	profile, err := profilebasissqlite.LoadCurrentWithin(
		ctx,
		transaction,
		root,
	)
	if err != nil {
		return typedmemorystore.CurrentProjectGraphObservation{}, nil, err
	}
	if profile.ProjectRoot() != root {
		return typedmemorystore.CurrentProjectGraphObservation{},
			nil,
			fmt.Errorf("genesis project-profile root differs from anchored project root")
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		return typedmemorystore.CurrentProjectGraphObservation{},
			nil,
			fmt.Errorf("commit Genesis observation snapshot: %w", finish.Err())
	}
	return graph, profile, nil
}

func requireExactReload(
	ctx context.Context,
	store *projecttypeenvstage.Store,
	candidate projecttypeenvpreparation.GenesisCandidate,
) error {
	reloaded, err := store.LoadSelectionReady(ctx, candidate.Stage().Ref())
	if err != nil {
		return fmt.Errorf("reload persisted Genesis preparation: %w", err)
	}
	if reloaded.Stage().Ref() != candidate.Stage().Ref() ||
		reloaded.FinalLowererVerification().Ref() != candidate.Verification().Ref() ||
		reloaded.ExecutableSnapshot().TypeEnvRef() != candidate.ExecutableSnapshot().TypeEnvRef() ||
		!bytes.Equal(
			reloaded.Stage().CanonicalBytes(),
			candidate.Stage().CanonicalBytes(),
		) ||
		!bytes.Equal(
			reloaded.FinalLowererVerification().CanonicalBytes(),
			candidate.Verification().CanonicalBytes(),
		) ||
		!bytes.Equal(
			reloaded.ExecutableSnapshot().Record().CanonicalBytes(),
			candidate.ExecutableSnapshot().Record().CanonicalBytes(),
		) {
		return fmt.Errorf("persisted Genesis preparation differs from exact candidate")
	}
	return nil
}
