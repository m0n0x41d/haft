package cli

import (
	"context"
	"database/sql"
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

	result, err := handleQuintQuery(context.Background(), store, t.TempDir(), map[string]any{
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

	result, err := handleQuintQuery(context.Background(), store, t.TempDir(), map[string]any{
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

	result, err := handleQuintQuery(context.Background(), store, t.TempDir(), map[string]any{
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

	result, err := handleQuintQuery(context.Background(), store, t.TempDir(), map[string]any{
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

	result, err := handleQuintQuery(context.Background(), store, t.TempDir(), map[string]any{
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
			claim_scope TEXT DEFAULT '[]', valid_until TEXT, created_at TEXT NOT NULL)`,
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
