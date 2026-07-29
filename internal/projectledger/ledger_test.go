package projectledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/db"
)

func TestParseProjectIDRejectsTraversalAndNonCanonicalSyntax(t *testing.T) {
	for _, valid := range []string{"qnt_a7f3b2c1", "qnt_00000000", "qnt_ffffffff"} {
		if _, err := ParseProjectID(valid); err != nil {
			t.Fatalf("ParseProjectID(%q): %v", valid, err)
		}
	}
	for _, invalid := range []string{
		"../qnt_a7f3b2c1",
		"qnt_../../escape",
		"qnt_A7F3B2C1",
		" qnt_a7f3b2c1",
		"qnt_a7f3b2c1 ",
		"qnt_a7f3b2c1/other",
		"qnt_a7f3b2c1\\other",
		"qnt_test",
		"qnt_cli-migration_2",
		"qnt_",
		"other_a7f3b2c1",
		"qnt_a7f3\x00b2c1",
	} {
		if _, err := ParseProjectID(invalid); err == nil {
			t.Fatalf("ParseProjectID accepted %q", invalid)
		}
	}
}

func TestInitializedProjectLedgerBindsAndReopensExactIdentity(t *testing.T) {
	fixture := newProjectLedgerFixture(t, "qnt_a7f3b2c1")
	boundAt := time.Date(2026, time.July, 15, 8, 0, 0, 0, time.UTC)
	if err := BindInitialized(context.Background(), fixture.root, boundAt); err != nil {
		t.Fatalf("BindInitialized: %v", err)
	}
	if err := BindInitialized(context.Background(), fixture.root, boundAt.Add(time.Hour)); err != nil {
		t.Fatalf("exact idempotent BindInitialized: %v", err)
	}

	handle, err := OpenExisting(context.Background(), fixture.root, ReadWrite)
	if err != nil {
		t.Fatalf("OpenExisting: %v", err)
	}
	defer handle.Close()
	if handle.ProjectID().String() != fixture.id || handle.ProjectRoot().String() != fixture.root {
		t.Fatalf("checked identity = %s / %s", handle.ProjectID().String(), handle.ProjectRoot().String())
	}
	if err := handle.Revalidate(context.Background()); err != nil {
		t.Fatalf("Revalidate: %v", err)
	}

	for _, statement := range []string{
		"UPDATE project_ledger_binding SET bound_at = bound_at",
		"DELETE FROM project_ledger_binding",
		"INSERT OR REPLACE INTO project_ledger_binding SELECT * FROM project_ledger_binding",
	} {
		if _, err := handle.Database().Exec(statement); err == nil {
			t.Fatalf("immutable binding accepted %q", statement)
		}
	}
}

func TestProjectLedgerRejectsCopiedRootWithSameID(t *testing.T) {
	fixture := newProjectLedgerFixture(t, "qnt_b7f3b2c1")
	if err := BindInitialized(context.Background(), fixture.root, time.Now().UTC()); err != nil {
		t.Fatalf("BindInitialized: %v", err)
	}
	copiedRoot := canonicalTempDirectory(t)
	writeProjectIdentity(t, copiedRoot, fixture.id)

	_, err := OpenExisting(context.Background(), copiedRoot, ReadOnly)
	if err == nil || !strings.Contains(err.Error(), "durably bound") {
		t.Fatalf("copied-root OpenExisting error = %v", err)
	}
}

func TestProjectLedgerRejectsSymlinkAndNonRegularDatabase(t *testing.T) {
	fixture := newProjectLedgerFixture(t, "qnt_c7f3b2c1")
	target := fixture.dbPath + ".target"
	if err := os.Rename(fixture.dbPath, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, fixture.dbPath); err != nil {
		t.Fatal(err)
	}
	_, err := OpenExisting(context.Background(), fixture.root, ReadOnly)
	if err == nil || !strings.Contains(err.Error(), "not a symlink") {
		t.Fatalf("symlink database error = %v", err)
	}

	if err := os.Remove(fixture.dbPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(fixture.dbPath, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err = OpenExisting(context.Background(), fixture.root, ReadOnly)
	if err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("directory database error = %v", err)
	}
}

func TestProjectLedgerOpenRequiresExplicitInitBinding(t *testing.T) {
	fixture := newProjectLedgerFixture(t, "qnt_d7f3b2c1")
	_, err := OpenExisting(context.Background(), fixture.root, ReadOnly)
	if !errorsIs(err, ErrBindingMissing) {
		t.Fatalf("unbound OpenExisting error = %v", err)
	}
}

func TestExplicitMigrationOpenRetainsTopologyChecksWithoutRequiringBinding(
	t *testing.T,
) {
	fixture := newProjectLedgerFixture(t, "qnt_d6f3b2c1")
	handle, err := OpenForExplicitMigration(
		context.Background(),
		fixture.root,
		ReadOnly,
	)
	if err != nil {
		t.Fatalf("OpenForExplicitMigration: %v", err)
	}
	defer handle.Close()
	if handle.ProjectID().String() != fixture.id {
		t.Fatalf(
			"migration handle project = %s, want %s",
			handle.ProjectID().String(),
			fixture.id,
		)
	}
	if handle.DatabasePath() != fixture.dbPath {
		t.Fatalf(
			"migration handle database = %s, want %s",
			handle.DatabasePath(),
			fixture.dbPath,
		)
	}
	if err := handle.RequireAttachedIdentity(
		context.Background(),
	); !errorsIs(err, ErrBindingMissing) {
		t.Fatalf("unbound migration handle identity error = %v", err)
	}
	directory := filepath.Dir(fixture.dbPath)
	moved := directory + ".moved"
	if err := os.Rename(directory, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(directory, "haft.db"),
		[]byte("replacement"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := handle.RequireAttachedIdentity(
		context.Background(),
	); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf(
			"unbound migration handle accepted a topology swap: %v",
			err,
		)
	}
}

func TestMigrationBindingRejectsOrdinaryTransactionAtCurrentSchema(
	t *testing.T,
) {
	fixture := newProjectLedgerFixture(t, "qnt_d5f3b2c1")
	handle, err := OpenForExplicitMigration(
		context.Background(),
		fixture.root,
		ReadWrite,
	)
	if err != nil {
		t.Fatalf("OpenForExplicitMigration: %v", err)
	}
	defer handle.Close()
	transaction, err := handle.Database().Begin()
	if err != nil {
		t.Fatalf("begin ordinary transaction: %v", err)
	}
	err = handle.BindDuringFirstDurableSchemaMigration(
		context.Background(),
		transaction,
		time.Now().UTC(),
	)
	rollbackErr := transaction.Rollback()
	if err == nil ||
		!strings.Contains(err.Error(), "exact contiguous schema-36 predecessor") {
		t.Fatalf("current-schema migration bind error = %v", err)
	}
	if rollbackErr != nil {
		t.Fatalf("roll back ordinary transaction: %v", rollbackErr)
	}
	if err := handle.RequireAttachedIdentity(
		context.Background(),
	); !errors.Is(err, ErrBindingMissing) {
		t.Fatalf("ordinary transaction repaired current ledger: %v", err)
	}
}

func TestProjectLedgerBindingRejectsPreexistingForeignRootRows(t *testing.T) {
	fixture := newProjectLedgerFixture(t, "qnt_d8f3b2c1")
	store, err := db.NewStore(fixture.dbPath)
	if err != nil {
		t.Fatalf("db.NewStore: %v", err)
	}
	_, err = store.GetRawDB().Exec(`INSERT INTO observed_project_bases (
		observed_project_basis_ref, project_root,
		observation_from, observation_until,
		detector_version, classifier_version,
		observed_project_basis_json, observed_project_basis_digest, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"observed-basis:foreign",
		"/tmp/foreign-root",
		"2026-07-15T10:00:00Z",
		"2026-07-15T10:00:01Z",
		"detector:v1",
		"classifier:v1",
		`{"project_root":"/tmp/foreign-root"}`,
		"sha256:foreign-root-basis",
		"2026-07-15T10:00:01Z",
	)
	if err != nil {
		_ = store.Close()
		t.Fatalf("insert prebinding foreign root: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	err = BindInitialized(context.Background(), fixture.root, time.Now().UTC())
	if err == nil || !strings.Contains(err.Error(), "observed_project_bases") {
		t.Fatalf("BindInitialized accepted preexisting foreign root: %v", err)
	}
}

func TestProjectLedgerBindingDiscoversFutureRootBearingTables(t *testing.T) {
	fixture := newProjectLedgerFixture(t, "qnt_d9f3b2c1")
	store, err := db.NewStore(fixture.dbPath)
	if err != nil {
		t.Fatalf("db.NewStore: %v", err)
	}
	_, err = store.GetRawDB().Exec(
		`CREATE TABLE "future""rooted_extension" (
			entry_id TEXT PRIMARY KEY,
			project_root TEXT NOT NULL
		) WITHOUT ROWID`,
	)
	if err != nil {
		_ = store.Close()
		t.Fatalf("create future root-bearing table: %v", err)
	}
	_, err = store.GetRawDB().Exec(
		`INSERT INTO "future""rooted_extension" (entry_id, project_root) VALUES (?, ?)`,
		"future:foreign",
		"/tmp/foreign-root",
	)
	if err != nil {
		_ = store.Close()
		t.Fatalf("insert future foreign root: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	err = BindInitialized(context.Background(), fixture.root, time.Now().UTC())
	if err == nil || !strings.Contains(err.Error(), `future"rooted_extension`) {
		t.Fatalf("BindInitialized accepted discovered future foreign root: %v", err)
	}
}

func TestProjectLedgerRevalidateRejectsAnchoredDirectorySwap(t *testing.T) {
	fixture := newProjectLedgerFixture(t, "qnt_e7f3b2c1")
	if err := BindInitialized(context.Background(), fixture.root, time.Now().UTC()); err != nil {
		t.Fatalf("BindInitialized: %v", err)
	}
	handle, err := OpenExisting(context.Background(), fixture.root, ReadOnly)
	if err != nil {
		t.Fatalf("OpenExisting: %v", err)
	}
	defer handle.Close()

	directory := filepath.Dir(fixture.dbPath)
	moved := directory + ".moved"
	if err := os.Rename(directory, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "haft.db"), []byte("replacement"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := handle.Revalidate(context.Background()); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("Revalidate after anchored directory swap = %v", err)
	}
}

func TestProjectLedgerRevalidateRejectsInPlaceIdentityCarrierMutation(t *testing.T) {
	fixture := newProjectLedgerFixture(t, "qnt_f7f3b2c1")
	if err := BindInitialized(context.Background(), fixture.root, time.Now().UTC()); err != nil {
		t.Fatalf("BindInitialized: %v", err)
	}
	handle, err := OpenExisting(context.Background(), fixture.root, ReadOnly)
	if err != nil {
		t.Fatalf("OpenExisting: %v", err)
	}
	defer handle.Close()
	carrier := filepath.Join(fixture.root, ".haft", "project.yaml")
	if err := os.WriteFile(carrier, []byte("id: qnt_11111111\nname: replacement\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := handle.Revalidate(context.Background()); err == nil || !strings.Contains(err.Error(), "content changed") {
		t.Fatalf("Revalidate after identity mutation = %v", err)
	}
}

func TestProjectLedgerRevalidateRejectsReplacedSQLiteSidecarGenerationBeforeSQLiteRead(
	t *testing.T,
) {
	fixture := newProjectLedgerFixture(t, "qnt_07f3b2c1")
	if err := BindInitialized(
		context.Background(),
		fixture.root,
		time.Now().UTC(),
	); err != nil {
		t.Fatalf("BindInitialized: %v", err)
	}
	handle, err := OpenExisting(
		context.Background(),
		fixture.root,
		ReadWrite,
	)
	if err != nil {
		t.Fatalf("OpenExisting: %v", err)
	}
	defer handle.Close()

	_, err = handle.Database().Exec(
		`CREATE TABLE project_ledger_sidecar_generation_probe (
			probe_id TEXT PRIMARY KEY
		) WITHOUT ROWID`,
	)
	if err != nil {
		t.Fatalf("create sidecar generation probe: %v", err)
	}
	if err := handle.Revalidate(context.Background()); err != nil {
		t.Fatalf("capture live sidecar generation: %v", err)
	}

	for _, suffix := range []string{"-wal", "-shm"} {
		path := fixture.dbPath + suffix
		priorPath := path + ".prior-generation"
		if err := os.Rename(path, priorPath); err != nil {
			t.Fatalf("replace SQLite sidecar %s: %v", suffix, err)
		}
		content, err := os.ReadFile(priorPath)
		if err != nil {
			t.Fatalf("read prior SQLite sidecar %s: %v", suffix, err)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("write replacement SQLite sidecar %s: %v", suffix, err)
		}
	}

	err = handle.Revalidate(context.Background())
	if err == nil ||
		!errors.Is(err, ErrSQLiteSidecarGenerationChanged) ||
		!strings.Contains(err.Error(), "SQLite sidecar generation changed") ||
		strings.Contains(strings.ToLower(err.Error()), "malformed") {
		t.Fatalf("Revalidate after SQLite sidecar replacement = %v", err)
	}
}

func TestProjectLedgerPreservesSidecarGenerationAcrossIndependentWriterClose(
	t *testing.T,
) {
	fixture := newProjectLedgerFixture(t, "qnt_08f3b2c1")
	if err := BindInitialized(
		context.Background(),
		fixture.root,
		time.Now().UTC(),
	); err != nil {
		t.Fatalf("BindInitialized: %v", err)
	}
	reader, err := OpenExisting(
		context.Background(),
		fixture.root,
		ReadWrite,
	)
	if err != nil {
		t.Fatalf("open long-lived project ledger: %v", err)
	}
	defer reader.Close()
	captured := observeProjectLedgerSidecars(t, fixture.dbPath)

	writer, err := OpenExisting(
		context.Background(),
		fixture.root,
		ReadWrite,
	)
	if err != nil {
		t.Fatalf("open independent project ledger writer: %v", err)
	}
	if _, err := writer.Database().Exec(
		`CREATE TABLE persistent_sidecar_generation_probe (
			probe_id TEXT PRIMARY KEY
		) WITHOUT ROWID`,
	); err != nil {
		_ = writer.Close()
		t.Fatalf("write through independent project ledger: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close independent project ledger writer: %v", err)
	}

	observed := observeProjectLedgerSidecars(t, fixture.dbPath)
	for index := range captured {
		if !os.SameFile(captured[index], observed[index]) {
			t.Fatalf(
				"project ledger sidecar %d changed generation after writer close",
				index,
			)
		}
	}
	if err := reader.Revalidate(context.Background()); err != nil {
		t.Fatalf(
			"long-lived project ledger rejected persistent WAL generation: %v",
			err,
		)
	}
}

func TestSQLiteSidecarGenerationAdoptsFirstActivationButNotReplacement(
	t *testing.T,
) {
	databasePath := filepath.Join(t.TempDir(), "haft.db")
	generation := newSQLiteSidecarGeneration(databasePath)
	if err := generation.Revalidate(); err != nil {
		t.Fatalf("capture absent sidecars: %v", err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.WriteFile(
			databasePath+suffix,
			[]byte("first-generation"),
			0o600,
		); err != nil {
			t.Fatalf("create first SQLite sidecar %s: %v", suffix, err)
		}
	}
	if err := generation.Revalidate(); err != nil {
		t.Fatalf("adopt first SQLite sidecar activation: %v", err)
	}
	walPath := databasePath + "-wal"
	walFile, err := os.OpenFile(walPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open active WAL generation: %v", err)
	}
	if _, err := walFile.WriteString("-growth"); err != nil {
		_ = walFile.Close()
		t.Fatalf("grow active WAL generation: %v", err)
	}
	if err := walFile.Close(); err != nil {
		t.Fatalf("close active WAL generation: %v", err)
	}
	if err := generation.Revalidate(); err != nil {
		t.Fatalf("same-inode WAL growth was rejected: %v", err)
	}
	if err := os.Rename(walPath, walPath+".prior"); err != nil {
		t.Fatalf("move active WAL generation: %v", err)
	}
	if err := os.WriteFile(
		walPath,
		[]byte("replacement-generation"),
		0o600,
	); err != nil {
		t.Fatalf("create replacement WAL generation: %v", err)
	}
	err = generation.Revalidate()
	if !errors.Is(err, ErrSQLiteSidecarGenerationChanged) {
		t.Fatalf("replacement WAL generation error = %v", err)
	}
}

func observeProjectLedgerSidecars(
	t *testing.T,
	databasePath string,
) []os.FileInfo {
	t.Helper()
	result := make([]os.FileInfo, 0, 2)
	for _, suffix := range []string{"-wal", "-shm"} {
		info, err := os.Lstat(databasePath + suffix)
		if err != nil {
			t.Fatalf("observe project ledger sidecar %s: %v", suffix, err)
		}
		result = append(result, info)
	}
	return result
}

func TestConcurrentProjectLedgerBindingAdmitsExactlyOnePhysicalRoot(t *testing.T) {
	fixture := newProjectLedgerFixture(t, "qnt_17f3b2c1")
	competingRoot := canonicalTempDirectory(t)
	writeProjectIdentity(t, competingRoot, fixture.id)
	roots := []string{fixture.root, competingRoot}
	errorsByRoot := make([]error, len(roots))
	start := make(chan struct{})
	wait := sync.WaitGroup{}
	wait.Add(len(roots))
	for index, root := range roots {
		go func(position int, candidate string) {
			defer wait.Done()
			<-start
			errorsByRoot[position] = BindInitialized(context.Background(), candidate, time.Now().UTC())
		}(index, root)
	}
	close(start)
	wait.Wait()
	successes := 0
	failures := 0
	for _, err := range errorsByRoot {
		if err == nil {
			successes++
			continue
		}
		if !strings.Contains(err.Error(), "durably bound") {
			t.Fatalf("unexpected concurrent bind error: %v", err)
		}
		failures++
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("concurrent bind outcomes: successes=%d failures=%d errors=%v", successes, failures, errorsByRoot)
	}
}

func TestProjectLedgerBindingReportsCommittedOutcomeWhenPathSwapsBeforePostCheck(t *testing.T) {
	fixture := newProjectLedgerFixture(t, "qnt_27f3b2c1")
	identity, identityAnchors, err := loadIdentityAnchored(fixture.root)
	if err != nil {
		t.Fatalf("loadIdentityAnchored: %v", err)
	}
	handle, err := openTopology(context.Background(), identity, identityAnchors, ReadWrite)
	if err != nil {
		closeAnchors(identityAnchors)
		t.Fatalf("openTopology: %v", err)
	}
	defer handle.Close()
	directory := filepath.Dir(fixture.dbPath)
	moved := directory + ".moved"
	hook := func(stage bindingStage) error {
		if stage != bindingStageAfterCommitBeforeCheck {
			return nil
		}
		if err := os.Rename(directory, moved); err != nil {
			return err
		}
		if err := os.Mkdir(directory, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(directory, "haft.db"), []byte("replacement"), 0o644)
	}
	err = handle.bindInitializedWithHook(
		context.Background(),
		time.Now().UTC(),
		hook,
	)
	if !errors.Is(err, ErrBindingCommittedTopologyChanged) {
		t.Fatalf("bind with post-commit topology swap = %v", err)
	}
	if err := requireAttachedIdentity(context.Background(), handle.Database(), identity); err != nil {
		t.Fatalf("anchored database did not retain committed binding: %v", err)
	}
}

func TestProjectLedgerBindingRollsBackWhenTopologyChangesBeforeCommit(t *testing.T) {
	fixture := newProjectLedgerFixture(t, "qnt_37f3b2c1")
	identity, identityAnchors, err := loadIdentityAnchored(fixture.root)
	if err != nil {
		t.Fatalf("loadIdentityAnchored: %v", err)
	}
	handle, err := openTopology(context.Background(), identity, identityAnchors, ReadWrite)
	if err != nil {
		closeAnchors(identityAnchors)
		t.Fatalf("openTopology: %v", err)
	}
	defer handle.Close()
	directory := filepath.Dir(fixture.dbPath)
	moved := directory + ".moved"
	hook := func(stage bindingStage) error {
		if stage != bindingStageBeforeCommit {
			return nil
		}
		if err := os.Rename(directory, moved); err != nil {
			return err
		}
		if err := os.Mkdir(directory, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(directory, "haft.db"), []byte("replacement"), 0o644)
	}
	err = handle.bindInitializedWithHook(
		context.Background(),
		time.Now().UTC(),
		hook,
	)
	if err == nil || !strings.Contains(err.Error(), "before binding commit") {
		t.Fatalf("bind with pre-commit topology swap = %v", err)
	}
	if errors.Is(err, ErrBindingCommittedTopologyChanged) {
		t.Fatalf("pre-commit topology swap reported a committed outcome: %v", err)
	}
	err = requireAttachedIdentity(context.Background(), handle.Database(), identity)
	if !errors.Is(err, ErrBindingMissing) {
		t.Fatalf("pre-commit topology drift did not roll binding back: %v", err)
	}
}

func TestMigrationBindingRollsBackVersionAndIdentityWhenTopologyChanges(
	t *testing.T,
) {
	if db.ProjectLedgerBindingSchemaVersion !=
		firstDurableBindingPredecessorSchema+1 {
		t.Fatalf(
			"binding schema boundary = %d, want predecessor %d plus one",
			db.ProjectLedgerBindingSchemaVersion,
			firstDurableBindingPredecessorSchema,
		)
	}
	fixture := newProjectLedgerFixture(t, "qnt_38f3b2c1")
	database, err := sql.Open("sqlite", fixture.dbPath)
	if err != nil {
		t.Fatalf("open project ledger fixture: %v", err)
	}
	if _, err := database.Exec(
		`DELETE FROM schema_version WHERE version > ?`,
		firstDurableBindingPredecessorSchema,
	); err != nil {
		_ = database.Close()
		t.Fatalf("rewind migration frontier fixture: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close rewound migration fixture: %v", err)
	}
	handle, err := OpenForExplicitMigration(
		context.Background(),
		fixture.root,
		ReadWrite,
	)
	if err != nil {
		t.Fatalf("OpenForExplicitMigration: %v", err)
	}
	defer handle.Close()
	directory := filepath.Dir(fixture.dbPath)
	moved := directory + ".moved"
	err = db.RunMigrationsThroughProjectLedgerBinding(
		handle.Database(),
		func(transaction db.MigrationTransaction) error {
			return handle.bindDuringFirstDurableSchemaMigration(
				context.Background(),
				transaction,
				time.Now().UTC(),
				func() error {
					if err := os.Rename(directory, moved); err != nil {
						return err
					}
					if err := os.Mkdir(directory, 0o755); err != nil {
						return err
					}
					return os.WriteFile(
						filepath.Join(directory, "haft.db"),
						[]byte("replacement"),
						0o644,
					)
				},
			)
		},
	)
	if err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("binding migration topology swap error = %v", err)
	}
	if errors.Is(err, ErrBindingCommittedTopologyChanged) {
		t.Fatalf("rolled-back binding reported committed outcome: %v", err)
	}
	var versionCount int
	if err := handle.Database().QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = ?",
		db.ProjectLedgerBindingSchemaVersion,
	).Scan(&versionCount); err != nil {
		t.Fatalf("inspect rolled-back binding version: %v", err)
	}
	if versionCount != 0 {
		t.Fatalf("rolled-back binding version count = %d, want 0", versionCount)
	}
	if err := requireAttachedIdentity(
		context.Background(),
		handle.Database(),
		handle.identity,
	); !errors.Is(err, ErrBindingMissing) {
		t.Fatalf("rolled-back migration retained a binding: %v", err)
	}
}

func TestProjectLedgerSQLiteIdentityBoundaryRejectsPersistentConnectWindowSwap(t *testing.T) {
	stages := []sqliteOpenStage{
		sqliteOpenStageBeforeConnect,
		sqliteOpenStageAfterConnectBeforeIdentityCheck,
	}
	for index, targetStage := range stages {
		t.Run(string(targetStage), func(t *testing.T) {
			fixture := newProjectLedgerFixture(t, fmt.Sprintf("qnt_47f3b2c%d", index))
			replacementDirectory := prepareReplacementLedgerDirectory(t, fixture, "persistent")
			identity, identityAnchors, err := loadIdentityAnchored(fixture.root)
			if err != nil {
				t.Fatalf("loadIdentityAnchored: %v", err)
			}
			originalDirectory := filepath.Dir(fixture.dbPath)
			movedDirectory := originalDirectory + ".anchored"
			hook := func(stage sqliteOpenStage) error {
				if stage != targetStage {
					return nil
				}
				if err := os.Rename(originalDirectory, movedDirectory); err != nil {
					return err
				}
				return os.Rename(replacementDirectory, originalDirectory)
			}
			handle, err := openTopologyWithHook(
				context.Background(),
				identity,
				identityAnchors,
				ReadWrite,
				hook,
			)
			if handle != nil {
				_ = handle.Close()
			}
			_ = closeAnchors(identityAnchors)
			if err == nil || !strings.Contains(err.Error(), "anchored database inode") {
				t.Fatalf("persistent %s swap error = %v", targetStage, err)
			}
		})
	}
}

// This test is the explicit limit of the process-local observed-path boundary.
// A replacement opened by SQLite and restored before the post-connect pathname
// observation cannot be distinguished without a descriptor-aware SQLite VFS.
// The production contract therefore detects drift visible at either boundary;
// it does not claim immunity to an adversarial swap-and-restore wholly inside
// that gap.
func TestProjectLedgerSQLiteObservedPathBoundaryDoesNotClaimSwapAndRestoreProof(t *testing.T) {
	fixture := newProjectLedgerFixture(t, "qnt_57f3b2c1")
	replacementDirectory := prepareReplacementLedgerDirectory(t, fixture, "replacement")
	identity, identityAnchors, err := loadIdentityAnchored(fixture.root)
	if err != nil {
		t.Fatalf("loadIdentityAnchored: %v", err)
	}
	originalDirectory := filepath.Dir(fixture.dbPath)
	anchoredDirectory := originalDirectory + ".anchored"
	openedReplacementDirectory := originalDirectory + ".opened-replacement"
	hook := func(stage sqliteOpenStage) error {
		switch stage {
		case sqliteOpenStageBeforeConnect:
			if err := os.Rename(originalDirectory, anchoredDirectory); err != nil {
				return err
			}
			return os.Rename(replacementDirectory, originalDirectory)
		case sqliteOpenStageAfterConnectBeforeIdentityCheck:
			if err := os.Rename(originalDirectory, openedReplacementDirectory); err != nil {
				return err
			}
			return os.Rename(anchoredDirectory, originalDirectory)
		default:
			return nil
		}
	}
	handle, err := openTopologyWithHook(
		context.Background(),
		identity,
		identityAnchors,
		ReadWrite,
		hook,
	)
	if err != nil {
		closeAnchors(identityAnchors)
		t.Fatalf("openTopologyWithHook at documented identity limit: %v", err)
	}
	defer handle.Close()
	marker := ""
	if err := handle.Database().QueryRow(`SELECT marker FROM project_ledger_identity_probe`).Scan(&marker); err != nil {
		t.Fatalf("read opened replacement marker: %v", err)
	}
	if marker != "replacement" {
		t.Fatalf("opened database marker = %q", marker)
	}
}

func prepareReplacementLedgerDirectory(
	t *testing.T,
	fixture projectLedgerFixture,
	marker string,
) string {
	t.Helper()
	directory := filepath.Dir(fixture.dbPath) + ".replacement"
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(directory, "haft.db")
	store, err := db.NewStore(databasePath)
	if err != nil {
		t.Fatalf("db.NewStore replacement: %v", err)
	}
	_, err = store.GetRawDB().Exec(`CREATE TABLE project_ledger_identity_probe (marker TEXT NOT NULL)`)
	if err != nil {
		_ = store.Close()
		t.Fatalf("create replacement identity marker: %v", err)
	}
	_, err = store.GetRawDB().Exec(
		`INSERT INTO project_ledger_identity_probe (marker) VALUES (?)`,
		marker,
	)
	if err != nil {
		_ = store.Close()
		t.Fatalf("insert replacement identity marker: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	return directory
}

type projectLedgerFixture struct {
	home   string
	root   string
	id     string
	dbPath string
}

func newProjectLedgerFixture(t *testing.T, id string) projectLedgerFixture {
	t.Helper()
	home := canonicalTempDirectory(t)
	t.Setenv("HOME", home)
	root := canonicalTempDirectory(t)
	writeProjectIdentity(t, root, id)
	directory := filepath.Join(home, ".haft", "projects", id)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(directory, "haft.db")
	store, err := db.NewStore(databasePath)
	if err != nil {
		t.Fatalf("db.NewStore: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	return projectLedgerFixture{home: home, root: root, id: id, dbPath: databasePath}
}

func writeProjectIdentity(t *testing.T, root string, id string) {
	t.Helper()
	haftDirectory := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("id: " + id + "\nname: fixture\n")
	if err := os.WriteFile(filepath.Join(haftDirectory, "project.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func canonicalTempDirectory(t *testing.T) string {
	t.Helper()
	physical, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(physical)
}

func errorsIs(err error, target error) bool {
	return err != nil && (err == target || strings.Contains(err.Error(), target.Error()))
}
