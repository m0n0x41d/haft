package codebase

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

const sampleGo = `package sample

type Foo struct{}
type Bar struct{}

func (f *Foo) Do() string { return "foo" }

func (b *Bar) Do() int { return 42 }

func Free() {}
`

func newSymbolStore(t *testing.T) (*SymbolStore, string) {
	t.Helper()
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, "cg.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	st := NewSymbolStore(db)
	if err := st.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	return st, root
}

// TestScanSymbols_PopulatesAcrossFiles checks the bulk-walk indexer reaches
// supported source files across directories.
func TestScanSymbols_PopulatesAcrossFiles(t *testing.T) {
	st, root := newSymbolStore(t)
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\nfunc Alpha() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "b.go"), []byte("package sub\nfunc Beta() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := NewScanner(st.db).ScanSymbols(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if n < 2 {
		t.Fatalf("expected >=2 files indexed, got %d", n)
	}
	if alpha, err := st.GetByName(ctx, "Alpha"); err != nil || len(alpha) != 1 {
		t.Fatalf("Alpha not indexed: %v / %v", alpha, err)
	}
	if beta, err := st.GetByName(ctx, "Beta"); err != nil || len(beta) != 1 {
		t.Fatalf("Beta (nested dir) not indexed: %v / %v", beta, err)
	}
}

func TestSymbolStore_GetByDirIncludesRootPackageFiles(t *testing.T) {
	st, root := newSymbolStore(t)
	ctx := context.Background()
	files := map[string]string{
		"a.go":     "package p\nfunc Alpha() {}\n",
		"b.go":     "package p\nfunc Beta() {}\n",
		"sub/c.go": "package sub\nfunc Gamma() {}\n",
	}
	for name, src := range files {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := st.IndexFileSymbols(ctx, root, name); err != nil {
			t.Fatal(err)
		}
	}

	syms, err := st.GetByDir(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, sym := range syms {
		got[sym.Name] = true
	}
	if !got["Alpha"] || !got["Beta"] {
		t.Fatalf("expected root package symbols Alpha and Beta, got %+v", syms)
	}
	if got["Gamma"] {
		t.Fatalf("expected nested package symbol Gamma to be excluded, got %+v", syms)
	}
}

func TestSymbolStoreGetByDirTreatsPercentAndUnderscoreLiterally(
	t *testing.T,
) {
	store, root := newSymbolStore(t)
	ctx := context.Background()
	files := map[string]string{
		"pkg/a_%/literal.go": "package literal\nfunc Literal() {}\n",
		"pkg/abx/sibling.go": "package sibling\nfunc Sibling() {}\n",
		"pkg/a_X/other.go":   "package other\nfunc Other() {}\n",
	}
	for name, source := range files {
		fullPath := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(
			filepath.Dir(fullPath),
			0o755,
		); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			fullPath,
			[]byte(source),
			0o644,
		); err != nil {
			t.Fatal(err)
		}
		if err := store.IndexFileSymbols(
			ctx,
			root,
			name,
		); err != nil {
			t.Fatal(err)
		}
	}

	symbols, err := store.GetByDir(ctx, `pkg\a_%`)
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 1 ||
		symbols[0].Name != "Literal" ||
		symbols[0].FilePath != "pkg/a_%/literal.go" {
		t.Fatalf("literal directory lookup = %+v", symbols)
	}
}

func TestSymbolStore_AnchorSurvivesLineShift(t *testing.T) {
	st, root := newSymbolStore(t)
	ctx := context.Background()
	rel := "service.ts"
	initial := "export function run(input: string): string {\n  return input\n}\n"
	if err := os.WriteFile(filepath.Join(root, rel), []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := st.IndexFileSymbols(ctx, root, rel); err != nil {
		t.Fatal(err)
	}
	first, err := st.GetByName(ctx, "run")
	if err != nil || len(first) != 1 {
		t.Fatalf("first index = %+v / %v", first, err)
	}
	if first[0].AnchorID == "" || first[0].ID != first[0].AnchorID || first[0].AnchorVersion != SymbolAnchorVersion {
		t.Fatalf("first symbol lacks canonical anchor: %+v", first[0])
	}

	shifted := "// inserted header\n// second line\n" + initial
	if err := os.WriteFile(filepath.Join(root, rel), []byte(shifted), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := st.IndexFileSymbols(ctx, root, rel); err != nil {
		t.Fatal(err)
	}
	second, err := st.GetByName(ctx, "run")
	if err != nil || len(second) != 1 {
		t.Fatalf("second index = %+v / %v", second, err)
	}
	if first[0].AnchorID != second[0].AnchorID {
		t.Fatalf("line shift changed anchor: %s != %s", first[0].AnchorID, second[0].AnchorID)
	}
	if first[0].StartLine == second[0].StartLine {
		t.Fatalf("test did not shift source coordinates: %d == %d", first[0].StartLine, second[0].StartLine)
	}
}

func TestSymbolStore_EnsureSchemaUpgradesLegacyDerivedTable(t *testing.T) {
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	_, err = database.Exec(`CREATE TABLE code_symbols (
		id TEXT PRIMARY KEY,
		file_path TEXT NOT NULL,
		name TEXT NOT NULL,
		kind TEXT,
		receiver TEXT,
		start_line INTEGER NOT NULL,
		end_line INTEGER,
		start_byte INTEGER,
		end_byte INTEGER,
		hash TEXT,
		exported INTEGER DEFAULT 0,
		lang TEXT,
		UNIQUE (file_path, name, start_line)
	)`)
	if err != nil {
		t.Fatal(err)
	}
	store := NewSymbolStore(database)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"anchor_id", "anchor_version", "qualified_name", "signature_hash"} {
		if !codeSymbolColumnExists(t, database, column) {
			t.Fatalf("legacy schema missing upgraded column %q", column)
		}
	}
}

func codeSymbolColumnExists(t *testing.T, database *sql.DB, wanted string) bool {
	t.Helper()
	rows, err := database.Query(`PRAGMA table_info(code_symbols)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		if name == wanted {
			return true
		}
	}
	return false
}

// TestSymbolStore_OverloadNodesAndByteExactSlice is the P0 gate for the node
// store: two same-name methods persist as DISTINCT nodes, and each node's
// stored byte offsets slice the file back to its EXACT body.
func TestSymbolStore_OverloadNodesAndByteExactSlice(t *testing.T) {
	st, root := newSymbolStore(t)
	ctx := context.Background()
	rel := "sample.go"
	if err := os.WriteFile(filepath.Join(root, rel), []byte(sampleGo), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := st.IndexFileSymbols(ctx, root, rel); err != nil {
		t.Fatal(err)
	}

	// GATE 2: two overloaded methods → two distinct nodes.
	dos, err := st.GetByName(ctx, "Do")
	if err != nil {
		t.Fatal(err)
	}
	if len(dos) != 2 {
		t.Fatalf("expected 2 distinct 'Do' nodes (overloads), got %d: %+v", len(dos), dos)
	}
	if dos[0].StartLine == dos[1].StartLine {
		t.Fatalf("overload nodes must differ by start_line (identity), got same: %+v", dos)
	}
	recv := map[string]bool{dos[0].Receiver: true, dos[1].Receiver: true}
	if !recv["Foo"] || !recv["Bar"] {
		t.Fatalf("expected receivers Foo + Bar, got %v", recv)
	}

	// GATE 1: byte-exact slice round-trip.
	content, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	wantByReceiver := map[string]string{
		"Foo": `func (f *Foo) Do() string { return "foo" }`,
		"Bar": `func (b *Bar) Do() int { return 42 }`,
	}
	for _, sym := range dos {
		body, ok := SliceBody(content, sym)
		if !ok {
			t.Fatalf("SliceBody failed for %s.Do", sym.Receiver)
		}
		if string(body) != wantByReceiver[sym.Receiver] {
			t.Fatalf("byte-exact mismatch for %s.Do:\n got: %q\nwant: %q", sym.Receiver, string(body), wantByReceiver[sym.Receiver])
		}
	}

	// Freshness: fresh right after index; stale after an edit.
	stale, err := st.FileSymbolsStale(ctx, root, rel)
	if err != nil {
		t.Fatal(err)
	}
	if stale {
		t.Fatal("file should not be stale immediately after indexing")
	}
	edited := sampleGo + "\nfunc Added() {}\n"
	if err := os.WriteFile(filepath.Join(root, rel), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	stale, err = st.FileSymbolsStale(ctx, root, rel)
	if err != nil {
		t.Fatal(err)
	}
	if !stale {
		t.Fatal("file should be stale after adding a symbol")
	}
	// rebuild-on-demand restores freshness.
	if err := st.IndexFileSymbols(ctx, root, rel); err != nil {
		t.Fatal(err)
	}
	stale, err = st.FileSymbolsStale(ctx, root, rel)
	if err != nil {
		t.Fatal(err)
	}
	if stale {
		t.Fatal("file should be fresh after re-indexing")
	}
}
