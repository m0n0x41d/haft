package contextgraph

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/graph"

	_ "modernc.org/sqlite"
)

// --- pure core ---

func TestBuildCodeContext_GroupsByKind(t *testing.T) {
	mk := func(id string, k artifact.Kind) *artifact.Artifact {
		return &artifact.Artifact{Meta: artifact.Meta{ID: id, Kind: k, Status: artifact.StatusActive, Title: id}}
	}
	linked := []*artifact.Artifact{
		mk("dec-1", artifact.KindDecisionRecord),
		mk("prob-1", artifact.KindProblemCard),
		mk("sol-1", artifact.KindSolutionPortfolio),
		mk("note-1", artifact.KindNote),
		mk("dec-2", artifact.KindDecisionRecord),
	}
	invs := []graph.Invariant{{Text: "p99 < 50ms", DecisionTitle: "JWT"}}

	cc := BuildCodeContext(Target{File: "a/b.go"}, linked, invs, "a", true)

	if len(cc.Decisions) != 2 || len(cc.Problems) != 1 || len(cc.Portfolios) != 1 || len(cc.Notes) != 1 {
		t.Fatalf("grouping wrong: dec=%d prob=%d sol=%d note=%d", len(cc.Decisions), len(cc.Problems), len(cc.Portfolios), len(cc.Notes))
	}
	if !cc.Governed || cc.Module != "a" || len(cc.Invariants) != 1 {
		t.Fatalf("module/governed/invariants wrong: %+v", cc)
	}
	if cc.Empty() {
		t.Fatal("Empty() must be false when artifacts present")
	}
}

func TestBuildCodeContext_EmptyWhenNothing(t *testing.T) {
	cc := BuildCodeContext(Target{File: "a/b.go"}, nil, nil, "", false)
	if !cc.Empty() {
		t.Fatalf("Empty() must be true for an untouched location, got %+v", cc)
	}
}

// --- shell (real SQLite) ---

func setupContextDB(t *testing.T) (*artifact.Store, *graph.Store, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", t.TempDir()+"/ctx.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	stmts := []string{
		`CREATE TABLE artifacts (
			id TEXT PRIMARY KEY, kind TEXT NOT NULL, version INTEGER NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'active', context TEXT, mode TEXT,
			title TEXT NOT NULL, content TEXT NOT NULL, file_path TEXT,
			valid_until TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			search_keywords TEXT DEFAULT '', structured_data TEXT DEFAULT '')`,
		`CREATE TABLE affected_files (
			artifact_id TEXT NOT NULL, file_path TEXT NOT NULL, file_hash TEXT,
			PRIMARY KEY (artifact_id, file_path))`,
		`CREATE TABLE affected_symbols (
			artifact_id TEXT NOT NULL, file_path TEXT NOT NULL, symbol_name TEXT NOT NULL,
			symbol_kind TEXT, symbol_line INTEGER, symbol_end_line INTEGER, symbol_hash TEXT,
			PRIMARY KEY (artifact_id, file_path, symbol_name))`,
		`CREATE TABLE artifact_symbol_bindings (
			artifact_id TEXT NOT NULL, anchor_id TEXT NOT NULL, anchor_version INTEGER NOT NULL,
			file_path TEXT NOT NULL, language TEXT NOT NULL, symbol_name TEXT NOT NULL,
			symbol_kind TEXT NOT NULL, receiver TEXT NOT NULL DEFAULT '', qualified_name TEXT NOT NULL,
			signature_hash TEXT NOT NULL, symbol_line INTEGER NOT NULL DEFAULT 0,
			symbol_end_line INTEGER NOT NULL DEFAULT 0, body_hash TEXT NOT NULL DEFAULT '',
			binding_status TEXT NOT NULL DEFAULT 'active', resolution_source TEXT NOT NULL DEFAULT '',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (artifact_id, anchor_id))`,
		`CREATE TABLE codebase_modules (
			module_id TEXT PRIMARY KEY, path TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL, lang TEXT, file_count INTEGER DEFAULT 0, last_scanned TEXT NOT NULL)`,
		`CREATE TABLE code_files (
			file_path TEXT PRIMARY KEY
		)`,
		`CREATE TABLE module_dependencies (
			source_module TEXT NOT NULL, target_module TEXT NOT NULL,
			dep_type TEXT NOT NULL DEFAULT 'import', file_path TEXT, last_scanned TEXT NOT NULL,
			PRIMARY KEY (source_module, target_module, dep_type))`,
		`CREATE VIRTUAL TABLE artifacts_fts USING fts5(id, title, content, kind, search_keywords, tokenize='porter unicode61')`,
		`CREATE TRIGGER artifacts_fts_insert AFTER INSERT ON artifacts BEGIN
			INSERT INTO artifacts_fts(id, title, content, kind, search_keywords) VALUES (new.id, new.title, new.content, new.kind, new.search_keywords);
		END`,
		`CREATE TRIGGER artifacts_fts_update AFTER UPDATE ON artifacts BEGIN
			DELETE FROM artifacts_fts WHERE id = old.id;
			INSERT INTO artifacts_fts(id, title, content, kind, search_keywords) VALUES (new.id, new.title, new.content, new.kind, new.search_keywords);
		END`,
		`CREATE TRIGGER artifacts_fts_delete AFTER DELETE ON artifacts BEGIN
			DELETE FROM artifacts_fts WHERE id = old.id;
		END`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(context.Background(), s); err != nil {
			t.Fatalf("schema: %v\n%s", err, s)
		}
	}
	return artifact.NewStore(db), graph.NewStore(db), db
}

func TestFetchCodeContext_FileAndSymbol(t *testing.T) {
	store, g, db := setupContextDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	mk := func(id string, k artifact.Kind, title string) {
		a := &artifact.Artifact{Meta: artifact.Meta{ID: id, Kind: k, Status: artifact.StatusActive, Title: title, CreatedAt: now, UpdatedAt: now}, Body: "x"}
		if err := store.Create(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	mk("dec-login", artifact.KindDecisionRecord, "JWT over sessions")
	mk("prob-login", artifact.KindProblemCard, "rate-limit login")
	mk("sol-login", artifact.KindSolutionPortfolio, "auth variants")

	file := "internal/auth/login.go"
	// All three touch the file; the decision additionally targets the Login symbol.
	for _, id := range []string{"dec-login", "prob-login", "sol-login"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO affected_files (artifact_id, file_path) VALUES (?, ?)`, id, file); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO affected_symbols (artifact_id, file_path, symbol_name, symbol_kind) VALUES (?, ?, ?, ?)`,
		"dec-login", file, "Login", "func"); err != nil {
		t.Fatal(err)
	}

	cc, err := FetchCodeContext(ctx, store, g, Target{File: file})
	if err != nil {
		t.Fatal(err)
	}
	if len(cc.Decisions) != 0 ||
		len(cc.AffectedPathContextDecisions) != 1 ||
		len(cc.Problems) != 1 ||
		len(cc.Portfolios) != 1 {
		t.Fatalf("file-level grouping wrong: %+v", cc)
	}
	if cc.Empty() {
		t.Fatal("expected non-empty context for a governed file")
	}

	// Symbol-scoped: union of file + symbol, deduped — still the same 3, decision present.
	ccSym, err := FetchCodeContext(ctx, store, g, Target{File: file, Symbol: "Login"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ccSym.Decisions) != 0 ||
		len(ccSym.AffectedPathContextDecisions) != 1 ||
		ccSym.AffectedPathContextDecisions[0].Meta.ID != "dec-login" {
		t.Fatalf("symbol-scoped context classification wrong: %+v", ccSym)
	}
	total := len(ccSym.AffectedPathContextDecisions) +
		len(ccSym.Problems) +
		len(ccSym.Portfolios)
	if total != 3 {
		t.Fatalf("symbol union should dedupe to 3 artifacts, got %d", total)
	}
}

func TestFetchCodeContextPrefersAuthorityBearingSymbolAnchor(t *testing.T) {
	store, g, db := setupContextDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	decision := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:        "dec-anchor",
			Kind:      artifact.KindDecisionRecord,
			Status:    artifact.StatusActive,
			Title:     "Anchor precise decision",
			CreatedAt: now,
			UpdatedAt: now,
		},
		Body: "x",
	}
	if err := store.Create(ctx, decision); err != nil {
		t.Fatal(err)
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO artifact_symbol_bindings (
			artifact_id, anchor_id, anchor_version, file_path, language, symbol_name, symbol_kind,
			qualified_name, signature_hash, binding_status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"dec-anchor",
		"sym:v2:run",
		2,
		"src/app.ts",
		"typescript",
		"run",
		"func",
		"run",
		"signature",
		artifact.SymbolBindingActive,
	)
	if err != nil {
		t.Fatal(err)
	}

	cc, err := FetchCodeContext(ctx, store, g, Target{
		File:     "src/app.ts",
		Symbol:   "run",
		AnchorID: "sym:v2:run",
		Line:     200,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cc.Decisions) != 1 || cc.Decisions[0].Meta.ID != "dec-anchor" {
		t.Fatalf("anchor-linked decisions = %+v", cc.Decisions)
	}
	if cc.SymbolGranularity != "anchor-precise" {
		t.Fatalf("symbol granularity = %q", cc.SymbolGranularity)
	}
}
