package projecttypeenvheadstore

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstagerevalidation"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

func TestLoadCurrentProjectTypeEnvHeadTxDistinguishesAbsenceAndCurrent(
	t *testing.T,
) {
	fixture := newHeadStoreFixture(t)
	ctx := context.Background()
	read, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead(absence): %v", err)
	}
	observation, err := fixture.store.LoadCurrentProjectTypeEnvHeadTx(
		ctx,
		read,
		fixture.project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentProjectTypeEnvHeadTx(absence): %v", err)
	}
	absent, ok := observation.(projecttypeenvstagerevalidation.ObservedNoProjectTypeEnvHead)
	if !ok {
		t.Fatalf("absence observation = %T", observation)
	}
	if absent.Project() != fixture.project ||
		absent.Head().Project() != fixture.project {
		t.Fatal("absence observation lost the exact project/head coordinate")
	}
	if err := read.RequireActive(); err != nil {
		t.Fatalf("load finished caller read transaction: %v", err)
	}
	if finish := read.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback absence read: %v", finish.Err())
	}
	if current, history := countHeadRows(t, fixture.database); current != 0 ||
		history != 0 {
		t.Fatalf(
			"absence read changed rows: current=%d history=%d",
			current,
			history,
		)
	}

	genesis := headStateFixture(t, fixture.project, 1, "1")
	write, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(Genesis): %v", err)
	}
	if err := fixture.store.CompareAndSwapGenesisProjectTypeEnvHeadTx(
		ctx,
		write,
		genesis,
	); err != nil {
		t.Fatalf("CompareAndSwapGenesisProjectTypeEnvHeadTx(): %v", err)
	}
	if err := write.RequireImmediate(); err != nil {
		t.Fatalf("Genesis CAS finished caller transaction: %v", err)
	}
	if finish := write.Commit(ctx); !finish.Succeeded() {
		t.Fatalf("commit Genesis: %v", finish.Err())
	}

	read, err = sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead(current): %v", err)
	}
	observation, err = fixture.store.LoadCurrentProjectTypeEnvHeadTx(
		ctx,
		read,
		fixture.project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentProjectTypeEnvHeadTx(current): %v", err)
	}
	current, ok := observation.(projecttypeenvstagerevalidation.ObservedProjectTypeEnvHead)
	if !ok {
		t.Fatalf("current observation = %T", observation)
	}
	if !sameHeadState(current.State(), genesis) {
		t.Fatal("current observation differs from committed Genesis state")
	}
	if finish := read.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback current read: %v", finish.Err())
	}
}

func TestHeadCASIsCallerOwnedAtomicAndTransitionExact(t *testing.T) {
	fixture := newHeadStoreFixture(t)
	ctx := context.Background()
	genesis := headStateFixture(t, fixture.project, 1, "2")

	transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(rollback): %v", err)
	}
	if err := fixture.store.CompareAndSwapGenesisProjectTypeEnvHeadTx(
		ctx,
		transaction,
		genesis,
	); err != nil {
		t.Fatalf("Genesis before rollback: %v", err)
	}
	if finish := transaction.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback Genesis: %v", finish.Err())
	}
	if current, history := countHeadRows(t, fixture.database); current != 0 ||
		history != 0 {
		t.Fatalf(
			"rolled-back CAS left rows: current=%d history=%d",
			current,
			history,
		)
	}

	transaction, err = sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(Genesis): %v", err)
	}
	if err := fixture.store.CompareAndSwapGenesisProjectTypeEnvHeadTx(
		ctx,
		transaction,
		genesis,
	); err != nil {
		t.Fatalf("Genesis: %v", err)
	}
	if finish := transaction.Commit(ctx); !finish.Succeeded() {
		t.Fatalf("commit Genesis: %v", finish.Err())
	}

	successor := headStateFixture(t, fixture.project, 2, "3")
	transaction, err = sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(Transition): %v", err)
	}
	if err := fixture.store.CompareAndSwapTransitionProjectTypeEnvHeadTx(
		ctx,
		transaction,
		genesis,
		successor,
	); err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if finish := transaction.Commit(ctx); !finish.Succeeded() {
		t.Fatalf("commit Transition: %v", finish.Err())
	}
	if current, history := countHeadRows(t, fixture.database); current != 1 ||
		history != 2 {
		t.Fatalf(
			"Transition rows: current=%d history=%d, want 1/2",
			current,
			history,
		)
	}

	transaction, err = sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(stale): %v", err)
	}
	staleSuccessor := headStateFixture(t, fixture.project, 2, "4")
	err = fixture.store.CompareAndSwapTransitionProjectTypeEnvHeadTx(
		ctx,
		transaction,
		genesis,
		staleSuccessor,
	)
	if !errors.Is(err, ErrProjectTypeEnvHeadCASConflict) {
		t.Fatalf("stale Transition error = %v", err)
	}
	if finish := transaction.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback stale Transition: %v", finish.Err())
	}
	if current, history := countHeadRows(t, fixture.database); current != 1 ||
		history != 2 {
		t.Fatalf(
			"stale Transition changed rows: current=%d history=%d",
			current,
			history,
		)
	}
}

func TestHeadCASRejectsReadTransactionAndDoesNotFinishIt(t *testing.T) {
	fixture := newHeadStoreFixture(t)
	ctx := context.Background()
	genesis := headStateFixture(t, fixture.project, 1, "5")
	read, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead(): %v", err)
	}
	err = fixture.store.CompareAndSwapGenesisProjectTypeEnvHeadTx(
		ctx,
		read,
		genesis,
	)
	if !errors.Is(err, sqlitetransaction.ErrImmediateRequired) {
		t.Fatalf("Genesis on read transaction = %v", err)
	}
	if err := read.RequireActive(); err != nil {
		t.Fatalf("rejected CAS finished read transaction: %v", err)
	}
	if finish := read.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback read transaction: %v", finish.Err())
	}
}

func TestLoadCurrentProjectTypeEnvHeadTxRejectsPositiveCorruptFootprints(
	t *testing.T,
) {
	t.Run("orphan history is not absence", func(t *testing.T) {
		fixture := newHeadStoreFixture(t)
		orphan := headStateFixture(t, fixture.project, 1, "6")
		insertHistoryRowDirect(t, fixture.database, orphan)
		assertHeadIntegrityFailure(t, fixture)
	})

	t.Run("current row canonical mismatch", func(t *testing.T) {
		fixture := newHeadStoreFixture(t)
		state := headStateFixture(t, fixture.project, 1, "7")
		row, err := prepareStoredHeadRow(state)
		if err != nil {
			t.Fatalf("prepareStoredHeadRow(): %v", err)
		}
		row.canonical = append([]byte(nil), row.canonical...)
		row.canonical[len(row.canonical)-1] ^= 0x01
		_, err = fixture.database.Exec(
			`INSERT INTO project_typeenv_heads (
				project_id,
				head_ref,
				head_revision,
				selected_composite_ref,
				state_digest,
				canonical_bytes
			) VALUES (?, ?, ?, ?, ?, ?)`,
			row.arguments()...,
		)
		if err != nil {
			t.Fatalf("insert malformed current row: %v", err)
		}
		assertHeadIntegrityFailure(t, fixture)
	})

	t.Run("history beyond current revision", func(t *testing.T) {
		fixture := newHeadStoreFixture(t)
		ctx := context.Background()
		genesis := headStateFixture(t, fixture.project, 1, "8")
		transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
		if err != nil {
			t.Fatalf("BeginImmediate(): %v", err)
		}
		if err := fixture.store.CompareAndSwapGenesisProjectTypeEnvHeadTx(
			ctx,
			transaction,
			genesis,
		); err != nil {
			t.Fatalf("Genesis: %v", err)
		}
		if finish := transaction.Commit(ctx); !finish.Succeeded() {
			t.Fatalf("commit Genesis: %v", finish.Err())
		}
		future := headStateFixture(t, fixture.project, 2, "9")
		insertHistoryRowDirect(t, fixture.database, future)
		assertHeadIntegrityFailure(t, fixture)
	})
}

func TestSchemaAndCurrentLoadAreIdempotentAndHistoryIsImmutable(t *testing.T) {
	fixture := newHeadStoreFixture(t)
	ctx := context.Background()
	reopened, err := New(ctx, fixture.database)
	if err != nil {
		t.Fatalf("second New(): %v", err)
	}
	if reopened == nil {
		t.Fatal("second New() returned nil store")
	}
	var version int
	err = fixture.database.QueryRow(
		`SELECT version
		FROM project_typeenv_head_store_schema
		WHERE singleton = 1`,
	).Scan(&version)
	if err != nil {
		t.Fatalf("load schema version: %v", err)
	}
	if version != CurrentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, CurrentSchemaVersion)
	}

	genesis := headStateFixture(t, fixture.project, 1, "a")
	transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(): %v", err)
	}
	if err := reopened.CompareAndSwapGenesisProjectTypeEnvHeadTx(
		ctx,
		transaction,
		genesis,
	); err != nil {
		t.Fatalf("Genesis: %v", err)
	}
	if finish := transaction.Commit(ctx); !finish.Succeeded() {
		t.Fatalf("commit Genesis: %v", finish.Err())
	}

	for index := 0; index < 2; index++ {
		read, err := sqlitetransaction.BeginRead(ctx, fixture.database)
		if err != nil {
			t.Fatalf("BeginRead(%d): %v", index, err)
		}
		observation, err := reopened.LoadCurrentProjectTypeEnvHeadTx(
			ctx,
			read,
			fixture.project,
		)
		if err != nil {
			t.Fatalf("LoadCurrent(%d): %v", index, err)
		}
		current, ok := observation.(projecttypeenvstagerevalidation.ObservedProjectTypeEnvHead)
		if !ok || !bytes.Equal(
			current.State().CanonicalBytes(),
			genesis.CanonicalBytes(),
		) {
			t.Fatalf("LoadCurrent(%d) did not return exact state", index)
		}
		if finish := read.Rollback(ctx); !finish.Succeeded() {
			t.Fatalf("rollback read %d: %v", index, finish.Err())
		}
	}
	if current, history := countHeadRows(t, fixture.database); current != 1 ||
		history != 1 {
		t.Fatalf(
			"repeated reads changed rows: current=%d history=%d",
			current,
			history,
		)
	}

	assertDirectMutationRejected(
		t,
		fixture,
		`UPDATE project_typeenv_head_states
		SET selected_composite_ref = selected_composite_ref
		WHERE project_id = ?`,
	)
	assertDirectMutationRejected(
		t,
		fixture,
		`DELETE FROM project_typeenv_head_states WHERE project_id = ?`,
	)
	assertDirectMutationRejected(
		t,
		fixture,
		`DELETE FROM project_typeenv_heads WHERE project_id = ?`,
	)
}

func TestHeadStoreRejectsInvalidBoundaries(t *testing.T) {
	fixture := newHeadStoreFixture(t)
	ctx := context.Background()
	genesis := headStateFixture(t, fixture.project, 1, "b")
	read, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead(): %v", err)
	}
	_, err = fixture.store.LoadCurrentProjectTypeEnvHeadTx(
		nil,
		read,
		fixture.project,
	)
	if !errors.Is(err, ErrContextRequired) {
		t.Fatalf("nil load context = %v", err)
	}
	var nilStore *Store
	_, err = nilStore.LoadCurrentProjectTypeEnvHeadTx(
		ctx,
		read,
		fixture.project,
	)
	if !errors.Is(err, ErrStoreRequired) {
		t.Fatalf("nil store load = %v", err)
	}
	if finish := read.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback read: %v", finish.Err())
	}

	tooLarge := headStateFixture(
		t,
		fixture.project,
		uint64(math.MaxInt64)+1,
		"c",
	)
	transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(): %v", err)
	}
	err = fixture.store.CompareAndSwapGenesisProjectTypeEnvHeadTx(
		ctx,
		transaction,
		tooLarge,
	)
	if !errors.Is(err, ErrHeadRevisionOutOfSQLiteRange) {
		t.Fatalf("out-of-range HeadRevision error = %v", err)
	}
	if finish := transaction.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback out-of-range CAS: %v", finish.Err())
	}

	transaction, err = sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(valid): %v", err)
	}
	if err := fixture.store.CompareAndSwapGenesisProjectTypeEnvHeadTx(
		ctx,
		transaction,
		genesis,
	); err != nil {
		t.Fatalf("valid Genesis after rejection: %v", err)
	}
	if finish := transaction.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback valid Genesis: %v", finish.Err())
	}
}

func TestSQLiteHeadRevisionPreservesTheSignedBoundary(t *testing.T) {
	maximum, err := projecttypeenvselection.NewHeadRevision(math.MaxInt64)
	if err != nil {
		t.Fatalf("NewHeadRevision(MaxInt64): %v", err)
	}
	stored, err := sqliteHeadRevision(maximum)
	if err != nil {
		t.Fatalf("sqliteHeadRevision(MaxInt64): %v", err)
	}
	if stored != math.MaxInt64 {
		t.Fatalf(
			"sqliteHeadRevision(MaxInt64) = %d, want %d",
			stored,
			int64(math.MaxInt64),
		)
	}

	overflow, err := projecttypeenvselection.NewHeadRevision(
		uint64(math.MaxInt64) + 1,
	)
	if err != nil {
		t.Fatalf("NewHeadRevision(MaxInt64+1): %v", err)
	}
	if _, err := sqliteHeadRevision(overflow); !errors.Is(
		err,
		ErrHeadRevisionOutOfSQLiteRange,
	) {
		t.Fatalf(
			"sqliteHeadRevision(MaxInt64+1) error = %v, want %v",
			err,
			ErrHeadRevisionOutOfSQLiteRange,
		)
	}
}

func assertHeadIntegrityFailure(t *testing.T, fixture headStoreFixture) {
	t.Helper()
	ctx := context.Background()
	read, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead(): %v", err)
	}
	_, err = fixture.store.LoadCurrentProjectTypeEnvHeadTx(
		ctx,
		read,
		fixture.project,
	)
	if !errors.Is(err, ErrStoredHeadIntegrity) {
		t.Fatalf("corrupt head load error = %v", err)
	}
	if err := read.RequireActive(); err != nil {
		t.Fatalf("corrupt load finished caller transaction: %v", err)
	}
	if finish := read.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback corrupt read: %v", finish.Err())
	}
}

func assertDirectMutationRejected(
	t *testing.T,
	fixture headStoreFixture,
	statement string,
) {
	t.Helper()
	if _, err := fixture.database.Exec(statement, fixture.project.String()); err == nil {
		t.Fatalf("direct mutation was accepted: %s", statement)
	}
}
