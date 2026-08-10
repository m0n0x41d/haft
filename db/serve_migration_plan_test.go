package db

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCompileServeMigrationPlanUsesExplicitPolicies(t *testing.T) {
	tests := []struct {
		name             string
		observed         int
		kind             ServeMigrationPlanKind
		pending          []int
		blocked          int
		snapshotRequired bool
	}{
		{
			name:             "schema 57 activates schema 58 with snapshot",
			observed:         57,
			kind:             ServeMigrationAutomatic,
			pending:          []int{58},
			snapshotRequired: true,
		},
		{
			name:     "schema 56 stops at manual schema 57",
			observed: 56,
			kind:     ServeMigrationManualRequired,
			pending:  []int{57, 58},
			blocked:  57,
		},
		{
			name:     "current schema is a no-op",
			observed: 58,
			kind:     ServeMigrationCurrent,
		},
		{
			name:     "future schema never migrates backward",
			observed: 59,
			kind:     ServeMigrationFutureSchema,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := CompileServeMigrationPlan(test.observed)
			if err != nil {
				t.Fatalf("CompileServeMigrationPlan: %v", err)
			}
			if plan.Kind != test.kind ||
				!reflect.DeepEqual(plan.PendingVersions, test.pending) ||
				plan.FirstBlockedVersion != test.blocked ||
				plan.SnapshotRequired != test.snapshotRequired {
				t.Fatalf("plan = %#v", plan)
			}
		})
	}
}

func TestCompileServeMigrationPlanFailsClosedForCatalogGap(t *testing.T) {
	plan := compileServeMigrationPlan(
		57,
		59,
		[]Migration{
			{Version: 58, ServeActivation: ServeActivationAutomatic},
		},
	)
	if plan.Kind != ServeMigrationInvalidCatalog {
		t.Fatalf("plan kind = %s, want %s", plan.Kind, ServeMigrationInvalidCatalog)
	}
}

func TestCompileServeMigrationPlanDefaultsNewMigrationToManual(t *testing.T) {
	plan := compileServeMigrationPlan(
		58,
		59,
		[]Migration{
			{Version: 59},
		},
	)
	if plan.Kind != ServeMigrationManualRequired ||
		plan.FirstBlockedVersion != 59 {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestCompileServeMigrationPlanPreservesManualSnapshotBoundary(
	t *testing.T,
) {
	plan := compileServeMigrationPlan(
		58,
		59,
		[]Migration{
			{
				Version:         59,
				ServeActivation: ServeActivationManualWithSnapshot,
			},
		},
	)
	if plan.Kind != ServeMigrationManualRequired ||
		plan.FirstBlockedVersion != 59 ||
		!plan.SnapshotRequired {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestStatementMigrationRollsBackStatementsAndVersionTogether(
	t *testing.T,
) {
	database, err := sql.Open(
		"sqlite",
		filepath.Join(t.TempDir(), "atomic-statements.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	err = Migrate(
		database,
		"schema_version",
		[]Migration{
			{
				Version:     1,
				Description: "prove statement rollback",
				Statements: []string{
					"CREATE TABLE partial_effect (id INTEGER PRIMARY KEY)",
					"INSERT INTO absent_table(id) VALUES (1)",
				},
			},
		},
	)
	if err == nil {
		t.Fatal("Migrate accepted a failing statement migration")
	}
	var tableCount int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master
		 WHERE type = 'table' AND name = 'partial_effect'`,
	).Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if tableCount != 0 {
		t.Fatalf("partial_effect table count = %d, want 0", tableCount)
	}
	var versionCount int
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = 1",
	).Scan(&versionCount); err != nil {
		t.Fatal(err)
	}
	if versionCount != 0 {
		t.Fatalf("schema version count = %d, want 0", versionCount)
	}
}
