-- Runs once on first Postgres init (empty data dir). Migrations (goose) own the
-- schema; this only enables extensions the app relies on.
CREATE EXTENSION IF NOT EXISTS pg_trgm;        -- fuzzy translation-memory search
CREATE EXTENSION IF NOT EXISTS fuzzystrmatch;  -- levenshtein() for TM match scoring
-- pgvector ships in this image but stays dormant until semantic TM is enabled:
-- CREATE EXTENSION IF NOT EXISTS vector;
