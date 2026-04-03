package db

import "database/sql"

// RunMigrations applies all pending kernel migrations to the database.
// Uses the shared Migrate runner with version tracking.
func RunMigrations(conn *sql.DB) error {
	return Migrate(conn, "schema_version", kernelMigrations)
}

// kernelMigrations defines the kernel schema evolution.
// Each migration has a version, description, and list of SQL statements.
// Append new migrations at the end. Never modify or reorder existing ones.
var kernelMigrations = []Migration{
	{
		Version:     1,
		Description: "Add parent_id to holons for L0->L1->L2 chain tracking",
		Statements:  []string{`ALTER TABLE holons ADD COLUMN parent_id TEXT REFERENCES holons(id)`},
	},
	{
		Version:     2,
		Description: "Add cached_r_score to holons for trust calculus",
		Statements:  []string{`ALTER TABLE holons ADD COLUMN cached_r_score REAL DEFAULT 0.0`},
	},
	{
		Version:     3,
		Description: "Add fpf_state table for FSM state",
		Statements: []string{`CREATE TABLE IF NOT EXISTS fpf_state (
			context_id TEXT PRIMARY KEY,
			active_role TEXT,
			active_session_id TEXT,
			active_role_context TEXT,
			last_commit TEXT,
			assurance_threshold REAL DEFAULT 0.8 CHECK(assurance_threshold BETWEEN 0.0 AND 1.0),
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`},
	},
	{
		Version:     4,
		Description: "Add FTS5 tables for full-text search",
		Statements: []string{
			`CREATE VIRTUAL TABLE IF NOT EXISTS holons_fts USING fts5(
				id, title, content, content='holons', content_rowid='rowid')`,
			`CREATE VIRTUAL TABLE IF NOT EXISTS evidence_fts USING fts5(
				id, content, content='evidence', content_rowid='rowid')`,
			`INSERT INTO holons_fts(holons_fts) VALUES('rebuild')`,
			`INSERT INTO evidence_fts(evidence_fts) VALUES('rebuild')`,
			`DROP TRIGGER IF EXISTS holons_ai`,
			`CREATE TRIGGER holons_ai AFTER INSERT ON holons BEGIN
				INSERT INTO holons_fts(rowid, id, title, content) VALUES (new.rowid, new.id, new.title, new.content);
			END`,
			`DROP TRIGGER IF EXISTS holons_ad`,
			`CREATE TRIGGER holons_ad AFTER DELETE ON holons BEGIN
				INSERT INTO holons_fts(holons_fts, rowid, id, title, content) VALUES('delete', old.rowid, old.id, old.title, old.content);
			END`,
			`DROP TRIGGER IF EXISTS holons_au`,
			`CREATE TRIGGER holons_au AFTER UPDATE ON holons BEGIN
				INSERT INTO holons_fts(holons_fts, rowid, id, title, content) VALUES('delete', old.rowid, old.id, old.title, old.content);
				INSERT INTO holons_fts(rowid, id, title, content) VALUES (new.rowid, new.id, new.title, new.content);
			END`,
			`DROP TRIGGER IF EXISTS evidence_ai`,
			`CREATE TRIGGER evidence_ai AFTER INSERT ON evidence BEGIN
				INSERT INTO evidence_fts(rowid, id, content) VALUES (new.rowid, new.id, new.content);
			END`,
			`DROP TRIGGER IF EXISTS evidence_ad`,
			`CREATE TRIGGER evidence_ad AFTER DELETE ON evidence BEGIN
				INSERT INTO evidence_fts(evidence_fts, rowid, id, content) VALUES('delete', old.rowid, old.id, old.content);
			END`,
			`DROP TRIGGER IF EXISTS evidence_au`,
			`CREATE TRIGGER evidence_au AFTER UPDATE ON evidence BEGIN
				INSERT INTO evidence_fts(evidence_fts, rowid, id, content) VALUES('delete', old.rowid, old.id, old.content);
				INSERT INTO evidence_fts(rowid, id, content) VALUES (new.rowid, new.id, new.content);
			END`,
		},
	},
	{
		Version:     5,
		Description: "Auto-resolve legacy reset DRRs",
		Statements: []string{
			`DROP TRIGGER IF EXISTS evidence_ai`,
			`INSERT INTO evidence (id, holon_id, type, content, verdict, created_at)
			SELECT 'migration-cleanup-' || id, id, 'abandonment',
				'Migrated: reset session marker, not a real decision.', 'PASS', CURRENT_TIMESTAMP
			FROM holons
			WHERE (type = 'DRR' OR layer = 'DRR') AND content LIKE '%No Decision%Reset%'
			AND NOT EXISTS (SELECT 1 FROM evidence e WHERE e.holon_id = holons.id AND e.type IN ('implementation', 'abandonment', 'supersession'))`,
			`INSERT INTO evidence_fts(evidence_fts) VALUES('rebuild')`,
			`CREATE TRIGGER evidence_ai AFTER INSERT ON evidence BEGIN
				INSERT INTO evidence_fts(rowid, id, content) VALUES (new.rowid, new.id, new.content);
			END`,
		},
	},
	{
		Version:     6,
		Description: "Add active_holons view",
		Statements: []string{
			`CREATE VIEW IF NOT EXISTS active_holons AS
			SELECT h.* FROM holons h
			WHERE h.layer NOT IN ('invalid')
			AND NOT EXISTS (SELECT 1 FROM relations r WHERE r.target_id = h.id AND r.relation_type IN ('selects', 'rejects', 'closes'))`,
		},
	},
	{
		Version:     7,
		Description: "Code Change Awareness: staleness tracking",
		Statements: []string{
			"ALTER TABLE evidence ADD COLUMN carrier_hash TEXT",
			"ALTER TABLE evidence ADD COLUMN carrier_commit TEXT",
			"ALTER TABLE evidence ADD COLUMN is_stale INTEGER DEFAULT 0",
			"ALTER TABLE evidence ADD COLUMN stale_reason TEXT",
			"ALTER TABLE evidence ADD COLUMN stale_since DATETIME",
			"ALTER TABLE holons ADD COLUMN needs_reverification INTEGER DEFAULT 0",
			"ALTER TABLE holons ADD COLUMN reverification_reason TEXT",
			"ALTER TABLE holons ADD COLUMN reverification_since DATETIME",
			"ALTER TABLE fpf_state ADD COLUMN last_commit_at DATETIME",
			"CREATE INDEX IF NOT EXISTS idx_evidence_carrier ON evidence(carrier_ref)",
			"CREATE INDEX IF NOT EXISTS idx_evidence_stale ON evidence(is_stale)",
			"CREATE INDEX IF NOT EXISTS idx_holons_reverification ON holons(needs_reverification)",
		},
	},
	{
		Version:     8,
		Description: "Decision Contexts: context_status and updated active_holons view",
		Statements: []string{
			"ALTER TABLE holons ADD COLUMN context_status TEXT DEFAULT NULL",
			"CREATE INDEX IF NOT EXISTS idx_holons_context_status ON holons(context_status)",
			"CREATE INDEX IF NOT EXISTS idx_relations_memberof ON relations(target_id, relation_type)",
			"DROP VIEW IF EXISTS active_holons",
			`CREATE VIEW active_holons AS
			SELECT h.* FROM holons h
			WHERE h.layer NOT IN ('invalid') AND h.type != 'context'
			AND (h.context_status IS NULL OR h.context_status = 'open')
			AND NOT EXISTS (SELECT 1 FROM relations r WHERE r.target_id = h.id AND r.relation_type IN ('selects', 'rejects', 'closes'))`,
		},
	},
	{
		Version:     9,
		Description: "Predictions tracking for L1-L2 enforcement",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS predictions (
				id TEXT PRIMARY KEY, holon_id TEXT NOT NULL, content TEXT NOT NULL,
				covered INTEGER DEFAULT 0, covered_by TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY(holon_id) REFERENCES holons(id), FOREIGN KEY(covered_by) REFERENCES evidence(id))`,
			"CREATE INDEX IF NOT EXISTS idx_predictions_holon ON predictions(holon_id)",
			"CREATE INDEX IF NOT EXISTS idx_predictions_uncovered ON predictions(holon_id) WHERE covered = 0",
		},
	},
	{
		Version:     10,
		Description: "Formality level on evidence for F-G-R triad",
		Statements:  []string{"ALTER TABLE evidence ADD COLUMN formality_level INTEGER DEFAULT 5"},
	},
	{
		Version:     11,
		Description: "Approach type on holons for NQD-CAL diversity",
		Statements: []string{
			"ALTER TABLE holons ADD COLUMN approach_type TEXT DEFAULT NULL",
			"CREATE INDEX IF NOT EXISTS idx_holons_approach_type ON holons(approach_type)",
		},
	},
	{
		Version:     12,
		Description: "Context facts for context.md projection",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS context_facts (
				category TEXT PRIMARY KEY, content TEXT NOT NULL, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		},
	},
	{
		Version:     13,
		Description: "v5 artifact model",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS artifacts (
				id TEXT PRIMARY KEY, kind TEXT NOT NULL, version INTEGER NOT NULL DEFAULT 1,
				status TEXT NOT NULL DEFAULT 'active', context TEXT, mode TEXT,
				title TEXT NOT NULL, content TEXT NOT NULL, file_path TEXT,
				valid_until TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
			`CREATE TABLE IF NOT EXISTS artifact_links (
				source_id TEXT NOT NULL REFERENCES artifacts(id), target_id TEXT NOT NULL REFERENCES artifacts(id),
				link_type TEXT NOT NULL, created_at TEXT NOT NULL, PRIMARY KEY (source_id, target_id, link_type))`,
			`CREATE TABLE IF NOT EXISTS evidence_items (
				id TEXT PRIMARY KEY, artifact_ref TEXT NOT NULL REFERENCES artifacts(id),
				type TEXT NOT NULL, content TEXT NOT NULL, verdict TEXT, carrier_ref TEXT,
				congruence_level INTEGER DEFAULT 3, formality_level INTEGER DEFAULT 5,
				valid_until TEXT, created_at TEXT NOT NULL)`,
			`CREATE TABLE IF NOT EXISTS affected_files (
				artifact_id TEXT NOT NULL REFERENCES artifacts(id), file_path TEXT NOT NULL,
				file_hash TEXT, PRIMARY KEY (artifact_id, file_path))`,
			`CREATE VIRTUAL TABLE IF NOT EXISTS artifacts_fts USING fts5(
				id, title, content, kind, tokenize='porter unicode61')`,
			`CREATE TRIGGER IF NOT EXISTS artifacts_fts_insert AFTER INSERT ON artifacts BEGIN
				INSERT INTO artifacts_fts(id, title, content, kind) VALUES (new.id, new.title, new.content, new.kind);
			END`,
			`CREATE TRIGGER IF NOT EXISTS artifacts_fts_update AFTER UPDATE ON artifacts BEGIN
				DELETE FROM artifacts_fts WHERE id = old.id;
				INSERT INTO artifacts_fts(id, title, content, kind) VALUES (new.id, new.title, new.content, new.kind);
			END`,
			`CREATE TRIGGER IF NOT EXISTS artifacts_fts_delete AFTER DELETE ON artifacts BEGIN
				DELETE FROM artifacts_fts WHERE id = old.id;
			END`,
			"CREATE INDEX IF NOT EXISTS idx_artifacts_kind ON artifacts(kind)",
			"CREATE INDEX IF NOT EXISTS idx_artifacts_context ON artifacts(context)",
			"CREATE INDEX IF NOT EXISTS idx_artifacts_status ON artifacts(status)",
			"CREATE INDEX IF NOT EXISTS idx_artifact_links_target ON artifact_links(target_id, link_type)",
			"CREATE INDEX IF NOT EXISTS idx_evidence_items_ref ON evidence_items(artifact_ref)",
			"CREATE INDEX IF NOT EXISTS idx_affected_files_path ON affected_files(file_path)",
		},
	},
	{
		Version:     14,
		Description: "Codebase awareness: module map and dependency graph",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS codebase_modules (
				module_id TEXT PRIMARY KEY, path TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
				lang TEXT, file_count INTEGER DEFAULT 0, last_scanned TEXT NOT NULL)`,
			"CREATE INDEX IF NOT EXISTS idx_codebase_modules_path ON codebase_modules(path)",
			`CREATE TABLE IF NOT EXISTS module_dependencies (
				source_module TEXT NOT NULL, target_module TEXT NOT NULL,
				dep_type TEXT NOT NULL DEFAULT 'import', file_path TEXT, last_scanned TEXT NOT NULL,
				PRIMARY KEY (source_module, target_module, dep_type))`,
			"CREATE INDEX IF NOT EXISTS idx_module_deps_target ON module_dependencies(target_module)",
		},
	},
	{
		Version:     15,
		Description: "FTS5 enrichment: search_keywords column",
		Statements: []string{
			"ALTER TABLE artifacts ADD COLUMN search_keywords TEXT DEFAULT ''",
			"DROP TRIGGER IF EXISTS artifacts_fts_insert",
			"DROP TRIGGER IF EXISTS artifacts_fts_update",
			"DROP TRIGGER IF EXISTS artifacts_fts_delete",
			"DROP TABLE IF EXISTS artifacts_fts",
			`CREATE VIRTUAL TABLE IF NOT EXISTS artifacts_fts USING fts5(
				id, title, content, kind, search_keywords, tokenize='porter unicode61')`,
			`CREATE TRIGGER IF NOT EXISTS artifacts_fts_insert AFTER INSERT ON artifacts BEGIN
				INSERT INTO artifacts_fts(id, title, content, kind, search_keywords)
				VALUES (new.id, new.title, new.content, new.kind, new.search_keywords);
			END`,
			`CREATE TRIGGER IF NOT EXISTS artifacts_fts_update AFTER UPDATE ON artifacts BEGIN
				DELETE FROM artifacts_fts WHERE id = old.id;
				INSERT INTO artifacts_fts(id, title, content, kind, search_keywords)
				VALUES (new.id, new.title, new.content, new.kind, new.search_keywords);
			END`,
			`CREATE TRIGGER IF NOT EXISTS artifacts_fts_delete AFTER DELETE ON artifacts BEGIN
				DELETE FROM artifacts_fts WHERE id = old.id;
			END`,
			`INSERT INTO artifacts_fts(id, title, content, kind, search_keywords)
				SELECT id, title, content, kind, COALESCE(search_keywords, '') FROM artifacts`,
		},
	},
	{
		Version:     16,
		Description: "Structured fields: canonical data alongside markdown body",
		Statements:  []string{"ALTER TABLE artifacts ADD COLUMN structured_data TEXT DEFAULT ''"},
	},
	{
		Version:     17,
		Description: "Symbol-level baselines for tree-sitter powered drift detection",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS affected_symbols (
				artifact_id TEXT NOT NULL REFERENCES artifacts(id),
				file_path TEXT NOT NULL,
				symbol_name TEXT NOT NULL,
				symbol_kind TEXT NOT NULL,
				symbol_line INTEGER,
				symbol_end_line INTEGER,
				symbol_hash TEXT,
				PRIMARY KEY (artifact_id, file_path, symbol_name)
			)`,
			"CREATE INDEX IF NOT EXISTS idx_affected_symbols_file ON affected_symbols(file_path)",
			"CREATE INDEX IF NOT EXISTS idx_affected_symbols_artifact ON affected_symbols(artifact_id)",
		},
	},
	{
		Version:     18,
		Description: "Claim scope on persisted evidence",
		Statements: []string{
			"ALTER TABLE evidence ADD COLUMN claim_scope TEXT DEFAULT '[]'",
			"ALTER TABLE evidence_items ADD COLUMN claim_scope TEXT DEFAULT '[]'",
		},
	},
	{
		Version:     19,
		Description: "Epistemic debt budget on shared FPF state",
		Statements: []string{
			"ALTER TABLE fpf_state ADD COLUMN epistemic_debt_budget REAL DEFAULT 30.0",
		},
	},
}
