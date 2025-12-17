-- schema.sql
-- FPF Core Schema

CREATE TABLE holons (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL, -- 'hypothesis', 'system', 'method', 'capability'
    kind TEXT, -- 'system' (architecture/code) or 'episteme' (knowledge/docs) - C.3 Kind-CAL
    layer TEXT NOT NULL, -- 'L0', 'L1', 'L2', 'invalid'
    title TEXT NOT NULL,
    content TEXT NOT NULL, -- Markdown content or reference
    context_id TEXT NOT NULL, -- Bounded Context ID
    scope TEXT, -- Claim Scope (G)
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE evidence (
    id TEXT PRIMARY KEY,
    holon_id TEXT NOT NULL,
    type TEXT NOT NULL, -- 'test_result', 'formal_proof', 'log'
    content TEXT NOT NULL,
    verdict TEXT NOT NULL, -- 'pass', 'fail', 'degrade'
    valid_until DATETIME, -- B.3.4 Evidence Decay
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(holon_id) REFERENCES holons(id)
);

CREATE TABLE characteristics (
    id TEXT PRIMARY KEY,
    holon_id TEXT NOT NULL,
    name TEXT NOT NULL, -- 'F', 'G', 'R', 'latency', 'coverage'
    scale TEXT NOT NULL, -- 'ordinal', 'ratio', 'interval', 'nominal'
    value TEXT NOT NULL,
    unit TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(holon_id) REFERENCES holons(id)
);

CREATE TABLE relations (
    source_id TEXT NOT NULL,
    target_id TEXT NOT NULL,
    relation_type TEXT NOT NULL, -- 'verifiedBy', 'componentOf', 'refines', 'performedBy'
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (source_id, target_id, relation_type),
    FOREIGN KEY(source_id) REFERENCES holons(id),
    FOREIGN KEY(target_id) REFERENCES holons(id)
);
