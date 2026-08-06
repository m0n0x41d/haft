package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestForeignKeyTableRebuildMigrationCommitsAndRestoresEnforcement(
	t *testing.T,
) {
	t.Parallel()

	database := openForeignKeyRebuildTestDatabase(t, 1)
	defer database.Close()
	seedForeignKeyRebuildParentAndChild(t, database)

	migration := foreignKeyRebuildTestMigration(
		1,
		func(transaction MigrationTransaction, _ []Migration) error {
			requireMigrationForeignKeys(t, transaction, 0)
			return rebuildForeignKeyTestParent(transaction, true)
		},
	)
	if err := Migrate(
		database,
		"fk_rebuild_schema_version",
		[]Migration{migration},
	); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	var payload string
	err := database.QueryRow(
		`SELECT parent.payload
		FROM fk_rebuild_child child
		JOIN fk_rebuild_parent parent ON parent.id = child.parent_id
		WHERE child.id = 'child'`,
	).Scan(&payload)
	if err != nil {
		t.Fatalf("read rebuilt parent through preserved child: %v", err)
	}
	if payload != "preserved" {
		t.Fatalf("rebuilt payload = %q; want preserved", payload)
	}
	requireDatabaseForeignKeys(t, database, 1)
	requireForeignKeyCheckClean(t, database)
}

func TestForeignKeyTableRebuildMigrationRollsBackApplyFailureAndRestoresEnforcement(
	t *testing.T,
) {
	t.Parallel()

	database := openForeignKeyRebuildTestDatabase(t, 1)
	defer database.Close()
	seedForeignKeyRebuildParentAndChild(t, database)
	injected := errors.New("injected apply failure")

	migration := foreignKeyRebuildTestMigration(
		1,
		func(transaction MigrationTransaction, _ []Migration) error {
			requireMigrationForeignKeys(t, transaction, 0)
			if _, err := transaction.Exec(
				"CREATE TABLE fk_rebuild_partial_marker (id INTEGER PRIMARY KEY)",
			); err != nil {
				return err
			}
			return injected
		},
	)
	err := Migrate(
		database,
		"fk_rebuild_schema_version",
		[]Migration{migration},
	)
	if !errors.Is(err, injected) {
		t.Fatalf("Migrate() error = %v; want injected failure", err)
	}
	requireSQLiteObjectCount(t, database, "table", "fk_rebuild_partial_marker", 0)
	requireMigrationVersionCount(t, database, "fk_rebuild_schema_version", 1, 0)
	requireDatabaseForeignKeys(t, database, 1)
	requireForeignKeyCheckClean(t, database)
}

func TestForeignKeyTableRebuildMigrationRejectsViolationBeforeVersionAndCommit(
	t *testing.T,
) {
	t.Parallel()

	database := openForeignKeyRebuildTestDatabase(t, 1)
	defer database.Close()
	seedForeignKeyRebuildParentAndChild(t, database)

	migration := foreignKeyRebuildTestMigration(
		1,
		func(transaction MigrationTransaction, _ []Migration) error {
			requireMigrationForeignKeys(t, transaction, 0)
			_, err := transaction.Exec("DELETE FROM fk_rebuild_parent")
			return err
		},
	)
	err := Migrate(
		database,
		"fk_rebuild_schema_version",
		[]Migration{migration},
	)
	if err == nil || !strings.Contains(err.Error(), "foreign_key_check") {
		t.Fatalf("Migrate() error = %v; want foreign_key_check rejection", err)
	}
	requireMigrationVersionCount(t, database, "fk_rebuild_schema_version", 1, 0)
	var parentCount int
	if scanErr := database.QueryRow(
		"SELECT COUNT(*) FROM fk_rebuild_parent",
	).Scan(&parentCount); scanErr != nil {
		t.Fatalf("count rolled-back parent: %v", scanErr)
	}
	if parentCount != 1 {
		t.Fatalf("parent count after rejected rebuild = %d; want 1", parentCount)
	}
	requireDatabaseForeignKeys(t, database, 1)
	requireForeignKeyCheckClean(t, database)
}

func TestForeignKeyTableRebuildMigrationChecksVersionInsertTriggerBeforeCommit(
	t *testing.T,
) {
	t.Parallel()

	database := openForeignKeyRebuildTestDatabase(t, 1)
	defer database.Close()
	seedForeignKeyRebuildParentAndChild(t, database)
	for _, statement := range []string{
		"CREATE TABLE fk_rebuild_schema_version (version INTEGER PRIMARY KEY)",
		"CREATE TABLE fk_version_trigger_parent (id TEXT PRIMARY KEY)",
		`CREATE TABLE fk_version_trigger_child (
			id TEXT PRIMARY KEY,
			parent_id TEXT NOT NULL REFERENCES fk_version_trigger_parent(id)
		)`,
		`CREATE TRIGGER fk_rebuild_version_poison
		AFTER INSERT ON fk_rebuild_schema_version
		BEGIN
			INSERT INTO fk_version_trigger_child (id, parent_id)
			VALUES ('poison', 'missing-parent');
		END`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("install version-trigger fixture with %q: %v", statement, err)
		}
	}
	migration := foreignKeyRebuildTestMigration(
		1,
		func(transaction MigrationTransaction, _ []Migration) error {
			return rebuildForeignKeyTestParent(transaction, true)
		},
	)
	err := Migrate(
		database,
		"fk_rebuild_schema_version",
		[]Migration{migration},
	)
	if err == nil || !strings.Contains(err.Error(), "foreign_key_check") {
		t.Fatalf("version-trigger migration error = %v; want FK rejection", err)
	}
	requireMigrationVersionCount(t, database, "fk_rebuild_schema_version", 1, 0)
	requireSQLiteTableRowCount(t, database, "fk_version_trigger_child", 0)
	requireForeignKeyCheckClean(t, database)
	requireDatabaseForeignKeys(t, database, 1)
}

func TestForeignKeyTableRebuildMigrationDiscardsConnectionWhenRestoreFails(
	t *testing.T,
) {
	t.Parallel()

	database := openForeignKeyRebuildTestDatabase(t, 1)
	defer database.Close()
	seedForeignKeyRebuildParentAndChild(t, database)
	if _, err := database.Exec(
		"CREATE TABLE fk_restore_schema_version (version INTEGER PRIMARY KEY)",
	); err != nil {
		t.Fatalf("create restore-failure version table: %v", err)
	}
	injected := errors.New("injected foreign-key restoration failure")
	migration := foreignKeyRebuildTestMigration(
		1,
		func(transaction MigrationTransaction, _ []Migration) error {
			return rebuildForeignKeyTestParent(transaction, true)
		},
	)
	err := applyForeignKeyTableRebuildMigrationWithRestorer(
		database,
		"fk_restore_schema_version",
		[]Migration{migration},
		migration,
		func(ctx context.Context, connection *sql.Conn) error {
			if checkErr := requireConnectionForeignKeys(
				ctx,
				connection,
				0,
			); checkErr != nil {
				return checkErr
			}
			return injected
		},
	)
	if !errors.Is(err, injected) {
		t.Fatalf("rebuild cleanup error = %v; want injected restoration failure", err)
	}
	if stats := database.Stats(); stats.OpenConnections != 0 {
		t.Fatalf(
			"unsafe FK-off connection returned to pool: %+v",
			stats,
		)
	}
	requireDatabaseForeignKeys(t, database, 1)
	requireMigrationVersionCount(t, database, "fk_restore_schema_version", 1, 1)
	requireForeignKeyCheckClean(t, database)
}

func TestForeignKeyTableRebuildMigrationConcurrentRecheckAppliesOnce(
	t *testing.T,
) {
	t.Parallel()

	database := openForeignKeyRebuildTestDatabase(t, 2)
	defer database.Close()
	seedForeignKeyRebuildParentAndChild(t, database)
	entered := make(chan struct{})
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	var applyCount atomic.Int32

	migration := foreignKeyRebuildTestMigration(
		1,
		func(transaction MigrationTransaction, _ []Migration) error {
			if applyCount.Add(1) == 1 {
				close(entered)
				<-release
			}
			return rebuildForeignKeyTestParent(transaction, true)
		},
	)
	results := make(chan error, 2)
	go func() {
		results <- Migrate(
			database,
			"fk_rebuild_schema_version",
			[]Migration{migration},
		)
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first concurrent migration did not enter Apply")
	}
	go func() {
		results <- Migrate(
			database,
			"fk_rebuild_schema_version",
			[]Migration{migration},
		)
	}()
	waitForTwoMigrationConnections(t, database)
	close(release)
	for range 2 {
		select {
		case err := <-results:
			if err != nil {
				t.Fatalf("concurrent Migrate() error = %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent migration did not return")
		}
	}
	if actual := applyCount.Load(); actual != 1 {
		t.Fatalf("Apply count = %d; want 1", actual)
	}
	requireMigrationVersionCount(t, database, "fk_rebuild_schema_version", 1, 1)
	requireDatabaseForeignKeys(t, database, 1)
	requireForeignKeyCheckClean(t, database)
}

func TestOrdinaryCustomMigrationKeepsForeignKeysEnforced(
	t *testing.T,
) {
	t.Parallel()

	database := openForeignKeyRebuildTestDatabase(t, 1)
	defer database.Close()

	migration := Migration{
		Version:     1,
		Description: "ordinary custom migration",
		Apply: func(transaction MigrationTransaction, _ []Migration) error {
			requireMigrationForeignKeys(t, transaction, 1)
			_, err := transaction.Exec(
				"CREATE TABLE ordinary_custom_migration_marker (id INTEGER PRIMARY KEY)",
			)
			return err
		},
	}
	if err := Migrate(
		database,
		"ordinary_schema_version",
		[]Migration{migration},
	); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	requireSQLiteObjectCount(t, database, "table", "ordinary_custom_migration_marker", 1)
	requireDatabaseForeignKeys(t, database, 1)
}

func TestOrdinaryCustomMigrationEstablishesForeignKeyEnforcement(
	t *testing.T,
) {
	t.Parallel()

	database := openForeignKeyRebuildTestDatabase(t, 1)
	defer database.Close()
	if _, err := database.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable fixture foreign keys: %v", err)
	}
	requireDatabaseForeignKeys(t, database, 0)
	migration := Migration{
		Version:     1,
		Description: "ordinary custom migration establishes foreign keys",
		Apply: func(transaction MigrationTransaction, _ []Migration) error {
			requireMigrationForeignKeys(t, transaction, 1)
			_, err := transaction.Exec(
				"CREATE TABLE ordinary_fk_on_marker (id INTEGER PRIMARY KEY)",
			)
			return err
		},
	}
	if err := Migrate(
		database,
		"ordinary_fk_on_schema_version",
		[]Migration{migration},
	); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	requireSQLiteObjectCount(t, database, "table", "ordinary_fk_on_marker", 1)
	requireDatabaseForeignKeys(t, database, 1)
}

func TestForeignKeyTableRebuildBoundaryRequiresCustomApply(t *testing.T) {
	t.Parallel()

	database := openForeignKeyRebuildTestDatabase(t, 1)
	defer database.Close()

	migration := Migration{
		Version:       1,
		Description:   "invalid statement rebuild",
		Statements:    []string{"CREATE TABLE must_not_exist (id INTEGER PRIMARY KEY)"},
		ApplyBoundary: ForeignKeyTableRebuildBoundary,
	}
	err := Migrate(
		database,
		"invalid_boundary_schema_version",
		[]Migration{migration},
	)
	if err == nil || !strings.Contains(err.Error(), "without an Apply callback") {
		t.Fatalf("Migrate() error = %v; want closed-boundary rejection", err)
	}
	requireSQLiteObjectCount(t, database, "table", "must_not_exist", 0)
	requireDatabaseForeignKeys(t, database, 1)
}

func TestMigrationForeignKeyVerifierRequiresTableRebuildBoundary(t *testing.T) {
	t.Parallel()

	database := openForeignKeyRebuildTestDatabase(t, 1)
	defer database.Close()

	migration := Migration{
		Version:     1,
		Description: "invalid ordinary verifier",
		Apply: func(transaction MigrationTransaction, _ []Migration) error {
			_, err := transaction.Exec(
				"CREATE TABLE ordinary_verifier_must_not_exist (id INTEGER PRIMARY KEY)",
			)
			return err
		},
		ForeignKeyVerifier: func(MigrationTransaction) error {
			return nil
		},
	}
	err := Migrate(
		database,
		"invalid_verifier_schema_version",
		[]Migration{migration},
	)
	if err == nil || !strings.Contains(
		err.Error(),
		"foreign-key verifier outside the table-rebuild boundary",
	) {
		t.Fatalf("Migrate() error = %v; want verifier-boundary rejection", err)
	}
	requireSQLiteObjectCount(
		t,
		database,
		"table",
		"ordinary_verifier_must_not_exist",
		0,
	)
	requireDatabaseForeignKeys(t, database, 1)
}

func foreignKeyRebuildTestMigration(
	version int,
	apply func(MigrationTransaction, []Migration) error,
) Migration {
	return Migration{
		Version:       version,
		Description:   "test foreign-key table rebuild",
		Apply:         apply,
		ApplyBoundary: ForeignKeyTableRebuildBoundary,
	}
}

func openForeignKeyRebuildTestDatabase(t *testing.T, maximumConnections int) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "migration.db")
	dsn := fmt.Sprintf(
		"file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)",
		filepath.ToSlash(path),
	)
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open SQLite migration fixture: %v", err)
	}
	database.SetMaxOpenConns(maximumConnections)
	database.SetMaxIdleConns(maximumConnections)
	if err := database.Ping(); err != nil {
		database.Close()
		t.Fatalf("ping SQLite migration fixture: %v", err)
	}
	return database
}

func seedForeignKeyRebuildParentAndChild(t *testing.T, database *sql.DB) {
	t.Helper()
	statements := []string{
		"CREATE TABLE fk_rebuild_parent (id TEXT PRIMARY KEY)",
		`CREATE TABLE fk_rebuild_child (
			id TEXT PRIMARY KEY,
			parent_id TEXT NOT NULL REFERENCES fk_rebuild_parent(id)
		)`,
		"INSERT INTO fk_rebuild_parent (id) VALUES ('parent')",
		"INSERT INTO fk_rebuild_child (id, parent_id) VALUES ('child', 'parent')",
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("seed FK rebuild fixture with %q: %v", statement, err)
		}
	}
}

func rebuildForeignKeyTestParent(
	transaction MigrationTransaction,
	preserveParent bool,
) error {
	statements := []string{
		`CREATE TABLE fk_rebuild_parent_v2 (
			id TEXT PRIMARY KEY,
			payload TEXT NOT NULL
		)`,
	}
	if preserveParent {
		statements = append(
			statements,
			`INSERT INTO fk_rebuild_parent_v2 (id, payload)
			SELECT id, 'preserved' FROM fk_rebuild_parent`,
		)
	}
	statements = append(
		statements,
		"DROP TABLE fk_rebuild_parent",
		"ALTER TABLE fk_rebuild_parent_v2 RENAME TO fk_rebuild_parent",
	)
	for _, statement := range statements {
		if _, err := transaction.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func requireMigrationForeignKeys(
	t *testing.T,
	transaction MigrationTransaction,
	expected int,
) {
	t.Helper()
	var actual int
	if err := transaction.QueryRow("PRAGMA foreign_keys").Scan(&actual); err != nil {
		t.Fatalf("read migration foreign_keys: %v", err)
	}
	if actual != expected {
		t.Fatalf("migration foreign_keys = %d; want %d", actual, expected)
	}
}

func requireDatabaseForeignKeys(t *testing.T, database *sql.DB, expected int) {
	t.Helper()
	var actual int
	if err := database.QueryRow("PRAGMA foreign_keys").Scan(&actual); err != nil {
		t.Fatalf("read database foreign_keys: %v", err)
	}
	if actual != expected {
		t.Fatalf("database foreign_keys = %d; want %d", actual, expected)
	}
}

func requireForeignKeyCheckClean(t *testing.T, database *sql.DB) {
	t.Helper()
	rows, err := database.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatalf("run foreign_key_check: %v", err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign_key_check returned a violation")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("drain foreign_key_check: %v", err)
	}
}

func requireMigrationVersionCount(
	t *testing.T,
	database *sql.DB,
	table string,
	version int,
	expected int,
) {
	t.Helper()
	var actual int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE version = ?", table)
	if err := database.QueryRow(query, version).Scan(&actual); err != nil {
		t.Fatalf("count migration version: %v", err)
	}
	if actual != expected {
		t.Fatalf("migration version count = %d; want %d", actual, expected)
	}
}

func requireSQLiteObjectCount(
	t *testing.T,
	database *sql.DB,
	kind string,
	name string,
	expected int,
) {
	t.Helper()
	var actual int
	err := database.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?",
		kind,
		name,
	).Scan(&actual)
	if err != nil {
		t.Fatalf("count SQLite object %s %s: %v", kind, name, err)
	}
	if actual != expected {
		t.Fatalf("SQLite object %s %s count = %d; want %d", kind, name, actual, expected)
	}
}

func requireSQLiteTableRowCount(
	t *testing.T,
	database *sql.DB,
	table string,
	expected int,
) {
	t.Helper()
	var actual int
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table),
	).Scan(&actual); err != nil {
		t.Fatalf("count %s rows: %v", table, err)
	}
	if actual != expected {
		t.Fatalf("%s row count = %d; want %d", table, actual, expected)
	}
}

func waitForTwoMigrationConnections(t *testing.T, database *sql.DB) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if database.Stats().InUse >= 2 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("second migration did not reserve a connection: %+v", database.Stats())
}
