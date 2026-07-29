package sqlite

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestCurrentCommittedClosureLoaderReturnsExactGenesisClosure(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	ctx := context.Background()
	result, err := fixture.service.SelectGenesis(
		ctx,
		genesisSelectionInput(fixture),
	)
	if err != nil {
		t.Fatalf("SelectGenesis(): %v", err)
	}
	fresh, ok := result.(projecttypeenvselectioneffect.FreshlyCommitted)
	if !ok {
		t.Fatalf("SelectGenesis() = %T, want FreshlyCommitted", result)
	}
	expected := fresh.Closure()
	successor := expected.SuccessorHead()

	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead(): %v", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	loader := NewCurrentCommittedClosureLoader()
	loaded, err := loader.LoadCommittedClosureForCurrentHeadTx(
		ctx,
		transaction,
		fixture.project,
		expected.CommittedGraphRevision(),
		successor.Ref(),
		successor.Revision(),
		successor.SelectedComposite(),
	)
	if err != nil {
		t.Fatalf("LoadCommittedClosureForCurrentHeadTx(): %v", err)
	}
	if loaded.Ref() != expected.Ref() ||
		!bytes.Equal(loaded.CanonicalBytes(), expected.CanonicalBytes()) {
		t.Fatal("current closure loader returned a different closure")
	}
	if err := transaction.RequireActive(); err != nil {
		t.Fatalf("loader finished the caller transaction: %v", err)
	}

	later, err := loader.LoadCommittedClosureForCurrentHeadTx(
		ctx,
		transaction,
		fixture.project,
		typedmemory.NewGraphRevision(
			expected.CommittedGraphRevision().Value()+1,
		),
		successor.Ref(),
		successor.Revision(),
		successor.SelectedComposite(),
	)
	if err != nil {
		t.Fatalf("later-revision load error = %v", err)
	}
	if later.Ref() != expected.Ref() ||
		!bytes.Equal(later.CanonicalBytes(), expected.CanonicalBytes()) {
		t.Fatal("later-revision load returned a different active closure")
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = loader.LoadCommittedClosureForCurrentHeadTx(
		canceled,
		transaction,
		fixture.project,
		expected.CommittedGraphRevision(),
		successor.Ref(),
		successor.Revision(),
		successor.SelectedComposite(),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled load error = %v, want context.Canceled", err)
	}
}

func TestCurrentCommittedClosureLoaderRejectsCorruptClosureDAG(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	ctx := context.Background()
	result, err := fixture.service.SelectGenesis(
		ctx,
		genesisSelectionInput(fixture),
	)
	if err != nil {
		t.Fatalf("SelectGenesis(): %v", err)
	}
	fresh, ok := result.(projecttypeenvselectioneffect.FreshlyCommitted)
	if !ok {
		t.Fatalf("SelectGenesis() = %T, want FreshlyCommitted", result)
	}
	closure := fresh.Closure()
	successor := closure.SuccessorHead()
	corruptGenesisProofCanonical(t, fixture)

	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead(): %v", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	loader := NewCurrentCommittedClosureLoader()
	loaded, err := loader.LoadCommittedClosureForCurrentHeadTx(
		ctx,
		transaction,
		fixture.project,
		closure.CommittedGraphRevision(),
		successor.Ref(),
		successor.Revision(),
		successor.SelectedComposite(),
	)
	if err == nil {
		t.Fatal("corrupt closure DAG was accepted")
	}
	if len(loaded.CanonicalBytes()) != 0 {
		t.Fatal("corrupt load returned a non-zero closure")
	}
	if err := transaction.RequireActive(); err != nil {
		t.Fatalf("corrupt load finished the caller transaction: %v", err)
	}
}
