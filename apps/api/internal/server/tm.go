package server

import (
	"context"
	"math"

	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/db"
	"github.com/portierglobal/hijau/apps/api/internal/i18n"
)

type tmSuggestReq struct {
	TargetLang string `json:"targetLang"`
}

type tmMatchDTO struct {
	SourceText string `json:"sourceText"`
	TargetText string `json:"targetText"`
	Score      int    `json:"score"` // 0-100; 100 = exact source match
	Exact      bool   `json:"exact"`
}

// tmSuggest returns translation-memory matches for a key's base text in the
// target language: exact source matches first (score 100), then trigram-fuzzy
// matches ordered by similarity.
func (s *Server) tmSuggest(ctx context.Context, path *extractor.Path[keyPath], body *espresso.JSON[tmSuggestReq]) (espresso.JSON[[]tmMatchDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermTranslationsRead, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[[]tmMatchDTO]{}, err
	}
	if body.Data.TargetLang == "" {
		return espresso.JSON[[]tmMatchDTO]{}, espresso.ErrBadRequest("targetLang is required")
	}
	key, err := s.store.GetKey(ctx, path.Data.KID)
	if err != nil || key.ProjectID != pid {
		return espresso.JSON[[]tmMatchDTO]{}, espresso.ErrNotFound("key not found")
	}
	proj, err := s.store.GetProject(ctx, pid)
	if err != nil || proj.BaseLanguageID.String == "" {
		return espresso.JSON[[]tmMatchDTO]{}, espresso.ErrPreconditionFailed("the project has no base language")
	}
	baseLang, err := s.store.GetLanguage(ctx, proj.BaseLanguageID.String)
	if err != nil {
		return espresso.JSON[[]tmMatchDTO]{}, espresso.ErrInternal("base language lookup failed")
	}

	var source string
	trs, err := s.store.ListTranslationsForKey(ctx, key.ID)
	if err != nil {
		return espresso.JSON[[]tmMatchDTO]{}, espresso.ErrInternal("could not load source text")
	}
	for _, t := range trs {
		if t.LanguageID == proj.BaseLanguageID.String {
			source = t.Text.String
		}
	}
	out := make([]tmMatchDTO, 0)
	if source == "" {
		return espresso.JSON[[]tmMatchDTO]{Data: out}, nil
	}

	seen := map[string]bool{}
	exact, err := s.store.FindTMExact(ctx, db.FindTMExactParams{
		ProjectID: pid, SourceLang: baseLang.Tag, TargetLang: body.Data.TargetLang,
		SourceHash: i18n.SourceHash(source),
	})
	if err != nil {
		return espresso.JSON[[]tmMatchDTO]{}, espresso.ErrInternal("translation memory lookup failed")
	}
	for _, e := range exact {
		if seen[e.TargetText] {
			continue
		}
		seen[e.TargetText] = true
		out = append(out, tmMatchDTO{SourceText: e.SourceText, TargetText: e.TargetText, Score: 100, Exact: true})
	}

	fuzzy, err := s.store.FindTMFuzzy(ctx, db.FindTMFuzzyParams{
		Query: source, ProjectID: pid, SourceLang: baseLang.Tag, TargetLang: body.Data.TargetLang, Lim: 5,
	})
	if err != nil {
		return espresso.JSON[[]tmMatchDTO]{}, espresso.ErrInternal("translation memory lookup failed")
	}
	for _, f := range fuzzy {
		if seen[f.TargetText] {
			continue
		}
		seen[f.TargetText] = true
		out = append(out, tmMatchDTO{
			SourceText: f.SourceText, TargetText: f.TargetText,
			Score: int(math.Round(f.Score * 100)), Exact: false,
		})
	}
	return espresso.JSON[[]tmMatchDTO]{Data: out}, nil
}
