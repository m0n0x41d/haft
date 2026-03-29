package session

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/m0n0x41d/quint-code/db"
	"github.com/m0n0x41d/quint-code/internal/agent"
)

// SQLiteStore implements SessionStore and MessageStore using the project's quint.db.
type SQLiteStore struct {
	db *sql.DB
}

// Compile-time interface checks.
var (
	_ SessionStore = (*SQLiteStore)(nil)
	_ MessageStore = (*SQLiteStore)(nil)
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
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_sessions (id, parent_id, title, model, current_phase, depth, interaction, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.ParentID, sess.Title, sess.Model, string(sess.CurrentPhase),
		string(sess.Depth), string(sess.Interaction),
		sess.CreatedAt.Format(time.RFC3339),
		sess.UpdatedAt.Format(time.RFC3339),
	)
	return err
}

func (s *SQLiteStore) Get(ctx context.Context, id string) (*agent.Session, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, COALESCE(parent_id, ''), title, model, COALESCE(current_phase, ''),
		        COALESCE(depth, 'standard'), COALESCE(interaction, 'symbiotic'),
		        created_at, updated_at
		 FROM agent_sessions WHERE id = ?`, id)
	return scanSession(row)
}

func (s *SQLiteStore) Update(ctx context.Context, sess *agent.Session) error {
	sess.UpdatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`UPDATE agent_sessions SET title = ?, model = ?, current_phase = ?,
		        depth = ?, interaction = ?, updated_at = ?
		 WHERE id = ?`,
		sess.Title, sess.Model, string(sess.CurrentPhase),
		string(sess.Depth), string(sess.Interaction),
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
		        COALESCE(depth, 'standard'), COALESCE(interaction, 'symbiotic'),
		        created_at, updated_at
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
		return fmt.Errorf("marshal parts: %w", err)
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO agent_messages (id, session_id, role, parts, model, token_count, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.SessionID, string(msg.Role),
		string(partsJSON), msg.Model, msg.Tokens,
		msg.CreatedAt.Format(time.RFC3339),
	)
	return err
}

func (s *SQLiteStore) ListBySession(ctx context.Context, sessionID string) ([]agent.Message, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, role, parts, model, token_count, created_at
		 FROM agent_messages WHERE session_id = ? ORDER BY created_at ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []agent.Message
	for rows.Next() {
		msg, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, *msg)
	}
	return messages, rows.Err()
}

func (s *SQLiteStore) LastUserMessage(ctx context.Context, sessionID string) (string, error) {
	var partsJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT parts FROM agent_messages
		 WHERE session_id = ? AND role = 'user'
		 ORDER BY created_at DESC LIMIT 1`, sessionID).Scan(&partsJSON)
	if err != nil {
		return "", nil // no message found — not an error
	}
	parts, err := agent.UnmarshalParts([]byte(partsJSON))
	if err != nil {
		return partsJSON, nil
	}
	for _, p := range parts {
		if tp, ok := p.(agent.TextPart); ok {
			return tp.Text, nil
		}
	}
	return "", nil
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
	err := row.Scan(&sess.ID, &sess.ParentID, &sess.Title, &sess.Model, &phase, &depth, &interaction, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	sess.CurrentPhase = agent.Phase(phase)
	sess.Depth = agent.Depth(depth)
	sess.Interaction = agent.Interaction(interaction)
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
