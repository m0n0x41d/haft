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
	{
		Version:     20,
		Description: "Congruence level on durable evidence",
		Statements: []string{
			"ALTER TABLE evidence ADD COLUMN congruence_level INTEGER",
		},
	},
	{
		Version:     21,
		Description: "Exact claim refs on persisted artifact evidence",
		Statements: []string{
			"ALTER TABLE evidence_items ADD COLUMN claim_refs TEXT DEFAULT '[]'",
		},
	},
	{
		Version:     22,
		Description: "Desktop runtime task persistence",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS desktop_tasks (
				id TEXT PRIMARY KEY,
				project_name TEXT NOT NULL,
				project_path TEXT NOT NULL,
				title TEXT NOT NULL,
				agent TEXT NOT NULL,
				status TEXT NOT NULL,
				prompt TEXT NOT NULL,
				branch TEXT NOT NULL DEFAULT '',
				worktree INTEGER NOT NULL DEFAULT 0,
				worktree_path TEXT NOT NULL DEFAULT '',
				reused_worktree INTEGER NOT NULL DEFAULT 0,
				error_message TEXT NOT NULL DEFAULT '',
				output_tail TEXT NOT NULL DEFAULT '',
				started_at TEXT NOT NULL,
				completed_at TEXT,
				updated_at TEXT NOT NULL,
				archived_at TEXT
			)`,
			"CREATE INDEX IF NOT EXISTS idx_desktop_tasks_project_status ON desktop_tasks(project_path, status)",
			"CREATE INDEX IF NOT EXISTS idx_desktop_tasks_started_at ON desktop_tasks(started_at DESC)",
			"CREATE INDEX IF NOT EXISTS idx_desktop_tasks_worktree_path ON desktop_tasks(worktree_path)",
		},
	},
	{
		Version:     23,
		Description: "Desktop governance scan state and problem candidates",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS desktop_governance_state (
				state_key TEXT PRIMARY KEY,
				state_value TEXT NOT NULL DEFAULT '',
				updated_at TEXT NOT NULL
			)`,
			`CREATE TABLE IF NOT EXISTS desktop_problem_candidates (
				id TEXT PRIMARY KEY,
				title TEXT NOT NULL,
				signal TEXT NOT NULL,
				acceptance TEXT NOT NULL,
				context TEXT NOT NULL DEFAULT 'desktop-governance',
				category TEXT NOT NULL,
				source_artifact_ref TEXT NOT NULL DEFAULT '',
				source_title TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'active',
				problem_ref TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			)`,
			"CREATE INDEX IF NOT EXISTS idx_desktop_problem_candidates_status ON desktop_problem_candidates(status, updated_at DESC)",
			"CREATE INDEX IF NOT EXISTS idx_desktop_problem_candidates_source ON desktop_problem_candidates(source_artifact_ref, status)",
		},
	},
	{
		Version:     24,
		Description: "Desktop automation flows",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS desktop_flows (
				id TEXT PRIMARY KEY,
				project_name TEXT NOT NULL,
				project_path TEXT NOT NULL,
				title TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				template_id TEXT NOT NULL DEFAULT '',
				agent TEXT NOT NULL,
				prompt TEXT NOT NULL,
				schedule TEXT NOT NULL,
				branch TEXT NOT NULL DEFAULT '',
				use_worktree INTEGER NOT NULL DEFAULT 0,
				enabled INTEGER NOT NULL DEFAULT 1,
				last_task_id TEXT NOT NULL DEFAULT '',
				last_run_at TEXT,
				next_run_at TEXT,
				last_error TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			)`,
			"CREATE INDEX IF NOT EXISTS idx_desktop_flows_project_enabled ON desktop_flows(project_path, enabled)",
			"CREATE INDEX IF NOT EXISTS idx_desktop_flows_next_run ON desktop_flows(next_run_at)",
		},
	},
	{
		Version:     25,
		Description: "Add auto_run column to desktop tasks",
		Statements: []string{
			"ALTER TABLE desktop_tasks ADD COLUMN auto_run INTEGER NOT NULL DEFAULT 0",
		},
	},
	{
		Version:     26,
		Description: "Add structured chat transcript persistence to desktop tasks",
		Statements: []string{
			"ALTER TABLE desktop_tasks ADD COLUMN chat_blocks_json TEXT NOT NULL DEFAULT '[]'",
			"ALTER TABLE desktop_tasks ADD COLUMN raw_output TEXT NOT NULL DEFAULT ''",
		},
	},
	{
		Version:     27,
		Description: "Backfill desktop task transcript fallbacks",
		Statements: []string{
			"UPDATE desktop_tasks SET chat_blocks_json = '[]' WHERE TRIM(COALESCE(chat_blocks_json, '')) = ''",
			"UPDATE desktop_tasks SET raw_output = output_tail WHERE TRIM(COALESCE(raw_output, '')) = '' AND TRIM(COALESCE(output_tail, '')) != ''",
		},
	},
	{
		Version:     28,
		Description: "Add spec_section_baselines for SpecSection drift detection",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS spec_section_baselines (
				project_id TEXT NOT NULL,
				section_id TEXT NOT NULL,
				hash TEXT NOT NULL,
				captured_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				approved_by TEXT NOT NULL DEFAULT '',
				PRIMARY KEY (project_id, section_id)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_spec_section_baselines_project ON spec_section_baselines(project_id)`,
		},
	},
	{
		Version:     29,
		Description: "Add artifact_embeddings cache for the optional hybrid recall layer",
		Statements: []string{
			// Optional vector cache for hybrid recall (dec-20260605-fe77b358).
			// One row per (artifact, model contract); content_hash gates re-embed
			// when an artifact changes; vector is float32 little-endian (dim*4
			// bytes). Additive — dropping it just forces a recompute, never a
			// migration to reverse. Search stays brute-force in-memory cosine.
			`CREATE TABLE IF NOT EXISTS artifact_embeddings (
				artifact_id TEXT NOT NULL,
				provider TEXT NOT NULL,
				model TEXT NOT NULL,
				dim INTEGER NOT NULL,
				content_hash TEXT NOT NULL,
				vector BLOB NOT NULL,
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (artifact_id, provider, model, dim)
			)`,
		},
	},
	{
		Version:     30,
		Description: "Evidence provenance: machine-collected evidence must stay distinguishable from human-reviewed (dec-20260611-overseer-maintenance-executor)",
		Statements: []string{
			"ALTER TABLE evidence_items ADD COLUMN provenance TEXT DEFAULT ''",
		},
	},
	{
		Version:     31,
		Description: "Versioned evidence formality scale with legacy bridge diagnostics",
		Statements: []string{
			"ALTER TABLE evidence_items ADD COLUMN formality_scale_id TEXT DEFAULT ''",
			"ALTER TABLE evidence_items ADD COLUMN formality_bridge TEXT DEFAULT ''",
		},
	},
	{
		Version:     32,
		Description: "Add current SpecSection edition storage for SQL-backed spec sync",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS spec_section_editions (
				project_id TEXT NOT NULL,
				section_id TEXT NOT NULL,
				semantic_hash TEXT NOT NULL,
				section_json TEXT NOT NULL,
				source_kind TEXT NOT NULL DEFAULT '',
				carrier_path TEXT NOT NULL DEFAULT '',
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (project_id, section_id)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_spec_section_editions_project ON spec_section_editions(project_id)`,
			`CREATE INDEX IF NOT EXISTS idx_spec_section_editions_hash ON spec_section_editions(project_id, semantic_hash)`,
		},
	},
	{
		Version:     33,
		Description: "Durable symbol-anchor governance bindings and rebind history",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS artifact_symbol_bindings (
				artifact_id TEXT NOT NULL REFERENCES artifacts(id),
				anchor_id TEXT NOT NULL,
				anchor_version INTEGER NOT NULL,
				file_path TEXT NOT NULL,
				language TEXT NOT NULL,
				symbol_name TEXT NOT NULL,
				symbol_kind TEXT NOT NULL,
				receiver TEXT NOT NULL DEFAULT '',
				qualified_name TEXT NOT NULL,
				signature_hash TEXT NOT NULL,
				symbol_line INTEGER NOT NULL DEFAULT 0,
				symbol_end_line INTEGER NOT NULL DEFAULT 0,
				body_hash TEXT NOT NULL DEFAULT '',
				binding_status TEXT NOT NULL DEFAULT 'active',
				resolution_source TEXT NOT NULL DEFAULT '',
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (artifact_id, anchor_id)
			)`,
			"CREATE INDEX IF NOT EXISTS idx_artifact_symbol_bindings_anchor ON artifact_symbol_bindings(anchor_id)",
			"CREATE INDEX IF NOT EXISTS idx_artifact_symbol_bindings_file ON artifact_symbol_bindings(file_path)",
			`CREATE TABLE IF NOT EXISTS symbol_rebind_history (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				artifact_id TEXT NOT NULL REFERENCES artifacts(id),
				previous_anchor_id TEXT NOT NULL,
				current_anchor_id TEXT NOT NULL,
				reason TEXT NOT NULL,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			"CREATE INDEX IF NOT EXISTS idx_symbol_rebind_history_artifact ON symbol_rebind_history(artifact_id, created_at)",
		},
	},
	{
		Version:     34,
		Description: "Canonical append-only authority ledger and project-profile admission records",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS authority_presentations (
				presentation_id TEXT PRIMARY KEY,
				speech_act_ref TEXT NOT NULL,
				speech_act_digest TEXT NOT NULL,
				authorization_content_ref TEXT NOT NULL,
				authorization_content_digest TEXT NOT NULL,
				permission_ref TEXT NOT NULL UNIQUE,
				permission_digest TEXT NOT NULL,
				permission_modality TEXT NOT NULL CHECK(permission_modality = 'MAY'),
				permission_source_speech_act_ref TEXT NOT NULL,
				permission_subject_role_assignment_ref TEXT NOT NULL,
				permission_authorization_content_ref TEXT NOT NULL,
				permission_action_kind TEXT NOT NULL,
				permission_project_root TEXT NOT NULL,
				permission_method_description_ref TEXT NOT NULL,
				permission_valid_from TEXT NOT NULL,
				permission_valid_until TEXT NOT NULL,
				permission_single_use_key TEXT NOT NULL,
				permission_profile_admission_predicate_ref TEXT NOT NULL,
				permission_context_policy_ref TEXT NOT NULL,
				context_policy_ref TEXT NOT NULL,
				context_policy_digest TEXT NOT NULL,
				action_kind TEXT NOT NULL,
				project_root TEXT NOT NULL,
				profile_author_role_assignment_ref TEXT NOT NULL REFERENCES profile_author_role_assignments(role_assignment_ref),
				profile_author_role_assignment_digest TEXT NOT NULL,
				method_description_ref TEXT NOT NULL REFERENCES profile_onboarding_method_descriptions(method_description_ref),
				method_description_digest TEXT NOT NULL,
				method_contract_ref TEXT NOT NULL REFERENCES profile_onboarding_method_contracts(method_contract_ref),
				method_contract_digest TEXT NOT NULL,
				classifier_version TEXT NOT NULL,
				policy_version TEXT NOT NULL,
				session_ref TEXT NOT NULL,
				allowed_work_from TEXT NOT NULL,
				allowed_work_until TEXT NOT NULL,
				basis_observation_from TEXT NOT NULL,
				basis_observation_until TEXT NOT NULL,
				valid_from TEXT NOT NULL,
				valid_until TEXT NOT NULL,
				single_use_key TEXT NOT NULL UNIQUE,
				project_binding_digest TEXT NOT NULL,
				envelope_digest TEXT NOT NULL,
				presentation_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL,
				UNIQUE(speech_act_ref, authorization_content_ref, permission_ref, context_policy_ref)
			) WITHOUT ROWID`,
			`CREATE TABLE IF NOT EXISTS authority_resolution_records (
				authority_resolution_id TEXT PRIMARY KEY,
				presentation_id TEXT NOT NULL UNIQUE REFERENCES authority_presentations(presentation_id),
				presentation_digest TEXT NOT NULL UNIQUE,
				profile_author_role_assignment_ref TEXT NOT NULL REFERENCES profile_author_role_assignments(role_assignment_ref),
				profile_author_role_assignment_digest TEXT NOT NULL,
				method_description_ref TEXT NOT NULL REFERENCES profile_onboarding_method_descriptions(method_description_ref),
				method_description_digest TEXT NOT NULL,
				method_contract_ref TEXT NOT NULL REFERENCES profile_onboarding_method_contracts(method_contract_ref),
				method_contract_digest TEXT NOT NULL,
				verifier_identity TEXT NOT NULL,
				verifier_version TEXT NOT NULL,
				verification_policy_ref TEXT NOT NULL,
				verification_policy_digest TEXT NOT NULL,
				resolved_at TEXT NOT NULL,
				valid_until TEXT NOT NULL,
				authority_resolution_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL
			) WITHOUT ROWID`,
			`CREATE TABLE IF NOT EXISTS authority_uses (
				use_id TEXT PRIMARY KEY,
				authority_resolution_ref TEXT NOT NULL UNIQUE REFERENCES authority_resolution_records(authority_resolution_id),
				authority_resolution_digest TEXT NOT NULL,
				single_use_key TEXT NOT NULL UNIQUE,
				action_kind TEXT NOT NULL,
				project_root TEXT NOT NULL,
				project_binding_digest TEXT NOT NULL,
				envelope_digest TEXT NOT NULL,
				authority_record_ref TEXT NOT NULL REFERENCES authority_presentations(presentation_id),
				authority_record_digest TEXT NOT NULL,
				admission_request_digest TEXT NOT NULL,
				verifier_identity TEXT NOT NULL,
				verifier_version TEXT NOT NULL,
				committed_result_ref TEXT NOT NULL UNIQUE REFERENCES project_profile_admissions(admission_id),
				committed_result_digest TEXT NOT NULL,
				consumed_at TEXT NOT NULL
			) WITHOUT ROWID`,
			`CREATE TABLE IF NOT EXISTS profile_onboarding_method_descriptions (
				method_description_ref TEXT PRIMARY KEY,
				described_method_ref TEXT NOT NULL,
				bounded_context_ref TEXT NOT NULL,
				source_revision TEXT NOT NULL,
				edition TEXT NOT NULL,
				required_role_ref TEXT NOT NULL,
				required_system_kind TEXT NOT NULL CHECK(required_system_kind = 'U.System'),
				state_plane_ref TEXT NOT NULL,
				affected_ref_kind TEXT NOT NULL,
				effect_witness_rule_ref TEXT NOT NULL,
				method_description_json TEXT NOT NULL CHECK(json_valid(method_description_json)),
				method_description_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL
			) WITHOUT ROWID`,
			`CREATE TABLE IF NOT EXISTS profile_onboarding_method_contracts (
				method_contract_ref TEXT PRIMARY KEY,
				edition TEXT NOT NULL,
				method_description_ref TEXT NOT NULL REFERENCES profile_onboarding_method_descriptions(method_description_ref),
				method_description_digest TEXT NOT NULL,
				bounded_context_ref TEXT NOT NULL,
				role_admission_policy_ref TEXT NOT NULL,
				system_admission_policy_ref TEXT NOT NULL,
				parameter_spec_set_digest TEXT NOT NULL,
				accepted_result_kinds_json TEXT NOT NULL CHECK(json_valid(accepted_result_kinds_json)),
				required_occurrence_slots_json TEXT NOT NULL CHECK(json_valid(required_occurrence_slots_json)),
				occurrence_coverage_rule_refs_json TEXT NOT NULL CHECK(json_valid(occurrence_coverage_rule_refs_json)),
				effect_state_witness_rule_ref TEXT NOT NULL,
				acceptance_standard_ref TEXT NOT NULL,
				acceptance_standard_edition TEXT NOT NULL,
				holder_equals_executed_within_rule_ref TEXT NOT NULL,
				method_contract_json TEXT NOT NULL CHECK(json_valid(method_contract_json)),
				method_contract_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL
			) WITHOUT ROWID`,
			`CREATE TABLE IF NOT EXISTS profile_onboarding_executor_system_admissions (
				system_admission_ref TEXT PRIMARY KEY,
				system_ref TEXT NOT NULL,
				admitted_system_kind TEXT NOT NULL CHECK(admitted_system_kind = 'U.System'),
				bounded_context_ref TEXT NOT NULL,
				governing_pattern_ref TEXT NOT NULL CHECK(governing_pattern_ref = 'A.1'),
				identity_basis_kind TEXT NOT NULL CHECK(identity_basis_kind IN ('kernel_owned', 'operator_designated')),
				identity_basis_system_ref TEXT NOT NULL,
				identity_basis_kernel_identity TEXT NOT NULL DEFAULT '',
				identity_basis_kernel_version TEXT NOT NULL DEFAULT '',
				identity_basis_designation_ref TEXT NOT NULL DEFAULT '',
				identity_basis_designation_digest TEXT NOT NULL DEFAULT '',
				acting_eligibility_basis_ref TEXT NOT NULL,
				acting_eligibility_basis_digest TEXT NOT NULL,
				session_ref TEXT NOT NULL,
				valid_from TEXT NOT NULL,
				valid_until TEXT NOT NULL,
				method_description_ref TEXT NOT NULL REFERENCES profile_onboarding_method_descriptions(method_description_ref),
				method_description_digest TEXT NOT NULL,
				method_contract_ref TEXT NOT NULL REFERENCES profile_onboarding_method_contracts(method_contract_ref),
				method_contract_digest TEXT NOT NULL,
				system_admission_policy_ref TEXT NOT NULL,
				system_admission_json TEXT NOT NULL CHECK(json_valid(system_admission_json)),
				system_admission_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL,
				CHECK(session_ref != ''),
				CHECK(identity_basis_system_ref = system_ref),
				CHECK(
					(identity_basis_kind = 'kernel_owned'
						AND identity_basis_kernel_identity != ''
						AND identity_basis_kernel_version != ''
						AND identity_basis_designation_ref = ''
						AND identity_basis_designation_digest = '')
					OR (identity_basis_kind = 'operator_designated'
						AND identity_basis_kernel_identity = ''
						AND identity_basis_kernel_version = ''
						AND identity_basis_designation_ref != ''
						AND identity_basis_designation_digest != '')
				),
				CHECK(julianday(valid_from) IS NOT NULL),
				CHECK(julianday(valid_until) IS NOT NULL),
				CHECK(julianday(valid_from) <= julianday(valid_until))
			) WITHOUT ROWID`,
			`CREATE TABLE IF NOT EXISTS profile_author_role_admissions (
				role_admission_ref TEXT PRIMARY KEY,
				role_ref TEXT NOT NULL,
				bounded_context_ref TEXT NOT NULL,
				governing_pattern_ref TEXT NOT NULL CHECK(governing_pattern_ref = 'A.2.1'),
				method_description_ref TEXT NOT NULL REFERENCES profile_onboarding_method_descriptions(method_description_ref),
				method_description_digest TEXT NOT NULL,
				method_contract_ref TEXT NOT NULL REFERENCES profile_onboarding_method_contracts(method_contract_ref),
				method_contract_digest TEXT NOT NULL,
				role_admission_policy_ref TEXT NOT NULL,
				role_admission_json TEXT NOT NULL CHECK(json_valid(role_admission_json)),
				role_admission_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL
			) WITHOUT ROWID`,
			`CREATE TABLE IF NOT EXISTS profile_author_assignment_support_carriers (
				assignment_justification_ref TEXT PRIMARY KEY,
				assignment_rule_ref TEXT NOT NULL,
				assignment_rule_statement TEXT NOT NULL,
				bounded_context_ref TEXT NOT NULL,
				system_admission_ref TEXT NOT NULL REFERENCES profile_onboarding_executor_system_admissions(system_admission_ref),
				system_admission_digest TEXT NOT NULL,
				role_admission_ref TEXT NOT NULL REFERENCES profile_author_role_admissions(role_admission_ref),
				role_admission_digest TEXT NOT NULL,
				assignment_from TEXT NOT NULL,
				assignment_until TEXT NOT NULL,
				method_contract_ref TEXT NOT NULL REFERENCES profile_onboarding_method_contracts(method_contract_ref),
				method_contract_digest TEXT NOT NULL,
				assignment_justification_json TEXT NOT NULL CHECK(json_valid(assignment_justification_json)),
				assignment_justification_digest TEXT NOT NULL UNIQUE,
				assignment_provenance_ref TEXT NOT NULL UNIQUE,
				provenance_justification_ref TEXT NOT NULL,
				provenance_justification_digest TEXT NOT NULL,
				session_ref TEXT NOT NULL,
				kernel_identity TEXT NOT NULL,
				kernel_version TEXT NOT NULL,
				runtime_identity TEXT NOT NULL,
				runtime_version TEXT NOT NULL,
				provenance_recorded_at TEXT NOT NULL,
				assignment_provenance_json TEXT NOT NULL CHECK(json_valid(assignment_provenance_json)),
				assignment_provenance_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL,
				CHECK(julianday(assignment_from) IS NOT NULL),
				CHECK(julianday(assignment_until) IS NOT NULL),
				CHECK(julianday(assignment_from) <= julianday(assignment_until)),
				CHECK(provenance_justification_ref = assignment_justification_ref),
				CHECK(provenance_justification_digest = assignment_justification_digest)
			) WITHOUT ROWID`,
			`CREATE TABLE IF NOT EXISTS profile_author_role_assignments (
				role_assignment_ref TEXT PRIMARY KEY,
				holder_system_ref TEXT NOT NULL,
				admitted_role_ref TEXT NOT NULL,
				bounded_context_ref TEXT NOT NULL,
				valid_from TEXT NOT NULL,
				valid_until TEXT NOT NULL,
				system_admission_ref TEXT NOT NULL REFERENCES profile_onboarding_executor_system_admissions(system_admission_ref),
				system_admission_digest TEXT NOT NULL,
				role_admission_ref TEXT NOT NULL REFERENCES profile_author_role_admissions(role_admission_ref),
				role_admission_digest TEXT NOT NULL,
				assignment_justification_ref TEXT NOT NULL REFERENCES profile_author_assignment_support_carriers(assignment_justification_ref),
				assignment_justification_digest TEXT NOT NULL,
				assignment_provenance_ref TEXT NOT NULL REFERENCES profile_author_assignment_support_carriers(assignment_provenance_ref),
				assignment_provenance_digest TEXT NOT NULL,
				role_assignment_json TEXT NOT NULL CHECK(json_valid(role_assignment_json)),
				role_assignment_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL,
				CHECK(julianday(valid_from) IS NOT NULL),
				CHECK(julianday(valid_until) IS NOT NULL),
				CHECK(julianday(valid_from) <= julianday(valid_until))
			) WITHOUT ROWID`,
			`CREATE TABLE IF NOT EXISTS observed_project_bases (
				observed_project_basis_ref TEXT PRIMARY KEY,
				project_root TEXT NOT NULL,
				observation_from TEXT NOT NULL,
				observation_until TEXT NOT NULL,
				detector_version TEXT NOT NULL,
				classifier_version TEXT NOT NULL,
				observed_project_basis_json TEXT NOT NULL CHECK(json_valid(observed_project_basis_json)),
				observed_project_basis_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL,
				CHECK(julianday(observation_from) IS NOT NULL),
				CHECK(julianday(observation_until) IS NOT NULL),
				CHECK(julianday(observation_from) <= julianday(observation_until))
			) WITHOUT ROWID`,
			`CREATE TABLE IF NOT EXISTS profile_onboarding_work_records (
				work_record_ref TEXT PRIMARY KEY,
				work_ref TEXT NOT NULL UNIQUE,
				project_root TEXT NOT NULL,
				enacts_method_ref TEXT NOT NULL,
				method_description_ref TEXT NOT NULL REFERENCES profile_onboarding_method_descriptions(method_description_ref),
				method_description_digest TEXT NOT NULL,
				method_contract_ref TEXT NOT NULL REFERENCES profile_onboarding_method_contracts(method_contract_ref),
				method_contract_digest TEXT NOT NULL,
				parameter_bindings_json TEXT NOT NULL,
				performed_by_role_assignment_ref TEXT NOT NULL,
				profile_author_role_assignment_ref TEXT NOT NULL REFERENCES profile_author_role_assignments(role_assignment_ref),
				profile_author_role_assignment_digest TEXT NOT NULL,
				executed_within_ref TEXT NOT NULL,
				work_from TEXT NOT NULL,
				work_until TEXT NOT NULL,
				bounded_context_ref TEXT NOT NULL,
				basis_observation_from TEXT NOT NULL,
				basis_observation_until TEXT NOT NULL,
				observed_project_basis_ref TEXT NOT NULL REFERENCES observed_project_bases(observed_project_basis_ref),
				observed_project_basis_digest TEXT NOT NULL,
				inputs_json TEXT NOT NULL,
				outputs_json TEXT NOT NULL,
				resources_json TEXT NOT NULL,
				affected_ref_kind TEXT NOT NULL,
				affected_refs_json TEXT NOT NULL,
				state_plane_ref TEXT NOT NULL,
				pre_state_ref TEXT NOT NULL DEFAULT '',
				post_state_ref TEXT NOT NULL DEFAULT '',
				delta_predicate_ref TEXT NOT NULL DEFAULT '',
				outcome_kind TEXT NOT NULL CHECK(outcome_kind IN ('CandidatePayloadProduced', 'ClassificationUnderdetermined')),
				profile_payload_digest TEXT NOT NULL DEFAULT '',
				observed_basis_digest TEXT NOT NULL DEFAULT '',
				missing_basis_digest TEXT NOT NULL DEFAULT '',
				work_record_json TEXT NOT NULL CHECK(json_valid(work_record_json)),
				work_record_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL,
				CHECK(json_valid(parameter_bindings_json)),
				CHECK(json_valid(inputs_json)),
				CHECK(json_valid(outputs_json)),
				CHECK(json_valid(resources_json)),
				CHECK(json_valid(affected_refs_json)),
				CHECK(julianday(work_from) IS NOT NULL),
				CHECK(julianday(work_until) IS NOT NULL),
				CHECK(julianday(work_from) <= julianday(work_until)),
				CHECK(julianday(basis_observation_from) IS NOT NULL),
				CHECK(julianday(basis_observation_until) IS NOT NULL),
				CHECK(julianday(basis_observation_from) <= julianday(basis_observation_until)),
				CHECK(
					(pre_state_ref != '' AND post_state_ref != '' AND delta_predicate_ref = '')
					OR (pre_state_ref != '' AND post_state_ref = '' AND delta_predicate_ref != '')
				),
				CHECK(
					(outcome_kind = 'CandidatePayloadProduced' AND profile_payload_digest != '' AND observed_basis_digest != '' AND missing_basis_digest = '')
					OR (outcome_kind = 'ClassificationUnderdetermined' AND profile_payload_digest = '' AND observed_basis_digest = '' AND missing_basis_digest != '')
				)
			) WITHOUT ROWID`,
			`CREATE TABLE IF NOT EXISTS profile_onboarding_effects (
				effect_ref TEXT PRIMARY KEY,
				work_record_ref TEXT NOT NULL REFERENCES profile_onboarding_work_records(work_record_ref),
				work_ref TEXT NOT NULL,
				work_record_digest TEXT NOT NULL,
				result_kind TEXT NOT NULL CHECK(result_kind IN ('CandidatePayloadProduced', 'ClassificationUnderdetermined')),
				output_ref TEXT NOT NULL,
				profile_payload_digest TEXT NOT NULL DEFAULT '',
				observed_project_basis_ref TEXT NOT NULL DEFAULT '',
				observed_project_basis_digest TEXT NOT NULL DEFAULT '',
				missing_basis_digest TEXT NOT NULL DEFAULT '',
				affected_entity_refs_json TEXT NOT NULL CHECK(json_valid(affected_entity_refs_json)),
				state_plane_ref TEXT NOT NULL,
				pre_state_ref TEXT NOT NULL,
				post_state_ref TEXT NOT NULL DEFAULT '',
				delta_predicate_ref TEXT NOT NULL DEFAULT '',
				evidence_provenance_path_refs_json TEXT NOT NULL CHECK(json_valid(evidence_provenance_path_refs_json)),
				effect_json TEXT NOT NULL CHECK(json_valid(effect_json)),
				effect_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL,
				CHECK(
					(pre_state_ref != '' AND post_state_ref != '' AND delta_predicate_ref = '')
					OR (pre_state_ref != '' AND post_state_ref = '' AND delta_predicate_ref != '')
				),
				CHECK(
					(result_kind = 'CandidatePayloadProduced' AND profile_payload_digest != '' AND observed_project_basis_ref != '' AND observed_project_basis_digest != '' AND missing_basis_digest = '')
					OR (result_kind = 'ClassificationUnderdetermined' AND profile_payload_digest = '' AND observed_project_basis_ref = '' AND observed_project_basis_digest = '' AND missing_basis_digest != '')
				)
			) WITHOUT ROWID`,
			`CREATE TABLE IF NOT EXISTS profile_onboarding_outcome_assessments (
				outcome_assessment_ref TEXT PRIMARY KEY,
				effect_ref TEXT NOT NULL REFERENCES profile_onboarding_effects(effect_ref),
				effect_digest TEXT NOT NULL,
				work_record_ref TEXT NOT NULL REFERENCES profile_onboarding_work_records(work_record_ref),
				work_ref TEXT NOT NULL,
				work_record_digest TEXT NOT NULL,
				acceptance_standard_ref TEXT NOT NULL,
				acceptance_standard_edition TEXT NOT NULL,
				comparator_ref TEXT NOT NULL,
				comparator_edition TEXT NOT NULL,
				verdict_kind TEXT NOT NULL CHECK(verdict_kind IN ('passed', 'failed', 'undetermined')),
				verdict_reason_ref TEXT NOT NULL DEFAULT '',
				missing_basis_digest TEXT NOT NULL DEFAULT '',
				evidence_provenance_path_refs_json TEXT NOT NULL CHECK(json_valid(evidence_provenance_path_refs_json)),
				outcome_assessment_json TEXT NOT NULL CHECK(json_valid(outcome_assessment_json)),
				outcome_assessment_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL,
				CHECK(
					(verdict_kind = 'passed' AND verdict_reason_ref = '' AND missing_basis_digest = '')
					OR (verdict_kind = 'failed' AND verdict_reason_ref != '' AND missing_basis_digest = '')
					OR (verdict_kind = 'undetermined' AND verdict_reason_ref = '' AND missing_basis_digest != '')
				)
			) WITHOUT ROWID`,
			`CREATE TABLE IF NOT EXISTS project_profile_admissions (
				admission_id TEXT PRIMARY KEY,
				action_kind TEXT NOT NULL CHECK(action_kind = 'profile.declare.from_onboarding_candidate'),
				project_root TEXT NOT NULL,
				project_binding_digest TEXT NOT NULL,
				profile_payload_json TEXT NOT NULL CHECK(json_valid(profile_payload_json)),
				candidate_provenance_json TEXT NOT NULL CHECK(json_valid(candidate_provenance_json)),
				candidate_provenance_digest TEXT NOT NULL,
				profile_author_role_assignment_ref TEXT NOT NULL REFERENCES profile_author_role_assignments(role_assignment_ref),
				profile_author_role_assignment_digest TEXT NOT NULL,
				profile_payload_digest TEXT NOT NULL,
				observed_project_basis_ref TEXT NOT NULL REFERENCES observed_project_bases(observed_project_basis_ref),
				observed_project_basis_digest TEXT NOT NULL,
				work_record_ref TEXT NOT NULL REFERENCES profile_onboarding_work_records(work_record_ref),
				work_record_digest TEXT NOT NULL,
				outcome_assessment_ref TEXT NOT NULL REFERENCES profile_onboarding_outcome_assessments(outcome_assessment_ref),
				outcome_assessment_digest TEXT NOT NULL,
				authority_basis_ref TEXT NOT NULL REFERENCES authority_presentations(presentation_id),
				authority_basis_digest TEXT NOT NULL,
				authority_resolution_ref TEXT NOT NULL UNIQUE REFERENCES authority_resolution_records(authority_resolution_id),
				authority_resolution_digest TEXT NOT NULL,
				receipt_json TEXT NOT NULL CHECK(json_valid(receipt_json)),
				receipt_digest TEXT NOT NULL UNIQUE,
				expected_ledger_revision INTEGER NOT NULL CHECK(
					expected_ledger_revision >= 0
					AND expected_ledger_revision < 9223372036854775807
				),
				ledger_revision INTEGER NOT NULL CHECK(ledger_revision = expected_ledger_revision + 1),
				single_use_key TEXT NOT NULL UNIQUE,
				admission_request_digest TEXT NOT NULL,
				admission_json TEXT NOT NULL CHECK(json_valid(admission_json)),
				admission_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL,
				UNIQUE(project_root, ledger_revision)
			) WITHOUT ROWID`,
			`CREATE TABLE IF NOT EXISTS project_profile_revisions (
				project_root TEXT NOT NULL,
				ledger_revision INTEGER NOT NULL CHECK(ledger_revision > 0),
				configured_profile_kind TEXT NOT NULL CHECK(configured_profile_kind = 'Declared'),
				profile_payload_json TEXT NOT NULL,
				profile_payload_digest TEXT NOT NULL,
				receipt_json TEXT NOT NULL,
				receipt_digest TEXT NOT NULL,
				admission_id TEXT NOT NULL UNIQUE REFERENCES project_profile_admissions(admission_id),
				admission_digest TEXT NOT NULL UNIQUE,
				recorded_at TEXT NOT NULL,
				PRIMARY KEY(project_root, ledger_revision)
			) WITHOUT ROWID`,
			`CREATE TABLE IF NOT EXISTS project_profile_projection_debt (
				event_id TEXT PRIMARY KEY,
				debt_id TEXT NOT NULL,
				admission_id TEXT NOT NULL REFERENCES project_profile_admissions(admission_id),
				admission_digest TEXT NOT NULL,
				project_root TEXT NOT NULL,
				ledger_revision INTEGER NOT NULL CHECK(ledger_revision > 0),
				profile_payload_digest TEXT NOT NULL,
				projection_path TEXT NOT NULL,
				event_kind TEXT NOT NULL CHECK(event_kind IN ('opened', 'resolved')),
				reason_code TEXT NOT NULL,
				detail TEXT NOT NULL,
				expected_projection_digest TEXT NOT NULL,
				observed_projection_digest TEXT NOT NULL DEFAULT '',
				supersedes_event_id TEXT,
				recorded_at TEXT NOT NULL
			) WITHOUT ROWID`,
			`CREATE VIEW IF NOT EXISTS current_project_profiles AS
			SELECT revision.*
			FROM project_profile_revisions revision
			WHERE NOT EXISTS (
				SELECT 1 FROM project_profile_revisions newer
				WHERE newer.project_root = revision.project_root
				AND newer.ledger_revision > revision.ledger_revision
			)`,
			"CREATE INDEX IF NOT EXISTS idx_authority_presentations_action_project ON authority_presentations(action_kind, project_root)",
			"CREATE INDEX IF NOT EXISTS idx_authority_resolution_presentation ON authority_resolution_records(presentation_id)",
			"CREATE INDEX IF NOT EXISTS idx_authority_uses_record ON authority_uses(authority_record_ref)",
			"CREATE INDEX IF NOT EXISTS idx_profile_onboarding_method_contract_description ON profile_onboarding_method_contracts(method_description_ref)",
			"CREATE INDEX IF NOT EXISTS idx_profile_onboarding_executor_system_admission_system ON profile_onboarding_executor_system_admissions(system_ref, bounded_context_ref)",
			"CREATE INDEX IF NOT EXISTS idx_profile_author_role_admission_role ON profile_author_role_admissions(role_ref, bounded_context_ref)",
			"CREATE INDEX IF NOT EXISTS idx_profile_author_assignment_support_admissions ON profile_author_assignment_support_carriers(system_admission_ref, role_admission_ref)",
			"CREATE INDEX IF NOT EXISTS idx_profile_author_role_assignment_holder ON profile_author_role_assignments(holder_system_ref, admitted_role_ref, bounded_context_ref)",
			"CREATE INDEX IF NOT EXISTS idx_observed_project_bases_project ON observed_project_bases(project_root, observation_from)",
			"CREATE INDEX IF NOT EXISTS idx_profile_onboarding_work_project ON profile_onboarding_work_records(project_root, work_from)",
			"CREATE INDEX IF NOT EXISTS idx_profile_onboarding_effect_work ON profile_onboarding_effects(work_record_ref)",
			"CREATE INDEX IF NOT EXISTS idx_profile_onboarding_outcome_assessment_effect ON profile_onboarding_outcome_assessments(effect_ref)",
			"CREATE INDEX IF NOT EXISTS idx_project_profile_admissions_project ON project_profile_admissions(project_root, ledger_revision)",
			"CREATE INDEX IF NOT EXISTS idx_project_profile_revisions_current ON project_profile_revisions(project_root, ledger_revision DESC)",
			"CREATE INDEX IF NOT EXISTS idx_project_profile_projection_debt_open ON project_profile_projection_debt(debt_id, event_kind, recorded_at)",
			`CREATE TRIGGER IF NOT EXISTS authority_presentations_no_replace
			BEFORE INSERT ON authority_presentations
			WHEN EXISTS (
				SELECT 1 FROM authority_presentations existing
				WHERE existing.presentation_id = NEW.presentation_id
				OR existing.permission_ref = NEW.permission_ref
				OR existing.single_use_key = NEW.single_use_key
				OR existing.presentation_digest = NEW.presentation_digest
			) BEGIN
				SELECT RAISE(ABORT, 'authority_presentations is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS authority_presentations_exact_assignment_method
			BEFORE INSERT ON authority_presentations
			WHEN NOT EXISTS (
				SELECT 1
				FROM profile_author_role_assignments assignment
				JOIN profile_author_assignment_support_carriers assignment_support
					ON assignment_support.assignment_justification_ref = assignment.assignment_justification_ref
				JOIN profile_onboarding_method_descriptions description
					ON description.method_description_ref = NEW.method_description_ref
				JOIN profile_onboarding_method_contracts contract
					ON contract.method_contract_ref = NEW.method_contract_ref
				WHERE assignment.role_assignment_ref = NEW.profile_author_role_assignment_ref
				AND assignment.role_assignment_digest = NEW.profile_author_role_assignment_digest
				AND NEW.permission_subject_role_assignment_ref = assignment.role_assignment_ref
				AND description.method_description_digest = NEW.method_description_digest
				AND contract.method_contract_digest = NEW.method_contract_digest
				AND contract.method_description_ref = description.method_description_ref
				AND contract.method_description_digest = description.method_description_digest
				AND assignment_support.method_contract_ref = contract.method_contract_ref
				AND assignment_support.method_contract_digest = contract.method_contract_digest
				AND NEW.permission_method_description_ref = description.method_description_ref
				AND NEW.permission_action_kind = NEW.action_kind
				AND NEW.permission_project_root = NEW.project_root
				AND NEW.permission_valid_from = NEW.valid_from
				AND NEW.permission_valid_until = NEW.valid_until
				AND NEW.permission_single_use_key = NEW.single_use_key
				AND assignment_support.session_ref = NEW.session_ref
				AND julianday(assignment.valid_from) <= julianday(NEW.valid_from)
				AND julianday(assignment.valid_until) >= julianday(NEW.valid_until)
				AND julianday(assignment.valid_from) <= julianday(NEW.allowed_work_from)
				AND julianday(assignment.valid_until) >= julianday(NEW.allowed_work_until)
			) BEGIN
				SELECT RAISE(ABORT, 'authority presentation does not consume the exact pre-existing assignment and method contract');
			END`,
			`CREATE TRIGGER IF NOT EXISTS authority_presentations_no_update
			BEFORE UPDATE ON authority_presentations BEGIN
				SELECT RAISE(ABORT, 'authority_presentations is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS authority_presentations_no_delete
			BEFORE DELETE ON authority_presentations BEGIN
				SELECT RAISE(ABORT, 'authority_presentations is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS authority_resolution_records_no_replace
			BEFORE INSERT ON authority_resolution_records
			WHEN EXISTS (
				SELECT 1 FROM authority_resolution_records existing
				WHERE existing.authority_resolution_id = NEW.authority_resolution_id
				OR existing.presentation_id = NEW.presentation_id
				OR existing.presentation_digest = NEW.presentation_digest
				OR existing.authority_resolution_digest = NEW.authority_resolution_digest
			) BEGIN
				SELECT RAISE(ABORT, 'authority_resolution_records is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS authority_resolution_records_exact_presentation
			BEFORE INSERT ON authority_resolution_records
			WHEN NOT EXISTS (
				SELECT 1
				FROM authority_presentations presentation
				WHERE presentation.presentation_id = NEW.presentation_id
				AND presentation.presentation_digest = NEW.presentation_digest
				AND presentation.profile_author_role_assignment_ref = NEW.profile_author_role_assignment_ref
				AND presentation.profile_author_role_assignment_digest = NEW.profile_author_role_assignment_digest
				AND presentation.method_description_ref = NEW.method_description_ref
				AND presentation.method_description_digest = NEW.method_description_digest
				AND presentation.method_contract_ref = NEW.method_contract_ref
				AND presentation.method_contract_digest = NEW.method_contract_digest
			) BEGIN
				SELECT RAISE(ABORT, 'authority resolution does not bind the exact presentation assignment and method contract');
			END`,
			`CREATE TRIGGER IF NOT EXISTS authority_resolution_records_no_update
			BEFORE UPDATE ON authority_resolution_records BEGIN
				SELECT RAISE(ABORT, 'authority_resolution_records is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS authority_resolution_records_no_delete
			BEFORE DELETE ON authority_resolution_records BEGIN
				SELECT RAISE(ABORT, 'authority_resolution_records is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS authority_uses_no_replace
			BEFORE INSERT ON authority_uses
			WHEN EXISTS (
				SELECT 1 FROM authority_uses existing
				WHERE existing.use_id = NEW.use_id
				OR existing.authority_resolution_ref = NEW.authority_resolution_ref
				OR existing.single_use_key = NEW.single_use_key
				OR existing.committed_result_ref = NEW.committed_result_ref
			) BEGIN
				SELECT RAISE(ABORT, 'authority_uses is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS authority_uses_exact_tuple
			BEFORE INSERT ON authority_uses
			WHEN NOT EXISTS (
				SELECT 1
				FROM authority_resolution_records resolution
				JOIN authority_presentations presentation
					ON presentation.presentation_id = resolution.presentation_id
				JOIN project_profile_admissions admission
					ON admission.admission_id = NEW.committed_result_ref
				WHERE resolution.authority_resolution_id = NEW.authority_resolution_ref
				AND resolution.authority_resolution_digest = NEW.authority_resolution_digest
				AND resolution.verifier_identity = NEW.verifier_identity
				AND resolution.verifier_version = NEW.verifier_version
				AND presentation.single_use_key = NEW.single_use_key
				AND presentation.action_kind = NEW.action_kind
				AND presentation.project_root = NEW.project_root
				AND presentation.project_binding_digest = NEW.project_binding_digest
				AND presentation.envelope_digest = NEW.envelope_digest
				AND presentation.presentation_id = NEW.authority_record_ref
				AND presentation.presentation_digest = NEW.authority_record_digest
				AND admission.admission_digest = NEW.committed_result_digest
				AND admission.admission_request_digest = NEW.admission_request_digest
				AND admission.authority_resolution_ref = NEW.authority_resolution_ref
				AND admission.authority_resolution_digest = NEW.authority_resolution_digest
				AND admission.single_use_key = NEW.single_use_key
				AND admission.action_kind = NEW.action_kind
			) BEGIN
				SELECT RAISE(ABORT, 'authority_uses tuple does not match canonical resolution and admission');
			END`,
			`CREATE TRIGGER IF NOT EXISTS authority_uses_no_update
			BEFORE UPDATE ON authority_uses BEGIN
				SELECT RAISE(ABORT, 'authority_uses is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS authority_uses_no_delete
			BEFORE DELETE ON authority_uses BEGIN
				SELECT RAISE(ABORT, 'authority_uses is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_method_descriptions_no_replace
			BEFORE INSERT ON profile_onboarding_method_descriptions
			WHEN EXISTS (
				SELECT 1 FROM profile_onboarding_method_descriptions existing
				WHERE existing.method_description_ref = NEW.method_description_ref
				OR existing.method_description_digest = NEW.method_description_digest
			) BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_method_descriptions is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_method_descriptions_no_update
			BEFORE UPDATE ON profile_onboarding_method_descriptions BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_method_descriptions is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_method_descriptions_no_delete
			BEFORE DELETE ON profile_onboarding_method_descriptions BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_method_descriptions is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_method_contracts_no_replace
			BEFORE INSERT ON profile_onboarding_method_contracts
			WHEN EXISTS (
				SELECT 1 FROM profile_onboarding_method_contracts existing
				WHERE existing.method_contract_ref = NEW.method_contract_ref
				OR existing.method_contract_digest = NEW.method_contract_digest
			) BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_method_contracts is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_method_contracts_exact_description
			BEFORE INSERT ON profile_onboarding_method_contracts
			WHEN NOT EXISTS (
				SELECT 1 FROM profile_onboarding_method_descriptions description
				WHERE description.method_description_ref = NEW.method_description_ref
				AND description.method_description_digest = NEW.method_description_digest
				AND description.bounded_context_ref = NEW.bounded_context_ref
				AND description.effect_witness_rule_ref = NEW.effect_state_witness_rule_ref
			) BEGIN
				SELECT RAISE(ABORT, 'MethodContract does not bind the exact MethodDescription');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_method_contracts_no_update
			BEFORE UPDATE ON profile_onboarding_method_contracts BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_method_contracts is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_method_contracts_no_delete
			BEFORE DELETE ON profile_onboarding_method_contracts BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_method_contracts is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_executor_system_admissions_no_replace
			BEFORE INSERT ON profile_onboarding_executor_system_admissions
			WHEN EXISTS (
				SELECT 1 FROM profile_onboarding_executor_system_admissions existing
				WHERE existing.system_admission_ref = NEW.system_admission_ref
				OR existing.system_admission_digest = NEW.system_admission_digest
			) BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_executor_system_admissions is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_executor_system_admissions_exact_method
			BEFORE INSERT ON profile_onboarding_executor_system_admissions
			WHEN NOT EXISTS (
				SELECT 1
				FROM profile_onboarding_method_descriptions description
				JOIN profile_onboarding_method_contracts contract
					ON contract.method_contract_ref = NEW.method_contract_ref
				WHERE description.method_description_ref = NEW.method_description_ref
				AND description.method_description_digest = NEW.method_description_digest
				AND contract.method_contract_digest = NEW.method_contract_digest
				AND contract.method_description_ref = description.method_description_ref
				AND contract.method_description_digest = description.method_description_digest
				AND description.bounded_context_ref = NEW.bounded_context_ref
				AND contract.bounded_context_ref = NEW.bounded_context_ref
				AND description.required_system_kind = NEW.admitted_system_kind
				AND contract.system_admission_policy_ref = NEW.system_admission_policy_ref
			) BEGIN
				SELECT RAISE(ABORT, 'executor-system admission does not bind the exact method contract');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_executor_system_admissions_no_update
			BEFORE UPDATE ON profile_onboarding_executor_system_admissions BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_executor_system_admissions is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_executor_system_admissions_no_delete
			BEFORE DELETE ON profile_onboarding_executor_system_admissions BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_executor_system_admissions is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_author_role_admissions_no_replace
			BEFORE INSERT ON profile_author_role_admissions
			WHEN EXISTS (
				SELECT 1 FROM profile_author_role_admissions existing
				WHERE existing.role_admission_ref = NEW.role_admission_ref
				OR existing.role_admission_digest = NEW.role_admission_digest
			) BEGIN
				SELECT RAISE(ABORT, 'profile_author_role_admissions is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_author_role_admissions_exact_method
			BEFORE INSERT ON profile_author_role_admissions
			WHEN NOT EXISTS (
				SELECT 1
				FROM profile_onboarding_method_descriptions description
				JOIN profile_onboarding_method_contracts contract
					ON contract.method_contract_ref = NEW.method_contract_ref
				WHERE description.method_description_ref = NEW.method_description_ref
				AND description.method_description_digest = NEW.method_description_digest
				AND contract.method_contract_digest = NEW.method_contract_digest
				AND contract.method_description_ref = description.method_description_ref
				AND contract.method_description_digest = description.method_description_digest
				AND description.bounded_context_ref = NEW.bounded_context_ref
				AND contract.bounded_context_ref = NEW.bounded_context_ref
				AND description.required_role_ref = NEW.role_ref
				AND contract.role_admission_policy_ref = NEW.role_admission_policy_ref
			) BEGIN
				SELECT RAISE(ABORT, 'ProfileAuthor role admission does not bind the exact method contract');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_author_role_admissions_no_update
			BEFORE UPDATE ON profile_author_role_admissions BEGIN
				SELECT RAISE(ABORT, 'profile_author_role_admissions is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_author_role_admissions_no_delete
			BEFORE DELETE ON profile_author_role_admissions BEGIN
				SELECT RAISE(ABORT, 'profile_author_role_admissions is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_author_assignment_support_carriers_no_replace
			BEFORE INSERT ON profile_author_assignment_support_carriers
			WHEN EXISTS (
				SELECT 1 FROM profile_author_assignment_support_carriers existing
				WHERE existing.assignment_justification_ref = NEW.assignment_justification_ref
				OR existing.assignment_justification_digest = NEW.assignment_justification_digest
				OR existing.assignment_provenance_ref = NEW.assignment_provenance_ref
				OR existing.assignment_provenance_digest = NEW.assignment_provenance_digest
			) BEGIN
				SELECT RAISE(ABORT, 'profile_author_assignment_support_carriers is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_author_assignment_support_carriers_exact_admissions
			BEFORE INSERT ON profile_author_assignment_support_carriers
			WHEN NOT EXISTS (
				SELECT 1
				FROM profile_onboarding_executor_system_admissions system_admission
				JOIN profile_author_role_admissions role_admission
					ON role_admission.role_admission_ref = NEW.role_admission_ref
				JOIN profile_onboarding_method_contracts contract
					ON contract.method_contract_ref = NEW.method_contract_ref
				WHERE system_admission.system_admission_ref = NEW.system_admission_ref
				AND system_admission.system_admission_digest = NEW.system_admission_digest
				AND role_admission.role_admission_digest = NEW.role_admission_digest
				AND contract.method_contract_digest = NEW.method_contract_digest
				AND system_admission.method_contract_ref = contract.method_contract_ref
				AND system_admission.method_contract_digest = contract.method_contract_digest
				AND role_admission.method_contract_ref = contract.method_contract_ref
				AND role_admission.method_contract_digest = contract.method_contract_digest
				AND system_admission.method_description_ref = role_admission.method_description_ref
				AND system_admission.method_description_digest = role_admission.method_description_digest
				AND system_admission.bounded_context_ref = NEW.bounded_context_ref
				AND role_admission.bounded_context_ref = NEW.bounded_context_ref
				AND contract.bounded_context_ref = NEW.bounded_context_ref
				AND julianday(system_admission.valid_from) <= julianday(NEW.assignment_from)
				AND julianday(system_admission.valid_until) >= julianday(NEW.assignment_until)
				AND system_admission.session_ref = NEW.session_ref
				AND (
					system_admission.identity_basis_kind != 'kernel_owned'
					OR (
						system_admission.identity_basis_kernel_identity = NEW.kernel_identity
						AND system_admission.identity_basis_kernel_version = NEW.kernel_version
					)
				)
			) BEGIN
				SELECT RAISE(ABORT, 'assignment support does not bind exact system and role admissions');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_author_assignment_support_carriers_no_update
			BEFORE UPDATE ON profile_author_assignment_support_carriers BEGIN
				SELECT RAISE(ABORT, 'profile_author_assignment_support_carriers is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_author_assignment_support_carriers_no_delete
			BEFORE DELETE ON profile_author_assignment_support_carriers BEGIN
				SELECT RAISE(ABORT, 'profile_author_assignment_support_carriers is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_author_role_assignments_no_replace
			BEFORE INSERT ON profile_author_role_assignments
			WHEN EXISTS (
				SELECT 1 FROM profile_author_role_assignments existing
				WHERE existing.role_assignment_ref = NEW.role_assignment_ref
				OR existing.role_assignment_digest = NEW.role_assignment_digest
			) BEGIN
				SELECT RAISE(ABORT, 'profile_author_role_assignments is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_author_role_assignments_exact_support
			BEFORE INSERT ON profile_author_role_assignments
			WHEN NOT EXISTS (
				SELECT 1
				FROM profile_onboarding_executor_system_admissions system_admission
				JOIN profile_author_role_admissions role_admission
					ON role_admission.role_admission_ref = NEW.role_admission_ref
				JOIN profile_author_assignment_support_carriers support
					ON support.assignment_justification_ref = NEW.assignment_justification_ref
				WHERE system_admission.system_admission_ref = NEW.system_admission_ref
				AND system_admission.system_admission_digest = NEW.system_admission_digest
				AND role_admission.role_admission_digest = NEW.role_admission_digest
				AND support.assignment_justification_digest = NEW.assignment_justification_digest
				AND support.assignment_provenance_ref = NEW.assignment_provenance_ref
				AND support.assignment_provenance_digest = NEW.assignment_provenance_digest
				AND support.system_admission_ref = system_admission.system_admission_ref
				AND support.system_admission_digest = system_admission.system_admission_digest
				AND support.role_admission_ref = role_admission.role_admission_ref
				AND support.role_admission_digest = role_admission.role_admission_digest
				AND NEW.holder_system_ref = system_admission.system_ref
				AND NEW.admitted_role_ref = role_admission.role_ref
				AND NEW.bounded_context_ref = system_admission.bounded_context_ref
				AND NEW.bounded_context_ref = role_admission.bounded_context_ref
				AND NEW.valid_from = support.assignment_from
				AND NEW.valid_until = support.assignment_until
				AND julianday(system_admission.valid_from) <= julianday(NEW.valid_from)
				AND julianday(system_admission.valid_until) >= julianday(NEW.valid_until)
				AND system_admission.session_ref = support.session_ref
			) BEGIN
				SELECT RAISE(ABORT, 'RoleAssignment does not bind exact admission and provenance support');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_author_role_assignments_no_update
			BEFORE UPDATE ON profile_author_role_assignments BEGIN
				SELECT RAISE(ABORT, 'profile_author_role_assignments is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_author_role_assignments_no_delete
			BEFORE DELETE ON profile_author_role_assignments BEGIN
				SELECT RAISE(ABORT, 'profile_author_role_assignments is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS observed_project_bases_no_replace
			BEFORE INSERT ON observed_project_bases
			WHEN EXISTS (
				SELECT 1 FROM observed_project_bases existing
				WHERE existing.observed_project_basis_ref = NEW.observed_project_basis_ref
				OR existing.observed_project_basis_digest = NEW.observed_project_basis_digest
			) BEGIN
				SELECT RAISE(ABORT, 'observed_project_bases is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS observed_project_bases_no_update
			BEFORE UPDATE ON observed_project_bases BEGIN
				SELECT RAISE(ABORT, 'observed_project_bases is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS observed_project_bases_no_delete
			BEFORE DELETE ON observed_project_bases BEGIN
				SELECT RAISE(ABORT, 'observed_project_bases is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_work_records_no_replace
			BEFORE INSERT ON profile_onboarding_work_records
			WHEN EXISTS (
				SELECT 1 FROM profile_onboarding_work_records existing
				WHERE existing.work_record_ref = NEW.work_record_ref
					OR existing.work_ref = NEW.work_ref
					OR existing.work_record_digest = NEW.work_record_digest
			) BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_work_records is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_work_records_exact_support
			BEFORE INSERT ON profile_onboarding_work_records
			WHEN NOT EXISTS (
				SELECT 1
				FROM profile_onboarding_method_descriptions description
				JOIN profile_onboarding_method_contracts contract
					ON contract.method_contract_ref = NEW.method_contract_ref
				JOIN profile_author_role_assignments assignment
					ON assignment.role_assignment_ref = NEW.profile_author_role_assignment_ref
				JOIN profile_author_assignment_support_carriers assignment_support
					ON assignment_support.assignment_justification_ref = assignment.assignment_justification_ref
				JOIN profile_onboarding_executor_system_admissions system_admission
					ON system_admission.system_admission_ref = assignment.system_admission_ref
				JOIN observed_project_bases basis
					ON basis.observed_project_basis_ref = NEW.observed_project_basis_ref
				WHERE description.method_description_ref = NEW.method_description_ref
				AND description.method_description_digest = NEW.method_description_digest
				AND contract.method_contract_digest = NEW.method_contract_digest
				AND contract.method_description_ref = description.method_description_ref
				AND contract.method_description_digest = description.method_description_digest
				AND assignment.role_assignment_digest = NEW.profile_author_role_assignment_digest
				AND assignment_support.assignment_justification_digest = assignment.assignment_justification_digest
				AND assignment_support.assignment_provenance_ref = assignment.assignment_provenance_ref
				AND assignment_support.assignment_provenance_digest = assignment.assignment_provenance_digest
				AND system_admission.system_admission_digest = assignment.system_admission_digest
				AND system_admission.session_ref = assignment_support.session_ref
				AND basis.observed_project_basis_digest = NEW.observed_project_basis_digest
				AND NEW.enacts_method_ref = description.described_method_ref
				AND NEW.performed_by_role_assignment_ref = assignment.role_assignment_ref
				AND NEW.profile_author_role_assignment_ref = NEW.performed_by_role_assignment_ref
				AND NEW.executed_within_ref = assignment.holder_system_ref
				AND NEW.bounded_context_ref = description.bounded_context_ref
				AND NEW.bounded_context_ref = contract.bounded_context_ref
				AND NEW.bounded_context_ref = assignment.bounded_context_ref
				AND NEW.state_plane_ref = description.state_plane_ref
				AND NEW.affected_ref_kind = description.affected_ref_kind
				AND julianday(assignment.valid_from) <= julianday(NEW.work_from)
				AND julianday(assignment.valid_until) >= julianday(NEW.work_until)
				AND basis.project_root = NEW.project_root
				AND basis.observation_from = NEW.basis_observation_from
				AND basis.observation_until = NEW.basis_observation_until
				AND EXISTS (
					SELECT 1 FROM json_each(NEW.parameter_bindings_json) binding
					WHERE json_extract(binding.value, '$.name') = 'project_root'
					AND json_extract(binding.value, '$.value') = NEW.project_root
				)
				AND EXISTS (
					SELECT 1 FROM json_each(NEW.parameter_bindings_json) binding
					WHERE json_extract(binding.value, '$.name') = 'classifier_version'
					AND json_extract(binding.value, '$.value') = basis.classifier_version
				)
				AND EXISTS (
					SELECT 1 FROM json_each(NEW.inputs_json) input_ref
					WHERE input_ref.value = basis.observed_project_basis_ref
				)
				AND (
					(NEW.outcome_kind = 'CandidatePayloadProduced'
						AND NEW.observed_basis_digest = basis.observed_project_basis_digest)
					OR NEW.outcome_kind = 'ClassificationUnderdetermined'
				)
			) BEGIN
				SELECT RAISE(ABORT, 'Work does not bind the exact method, assignment, and observed basis');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_work_records_no_update
			BEFORE UPDATE ON profile_onboarding_work_records BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_work_records is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_work_records_no_delete
			BEFORE DELETE ON profile_onboarding_work_records BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_work_records is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_effects_no_replace
			BEFORE INSERT ON profile_onboarding_effects
			WHEN EXISTS (
				SELECT 1 FROM profile_onboarding_effects existing
				WHERE existing.effect_ref = NEW.effect_ref
				OR existing.effect_digest = NEW.effect_digest
			) BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_effects is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_effects_exact_work
			BEFORE INSERT ON profile_onboarding_effects
			WHEN NOT EXISTS (
				SELECT 1
				FROM profile_onboarding_work_records work_record
				LEFT JOIN observed_project_bases basis
					ON basis.observed_project_basis_ref = NEW.observed_project_basis_ref
				WHERE work_record.work_record_ref = NEW.work_record_ref
				AND work_record.work_ref = NEW.work_ref
				AND work_record.work_record_digest = NEW.work_record_digest
				AND work_record.outcome_kind = NEW.result_kind
				AND work_record.state_plane_ref = NEW.state_plane_ref
				AND work_record.pre_state_ref = NEW.pre_state_ref
				AND work_record.post_state_ref = NEW.post_state_ref
				AND work_record.delta_predicate_ref = NEW.delta_predicate_ref
				AND work_record.affected_refs_json = NEW.affected_entity_refs_json
				AND EXISTS (
					SELECT 1 FROM json_each(work_record.outputs_json) output
					WHERE output.value = NEW.output_ref
				)
				AND (
					(NEW.result_kind = 'CandidatePayloadProduced'
						AND work_record.profile_payload_digest = NEW.profile_payload_digest
						AND work_record.observed_project_basis_ref = NEW.observed_project_basis_ref
						AND work_record.observed_project_basis_digest = NEW.observed_project_basis_digest
						AND work_record.observed_basis_digest = NEW.observed_project_basis_digest
						AND basis.observed_project_basis_digest = NEW.observed_project_basis_digest)
					OR (NEW.result_kind = 'ClassificationUnderdetermined'
						AND work_record.missing_basis_digest = NEW.missing_basis_digest)
				)
			) BEGIN
				SELECT RAISE(ABORT, 'ProfileOnboardingEffect does not bind the exact Work result and state witness');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_effects_no_update
			BEFORE UPDATE ON profile_onboarding_effects BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_effects is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_effects_no_delete
			BEFORE DELETE ON profile_onboarding_effects BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_effects is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_outcome_assessments_no_replace
			BEFORE INSERT ON profile_onboarding_outcome_assessments
			WHEN EXISTS (
				SELECT 1 FROM profile_onboarding_outcome_assessments existing
				WHERE existing.outcome_assessment_ref = NEW.outcome_assessment_ref
				OR existing.outcome_assessment_digest = NEW.outcome_assessment_digest
			) BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_outcome_assessments is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_outcome_assessments_exact_effect
			BEFORE INSERT ON profile_onboarding_outcome_assessments
			WHEN NOT EXISTS (
				SELECT 1
				FROM profile_onboarding_effects effect
				JOIN profile_onboarding_work_records work_record
					ON work_record.work_record_ref = NEW.work_record_ref
				JOIN profile_onboarding_method_contracts contract
					ON contract.method_contract_ref = work_record.method_contract_ref
				WHERE effect.effect_ref = NEW.effect_ref
				AND effect.effect_digest = NEW.effect_digest
				AND effect.work_record_ref = work_record.work_record_ref
				AND effect.work_ref = NEW.work_ref
				AND effect.work_record_digest = NEW.work_record_digest
				AND work_record.work_ref = NEW.work_ref
				AND work_record.work_record_digest = NEW.work_record_digest
				AND contract.acceptance_standard_ref = NEW.acceptance_standard_ref
				AND contract.acceptance_standard_edition = NEW.acceptance_standard_edition
				AND (
					(effect.result_kind = 'CandidatePayloadProduced' AND NEW.verdict_kind IN ('passed', 'failed'))
					OR (effect.result_kind = 'ClassificationUnderdetermined'
						AND NEW.verdict_kind = 'undetermined'
						AND effect.missing_basis_digest = NEW.missing_basis_digest)
				)
			) BEGIN
				SELECT RAISE(ABORT, 'outcome assessment does not bind the exact effect and method contract');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_outcome_assessments_no_update
			BEFORE UPDATE ON profile_onboarding_outcome_assessments BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_outcome_assessments is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_outcome_assessments_no_delete
			BEFORE DELETE ON profile_onboarding_outcome_assessments BEGIN
				SELECT RAISE(ABORT, 'profile_onboarding_outcome_assessments is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS project_profile_admissions_no_replace
			BEFORE INSERT ON project_profile_admissions
			WHEN EXISTS (
				SELECT 1 FROM project_profile_admissions existing
				WHERE existing.admission_id = NEW.admission_id
				OR existing.authority_resolution_ref = NEW.authority_resolution_ref
				OR existing.receipt_digest = NEW.receipt_digest
				OR existing.single_use_key = NEW.single_use_key
				OR existing.admission_digest = NEW.admission_digest
				OR (existing.project_root = NEW.project_root AND existing.ledger_revision = NEW.ledger_revision)
			) BEGIN
				SELECT RAISE(ABORT, 'project_profile_admissions is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS project_profile_admissions_revision_cas
			BEFORE INSERT ON project_profile_admissions
			WHEN COALESCE((
				SELECT MAX(existing.ledger_revision)
				FROM project_profile_revisions existing
				WHERE existing.project_root = NEW.project_root
			), 0) != NEW.expected_ledger_revision BEGIN
				SELECT RAISE(ABORT, 'project profile ledger revision conflict');
			END`,
			`CREATE TRIGGER IF NOT EXISTS project_profile_admissions_exact_authority
			BEFORE INSERT ON project_profile_admissions
			WHEN NOT EXISTS (
				SELECT 1
				FROM authority_resolution_records resolution
				JOIN authority_presentations presentation
					ON presentation.presentation_id = resolution.presentation_id
				JOIN profile_onboarding_work_records work_record
					ON work_record.work_record_ref = NEW.work_record_ref
				JOIN profile_author_role_assignments assignment
					ON assignment.role_assignment_ref = NEW.profile_author_role_assignment_ref
				JOIN profile_author_assignment_support_carriers assignment_support
					ON assignment_support.assignment_justification_ref = assignment.assignment_justification_ref
				JOIN profile_onboarding_executor_system_admissions system_admission
					ON system_admission.system_admission_ref = assignment.system_admission_ref
				JOIN observed_project_bases basis
					ON basis.observed_project_basis_ref = NEW.observed_project_basis_ref
				JOIN profile_onboarding_outcome_assessments assessment
					ON assessment.outcome_assessment_ref = NEW.outcome_assessment_ref
				JOIN profile_onboarding_effects effect
					ON effect.effect_ref = assessment.effect_ref
				WHERE presentation.presentation_id = NEW.authority_basis_ref
				AND presentation.presentation_digest = NEW.authority_basis_digest
				AND presentation.action_kind = NEW.action_kind
				AND NEW.action_kind = 'profile.declare.from_onboarding_candidate'
				AND presentation.project_root = NEW.project_root
				AND presentation.permission_project_root = NEW.project_root
				AND presentation.project_binding_digest = NEW.project_binding_digest
				AND presentation.single_use_key = NEW.single_use_key
				AND presentation.permission_single_use_key = NEW.single_use_key
				AND presentation.permission_action_kind = NEW.action_kind
				AND presentation.permission_subject_role_assignment_ref = NEW.profile_author_role_assignment_ref
				AND presentation.profile_author_role_assignment_ref = NEW.profile_author_role_assignment_ref
				AND presentation.profile_author_role_assignment_digest = NEW.profile_author_role_assignment_digest
				AND resolution.authority_resolution_id = NEW.authority_resolution_ref
				AND resolution.authority_resolution_digest = NEW.authority_resolution_digest
				AND resolution.presentation_digest = presentation.presentation_digest
				AND resolution.profile_author_role_assignment_ref = NEW.profile_author_role_assignment_ref
				AND resolution.profile_author_role_assignment_digest = NEW.profile_author_role_assignment_digest
				AND resolution.method_description_ref = work_record.method_description_ref
				AND resolution.method_description_digest = work_record.method_description_digest
				AND resolution.method_contract_ref = work_record.method_contract_ref
				AND resolution.method_contract_digest = work_record.method_contract_digest
				AND julianday(presentation.valid_from) <= julianday(NEW.recorded_at)
				AND julianday(presentation.valid_until) >= julianday(NEW.recorded_at)
				AND julianday(resolution.resolved_at) <= julianday(NEW.recorded_at)
				AND julianday(resolution.valid_until) >= julianday(NEW.recorded_at)
				AND work_record.work_record_digest = NEW.work_record_digest
				AND work_record.project_root = NEW.project_root
				AND work_record.method_description_ref = presentation.method_description_ref
				AND work_record.method_description_digest = presentation.method_description_digest
				AND work_record.method_description_ref = presentation.permission_method_description_ref
				AND work_record.method_contract_ref = presentation.method_contract_ref
				AND work_record.method_contract_digest = presentation.method_contract_digest
				AND work_record.performed_by_role_assignment_ref = NEW.profile_author_role_assignment_ref
				AND work_record.profile_author_role_assignment_ref = NEW.profile_author_role_assignment_ref
				AND work_record.profile_author_role_assignment_digest = NEW.profile_author_role_assignment_digest
				AND assignment.role_assignment_digest = NEW.profile_author_role_assignment_digest
				AND assignment_support.assignment_justification_digest = assignment.assignment_justification_digest
				AND assignment_support.assignment_provenance_ref = assignment.assignment_provenance_ref
				AND assignment_support.assignment_provenance_digest = assignment.assignment_provenance_digest
				AND system_admission.system_admission_digest = assignment.system_admission_digest
				AND system_admission.session_ref = assignment_support.session_ref
				AND presentation.session_ref = assignment_support.session_ref
				AND assignment.holder_system_ref = work_record.executed_within_ref
				AND assignment.bounded_context_ref = work_record.bounded_context_ref
				AND julianday(assignment.valid_from) <= julianday(work_record.work_from)
				AND julianday(assignment.valid_until) >= julianday(work_record.work_until)
				AND julianday(presentation.allowed_work_from) <= julianday(work_record.work_from)
				AND julianday(presentation.allowed_work_until) >= julianday(work_record.work_until)
				AND work_record.observed_project_basis_ref = NEW.observed_project_basis_ref
				AND work_record.observed_project_basis_digest = NEW.observed_project_basis_digest
				AND basis.observed_project_basis_digest = NEW.observed_project_basis_digest
				AND basis.project_root = NEW.project_root
				AND basis.observation_from = work_record.basis_observation_from
				AND basis.observation_until = work_record.basis_observation_until
				AND julianday(presentation.basis_observation_from) <= julianday(basis.observation_from)
				AND julianday(presentation.basis_observation_until) >= julianday(basis.observation_until)
				AND work_record.outcome_kind = 'CandidatePayloadProduced'
				AND work_record.profile_payload_digest = NEW.profile_payload_digest
				AND work_record.observed_basis_digest = NEW.observed_project_basis_digest
				AND assessment.outcome_assessment_digest = NEW.outcome_assessment_digest
				AND assessment.work_record_ref = work_record.work_record_ref
				AND assessment.work_record_digest = work_record.work_record_digest
				AND assessment.verdict_kind = 'passed'
				AND effect.effect_digest = assessment.effect_digest
				AND effect.work_record_ref = work_record.work_record_ref
				AND effect.work_record_digest = work_record.work_record_digest
				AND effect.result_kind = 'CandidatePayloadProduced'
				AND effect.profile_payload_digest = NEW.profile_payload_digest
				AND effect.observed_project_basis_ref = NEW.observed_project_basis_ref
				AND effect.observed_project_basis_digest = NEW.observed_project_basis_digest
				AND json_extract(NEW.candidate_provenance_json, '$.authority_basis_ref') = NEW.authority_basis_ref
				AND json_extract(NEW.candidate_provenance_json, '$.work_record_ref') = NEW.work_record_ref
				AND json_extract(NEW.candidate_provenance_json, '$.work_record_digest') = NEW.work_record_digest
				AND json_extract(NEW.candidate_provenance_json, '$.profile_author_role_assignment_ref') = NEW.profile_author_role_assignment_ref
				AND json_extract(NEW.candidate_provenance_json, '$.profile_author_role_assignment_digest') = NEW.profile_author_role_assignment_digest
				AND json_extract(NEW.candidate_provenance_json, '$.observed_project_basis_ref') = NEW.observed_project_basis_ref
				AND json_extract(NEW.candidate_provenance_json, '$.observed_project_basis_digest') = NEW.observed_project_basis_digest
				AND json_extract(NEW.candidate_provenance_json, '$.outcome_assessment_ref') = NEW.outcome_assessment_ref
				AND json_extract(NEW.candidate_provenance_json, '$.outcome_assessment_digest') = NEW.outcome_assessment_digest
				AND json_extract(NEW.candidate_provenance_json, '$.project_root') = NEW.project_root
				AND json_extract(NEW.candidate_provenance_json, '$.classifier_version') = presentation.classifier_version
				AND json_extract(NEW.candidate_provenance_json, '$.classifier_version') = basis.classifier_version
				AND json_extract(NEW.candidate_provenance_json, '$.policy_version') = presentation.policy_version
				AND json_extract(NEW.candidate_provenance_json, '$.session_ref') = presentation.session_ref
				AND json_extract(NEW.candidate_provenance_json, '$.payload_digest') = NEW.profile_payload_digest
				AND json_extract(NEW.candidate_provenance_json, '$.provenance_digest') = NEW.candidate_provenance_digest
				AND EXISTS (
					SELECT 1 FROM json_each(work_record.parameter_bindings_json) binding
					WHERE json_extract(binding.value, '$.name') = 'policy_version'
					AND json_extract(binding.value, '$.value') = presentation.policy_version
				)
				AND EXISTS (
					SELECT 1 FROM json_each(work_record.parameter_bindings_json) binding
					WHERE json_extract(binding.value, '$.name') = 'session_ref'
					AND json_extract(binding.value, '$.value') = presentation.session_ref
				)
			) BEGIN
				SELECT RAISE(ABORT, 'profile admission does not match canonical authority resolution');
			END`,
			`CREATE TRIGGER IF NOT EXISTS project_profile_admissions_no_update
			BEFORE UPDATE ON project_profile_admissions BEGIN
				SELECT RAISE(ABORT, 'project_profile_admissions is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS project_profile_admissions_no_delete
			BEFORE DELETE ON project_profile_admissions BEGIN
				SELECT RAISE(ABORT, 'project_profile_admissions is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS project_profile_revisions_no_replace
			BEFORE INSERT ON project_profile_revisions
			WHEN EXISTS (
				SELECT 1 FROM project_profile_revisions existing
				WHERE (existing.project_root = NEW.project_root AND existing.ledger_revision = NEW.ledger_revision)
				OR existing.admission_id = NEW.admission_id
				OR existing.admission_digest = NEW.admission_digest
			) BEGIN
				SELECT RAISE(ABORT, 'project_profile_revisions is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS project_profile_revisions_exact_admission
			BEFORE INSERT ON project_profile_revisions
			WHEN NOT EXISTS (
				SELECT 1
				FROM project_profile_admissions admission
				JOIN authority_uses authority_use
					ON authority_use.committed_result_ref = admission.admission_id
				WHERE admission.admission_id = NEW.admission_id
				AND admission.admission_digest = NEW.admission_digest
				AND admission.project_root = NEW.project_root
				AND admission.ledger_revision = NEW.ledger_revision
				AND admission.profile_payload_json = NEW.profile_payload_json
				AND admission.profile_payload_digest = NEW.profile_payload_digest
				AND admission.receipt_json = NEW.receipt_json
				AND admission.receipt_digest = NEW.receipt_digest
				AND authority_use.committed_result_digest = admission.admission_digest
				AND authority_use.authority_resolution_ref = admission.authority_resolution_ref
				AND authority_use.authority_resolution_digest = admission.authority_resolution_digest
				AND authority_use.single_use_key = admission.single_use_key
				AND authority_use.action_kind = admission.action_kind
				AND authority_use.project_root = admission.project_root
				AND authority_use.project_binding_digest = admission.project_binding_digest
				AND authority_use.admission_request_digest = admission.admission_request_digest
			) BEGIN
				SELECT RAISE(ABORT, 'project profile revision does not match canonical admission');
			END`,
			`CREATE TRIGGER IF NOT EXISTS project_profile_revisions_no_update
			BEFORE UPDATE ON project_profile_revisions BEGIN
				SELECT RAISE(ABORT, 'project_profile_revisions is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS project_profile_revisions_no_delete
			BEFORE DELETE ON project_profile_revisions BEGIN
				SELECT RAISE(ABORT, 'project_profile_revisions is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS project_profile_projection_debt_no_replace
			BEFORE INSERT ON project_profile_projection_debt
			WHEN EXISTS (
				SELECT 1 FROM project_profile_projection_debt existing
				WHERE existing.event_id = NEW.event_id
			) BEGIN
				SELECT RAISE(ABORT, 'project_profile_projection_debt is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS project_profile_projection_debt_no_update
			BEFORE UPDATE ON project_profile_projection_debt BEGIN
				SELECT RAISE(ABORT, 'project_profile_projection_debt is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS project_profile_projection_debt_no_delete
			BEFORE DELETE ON project_profile_projection_debt BEGIN
				SELECT RAISE(ABORT, 'project_profile_projection_debt is append-only');
			END`,
		},
	},
	{
		Version:     35,
		Description: "Append-only semantic-review SpeechAct and migration-review admission ledger",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS migration_review_speech_acts (
				speech_act_ref TEXT PRIMARY KEY,
				speech_act_digest TEXT NOT NULL UNIQUE CHECK(length(speech_act_digest) = 71 AND substr(speech_act_digest, 1, 7) = 'sha256:'),
				project_root TEXT NOT NULL,
				packet_digest TEXT NOT NULL CHECK(length(packet_digest) = 71 AND substr(packet_digest, 1, 7) = 'sha256:'),
				packet_carrier_digest TEXT NOT NULL CHECK(length(packet_carrier_digest) = 71 AND substr(packet_carrier_digest, 1, 7) = 'sha256:'),
				partition_audit_schema TEXT NOT NULL CHECK(partition_audit_schema = 'haft.spec-migration-v2.packet-partition-audit/v1'),
				partition_audit_status TEXT NOT NULL CHECK(partition_audit_status = 'verified'),
				partition_audit_digest TEXT NOT NULL CHECK(length(partition_audit_digest) = 71 AND substr(partition_audit_digest, 1, 7) = 'sha256:'),
				reviewer_role_ref TEXT NOT NULL,
				judgement_context_ref TEXT NOT NULL,
				session_ref TEXT NOT NULL,
				canonical_utterance TEXT NOT NULL,
				occurred_at TEXT NOT NULL,
				valid_from TEXT NOT NULL,
				valid_until TEXT NOT NULL,
				speech_act_json TEXT NOT NULL CHECK(json_valid(speech_act_json)),
				recorded_at TEXT NOT NULL,
				CHECK(canonical_utterance = 'ACCEPT ' || packet_carrier_digest),
				CHECK(julianday(valid_from) <= julianday(occurred_at)),
				CHECK(julianday(occurred_at) < julianday(valid_until)),
				CHECK(json_extract(speech_act_json, '$.schema') = 'haft.spec-migration-v2.semantic-review-speech-act/v1'),
				CHECK(json_extract(speech_act_json, '$.speech_act_ref') = speech_act_ref),
				CHECK(json_extract(speech_act_json, '$.project_root') = project_root),
				CHECK(json_extract(speech_act_json, '$.packet_digest') = packet_digest),
				CHECK(json_extract(speech_act_json, '$.packet_carrier_digest') = packet_carrier_digest),
				CHECK(json_extract(speech_act_json, '$.partition_audit_schema') = partition_audit_schema),
				CHECK(json_extract(speech_act_json, '$.partition_audit_status') = partition_audit_status),
				CHECK(json_extract(speech_act_json, '$.partition_audit_digest') = partition_audit_digest),
				CHECK(json_extract(speech_act_json, '$.reviewer_role_ref') = reviewer_role_ref),
				CHECK(json_extract(speech_act_json, '$.judgement_context_ref') = judgement_context_ref),
				CHECK(json_extract(speech_act_json, '$.session_ref') = session_ref),
				CHECK(json_extract(speech_act_json, '$.canonical_utterance') = canonical_utterance),
				CHECK(json_extract(speech_act_json, '$.occurred_at') = occurred_at),
				CHECK(json_extract(speech_act_json, '$.valid_from') = valid_from),
				CHECK(json_extract(speech_act_json, '$.valid_until') = valid_until)
			) WITHOUT ROWID`,
			`CREATE TABLE IF NOT EXISTS migration_review_admissions (
				admission_ref TEXT PRIMARY KEY,
				admission_digest TEXT NOT NULL UNIQUE CHECK(length(admission_digest) = 71 AND substr(admission_digest, 1, 7) = 'sha256:'),
				project_root TEXT NOT NULL,
				packet_digest TEXT NOT NULL CHECK(length(packet_digest) = 71 AND substr(packet_digest, 1, 7) = 'sha256:'),
				packet_carrier_digest TEXT NOT NULL CHECK(length(packet_carrier_digest) = 71 AND substr(packet_carrier_digest, 1, 7) = 'sha256:'),
				partition_audit_schema TEXT NOT NULL CHECK(partition_audit_schema = 'haft.spec-migration-v2.packet-partition-audit/v1'),
				partition_audit_status TEXT NOT NULL CHECK(partition_audit_status = 'verified'),
				partition_audit_digest TEXT NOT NULL CHECK(length(partition_audit_digest) = 71 AND substr(partition_audit_digest, 1, 7) = 'sha256:'),
				source_carrier TEXT NOT NULL,
				source_digest TEXT NOT NULL CHECK(length(source_digest) = 71 AND substr(source_digest, 1, 7) = 'sha256:'),
				target_carrier_digests_json TEXT NOT NULL CHECK(json_valid(target_carrier_digests_json)),
				fpf_revision TEXT NOT NULL CHECK(length(fpf_revision) = 40),
				semantic_zero_pass_carrier TEXT NOT NULL,
				semantic_zero_pass_digest TEXT NOT NULL CHECK(length(semantic_zero_pass_digest) = 71 AND substr(semantic_zero_pass_digest, 1, 7) = 'sha256:'),
				lifecycle_intent_json TEXT NOT NULL CHECK(json_valid(lifecycle_intent_json)),
				speech_act_ref TEXT NOT NULL UNIQUE REFERENCES migration_review_speech_acts(speech_act_ref),
				speech_act_digest TEXT NOT NULL,
				admission_json TEXT NOT NULL CHECK(json_valid(admission_json)),
				admitted_at TEXT NOT NULL,
				recorded_at TEXT NOT NULL,
				UNIQUE(project_root, packet_carrier_digest),
				CHECK(json_extract(admission_json, '$.schema') = 'haft.spec-migration-v2.semantic-review-admission/v1'),
				CHECK(json_extract(admission_json, '$.admission_ref') = admission_ref),
				CHECK(json_extract(admission_json, '$.speech_act_ref') = speech_act_ref),
				CHECK(json_extract(admission_json, '$.speech_act_digest') = speech_act_digest),
				CHECK(json_extract(admission_json, '$.project_root') = project_root),
				CHECK(json_extract(admission_json, '$.packet_digest') = packet_digest),
				CHECK(json_extract(admission_json, '$.packet_carrier_digest') = packet_carrier_digest),
				CHECK(json_extract(admission_json, '$.partition_audit_schema') = partition_audit_schema),
				CHECK(json_extract(admission_json, '$.partition_audit_status') = partition_audit_status),
				CHECK(json_extract(admission_json, '$.partition_audit_digest') = partition_audit_digest),
				CHECK(json_extract(admission_json, '$.source_carrier') = source_carrier),
				CHECK(json_extract(admission_json, '$.source_digest') = source_digest),
				CHECK(json_extract(admission_json, '$.fpf_revision') = fpf_revision),
				CHECK(json_extract(admission_json, '$.semantic_zero_pass_carrier') = semantic_zero_pass_carrier),
				CHECK(json_extract(admission_json, '$.semantic_zero_pass_digest') = semantic_zero_pass_digest),
				CHECK(json_extract(admission_json, '$.admitted_at') = admitted_at)
			) WITHOUT ROWID`,
			"CREATE INDEX IF NOT EXISTS idx_migration_review_speech_acts_project_packet ON migration_review_speech_acts(project_root, packet_carrier_digest)",
			"CREATE INDEX IF NOT EXISTS idx_migration_review_admissions_project_packet ON migration_review_admissions(project_root, packet_carrier_digest)",
			`CREATE TRIGGER IF NOT EXISTS migration_review_speech_acts_no_replace
			BEFORE INSERT ON migration_review_speech_acts
			WHEN EXISTS (
				SELECT 1 FROM migration_review_speech_acts existing
				WHERE existing.speech_act_ref = NEW.speech_act_ref
					OR existing.speech_act_digest = NEW.speech_act_digest
			) BEGIN
				SELECT RAISE(ABORT, 'migration_review_speech_acts is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS migration_review_speech_acts_no_update
			BEFORE UPDATE ON migration_review_speech_acts BEGIN
				SELECT RAISE(ABORT, 'migration_review_speech_acts is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS migration_review_speech_acts_no_delete
			BEFORE DELETE ON migration_review_speech_acts BEGIN
				SELECT RAISE(ABORT, 'migration_review_speech_acts is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS migration_review_admissions_no_replace
			BEFORE INSERT ON migration_review_admissions
			WHEN EXISTS (
				SELECT 1 FROM migration_review_admissions existing
				WHERE existing.admission_ref = NEW.admission_ref
					OR existing.admission_digest = NEW.admission_digest
					OR existing.speech_act_ref = NEW.speech_act_ref
					OR (existing.project_root = NEW.project_root
						AND existing.packet_carrier_digest = NEW.packet_carrier_digest)
			) BEGIN
				SELECT RAISE(ABORT, 'migration_review_admissions is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS migration_review_admissions_exact_speech_act
			BEFORE INSERT ON migration_review_admissions
			WHEN NOT EXISTS (
				SELECT 1 FROM migration_review_speech_acts act
				WHERE act.speech_act_ref = NEW.speech_act_ref
					AND act.speech_act_digest = NEW.speech_act_digest
					AND act.project_root = NEW.project_root
					AND act.packet_digest = NEW.packet_digest
					AND act.packet_carrier_digest = NEW.packet_carrier_digest
					AND act.partition_audit_schema = NEW.partition_audit_schema
					AND act.partition_audit_status = NEW.partition_audit_status
					AND act.partition_audit_digest = NEW.partition_audit_digest
					AND julianday(act.valid_from) <= julianday(NEW.admitted_at)
					AND julianday(NEW.admitted_at) < julianday(act.valid_until)
			) BEGIN
				SELECT RAISE(ABORT, 'migration review admission does not bind the exact human SpeechAct');
			END`,
			`CREATE TRIGGER IF NOT EXISTS migration_review_admissions_exact_json_bindings
			BEFORE INSERT ON migration_review_admissions
			WHEN json(json_extract(NEW.admission_json, '$.target_carrier_digests')) != json(NEW.target_carrier_digests_json)
				OR json(json_extract(NEW.admission_json, '$.lifecycle_intent')) != json(NEW.lifecycle_intent_json)
			BEGIN
				SELECT RAISE(ABORT, 'migration review admission JSON does not bind exact carriers and lifecycle intent');
			END`,
			`CREATE TRIGGER IF NOT EXISTS migration_review_admissions_no_update
			BEFORE UPDATE ON migration_review_admissions BEGIN
				SELECT RAISE(ABORT, 'migration_review_admissions is append-only');
			END`,
			`CREATE TRIGGER IF NOT EXISTS migration_review_admissions_no_delete
			BEFORE DELETE ON migration_review_admissions BEGIN
				SELECT RAISE(ABORT, 'migration_review_admissions is append-only');
			END`,
		},
	},
	{
		Version:     36,
		Description: "Reconcile the known empty legacy-v34 authority/profile schema",
		Apply:       reconcileAuthorityProfileSchema,
	},
	{
		Version:     37,
		Description: "Bind one physical project root and project ID to each durable ledger",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS project_ledger_binding (
				binding_slot INTEGER PRIMARY KEY CHECK(binding_slot = 1),
				project_id TEXT NOT NULL UNIQUE CHECK(
					length(project_id) = 12
					AND substr(project_id, 1, 4) = 'qnt_'
					AND substr(project_id, 5) NOT GLOB '*[^0-9a-f]*'
				),
				project_root TEXT NOT NULL UNIQUE,
				binding_digest TEXT NOT NULL UNIQUE CHECK(length(binding_digest) = 71 AND substr(binding_digest, 1, 7) = 'sha256:'),
				binding_json TEXT NOT NULL UNIQUE CHECK(json_valid(binding_json)),
				bound_at TEXT NOT NULL,
				CHECK(json_extract(binding_json, '$.schema') = 'haft.project-ledger-binding/v1'),
				CHECK(json_extract(binding_json, '$.project_id') = project_id),
				CHECK(json_extract(binding_json, '$.project_root') = project_root),
				CHECK(json_extract(binding_json, '$.bound_at') = bound_at)
			) WITHOUT ROWID`,
			`CREATE TRIGGER IF NOT EXISTS project_ledger_binding_no_replace
			BEFORE INSERT ON project_ledger_binding
			WHEN EXISTS (SELECT 1 FROM project_ledger_binding) BEGIN
				SELECT RAISE(ABORT, 'project_ledger_binding is immutable');
			END`,
			`CREATE TRIGGER IF NOT EXISTS project_ledger_binding_no_update
			BEFORE UPDATE ON project_ledger_binding BEGIN
				SELECT RAISE(ABORT, 'project_ledger_binding is immutable');
			END`,
			`CREATE TRIGGER IF NOT EXISTS project_ledger_binding_no_delete
			BEFORE DELETE ON project_ledger_binding BEGIN
				SELECT RAISE(ABORT, 'project_ledger_binding is immutable');
			END`,
			`CREATE TRIGGER IF NOT EXISTS migration_review_speech_acts_project_ledger_root
			BEFORE INSERT ON migration_review_speech_acts
			WHEN EXISTS (SELECT 1 FROM project_ledger_binding)
			AND NOT EXISTS (
				SELECT 1 FROM project_ledger_binding binding
				WHERE binding.project_root = NEW.project_root
			) BEGIN
				SELECT RAISE(ABORT, 'semantic-review SpeechAct does not match the bound project ledger root');
			END`,
			`CREATE TRIGGER IF NOT EXISTS migration_review_admissions_project_ledger_root
			BEFORE INSERT ON migration_review_admissions
			WHEN EXISTS (SELECT 1 FROM project_ledger_binding)
			AND NOT EXISTS (
				SELECT 1 FROM project_ledger_binding binding
				WHERE binding.project_root = NEW.project_root
			) BEGIN
				SELECT RAISE(ABORT, 'semantic-review admission does not match the bound project ledger root');
			END`,
			`CREATE TRIGGER IF NOT EXISTS authority_presentations_project_ledger_root
			BEFORE INSERT ON authority_presentations
			WHEN EXISTS (SELECT 1 FROM project_ledger_binding)
			AND NOT EXISTS (
				SELECT 1 FROM project_ledger_binding binding
				WHERE binding.project_root = NEW.project_root
			) BEGIN
				SELECT RAISE(ABORT, 'authority presentation does not match the bound project ledger root');
			END`,
			`CREATE TRIGGER IF NOT EXISTS authority_uses_project_ledger_root
			BEFORE INSERT ON authority_uses
			WHEN EXISTS (SELECT 1 FROM project_ledger_binding)
			AND NOT EXISTS (
				SELECT 1 FROM project_ledger_binding binding
				WHERE binding.project_root = NEW.project_root
			) BEGIN
				SELECT RAISE(ABORT, 'authority use does not match the bound project ledger root');
			END`,
			`CREATE TRIGGER IF NOT EXISTS observed_project_bases_project_ledger_root
			BEFORE INSERT ON observed_project_bases
			WHEN EXISTS (SELECT 1 FROM project_ledger_binding)
			AND NOT EXISTS (
				SELECT 1 FROM project_ledger_binding binding
				WHERE binding.project_root = NEW.project_root
			) BEGIN
				SELECT RAISE(ABORT, 'observed project basis does not match the bound project ledger root');
			END`,
			`CREATE TRIGGER IF NOT EXISTS profile_onboarding_work_records_project_ledger_root
			BEFORE INSERT ON profile_onboarding_work_records
			WHEN EXISTS (SELECT 1 FROM project_ledger_binding)
			AND NOT EXISTS (
				SELECT 1 FROM project_ledger_binding binding
				WHERE binding.project_root = NEW.project_root
			) BEGIN
				SELECT RAISE(ABORT, 'profile onboarding work does not match the bound project ledger root');
			END`,
			`CREATE TRIGGER IF NOT EXISTS project_profile_admissions_project_ledger_root
			BEFORE INSERT ON project_profile_admissions
			WHEN EXISTS (SELECT 1 FROM project_ledger_binding)
			AND NOT EXISTS (
				SELECT 1 FROM project_ledger_binding binding
				WHERE binding.project_root = NEW.project_root
			) BEGIN
				SELECT RAISE(ABORT, 'project-profile admission does not match the bound project ledger root');
			END`,
			`CREATE TRIGGER IF NOT EXISTS project_profile_revisions_project_ledger_root
			BEFORE INSERT ON project_profile_revisions
			WHEN EXISTS (SELECT 1 FROM project_ledger_binding)
			AND NOT EXISTS (
				SELECT 1 FROM project_ledger_binding binding
				WHERE binding.project_root = NEW.project_root
			) BEGIN
				SELECT RAISE(ABORT, 'project-profile revision does not match the bound project ledger root');
			END`,
			`CREATE TRIGGER IF NOT EXISTS project_profile_projection_debt_project_ledger_root
			BEFORE INSERT ON project_profile_projection_debt
			WHEN EXISTS (SELECT 1 FROM project_ledger_binding)
			AND NOT EXISTS (
				SELECT 1 FROM project_ledger_binding binding
				WHERE binding.project_root = NEW.project_root
			) BEGIN
				SELECT RAISE(ABORT, 'project-profile projection debt does not match the bound project ledger root');
			END`,
		},
	},
	authorityBasisMigration38,
	migrationReviewProtocolV2Migration39,
	semanticSpeechActUtteranceMigration40,
	decisionRecordEffectMigration41,
	semanticSpeechActProtocolMigration42,
	profileAuthorityV2Migration43,
	profileAuthorityAdmissionV2Migration44,
	typedMemoryStorageMigration45,
	typedMemoryStorageMigration46,
	projectTypeEnvHeadSelectionMigration47,
	projectTypeEnvHeadSelectionMigration48,
	typedMemoryDisjointEntailmentMigration49,
	legacyImportMigration50,
	profileAuthorityUnionMigration51,
	typedMemoryIdentityReconciliationMigration52,
	typedMemoryRelationalAssertionMigration53,
	typedMemoryKindClassificationMigration54,
	profileAutomaticBootstrapMigration55,
	hostRoutedOperatorAuthorityMigration56,
	projectTypeEnvCompatibleSuccessorMigration57,
}
