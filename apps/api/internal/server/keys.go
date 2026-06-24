package server

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/db"
	"github.com/portierglobal/hijau/apps/api/internal/id"
)

type keyPath struct {
	PID string `path:"pid"`
	KID string `path:"kid"`
}

type listKeysQuery struct {
	NamespaceID string `query:"namespaceId"`
	Search      string `query:"search"`
	Limit       int    `query:"limit"`
	Offset      int    `query:"offset"`
}

type createKeyReq struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Description string `json:"description"`
	IsPlural    bool   `json:"isPlural"`
}

type keyDTO struct {
	ID          string   `json:"id"`
	ProjectID   string   `json:"projectId"`
	NamespaceID string   `json:"namespaceId"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	IsPlural    bool     `json:"isPlural"`
	Tags        []string `json:"tags"`
	CreatedAt   string   `json:"createdAt"`
}

func toKeyDTO(k db.TranslationKey) keyDTO {
	return keyDTO{
		ID: k.ID, ProjectID: k.ProjectID, NamespaceID: k.NamespaceID.String,
		Name: k.Name, Description: k.Description.String, IsPlural: k.IsPlural,
		Tags: k.Tags, CreatedAt: k.CreatedAt.Time.UTC().Format(time.RFC3339),
	}
}

func (s *Server) listKeys(ctx context.Context, path *extractor.Path[projectPath], q *extractor.Query[listKeysQuery]) (espresso.JSON[[]keyDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectRead, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[[]keyDTO]{}, err
	}
	qp := q.Data
	limit := qp.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
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
		return espresso.JSON[[]keyDTO]{}, espresso.ErrInternal("could not list keys")
	}
	out := make([]keyDTO, 0, len(keys))
	for _, k := range keys {
		out = append(out, toKeyDTO(k))
	}
	return espresso.JSON[[]keyDTO]{Data: out}, nil
}

func (s *Server) createKey(ctx context.Context, path *extractor.Path[projectPath], body *espresso.JSON[createKeyReq]) (espresso.JSON[keyDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermKeysWrite, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[keyDTO]{}, err
	}
	in := body.Data
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return espresso.JSON[keyDTO]{}, espresso.ErrBadRequest("name is required")
	}
	p := auth.FromContext(ctx)

	var key db.TranslationKey
	err := s.store.WithTx(ctx, func(q *db.Queries) error {
		var nsID pgtype.Text
		if ns, ok, e := resolveNamespaceID(ctx, q, pid, in.Namespace); e != nil {
			return e
		} else if ok {
			nsID = pgtype.Text{String: ns.ID, Valid: true}
		}
		var e error
		key, e = q.CreateKey(ctx, db.CreateKeyParams{
			ID: id.New(), ProjectID: pid, NamespaceID: nsID, Name: in.Name,
			Description: pgText(in.Description), IsPlural: in.IsPlural,
		})
		if e != nil {
			return e
		}
		langs, e := q.ListLanguages(ctx, pid)
		if e != nil {
			return e
		}
		for _, l := range langs {
			if _, e = q.CreateTranslation(ctx, db.CreateTranslationParams{
				ID: id.New(), KeyID: key.ID, LanguageID: l.ID,
				State: db.TranslationStateUntranslated, Origin: db.TranslationOriginHuman,
			}); e != nil {
				return e
			}
		}
		return q.InsertActivity(ctx, db.InsertActivityParams{
			ID: id.New(), ProjectID: pid, Type: db.ActivityTypeKeyCreated,
			ActorID: pgText(p.UserID), ActorKind: principalActorKind(p), KeyID: pgText(key.ID),
		})
	})
	if err != nil {
		if isUniqueViolation(err) {
			return espresso.JSON[keyDTO]{}, espresso.ErrConflict("a key with that name already exists in the namespace")
		}
		return espresso.JSON[keyDTO]{}, espresso.ErrInternal("could not create key")
	}
	return espresso.JSON[keyDTO]{StatusCode: 201, Data: toKeyDTO(key)}, nil
}

func (s *Server) deleteKey(ctx context.Context, path *extractor.Path[keyPath]) (espresso.JSON[okDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermKeysWrite, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[okDTO]{}, err
	}
	key, err := s.store.GetKey(ctx, path.Data.KID)
	if err != nil || key.ProjectID != pid {
		return espresso.JSON[okDTO]{}, espresso.ErrNotFound("key not found")
	}
	if err := s.store.SoftDeleteKey(ctx, key.ID); err != nil {
		return espresso.JSON[okDTO]{}, espresso.ErrInternal("could not delete key")
	}
	return espresso.JSON[okDTO]{Data: okDTO{OK: true}}, nil
}
