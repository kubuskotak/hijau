-- +goose Up
-- Screenshots captured in-context, plus the pixel regions that map a captured
-- image to the translation keys visible in it.
CREATE TABLE screenshots (
  id          text PRIMARY KEY,
  project_id  text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  storage_key text NOT NULL,                 -- path within the storage backend
  name        text NOT NULL DEFAULT '',
  width       integer NOT NULL DEFAULT 0,
  height      integer NOT NULL DEFAULT 0,
  created_by  text REFERENCES users(id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_screenshots_project ON screenshots (project_id, created_at DESC);

CREATE TABLE screenshot_regions (
  id             text PRIMARY KEY,
  screenshot_id  text NOT NULL REFERENCES screenshots(id) ON DELETE CASCADE,
  key_id         text NOT NULL REFERENCES translation_keys(id) ON DELETE CASCADE,
  translation_id text REFERENCES translations(id) ON DELETE SET NULL,
  x integer NOT NULL,
  y integer NOT NULL,
  w integer NOT NULL,
  h integer NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_screenshot_regions_key ON screenshot_regions (key_id);
CREATE INDEX idx_screenshot_regions_screenshot ON screenshot_regions (screenshot_id);

-- +goose Down
DROP TABLE IF EXISTS screenshot_regions;
DROP TABLE IF EXISTS screenshots;
