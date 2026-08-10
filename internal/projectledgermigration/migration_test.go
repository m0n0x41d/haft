package projectledgermigration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

const bundledPrebindingSchema35Digest = "sha256:3377719306d4ae76e4db7cf1286ae985b92d85f20b5278bd1622b1ad7edf0828"

func TestApplyIsIdempotentAndHasNoHostCarrierEffects(t *testing.T) {
	fixture := newCurrentProjectFixture(t)
	hostPaths := writeHostSentinels(t, fixture.root)
	before := readFiles(t, hostPaths)
	databasePath, err := fixture.config.DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}
	ledgerBefore := discoverHistoricalLedgerSnapshot(t, databasePath)

	request, err := NewRequest(fixture.root, fixture.config.ID)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	first, err := Apply(context.Background(), request)
	if err != nil {
		t.Fatalf("Apply first: %v", err)
	}
	second, err := Apply(context.Background(), request)
	if err != nil {
		t.Fatalf("Apply second: %v", err)
	}

	current, err := db.CurrentSchemaVersion()
	if err != nil {
		t.Fatalf("CurrentSchemaVersion: %v", err)
	}
	if first.Outcome != OutcomeAlreadyCurrent || second.Outcome != OutcomeAlreadyCurrent {
		t.Fatalf("outcomes = %s / %s, want already_current", first.Outcome, second.Outcome)
	}
	if first.BeforeSchema != current || first.AfterSchema != current {
		t.Fatalf("first schema = %d -> %d, want %d", first.BeforeSchema, first.AfterSchema, current)
	}
	if first.DatabasePath != databasePath {
		t.Fatalf("first database path = %s, want %s", first.DatabasePath, databasePath)
	}
	if second.BeforeSchema != current || second.AfterSchema != current {
		t.Fatalf("second schema = %d -> %d, want %d", second.BeforeSchema, second.AfterSchema, current)
	}
	if second.DatabasePath != databasePath {
		t.Fatalf("second database path = %s, want %s", second.DatabasePath, databasePath)
	}
	ledgerAfter := repeatHistoricalLedgerSnapshot(t, databasePath, ledgerBefore)
	assertHistoricalLedgerPreserved(t, ledgerBefore, ledgerAfter)
	assertFilesEqual(t, hostPaths, before)
}

func TestObserveReportsExactSchemaWithoutMutation(t *testing.T) {
	fixture := newCurrentProjectFixture(t)
	request, err := NewRequest(fixture.root, fixture.config.ID)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	databasePath, err := fixture.config.DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}
	before := digestFileForTest(t, databasePath)

	observation, err := Observe(context.Background(), request)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	current, err := db.CurrentSchemaVersion()
	if err != nil {
		t.Fatalf("CurrentSchemaVersion: %v", err)
	}
	if observation.ProjectRoot != fixture.root ||
		observation.ProjectID != fixture.config.ID ||
		observation.DatabasePath != databasePath ||
		observation.ObservedSchema != current ||
		observation.CompiledSchema != current {
		t.Fatalf("observation = %#v", observation)
	}
	after := digestFileForTest(t, databasePath)
	if after != before {
		t.Fatal("read-only schema observation changed the project ledger")
	}
}

func TestObserveAdmitsPreBindingSchemaWithoutMutation(t *testing.T) {
	for _, frontier := range []int{35, 36} {
		t.Run(fmt.Sprintf("schema_%d", frontier), func(t *testing.T) {
			fixture := newUnboundSchemaFrontierFixture(t, frontier)
			request, err := NewRequest(
				fixture.root,
				fixture.config.ID,
			)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			databasePath, err := fixture.config.DBPath()
			if err != nil {
				t.Fatalf("DBPath: %v", err)
			}
			before := digestFileForTest(t, databasePath)

			observation, err := Observe(
				context.Background(),
				request,
			)
			if err != nil {
				t.Fatalf(
					"Observe schema %d: %v",
					frontier,
					err,
				)
			}
			current, err := db.CurrentSchemaVersion()
			if err != nil {
				t.Fatalf("CurrentSchemaVersion: %v", err)
			}
			if observation.ProjectRoot != fixture.root ||
				observation.ProjectID != fixture.config.ID ||
				observation.DatabasePath != databasePath ||
				observation.ObservedSchema != frontier ||
				observation.CompiledSchema != current {
				t.Fatalf("observation = %#v", observation)
			}
			after := digestFileForTest(t, databasePath)
			if after != before {
				t.Fatal(
					"pre-binding schema observation changed the project ledger",
				)
			}
		})
	}
}

func TestObserveRejectsMissingBindingAtBindingAwareSchema(t *testing.T) {
	fixture := newUnboundSchemaFrontierFixture(
		t,
		db.ProjectLedgerBindingSchemaVersion,
	)
	request, err := NewRequest(fixture.root, fixture.config.ID)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	_, err = Observe(context.Background(), request)
	if err == nil ||
		!strings.Contains(err.Error(), "requires a durable project identity binding") {
		t.Fatalf("Observe unbound schema 37 error = %v", err)
	}
}

func TestObserveRejectsGappedPreBindingSchemaPrefixWithoutMutation(
	t *testing.T,
) {
	fixture := newUnboundSchemaFrontierFixture(t, 35)
	request, err := NewRequest(fixture.root, fixture.config.ID)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	databasePath, err := fixture.config.DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open gapped project ledger: %v", err)
	}
	_, deleteErr := database.Exec(
		"DELETE FROM schema_version WHERE version = 17",
	)
	closeErr := database.Close()
	if deleteErr != nil || closeErr != nil {
		t.Fatalf(
			"gap project ledger prefix: delete=%v close=%v",
			deleteErr,
			closeErr,
		)
	}
	before := digestFileForTest(t, databasePath)

	_, err = Observe(context.Background(), request)
	if err == nil || !strings.Contains(
		err.Error(),
		"not an exact prefix through version 35",
	) {
		t.Fatalf("Observe gapped schema prefix error = %v", err)
	}
	after := digestFileForTest(t, databasePath)
	if after != before {
		t.Fatal("gapped schema observation changed the project ledger")
	}
}

func TestApplyRejectsMissingBindingAtBindingAwareSchemaBeforeMigration(
	t *testing.T,
) {
	fixture := newUnboundSchemaFrontierFixture(
		t,
		db.ProjectLedgerBindingSchemaVersion,
	)
	request, err := NewRequest(fixture.root, fixture.config.ID)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	databasePath, err := fixture.config.DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}
	before := digestFileForTest(t, databasePath)

	_, err = Apply(context.Background(), request)
	if err == nil ||
		!strings.Contains(err.Error(), "requires a durable project identity binding") {
		t.Fatalf("Apply unbound schema 37 error = %v", err)
	}
	after := digestFileForTest(t, databasePath)
	if after != before {
		t.Fatal("rejected unbound schema-37 migration changed the ledger")
	}
}

func TestApplyRejectsWrongProjectIdentityBeforeMigration(t *testing.T) {
	fixture := newCurrentProjectFixture(t)
	request, err := NewRequest(fixture.root, "qnt_ffffffff")
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	_, err = Apply(context.Background(), request)
	if err == nil {
		t.Fatal("Apply accepted a different expected project identity")
	}
}

func TestNewRequestRejectsWeakIdentityAndMissingRoot(t *testing.T) {
	if _, err := NewRequest("", "qnt_a7f3b2c1"); err == nil {
		t.Fatal("NewRequest accepted a missing project root")
	}
	if _, err := NewRequest(t.TempDir(), "project"); err == nil {
		t.Fatal("NewRequest accepted a weak project identity")
	}
}

func TestApplyExactRejectsStaleSchemaPlanBeforeMigration(t *testing.T) {
	fixture := newCurrentProjectFixture(t)
	request, err := NewRequest(fixture.root, fixture.config.ID)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	current, err := db.CurrentSchemaVersion()
	if err != nil {
		t.Fatalf("CurrentSchemaVersion: %v", err)
	}
	transition, err := NewExactTransition(current-1, current)
	if err != nil {
		t.Fatalf("NewExactTransition: %v", err)
	}
	if _, err := ApplyExact(
		context.Background(),
		request,
		transition,
	); err == nil || !strings.Contains(err.Error(), "no migration was attempted") {
		t.Fatalf("ApplyExact stale-plan error = %v", err)
	}
	databasePath, err := fixture.config.DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}
	if observed := readSchemaFrontierForTest(t, databasePath); observed != current {
		t.Fatalf("schema after stale exact plan = %d, want %d", observed, current)
	}
	if _, err := NewExactTransition(current, current); err == nil {
		t.Fatal("NewExactTransition accepted a no-op transition")
	}
}

func TestApplyMigratesConfiguredDetachedLedgerCopyWithoutHostEffects(t *testing.T) {
	sourceDB := strings.TrimSpace(os.Getenv("HAFT_I0_COPY_SOURCE_DB"))
	projectRoot := strings.TrimSpace(os.Getenv("HAFT_I0_PROJECT_ROOT"))
	projectID := strings.TrimSpace(os.Getenv("HAFT_I0_PROJECT_ID"))
	if sourceDB == "" || projectRoot == "" || projectID == "" {
		t.Skip("detached ledger copy input is not configured")
	}
	wal := sourceDB + "-wal"
	walInfo, err := os.Stat(wal)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("inspect source WAL: %v", err)
	}
	if err == nil && walInfo.Size() != 0 {
		t.Fatalf("source WAL contains %d bytes; checkpoint before copy rehearsal", walInfo.Size())
	}
	beforeLiveSchema := readSchemaFrontierForTest(t, sourceDB)
	if beforeLiveSchema != 53 {
		t.Fatalf("source schema = %d, want exact rehearsal predecessor 53", beforeLiveSchema)
	}
	sourceDigestBefore := digestFileForTest(t, sourceDB)
	hostBefore := snapshotHostCarriers(t, projectRoot)

	temporaryHome := canonicalTempDir(t)
	t.Setenv("HOME", temporaryHome)
	destinationDB := filepath.Join(
		temporaryHome,
		".haft",
		"projects",
		projectID,
		"haft.db",
	)
	copyFileForTest(t, sourceDB, destinationDB)
	destinationDigestBefore := digestFileForTest(t, destinationDB)
	if destinationDigestBefore != sourceDigestBefore {
		t.Fatalf(
			"detached ledger digest = %s, want exact source digest %s",
			destinationDigestBefore,
			sourceDigestBefore,
		)
	}
	ledgerBefore := discoverHistoricalLedgerSnapshot(t, destinationDB)
	assertRequiredI0Frontiers(t, ledgerBefore)
	request, err := NewRequest(projectRoot, projectID)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	result, err := Apply(context.Background(), request)
	if err != nil {
		t.Fatalf("Apply detached copy: %v", err)
	}
	if result.Outcome != OutcomeApplied ||
		result.BeforeSchema != 53 ||
		result.AfterSchema != 54 ||
		result.ProjectRoot != projectRoot ||
		result.ProjectID != projectID ||
		result.DatabasePath != destinationDB {
		t.Fatalf("detached result = %+v, want applied 53 -> 54", result)
	}
	ledgerAfterFirst := repeatHistoricalLedgerSnapshot(t, destinationDB, ledgerBefore)
	assertHistoricalLedgerPreserved(t, ledgerBefore, ledgerAfterFirst)
	currentBeforeRetry := discoverHistoricalLedgerSnapshot(t, destinationDB)
	second, err := Apply(context.Background(), request)
	if err != nil {
		t.Fatalf("Apply detached copy retry: %v", err)
	}
	if second.Outcome != OutcomeAlreadyCurrent ||
		second.BeforeSchema != 54 ||
		second.AfterSchema != 54 ||
		second.ProjectRoot != projectRoot ||
		second.ProjectID != projectID ||
		second.DatabasePath != destinationDB {
		t.Fatalf("detached retry result = %+v, want already_current 54 -> 54", second)
	}
	currentAfterRetry := repeatHistoricalLedgerSnapshot(
		t,
		destinationDB,
		currentBeforeRetry,
	)
	assertHistoricalLedgerPreserved(t, currentBeforeRetry, currentAfterRetry)
	if liveSchema := readSchemaFrontierForTest(t, sourceDB); liveSchema != beforeLiveSchema {
		t.Fatalf("live source schema changed from %d to %d", beforeLiveSchema, liveSchema)
	}
	sourceDigestAfter := digestFileForTest(t, sourceDB)
	if sourceDigestAfter != sourceDigestBefore {
		t.Fatalf(
			"live source digest changed from %s to %s",
			sourceDigestBefore,
			sourceDigestAfter,
		)
	}
	hostAfter := snapshotHostCarriers(t, projectRoot)
	if !reflect.DeepEqual(hostAfter, hostBefore) {
		t.Fatalf("project host carriers changed\nbefore: %v\nafter:  %v", hostBefore, hostAfter)
	}
	t.Logf(
		"preserved source %s, %d schema-53 tables, %d schema-54 tables, and %d foreign-key witnesses",
		sourceDigestBefore,
		len(ledgerBefore.tables),
		len(currentBeforeRetry.tables),
		len(ledgerBefore.foreignKeyWitnesses),
	)
}

func TestApplyMigratesConfiguredPreBindingLedgerCopy(t *testing.T) {
	sourceDB := strings.TrimSpace(
		os.Getenv("HAFT_PREBINDING_COPY_SOURCE_DB"),
	)
	projectRoot := strings.TrimSpace(
		os.Getenv("HAFT_PREBINDING_PROJECT_ROOT"),
	)
	projectID := strings.TrimSpace(
		os.Getenv("HAFT_PREBINDING_PROJECT_ID"),
	)
	configuredInputs := 0
	for _, value := range []string{sourceDB, projectRoot, projectID} {
		if value != "" {
			configuredInputs++
		}
	}
	if configuredInputs != 0 && configuredInputs != 3 {
		t.Fatal(
			"HAFT_PREBINDING_COPY_SOURCE_DB, HAFT_PREBINDING_PROJECT_ROOT, and HAFT_PREBINDING_PROJECT_ID must be configured together",
		)
	}
	bundledFixture := configuredInputs == 0
	if bundledFixture {
		sourceDB = filepath.Join("testdata", "schema35.db")
		projectRoot = canonicalTempDir(t)
		haftDir := filepath.Join(projectRoot, ".haft")
		if err := os.MkdirAll(haftDir, 0o755); err != nil {
			t.Fatalf("create fixture project .haft: %v", err)
		}
		config, err := project.Create(haftDir, projectRoot)
		if err != nil {
			t.Fatalf("create fixture project identity: %v", err)
		}
		projectID = config.ID
	}
	wal := sourceDB + "-wal"
	walInfo, err := os.Stat(wal)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("inspect source WAL: %v", err)
	}
	if err == nil && walInfo.Size() != 0 {
		t.Fatalf(
			"source WAL contains %d bytes; checkpoint before copy rehearsal",
			walInfo.Size(),
		)
	}
	beforeLiveSchema := readSchemaFrontierForTest(t, sourceDB)
	if beforeLiveSchema != 35 {
		t.Fatalf(
			"source schema = %d, want exact pre-binding predecessor 35",
			beforeLiveSchema,
		)
	}
	sourceDigestBefore := digestFileForTest(t, sourceDB)
	if bundledFixture &&
		sourceDigestBefore != bundledPrebindingSchema35Digest {
		t.Fatalf(
			"bundled schema-35 fixture digest = %s, want %s; regenerate it with %s=1 go test ./db -run '^TestGeneratePrebindingSchema35Fixture$'",
			sourceDigestBefore,
			bundledPrebindingSchema35Digest,
			"HAFT_GENERATE_PREBINDING_SCHEMA35_FIXTURE",
		)
	}
	hostBefore := snapshotHostCarriers(t, projectRoot)

	temporaryHome := canonicalTempDir(t)
	t.Setenv("HOME", temporaryHome)
	destinationDB := filepath.Join(
		temporaryHome,
		".haft",
		"projects",
		projectID,
		"haft.db",
	)
	copyFileForTest(t, sourceDB, destinationDB)
	ledgerBefore := discoverHistoricalLedgerSnapshot(t, destinationDB)
	request, err := NewRequest(projectRoot, projectID)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	observation, err := Observe(context.Background(), request)
	if err != nil {
		t.Fatalf("Observe detached pre-binding copy: %v", err)
	}
	current, err := db.CurrentSchemaVersion()
	if err != nil {
		t.Fatalf("CurrentSchemaVersion: %v", err)
	}
	if observation.ObservedSchema != 35 ||
		observation.CompiledSchema != current {
		t.Fatalf(
			"pre-binding observation = %+v, want 35 -> %d",
			observation,
			current,
		)
	}
	transition, err := NewExactTransition(35, current)
	if err != nil {
		t.Fatalf("NewExactTransition: %v", err)
	}
	result, err := ApplyExact(
		context.Background(),
		request,
		transition,
	)
	if err != nil {
		t.Fatalf("ApplyExact detached pre-binding copy: %v", err)
	}
	if result.Outcome != OutcomeApplied ||
		result.BeforeSchema != 35 ||
		result.AfterSchema != current ||
		result.ProjectRoot != projectRoot ||
		result.ProjectID != projectID ||
		result.DatabasePath != destinationDB {
		t.Fatalf(
			"detached result = %+v, want applied 35 -> %d",
			result,
			current,
		)
	}
	handle, err := projectledger.OpenExisting(
		context.Background(),
		projectRoot,
		projectledger.ReadOnly,
	)
	if err != nil {
		t.Fatalf("OpenExisting migrated pre-binding copy: %v", err)
	}
	if err := handle.Close(); err != nil {
		t.Fatalf("close migrated pre-binding copy: %v", err)
	}
	ledgerAfter := repeatHistoricalLedgerSnapshot(
		t,
		destinationDB,
		ledgerBefore,
	)
	assertHistoricalLedgerPreserved(t, ledgerBefore, ledgerAfter)
	second, err := Apply(context.Background(), request)
	if err != nil {
		t.Fatalf("Apply detached pre-binding copy retry: %v", err)
	}
	if second.Outcome != OutcomeAlreadyCurrent ||
		second.BeforeSchema != current ||
		second.AfterSchema != current {
		t.Fatalf(
			"detached retry result = %+v, want already_current %d -> %d",
			second,
			current,
			current,
		)
	}
	if liveSchema := readSchemaFrontierForTest(t, sourceDB); liveSchema != 35 {
		t.Fatalf("live source schema changed from 35 to %d", liveSchema)
	}
	sourceDigestAfter := digestFileForTest(t, sourceDB)
	if sourceDigestAfter != sourceDigestBefore {
		t.Fatalf(
			"live source digest changed from %s to %s",
			sourceDigestBefore,
			sourceDigestAfter,
		)
	}
	hostAfter := snapshotHostCarriers(t, projectRoot)
	if !reflect.DeepEqual(hostAfter, hostBefore) {
		t.Fatalf(
			"project host carriers changed\nbefore: %v\nafter:  %v",
			hostBefore,
			hostAfter,
		)
	}
	t.Logf(
		"migrated detached schema-35 copy to %d, bound exact identity, and preserved %d historical tables",
		current,
		len(ledgerBefore.tables),
	)
}

func TestApplyRollsBackBindingMigrationWhenCanonicalBinderRejects(
	t *testing.T,
) {
	fixture := newBundledPrebindingCopyFixture(t)
	database, err := sql.Open("sqlite", fixture.databasePath)
	if err != nil {
		t.Fatalf("open pre-binding copy: %v", err)
	}
	_, insertErr := database.Exec(
		`CREATE TABLE future_rooted_extension (
			entry_id TEXT PRIMARY KEY,
			project_root TEXT NOT NULL
		) WITHOUT ROWID;
		INSERT INTO future_rooted_extension (
			entry_id,
			project_root
		) VALUES ('future:foreign', '/tmp/foreign-root')`,
	)
	closeErr := database.Close()
	if insertErr != nil || closeErr != nil {
		t.Fatalf(
			"seed conflicting project root: insert=%v close=%v",
			insertErr,
			closeErr,
		)
	}
	request, err := NewRequest(fixture.root, fixture.projectID)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	current, err := db.CurrentSchemaVersion()
	if err != nil {
		t.Fatalf("CurrentSchemaVersion: %v", err)
	}
	transition, err := NewExactTransition(35, current)
	if err != nil {
		t.Fatalf("NewExactTransition: %v", err)
	}
	_, err = ApplyExact(context.Background(), request, transition)
	if err == nil || !strings.Contains(err.Error(), "future_rooted_extension") {
		t.Fatalf("canonical binder conflict error = %v", err)
	}
	if frontier := readSchemaFrontierForTest(
		t,
		fixture.databasePath,
	); frontier != db.ProjectLedgerBindingSchemaVersion-1 {
		t.Fatalf(
			"frontier after rejected binding = %d, want %d",
			frontier,
			db.ProjectLedgerBindingSchemaVersion-1,
		)
	}
	assertSQLiteObjectCountForTest(
		t,
		fixture.databasePath,
		"table",
		"project_ledger_binding",
		0,
	)

	database, err = sql.Open("sqlite", fixture.databasePath)
	if err != nil {
		t.Fatalf("reopen rejected binding copy: %v", err)
	}
	_, deleteErr := database.Exec("DROP TABLE future_rooted_extension")
	closeErr = database.Close()
	if deleteErr != nil || closeErr != nil {
		t.Fatalf(
			"remove conflicting project root: delete=%v close=%v",
			deleteErr,
			closeErr,
		)
	}
	result, err := Apply(context.Background(), request)
	if err != nil {
		t.Fatalf("retry after canonical binder rejection: %v", err)
	}
	if result.Outcome != OutcomeApplied ||
		result.BeforeSchema != db.ProjectLedgerBindingSchemaVersion-1 ||
		result.AfterSchema != current {
		t.Fatalf(
			"retry result = %+v, want %d -> %d",
			result,
			db.ProjectLedgerBindingSchemaVersion-1,
			current,
		)
	}
}

func TestApplyKeepsBindingWhenLaterMigrationFailsAndRetries(
	t *testing.T,
) {
	fixture := newBundledPrebindingCopyFixture(t)
	database, err := sql.Open("sqlite", fixture.databasePath)
	if err != nil {
		t.Fatalf("open pre-binding copy: %v", err)
	}
	_, triggerErr := database.Exec(
		`CREATE TRIGGER reject_schema_version_38
		 BEFORE INSERT ON schema_version
		 WHEN NEW.version = 38 BEGIN
			SELECT RAISE(ABORT, 'injected post-binding migration failure');
		 END`,
	)
	closeErr := database.Close()
	if triggerErr != nil || closeErr != nil {
		t.Fatalf(
			"install post-binding failure: create=%v close=%v",
			triggerErr,
			closeErr,
		)
	}
	request, err := NewRequest(fixture.root, fixture.projectID)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	current, err := db.CurrentSchemaVersion()
	if err != nil {
		t.Fatalf("CurrentSchemaVersion: %v", err)
	}
	transition, err := NewExactTransition(35, current)
	if err != nil {
		t.Fatalf("NewExactTransition: %v", err)
	}
	_, err = ApplyExact(context.Background(), request, transition)
	if err == nil ||
		!strings.Contains(err.Error(), "injected post-binding migration failure") {
		t.Fatalf("post-binding migration failure = %v", err)
	}
	if frontier := readSchemaFrontierForTest(
		t,
		fixture.databasePath,
	); frontier != db.ProjectLedgerBindingSchemaVersion {
		t.Fatalf(
			"frontier after later failure = %d, want %d",
			frontier,
			db.ProjectLedgerBindingSchemaVersion,
		)
	}
	handle, err := projectledger.OpenExisting(
		context.Background(),
		fixture.root,
		projectledger.ReadWrite,
	)
	if err != nil {
		t.Fatalf("open bound ledger after later migration failure: %v", err)
	}
	_, dropErr := handle.Database().Exec(
		"DROP TRIGGER reject_schema_version_38",
	)
	closeErr = handle.Close()
	if dropErr != nil || closeErr != nil {
		t.Fatalf(
			"remove post-binding failure: drop=%v close=%v",
			dropErr,
			closeErr,
		)
	}
	result, err := Apply(context.Background(), request)
	if err != nil {
		t.Fatalf("retry after later migration failure: %v", err)
	}
	if result.Outcome != OutcomeApplied ||
		result.BeforeSchema != db.ProjectLedgerBindingSchemaVersion ||
		result.AfterSchema != current {
		t.Fatalf(
			"retry result = %+v, want %d -> %d",
			result,
			db.ProjectLedgerBindingSchemaVersion,
			current,
		)
	}
}

type currentProjectFixture struct {
	root   string
	config *project.Config
}

type bundledPrebindingCopyFixture struct {
	root         string
	projectID    string
	databasePath string
}

func newBundledPrebindingCopyFixture(
	t *testing.T,
) bundledPrebindingCopyFixture {
	t.Helper()
	source := filepath.Join("testdata", "schema35.db")
	if digest := digestFileForTest(t, source); digest != bundledPrebindingSchema35Digest {
		t.Fatalf(
			"bundled schema-35 fixture digest = %s, want %s",
			digest,
			bundledPrebindingSchema35Digest,
		)
	}
	root := canonicalTempDir(t)
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatalf("create fixture project .haft: %v", err)
	}
	config, err := project.Create(haftDir, root)
	if err != nil {
		t.Fatalf("create fixture project identity: %v", err)
	}
	home := canonicalTempDir(t)
	t.Setenv("HOME", home)
	databasePath := filepath.Join(
		home,
		".haft",
		"projects",
		config.ID,
		"haft.db",
	)
	copyFileForTest(t, source, databasePath)
	return bundledPrebindingCopyFixture{
		root:         root,
		projectID:    config.ID,
		databasePath: databasePath,
	}
}

func newUnboundSchemaFrontierFixture(
	t *testing.T,
	frontier int,
) currentProjectFixture {
	t.Helper()
	home := canonicalTempDir(t)
	root := canonicalTempDir(t)
	t.Setenv("HOME", home)
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatalf("create .haft: %v", err)
	}
	config, err := project.Create(haftDir, root)
	if err != nil {
		t.Fatalf("project.Create: %v", err)
	}
	databasePath, err := config.DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
		t.Fatalf("create project ledger directory: %v", err)
	}
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open project ledger: %v", err)
	}
	_, createErr := database.Exec(
		`CREATE TABLE schema_version (
			version INTEGER PRIMARY KEY,
			applied_at TEXT DEFAULT CURRENT_TIMESTAMP
		);
		WITH RECURSIVE versions(version) AS (
			SELECT 1
			UNION ALL
			SELECT version + 1 FROM versions WHERE version < ?
		)
		INSERT INTO schema_version(version)
		SELECT version FROM versions`,
		frontier,
	)
	closeErr := database.Close()
	if createErr != nil || closeErr != nil {
		t.Fatalf(
			"create schema-%d project ledger: create=%v close=%v",
			frontier,
			createErr,
			closeErr,
		)
	}
	return currentProjectFixture{
		root:   root,
		config: config,
	}
}

func newCurrentProjectFixture(t *testing.T) currentProjectFixture {
	t.Helper()
	home := canonicalTempDir(t)
	root := canonicalTempDir(t)
	t.Setenv("HOME", home)
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatalf("create .haft: %v", err)
	}
	config, err := project.Create(haftDir, root)
	if err != nil {
		t.Fatalf("project.Create: %v", err)
	}
	databasePath, err := config.DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}
	store, err := db.NewStore(databasePath)
	if err != nil {
		t.Fatalf("db.NewStore: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close current store: %v", err)
	}
	boundAt := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	if err := projectledger.BindInitialized(context.Background(), root, boundAt); err != nil {
		t.Fatalf("BindInitialized: %v", err)
	}
	return currentProjectFixture{
		root:   root,
		config: config,
	}
}

func canonicalTempDir(t *testing.T) string {
	t.Helper()
	physical, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	return filepath.Clean(physical)
}

func writeHostSentinels(t *testing.T, root string) []string {
	t.Helper()
	paths := []string{
		filepath.Join(root, ".agents", "skills", "keep.txt"),
		filepath.Join(root, ".codex", "config.toml"),
		filepath.Join(root, ".mcp.json"),
		filepath.Join(root, "CLAUDE.md"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create host sentinel parent: %v", err)
		}
		if err := os.WriteFile(path, []byte("foreign-user-byte\n"), 0o644); err != nil {
			t.Fatalf("write host sentinel: %v", err)
		}
	}
	return paths
}

func readFiles(t *testing.T, paths []string) map[string]string {
	t.Helper()
	result := make(map[string]string, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		result[path] = string(content)
	}
	return result
}

func assertFilesEqual(t *testing.T, paths []string, expected map[string]string) {
	t.Helper()
	observed := readFiles(t, paths)
	for _, path := range paths {
		if observed[path] != expected[path] {
			t.Fatalf("host carrier changed at %s", path)
		}
	}
}

func copyFileForTest(t *testing.T, source string, destination string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatalf("create detached ledger directory: %v", err)
	}
	input, err := os.Open(source)
	if err != nil {
		t.Fatalf("open source ledger: %v", err)
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("create detached ledger: %v", err)
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		t.Fatalf("copy detached ledger: %v", err)
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		t.Fatalf("sync detached ledger: %v", err)
	}
	if err := output.Close(); err != nil {
		t.Fatalf("close detached ledger: %v", err)
	}
}

func readSchemaFrontierForTest(t *testing.T, path string) int {
	t.Helper()
	database, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatalf("open ledger read-only: %v", err)
	}
	defer database.Close()
	frontier := 0
	if err := database.QueryRow(
		"SELECT COALESCE(MAX(version), 0) FROM schema_version",
	).Scan(&frontier); err != nil {
		t.Fatalf("read schema frontier: %v", err)
	}
	return frontier
}

func assertSQLiteObjectCountForTest(
	t *testing.T,
	path string,
	kind string,
	name string,
	expected int,
) {
	t.Helper()
	database := openLedgerReadOnlyForTest(t, path)
	defer database.Close()
	var observed int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM sqlite_schema WHERE type = ? AND name = ?`,
		kind,
		name,
	).Scan(&observed); err != nil {
		t.Fatalf("count SQLite %s %s: %v", kind, name, err)
	}
	if observed != expected {
		t.Fatalf(
			"SQLite %s %s count = %d, want %d",
			kind,
			name,
			observed,
			expected,
		)
	}
}

type logicalTableSnapshot struct {
	columns    []string
	rowDigests []string
}

type historicalLedgerSnapshot struct {
	tables              map[string]logicalTableSnapshot
	foreignKeyWitnesses []string
	integrity           string
}

func discoverHistoricalLedgerSnapshot(
	t *testing.T,
	path string,
) historicalLedgerSnapshot {
	t.Helper()
	database := openLedgerReadOnlyForTest(t, path)
	defer database.Close()
	tableNames := loadHistoricalTableNames(t, database)
	tables := make(map[string]logicalTableSnapshot, len(tableNames))
	for _, tableName := range tableNames {
		columns := loadVisibleTableColumns(t, database, tableName)
		tables[tableName] = snapshotLogicalTable(t, database, tableName, columns)
	}
	return historicalLedgerSnapshot{
		tables:              tables,
		foreignKeyWitnesses: loadForeignKeyWitnesses(t, database),
		integrity:           loadIntegrityResult(t, database),
	}
}

func repeatHistoricalLedgerSnapshot(
	t *testing.T,
	path string,
	basis historicalLedgerSnapshot,
) historicalLedgerSnapshot {
	t.Helper()
	database := openLedgerReadOnlyForTest(t, path)
	defer database.Close()
	tableNames := make([]string, 0, len(basis.tables))
	for tableName := range basis.tables {
		tableNames = append(tableNames, tableName)
	}
	sort.Strings(tableNames)
	tables := make(map[string]logicalTableSnapshot, len(tableNames))
	for _, tableName := range tableNames {
		columns := basis.tables[tableName].columns
		tables[tableName] = snapshotLogicalTable(t, database, tableName, columns)
	}
	return historicalLedgerSnapshot{
		tables:              tables,
		foreignKeyWitnesses: loadForeignKeyWitnesses(t, database),
		integrity:           loadIntegrityResult(t, database),
	}
}

func openLedgerReadOnlyForTest(t *testing.T, path string) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatalf("open ledger read-only: %v", err)
	}
	if err := database.Ping(); err != nil {
		_ = database.Close()
		t.Fatalf("ping ledger read-only: %v", err)
	}
	return database
}

func loadHistoricalTableNames(t *testing.T, database *sql.DB) []string {
	t.Helper()
	rows, err := database.Query(`SELECT name
		FROM sqlite_schema
		WHERE type = 'table'
			AND name NOT LIKE 'sqlite_%'
			AND name != 'schema_version'
		ORDER BY name`)
	if err != nil {
		t.Fatalf("list historical ledger tables: %v", err)
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("decode historical ledger table: %v", err)
		}
		result = append(result, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("list historical ledger tables: %v", err)
	}
	return result
}

func loadVisibleTableColumns(
	t *testing.T,
	database *sql.DB,
	tableName string,
) []string {
	t.Helper()
	quotedTable := quoteSQLiteIdentifierForTest(tableName)
	query := "PRAGMA table_xinfo(" + quotedTable + ")"
	rows, err := database.Query(query)
	if err != nil {
		t.Fatalf("inspect historical table %s: %v", tableName, err)
	}
	defer rows.Close()
	columns := make([]string, 0)
	for rows.Next() {
		var columnID int
		var name string
		var declaredType string
		var notNull int
		var defaultValue any
		var primaryKey int
		var hidden int
		err := rows.Scan(
			&columnID,
			&name,
			&declaredType,
			&notNull,
			&defaultValue,
			&primaryKey,
			&hidden,
		)
		if err != nil {
			t.Fatalf("decode historical table %s column: %v", tableName, err)
		}
		if hidden == 0 {
			columns = append(columns, name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("inspect historical table %s: %v", tableName, err)
	}
	if len(columns) == 0 {
		t.Fatalf("historical table %s has no visible columns", tableName)
	}
	return columns
}

func snapshotLogicalTable(
	t *testing.T,
	database *sql.DB,
	tableName string,
	columns []string,
) logicalTableSnapshot {
	t.Helper()
	quotedColumns := make([]string, len(columns))
	for index, column := range columns {
		quotedColumns[index] = quoteSQLiteIdentifierForTest(column)
	}
	quotedTable := quoteSQLiteIdentifierForTest(tableName)
	projection := strings.Join(quotedColumns, ", ")
	query := "SELECT " + projection + " FROM " + quotedTable
	rows, err := database.Query(query)
	if err != nil {
		t.Fatalf("snapshot historical table %s: %v", tableName, err)
	}
	defer rows.Close()
	rowDigests := make([]string, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		destinations := make([]any, len(columns))
		for index := range values {
			destinations[index] = &values[index]
		}
		if err := rows.Scan(destinations...); err != nil {
			t.Fatalf("decode historical table %s row: %v", tableName, err)
		}
		rowDigest := digestLogicalRow(t, tableName, values)
		rowDigests = append(rowDigests, rowDigest)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("snapshot historical table %s: %v", tableName, err)
	}
	sort.Strings(rowDigests)
	return logicalTableSnapshot{
		columns:    append([]string(nil), columns...),
		rowDigests: rowDigests,
	}
}

func digestLogicalRow(t *testing.T, tableName string, values []any) string {
	t.Helper()
	cells := make([]string, len(values))
	for index, value := range values {
		cell, err := canonicalSQLiteCell(value)
		if err != nil {
			t.Fatalf("canonicalize historical table %s row: %v", tableName, err)
		}
		cells[index] = cell
	}
	canonical := strings.Join(cells, "\x1f")
	digest := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("sha256:%x", digest)
}

func canonicalSQLiteCell(value any) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "null", nil
	case int64:
		return fmt.Sprintf("integer:%d", typed), nil
	case float64:
		return fmt.Sprintf("real:%016x", math.Float64bits(typed)), nil
	case string:
		return digestSQLiteBytes("text", []byte(typed)), nil
	case []byte:
		return digestSQLiteBytes("blob", typed), nil
	case time.Time:
		canonical := typed.UTC().Format(time.RFC3339Nano)
		return digestSQLiteBytes("time", []byte(canonical)), nil
	default:
		return "", fmt.Errorf("unsupported SQLite value type %T", value)
	}
}

func digestSQLiteBytes(kind string, value []byte) string {
	digest := sha256.Sum256(value)
	return fmt.Sprintf("%s:%d:%x", kind, len(value), digest)
}

func loadForeignKeyWitnesses(t *testing.T, database *sql.DB) []string {
	t.Helper()
	rows, err := database.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatalf("inspect historical foreign-key witnesses: %v", err)
	}
	defer rows.Close()
	witnesses := make([]string, 0)
	for rows.Next() {
		var tableName string
		var rowID any
		var parent string
		var foreignKeyID int
		err := rows.Scan(&tableName, &rowID, &parent, &foreignKeyID)
		if err != nil {
			t.Fatalf("decode historical foreign-key witness: %v", err)
		}
		canonicalRowID, err := canonicalSQLiteCell(rowID)
		if err != nil {
			t.Fatalf("canonicalize historical foreign-key witness: %v", err)
		}
		witness := fmt.Sprintf(
			"%s|%s|%s|%d",
			tableName,
			canonicalRowID,
			parent,
			foreignKeyID,
		)
		witnesses = append(witnesses, witness)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("inspect historical foreign-key witnesses: %v", err)
	}
	sort.Strings(witnesses)
	return witnesses
}

func loadIntegrityResult(t *testing.T, database *sql.DB) string {
	t.Helper()
	var result string
	if err := database.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		t.Fatalf("inspect historical ledger integrity: %v", err)
	}
	return result
}

func assertRequiredI0Frontiers(t *testing.T, snapshot historicalLedgerSnapshot) {
	t.Helper()
	required := []string{
		"project_ledger_binding",
		"project_typeenv_heads",
		"typed_memory_graph_heads",
	}
	for _, tableName := range required {
		table, ok := snapshot.tables[tableName]
		if !ok {
			t.Fatalf("I0 predecessor lacks required table %s", tableName)
		}
		if len(table.rowDigests) != 1 {
			t.Fatalf(
				"I0 predecessor has %d rows in %s, want exactly one",
				len(table.rowDigests),
				tableName,
			)
		}
	}
}

func assertHistoricalLedgerPreserved(
	t *testing.T,
	before historicalLedgerSnapshot,
	after historicalLedgerSnapshot,
) {
	t.Helper()
	if before.integrity != "ok" || after.integrity != "ok" {
		t.Fatalf(
			"ledger integrity changed or is not clean: before=%q after=%q",
			before.integrity,
			after.integrity,
		)
	}
	if !reflect.DeepEqual(after.foreignKeyWitnesses, before.foreignKeyWitnesses) {
		t.Fatalf(
			"historical foreign-key witness set changed\nbefore: %v\nafter:  %v",
			before.foreignKeyWitnesses,
			after.foreignKeyWitnesses,
		)
	}
	tableNames := make([]string, 0, len(before.tables))
	for tableName := range before.tables {
		tableNames = append(tableNames, tableName)
	}
	sort.Strings(tableNames)
	for _, tableName := range tableNames {
		beforeTable := before.tables[tableName]
		afterTable, ok := after.tables[tableName]
		if !ok {
			t.Fatalf("historical table %s disappeared", tableName)
		}
		if !reflect.DeepEqual(afterTable, beforeTable) {
			t.Fatalf(
				"historical table %s changed: before=%d rows after=%d rows",
				tableName,
				len(beforeTable.rowDigests),
				len(afterTable.rowDigests),
			)
		}
	}
}

func quoteSQLiteIdentifierForTest(value string) string {
	escaped := strings.ReplaceAll(value, `"`, `""`)
	return `"` + escaped + `"`
}

func digestFileForTest(t *testing.T, path string) string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open file for digest %s: %v", path, err)
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		t.Fatalf("digest file %s: %v", path, err)
	}
	return fmt.Sprintf("sha256:%x", hasher.Sum(nil))
}

func snapshotHostCarriers(t *testing.T, root string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	roots := []string{
		".agents",
		".claude",
		".codex",
		".cursor",
		".grok",
		".pi",
		".mcp.json",
		"AGENTS.md",
		"CLAUDE.md",
		"opencode.json",
	}
	sort.Strings(roots)
	for _, relativeRoot := range roots {
		absoluteRoot := filepath.Join(root, relativeRoot)
		if _, err := os.Lstat(absoluteRoot); os.IsNotExist(err) {
			result[relativeRoot] = "missing"
			continue
		}
		err := filepath.WalkDir(absoluteRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if entry.IsDir() {
				result[relative] = fmt.Sprintf("dir:%o", info.Mode().Perm())
				return nil
			}
			if info.Mode()&os.ModeSymlink != 0 {
				target, err := os.Readlink(path)
				if err != nil {
					return err
				}
				result[relative] = "symlink:" + target
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(content)
			result[relative] = fmt.Sprintf("file:%o:%x", info.Mode().Perm(), digest)
			return nil
		})
		if err != nil {
			t.Fatalf("snapshot host carrier %s: %v", relativeRoot, err)
		}
	}
	return result
}
