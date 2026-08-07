package codeintel

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"

	_ "modernc.org/sqlite"
)

func TestIndexCoordinationPolicyClosedOutcomes(t *testing.T) {
	tests := []struct {
		name        string
		input       indexCoordinationPolicyInput
		wantOutcome IndexCoordinationOutcome
		wantRebuild bool
	}{
		{
			name: "already fresh",
			input: indexCoordinationPolicyInput{
				Fresh:         true,
				LeaseAcquired: true,
				Epoch:         3,
			},
			wantOutcome: IndexAlreadyFresh,
		},
		{
			name: "fresh after wait",
			input: indexCoordinationPolicyInput{
				Fresh:         true,
				LeaseAcquired: true,
				Contended:     true,
				Epoch:         3,
			},
			wantOutcome: IndexFreshAfterWait,
		},
		{
			name: "startup deferred",
			input: indexCoordinationPolicyInput{
				StartupDeferred: true,
				Epoch:           3,
			},
			wantOutcome: IndexDeferredBusy,
		},
		{
			name: "retained after failure",
			input: indexCoordinationPolicyInput{
				Failure: "bounded wait expired",
				Epoch:   3,
			},
			wantOutcome: IndexRetainedAfterFailure,
		},
		{
			name: "no complete epoch",
			input: indexCoordinationPolicyInput{
				Failure: "bounded wait expired",
			},
			wantOutcome: IndexNoCompleteEpoch,
		},
		{
			name: "leader enters rebuild",
			input: indexCoordinationPolicyInput{
				LeaseAcquired: true,
			},
			wantRebuild: true,
		},
		{
			name: "follower cannot rebuild",
			input: indexCoordinationPolicyInput{
				Epoch: 3,
			},
			wantOutcome: IndexRetainedAfterFailure,
		},
		{
			name: "rebuilt and published",
			input: indexCoordinationPolicyInput{
				LeaseAcquired:    true,
				RefreshComplete:  true,
				RefreshPublished: true,
				Epoch:            4,
			},
			wantOutcome: IndexRebuiltPublished,
		},
		{
			name: "semantic no-op after wait",
			input: indexCoordinationPolicyInput{
				LeaseAcquired:   true,
				Contended:       true,
				RefreshComplete: true,
				Epoch:           4,
			},
			wantOutcome: IndexFreshAfterWait,
		},
	}
	seen := map[IndexCoordinationOutcome]bool{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision := decideIndexCoordination(test.input)
			if decision.Outcome != test.wantOutcome ||
				decision.EnterRebuild != test.wantRebuild {
				t.Fatalf(
					"decision = %#v, want outcome=%q rebuild=%v",
					decision,
					test.wantOutcome,
					test.wantRebuild,
				)
			}
			if decision.EnterRebuild && !test.input.LeaseAcquired {
				t.Fatal("follower entered parse/publication without a lease")
			}
			if decision.Outcome != "" {
				seen[decision.Outcome] = true
			}
		})
	}
	for _, outcome := range []IndexCoordinationOutcome{
		IndexAlreadyFresh,
		IndexRebuiltPublished,
		IndexFreshAfterWait,
		IndexDeferredBusy,
		IndexRetainedAfterFailure,
		IndexNoCompleteEpoch,
	} {
		if !seen[outcome] {
			t.Fatalf("closed policy outcome %q lacks a table case", outcome)
		}
	}
}

func TestIndexObservationFreshnessPolicy(t *testing.T) {
	current := codebase.IndexFreshnessObservation{
		SourceFingerprint:       "source",
		StoredSourceFingerprint: "source",
		ConfigFingerprint:       "config",
		StoredConfigFingerprint: "config",
		CurrentSchemaVersion:    codebase.CodeIndexSchemaVersion,
		StoredSchemaVersion:     codebase.CodeIndexSchemaVersion,
		PublishedEpoch:          7,
	}
	if !indexObservationIsFresh(current) {
		t.Fatal("exact current observation was not fresh")
	}
	mutations := []func(*codebase.IndexFreshnessObservation){
		func(value *codebase.IndexFreshnessObservation) {
			value.StoredSourceFingerprint = "old"
		},
		func(value *codebase.IndexFreshnessObservation) {
			value.StoredConfigFingerprint = "old"
		},
		func(value *codebase.IndexFreshnessObservation) {
			value.StoredSchemaVersion--
		},
		func(value *codebase.IndexFreshnessObservation) {
			value.PublishedEpoch = 0
		},
		func(value *codebase.IndexFreshnessObservation) {
			value.Degraded = true
		},
	}
	for position, mutate := range mutations {
		candidate := current
		mutate(&candidate)
		if indexObservationIsFresh(candidate) {
			t.Fatalf("stale mutation %d was classified fresh: %#v", position, candidate)
		}
	}
}

func TestEnsureIndexSameProcessSingleFlight(t *testing.T) {
	fixture := newIndexCoordinatorFixture(t, "qnt_a1a1a1a1")
	writeCoordinatorSource(t, fixture.root, "package sample\nfunc A() {}\n")

	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var entries atomic.Int64
	var active atomic.Int64
	var peak atomic.Int64
	fixture.coordinator.hooks.beforeRefresh = func(context.Context) error {
		entries.Add(1)
		current := active.Add(1)
		for {
			observed := peak.Load()
			if current <= observed || peak.CompareAndSwap(observed, current) {
				break
			}
		}
		entered <- struct{}{}
		<-release
		return nil
	}
	fixture.coordinator.hooks.afterRefresh = func(
		context.Context,
		codebase.IndexRefreshResult,
		error,
	) error {
		active.Add(-1)
		return nil
	}

	const contenders = 4
	start := make(chan struct{})
	results := make(chan IndexCoordinationResult, contenders)
	errorsFound := make(chan error, contenders)
	var wait sync.WaitGroup
	for range contenders {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			service := NewServiceWithIndexCoordinator(
				fixture.store,
				fixture.coordinator,
			)
			result, err := service.EnsureIndex(
				context.Background(),
				fixture.root,
			)
			results <- result
			errorsFound <- err
		}()
	}
	close(start)
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("leader did not reach the deterministic refresh checkpoint")
	}
	close(release)
	wait.Wait()
	close(results)
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	counts := map[IndexCoordinationOutcome]int{}
	for result := range results {
		counts[result.Outcome]++
		if strings.Contains(strings.ToUpper(result.Reason), "SQLITE_BUSY") {
			t.Fatalf("coordination leaked SQLite contention: %#v", result)
		}
	}
	if entries.Load() != 1 || peak.Load() != 1 {
		t.Fatalf("entries=%d peak=%d, want 1/1", entries.Load(), peak.Load())
	}
	if counts[IndexRebuiltPublished] != 1 ||
		counts[IndexFreshAfterWait] != contenders-1 {
		t.Fatalf("outcomes = %#v", counts)
	}
}

func TestEnsureIndexRejectsCorruptLedgerBeforeParserWork(t *testing.T) {
	fixture := newIndexCoordinatorFixture(t, "qnt_c0c0c0c0")
	writeCoordinatorSource(t, fixture.root, "package sample\nfunc A() {}\n")
	service := NewServiceWithIndexCoordinator(
		fixture.store,
		fixture.coordinator,
	)
	initial, err := service.EnsureIndex(context.Background(), fixture.root)
	if err != nil || initial.Outcome != IndexRebuiltPublished {
		t.Fatalf("initial index = %#v, %v", initial, err)
	}

	if _, err := fixture.database.Exec(`
		CREATE TABLE corruption_probe (payload BLOB NOT NULL);
		INSERT INTO corruption_probe(payload) VALUES (zeroblob(8192));
		PRAGMA wal_checkpoint(TRUNCATE);
	`); err != nil {
		t.Fatal(err)
	}
	var pageSize, rootPage int64
	if err := fixture.database.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
		t.Fatal(err)
	}
	if err := fixture.database.QueryRow(`
		SELECT rootpage FROM sqlite_schema WHERE name = 'corruption_probe'
	`).Scan(&rootPage); err != nil {
		t.Fatal(err)
	}
	writeCoordinatorSource(
		t,
		fixture.root,
		"package sample\nfunc A() {}\nfunc B() {}\n",
	)
	if err := fixture.database.Close(); err != nil {
		t.Fatal(err)
	}
	ledger, err := os.OpenFile(fixture.ledgerPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.WriteAt([]byte{0xff}, (rootPage-1)*pageSize); err != nil {
		_ = ledger.Close()
		t.Fatal(err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := sql.Open(
		"sqlite",
		fixture.ledgerPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(3000)",
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	var parserEntries atomic.Int64
	fixture.coordinator.hooks.beforeRefresh = func(context.Context) error {
		parserEntries.Add(1)
		return nil
	}
	result, err := NewServiceWithIndexCoordinator(
		artifact.NewStore(reopened),
		fixture.coordinator,
	).EnsureIndex(context.Background(), fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != IndexRetainedAfterFailure {
		t.Fatalf("corrupt-ledger outcome = %#v", result)
	}
	if result.PublishedEpoch != initial.PublishedEpoch {
		t.Fatalf(
			"corrupt ledger changed retained epoch: got %d want %d",
			result.PublishedEpoch,
			initial.PublishedEpoch,
		)
	}
	if parserEntries.Load() != 0 {
		t.Fatalf("parser entries = %d, want 0", parserEntries.Load())
	}
	reason := strings.ToLower(result.Reason)
	if !strings.Contains(reason, "integrity") ||
		(!strings.Contains(reason, "malformed") &&
			!strings.Contains(reason, "quick_check")) {
		t.Fatalf("corrupt-ledger reason is not actionable: %#v", result)
	}
}

func TestEnsureIndexStartupDefersAndRequestRetains(t *testing.T) {
	fixture := newIndexCoordinatorFixture(t, "qnt_b2b2b2b2")
	writeCoordinatorSource(t, fixture.root, "package sample\nfunc A() {}\n")
	service := NewServiceWithIndexCoordinator(fixture.store, fixture.coordinator)
	first, err := service.EnsureIndex(context.Background(), fixture.root)
	if err != nil || !first.Rebuilt() {
		t.Fatalf("initial build = %#v, %v", first, err)
	}
	oldEpoch := first.PublishedEpoch
	writeCoordinatorSource(t, fixture.root, "package sample\nfunc A() {}\nfunc B() {}\n")

	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	fixture.coordinator.hooks.beforeRefresh = func(context.Context) error {
		once.Do(func() { close(entered) })
		<-release
		return nil
	}
	leaderDone := make(chan IndexCoordinationResult, 1)
	go func() {
		result, _ := service.EnsureIndex(context.Background(), fixture.root)
		leaderDone <- result
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("leader did not pause after lease acquisition")
	}

	startup, err := service.EnsureIndexForStartup(
		context.Background(),
		fixture.root,
	)
	if err != nil || startup.Outcome != IndexDeferredBusy {
		t.Fatalf("startup follower = %#v, %v", startup, err)
	}
	requestContext, cancel := context.WithTimeout(
		context.Background(),
		150*time.Millisecond,
	)
	defer cancel()
	retained, err := service.EnsureIndex(requestContext, fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	if retained.Outcome != IndexRetainedAfterFailure ||
		retained.PublishedEpoch != oldEpoch {
		t.Fatalf("bounded follower = %#v, want retained epoch %d", retained, oldEpoch)
	}
	close(release)
	if leader := <-leaderDone; leader.Outcome != IndexRebuiltPublished {
		t.Fatalf("leader = %#v", leader)
	}
}

func TestEnsureIndexDistinctProjectsRemainParallel(t *testing.T) {
	left := newIndexCoordinatorFixture(t, "qnt_c3c3c3c3")
	right := newIndexCoordinatorFixture(t, "qnt_d4d4d4d4")
	writeCoordinatorSource(t, left.root, "package left\nfunc Left() {}\n")
	writeCoordinatorSource(t, right.root, "package right\nfunc Right() {}\n")

	entered := make(chan string, 2)
	release := make(chan struct{})
	var active atomic.Int64
	var peak atomic.Int64
	install := func(label string, coordinator *ProjectIndexCoordinator) {
		coordinator.hooks.beforeRefresh = func(context.Context) error {
			current := active.Add(1)
			for {
				observed := peak.Load()
				if current <= observed || peak.CompareAndSwap(observed, current) {
					break
				}
			}
			entered <- label
			<-release
			return nil
		}
		coordinator.hooks.afterRefresh = func(
			context.Context,
			codebase.IndexRefreshResult,
			error,
		) error {
			active.Add(-1)
			return nil
		}
	}
	install("left", left.coordinator)
	install("right", right.coordinator)

	var wait sync.WaitGroup
	for _, fixture := range []*indexCoordinatorFixture{left, right} {
		wait.Add(1)
		go func(fixture *indexCoordinatorFixture) {
			defer wait.Done()
			service := NewServiceWithIndexCoordinator(
				fixture.store,
				fixture.coordinator,
			)
			result, err := service.EnsureIndex(
				context.Background(),
				fixture.root,
			)
			if err != nil || result.Outcome != IndexRebuiltPublished {
				t.Errorf("result = %#v, %v", result, err)
			}
		}(fixture)
	}
	seen := map[string]bool{}
	for range 2 {
		select {
		case label := <-entered:
			seen[label] = true
		case <-time.After(2 * time.Second):
			close(release)
			wait.Wait()
			t.Fatalf("distinct projects serialized: entered=%#v peak=%d", seen, peak.Load())
		}
	}
	close(release)
	wait.Wait()
	if peak.Load() != 2 || !seen["left"] || !seen["right"] {
		t.Fatalf("entered=%#v peak=%d, want both and peak 2", seen, peak.Load())
	}
}

func TestCodeIndexLeaseRejectsSymlinkAndUnsafeMode(t *testing.T) {
	for _, test := range []struct {
		name    string
		prepare func(*testing.T, *indexCoordinatorFixture)
	}{
		{
			name: "symlink",
			prepare: func(t *testing.T, fixture *indexCoordinatorFixture) {
				target := filepath.Join(fixture.runtimeDir, "target")
				if err := os.WriteFile(target, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(
					target,
					filepath.Join(fixture.runtimeDir, "code-index-rebuild.lock"),
				); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "unsafe mode",
			prepare: func(t *testing.T, fixture *indexCoordinatorFixture) {
				if err := os.WriteFile(
					filepath.Join(fixture.runtimeDir, "code-index-rebuild.lock"),
					nil,
					0o644,
				); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newIndexCoordinatorFixture(t, "qnt_e5e5e5e5")
			writeCoordinatorSource(t, fixture.root, "package sample\nfunc A() {}\n")
			test.prepare(t, fixture)
			var parserEntries atomic.Int64
			fixture.coordinator.hooks.beforeRefresh = func(context.Context) error {
				parserEntries.Add(1)
				return nil
			}
			service := NewServiceWithIndexCoordinator(
				fixture.store,
				fixture.coordinator,
			)
			result, err := service.EnsureIndex(
				context.Background(),
				fixture.root,
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != IndexNoCompleteEpoch || parserEntries.Load() != 0 {
				t.Fatalf("result=%#v parser_entries=%d", result, parserEntries.Load())
			}
			if !strings.Contains(strings.ToLower(result.Reason), "symlink") &&
				!strings.Contains(strings.ToLower(result.Reason), "unsafe") {
				t.Fatalf("result reason does not name rejected carrier: %#v", result)
			}
		})
	}
}

type indexCoordinatorFixture struct {
	root        string
	runtimeDir  string
	ledgerPath  string
	database    *sql.DB
	store       *artifact.Store
	coordinator *ProjectIndexCoordinator
}

func newIndexCoordinatorFixture(
	t *testing.T,
	projectID string,
) *indexCoordinatorFixture {
	t.Helper()
	root := canonicalTestDirectory(t)
	runtimeDir := canonicalTestDirectory(t)
	ledgerPath := filepath.Join(runtimeDir, "haft.db")
	database, err := sql.Open(
		"sqlite",
		ledgerPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(3000)",
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if _, err := database.Exec(`CREATE TABLE codebase_modules (
		module_id TEXT PRIMARY KEY,
		path TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		lang TEXT,
		file_count INTEGER DEFAULT 0,
		last_scanned TEXT NOT NULL
	)`); err != nil {
		t.Fatal(err)
	}
	coordinator, err := NewProjectIndexCoordinator(ProjectIndexCoordinates{
		ProjectID:   projectID,
		ProjectRoot: root,
		LedgerPath:  ledgerPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &indexCoordinatorFixture{
		root:        root,
		runtimeDir:  runtimeDir,
		ledgerPath:  ledgerPath,
		database:    database,
		store:       artifact.NewStore(database),
		coordinator: coordinator,
	}
}

func canonicalTestDirectory(t *testing.T) string {
	t.Helper()
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(directory)
}

func writeCoordinatorSource(t *testing.T, root string, source string) {
	t.Helper()
	if err := os.WriteFile(
		filepath.Join(root, "sample.go"),
		[]byte(source),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
}
