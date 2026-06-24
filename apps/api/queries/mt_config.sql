-- name: UpsertMTConfig :one
INSERT INTO mt_config (id, project_id, provider, enabled, model, credentials)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (project_id) DO UPDATE SET
  provider = EXCLUDED.provider,
  enabled = EXCLUDED.enabled,
  model = EXCLUDED.model,
  credentials = EXCLUDED.credentials,
  updated_at = now()
RETURNING *;

-- name: GetMTConfig :one
SELECT * FROM mt_config WHERE project_id = $1;

-- name: DeleteMTConfig :exec
DELETE FROM mt_config WHERE project_id = $1;
