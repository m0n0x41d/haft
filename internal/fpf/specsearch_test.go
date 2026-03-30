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
		{ID: 0, Heading: "1. Introduction", Level: 1, Body: "This is the introduction to FPF methodology."},
		{ID: 1, Heading: "2. WLNK — Weakest Link", Level: 2, Body: "System quality equals the minimum of component qualities. The weakest link bounds the whole."},
		{ID: 2, Heading: "3. ADI Cycle", Level: 2, Body: "Abduction generates hypotheses. Deduction derives predictions. Induction tests against evidence."},
		{ID: 3, Heading: "4. Evidence Records", Level: 2, Body: "F-G-R assessment: Formality, ClaimScope (G), Reliability. Min across chain for F and R."},
		{ID: 4, Heading: "5. Pareto Selection", Level: 2, Body: "Hold the Pareto front. State selection policy before applying it. Ensure parity for fair comparison."},
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

	chunks := []SpecChunk{
		{ID: 0, Heading: "Test", Level: 1, Body: "Content"},
	}

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

	// Build twice — second should overwrite
	chunks := []SpecChunk{{ID: 0, Heading: "V1", Level: 1, Body: "Version 1"}}
	if err := BuildSpecIndex(dbPath, chunks); err != nil {
		t.Fatal(err)
	}

	chunks2 := []SpecChunk{{ID: 0, Heading: "V2", Level: 1, Body: "Version 2"}}
	if err := BuildSpecIndex(dbPath, chunks2); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	body, err := GetSpecSection(db, "V2")
	if err != nil {
		t.Fatalf("V2 section not found after overwrite: %v", err)
	}
	if body != "Version 2" {
		t.Errorf("expected 'Version 2', got %q", body)
	}

	_, err = GetSpecSection(db, "V1")
	if err == nil {
		t.Error("V1 should not exist after overwrite")
	}
}

func TestSearchSpec_FindsByKeyword(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	results, err := SearchSpec(db, "weakest link", 10)
	if err != nil {
		t.Fatalf("SearchSpec failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results for 'weakest link', got none")
	}

	found := false
	for _, r := range results {
		if strings.Contains(r.Heading, "WLNK") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected WLNK section in results")
	}
}

func TestSearchSpec_RespectsLimit(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	results, err := SearchSpec(db, "the", 2)
	if err != nil {
		t.Fatalf("SearchSpec failed: %v", err)
	}

	if len(results) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(results))
	}
}

func TestSearchSpec_NoResults(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	results, err := SearchSpec(db, "xyznonexistent", 10)
	if err != nil {
		t.Fatalf("SearchSpec failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for nonsense query, got %d", len(results))
	}
}

func TestSearchSpec_DefaultLimit(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	// limit=0 should default to 10
	results, err := SearchSpec(db, "the", 0)
	if err != nil {
		t.Fatalf("SearchSpec failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected results with default limit")
	}
}

func TestSearchSpec_SnippetReturned(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	results, err := SearchSpec(db, "WLNK", 1)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Fatal("expected results")
	}

	// Snippet should contain some text from the matching section
	if results[0].Snippet == "" {
		t.Error("snippet should not be empty")
	}
	if !strings.Contains(results[0].Snippet, "weakest") && !strings.Contains(results[0].Snippet, "link") {
		t.Errorf("snippet should contain relevant text, got: %s", results[0].Snippet)
	}
}

func TestGetSpecSection_ExactMatch(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	body, err := GetSpecSection(db, "3. ADI Cycle")
	if err != nil {
		t.Fatalf("GetSpecSection failed: %v", err)
	}

	if !strings.Contains(body, "Abduction") {
		t.Errorf("body should contain 'Abduction', got: %s", body)
	}
}

func TestGetSpecSection_NotFound(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	_, err := GetSpecSection(db, "Nonexistent Section")
	if err == nil {
		t.Error("expected error for nonexistent section")
	}
}

func TestSetSpecMeta_AndGetSpecMeta(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	val, err := GetSpecMeta(db, "fpf_commit")
	if err != nil {
		t.Fatalf("GetSpecMeta failed: %v", err)
	}

	if val != "abc1234" {
		t.Errorf("expected 'abc1234', got %q", val)
	}
}

func TestGetSpecMeta_NotFound(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	_, err := GetSpecMeta(db, "nonexistent_key")
	if err == nil {
		t.Error("expected error for nonexistent meta key")
	}
}

func TestSearchSpec_PrefixMatching(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	// "hypothes" should match "hypotheses" via prefix matching
	results, err := SearchSpec(db, "hypothes", 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Error("expected prefix match for 'hypothes' → 'hypotheses'")
	}
}

func TestSearchSpec_StemMatching(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	// porter stemmer: "predictions" should match "prediction"
	results, err := SearchSpec(db, "predictions", 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Error("expected stem match for 'predictions'")
	}
}
