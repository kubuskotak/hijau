-- name: CreateKey :one
INSERT INTO translation_keys (id, project_id, namespace_id, name, description, is_plural)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetKey :one
SELECT * FROM translation_keys WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateKey :one
UPDATE translation_keys
SET description = $2, is_plural = $3, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SetKeySourceHash :exec
UPDATE translation_keys SET source_hash = $2, updated_at = now() WHERE id = $1;

-- name: SoftDeleteKey :exec
UPDATE translation_keys SET deleted_at = now() WHERE id = $1;

-- name: CountKeys :one
SELECT count(*) FROM translation_keys WHERE project_id = $1 AND deleted_at IS NULL;

-- name: ListKeys :many
SELECT * FROM translation_keys
WHERE project_id = sqlc.arg('project_id')
  AND deleted_at IS NULL
  AND (sqlc.narg('namespace_id')::text IS NULL OR namespace_id = sqlc.narg('namespace_id'))
  AND (sqlc.narg('search')::text IS NULL OR name ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY name
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');
