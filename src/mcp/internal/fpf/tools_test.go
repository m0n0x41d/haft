package fpf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/quint-code/db"
)

// Helper to create a dummy Tools instance for testing
func setupTools(t *testing.T) (*Tools, *FSM, string) {
	tempDir := t.TempDir()
	quintDir := filepath.Join(tempDir, ".quint")
	if err := os.MkdirAll(quintDir, 0755); err != nil { // Ensure .quint exists
		t.Fatalf("Failed to create .quint directory: %v", err)
	}

	// Create a dummy DB file
	dbPath := filepath.Join(quintDir, "quint.db")
	database, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize DB: %v", err)
	}

	fsm := &FSM{State: State{Phase: PhaseIdle}, DB: database.GetRawDB()} // Initial FSM state with DB

	tools := NewTools(fsm, tempDir, database)

	// Initialize the project structure for tools to operate
	err = tools.InitProject()
	if err != nil {
		t.Fatalf("Failed to initialize project: %v", err)
	}

	return tools, fsm, tempDir
}

func TestSlugify(t *testing.T) {

	tools, _, _ := setupTools(t)
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"Another_Test-Case", "another-test-case"},
		{"123 FPF Hypo!", "123-fpf-hypo"},
		{"  leading and trailing   ", "leading-and-trailing"},
		{"-dash-start-and-end-", "dash-start-and-end"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Input:%s", tt.input), func(t *testing.T) {
			result := tools.Slugify(tt.input)
			if result != tt.expected {
				t.Errorf("slugify(%q) got %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestInitProject(t *testing.T) {
	_, _, tempDir := setupTools(t) // setupTools already calls InitProject

	expectedDirs := []string{
		"evidence", "decisions", "sessions",
		"knowledge/L0", "knowledge/L1", "knowledge/L2", "knowledge/invalid",
	}

	for _, d := range expectedDirs {
		path := filepath.Join(tempDir, ".quint", d)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Directory %s was not created", path)
		}
		gitkeepPath := filepath.Join(path, ".gitkeep")
		if _, err := os.Stat(gitkeepPath); os.IsNotExist(err) {
			t.Errorf(".gitkeep file in %s was not created", path)
		}
	}
}

func TestProposeHypothesis(t *testing.T) {

	tools, fsm, tempDir := setupTools(t)
	fsm.State.Phase = PhaseAbduction // Set phase for valid Propose

	title := "My First Hypothesis"
	content := "This is the content of my hypothesis."
	scope := "global"
	kind := "system"
	rationale := "This is the rationale."

	path, err := tools.ProposeHypothesis(title, content, scope, kind, rationale, "", nil, 3)
	if err != nil {
		t.Fatalf("ProposeHypothesis failed: %v", err)
	}

	expectedFile := filepath.Join(tempDir, ".quint", "knowledge", "L0", "my-first-hypothesis.md")
	if path != expectedFile {
		t.Errorf("Returned path %q, expected %q", path, expectedFile)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Hypothesis file was not created at %s", path)
	}

	readContent, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read hypothesis file: %v", err)
	}
	contentStr := string(readContent)
	if !strings.Contains(contentStr, "scope: "+scope) {
		t.Errorf("Missing scope in frontmatter")
	}
	if !strings.Contains(contentStr, "kind: "+kind) {
		t.Errorf("Missing kind in frontmatter")
	}
	if !strings.Contains(contentStr, "content_hash:") {
		t.Errorf("Missing content_hash in frontmatter")
	}
	if !strings.Contains(contentStr, "# Hypothesis: "+title) {
		t.Errorf("Missing hypothesis title in body")
	}
	if !strings.Contains(contentStr, content) {
		t.Errorf("Missing content in body")
	}
	if !strings.Contains(contentStr, "## Rationale") {
		t.Errorf("Missing rationale section")
	}
}

func TestManageEvidence(t *testing.T) {

	tools, fsm, tempDir := setupTools(t)
	hypoID := "test-hypo"
	hypoPath := filepath.Join(tempDir, ".quint", "knowledge", "L0", hypoID+".md")
	if err := os.WriteFile(hypoPath, []byte("Hypothesis content"), 0644); err != nil {
		t.Fatalf("Failed to create dummy hypothesis file: %v", err)
	}

	tests := []struct {
		name              string
		currentPhase      Phase
		targetID          string
		evidenceType      string
		content           string
		verdict           string
		assuranceLevel    string // New field
		expectedMove      bool
		expectedDestLevel string // e.g., "L1", "L2", "invalid"
		expectErr         bool
	}{
		// Deductor (DEDUCTION phase)
		{"DeductionPass", PhaseDeduction, hypoID, "logic", "Logic check passed.", "PASS", "L1", true, "L1", false},
		{"DeductionFail", PhaseDeduction, hypoID, "logic", "Logic check failed.", "FAIL", "L1", true, "invalid", false},
		{"DeductionRefine", PhaseDeduction, hypoID, "logic", "Needs more refinement.", "REFINE", "L1", true, "invalid", false},

		// Inductor (INDUCTION phase) - need another hypo in L1
		{"InductionPass", PhaseInduction, "hypo-L1", "empirical", "Experiment passed.", "PASS", "L2", true, "L2", false},
		{"InductionFail", PhaseInduction, "hypo-L1", "empirical", "Experiment failed.", "FAIL", "L2", true, "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure FSM is in correct phase
			fsm.State.Phase = tt.currentPhase

			// Declare srcLevel outside conditional blocks to have proper scope
			var srcLevel string

			// Prepare for move if needed (create dummy hypo in source level)
			if tt.expectedMove {
				switch tt.currentPhase {
				case PhaseInduction:
					srcLevel = "L1"
					// Create dummy L1 hypo for induction tests
					hypoL1Path := filepath.Join(tempDir, ".quint", "knowledge", "L1", tt.targetID+".md")
					if err := os.WriteFile(hypoL1Path, []byte("L1 Hypothesis content"), 0644); err != nil {
						t.Fatalf("Failed to create dummy L1 hypothesis file: %v", err)
					}
				case PhaseDeduction: // Use else if for correct logic
					srcLevel = "L0"
					// Create dummy L0 hypo for deduction tests
					hypoL0Path := filepath.Join(tempDir, ".quint", "knowledge", "L0", tt.targetID+".md")
					if err := os.WriteFile(hypoL0Path, []byte("L0 Hypothesis content"), 0644); err != nil {
						t.Fatalf("Failed to create dummy L0 hypothesis file: %v", err)
					}
				}
			}

			evidencePath, err := tools.ManageEvidence(tt.currentPhase, "add", tt.targetID, tt.evidenceType, tt.content, tt.verdict, tt.assuranceLevel, "file://carrier", "2025-12-31")

			if (err != nil) != tt.expectErr {
				t.Errorf("ManageEvidence() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if tt.expectErr {
				return
			}

			// Verify evidence file creation
			if _, err := os.Stat(evidencePath); os.IsNotExist(err) {
				t.Errorf("Evidence file was not created at %s", evidencePath)
			}

			// Verify hypothesis move
			if tt.expectedMove {
				expectedDestPath := filepath.Join(tempDir, ".quint", "knowledge", tt.expectedDestLevel, tt.targetID+".md")
				if _, err := os.Stat(expectedDestPath); os.IsNotExist(err) {
					t.Errorf("Hypothesis %s was not moved to %s. Expected path: %s", tt.targetID, tt.expectedDestLevel, expectedDestPath)
				}
				// Also check it's gone from source level
				// Deductor works on L0, Inductor on L1
				// sourceLevel is already correctly set by srcLevel in this context
				// srcLevel is already correctly set by srcLevel in this context
				srcOldPath := filepath.Join(tempDir, ".quint", "knowledge", srcLevel, tt.targetID+".md")
				if _, err := os.Stat(srcOldPath); err == nil {
					t.Errorf("Hypothesis %s was not removed from source level %s", tt.targetID, srcLevel)
				}
			}
		})
	}
}

func TestRefineLoopback(t *testing.T) {

	tools, fsm, tempDir := setupTools(t)
	parentID := "parent-hypo"
	parentPath := filepath.Join(tempDir, ".quint", "knowledge", "L1", parentID+".md") // Assume L1 for Induction -> Deduction
	if err := os.WriteFile(parentPath, []byte("Parent Hypothesis content"), 0644); err != nil {
		t.Fatalf("Failed to create dummy parent hypothesis file: %v", err)
	}

	fsm.State.Phase = PhaseInduction // Simulate coming from Induction

	insight := "New insight from failure"
	newTitle := "Refined Child Hypothesis"
	newContent := "This is the refined content."
	scope := "system"

	childPath, err := tools.RefineLoopback(fsm.State.Phase, parentID, insight, newTitle, newContent, scope)
	if err != nil {
		t.Fatalf("RefineLoopback failed: %v", err)
	}

	// Verify parent moved to invalid
	invalidParentPath := filepath.Join(tempDir, ".quint", "knowledge", "invalid", parentID+".md")
	if _, err := os.Stat(invalidParentPath); os.IsNotExist(err) {
		t.Errorf("Parent hypothesis %s was not moved to invalid", parentID)
	}

	// Verify child created in L0
	expectedChildPath := filepath.Join(tempDir, ".quint", "knowledge", "L0", "refined-child-hypothesis.md")
	if childPath != expectedChildPath {
		t.Errorf("Returned child path %q, expected %q", childPath, expectedChildPath)
	}
	if _, err := os.Stat(childPath); os.IsNotExist(err) {
		t.Errorf("Child hypothesis file was not created at %s", childPath)
	}

	// Verify log file created
	sessionDir := filepath.Join(tempDir, ".quint", "sessions")
	matches, err := filepath.Glob(filepath.Join(sessionDir, "loopback-*.md"))
	if err != nil || len(matches) == 0 {
		t.Errorf("Loopback log file was not created")
	}
}

func TestFinalizeDecision(t *testing.T) {

	tools, fsm, tempDir := setupTools(t)
	fsm.State.Phase = PhaseDecision // Simulate being in Decision phase

	winnerID := "final-winner"
	winnerPath := filepath.Join(tempDir, ".quint", "knowledge", "L1", winnerID+".md") // Assume winner is in L1
	if err := os.WriteFile(winnerPath, []byte("Winner Hypothesis Content"), 0644); err != nil {
		t.Fatalf("Failed to create dummy winner hypothesis file: %v", err)
	}

	title := "Final Project Decision"
	content := "This is the DRR content for the decision."

	drrPath, err := tools.FinalizeDecision(title, winnerID, nil, "Context", content, "Rationale", "Consequences", "Characteristics")
	if err != nil {
		t.Fatalf("FinalizeDecision failed: %v", err)
	}

	// Verify DRR file creation
	drrPattern := filepath.Join(tempDir, ".quint", "decisions", fmt.Sprintf("DRR-*-%s.md", tools.Slugify(title)))
	matches, err := filepath.Glob(drrPattern)
	if err != nil {
		t.Fatalf("Failed to glob for DRR file: %v", err)
	}
	if len(matches) == 0 {
		t.Errorf("DRR file was not created with expected pattern")
	}
	// Check if the returned path is one of the matched paths
	found := false
	for _, match := range matches {
		if match == drrPath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Returned DRR path %q does not match any expected pattern %q", drrPath, drrPattern)
	}

	// Verify winner moved to L2
	expectedWinnerL2Path := filepath.Join(tempDir, ".quint", "knowledge", "L2", winnerID+".md")
	if _, err := os.Stat(expectedWinnerL2Path); os.IsNotExist(err) {
		t.Errorf("Winner hypothesis %s was not moved to L2", winnerID)
	}
	// Verify it's gone from L1
	if _, err := os.Stat(winnerPath); err == nil {
		t.Errorf("Winner hypothesis %s was not removed from L1", winnerID)
	}
}

func TestVerifyHypothesis(t *testing.T) {

	tools, fsm, tempDir := setupTools(t)
	hypoID := "test-verify-hypo"

	// Create dummy L0 hypothesis
	hypoPath := filepath.Join(tempDir, ".quint", "knowledge", "L0", hypoID+".md")
	if err := os.WriteFile(hypoPath, []byte("L0 content"), 0644); err != nil {
		t.Fatalf("Failed to create dummy L0 hypothesis: %v", err)
	}

	// Case 1: PASS -> Promote to L1
	fsm.State.Phase = PhaseDeduction
	msg, err := tools.VerifyHypothesis(hypoID, `{"check":"ok"}`, "PASS")
	if err != nil {
		t.Errorf("VerifyHypothesis(PASS) failed: %v", err)
	}
	if !strings.Contains(msg, "promoted to L1") {
		t.Errorf("Expected message to contain 'promoted to L1', got %q", msg)
	}
	if _, err := os.Stat(filepath.Join(tempDir, ".quint", "knowledge", "L1", hypoID+".md")); os.IsNotExist(err) {
		t.Errorf("Hypothesis not moved to L1")
	}

	// Case 2: FAIL -> Move to invalid
	// Setup another L0 hypo
	hypoID2 := "test-fail-hypo"
	hypoPath2 := filepath.Join(tempDir, ".quint", "knowledge", "L0", hypoID2+".md")
	if err := os.WriteFile(hypoPath2, []byte("L0 content"), 0644); err != nil {
		t.Fatalf("Failed to create dummy L0 hypothesis 2: %v", err)
	}

	msg, err = tools.VerifyHypothesis(hypoID2, `{"check":"bad"}`, "FAIL")
	if err != nil {
		t.Errorf("VerifyHypothesis(FAIL) failed: %v", err)
	}
	expectedMsgFail := fmt.Sprintf("Hypothesis %s moved to invalid", hypoID2)
	if msg != expectedMsgFail {
		t.Errorf("Expected message %q, got %q", expectedMsgFail, msg)
	}
	if _, err := os.Stat(filepath.Join(tempDir, ".quint", "knowledge", "invalid", hypoID2+".md")); os.IsNotExist(err) {
		t.Errorf("Hypothesis not moved to invalid")
	}
}

func TestAuditEvidence(t *testing.T) {

	tools, fsm, _ := setupTools(t)
	fsm.State.Phase = PhaseDecision // Audit typically happens near decision or end of induction

	hypoID := "audit-hypo"
	// Create dummy L1 or L2 hypothesis (Audit doesn't strictly check existence in file system for the call itself,
	// but ManageEvidence might rely on DB. For unit test, we focus on the wrapper call.)

	// AuditEvidence calls ManageEvidence with PhaseDecision.
	// ManageEvidence checks DB if action is "check", but here action is "add" (implied).
	// We need to ensure DB is happy if it checks constraints.

	// In tools.go, AuditEvidence calls:
	// t.ManageEvidence(PhaseDecision, "add", hypothesisID, "audit_report", risks, "PASS", "L2", "auditor", "")

	msg, err := tools.AuditEvidence(hypoID, "Risk analysis content")
	if err != nil {
		t.Errorf("AuditEvidence failed: %v", err)
	}
	expectedMsg := "Audit recorded for " + hypoID
	if msg != expectedMsg {
		t.Errorf("Expected message %q, got %q", expectedMsg, msg)
	}

	// We could verify DB side effects if we exposed DB in tests more directly,
	// but for now we verify no error and correct return message.
}

func TestCalculateR(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a holon with evidence
	err := tools.DB.CreateHolon(ctx, "calc-r-test", "hypothesis", "system", "L1", "Test Holon", "Content", "ctx", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	// Add passing evidence
	err = tools.DB.AddEvidence(ctx, "e1", "calc-r-test", "test", "Test passed", "pass", "L1", "test-runner", "2099-12-31")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	// Calculate R
	result, err := tools.CalculateR("calc-r-test")
	if err != nil {
		t.Fatalf("CalculateR failed: %v", err)
	}

	// Verify output contains expected elements
	if !strings.Contains(result, "Reliability Report") {
		t.Errorf("Expected 'Reliability Report' in output, got: %s", result)
	}
	if !strings.Contains(result, "R_eff:") {
		t.Errorf("Expected 'R_eff:' in output, got: %s", result)
	}
	if !strings.Contains(result, "1.00") {
		t.Errorf("Expected R_eff of 1.00 for passing evidence, got: %s", result)
	}
}

func TestCalculateR_WithDecay(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a holon with expired evidence
	err := tools.DB.CreateHolon(ctx, "decay-r-test", "hypothesis", "system", "L1", "Decay Test", "Content", "ctx", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	// Add expired evidence (past date)
	err = tools.DB.AddEvidence(ctx, "e-expired", "decay-r-test", "test", "Old test", "pass", "L1", "test-runner", "2020-01-01")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	// Calculate R
	result, err := tools.CalculateR("decay-r-test")
	if err != nil {
		t.Fatalf("CalculateR failed: %v", err)
	}

	// Verify decay is mentioned
	if !strings.Contains(result, "Decay") || !strings.Contains(result, "expired") {
		t.Errorf("Expected decay/expired mention in output, got: %s", result)
	}
}

func TestCheckDecay_NoExpired(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a holon with fresh evidence
	err := tools.DB.CreateHolon(ctx, "fresh-holon", "hypothesis", "system", "L2", "Fresh", "Content", "ctx", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	// Add future-dated evidence
	err = tools.DB.AddEvidence(ctx, "e-fresh", "fresh-holon", "test", "Fresh test", "pass", "L2", "test-runner", "2099-12-31")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	// Check decay (freshness report mode - all empty params)
	result, err := tools.CheckDecay("", "", "", "")
	if err != nil {
		t.Fatalf("CheckDecay failed: %v", err)
	}

	// Should report all fresh
	if !strings.Contains(result, "All holons FRESH") && !strings.Contains(result, "No expired evidence") {
		t.Errorf("Expected fresh holons message, got: %s", result)
	}
}

func TestCheckDecay_WithExpired(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a holon with expired evidence
	err := tools.DB.CreateHolon(ctx, "stale-holon", "hypothesis", "system", "L2", "Stale Holon", "Content", "ctx", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	// Add expired evidence
	err = tools.DB.AddEvidence(ctx, "e-stale", "stale-holon", "test", "Old test", "pass", "L2", "test-runner", "2020-01-01")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	// Check decay (freshness report mode - all empty params)
	result, err := tools.CheckDecay("", "", "", "")
	if err != nil {
		t.Fatalf("CheckDecay failed: %v", err)
	}

	// Should report the expired evidence
	if !strings.Contains(result, "stale-holon") && !strings.Contains(result, "Stale Holon") {
		t.Errorf("Expected stale-holon in output, got: %s", result)
	}
	if !strings.Contains(result, "STALE") && !strings.Contains(result, "EXPIRED") {
		t.Errorf("Expected STALE or EXPIRED in output, got: %s", result)
	}
}

func TestCheckDecay_Deprecate(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Get the FPF directory from tools (this is where InitProject creates structure)
	fpfDir := tools.GetFPFDir()
	l2Dir := filepath.Join(fpfDir, "knowledge", "L2")
	l1Dir := filepath.Join(fpfDir, "knowledge", "L1")

	// Create L2 holon with file
	holonID := "deprecate-test"
	err := tools.DB.CreateHolon(ctx, holonID, "hypothesis", "system", "L2", "Deprecate Test", "Content", "ctx", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}
	if err := os.WriteFile(filepath.Join(l2Dir, holonID+".md"), []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Deprecate (L2 -> L1)
	result, err := tools.CheckDecay(holonID, "", "", "")
	if err != nil {
		t.Fatalf("CheckDecay deprecate failed: %v", err)
	}

	if !strings.Contains(result, "Deprecated") || !strings.Contains(result, "L2 → L1") {
		t.Errorf("Expected deprecation message, got: %s", result)
	}

	// Verify holon moved to L1 in DB
	holon, err := tools.DB.GetHolon(ctx, holonID)
	if err != nil {
		t.Fatalf("Failed to get holon: %v", err)
	}
	if holon.Layer != "L1" {
		t.Errorf("Expected layer L1, got: %s", holon.Layer)
	}

	// Verify file moved
	if _, err := os.Stat(filepath.Join(l1Dir, holonID+".md")); os.IsNotExist(err) {
		t.Error("Expected file to exist in L1 directory")
	}
}

func TestCheckDecay_Waive(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create holon with expired evidence
	holonID := "waive-test-holon"
	evidenceID := "waive-test-evidence"
	err := tools.DB.CreateHolon(ctx, holonID, "hypothesis", "system", "L2", "Waive Test", "Content", "ctx", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	err = tools.DB.AddEvidence(ctx, evidenceID, holonID, "test", "Old test", "pass", "L2", "test-runner", "2020-01-01")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	// Verify initially shows as stale
	result, err := tools.CheckDecay("", "", "", "")
	if err != nil {
		t.Fatalf("CheckDecay failed: %v", err)
	}
	if !strings.Contains(result, "STALE") {
		t.Errorf("Expected STALE before waive, got: %s", result)
	}

	// Waive the evidence
	futureDate := "2099-12-31"
	rationale := "Test waiver"
	result, err = tools.CheckDecay("", evidenceID, futureDate, rationale)
	if err != nil {
		t.Fatalf("CheckDecay waive failed: %v", err)
	}

	if !strings.Contains(result, "Waiver recorded") {
		t.Errorf("Expected waiver confirmation, got: %s", result)
	}
	if !strings.Contains(result, evidenceID) {
		t.Errorf("Expected evidence ID in output, got: %s", result)
	}

	// Check that it no longer shows as stale
	result, err = tools.CheckDecay("", "", "", "")
	if err != nil {
		t.Fatalf("CheckDecay report failed: %v", err)
	}

	// Should show waived instead of stale
	if strings.Contains(result, holonID) && strings.Contains(result, "STALE") && !strings.Contains(result, "WAIVED") {
		t.Errorf("Expected waived evidence to not show as STALE, got: %s", result)
	}
}

func TestCheckDecay_WaiveMissingParams(t *testing.T) {
	tools, _, _ := setupTools(t)

	// Waive without until date
	_, err := tools.CheckDecay("", "some-evidence", "", "some rationale")
	if err == nil {
		t.Error("Expected error when waive_until is missing")
	}

	// Waive without rationale
	_, err = tools.CheckDecay("", "some-evidence", "2099-12-31", "")
	if err == nil {
		t.Error("Expected error when rationale is missing")
	}
}

func TestCheckDecay_DeprecateL0Fails(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create L0 holon
	holonID := "l0-deprecate-test"
	err := tools.DB.CreateHolon(ctx, holonID, "hypothesis", "system", "L0", "L0 Test", "Content", "ctx", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	// Try to deprecate L0 - should fail
	_, err = tools.CheckDecay(holonID, "", "", "")
	if err == nil {
		t.Error("Expected error when deprecating L0 holon")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot deprecate") {
		t.Errorf("Expected 'cannot deprecate' error, got: %v", err)
	}
}

func TestVisualizeAudit(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a holon
	err := tools.DB.CreateHolon(ctx, "audit-viz-test", "hypothesis", "system", "L2", "Audit Viz Test", "Content", "ctx", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	// Add evidence
	err = tools.DB.AddEvidence(ctx, "e-viz", "audit-viz-test", "test", "Test", "pass", "L2", "test-runner", "2099-12-31")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	// Visualize audit
	result, err := tools.VisualizeAudit("audit-viz-test")
	if err != nil {
		t.Fatalf("VisualizeAudit failed: %v", err)
	}

	// Should contain the holon ID and R score
	if !strings.Contains(result, "audit-viz-test") {
		t.Errorf("Expected 'audit-viz-test' in output, got: %s", result)
	}
	if !strings.Contains(result, "R:") {
		t.Errorf("Expected 'R:' score in output, got: %s", result)
	}
}

func TestPropose_WithDecisionContext(t *testing.T) {
	tools, fsm, _ := setupTools(t)
	ctx := context.Background()
	fsm.State.Phase = PhaseAbduction

	// First create a decision context holon
	err := tools.DB.CreateHolon(ctx, "caching-decision", "decision", "episteme", "L0", "Caching Decision", "Content", "default", "backend", "")
	if err != nil {
		t.Fatalf("Failed to create decision context: %v", err)
	}

	// Propose hypothesis with decision_context
	_, err = tools.ProposeHypothesis(
		"Use Redis",
		"Use Redis for caching",
		"backend",
		"system",
		`{"approach": "distributed cache"}`,
		"caching-decision", // decision_context
		nil,                // no depends_on
		3,
	)
	if err != nil {
		t.Fatalf("ProposeHypothesis failed: %v", err)
	}

	// Verify MemberOf relation was created
	rawDB := tools.DB.GetRawDB()
	var count int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM relations
		WHERE source_id = 'use-redis'
		AND target_id = 'caching-decision'
		AND relation_type = 'memberOf'
	`).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query relations: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 MemberOf relation, got %d", count)
	}
}

func TestPropose_WithDependsOn(t *testing.T) {
	tools, fsm, _ := setupTools(t)
	ctx := context.Background()
	fsm.State.Phase = PhaseAbduction

	// Create dependency holons first
	err := tools.DB.CreateHolon(ctx, "auth-module", "hypothesis", "system", "L2", "Auth Module", "Content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create auth-module: %v", err)
	}
	err = tools.DB.CreateHolon(ctx, "rate-limiter", "hypothesis", "system", "L2", "Rate Limiter", "Content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create rate-limiter: %v", err)
	}

	// Propose hypothesis with depends_on
	_, err = tools.ProposeHypothesis(
		"API Gateway",
		"Gateway with auth and rate limiting",
		"external traffic",
		"system",
		`{"anomaly": "need unified entry point"}`,
		"",                                      // no decision_context
		[]string{"auth-module", "rate-limiter"}, // depends_on
		3,                                       // CL3
	)
	if err != nil {
		t.Fatalf("ProposeHypothesis failed: %v", err)
	}

	// Verify componentOf relations were created
	rawDB := tools.DB.GetRawDB()
	var count int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM relations
		WHERE target_id = 'api-gateway'
		AND relation_type = 'componentOf'
	`).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query relations: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 componentOf relations, got %d", count)
	}
}

func TestPropose_CycleDetection(t *testing.T) {
	tools, fsm, _ := setupTools(t)
	ctx := context.Background()
	fsm.State.Phase = PhaseAbduction

	// Create holon A
	err := tools.DB.CreateHolon(ctx, "holon-a", "hypothesis", "system", "L1", "Holon A", "Content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon-a: %v", err)
	}

	// Create holon B that depends on A
	_, err = tools.ProposeHypothesis("Holon B", "B depends on A", "global", "system", "{}", "", []string{"holon-a"}, 3)
	if err != nil {
		t.Fatalf("ProposeHypothesis for B failed: %v", err)
	}

	// Now try to create holon C that would create a cycle: A → B → C → A
	// First add B→C relation manually
	err = tools.DB.CreateRelation(ctx, "holon-b", "componentOf", "holon-c-temp", 3)
	if err != nil {
		// This is okay, C doesn't exist yet
	}

	// Try to make A depend on B (would create cycle since B already depends on A)
	// This should be skipped with a warning, not error
	_, err = tools.ProposeHypothesis("Holon C Cyclic", "C tries to depend on B", "global", "system", "{}", "", []string{"holon-b"}, 3)
	// Should NOT error - cycles are skipped with warning
	if err != nil {
		t.Fatalf("ProposeHypothesis should not error on cycle, got: %v", err)
	}

	// The relation should still be created since holon-c-cyclic → holon-b is not itself a cycle
	// (holon-b → holon-a exists, but holon-a doesn't depend on holon-c-cyclic)
	rawDB := tools.DB.GetRawDB()
	var count int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM relations
		WHERE target_id = 'holon-c-cyclic'
		AND source_id = 'holon-b'
		AND relation_type = 'componentOf'
	`).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query relations: %v", err)
	}
	// This should exist since it's not actually a cycle
	if count != 1 {
		t.Errorf("Expected 1 componentOf relation for non-cyclic dependency, got %d", count)
	}
}

func TestPropose_InvalidDependency(t *testing.T) {
	tools, fsm, _ := setupTools(t)
	fsm.State.Phase = PhaseAbduction

	// Propose hypothesis with non-existent dependency
	_, err := tools.ProposeHypothesis(
		"Orphan Hypo",
		"Depends on non-existent holon",
		"global",
		"system",
		"{}",
		"",
		[]string{"does-not-exist", "also-missing"}, // These don't exist
		3,
	)
	// Should NOT error - invalid deps are skipped with warning
	if err != nil {
		t.Fatalf("ProposeHypothesis should not error on invalid deps, got: %v", err)
	}

	// Verify no relations were created
	rawDB := tools.DB.GetRawDB()
	var count int
	ctx := context.Background()
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM relations
		WHERE target_id = 'orphan-hypo'
	`).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query relations: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 relations for invalid deps, got %d", count)
	}
}

func TestPropose_KindDeterminesRelation(t *testing.T) {
	tools, fsm, _ := setupTools(t)
	ctx := context.Background()
	fsm.State.Phase = PhaseAbduction

	// Create a dependency holon
	err := tools.DB.CreateHolon(ctx, "base-claim", "hypothesis", "episteme", "L2", "Base Claim", "Content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create base-claim: %v", err)
	}

	// Propose system hypothesis - should create componentOf
	_, err = tools.ProposeHypothesis("System Hypo", "A system thing", "global", "system", "{}", "", []string{"base-claim"}, 3)
	if err != nil {
		t.Fatalf("ProposeHypothesis for system failed: %v", err)
	}

	// Propose episteme hypothesis - should create constituentOf
	_, err = tools.ProposeHypothesis("Episteme Hypo", "An epistemic claim", "global", "episteme", "{}", "", []string{"base-claim"}, 3)
	if err != nil {
		t.Fatalf("ProposeHypothesis for episteme failed: %v", err)
	}

	rawDB := tools.DB.GetRawDB()

	// Check system → componentOf
	var componentCount int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM relations
		WHERE target_id = 'system-hypo'
		AND relation_type = 'componentOf'
	`).Scan(&componentCount)
	if err != nil {
		t.Fatalf("Failed to query componentOf: %v", err)
	}
	if componentCount != 1 {
		t.Errorf("Expected 1 componentOf for system kind, got %d", componentCount)
	}

	// Check episteme → constituentOf
	var constituentCount int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM relations
		WHERE target_id = 'episteme-hypo'
		AND relation_type = 'constituentOf'
	`).Scan(&constituentCount)
	if err != nil {
		t.Fatalf("Failed to query constituentOf: %v", err)
	}
	if constituentCount != 1 {
		t.Errorf("Expected 1 constituentOf for episteme kind, got %d", constituentCount)
	}
}

func TestWLNK_MemberOf_NoPropagation(t *testing.T) {
	tools, fsm, _ := setupTools(t)
	ctx := context.Background()
	fsm.State.Phase = PhaseAbduction

	// Create decision context with low R (failing evidence)
	err := tools.DB.CreateHolon(ctx, "bad-decision", "decision", "episteme", "L1", "Bad Decision", "Content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create bad-decision: %v", err)
	}
	err = tools.DB.AddEvidence(ctx, "e-bad", "bad-decision", "test", "Failed", "fail", "L1", "test", "2099-12-31")
	if err != nil {
		t.Fatalf("Failed to add failing evidence: %v", err)
	}

	// Create good hypothesis that is member of bad decision
	_, err = tools.ProposeHypothesis(
		"Good Member",
		"A good hypothesis",
		"global",
		"system",
		"{}",
		"bad-decision", // MemberOf the bad decision
		nil,
		3,
	)
	if err != nil {
		t.Fatalf("ProposeHypothesis failed: %v", err)
	}

	// Add passing evidence to good-member
	err = tools.DB.AddEvidence(ctx, "e-good", "good-member", "test", "Passed", "pass", "L1", "test", "2099-12-31")
	if err != nil {
		t.Fatalf("Failed to add passing evidence: %v", err)
	}

	// Calculate R for good-member
	result, err := tools.CalculateR("good-member")
	if err != nil {
		t.Fatalf("CalculateR failed: %v", err)
	}

	// MemberOf should NOT propagate R - good-member should have R=1.00
	// despite bad-decision having R=0.00
	if !strings.Contains(result, "1.00") {
		t.Errorf("Expected R=1.00 (MemberOf should not propagate), got: %s", result)
	}
}

func TestFormatVocabulary(t *testing.T) {
	input := "Channel: A Telegram channel or chat being monitored (has telegram_id, name, kind, is_active status). Message: A post from a monitored channel (has id, content, author_id, telegram_url, processing state). Result[T,E]: Either Ok(value) or Err(error) - functional error handling pattern."

	result := formatVocabulary(input)

	// Should have separate lines for each term
	if !strings.Contains(result, "- **Channel**:") {
		t.Errorf("Expected '- **Channel**:', got: %s", result)
	}
	if !strings.Contains(result, "- **Message**:") {
		t.Errorf("Expected '- **Message**:', got: %s", result)
	}
	if !strings.Contains(result, "- **Result[T,E]**:") {
		t.Errorf("Expected '- **Result[T,E]**:', got: %s", result)
	}

	// Should have newlines between entries
	lines := strings.Split(result, "\n")
	if len(lines) < 3 {
		t.Errorf("Expected at least 3 lines, got %d: %s", len(lines), result)
	}
}

func TestFormatInvariants(t *testing.T) {
	input := "1. Python 3.12+ with strict mypy type checking. 2. DuckDB as the only database (file-based, path from config.yaml). 3. Telethon for Telegram API interaction (requires session file)."

	result := formatInvariants(input)

	// Should have separate lines for each numbered item
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d: %s", len(lines), result)
	}

	if !strings.HasPrefix(lines[0], "1. Python") {
		t.Errorf("Expected line 1 to start with '1. Python', got: %s", lines[0])
	}
	if !strings.HasPrefix(lines[1], "2. DuckDB") {
		t.Errorf("Expected line 2 to start with '2. DuckDB', got: %s", lines[1])
	}
	if !strings.HasPrefix(lines[2], "3. Telethon") {
		t.Errorf("Expected line 3 to start with '3. Telethon', got: %s", lines[2])
	}
}

func TestManageEvidence_ValidUntilDefault(t *testing.T) {
	tools, fsm, tempDir := setupTools(t)
	ctx := context.Background()

	// Create a hypothesis in L1
	hypoID := "valid-until-test-hypo"
	hypoPath := filepath.Join(tempDir, ".quint", "knowledge", "L1", hypoID+".md")
	if err := os.WriteFile(hypoPath, []byte("Test hypothesis content"), 0644); err != nil {
		t.Fatalf("Failed to create hypothesis file: %v", err)
	}

	// Create holon in DB
	err := tools.DB.CreateHolon(ctx, hypoID, "hypothesis", "system", "L1", "Test Hypothesis", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	// Set FSM phase to Induction
	fsm.State.Phase = PhaseInduction

	// Call ManageEvidence with EMPTY validUntil (like quint_test does)
	_, err = tools.ManageEvidence(PhaseInduction, "add", hypoID, "internal", "Test passed", "pass", "L2", "test-runner", "")
	if err != nil {
		t.Fatalf("ManageEvidence failed: %v", err)
	}

	// Query evidence from DB
	evidence, err := tools.DB.GetEvidence(ctx, hypoID)
	if err != nil {
		t.Fatalf("GetEvidence failed: %v", err)
	}

	if len(evidence) == 0 {
		t.Fatal("No evidence found in DB")
	}

	e := evidence[0]
	t.Logf("Evidence ID: %s", e.ID)
	t.Logf("ValidUntil.Valid: %v", e.ValidUntil.Valid)
	if e.ValidUntil.Valid {
		t.Logf("ValidUntil.Time: %v", e.ValidUntil.Time)
	}

	if !e.ValidUntil.Valid {
		t.Error("BUG CONFIRMED: valid_until is NULL in DB despite ManageEvidence setting default!")
	} else {
		// Should be ~90 days from now
		expectedMin := time.Now().AddDate(0, 0, 85)
		expectedMax := time.Now().AddDate(0, 0, 95)
		if e.ValidUntil.Time.Before(expectedMin) || e.ValidUntil.Time.After(expectedMax) {
			t.Errorf("valid_until %v is not ~90 days from now", e.ValidUntil.Time)
		} else {
			t.Logf("OK: valid_until correctly set to %v (~90 days from now)", e.ValidUntil.Time)
		}
	}
}

func TestInternalize_FirstCall(t *testing.T) {
	tempDir := t.TempDir()
	// Do NOT create .quint - let Internalize do it
	dbPath := filepath.Join(tempDir, ".quint", "quint.db")
	fsm := &FSM{State: State{Phase: PhaseIdle}}
	tools := &Tools{FSM: fsm, RootDir: tempDir, DB: nil}

	result, err := tools.Internalize()
	if err != nil {
		t.Fatalf("Internalize() error = %v", err)
	}

	if !strings.Contains(result, "Status: INITIALIZED") {
		t.Errorf("First call should return INITIALIZED, got: %s", result)
	}
	if !strings.Contains(result, "Phase: ABDUCTION") {
		t.Errorf("Phase should be ABDUCTION after init, got: %s", result)
	}

	// Verify .quint was created
	if _, err := os.Stat(filepath.Join(tempDir, ".quint")); os.IsNotExist(err) {
		t.Error(".quint directory should be created")
	}

	// Verify DB was initialized
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("quint.db should be created")
	}
}

func TestInternalize_SubsequentCall(t *testing.T) {
	tools, _, _ := setupTools(t)

	// First call - sets up everything
	_, err := tools.Internalize()
	if err != nil {
		t.Fatalf("First Internalize() error = %v", err)
	}

	// Second call - should return CURRENT
	result, err := tools.Internalize()
	if err != nil {
		t.Fatalf("Second Internalize() error = %v", err)
	}

	if !strings.Contains(result, "Status: CURRENT") {
		t.Errorf("Subsequent call should return CURRENT, got: %s", result)
	}
}

func TestInternalize_LayerCounts(t *testing.T) {
	tools, _, tempDir := setupTools(t)

	// Create some L0 holons
	l0Dir := filepath.Join(tempDir, ".quint", "knowledge", "L0")
	os.WriteFile(filepath.Join(l0Dir, "hypo1.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(l0Dir, "hypo2.md"), []byte("test"), 0644)

	result, err := tools.Internalize()
	if err != nil {
		t.Fatalf("Internalize() error = %v", err)
	}

	if !strings.Contains(result, "L0 (Conjecture): 2") {
		t.Errorf("Should show 2 L0 holons, got: %s", result)
	}
}

func TestSearch_NoResults(t *testing.T) {
	tools, _, _ := setupTools(t)

	result, err := tools.Search("xyznonexistentquery", "", "", "", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if !strings.Contains(result, "No results found") {
		t.Errorf("Should return 'No results found', got: %s", result)
	}
}

func TestSearch_WithResults(t *testing.T) {
	tools, _, _ := setupTools(t)

	// Create a holon that will be indexed
	ctx := context.Background()
	tools.DB.CreateHolon(ctx, "search-test-holon", "hypothesis", "system", "L0",
		"Authentication Decision", "How to handle user authentication", "default", "", "")

	// Search for it
	result, err := tools.Search("authentication", "", "", "", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if strings.Contains(result, "No results found") {
		t.Errorf("Should find results, got: %s", result)
	}
	if !strings.Contains(result, "Authentication Decision") {
		t.Errorf("Should find 'Authentication Decision', got: %s", result)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	tools, _, _ := setupTools(t)

	_, err := tools.Search("", "", "", "", 10)
	if err == nil {
		t.Error("Empty query should return error")
	}
}

func TestSearch_NoDB(t *testing.T) {
	tempDir := t.TempDir()
	fsm := &FSM{State: State{Phase: PhaseIdle}}
	tools := &Tools{FSM: fsm, RootDir: tempDir, DB: nil}

	_, err := tools.Search("test", "", "", "", 10)
	if err == nil {
		t.Error("Search without DB should return error")
	}
	if !strings.Contains(err.Error(), "database not initialized") {
		t.Errorf("Should mention database not initialized, got: %v", err)
	}
}

func TestSearch_LayerFilter(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create holons in different layers
	tools.DB.CreateHolon(ctx, "l0-holon", "hypothesis", "system", "L0",
		"L0 Test Holon", "Content for L0", "default", "", "")
	tools.DB.CreateHolon(ctx, "l2-holon", "hypothesis", "system", "L2",
		"L2 Test Holon", "Content for L2", "default", "", "")

	// Search with L2 filter
	result, err := tools.Search("Test Holon", "", "L2", "", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if strings.Contains(result, "L0 Test Holon") {
		t.Errorf("L2 filter should exclude L0 holons, got: %s", result)
	}
}

func TestSearch_SpecialCharacters(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a holon with hyphenated title
	tools.DB.CreateHolon(ctx, "special-char-holon", "hypothesis", "system", "L0",
		"Redis-backed Cache Strategy", "Use redis-cluster for caching", "default", "", "")

	// Search with hyphenated query (previously would cause FTS5 parse error)
	result, err := tools.Search("redis-backed", "", "", "", 10)
	if err != nil {
		t.Fatalf("Search with hyphens should not error: %v", err)
	}

	if strings.Contains(result, "No results found") {
		t.Errorf("Should find hyphenated content, got: %s", result)
	}
}

func TestResolve_Implemented(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a DRR holon
	err := tools.DB.CreateHolon(ctx, "DRR-test-decision", "DRR", "system", "DRR",
		"Test Decision", "Decision content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	// Resolve as implemented
	input := ResolveInput{
		DecisionID: "DRR-test-decision",
		Resolution: "implemented",
		Reference:  "commit:abc1234",
	}
	result, err := tools.Resolve(input)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if !strings.Contains(result, "implemented") {
		t.Errorf("Should confirm implemented status, got: %s", result)
	}
	if !strings.Contains(result, "commit:abc1234") {
		t.Errorf("Should include reference, got: %s", result)
	}

	// Verify evidence was created
	evidence, err := tools.DB.GetEvidence(ctx, "DRR-test-decision")
	if err != nil {
		t.Fatalf("GetEvidence() error = %v", err)
	}
	if len(evidence) == 0 {
		t.Error("Expected evidence to be created")
	}
	if evidence[0].Type != "implementation" {
		t.Errorf("Evidence type should be 'implementation', got: %s", evidence[0].Type)
	}
}

func TestResolve_Abandoned(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a DRR holon
	err := tools.DB.CreateHolon(ctx, "DRR-abandoned-test", "DRR", "system", "DRR",
		"Abandoned Decision", "Decision content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	// Resolve as abandoned
	input := ResolveInput{
		DecisionID: "DRR-abandoned-test",
		Resolution: "abandoned",
		Notes:      "Requirements changed",
	}
	result, err := tools.Resolve(input)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if !strings.Contains(result, "abandoned") {
		t.Errorf("Should confirm abandoned status, got: %s", result)
	}

	// Verify evidence was created
	evidence, err := tools.DB.GetEvidence(ctx, "DRR-abandoned-test")
	if err != nil {
		t.Fatalf("GetEvidence() error = %v", err)
	}
	if len(evidence) == 0 {
		t.Error("Expected evidence to be created")
	}
	if evidence[0].Type != "abandonment" {
		t.Errorf("Evidence type should be 'abandonment', got: %s", evidence[0].Type)
	}
}

func TestResolve_Superseded(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create old DRR
	err := tools.DB.CreateHolon(ctx, "DRR-old-decision", "DRR", "system", "DRR",
		"Old Decision", "Old decision content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create old DRR: %v", err)
	}

	// Create new DRR that supersedes old
	err = tools.DB.CreateHolon(ctx, "DRR-new-decision", "DRR", "system", "DRR",
		"New Decision", "New decision content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create new DRR: %v", err)
	}

	// Resolve old as superseded
	input := ResolveInput{
		DecisionID:   "DRR-old-decision",
		Resolution:   "superseded",
		SupersededBy: "DRR-new-decision",
		Notes:        "Better approach found",
	}
	result, err := tools.Resolve(input)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if !strings.Contains(result, "superseded") {
		t.Errorf("Should confirm superseded status, got: %s", result)
	}
	if !strings.Contains(result, "DRR-new-decision") {
		t.Errorf("Should mention replacement DRR, got: %s", result)
	}
}

func TestResolve_InvalidDecision(t *testing.T) {
	tools, _, _ := setupTools(t)

	// Try to resolve non-existent decision
	input := ResolveInput{
		DecisionID: "DRR-does-not-exist",
		Resolution: "implemented",
		Reference:  "commit:abc1234",
	}
	_, err := tools.Resolve(input)
	if err == nil {
		t.Error("Should error on non-existent decision")
	}
}

func TestResolve_InvalidResolution(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a DRR holon
	err := tools.DB.CreateHolon(ctx, "DRR-invalid-res", "DRR", "system", "DRR",
		"Test Decision", "Decision content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	// Try invalid resolution type
	input := ResolveInput{
		DecisionID: "DRR-invalid-res",
		Resolution: "invalid_type",
	}
	_, err = tools.Resolve(input)
	if err == nil {
		t.Error("Should error on invalid resolution type")
	}
}

func TestResolve_MissingRequiredParams(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a DRR holon
	err := tools.DB.CreateHolon(ctx, "DRR-missing-params", "DRR", "system", "DRR",
		"Test Decision", "Decision content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	// implemented without reference
	input := ResolveInput{
		DecisionID: "DRR-missing-params",
		Resolution: "implemented",
	}
	_, err = tools.Resolve(input)
	if err == nil {
		t.Error("implemented should require reference")
	}

	// abandoned without notes
	input = ResolveInput{
		DecisionID: "DRR-missing-params",
		Resolution: "abandoned",
	}
	_, err = tools.Resolve(input)
	if err == nil {
		t.Error("abandoned should require notes")
	}

	// superseded without superseded_by
	input = ResolveInput{
		DecisionID: "DRR-missing-params",
		Resolution: "superseded",
	}
	_, err = tools.Resolve(input)
	if err == nil {
		t.Error("superseded should require superseded_by")
	}
}

func TestSearch_StatusFilterOpen(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create open DRR (no resolution evidence)
	err := tools.DB.CreateHolon(ctx, "DRR-open-test", "DRR", "system", "DRR",
		"Open Decision", "Pending resolution", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create open DRR: %v", err)
	}

	// Create resolved DRR (with implementation evidence)
	err = tools.DB.CreateHolon(ctx, "DRR-resolved-test", "DRR", "system", "DRR",
		"Resolved Decision", "Already implemented", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create resolved DRR: %v", err)
	}
	err = tools.DB.AddEvidence(ctx, "impl-evidence", "DRR-resolved-test", "implementation",
		"Implemented in commit abc123", "pass", "L2", "developer", "2099-12-31")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	// Search for open decisions
	result, err := tools.Search("Decision", "", "", "open", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if !strings.Contains(result, "Open Decision") {
		t.Errorf("Should find open decision, got: %s", result)
	}
	if strings.Contains(result, "Resolved Decision") {
		t.Errorf("Should NOT find resolved decision with open filter, got: %s", result)
	}
}

func TestSearch_StatusFilterImplemented(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create implemented DRR
	err := tools.DB.CreateHolon(ctx, "DRR-impl-search", "DRR", "system", "DRR",
		"Implemented Decision", "Already done", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}
	err = tools.DB.AddEvidence(ctx, "impl-evidence-2", "DRR-impl-search", "implementation",
		"Done", "pass", "L2", "developer", "2099-12-31")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	// Search for implemented decisions
	result, err := tools.Search("Decision", "", "", "implemented", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if !strings.Contains(result, "Implemented Decision") {
		t.Errorf("Should find implemented decision, got: %s", result)
	}
}

func TestInternalize_ShowsOpenDecisions(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create an open DRR
	err := tools.DB.CreateHolon(ctx, "DRR-open-internalize", "DRR", "system", "DRR",
		"Pending Decision", "Awaiting resolution", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	// Internalize and check output
	result, err := tools.Internalize()
	if err != nil {
		t.Fatalf("Internalize() error = %v", err)
	}

	if !strings.Contains(result, "Open Decisions") || !strings.Contains(result, "Pending Decision") {
		t.Errorf("Should show open decision in internalize output, got: %s", result)
	}
}

func TestGetOpenDecisions(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create open DRR
	err := tools.DB.CreateHolon(ctx, "DRR-get-open", "DRR", "system", "DRR",
		"Open for Query", "Test", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	decisions, err := tools.GetOpenDecisions(ctx)
	if err != nil {
		t.Fatalf("GetOpenDecisions() error = %v", err)
	}

	if len(decisions) == 0 {
		t.Error("Expected at least one open decision")
	}

	found := false
	for _, d := range decisions {
		if d.ID == "DRR-get-open" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find DRR-get-open in open decisions")
	}
}

func TestGetResolvedDecisions(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create and resolve a DRR
	err := tools.DB.CreateHolon(ctx, "DRR-get-resolved", "DRR", "system", "DRR",
		"Resolved for Query", "Test", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	// Add resolution evidence
	err = tools.DB.AddEvidence(ctx, "impl-get-resolved", "DRR-get-resolved", "implementation",
		"Done", "pass", "L2", "dev", "2099-12-31")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	decisions, err := tools.GetResolvedDecisions(ctx, "implemented", 10)
	if err != nil {
		t.Fatalf("GetResolvedDecisions() error = %v", err)
	}

	found := false
	for _, d := range decisions {
		if d.ID == "DRR-get-resolved" {
			found = true
			if d.Resolution != "implemented" {
				t.Errorf("Expected resolution 'implemented', got: %s", d.Resolution)
			}
			break
		}
	}
	if !found {
		t.Error("Expected to find DRR-get-resolved in resolved decisions")
	}
}
