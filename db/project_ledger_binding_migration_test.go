package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunMigrationsThroughProjectLedgerBindingRollsBackSchemaAndBindingTogether(
	t *testing.T,
) {
	t.Parallel()

	database := openPreBindingMigrationDatabase(t)
	defer database.Close()
	injected := errors.New("injected project ledger binding failure")
	projectRoot := filepath.Join(t.TempDir(), "project")

	err := RunMigrationsThroughProjectLedgerBinding(
		database,
		func(transaction MigrationTransaction) error {
			if err := insertProjectLedgerBindingForMigrationTest(
				transaction,
				projectRoot,
			); err != nil {
				return err
			}
			return injected
		},
	)
	if !errors.Is(err, injected) {
		t.Fatalf("binding migration failure = %v, want %v", err, injected)
	}
	assertMigrationVersionCount(
		t,
		database,
		ProjectLedgerBindingSchemaVersion,
		0,
	)
	assertSchemaObjectCountForMigrationTest(
		t,
		database,
		"table",
		"project_ledger_binding",
		0,
	)

	if err := RunMigrationsThroughProjectLedgerBinding(
		database,
		func(transaction MigrationTransaction) error {
			return insertProjectLedgerBindingForMigrationTest(
				transaction,
				projectRoot,
			)
		},
	); err != nil {
		t.Fatalf("retry project ledger binding migration: %v", err)
	}
	assertMigrationVersionCount(
		t,
		database,
		ProjectLedgerBindingSchemaVersion,
		1,
	)
	assertSchemaObjectCountForMigrationTest(
		t,
		database,
		"table",
		"project_ledger_binding",
		1,
	)
	var bindingCount int
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM project_ledger_binding",
	).Scan(&bindingCount); err != nil {
		t.Fatalf("count committed project ledger binding: %v", err)
	}
	if bindingCount != 1 {
		t.Fatalf("committed project ledger bindings = %d, want 1", bindingCount)
	}
	assertMigrationVersionCount(
		t,
		database,
		ProjectLedgerBindingSchemaVersion+1,
		0,
	)
}

func TestRunMigrationsThroughProjectLedgerBindingDoesNotRepairRecordedSchema(
	t *testing.T,
) {
	t.Parallel()

	database := openPreBindingMigrationDatabase(t)
	defer database.Close()
	throughBinding := migrationsBeforeVersion(
		kernelMigrations,
		ProjectLedgerBindingSchemaVersion+1,
		0,
		nil,
	)
	if err := Migrate(
		database,
		"schema_version",
		throughBinding,
	); err != nil {
		t.Fatalf("install unbound schema-37 fixture: %v", err)
	}

	called := false
	err := RunMigrationsThroughProjectLedgerBinding(
		database,
		func(MigrationTransaction) error {
			called = true
			return errors.New("binding callback must not run")
		},
	)
	if err != nil {
		t.Fatalf("repeat project ledger binding migration: %v", err)
	}
	if called {
		t.Fatal("recorded schema-37 migration invoked the binding callback")
	}
	var bindingCount int
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM project_ledger_binding",
	).Scan(&bindingCount); err != nil {
		t.Fatalf("count unbound schema-37 rows: %v", err)
	}
	if bindingCount != 0 {
		t.Fatalf("recorded schema-37 binding rows = %d, want 0", bindingCount)
	}
}

func TestRunMigrationsThroughProjectLedgerBindingRejectsMissingEffect(
	t *testing.T,
) {
	t.Parallel()

	database := openPreBindingMigrationDatabase(t)
	defer database.Close()
	err := RunMigrationsThroughProjectLedgerBinding(
		database,
		func(MigrationTransaction) error {
			return nil
		},
	)
	if err == nil ||
		!strings.Contains(err.Error(), "want exactly 1") {
		t.Fatalf("missing binding effect error = %v", err)
	}
	assertMigrationVersionCount(
		t,
		database,
		ProjectLedgerBindingSchemaVersion,
		0,
	)
	assertSchemaObjectCountForMigrationTest(
		t,
		database,
		"table",
		"project_ledger_binding",
		0,
	)
}

func openPreBindingMigrationDatabase(t *testing.T) *sql.DB {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "pre-binding.db")
	dsn, err := sqliteConnectionDSN(databasePath)
	if err != nil {
		t.Fatalf("build pre-binding SQLite DSN: %v", err)
	}
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open pre-binding database: %v", err)
	}
	if err := database.Ping(); err != nil {
		_ = database.Close()
		t.Fatalf("ping pre-binding database: %v", err)
	}
	if _, err := database.Exec(schema); err != nil {
		_ = database.Close()
		t.Fatalf("install pre-binding base schema: %v", err)
	}
	beforeBinding := migrationsBeforeVersion(
		kernelMigrations,
		ProjectLedgerBindingSchemaVersion,
		0,
		nil,
	)
	if err := Migrate(
		database,
		"schema_version",
		beforeBinding,
	); err != nil {
		_ = database.Close()
		t.Fatalf("migrate through pre-binding schema: %v", err)
	}
	assertMigrationVersionCount(
		t,
		database,
		ProjectLedgerBindingSchemaVersion-1,
		1,
	)
	assertSchemaObjectCountForMigrationTest(
		t,
		database,
		"table",
		"project_ledger_binding",
		0,
	)
	return database
}

func insertProjectLedgerBindingForMigrationTest(
	transaction MigrationTransaction,
	projectRoot string,
) error {
	const (
		projectID = "qnt_a7f3b2c1"
		boundAt   = "2026-07-30T00:00:00Z"
	)
	bindingJSON := `{"schema":"haft.project-ledger-binding/v1","project_id":"` +
		projectID +
		`","project_root":"` +
		projectRoot +
		`","bound_at":"` +
		boundAt +
		`"}`
	digest := sha256.Sum256([]byte(bindingJSON))
	_, err := transaction.Exec(
		`INSERT INTO project_ledger_binding (
			binding_slot, project_id, project_root,
			binding_digest, binding_json, bound_at
		) VALUES (1, ?, ?, ?, ?, ?)`,
		projectID,
		projectRoot,
		"sha256:"+hex.EncodeToString(digest[:]),
		bindingJSON,
		boundAt,
	)
	if err != nil {
		return err
	}
	return nil
}

func assertSchemaObjectCountForMigrationTest(
	t *testing.T,
	database *sql.DB,
	kind string,
	name string,
	expected int,
) {
	t.Helper()
	var count int
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM sqlite_schema WHERE type = ? AND name = ?",
		kind,
		name,
	).Scan(&count); err != nil {
		t.Fatalf("count %s %s: %v", kind, name, err)
	}
	if count != expected {
		t.Fatalf("%s %s count = %d, want %d", kind, name, count, expected)
	}
}
