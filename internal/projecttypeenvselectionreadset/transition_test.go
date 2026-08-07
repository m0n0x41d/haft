package projecttypeenvselectionreadset

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvassertionreport"
	"github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilecompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type transitionReadSetFixture struct {
	genesisReadSetFixture
	prior projecttypeenvselection.ProjectTypeEnvHeadState
	input TransitionHeadObservationInput
}

func TestObserveTransitionMintsExactReadSet(t *testing.T) {
	fixture := newTransitionReadSetFixture(t, 9)
	ctx := context.Background()
	seedTransitionPriorHead(t, fixture)
	transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(): %v", err)
	}
	outcome, err := ObserveTransitionHeadTx(
		ctx,
		fixture.store,
		transaction,
		fixture.input,
	)
	if err != nil {
		t.Fatalf("ObserveTransitionHeadTx(): %v", err)
	}
	readSet, ok := outcome.(TransitionHeadSelectionReadSet)
	if !ok {
		t.Fatalf("outcome = %T; want TransitionHeadSelectionReadSet", outcome)
	}
	if err := readSet.VerifyForTransaction(transaction); err != nil {
		t.Fatalf("VerifyForTransaction(): %v", err)
	}
	prior, exists := readSet.PriorHead()
	if !exists || !sameHeadState(prior, fixture.prior) {
		t.Fatal("Transition read set lost exact prior head")
	}
	successor, exists := readSet.SuccessorHead()
	if !exists {
		t.Fatal("Transition read set omitted successor")
	}
	if successor.Revision().Value() != 2 ||
		successor.SelectedComposite() != fixture.stage.VerifiedComposite() {
		t.Fatalf(
			"successor = revision %d C %s",
			successor.Revision().Value(),
			successor.SelectedComposite().String(),
		)
	}
	graphRevision, exists := readSet.CommittedGraphRevision()
	if !exists || graphRevision.Value() != 10 {
		t.Fatalf("committed GraphRevision = %#v", graphRevision)
	}
	if _, err := json.Marshal(readSet); !errors.Is(
		err,
		ErrTransitionHeadSelectionReadSetNotSerializable,
	) {
		t.Fatalf("MarshalJSON error = %v", err)
	}
	copy := *transaction
	if err := readSet.VerifyForTransaction(&copy); !errors.Is(
		err,
		ErrTransitionHeadSelectionTransactionMismatch,
	) {
		t.Fatalf("copied transaction verification error = %v", err)
	}
	if finish := transaction.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback: %v", finish.Err())
	}
}

func TestObserveTransitionRefusesAbsentHead(t *testing.T) {
	fixture := newTransitionReadSetFixture(t, 3)
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(): %v", err)
	}
	outcome, err := ObserveTransitionHeadTx(
		ctx,
		fixture.store,
		transaction,
		fixture.input,
	)
	if err != nil {
		t.Fatalf("ObserveTransitionHeadTx(): %v", err)
	}
	absent, ok := outcome.(TransitionHeadAbsent)
	if !ok {
		t.Fatalf("outcome = %T; want TransitionHeadAbsent", outcome)
	}
	if absent.Project() != fixture.project || absent.Head() != fixture.prior.Ref() {
		t.Fatal("Transition absence lost project/head coordinate")
	}
	assertHeadRowsTx(t, ctx, transaction, 0, 0)
	if finish := transaction.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback: %v", finish.Err())
	}
}

func TestObserveTransitionReturnsStalePredecessor(t *testing.T) {
	fixture := newTransitionReadSetFixture(t, 5)
	ctx := context.Background()
	seedTransitionPriorHead(t, fixture)
	currentComposite, err := typedmemory.NewTypeEnvRef(readSetDigest(t, "e"))
	if err != nil {
		t.Fatalf("NewTypeEnvRef(): %v", err)
	}
	current := readSetHeadState(t, fixture.project, currentComposite, 2)
	write, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(advance): %v", err)
	}
	if err := fixture.store.CompareAndSwapTransitionProjectTypeEnvHeadTx(
		ctx,
		write,
		fixture.prior,
		current,
	); err != nil {
		t.Fatalf("advance current head: %v", err)
	}
	if finish := write.Commit(ctx); !finish.Succeeded() {
		t.Fatalf("commit current head: %v", finish.Err())
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(observe): %v", err)
	}
	outcome, err := ObserveTransitionHeadTx(
		ctx,
		fixture.store,
		transaction,
		fixture.input,
	)
	if err != nil {
		t.Fatalf("ObserveTransitionHeadTx(): %v", err)
	}
	conflict, ok := outcome.(TransitionPredecessorConflict)
	if !ok {
		t.Fatalf("outcome = %T; want TransitionPredecessorConflict", outcome)
	}
	expected, expectedOK := conflict.ExpectedHead()
	observed, observedOK := conflict.CurrentHead()
	if !expectedOK || !observedOK ||
		!sameHeadState(expected, fixture.prior) ||
		!sameHeadState(observed, current) {
		t.Fatal("stale predecessor outcome lost exact expected/current states")
	}
	if finish := transaction.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback: %v", finish.Err())
	}
}

func newTransitionReadSetFixture(
	t *testing.T,
	graphRevision uint64,
) transitionReadSetFixture {
	t.Helper()
	genesis := newGenesisReadSetFixture(t, graphRevision)
	priorRef, err := typedmemory.NewTypeEnvRef(readSetDigest(t, "c"))
	if err != nil {
		t.Fatalf("NewTypeEnvRef(prior): %v", err)
	}
	prior := readSetHeadState(t, genesis.project, priorRef, 1)
	stage := readSetTransitionStage(
		t,
		genesis.project,
		genesis.graph,
		prior,
	)
	key, err := projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
		"transition-readset",
	)
	if err != nil {
		t.Fatalf("NewProjectTypeEnvHeadSelectionIdempotencyKey(): %v", err)
	}
	request, err := projecttypeenvselection.SealTransitionProjectTypeEnvHeadSelectionRequest(
		projecttypeenvselection.TransitionProjectTypeEnvHeadSelectionRequestInput{
			Project:               genesis.project,
			ExactPriorHead:        prior,
			Stage:                 stage,
			ExpectedGraphRevision: genesis.graph.GraphRevision(),
			IdempotencyKey:        key,
		},
	)
	if err != nil {
		t.Fatalf("SealTransitionProjectTypeEnvHeadSelectionRequest(): %v", err)
	}
	genesis.stage = stage
	genesis.request = request
	return transitionReadSetFixture{
		genesisReadSetFixture: genesis,
		prior:                 prior,
		input: TransitionHeadObservationInput{
			Request:      request,
			Stage:        stage,
			CurrentGraph: genesis.graph,
		},
	}
}

func readSetTransitionStage(
	t *testing.T,
	project projectidentity.ProjectID,
	graph projecttypeenvselection.ProjectGraphSnapshotBasis,
	prior projecttypeenvselection.ProjectTypeEnvHeadState,
) projecttypeenvselection.ProjectTypeEnvStage {
	t.Helper()
	target := readSetTargetClosure(t)
	previous := readSetPriorTypeEnv(
		t,
		prior.SelectedComposite(),
		target.snapshot.Environment(),
	)
	diff, err := projecttypeenvcompatibility.Compare(
		previous,
		target.snapshot.Environment(),
	)
	if err != nil {
		t.Fatalf("projecttypeenvcompatibility.Compare(): %v", err)
	}
	compatibility, err := projecttypeenvselection.NewComparedStageCompatibility(diff)
	if err != nil {
		t.Fatalf("NewComparedStageCompatibility(): %v", err)
	}
	successor, err := projecttypeenvcompatibility.CompareSuccessor(
		previous,
		target.snapshot.Environment(),
	)
	if err != nil {
		t.Fatalf("projecttypeenvcompatibility.CompareSuccessor(): %v", err)
	}
	transitionProfiles, err := projecttypeenvprofilecompatibility.AssessTransitionProjectionProfiles(
		successor,
	)
	if err != nil {
		t.Fatalf("AssessTransitionProjectionProfiles(): %v", err)
	}
	predecessor, err := prior.ExactPriorHead()
	if err != nil {
		t.Fatalf("ExactPriorHead(): %v", err)
	}
	graphRef, err := projecttypeenvassertionreport.ParseGraphSnapshotRef(
		graph.Ref().String(),
	)
	if err != nil {
		t.Fatalf("ParseGraphSnapshotRef(): %v", err)
	}
	graphCoordinate, err := projecttypeenvassertionreport.NewGraphSnapshotCoordinate(
		graphRef,
		graph.GraphRevision(),
		graph.Ref().Digest(),
	)
	if err != nil {
		t.Fatalf("NewGraphSnapshotCoordinate(): %v", err)
	}
	revalidation, err := projecttypeenvassertionreport.NewReport(
		target.verification.CompositeRef(),
		graphCoordinate,
		target.verification.RuntimeEvaluationBasisRef(),
		target.verification.RuntimeEvaluationBasisRef().Digest(),
		nil,
	)
	if err != nil {
		t.Fatalf("projecttypeenvassertionreport.NewReport(): %v", err)
	}
	profileRoot, err := projectprofile.NewProjectRootV1(
		"/tmp/haft-transition-readset-" + project.String(),
	)
	if err != nil {
		t.Fatalf("NewProjectRootV1(): %v", err)
	}
	profileBasis, err := projecttypeenvprofilebasis.NewNoCanonicalProjectProfile(
		profileRoot,
	)
	if err != nil {
		t.Fatalf("NewNoCanonicalProjectProfile(): %v", err)
	}
	profileFit, err := projecttypeenvprofilefit.AssessProjectTypeEnvProfileFit(
		profileBasis,
		target.snapshot,
	)
	if err != nil {
		t.Fatalf("AssessProjectTypeEnvProfileFit(): %v", err)
	}
	stage, err := projecttypeenvselection.SealProjectTypeEnvStage(
		projecttypeenvselection.ProjectTypeEnvStageInput{
			Project:                                  project,
			Predecessor:                              predecessor,
			Base:                                     target.verification.BaseTypeEnvRef(),
			OrderedExtensions:                        target.verification.ExtensionRefs(),
			RuntimeBasis:                             target.verification.RuntimeEvaluationBasisRef(),
			VerifiedComposite:                        target.verification,
			Composite:                                target.verification.CompositeRef(),
			GraphSnapshotBasis:                       graph,
			GraphSnapshotBasisRef:                    graph.Ref(),
			GraphSnapshotBasisDigest:                 graph.Ref().Digest(),
			GraphRevision:                            graph.GraphRevision(),
			ProfileLedgerRevision:                    profileBasis.LedgerRevision(),
			ProfileLedgerDigest:                      profileBasis.ProfileLedgerDigest(),
			Compatibility:                            compatibility,
			ExistingAssertionRevalidation:            revalidation,
			ProfileCompatibility:                     profileFit,
			TransitionProjectionProfileCompatibility: transitionProfiles,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvStage(): %v", err)
	}
	return stage
}

func seedTransitionPriorHead(t *testing.T, fixture transitionReadSetFixture) {
	t.Helper()
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(seed): %v", err)
	}
	if err := fixture.store.CompareAndSwapGenesisProjectTypeEnvHeadTx(
		ctx,
		transaction,
		fixture.prior,
	); err != nil {
		t.Fatalf("seed prior head: %v", err)
	}
	if finish := transaction.Commit(ctx); !finish.Succeeded() {
		t.Fatalf("commit prior head: %v", finish.Err())
	}
}

func readSetPriorTypeEnv(
	t *testing.T,
	ref typedmemory.TypeEnvRef,
	target typedmemory.TypeEnv,
) typedmemory.TypeEnv {
	t.Helper()
	builder := typedmemory.NewTypeEnvBuilder(ref).
		SetSourceRevision(target.SourceRevision()).
		SetCompilerSchemaVersion(target.CompilerSchemaVersion()).
		SetCoverageManifest(target.CoverageManifest())
	for _, contextValue := range target.BoundedContexts() {
		builder = builder.AddBoundedContext(contextValue)
	}
	value, err := builder.Build()
	if err != nil {
		t.Fatalf("build prior TypeEnv: %v", err)
	}
	return value
}
