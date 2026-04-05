package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/agent"
)

// SQLiteStore implements SessionStore and MessageStore using the project's haft.db.
type SQLiteStore struct {
	db *sql.DB
}

// Compile-time interface checks.
var (
	_ SessionStore = (*SQLiteStore)(nil)
	_ MessageStore = (*SQLiteStore)(nil)
	_ CycleStore   = (*SQLiteStore)(nil)
)

// NewSQLiteStore creates a store and ensures agent tables exist.
func NewSQLiteStore(sqlDB *sql.DB) (*SQLiteStore, error) {
	if err := db.Migrate(sqlDB, "agent_schema_version", AgentMigrations); err != nil {
		return nil, fmt.Errorf("agent migration: %w", err)
	}
	return &SQLiteStore{db: sqlDB}, nil
}

// ---------------------------------------------------------------------------
// SessionStore
// ---------------------------------------------------------------------------

func (s *SQLiteStore) Create(ctx context.Context, sess *agent.Session) error {
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = time.Now().UTC()
	}
	if sess.UpdatedAt.IsZero() {
		sess.UpdatedAt = sess.CreatedAt
	}
	sess.SetExecutionMode(sess.ExecutionMode())
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_sessions (id, parent_id, title, model, current_phase, depth, interaction, yolo, active_cycle_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.ParentID, sess.Title, sess.Model, string(sess.CurrentPhase),
		string(sess.Depth), string(sess.ExecutionMode()), sess.Yolo, sess.ActiveCycleID,
		sess.CreatedAt.Format(time.RFC3339),
		sess.UpdatedAt.Format(time.RFC3339),
	)
	return err
}

func (s *SQLiteStore) Get(ctx context.Context, id string) (*agent.Session, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, COALESCE(parent_id, ''), title, model, COALESCE(current_phase, ''),
		        COALESCE(depth, 'standard'), COALESCE(interaction, 'symbiotic'), COALESCE(yolo, 0),
		        COALESCE(active_cycle_id, ''), created_at, updated_at
		 FROM agent_sessions WHERE id = ?`, id)
	return scanSession(row)
}

func (s *SQLiteStore) Update(ctx context.Context, sess *agent.Session) error {
	sess.UpdatedAt = time.Now().UTC()
	sess.SetExecutionMode(sess.ExecutionMode())
	_, err := s.db.ExecContext(ctx,
		`UPDATE agent_sessions SET title = ?, model = ?, current_phase = ?,
		        depth = ?, interaction = ?, yolo = ?, active_cycle_id = ?, updated_at = ?
		 WHERE id = ?`,
		sess.Title, sess.Model, string(sess.CurrentPhase),
		string(sess.Depth), string(sess.ExecutionMode()), sess.Yolo, sess.ActiveCycleID,
		sess.UpdatedAt.Format(time.RFC3339), sess.ID,
	)
	return err
}

func (s *SQLiteStore) ListRecent(ctx context.Context, limit int) ([]agent.Session, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, COALESCE(parent_id, ''), title, model, COALESCE(current_phase, ''),
		        COALESCE(depth, 'standard'), COALESCE(interaction, 'symbiotic'), COALESCE(yolo, 0),
		        COALESCE(active_cycle_id, ''), created_at, updated_at
		 FROM agent_sessions WHERE COALESCE(parent_id, '') = ''
		 ORDER BY updated_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []agent.Session
	for rows.Next() {
		sess, err := scanSessionRow(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *sess)
	}
	return sessions, rows.Err()
}

// ---------------------------------------------------------------------------
// MessageStore
// ---------------------------------------------------------------------------

func (s *SQLiteStore) Save(ctx context.Context, msg *agent.Message) error {
	partsJSON, err := agent.MarshalParts(msg.Parts)
	if err != nil {
		return fmt.Errorf("marshal message parts: %w", err)
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO agent_messages (id, session_id, role, parts_json, model, tokens, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.SessionID, string(msg.Role), string(partsJSON), msg.Model, msg.Tokens,
		msg.CreatedAt.Format(time.RFC3339),
	)
	return err
}

func (s *SQLiteStore) UpdateMessage(ctx context.Context, msg *agent.Message) error {
	partsJSON, err := agent.MarshalParts(msg.Parts)
	if err != nil {
		return fmt.Errorf("marshal message parts: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE agent_messages SET parts_json = ?, model = ?, tokens = ? WHERE id = ?`,
		string(partsJSON), msg.Model, msg.Tokens, msg.ID,
	)
	return err
}

func (s *SQLiteStore) ListBySession(ctx context.Context, sessionID string) ([]agent.Message, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, role, parts_json, model, tokens, created_at
		 FROM agent_messages WHERE session_id = ?
		 ORDER BY created_at ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []agent.Message
	for rows.Next() {
		msg, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, *msg)
	}
	return msgs, rows.Err()
}

func (s *SQLiteStore) LastUserMessage(ctx context.Context, sessionID string) (string, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT parts_json FROM agent_messages
		 WHERE session_id = ? AND role = 'user'
		 ORDER BY created_at DESC LIMIT 1`, sessionID)
	var partsJSON string
	if err := row.Scan(&partsJSON); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	parts, err := agent.UnmarshalParts([]byte(partsJSON))
	if err != nil {
		return "", err
	}
	for _, p := range parts {
		if tp, ok := p.(agent.TextPart); ok {
			return tp.Text, nil
		}
	}
	return "", nil
}

func (s *SQLiteStore) DeleteOlderThan(ctx context.Context, sessionID string, keepLastN int) (int, error) {
	if keepLastN <= 0 {
		keepLastN = 20
	}
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM agent_messages
		 WHERE session_id = ? AND id NOT IN (
			SELECT id FROM agent_messages WHERE session_id = ? ORDER BY created_at DESC LIMIT ?
		 )`,
		sessionID, sessionID, keepLastN,
	)
	if err != nil {
		return 0, err
	}
	rows, _ := res.RowsAffected()
	return int(rows), nil
}

// ---------------------------------------------------------------------------
// CycleStore
// ---------------------------------------------------------------------------

func (s *SQLiteStore) CreateCycle(ctx context.Context, cycle *agent.Cycle) error {
	if cycle.CreatedAt.IsZero() {
		cycle.CreatedAt = time.Now().UTC()
	}
	if cycle.UpdatedAt.IsZero() {
		cycle.UpdatedAt = cycle.CreatedAt
	}
	if normalized := agent.CanonicalizeCycleForPersistence(cycle); normalized != nil {
		*cycle = *normalized
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_cycles (id, session_id, phase, status, lineage, problem_ref, portfolio_ref, compared_portfolio_ref, selected_portfolio_ref, selected_variant_ref, decision_ref, r_eff, cl_min, skip_json, governance_json, f_eff, g_eff, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cycle.ID, cycle.SessionID, string(cycle.Phase), string(cycle.Status), cycle.LineageRef,
		cycle.ProblemRef, cycle.PortfolioRef, cycle.ComparedPortfolioRef, cycle.SelectedPortfolioRef, cycle.SelectedVariantRef, cycle.DecisionRef, effectiveREff(cycle), cycle.CLMin,
		marshalCycleSkips(cycle.SkipLog), marshalCycleGovernance(cycle.Governance), cycle.Assurance.F, marshalAssuranceG(cycle.Assurance.G),
		cycle.CreatedAt.Format(time.RFC3339), cycle.UpdatedAt.Format(time.RFC3339),
	)
	return err
}

func (s *SQLiteStore) UpdateCycle(ctx context.Context, cycle *agent.Cycle) error {
	cycle.UpdatedAt = time.Now().UTC()
	if normalized := agent.CanonicalizeCycleForPersistence(cycle); normalized != nil {
		normalized.UpdatedAt = cycle.UpdatedAt
		*cycle = *normalized
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE agent_cycles SET phase = ?, status = ?, lineage = ?, problem_ref = ?, portfolio_ref = ?, compared_portfolio_ref = ?, selected_portfolio_ref = ?, selected_variant_ref = ?, decision_ref = ?,
		        r_eff = ?, cl_min = ?, skip_json = ?, governance_json = ?, f_eff = ?, g_eff = ?, updated_at = ?
		 WHERE id = ?`,
		string(cycle.Phase), string(cycle.Status), cycle.LineageRef,
		cycle.ProblemRef, cycle.PortfolioRef, cycle.ComparedPortfolioRef, cycle.SelectedPortfolioRef, cycle.SelectedVariantRef, cycle.DecisionRef,
		effectiveREff(cycle), cycle.CLMin,
		marshalCycleSkips(cycle.SkipLog), marshalCycleGovernance(cycle.Governance), cycle.Assurance.F, marshalAssuranceG(cycle.Assurance.G),
		cycle.UpdatedAt.Format(time.RFC3339), cycle.ID,
	)
	return err
}

func (s *SQLiteStore) GetCycle(ctx context.Context, id string) (*agent.Cycle, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, session_id, phase, status, COALESCE(lineage, ''), COALESCE(problem_ref, ''),
		        COALESCE(portfolio_ref, ''), COALESCE(compared_portfolio_ref, ''), COALESCE(selected_portfolio_ref, ''), COALESCE(selected_variant_ref, ''), COALESCE(decision_ref, ''), COALESCE(r_eff, 0), COALESCE(cl_min, 3),
		        COALESCE(skip_json, '[]'), COALESCE(governance_json, '[]'), COALESCE(f_eff, 0), COALESCE(g_eff, '[]'), created_at, updated_at
		 FROM agent_cycles WHERE id = ?`, id)
	return scanCycle(row)
}

func (s *SQLiteStore) GetActiveCycle(ctx context.Context, sessionID string) (*agent.Cycle, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, session_id, phase, status, COALESCE(lineage, ''), COALESCE(problem_ref, ''),
		        COALESCE(portfolio_ref, ''), COALESCE(compared_portfolio_ref, ''), COALESCE(selected_portfolio_ref, ''), COALESCE(selected_variant_ref, ''), COALESCE(decision_ref, ''), COALESCE(r_eff, 0), COALESCE(cl_min, 3),
		        COALESCE(skip_json, '[]'), COALESCE(governance_json, '[]'), COALESCE(f_eff, 0), COALESCE(g_eff, '[]'), created_at, updated_at
		 FROM agent_cycles WHERE session_id = ? AND status = 'active'
		 ORDER BY updated_at DESC LIMIT 1`, sessionID)
	cycle, err := scanCycle(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return cycle, err
}

func (s *SQLiteStore) ListCyclesBySession(ctx context.Context, sessionID string) ([]agent.Cycle, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, phase, status, COALESCE(lineage, ''), COALESCE(problem_ref, ''),
		        COALESCE(portfolio_ref, ''), COALESCE(compared_portfolio_ref, ''), COALESCE(selected_portfolio_ref, ''), COALESCE(selected_variant_ref, ''), COALESCE(decision_ref, ''), COALESCE(r_eff, 0), COALESCE(cl_min, 3),
		        COALESCE(skip_json, '[]'), COALESCE(governance_json, '[]'), COALESCE(f_eff, 0), COALESCE(g_eff, '[]'), created_at, updated_at
		 FROM agent_cycles WHERE session_id = ?
		 ORDER BY created_at ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cycles []agent.Cycle
	for rows.Next() {
		c, err := scanCycle(rows)
		if err != nil {
			return nil, err
		}
		cycles = append(cycles, *c)
	}
	return cycles, rows.Err()
}

// ---------------------------------------------------------------------------
// Scanners
// ---------------------------------------------------------------------------

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSession(row rowScanner) (*agent.Session, error) {
	var sess agent.Session
	var createdAt, updatedAt, phase, depth, interaction string
	err := row.Scan(&sess.ID, &sess.ParentID, &sess.Title, &sess.Model, &phase, &depth, &interaction, &sess.Yolo, &sess.ActiveCycleID, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	sess.CurrentPhase = agent.Phase(phase)
	sess.Depth = agent.Depth(depth)
	sess.SetExecutionMode(agent.NormalizeExecutionMode(interaction))
	sess.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	sess.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &sess, nil
}

func scanSessionRow(rows *sql.Rows) (*agent.Session, error) {
	return scanSession(rows)
}

func scanMessage(rows *sql.Rows) (*agent.Message, error) {
	var msg agent.Message
	var partsJSON, createdAt string
	err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &partsJSON, &msg.Model, &msg.Tokens, &createdAt)
	if err != nil {
		return nil, err
	}
	msg.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	msg.Parts, err = agent.UnmarshalParts([]byte(partsJSON))
	if err != nil {
		return nil, fmt.Errorf("unmarshal message parts: %w", err)
	}
	return &msg, nil
}

func scanCycle(row rowScanner) (*agent.Cycle, error) {
	var c agent.Cycle
	var phase, status, createdAt, updatedAt, skipJSON, govJSON, gEffJSON string
	err := row.Scan(
		&c.ID, &c.SessionID, &phase, &status, &c.LineageRef,
		&c.ProblemRef, &c.PortfolioRef, &c.ComparedPortfolioRef, &c.SelectedPortfolioRef, &c.SelectedVariantRef, &c.DecisionRef, &c.REff, &c.CLMin,
		&skipJSON, &govJSON, &c.Assurance.F, &gEffJSON, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	c.Phase = agent.Phase(phase)
	c.Status = agent.CycleStatus(status)
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	c.Assurance.R = c.REff
	_ = json.Unmarshal([]byte(skipJSON), &c.SkipLog)
	_ = json.Unmarshal([]byte(govJSON), &c.Governance)
	_ = json.Unmarshal([]byte(gEffJSON), &c.Assurance.G)
	return &c, nil
}

func marshalCycleSkips(skips []agent.SkipEntry) string {
	if len(skips) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(skips)
	return string(b)
}

func marshalCycleGovernance(g []agent.GovernanceEntry) string {
	if len(g) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(g)
	return string(b)
}

func marshalAssuranceG(g []string) string {
	if len(g) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(g)
	return string(b)
}

func effectiveREff(cycle *agent.Cycle) float64 {
	if cycle.REff != 0 {
		return cycle.REff
	}
	return cycle.Assurance.R
}
