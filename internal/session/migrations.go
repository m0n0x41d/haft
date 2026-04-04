package session

import "github.com/m0n0x41d/haft/db"

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
	{
		Version:     6,
		Description: "Reasoning cycles table",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS agent_cycles (
				id TEXT PRIMARY KEY,
				session_id TEXT NOT NULL,
				problem_ref TEXT DEFAULT '',
				portfolio_ref TEXT DEFAULT '',
				decision_ref TEXT DEFAULT '',
				phase TEXT NOT NULL DEFAULT 'framer',
				depth TEXT NOT NULL DEFAULT 'tactical',
				status TEXT NOT NULL DEFAULT 'active',
				lineage TEXT DEFAULT '',
				r_eff REAL DEFAULT 0.0,
				cl_min INTEGER DEFAULT 3,
				governance_json TEXT DEFAULT '[]',
				skip_json TEXT DEFAULT '[]',
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_cycles_session ON agent_cycles(session_id)`,
			`CREATE INDEX IF NOT EXISTS idx_cycles_status ON agent_cycles(status)`,
		},
	},
	{
		Version:     7,
		Description: "Add active_cycle_id to sessions",
		Statements: []string{
			"ALTER TABLE agent_sessions ADD COLUMN active_cycle_id TEXT DEFAULT ''",
		},
	},
	{
		Version:     8,
		Description: "Persist assurance tuple on reasoning cycles",
		Statements: []string{
			"ALTER TABLE agent_cycles ADD COLUMN f_eff INTEGER DEFAULT 0",
			"ALTER TABLE agent_cycles ADD COLUMN g_eff TEXT DEFAULT '[]'",
		},
	},
	{
		Version:     9,
		Description: "Add yolo column to sessions",
		Statements: []string{
			"ALTER TABLE agent_sessions ADD COLUMN yolo INTEGER DEFAULT 0",
		},
	},
	{
		Version:     10,
		Description: "Rename message columns to match code",
		Statements: []string{
			"ALTER TABLE agent_messages RENAME COLUMN parts TO parts_json",
			"ALTER TABLE agent_messages RENAME COLUMN token_count TO tokens",
		},
	},
	{
		Version:     11,
		Description: "Persist compared portfolio ref on reasoning cycles",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS agent_cycles (
				id TEXT PRIMARY KEY,
				session_id TEXT NOT NULL,
				problem_ref TEXT DEFAULT '',
				portfolio_ref TEXT DEFAULT '',
				compared_portfolio_ref TEXT DEFAULT '',
				decision_ref TEXT DEFAULT '',
				phase TEXT NOT NULL DEFAULT 'framer',
				depth TEXT NOT NULL DEFAULT 'tactical',
				status TEXT NOT NULL DEFAULT 'active',
				lineage TEXT DEFAULT '',
				r_eff REAL DEFAULT 0.0,
				cl_min INTEGER DEFAULT 3,
				governance_json TEXT DEFAULT '[]',
				skip_json TEXT DEFAULT '[]',
				f_eff INTEGER DEFAULT 0,
				g_eff TEXT DEFAULT '[]',
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_cycles_session ON agent_cycles(session_id)`,
			`CREATE INDEX IF NOT EXISTS idx_cycles_status ON agent_cycles(status)`,
			"ALTER TABLE agent_cycles ADD COLUMN f_eff INTEGER DEFAULT 0",
			"ALTER TABLE agent_cycles ADD COLUMN g_eff TEXT DEFAULT '[]'",
			"ALTER TABLE agent_cycles ADD COLUMN compared_portfolio_ref TEXT DEFAULT ''",
		},
	},
	{
		Version:     12,
		Description: "Persist explicit selected portfolio and variant refs on reasoning cycles",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS agent_cycles (
				id TEXT PRIMARY KEY,
				session_id TEXT NOT NULL,
				problem_ref TEXT DEFAULT '',
				portfolio_ref TEXT DEFAULT '',
				compared_portfolio_ref TEXT DEFAULT '',
				selected_portfolio_ref TEXT DEFAULT '',
				selected_variant_ref TEXT DEFAULT '',
				decision_ref TEXT DEFAULT '',
				phase TEXT NOT NULL DEFAULT 'framer',
				depth TEXT NOT NULL DEFAULT 'tactical',
				status TEXT NOT NULL DEFAULT 'active',
				lineage TEXT DEFAULT '',
				r_eff REAL DEFAULT 0.0,
				cl_min INTEGER DEFAULT 3,
				governance_json TEXT DEFAULT '[]',
				skip_json TEXT DEFAULT '[]',
				f_eff INTEGER DEFAULT 0,
				g_eff TEXT DEFAULT '[]',
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_cycles_session ON agent_cycles(session_id)`,
			`CREATE INDEX IF NOT EXISTS idx_cycles_status ON agent_cycles(status)`,
			"ALTER TABLE agent_cycles ADD COLUMN selected_portfolio_ref TEXT DEFAULT ''",
			"ALTER TABLE agent_cycles ADD COLUMN selected_variant_ref TEXT DEFAULT ''",
		},
	},
}
