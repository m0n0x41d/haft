package session

import "github.com/m0n0x41d/quint-code/db"

// AgentMigrations defines the agent session/message schema evolution.
// Uses the shared db.Migrate runner with version tracking.
var AgentMigrations = []db.Migration{
	{
		Version:     1,
		Description: "Agent sessions table",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS agent_sessions (
				id TEXT PRIMARY KEY,
				title TEXT NOT NULL DEFAULT '',
				model TEXT NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			)`,
		},
	},
	{
		Version:     2,
		Description: "Agent messages table",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS agent_messages (
				id TEXT PRIMARY KEY,
				session_id TEXT NOT NULL REFERENCES agent_sessions(id),
				role TEXT NOT NULL,
				parts TEXT NOT NULL,
				model TEXT DEFAULT '',
				token_count INTEGER DEFAULT 0,
				created_at TEXT NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_agent_messages_session
				ON agent_messages(session_id, created_at)`,
		},
	},
	{
		Version:     3,
		Description: "Add phase column to sessions",
		Statements: []string{
			"ALTER TABLE agent_sessions ADD COLUMN current_phase TEXT DEFAULT ''",
		},
	},
	{
		Version:     4,
		Description: "Add depth and interaction columns",
		Statements: []string{
			"ALTER TABLE agent_sessions ADD COLUMN depth TEXT DEFAULT 'standard'",
			"ALTER TABLE agent_sessions ADD COLUMN interaction TEXT DEFAULT 'symbiotic'",
		},
	},
	{
		Version:     5,
		Description: "Add parent_id for subagent child sessions",
		Statements: []string{
			"ALTER TABLE agent_sessions ADD COLUMN parent_id TEXT DEFAULT ''",
		},
	},
}
