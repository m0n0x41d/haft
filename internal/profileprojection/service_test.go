package profileprojection

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
)

func TestRebuildReturnsAutoOnlyWhenLedgerAndProjectionAreBothAbsent(t *testing.T) {
	rootPath := t.TempDir()
	haftPath := filepath.Join(rootPath, ".haft")
	if err := os.Mkdir(haftPath, 0o755); err != nil {
		t.Fatalf("create .haft: %v", err)
	}
	store, err := kerneldbfixture.OpenCurrentStore(
		filepath.Join(haftPath, "haft.db"),
	)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service, err := NewService(store.GetRawDB())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	root := mustProjectRoot(t, rootPath)
	result, err := service.Rebuild(context.Background(), root)
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if result.Kind() != ResultAuto {
		t.Fatalf("result kind = %q, want Auto", result.Kind())
	}
	projectionPath, err := ProjectionPath(root)
	if err != nil {
		t.Fatalf("ProjectionPath: %v", err)
	}
	if err := os.WriteFile(projectionPath, []byte("legacy: true\n"), 0o644); err != nil {
		t.Fatalf("write orphan projection: %v", err)
	}
	result, err = service.Rebuild(context.Background(), root)
	if err != nil {
		t.Fatalf("Rebuild orphan: %v", err)
	}
	if result.Kind() != ResultProjectionWithoutLedger {
		t.Fatalf("orphan result kind = %q", result.Kind())
	}
	if result.DiagnosticCode() != "projection_without_ledger" {
		t.Fatalf("orphan diagnostic = %q", result.DiagnosticCode())
	}
}

func TestZeroServiceAndNilContextFailClosed(t *testing.T) {
	root := mustProjectRoot(t, t.TempDir())
	if _, err := (Service{}).Rebuild(context.Background(), root); err == nil {
		t.Fatal("zero service rebuilt a profile projection")
	}
	storePath := filepath.Join(t.TempDir(), "haft.db")
	store, err := kerneldbfixture.OpenCurrentStore(storePath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service, err := NewService(store.GetRawDB())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if _, err := service.Rebuild(nil, root); err == nil {
		t.Fatal("nil context rebuilt a profile projection")
	}
}
