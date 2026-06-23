-- name: GetProjectMember :one
SELECT * FROM project_members WHERE project_id = $1 AND user_id = $2;

-- name: GetOrgMembership :one
SELECT * FROM org_memberships WHERE org_id = $1 AND user_id = $2;

-- name: ListProjectMemberLanguageIDs :many
SELECT language_id FROM project_member_languages WHERE member_id = $1;

-- name: GetProjectForAuth :one
SELECT id, org_id FROM projects WHERE id = $1 AND deleted_at IS NULL;
