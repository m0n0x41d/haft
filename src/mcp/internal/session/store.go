package session

import (
	"context"

	"github.com/m0n0x41d/quint-code/internal/agent"
)

// SessionStore persists agent sessions.
type SessionStore interface {
	Create(ctx context.Context, s *agent.Session) error
	Get(ctx context.Context, id string) (*agent.Session, error)
	Update(ctx context.Context, s *agent.Session) error
	ListRecent(ctx context.Context, limit int) ([]agent.Session, error)
}

// MessageStore persists conversation messages.
type MessageStore interface {
	Save(ctx context.Context, msg *agent.Message) error
	ListBySession(ctx context.Context, sessionID string) ([]agent.Message, error)
	LastUserMessage(ctx context.Context, sessionID string) (string, error)
}
