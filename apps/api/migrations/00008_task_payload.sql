-- +goose Up
-- The tasks table (00002_init) holds task status/progress/result but no input.
-- payload carries the enqueued job's parameters so the worker is fully
-- DB-driven and restart-safe (no in-memory handoff from the enqueuing request).
ALTER TABLE tasks ADD COLUMN payload jsonb;

-- A partial index keeps the worker's "claim next queued task" scan cheap as the
-- table accumulates finished rows.
CREATE INDEX idx_tasks_queued ON tasks (created_at) WHERE status = 'queued';

-- +goose Down
DROP INDEX IF EXISTS idx_tasks_queued;
ALTER TABLE tasks DROP COLUMN IF EXISTS payload;
