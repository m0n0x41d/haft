package kerneldbfixture

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kerneldb "github.com/m0n0x41d/haft/db"
)

func TestCurrentSchemaTemplateBuildsCleanSealedIsolatedClones(
	t *testing.T,
) {
	buildRoot := t.TempDir()
	templateContents, err := buildCurrentSchemaTemplateIn(buildRoot)
	if err != nil {
		t.Fatalf("build current schema template: %v", err)
	}
	if len(templateContents) == 0 {
		t.Fatal("current schema template is empty")
	}
	entries, err := os.ReadDir(buildRoot)
	if err != nil {
		t.Fatalf("inspect template build root: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf(
			"template build directory was retained: %q",
			entries[0].Name(),
		)
	}

	firstPath := filepath.Join(t.TempDir(), "first-raw.db")
	if err := cloneTemplate(templateContents, firstPath); err != nil {
		t.Fatalf("clone first raw template: %v", err)
	}
	requireNoSQLiteSidecars(t, firstPath)
	first, err := sql.Open("sqlite", firstPath)
	if err != nil {
		t.Fatalf("open first raw clone: %v", err)
	}
	if err := kerneldb.RequireCurrentSchemaReadOnly(
		context.Background(),
		first,
	); err != nil {
		_ = first.Close()
		t.Fatalf("first raw clone current schema: %v", err)
	}
	requireJournalMode(t, first, "delete")
	requireNoSQLiteSidecars(t, firstPath)
	_, err = first.Exec(
		"CREATE TABLE fixture_isolation_marker (id INTEGER PRIMARY KEY)",
	)
	if err != nil {
		_ = first.Close()
		t.Fatalf("create first raw-clone marker: %v", err)
	}
	if err := sealTemplateDatabase(first); err != nil {
		_ = first.Close()
		t.Fatalf("checkpoint first raw clone: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first raw clone: %v", err)
	}
	requireNoSQLiteSidecars(t, firstPath)

	secondPath := filepath.Join(t.TempDir(), "second-raw.db")
	if err := cloneTemplate(templateContents, secondPath); err != nil {
		t.Fatalf("clone second raw template: %v", err)
	}
	requireNoSQLiteSidecars(t, secondPath)
	second, err := sql.Open("sqlite", secondPath)
	if err != nil {
		t.Fatalf("open second raw clone: %v", err)
	}
	defer func() { _ = second.Close() }()
	if err := kerneldb.RequireCurrentSchemaReadOnly(
		context.Background(),
		second,
	); err != nil {
		t.Fatalf("second raw clone current schema: %v", err)
	}
	requireJournalMode(t, second, "delete")
	requireNoSQLiteSidecars(t, secondPath)
	var markerCount int
	err = second.QueryRowContext(
		context.Background(),
		`SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name = 'fixture_isolation_marker'`,
	).Scan(&markerCount)
	if err != nil {
		t.Fatalf("inspect second raw-clone marker: %v", err)
	}
	if markerCount != 0 {
		t.Fatal("template bytes retained a mutation from another raw clone")
	}
}

func TestOpenCurrentStoreClonesIsolatedCurrentSchemas(t *testing.T) {
	firstPath := filepath.Join(t.TempDir(), "first.db")
	first, err := OpenCurrentStore(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	if err := kerneldb.RequireCurrentSchemaReadOnly(
		context.Background(),
		first.GetRawDB(),
	); err != nil {
		t.Fatalf("first current schema: %v", err)
	}
	_, err = first.GetRawDB().Exec(
		"CREATE TABLE fixture_isolation_marker (id INTEGER PRIMARY KEY)",
	)
	if err != nil {
		t.Fatalf("create first-store marker: %v", err)
	}

	secondPath := filepath.Join(t.TempDir(), "second.db")
	second, err := OpenCurrentStore(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	if err := kerneldb.RequireCurrentSchemaReadOnly(
		context.Background(),
		second.GetRawDB(),
	); err != nil {
		t.Fatalf("second current schema: %v", err)
	}
	var markerCount int
	err = second.GetRawDB().QueryRowContext(
		context.Background(),
		`SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name = 'fixture_isolation_marker'`,
	).Scan(&markerCount)
	if err != nil {
		t.Fatalf("inspect second-store marker: %v", err)
	}
	if markerCount != 0 {
		t.Fatal("current-schema template retained a mutation from another clone")
	}
}

func TestCurrentSchemaTemplateRetriesAfterTransientBuildFailure(t *testing.T) {
	t.Parallel()

	transientErr := errors.New("transient template failure")
	buildCalls := 0
	template := currentSchemaTemplate{
		build: func() ([]byte, error) {
			buildCalls++
			if buildCalls == 1 {
				return nil, transientErr
			}
			return []byte("current schema template"), nil
		},
	}

	if _, err := template.load(); !errors.Is(err, transientErr) {
		t.Fatalf("first load error = %v, want transient failure", err)
	}
	contents, err := template.load()
	if err != nil {
		t.Fatalf("retry load error = %v", err)
	}
	if string(contents) != "current schema template" {
		t.Fatalf("retry contents = %q", contents)
	}
	if _, err := template.load(); err != nil {
		t.Fatalf("cached load error = %v", err)
	}
	if buildCalls != 2 {
		t.Fatalf("builder calls = %d, want one failure and one successful retry", buildCalls)
	}
}

func TestJoinTemplateCleanupErrorPreservesSuccessfulBuild(t *testing.T) {
	t.Parallel()

	cleanupErr := errors.New("cleanup failed")
	if err := joinTemplateCleanupError(nil, cleanupErr); err != nil {
		t.Fatalf("successful build invalidated by cleanup: %v", err)
	}

	buildErr := errors.New("build failed")
	joined := joinTemplateCleanupError(buildErr, cleanupErr)
	if !errors.Is(joined, buildErr) || !errors.Is(joined, cleanupErr) {
		t.Fatalf("failed build cleanup error = %v, want both causes", joined)
	}
}

func TestOpenCurrentStorePreservesExistingDatabaseState(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "restart.db")
	first, err := OpenCurrentStore(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = first.GetRawDB().Exec(
		"CREATE TABLE fixture_restart_marker (id INTEGER PRIMARY KEY)",
	)
	if err != nil {
		t.Fatalf("create restart marker: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first store: %v", err)
	}

	reopened, err := OpenCurrentStore(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	var markerCount int
	err = reopened.GetRawDB().QueryRowContext(
		context.Background(),
		`SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name = 'fixture_restart_marker'`,
	).Scan(&markerCount)
	if err != nil {
		t.Fatalf("inspect restart marker: %v", err)
	}
	if markerCount != 1 {
		t.Fatal("opening an existing fixture replaced its durable state")
	}
}

func TestCloneTemplatePublishesWithoutReplacingAnExistingPath(t *testing.T) {
	directory := t.TempDir()
	destinationPath := filepath.Join(directory, "owned.db")
	const existing = "operator-owned fixture"
	if err := os.WriteFile(destinationPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("write existing destination: %v", err)
	}

	err := cloneTemplate([]byte("replacement"), destinationPath)
	if err == nil {
		t.Fatal("clone replaced an existing destination")
	}
	contents, readErr := os.ReadFile(destinationPath)
	if readErr != nil {
		t.Fatalf("read existing destination: %v", readErr)
	}
	if string(contents) != existing {
		t.Fatalf(
			"existing destination contents = %q, want %q",
			contents,
			existing,
		)
	}
	entries, readDirErr := os.ReadDir(directory)
	if readDirErr != nil {
		t.Fatalf("inspect clone directory: %v", readDirErr)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(destinationPath) {
		t.Fatalf("failed clone retained temporary entries: %#v", entries)
	}
}

func requireJournalMode(t *testing.T, database *sql.DB, want string) {
	t.Helper()
	var got string
	if err := database.QueryRow("PRAGMA journal_mode").Scan(&got); err != nil {
		t.Fatalf("inspect journal mode: %v", err)
	}
	if !strings.EqualFold(got, want) {
		t.Fatalf("journal mode = %q, want %q", got, want)
	}
}

func requireNoSQLiteSidecars(t *testing.T, databasePath string) {
	t.Helper()
	for _, suffix := range []string{"-wal", "-shm"} {
		sidecarPath := databasePath + suffix
		_, err := os.Stat(sidecarPath)
		switch {
		case errors.Is(err, os.ErrNotExist):
			continue
		case err != nil:
			t.Fatalf("inspect SQLite sidecar %q: %v", sidecarPath, err)
		default:
			t.Fatalf("unexpected SQLite sidecar %q", sidecarPath)
		}
	}
}
