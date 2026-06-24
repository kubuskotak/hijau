-- +goose Up

-- ---------- enums ----------
CREATE TYPE org_role AS ENUM ('owner', 'admin', 'member');
CREATE TYPE project_role AS ENUM ('owner', 'admin', 'developer', 'translator', 'reviewer');
CREATE TYPE translation_state AS ENUM ('untranslated', 'translated', 'reviewed', 'needs_work', 'outdated');
CREATE TYPE translation_origin AS ENUM ('human', 'machine_mt', 'machine_tm', 'import', 'auto');
CREATE TYPE author_kind AS ENUM ('user', 'api_key', 'system', 'mt');
CREATE TYPE api_key_type AS ENUM ('pat', 'project', 'editor');
CREATE TYPE activity_type AS ENUM (
  'key_created', 'key_updated', 'key_deleted',
  'translation_updated', 'translation_state_changed', 'source_changed',
  'comment_added', 'comment_resolved',
  'import_completed', 'export_run',
  'member_added', 'member_removed',
  'screenshot_added', 'auto_translate_run'
);
CREATE TYPE task_status AS ENUM ('queued', 'running', 'succeeded', 'failed');
CREATE TYPE task_type AS ENUM (
  'import', 'export', 'bulk_translation', 'auto_translate',
  'tm_reindex', 'screenshot_ocr', 'webhook_delivery'
);

-- ---------- identity & org ----------
CREATE TABLE users (
  id            text PRIMARY KEY,
  email         text NOT NULL UNIQUE,
  password_hash text,
  name          text,
  avatar_url    text,
  is_active     boolean NOT NULL DEFAULT true,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE organizations (
  id         text PRIMARY KEY,
  name       text NOT NULL,
  slug       text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE org_memberships (
  id         text PRIMARY KEY,
  org_id     text NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id    text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role       org_role NOT NULL DEFAULT 'member',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, user_id)
);
CREATE INDEX idx_org_memberships_user ON org_memberships (user_id);

-- ---------- project & languages ----------
-- base_language_id FK is added after `languages` exists (circular reference).
CREATE TABLE projects (
  id               text PRIMARY KEY,
  org_id           text NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name             text NOT NULL,
  slug             text NOT NULL,
  description      text,
  base_language_id text,
  icu_enabled      boolean NOT NULL DEFAULT true,
  auto_translate   boolean NOT NULL DEFAULT false,
  mt_provider      text,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz,
  UNIQUE (org_id, slug)
);

CREATE TABLE languages (
  id           text PRIMARY KEY,
  project_id   text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  tag          text NOT NULL,            -- BCP-47, e.g. "pt-BR"
  name         text NOT NULL,
  is_rtl       boolean NOT NULL DEFAULT false,
  plural_forms text[] NOT NULL DEFAULT '{}',  -- CLDR categories present, e.g. {one,other}
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, tag)
);

ALTER TABLE projects
  ADD CONSTRAINT fk_projects_base_language
  FOREIGN KEY (base_language_id) REFERENCES languages(id) ON DELETE SET NULL;

CREATE TABLE project_members (
  id         text PRIMARY KEY,
  project_id text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  user_id    text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role       project_role NOT NULL DEFAULT 'translator',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, user_id)
);
CREATE INDEX idx_project_members_user ON project_members (user_id);

-- language scoping for translators/reviewers; no rows = all languages
CREATE TABLE project_member_languages (
  member_id   text NOT NULL REFERENCES project_members(id) ON DELETE CASCADE,
  language_id text NOT NULL REFERENCES languages(id) ON DELETE CASCADE,
  PRIMARY KEY (member_id, language_id)
);

CREATE TABLE namespaces (
  id         text PRIMARY KEY,
  project_id text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name       text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, name)
);

-- ---------- keys & translations (the heart) ----------
CREATE TABLE translation_keys (
  id           text PRIMARY KEY,
  project_id   text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  namespace_id text REFERENCES namespaces(id) ON DELETE SET NULL,
  name         text NOT NULL,
  description  text,
  is_plural    boolean NOT NULL DEFAULT false,
  source_hash  text,                    -- hash of normalized base-language text (OUTDATED detection)
  tags         text[] NOT NULL DEFAULT '{}',
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz
);
-- unique key name within (project, namespace); NULL namespace treated as '' so
-- a key without a namespace is still uniquely named. Excludes soft-deleted rows.
CREATE UNIQUE INDEX uq_translation_keys_name
  ON translation_keys (project_id, COALESCE(namespace_id, ''), name)
  WHERE deleted_at IS NULL;
CREATE INDEX idx_translation_keys_project_updated ON translation_keys (project_id, updated_at);
CREATE INDEX idx_translation_keys_name_trgm ON translation_keys USING gin (name gin_trgm_ops);

CREATE TABLE translations (
  id          text PRIMARY KEY,
  key_id      text NOT NULL REFERENCES translation_keys(id) ON DELETE CASCADE,
  language_id text NOT NULL REFERENCES languages(id) ON DELETE CASCADE,
  text        text,                                   -- raw ICU; NULL when untranslated
  state       translation_state NOT NULL DEFAULT 'untranslated',
  origin      translation_origin NOT NULL DEFAULT 'human',
  is_machine  boolean NOT NULL DEFAULT false,
  sub_id      bigint GENERATED ALWAYS AS IDENTITY,    -- compact id encoded by the in-context marker codec
  version     integer NOT NULL DEFAULT 0,             -- optimistic concurrency
  updated_by  text REFERENCES users(id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (key_id, language_id)
);
CREATE UNIQUE INDEX uq_translations_subid ON translations (sub_id);
CREATE INDEX idx_translations_lang_state ON translations (language_id, state);
CREATE INDEX idx_translations_text_trgm ON translations USING gin (text gin_trgm_ops);

-- ---------- audit & collaboration ----------
CREATE TABLE translation_history (
  id             text PRIMARY KEY,
  translation_id text NOT NULL REFERENCES translations(id) ON DELETE CASCADE,
  key_id         text NOT NULL,                       -- denormalized for fast per-key history
  language_id    text NOT NULL,
  old_text       text,
  new_text       text,
  old_state      translation_state,
  new_state      translation_state,
  origin         translation_origin NOT NULL,
  author_kind    author_kind NOT NULL DEFAULT 'user',
  author_id      text REFERENCES users(id) ON DELETE SET NULL,
  api_key_id     text,
  created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_history_key_created ON translation_history (key_id, created_at DESC);
CREATE INDEX idx_history_translation_created ON translation_history (translation_id, created_at DESC);

CREATE TABLE activity (
  id             text PRIMARY KEY,
  project_id     text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  type           activity_type NOT NULL,
  actor_id       text REFERENCES users(id) ON DELETE SET NULL,
  actor_kind     author_kind NOT NULL DEFAULT 'user',
  key_id         text,
  translation_id text,
  language_id    text,
  meta           jsonb,
  created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_activity_project_created ON activity (project_id, created_at DESC);

CREATE TABLE comments (
  id             text PRIMARY KEY,
  translation_id text REFERENCES translations(id) ON DELETE CASCADE,
  key_id         text REFERENCES translation_keys(id) ON DELETE CASCADE,
  author_id      text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  body           text NOT NULL,
  parent_id      text REFERENCES comments(id) ON DELETE CASCADE,
  resolved_at    timestamptz,
  resolved_by    text REFERENCES users(id) ON DELETE SET NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  CHECK (translation_id IS NOT NULL OR key_id IS NOT NULL)
);
CREATE INDEX idx_comments_translation ON comments (translation_id);
CREATE INDEX idx_comments_key ON comments (key_id);
CREATE INDEX idx_comments_parent ON comments (parent_id);

-- ---------- platform: sessions, api keys, async tasks ----------
CREATE TABLE sessions (
  id         text PRIMARY KEY,
  user_id    text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash text NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  user_agent text,
  ip         text,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_sessions_user ON sessions (user_id);

CREATE TABLE api_keys (
  id            text PRIMARY KEY,
  type          api_key_type NOT NULL,
  name          text NOT NULL,
  key_hash      text NOT NULL UNIQUE,    -- sha256 of the raw token
  prefix        text NOT NULL,           -- first chars shown in UI for identification
  scopes        text[] NOT NULL DEFAULT '{}',
  owner_user_id text REFERENCES users(id) ON DELETE CASCADE,   -- for PAT
  project_id    text REFERENCES projects(id) ON DELETE CASCADE, -- for PROJECT / EDITOR
  expires_at    timestamptz,
  last_used_at  timestamptz,
  revoked_at    timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_api_keys_project ON api_keys (project_id);
CREATE INDEX idx_api_keys_owner ON api_keys (owner_user_id);

CREATE TABLE tasks (
  id          text PRIMARY KEY,
  project_id  text REFERENCES projects(id) ON DELETE CASCADE,
  type        task_type NOT NULL,
  status      task_status NOT NULL DEFAULT 'queued',
  progress    integer NOT NULL DEFAULT 0,
  total       integer,
  processed   integer,
  result      jsonb,
  error       text,
  created_by  text REFERENCES users(id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  started_at  timestamptz,
  finished_at timestamptz
);
CREATE INDEX idx_tasks_project_status ON tasks (project_id, status);

-- +goose Down
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS activity;
DROP TABLE IF EXISTS translation_history;
DROP TABLE IF EXISTS translations;
DROP TABLE IF EXISTS translation_keys;
DROP TABLE IF EXISTS namespaces;
DROP TABLE IF EXISTS project_member_languages;
DROP TABLE IF EXISTS project_members;
ALTER TABLE projects DROP CONSTRAINT IF EXISTS fk_projects_base_language;
DROP TABLE IF EXISTS languages;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS org_memberships;
DROP TABLE IF EXISTS organizations;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS task_type;
DROP TYPE IF EXISTS task_status;
DROP TYPE IF EXISTS activity_type;
DROP TYPE IF EXISTS api_key_type;
DROP TYPE IF EXISTS author_kind;
DROP TYPE IF EXISTS translation_origin;
DROP TYPE IF EXISTS translation_state;
DROP TYPE IF EXISTS project_role;
DROP TYPE IF EXISTS org_role;
