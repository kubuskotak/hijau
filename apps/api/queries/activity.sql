-- name: ListActivityFeed :many
-- Recent project activity enriched for display. key_id/language_id are plain
-- text (no FK), so LEFT JOINs tolerate deleted keys/languages.
SELECT a.id, a.type, a.actor_kind, a.created_at,
       u.email AS actor_email,
       k.name  AS key_name,
       l.tag   AS language_tag
FROM activity a
LEFT JOIN users u ON u.id = a.actor_id
LEFT JOIN translation_keys k ON k.id = a.key_id
LEFT JOIN languages l ON l.id = a.language_id
WHERE a.project_id = $1
ORDER BY a.created_at DESC
LIMIT $2;
