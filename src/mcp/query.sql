-- query.sql

-- name: GetHolon :one
SELECT * FROM holons
WHERE id = ? LIMIT 1;

-- name: ListHolonsByLayer :many
SELECT * FROM holons
WHERE layer = ?
ORDER BY created_at DESC;

-- name: CreateHolon :one
INSERT INTO holons (id, type, layer, title, content, context_id)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateHolonLayer :exec
UPDATE holons
SET layer = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: CreateEvidence :one
INSERT INTO evidence (id, holon_id, type, content, verdict, valid_until)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetEvidenceForHolon :many
SELECT * FROM evidence
WHERE holon_id = ?
ORDER BY created_at DESC;

-- name: AddCharacteristic :exec
INSERT INTO characteristics (id, holon_id, name, scale, value, unit)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetCharacteristics :many
SELECT * FROM characteristics
WHERE holon_id = ?;

-- name: AddRelation :exec
INSERT INTO relations (source_id, target_id, relation_type)
VALUES (?, ?, ?);

-- name: GetLinksFrom :many
SELECT h.*, r.relation_type
FROM holons h
JOIN relations r ON h.id = r.target_id
WHERE r.source_id = ?;

-- name: GetLinksTo :many
SELECT h.*, r.relation_type
FROM holons h
JOIN relations r ON h.id = r.source_id
WHERE r.target_id = ?;
