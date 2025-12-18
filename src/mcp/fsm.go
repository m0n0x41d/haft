package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"quint-mcp/assurance"
)

// Phase definitions
type Phase string

const (
	PhaseIdle      Phase = "IDLE"
	PhaseAbduction Phase = "ABDUCTION"
	PhaseDeduction Phase = "DEDUCTION"
	PhaseInduction Phase = "INDUCTION"
	PhaseAudit     Phase = "AUDIT"
	PhaseDecision  Phase = "DECISION"
	PhaseOperation Phase = "OPERATION" // Added for B.3 check
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
	HolonID     string `json:"holon_id"`    // ID of the holon being transitioned
}

// State represents the persistent state of the FPF session
type State struct {
	Phase              Phase          `json:"phase"`
	ActiveRole         RoleAssignment `json:"active_role,omitempty"`
	LastCommit         string         `json:"last_commit,omitempty"`
	AssuranceThreshold float64        `json:"assurance_threshold,omitempty"` // B.3: default 0.8
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
	DB    *sql.DB // Added for Assurance checks
}

// LoadState reads state from .quint/state.json
func LoadState(path string, db *sql.DB) (*FSM, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &FSM{State: State{Phase: PhaseIdle}, DB: db}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &FSM{State: state, DB: db}, nil
}

// SaveState writes state to .quint/state.json
func (f *FSM) SaveState(path string) error {
	data, err := json.MarshalIndent(f.State, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetAssuranceThreshold returns the configured threshold, defaulting to 0.8
func (f *FSM) GetAssuranceThreshold() float64 {
	if f.State.AssuranceThreshold <= 0 {
		return 0.8 // Default threshold
	}
	return f.State.AssuranceThreshold
}

// CanTransition checks if a role can move the system to a target phase
func (f *FSM) CanTransition(target Phase, assignment RoleAssignment, evidence *EvidenceStub) (bool, string) {
	// 1. Role Validation
	if assignment.Role == "" {
		return false, "Role is required"
	}

	// Self-loop (staying in phase) check
	if f.State.Phase == target {
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
		{PhaseInduction, PhaseAudit, RoleAuditor},      // Handover to Audit
		{PhaseInduction, PhaseDecision, RoleDecider},   // Skip Audit (Optional)
		{PhaseAudit, PhaseDecision, RoleDecider},       // Handover from Audit
		{PhaseDecision, PhaseIdle, RoleDecider},        // Close
		{PhaseDecision, PhaseOperation, RoleDecider},   // Deploy
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
	if !validateEvidence(f.State.Phase, target, evidence) {
		return false, fmt.Sprintf("Transition to %s requires valid Evidence Anchor (A.10) from %s", target, f.State.Phase)
	}

	// 4. Assurance Guard (B.3) - Critical for Operation
	if target == PhaseOperation {
		if evidence == nil || evidence.HolonID == "" {
			return false, "Transition to Operation requires a specific Holon ID in evidence stub"
		}

		calc := assurance.New(f.DB)
		report, err := calc.CalculateReliability(context.Background(), evidence.HolonID)
		if err != nil {
			return false, fmt.Sprintf("Failed to calculate assurance: %v", err)
		}

		threshold := f.GetAssuranceThreshold()
		if report.FinalScore < threshold {
			return false, fmt.Sprintf("Transition Denied: Reliability (%.2f) is below threshold (%.2f). Weakest link: %s", report.FinalScore, threshold, report.WeakestLink)
		}
	}

	return true, "OK"
}

// validateEvidence checks the evidence stub based on the transition
func validateEvidence(fromPhase, toPhase Phase, evidence *EvidenceStub) bool {
	if evidence == nil || evidence.URI == "" {
		return false
	}

	// Helper to check file content
	checkFile := func(path string) bool {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return false
		}
		content, err := os.ReadFile(path)
		if err != nil || len(content) == 0 {
			return false
		}
		return true
	}

	switch toPhase {
	case PhaseDeduction:
		info, err := os.Stat(evidence.URI)
		if err != nil || !info.IsDir() {
			return false
		}
		files, err := os.ReadDir(evidence.URI)
		if err != nil || len(files) == 0 {
			return false
		}
		return true

	case PhaseInduction:
		if !strings.Contains(evidence.URI, "knowledge/L1/") || filepath.Ext(evidence.URI) != ".md" {
			return false
		}
		return checkFile(evidence.URI)
	
	case PhaseAudit:
		if !strings.Contains(evidence.URI, "knowledge/L2/") || filepath.Ext(evidence.URI) != ".md" {
			return false
		}
		return checkFile(evidence.URI)

	case PhaseDecision:
		if !strings.Contains(evidence.URI, "knowledge/L2/") || filepath.Ext(evidence.URI) != ".md" {
			return false
		}
		return checkFile(evidence.URI)
	}
	return true
}

func isValidRoleForPhase(phase Phase, role Role) bool {
	switch phase {
	case PhaseIdle:
		return true
	case PhaseAbduction:
		return role == RoleAbductor
	case PhaseDeduction:
		return role == RoleDeductor
	case PhaseInduction:
		return role == RoleInductor
	case PhaseAudit:
		return role == RoleAuditor
	case PhaseDecision:
		return role == RoleDecider || role == RoleAuditor
	case PhaseOperation:
		return role == RoleDecider
	}
	return false
}