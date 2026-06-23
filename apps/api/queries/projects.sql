-- name: CreateProject :one
INSERT INTO projects (id, org_id, name, slug, description)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetProject :one
SELECT * FROM projects WHERE id = $1 AND deleted_at IS NULL;

-- name: ListProjectsForUser :many
SELECT DISTINCT p.id, p.org_id, p.name, p.slug, p.description, p.base_language_id,
       p.icu_enabled, p.auto_translate, p.mt_provider, p.created_at, p.updated_at, p.deleted_at
FROM projects p
LEFT JOIN project_members pm ON pm.project_id = p.id AND pm.user_id = $1
LEFT JOIN org_memberships om ON om.org_id = p.org_id AND om.user_id = $1
WHERE p.deleted_at IS NULL
  AND (pm.user_id IS NOT NULL OR om.role IN ('owner', 'admin'))
ORDER BY p.created_at DESC;

-- name: UpdateProject :one
UPDATE projects
SET name = $2, description = $3, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SetProjectBaseLanguage :exec
UPDATE projects SET base_language_id = $2, updated_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteProject :exec
UPDATE projects SET deleted_at = now() WHERE id = $1;

-- name: CreateProjectMember :one
INSERT INTO project_members (id, project_id, user_id, role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListProjectMembers :many
SELECT pm.id, pm.project_id, pm.user_id, pm.role, pm.created_at, u.email, u.name
FROM project_members pm
JOIN users u ON u.id = pm.user_id
WHERE pm.project_id = $1
ORDER BY pm.created_at;
