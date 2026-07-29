package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
)

// MigrationTransaction is the transaction surface available to migrations
// that need custom, atomic reconciliation logic.
type MigrationTransaction interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// MigrationApplyBoundary is a closed choice of SQLite effect boundary for a
// custom migration. The zero value preserves the ordinary foreign-key-enforced
// transaction. ForeignKeyTableRebuildBoundary is reserved for migrations that
// must replace an existing FK target table without weakening any committed
// database state.
type MigrationApplyBoundary uint8

const (
	ForeignKeysEnforcedBoundary MigrationApplyBoundary = iota
	ForeignKeyTableRebuildBoundary
)

// MigrationForeignKeyVerifier is the domain port for validating the complete
// post-apply foreign-key state of a table-rebuild migration. The runner owns
// transaction timing and rollback; the migration owns any exact,
// independently witnessed polymorphic relation that its schema admits.
type MigrationForeignKeyVerifier func(MigrationTransaction) error

// Migration defines a single versioned schema change.
type Migration struct {
	Version            int
	Description        string
	Statements         []string // executed sequentially within the version
	Apply              func(MigrationTransaction, []Migration) error
	ApplyBoundary      MigrationApplyBoundary
	ForeignKeyVerifier MigrationForeignKeyVerifier
}

// Migrate applies all pending migrations to the database.
// Tracks applied versions in the given table name (e.g., "schema_version").
// Skips already-applied versions. Idempotent for ALTER TABLE / CREATE TABLE
// statements (catches "duplicate column" and "already exists" errors).
//
// Statement migrations use only their supplied SQL. Custom Apply callbacks are
// serialized through SQLite BEGIN IMMEDIATE and may be backend-specific.
func Migrate(conn *sql.DB, versionTable string, migrations []Migration) error {
	if err := rejectFutureMigrationEdition(
		conn,
		versionTable,
		migrations,
	); err != nil {
		return err
	}
	// Ensure version tracking table exists
	_, err := conn.Exec(fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s (version INTEGER PRIMARY KEY, applied_at TEXT DEFAULT CURRENT_TIMESTAMP)`,
		versionTable,
	))
	if err != nil {
		return fmt.Errorf("create %s table: %w", versionTable, err)
	}

	for _, m := range migrations {
		// Skip already-applied migrations
		var exists int
		row := conn.QueryRow(
			fmt.Sprintf("SELECT 1 FROM %s WHERE version = ?", versionTable),
			m.Version,
		)
		if row.Scan(&exists) == nil && exists == 1 {
			continue
		}
		if m.Apply != nil {
			if len(m.Statements) != 0 {
				return fmt.Errorf("migration %d (%s) defines both statements and a transactional apply callback", m.Version, m.Description)
			}
			if err := applyCustomMigration(conn, versionTable, migrations, m); err != nil {
				return err
			}
			continue
		}
		if m.ApplyBoundary != ForeignKeysEnforcedBoundary {
			return fmt.Errorf(
				"migration %d (%s) requests a custom apply boundary without an Apply callback",
				m.Version,
				m.Description,
			)
		}
		if m.ForeignKeyVerifier != nil {
			return fmt.Errorf(
				"migration %d (%s) defines a foreign-key verifier without a table-rebuild Apply callback",
				m.Version,
				m.Description,
			)
		}

		// Execute all statements for this migration
		for _, stmt := range m.Statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, execErr := conn.Exec(stmt); execErr != nil {
				if !isIdempotentError(execErr) {
					return fmt.Errorf("migration %d (%s) failed: %w", m.Version, m.Description, execErr)
				}
			}
		}

		// Record applied version
		if _, err := conn.Exec(
			fmt.Sprintf("INSERT INTO %s (version) VALUES (?)", versionTable),
			m.Version,
		); err != nil {
			return fmt.Errorf("record migration %d: %w", m.Version, err)
		}
	}

	return nil
}

func rejectFutureMigrationEdition(
	connection *sql.DB,
	versionTable string,
	migrations []Migration,
) error {
	if connection == nil {
		return fmt.Errorf("migration database is required")
	}
	maximumSupported := 0
	for _, migration := range migrations {
		if migration.Version > maximumSupported {
			maximumSupported = migration.Version
		}
	}
	exists := 0
	err := connection.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
		versionTable,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf(
			"inspect %s before migration: %w",
			versionTable,
			err,
		)
	}
	if exists == 0 {
		return nil
	}
	observedMaximum := 0
	err = connection.QueryRow(fmt.Sprintf(
		"SELECT COALESCE(MAX(version), 0) FROM %s",
		versionTable,
	)).Scan(&observedMaximum)
	if err != nil {
		return fmt.Errorf(
			"read current %s edition before migration: %w",
			versionTable,
			err,
		)
	}
	if observedMaximum <= maximumSupported {
		return nil
	}
	return fmt.Errorf(
		"refuse migration: database %s edition %d is newer than this binary's supported edition %d; use an equal or newer Haft binary",
		versionTable,
		observedMaximum,
		maximumSupported,
	)
}

func applyCustomMigration(
	conn *sql.DB,
	versionTable string,
	migrations []Migration,
	migration Migration,
) error {
	switch migration.ApplyBoundary {
	case ForeignKeysEnforcedBoundary:
		if migration.ForeignKeyVerifier != nil {
			return fmt.Errorf(
				"migration %d (%s) defines a foreign-key verifier outside the table-rebuild boundary",
				migration.Version,
				migration.Description,
			)
		}
		return applyTransactionalMigration(
			conn,
			versionTable,
			migrations,
			migration,
		)
	case ForeignKeyTableRebuildBoundary:
		return applyForeignKeyTableRebuildMigration(
			conn,
			versionTable,
			migrations,
			migration,
		)
	default:
		return fmt.Errorf(
			"migration %d (%s) has unknown custom apply boundary %d",
			migration.Version,
			migration.Description,
			migration.ApplyBoundary,
		)
	}
}

func applyTransactionalMigration(
	conn *sql.DB,
	versionTable string,
	migrations []Migration,
	migration Migration,
) error {
	tx, err := beginImmediateMigrationTransaction(conn)
	if err != nil {
		return fmt.Errorf("begin migration %d (%s): %w", migration.Version, migration.Description, err)
	}
	alreadyApplied, err := migrationVersionExists(tx, versionTable, migration.Version)
	if err != nil {
		return errors.Join(
			fmt.Errorf("recheck migration %d: %w", migration.Version, err),
			tx.Rollback(),
		)
	}
	if alreadyApplied {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit concurrent migration %d recheck: %w", migration.Version, err)
		}
		return nil
	}
	if err := migration.Apply(tx, migrations); err != nil {
		return errors.Join(
			fmt.Errorf(
				"migration %d (%s) failed: %w",
				migration.Version,
				migration.Description,
				err,
			),
			tx.Rollback(),
		)
	}
	_, err = tx.Exec(
		fmt.Sprintf("INSERT INTO %s (version) VALUES (?)", versionTable),
		migration.Version,
	)
	if err != nil {
		return errors.Join(
			fmt.Errorf("record migration %d: %w", migration.Version, err),
			tx.Rollback(),
		)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %d (%s): %w", migration.Version, migration.Description, err)
	}
	return nil
}

func applyForeignKeyTableRebuildMigration(
	database *sql.DB,
	versionTable string,
	migrations []Migration,
	migration Migration,
) error {
	return applyForeignKeyTableRebuildMigrationWithRestorer(
		database,
		versionTable,
		migrations,
		migration,
		restoreConnectionForeignKeys,
	)
}

func applyForeignKeyTableRebuildMigrationWithRestorer(
	database *sql.DB,
	versionTable string,
	migrations []Migration,
	migration Migration,
	restoreForeignKeys func(context.Context, *sql.Conn) error,
) (resultErr error) {
	if database == nil {
		return fmt.Errorf(
			"migration %d (%s) requires a database",
			migration.Version,
			migration.Description,
		)
	}
	ctx := context.Background()
	connection, err := database.Conn(ctx)
	if err != nil {
		return fmt.Errorf(
			"reserve connection for migration %d (%s): %w",
			migration.Version,
			migration.Description,
			err,
		)
	}
	defer func() {
		restoreErr := restoreForeignKeys(ctx, connection)
		if restoreErr != nil {
			discardErr := discardReservedMigrationConnection(connection)
			resultErr = errors.Join(resultErr, restoreErr, discardErr)
			return
		}
		resultErr = errors.Join(resultErr, connection.Close())
	}()
	if err := requireConnectionForeignKeys(ctx, connection, 1); err != nil {
		return fmt.Errorf(
			"prepare migration %d (%s) FK-table rebuild: %w",
			migration.Version,
			migration.Description,
			err,
		)
	}
	if err := setConnectionForeignKeys(ctx, connection, false); err != nil {
		return fmt.Errorf(
			"prepare migration %d (%s) FK-table rebuild: %w",
			migration.Version,
			migration.Description,
			err,
		)
	}
	if _, err := connection.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf(
			"begin migration %d (%s): %w",
			migration.Version,
			migration.Description,
			err,
		)
	}
	transaction := &reservedConnectionMigrationTransaction{
		context:    ctx,
		connection: connection,
	}
	fail := func(cause error) error {
		rollbackErr := finishReservedMigrationTransaction(
			ctx,
			connection,
			"ROLLBACK",
		)
		return errors.Join(cause, rollbackErr)
	}
	alreadyApplied, err := migrationVersionExists(
		transaction,
		versionTable,
		migration.Version,
	)
	if err != nil {
		return fail(fmt.Errorf("recheck migration %d: %w", migration.Version, err))
	}
	if alreadyApplied {
		if err := finishReservedMigrationTransaction(
			ctx,
			connection,
			"COMMIT",
		); err != nil {
			return fail(fmt.Errorf(
				"commit concurrent migration %d recheck: %w",
				migration.Version,
				err,
			))
		}
		return nil
	}
	if err := migration.Apply(transaction, migrations); err != nil {
		return fail(fmt.Errorf(
			"migration %d (%s) failed: %w",
			migration.Version,
			migration.Description,
			err,
		))
	}
	_, err = transaction.Exec(
		fmt.Sprintf("INSERT INTO %s (version) VALUES (?)", versionTable),
		migration.Version,
	)
	if err != nil {
		return fail(fmt.Errorf("record migration %d: %w", migration.Version, err))
	}
	if err := verifyForeignKeyTableRebuildResult(
		ctx,
		connection,
		transaction,
		migration.ForeignKeyVerifier,
	); err != nil {
		return fail(fmt.Errorf(
			"migration %d (%s) failed: %w",
			migration.Version,
			migration.Description,
			err,
		))
	}
	if err := finishReservedMigrationTransaction(
		ctx,
		connection,
		"COMMIT",
	); err != nil {
		return fail(fmt.Errorf(
			"commit migration %d (%s): %w",
			migration.Version,
			migration.Description,
			err,
		))
	}
	return nil
}

func verifyForeignKeyTableRebuildResult(
	ctx context.Context,
	connection *sql.Conn,
	transaction MigrationTransaction,
	verifier MigrationForeignKeyVerifier,
) error {
	if verifier == nil {
		return requireNoForeignKeyViolations(ctx, connection)
	}
	return verifier(transaction)
}

type reservedConnectionMigrationTransaction struct {
	context    context.Context
	connection *sql.Conn
}

func (transaction *reservedConnectionMigrationTransaction) Exec(
	query string,
	args ...any,
) (sql.Result, error) {
	return transaction.connection.ExecContext(transaction.context, query, args...)
}

func (transaction *reservedConnectionMigrationTransaction) Query(
	query string,
	args ...any,
) (*sql.Rows, error) {
	return transaction.connection.QueryContext(transaction.context, query, args...)
}

func (transaction *reservedConnectionMigrationTransaction) QueryRow(
	query string,
	args ...any,
) *sql.Row {
	return transaction.connection.QueryRowContext(transaction.context, query, args...)
}

func setConnectionForeignKeys(
	ctx context.Context,
	connection *sql.Conn,
	enabled bool,
) error {
	value := 0
	if enabled {
		value = 1
	}
	if _, err := connection.ExecContext(
		ctx,
		fmt.Sprintf("PRAGMA foreign_keys = %d", value),
	); err != nil {
		return fmt.Errorf("set SQLite foreign_keys=%d: %w", value, err)
	}
	if err := requireConnectionForeignKeys(ctx, connection, value); err != nil {
		return err
	}
	return nil
}

func restoreConnectionForeignKeys(
	ctx context.Context,
	connection *sql.Conn,
) error {
	first := setConnectionForeignKeys(ctx, connection, true)
	if first == nil {
		return nil
	}
	second := setConnectionForeignKeys(ctx, connection, true)
	if second == nil {
		return nil
	}
	return errors.Join(
		fmt.Errorf("first SQLite foreign-key restoration attempt: %w", first),
		fmt.Errorf("second SQLite foreign-key restoration attempt: %w", second),
	)
}

func discardReservedMigrationConnection(connection *sql.Conn) error {
	if connection == nil {
		return fmt.Errorf("discard reserved migration connection: connection is required")
	}
	rawErr := connection.Raw(func(any) error {
		return driver.ErrBadConn
	})
	if rawErr != nil && !errors.Is(rawErr, driver.ErrBadConn) {
		return errors.Join(
			fmt.Errorf("mark reserved migration connection bad: %w", rawErr),
			connection.Close(),
		)
	}
	return connection.Close()
}

func requireConnectionForeignKeys(
	ctx context.Context,
	connection *sql.Conn,
	expected int,
) error {
	var actual int
	if err := connection.QueryRowContext(
		ctx,
		"PRAGMA foreign_keys",
	).Scan(&actual); err != nil {
		return fmt.Errorf("read SQLite foreign_keys: %w", err)
	}
	if actual != expected {
		return fmt.Errorf(
			"SQLite foreign_keys=%d; require %d",
			actual,
			expected,
		)
	}
	return nil
}

func requireNoForeignKeyViolations(
	ctx context.Context,
	connection *sql.Conn,
) error {
	rows, err := connection.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("run SQLite foreign_key_check: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return fmt.Errorf("drain SQLite foreign_key_check: %w", err)
		}
		return nil
	}
	var table string
	var rowID sql.NullInt64
	var parent string
	var foreignKeyID int
	if err := rows.Scan(
		&table,
		&rowID,
		&parent,
		&foreignKeyID,
	); err != nil {
		return fmt.Errorf("scan SQLite foreign_key_check: %w", err)
	}
	for rows.Next() {
		var ignoredTable string
		var ignoredRowID sql.NullInt64
		var ignoredParent string
		var ignoredForeignKeyID int
		if err := rows.Scan(
			&ignoredTable,
			&ignoredRowID,
			&ignoredParent,
			&ignoredForeignKeyID,
		); err != nil {
			return fmt.Errorf("drain SQLite foreign_key_check: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("drain SQLite foreign_key_check: %w", err)
	}
	row := "WITHOUT ROWID"
	if rowID.Valid {
		row = fmt.Sprintf("%d", rowID.Int64)
	}
	return fmt.Errorf(
		"SQLite foreign_key_check found %s row %s referencing %s (fk %d)",
		table,
		row,
		parent,
		foreignKeyID,
	)
}

func finishReservedMigrationTransaction(
	ctx context.Context,
	connection *sql.Conn,
	statement string,
) error {
	if _, err := connection.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("%s SQLite migration transaction: %w", statement, err)
	}
	return nil
}

type immediateMigrationTransaction struct {
	context    context.Context
	connection *sql.Conn
}

func beginImmediateMigrationTransaction(conn *sql.DB) (*immediateMigrationTransaction, error) {
	ctx := context.Background()
	connection, err := conn.Conn(ctx)
	if err != nil {
		return nil, err
	}
	transaction := &immediateMigrationTransaction{
		context:    ctx,
		connection: connection,
	}
	if err := setConnectionForeignKeys(ctx, connection, true); err != nil {
		return nil, errors.Join(
			err,
			discardReservedMigrationConnection(connection),
		)
	}
	if _, err := transaction.Exec("BEGIN IMMEDIATE"); err != nil {
		return nil, errors.Join(
			err,
			discardReservedMigrationConnection(connection),
		)
	}
	return transaction, nil
}

func (transaction *immediateMigrationTransaction) Exec(
	query string,
	args ...any,
) (sql.Result, error) {
	return transaction.connection.ExecContext(transaction.context, query, args...)
}

func (transaction *immediateMigrationTransaction) Query(
	query string,
	args ...any,
) (*sql.Rows, error) {
	return transaction.connection.QueryContext(transaction.context, query, args...)
}

func (transaction *immediateMigrationTransaction) QueryRow(
	query string,
	args ...any,
) *sql.Row {
	return transaction.connection.QueryRowContext(transaction.context, query, args...)
}

func (transaction *immediateMigrationTransaction) Commit() error {
	return transaction.finish("COMMIT")
}

func (transaction *immediateMigrationTransaction) Rollback() error {
	return transaction.finish("ROLLBACK")
}

func (transaction *immediateMigrationTransaction) finish(statement string) error {
	if _, err := transaction.Exec(statement); err != nil {
		return errors.Join(
			err,
			discardReservedMigrationConnection(transaction.connection),
		)
	}
	return transaction.connection.Close()
}

func migrationVersionExists(
	queryer interface {
		QueryRow(query string, args ...any) *sql.Row
	},
	versionTable string,
	version int,
) (bool, error) {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE version = ?", versionTable)
	row := queryer.QueryRow(query, version)
	err := row.Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 1, nil
}

// isIdempotentError returns true for errors that mean "already done" —
// safe to ignore when re-running migrations on existing databases.
func isIdempotentError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate column") ||
		strings.Contains(msg, "already exists")
}
