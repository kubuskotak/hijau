-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;        -- fuzzy search (keys/translations) + TM later
CREATE EXTENSION IF NOT EXISTS fuzzystrmatch;  -- levenshtein() for TM match scoring

-- +goose Down
-- Extensions are intentionally left in place on rollback; other objects depend on them.
