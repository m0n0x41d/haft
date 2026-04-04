package fpf

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func buildTestIndex(t *testing.T) (string, *sql.DB, func()) {
	t.Helper()

	chunks := []SpecChunk{
		{ID: 0, Heading: "A.6 - Signature Stack & Boundary Discipline", Level: 2, Body: "Boundary statements need routing.", PatternID: "A.6", Keywords: []string{"boundary", "routing"}, Queries: []string{"How do I route boundary statements?"}, RelatedIDs: []string{"A.6.B"}},
		{ID: 1, Heading: "A.6.B — Boundary Norm Square", Level: 2, Body: "Laws, admissibility, deontics, and work-effects.", PatternID: "A.6.B", Keywords: []string{"boundary", "deontics"}, Queries: []string{"What is the Boundary Norm Square?"}},
		{ID: 2, Heading: "A.16 — Language-State Transduction Coordination", Level: 2, Body: "Lawful moves for cues and handoffs.", PatternID: "A.16", Keywords: []string{"language-state", "route"}, Queries: []string{"How do cues get routed?"}, RelatedIDs: []string{"B.4.1"}},
		{ID: 3, Heading: "B.4.1 — Observe -> Notice -> Stabilize -> Route", Level: 2, Body: "Observe, notice, stabilize, route.", PatternID: "B.4.1", Keywords: []string{"route", "stabilize"}, Queries: []string{"What is the observe-to-route seam?"}},
		{ID: 4, Heading: "E.9 — Decision Record", Level: 2, Body: "Decision rationale and invariants.", PatternID: "E.9", Keywords: []string{"decision", "record", "drr"}, Queries: []string{"What is a decision record?"}},
	}

	return buildIndexWithChunks(t, chunks, true)
}

func buildIndexWithChunks(t *testing.T, chunks []SpecChunk, withMeta bool) (string, *sql.DB, func()) {
	t.Helper()

	routes := testRoutesForChunks(chunks)
	return buildIndexWithChunksAndRoutes(t, chunks, routes, withMeta)
}

func buildIndexWithChunksAndRoutes(t *testing.T, chunks []SpecChunk, routes []Route, withMeta bool) (string, *sql.DB, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	if err := BuildSpecIndex(dbPath, chunks, routes); err != nil {
		t.Fatalf("BuildSpecIndex failed: %v", err)
	}

	if withMeta {
		if err := SetSpecMeta(dbPath, "fpf_commit", "abc1234"); err != nil {
			t.Fatalf("SetSpecMeta failed: %v", err)
		}
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

func testRoutesForChunks(chunks []SpecChunk) []Route {
	patternIDs := collectChunkPatternIDs(normalizeChunksForIndex(chunks))
	routes := make([]Route, 0, 2)

	if hasAllPatternIDs(patternIDs, "A.6", "A.6.B") {
		routes = append(routes, Route{
			ID:          "boundary-unpacking",
			Title:       "Boundary discipline and routing",
			Description: "Boundary statements, contracts, and routing language.",
			Matchers:    []string{"boundary", "contract", "routing", "deontic"},
			Core:        []string{"A.6", "A.6.B"},
			Chain:       []string{"A.6", "A.6.B"},
		})
	}

	if hasAllPatternIDs(patternIDs, "A.16", "B.4.1") {
		chain := make([]string, 0, 5)
		if hasAllPatternIDs(patternIDs, "C.2.2a") {
			chain = append(chain, "C.2.2a")
		}
		chain = append(chain, "A.16")
		if hasAllPatternIDs(patternIDs, "A.16.1") {
			chain = append(chain, "A.16.1")
		}
		if hasAllPatternIDs(patternIDs, "A.16.2") {
			chain = append(chain, "A.16.2")
		}
		chain = append(chain, "B.4.1")
		if hasAllPatternIDs(patternIDs, "B.5.2.0") {
			chain = append(chain, "B.5.2.0")
		}

		routes = append(routes, Route{
			ID:          "language-discovery",
			Title:       "Language-state and routing cues",
			Description: "How partial articulation becomes routed and stabilized.",
			Matchers:    []string{"language", "cue", "route", "stabilize", "routed"},
			Core:        []string{"A.16", "B.4.1"},
			Chain:       chain,
		})
	}

	return routes
}

func hasAllPatternIDs(patternIDs map[string]struct{}, ids ...string) bool {
	for _, id := range ids {
		if _, ok := patternIDs[id]; !ok {
			return false
		}
	}

	return true
}

func TestBuildSpecIndex_CreatesDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	chunks := []SpecChunk{{ID: 0, Heading: "Test", Level: 1, Body: "Content"}}
	if err := BuildSpecIndex(dbPath, chunks, testRoutesForChunks(chunks)); err != nil {
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
	if err := BuildSpecIndex(dbPath, chunks, testRoutesForChunks(chunks)); err != nil {
		t.Fatal(err)
	}

	chunks2 := []SpecChunk{{ID: 0, Heading: "V2", Level: 1, Body: "Version 2", PatternID: "A.2"}}
	if err := BuildSpecIndex(dbPath, chunks2, testRoutesForChunks(chunks2)); err != nil {
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

	if err := BuildSpecIndex(dbPath, chunks, testRoutesForChunks(chunks)); err != nil {
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

	if err := BuildSpecIndex(dbPath, chunks, testRoutesForChunks(chunks)); err != nil {
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
	if results[0].Summary != "Boundary statements need routing." {
		t.Fatalf("expected summary to be indexed, got %q", results[0].Summary)
	}
}

func TestBuildSpecIndex_NormalizesPatternIDs(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	chunks := []SpecChunk{
		{
			ID:         0,
			Heading:    "A.6 - Signature Stack & Boundary Discipline",
			Level:      2,
			Body:       "Boundary statements need routing.",
			PatternID:  "a6",
			RelatedIDs: []string{"a.6.b"},
		},
		{
			ID:        1,
			Heading:   "A.6.B - Boundary Norm Square",
			Level:     2,
			Body:      "Norm square.",
			PatternID: "a.6.b",
		},
	}

	if err := BuildSpecIndex(dbPath, chunks, testRoutesForChunks(chunks)); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var storedPatternID string
	err = db.QueryRow(`SELECT pattern_id FROM sections WHERE heading = ?`, "A.6 - Signature Stack & Boundary Discipline").Scan(&storedPatternID)
	if err != nil {
		t.Fatal(err)
	}
	if storedPatternID != "A.6" {
		t.Fatalf("expected normalized pattern id A.6, got %q", storedPatternID)
	}

	var edgeType string
	err = db.QueryRow(`
		SELECT edge_type
		FROM section_edges
		WHERE from_pattern_id = ? AND to_pattern_id = ?
	`, "A.6", "A.6.B").Scan(&edgeType)
	if err != nil {
		t.Fatal(err)
	}
	if edgeType != string(SpecEdgeTypeRelated) {
		t.Fatalf("expected related edge, got %q", edgeType)
	}
}

func TestBuildSpecIndex_PersistsSectionSummary(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	chunks := []SpecChunk{
		{
			ID:        0,
			Heading:   "A.2.8:4.1 - Normative definition",
			Level:     4,
			Body:      "A `U.Commitment` is a governance object with explicit scope. Additional detail follows later.",
			PatternID: "A.2.8:4.1",
		},
	}

	if err := BuildSpecIndex(dbPath, chunks, testRoutesForChunks(chunks)); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var summary string
	err = db.QueryRow(`SELECT summary FROM sections WHERE pattern_id = ?`, "A.2.8:4.1").Scan(&summary)
	if err != nil {
		t.Fatal(err)
	}

	want := "A U.Commitment is a governance object with explicit scope."
	if summary != want {
		t.Fatalf("summary = %q, want %q", summary, want)
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

func TestSearchSpec_RelatedExpansionPrefersTypedEdges(t *testing.T) {
	chunks := []SpecChunk{
		{
			ID:        0,
			Heading:   "A.6 - Signature Stack & Boundary Discipline",
			Level:     2,
			Body:      "Boundary statements need routing.",
			PatternID: "A.6",
			Edges: []SpecEdge{
				{FromPatternID: "A.6", ToPatternID: "B.1", EdgeType: SpecEdgeTypeBuildsOn},
				{FromPatternID: "A.6", ToPatternID: "B.2", EdgeType: SpecEdgeTypePrerequisiteFor},
				{FromPatternID: "A.6", ToPatternID: "B.3", EdgeType: SpecEdgeTypeCoordinatesWith},
				{FromPatternID: "A.6", ToPatternID: "B.5", EdgeType: SpecEdgeTypeInforms},
			},
			RelatedIDs: []string{"B.4"},
		},
		{ID: 1, Heading: "A.6.B — Boundary Norm Square", Level: 2, Body: "Norm square.", PatternID: "A.6.B"},
		{ID: 2, Heading: "B.1 — Builds On Target", Level: 2, Body: "Builds on.", PatternID: "B.1"},
		{ID: 3, Heading: "B.2 — Prerequisite Target", Level: 2, Body: "Prerequisite.", PatternID: "B.2"},
		{ID: 4, Heading: "B.3 — Coordinates Target", Level: 2, Body: "Coordinates.", PatternID: "B.3"},
		{ID: 5, Heading: "B.4 — Related Target", Level: 2, Body: "Related.", PatternID: "B.4"},
		{ID: 6, Heading: "B.5 — Informs Target", Level: 2, Body: "Informs.", PatternID: "B.5"},
	}

	_, db, cleanup := buildIndexWithChunks(t, chunks, false)
	defer cleanup()

	results, err := SearchSpec(db, "boundary routing", 10)
	if err != nil {
		t.Fatal(err)
	}

	relatedResults := filterResultsByTier(results, "related")
	got := resultPatternIDs(relatedResults)
	wantPrefix := []string{"B.1", "B.2", "B.3"}
	if len(got) != 5 {
		t.Fatalf("unexpected related expansion count: got %v", got)
	}
	if !reflect.DeepEqual(got[:3], wantPrefix) {
		t.Fatalf("unexpected preferred-edge order: got %v want prefix %v", got, wantPrefix)
	}

	buildsOnResult := findResultByPatternID(relatedResults, "B.1")
	if buildsOnResult == nil || buildsOnResult.Reason != "builds_on via A.6" {
		t.Fatalf("unexpected builds_on reason: %#v", buildsOnResult)
	}

	relatedResult := findResultByPatternID(relatedResults, "B.4")
	if relatedResult == nil || relatedResult.Reason != "related via A.6" {
		t.Fatalf("unexpected fallback reason: %#v", relatedResult)
	}

	informsResult := findResultByPatternID(relatedResults, "B.5")
	if informsResult == nil || informsResult.Reason != "informs via A.6" {
		t.Fatalf("unexpected weaker-edge reason: %#v", informsResult)
	}
}

func TestSearchSpec_RelatedExpansionFallsBackToWeakerEdges(t *testing.T) {
	chunks := []SpecChunk{
		{
			ID:         0,
			Heading:    "A.6 - Signature Stack & Boundary Discipline",
			Level:      2,
			Body:       "Boundary statements need routing.",
			PatternID:  "A.6",
			RelatedIDs: []string{"B.9"},
		},
		{ID: 1, Heading: "A.6.B — Boundary Norm Square", Level: 2, Body: "Norm square.", PatternID: "A.6.B"},
		{ID: 2, Heading: "B.9 — Related Target", Level: 2, Body: "Related.", PatternID: "B.9"},
	}

	_, db, cleanup := buildIndexWithChunks(t, chunks, false)
	defer cleanup()

	results, err := SearchSpec(db, "boundary routing", 10)
	if err != nil {
		t.Fatal(err)
	}

	relatedResults := filterResultsByTier(results, "related")
	if len(relatedResults) != 1 {
		t.Fatalf("expected one related fallback result, got %#v", relatedResults)
	}
	if relatedResults[0].PatternID != "B.9" {
		t.Fatalf("unexpected fallback result: %#v", relatedResults[0])
	}
	if relatedResults[0].Reason != "related via A.6" {
		t.Fatalf("unexpected fallback reason: %#v", relatedResults[0])
	}
}

func TestSearchSpec_RelatedExpansionPrefersWeakTypedEdgesOverGenericRelated(t *testing.T) {
	chunks := []SpecChunk{
		{
			ID:        0,
			Heading:   "A.6 - Signature Stack & Boundary Discipline",
			Level:     2,
			Body:      "Boundary statements need routing.",
			PatternID: "A.6",
			Edges: []SpecEdge{
				{FromPatternID: "A.6", ToPatternID: "B.1", EdgeType: SpecEdgeTypeConstrains},
				{FromPatternID: "A.6", ToPatternID: "B.2", EdgeType: SpecEdgeTypeInforms},
			},
			RelatedIDs: []string{"B.3"},
		},
		{ID: 1, Heading: "A.6.B — Boundary Norm Square", Level: 2, Body: "Norm square.", PatternID: "A.6.B"},
		{ID: 2, Heading: "B.1 — Constrains Target", Level: 2, Body: "Constrains.", PatternID: "B.1"},
		{ID: 3, Heading: "B.2 — Informs Target", Level: 2, Body: "Informs.", PatternID: "B.2"},
		{ID: 4, Heading: "B.3 — Related Target", Level: 2, Body: "Related.", PatternID: "B.3"},
	}

	_, db, cleanup := buildIndexWithChunks(t, chunks, false)
	defer cleanup()

	results, err := SearchSpec(db, "boundary routing", 10)
	if err != nil {
		t.Fatal(err)
	}

	relatedResults := filterResultsByTier(results, "related")
	got := resultPatternIDs(relatedResults)
	want := []string{"B.1", "B.2", "B.3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected weak-edge fallback order: got %v want %v", got, want)
	}
}

func TestSearchSpec_RelatedExpansionIsBounded(t *testing.T) {
	chunks := []SpecChunk{
		{
			ID:        0,
			Heading:   "A.6 - Signature Stack & Boundary Discipline",
			Level:     2,
			Body:      "Boundary statements need routing.",
			PatternID: "A.6",
			RelatedIDs: []string{
				"Z.1",
				"Z.2",
				"Z.3",
			},
		},
		{ID: 1, Heading: "A.6.B — Boundary Norm Square", Level: 2, Body: "Norm square.", PatternID: "A.6.B"},
	}

	for index := 0; index < relatedExpansionLimit+3; index++ {
		patternID := fmt.Sprintf("B.%d", index+1)
		chunks[0].Edges = append(chunks[0].Edges, SpecEdge{
			FromPatternID: "A.6",
			ToPatternID:   patternID,
			EdgeType:      SpecEdgeTypeBuildsOn,
		})
		chunks = append(chunks, SpecChunk{
			ID:        index + 2,
			Heading:   patternID + " — Related Target",
			Level:     2,
			Body:      "Related.",
			PatternID: patternID,
		})
	}

	for index := 0; index < 3; index++ {
		patternID := fmt.Sprintf("Z.%d", index+1)
		chunks = append(chunks, SpecChunk{
			ID:        relatedExpansionLimit + index + 5,
			Heading:   patternID + " — Fallback Target",
			Level:     2,
			Body:      "Fallback.",
			PatternID: patternID,
		})
	}

	_, db, cleanup := buildIndexWithChunks(t, chunks, false)
	defer cleanup()

	results, err := SearchSpecWithOptions(db, "boundary routing", SpecSearchOptions{
		Limit: relatedExpansionLimit,
		Tier:  SpecSearchTierRelated,
	})
	if err != nil {
		t.Fatal(err)
	}

	relatedResults := filterResultsByTier(results, SpecSearchTierRelated)
	got := resultPatternIDs(relatedResults)
	if len(relatedResults) != relatedExpansionLimit {
		t.Fatalf("expected %d related results, got %d (%v)", relatedExpansionLimit, len(relatedResults), got)
	}
	if containsString(got, "Z.1") || containsString(got, "Z.2") || containsString(got, "Z.3") {
		t.Fatalf("expected cap to exclude fallback overflow, got %v", got)
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

	tests := []struct {
		name   string
		lookup string
	}{
		{name: "pattern id", lookup: "E.9"},
		{name: "heading", lookup: "E.9 — Decision Record"},
	}

	for _, tt := range tests {
		body, err := GetSpecSection(db, tt.lookup)
		if err != nil {
			t.Fatalf("%s lookup failed: %v", tt.name, err)
		}
		if !strings.Contains(body, "Decision rationale") {
			t.Fatalf("%s lookup returned unexpected body: %s", tt.name, body)
		}
	}
}

func TestSearchSpec_ExactPatternLookupNormalizesVariants(t *testing.T) {
	chunks := []SpecChunk{
		{ID: 0, Heading: "A.6 - Signature Stack & Boundary Discipline", Level: 2, Body: "Boundary statements need routing.", PatternID: "A.6"},
		{ID: 1, Heading: "A.6.B - Boundary Norm Square", Level: 2, Body: "Norm square.", PatternID: "A.6.B"},
		{ID: 2, Heading: "A.6:4.1 - Worked Example", Level: 3, Body: "Worked example.", PatternID: "A.6:4.1"},
		{ID: 3, Heading: "C.2.2a - Language-State Space", Level: 2, Body: "Language-state chart.", PatternID: "C.2.2a"},
		{ID: 4, Heading: "A.19.CN - CN-frame", Level: 2, Body: "Comparability and normalization.", PatternID: "A.19.CN"},
		{ID: 5, Heading: "G.Core - Part G Core Invariants", Level: 2, Body: "Part G core invariants.", PatternID: "G.Core"},
		{ID: 6, Heading: "G.Core:1 - Problem frame", Level: 3, Body: "Problem frame.", PatternID: "G.Core:1"},
		{ID: 7, Heading: "A.0:End - End", Level: 3, Body: "End marker.", PatternID: "A.0:End"},
		{ID: 8, Heading: "C.3.A:A.1 - Purpose & fit", Level: 5, Body: "Annex purpose.", PatternID: "C.3.A:A.1"},
	}

	_, db, cleanup := buildIndexWithChunks(t, chunks, false)
	defer cleanup()

	tests := []struct {
		query          string
		wantPatternID  string
		wantBodySubstr string
	}{
		{query: "a.6", wantPatternID: "A.6", wantBodySubstr: "Boundary statements"},
		{query: "A6", wantPatternID: "A.6", wantBodySubstr: "Boundary statements"},
		{query: "A.6:", wantPatternID: "A.6", wantBodySubstr: "Boundary statements"},
		{query: "a.6.b", wantPatternID: "A.6.B", wantBodySubstr: "Norm square"},
		{query: "a.6:4.1", wantPatternID: "A.6:4.1", wantBodySubstr: "Worked example"},
		{query: "c.2.2A", wantPatternID: "C.2.2a", wantBodySubstr: "Language-state chart"},
		{query: "a.19.cn", wantPatternID: "A.19.CN", wantBodySubstr: "Comparability and normalization"},
		{query: "g.core", wantPatternID: "G.CORE", wantBodySubstr: "Part G core invariants"},
		{query: "g.core:1", wantPatternID: "G.CORE:1", wantBodySubstr: "Problem frame"},
		{query: "a.0:end", wantPatternID: "A.0:END", wantBodySubstr: "End marker"},
		{query: "c.3.a:a.1", wantPatternID: "C.3.A:A.1", wantBodySubstr: "Annex purpose"},
	}

	for _, tt := range tests {
		results, err := SearchSpec(db, tt.query, 5)
		if err != nil {
			t.Fatalf("SearchSpec(%q) error: %v", tt.query, err)
		}
		if len(results) == 0 {
			t.Fatalf("SearchSpec(%q) returned no results", tt.query)
		}
		if results[0].PatternID != tt.wantPatternID {
			t.Fatalf("SearchSpec(%q) first pattern = %q, want %q", tt.query, results[0].PatternID, tt.wantPatternID)
		}
		if results[0].Tier != "pattern" {
			t.Fatalf("SearchSpec(%q) first tier = %q, want pattern", tt.query, results[0].Tier)
		}

		body, err := GetSpecSection(db, tt.query)
		if err != nil {
			t.Fatalf("GetSpecSection(%q) error: %v", tt.query, err)
		}
		if !strings.Contains(body, tt.wantBodySubstr) {
			t.Fatalf("GetSpecSection(%q) body = %q, want substring %q", tt.query, body, tt.wantBodySubstr)
		}
	}
}

func TestNormalizeChunkForIndex_NormalizesAliases(t *testing.T) {
	chunk := SpecChunk{
		PatternID: "A.17",
		Aliases: []string{
			` Canonical “Characteristic” `,
			`Canonical "Characteristic"`,
			`A.CHR‑NORM`,
			`a.chr-norm`,
		},
	}

	got := normalizeChunkForIndex(chunk)
	want := []string{
		"Canonical Characteristic",
		"A.CHR-NORM",
		"A.17",
	}
	if !reflect.DeepEqual(got.Aliases, want) {
		t.Fatalf("normalizeChunkForIndex aliases = %#v, want %#v", got.Aliases, want)
	}
}

func TestSearchSpec_HeadingOnlyRootPatternAliasLookup(t *testing.T) {
	chunks := []SpecChunk{
		{
			ID:        0,
			Heading:   "A.17 - Canonical Characteristic (A.CHR-NORM)",
			Level:     2,
			Body:      "",
			Summary:   "Canonical Characteristic (A.CHR-NORM)",
			PatternID: "A.17",
			Aliases:   []string{"A.17", "A.CHR-NORM", "Canonical Characteristic"},
		},
		{
			ID:              1,
			Heading:         "A.17:1 - Context",
			Level:           3,
			Body:            "To have reproducibility and explainability there is a need to measure various aspects.",
			PatternID:       "A.17:1",
			ParentPatternID: "A.17",
		},
	}

	_, db, cleanup := buildIndexWithChunks(t, chunks, false)
	defer cleanup()

	patternResults, err := SearchSpec(db, "A.17", 5)
	if err != nil {
		t.Fatalf("SearchSpec(A.17) error: %v", err)
	}
	if len(patternResults) == 0 {
		t.Fatal("expected pattern results for A.17")
	}
	if patternResults[0].PatternID != "A.17" || patternResults[0].Tier != SpecSearchTierPattern {
		t.Fatalf("unexpected A.17 lookup result: %#v", patternResults[0])
	}

	aliasResults, err := SearchSpec(db, "A.CHR-NORM", 5)
	if err != nil {
		t.Fatalf("SearchSpec(A.CHR-NORM) error: %v", err)
	}
	if len(aliasResults) == 0 {
		t.Fatal("expected alias results for A.CHR-NORM")
	}
	if aliasResults[0].PatternID != "A.17" {
		t.Fatalf("expected A.17 for alias lookup, got %#v", aliasResults[0])
	}

	body, err := GetSpecSection(db, "A.17")
	if err != nil {
		t.Fatalf("GetSpecSection(A.17) error: %v", err)
	}
	if body != "Canonical Characteristic (A.CHR-NORM)" {
		t.Fatalf("GetSpecSection(A.17) = %q, want summary fallback", body)
	}
}

func TestSearchSpecWithOptions_TierFiltersRemainTierSpecific(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	tests := []struct {
		name      string
		query     string
		tier      string
		wantCount int
	}{
		{name: "route", query: "boundary", tier: SpecSearchTierRoute, wantCount: 2},
		{name: "fts", query: "boundary", tier: SpecSearchTierFTS, wantCount: 2},
	}

	for _, tt := range tests {
		results, err := SearchSpecWithOptions(db, tt.query, SpecSearchOptions{
			Limit: 10,
			Tier:  tt.tier,
		})
		if err != nil {
			t.Fatalf("%s tier search error: %v", tt.name, err)
		}
		if len(results) != tt.wantCount {
			t.Fatalf("%s tier returned %d results, want %d", tt.name, len(results), tt.wantCount)
		}
		for _, result := range results {
			if result.Tier != tt.tier {
				t.Fatalf("%s tier returned mixed tier result %#v", tt.name, result)
			}
		}
	}
}

func TestSearchSpecWithOptions_InvalidTier(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	_, err := SearchSpecWithOptions(db, "boundary routing", SpecSearchOptions{
		Limit: 5,
		Tier:  "bogus",
	})
	if err == nil {
		t.Fatal("expected invalid tier error")
	}
	if !strings.Contains(err.Error(), "unsupported search tier") {
		t.Fatalf("unexpected invalid tier error: %v", err)
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

func TestSetSpecMetaEntries_AndGetSpecIndexInfo(t *testing.T) {
	dbPath, db, cleanup := buildIndexWithChunks(t, []SpecChunk{
		{ID: 0, Heading: "A.6 - Signature Stack & Boundary Discipline", Level: 2, Body: "Boundary statements need routing.", PatternID: "A.6"},
	}, false)
	defer cleanup()

	metadata := map[string]string{
		"fpf_commit":       "9442ffb733de574859cfd715b5fe67c06c7bb239",
		"indexed_sections": "1",
		"build_time":       "2026-03-26T12:34:56Z",
		"spec_path":        "data/FPF/FPF-Spec.md",
		"schema_version":   SpecIndexSchemaVersion,
	}
	if err := SetSpecMetaEntries(dbPath, metadata); err != nil {
		t.Fatalf("SetSpecMetaEntries failed: %v", err)
	}

	info, err := GetSpecIndexInfo(db)
	if err != nil {
		t.Fatalf("GetSpecIndexInfo failed: %v", err)
	}

	if info.Commit != metadata["fpf_commit"] {
		t.Fatalf("expected commit %q, got %q", metadata["fpf_commit"], info.Commit)
	}
	if info.IndexedSections != metadata["indexed_sections"] {
		t.Fatalf("expected indexed sections %q, got %q", metadata["indexed_sections"], info.IndexedSections)
	}
	if info.BuildTime != metadata["build_time"] {
		t.Fatalf("expected build time %q, got %q", metadata["build_time"], info.BuildTime)
	}
	if info.SpecPath != metadata["spec_path"] {
		t.Fatalf("expected spec path %q, got %q", metadata["spec_path"], info.SpecPath)
	}
	if info.SchemaVersion != metadata["schema_version"] {
		t.Fatalf("expected schema version %q, got %q", metadata["schema_version"], info.SchemaVersion)
	}
}

func filterResultsByTier(results []SpecSearchResult, tier string) []SpecSearchResult {
	filtered := make([]SpecSearchResult, 0, len(results))
	for _, result := range results {
		if result.Tier == tier {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

func resultPatternIDs(results []SpecSearchResult) []string {
	patternIDs := make([]string, 0, len(results))
	for _, result := range results {
		patternIDs = append(patternIDs, result.PatternID)
	}
	return patternIDs
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func findResultByPatternID(results []SpecSearchResult, patternID string) *SpecSearchResult {
	for index := range results {
		if results[index].PatternID == patternID {
			return &results[index]
		}
	}
	return nil
}
