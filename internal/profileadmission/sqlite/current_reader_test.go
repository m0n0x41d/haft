package sqlite

import (
	"context"
	"testing"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

func TestResolveCurrentWithinReturnsExactAbsenceAndLeavesTransactionActive(t *testing.T) {
	fixture := newTransactionFixture(t, "current-reader-absence", "current-reader-absence.nonce")
	transaction, err := sqlitetransaction.BeginRead(context.Background(), fixture.database)
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback(context.Background())

	observation, err := ResolveCurrentWithin(
		context.Background(),
		transaction,
		fixture.root,
	)
	if err != nil {
		t.Fatalf("ResolveCurrentWithin(): %v", err)
	}
	if _, ok := observation.(NoCurrentCanonicalProfile); !ok {
		t.Fatalf("observation = %T, want NoCurrentCanonicalProfile", observation)
	}
	if observation.LedgerRevision().Value() != 0 {
		t.Fatalf("revision = %d, want 0", observation.LedgerRevision().Value())
	}
	if err := transaction.RequireActive(); err != nil {
		t.Fatalf("reader consumed caller transaction: %v", err)
	}
}

func TestResolveCurrentWithinReturnsStrictDeclaredAdmissionAndLeavesTransactionActive(t *testing.T) {
	fixture := newTransactionFixture(t, "current-reader-declared", "current-reader-declared.nonce")
	requireAdmitted(
		t,
		fixture.adapter.Admit(context.Background(), fixture.request),
		CanonicalAdmissionFresh,
	)
	transaction, err := sqlitetransaction.BeginRead(context.Background(), fixture.database)
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback(context.Background())

	observation, err := ResolveCurrentWithin(
		context.Background(),
		transaction,
		fixture.root,
	)
	if err != nil {
		t.Fatalf("ResolveCurrentWithin(): %v", err)
	}
	declared, ok := observation.(DeclaredCurrentCanonicalProfile)
	if !ok {
		t.Fatalf("observation = %T, want DeclaredCurrentCanonicalProfile", observation)
	}
	if !declared.Admission().Valid() {
		t.Fatal("declared observation omitted strict canonical admission")
	}
	if declared.LedgerRevision().Value() != 1 {
		t.Fatalf("revision = %d, want 1", declared.LedgerRevision().Value())
	}
	if err := transaction.RequireActive(); err != nil {
		t.Fatalf("reader consumed caller transaction: %v", err)
	}
}

func TestResolveCurrentWithinSeparatesCorruptionFromAbsence(t *testing.T) {
	fixture := newTransactionFixture(t, "current-reader-corrupt", "current-reader-corrupt.nonce")
	requireAdmitted(
		t,
		fixture.adapter.Admit(context.Background(), fixture.request),
		CanonicalAdmissionFresh,
	)
	_, err := fixture.database.Exec("DROP TRIGGER profile_declaration_authority_uses_v5_no_delete")
	if err != nil {
		t.Fatal(err)
	}
	_, err = fixture.database.Exec("DELETE FROM profile_declaration_authority_uses_v5")
	if err != nil {
		t.Fatal(err)
	}
	transaction, err := sqlitetransaction.BeginRead(context.Background(), fixture.database)
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback(context.Background())

	observation, err := ResolveCurrentWithin(
		context.Background(),
		transaction,
		fixture.root,
	)
	if observation != nil {
		t.Fatalf("corrupt observation = %T, want nil", observation)
	}
	assertCurrentProfileReadFailureKind(t, err, CurrentProfileStoreCorruption)
}

func TestResolveCurrentWithinRehashesV5AuthorityUseInsideCallerSnapshot(t *testing.T) {
	fixture := newTransactionFixture(t, "current-reader-authority", "current-reader-authority.nonce")
	requireAdmitted(
		t,
		fixture.adapter.Admit(context.Background(), fixture.request),
		CanonicalAdmissionFresh,
	)
	_, err := fixture.database.Exec("DROP TRIGGER profile_declaration_authority_uses_v5_no_update")
	if err != nil {
		t.Fatal(err)
	}
	_, err = fixture.database.Exec(
		`UPDATE profile_declaration_authority_uses_v5
		 SET canonical_json = ' ' || canonical_json`,
	)
	if err != nil {
		t.Fatal(err)
	}
	transaction, err := sqlitetransaction.BeginRead(context.Background(), fixture.database)
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback(context.Background())

	_, err = ResolveCurrentWithin(context.Background(), transaction, fixture.root)
	assertCurrentProfileReadFailureKind(t, err, CurrentProfileStoreCorruption)
	if err := transaction.RequireActive(); err != nil {
		t.Fatalf("failed reader consumed caller transaction: %v", err)
	}
}

func TestResolveCurrentWithinSeparatesFinishedTransactionFailure(t *testing.T) {
	fixture := newTransactionFixture(t, "current-reader-finished", "current-reader-finished.nonce")
	transaction, err := sqlitetransaction.BeginRead(context.Background(), fixture.database)
	if err != nil {
		t.Fatal(err)
	}
	if finish := transaction.Rollback(context.Background()); !finish.Succeeded() {
		t.Fatal(finish.Err())
	}

	observation, err := ResolveCurrentWithin(
		context.Background(),
		transaction,
		fixture.root,
	)
	if observation != nil {
		t.Fatalf("finished-transaction observation = %T, want nil", observation)
	}
	assertCurrentProfileReadFailureKind(t, err, CurrentProfileStoreOperationalFailure)
}

func assertCurrentProfileReadFailureKind(
	t *testing.T,
	err error,
	want CurrentProfileReadFailureKind,
) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %s", want.String())
	}
	typed, ok := err.(CurrentProfileReadError)
	if !ok {
		t.Fatalf("error = %T, want CurrentProfileReadError: %v", err, err)
	}
	if typed.Kind() != want {
		t.Fatalf("error kind = %s, want %s: %v", typed.Kind().String(), want.String(), err)
	}
}
