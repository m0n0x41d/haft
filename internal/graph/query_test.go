package graph

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	// Create minimal schema matching Haft's tables
	stmts := []string{
		`CREATE TABLE artifacts (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			title TEXT NOT NULL DEFAULT '',
			structured_data TEXT NOT NULL DEFAULT '{}'
		)`,
		`CREATE TABLE affected_files (
			artifact_id TEXT NOT NULL,
			file_path TEXT NOT NULL,
			file_hash TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE codebase_modules (
			module_id TEXT PRIMARY KEY,
			path TEXT NOT NULL,
			name TEXT NOT NULL,
			lang TEXT NOT NULL DEFAULT 'go',
			file_count INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE module_dependencies (
			source_module TEXT NOT NULL,
			target_module TEXT NOT NULL,
			dep_type TEXT NOT NULL DEFAULT 'import'
		)`,
		`CREATE INDEX idx_af_path ON affected_files(file_path)`,
		`CREATE INDEX idx_af_artifact ON affected_files(artifact_id)`,
		`CREATE INDEX idx_md_source ON module_dependencies(source_module)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}

	return db
}

func seedDecision(t *testing.T, db *sql.DB, id, title string, invariants []string, files []string) {
	t.Helper()

	sd, _ := json.Marshal(struct {
		Invariants []string `json:"invariants"`
	}{Invariants: invariants})

	_, err := db.Exec(`INSERT INTO artifacts (id, kind, status, title, structured_data) VALUES (?, 'DecisionRecord', 'active', ?, ?)`,
		id, title, string(sd))
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range files {
		_, err := db.Exec(`INSERT INTO affected_files (artifact_id, file_path) VALUES (?, ?)`, id, f)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func seedModule(t *testing.T, db *sql.DB, id, path, name string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO codebase_modules (module_id, path, name) VALUES (?, ?, ?)`, id, path, name)
	if err != nil {
		t.Fatal(err)
	}
}

func seedDep(t *testing.T, db *sql.DB, source, target string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO module_dependencies (source_module, target_module) VALUES (?, ?)`, source, target)
	if err != nil {
		t.Fatal(err)
	}
}

func TestFindDecisionsForFile_Direct(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedDecision(t, db, "dec-001", "Use Redis for caching",
		[]string{"Cache layer must not access DB directly"},
		[]string{"internal/cache/redis.go", "internal/cache/store.go"})

	decisions, err := store.FindDecisionsForFile(ctx, "internal/cache/redis.go")
	if err != nil {
		t.Fatal(err)
	}

	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].ID != "dec-001" {
		t.Fatalf("expected dec-001, got %s", decisions[0].ID)
	}
}

func TestFindDecisionsForFile_NoMatch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedDecision(t, db, "dec-001", "Use Redis",
		nil,
		[]string{"internal/cache/redis.go"})

	decisions, err := store.FindDecisionsForFile(ctx, "internal/auth/handler.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 0 {
		t.Fatalf("expected 0 decisions, got %d", len(decisions))
	}
}

func TestFindInvariantsForFile(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedDecision(t, db, "dec-001", "Cache architecture",
		[]string{"No direct DB access from cache layer", "All cache keys must have TTL"},
		[]string{"internal/cache/redis.go"})

	seedDecision(t, db, "dec-002", "Error handling",
		[]string{"All public functions return error"},
		[]string{"internal/cache/redis.go", "internal/api/handler.go"})

	invariants, err := store.FindInvariantsForFile(ctx, "internal/cache/redis.go")
	if err != nil {
		t.Fatal(err)
	}

	if len(invariants) != 3 {
		t.Fatalf("expected 3 invariants, got %d", len(invariants))
	}

	// Check that invariants come from both decisions
	decIDs := map[string]bool{}
	for _, inv := range invariants {
		decIDs[inv.DecisionID] = true
	}
	if !decIDs["dec-001"] || !decIDs["dec-002"] {
		t.Fatalf("expected invariants from both decisions, got %v", decIDs)
	}
}

func TestFindModuleForFile(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedModule(t, db, "mod-cache", "internal/cache", "cache")
	seedModule(t, db, "mod-auth", "internal/auth", "auth")
	seedModule(t, db, "mod-internal", "internal", "internal")

	// Should match the LONGEST prefix
	module, err := store.FindModuleForFile(ctx, "internal/cache/redis.go")
	if err != nil {
		t.Fatal(err)
	}
	if module == nil {
		t.Fatal("expected module, got nil")
	}
	if module.ID != "mod-cache" {
		t.Fatalf("expected mod-cache, got %s", module.ID)
	}

	// File in auth module
	module, err = store.FindModuleForFile(ctx, "internal/auth/handler.go")
	if err != nil {
		t.Fatal(err)
	}
	if module == nil || module.ID != "mod-auth" {
		t.Fatalf("expected mod-auth, got %v", module)
	}

	// File not in any module
	module, err = store.FindModuleForFile(ctx, "cmd/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if module != nil {
		t.Fatalf("expected nil, got %v", module)
	}
}

func TestTransitiveDependents(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedModule(t, db, "mod-core", "internal/core", "core")
	seedModule(t, db, "mod-cache", "internal/cache", "cache")
	seedModule(t, db, "mod-api", "internal/api", "api")
	seedModule(t, db, "mod-web", "internal/web", "web")

	// web imports api, api imports cache, cache imports core
	seedDep(t, db, "mod-cache", "mod-core")
	seedDep(t, db, "mod-api", "mod-cache")
	seedDep(t, db, "mod-web", "mod-api")

	deps, err := store.TransitiveDependents(ctx, "mod-core")
	if err != nil {
		t.Fatal(err)
	}

	if len(deps) != 3 {
		t.Fatalf("expected 3 transitive dependents, got %d: %v", len(deps), deps)
	}

	// Should be ordered by depth
	paths := make([]string, len(deps))
	for i, d := range deps {
		paths[i] = d.Path
	}
	if paths[0] != "internal/cache" {
		t.Fatalf("expected internal/cache first (depth 1), got %s", paths[0])
	}
}

func TestTransitiveDependents_CycleSafe(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedModule(t, db, "mod-a", "pkg/a", "a")
	seedModule(t, db, "mod-b", "pkg/b", "b")
	seedModule(t, db, "mod-c", "pkg/c", "c")

	// Circular imports: a -> b -> c -> a
	seedDep(t, db, "mod-a", "mod-b")
	seedDep(t, db, "mod-b", "mod-c")
	seedDep(t, db, "mod-c", "mod-a")

	deps, err := store.TransitiveDependents(ctx, "mod-a")
	if err != nil {
		t.Fatal(err)
	}

	// Dependents of a are c directly and b transitively.
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps in cycle, got %d", len(deps))
	}
}

func TestFindDecisionsForModule(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedModule(t, db, "mod-cache", "internal/cache", "cache")
	seedDecision(t, db, "dec-001", "Cache architecture",
		[]string{"No direct DB access"},
		[]string{"internal/cache/redis.go", "internal/cache/store.go"})
	seedDecision(t, db, "dec-002", "API design",
		nil,
		[]string{"internal/api/handler.go"})

	decisions, err := store.FindDecisionsForModule(ctx, "mod-cache")
	if err != nil {
		t.Fatal(err)
	}

	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision for cache module, got %d", len(decisions))
	}
	if decisions[0].ID != "dec-001" {
		t.Fatalf("expected dec-001, got %s", decisions[0].ID)
	}
}
