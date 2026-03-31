package session

import (
	"context"

	"github.com/m0n0x41d/haft/internal/agent"
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
	UpdateMessage(ctx context.Context, msg *agent.Message) error
	ListBySession(ctx context.Context, sessionID string) ([]agent.Message, error)
	LastUserMessage(ctx context.Context, sessionID string) (string, error)
	// DeleteOlderThan removes all but the most recent keepLastN messages for a session.
	DeleteOlderThan(ctx context.Context, sessionID string, keepLastN int) (int, error)
}

// CycleStore persists reasoning cycles.
type CycleStore interface {
	CreateCycle(ctx context.Context, cycle *agent.Cycle) error
	GetCycle(ctx context.Context, id string) (*agent.Cycle, error)
	UpdateCycle(ctx context.Context, cycle *agent.Cycle) error
	GetActiveCycle(ctx context.Context, sessionID string) (*agent.Cycle, error)
	ListCyclesBySession(ctx context.Context, sessionID string) ([]agent.Cycle, error)
}
