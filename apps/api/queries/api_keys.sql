-- name: CreateAPIKey :one
INSERT INTO api_keys (id, type, name, key_hash, prefix, scopes, owner_user_id, project_id, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetAPIKeyByHash :one
SELECT * FROM api_keys
WHERE key_hash = $1
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > now());

-- name: TouchAPIKey :exec
UPDATE api_keys SET last_used_at = now() WHERE id = $1;

-- name: RevokeAPIKey :exec
UPDATE api_keys SET revoked_at = now() WHERE id = $1;

-- name: ListAPIKeysByProject :many
SELECT * FROM api_keys
WHERE project_id = $1 AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: ListPATsByUser :many
SELECT * FROM api_keys
WHERE owner_user_id = $1 AND type = 'pat' AND revoked_at IS NULL
ORDER BY created_at DESC;
