package typedmemorystore

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestSQLiteAdapterReplayRejectsCorruptedDurableMaterializationAfterReopen(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	candidate := fixture.declaration(
		t,
		"authorization-service",
		"Authorization service",
	)
	request := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"declare:authorization",
		candidate,
	)
	first, err := fixture.adapter.commitDeclareEntity(context.Background(), request)
	if err != nil {
		t.Fatalf("seed CommitDeclareEntity: %v", err)
	}
	if first.Disposition() != CommitApplied {
		t.Fatalf("seed disposition = %q; want %q", first.Disposition(), CommitApplied)
	}

	if err := fixture.store.Close(); err != nil {
		t.Fatalf("close committed database: %v", err)
	}
	reopened := openStoreAt(t, fixture.databasePath)
	reopenedDatabase := reopened.GetRawDB()
	reopenedAdapter := fixture.adapterFor(t, reopenedDatabase)
	bypassEntityContextImmutabilityForCorruptionTest(t, reopenedDatabase)

	receipt, err := reopenedAdapter.commitDeclareEntity(context.Background(), request)
	if err == nil {
		t.Fatalf("corrupted replay minted receipt %+v", receipt)
	}
	if !errors.Is(err, ErrStoredAdmissionIntegrity) ||
		errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("corrupted replay error = %v; want exact stored-admission integrity rejection", err)
	}
	if receipt != (CommitReceipt{}) {
		t.Fatalf("corrupted replay leaked receipt %+v", receipt)
	}
	assertTypedMemoryRowCounts(t, reopenedDatabase, committedDeclarationCounts())

	head, headErr := reopenedAdapter.LoadHead(context.Background(), fixture.project)
	if headErr != nil {
		t.Fatalf("LoadHead after corrupted replay rejection: %v", headErr)
	}
	if head.Revision() != first.GraphRevision() ||
		head.LastEventRef() != first.EventRef() ||
		head.LastCommitRef() != first.CommitRef() {
		t.Fatal("corrupted replay rejection changed the committed graph head")
	}
}

func bypassEntityContextImmutabilityForCorruptionTest(
	t *testing.T,
	database *sql.DB,
) {
	t.Helper()
	if _, err := database.Exec(
		`DROP TRIGGER typed_memory_entity_contexts_no_update`,
	); err != nil {
		t.Fatalf("drop entity-context immutability trigger: %v", err)
	}
	result, err := database.Exec(
		`UPDATE typed_memory_entity_contexts
		SET label = 'Corrupted authorization service'`,
	)
	if err != nil {
		t.Fatalf("inject durable entity-context corruption: %v", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("count corrupted entity-context rows: %v", err)
	}
	if changed != 1 {
		t.Fatalf("corrupted entity-context rows = %d; want 1", changed)
	}
}
