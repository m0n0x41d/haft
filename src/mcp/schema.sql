-- schema.sql
-- FPF Core Schema

CREATE TABLE holons (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    kind TEXT,
    layer TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    context_id TEXT NOT NULL,
    scope TEXT,
    parent_id TEXT REFERENCES holons(id),
    cached_r_score REAL DEFAULT 0.0 CHECK(cached_r_score BETWEEN 0.0 AND 1.0),
    needs_reverification INTEGER DEFAULT 0,
    reverification_reason TEXT,
    reverification_since DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE evidence (
    id TEXT PRIMARY KEY,
    holon_id TEXT NOT NULL,
    type TEXT NOT NULL,
    content TEXT NOT NULL,
    verdict TEXT NOT NULL,
    assurance_level TEXT,
    carrier_ref TEXT,
    carrier_hash TEXT,
    carrier_commit TEXT,
    is_stale INTEGER DEFAULT 0,
    stale_reason TEXT,
    stale_since DATETIME,
    valid_until DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(holon_id) REFERENCES holons(id)
);

CREATE TABLE characteristics (
    id TEXT PRIMARY KEY,
    holon_id TEXT NOT NULL,
    name TEXT NOT NULL,
    scale TEXT NOT NULL,
    value TEXT NOT NULL,
    unit TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(holon_id) REFERENCES holons(id)
);

CREATE TABLE relations (
    source_id TEXT NOT NULL,
    target_id TEXT NOT NULL,
    relation_type TEXT NOT NULL,
    congruence_level INTEGER DEFAULT 3 CHECK(congruence_level BETWEEN 0 AND 3),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (source_id, target_id, relation_type),
    FOREIGN KEY(source_id) REFERENCES holons(id),
    FOREIGN KEY(target_id) REFERENCES holons(id)
);

CREATE TABLE work_records (
    id TEXT PRIMARY KEY,
    method_ref TEXT NOT NULL,
    performer_ref TEXT NOT NULL,
    started_at DATETIME NOT NULL,
    ended_at DATETIME,
    resource_ledger TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE audit_log (
    id TEXT PRIMARY KEY,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    tool_name TEXT NOT NULL,
    operation TEXT NOT NULL,
    actor TEXT NOT NULL,
    target_id TEXT,
    input_hash TEXT,
    result TEXT NOT NULL,
    details TEXT,
    context_id TEXT NOT NULL DEFAULT 'default'
);

CREATE TABLE waivers (
    id TEXT PRIMARY KEY,
    evidence_id TEXT NOT NULL,
    waived_by TEXT NOT NULL,
    waived_until DATETIME NOT NULL,
    rationale TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(evidence_id) REFERENCES evidence(id)
);

CREATE TABLE fpf_state (
    context_id TEXT PRIMARY KEY,
    active_role TEXT,
    active_session_id TEXT,
    active_role_context TEXT,
    last_commit TEXT,
    last_commit_at DATETIME,
    assurance_threshold REAL DEFAULT 0.8 CHECK(assurance_threshold BETWEEN 0.0 AND 1.0),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for WLNK traversal
CREATE INDEX IF NOT EXISTS idx_relations_target ON relations(target_id, relation_type);
CREATE INDEX IF NOT EXISTS idx_relations_source ON relations(source_id, relation_type);
CREATE INDEX IF NOT EXISTS idx_waivers_evidence ON waivers(evidence_id);

-- Indexes for Code Change Awareness
CREATE INDEX IF NOT EXISTS idx_evidence_carrier ON evidence(carrier_ref);
CREATE INDEX IF NOT EXISTS idx_evidence_stale ON evidence(is_stale) WHERE is_stale = 1;
CREATE INDEX IF NOT EXISTS idx_holons_reverification ON holons(needs_reverification) WHERE needs_reverification = 1;

-- Active holons: not selected/rejected by a resolved DRR
-- A DRR is "resolved" if it has implementation/abandonment/supersession evidence
-- Used by: DerivePhase (fsm.go), GetActiveRecentHolons, CountActiveHolonsByLayer
CREATE VIEW active_holons AS
SELECT h.*
FROM holons h
WHERE h.layer NOT IN ('invalid')
  AND NOT EXISTS (
    SELECT 1 FROM relations r
    INNER JOIN holons drr ON drr.id = r.source_id
    WHERE r.target_id = h.id
      AND r.relation_type IN ('selects', 'rejects')
      AND (drr.type = 'DRR' OR drr.layer = 'DRR')
      AND EXISTS (
          SELECT 1 FROM evidence e
          WHERE e.holon_id = drr.id
          AND e.type IN ('implementation', 'abandonment', 'supersession')
      )
  );
