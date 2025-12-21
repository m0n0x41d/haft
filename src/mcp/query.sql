-- query.sql
-- sqlc queries for FPF database operations

-- Holon queries

-- name: CreateHolon :exec
INSERT INTO holons (id, type, kind, layer, title, content, context_id, scope, parent_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

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
SELECT layer, COUNT(*) as count FROM holons WHERE context_id = ? GROUP BY layer;

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
INSERT INTO evidence (id, holon_id, type, content, verdict, assurance_level, carrier_ref, valid_until, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetEvidenceByHolon :many
SELECT * FROM evidence WHERE holon_id = ? ORDER BY created_at DESC;

-- name: GetEvidenceWithCarrier :many
SELECT * FROM evidence WHERE carrier_ref IS NOT NULL AND carrier_ref != '';

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
