-- query.sql
-- sqlc queries for FPF database operations

-- Holon queries

-- name: CreateHolon :exec
INSERT INTO holons (id, type, kind, layer, title, content, context_id, scope, parent_id, approach_type, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetHolon :one
SELECT * FROM holons WHERE id = ? LIMIT 1;

-- name: GetHolonTitle :one
SELECT title FROM holons WHERE id = ? LIMIT 1;

-- name: ListAllHolonIDs :many
SELECT id FROM holons;

-- name: ListHolonsByLayer :many
SELECT * FROM holons WHERE layer = ? ORDER BY created_at DESC;

-- name: UpdateHolonLayer :exec
UPDATE holons SET layer = ?, updated_at = ? WHERE id = ?;

-- name: UpdateHolonRScore :exec
UPDATE holons SET cached_r_score = ?, updated_at = ? WHERE id = ?;

-- name: GetHolonsByParent :many
SELECT * FROM holons WHERE parent_id = ? ORDER BY created_at DESC;

-- name: CountHolonsByLayer :many
SELECT layer, COUNT(*) as count FROM active_holons WHERE context_id = ? GROUP BY layer;

-- name: GetLatestHolonByContext :one
SELECT * FROM holons WHERE context_id = ? ORDER BY updated_at DESC LIMIT 1;

-- name: GetHolonLineage :many
WITH RECURSIVE lineage AS (
    SELECT h.id, h.type, h.kind, h.layer, h.title, h.content, h.context_id, h.scope, h.parent_id, h.cached_r_score, h.created_at, h.updated_at, 0 as depth
    FROM holons h WHERE h.id = ?
    UNION ALL
    SELECT p.id, p.type, p.kind, p.layer, p.title, p.content, p.context_id, p.scope, p.parent_id, p.cached_r_score, p.created_at, p.updated_at, l.depth + 1
    FROM holons p
    INNER JOIN lineage l ON p.id = l.parent_id
)
SELECT id, type, kind, layer, title, content, context_id, scope, parent_id, cached_r_score, created_at, updated_at, depth FROM lineage ORDER BY depth DESC;

-- Evidence queries

-- name: AddEvidence :exec
INSERT INTO evidence (id, holon_id, type, content, verdict, assurance_level, formality_level, carrier_ref, carrier_hash, carrier_commit, valid_until, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetEvidenceByHolon :many
SELECT * FROM evidence WHERE holon_id = ? ORDER BY created_at DESC;

-- name: GetEvidenceWithCarrier :many
SELECT * FROM evidence WHERE carrier_ref IS NOT NULL AND carrier_ref != '';

-- name: GetEvidenceWithCarrierCommit :many
SELECT e.id, e.holon_id, e.type, e.content, e.verdict,
       e.assurance_level, e.carrier_ref, e.carrier_commit,
       e.valid_until, e.created_at,
       h.title as holon_title, h.layer as holon_layer
FROM evidence e
JOIN holons h ON e.holon_id = h.id
WHERE e.carrier_commit IS NOT NULL
  AND e.carrier_commit != ''
  AND e.carrier_ref IS NOT NULL
  AND e.carrier_ref != '';

-- Relation queries

-- name: AddRelation :exec
INSERT INTO relations (source_id, target_id, relation_type, created_at)
VALUES (?, ?, ?, ?);

-- name: CreateRelation :exec
INSERT INTO relations (source_id, relation_type, target_id, congruence_level)
VALUES (?, ?, ?, ?)
ON CONFLICT(source_id, relation_type, target_id)
DO UPDATE SET congruence_level = excluded.congruence_level;

-- name: GetRelationsByTarget :many
SELECT * FROM relations WHERE target_id = ? AND relation_type = ?;

-- name: GetComponentsOf :many
SELECT source_id, congruence_level FROM relations
WHERE target_id = ? AND relation_type = 'componentOf';

-- name: GetDependencies :many
SELECT target_id, relation_type, congruence_level
FROM relations
WHERE source_id = ? AND relation_type IN ('componentOf', 'constituentOf');

-- name: GetDependents :many
SELECT source_id, relation_type, congruence_level
FROM relations
WHERE target_id = ? AND relation_type IN ('componentOf', 'constituentOf');

-- name: GetCollectionMembers :many
SELECT source_id, congruence_level
FROM relations
WHERE target_id = ? AND relation_type = 'memberOf';

-- Work record queries

-- name: RecordWork :exec
INSERT INTO work_records (id, method_ref, performer_ref, started_at, ended_at, resource_ledger, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- Characteristic queries

-- name: AddCharacteristic :exec
INSERT INTO characteristics (id, holon_id, name, scale, value, unit, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetCharacteristics :many
SELECT * FROM characteristics WHERE holon_id = ?;

-- Audit log queries

-- name: InsertAuditLog :exec
INSERT INTO audit_log (id, tool_name, operation, actor, target_id, input_hash, result, details, context_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAuditLogByContext :many
SELECT * FROM audit_log WHERE context_id = ? ORDER BY timestamp DESC;

-- name: GetAuditLogByTarget :many
SELECT * FROM audit_log WHERE target_id = ? ORDER BY timestamp DESC;

-- name: GetRecentAuditLog :many
SELECT * FROM audit_log ORDER BY timestamp DESC LIMIT ?;

-- Waiver queries

-- name: CreateWaiver :exec
INSERT INTO waivers (id, evidence_id, waived_by, waived_until, rationale, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetActiveWaiverForEvidence :one
SELECT * FROM waivers
WHERE evidence_id = ? AND waived_until > datetime('now')
ORDER BY waived_until DESC LIMIT 1;

-- name: GetWaiversByEvidence :many
SELECT * FROM waivers WHERE evidence_id = ? ORDER BY created_at DESC;

-- name: GetAllActiveWaivers :many
SELECT * FROM waivers WHERE waived_until > datetime('now') ORDER BY waived_until ASC;

-- name: GetEvidenceByID :one
SELECT * FROM evidence WHERE id = ? LIMIT 1;

-- Active holon queries (exclude holons in resolved decisions)
-- NOTE: Decisions are holons with type='DRR' or layer='DRR'.
-- Resolution is tracked via evidence records of type 'implementation'/'abandonment'/'supersession'.
-- DRRs create 'selects' relations to winner hypotheses and 'rejects' relations to rejected ones.
--
-- The active_holons VIEW (migration v6) defines "active" = not selected/rejected by resolved DRR.
-- Queries below use the view and add layer filters as needed.

-- name: GetActiveRecentHolons :many
-- Returns working holons (L0/L1/L2) not belonging to resolved decisions.
-- Uses active_holons view + excludes DRR/invalid layers for display purposes.
SELECT id, type, kind, layer, title, content, context_id,
       scope, parent_id, cached_r_score, created_at, updated_at
FROM active_holons
WHERE layer NOT IN ('DRR', 'invalid')
  AND type != 'DRR'
ORDER BY updated_at DESC
LIMIT ?;

-- name: CountActiveHolonsByLayer :many
-- Counts working holons by layer, using active_holons view.
SELECT layer, COUNT(*) as count
FROM active_holons
WHERE layer NOT IN ('DRR', 'invalid')
  AND type != 'DRR'
GROUP BY layer;

-- name: CountArchivedHolonsByLayer :many
-- Counts holons by layer that ARE selected/rejected by resolved DRRs (archived).
-- INVERSE of active_holons VIEW logic. If active_holons definition changes,
-- update this query accordingly. See migration v6.
SELECT h.layer, COUNT(*) as count
FROM holons h
WHERE h.layer NOT IN ('DRR', 'invalid')
  AND h.type != 'DRR'
  AND EXISTS (
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
)
GROUP BY h.layer;

-- ============================================
-- COMPACTION QUERIES (v5.1.0)
-- ============================================

-- name: GetArchivedHolonsForCompaction :many
-- Returns archived holons older than retention_days after decision resolution.
-- These are candidates for compaction (content summarization, evidence deletion).
-- sqlc: arg(retention_days) type: int64
SELECT h.id, h.type, h.kind, h.layer, h.title, h.content, h.context_id,
       h.scope, h.parent_id, h.cached_r_score, h.created_at, h.updated_at,
       drr.id as decision_id, drr.title as decision_title,
       r.relation_type as decision_outcome,
       MAX(e.created_at) as resolved_at
FROM holons h
INNER JOIN relations r ON r.target_id = h.id AND r.relation_type IN ('selects', 'rejects')
INNER JOIN holons drr ON drr.id = r.source_id AND (drr.type = 'DRR' OR drr.layer = 'DRR')
INNER JOIN evidence e ON e.holon_id = drr.id AND e.type IN ('implementation', 'abandonment', 'supersession')
WHERE h.layer NOT IN ('DRR', 'invalid')
  AND h.type != 'DRR'
  AND h.content != '[COMPACTED]'
  AND CAST(JULIANDAY('now') - JULIANDAY(e.created_at) AS INTEGER) > CAST(sqlc.arg(retention_days) AS INTEGER)
GROUP BY h.id
ORDER BY resolved_at ASC;

-- name: CompactHolonContent :exec
UPDATE holons
SET content = '[COMPACTED]',
    scope = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteEvidenceForHolon :exec
DELETE FROM evidence WHERE holon_id = ?;

-- name: DeleteCharacteristicsForHolon :exec
DELETE FROM characteristics WHERE holon_id = ?;

-- name: DeleteWaiversForHolon :exec
DELETE FROM waivers WHERE evidence_id IN (SELECT id FROM evidence WHERE holon_id = ?);

-- name: CountCompactableHolons :one
SELECT COUNT(*) as count
FROM holons h
INNER JOIN relations r ON r.target_id = h.id AND r.relation_type IN ('selects', 'rejects')
INNER JOIN holons drr ON drr.id = r.source_id AND (drr.type = 'DRR' OR drr.layer = 'DRR')
INNER JOIN evidence e ON e.holon_id = drr.id AND e.type IN ('implementation', 'abandonment', 'supersession')
WHERE h.layer NOT IN ('DRR', 'invalid')
  AND h.type != 'DRR'
  AND h.content != '[COMPACTED]'
  AND CAST(JULIANDAY('now') - JULIANDAY(e.created_at) AS INTEGER) > CAST(sqlc.arg(retention_days) AS INTEGER);

-- ============================================
-- REVERIFICATION QUERIES (v5.0.0)
-- ============================================
-- Note: Evidence staleness by carrier-file hash was removed in v5.1.0.
-- Time-based decay via valid_until remains as per FPF spec B.3.4.
-- DRR affected_scope tracking uses carrier_ref for implementation warnings.

-- name: MarkHolonNeedsReverification :exec
UPDATE holons
SET needs_reverification = 1,
    reverification_reason = ?,
    reverification_since = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: ClearHolonReverification :exec
UPDATE holons
SET needs_reverification = 0,
    reverification_reason = NULL,
    reverification_since = NULL
WHERE id = ?;

-- name: GetHolonsNeedingReverification :many
SELECT * FROM active_holons
WHERE needs_reverification = 1
ORDER BY reverification_since DESC;

-- name: CountHolonsNeedingReverification :one
SELECT COUNT(*) as count FROM active_holons WHERE needs_reverification = 1;

-- name: UpdateLastCommit :exec
UPDATE fpf_state
SET last_commit = ?,
    last_commit_at = CURRENT_TIMESTAMP
WHERE context_id = ?;

-- name: GetLastCommit :one
SELECT last_commit, last_commit_at
FROM fpf_state
WHERE context_id = ?;

-- name: GetEvidenceByCarrierPattern :many
SELECT e.id, e.holon_id, e.type, e.content, e.verdict,
       e.assurance_level, e.carrier_ref, e.carrier_commit,
       e.valid_until, e.created_at,
       h.title as holon_title, h.layer as holon_layer
FROM evidence e
JOIN holons h ON e.holon_id = h.id
WHERE e.carrier_ref LIKE ?;

-- ============================================
-- PREDICTIONS QUERIES (v5.1.0)
-- ============================================

-- name: AddPrediction :exec
INSERT INTO predictions (id, holon_id, content)
VALUES (?, ?, ?);

-- name: GetPredictionsByHolon :many
SELECT id, holon_id, content, covered, covered_by, created_at
FROM predictions
WHERE holon_id = ?;

-- name: GetUncoveredPredictions :many
SELECT id, holon_id, content, created_at
FROM predictions
WHERE holon_id = ? AND covered = 0;

-- name: MarkPredictionCovered :exec
UPDATE predictions
SET covered = 1, covered_by = ?
WHERE id = ?;

-- name: CountUncoveredPredictions :one
SELECT COUNT(*) as count
FROM predictions
WHERE holon_id = ? AND covered = 0;
