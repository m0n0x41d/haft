package sqlitetransaction

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestImmediateTransactionOwnsEffectAndLifecycle(t *testing.T) {
	database := openTransactionDatabase(t)
	ctx := context.Background()
	transaction, err := BeginImmediate(ctx, database)
	if err != nil {
		t.Fatalf("BeginImmediate: %v", err)
	}
	if err := transaction.RequireImmediate(); err != nil {
		t.Fatalf("RequireImmediate: %v", err)
	}
	_, err = transaction.Execute(
		ctx,
		"INSERT INTO transaction_probe (id, value) VALUES (?, ?)",
		[]any{1, "committed"},
	)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var value string
	err = transaction.ScanOne(
		ctx,
		"SELECT value FROM transaction_probe WHERE id = ?",
		[]any{1},
		[]any{&value},
	)
	if err != nil {
		t.Fatalf("ScanOne: %v", err)
	}
	if value != "committed" {
		t.Fatalf("value = %q, want committed", value)
	}
	result := transaction.Commit(ctx)
	if !result.Succeeded() {
		t.Fatalf("Commit: %v", result.Err())
	}
	if err := transaction.RequireActive(); !errors.Is(err, ErrTransactionFinished) {
		t.Fatalf("RequireActive after Commit = %v", err)
	}
	_, err = transaction.Execute(ctx, "DELETE FROM transaction_probe", []any{})
	if !errors.Is(err, ErrTransactionFinished) {
		t.Fatalf("Execute after Commit = %v", err)
	}
	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM transaction_probe").Scan(&count)
	if err != nil {
		t.Fatalf("count committed rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("committed row count = %d, want 1", count)
	}
}

func TestReadTransactionRejectsMutationAndRollbackFinishes(t *testing.T) {
	database := openTransactionDatabase(t)
	ctx := context.Background()
	transaction, err := BeginRead(ctx, database)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	if err := transaction.RequireActive(); err != nil {
		t.Fatalf("RequireActive: %v", err)
	}
	if err := transaction.RequireImmediate(); !errors.Is(err, ErrImmediateRequired) {
		t.Fatalf("RequireImmediate on read transaction = %v", err)
	}
	_, err = transaction.Execute(
		ctx,
		"INSERT INTO transaction_probe (id, value) VALUES (?, ?)",
		[]any{1, "forbidden"},
	)
	if !errors.Is(err, ErrImmediateRequired) {
		t.Fatalf("read transaction Execute = %v", err)
	}
	result := transaction.Rollback(ctx)
	if !result.Succeeded() {
		t.Fatalf("Rollback: %v", result.Err())
	}
	if err := transaction.RequireActive(); !errors.Is(err, ErrTransactionFinished) {
		t.Fatalf("RequireActive after Rollback = %v", err)
	}
}

func TestNilFinishContextIsRetryableAndDoesNotConsumeCapability(t *testing.T) {
	database := openTransactionDatabase(t)
	transaction, err := BeginRead(context.Background(), database)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	result := transaction.Commit(nil)
	if !errors.Is(result.StatementError(), ErrContextRequired) {
		t.Fatalf("Commit(nil) = %v, want context-required", result.Err())
	}
	if err := transaction.RequireActive(); err != nil {
		t.Fatalf("RequireActive after Commit(nil): %v", err)
	}
	result = transaction.Rollback(context.Background())
	if !result.Succeeded() {
		t.Fatalf("Rollback after Commit(nil): %v", result.Err())
	}
}

func TestZeroAndNilTransactionsFailClosed(t *testing.T) {
	ctx := context.Background()
	zero := &Transaction{}
	assertInvalidTransaction(t, ctx, nil)
	assertInvalidTransaction(t, ctx, zero)
}

func TestFailedCommitCleansUpBeforeConnectionReuse(t *testing.T) {
	database := openTransactionDatabase(t)
	_, err := database.Exec(`CREATE TABLE transaction_parent (
		id INTEGER PRIMARY KEY
	)`)
	if err != nil {
		t.Fatalf("create parent table: %v", err)
	}
	_, err = database.Exec(`CREATE TABLE transaction_child (
		id INTEGER PRIMARY KEY,
		parent_id INTEGER NOT NULL,
		FOREIGN KEY(parent_id) REFERENCES transaction_parent(id)
			DEFERRABLE INITIALLY DEFERRED
	)`)
	if err != nil {
		t.Fatalf("create child table: %v", err)
	}
	ctx := context.Background()
	transaction, err := BeginImmediate(ctx, database)
	if err != nil {
		t.Fatalf("BeginImmediate: %v", err)
	}
	_, err = transaction.Execute(
		ctx,
		"INSERT INTO transaction_child (id, parent_id) VALUES (?, ?)",
		[]any{1, 999},
	)
	if err != nil {
		t.Fatalf("insert deferred-FK row: %v", err)
	}
	result := transaction.Commit(ctx)
	if result.StatementError() == nil {
		t.Fatal("deferred-FK Commit unexpectedly succeeded")
	}
	if result.CleanupError() != nil {
		t.Fatalf("failed Commit cleanup: %v", result.CleanupError())
	}
	next, err := BeginImmediate(ctx, database)
	if err != nil {
		t.Fatalf("BeginImmediate after failed Commit: %v", err)
	}
	var count int
	err = next.ScanOne(
		ctx,
		"SELECT COUNT(*) FROM transaction_child",
		[]any{},
		[]any{&count},
	)
	if err != nil {
		t.Fatalf("count child rows after cleanup: %v", err)
	}
	if count != 0 {
		t.Fatalf("child rows after failed Commit = %d, want 0", count)
	}
	finish := next.Rollback(ctx)
	if !finish.Succeeded() {
		t.Fatalf("finish next transaction: %v", finish.Err())
	}
}

func TestFailedBeginDoesNotReturnAmbiguousConnectionToPool(t *testing.T) {
	database := openTransactionDatabase(t)
	database.SetMaxOpenConns(2)
	database.SetMaxIdleConns(2)
	blocker, err := BeginImmediate(context.Background(), database)
	if err != nil {
		t.Fatalf("begin blocking transaction: %v", err)
	}
	blockedCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = BeginImmediate(blockedCtx, database)
	if err == nil {
		t.Fatal("contending BeginImmediate unexpectedly succeeded")
	}
	finish := blocker.Rollback(context.Background())
	if !finish.Succeeded() {
		t.Fatalf("finish blocking transaction: %v", finish.Err())
	}
	next, err := BeginImmediate(context.Background(), database)
	if err != nil {
		t.Fatalf("BeginImmediate after failed begin cleanup: %v", err)
	}
	finish = next.Rollback(context.Background())
	if !finish.Succeeded() {
		t.Fatalf("finish transaction after failed begin cleanup: %v", finish.Err())
	}
}

func assertInvalidTransaction(
	t *testing.T,
	ctx context.Context,
	transaction *Transaction,
) {
	t.Helper()
	if err := transaction.RequireActive(); !errors.Is(err, ErrTransactionInvalid) {
		t.Fatalf("RequireActive = %v, want invalid", err)
	}
	_, err := transaction.Execute(ctx, "SELECT 1", []any{})
	if !errors.Is(err, ErrTransactionInvalid) {
		t.Fatalf("Execute = %v, want invalid", err)
	}
	var value int
	err = transaction.ScanOne(ctx, "SELECT 1", []any{}, []any{&value})
	if !errors.Is(err, ErrTransactionInvalid) {
		t.Fatalf("ScanOne = %v, want invalid", err)
	}
}

func openTransactionDatabase(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transaction.db")
	database, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	_, err = database.Exec(`CREATE TABLE transaction_probe (
		id INTEGER PRIMARY KEY,
		value TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("create transaction probe: %v", err)
	}
	return database
}
