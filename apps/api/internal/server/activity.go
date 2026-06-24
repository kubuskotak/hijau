package server

import (
	"context"
	"time"

	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/db"
)

type activityQuery struct {
	Limit int `query:"limit"`
}

type activityDTO struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	ActorKind   string `json:"actorKind"`
	ActorEmail  string `json:"actorEmail"`
	KeyName     string `json:"keyName"`
	LanguageTag string `json:"languageTag"`
	CreatedAt   string `json:"createdAt"`
}

// listActivity returns the project's recent activity feed (newest first).
func (s *Server) listActivity(ctx context.Context, path *extractor.Path[projectPath], q *extractor.Query[activityQuery]) (espresso.JSON[[]activityDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectRead, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[[]activityDTO]{}, err
	}
	limit := q.Data.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.store.ListActivityFeed(ctx, db.ListActivityFeedParams{ProjectID: pid, Limit: int32(limit)})
	if err != nil {
		return espresso.JSON[[]activityDTO]{}, espresso.ErrInternal("could not load activity")
	}
	out := make([]activityDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, activityDTO{
			ID: r.ID, Type: string(r.Type), ActorKind: string(r.ActorKind),
			ActorEmail: r.ActorEmail.String, KeyName: r.KeyName.String, LanguageTag: r.LanguageTag.String,
			CreatedAt: r.CreatedAt.Time.UTC().Format(time.RFC3339),
		})
	}
	return espresso.JSON[[]activityDTO]{Data: out}, nil
}
