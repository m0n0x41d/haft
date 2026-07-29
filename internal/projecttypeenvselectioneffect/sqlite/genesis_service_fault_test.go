package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

type genesisCommitThenReportFalse struct {
	called bool
	finish sqlitetransaction.FinishResult
}

func (committer *genesisCommitThenReportFalse) commit(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
) bool {
	committer.called = true
	committer.finish = transaction.Commit(ctx)
	return false
}

type genesisRollbackThenReportFalse struct {
	called bool
	finish sqlitetransaction.FinishResult
}

func (committer *genesisRollbackThenReportFalse) commit(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
) bool {
	committer.called = true
	committer.finish = transaction.Rollback(ctx)
	return false
}

func TestGenesisServiceMidEffectAbortRollsBackEveryWrite(t *testing.T) {
	fixture := newGenesisE2EFixture(t)
	before := observeGenesisFaultSnapshot(t, fixture)
	installGenesisReceiptAbortTrigger(t, fixture.database)
	result, err := fixture.service.SelectGenesis(
		context.Background(),
		genesisSelectionInput(fixture),
	)
	if err == nil {
		t.Fatalf("SelectGenesis(mid-effect abort) = %T, want error", result)
	}
	if !strings.Contains(err.Error(), "injected Genesis mid-effect abort") {
		t.Fatalf("SelectGenesis(mid-effect abort) error = %v", err)
	}
	after := observeGenesisFaultSnapshot(t, fixture)
	assertGenesisFaultSnapshotEqual(t, before, after)
}

func TestGenesisServiceRecoversPhysicalCommitHiddenByCommitter(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	committer := &genesisCommitThenReportFalse{}
	fixture.service.committer = committer
	result, err := fixture.service.SelectGenesis(
		context.Background(),
		genesisSelectionInput(fixture),
	)
	if err != nil {
		t.Fatalf("SelectGenesis(commit then report false): %v", err)
	}
	if !committer.called || !committer.finish.Succeeded() {
		t.Fatalf(
			"hidden commit outcome = called:%t succeeded:%t err:%v",
			committer.called,
			committer.finish.Succeeded(),
			committer.finish.Err(),
		)
	}
	fresh, ok := result.(projecttypeenvselectioneffect.FreshlyCommitted)
	if !ok {
		t.Fatalf(
			"SelectGenesis(commit then report false) = %T, want FreshlyCommitted",
			result,
		)
	}
	if _, ok := fresh.Delivery().(projecttypeenvselectioneffect.CommitRecoveredByExactClosureReread); !ok {
		t.Fatalf(
			"recovered delivery posture = %T",
			fresh.Delivery(),
		)
	}
	assertGenesisE2ECommittedFootprint(t, fixture, fresh.Closure())
}

func TestGenesisServiceReturnsUnknownAfterHiddenRollback(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	before := observeGenesisFaultSnapshot(t, fixture)
	committer := &genesisRollbackThenReportFalse{}
	fixture.service.committer = committer
	result, err := fixture.service.SelectGenesis(
		context.Background(),
		genesisSelectionInput(fixture),
	)
	if err != nil {
		t.Fatalf("SelectGenesis(rollback then report false): %v", err)
	}
	if !committer.called || !committer.finish.Succeeded() {
		t.Fatalf(
			"hidden rollback outcome = called:%t succeeded:%t err:%v",
			committer.called,
			committer.finish.Succeeded(),
			committer.finish.Err(),
		)
	}
	unknown, ok := result.(projecttypeenvselectioneffect.CommitOutcomeUnknown)
	if !ok {
		t.Fatalf(
			"SelectGenesis(rollback then report false) = %T, want CommitOutcomeUnknown",
			result,
		)
	}
	if unknown.RetryKey() != fixture.request.IdempotencyKey() ||
		unknown.RequestDigest() != fixture.request.Ref().Digest() ||
		unknown.ContentDigest() != fixture.content.Digest() {
		t.Fatal("CommitOutcomeUnknown lost exact retry coordinates")
	}
	after := observeGenesisFaultSnapshot(t, fixture)
	assertGenesisFaultSnapshotEqual(t, before, after)
}

func TestGenesisServiceSameKeyDifferentContentConflictsWithoutWrites(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	freshResult, err := fixture.service.SelectGenesis(
		context.Background(),
		genesisSelectionInput(fixture),
	)
	if err != nil {
		t.Fatalf("SelectGenesis(seed exact owner): %v", err)
	}
	if _, ok := freshResult.(projecttypeenvselectioneffect.FreshlyCommitted); !ok {
		t.Fatalf("seed SelectGenesis = %T, want FreshlyCommitted", freshResult)
	}
	conflictingContent := genesisConflictingContent(t, fixture)
	if conflictingContent.Digest() == fixture.content.Digest() {
		t.Fatal("conflict fixture did not change the authorization-content digest")
	}
	before := observeGenesisFaultSnapshot(t, fixture)
	result, err := fixture.service.SelectGenesis(
		context.Background(),
		GenesisSelectionInput{
			Request:   fixture.request,
			Content:   conflictingContent,
			Authority: NewDedicatedCLIInvocation(),
		},
	)
	if err != nil {
		t.Fatalf("SelectGenesis(conflicting replay): %v", err)
	}
	conflict, ok := result.(projecttypeenvselectioneffect.ReplayConflict)
	if !ok {
		t.Fatalf("conflicting SelectGenesis = %T, want ReplayConflict", result)
	}
	if conflict.Key() != fixture.request.IdempotencyKey() ||
		conflict.ExistingRequestDigest() != fixture.request.Ref().Digest() ||
		conflict.PresentedRequestDigest() != fixture.request.Ref().Digest() ||
		conflict.ExistingContentDigest() != fixture.content.Digest() ||
		conflict.PresentedContentDigest() != conflictingContent.Digest() {
		t.Fatal("ReplayConflict lost exact existing/presented coordinates")
	}
	after := observeGenesisFaultSnapshot(t, fixture)
	assertGenesisFaultSnapshotEqual(t, before, after)
}

func TestGenesisServiceExactReplayBypassesPoisonedCurrentRuntime(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	result, err := fixture.service.SelectGenesis(
		context.Background(),
		genesisSelectionInput(fixture),
	)
	if err != nil {
		t.Fatalf("SelectGenesis(seed exact replay): %v", err)
	}
	fresh, ok := result.(projecttypeenvselectioneffect.FreshlyCommitted)
	if !ok {
		t.Fatalf("seed SelectGenesis = %T, want FreshlyCommitted", result)
	}
	poisonedRuntime := projecttypeenvruntime.InstalledRuntimeRegistryInput{}
	observation := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: fixture.target.runtime,
			Installed:    poisonedRuntime,
		},
	)
	if _, matched := observation.(projecttypeenvruntime.Matched); matched {
		t.Fatal("poisoned runtime fixture still matches the committed target")
	}
	poisonGenesisCurrentAuthorityConfig(t, fixture)
	fixture.service.installedRuntime = poisonedRuntime
	before := observeGenesisFaultSnapshot(t, fixture)
	replayedResult, err := fixture.service.SelectGenesis(
		context.Background(),
		genesisSelectionInput(fixture),
	)
	if err != nil {
		t.Fatalf("SelectGenesis(exact replay with poisoned runtime): %v", err)
	}
	replayed, ok := replayedResult.(projecttypeenvselectioneffect.ReplayedExisting)
	if !ok {
		t.Fatalf(
			"poisoned-current-state replay = %T, want ReplayedExisting",
			replayedResult,
		)
	}
	if replayed.Closure().Ref() != fresh.Closure().Ref() ||
		!bytes.Equal(
			replayed.Closure().CanonicalBytes(),
			fresh.Closure().CanonicalBytes(),
		) {
		t.Fatal("poisoned-current-state replay returned a different closure")
	}
	after := observeGenesisFaultSnapshot(t, fixture)
	assertGenesisFaultSnapshotEqual(t, before, after)
}

func poisonGenesisCurrentAuthorityConfig(
	t *testing.T,
	fixture genesisE2EFixture,
) {
	t.Helper()
	configDirectory := filepath.Join(
		fixture.service.projectRoot.String(),
		".haft",
	)
	if err := os.MkdirAll(configDirectory, 0o700); err != nil {
		t.Fatalf("create strict-config fixture directory: %v", err)
	}
	configPath := filepath.Join(configDirectory, "config.yaml")
	strictConfig := strings.Join(
		[]string{
			"schema_version: 1",
			"authority:",
			"  decision_binding_mode: strict_cli_speech_act",
			"  project_typeenv_head_selection_mode: strict_cli_speech_act",
			"",
		},
		"\n",
	)
	if err := os.WriteFile(configPath, []byte(strictConfig), 0o600); err != nil {
		t.Fatalf("write strict current-authority config fixture: %v", err)
	}
}

func TestGenesisServiceRejectsMissingOrCorruptProofOnExactReplay(
	t *testing.T,
) {
	cases := []struct {
		name    string
		corrupt func(*testing.T, genesisE2EFixture)
	}{
		{
			name:    "missing-proof-row",
			corrupt: deleteGenesisProofRow,
		},
		{
			name:    "corrupt-proof-canonical",
			corrupt: corruptGenesisProofCanonical,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newGenesisE2EFixture(t)
			result, err := fixture.service.SelectGenesis(
				context.Background(),
				genesisSelectionInput(fixture),
			)
			if err != nil {
				t.Fatalf("SelectGenesis(seed exact replay): %v", err)
			}
			if _, ok := result.(projecttypeenvselectioneffect.FreshlyCommitted); !ok {
				t.Fatalf("seed SelectGenesis = %T, want FreshlyCommitted", result)
			}
			testCase.corrupt(t, fixture)
			before := observeGenesisFaultSnapshot(t, fixture)
			replayed, replayErr := fixture.service.SelectGenesis(
				context.Background(),
				genesisSelectionInput(fixture),
			)
			if replayErr == nil {
				t.Fatalf("SelectGenesis(corrupt replay) = %T, want error", replayed)
			}
			if !strings.Contains(
				replayErr.Error(),
				"corrupt TypeEnv head-selection replay",
			) {
				t.Fatalf("SelectGenesis(corrupt replay) error = %v", replayErr)
			}
			after := observeGenesisFaultSnapshot(t, fixture)
			assertGenesisFaultSnapshotEqual(t, before, after)
		})
	}
}

func TestGenesisServiceConcurrentSameKeyCommitsOnceAndReplaysExactly(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	input := genesisSelectionInput(fixture)
	start := make(chan struct{})
	results := make(chan genesisConcurrentResult, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	for range 2 {
		go func() {
			defer wait.Done()
			<-start
			result, err := fixture.service.SelectGenesis(
				context.Background(),
				input,
			)
			results <- genesisConcurrentResult{
				result: result,
				err:    err,
			}
		}()
	}
	close(start)
	wait.Wait()
	close(results)

	var fresh projecttypeenvselectioneffect.FreshlyCommitted
	var replayed projecttypeenvselectioneffect.ReplayedExisting
	freshCount := 0
	replayedCount := 0
	storageFailureCount := 0
	for outcome := range results {
		if outcome.err != nil {
			t.Fatalf("concurrent SelectGenesis: %v", outcome.err)
		}
		switch value := outcome.result.(type) {
		case projecttypeenvselectioneffect.FreshlyCommitted:
			fresh = value
			freshCount++
		case projecttypeenvselectioneffect.ReplayedExisting:
			replayed = value
			replayedCount++
		case projecttypeenvselectioneffect.NotSelected:
			if value.Reason() !=
				projecttypeenvselectioneffect.NotSelectedStorageFailure() {
				t.Fatalf(
					"concurrent SelectGenesis not selected: %s",
					value.Reason().String(),
				)
			}
			storageFailureCount++
		default:
			t.Fatalf("concurrent SelectGenesis result = %T", outcome.result)
		}
	}
	if freshCount != 1 || replayedCount+storageFailureCount != 1 {
		t.Fatalf(
			"concurrent results = fresh:%d replayed:%d storage_failure:%d, want one fresh and one replay-or-contention",
			freshCount,
			replayedCount,
			storageFailureCount,
		)
	}
	if replayedCount == 1 {
		assertGenesisReplayMatchesCommitted(
			t,
			fresh,
			replayed,
			"concurrent exact replay",
		)
	}
	afterContention, err := fixture.service.SelectGenesis(
		context.Background(),
		input,
	)
	if err != nil {
		t.Fatalf("SelectGenesis(after contention): %v", err)
	}
	afterReplay, ok :=
		afterContention.(projecttypeenvselectioneffect.ReplayedExisting)
	if !ok {
		t.Fatalf(
			"SelectGenesis(after contention) = %T, want ReplayedExisting",
			afterContention,
		)
	}
	assertGenesisReplayMatchesCommitted(
		t,
		fresh,
		afterReplay,
		"post-contention exact replay",
	)
	assertGenesisE2ECommittedFootprint(t, fixture, fresh.Closure())
}

func assertGenesisReplayMatchesCommitted(
	t *testing.T,
	fresh projecttypeenvselectioneffect.FreshlyCommitted,
	replayed projecttypeenvselectioneffect.ReplayedExisting,
	label string,
) {
	t.Helper()
	if fresh.Closure().Ref() != replayed.Closure().Ref() ||
		!bytes.Equal(
			fresh.Closure().CanonicalBytes(),
			replayed.Closure().CanonicalBytes(),
		) {
		t.Fatalf("%s returned a different closure", label)
	}
}

type genesisConcurrentResult struct {
	result projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionResult
	err    error
}

func genesisSelectionInput(
	fixture genesisE2EFixture,
) GenesisSelectionInput {
	return GenesisSelectionInput{
		Request:   fixture.request,
		Content:   fixture.content,
		Authority: NewDedicatedCLIInvocation(),
	}
}

func genesisConflictingContent(
	t *testing.T,
	fixture genesisE2EFixture,
) projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent {
	t.Helper()
	description, err := authority.NewClaimIDDescriptionRef(
		"claim:project-typeenv-head-selection:genesis-conflict",
	)
	if err != nil {
		t.Fatalf("NewClaimIDDescriptionRef(conflict): %v", err)
	}
	content, err :=
		projecttypeenvselectionauthority.SealProjectTypeEnvHeadSelectionAuthorizationContent(
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContentInput{
				DescriptionRef:   description,
				Request:          fixture.request,
				Stage:            fixture.stage,
				JudgementContext: fixture.content.JudgementContext(),
				ValidityWindow:   fixture.content.ValidityWindow(),
			},
		)
	if err != nil {
		t.Fatalf("Seal conflicting authorization content: %v", err)
	}
	return content
}

func installGenesisReceiptAbortTrigger(
	t *testing.T,
	database *sql.DB,
) {
	t.Helper()
	_, err := database.Exec(
		`CREATE TRIGGER genesis_test_abort_before_receipt
		BEFORE INSERT ON project_typeenv_head_selection_receipts
		BEGIN
			SELECT RAISE(ABORT, 'injected Genesis mid-effect abort');
		END`,
	)
	if err != nil {
		t.Fatalf("install mid-effect abort trigger: %v", err)
	}
	t.Cleanup(func() {
		_, _ = database.Exec(
			"DROP TRIGGER IF EXISTS genesis_test_abort_before_receipt",
		)
	})
}

func deleteGenesisProofRow(
	t *testing.T,
	fixture genesisE2EFixture,
) {
	t.Helper()
	mutateGenesisProofHistory(
		t,
		fixture,
		func(
			ctx context.Context,
			transaction *sql.Tx,
		) (sql.Result, error) {
			return transaction.ExecContext(
				ctx,
				`DELETE FROM project_typeenv_no_prior_head_proofs
				WHERE project_id = ?`,
				fixture.project.String(),
			)
		},
		"deleted proof rows",
	)
}

func corruptGenesisProofCanonical(
	t *testing.T,
	fixture genesisE2EFixture,
) {
	t.Helper()
	mutateGenesisProofHistory(
		t,
		fixture,
		func(
			ctx context.Context,
			transaction *sql.Tx,
		) (sql.Result, error) {
			return transaction.ExecContext(
				ctx,
				`UPDATE project_typeenv_no_prior_head_proofs
				SET canonical_bytes = ?
				WHERE project_id = ?`,
				[]byte("corrupt-effect-owned-proof"),
				fixture.project.String(),
			)
		},
		"corrupted proof rows",
	)
}

func requireOneGenesisFaultMutation(
	t *testing.T,
	result sql.Result,
	name string,
) {
	t.Helper()
	count, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("%s RowsAffected(): %v", name, err)
	}
	if count != 1 {
		t.Fatalf("%s = %d, want 1", name, count)
	}
}

type genesisTriggerDDL struct {
	name string
	sql  string
}

func mutateGenesisProofHistory(
	t *testing.T,
	fixture genesisE2EFixture,
	mutate func(context.Context, *sql.Tx) (sql.Result, error),
	mutationName string,
) {
	t.Helper()
	ctx := context.Background()
	connection, err := fixture.database.Conn(ctx)
	if err != nil {
		t.Fatalf("acquire proof-corruption connection: %v", err)
	}
	defer connection.Close()
	before := loadGenesisProofTriggerDDL(t, ctx, connection)
	if len(before) == 0 {
		t.Fatal("proof history has no immutable trigger to isolate")
	}
	if _, err := connection.ExecContext(
		ctx,
		"PRAGMA foreign_keys = OFF",
	); err != nil {
		t.Fatalf("disable foreign keys for proof-corruption fixture: %v", err)
	}
	defer func() {
		if _, err := connection.ExecContext(
			ctx,
			"PRAGMA foreign_keys = ON",
		); err != nil {
			t.Fatalf("restore foreign keys after proof-corruption fixture: %v", err)
		}
	}()
	transaction, err := connection.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin isolated proof-corruption fixture: %v", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = transaction.Rollback()
		}
	}()
	for _, trigger := range before {
		statement := "DROP TRIGGER " + quoteGenesisSQLiteIdentifier(trigger.name)
		if _, err := transaction.ExecContext(ctx, statement); err != nil {
			t.Fatalf("drop proof trigger %q: %v", trigger.name, err)
		}
	}
	result, err := mutate(ctx, transaction)
	if err != nil {
		t.Fatalf("%s: %v", mutationName, err)
	}
	requireOneGenesisFaultMutation(t, result, mutationName)
	for _, trigger := range before {
		if _, err := transaction.ExecContext(ctx, trigger.sql); err != nil {
			t.Fatalf("restore proof trigger %q: %v", trigger.name, err)
		}
	}
	if err := transaction.Commit(); err != nil {
		t.Fatalf("commit isolated proof-corruption fixture: %v", err)
	}
	committed = true
	after := loadGenesisProofTriggerDDL(t, ctx, connection)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf(
			"proof trigger schema changed by corruption fixture:\n"+
				"before=%#v\nafter=%#v",
			before,
			after,
		)
	}
}

func loadGenesisProofTriggerDDL(
	t *testing.T,
	ctx context.Context,
	connection *sql.Conn,
) []genesisTriggerDDL {
	t.Helper()
	rows, err := connection.QueryContext(
		ctx,
		`SELECT name, sql
		FROM sqlite_master
		WHERE type = 'trigger'
			AND tbl_name = 'project_typeenv_no_prior_head_proofs'
		ORDER BY name`,
	)
	if err != nil {
		t.Fatalf("load proof trigger DDL: %v", err)
	}
	defer rows.Close()
	triggers := make([]genesisTriggerDDL, 0)
	for rows.Next() {
		var trigger genesisTriggerDDL
		if err := rows.Scan(&trigger.name, &trigger.sql); err != nil {
			t.Fatalf("scan proof trigger DDL: %v", err)
		}
		triggers = append(triggers, trigger)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate proof trigger DDL: %v", err)
	}
	return triggers
}

func quoteGenesisSQLiteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

var genesisFaultTables = []string{
	"project_typeenv_head_selection_config_authority_bases",
	"project_typeenv_head_selection_mode_policies",
	"project_typeenv_no_prior_head_proofs",
	"project_typeenv_head_selection_requests",
	"project_typeenv_head_selection_authorization_contents",
	"project_typeenv_head_selection_trusted_cli_sources",
	"project_typeenv_head_selection_speech_act_records",
	"project_typeenv_head_selection_permissions_v3",
	"project_typeenv_head_selection_authority_resolutions",
	"project_typeenv_head_selection_explicit_policy_acceptance_resolutions",
	"project_typeenv_head_selection_authority_resolution_bases",
	"project_typeenv_head_selection_strict_permission_resolutions",
	"project_typeenv_head_selection_authority_uses",
	"project_typeenv_head_cas_work_records",
	"project_typeenv_heads",
	"project_typeenv_head_states",
	"typed_memory_type_env_activations",
	"project_typeenv_head_history",
	"project_typeenv_head_selection_receipts",
	"project_typeenv_head_selection_closures",
	"typed_memory_graph_events",
	"typed_memory_event_writer_generations",
	"typed_memory_event_admission_bases",
	"typed_memory_idempotency_history",
	"typed_memory_projection_jobs",
	"typed_memory_commit_materialization_closures",
	"typed_memory_graph_commits",
}

type genesisFaultSnapshot struct {
	tableCounts   []int
	graphRevision int64
	activeTypeEnv string
}

func observeGenesisFaultSnapshot(
	t *testing.T,
	fixture genesisE2EFixture,
) genesisFaultSnapshot {
	t.Helper()
	counts := make([]int, len(genesisFaultTables))
	for index, table := range genesisFaultTables {
		query := "SELECT COUNT(*) FROM " + table
		if err := fixture.database.QueryRow(query).Scan(&counts[index]); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
	}
	var graphRevision int64
	var activeTypeEnv string
	err := fixture.database.QueryRow(
		`SELECT graph_revision, active_type_env_ref
		FROM typed_memory_graph_heads
		WHERE project_id = ?`,
		fixture.project.String(),
	).Scan(&graphRevision, &activeTypeEnv)
	if err != nil {
		t.Fatalf("read typed-memory graph head: %v", err)
	}
	return genesisFaultSnapshot{
		tableCounts:   counts,
		graphRevision: graphRevision,
		activeTypeEnv: activeTypeEnv,
	}
}

func assertGenesisFaultSnapshotEqual(
	t *testing.T,
	before genesisFaultSnapshot,
	after genesisFaultSnapshot,
) {
	t.Helper()
	if reflect.DeepEqual(before, after) {
		return
	}
	t.Fatalf(
		"Genesis effect changed after failed/non-writing path:\n"+
			"before=%#v\nafter=%#v",
		before,
		after,
	)
}
