-- name: CreateNamespace :one
INSERT INTO namespaces (id, project_id, name) VALUES ($1, $2, $3) RETURNING *;

-- name: ListNamespaces :many
SELECT * FROM namespaces WHERE project_id = $1 ORDER BY name;

-- name: GetNamespaceByName :one
SELECT * FROM namespaces WHERE project_id = $1 AND name = $2;
