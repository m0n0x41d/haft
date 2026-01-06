package fpf

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ContextStage represents the derived stage of a decision context.
// Replaces global Phase - stage is now per-context and derived from hypotheses.
type ContextStage string

const (
	StageEmpty           ContextStage = "EMPTY"
	StageNeedsVerify     ContextStage = "NEEDS_VERIFICATION"
	StageNeedsValidation ContextStage = "NEEDS_VALIDATION"
	StageNeedsAudit      ContextStage = "NEEDS_AUDIT"
	StageReadyToDecide   ContextStage = "READY_TO_DECIDE"
)

// State holds session configuration (not phase - phase is removed).
type State struct {
	ActiveRole         RoleAssignment `json:"active_role,omitempty"`
	LastCommit         string         `json:"last_commit,omitempty"`
	AssuranceThreshold float64        `json:"assurance_threshold,omitempty"`
}

// RoleAssignment tracks the current role assignment (kept for audit logging).
type RoleAssignment struct {
	Role      Role   `json:"role"`
	SessionID string `json:"session_id"`
	Context   string `json:"context"`
}

// FSM holds state configuration. Global phase removed - use GetContextStage for per-context stage.
type FSM struct {
	State State
	DB    *sql.DB
}

// LoadState loads session state from database.
func LoadState(contextID string, db *sql.DB) (*FSM, error) {
	fsm := &FSM{
		State: State{
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

// SaveState persists session state to database.
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

// GetAssuranceThreshold returns the configured assurance threshold.
func (f *FSM) GetAssuranceThreshold() float64 {
	if f.State.AssuranceThreshold <= 0 {
		return 0.8
	}
	return f.State.AssuranceThreshold
}

// GetContextStage computes the stage of a decision context from its hypotheses.
// This replaces global phase - stage is derived per-context, not stored.
func (f *FSM) GetContextStage(decisionContextID string) ContextStage {
	if f.DB == nil {
		return StageEmpty
	}

	rows, err := f.DB.QueryContext(context.Background(), `
		SELECT h.layer, COUNT(*) as count
		FROM holons h
		JOIN relations r ON h.id = r.source_id
		WHERE r.target_id = ? AND r.relation_type = 'memberOf'
		  AND h.layer NOT IN ('invalid')
		GROUP BY h.layer`,
		decisionContextID)
	if err != nil {
		return StageEmpty
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

	if l2 > 0 {
		var allL2Audited bool
		auditRow := f.DB.QueryRowContext(context.Background(), `
			SELECT NOT EXISTS(
				SELECT 1 FROM holons h
				JOIN relations r ON h.id = r.source_id
				WHERE r.target_id = ? AND r.relation_type = 'memberOf'
				  AND h.layer = 'L2'
				  AND NOT EXISTS (
				      SELECT 1 FROM evidence e
				      WHERE e.holon_id = h.id AND e.type = 'audit_report'
				  )
			)`, decisionContextID)
		if err := auditRow.Scan(&allL2Audited); err == nil && allL2Audited && l2 > 0 {
			return StageReadyToDecide
		}
		return StageNeedsAudit
	}
	if l1 > 0 {
		return StageNeedsValidation
	}
	if l0 > 0 {
		return StageNeedsVerify
	}
	return StageEmpty
}

// GetContextStageDescription returns a human-readable description and next action for a stage.
func GetContextStageDescription(stage ContextStage) (description, nextAction string) {
	switch stage {
	case StageEmpty:
		return "No hypotheses yet", "Use /q1-hypothesize to add hypotheses"
	case StageNeedsVerify:
		return "Hypotheses need verification", "Use /q2-verify to verify L0 hypotheses"
	case StageNeedsValidation:
		return "Hypotheses need validation", "Use /q3-validate to test L1 hypotheses"
	case StageNeedsAudit:
		return "L2 hypotheses need audit", "Use /q4-audit to audit L2 hypotheses"
	case StageReadyToDecide:
		return "Ready for decision", "Use /q5-decide to finalize decision"
	default:
		return "Unknown stage", "Check context status"
	}
}
