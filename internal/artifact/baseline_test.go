package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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

func TestBaselineBindingFailurePreservesPriorBaseline(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "app.go", `package main

func Run() string {
	return "run"
}
`)

	dec := createTestDecision(t, store, "dec-test-002b", "Storage choice")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "app.go"}}); err != nil {
		t.Fatal(err)
	}

	result, err := Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: dec.Meta.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("baseline files = %+v, want one", result)
	}
	priorHash := result[0].Hash

	artifactBefore, err := store.Get(ctx, dec.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	fieldsBefore := artifactBefore.UnmarshalDecisionFields()
	if len(fieldsBefore.BindingTargets) != 1 || fieldsBefore.BindingTargets[0].SymbolName != "Run" {
		t.Fatalf("initial binding targets = %+v, want Run", fieldsBefore.BindingTargets)
	}

	writeTestFile(t, projectRoot, "app.go", `package main

func Run() string {
	return "run"
}

func Stop() string {
	return "stop"
}
`)

	_, err = Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: dec.Meta.ID})
	if err == nil {
		t.Fatal("expected ambiguous binding resolution to fail")
	}
	if !strings.Contains(err.Error(), "multiple parseable symbols") {
		t.Fatalf("error = %q, want multiple parseable symbols", err.Error())
	}

	storedFiles, err := store.GetAffectedFiles(ctx, dec.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(storedFiles) != 1 {
		t.Fatalf("stored files = %+v, want one", storedFiles)
	}
	if storedFiles[0].Hash != priorHash {
		t.Fatalf("stored hash = %q, want prior hash %q", storedFiles[0].Hash, priorHash)
	}

	artifactAfter, err := store.Get(ctx, dec.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	fieldsAfter := artifactAfter.UnmarshalDecisionFields()
	if len(fieldsAfter.BindingTargets) != 1 || fieldsAfter.BindingTargets[0].SymbolName != "Run" {
		t.Fatalf("binding targets = %+v, want prior Run target", fieldsAfter.BindingTargets)
	}
	if len(fieldsAfter.BindingDiagnostics) == 0 {
		t.Fatal("expected binding diagnostics to be persisted")
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

	files, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef: dec.Meta.ID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v (missing files should be skipped, not fatal)", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 baselined files (missing files skipped), got %d", len(files))
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
	if r.BaselineKind != BaselineKindVerifiedStateSnapshot {
		t.Fatalf("baseline_kind = %q, want verified-state snapshot", r.BaselineKind)
	}
	if r.BaselineProfile == nil {
		t.Fatal("baseline_profile missing")
	}
	if r.BaselineProfile.AuthorityBoundary != "drift_detection_snapshot_not_spec_approval_or_pre_work_reference" {
		t.Fatalf("baseline_profile = %+v", r.BaselineProfile)
	}
	if len(r.Files) != 1 {
		t.Fatalf("expected 1 drifted file, got %d", len(r.Files))
	}
	if r.Files[0].Status != DriftModified {
		t.Errorf("expected DriftModified, got %s", r.Files[0].Status)
	}
}

func TestCheckDriftClassifiesAddedSymbolsAsAdjacentChurn(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "app.go", `package main

func Run() string {
	return "run"
}
`)

	dec := createTestDecision(t, store, "dec-test-010b", "App runner")
	err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "app.go"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: dec.Meta.ID})
	if err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "app.go", `package main

var enabled = true

func Run() string {
	return "run"
}

func Extra() string {
	return "extra"
}
`)

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 drift report, got %d", len(reports))
	}

	file := reports[0].Files[0]
	if file.Status != DriftModified {
		t.Fatalf("status = %s, want %s", file.Status, DriftModified)
	}
	if file.Materiality != DriftMaterialityAdjacentFileChurn {
		t.Fatalf("materiality = %s, want %s", file.Materiality, DriftMaterialityAdjacentFileChurn)
	}
	if !file.AuditOnly {
		t.Fatal("added-symbol churn should be audit-only in compact status")
	}
	if len(file.Symbols) != 1 || file.Symbols[0].Status != "added" || file.Symbols[0].SymbolName != "Extra" {
		t.Fatalf("symbols = %+v, want one added Extra symbol", file.Symbols)
	}
	if got := reports[0].SymbolVerdict(); got != SymbolVerdictAdditiveOnly {
		t.Fatalf("SymbolVerdict() = %q, want %q", got, SymbolVerdictAdditiveOnly)
	}
}

func TestCheckDriftClassifiesUnchangedGovernedSymbolsAsAdjacentChurn(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "app.go", `package main

func Run() string {
	return "run"
}
`)

	dec := createTestDecision(t, store, "dec-test-010c", "App runner")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "app.go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: dec.Meta.ID}); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "app.go", `package main

var enabled = true

func Run() string {
	return "run"
}
`)

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 drift report, got %d", len(reports))
	}

	file := reports[0].Files[0]
	if file.Materiality != DriftMaterialityAdjacentFileChurn {
		t.Fatalf("materiality = %s, want %s", file.Materiality, DriftMaterialityAdjacentFileChurn)
	}
	if !file.AuditOnly {
		t.Fatal("unchanged-symbol file churn should be audit-only")
	}
	if len(file.Symbols) != 0 {
		t.Fatalf("symbols = %+v, want no changed governed symbols", file.Symbols)
	}
	if got := reports[0].SymbolVerdict(); got != SymbolVerdictAdditiveOnly {
		t.Fatalf("SymbolVerdict() = %q, want %q", got, SymbolVerdictAdditiveOnly)
	}
}

func TestAssessModifiedFileDriftClassifiesGeneratedAndCarrierPaths(t *testing.T) {
	cases := []struct {
		path string
		want DriftMateriality
	}{
		{path: "internal/cli/fpf.db", want: DriftMaterialityGeneratedOrIgnored},
		{path: "data/FPF", want: DriftMaterialityGeneratedOrIgnored},
		{path: "embed-sidecar/target/debug/libhaft_embed.a", want: DriftMaterialityGeneratedOrIgnored},
		{path: "CHANGELOG.md", want: DriftMaterialityCarrierOnly},
		{path: ".context/current.plan", want: DriftMaterialityCarrierOnly},
		{path: ".haft/specs/target-system.md", want: DriftMaterialityCarrierOnly},
		{path: "internal/cli/skill/h-frame/SKILL.md", want: DriftMaterialityCarrierOnly},
		{path: "open-sleigh/.haft/project.yaml", want: DriftMaterialityCarrierOnly},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got := assessModifiedFileDrift("", tc.path, nil)
			if got.Materiality != tc.want {
				t.Fatalf("materiality = %s, want %s", got.Materiality, tc.want)
			}
			if !got.AuditOnly {
				t.Fatal("generated/carrier drift should be audit-only")
			}
		})
	}
}

func TestAssessModifiedFileDriftDoesNotTreatEveryTextFileAsCarrier(t *testing.T) {
	got := assessModifiedFileDrift("", "notes.txt", nil)

	if got.Materiality == DriftMaterialityCarrierOnly {
		t.Fatal("plain text files should not be globally treated as carrier-only")
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

func TestCheckDriftDetectsAddedFileInGovernedScope(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "pkg/base.go", "package pkg\n")

	dec := createTestDecision(t, store, "dec-test-014", "Governed package")
	setDecisionInvariants(t, ctx, store, dec.Meta.ID, []string{"Preserve the pkg package boundary"})

	err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "pkg/base.go"}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: dec.Meta.ID})
	if err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "pkg/extra.go", "package pkg\n")

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}

	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}

	files := reports[0].Files
	if len(files) != 1 {
		t.Fatalf("expected 1 drift item, got %d", len(files))
	}
	if files[0].Path != "pkg/extra.go" {
		t.Fatalf("drift path = %q, want pkg/extra.go", files[0].Path)
	}
	if files[0].Status != DriftAdded {
		t.Fatalf("drift status = %s, want %s", files[0].Status, DriftAdded)
	}
	if !reflect.DeepEqual(files[0].Invariants, []string{"Preserve the pkg package boundary"}) {
		t.Fatalf("invariants = %#v, want decision invariants", files[0].Invariants)
	}
}

func TestCheckDriftDetectsAddedNestedFileFromRootScope(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "README.md", "# governed root\n")

	dec := createTestDecision(t, store, "dec-test-015", "Governed repository root")
	err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "README.md"}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: dec.Meta.ID})
	if err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "pkg/nested/new.go", "package nested\n")

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}

	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}

	files := reports[0].Files
	if len(files) != 1 {
		t.Fatalf("expected 1 drift item, got %d", len(files))
	}
	if files[0].Path != "pkg/nested/new.go" {
		t.Fatalf("drift path = %q, want pkg/nested/new.go", files[0].Path)
	}
	if files[0].Status != DriftAdded {
		t.Fatalf("drift status = %s, want %s", files[0].Status, DriftAdded)
	}
}

func TestCheckDriftIgnoresAddedFilesExcludedByGitignore(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, ".gitignore", "generated/\n")
	writeTestFile(t, projectRoot, "README.md", "# governed root\n")

	dec := createTestDecision(t, store, "dec-test-016", "Governed root with ignores")
	err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "README.md"}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: dec.Meta.ID})
	if err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "generated/output.txt", "ignored artifact\n")

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 0 {
		t.Fatalf("expected ignored generated file to stay out of drift, got %#v", reports)
	}
}

func TestCheckDriftIgnoresAddedFilesExcludedByNestedGitignore(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	initTestGitRepository(t, projectRoot)

	writeTestFile(t, projectRoot, "embed-sidecar/.gitignore", "/target\n")
	writeTestFile(t, projectRoot, "README.md", "# governed root\n")

	dec := createTestDecision(t, store, "dec-test-016b", "Governed root with nested ignores")
	err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "README.md"}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: dec.Meta.ID})
	if err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "embed-sidecar/target/release/libhaft_embed.a", "ignored build artifact\n")

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 0 {
		t.Fatalf("expected nested-gitignored build artifact to stay out of drift, got %#v", reports)
	}
}

func TestCheckDriftIgnoresAddedRuntimeCarrierDirectories(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	initTestGitRepository(t, projectRoot)

	writeTestFile(t, projectRoot, "README.md", "# governed root\n")

	dec := createTestDecision(t, store, "dec-test-016c", "Governed root with runtime carrier")
	err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "README.md"}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: dec.Meta.ID})
	if err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "open-sleigh/.haft/project.yaml", "project_id: local-runtime\n")

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 0 {
		t.Fatalf("expected runtime carrier directory to stay out of drift, got %#v", reports)
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

func TestCheckDriftIncludesOldActiveDecisionBeyondFiveHundred(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	writeTestFile(t, projectRoot, "drift-old.go", "package orig\n")

	oldDecision := createTestDecisionAt(t, store, "dec-test-500-old", "Old drift", createdAt)
	err := store.SetAffectedFiles(ctx, oldDecision.Meta.ID, []AffectedFile{{Path: "drift-old.go"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: oldDecision.Meta.ID})
	if err != nil {
		t.Fatal(err)
	}

	for index := 0; index < 500; index++ {
		id := fmt.Sprintf("dec-test-500-new-%03d", index)
		title := fmt.Sprintf("Newer decision %03d", index)
		newerCreatedAt := createdAt.Add(time.Duration(index+1) * time.Second)
		createTestDecisionAt(t, store, id, title, newerCreatedAt)
	}

	writeTestFile(t, projectRoot, "drift-old.go", "package changed\n")

	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !containsDriftReport(reports, oldDecision.Meta.ID) {
		t.Fatalf("expected drift report for %s beyond 500 newer decisions", oldDecision.Meta.ID)
	}

	items, err := ScanStale(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !containsStaleItem(items, oldDecision.Meta.ID) {
		t.Fatalf("expected stale scan to include %s beyond 500 newer decisions", oldDecision.Meta.ID)
	}
}

// TestFormatDriftResponse_LikelyImplemented moved to internal/present/format_test.go

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

func initTestGitRepository(t *testing.T, root string) {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "init")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git init: %v\n%s", err, string(output))
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

	return createTestDecisionAt(t, store, id, title, time.Time{})
}

func createTestDecisionAt(t *testing.T, store *Store, id, title string, createdAt time.Time) *Artifact {
	t.Helper()

	a := &Artifact{
		Meta: Meta{
			ID:        id,
			Kind:      KindDecisionRecord,
			Title:     title,
			Status:    StatusActive,
			CreatedAt: createdAt,
		},
		Body: "# " + title + "\n\nTest decision.\n",
	}
	if err := store.Create(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	return a
}

func setDecisionInvariants(t *testing.T, ctx context.Context, store *Store, decisionID string, invariants []string) {
	t.Helper()

	artifact, err := store.Get(ctx, decisionID)
	if err != nil {
		t.Fatal(err)
	}

	fields := artifact.UnmarshalDecisionFields()
	fields.Invariants = invariants

	data, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}

	artifact.StructuredData = string(data)

	err = store.Update(ctx, artifact)
	if err != nil {
		t.Fatal(err)
	}
}

func containsDriftReport(reports []DriftReport, decisionID string) bool {
	for _, report := range reports {
		if report.DecisionID == decisionID {
			return true
		}
	}

	return false
}

func containsStaleItem(items []StaleItem, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}

	return false
}
