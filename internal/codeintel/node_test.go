package codeintel

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/codebase"

	_ "modernc.org/sqlite"
)

func TestMethodsOfAndContainerKind(t *testing.T) {
	syms := []codebase.CodeSymbol{
		{Name: "Get", Receiver: "Store"},
		{Name: "Set", Receiver: "Store"},
		{Name: "Free", Receiver: ""},
		{Name: "Get", Receiver: "Cache"},
	}
	got := methodsOf(syms, "Store")
	if len(got) != 2 {
		t.Fatalf("Store has 2 methods, got %d", len(got))
	}
	for _, k := range []string{"type", "type_alias", "interface", "class", "struct", "enum"} {
		if !isContainerKind(k) {
			t.Fatalf("%q should be a container kind", k)
		}
	}
	if isContainerKind("func") || isContainerKind("method") {
		t.Fatalf("callables are not container kinds")
	}
}

// The P3 keystone, end-to-end: a file edited after indexing must yield a
// byte-exact body from the NEW content (re-indexed), never the stale offsets'
// bytes — proving re-read + re-hash, while refusing a private per-file write
// outside the atomic index epoch transaction.
func TestFreshBody_RejectsEditedFileUntilAtomicRefresh(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, "cg.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := codebase.NewSymbolStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	svc := &Service{symbols: store}

	rel := "x.go"
	v1 := "package x\n\nfunc Target() int { return 1 }\n"
	if err := os.WriteFile(filepath.Join(root, rel), []byte(v1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.IndexFileSymbols(ctx, root, rel); err != nil {
		t.Fatal(err)
	}
	syms, _ := store.GetByName(ctx, "Target")
	if len(syms) != 1 {
		t.Fatalf("expected 1 Target, got %d", len(syms))
	}
	stale := syms[0]

	// Edit: prepend lines (shifts the symbol down) AND change the body.
	v2 := "package x\n\n// inserted\n// lines\nfunc Target() int { return 42 }\n"
	if err := os.WriteFile(filepath.Join(root, rel), []byte(v2), 0o644); err != nil {
		t.Fatal(err)
	}

	body, fresh, reindexed, freshSym, err := svc.freshBody(ctx, root, stale)
	if err != nil {
		t.Fatal(err)
	}
	if reindexed {
		t.Fatalf("freshBody must not publish a private per-file re-index")
	}
	if fresh || body != nil {
		t.Fatalf("edited source must remain unverified until the next atomic refresh")
	}
	if freshSym.ID != stale.ID {
		t.Fatalf("freshBody must preserve the stored symbol, got %+v", freshSym)
	}
	// The stale offsets, sliced from the NEW content, would NOT verify — the
	// whole point of re-reading.
	if _, ok := codebase.VerifyBody([]byte(v2), stale); ok {
		t.Fatalf("stale offsets should fail verification against edited content")
	}
}
