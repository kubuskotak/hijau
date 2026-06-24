-- name: ListGlossaryTerms :many
SELECT * FROM glossary_terms WHERE project_id = $1 ORDER BY term;

-- name: ListGlossaryTranslationsByProject :many
SELECT gt.* FROM glossary_translations gt
JOIN glossary_terms t ON t.id = gt.term_id
WHERE t.project_id = $1;

-- name: CreateGlossaryTerm :one
INSERT INTO glossary_terms (id, project_id, term, description, case_sensitive, do_not_translate)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetGlossaryTerm :one
SELECT * FROM glossary_terms WHERE id = $1;

-- name: DeleteGlossaryTerm :exec
DELETE FROM glossary_terms WHERE id = $1;

-- name: UpsertGlossaryTranslation :one
INSERT INTO glossary_translations (id, term_id, language_id, text)
VALUES ($1, $2, $3, $4)
ON CONFLICT (term_id, language_id) DO UPDATE SET text = EXCLUDED.text
RETURNING *;

-- name: MatchGlossary :many
SELECT t.term, t.do_not_translate, gt.text AS translation
FROM glossary_terms t
LEFT JOIN glossary_translations gt
  ON gt.term_id = t.id AND gt.language_id = sqlc.arg(target_language_id)
WHERE t.project_id = sqlc.arg(project_id)
  AND CASE WHEN t.case_sensitive
           THEN sqlc.arg(source_text) LIKE '%' || t.term || '%'
           ELSE sqlc.arg(source_text) ILIKE '%' || t.term || '%' END;
