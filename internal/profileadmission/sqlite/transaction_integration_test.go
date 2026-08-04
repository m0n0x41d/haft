package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	kerneldb "github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileadmission"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
)

type transactionFixture struct {
	store    *kerneldb.Store
	database *sql.DB
	adapter  adapter
	request  profileadmission.ProfileDeclarationAdmissionRequest
	root     projectprofile.ProjectRootV1
}

func TestAdapterCommitsReplaysAndSurvivesRestart(t *testing.T) {
	fixture := newTransactionFixture(t, "fresh-replay", "fresh-replay.nonce")
	first := fixture.adapter.Admit(context.Background(), fixture.request)
	firstAdmitted := requireAdmitted(t, first, CanonicalAdmissionFresh)
	assertCanonicalMutationCounts(t, fixture.database, 1)
	replay := fixture.adapter.Admit(context.Background(), fixture.request)
	replayed := requireAdmitted(t, replay, CanonicalAdmissionReplayed)
	if firstAdmitted.admission.admissionDigest != replayed.admission.admissionDigest {
		t.Fatal("exact replay returned another admission record")
	}
	assertCanonicalMutationCounts(t, fixture.database, 1)
	databasePath := databasePath(t, fixture.database)
	err := fixture.store.Close()
	if err != nil {
		t.Fatalf("close first store: %v", err)
	}
	restarted, err := kerneldb.NewStore(databasePath)
	if err != nil {
		t.Fatalf("restart NewStore: %v", err)
	}
	t.Cleanup(func() { _ = restarted.Close() })
	adapter, err := newAdapter(restarted.GetRawDB())
	if err != nil {
		t.Fatalf("restart newAdapter: %v", err)
	}
	restartReplay := adapter.Admit(context.Background(), fixture.request)
	requireAdmitted(t, restartReplay, CanonicalAdmissionReplayed)
	assertCanonicalMutationCounts(t, restarted.GetRawDB(), 1)
}

func TestAdapterRejectsStaleProfileChangeHeadInsideTransaction(t *testing.T) {
	fixture := newTransactionFixture(t, "profile-change-cas", "profile-change-cas.nonce")
	first := fixture.adapter.Admit(context.Background(), fixture.request)
	requireAdmitted(t, first, CanonicalAdmissionFresh)
	second := prepareHistoricalRevision(
		t,
		fixture.database,
		fixture.root,
		"profile-change-cas-second",
	)
	stale, err := profileadmission.NewProfileChangeAdmissionRequest(
		second.Candidate(),
		projectprofile.NewLedgerRevision(2),
	)
	if err != nil {
		t.Fatal(err)
	}
	outcome := fixture.adapter.Admit(context.Background(), stale)
	if outcome.kind != AdmissionResultNotAdmitted || len(outcome.denials) != 1 {
		t.Fatalf("stale outcome = %#v", outcome)
	}
	if outcome.denials[0].Code() != "ledger_revision_conflict" {
		t.Fatalf("stale denial = %q", outcome.denials[0].Code())
	}
	assertCanonicalMutationCounts(t, fixture.database, 1)
}

func TestAdapterRequiresPreWorkResolutionWithoutCreatingOne(t *testing.T) {
	fixture := newTransactionFixture(
		t,
		"missing-resolution",
		"missing-resolution.nonce",
	)
	_, err := fixture.database.Exec(
		"DROP TRIGGER profile_declaration_authority_resolutions_v5_no_delete",
	)
	if err != nil {
		t.Fatalf("drop v5 resolution delete guard: %v", err)
	}
	_, err = fixture.database.Exec(
		"DELETE FROM profile_declaration_authority_resolutions_v5",
	)
	if err != nil {
		t.Fatalf("remove prepared v5 resolution: %v", err)
	}
	outcome := fixture.adapter.Admit(context.Background(), fixture.request)
	if outcome.kind != AdmissionResultNotAdmitted || len(outcome.denials) != 1 {
		t.Fatalf("outcome = %#v, want one typed denial", outcome)
	}
	if outcome.denials[0].Code() != "authority_closure_unavailable" {
		t.Fatalf("denial = %q, want authority_closure_unavailable", outcome.denials[0].Code())
	}
	assertCanonicalMutationCounts(t, fixture.database, 0)
	assertTableCounts(
		t,
		fixture.database,
		[]string{"profile_declaration_authority_resolutions_v5"},
		0,
		0,
	)
}

func TestAdapterSerializesConcurrentSameNonceAsFreshPlusReplay(t *testing.T) {
	fixture := newTransactionFixture(t, "concurrent", "concurrent.nonce")
	fixture.database.SetMaxOpenConns(8)
	results := make([]adapterOutcome, 2)
	barrier := make(chan struct{})
	wait := sync.WaitGroup{}
	wait.Add(2)
	launchAdmission(t, &wait, barrier, fixture.adapter, fixture.request, results, 0)
	launchAdmission(t, &wait, barrier, fixture.adapter, fixture.request, results, 1)
	close(barrier)
	wait.Wait()
	postures := []CanonicalAdmissionDelivery{
		admissionPosture(t, results[0]),
		admissionPosture(t, results[1]),
	}
	if !containsPosture(postures, CanonicalAdmissionFresh) {
		t.Fatalf("concurrent postures = %v, missing fresh", postures)
	}
	if !containsPosture(postures, CanonicalAdmissionReplayed) {
		t.Fatalf("concurrent postures = %v, missing replay", postures)
	}
	assertCanonicalMutationCounts(t, fixture.database, 1)
}

func TestDeferredAuthorityCoverageRequiresExactClosedSet(t *testing.T) {
	now := time.Now().UTC().Round(0)
	workWindow, err := projectprofile.NewWorkIntervalV1(
		now.Add(-2*time.Hour),
		now.Add(-time.Hour),
	)
	if err != nil {
		t.Fatalf("build Work interval: %v", err)
	}
	basisWindow, err := projectprofile.NewBasisObservationWindowV1(
		now.Add(-3*time.Hour),
		now.Add(-2*time.Hour),
	)
	if err != nil {
		t.Fatalf("build basis-observation interval: %v", err)
	}
	allowedWork := mustAuthorityWindow(
		t,
		workWindow.From().Add(-time.Minute),
		workWindow.Until().Add(time.Minute),
	)
	allowedBasis := mustAuthorityWindow(
		t,
		basisWindow.From().Add(-time.Minute),
		basisWindow.Until().Add(time.Minute),
	)
	complete := []authorityCoverageRequirement{
		{rule: authorityCoversWorkRule, slot: workIntervalSlot},
		{rule: authorityCoversBasisRule, slot: basisObservationSlot},
	}
	tests := []struct {
		name         string
		requirements []authorityCoverageRequirement
		wantError    bool
	}{
		{name: "complete", requirements: complete, wantError: false},
		{name: "missing", requirements: complete[:1], wantError: true},
		{name: "unknown", requirements: []authorityCoverageRequirement{
			{rule: "haft:rule:unknown/v1", slot: workIntervalSlot},
			complete[1],
		}, wantError: true},
		{name: "duplicate", requirements: []authorityCoverageRequirement{
			complete[0],
			complete[0],
		}, wantError: true},
		{name: "wrong slot", requirements: []authorityCoverageRequirement{
			{rule: authorityCoversWorkRule, slot: basisObservationSlot},
			complete[1],
		}, wantError: true},
	}
	runCoverageCases(
		t,
		tests,
		workWindow,
		basisWindow,
		allowedWork,
		allowedBasis,
		0,
	)
}

func TestAdapterRollsBackEveryStatementFailure(t *testing.T) {
	tests := []struct {
		name  string
		table string
	}{
		{name: "admission", table: "project_profile_admissions_v5"},
		{name: "authority use", table: "profile_declaration_authority_uses_v5"},
		{name: "revision", table: "project_profile_revisions_v5"},
	}
	runStatementFailureCases(t, tests, 0)
}

func runStatementFailureCases(
	t *testing.T,
	tests []struct {
		name  string
		table string
	},
	index int,
) {
	t.Helper()
	if index >= len(tests) {
		return
	}
	test := tests[index]
	t.Run(test.name, func(t *testing.T) {
		fixture := newTransactionFixture(t, "failure-"+test.table, "failure."+test.table)
		trigger := `CREATE TRIGGER injected_statement_failure
			BEFORE INSERT ON ` + test.table + ` BEGIN
				SELECT RAISE(ABORT, 'injected statement failure');
			END`
		_, err := fixture.database.Exec(trigger)
		if err != nil {
			t.Fatalf("create failure trigger: %v", err)
		}
		outcome := fixture.adapter.Admit(context.Background(), fixture.request)
		if outcome.kind != AdmissionResultWriteFailed {
			t.Fatalf("outcome kind = %q, want write_failed", outcome.kind)
		}
		if outcome.failure.commitPosture != AdmissionDefinitelyNotCommitted {
			t.Fatalf("posture = %q, want DefinitelyNotCommitted", outcome.failure.commitPosture)
		}
		assertCanonicalMutationCounts(t, fixture.database, 0)
	})
	runStatementFailureCases(t, tests, index+1)
}

func TestAdapterRecoversAmbiguousCommitByExactDurableReread(t *testing.T) {
	fixture := newTransactionFixture(t, "ambiguous", "ambiguous.nonce")
	finisher := &ambiguousCommitFinisher{}
	fixture.adapter.finisher = finisher
	outcome := fixture.adapter.Admit(context.Background(), fixture.request)
	recovered := requireAdmitted(t, outcome, CanonicalAdmissionRecovered)
	if recovered.admission.admissionRef.String() == "" {
		t.Fatal("recovered outcome omitted rehydrated admission")
	}
	assertCanonicalMutationCounts(t, fixture.database, 1)
	if finisher.commitCalls != 1 {
		t.Fatalf("commit calls = %d, want one", finisher.commitCalls)
	}
}

func TestExactLedgerHeadRejectsAdmissionWithoutAuthorityUse(t *testing.T) {
	fixture := newTransactionFixture(t, "ledger-integrity", "ledger-integrity.nonce")
	outcome := fixture.adapter.Admit(context.Background(), fixture.request)
	requireAdmitted(t, outcome, CanonicalAdmissionFresh)
	_, err := fixture.database.Exec("DROP TRIGGER profile_declaration_authority_uses_v5_no_delete")
	if err != nil {
		t.Fatalf("drop authority-use delete guard: %v", err)
	}
	_, err = fixture.database.Exec("DELETE FROM profile_declaration_authority_uses_v5")
	if err != nil {
		t.Fatalf("delete authority use: %v", err)
	}
	transaction, err := sqlitetransaction.BeginRead(context.Background(), fixture.database)
	if err != nil {
		t.Fatalf("begin integrity read: %v", err)
	}
	_, loadErr := loadExactLedgerHead(context.Background(), transaction, fixture.root)
	finish := transaction.Rollback(context.Background())
	if !finish.Succeeded() {
		t.Fatalf("finish integrity read: %v", finish.Err())
	}
	if loadErr == nil {
		t.Fatal("ledger head accepted an admission without its exact authority use")
	}
}

func TestExactReplaySurvivesExpiredAuthorization(t *testing.T) {
	fixture := newTransactionFixture(
		t,
		"expired-replay",
		"expired-replay.nonce",
	)
	first := fixture.adapter.Admit(context.Background(), fixture.request)
	requireAdmitted(t, first, CanonicalAdmissionFresh)
	fixture.adapter.now = func() time.Time {
		return time.Now().UTC().Round(0).Add(48 * time.Hour)
	}
	replay := fixture.adapter.Admit(context.Background(), fixture.request)
	requireAdmitted(t, replay, CanonicalAdmissionReplayed)
	service := Service{adapter: fixture.adapter}
	resolved := service.ResolveCurrent(context.Background(), fixture.root)
	requireCanonicalAdmission(t, resolved, CanonicalAdmissionResolvedAfterRestart)
	assertCanonicalMutationCounts(t, fixture.database, 1)
}

func TestAdapterRejectsCorruptedHistoricalCanonicalMaterial(t *testing.T) {
	tests := []struct {
		name       string
		statements []string
	}{
		{
			name: "payload",
			statements: []string{
				`UPDATE project_profile_admissions_v5 SET profile_payload_json = '{}' WHERE ledger_revision = 1`,
				`UPDATE project_profile_revisions_v5 SET profile_payload_json = '{}' WHERE ledger_revision = 1`,
			},
		},
		{
			name: "receipt",
			statements: []string{
				`UPDATE project_profile_admissions_v5 SET receipt_json = '{}' WHERE ledger_revision = 1`,
				`UPDATE project_profile_revisions_v5 SET receipt_json = '{}' WHERE ledger_revision = 1`,
			},
		},
		{
			name: "admission",
			statements: []string{
				`UPDATE project_profile_admissions_v5 SET admission_json = '{}' WHERE ledger_revision = 1`,
			},
		},
		{
			name: "provenance",
			statements: []string{
				`UPDATE project_profile_admissions_v5 SET candidate_provenance_json = '{}' WHERE ledger_revision = 1`,
			},
		},
	}
	runHistoricalCorruptionCases(t, tests, 0)
}

func runHistoricalCorruptionCases(
	t *testing.T,
	tests []struct {
		name       string
		statements []string
	},
	index int,
) {
	t.Helper()
	if index >= len(tests) {
		return
	}
	test := tests[index]
	t.Run(test.name, func(t *testing.T) {
		fixture := newTransactionFixture(t, test.name+"-1", test.name+"-1.nonce")
		requireAdmitted(
			t,
			fixture.adapter.Admit(context.Background(), fixture.request),
			CanonicalAdmissionFresh,
		)
		second := prepareHistoricalRevision(
			t,
			fixture.database,
			fixture.root,
			test.name+"-2",
		)
		requireAdmitted(
			t,
			fixture.adapter.Admit(context.Background(), second),
			CanonicalAdmissionFresh,
		)
		pending := prepareHistoricalRevision(
			t,
			fixture.database,
			fixture.root,
			test.name+"-3",
		)
		dropHistoricalUpdateGuards(t, fixture.database)
		executeCorruptionsIgnoringChecks(t, fixture.database, test.statements)
		outcome := fixture.adapter.Admit(context.Background(), pending)
		if outcome.kind != AdmissionResultWriteFailed {
			t.Fatalf("outcome kind = %q, want write_failed", outcome.kind)
		}
		if outcome.failure.commitPosture != AdmissionDefinitelyNotCommitted {
			t.Fatalf("posture = %q, want DefinitelyNotCommitted", outcome.failure.commitPosture)
		}
		assertCanonicalMutationCounts(t, fixture.database, 2)
	})
	runHistoricalCorruptionCases(t, tests, index+1)
}

func prepareHistoricalRevision(
	t *testing.T,
	database *sql.DB,
	root projectprofile.ProjectRootV1,
	suffix string,
) profileadmission.ProfileDeclarationAdmissionRequest {
	t.Helper()
	payload := newIntegrationPayload(t, suffix)
	return prepareV3AdmissionRequest(t, database, root, payload, suffix)
}

func dropHistoricalUpdateGuards(t *testing.T, database *sql.DB) {
	t.Helper()
	_, err := database.Exec("DROP TRIGGER project_profile_admissions_v5_no_update")
	if err != nil {
		t.Fatalf("drop admission update guard: %v", err)
	}
	_, err = database.Exec("DROP TRIGGER project_profile_revisions_v5_no_update")
	if err != nil {
		t.Fatalf("drop revision update guard: %v", err)
	}
}

func executeCorruptionsIgnoringChecks(
	t *testing.T,
	database *sql.DB,
	statements []string,
) {
	t.Helper()
	ctx := context.Background()
	connection, err := database.Conn(ctx)
	if err != nil {
		t.Fatalf("reserve connection for malformed historical fixture: %v", err)
	}
	defer func() {
		if closeErr := connection.Close(); closeErr != nil {
			t.Errorf("close malformed historical fixture connection: %v", closeErr)
		}
	}()
	_, err = connection.ExecContext(ctx, "PRAGMA ignore_check_constraints = ON")
	if err != nil {
		t.Fatalf("disable CHECK constraints for malformed historical fixture: %v", err)
	}
	defer func() {
		_, resetErr := connection.ExecContext(ctx, "PRAGMA ignore_check_constraints = OFF")
		if resetErr != nil {
			t.Errorf("restore CHECK constraints after malformed historical fixture: %v", resetErr)
		}
	}()
	executeCorruptions(t, ctx, connection, statements, 0)
	_, err = connection.ExecContext(ctx, "PRAGMA ignore_check_constraints = OFF")
	if err != nil {
		t.Fatalf("restore CHECK constraints before historical loader check: %v", err)
	}
}

func executeCorruptions(
	t *testing.T,
	ctx context.Context,
	connection *sql.Conn,
	statements []string,
	index int,
) {
	t.Helper()
	if index >= len(statements) {
		return
	}
	_, err := connection.ExecContext(ctx, statements[index])
	if err != nil {
		t.Fatalf("corrupt historical row: %v", err)
	}
	executeCorruptions(t, ctx, connection, statements, index+1)
}

type ambiguousCommitFinisher struct {
	commitCalls int
}

func (finisher *ambiguousCommitFinisher) Commit(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
) transactionFinishEvidence {
	finisher.commitCalls++
	canonical := canonicalTransactionFinisher{}
	evidence := canonical.Commit(ctx, transaction)
	if evidence.statementErr != nil {
		return evidence
	}
	evidence.statementErr = errors.New("injected ambiguous COMMIT response")
	evidence.cleanupErr = errors.New("injected cleanup ambiguity")
	return evidence
}

func (finisher *ambiguousCommitFinisher) Rollback(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
) transactionFinishEvidence {
	canonical := canonicalTransactionFinisher{}
	return canonical.Rollback(ctx, transaction)
}

func runCoverageCases(
	t *testing.T,
	tests []struct {
		name         string
		requirements []authorityCoverageRequirement
		wantError    bool
	},
	work projectprofile.WorkIntervalV1,
	basis projectprofile.BasisObservationWindowV1,
	allowedWork authority.TimeWindow,
	allowedBasis authority.TimeWindow,
	index int,
) {
	t.Helper()
	if index >= len(tests) {
		return
	}
	test := tests[index]
	t.Run(test.name, func(t *testing.T) {
		err := dischargeAuthorityCoverageRequirements(
			test.requirements,
			work.From(),
			work.Until(),
			basis.From(),
			basis.Until(),
			allowedWork,
			allowedBasis,
		)
		if (err != nil) != test.wantError {
			t.Fatalf("error = %v, wantError = %t", err, test.wantError)
		}
	})
	runCoverageCases(
		t,
		tests,
		work,
		basis,
		allowedWork,
		allowedBasis,
		index+1,
	)
}

func launchAdmission(
	t *testing.T,
	wait *sync.WaitGroup,
	barrier <-chan struct{},
	adapter adapter,
	request profileadmission.ProfileDeclarationAdmissionRequest,
	results []adapterOutcome,
	index int,
) {
	t.Helper()
	go func() {
		defer wait.Done()
		<-barrier
		results[index] = adapter.Admit(context.Background(), request)
	}()
}

func newTransactionFixture(
	t *testing.T,
	suffix string,
	nonce string,
) transactionFixture {
	t.Helper()
	payload := newIntegrationPayload(t, suffix)
	return newTransactionFixtureWithPayload(t, suffix, nonce, payload)
}

func newTransactionFixtureWithPayload(
	t *testing.T,
	suffix string,
	nonce string,
	payload projectprofile.ProfileDeclarationPayload,
) transactionFixture {
	t.Helper()
	directory := t.TempDir()
	projectPath := filepath.Join(directory, "project")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("create fixture project root: %v", err)
	}
	physicalRoot, err := filepath.EvalSymlinks(projectPath)
	if err != nil {
		t.Fatalf("resolve fixture project root: %v", err)
	}
	root := mustValue(t, physicalRoot, projectprofile.NewProjectRootV1)
	projectID := "qnt_f17e0001"
	projectConfigDirectory := filepath.Join(projectPath, ".haft")
	if err := os.MkdirAll(projectConfigDirectory, 0o755); err != nil {
		t.Fatalf("create fixture project config directory: %v", err)
	}
	projectConfig := []byte("id: " + projectID + "\nname: profile-admission-fixture\n")
	if err := os.WriteFile(
		filepath.Join(projectConfigDirectory, "project.yaml"),
		projectConfig,
		0o644,
	); err != nil {
		t.Fatalf("write fixture project identity: %v", err)
	}
	home := filepath.Join(directory, "home")
	t.Setenv("HOME", home)
	databaseDirectory := filepath.Join(home, ".haft", "projects", projectID)
	if err := os.MkdirAll(databaseDirectory, 0o755); err != nil {
		t.Fatalf("create fixture project-ledger directory: %v", err)
	}
	databasePath := filepath.Join(databaseDirectory, "haft.db")
	store, err := kerneldbfixture.OpenCurrentStore(databasePath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	database := store.GetRawDB()
	if err := projectledger.BindInitialized(
		context.Background(),
		root.String(),
		time.Now().UTC().Round(0),
	); err != nil {
		_ = store.Close()
		t.Fatalf("bind initialized fixture project ledger: %v", err)
	}
	request := prepareV3AdmissionRequest(t, database, root, payload, suffix+"-"+nonce)
	adapter, err := newAdapter(database)
	if err != nil {
		_ = store.Close()
		t.Fatalf("newAdapter: %v", err)
	}
	return transactionFixture{
		store:    store,
		database: database,
		adapter:  adapter,
		request:  request,
		root:     root,
	}
}

func newIntegrationPayload(t *testing.T, suffix string) projectprofile.ProfileDeclarationPayload {
	t.Helper()
	scopeID := mustValue(t, "software-"+suffix, projectprofile.NewScopeID)
	scope, err := projectprofile.NewSoftwareRealization(scopeID, projectprofile.NoEntityReference{})
	if err != nil {
		t.Fatalf("NewSoftwareRealization: %v", err)
	}
	scopes, err := projectprofile.NewScopeSet([]projectprofile.RealizationScope{scope})
	if err != nil {
		t.Fatalf("NewScopeSet: %v", err)
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(scopes)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload: %v", err)
	}
	return payload
}

func requireAdmitted(
	t *testing.T,
	outcome adapterOutcome,
	want CanonicalAdmissionDelivery,
) adapterOutcome {
	t.Helper()
	if outcome.kind != AdmissionResultAdmitted {
		t.Fatalf("outcome kind = %q, denials = %#v, failure = %#v, want admitted", outcome.kind, outcome.denials, outcome.failure)
	}
	if outcome.DeliveryPosture() != want {
		t.Fatalf("delivery posture = %q, want %q", outcome.DeliveryPosture(), want)
	}
	return outcome
}

func admissionPosture(
	t *testing.T,
	outcome adapterOutcome,
) CanonicalAdmissionDelivery {
	t.Helper()
	if outcome.kind != AdmissionResultAdmitted {
		t.Fatalf("outcome kind = %q, want admitted", outcome.kind)
	}
	return outcome.DeliveryPosture()
}

func containsPosture(
	values []CanonicalAdmissionDelivery,
	want CanonicalAdmissionDelivery,
) bool {
	if len(values) == 0 {
		return false
	}
	if values[0] == want {
		return true
	}
	return containsPosture(values[1:], want)
}

func assertCanonicalMutationCounts(t *testing.T, database *sql.DB, want int) {
	t.Helper()
	tables := []string{
		"project_profile_admissions_v5",
		"profile_declaration_authority_uses_v5",
		"project_profile_revisions_v5",
	}
	assertTableCounts(t, database, tables, want, 0)
}

func assertTableCounts(
	t *testing.T,
	database *sql.DB,
	tables []string,
	want int,
	index int,
) {
	t.Helper()
	if index >= len(tables) {
		return
	}
	query := "SELECT COUNT(*) FROM " + tables[index]
	var got int
	err := database.QueryRow(query).Scan(&got)
	if err != nil {
		t.Fatalf("count %s: %v", tables[index], err)
	}
	if got != want {
		t.Fatalf("%s row count = %d, want %d", tables[index], got, want)
	}
	assertTableCounts(t, database, tables, want, index+1)
}

func databasePath(t *testing.T, database *sql.DB) string {
	t.Helper()
	var sequence int
	var name string
	var path string
	err := database.QueryRow("PRAGMA database_list").Scan(&sequence, &name, &path)
	if err != nil {
		t.Fatalf("PRAGMA database_list: %v", err)
	}
	return path
}
