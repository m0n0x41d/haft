package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Phase definitions
type Phase string

const (
	PhaseIdle      Phase = "IDLE"
	PhaseAbduction Phase = "ABDUCTION"
	PhaseDeduction Phase = "DEDUCTION"
	PhaseInduction Phase = "INDUCTION"
	PhaseDecision  Phase = "DECISION"
)

// Role definitions
type Role string

const (
	RoleAbductor Role = "Abductor"
	RoleDeductor Role = "Deductor"
	RoleInductor Role = "Inductor"
	RoleAuditor  Role = "Auditor"
	RoleDecider  Role = "Decider"
)

// State represents the persistent state of the FPF session
type State struct {
	Phase Phase `json:"phase"`
}

// TransitionRule defines a valid state change
type TransitionRule struct {
	From Phase
	To   Phase
	Role Role
}

// FSM manages the state transitions
type FSM struct {
	State State
}

// LoadState reads state from .fpf/state.json
func LoadState(path string) (*FSM, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &FSM{State: State{Phase: PhaseIdle}}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &FSM{State: state}, nil
}

// SaveState writes state to .fpf/state.json
func (f *FSM) SaveState(path string) error {
	data, err := json.MarshalIndent(f.State, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// CanTransition checks if a role can move the system to a target phase
func (f *FSM) CanTransition(target Phase, role Role) (bool, string) {
	// Self-loop (staying in phase) check
	if f.State.Phase == target {
		// Verify role is valid for current phase
		if isValidRoleForPhase(f.State.Phase, role) {
			return true, "OK"
		}
		return false, fmt.Sprintf("Role %s is not active in %s phase", role, f.State.Phase)
	}

	// Valid Transitions Map
	valid := []TransitionRule{
		{PhaseIdle, PhaseAbduction, RoleAbductor},
		{PhaseAbduction, PhaseDeduction, RoleDeductor}, // Handover
		{PhaseDeduction, PhaseInduction, RoleInductor}, // Handover
		{PhaseInduction, PhaseDeduction, RoleDeductor}, // Loopback (Refinement)
		{PhaseInduction, PhaseDecision, RoleDecider},   // Handover
		{PhaseDecision, PhaseIdle, RoleDecider},        // Close
	}

	for _, rule := range valid {
		if rule.From == f.State.Phase && rule.To == target {
			if rule.Role == role || (rule.From == PhaseAbduction && role == RoleAbductor) { 
				// Note: Abductor submits to Deduction, Deductor accepts. 
				// Simplified: The actor initiating the transition drives it.
				return true, "OK"
			}
		}
	}

	return false, fmt.Sprintf("Invalid transition: %s -> %s by %s", f.State.Phase, target, role)
}

func isValidRoleForPhase(phase Phase, role Role) bool {
	switch phase {
	case PhaseIdle:
		return true // Anyone can init
	case PhaseAbduction:
		return role == RoleAbductor
	case PhaseDeduction:
		return role == RoleDeductor
	case PhaseInduction:
		return role == RoleInductor
	case PhaseDecision:
		return role == RoleDecider || role == RoleAuditor
	}
	return false
}
