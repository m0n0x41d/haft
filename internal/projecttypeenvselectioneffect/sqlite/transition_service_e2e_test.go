package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"reflect"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projecttypeenvassertionrevalidation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilecompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

type transitionE2EFixture struct {
	stage   projecttypeenvselection.ProjectTypeEnvStage
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	content projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent
	service *TransitionService
}

func TestTransitionServiceCommitsExactSuccessorAndReplays(t *testing.T) {
	fixture := newGenesisE2EFixture(t)
	ctx := context.Background()
	genesisResult, err := fixture.service.SelectGenesis(
		ctx,
		genesisSelectionInput(fixture),
	)
	if err != nil {
		t.Fatalf("SelectGenesis(): %v", err)
	}
	genesis, ok := genesisResult.(projecttypeenvselectioneffect.FreshlyCommitted)
	if !ok {
		t.Fatalf("Genesis result = %T, want FreshlyCommitted", genesisResult)
	}
	prior := genesis.Closure().SuccessorHead()
	target := newGenesisE2ETargetWithRuntime(
		t,
		"artifact:transition-e2e-runtime",
		"2.0.0",
	)
	transition := newTransitionE2EFixture(
		t,
		fixture,
		prior,
		fixture.target.snapshot,
		target,
		"transition-e2e-key",
		"transition-e2e",
	)
	input := transitionSelectionInput(transition)
	before := genesisE2EEffectCounts(t, fixture.database)

	result, err := transition.service.SelectTransition(ctx, input)
	if err != nil {
		t.Fatalf("SelectTransition(fresh): %v", err)
	}
	fresh, ok := result.(projecttypeenvselectioneffect.FreshlyCommitted)
	if !ok {
		t.Fatalf("Transition result = %T, want FreshlyCommitted", result)
	}
	closure := fresh.Closure()
	if closure.SuccessorHead().Revision().Value() != 2 ||
		closure.SuccessorHead().SelectedComposite() != target.snapshot.TypeEnvRef() ||
		closure.CommittedGraphRevision().Value() != 2 {
		t.Fatalf(
			"Transition successor = head %d C %s graph %d",
			closure.SuccessorHead().Revision().Value(),
			closure.SuccessorHead().SelectedComposite().String(),
			closure.CommittedGraphRevision().Value(),
		)
	}
	if _, ok := closure.Predecessor().(projecttypeenvselection.TransitionStagePredecessor); !ok {
		t.Fatalf("Transition closure predecessor = %T", closure.Predecessor())
	}
	afterFresh := genesisE2EEffectCounts(t, fixture.database)
	if afterFresh.proofs != before.proofs {
		t.Fatalf(
			"Transition changed Genesis proof count from %d to %d",
			before.proofs,
			afterFresh.proofs,
		)
	}
	assertTransitionE2EStoredPredecessor(t, fixture.database, transition, prior)

	replayedResult, err := transition.service.SelectTransition(ctx, input)
	if err != nil {
		t.Fatalf("SelectTransition(replay): %v", err)
	}
	replayed, ok := replayedResult.(projecttypeenvselectioneffect.ReplayedExisting)
	if !ok {
		t.Fatalf("Transition replay = %T, want ReplayedExisting", replayedResult)
	}
	if !bytes.Equal(
		replayed.Closure().CanonicalBytes(),
		closure.CanonicalBytes(),
	) {
		t.Fatal("Transition exact replay returned another closure")
	}
	afterReplay := genesisE2EEffectCounts(t, fixture.database)
	if !reflect.DeepEqual(afterReplay, afterFresh) {
		t.Fatalf(
			"Transition replay changed rows: before=%v after=%v",
			afterFresh,
			afterReplay,
		)
	}
}

func TestTransitionServiceNeverTreatsAbsentHeadAsGenesis(t *testing.T) {
	fixture := newGenesisE2EFixture(t)
	prior, err := projecttypeenvselection.SealProjectTypeEnvHeadState(
		projecttypeenvselection.ProjectTypeEnvHeadStateInput{
			Project:           fixture.project,
			SelectedComposite: fixture.target.snapshot.TypeEnvRef(),
			Revision:          mustTransitionHeadRevision(t, 1),
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadState(): %v", err)
	}
	target := newGenesisE2ETargetWithRuntime(
		t,
		"artifact:transition-absent-runtime",
		"2.1.0",
	)
	transition := newTransitionE2EFixture(
		t,
		fixture,
		prior,
		fixture.target.snapshot,
		target,
		"transition-absent-key",
		"transition-absent",
	)
	before := genesisE2EEffectCounts(t, fixture.database)

	result, err := transition.service.SelectTransition(
		context.Background(),
		transitionSelectionInput(transition),
	)
	if err != nil {
		t.Fatalf("SelectTransition(absent): %v", err)
	}
	notSelected, ok := result.(projecttypeenvselectioneffect.NotSelected)
	if !ok {
		t.Fatalf("absent Transition result = %T, want NotSelected", result)
	}
	if notSelected.Reason() != projecttypeenvselectioneffect.NotSelectedPriorHeadAbsent() {
		t.Fatalf("absent reason = %s", notSelected.Reason())
	}
	after := genesisE2EEffectCounts(t, fixture.database)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("absent Transition wrote effect rows: before=%v after=%v", before, after)
	}
}

func TestTransitionServiceRejectsExactStalePriorHead(t *testing.T) {
	fixture := newGenesisE2EFixture(t)
	ctx := context.Background()
	genesisResult, err := fixture.service.SelectGenesis(
		ctx,
		genesisSelectionInput(fixture),
	)
	if err != nil {
		t.Fatalf("SelectGenesis(): %v", err)
	}
	genesis := genesisResult.(projecttypeenvselectioneffect.FreshlyCommitted)
	oldPrior := genesis.Closure().SuccessorHead()
	target2 := newGenesisE2ETargetWithRuntime(
		t,
		"artifact:transition-stale-current-runtime",
		"3.0.0",
	)
	first := newTransitionE2EFixture(
		t,
		fixture,
		oldPrior,
		fixture.target.snapshot,
		target2,
		"transition-stale-first-key",
		"transition-stale-first",
	)
	firstResult, err := first.service.SelectTransition(
		ctx,
		transitionSelectionInput(first),
	)
	if err != nil {
		t.Fatalf("SelectTransition(first): %v", err)
	}
	if _, ok := firstResult.(projecttypeenvselectioneffect.FreshlyCommitted); !ok {
		t.Fatalf("first Transition = %T, want FreshlyCommitted", firstResult)
	}
	stale := newTransitionE2EFixture(
		t,
		fixture,
		oldPrior,
		fixture.target.snapshot,
		fixture.target,
		"transition-stale-second-key",
		"transition-stale-second",
	)
	before := genesisE2EEffectCounts(t, fixture.database)

	result, err := stale.service.SelectTransition(
		ctx,
		transitionSelectionInput(stale),
	)
	if err != nil {
		t.Fatalf("SelectTransition(stale): %v", err)
	}
	notSelected, ok := result.(projecttypeenvselectioneffect.NotSelected)
	if !ok {
		t.Fatalf("stale Transition = %T, want NotSelected", result)
	}
	if notSelected.Reason() != projecttypeenvselectioneffect.NotSelectedStalePriorHead() {
		t.Fatalf("stale reason = %s", notSelected.Reason())
	}
	after := genesisE2EEffectCounts(t, fixture.database)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("stale Transition wrote effect rows: before=%v after=%v", before, after)
	}
}

func TestTransitionServiceRollbackSelectsPriorImmutableCWithoutDeletingAssertions(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	ctx := context.Background()
	genesisResult, err := fixture.service.SelectGenesis(
		ctx,
		genesisSelectionInput(fixture),
	)
	if err != nil {
		t.Fatalf("SelectGenesis(): %v", err)
	}
	genesis := genesisResult.(projecttypeenvselectioneffect.FreshlyCommitted)
	priorC := genesis.Closure().SuccessorHead()
	target2 := newGenesisE2ETargetWithRuntime(
		t,
		"artifact:transition-rollback-runtime",
		"4.0.0",
	)
	forward := newTransitionE2EFixture(
		t,
		fixture,
		priorC,
		fixture.target.snapshot,
		target2,
		"transition-rollback-forward-key",
		"transition-rollback-forward",
	)
	forwardResult, err := forward.service.SelectTransition(
		ctx,
		transitionSelectionInput(forward),
	)
	if err != nil {
		t.Fatalf("SelectTransition(forward): %v", err)
	}
	forwardFresh := forwardResult.(projecttypeenvselectioneffect.FreshlyCommitted)
	current := forwardFresh.Closure().SuccessorHead()
	assertionsBefore := transitionE2ERelationCount(t, fixture.database, fixture.project.String())
	rollback := newTransitionE2EFixture(
		t,
		fixture,
		current,
		target2.snapshot,
		fixture.target,
		"transition-rollback-back-key",
		"transition-rollback-back",
	)

	rollbackResult, err := rollback.service.SelectTransition(
		ctx,
		transitionSelectionInput(rollback),
	)
	if err != nil {
		t.Fatalf("SelectTransition(rollback): %v", err)
	}
	rollbackFresh, ok := rollbackResult.(projecttypeenvselectioneffect.FreshlyCommitted)
	if !ok {
		t.Fatalf("rollback result = %T, want FreshlyCommitted", rollbackResult)
	}
	successor := rollbackFresh.Closure().SuccessorHead()
	if successor.Revision().Value() != 3 ||
		successor.SelectedComposite() != fixture.target.snapshot.TypeEnvRef() ||
		rollbackFresh.Closure().CommittedGraphRevision().Value() != 3 {
		t.Fatalf(
			"rollback successor = head %d C %s graph %d",
			successor.Revision().Value(),
			successor.SelectedComposite().String(),
			rollbackFresh.Closure().CommittedGraphRevision().Value(),
		)
	}
	assertionsAfter := transitionE2ERelationCount(t, fixture.database, fixture.project.String())
	if assertionsAfter != assertionsBefore {
		t.Fatalf(
			"rollback changed historical assertion count from %d to %d",
			assertionsBefore,
			assertionsAfter,
		)
	}
}

func newTransitionE2EFixture(
	t *testing.T,
	genesis genesisE2EFixture,
	prior projecttypeenvselection.ProjectTypeEnvHeadState,
	priorExecutable projecttypeenv.ProjectTypeEnvExecutableSnapshot,
	target genesisE2ETarget,
	keyText string,
	descriptionSeed string,
) transitionE2EFixture {
	t.Helper()
	ctx := context.Background()
	stageStore, err := projecttypeenvstage.New(ctx, genesis.database)
	if err != nil {
		t.Fatalf("projecttypeenvstage.New(): %v", err)
	}
	if err := stageStore.PutArtifactClosure(ctx, target.closure); err != nil {
		t.Fatalf("PutArtifactClosure(Transition): %v", err)
	}
	currentGraph, currentProfile := loadTransitionE2ECurrentBasis(t, genesis)
	predecessor, err := prior.ExactPriorHead()
	if err != nil {
		t.Fatalf("ExactPriorHead(): %v", err)
	}
	diff, err := projecttypeenvcompatibility.Compare(
		priorExecutable.Environment(),
		target.snapshot.Environment(),
	)
	if err != nil {
		t.Fatalf("Compare(Transition): %v", err)
	}
	compatibility, err := projecttypeenvselection.NewComparedStageCompatibility(diff)
	if err != nil {
		t.Fatalf("NewComparedStageCompatibility(): %v", err)
	}
	successor, err := projecttypeenvcompatibility.CompareSuccessor(
		priorExecutable.Environment(),
		target.snapshot.Environment(),
	)
	if err != nil {
		t.Fatalf("CompareSuccessor(Transition): %v", err)
	}
	transitionProfiles, err := projecttypeenvprofilecompatibility.AssessTransitionProjectionProfiles(
		successor,
	)
	if err != nil {
		t.Fatalf("AssessTransitionProjectionProfiles(Transition): %v", err)
	}
	revalidation, err := projecttypeenvassertionrevalidation.Revalidate(
		projecttypeenvassertionrevalidation.Input{
			CurrentGraph:  currentGraph,
			TargetTypeEnv: target.snapshot.Environment(),
			TargetRuntime: target.registry,
		},
	)
	if err != nil {
		t.Fatalf("Revalidate(Transition assertions): %v", err)
	}
	profileFit, err := projecttypeenvprofilefit.AssessProjectTypeEnvProfileFit(
		currentProfile,
		target.snapshot,
	)
	if err != nil {
		t.Fatalf("AssessProjectTypeEnvProfileFit(Transition): %v", err)
	}
	graphBasis := currentGraph.GraphSnapshotBasis()
	stage, err := projecttypeenvselection.SealProjectTypeEnvStage(
		projecttypeenvselection.ProjectTypeEnvStageInput{
			Project:                                  genesis.project,
			Predecessor:                              predecessor,
			Base:                                     target.verification.BaseTypeEnvRef(),
			OrderedExtensions:                        target.verification.ExtensionRefs(),
			RuntimeBasis:                             target.verification.RuntimeEvaluationBasisRef(),
			VerifiedComposite:                        target.verification,
			Composite:                                target.verification.CompositeRef(),
			GraphSnapshotBasis:                       graphBasis,
			GraphSnapshotBasisRef:                    graphBasis.Ref(),
			GraphSnapshotBasisDigest:                 graphBasis.Ref().Digest(),
			GraphRevision:                            graphBasis.GraphRevision(),
			ProfileLedgerRevision:                    currentProfile.LedgerRevision(),
			ProfileLedgerDigest:                      currentProfile.ProfileLedgerDigest(),
			Compatibility:                            compatibility,
			ExistingAssertionRevalidation:            revalidation,
			ProfileCompatibility:                     profileFit,
			TransitionProjectionProfileCompatibility: transitionProfiles,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvStage(Transition): %v", err)
	}
	if err := stageStore.Put(
		ctx,
		stage,
		target.verification.Record(),
		target.snapshot.Record(),
	); err != nil {
		t.Fatalf("Stage Put(Transition): %v", err)
	}
	key, err := projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
		keyText,
	)
	if err != nil {
		t.Fatalf("NewProjectTypeEnvHeadSelectionIdempotencyKey(): %v", err)
	}
	request, err := projecttypeenvselection.SealTransitionProjectTypeEnvHeadSelectionRequest(
		projecttypeenvselection.TransitionProjectTypeEnvHeadSelectionRequestInput{
			Project:               genesis.project,
			ExactPriorHead:        prior,
			Stage:                 stage,
			ExpectedGraphRevision: graphBasis.GraphRevision(),
			IdempotencyKey:        key,
		},
	)
	if err != nil {
		t.Fatalf("SealTransitionProjectTypeEnvHeadSelectionRequest(): %v", err)
	}
	description, err := authority.NewClaimIDDescriptionRef(
		"claim:project-typeenv-head-selection:" + descriptionSeed,
	)
	if err != nil {
		t.Fatalf("NewClaimIDDescriptionRef(): %v", err)
	}
	judgementContext, err := authority.NewBoundedContextRef(
		"bounded-context:project-typeenv-head-selection",
	)
	if err != nil {
		t.Fatalf("NewBoundedContextRef(): %v", err)
	}
	now := genesis.service.clock.Now()
	validity, err := authority.NewTimeWindow(
		now.Add(-time.Hour),
		now.Add(time.Hour),
	)
	if err != nil {
		t.Fatalf("NewTimeWindow(): %v", err)
	}
	content, err := projecttypeenvselectionauthority.SealProjectTypeEnvHeadSelectionAuthorizationContent(
		projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContentInput{
			DescriptionRef:   description,
			Request:          request,
			Stage:            stage,
			JudgementContext: judgementContext,
			ValidityWindow:   validity,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadSelectionAuthorizationContent(): %v", err)
	}
	service, err := NewTransitionService(
		ctx,
		genesis.database,
		genesis.service.projectRoot.String(),
		target.installed,
		genesis.service.clock,
	)
	if err != nil {
		t.Fatalf("NewTransitionService(): %v", err)
	}
	return transitionE2EFixture{
		stage:   stage,
		request: request,
		content: content,
		service: service,
	}
}

func loadTransitionE2ECurrentBasis(
	t *testing.T,
	fixture genesisE2EFixture,
) (
	typedmemorystore.CurrentProjectGraphObservation,
	projecttypeenvprofilebasis.CurrentProjectProfileBasis,
) {
	t.Helper()
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead(Transition basis): %v", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	currentGraph, err := typedmemorystore.LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		fixture.project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentGraphRevalidationBasisTx(Transition): %v", err)
	}
	currentProfile, err := loadCurrentProjectProfileBasisTx(
		ctx,
		transaction,
		fixture.service.projectRoot,
	)
	if err != nil {
		t.Fatalf("loadCurrentProjectProfileBasisTx(Transition): %v", err)
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		t.Fatalf("commit Transition basis read: %v", finish.Err())
	}
	return currentGraph, currentProfile
}

func transitionSelectionInput(
	fixture transitionE2EFixture,
) TransitionSelectionInput {
	return TransitionSelectionInput{
		Request:   fixture.request,
		Content:   fixture.content,
		Authority: hostRoutedIngressForFixture(fixture.request, fixture.content),
	}
}

func mustTransitionHeadRevision(
	t *testing.T,
	value uint64,
) projecttypeenvselection.HeadRevision {
	t.Helper()
	revision, err := projecttypeenvselection.NewHeadRevision(value)
	if err != nil {
		t.Fatalf("NewHeadRevision(%d): %v", value, err)
	}
	return revision
}

func assertTransitionE2EStoredPredecessor(
	t *testing.T,
	database *sql.DB,
	fixture transitionE2EFixture,
	prior projecttypeenvselection.ProjectTypeEnvHeadState,
) {
	t.Helper()
	var kind string
	var headRef string
	var headRevision int64
	var selectedComposite string
	var proofRef sql.NullString
	var proofDigest sql.NullString
	err := database.QueryRow(
		`SELECT predecessor_kind, prior_head_ref, prior_head_revision,
			prior_selected_composite_ref, no_prior_head_proof_ref,
			no_prior_head_proof_digest
		 FROM project_typeenv_head_selection_requests
		 WHERE request_ref = ?`,
		fixture.request.Ref().String(),
	).Scan(
		&kind,
		&headRef,
		&headRevision,
		&selectedComposite,
		&proofRef,
		&proofDigest,
	)
	if err != nil {
		t.Fatalf("read stored Transition predecessor: %v", err)
	}
	if kind != "transition" ||
		headRef != prior.Ref().String() ||
		headRevision != int64(prior.Revision().Value()) ||
		selectedComposite != prior.SelectedComposite().String() ||
		proofRef.Valid || proofDigest.Valid {
		t.Fatalf(
			"stored Transition predecessor = %s/%s/%d/%s proof=(%v,%v)",
			kind,
			headRef,
			headRevision,
			selectedComposite,
			proofRef,
			proofDigest,
		)
	}
}

func transitionE2ERelationCount(
	t *testing.T,
	database *sql.DB,
	project string,
) int {
	t.Helper()
	var count int
	err := database.QueryRow(
		`SELECT COUNT(*)
		 FROM typed_memory_relation_instances
		 WHERE project_id = ?`,
		project,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count Transition relation instances: %v", err)
	}
	return count
}
