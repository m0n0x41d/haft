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

// RoleAssignment binds a Holder (SessionID) to a Role within a Context
type RoleAssignment struct {
	Role      Role   `json:"role"`
	SessionID string `json:"session_id"`
	Context   string `json:"context"` // e.g. "ProjectPhoenix"
}

// EvidenceStub represents the anchor required for a transition
type EvidenceStub struct {
	Type        string `json:"type"`        // e.g. "hypothesis", "test_result"
	URI         string `json:"uri"`         // e.g. "knowledge/L0/h1.md"
	Description string `json:"description"` // e.g. "Initial hypothesis"
}

// State represents the persistent state of the FPF session
type State struct {
	Phase      Phase          `json:"phase"`
	ActiveRole RoleAssignment `json:"active_role,omitempty"`
	LastCommit string         `json:"last_commit,omitempty"`
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

// LoadState reads state from .quint/state.json
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

// SaveState writes state to .quint/state.json
func (f *FSM) SaveState(path string) error {
	data, err := json.MarshalIndent(f.State, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// CanTransition checks if a role can move the system to a target phase
// It now requires a RoleAssignment and an optional EvidenceStub
func (f *FSM) CanTransition(target Phase, assignment RoleAssignment, evidence *EvidenceStub) (bool, string) {
	// 1. Role Validation
	if assignment.Role == "" {
		return false, "Role is required"
	}

	// Self-loop (staying in phase) check
	if f.State.Phase == target {
		// Verify role is valid for current phase
		if isValidRoleForPhase(f.State.Phase, assignment.Role) {
			return true, "OK"
		}
		return false, fmt.Sprintf("Role %s is not active in %s phase", assignment.Role, f.State.Phase)
	}

	// 2. Transition Validation
	valid := []TransitionRule{
		{PhaseIdle, PhaseAbduction, RoleAbductor},
		{PhaseAbduction, PhaseDeduction, RoleDeductor}, // Handover
		{PhaseDeduction, PhaseInduction, RoleInductor}, // Handover
		{PhaseInduction, PhaseDeduction, RoleDeductor}, // Loopback (Refinement)
		{PhaseInduction, PhaseDecision, RoleDecider},   // Handover
		{PhaseDecision, PhaseIdle, RoleDecider},        // Close
	}

	isValidTransition := false
	for _, rule := range valid {
		if rule.From == f.State.Phase && rule.To == target {
			if rule.Role == assignment.Role {
				isValidTransition = true
				break
			}
		}
	}

	if !isValidTransition {
		return false, fmt.Sprintf("Invalid transition: %s -> %s by %s", f.State.Phase, target, assignment.Role)
	}

	// 3. Evidence Anchoring (A.10)
	if evidenceRequired(target) {
		if evidence == nil || evidence.URI == "" {
			return false, fmt.Sprintf("Transition to %s requires Evidence Anchor (A.10)", target)
		}
		// In a real implementation, we would verify the file exists at evidence.URI
	}

	return true, "OK"
}

func evidenceRequired(target Phase) bool {
	// Entering Deduction requires an L0 hypothesis (Abduction -> Deduction)
	// Entering Induction requires Deductive Analysis (Deduction -> Induction)
	// Entering Decision requires Validation Results (Induction -> Decision)
	switch target {
	case PhaseDeduction, PhaseInduction, PhaseDecision:
		return true
	}
	return false
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
