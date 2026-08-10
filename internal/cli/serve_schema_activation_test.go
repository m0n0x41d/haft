package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/projectledgermigration"
)

func TestOpenServeProjectLedgerRemainsCurrentOnlyAfterActivationBoundary(
	t *testing.T,
) {
	fixture := newReadOnlyProjectValidationFixture(t, "qnt_6eadbeef")
	executeReadOnlyProjectValidationFixtureSQL(
		t,
		fixture.database,
		"DELETE FROM schema_version WHERE version >= 45",
	)
	beforeSchema := readOnlyProjectValidationSchema(t, fixture.database)
	beforeFiles := readOnlyProjectValidationFiles(
		t,
		fixture.databaseDirectory,
	)

	ledger, err := openServeProjectLedger(
		context.Background(),
		fixture.binding,
	)
	if ledger != nil {
		_ = ledger.Close()
		t.Fatal("serve project ledger accepted an old kernel schema")
	}
	if err == nil || !strings.Contains(err.Error(), "kernel schema is not current") {
		t.Fatalf("openServeProjectLedger() error = %v", err)
	}
	presented := serveProjectLedgerError(fixture.binding, err).Error()
	for _, want := range []string{
		"became unavailable after startup activation",
		fixture.binding.ProjectRoot,
		"do not run `haft project migrate` unless",
	} {
		if !strings.Contains(presented, want) {
			t.Fatalf("serve repair error missing %q:\n%s", want, presented)
		}
	}

	afterSchema := readOnlyProjectValidationSchema(t, fixture.database)
	afterFiles := readOnlyProjectValidationFiles(
		t,
		fixture.databaseDirectory,
	)
	if !reflect.DeepEqual(afterSchema, beforeSchema) {
		t.Fatalf(
			"serve open changed old SQLite schema\nbefore: %v\nafter:  %v",
			beforeSchema,
			afterSchema,
		)
	}
	if !reflect.DeepEqual(afterFiles, beforeFiles) {
		t.Fatalf(
			"serve open changed old project-store files\nbefore: %v\nafter:  %v",
			beforeFiles,
			afterFiles,
		)
	}
}

func TestServeProjectActivationErrorRoutesExactRecovery(t *testing.T) {
	fixture := newReadOnlyProjectValidationFixture(t, "qnt_6eadbabe")
	tests := []struct {
		name       string
		result     projectledgermigration.ServeActivationResult
		contains   []string
		notContain []string
	}{
		{
			name: "manual chain",
			result: projectledgermigration.ServeActivationResult{
				Outcome:             projectledgermigration.ServeActivationManualRequired,
				Blocker:             projectledgermigration.ServeBlockerManualChain,
				FirstBlockedVersion: 57,
			},
			contains: []string{
				"migration 57",
				"haft project migrate --project-root",
			},
		},
		{
			name: "missing binding",
			result: projectledgermigration.ServeActivationResult{
				Outcome: projectledgermigration.ServeActivationBlocked,
				Blocker: projectledgermigration.ServeBlockerMissingBinding,
			},
			contains: []string{"haft project recover-binding"},
			notContain: []string{
				"haft project migrate --project-root",
			},
		},
		{
			name: "future schema",
			result: projectledgermigration.ServeActivationResult{
				Outcome:      projectledgermigration.ServeActivationBlocked,
				Blocker:      projectledgermigration.ServeBlockerFutureSchema,
				BeforeSchema: 59,
				AfterSchema:  58,
			},
			contains: []string{
				"schema 59 newer",
				"equal or newer Haft binary",
				"do not run `haft project migrate`",
			},
			notContain: []string{
				"haft project migrate --project-root",
			},
		},
		{
			name: "invalid prefix",
			result: projectledgermigration.ServeActivationResult{
				Outcome: projectledgermigration.ServeActivationBlocked,
				Blocker: projectledgermigration.ServeBlockerInvalidSchema,
			},
			contains: []string{
				"inspect or restore",
				"do not run `haft project migrate`",
			},
			notContain: []string{
				"haft project migrate --project-root",
			},
		},
		{
			name: "lease timeout",
			result: projectledgermigration.ServeActivationResult{
				Outcome: projectledgermigration.ServeActivationRetryRequired,
				Blocker: projectledgermigration.ServeBlockerLeaseTimeout,
			},
			contains: []string{
				"timed out waiting",
				"retry or reconnect",
			},
		},
		{
			name: "snapshot failure",
			result: projectledgermigration.ServeActivationResult{
				Outcome: projectledgermigration.ServeActivationBlocked,
				Blocker: projectledgermigration.ServeBlockerSnapshot,
			},
			contains: []string{
				"healthy verified snapshot",
				"`.partial` snapshot",
				"do not run a generic migration",
			},
			notContain: []string{
				"haft project migrate --project-root",
			},
		},
		{
			name: "atomic migration failure",
			result: projectledgermigration.ServeActivationResult{
				Outcome:      projectledgermigration.ServeActivationBlocked,
				Blocker:      projectledgermigration.ServeBlockerMigration,
				BackupPath:   "/tmp/haft.db.pre-serve-v57-to-v58.bak",
				BackupDigest: "sha256-fixture",
			},
			contains: []string{
				"verified pre-migration snapshot retained",
				"/tmp/haft.db.pre-serve-v57-to-v58.bak",
				"sha256-fixture",
				"migration version is atomic",
			},
		},
		{
			name: "unsafe lease carrier",
			result: projectledgermigration.ServeActivationResult{
				Outcome: projectledgermigration.ServeActivationBlocked,
				Blocker: projectledgermigration.ServeBlockerLeaseUnavailable,
			},
			contains: []string{
				"invalid or uncoordinated schema",
				"do not run `haft project migrate`",
			},
			notContain: []string{
				"haft project migrate --project-root",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			presented := serveProjectActivationError(
				fixture.binding,
				test.result,
				fmt.Errorf("fixture cause"),
			).Error()
			for _, fragment := range test.contains {
				if !strings.Contains(presented, fragment) {
					t.Fatalf("activation error missing %q:\n%s", fragment, presented)
				}
			}
			for _, fragment := range test.notContain {
				if strings.Contains(presented, fragment) {
					t.Fatalf("activation error unexpectedly contains %q:\n%s", fragment, presented)
				}
			}
		})
	}
}

func TestInitializeDatabaseRemainsExplicitSchemaActivationEffect(
	t *testing.T,
) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "nested", "haft.db")
	if _, err := os.Stat(databasePath); !os.IsNotExist(err) {
		t.Fatalf("precondition database state = %v, want missing", err)
	}

	if err := initializeDatabase(databasePath); err != nil {
		t.Fatalf("initializeDatabase() error = %v", err)
	}

	database, err := sql.Open(
		"sqlite",
		"file:"+databasePath+"?mode=ro",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	var versionCount int
	var minimumVersion int
	var maximumVersion int
	if err := database.QueryRow(
		`SELECT COUNT(*), MIN(version), MAX(version) FROM schema_version`,
	).Scan(&versionCount, &minimumVersion, &maximumVersion); err != nil {
		t.Fatal(err)
	}
	if minimumVersion != 1 || versionCount != maximumVersion {
		t.Fatalf(
			"explicit initialization versions = count %d, range %d..%d; want one contiguous 1..N catalog",
			versionCount,
			minimumVersion,
			maximumVersion,
		)
	}
	if err := db.RequireCurrentSchemaReadOnly(
		context.Background(),
		database,
	); err != nil {
		t.Fatalf("explicit initialization did not install the current schema: %v", err)
	}
	for _, table := range []string{
		"typed_memory_graph_heads",
		"project_typeenv_heads",
	} {
		var count int
		query := "SELECT COUNT(*) FROM " + table
		if err := database.QueryRow(query).Scan(&count); err != nil {
			t.Fatalf("read %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s rows = %d, want 0", table, count)
		}
	}
}
