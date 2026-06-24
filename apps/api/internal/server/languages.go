package server

import (
	"context"
	"strings"

	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/db"
	"github.com/portierglobal/hijau/apps/api/internal/id"
)

type languageDTO struct {
	ID          string   `json:"id"`
	Tag         string   `json:"tag"`
	Name        string   `json:"name"`
	IsRtl       bool     `json:"isRtl"`
	PluralForms []string `json:"pluralForms"`
}

type createLanguageReq struct {
	Tag         string   `json:"tag"`
	Name        string   `json:"name"`
	IsRtl       bool     `json:"isRtl"`
	PluralForms []string `json:"pluralForms"`
}

type setBaseLanguageReq struct {
	LanguageID string `json:"languageId"`
}

func toLanguageDTO(l db.Language) languageDTO {
	return languageDTO{ID: l.ID, Tag: l.Tag, Name: l.Name, IsRtl: l.IsRtl, PluralForms: l.PluralForms}
}

func (s *Server) listLanguages(ctx context.Context, path *extractor.Path[projectPath]) (espresso.JSON[[]languageDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectRead, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[[]languageDTO]{}, err
	}
	langs, err := s.store.ListLanguages(ctx, pid)
	if err != nil {
		return espresso.JSON[[]languageDTO]{}, espresso.ErrInternal("could not list languages")
	}
	out := make([]languageDTO, 0, len(langs))
	for _, l := range langs {
		out = append(out, toLanguageDTO(l))
	}
	return espresso.JSON[[]languageDTO]{Data: out}, nil
}

func (s *Server) createLanguage(ctx context.Context, path *extractor.Path[projectPath], body *espresso.JSON[createLanguageReq]) (espresso.JSON[languageDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectWrite, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[languageDTO]{}, err
	}
	in := body.Data
	in.Tag = strings.TrimSpace(in.Tag)
	in.Name = strings.TrimSpace(in.Name)
	if in.Tag == "" || in.Name == "" {
		return espresso.JSON[languageDTO]{}, espresso.ErrBadRequest("tag and name are required")
	}
	if in.PluralForms == nil {
		in.PluralForms = []string{}
	}

	langID := id.New()
	var lang db.Language
	err := s.store.WithTx(ctx, func(q *db.Queries) error {
		var e error
		lang, e = q.CreateLanguage(ctx, db.CreateLanguageParams{
			ID: langID, ProjectID: pid, Tag: in.Tag, Name: in.Name, IsRtl: in.IsRtl, PluralForms: in.PluralForms,
		})
		if e != nil {
			return e
		}
		proj, e := q.GetProject(ctx, pid)
		if e != nil {
			return e
		}
		if !proj.BaseLanguageID.Valid {
			if e = q.SetProjectBaseLanguage(ctx, db.SetProjectBaseLanguageParams{ID: pid, BaseLanguageID: pgText(langID)}); e != nil {
				return e
			}
		}
		// Eager-create untranslated rows for every existing key so each cell has
		// a stable sub_id (used by the in-context marker codec).
		keys, e := q.ListKeys(ctx, db.ListKeysParams{ProjectID: pid, Lim: 1_000_000, Off: 0})
		if e != nil {
			return e
		}
		for _, k := range keys {
			if _, e = q.CreateTranslation(ctx, db.CreateTranslationParams{
				ID: id.New(), KeyID: k.ID, LanguageID: langID,
				State: db.TranslationStateUntranslated, Origin: db.TranslationOriginHuman,
			}); e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			return espresso.JSON[languageDTO]{}, espresso.ErrConflict("that language tag already exists in the project")
		}
		return espresso.JSON[languageDTO]{}, espresso.ErrInternal("could not create language")
	}
	return espresso.JSON[languageDTO]{StatusCode: 201, Data: toLanguageDTO(lang)}, nil
}

func (s *Server) setBaseLanguage(ctx context.Context, path *extractor.Path[projectPath], body *espresso.JSON[setBaseLanguageReq]) (espresso.JSON[okDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectWrite, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[okDTO]{}, err
	}
	lang, err := s.store.GetLanguage(ctx, body.Data.LanguageID)
	if err != nil || lang.ProjectID != pid {
		return espresso.JSON[okDTO]{}, espresso.ErrBadRequest("language does not belong to this project")
	}
	if err := s.store.SetProjectBaseLanguage(ctx, db.SetProjectBaseLanguageParams{ID: pid, BaseLanguageID: pgText(lang.ID)}); err != nil {
		return espresso.JSON[okDTO]{}, espresso.ErrInternal("could not set base language")
	}
	return espresso.JSON[okDTO]{Data: okDTO{OK: true}}, nil
}
