package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/problemcardadapter"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemoryvalidation"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestProductionProblemCardAtConcernAdmitsAndRereadsExactTypedMemory(
	t *testing.T,
) {
	fixture := newProductionNoteSelectedFixture(t)
	ctx := context.Background()
	selection, err := fixture.service.SelectGenesis(
		ctx,
		genesisSelectionInput(fixture),
	)
	mustProductionNoteNoError(t, err)
	fresh, ok := selection.(projecttypeenvselectioneffect.FreshlyCommitted)
	if !ok {
		t.Fatalf(
			"production TypeEnv selection = %T, want FreshlyCommitted",
			selection,
		)
	}
	selected := fresh.Closure().CommittedGraphRevision()

	resolver := genesisE2EProjectRuntimeResolver(t, fixture)
	baseLoader, err := typedmemorystore.NewProjectAwareSQLiteCurrentProjectSnapshotLoader(
		fixture.database,
		projectmemory.NewBaseTypeEnvLoader(),
		resolver,
	)
	mustProductionNoteNoError(t, err)
	clock := &genesisE2EClock{
		value: time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC),
	}
	baseAdapter := newProductionNoteCommitAdapter(
		t,
		fixture,
		resolver,
		clock,
		productionNoteUnavailableObservableProvider{},
	)
	baseSource := newGenesisE2ECurrentProjectBasisSource(t, baseLoader)
	baseRuntime, err := projectmemory.NewAdmissionRuntime(
		fixture.project,
		baseSource,
		baseAdapter,
	)
	mustProductionNoteNoError(t, err)
	contextRef, err := typedmemory.NewBoundedContextRef("haft-project")
	mustProductionNoteNoError(t, err)
	concern := productionNoteConcernDeclaration(t, contextRef)
	concernCandidate, err := typedmemory.NewMemoryChangeSet(
		[]typedmemory.MemoryChange{concern},
	)
	mustProductionNoteNoError(t, err)
	concernValid, err := baseRuntime.PrepareCandidate(
		ctx,
		typedmemorywire.ProjectCurrentSelector{},
		concernCandidate,
	)
	mustProductionNoteNoError(t, err)
	concernKey, err := typedmemorystore.NewIdempotencyKey(
		"production-problem-card-concern-admission",
	)
	mustProductionNoteNoError(t, err)
	concernReceipt, err := baseRuntime.AdmitValidated(
		ctx,
		concernValid,
		concernKey,
		concern.Provenance(),
	)
	mustProductionNoteNoError(t, err)
	if concernReceipt.GraphRevision().Value() != selected.Value()+1 {
		t.Fatalf(
			"concern graph revision = %d, want %d",
			concernReceipt.GraphRevision().Value(),
			selected.Value()+1,
		)
	}

	current, err := baseLoader.LoadCurrentProjectSnapshot(ctx, fixture.project)
	mustProductionNoteNoError(t, err)
	concernBinding := productionNoteConcernBinding(
		t,
		current,
		concern.Entity(),
		contextRef,
	)
	exactRuntime := productionNoteExactRuntime(t, fixture, current)
	draft, recordEntity, assertionID := productionProblemCardDraft(
		t,
		fixture,
		current,
		contextRef,
	)
	adapted := problemcardadapter.Adapt(
		draft,
		exactRuntime,
		concernBinding,
	)
	candidate, ok := adapted.(problemcardadapter.ValidCandidate)
	if !ok {
		t.Fatalf(
			"ProblemCard adapter result = %T, want ValidCandidate",
			adapted,
		)
	}
	stage, err := problemcardadapter.SealPreAdmissionSourceStage(candidate)
	mustProductionNoteNoError(t, err)
	overlayLoader, err :=
		typedmemorystore.NewCurrentProjectSnapshotLoaderWithObservableInputOverlay(
			baseLoader,
			stage,
		)
	mustProductionNoteNoError(t, err)
	source := newGenesisE2ECurrentProjectBasisSource(t, overlayLoader)
	adapter := newProductionNoteCommitAdapter(
		t,
		fixture,
		resolver,
		clock,
		stage,
	)
	runtime, err := projectmemory.NewAdmissionRuntime(
		fixture.project,
		source,
		adapter,
	)
	mustProductionNoteNoError(t, err)
	validation, err := projectmemory.NewValidationRuntime(
		fixture.project,
		source,
	)
	mustProductionNoteNoError(t, err)
	outcome, err := validation.EvaluateCandidate(
		ctx,
		typedmemorywire.ProjectCurrentSelector{},
		candidate.ChangeSet(),
	)
	mustProductionNoteNoError(t, err)
	valid, ok := outcome.(typedmemoryvalidation.ValidOutcome)
	if !ok {
		t.Fatalf(
			"production ProblemCard validation = %T/%s diagnostics=%#v",
			outcome,
			outcome.Verdict(),
			outcome.Diagnostics(),
		)
	}
	assertProductionProblemCardAdmissionBasis(t, valid.AdmissionBasis())
	assertion := productionNoteValidatedAssertion(t, valid)
	if assertion.Signature().ID().String() != "Haft.ProblemCardAtConcern" {
		t.Fatalf(
			"validated signature = %s, want Haft.ProblemCardAtConcern",
			assertion.Signature().ID().String(),
		)
	}

	key, err := typedmemorystore.NewIdempotencyKey(
		"production-problem-card-at-concern-admission",
	)
	mustProductionNoteNoError(t, err)
	provenance, err := typedmemory.NewProvenanceRef(
		"memory:test:production-problem-card-at-concern-admission",
	)
	mustProductionNoteNoError(t, err)
	receipt, err := runtime.AdmitValidated(
		ctx,
		valid,
		key,
		provenance,
	)
	mustProductionNoteNoError(t, err)
	if receipt.Disposition() != typedmemorystore.CommitApplied ||
		receipt.GraphRevision().Value() != selected.Value()+2 {
		t.Fatalf(
			"ProblemCard admission = %s/revision %d, want applied/revision %d",
			receipt.Disposition(),
			receipt.GraphRevision().Value(),
			selected.Value()+2,
		)
	}
	replay, err := runtime.AdmitValidated(ctx, valid, key, provenance)
	mustProductionNoteNoError(t, err)
	if replay.Disposition() != typedmemorystore.CommitReplay ||
		replay.EventRef() != receipt.EventRef() ||
		replay.ResultDigest() != receipt.ResultDigest() {
		t.Fatal("ProblemCard idempotent replay changed the durable result")
	}
	stored, err := adapter.LoadEntity(ctx, fixture.project, recordEntity)
	mustProductionNoteNoError(t, err)
	if stored.Label().String() != "Production ProblemCard at concern" {
		t.Fatalf("stored ProblemCard label = %q", stored.Label().String())
	}
	transaction, err := sqlitetransaction.BeginRead(
		ctx,
		fixture.database,
	)
	mustProductionNoteNoError(t, err)
	observation, err := typedmemorystore.LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		fixture.project,
	)
	mustProductionNoteNoError(t, err)
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		t.Fatalf("commit ProblemCard durable read: %v", finish.Err())
	}
	relations := observation.ActiveAssertions().Relations()
	if len(relations) != 1 {
		t.Fatalf(
			"durable ProblemCard relations = %#v, want one exact assertion",
			relations,
		)
	}
	durable := relations[0]
	carrier := productionFreshCurrentAssertionCarrier(t, durable)
	if durable.AssertionID() != assertionID ||
		carrier.Signature().ID().String() !=
			"Haft.ProblemCardAtConcern" {
		t.Fatalf(
			"durable ProblemCard relations = %#v, want one exact assertion",
			relations,
		)
	}
}

func productionProblemCardDraft(
	t *testing.T,
	fixture genesisE2EFixture,
	current typedmemorystore.CurrentProjectSnapshot,
	contextRef typedmemory.BoundedContextRef,
) (
	problemcardadapter.Draft,
	typedmemory.EntityID,
	typedmemory.AssertionID,
) {
	t.Helper()
	base, _, _ := productionNoteDraft(
		t,
		fixture.project,
		current.Environment(),
		contextRef,
	)
	recordEntity, err := typedmemory.NewEntityID(
		"record:production-problem-card-1",
	)
	mustProductionNoteNoError(t, err)
	local, err := typedmemory.NewBatchLocalRef(
		"record:production-problem-card-1",
	)
	mustProductionNoteNoError(t, err)
	label, err := typedmemory.NewEntityLabel(
		"Production ProblemCard at concern",
	)
	mustProductionNoteNoError(t, err)
	assertion, err := typedmemory.NewAssertionID(
		"assertion:production-problem-card-1-at-concern",
	)
	mustProductionNoteNoError(t, err)
	provenance, err := typedmemory.NewProvenanceRef(
		"memory:test:production-problem-card-at-concern",
	)
	mustProductionNoteNoError(t, err)
	draft, err := problemcardadapter.NewDraft(
		problemcardadapter.DraftInput{
			ProjectID:      fixture.project,
			RecordEntity:   recordEntity,
			RecordLocalRef: local,
			RecordLabel:    label,
			AssertionID:    assertion,
			ContextSlice:   base.ContextSlice(),
			ClaimGraph:     base.ClaimGraph(),
			Provenance:     provenance,
		},
	)
	mustProductionNoteNoError(t, err)
	return draft, recordEntity, assertion
}

func assertProductionProblemCardAdmissionBasis(
	t *testing.T,
	admission typedmemory.AdmissionBasis,
) {
	t.Helper()
	basis, ok := admission.(typedmemory.ContextSliceMembershipBasis)
	if !ok {
		t.Fatalf(
			"ProblemCard admission basis = %T, want ContextSliceMembershipBasis",
			admission,
		)
	}
	for _, use := range basis.ReferenceFillerAdmissionUses() {
		if use.Coordinate().Slot().String() !=
			"Haft.ProblemCardAtConcern.ProblemCardSlot" {
			continue
		}
		if _, ok := use.RequiredMembership().
			Basis().
			Posture().(typedmemory.C32PrerequisiteMemberOfBasisV3); !ok {
			t.Fatalf(
				"ProblemCardSlot membership basis = %T, want C.3.2 v3",
				use.RequiredMembership().Basis().Posture(),
			)
		}
		return
	}
	t.Fatal("ProblemCard admission basis omitted ProblemCardSlot")
}
