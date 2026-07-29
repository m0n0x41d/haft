// Package sqlite owns the SQLite effect shell that prepares and persists one
// exact post-Genesis project-TypeEnv successor. It never selects a project
// head or creates authority, Work, activation, or receipts.
package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"

	typedmemorycandidates "github.com/m0n0x41d/haft/data/haft/local-practice/typed-memory/candidates"
	kerneldb "github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/projectgraphobservation"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/kindclassificationengine"
	"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvheadstore"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	profilebasissqlite "github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis/sqlite"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstagerevalidation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvtransitionpreparation"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

// PreparationResult is a closed non-binding outcome.
type PreparationResult interface {
	preparationResultVariant()
}

// Prepared proves that exact immutable B/E/X/C and Stage bytes were persisted
// and reread. It does not establish that the project still has the same head
// after this method returns; selection performs its own exact CAS reread.
type Prepared struct {
	candidate projecttypeenvtransitionpreparation.Candidate
}

func (Prepared) preparationResultVariant() {}

func (result Prepared) Candidate() projecttypeenvtransitionpreparation.Candidate {
	return result.candidate
}

// NoPriorHead keeps Transition distinct from Genesis. The CLI may route this
// result to the separately implemented Genesis preparation path.
type NoPriorHead struct{}

func (NoPriorHead) preparationResultVariant() {}

// AlreadySelected means the bundled target exactly equals the selected
// composite. No Stage or head-selection review is created.
type AlreadySelected struct {
	head projecttypeenvselection.ProjectTypeEnvHeadState
}

func (AlreadySelected) preparationResultVariant() {}

func (result AlreadySelected) Head() projecttypeenvselection.ProjectTypeEnvHeadState {
	return result.head
}

type Service struct {
	ledger ProjectLedger
}

// ProjectLedger is the exact effect boundary required by successor
// preparation. Production supplies the identity-anchored project ledger; tests
// may supply an isolated byte-for-byte database copy with the same immutable
// project coordinates. The service still verifies the durable binding inside
// every database snapshot and never selects the head.
type ProjectLedger interface {
	Database() *sql.DB
	ProjectID() projectledger.ProjectID
	ProjectRoot() projectledger.ProjectRoot
	Revalidate(context.Context) error
}

var _ ProjectLedger = (*projectledger.Handle)(nil)

func NewService(
	ctx context.Context,
	ledger ProjectLedger,
) (*Service, error) {
	if ctx == nil {
		return nil, fmt.Errorf("open Transition preparation service: context is required")
	}
	if ledger == nil {
		return nil, fmt.Errorf("open Transition preparation service: project ledger is required")
	}
	if err := ledger.Revalidate(ctx); err != nil {
		return nil, fmt.Errorf("open Transition preparation service: %w", err)
	}
	return &Service{ledger: ledger}, nil
}

// PrepareAtBase compiles the package-owned target before any write, observes
// one exact current head/graph/profile snapshot, persists immutable successor
// preparation records, and rereads them. A non-nil result returned with an
// error means the semantic outcome exists but project topology could not be
// revalidated afterward.
func (service *Service) PrepareAtBase(
	ctx context.Context,
	base typeenv.BaseTypeEnvArtifact,
) (PreparationResult, error) {
	if ctx == nil {
		return nil, fmt.Errorf("prepare Transition project TypeEnv: context is required")
	}
	if service == nil || service.ledger == nil {
		return nil, fmt.Errorf("prepare Transition project TypeEnv: service is unavailable")
	}
	if err := service.ledger.Revalidate(ctx); err != nil {
		return nil, fmt.Errorf("prepare Transition project TypeEnv: %w", err)
	}
	target, err := localpracticeruntime.Build(
		base,
		typedmemorycandidates.SourceV1_3(),
	)
	if err != nil {
		return nil, fmt.Errorf("prepare Transition project TypeEnv target: %w", err)
	}
	targetRuntime, runtimePresent := target.ExactRuntimeRegistry()
	targetSnapshot, snapshotPresent := target.Preparation().ExecutableSnapshot()
	if !runtimePresent || !snapshotPresent {
		return nil, fmt.Errorf(
			"prepare Transition project TypeEnv target: exact executable C/X is absent",
		)
	}
	if err := kerneldb.RequireCurrentProjectTypeEnvCandidateFootprintReadOnly(
		ctx,
		service.ledger.Database(),
	); err != nil {
		return nil, fmt.Errorf("verify Transition project-TypeEnv candidate storage: %w", err)
	}
	stageStore, err := projecttypeenvstage.OpenExisting(ctx, service.ledger.Database())
	if err != nil {
		return nil, fmt.Errorf("open existing Transition Stage store: %w", err)
	}
	headStore, err := projecttypeenvheadstore.OpenReadOnly(ctx, service.ledger.Database())
	if err != nil {
		return nil, fmt.Errorf("open existing Transition head store: %w", err)
	}
	basis, headPresent, err := service.loadCurrentBasis(
		ctx,
		stageStore,
		headStore,
		targetSnapshot.Environment(),
		targetRuntime,
	)
	if err != nil {
		return nil, err
	}
	if !headPresent {
		return service.finish(ctx, NoPriorHead{})
	}
	if basis.head.SelectedComposite() == target.Composite().Ref() {
		return service.finish(ctx, AlreadySelected{head: basis.head})
	}
	candidate, err := projecttypeenvtransitionpreparation.PrepareCandidate(
		projecttypeenvtransitionpreparation.CandidateInput{
			Project:            service.ledger.ProjectID(),
			ProjectRoot:        basis.profile.ProjectRoot(),
			PriorHead:          basis.head,
			PriorExecutable:    basis.prior,
			Target:             target,
			CurrentGraph:       basis.graph,
			ReferenceKindFacts: basis.referenceKindFacts,
			CurrentProfile:     basis.profile,
		},
	)
	if err != nil {
		return nil, err
	}
	baseSnapshot, err := projectmemory.NewBaseTypeEnvSnapshot(target.Base())
	if err != nil {
		return nil, fmt.Errorf("prepare Transition base snapshot: %w", err)
	}
	snapshotStore, err := typedmemorystore.NewSQLiteSnapshotPort(
		service.ledger.Database(),
		projectmemory.NewBaseTypeEnvLoader(),
		typedmemorystore.SystemClock{},
	)
	if err != nil {
		return nil, fmt.Errorf("open Transition base snapshot store: %w", err)
	}
	if err := snapshotStore.PutTypeEnvSnapshot(ctx, baseSnapshot); err != nil {
		return nil, fmt.Errorf("persist Transition base snapshot: %w", err)
	}
	if err := stageStore.PutArtifactClosure(ctx, candidate.ArtifactClosure()); err != nil {
		return nil, fmt.Errorf("persist Transition B/E/X/C closure: %w", err)
	}
	if err := stageStore.Put(
		ctx,
		candidate.Stage(),
		candidate.Verification().Record(),
		candidate.ExecutableSnapshot().Record(),
	); err != nil {
		return nil, fmt.Errorf("persist Transition Stage: %w", err)
	}
	if err := requireExactReload(ctx, stageStore, candidate); err != nil {
		return nil, err
	}
	result := Prepared{candidate: candidate}
	if err := service.requireCurrentBasis(ctx, headStore, candidate); err != nil {
		return service.finishWithSemanticResult(ctx, result, err)
	}
	return service.finish(ctx, result)
}

type currentBasis struct {
	head               projecttypeenvselection.ProjectTypeEnvHeadState
	prior              projecttypeenv.ProjectTypeEnvExecutableSnapshot
	graph              typedmemorystore.CurrentProjectGraphObservation
	referenceKindFacts projectgraphobservation.ExactTargetReferenceKindFactView
	profile            projecttypeenvprofilebasis.CurrentProjectProfileBasis
}

func (service *Service) loadCurrentBasis(
	ctx context.Context,
	stageStore *projecttypeenvstage.Store,
	headStore *projecttypeenvheadstore.Store,
	target typedmemory.TypeEnv,
	targetRuntime projecttypeenvruntime.ExactTargetRuntimeRegistry,
) (currentBasis, bool, error) {
	transaction, err := sqlitetransaction.BeginRead(ctx, service.ledger.Database())
	if err != nil {
		return currentBasis{}, false,
			fmt.Errorf("begin Transition observation snapshot: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if err := projectledger.RequireExactPersistedBinding(
		ctx,
		transaction,
		service.ledger.ProjectID(),
	); err != nil {
		return currentBasis{}, false,
			fmt.Errorf("verify Transition project binding in snapshot: %w", err)
	}
	headObservation, err := headStore.LoadCurrentProjectTypeEnvHeadTx(
		ctx,
		transaction,
		service.ledger.ProjectID(),
	)
	if err != nil {
		return currentBasis{}, false, err
	}
	if _, absent := headObservation.(projecttypeenvstagerevalidation.ObservedNoProjectTypeEnvHead); absent {
		finish := transaction.Commit(ctx)
		if !finish.Succeeded() {
			return currentBasis{}, false,
				fmt.Errorf("commit Transition absence observation: %w", finish.Err())
		}
		return currentBasis{}, false, nil
	}
	observed, present := headObservation.(projecttypeenvstagerevalidation.ObservedProjectTypeEnvHead)
	if !present {
		return currentBasis{}, false,
			fmt.Errorf("transition head observation variant is invalid: %T", headObservation)
	}
	graph, err := typedmemorystore.LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		service.ledger.ProjectID(),
	)
	if err != nil {
		return currentBasis{}, false, err
	}
	classificationEngine, err := kindclassificationengine.ForExactTargetRuntime(
		targetRuntime,
	)
	if err != nil {
		return currentBasis{}, false,
			fmt.Errorf("construct Transition target-C classification engine: %w", err)
	}
	referenceKindFacts, err := typedmemorystore.LoadExactTargetReferenceKindFactViewTx(
		ctx,
		transaction,
		service.ledger.ProjectID(),
		graph.GraphSnapshotBasis(),
		target,
		targetRuntime,
		classificationEngine,
	)
	if err != nil {
		return currentBasis{}, false,
			fmt.Errorf("load Transition target-C reference-kind facts: %w", err)
	}
	root, err := projectprofile.NewProjectRootV1(service.ledger.ProjectRoot().String())
	if err != nil {
		return currentBasis{}, false,
			fmt.Errorf("bind Transition project root: %w", err)
	}
	profile, err := profilebasissqlite.LoadCurrentWithin(ctx, transaction, root)
	if err != nil {
		return currentBasis{}, false, err
	}
	if profile.ProjectRoot() != root {
		return currentBasis{}, false,
			fmt.Errorf("transition project-profile root differs from anchored project root")
	}
	head := observed.State()
	prior, err := stageStore.LoadExecutableSnapshotTx(
		ctx,
		transaction,
		head.SelectedComposite(),
	)
	if err != nil {
		return currentBasis{}, false,
			fmt.Errorf("load prior selected executable C: %w", err)
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		return currentBasis{}, false,
			fmt.Errorf("commit Transition observation snapshot: %w", finish.Err())
	}
	return currentBasis{
		head:               head,
		prior:              prior,
		graph:              graph,
		referenceKindFacts: referenceKindFacts,
		profile:            profile,
	}, true, nil
}

func (service *Service) requireCurrentBasis(
	ctx context.Context,
	headStore *projecttypeenvheadstore.Store,
	candidate projecttypeenvtransitionpreparation.Candidate,
) error {
	transaction, err := sqlitetransaction.BeginRead(ctx, service.ledger.Database())
	if err != nil {
		return fmt.Errorf("begin Transition post-prepare snapshot: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	graph, err := typedmemorystore.LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		service.ledger.ProjectID(),
	)
	if err != nil {
		return err
	}
	root, err := projectprofile.NewProjectRootV1(service.ledger.ProjectRoot().String())
	if err != nil {
		return err
	}
	profile, err := profilebasissqlite.LoadCurrentWithin(ctx, transaction, root)
	if err != nil {
		return err
	}
	headObservation, err := headStore.LoadCurrentProjectTypeEnvHeadTx(
		ctx,
		transaction,
		service.ledger.ProjectID(),
	)
	if err != nil {
		return err
	}
	head, ok := headObservation.(projecttypeenvstagerevalidation.ObservedProjectTypeEnvHead)
	if !ok {
		return fmt.Errorf("transition prior head disappeared after preparation")
	}
	stage := candidate.Stage()
	if !bytes.Equal(
		head.State().CanonicalBytes(),
		candidate.PriorHead().CanonicalBytes(),
	) {
		return fmt.Errorf("transition prior head changed after preparation")
	}
	graphBasis := graph.GraphSnapshotBasis()
	if graphBasis.Ref() != stage.GraphSnapshotBasis() ||
		graphBasis.Ref().Digest() != stage.GraphSnapshotBasisDigest() ||
		graphBasis.GraphRevision() != stage.GraphRevision() {
		return fmt.Errorf("transition graph changed after preparation")
	}
	if profile.LedgerRevision() != stage.ProfileLedgerRevision() ||
		profile.ProfileLedgerDigest() != stage.ProfileLedgerDigest() {
		return fmt.Errorf("transition project profile changed after preparation")
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		return fmt.Errorf("commit Transition post-prepare snapshot: %w", finish.Err())
	}
	return nil
}

func requireExactReload(
	ctx context.Context,
	store *projecttypeenvstage.Store,
	candidate projecttypeenvtransitionpreparation.Candidate,
) error {
	reloaded, err := store.LoadSelectionReady(ctx, candidate.Stage().Ref())
	if err != nil {
		return fmt.Errorf("reload persisted Transition preparation: %w", err)
	}
	if reloaded.Stage().Ref() != candidate.Stage().Ref() ||
		reloaded.FinalLowererVerification().Ref() != candidate.Verification().Ref() ||
		reloaded.ExecutableSnapshot().TypeEnvRef() != candidate.ExecutableSnapshot().TypeEnvRef() ||
		!bytes.Equal(reloaded.Stage().CanonicalBytes(), candidate.Stage().CanonicalBytes()) ||
		!bytes.Equal(
			reloaded.FinalLowererVerification().CanonicalBytes(),
			candidate.Verification().CanonicalBytes(),
		) ||
		!bytes.Equal(
			reloaded.ExecutableSnapshot().Record().CanonicalBytes(),
			candidate.ExecutableSnapshot().Record().CanonicalBytes(),
		) {
		return fmt.Errorf("persisted Transition preparation differs from exact candidate")
	}
	return nil
}

func (service *Service) finish(
	ctx context.Context,
	result PreparationResult,
) (PreparationResult, error) {
	if result == nil {
		return nil, fmt.Errorf("finish Transition preparation: semantic result is required")
	}
	if err := service.ledger.Revalidate(ctx); err != nil {
		return result,
			fmt.Errorf("revalidate project topology after Transition preparation: %w", err)
	}
	return result, nil
}

func (service *Service) finishWithSemanticResult(
	ctx context.Context,
	result PreparationResult,
	cause error,
) (PreparationResult, error) {
	_, topologyErr := service.finish(ctx, result)
	if topologyErr == nil {
		return result, cause
	}
	return result, errors.Join(cause, topologyErr)
}
