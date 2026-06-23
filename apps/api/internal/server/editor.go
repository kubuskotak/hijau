package server

import (
	"context"

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
