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

// EnsureIndex must build on an empty index, skip on an unchanged tree, and
// rebuild once the source changes — the freshness contract that removes the
// manual-rebuild step after a code change.
func TestEnsureIndex_RebuildsOnStaleness(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, "cg.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE codebase_modules (
		module_id TEXT PRIMARY KEY,
		path TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		lang TEXT,
		file_count INTEGER DEFAULT 0,
		last_scanned TEXT NOT NULL
	)`); err != nil {
		t.Fatal(err)
	}

	// indexStale + EnsureIndex's build path only touch scanner/symbols/edges.
	svc := &Service{
		scanner: codebase.NewScanner(db),
		symbols: codebase.NewSymbolStore(db),
		edges:   codebase.NewEdgeStore(db),
	}
	pkg := filepath.Join(root, "pkg")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(body string) {
		if err := os.WriteFile(filepath.Join(pkg, "a.go"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("package a\nfunc A() { B() }\nfunc B() {}\n")
	built, err := svc.EnsureIndex(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !built {
		t.Fatal("first EnsureIndex must build (empty index)")
	}

	built, err = svc.EnsureIndex(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if built {
		t.Fatal("unchanged tree must NOT rebuild")
	}

	// Source change (different size) → detected stale → rebuilt.
	write("package a\nfunc A() { B(); C() }\nfunc B() {}\nfunc C() {}\n")
	built, err = svc.EnsureIndex(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !built {
		t.Fatal("a source change must trigger a rebuild")
	}
}
