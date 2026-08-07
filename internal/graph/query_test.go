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
		`CREATE TABLE code_files (
			file_path TEXT PRIMARY KEY
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
		_, err = db.Exec(`INSERT OR IGNORE INTO code_files (file_path) VALUES (?)`, f)
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
	seedModule(t, db, "mod-cache", "internal/cache", "cache")

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

func TestDirectoryAffectedPathAtModuleRootIsModuleContext(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedModule(t, db, "mod-cli", "internal/cli", "cli")
	seedDecision(
		t,
		db,
		"dec-directory",
		"CLI boundary",
		[]string{"CLI entrypoints preserve the public contract"},
		[]string{"internal/cli"},
	)

	decisions, err := store.FindDecisionsForFile(
		ctx,
		"internal/cli/memory_read_runtime.go",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 1 || decisions[0].ID != "dec-directory" {
		t.Fatalf("directory decision = %+v", decisions)
	}

	invariants, err := store.FindInvariantsForFile(
		ctx,
		"internal/cli/memory_read_runtime.go",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(invariants) != 1 || invariants[0].DecisionID != "dec-directory" {
		t.Fatalf("directory invariants = %+v", invariants)
	}
}

func TestModulePathMatchingIsSegmentSafeAndTreatsWildcardsLiterally(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedModule(t, db, "mod-cli", "internal/cli", "cli")
	seedModule(t, db, "mod-wild", "pkg/a_%", "wild")
	seedDecision(t, db, "dec-cli", "CLI", nil, []string{"internal/cli"})
	seedDecision(t, db, "dec-wild", "Wild", nil, []string{"pkg/a_%/root.go"})

	for _, target := range []string{
		"internal/client/x.go",
		"internal/cli2/x.go",
		"pkg/abx/root.go",
	} {
		decisions, err := store.FindDecisionsForFile(ctx, target)
		if err != nil {
			t.Fatal(err)
		}
		if len(decisions) != 0 {
			t.Fatalf("%s unexpectedly matched %+v", target, decisions)
		}
	}
	decisions, err := store.FindDecisionsForFile(ctx, "pkg/a_%/child.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 1 || decisions[0].ID != "dec-wild" {
		t.Fatalf("literal wildcard module = %+v", decisions)
	}
}

func TestExactGovernanceModeDoesNotWidenToSibling(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedModule(t, db, "mod-cli", "internal/cli", "cli")
	seedDecision(
		t,
		db,
		"dec-exact",
		"Exact file",
		[]string{"Only the selected file is governed"},
		[]string{"internal/cli/a.go"},
	)
	if _, err := db.Exec(
		`UPDATE artifacts
		 SET structured_data = json_set(structured_data, '$.governance_mode', 'exact')
		 WHERE id = 'dec-exact'`,
	); err != nil {
		t.Fatal(err)
	}

	direct, err := store.FindDecisionsForFile(ctx, "internal/cli/a.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(direct) != 1 || direct[0].ID != "dec-exact" {
		t.Fatalf("exact direct decision = %+v", direct)
	}
	sibling, err := store.FindDecisionsForFile(ctx, "internal/cli/b.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(sibling) != 0 {
		t.Fatalf("exact decision widened to sibling: %+v", sibling)
	}
}

func TestImplementationFootprintOnlyIsNotGovernanceAuthority(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedModule(t, db, "mod-cli", "internal/cli", "cli")
	seedDecision(
		t,
		db,
		"dec-footprint",
		"Historical touch",
		[]string{"This historical touch is not authority"},
		[]string{"internal/cli/a.go"},
	)
	if _, err := db.Exec(
		`UPDATE artifacts
		 SET structured_data = json_set(
			structured_data,
			'$.implementation_footprint',
			json('{"files":["internal/cli/a.go"]}')
		 )
		 WHERE id = 'dec-footprint'`,
	); err != nil {
		t.Fatal(err)
	}

	contextDecisions, err := store.FindAffectedPathContextDecisions(
		ctx,
		"internal/cli/a.go",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(contextDecisions) != 1 ||
		contextDecisions[0].ID != "dec-footprint" {
		t.Fatalf("affected-path context = %+v", contextDecisions)
	}
	moduleDecisions, err := store.FindModuleDecisionsForFile(
		ctx,
		"internal/cli/a.go",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(moduleDecisions) != 0 {
		t.Fatalf("footprint became module authority: %+v", moduleDecisions)
	}
	invariants, err := store.FindInvariantsForFile(
		ctx,
		"internal/cli/a.go",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(invariants) != 0 {
		t.Fatalf("footprint projected invariants: %+v", invariants)
	}
	sibling, err := store.FindDecisionsForFile(
		ctx,
		"internal/cli/b.go",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(sibling) != 0 {
		t.Fatalf("footprint widened to sibling: %+v", sibling)
	}
}

func TestCodeGovernancePathSemanticsMatrix(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedModule(t, db, "mod-root", "", "root")
	seedModule(t, db, "mod-internal", "internal", "internal")
	seedModule(t, db, "mod-cli", "internal/cli", "cli")
	seedModule(t, db, "mod-nested", "internal/cli/nested", "nested")
	seedModule(t, db, "mod-literal", "pkg/a_%", "literal")

	seedDecision(
		t,
		db,
		"dec-cli-file",
		"CLI file context",
		nil,
		[]string{`internal\cli\main.go`},
	)
	seedDecision(
		t,
		db,
		"dec-cli-dot-file",
		"CLI dot-cleaned file context",
		nil,
		[]string{"internal/cli/./main.go"},
	)
	seedDecision(
		t,
		db,
		"dec-nested",
		"Nested module",
		nil,
		[]string{"internal/cli/nested/main.go"},
	)
	seedDecision(
		t,
		db,
		"dec-non-module-dir",
		"Non-module directory",
		nil,
		[]string{"internal/cli/subdir"},
	)
	if _, err := db.Exec(
		`DELETE FROM code_files WHERE file_path = 'internal/cli/subdir'`,
	); err != nil {
		t.Fatal(err)
	}
	seedDecision(
		t,
		db,
		"dec-literal",
		"Literal wildcard",
		nil,
		[]string{"pkg/a_%/main.go"},
	)
	seedDecision(
		t,
		db,
		"dec-explicit-module",
		"Explicit module target",
		nil,
		nil,
	)
	if _, err := db.Exec(
		`UPDATE artifacts
		 SET structured_data = json(
			'{"binding_targets":[{"kind":"module","module_path":"internal/cli"}]}'
		 )
		 WHERE id = 'dec-explicit-module'`,
	); err != nil {
		t.Fatal(err)
	}
	seedDecision(
		t,
		db,
		"dec-partial",
		"Partial binding",
		nil,
		[]string{
			"internal/cli/bound.go",
			"internal/cli/unbound.go",
		},
	)
	if _, err := db.Exec(
		`UPDATE artifacts
		 SET structured_data = json(
			'{"binding_targets":[{"kind":"whole_file_fallback","file_path":"internal/cli/bound.go"}]}'
		 )
		 WHERE id = 'dec-partial'`,
	); err != nil {
		t.Fatal(err)
	}
	seedDecision(
		t,
		db,
		"dec-20260716-11f33e36",
		"Typed-memory architecture",
		nil,
		[]string{"internal/cli"},
	)
	if _, err := db.Exec(
		`UPDATE artifacts
		 SET structured_data = json(
			'{"binding_targets":[{"kind":"symbol","file_path":"db/migrations.go"}]}'
		 )
		 WHERE id = 'dec-20260716-11f33e36'`,
	); err != nil {
		t.Fatal(err)
	}
	seedDecision(
		t,
		db,
		"dec-typed-sibling-file",
		"Typed sibling file",
		nil,
		[]string{"internal/cli/sibling.go"},
	)
	if _, err := db.Exec(
		`UPDATE artifacts
		 SET structured_data = json(
			'{"binding_targets":[{"kind":"symbol","file_path":"db/migrations.go"}]}'
		 )
		 WHERE id = 'dec-typed-sibling-file'`,
	); err != nil {
		t.Fatal(err)
	}
	seedDecision(
		t,
		db,
		"dec-typed-footprint-root",
		"Typed footprint root",
		nil,
		[]string{"internal/cli"},
	)
	if _, err := db.Exec(
		`UPDATE artifacts
		 SET structured_data = json(
			'{
				"implementation_footprint":{"files":["internal/cli"]},
				"binding_targets":[
					{"kind":"symbol","file_path":"db/migrations.go"}
				]
			}'
		 )
		 WHERE id = 'dec-typed-footprint-root'`,
	); err != nil {
		t.Fatal(err)
	}
	seedDecision(
		t,
		db,
		"dec-20260713-9ed66ef0",
		"Superseded typed-memory architecture",
		nil,
		[]string{"internal/cli"},
	)
	if _, err := db.Exec(
		`UPDATE artifacts
		 SET status = 'superseded',
		     structured_data = json(
				'{"binding_targets":[{"kind":"symbol","file_path":"db/migrations.go"}]}'
		     )
		 WHERE id = 'dec-20260713-9ed66ef0'`,
	); err != nil {
		t.Fatal(err)
	}

	cliDecisions, err := store.FindModuleDecisionsForFile(
		ctx,
		"internal/cli/other.go",
	)
	if err != nil {
		t.Fatal(err)
	}
	cliIDs := make(map[string]bool)
	relevantIndex := -1
	for index, decision := range cliDecisions {
		cliIDs[decision.ID] = true
		if decision.ID == "dec-20260716-11f33e36" {
			relevantIndex = index
		}
	}
	if relevantIndex < 0 || relevantIndex >= 5 {
		t.Fatalf(
			"active typed-memory module decision index = %d, decisions = %+v",
			relevantIndex,
			cliDecisions,
		)
	}
	for _, expected := range []string{
		"dec-20260716-11f33e36",
		"dec-explicit-module",
		"dec-literal",
	} {
		if expected == "dec-literal" {
			continue
		}
		if !cliIDs[expected] {
			t.Fatalf("%s missing from CLI module context: %+v", expected, cliDecisions)
		}
	}
	for _, excluded := range []string{
		"dec-nested",
		"dec-non-module-dir",
		"dec-partial",
		"dec-20260713-9ed66ef0",
		"dec-cli-dot-file",
		"dec-cli-file",
		"dec-typed-footprint-root",
		"dec-typed-sibling-file",
	} {
		if cliIDs[excluded] {
			t.Fatalf("%s leaked into CLI module context: %+v", excluded, cliDecisions)
		}
	}
	for _, test := range []struct {
		path string
		id   string
	}{
		{path: "internal/cli/main.go", id: "dec-cli-file"},
		{path: "internal/cli/./main.go", id: "dec-cli-dot-file"},
	} {
		exactContext, err := store.FindAffectedPathContextDecisions(
			ctx,
			test.path,
		)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, decision := range exactContext {
			if decision.ID == test.id {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf(
				"legacy exact-path context %s missing: %+v",
				test.id,
				exactContext,
			)
		}
	}

	nested, err := store.FindModuleDecisionsForFile(
		ctx,
		"internal/cli/nested/other.go",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(nested) != 1 || nested[0].ID != "dec-nested" {
		t.Fatalf("most-specific nested context = %+v", nested)
	}
	literal, err := store.FindModuleDecisionsForFile(
		ctx,
		"pkg/a_%/other.go",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(literal) != 1 || literal[0].ID != "dec-literal" {
		t.Fatalf("literal wildcard context = %+v", literal)
	}
	wildcardSibling, err := store.FindModuleDecisionsForFile(
		ctx,
		"pkg/abx/other.go",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(wildcardSibling) != 0 {
		t.Fatalf("wildcard widened authority: %+v", wildcardSibling)
	}

	for _, invalid := range []string{
		"../escape.go",
		`C:relative\file.go`,
		`\\server\share\file.go`,
	} {
		if _, err := store.FindModuleDecisionsForFile(ctx, invalid); err == nil {
			t.Fatalf("invalid project path %q was accepted", invalid)
		}
	}
}

func TestRootModuleIsFallbackAndRefreshDueRemainsCurrent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	seedModule(t, db, "mod-root", "", "root")
	seedModule(t, db, "mod-cli", "internal/cli", "cli")
	seedDecision(t, db, "dec-root", "Root policy", nil, []string{"main.go"})
	if _, err := db.Exec(
		`UPDATE artifacts SET status = 'refresh_due' WHERE id = 'dec-root'`,
	); err != nil {
		t.Fatal(err)
	}

	root, err := store.FindModuleForFile(ctx, "cmd/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if root == nil || root.ID != "mod-root" {
		t.Fatalf("root fallback = %+v", root)
	}
	nested, err := store.FindModuleForFile(ctx, "internal/cli/query.go")
	if err != nil {
		t.Fatal(err)
	}
	if nested == nil || nested.ID != "mod-cli" {
		t.Fatalf("nested module = %+v", nested)
	}
	decisions, err := store.FindDecisionsForModule(ctx, "mod-root")
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 1 || decisions[0].ID != "dec-root" {
		t.Fatalf("root current decisions = %+v", decisions)
	}
}
