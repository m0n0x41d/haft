package fpf

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"quint-mcp/db"
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
	database, err := db.New(dbPath)
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

	path, err := tools.ProposeHypothesis(title, content, scope, kind, rationale)
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
	expectedContent := fmt.Sprintf("---\nscope: %s\nkind: %s\n---\n\n# Hypothesis: %s\n\n%s\n\n## Rationale\n%s", scope, kind, title, content, rationale)
	if string(readContent) != expectedContent {
		t.Errorf("File content mismatch. Got %q, expected %q", string(readContent), expectedContent)
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
		name        string
		currentPhase Phase
		targetID    string
		evidenceType string
		content     string
		verdict     string
		assuranceLevel string // New field
		expectedMove bool
		expectedDestLevel string // e.g., "L1", "L2", "invalid"
		expectErr   bool
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

	drrPath, err := tools.FinalizeDecision(title, winnerID, "Context", content, "Rationale", "Consequences", "Characteristics")
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
	expectedMsg := fmt.Sprintf("Hypothesis %s promoted to L1", hypoID)
	if msg != expectedMsg {
		t.Errorf("Expected message %q, got %q", expectedMsg, msg)
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