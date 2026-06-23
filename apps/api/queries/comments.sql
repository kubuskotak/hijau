-- name: CreateComment :one
INSERT INTO comments (id, translation_id, key_id, author_id, body, parent_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetComment :one
SELECT * FROM comments WHERE id = $1;

-- name: GetCommentProjectID :one
SELECT k.project_id
FROM comments c
LEFT JOIN translations t ON t.id = c.translation_id
JOIN translation_keys k ON k.id = COALESCE(c.key_id, t.key_id)
WHERE c.id = $1;

-- name: ListCommentsForTranslation :many
SELECT c.id, c.translation_id, c.key_id, c.author_id, c.body, c.parent_id,
       c.resolved_at, c.resolved_by, c.created_at, c.updated_at,
       u.email AS author_email, u.name AS author_name
FROM comments c
JOIN users u ON u.id = c.author_id
WHERE c.translation_id = $1
ORDER BY c.created_at ASC;

-- name: ResolveComment :one
UPDATE comments SET resolved_at = now(), resolved_by = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UnresolveComment :one
UPDATE comments SET resolved_at = NULL, resolved_by = NULL, updated_at = now()
WHERE id = $1
RETURNING *;
