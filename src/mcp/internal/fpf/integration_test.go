package fpf_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"quint-mcp/db"
	"quint-mcp/internal/fpf"
)

// Helper to get FPF knowledge path for a level
func getKnowledgePath(t *testing.T, baseDir, level string) string {
	return filepath.Join(baseDir, ".quint", "knowledge", level)
}

// Helper to check if a hypothesis exists in a specific level
func checkHypothesisExists(t *testing.T, baseDir, level, hypoID string) bool {
	path := filepath.Join(getKnowledgePath(t, baseDir, level), hypoID+".md")
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// Helper to check if a file exists
func checkFileExists(t *testing.T, path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func TestFullFPFWorkflowIntegration(t *testing.T) {
	tempDir := t.TempDir()
	quintDir := filepath.Join(tempDir, ".quint")
	stateFile := filepath.Join(quintDir, "state.json")

	// Create .quint directory and DB
	if err := os.MkdirAll(quintDir, 0755); err != nil {
		t.Fatalf("Failed to create .quint directory: %v", err)
	}
	dbPath := filepath.Join(quintDir, "quint.db")
	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize DB: %v", err)
	}

	// --- 0. Initialize FPF Project ---
	t.Run("0_InitProject", func(t *testing.T) {
		fsm, err := fpf.LoadState(stateFile, database.GetRawDB())
		if err != nil {
			t.Fatalf("Failed to load initial state: %v", err)
		}
		tools := fpf.NewTools(fsm, tempDir, database)

		fsm.State.Phase = fpf.PhaseIdle // Ensure idle for init
		err = tools.InitProject()
		if err != nil {
			t.Fatalf("InitProject failed: %v", err)
		}
		fsm.State.Phase = fpf.PhaseAbduction // Simulate transition in fpf.go
		if err := fsm.SaveState(stateFile); err != nil {
			t.Fatalf("SaveState failed: %v", err)
		}

		if !checkFileExists(t, stateFile) {
			t.Errorf("state.json not created")
		}
	})

	// Reload FSM state for subsequent steps
	fsm, err := fpf.LoadState(stateFile, database.GetRawDB())
	if err != nil {
		t.Fatalf("Failed to load state after init: %v", err)
	}
	tools := fpf.NewTools(fsm, tempDir, database)

	// Helper for RoleAssignment
	ra := func(r fpf.Role) fpf.RoleAssignment {
		return fpf.RoleAssignment{Role: r, SessionID: "test", Context: "test"}
	}

	// Helper for EvidenceStub
	ev := func(uri string) *fpf.EvidenceStub {
		if uri == "" { return nil }
		return &fpf.EvidenceStub{URI: uri, Type: "test"}
	}

	// --- 1. Propose Hypothesis (Abduction) ---
	hypo1Title := "Initial Hypothesis"
	hypo1Content := "Content for initial hypothesis."
	hypo1ID := tools.Slugify(hypo1Title)

	t.Run("1_ProposeHypothesis", func(t *testing.T) {
		// Ensure current phase is ABDUCTION
		if fsm.State.Phase != fpf.PhaseAbduction {
			t.Fatalf("Expected phase ABDUCTION, got %s", fsm.State.Phase)
		}
		path, err := tools.ProposeHypothesis(hypo1Title, hypo1Content, "global", "system", "Integration Test Rationale")
		if err != nil {
			t.Fatalf("ProposeHypothesis failed: %v", err)
		}
		if !checkHypothesisExists(t, tempDir, "L0", hypo1ID) {
			t.Errorf("Hypothesis %s not found in L0", hypo1ID)
		}
		if !strings.Contains(path, hypo1ID) {
			t.Errorf("Returned path %q does not contain hypothesis ID", path)
		}
	})

	// --- 2. Manage Evidence (Deduction - PASS) ---
	t.Run("2_ManageEvidence_DeductionPass", func(t *testing.T) {
		// Transition to DEDUCTION
		// Needs L0 evidence (which ProposeHypothesis created)
		l0Dir := filepath.Join(tempDir, ".quint", "knowledge", "L0")
		ok, msg := fsm.CanTransition(fpf.PhaseDeduction, ra(fpf.RoleDeductor), ev(l0Dir))
		_ = msg
		if !ok {
			t.Fatalf("Failed to transition to DEDUCTION: %s", msg)
		}
		fsm.State.Phase = fpf.PhaseDeduction
		if err := fsm.SaveState(stateFile); err != nil {
			t.Fatalf("SaveState failed: %v", err)
		}

		evidenceContent := "Deductive logic check passes."
		verdict := "PASS"
		
		evidencePath, err := tools.ManageEvidence(fsm.State.Phase, "add", hypo1ID, "logic", evidenceContent, verdict, "L1", "logic-carrier", "2025-12-31")
		if err != nil {
			t.Fatalf("ManageEvidence (Deduction PASS) failed: %v", err)
		}

		if !checkHypothesisExists(t, tempDir, "L1", hypo1ID) {
			t.Errorf("Hypothesis %s not moved to L1 after Deduction PASS", hypo1ID)
		}
		if checkHypothesisExists(t, tempDir, "L0", hypo1ID) {
			t.Errorf("Hypothesis %s still found in L0 after Deduction PASS", hypo1ID)
		}
		if !checkFileExists(t, evidencePath) {
			t.Errorf("Evidence file not created")
		}
	})

	// --- 3. Manage Evidence (Induction - PASS) ---
	t.Run("3_ManageEvidence_InductionPass", func(t *testing.T) {
		// Needs L1 evidence (which Deduction PASS created)
		l1File := filepath.Join(tempDir, ".quint", "knowledge", "L1", hypo1ID+".md")
		ok, msg := fsm.CanTransition(fpf.PhaseInduction, ra(fpf.RoleInductor), ev(l1File))
		_ = msg
		if !ok {
			t.Fatalf("Failed to transition to INDUCTION: %s", msg)
		}
		fsm.State.Phase = fpf.PhaseInduction
		if err := fsm.SaveState(stateFile); err != nil {
			t.Fatalf("SaveState failed: %v", err)
		}

		evidenceContent := "Empirical test results confirm hypothesis."
		verdict := "PASS"

		// hypo1ID should now be in L1
		if !checkHypothesisExists(t, tempDir, "L1", hypo1ID) {
			t.Fatalf("Hypothesis %s not found in L1 before Induction PASS test", hypo1ID)
		}
		
		evidencePath, err := tools.ManageEvidence(fsm.State.Phase, "add", hypo1ID, "empirical", evidenceContent, verdict, "L2", "empirical-carrier", "2025-12-31")
		if err != nil {
			t.Fatalf("ManageEvidence (Induction PASS) failed: %v", err)
		}

		if !checkHypothesisExists(t, tempDir, "L2", hypo1ID) {
			t.Errorf("Hypothesis %s not moved to L2 after Induction PASS", hypo1ID)
		}
		if checkHypothesisExists(t, tempDir, "L1", hypo1ID) {
			t.Errorf("Hypothesis %s still found in L1 after Induction PASS", hypo1ID)
		}
		if !checkFileExists(t, evidencePath) {
			t.Errorf("Evidence file not created")
		}
	})

	// --- 4. Loopback (Induction -> Deduction - REFINE) ---
	hypo2Title := "Refined Hypothesis"
	hypo2Content := "This is a refined version."
	hypo2ID := tools.Slugify(hypo2Title)

	t.Run("4_RefineLoopback", func(t *testing.T) {
		// First, move the L2 hypothesis back to L1 for the loopback test
		// In a real scenario, this would be a different hypothesis being refined,
		// but for integration testing, we simulate it.
		// So we create a new hypothesis in L1 to be refined.
		loopbackHypoID := "loopback-candidate"
		loopbackHypoPath := filepath.Join(getKnowledgePath(t, tempDir, "L1"), loopbackHypoID+".md")
		if err := os.WriteFile(loopbackHypoPath, []byte("Loopback candidate content"), 0644); err != nil {
			t.Fatalf("Failed to create loopback hypothesis file: %v", err)
		}

		// Transition FSM to INDUCTION before loopback
		fsm.State.Phase = fpf.PhaseInduction
		if err := fsm.SaveState(stateFile); err != nil {
			t.Fatalf("SaveState failed: %v", err)
		}

		insight := "New insight from empirical failure."
		
		childPath, err := tools.RefineLoopback(fsm.State.Phase, loopbackHypoID, insight, hypo2Title, hypo2Content, "system")
		if err != nil {
			t.Fatalf("RefineLoopback failed: %v", err)
		}

		// Manually update FSM state in test as RefineLoopback in tools.go doesn't do it
		fsm.State.Phase = fpf.PhaseDeduction
		if err := fsm.SaveState(stateFile); err != nil {
			t.Fatalf("SaveState failed: %v", err)
		}

		if !checkHypothesisExists(t, tempDir, "invalid", loopbackHypoID) {
			t.Errorf("Loopback hypothesis %s not moved to invalid", loopbackHypoID)
		}
		if checkHypothesisExists(t, tempDir, "L1", loopbackHypoID) {
			t.Errorf("Loopback hypothesis %s still found in L1", loopbackHypoID)
		}
		if !checkHypothesisExists(t, tempDir, "L0", hypo2ID) {
			t.Errorf("Child hypothesis %s not created in L0", hypo2ID)
		}
		if fsm.State.Phase != fpf.PhaseDeduction {
			t.Fatalf("Expected phase DEDUCTION, got %s", fsm.State.Phase)
		}
		if !strings.Contains(childPath, hypo2ID) {
			t.Errorf("Returned child path %q does not contain hypothesis ID", childPath)
		}
	})

	// --- 4.5. Manage Evidence (Deduction - PASS) for the new child hypothesis ---
	t.Run("4_5_ManageEvidence_DeductionPass_AfterLoopback", func(t *testing.T) {
		// FSM should already be in DEDUCTION from loopback
		if fsm.State.Phase != fpf.PhaseDeduction {
			t.Fatalf("Expected phase DEDUCTION, got %s", fsm.State.Phase)
		}

		evidenceContent := "Deductive logic check passes for refined hypothesis."
		verdict := "PASS"

		// hypo2ID is the new child hypothesis, created in L0
		evidencePath, err := tools.ManageEvidence(fsm.State.Phase, "add", hypo2ID, "logic", evidenceContent, verdict, "L1", "logic-carrier-2", "2025-12-31")
		if err != nil {
			t.Fatalf("ManageEvidence (Deduction PASS for refined) failed: %v", err)
		}

		if !checkHypothesisExists(t, tempDir, "L1", hypo2ID) {
			t.Errorf("Hypothesis %s not moved to L1 after Deduction PASS for refined", hypo2ID)
		}
		if checkHypothesisExists(t, tempDir, "L0", hypo2ID) {
			t.Errorf("Hypothesis %s still found in L0 after Deduction PASS for refined", hypo2ID)
		}
		if !checkFileExists(t, evidencePath) {
			t.Errorf("Evidence file not created")
		}

		// Transition to INDUCTION for next step
		// Needs L1 evidence (which Deduction PASS for refined created)
		l1File := filepath.Join(tempDir, ".quint", "knowledge", "L1", hypo2ID+".md")
		ok, msg := fsm.CanTransition(fpf.PhaseInduction, ra(fpf.RoleInductor), ev(l1File))
		if !ok {
			t.Fatalf("Failed to transition to INDUCTION: %s", msg)
		}
		fsm.State.Phase = fpf.PhaseInduction
		if err := fsm.SaveState(stateFile); err != nil {
			t.Fatalf("SaveState failed: %v", err)
		}
	})

	// --- 4.6. Manage Evidence (Induction - PASS) for the refined hypothesis ---
	t.Run("4_6_ManageEvidence_InductionPass_Refined", func(t *testing.T) {
		// FSM should be in INDUCTION (transitioned at end of 4.5)
		if fsm.State.Phase != fpf.PhaseInduction {
			t.Fatalf("Expected phase INDUCTION, got %s", fsm.State.Phase)
		}

		evidenceContent := "Empirical test results confirm refined hypothesis."
		verdict := "PASS"
		
		// hypo2ID is in L1
		evidencePath, err := tools.ManageEvidence(fsm.State.Phase, "add", hypo2ID, "empirical", evidenceContent, verdict, "L2", "empirical-carrier-2", "2025-12-31")
		if err != nil {
			t.Fatalf("ManageEvidence (Induction PASS refined) failed: %v", err)
		}

		if !checkHypothesisExists(t, tempDir, "L2", hypo2ID) {
			t.Errorf("Hypothesis %s not moved to L2 after Induction PASS refined", hypo2ID)
		}
		if !checkFileExists(t, evidencePath) {
			t.Errorf("Evidence file not created")
		}
	})

	// --- 5. Finalize Decision (DECISION -> IDLE) ---
	drrContent := "Final DRR content endorsing the winner."
	// hypo2ID should now be in L2 after the Induction Pass (4_6)
	finalWinnerID := hypo2ID

	t.Run("5_FinalizeDecision", func(t *testing.T) {
		// Ensure FSM is in INDUCTION for the Decider to transition
		if fsm.State.Phase != fpf.PhaseInduction {
			t.Fatalf("Expected phase INDUCTION before FinalizeDecision, got %s", fsm.State.Phase)
		}

		// Transition to DECISION
		// Needs L2 evidence (which Induction PASS created)
		l2File := filepath.Join(tempDir, ".quint", "knowledge", "L2", finalWinnerID+".md")
		ok, msg := fsm.CanTransition(fpf.PhaseDecision, ra(fpf.RoleDecider), ev(l2File))
		_ = msg
		if !ok {
			t.Fatalf("Failed to transition to DECISION: %s", msg)
		}
		fsm.State.Phase = fpf.PhaseDecision
		if err := fsm.SaveState(stateFile); err != nil {
			t.Fatalf("SaveState failed: %v", err)
		}

		path, err := tools.FinalizeDecision("Final Decision", finalWinnerID, "Context", "Decision", drrContent, "Consequences", "Characteristics")
		if err != nil {
			t.Fatalf("FinalizeDecision failed: %v", err)
		}

		// Manually update FSM state in test as FinalizeDecision in tools.go doesn't do it
		fsm.State.Phase = fpf.PhaseIdle
		if err := fsm.SaveState(stateFile); err != nil {
			t.Fatalf("SaveState failed: %v", err)
		}

		// Verify DRR file creation
		drrPattern := filepath.Join(tempDir, ".quint", "decisions", fmt.Sprintf("DRR-*-%s.md", tools.Slugify("Final Decision")))
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
			if match == path { // 'path' is the returned DRR path from FinalizeDecision
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Returned DRR path %q does not match any expected pattern %q", path, drrPattern)
		}
		
		// Verify winner moved to L2
		if !checkHypothesisExists(t, tempDir, "L2", finalWinnerID) {
			t.Errorf("Final winner %s not moved to L2", finalWinnerID)
		}
		if checkHypothesisExists(t, tempDir, "L1", finalWinnerID) {
			t.Errorf("Final winner %s still found in L1", finalWinnerID)
		}
		if fsm.State.Phase != fpf.PhaseIdle {
			t.Errorf("Expected phase IDLE after decision, got %s", fsm.State.Phase)
		}
	})}
