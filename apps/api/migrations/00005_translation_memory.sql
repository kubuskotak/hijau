-- +goose Up
-- Translation memory: approved (source, target) pairs, reused to suggest
-- translations and to avoid paying for MT on strings already translated.
-- Populated only on review/approval so it stays high quality.
CREATE TABLE tm_segments (
  id          text PRIMARY KEY,
  project_id  text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  source_lang text NOT NULL,            -- BCP-47 tag
  target_lang text NOT NULL,            -- BCP-47 tag
  source_text text NOT NULL,
  target_text text NOT NULL,
  source_hash text NOT NULL,            -- hash of the source, for exact match
  key_id      text REFERENCES translation_keys(id) ON DELETE SET NULL,
  origin      text NOT NULL DEFAULT 'human',
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, source_lang, target_lang, source_hash, target_text)
);
-- exact match (source_hash) and fuzzy match (trigram on the source text)
CREATE INDEX idx_tm_exact ON tm_segments (project_id, source_lang, target_lang, source_hash);
CREATE INDEX idx_tm_trgm ON tm_segments USING gin (lower(source_text) gin_trgm_ops);

-- +goose Down
DROP TABLE IF EXISTS tm_segments;
