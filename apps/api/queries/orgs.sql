-- name: CreateOrganization :one
INSERT INTO organizations (id, name, slug) VALUES ($1, $2, $3) RETURNING *;

-- name: CreateOrgMembership :one
INSERT INTO org_memberships (id, org_id, user_id, role) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: ListUserOrganizations :many
SELECT o.* FROM organizations o
JOIN org_memberships m ON m.org_id = o.id
WHERE m.user_id = $1
ORDER BY o.created_at;

-- name: GetOrganization :one
SELECT * FROM organizations WHERE id = $1;
