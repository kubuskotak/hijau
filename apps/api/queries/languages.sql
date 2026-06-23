-- name: CreateLanguage :one
INSERT INTO languages (id, project_id, tag, name, is_rtl, plural_forms)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListLanguages :many
SELECT * FROM languages WHERE project_id = $1 ORDER BY tag;

-- name: GetLanguage :one
SELECT * FROM languages WHERE id = $1;

-- name: GetLanguageByTag :one
SELECT * FROM languages WHERE project_id = $1 AND tag = $2;

-- name: DeleteLanguage :exec
DELETE FROM languages WHERE id = $1;
