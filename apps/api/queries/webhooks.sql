-- name: CreateWebhook :one
INSERT INTO webhooks (id, project_id, url, secret, events, active)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListWebhooks :many
SELECT * FROM webhooks WHERE project_id = $1 ORDER BY created_at DESC;

-- name: ListActiveWebhooks :many
SELECT * FROM webhooks WHERE project_id = $1 AND active = true;

-- name: GetWebhook :one
SELECT * FROM webhooks WHERE id = $1;

-- name: DeleteWebhook :exec
DELETE FROM webhooks WHERE id = $1;

-- name: InsertWebhookDelivery :exec
INSERT INTO webhook_deliveries (id, webhook_id, event, status_code, success, error)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListWebhookDeliveries :many
SELECT * FROM webhook_deliveries WHERE webhook_id = $1 ORDER BY created_at DESC LIMIT 50;
