package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
)

func TestLoadBoundProjectRootUsesRelocationLineageHead(t *testing.T) {
	const projectID = "qnt_f17e0060"
	previousRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(previousRoot, ".haft"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := []byte("id: " + projectID + "\nname: relocated-current-binding\n")
	if err := os.WriteFile(
		filepath.Join(previousRoot, ".haft", "project.yaml"),
		config,
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	databasePath := filepath.Join(
		home,
		".haft",
		"projects",
		projectID,
		"haft.db",
	)
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := kerneldbfixture.OpenCurrentStore(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	boundAt := time.Now().UTC()
	if err := projectledger.BindInitialized(
		context.Background(),
		previousRoot,
		boundAt,
	); err != nil {
		t.Fatal(err)
	}

	currentRoot := previousRoot + "-moved"
	if err := os.Rename(previousRoot, currentRoot); err != nil {
		t.Fatal(err)
	}
	handle, err := projectledger.OpenForExplicitMigration(
		context.Background(),
		currentRoot,
		projectledger.ReadWrite,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handle.RelocateRoot(
		context.Background(),
		previousRoot,
		boundAt.Add(time.Minute),
	); err != nil {
		_ = handle.Close()
		t.Fatal(err)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}

	transaction, err := sqlitetransaction.BeginRead(
		context.Background(),
		store.GetRawDB(),
	)
	if err != nil {
		t.Fatal(err)
	}
	root, loadErr := loadBoundProjectRootTx(
		context.Background(),
		transaction,
		projectID,
	)
	finish := transaction.Rollback(context.Background())
	if loadErr != nil || finish.Err() != nil {
		t.Fatalf("load relocated project root: load=%v finish=%v", loadErr, finish.Err())
	}
	if root.String() != currentRoot {
		t.Fatalf("bound project root = %q, want %q", root.String(), currentRoot)
	}
}
