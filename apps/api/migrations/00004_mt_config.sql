-- +goose Up
-- Per-project machine-translation configuration. One active config per project
-- in v1. Credentials (the provider API key) are AES-256-GCM sealed via
-- HIJAU_ENCRYPTION_KEY; keyless providers (mock) leave it NULL. MT stays
-- disabled until a row exists and is enabled.
CREATE TABLE mt_config (
  id          text PRIMARY KEY,
  project_id  text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  provider    text NOT NULL,
  enabled     boolean NOT NULL DEFAULT true,
  model       text NOT NULL DEFAULT '',
  credentials bytea,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id)
);

-- +goose Down
DROP TABLE IF EXISTS mt_config;
