package fpf

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func buildTestIndex(t *testing.T) (string, *sql.DB, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	chunks := []SpecChunk{
		{ID: 0, Heading: "A.6 - Signature Stack & Boundary Discipline", Level: 2, Body: "Boundary statements need routing.", PatternID: "A.6", Keywords: []string{"boundary", "routing"}, Queries: []string{"How do I route boundary statements?"}, RelatedIDs: []string{"A.6.B"}},
		{ID: 1, Heading: "A.6.B — Boundary Norm Square", Level: 2, Body: "Laws, admissibility, deontics, and work-effects.", PatternID: "A.6.B", Keywords: []string{"boundary", "deontics"}, Queries: []string{"What is the Boundary Norm Square?"}},
		{ID: 2, Heading: "A.16 — Language-State Transduction Coordination", Level: 2, Body: "Lawful moves for cues and handoffs.", PatternID: "A.16", Keywords: []string{"language-state", "route"}, Queries: []string{"How do cues get routed?"}, RelatedIDs: []string{"B.4.1"}},
		{ID: 3, Heading: "B.4.1 — Observe -> Notice -> Stabilize -> Route", Level: 2, Body: "Observe, notice, stabilize, route.", PatternID: "B.4.1", Keywords: []string{"route", "stabilize"}, Queries: []string{"What is the observe-to-route seam?"}},
		{ID: 4, Heading: "E.9 — Decision Record", Level: 2, Body: "Decision rationale and invariants.", PatternID: "E.9", Keywords: []string{"decision", "record", "drr"}, Queries: []string{"What is a decision record?"}},
	}

	if err := BuildSpecIndex(dbPath, chunks); err != nil {
		t.Fatalf("BuildSpecIndex failed: %v", err)
	}

	if err := SetSpecMeta(dbPath, "fpf_commit", "abc1234"); err != nil {
		t.Fatalf("SetSpecMeta failed: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	cleanup := func() {
		db.Close()
	}
	return dbPath, db, cleanup
}

func TestBuildSpecIndex_CreatesDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	chunks := []SpecChunk{{ID: 0, Heading: "Test", Level: 1, Body: "Content"}}
	if err := BuildSpecIndex(dbPath, chunks); err != nil {
		t.Fatalf("BuildSpecIndex failed: %v", err)
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}
}

func TestBuildSpecIndex_OverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	chunks := []SpecChunk{{ID: 0, Heading: "V1", Level: 1, Body: "Version 1", PatternID: "A.1"}}
	if err := BuildSpecIndex(dbPath, chunks); err != nil {
		t.Fatal(err)
	}

	chunks2 := []SpecChunk{{ID: 0, Heading: "V2", Level: 1, Body: "Version 2", PatternID: "A.2"}}
	if err := BuildSpecIndex(dbPath, chunks2); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	body, err := GetSpecSection(db, "A.2")
	if err != nil {
		t.Fatalf("A.2 section not found after overwrite: %v", err)
	}
	if body != "Version 2" {
		t.Errorf("expected 'Version 2', got %q", body)
	}
}

func TestBuildSpecIndex_PersistsTypedEdges(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	chunks := []SpecChunk{
		{
			ID:        0,
			Heading:   "A.1 - Source",
			Level:     2,
			Body:      "Body",
			PatternID: "A.1",
			Edges: []SpecEdge{{
				FromPatternID: "A.1",
				ToPatternID:   "B.1",
				EdgeType:      SpecEdgeTypeBuildsOn,
			}},
		},
		{ID: 1, Heading: "B.1 - Target", Level: 2, Body: "Body", PatternID: "B.1"},
	}

	if err := BuildSpecIndex(dbPath, chunks); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var fromPatternID string
	var toPatternID string
	var edgeType string
	err = db.QueryRow(`
		SELECT from_pattern_id, to_pattern_id, edge_type
		FROM section_edges
		WHERE from_pattern_id = ? AND to_pattern_id = ?
	`, "A.1", "B.1").Scan(&fromPatternID, &toPatternID, &edgeType)
	if err != nil {
		t.Fatal(err)
	}
	if edgeType != string(SpecEdgeTypeBuildsOn) {
		t.Fatalf("expected builds_on edge, got %q", edgeType)
	}
}

func TestBuildSpecIndex_FallsBackToRelatedEdges(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	chunks := []SpecChunk{
		{
			ID:         0,
			Heading:    "A.2 - Source",
			Level:      2,
			Body:       "Body",
			PatternID:  "A.2",
			RelatedIDs: []string{"B.2"},
		},
		{ID: 1, Heading: "B.2 - Target", Level: 2, Body: "Body", PatternID: "B.2"},
	}

	if err := BuildSpecIndex(dbPath, chunks); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var edgeType string
	err = db.QueryRow(`
		SELECT edge_type
		FROM section_edges
		WHERE from_pattern_id = ? AND to_pattern_id = ?
	`, "A.2", "B.2").Scan(&edgeType)
	if err != nil {
		t.Fatal(err)
	}
	if edgeType != string(SpecEdgeTypeRelated) {
		t.Fatalf("expected related edge, got %q", edgeType)
	}
}

func TestSearchSpec_ExactPatternLookupWins(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	results, err := SearchSpec(db, "A.6", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatalf("expected results, got none for query %q", "decision")
	}
	if results[0].PatternID != "A.6" {
		t.Fatalf("expected A.6 first, got %#v", results[0])
	}
	if results[0].Tier != "pattern" {
		t.Fatalf("expected pattern tier, got %q", results[0].Tier)
	}
}

func TestSearchSpec_RouteQueryLoadsCoreSections(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	results, err := SearchSpec(db, "boundary routing", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatalf("expected results, got none for query %q", "decision")
	}
	foundA6 := false
	for _, r := range results {
		if r.PatternID == "A.6" && (r.Tier == "route" || r.Tier == "pattern") {
			foundA6 = true
		}
	}
	if !foundA6 {
		t.Fatalf("expected route result for A.6, got %#v", results)
	}
}

func TestSearchSpec_RelatedExpansion(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	results, err := SearchSpec(db, "How do cues get routed?", 10)
	if err != nil {
		t.Fatal(err)
	}
	foundRelated := false
	for _, r := range results {
		if r.PatternID == "B.4.1" {
			foundRelated = true
		}
	}
	if !foundRelated {
		t.Fatalf("expected related result B.4.1, got %#v", results)
	}
}

func TestSearchSpec_FindsByKeywordFallback(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	results, err := SearchSpec(db, "deontics", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected fallback keyword results")
	}
	found := false
	for _, result := range results {
		if result.PatternID == "A.6.B" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected A.6.B in results, got %#v", results)
	}
}

func TestSearchSpec_NoResults(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	results, err := SearchSpec(db, "xyznonexistent", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %#v", results)
	}
}

func TestSearchSpec_DefaultLimit(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	results, err := SearchSpec(db, "route", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatalf("expected results, got none for query %q", "decision")
	}
}

func TestSearchSpec_SnippetReturned(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	results, err := SearchSpec(db, "decision", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatalf("expected results, got none for query %q", "decision")
	}
	foundSnippet := false
	for _, result := range results {
		if result.PatternID == "E.9" && result.Snippet != "" {
			foundSnippet = true
		}
	}
	if !foundSnippet {
		t.Fatalf("expected E.9 snippet, got %#v", results)
	}
}

func TestGetSpecSection_HeadingOrPattern(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	body, err := GetSpecSection(db, "E.9")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "Decision rationale") {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestSetSpecMeta_AndGetSpecMeta(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	val, err := GetSpecMeta(db, "fpf_commit")
	if err != nil {
		t.Fatal(err)
	}
	if val != "abc1234" {
		t.Fatalf("expected abc1234, got %q", val)
	}
}
