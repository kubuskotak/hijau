-- +goose Up
-- Glossary: approved source terms (optionally do-not-translate) with per-language
-- translations. Injected into MT prompts to steer terminology, and surfaced in
-- the editor.
CREATE TABLE glossary_terms (
  id               text PRIMARY KEY,
  project_id       text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  term             text NOT NULL,
  description      text NOT NULL DEFAULT '',
  case_sensitive   boolean NOT NULL DEFAULT false,
  do_not_translate boolean NOT NULL DEFAULT false,
  created_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, term)
);
CREATE INDEX idx_glossary_terms_project ON glossary_terms (project_id);

CREATE TABLE glossary_translations (
  id          text PRIMARY KEY,
  term_id     text NOT NULL REFERENCES glossary_terms(id) ON DELETE CASCADE,
  language_id text NOT NULL REFERENCES languages(id) ON DELETE CASCADE,
  text        text NOT NULL,
  UNIQUE (term_id, language_id)
);

-- +goose Down
DROP TABLE IF EXISTS glossary_translations;
DROP TABLE IF EXISTS glossary_terms;
