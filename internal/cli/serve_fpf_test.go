package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/fpf"
	_ "modernc.org/sqlite"
)

func TestHandleQuintQuery_FPFSupportsExplainFullAndLimit(t *testing.T) {
	dbPath := buildFPFSearchTestDB(t)

	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	store := setupCLIArtifactStore(t)

	result, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action":  "fpf",
		"query":   "boundary",
		"limit":   float64(1),
		"full":    true,
		"explain": true,
	})
	if err != nil {
		t.Fatalf("handleQuintQuery returned error: %v", err)
	}
	if !strings.Contains(result, "tier: route · Boundary discipline and routing") {
		t.Fatalf("expected explain metadata in output, got:\n%s", result)
	}
	if !strings.Contains(result, "summary: Boundary routing keeps claims on the right layer.") {
		t.Fatalf("expected explain output to include the section summary, got:\n%s", result)
	}
	if !strings.Contains(result, "TAIL-MARKER") {
		t.Fatalf("expected full output to include the complete section body, got:\n%s", result)
	}
	if strings.Contains(result, "### 2.") {
		t.Fatalf("expected limit=1 to cap output, got:\n%s", result)
	}
}

func TestHandleQuintQuery_FPFSupportsExperimentalTreeMode(t *testing.T) {
	dbPath := buildFPFSearchTestDB(t)

	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	store := setupCLIArtifactStore(t)

	result, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action":  "fpf",
		"query":   "boundary deontics",
		"limit":   float64(3),
		"mode":    fpf.SpecSearchModeTree,
		"explain": true,
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(tree mode) returned error: %v", err)
	}
	if !strings.Contains(result, "tier: drilldown · tree drill-down leaf A.6.B") {
		t.Fatalf("expected drilldown explain metadata in output, got:\n%s", result)
	}
	if !strings.Contains(result, "### 2. A.6 - Signature Stack & Boundary Discipline") {
		t.Fatalf("expected ancestor path output, got:\n%s", result)
	}
}

func TestHandleQuintQuery_FPFQueryOnlyStaysBackwardCompatible(t *testing.T) {
	dbPath := buildFPFSearchTestDB(t)

	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	store := setupCLIArtifactStore(t)

	result, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action": "fpf",
		"query":  "A.6",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery returned error: %v", err)
	}
	if strings.Contains(result, "tier:") {
		t.Fatalf("expected default MCP output to hide explain metadata, got:\n%s", result)
	}
	if strings.Contains(result, "TAIL-MARKER") {
		t.Fatalf("expected default MCP output to stay snippet-sized, got:\n%s", result)
	}
	if !strings.Contains(result, "### 1. A.6 - Signature Stack & Boundary Discipline") {
		t.Fatalf("expected pattern result in output, got:\n%s", result)
	}
	if !strings.Contains(result, "── Haft") {
		t.Fatalf("expected nav strip in output, got:\n%s", result)
	}
}

func TestHandleQuintQuery_FPFQueryOnlyUsesSharedDefaultLimit(t *testing.T) {
	dbPath := buildFPFManyResultsTestDB(t, fpf.DefaultSpecSearchLimit+2)

	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	store := setupCLIArtifactStore(t)

	result, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action": "fpf",
		"query":  "governance",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery returned error: %v", err)
	}

	resultCount := strings.Count(result, "### ")
	if resultCount != fpf.DefaultSpecSearchLimit {
		t.Fatalf("expected default limit %d, got %d results:\n%s", fpf.DefaultSpecSearchLimit, resultCount, result)
	}
}

func TestHandleQuintQuery_FPFEmptyStateKeepsNavStrip(t *testing.T) {
	dbPath := buildFPFSearchTestDB(t)

	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	store := setupCLIArtifactStore(t)

	result, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action": "fpf",
		"query":  "definitely-not-present",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery returned error: %v", err)
	}
	if !strings.Contains(result, "No results found.") {
		t.Fatalf("expected empty-state message, got:\n%s", result)
	}
	if !strings.Contains(result, "── Haft") {
		t.Fatalf("expected nav strip in empty-state output, got:\n%s", result)
	}
}

func TestHandleQuintQuery_RelatedArtifactIDReturnsProblemCardJSON(t *testing.T) {
	ctx := context.Background()
	store := setupCLIArtifactStore(t)

	problem := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:         "prob-20260423-test",
			Kind:       artifact.KindProblemCard,
			Title:      "Portable project MCP config",
			ValidUntil: "2026-05-23",
		},
		Body:           "Problem body for Open-Sleigh frame verification.",
		StructuredData: `{"signal":"shared config embeds an absolute path"}`,
	}
	if err := store.Create(ctx, problem); err != nil {
		t.Fatalf("create problem: %v", err)
	}

	result, err := handleQuintQuery(ctx, store, nil, t.TempDir(), map[string]any{
		"action":      "related",
		"artifact_id": problem.Meta.ID,
	})
	if err != nil {
		t.Fatalf("handleQuintQuery returned error: %v", err)
	}

	var payload map[string]map[string]any
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("expected JSON response, got %v:\n%s", err, result)
	}

	card := payload["problem_card"]
	if card["id"] != problem.Meta.ID {
		t.Fatalf("problem_card.id = %#v, want %s", card["id"], problem.Meta.ID)
	}
	if card["kind"] != string(artifact.KindProblemCard) {
		t.Fatalf("problem_card.kind = %#v", card["kind"])
	}
	if card["body"] != problem.Body {
		t.Fatalf("problem_card.body = %#v, want %q", card["body"], problem.Body)
	}
	if card["structured_data"] != problem.StructuredData {
		t.Fatalf("problem_card.structured_data = %#v", card["structured_data"])
	}
	semantic, ok := card["semantic"].(map[string]any)
	if !ok {
		t.Fatalf("problem_card.semantic missing or wrong shape: %#v", card["semantic"])
	}
	if semantic["status"] != string(artifact.SemanticStatusLegacy) {
		t.Fatalf("problem_card.semantic.status = %#v, want legacy", semantic["status"])
	}
	views, ok := card["views"].(map[string]any)
	if !ok {
		t.Fatalf("problem_card.views missing or wrong shape: %#v", card["views"])
	}
	if _, ok := views["working"]; !ok {
		t.Fatalf("problem_card.views missing working view: %#v", views)
	}
	if _, ok := views["exact"]; !ok {
		t.Fatalf("problem_card.views missing exact view: %#v", views)
	}
	if _, ok := views["audit"]; !ok {
		t.Fatalf("problem_card.views missing audit view: %#v", views)
	}
}

func TestHandleQuintQuery_ExactSearchAndCanonicalRelatedContract(t *testing.T) {
	ctx := context.Background()
	store := setupCLIArtifactStore(t)
	target := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "dec-20260711-exact",
			Kind:  artifact.KindDecisionRecord,
			Title: "Exact decision",
		},
		Body:           "Full decision body.",
		StructuredData: `{"claims":[{"id":"claim-1","observable":"go test ./...","threshold":"passes"}]}`,
	}
	neighbor := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "dec-20260711-neighbor",
			Kind:  artifact.KindDecisionRecord,
			Title: "Neighbor decision",
		},
		Body: "Full decision body with similar terms.",
	}
	for _, item := range []*artifact.Artifact{target, neighbor} {
		if err := store.Create(ctx, item); err != nil {
			t.Fatal(err)
		}
	}

	compact, err := handleQuintQuery(ctx, store, nil, t.TempDir(), map[string]any{
		"action": "search",
		"query":  target.Meta.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(compact, target.Meta.ID) || strings.Contains(compact, neighbor.Meta.ID) {
		t.Fatalf("compact exact search returned wrong set:\n%s", compact)
	}

	fullSearch, err := handleQuintQuery(ctx, store, nil, t.TempDir(), map[string]any{
		"action": "search",
		"query":  target.Meta.ID,
		"full":   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := handleQuintQuery(ctx, store, nil, t.TempDir(), map[string]any{
		"action":       "related",
		"artifact_ref": target.Meta.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if fullSearch != canonical {
		t.Fatalf("full exact search and canonical related differ:\nsearch=%s\nrelated=%s", fullSearch, canonical)
	}
	if !strings.Contains(canonical, target.Body) || !strings.Contains(canonical, "structured_data") || !strings.Contains(canonical, "claim-1") {
		t.Fatalf("full exact payload missing body or structured data: %s", canonical)
	}

	_, err = handleQuintQuery(ctx, store, nil, t.TempDir(), map[string]any{
		"action": "search",
		"query":  "decision body",
		"full":   true,
	})
	if err == nil || !strings.Contains(err.Error(), "run haft_query(action=\"search\"") {
		t.Fatalf("non-exact full search error = %v", err)
	}

	_, err = handleQuintQuery(ctx, store, nil, t.TempDir(), map[string]any{
		"action": "search",
		"query":  "dec-20260711-missing",
	})
	if err == nil || !strings.Contains(err.Error(), "semantic fallback was not used") {
		t.Fatalf("exact miss error = %v", err)
	}
}

func TestHandleQuintQuery_RelatedFileModeRemainsAvailable(t *testing.T) {
	ctx := context.Background()
	store := setupCLIArtifactStore(t)
	decision := &artifact.Artifact{
		Meta: artifact.Meta{ID: "dec-20260711-file", Kind: artifact.KindDecisionRecord, Title: "File decision"},
	}
	if err := store.Create(ctx, decision); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAffectedFiles(ctx, decision.Meta.ID, []artifact.AffectedFile{{Path: "internal/example.go"}}); err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(ctx, store, nil, t.TempDir(), map[string]any{
		"action": "related",
		"file":   "internal/example.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, decision.Meta.ID) {
		t.Fatalf("file-related result missing decision: %s", result)
	}
}

func TestHandleQuintQuery_RelatedProblemPayloadIncludesExactSemanticViews(t *testing.T) {
	ctx := context.Background()
	store := setupCLIArtifactStore(t)

	problem, _, err := artifact.FrameProblem(ctx, store, t.TempDir(), artifact.ProblemFrameInput{
		Title:                "Exact semantic payload",
		ProblemProfile:       artifact.ProblemProfileDeep,
		Signal:               "Fresh ProblemCards should expose exact semantic views.",
		WhyNow:               "PublicationUnit is now part of the semantic spine.",
		Scope:                "ProblemCard related payload views.",
		AcceptanceProbe:      "related payload includes working, exact, and audit views.",
		FreshnessDisposition: "Re-check when related view shape changes.",
		Acceptance:           "related payload includes working, exact, and audit views.",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(ctx, store, nil, t.TempDir(), map[string]any{
		"action": "related",
		"ref":    problem.Meta.ID,
	})
	if err != nil {
		t.Fatalf("handleQuintQuery returned error: %v", err)
	}

	var payload map[string]map[string]any
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("expected JSON response, got %v:\n%s", err, result)
	}

	card := payload["problem_card"]
	semantic := card["semantic"].(map[string]any)
	if semantic["status"] != string(artifact.SemanticStatusExact) {
		t.Fatalf("semantic.status = %#v, want exact", semantic["status"])
	}
	publicationUnit := semantic["publication_unit"].(map[string]any)
	if publicationUnit["publication_hash"] == "" || publicationUnit["carrier_hash"] == "" {
		t.Fatalf("semantic publication_unit missing hashes: %#v", publicationUnit)
	}

	views := card["views"].(map[string]any)
	working := views["working"].(map[string]any)
	if working["semantic_status"] != string(artifact.SemanticStatusExact) {
		t.Fatalf("working.semantic_status = %#v, want exact", working["semantic_status"])
	}
	if working["source_edition_hash"] == "" || working["publication_hash"] == "" {
		t.Fatalf("working view missing source/publication hashes: %#v", working)
	}
	if working["problem_profile"] != artifact.ProblemProfileDeep {
		t.Fatalf("working.problem_profile = %#v, want %q", working["problem_profile"], artifact.ProblemProfileDeep)
	}
	if working["p2w_readiness"] != artifact.ProblemReadinessReady {
		t.Fatalf("working.p2w_readiness = %#v, want %q", working["p2w_readiness"], artifact.ProblemReadinessReady)
	}
	exact := views["exact"].(map[string]any)
	if _, ok := exact["source_episteme"].(map[string]any); !ok {
		t.Fatalf("exact view missing source_episteme: %#v", exact)
	}
	if _, ok := exact["publication_projection"].(map[string]any); !ok {
		t.Fatalf("exact view missing publication_projection: %#v", exact)
	}
	if _, ok := exact["carrier_bytes"].(map[string]any); !ok {
		t.Fatalf("exact view missing carrier_bytes: %#v", exact)
	}
	audit := views["audit"].(map[string]any)
	if audit["semantic_status"] != string(artifact.SemanticStatusExact) {
		t.Fatalf("audit.semantic_status = %#v, want exact", audit["semantic_status"])
	}
	if _, ok := audit["publication_unit"].(map[string]any); !ok {
		t.Fatalf("audit view missing publication_unit: %#v", audit)
	}
}

func TestHandleQuintProblem_FramePersistsProblemProfile(t *testing.T) {
	ctx := context.Background()
	store := setupCLIArtifactStore(t)

	result, err := handleQuintProblem(ctx, store, t.TempDir(), map[string]any{
		"action":                "frame",
		"title":                 "Ticket cannot become work without boundary",
		"problem_profile":       artifact.ProblemProfileDeep,
		"source_kind":           artifact.ProblemSourceTicket,
		"signal":                "A ticket names a request but not an implementation boundary.",
		"why_now":               "The slice train is consuming problem frames.",
		"freshness_disposition": "Re-check before admission.",
	})
	if err != nil {
		t.Fatalf("handleQuintProblem returned error: %v", err)
	}
	if !strings.Contains(result, "Problem framed") {
		t.Fatalf("unexpected frame response:\n%s", result)
	}

	items, err := artifact.SelectProblems(ctx, store, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("created problem count = %d, want 1", len(items))
	}

	reloaded, err := store.Get(ctx, items[0].Meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	profile := reloaded.UnmarshalProblemFields().Profile
	if profile == nil {
		t.Fatal("problem profile missing")
	}
	if profile.SourceKind != artifact.ProblemSourceTicket {
		t.Fatalf("profile.source_kind = %q, want %q", profile.SourceKind, artifact.ProblemSourceTicket)
	}
	if profile.Readiness != artifact.ProblemReadinessBlocked {
		t.Fatalf("profile.readiness = %q, want %q", profile.Readiness, artifact.ProblemReadinessBlocked)
	}
}

func setupCLIArtifactStore(t *testing.T) *artifact.Store {
	t.Helper()

	db, err := sql.Open("sqlite", t.TempDir()+"/cli-tools.db")
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
		`CREATE TABLE artifact_links (
			source_id TEXT NOT NULL, target_id TEXT NOT NULL, link_type TEXT NOT NULL,
			created_at TEXT NOT NULL, PRIMARY KEY (source_id, target_id, link_type))`,
		`CREATE TABLE evidence_items (
			id TEXT PRIMARY KEY, artifact_ref TEXT NOT NULL, type TEXT NOT NULL,
			content TEXT NOT NULL, verdict TEXT, carrier_ref TEXT,
			congruence_level INTEGER DEFAULT 3, formality_level INTEGER DEFAULT 5,
			claim_refs TEXT DEFAULT '[]', claim_scope TEXT DEFAULT '[]',
			valid_until TEXT, created_at TEXT NOT NULL)`,
		`CREATE TABLE affected_files (
			artifact_id TEXT NOT NULL, file_path TEXT NOT NULL, file_hash TEXT,
			PRIMARY KEY (artifact_id, file_path))`,
		`CREATE TABLE codebase_modules (
			module_id TEXT PRIMARY KEY, path TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL, lang TEXT, file_count INTEGER DEFAULT 0,
			last_scanned TEXT NOT NULL)`,
		`CREATE TABLE module_dependencies (
			source_module TEXT NOT NULL, target_module TEXT NOT NULL,
			dep_type TEXT NOT NULL DEFAULT 'import', file_path TEXT,
			last_scanned TEXT NOT NULL,
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

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup: %v\nSQL: %s", err, stmt)
		}
	}

	return artifact.NewStore(db)
}

func buildFPFManyResultsTestDB(t *testing.T, total int) string {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "fpf-many-results.db")
	chunks := make([]fpf.SpecChunk, 0, total)

	for index := range total {
		patternID := fmt.Sprintf("A.%d", index+1)
		heading := fmt.Sprintf("%s - Governance Pattern %d", patternID, index+1)
		body := fmt.Sprintf("Governance result %d keeps reasoning explicit.", index+1)
		keywords := []string{"governance", "policy"}
		queries := []string{fmt.Sprintf("How do I handle governance case %d?", index+1)}

		chunks = append(chunks, fpf.SpecChunk{
			ID:        index,
			Heading:   heading,
			Level:     2,
			Body:      body,
			PatternID: patternID,
			Keywords:  keywords,
			Queries:   queries,
		})
	}

	if err := fpf.BuildSpecIndex(dbPath, chunks, nil); err != nil {
		t.Fatalf("BuildSpecIndex failed: %v", err)
	}

	return dbPath
}
