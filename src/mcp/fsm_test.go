package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadState(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	// Test loading non-existent state (should initialize to IDLE)
	fsm, err := LoadState(stateFile)
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

	fsm2, err := LoadState(stateFile)
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
	fsm := &FSM{State: State{Phase: PhaseIdle}}

	// Valid transitions
	tests := []struct {
		name        string
		from        Phase
		to          Phase
		role        Role
		expectedOk  bool
		expectedMsg string
	}{
		{"IdleToAbduction", PhaseIdle, PhaseAbduction, RoleAbductor, true, "OK"},
		{"AbductionToDeduction", PhaseAbduction, PhaseDeduction, RoleDeductor, true, "OK"},
		{"DeductionToInduction", PhaseDeduction, PhaseInduction, RoleInductor, true, "OK"},
		{"InductionToDeductionLoopback", PhaseInduction, PhaseDeduction, RoleDeductor, true, "OK"},
		{"InductionToDecision", PhaseInduction, PhaseDecision, RoleDecider, true, "OK"},
		{"DecisionToIdle", PhaseDecision, PhaseIdle, RoleDecider, true, "OK"},
		{"SelfLoopValid", PhaseAbduction, PhaseAbduction, RoleAbductor, true, "OK"}, // Role is valid for self-loop
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsm.State.Phase = tt.from
			ok, msg := fsm.CanTransition(tt.to, tt.role)
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
		expectedOk  bool
	}{
		{"AbductionToInductionDirect", PhaseAbduction, PhaseInduction, RoleInductor, false},
		{"DeductionToAbduction", PhaseDeduction, PhaseAbduction, RoleAbductor, false},
		{"AbductorInDeduction", PhaseDeduction, PhaseDeduction, RoleAbductor, false}, // Invalid role for self-loop
		{"InvalidRoleForTransition", PhaseAbduction, PhaseDeduction, RoleAbductor, false}, // Abductor cannot transition to Deduction
		{"InvalidPhaseTransition", PhaseDecision, PhaseAbduction, RoleDecider, false},
	}

	for _, tt := range invalidTests {
		t.Run(tt.name, func(t *testing.T) {
			fsm.State.Phase = tt.from
			ok, _ := fsm.CanTransition(tt.to, tt.role)
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
