package server

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/db"
)

type editorQuery struct {
	NamespaceID string `query:"namespaceId"`
	Search      string `query:"search"`
	Limit       int    `query:"limit"`
	Offset      int    `query:"offset"`
}

// editorRowDTO is a key plus its translations keyed by language id.
type editorRowDTO struct {
	keyDTO
	Translations map[string]translationDTO `json:"translations"`
}

type editorFeedDTO struct {
	Keys  []editorRowDTO `json:"keys"`
	Total int64          `json:"total"`
}

// editorFeed returns a page of keys with their translations across all
// languages — the data backing the translation editor grid.
func (s *Server) editorFeed(ctx context.Context, path *extractor.Path[projectPath], q *extractor.Query[editorQuery]) (espresso.JSON[editorFeedDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectRead, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[editorFeedDTO]{}, err
	}
	qp := q.Data
	limit := qp.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	params := db.ListKeysParams{ProjectID: pid, Lim: int32(limit), Off: int32(qp.Offset)}
	if qp.NamespaceID != "" {
		params.NamespaceID = pgtype.Text{String: qp.NamespaceID, Valid: true}
	}
	if qp.Search != "" {
		params.Search = pgtype.Text{String: qp.Search, Valid: true}
	}

	keys, err := s.store.ListKeys(ctx, params)
	if err != nil {
		return espresso.JSON[editorFeedDTO]{}, espresso.ErrInternal("could not load keys")
	}
	total, err := s.store.CountKeys(ctx, pid)
	if err != nil {
		return espresso.JSON[editorFeedDTO]{}, espresso.ErrInternal("could not count keys")
	}

	ids := make([]string, len(keys))
	for i, k := range keys {
		ids[i] = k.ID
	}
	byKey := make(map[string]map[string]translationDTO, len(keys))
	if len(ids) > 0 {
		trs, err := s.store.ListTranslationsForKeys(ctx, ids)
		if err != nil {
			return espresso.JSON[editorFeedDTO]{}, espresso.ErrInternal("could not load translations")
		}
		for _, t := range trs {
			m := byKey[t.KeyID]
			if m == nil {
				m = make(map[string]translationDTO)
				byKey[t.KeyID] = m
			}
			m[t.LanguageID] = toTranslationDTO(t)
		}
	}

	rows := make([]editorRowDTO, 0, len(keys))
	for _, k := range keys {
		tr := byKey[k.ID]
		if tr == nil {
			tr = map[string]translationDTO{}
		}
		rows = append(rows, editorRowDTO{keyDTO: toKeyDTO(k), Translations: tr})
	}
	return espresso.JSON[editorFeedDTO]{Data: editorFeedDTO{Keys: rows, Total: total}}, nil
}

type subIDPath struct {
	PID string `path:"pid"`
	N   string `path:"n"`
}

// editContextDTO is everything the in-context overlay editor needs once it has
// decoded a marker: the translation, its key, the language, and the source
// (base-language) text shown for reference.
type editContextDTO struct {
	Translation translationDTO `json:"translation"`
	Key         keyDTO         `json:"key"`
	Language    languageDTO    `json:"language"`
	SourceText  string         `json:"sourceText"`
}

// resolveBySubID maps a marker-decoded sub_id back to its full edit context.
// It is scoped to the project so an editor token for project A can't read
// project B's strings by guessing sub_ids (sub_id is globally unique, so the
// project check is the boundary).
func (s *Server) resolveBySubID(ctx context.Context, path *extractor.Path[subIDPath]) (espresso.JSON[editContextDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermTranslationsRead, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[editContextDTO]{}, err
	}
	n, err := strconv.ParseInt(path.Data.N, 10, 64)
	if err != nil || n < 0 {
		return espresso.JSON[editContextDTO]{}, espresso.ErrBadRequest("sub id must be a non-negative integer")
	}
	tr, err := s.store.GetTranslationBySubID(ctx, pgtype.Int8{Int64: n, Valid: true})
	if err != nil {
		return espresso.JSON[editContextDTO]{}, espresso.ErrNotFound("translation not found")
	}
	key, err := s.store.GetKey(ctx, tr.KeyID)
	if err != nil || key.ProjectID != pid {
		return espresso.JSON[editContextDTO]{}, espresso.ErrNotFound("translation not found")
	}
	lang, err := s.store.GetLanguage(ctx, tr.LanguageID)
	if err != nil {
		return espresso.JSON[editContextDTO]{}, espresso.ErrInternal("language lookup failed")
	}

	sourceText := ""
	if proj, err := s.store.GetProject(ctx, pid); err == nil && proj.BaseLanguageID.String != "" {
		baseID := proj.BaseLanguageID.String
		if trs, err := s.store.ListTranslationsForKey(ctx, key.ID); err == nil {
			for _, t := range trs {
				if t.LanguageID == baseID {
					sourceText = t.Text.String
					break
				}
			}
		}
	}

	return espresso.JSON[editContextDTO]{Data: editContextDTO{
		Translation: toTranslationDTO(tr),
		Key:         toKeyDTO(key),
		Language:    toLanguageDTO(lang),
		SourceText:  sourceText,
	}}, nil
}
