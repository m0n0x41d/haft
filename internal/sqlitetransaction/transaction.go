// Package sqlitetransaction owns the concrete SQLite unit-of-work boundary used
// by reliance-bearing kernel adapters. It exposes no raw connection and no
// caller-implementable query interface.
package sqlitetransaction

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"
	"time"
)

const cleanupTimeout = 2 * time.Second

var (
	ErrContextRequired     = errors.New("SQLite transaction context is required")
	ErrDatabaseRequired    = errors.New("SQLite transaction database is required")
	ErrTransactionInvalid  = errors.New("SQLite transaction capability is invalid")
	ErrTransactionFinished = errors.New("SQLite transaction capability is finished")
	ErrImmediateRequired   = errors.New("SQLite immediate transaction is required")
)

type transactionMode uint8

const (
	readTransaction transactionMode = iota + 1
	immediateTransaction
)

type transactionPhase uint8

const (
	activeTransaction transactionPhase = iota + 1
	finishedTransaction
)

type transactionState struct {
	mutex      sync.Mutex
	connection *sql.Conn
	mode       transactionMode
	phase      transactionPhase
}

// Transaction is an opaque package-owned SQLite transaction capability. Only
// BeginRead and BeginImmediate can mint an active value. A zero value, a value
// after Commit or Rollback, and a structurally unrelated value fail closed.
type Transaction struct {
	state *transactionState
}

// FinishResult keeps the SQL statement result distinct from connection-close
// evidence. Commit callers need both to classify an ambiguous delivery.
type FinishResult struct {
	statementError error
	cleanupError   error
	closeError     error
}

func (result FinishResult) StatementError() error {
	return result.statementError
}

func (result FinishResult) CloseError() error {
	return result.closeError
}

func (result FinishResult) CleanupError() error {
	return result.cleanupError
}

func (result FinishResult) Err() error {
	return errors.Join(
		result.statementError,
		result.cleanupError,
		result.closeError,
	)
}

func (result FinishResult) Succeeded() bool {
	return result.statementError == nil &&
		result.cleanupError == nil &&
		result.closeError == nil
}

// BeginRead owns one dedicated connection and starts one deferred read
// transaction. The returned capability owns both rollback and close.
func BeginRead(ctx context.Context, database *sql.DB) (*Transaction, error) {
	return begin(ctx, database, readTransaction, "BEGIN")
}

// BeginImmediate owns one dedicated connection and acquires the SQLite write
// reservation before returning. The returned capability owns commit/rollback
// and close.
func BeginImmediate(ctx context.Context, database *sql.DB) (*Transaction, error) {
	return begin(ctx, database, immediateTransaction, "BEGIN IMMEDIATE")
}

func begin(
	ctx context.Context,
	database *sql.DB,
	mode transactionMode,
	statement string,
) (*Transaction, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if database == nil {
		return nil, ErrDatabaseRequired
	}
	connection, err := database.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire dedicated SQLite connection: %w", err)
	}
	_, beginErr := connection.ExecContext(ctx, statement)
	if beginErr != nil {
		cleanupErr := cleanupFailedFinish(connection, beginErr)
		closeErr := connection.Close()
		return nil, errors.Join(
			fmt.Errorf("start SQLite transaction: %w", beginErr),
			cleanupErr,
			closeErr,
		)
	}
	state := transactionState{
		connection: connection,
		mode:       mode,
		phase:      activeTransaction,
	}
	return &Transaction{state: &state}, nil
}

// RequireActive validates the opaque lifecycle without exposing its
// connection. Reliance-bearing entry points call it before doing local work.
func (transaction *Transaction) RequireActive() error {
	state, err := transactionStateOf(transaction)
	if err != nil {
		return err
	}
	state.mutex.Lock()
	defer state.mutex.Unlock()
	return validateActiveState(state)
}

// RequireImmediate additionally proves that BEGIN IMMEDIATE succeeded for this
// capability. Read-only transactions cannot cross a storage mutation boundary.
func (transaction *Transaction) RequireImmediate() error {
	state, err := transactionStateOf(transaction)
	if err != nil {
		return err
	}
	state.mutex.Lock()
	defer state.mutex.Unlock()
	err = validateActiveState(state)
	if err != nil {
		return err
	}
	if state.mode != immediateTransaction {
		return ErrImmediateRequired
	}
	return nil
}

// Execute performs one write statement while holding the transaction lifecycle
// lock for the complete SQL effect. It is unavailable on read transactions.
func (transaction *Transaction) Execute(
	ctx context.Context,
	statement string,
	arguments []any,
) (sql.Result, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	state, err := transactionStateOf(transaction)
	if err != nil {
		return nil, err
	}
	state.mutex.Lock()
	defer state.mutex.Unlock()
	err = validateActiveState(state)
	if err != nil {
		return nil, err
	}
	if state.mode != immediateTransaction {
		return nil, ErrImmediateRequired
	}
	return state.connection.ExecContext(ctx, statement, arguments...)
}

// ScanOne performs and scans one query while holding the transaction lifecycle
// lock for the complete SQL effect. No row or raw connection escapes.
func (transaction *Transaction) ScanOne(
	ctx context.Context,
	statement string,
	arguments []any,
	destinations []any,
) error {
	if ctx == nil {
		return ErrContextRequired
	}
	state, err := transactionStateOf(transaction)
	if err != nil {
		return err
	}
	state.mutex.Lock()
	defer state.mutex.Unlock()
	err = validateActiveState(state)
	if err != nil {
		return err
	}
	row := state.connection.QueryRowContext(ctx, statement, arguments...)
	return row.Scan(destinations...)
}

// Commit with a non-nil context attempts COMMIT, closes the dedicated
// connection, and permanently finishes the capability regardless of the SQL
// outcome. A nil context is an invalid call and leaves the capability active so
// the owner can retry Commit or Rollback with a valid context.
func (transaction *Transaction) Commit(ctx context.Context) FinishResult {
	return transaction.finish(ctx, "COMMIT")
}

// Rollback with a non-nil context attempts ROLLBACK, closes the dedicated
// connection, and permanently finishes the capability regardless of the SQL
// outcome. A nil context is an invalid call and leaves the capability active so
// the owner can retry Rollback with a valid context.
func (transaction *Transaction) Rollback(ctx context.Context) FinishResult {
	return transaction.finish(ctx, "ROLLBACK")
}

func (transaction *Transaction) finish(
	ctx context.Context,
	statement string,
) FinishResult {
	if ctx == nil {
		return FinishResult{statementError: ErrContextRequired}
	}
	state, err := transactionStateOf(transaction)
	if err != nil {
		return FinishResult{statementError: err}
	}
	state.mutex.Lock()
	defer state.mutex.Unlock()
	err = validateActiveState(state)
	if err != nil {
		return FinishResult{statementError: err}
	}
	_, statementErr := state.connection.ExecContext(ctx, statement)
	cleanupErr := cleanupFailedFinish(state.connection, statementErr)
	closeErr := state.connection.Close()
	state.phase = finishedTransaction
	state.connection = nil
	return FinishResult{
		statementError: statementErr,
		cleanupError:   cleanupErr,
		closeError:     closeErr,
	}
}

func cleanupFailedFinish(connection *sql.Conn, statementErr error) error {
	if statementErr == nil {
		return nil
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	_, rollbackErr := connection.ExecContext(cleanupCtx, "ROLLBACK")
	if rollbackErr == nil {
		return nil
	}
	invalidationErr := invalidateConnection(connection)
	return errors.Join(
		fmt.Errorf("cleanup failed SQLite transaction: %w", rollbackErr),
		invalidationErr,
	)
}

func invalidateConnection(connection *sql.Conn) error {
	err := connection.Raw(func(any) error {
		return driver.ErrBadConn
	})
	if errors.Is(err, driver.ErrBadConn) {
		return nil
	}
	return fmt.Errorf("invalidate failed SQLite connection: %w", err)
}

func transactionStateOf(transaction *Transaction) (*transactionState, error) {
	if transaction == nil || transaction.state == nil {
		return nil, ErrTransactionInvalid
	}
	return transaction.state, nil
}

func validateActiveState(state *transactionState) error {
	if state.phase == finishedTransaction {
		return ErrTransactionFinished
	}
	if state.phase != activeTransaction || state.connection == nil {
		return ErrTransactionInvalid
	}
	return nil
}
