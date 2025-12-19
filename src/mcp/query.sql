-- query.sql
-- sqlc queries for FPF database operations

-- Holon queries

-- name: CreateHolon :exec
INSERT INTO holons (id, type, kind, layer, title, content, context_id, scope, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

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

-- name: GetRelationsByTarget :many
SELECT * FROM relations WHERE target_id = ? AND relation_type = ?;

-- name: GetComponentsOf :many
SELECT source_id, congruence_level FROM relations
WHERE target_id = ? AND relation_type = 'componentOf';

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
