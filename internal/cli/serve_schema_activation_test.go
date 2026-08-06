package cli

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/db"
)

func TestOpenServeProjectLedgerRejectsOldSchemaWithoutMigration(
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
		"haft project migrate",
		fixture.binding.ProjectRoot,
		"no startup migration was attempted",
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
