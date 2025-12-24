package fpf

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/quint-code/assurance"
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
	PhaseOperation Phase = "OPERATION"
)

// Role type is defined in roles.go

// RoleAssignment binds a Holder (SessionID) to a Role within a Context
type RoleAssignment struct {
	Role      Role   `json:"role"`
	SessionID string `json:"session_id"`
	Context   string `json:"context"`
}

// EvidenceStub represents the anchor required for a transition
type EvidenceStub struct {
	Type        string `json:"type"`
	URI         string `json:"uri"`
	Description string `json:"description"`
	HolonID     string `json:"holon_id"`
}

// State represents the persistent state of the FPF session
type State struct {
	Phase              Phase          `json:"phase"`
	ActiveRole         RoleAssignment `json:"active_role,omitempty"`
	LastCommit         string         `json:"last_commit,omitempty"`
	AssuranceThreshold float64        `json:"assurance_threshold,omitempty"`
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
	DB    *sql.DB
}

// LoadState reads state from fpf_state table in SQLite
func LoadState(contextID string, db *sql.DB) (*FSM, error) {
	fsm := &FSM{
		State: State{
			Phase:              PhaseIdle,
			AssuranceThreshold: 0.8,
		},
		DB: db,
	}

	if db == nil {
		return fsm, nil
	}

	row := db.QueryRow(`
		SELECT active_role, active_session_id, active_role_context, last_commit, assurance_threshold
		FROM fpf_state WHERE context_id = ?`, contextID)

	var activeRole, activeSessionID, activeRoleContext, lastCommit sql.NullString
	var threshold sql.NullFloat64

	err := row.Scan(&activeRole, &activeSessionID, &activeRoleContext, &lastCommit, &threshold)
	if err == sql.ErrNoRows {
		return fsm, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	if activeRole.Valid {
		fsm.State.ActiveRole = RoleAssignment{
			Role:      Role(activeRole.String),
			SessionID: activeSessionID.String,
			Context:   activeRoleContext.String,
		}
	}
	if lastCommit.Valid {
		fsm.State.LastCommit = lastCommit.String
	}
	if threshold.Valid {
		fsm.State.AssuranceThreshold = threshold.Float64
	}

	return fsm, nil
}

// GetPhase returns the current phase, deriving from DB if available
func (f *FSM) GetPhase() Phase {
	if f.DB != nil {
		return f.DerivePhase("default")
	}
	return f.State.Phase
}

// DerivePhase computes the current phase from ACTIVE holons in the database.
// Active holons are defined by the active_holons VIEW (migration v6).
//
// DESIGN: This is INFORMATIONAL ONLY - used for status display in quint_internalize.
// It does NOT gate any operations. Semantic preconditions handle validation.
// See roles.go for the design decision on removing phase gates.
func (f *FSM) DerivePhase(contextID string) Phase {
	if f.DB == nil {
		return PhaseIdle
	}

	// Count active holons by layer using the active_holons view
	rows, err := f.DB.QueryContext(context.Background(),
		"SELECT layer, COUNT(*) as count FROM active_holons WHERE context_id = ? GROUP BY layer",
		contextID)
	if err != nil {
		return PhaseIdle
	}
	defer rows.Close() //nolint:errcheck

	counts := make(map[string]int64)
	for rows.Next() {
		var layer string
		var count int64
		if err := rows.Scan(&layer, &count); err != nil {
			continue
		}
		counts[layer] = count
	}

	l0 := counts["L0"]
	l1 := counts["L1"]
	l2 := counts["L2"]
	drr := counts["DRR"]

	// Simple phase derivation based on what exists.
	// No complex "recent L0" or timestamp logic needed since phase is informational only.
	if drr > 0 {
		return PhaseDecision
	}
	if l2 > 0 {
		// Check if any L2 has audit_report evidence
		var hasAudit bool
		auditRow := f.DB.QueryRowContext(context.Background(), `
			SELECT EXISTS(
				SELECT 1 FROM evidence e
				JOIN active_holons h ON e.holon_id = h.id
				WHERE h.context_id = ? AND h.layer = 'L2'
				AND e.type = 'audit_report'
			)`, contextID)
		if err := auditRow.Scan(&hasAudit); err == nil && hasAudit {
			return PhaseAudit
		}
		return PhaseInduction
	}
	if l1 > 0 {
		return PhaseDeduction
	}
	if l0 > 0 {
		return PhaseAbduction
	}
	return PhaseIdle
}

// SaveState writes state to fpf_state table in SQLite
func (f *FSM) SaveState(contextID string) error {
	if f.DB == nil {
		return fmt.Errorf("database connection required for SaveState")
	}

	_, err := f.DB.Exec(`
		INSERT INTO fpf_state (context_id, active_role, active_session_id, active_role_context, last_commit, assurance_threshold, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(context_id) DO UPDATE SET
			active_role = excluded.active_role,
			active_session_id = excluded.active_session_id,
			active_role_context = excluded.active_role_context,
			last_commit = excluded.last_commit,
			assurance_threshold = excluded.assurance_threshold,
			updated_at = excluded.updated_at`,
		contextID,
		string(f.State.ActiveRole.Role),
		f.State.ActiveRole.SessionID,
		f.State.ActiveRole.Context,
		f.State.LastCommit,
		f.State.AssuranceThreshold,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}
	return nil
}

// GetAssuranceThreshold returns the configured threshold, defaulting to 0.8
func (f *FSM) GetAssuranceThreshold() float64 {
	if f.State.AssuranceThreshold <= 0 {
		return 0.8
	}
	return f.State.AssuranceThreshold
}

// CanTransition checks if a role can move the system to a target phase
func (f *FSM) CanTransition(target Phase, assignment RoleAssignment, evidence *EvidenceStub) (bool, string) {
	if assignment.Role == "" {
		return false, "Role is required"
	}

	currentPhase := f.GetPhase()

	if currentPhase == target {
		if isValidRoleForPhase(currentPhase, assignment.Role) {
			return true, "OK"
		}
		return false, fmt.Sprintf("Role %s is not active in %s phase", assignment.Role, currentPhase)
	}

	valid := []TransitionRule{
		{PhaseIdle, PhaseAbduction, RoleAbductor},
		{PhaseAbduction, PhaseDeduction, RoleDeductor},
		{PhaseDeduction, PhaseInduction, RoleInductor},
		{PhaseInduction, PhaseDeduction, RoleDeductor},
		{PhaseInduction, PhaseAudit, RoleAuditor},
		{PhaseInduction, PhaseDecision, RoleDecider},
		{PhaseAudit, PhaseDecision, RoleDecider},
		{PhaseDecision, PhaseIdle, RoleDecider},
		{PhaseDecision, PhaseOperation, RoleDecider},
	}

	isValidTransition := false
	for _, rule := range valid {
		if rule.From == currentPhase && rule.To == target {
			if rule.Role == assignment.Role {
				isValidTransition = true
				break
			}
		}
	}

	if !isValidTransition {
		return false, fmt.Sprintf("Invalid transition: %s -> %s by %s", currentPhase, target, assignment.Role)
	}

	if !validateEvidence(currentPhase, target, evidence) {
		return false, fmt.Sprintf("Transition to %s requires valid Evidence Anchor (A.10) from %s", target, currentPhase)
	}

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

func validateEvidence(fromPhase, toPhase Phase, evidence *EvidenceStub) bool {
	if evidence == nil || evidence.URI == "" {
		return false
	}

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
