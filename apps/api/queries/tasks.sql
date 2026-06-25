-- name: CreateTask :one
INSERT INTO tasks (id, project_id, type, payload, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetTask :one
SELECT * FROM tasks WHERE id = $1;

-- name: ListTasksByProject :many
SELECT * FROM tasks
WHERE project_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: ClaimNextTask :one
-- Atomically grab the oldest queued task and mark it running. FOR UPDATE SKIP
-- LOCKED makes this safe even if more than one worker ever runs concurrently.
UPDATE tasks
SET status = 'running', started_at = now()
WHERE id = (
  SELECT id FROM tasks
  WHERE status = 'queued'
  ORDER BY created_at, id
  FOR UPDATE SKIP LOCKED
  LIMIT 1
)
RETURNING *;

-- name: SetTaskProgress :exec
UPDATE tasks SET processed = $2, total = $3, progress = $4 WHERE id = $1;

-- name: CompleteTask :exec
-- Clear error too: a task may carry a stale error from a prior partial run that
-- was requeued by RecoverRunningTasks and then succeeded.
UPDATE tasks
SET status = 'succeeded', result = $2, error = NULL, progress = 100, finished_at = now()
WHERE id = $1;

-- name: FailTask :exec
-- Clear result for the symmetric reason (a requeued run that then fails).
UPDATE tasks
SET status = 'failed', error = $2, result = NULL, finished_at = now()
WHERE id = $1;

-- name: RecoverRunningTasks :exec
-- On boot, any task still marked running is orphaned (the worker that owned it
-- died); requeue it so a single-instance deployment recovers cleanly. Reset the
-- progress counters so a polling client doesn't briefly see last-run's numbers.
UPDATE tasks
SET status = 'queued', started_at = NULL, progress = 0, processed = NULL, total = NULL
WHERE status = 'running';
