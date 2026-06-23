-- name: GetTranslation :one
SELECT * FROM translations WHERE key_id = $1 AND language_id = $2;

-- name: GetTranslationForUpdate :one
SELECT * FROM translations WHERE key_id = $1 AND language_id = $2 FOR UPDATE;

-- name: GetTranslationBySubID :one
SELECT * FROM translations WHERE sub_id = $1;

-- name: CreateTranslation :one
INSERT INTO translations (id, key_id, language_id, text, state, origin, is_machine, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateTranslation :one
UPDATE translations
SET text = $2, state = $3, origin = $4, is_machine = $5,
    version = version + 1, updated_by = $6, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListTranslationsForKey :many
SELECT * FROM translations WHERE key_id = $1;

-- name: MarkSiblingsOutdated :many
UPDATE translations
SET state = 'outdated', version = version + 1, updated_at = now()
WHERE key_id = $1 AND language_id <> $2 AND state IN ('translated', 'reviewed')
RETURNING *;

-- name: InsertTranslationHistory :exec
INSERT INTO translation_history
  (id, translation_id, key_id, language_id, old_text, new_text, old_state, new_state, origin, author_kind, author_id, api_key_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: InsertActivity :exec
INSERT INTO activity (id, project_id, type, actor_id, actor_kind, key_id, translation_id, language_id, meta)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: ListKeyHistory :many
SELECT * FROM translation_history WHERE key_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: ListProjectActivity :many
SELECT * FROM activity WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2;
