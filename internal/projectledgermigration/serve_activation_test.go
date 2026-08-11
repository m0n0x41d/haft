package projectledgermigration

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/db"
)

var serveMigrationTestTime = time.Date(
	2026,
	time.August,
	10,
	12,
	30,
	0,
	123,
	time.UTC,
)

func TestEnsureCurrentForServeMigrates57AfterVerifiedSnapshot(
	t *testing.T,
) {
	fixture, databasePath := newSchema57ServeFixture(t, true)
	request := serveFixtureRequest(t, fixture)

	result, err := EnsureCurrentForServe(
		context.Background(),
		request,
		serveMigrationTestTime,
	)
	if err != nil {
		t.Fatalf("EnsureCurrentForServe: %v", err)
	}
	if result.Outcome != ServeActivationMigrated ||
		!result.Ready() ||
		result.BeforeSchema != 57 ||
		result.AfterSchema != 59 ||
		result.BackupPath == "" ||
		result.BackupDigest == "" {
		t.Fatalf("activation result = %#v", result)
	}
	if !strings.HasSuffix(result.BackupPath, ".bak") {
		t.Fatalf("backup path = %s, want .bak", result.BackupPath)
	}
	_, schema58Snapshot := serveMigrationSnapshotPaths(
		databasePath,
		57,
		58,
		serveMigrationTestTime,
	)
	_, schema59Snapshot := serveMigrationSnapshotPaths(
		databasePath,
		58,
		59,
		serveMigrationTestTime,
	)
	if result.BackupPath != schema59Snapshot {
		t.Fatalf("reported backup path = %s, want latest boundary %s", result.BackupPath, schema59Snapshot)
	}
	assertSecureServeSnapshot(t, schema58Snapshot)
	assertSecureServeSnapshot(t, schema59Snapshot)
	if got := digestFileForTest(t, result.BackupPath); got != result.BackupDigest {
		t.Fatalf("backup digest = %s, want %s", got, result.BackupDigest)
	}
	if frontier := readSchemaFrontierForTest(t, databasePath); frontier != 59 {
		t.Fatalf("live schema frontier = %d, want 59", frontier)
	}
	if count := affectedPathCountForServeTest(
		t,
		databasePath,
		"/legacy/absolute.go",
	); count != 0 {
		t.Fatalf("live legacy affected path count = %d, want 0", count)
	}
	if frontier := readSchemaFrontierForTest(
		t,
		schema58Snapshot,
	); frontier != 57 {
		t.Fatalf("schema-58 boundary backup frontier = %d, want 57", frontier)
	}
	if count := affectedPathCountForServeTest(
		t,
		schema58Snapshot,
		"/legacy/absolute.go",
	); count != 1 {
		t.Fatalf("schema-58 boundary backup legacy path count = %d, want 1", count)
	}
	if frontier := readSchemaFrontierForTest(t, schema59Snapshot); frontier != 58 {
		t.Fatalf("schema-59 boundary backup frontier = %d, want 58", frontier)
	}
	if count := affectedPathCountForServeTest(
		t,
		schema59Snapshot,
		"/legacy/absolute.go",
	); count != 0 {
		t.Fatalf("schema-59 boundary backup legacy path count = %d, want 0", count)
	}
}

func TestEnsureCurrentForServeSnapshotsCleanSchema57(t *testing.T) {
	fixture, _ := newSchema57ServeFixture(t, false)
	result, err := EnsureCurrentForServe(
		context.Background(),
		serveFixtureRequest(t, fixture),
		serveMigrationTestTime,
	)
	if err != nil {
		t.Fatalf("EnsureCurrentForServe: %v", err)
	}
	if result.Outcome != ServeActivationMigrated || result.BackupPath == "" {
		t.Fatalf("clean activation result = %#v", result)
	}
	assertSecureServeSnapshot(t, result.BackupPath)
}

func TestEnsureCurrentForServeRequiresTimestampOnlyForSnapshotChain(
	t *testing.T,
) {
	fixture, databasePath := newSchema57ServeFixture(t, false)
	result, err := EnsureCurrentForServe(
		context.Background(),
		serveFixtureRequest(t, fixture),
		time.Time{},
	)
	if err == nil || result.Blocker != ServeBlockerSnapshot {
		t.Fatalf("zero-time snapshot activation = %#v, %v", result, err)
	}
	if frontier := readSchemaFrontierForTest(t, databasePath); frontier != 57 {
		t.Fatalf("zero-time schema frontier = %d, want 57", frontier)
	}
	assertNoServeMigrationArtifacts(t, filepath.Dir(databasePath))
}

func TestApplyUsesSharedLeaseAndRecoverySnapshotBoundary(t *testing.T) {
	fixture, _ := newSchema57ServeFixture(t, false)
	result, err := Apply(
		context.Background(),
		serveFixtureRequest(t, fixture),
	)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.Outcome != OutcomeApplied ||
		result.BeforeSchema != 57 ||
		result.AfterSchema != 59 ||
		result.BackupPath == "" ||
		result.BackupDigest == "" {
		t.Fatalf("manual migration result = %#v", result)
	}
	assertSecureServeSnapshot(t, result.BackupPath)
}

func TestEnsureCurrentForServeRetainsSnapshotWhenMigrationRollsBack(
	t *testing.T,
) {
	fixture, databasePath := newSchema57ServeFixture(t, true)
	execServeMigrationFixtureSQL(
		t,
		databasePath,
		`CREATE TRIGGER reject_serve_schema_58
		 BEFORE INSERT ON schema_version
		 WHEN NEW.version = 58 BEGIN
			SELECT RAISE(ABORT, 'injected schema-58 failure');
		 END`,
	)
	result, err := EnsureCurrentForServe(
		context.Background(),
		serveFixtureRequest(t, fixture),
		serveMigrationTestTime,
	)
	if err == nil || !strings.Contains(err.Error(), "injected schema-58 failure") {
		t.Fatalf("migration failure = %#v, %v", result, err)
	}
	if result.Blocker != ServeBlockerMigration ||
		result.BackupPath == "" ||
		result.BackupDigest == "" {
		t.Fatalf("failed activation result = %#v", result)
	}
	assertSecureServeSnapshot(t, result.BackupPath)
	if frontier := readSchemaFrontierForTest(t, databasePath); frontier != 57 {
		t.Fatalf("rolled-back live schema frontier = %d, want 57", frontier)
	}
	if count := affectedPathCountForServeTest(
		t,
		databasePath,
		"/legacy/absolute.go",
	); count != 1 {
		t.Fatalf("rolled-back legacy path count = %d, want 1", count)
	}
}

func TestEnsureCurrentForServeLeavesPartialCollisionUntouched(
	t *testing.T,
) {
	fixture, databasePath := newSchema57ServeFixture(t, false)
	partialPath, finalPath := serveMigrationSnapshotPaths(
		databasePath,
		57,
		58,
		serveMigrationTestTime,
	)
	if err := os.WriteFile(partialPath, []byte("operator-owned partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := EnsureCurrentForServe(
		context.Background(),
		serveFixtureRequest(t, fixture),
		serveMigrationTestTime,
	)
	if err == nil || result.Blocker != ServeBlockerSnapshot {
		t.Fatalf("partial collision activation = %#v, %v", result, err)
	}
	content, readErr := os.ReadFile(partialPath)
	if readErr != nil || string(content) != "operator-owned partial" {
		t.Fatalf("partial collision content = %q, %v", content, readErr)
	}
	if _, statErr := os.Stat(finalPath); !os.IsNotExist(statErr) {
		t.Fatalf("unexpected published backup after collision: %v", statErr)
	}
	if frontier := readSchemaFrontierForTest(t, databasePath); frontier != 57 {
		t.Fatalf("schema after partial collision = %d, want 57", frontier)
	}
}

func TestEnsureCurrentForServeRejectsUnhealthyLedgerBeforeSnapshot(
	t *testing.T,
) {
	fixture, databasePath := newSchema57ServeFixture(t, false)
	execServeMigrationFixtureSQL(
		t,
		databasePath,
		`INSERT INTO affected_files (artifact_id, file_path)
		 VALUES ('missing-artifact', 'orphan.go')`,
	)
	result, err := EnsureCurrentForServe(
		context.Background(),
		serveFixtureRequest(t, fixture),
		serveMigrationTestTime,
	)
	if err == nil ||
		result.Blocker != ServeBlockerSnapshot ||
		!strings.Contains(err.Error(), "foreign-key violation") {
		t.Fatalf("unhealthy activation = %#v, %v", result, err)
	}
	if frontier := readSchemaFrontierForTest(t, databasePath); frontier != 57 {
		t.Fatalf("unhealthy schema frontier = %d, want 57", frontier)
	}
	backups, globErr := filepath.Glob(
		filepath.Join(filepath.Dir(databasePath), "*.pre-serve-migration-*.bak"),
	)
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(backups) != 0 {
		t.Fatalf("unhealthy ledger produced backups: %v", backups)
	}
}

func TestEnsureCurrentForServeCurrentSchemaCreatesNoMigrationArtifacts(
	t *testing.T,
) {
	fixture := newCurrentProjectFixture(t)
	databasePath, err := fixture.config.DBPath()
	if err != nil {
		t.Fatal(err)
	}
	result, err := EnsureCurrentForServe(
		context.Background(),
		serveFixtureRequest(t, fixture),
		time.Time{},
	)
	if err != nil {
		t.Fatalf("EnsureCurrentForServe: %v", err)
	}
	if result.Outcome != ServeActivationCurrent ||
		result.BackupPath != "" ||
		result.BackupDigest != "" {
		t.Fatalf("current activation result = %#v", result)
	}
	assertNoServeMigrationArtifacts(t, filepath.Dir(databasePath))
}

func TestEnsureCurrentForServeManualChainDoesNotMutate(t *testing.T) {
	fixture := newCurrentProjectFixture(t)
	databasePath, err := fixture.config.DBPath()
	if err != nil {
		t.Fatal(err)
	}
	execServeMigrationFixtureSQL(
		t,
		databasePath,
		"DELETE FROM schema_version WHERE version >= 57",
	)
	before := digestFileForTest(t, databasePath)
	result, err := EnsureCurrentForServe(
		context.Background(),
		serveFixtureRequest(t, fixture),
		serveMigrationTestTime,
	)
	if err == nil {
		t.Fatal("manual schema chain was activated automatically")
	}
	if result.Outcome != ServeActivationManualRequired ||
		result.Blocker != ServeBlockerManualChain ||
		result.FirstBlockedVersion != 57 {
		t.Fatalf("manual activation result = %#v, error = %v", result, err)
	}
	if after := digestFileForTest(t, databasePath); after != before {
		t.Fatal("manual schema chain changed the database")
	}
	assertNoServeMigrationArtifacts(t, filepath.Dir(databasePath))
}

func TestEnsureCurrentForServeFutureSchemaNeverMigrates(t *testing.T) {
	fixture := newCurrentProjectFixture(t)
	databasePath, err := fixture.config.DBPath()
	if err != nil {
		t.Fatal(err)
	}
	execServeMigrationFixtureSQL(
		t,
		databasePath,
		"INSERT INTO schema_version(version) VALUES (60)",
	)
	before := digestFileForTest(t, databasePath)
	result, err := EnsureCurrentForServe(
		context.Background(),
		serveFixtureRequest(t, fixture),
		serveMigrationTestTime,
	)
	if err == nil {
		t.Fatal("future schema was accepted")
	}
	if result.Blocker != ServeBlockerFutureSchema ||
		result.BeforeSchema != 60 ||
		result.AfterSchema != 59 {
		t.Fatalf("future activation result = %#v, error = %v", result, err)
	}
	if after := digestFileForTest(t, databasePath); after != before {
		t.Fatal("future schema changed the database")
	}
	assertNoServeMigrationArtifacts(t, filepath.Dir(databasePath))
}

func TestEnsureCurrentForServeRejectsGapAndMissingBinding(t *testing.T) {
	t.Run("gap", func(t *testing.T) {
		fixture := newCurrentProjectFixture(t)
		databasePath, err := fixture.config.DBPath()
		if err != nil {
			t.Fatal(err)
		}
		execServeMigrationFixtureSQL(
			t,
			databasePath,
			"DELETE FROM schema_version WHERE version = 40",
		)
		before := digestFileForTest(t, databasePath)
		result, err := EnsureCurrentForServe(
			context.Background(),
			serveFixtureRequest(t, fixture),
			serveMigrationTestTime,
		)
		if err == nil || result.Blocker != ServeBlockerInvalidSchema {
			t.Fatalf("gapped activation = %#v, %v", result, err)
		}
		if after := digestFileForTest(t, databasePath); after != before {
			t.Fatal("gapped schema changed the database")
		}
	})
	t.Run("missing binding", func(t *testing.T) {
		fixture := newCurrentUnboundRecoveryFixture(t)
		request, err := NewRequest(fixture.root, fixture.config.ID)
		if err != nil {
			t.Fatal(err)
		}
		result, err := EnsureCurrentForServe(
			context.Background(),
			request,
			serveMigrationTestTime,
		)
		if err == nil || result.Blocker != ServeBlockerMissingBinding {
			t.Fatalf("unbound activation = %#v, %v", result, err)
		}
	})
}

func TestEnsureCurrentForServeSerializesConcurrentCallers(t *testing.T) {
	fixture, databasePath := newSchema57ServeFixture(t, true)
	request := serveFixtureRequest(t, fixture)
	const callers = 8
	results := make([]ServeActivationResult, callers)
	errorsByCaller := make([]error, callers)
	start := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(callers)
	for index := range callers {
		go func() {
			defer wait.Done()
			<-start
			results[index], errorsByCaller[index] = EnsureCurrentForServe(
				context.Background(),
				request,
				serveMigrationTestTime,
			)
		}()
	}
	close(start)
	wait.Wait()
	migrated := 0
	for index, result := range results {
		if errorsByCaller[index] != nil || !result.Ready() {
			t.Fatalf(
				"caller %d activation = %#v, %v",
				index,
				result,
				errorsByCaller[index],
			)
		}
		if result.Outcome == ServeActivationMigrated {
			migrated++
		}
	}
	if migrated != 1 {
		t.Fatalf("migrating callers = %d, want 1", migrated)
	}
	backups, err := filepath.Glob(
		filepath.Join(filepath.Dir(databasePath), "*.pre-serve-migration-*.bak"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 2 {
		t.Fatalf("serve migration backups = %v, want one per snapshot boundary", backups)
	}
	if frontier := readSchemaFrontierForTest(t, databasePath); frontier != 59 {
		t.Fatalf("concurrent live schema frontier = %d, want 59", frontier)
	}
}

func TestMigrationCoordinatorHonorsRetryDeadline(t *testing.T) {
	fixture := newCurrentProjectFixture(t)
	request := serveFixtureRequest(t, fixture)
	observation, err := Observe(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	coordinator, err := newMigrationCoordinator(observation)
	if err != nil {
		t.Fatal(err)
	}
	leader, _, err := coordinator.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer leader.release()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, _, err = coordinator.acquire(ctx)
	if !errors.Is(err, ErrMigrationLeaseTimeout) {
		t.Fatalf("follower lease error = %v", err)
	}
}

func newSchema57ServeFixture(
	t *testing.T,
	withInvalidAffectedPath bool,
) (currentProjectFixture, string) {
	t.Helper()
	fixture := newCurrentProjectFixture(t)
	databasePath, err := fixture.config.DBPath()
	if err != nil {
		t.Fatal(err)
	}
	execServeMigrationFixtureSQL(
		t,
		databasePath,
		"DELETE FROM schema_version WHERE version >= 58",
	)
	if withInvalidAffectedPath {
		execServeMigrationFixtureSQL(
			t,
			databasePath,
			`INSERT INTO artifacts (
				id, kind, title, content, created_at, updated_at
			) VALUES (
				'serve-migration-fixture', 'decision', 'fixture', 'fixture',
				'2026-08-10T00:00:00Z', '2026-08-10T00:00:00Z'
			);
			INSERT INTO affected_files (artifact_id, file_path)
			VALUES ('serve-migration-fixture', '/legacy/absolute.go')`,
		)
	}
	return fixture, databasePath
}

func serveFixtureRequest(
	t *testing.T,
	fixture currentProjectFixture,
) Request {
	t.Helper()
	request, err := NewRequest(fixture.root, fixture.config.ID)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func execServeMigrationFixtureSQL(
	t *testing.T,
	databasePath string,
	statement string,
) {
	t.Helper()
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	_, execErr := database.Exec(statement)
	closeErr := database.Close()
	if execErr != nil || closeErr != nil {
		t.Fatalf("fixture SQL failed: exec=%v close=%v", execErr, closeErr)
	}
}

func affectedPathCountForServeTest(
	t *testing.T,
	databasePath string,
	path string,
) int {
	t.Helper()
	database, err := sql.Open("sqlite", "file:"+databasePath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var count int
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM affected_files WHERE file_path = ?",
		path,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func assertSecureServeSnapshot(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("snapshot mode = %v, want regular 0600", info.Mode())
	}
}

func assertNoServeMigrationArtifacts(t *testing.T, directory string) {
	t.Helper()
	patterns := []string{
		"*.pre-serve-migration-*",
		migrationLeaseFilename,
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(directory, pattern))
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 0 {
			t.Fatalf("unexpected serve migration artifacts: %v", matches)
		}
	}
}

func TestServeMigrationPolicyStillTargetsCurrentSchema59(t *testing.T) {
	current, err := db.CurrentSchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if current != 59 {
		t.Fatalf(
			"test fixture requires schema 59, compiled schema is %d; declare and test the new serve activation policy",
			current,
		)
	}
}
