package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/projectmemory/specsectionadapter"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

func TestProductionSpecSectionAdapterPreservesSpecializedRecordWithoutLifecycleEffect(
	t *testing.T,
) {
	fixture := newProductionNoteSelectedFixture(t)
	ctx := context.Background()
	selection, err := fixture.service.SelectGenesis(
		ctx,
		genesisSelectionInput(fixture),
	)
	mustProductionNoteNoError(t, err)
	if _, ok := selection.(projecttypeenvselectioneffect.FreshlyCommitted); !ok {
		t.Fatalf(
			"production TypeEnv selection = %T, want FreshlyCommitted",
			selection,
		)
	}
	resolver := genesisE2EProjectRuntimeResolver(t, fixture)
	baseLoader, err :=
		typedmemorystore.NewProjectAwareSQLiteCurrentProjectSnapshotLoader(
			fixture.database,
			projectmemory.NewBaseTypeEnvLoader(),
			resolver,
		)
	mustProductionNoteNoError(t, err)
	clock := &genesisE2EClock{
		value: time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC),
	}
	contextRef, err := typedmemory.NewBoundedContextRef("haft-project")
	mustProductionNoteNoError(t, err)
	concern := productionNoteConcernDeclaration(t, contextRef)
	admitProductionPortfolioConcern(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		concern,
	)

	current := loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	input := productionPortfolioRecordInput(
		t,
		fixture.project,
		current.Environment(),
		contextRef,
		"spec-section",
		"SS.constraints.typed-memory.001.D1",
	)
	draft, err := specsectionadapter.NewDraft(input)
	mustProductionNoteNoError(t, err)
	candidate := mustProductionRecordCandidate(
		t,
		specsectionadapter.Adapt(
			draft,
			productionNoteExactRuntime(t, fixture, current),
			productionNoteConcernBinding(
				t,
				current,
				concern.Entity(),
				contextRef,
			),
		),
	)
	if candidate.Carrier().Variant().Token() !=
		(recordcarrier.SpecSectionRecordVariantV1{}).Token() {
		t.Fatalf(
			"SpecSection carrier variant = %q, want spec_section_record",
			candidate.Carrier().Variant().Token(),
		)
	}
	assertProductionSpecSectionRelation(t, candidate)
	valid, _ := admitProductionPortfolioRecord(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		candidate,
		"production-spec-section-at-concern",
	)
	assertProductionSelectedConstraintPresent(
		t,
		valid,
		current.Environment(),
		"Haft.Constraint.SpecSectionAtConcern.SpecSectionRecordSlot.CardinalityV1",
	)
	assertProductionSpecSectionReread(t, ctx, fixture)
}

func assertProductionSpecSectionRelation(
	t *testing.T,
	candidate specsectionadapter.ValidCandidate,
) {
	t.Helper()
	changes := candidate.ChangeSet().Changes()
	if len(changes) != 2 {
		t.Fatalf(
			"SpecSection candidate changes = %d, want declaration plus relation only",
			len(changes),
		)
	}
	relation := productionCandidateRelation(t, candidate)
	if relation.Signature().ID().String() != "Haft.SpecSectionAtConcern" {
		t.Fatalf(
			"SpecSection relation = %s, want Haft.SpecSectionAtConcern",
			relation.Signature().ID(),
		)
	}
	for _, binding := range relation.Bindings() {
		if binding.Name().String() !=
			"Haft.SpecSectionAtConcern.SpecSectionRecordSlot" {
			continue
		}
		filler, ok := binding.Fillers()[0].(typedmemory.ByReferenceCandidate)
		if !ok {
			t.Fatalf(
				"SpecSectionRecordSlot filler = %T, want ByReferenceCandidate",
				binding.Fillers()[0],
			)
		}
		if filler.Reference().RefKind().ID().String() !=
			"Haft.SpecSectionRecordRef" {
			t.Fatalf(
				"SpecSectionRecordSlot ref kind = %s, want Haft.SpecSectionRecordRef",
				filler.Reference().RefKind().ID(),
			)
		}
		return
	}
	t.Fatal("SpecSection candidate omitted its specialized record slot")
}

func assertProductionSpecSectionReread(
	t *testing.T,
	ctx context.Context,
	fixture genesisE2EFixture,
) {
	t.Helper()
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
		t.Fatalf("commit SpecSection durable read: %v", finish.Err())
	}
	count := 0
	for _, active := range observation.ActiveAssertions().Relations() {
		carrier := productionFreshCurrentAssertionCarrier(t, active)
		if carrier.Signature().ID().String() ==
			"Haft.SpecSectionAtConcern" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf(
			"durable SpecSectionAtConcern relation count = %d, want 1",
			count,
		)
	}
}
