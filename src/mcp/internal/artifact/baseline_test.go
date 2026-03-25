package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBaselineStoresHashes(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	// Create test files
	writeTestFile(t, projectRoot, "src/main.go", "package main\nfunc main() {}\n")
	writeTestFile(t, projectRoot, "src/util.go", "package main\nfunc helper() {}\n")
	writeTestFile(t, projectRoot, "README.md", "# Hello\n")

	// Create a decision with affected files (no hashes)
	dec := createTestDecision(t, store, "dec-test-001", "Use Redis")
	files := []AffectedFile{
		{Path: "src/main.go"},
		{Path: "src/util.go"},
		{Path: "README.md"},
	}
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, files); err != nil {
		t.Fatal(err)
	}

	// Baseline should compute and store SHA-256
	result, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef: dec.Meta.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 files, got %d", len(result))
	}

	// Verify hashes are correct
	for _, f := range result {
		if f.Hash == "" {
			t.Errorf("file %s has empty hash", f.Path)
			continue
		}
		expected := hashTestFile(t, projectRoot, f.Path)
		if f.Hash != expected {
			t.Errorf("file %s: hash %s != expected %s", f.Path, f.Hash, expected)
		}
	}

	// Verify hashes persisted to DB
	stored, err := store.GetAffectedFiles(ctx, dec.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range stored {
		if f.Hash == "" {
			t.Errorf("stored file %s has empty hash", f.Path)
		}
	}
}

func TestBaselineWithReplacedFiles(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "old.go", "package old\n")
	writeTestFile(t, projectRoot, "new.go", "package new\n")
	writeTestFile(t, projectRoot, "also-new.go", "package also\n")

	// Create decision with old file
	dec := createTestDecision(t, store, "dec-test-002", "Old approach")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "old.go"}}); err != nil {
		t.Fatal(err)
	}

	// Baseline with NEW files — should replace list and hash
	result, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef:   dec.Meta.ID,
		AffectedFiles: []string{"new.go", "also-new.go"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 files, got %d", len(result))
	}

	// Verify old.go is gone, new files are there
	stored, err := store.GetAffectedFiles(ctx, dec.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, f := range stored {
		paths[f.Path] = true
	}
	if paths["old.go"] {
		t.Error("old.go should have been replaced")
	}
	if !paths["new.go"] || !paths["also-new.go"] {
		t.Error("new files should be present")
	}
}

func TestBaselineFailsOnMissingFile(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	dec := createTestDecision(t, store, "dec-test-003", "Ghost files")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "nonexistent.go"}}); err != nil {
		t.Fatal(err)
	}

	_, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef: dec.Meta.ID,
	})
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestBaselineFailsWithNoFiles(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	dec := createTestDecision(t, store, "dec-test-004", "No files")

	_, err := Baseline(ctx, store, t.TempDir(), BaselineInput{
		DecisionRef: dec.Meta.ID,
	})
	if err == nil {
		t.Fatal("expected error for no affected files, got nil")
	}
}

func TestCheckDriftDetectsModifiedFile(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "app.go", "package main\nfunc Run() {}\n")

	// Create decision and baseline
	dec := createTestDecision(t, store, "dec-test-010", "App runner")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "app.go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: dec.Meta.ID}); err != nil {
		t.Fatal(err)
	}

	// Modify the file
	writeTestFile(t, projectRoot, "app.go", "package main\nfunc Run() { fmt.Println(\"changed\") }\n")

	// Check drift
	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}

	if len(reports) != 1 {
		t.Fatalf("expected 1 drift report, got %d", len(reports))
	}

	r := reports[0]
	if r.DecisionID != dec.Meta.ID {
		t.Errorf("expected decision %s, got %s", dec.Meta.ID, r.DecisionID)
	}
	if !r.HasBaseline {
		t.Error("expected HasBaseline=true")
	}
	if len(r.Files) != 1 {
		t.Fatalf("expected 1 drifted file, got %d", len(r.Files))
	}
	if r.Files[0].Status != DriftModified {
		t.Errorf("expected DriftModified, got %s", r.Files[0].Status)
	}
}

func TestCheckDriftDetectsDeletedFile(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "temp.go", "package temp\n")

	// Create decision and baseline
	dec := createTestDecision(t, store, "dec-test-011", "Temp file")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "temp.go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: dec.Meta.ID}); err != nil {
		t.Fatal(err)
	}

	// Delete the file
	os.Remove(filepath.Join(projectRoot, "temp.go"))

	// Check drift
	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}

	if len(reports) != 1 {
		t.Fatalf("expected 1 drift report, got %d", len(reports))
	}
	if reports[0].Files[0].Status != DriftMissing {
		t.Errorf("expected DriftMissing, got %s", reports[0].Files[0].Status)
	}
}

func TestCheckDriftNoDriftWhenUnchanged(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "stable.go", "package stable\n")

	dec := createTestDecision(t, store, "dec-test-012", "Stable file")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "stable.go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: dec.Meta.ID}); err != nil {
		t.Fatal(err)
	}

	// No changes — should have no drift
	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}

	if len(reports) != 0 {
		t.Fatalf("expected 0 drift reports, got %d", len(reports))
	}
}

func TestCheckDriftReportsNoBaseline(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	// Create decision with affected files but NO baseline
	dec := createTestDecision(t, store, "dec-test-013", "Unbaselined")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "some.go"}}); err != nil {
		t.Fatal(err)
	}

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}

	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	if reports[0].HasBaseline {
		t.Error("expected HasBaseline=false")
	}
	if reports[0].Files[0].Status != DriftNoBaseline {
		t.Errorf("expected DriftNoBaseline, got %s", reports[0].Files[0].Status)
	}
}

func TestScanStaleIncludesDrift(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "drifted.go", "package orig\n")

	dec := createTestDecision(t, store, "dec-test-020", "Will drift")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "drifted.go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: dec.Meta.ID}); err != nil {
		t.Fatal(err)
	}

	// Modify
	writeTestFile(t, projectRoot, "drifted.go", "package changed\n")

	// ScanStale with projectRoot should include drift
	items, err := ScanStale(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, item := range items {
		if item.ID == dec.Meta.ID {
			found = true
			if item.Reason == "" {
				t.Error("expected drift reason")
			}
		}
	}
	if !found {
		t.Error("expected drifted decision in ScanStale results")
	}
}

func TestFormatDriftResponse_LikelyImplemented(t *testing.T) {
	reports := []DriftReport{
		{
			DecisionID:        "dec-001",
			DecisionTitle:     "Implemented decision",
			HasBaseline:       false,
			LikelyImplemented: true,
			Files:             []DriftItem{{Path: "app.go", Status: DriftNoBaseline}},
		},
		{
			DecisionID:    "dec-002",
			DecisionTitle: "Not started decision",
			HasBaseline:   false,
			Files:         []DriftItem{{Path: "other.go", Status: DriftNoBaseline}},
		},
	}

	output := FormatDriftResponse(reports, "")

	if !strings.Contains(output, "files changed since decision") {
		t.Errorf("should flag likely-implemented decision:\n%s", output)
	}
	if !strings.Contains(output, "likely implemented") {
		t.Errorf("should say 'likely implemented':\n%s", output)
	}
	if !strings.Contains(output, "files unchanged") {
		t.Errorf("should say 'files unchanged' for not-started:\n%s", output)
	}
	if !strings.Contains(output, "not yet implemented") {
		t.Errorf("should say 'not yet implemented':\n%s", output)
	}
}

func TestCheckDriftReportsNoBaseline_LikelyImplementedFalseWithoutGit(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir() // not a git repo

	dec := createTestDecision(t, store, "dec-test-li", "No git")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "some.go"}}); err != nil {
		t.Fatal(err)
	}

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	if reports[0].LikelyImplemented {
		t.Error("expected LikelyImplemented=false when git is not available")
	}
}

// --- test helpers ---

func writeTestFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hashTestFile(t *testing.T, root, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func createTestDecision(t *testing.T, store *Store, id, title string) *Artifact {
	t.Helper()
	a := &Artifact{
		Meta: Meta{
			ID:     id,
			Kind:   KindDecisionRecord,
			Title:  title,
			Status: StatusActive,
		},
		Body: "# " + title + "\n\nTest decision.\n",
	}
	if err := store.Create(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	return a
}
