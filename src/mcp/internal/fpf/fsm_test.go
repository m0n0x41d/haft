package fpf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadState(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	// Test loading non-existent state (should initialize to IDLE)
	fsm, err := LoadState(stateFile, nil)
	if err != nil {
		t.Fatalf("LoadState failed for non-existent file: %v", err)
	}
	if fsm.State.Phase != PhaseIdle {
		t.Errorf("Expected initial phase to be IDLE, got %s", fsm.State.Phase)
	}

	// Test loading existing state
	fsm.State.Phase = PhaseAbduction
	if err := fsm.SaveState(stateFile); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	fsm2, err := LoadState(stateFile, nil)
	if err != nil {
		t.Fatalf("LoadState failed for existing file: %v", err)
	}
	if fsm2.State.Phase != PhaseAbduction {
		t.Errorf("Expected loaded phase to be ABDUCTION, got %s", fsm2.State.Phase)
	}
}

func TestSaveState(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	fsm := &FSM{State: State{Phase: PhaseDeduction}}
	err := fsm.SaveState(stateFile)
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Errorf("State file was not created")
	}
}

func TestCanTransition(t *testing.T) {
	// Setup temp dir for dummy evidence
	tempDir := t.TempDir()
	
	// Create dummy evidence directories/files
	// Deduction needs L0 directory
	l0Dir := filepath.Join(tempDir, "knowledge", "L0")
	os.MkdirAll(l0Dir, 0755)
	os.WriteFile(filepath.Join(l0Dir, "h1.md"), []byte("hypo"), 0644)

	// Induction needs L1 file
	l1Dir := filepath.Join(tempDir, "knowledge", "L1")
	os.MkdirAll(l1Dir, 0755)
	l1File := filepath.Join(l1Dir, "h1.md")
	os.WriteFile(l1File, []byte("hypo"), 0644)

	// Decision needs L2 file
	l2Dir := filepath.Join(tempDir, "knowledge", "L2")
	os.MkdirAll(l2Dir, 0755)
	l2File := filepath.Join(l2Dir, "h1.md")
	os.WriteFile(l2File, []byte("hypo"), 0644)

	fsm := &FSM{State: State{Phase: PhaseIdle}}

	// Helper for RoleAssignment
	ra := func(r Role) RoleAssignment {
		return RoleAssignment{Role: r, SessionID: "test", Context: "test"}
	}

	// Helper for EvidenceStub
	ev := func(uri string) *EvidenceStub {
		if uri == "" { return nil }
		return &EvidenceStub{URI: uri, Type: "test"}
	}

	// Valid transitions
	tests := []struct {
		name        string
		from        Phase
		to          Phase
		role        Role
		evidenceURI string
		expectedOk  bool
		expectedMsg string
	}{
		{"IdleToAbduction", PhaseIdle, PhaseAbduction, RoleAbductor, "any", true, "OK"}, // Evidence not strictly checked for Abduction entry
		{"AbductionToDeduction", PhaseAbduction, PhaseDeduction, RoleDeductor, l0Dir, true, "OK"},
		{"DeductionToInduction", PhaseDeduction, PhaseInduction, RoleInductor, l1File, true, "OK"},
		{"InductionToDeductionLoopback", PhaseInduction, PhaseDeduction, RoleDeductor, l0Dir, true, "OK"}, // Loopback might point to L0 logic? 
		// Actually loopback usually implies we have failed evidence. 
		// validateEvidence for Deduction checks if URI is a dir with files.
		// So l0Dir works.
		{"InductionToDecision", PhaseInduction, PhaseDecision, RoleDecider, l2File, true, "OK"},
		{"DecisionToIdle", PhaseDecision, PhaseIdle, RoleDecider, "any", true, "OK"},
		{"SelfLoopValid", PhaseAbduction, PhaseAbduction, RoleAbductor, "", true, "OK"}, 
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsm.State.Phase = tt.from
			ok, msg := fsm.CanTransition(tt.to, ra(tt.role), ev(tt.evidenceURI))
			if ok != tt.expectedOk {
				t.Errorf("CanTransition(%s -> %s by %s) got ok=%t, expected %t. Msg: %s", tt.from, tt.to, tt.role, ok, tt.expectedOk, msg)
			}
			if ok && msg != tt.expectedMsg {
				t.Errorf("CanTransition(%s -> %s by %s) got msg='%s', expected '%s'", tt.from, tt.to, tt.role, msg, tt.expectedMsg)
			}
		})
	}

	// Invalid transitions
	invalidTests := []struct {
		name        string
		from        Phase
		to          Phase
		role        Role
		evidenceURI string
		expectedOk  bool
	}{
		{"AbductionToInductionDirect", PhaseAbduction, PhaseInduction, RoleInductor, "", false},
		{"DeductionToAbduction", PhaseDeduction, PhaseAbduction, RoleAbductor, "", false},
		{"AbductorInDeduction", PhaseDeduction, PhaseDeduction, RoleAbductor, "", false},
		{"InvalidRoleForTransition", PhaseAbduction, PhaseDeduction, RoleAbductor, l0Dir, false},
		{"InvalidPhaseTransition", PhaseDecision, PhaseAbduction, RoleDecider, "", false},
		{"MissingEvidenceForDeduction", PhaseAbduction, PhaseDeduction, RoleDeductor, "", false},
	}

	for _, tt := range invalidTests {
		t.Run(tt.name, func(t *testing.T) {
			fsm.State.Phase = tt.from
			ok, _ := fsm.CanTransition(tt.to, ra(tt.role), ev(tt.evidenceURI))
			if ok != tt.expectedOk {
				t.Errorf("CanTransition(%s -> %s by %s) got ok=%t, expected %t", tt.from, tt.to, tt.role, ok, tt.expectedOk)
			}
		})
	}
}

func TestIsValidRoleForPhase(t *testing.T) {
	tests := []struct {
		name        string
		phase       Phase
		role        Role
		expected    bool
	}{
		{"IdleAnyRole", PhaseIdle, RoleAbductor, true},
		{"AbductorInAbduction", PhaseAbduction, RoleAbductor, true},
		{"DeductorInDeduction", PhaseDeduction, RoleDeductor, true},
		{"InductorInInduction", PhaseInduction, RoleInductor, true},
		{"DeciderInDecision", PhaseDecision, RoleDecider, true},
		{"AuditorInDecision", PhaseDecision, RoleAuditor, true},

		{"AbductorInDeductionInvalid", PhaseDeduction, RoleAbductor, false},
		{"InductorInAbductionInvalid", PhaseAbduction, RoleInductor, false},
		{"AbductorInDecisionInvalid", PhaseDecision, RoleAbductor, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidRoleForPhase(tt.phase, tt.role)
			if result != tt.expected {
				t.Errorf("isValidRoleForPhase(%s, %s) got %t, expected %t", tt.phase, tt.role, result, tt.expected)
			}
		})
	}
}