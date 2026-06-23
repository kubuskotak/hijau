-- name: CreateScreenshot :one
INSERT INTO screenshots (id, project_id, storage_key, name, width, height, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetScreenshot :one
SELECT * FROM screenshots WHERE id = $1;

-- name: CreateScreenshotRegion :one
INSERT INTO screenshot_regions (id, screenshot_id, key_id, translation_id, x, y, w, h)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListKeyScreenshotRegions :many
SELECT sqlc.embed(s), sqlc.embed(r)
FROM screenshot_regions r
JOIN screenshots s ON s.id = r.screenshot_id
WHERE r.key_id = $1
ORDER BY s.created_at DESC, r.id;
