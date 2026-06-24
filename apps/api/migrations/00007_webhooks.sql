-- +goose Up
-- Webhooks: outbound HTTP notifications on translation events, HMAC-signed.
-- The signing secret is AES-256-GCM sealed (HIJAU_ENCRYPTION_KEY). Deliveries
-- are logged for observability.
CREATE TABLE webhooks (
  id         text PRIMARY KEY,
  project_id text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  url        text NOT NULL,
  secret     bytea NOT NULL,                  -- sealed HMAC signing secret
  events     text[] NOT NULL DEFAULT '{}',    -- empty = all events
  active     boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_webhooks_project ON webhooks (project_id);

CREATE TABLE webhook_deliveries (
  id          text PRIMARY KEY,
  webhook_id  text NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
  event       text NOT NULL,
  status_code integer NOT NULL DEFAULT 0,
  success     boolean NOT NULL DEFAULT false,
  error       text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries (webhook_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhooks;
