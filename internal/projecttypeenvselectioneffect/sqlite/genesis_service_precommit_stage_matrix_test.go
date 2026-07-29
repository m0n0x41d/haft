package sqlite

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	sqlitedriver "modernc.org/sqlite"
	sqlitelib "modernc.org/sqlite/lib"
)

type genesisPrecommitFailureStage struct {
	number    int
	name      string
	timing    string
	operation string
	table     string
}

func TestGenesisServicePrecommitFailureMatrixIsAtomic(t *testing.T) {
	fixture := newGenesisE2EFixture(t)
	fixture.database.SetMaxOpenConns(1)
	fixture.database.SetMaxIdleConns(1)
	stages := []genesisPrecommitFailureStage{
		{
			number:    1,
			name:      "generic graph event",
			operation: "INSERT",
			table:     "typed_memory_graph_events",
		},
		{
			number:    2,
			name:      "graph admission",
			operation: "INSERT",
			table:     "typed_memory_event_admission_bases",
		},
		{
			number:    3,
			name:      "graph idempotency",
			operation: "INSERT",
			table:     "typed_memory_idempotency_history",
		},
		{
			number:    4,
			name:      "graph projection",
			operation: "INSERT",
			table:     "typed_memory_projection_jobs",
		},
		{
			number:    5,
			name:      "effect-owned no-prior-head proof",
			operation: "INSERT",
			table:     "project_typeenv_no_prior_head_proofs",
		},
		{
			number:    6,
			name:      "head-selection request",
			operation: "INSERT",
			table:     "project_typeenv_head_selection_requests",
		},
		{
			number:    7,
			name:      "authority source",
			operation: "INSERT",
			table:     "project_typeenv_head_selection_trusted_cli_sources",
		},
		{
			number:    8,
			name:      "authority resolution",
			operation: "INSERT",
			table:     "project_typeenv_head_selection_authority_resolutions",
		},
		{
			number:    9,
			name:      "authority use",
			operation: "INSERT",
			table:     "project_typeenv_head_selection_authority_uses",
		},
		{
			number:    10,
			name:      "CAS Work",
			operation: "INSERT",
			table:     "project_typeenv_head_cas_work_records",
		},
		{
			number:    11,
			name:      "dedicated head projection and immutable state",
			operation: "INSERT",
			table:     "project_typeenv_head_states",
		},
		{
			number:    12,
			name:      "TypeEnv activation",
			operation: "INSERT",
			table:     "typed_memory_type_env_activations",
		},
		{
			number:    13,
			name:      "dedicated head history",
			operation: "INSERT",
			table:     "project_typeenv_head_history",
		},
		{
			number:    14,
			name:      "receipt",
			operation: "INSERT",
			table:     "project_typeenv_head_selection_receipts",
		},
	}
	runGenesisPrecommitFailureStages(t, fixture, stages)
}

func runGenesisPrecommitFailureStages(
	t *testing.T,
	fixture genesisE2EFixture,
	stages []genesisPrecommitFailureStage,
) {
	t.Helper()
	baseline := observeGenesisFaultSnapshot(t, fixture)
	for _, stage := range stages {
		stage := stage
		t.Run(
			fmt.Sprintf("%02d_%s", stage.number, strings.ReplaceAll(stage.name, " ", "_")),
			func(t *testing.T) {
				sentinel := installGenesisPrecommitFailureTrigger(
					t,
					fixture,
					stage,
				)
				result, err := fixture.service.SelectGenesis(
					context.Background(),
					genesisSelectionInput(fixture),
				)
				assertGenesisPrecommitFailure(
					t,
					stage,
					sentinel,
					result,
					err,
				)
				after := observeGenesisFaultSnapshot(t, fixture)
				assertGenesisFaultSnapshotEqual(t, baseline, after)
			},
		)
	}

}

func TestGenesisServicePrecommitClosureBoundaryFailureIsAtomic(t *testing.T) {
	fixture := newGenesisE2EFixture(t)
	stage := genesisPrecommitFailureStage{
		number:    15,
		name:      "selection closure boundary after receipt",
		timing:    "AFTER",
		operation: "INSERT",
		table:     "project_typeenv_head_selection_receipts",
	}
	runGenesisPrecommitFailureStages(
		t,
		fixture,
		[]genesisPrecommitFailureStage{stage},
	)
	assertGenesisUntriggeredSuccess(t, fixture)
}

func TestGenesisServicePrecommitGraphFinalizationFailureMatrixIsAtomic(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	stages := []genesisPrecommitFailureStage{
		{
			number:    16,
			name:      "graph materialization closure",
			operation: "INSERT",
			table:     "typed_memory_commit_materialization_closures",
		},
		{
			number:    17,
			name:      "graph-head finalization",
			operation: "UPDATE",
			table:     "typed_memory_graph_heads",
		},
		{
			number:    18,
			name:      "graph commit",
			operation: "INSERT",
			table:     "typed_memory_graph_commits",
		},
	}
	runGenesisPrecommitFailureStages(t, fixture, stages)
	assertGenesisUntriggeredSuccess(t, fixture)
}

func assertGenesisUntriggeredSuccess(
	t *testing.T,
	fixture genesisE2EFixture,
) {
	t.Helper()
	result, err := fixture.service.SelectGenesis(
		context.Background(),
		genesisSelectionInput(fixture),
	)
	if err != nil {
		t.Fatalf("SelectGenesis(after failure matrix): %v", err)
	}
	if _, ok := result.(projecttypeenvselectioneffect.FreshlyCommitted); !ok {
		t.Fatalf(
			"SelectGenesis(after failure matrix) = %T, want FreshlyCommitted",
			result,
		)
	}
}

func installGenesisPrecommitFailureTrigger(
	t *testing.T,
	fixture genesisE2EFixture,
	stage genesisPrecommitFailureStage,
) string {
	t.Helper()
	triggerName := fmt.Sprintf("genesis_precommit_stage_%02d_abort", stage.number)
	sentinel := fmt.Sprintf("injected Genesis precommit stage %02d abort", stage.number)
	statement := fmt.Sprintf(
		`CREATE TRIGGER %s
		%s %s ON %s
		BEGIN
			SELECT RAISE(ABORT, '%s');
		END`,
		triggerName,
		genesisPrecommitTriggerTiming(stage),
		stage.operation,
		stage.table,
		sentinel,
	)
	if _, err := fixture.database.Exec(statement); err != nil {
		t.Fatalf(
			"install precommit failure trigger for stage %02d %s: %v",
			stage.number,
			stage.name,
			err,
		)
	}
	t.Cleanup(func() {
		if _, err := fixture.database.Exec(
			"DROP TRIGGER IF EXISTS " + triggerName,
		); err != nil {
			t.Errorf(
				"drop precommit failure trigger for stage %02d %s: %v",
				stage.number,
				stage.name,
				err,
			)
		}
	})
	return sentinel
}

func genesisPrecommitTriggerTiming(stage genesisPrecommitFailureStage) string {
	if stage.timing != "" {
		return stage.timing
	}
	return "BEFORE"
}

func assertGenesisPrecommitFailure(
	t *testing.T,
	stage genesisPrecommitFailureStage,
	sentinel string,
	result projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionResult,
	err error,
) {
	t.Helper()
	if result != nil {
		t.Fatalf(
			"stage %02d %s returned success %T, want nil result",
			stage.number,
			stage.name,
			result,
		)
	}
	if err == nil {
		t.Fatalf(
			"stage %02d %s returned no error",
			stage.number,
			stage.name,
		)
	}
	if !strings.Contains(err.Error(), sentinel) {
		t.Fatalf(
			"stage %02d %s error = %v, want sentinel %q",
			stage.number,
			stage.name,
			err,
			sentinel,
		)
	}
	var sqliteError *sqlitedriver.Error
	if !errors.As(err, &sqliteError) {
		t.Fatalf(
			"stage %02d %s error type = %T, want *sqlite.Error in chain: %v",
			stage.number,
			stage.name,
			err,
			err,
		)
	}
	if sqliteError.Code() != sqlitelib.SQLITE_CONSTRAINT_TRIGGER {
		t.Fatalf(
			"stage %02d %s SQLite code = %d, want SQLITE_CONSTRAINT_TRIGGER (%d)",
			stage.number,
			stage.name,
			sqliteError.Code(),
			sqlitelib.SQLITE_CONSTRAINT_TRIGGER,
		)
	}
}
