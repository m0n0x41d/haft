package projectledgermigration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

func TestRelocateRootUpgradesSchema57PublishesBackupAndPreservesGenesisBinding(
	t *testing.T,
) {
	fixture, _ := newSchema57ServeFixture(t, false)
	previousRoot := fixture.root
	currentRoot := previousRoot + "-renamed"
	if err := os.Rename(previousRoot, currentRoot); err != nil {
		t.Fatalf("rename project root: %v", err)
	}
	request, err := NewRootRelocationRequest(
		previousRoot,
		currentRoot,
		fixture.config.ID,
	)
	if err != nil {
		t.Fatalf("NewRootRelocationRequest: %v", err)
	}
	databasePath, err := fixture.config.DBPath()
	if err != nil {
		t.Fatal(err)
	}
	beforeActivation := digestFileForTest(t, databasePath)
	activation, activationErr := EnsureCurrentForServe(
		context.Background(),
		request.request,
		time.Date(2026, time.September, 4, 10, 29, 0, 0, time.UTC),
	)
	if activationErr == nil || activation.Blocker != ServeBlockerRootRelocation {
		t.Fatalf(
			"moved-root serve activation = %#v / %v",
			activation,
			activationErr,
		)
	}
	if afterActivation := digestFileForTest(t, databasePath); afterActivation != beforeActivation {
		t.Fatal("moved-root serve activation changed the project ledger")
	}
	at := time.Date(2026, time.September, 4, 10, 30, 0, 0, time.UTC)
	result, err := RelocateRoot(context.Background(), request, at)
	if err != nil {
		t.Fatalf("RelocateRoot: %v", err)
	}
	currentSchema, err := db.CurrentSchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != rootRelocationOutcome ||
		result.ProjectID != fixture.config.ID ||
		result.FromRoot != previousRoot ||
		result.ToRoot != currentRoot ||
		result.BeforeSchema != 57 ||
		result.AfterSchema != currentSchema ||
		result.RelocationNumber != 1 ||
		result.RelocationDigest == "" ||
		result.BackupPath == "" ||
		result.BackupDigest == "" {
		t.Fatalf("relocation result = %#v", result)
	}
	if digestFileForTest(t, result.BackupPath) != result.BackupDigest {
		t.Fatalf("relocation backup digest changed: %#v", result)
	}
	info, err := os.Lstat(result.BackupPath)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("relocation backup mode = %v", info.Mode())
	}

	handle, err := projectledger.OpenExisting(
		context.Background(),
		currentRoot,
		projectledger.ReadOnly,
	)
	if err != nil {
		t.Fatalf("OpenExisting relocated root: %v", err)
	}
	defer handle.Close()
	state, err := handle.InspectPersistedRootState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.GenesisRoot != previousRoot ||
		state.CurrentRoot != currentRoot ||
		state.RelocationCount != 1 {
		t.Fatalf("relocated root state = %#v", state)
	}
	futureSnapshot := filepath.Join(t.TempDir(), "haft.db")
	// #nosec G202 -- VACUUM INTO does not accept a bind parameter and the
	// shared helper escapes this test-owned path as one SQL string literal.
	if _, err := handle.Database().Exec(
		"VACUUM INTO " + sqliteStringLiteral(futureSnapshot),
	); err != nil {
		t.Fatalf("create future migration snapshot: %v", err)
	}
	if err := os.Chmod(futureSnapshot, 0o600); err != nil {
		t.Fatalf("secure future migration snapshot: %v", err)
	}
	if err := verifyServeMigrationSnapshot(
		context.Background(),
		futureSnapshot,
		request.request,
		currentSchema,
		nil,
	); err != nil {
		t.Fatalf("future migration snapshot at relocated root: %v", err)
	}

	_, err = RelocateRoot(context.Background(), request, at.Add(time.Second))
	if err == nil || !strings.Contains(err.Error(), "not declared predecessor") {
		t.Fatalf("replayed relocation error = %v", err)
	}
}

func TestRelocateRootRejectsLivePredecessorBeforeCreatingBackup(t *testing.T) {
	fixture := newCurrentProjectFixture(t)
	currentRoot := canonicalTempDir(t)
	currentHaftDir := filepath.Join(currentRoot, ".haft")
	if err := os.MkdirAll(currentHaftDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configBytes, err := os.ReadFile(
		filepath.Join(fixture.root, ".haft", "project.yaml"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(currentHaftDir, "project.yaml"),
		configBytes,
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	request, err := NewRootRelocationRequest(
		fixture.root,
		currentRoot,
		fixture.config.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	databasePath, err := fixture.config.DBPath()
	if err != nil {
		t.Fatal(err)
	}
	before := digestFileForTest(t, databasePath)
	_, err = RelocateRoot(
		context.Background(),
		request,
		time.Date(2026, time.September, 4, 11, 0, 0, 0, time.UTC),
	)
	if err == nil || !strings.Contains(err.Error(), "two live roots") {
		t.Fatalf("live-predecessor relocation error = %v", err)
	}
	if after := digestFileForTest(t, databasePath); after != before {
		t.Fatal("rejected relocation changed the project ledger")
	}
	matches, err := filepath.Glob(databasePath + ".pre-root-relocation-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("rejected relocation created backup artifacts: %v", matches)
	}
}
