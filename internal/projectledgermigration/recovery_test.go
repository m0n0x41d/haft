package projectledgermigration

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

func TestRecoverMissingBindingBacksUpAndBindsCurrentLedger(t *testing.T) {
	fixture := newCurrentUnboundRecoveryFixture(t)
	request, err := NewRequest(fixture.root, fixture.config.ID)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	boundAt := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)

	result, err := RecoverMissingBinding(
		context.Background(),
		request,
		boundAt,
	)
	if err != nil {
		t.Fatalf("RecoverMissingBinding: %v", err)
	}
	if result.Outcome != bindingRecoveryOutcome ||
		result.ProjectRoot != fixture.root ||
		result.ProjectID != fixture.config.ID ||
		result.DatabasePath != fixture.databasePath ||
		result.BackupPath == "" ||
		!strings.HasPrefix(result.BackupDigest, "sha256:") ||
		result.BoundAt != boundAt {
		t.Fatalf("recovery result = %+v", result)
	}
	if _, err := os.Stat(result.BackupPath); err != nil {
		t.Fatalf("stat recovery backup: %v", err)
	}

	handle, err := projectledger.OpenExisting(
		context.Background(),
		fixture.root,
		projectledger.ReadOnly,
	)
	if err != nil {
		t.Fatalf("OpenExisting recovered ledger: %v", err)
	}
	if err := handle.Close(); err != nil {
		t.Fatalf("close recovered ledger: %v", err)
	}

	backup, err := sql.Open("sqlite", result.BackupPath)
	if err != nil {
		t.Fatalf("open recovery backup: %v", err)
	}
	defer backup.Close()
	var bindingCount int
	if err := backup.QueryRow(
		"SELECT COUNT(*) FROM project_ledger_binding",
	).Scan(&bindingCount); err != nil {
		t.Fatalf("count backup bindings: %v", err)
	}
	if bindingCount != 0 {
		t.Fatalf("backup binding count = %d, want 0", bindingCount)
	}
}

func TestRecoverMissingBindingRejectsAlreadyBoundLedger(t *testing.T) {
	fixture := newCurrentUnboundRecoveryFixture(t)
	if err := projectledger.BindInitialized(
		context.Background(),
		fixture.root,
		time.Now().UTC(),
	); err != nil {
		t.Fatalf("BindInitialized: %v", err)
	}
	request, err := NewRequest(fixture.root, fixture.config.ID)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	_, err = RecoverMissingBinding(
		context.Background(),
		request,
		time.Now().UTC(),
	)
	if err == nil || !strings.Contains(err.Error(), "already has") {
		t.Fatalf("already-bound recovery error = %v", err)
	}
}

func TestRecoverMissingBindingRejectsPreBindingLedger(t *testing.T) {
	fixture := newUnboundSchemaFrontierFixture(t, 36)
	request, err := NewRequest(fixture.root, fixture.config.ID)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	_, err = RecoverMissingBinding(
		context.Background(),
		request,
		time.Now().UTC(),
	)
	if err == nil || !strings.Contains(err.Error(), "requires binding-aware schema") {
		t.Fatalf("pre-binding recovery error = %v", err)
	}
}

type currentUnboundRecoveryFixture struct {
	root         string
	config       *project.Config
	databasePath string
}

func newCurrentUnboundRecoveryFixture(
	t *testing.T,
) currentUnboundRecoveryFixture {
	t.Helper()
	home := canonicalTempDir(t)
	root := canonicalTempDir(t)
	t.Setenv("HOME", home)
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatalf("create fixture .haft: %v", err)
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
		t.Fatalf("close unbound current store: %v", err)
	}
	return currentUnboundRecoveryFixture{
		root:         root,
		config:       config,
		databasePath: databasePath,
	}
}

func TestRecoverMissingBindingRejectsCancelledContext(t *testing.T) {
	fixture := newCurrentUnboundRecoveryFixture(t)
	request, err := NewRequest(fixture.root, fixture.config.ID)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = RecoverMissingBinding(ctx, request, time.Now().UTC())
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled recovery error = %v", err)
	}
}
