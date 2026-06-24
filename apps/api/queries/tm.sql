-- name: InsertTMSegment :exec
INSERT INTO tm_segments (id, project_id, source_lang, target_lang, source_text, target_text, source_hash, key_id, origin)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (project_id, source_lang, target_lang, source_hash, target_text) DO NOTHING;

-- name: FindTMExact :many
SELECT id, source_text, target_text
FROM tm_segments
WHERE project_id = sqlc.arg(project_id)
  AND source_lang = sqlc.arg(source_lang)
  AND target_lang = sqlc.arg(target_lang)
  AND source_hash = sqlc.arg(source_hash)
LIMIT 5;

-- name: FindTMFuzzy :many
SELECT id, source_text, target_text,
       (similarity(lower(source_text), lower(sqlc.arg(query))))::float8 AS score
FROM tm_segments
WHERE project_id = sqlc.arg(project_id)
  AND source_lang = sqlc.arg(source_lang)
  AND target_lang = sqlc.arg(target_lang)
  AND source_text <> sqlc.arg(query)
  AND lower(source_text) % lower(sqlc.arg(query))
ORDER BY score DESC
LIMIT sqlc.arg(lim);
