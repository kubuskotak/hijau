package server

import (
	"context"
	"errors"
	"time"

	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/db"
	"github.com/portierglobal/hijau/apps/api/internal/service"
)

type transPath struct {
	PID  string `path:"pid"`
	KID  string `path:"kid"`
	Lang string `path:"lang"`
}

type setTextReq struct {
	Text string `json:"text"`
}

type transitionReq struct {
	Action string `json:"action"` // "approve" | "reject"
}

type translationDTO struct {
	ID         string `json:"id"`
	KeyID      string `json:"keyId"`
	LanguageID string `json:"languageId"`
	Text       string `json:"text"`
	State      string `json:"state"`
	Origin     string `json:"origin"`
	IsMachine  bool   `json:"isMachine"`
	SubID      int64  `json:"subId"`
	Version    int32  `json:"version"`
	UpdatedAt  string `json:"updatedAt"`
}

func toTranslationDTO(t db.Translation) translationDTO {
	return translationDTO{
		ID: t.ID, KeyID: t.KeyID, LanguageID: t.LanguageID,
		Text: t.Text.String, State: string(t.State), Origin: string(t.Origin),
		IsMachine: t.IsMachine, SubID: t.SubID.Int64, Version: t.Version,
		UpdatedAt: t.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}
}

func principalActorKind(p auth.Principal) db.AuthorKind {
	if p.Kind == auth.APIKeyPrincipal {
		return db.AuthorKindApiKey
	}
	return db.AuthorKindUser
}

func actorOf(p auth.Principal) service.Actor {
	return service.Actor{Kind: principalActorKind(p), UserID: p.UserID, APIKeyID: p.APIKeyID}
}

// loadKeyLang loads and cross-checks the key and language for a project, and
// returns the project's base language id. Errors are espresso HTTP errors.
func (s *Server) loadKeyLang(ctx context.Context, pid, kid, langTag string) (db.TranslationKey, db.Language, string, error) {
	key, err := s.store.GetKey(ctx, kid)
	if err != nil || key.ProjectID != pid {
		return db.TranslationKey{}, db.Language{}, "", espresso.ErrNotFound("key not found")
	}
	lang, err := s.store.GetLanguageByTag(ctx, db.GetLanguageByTagParams{ProjectID: pid, Tag: langTag})
	if err != nil {
		return db.TranslationKey{}, db.Language{}, "", espresso.ErrNotFound("language not found")
	}
	proj, err := s.store.GetProject(ctx, pid)
	if err != nil {
		return db.TranslationKey{}, db.Language{}, "", espresso.ErrInternal("project lookup failed")
	}
	return key, lang, proj.BaseLanguageID.String, nil
}

func (s *Server) listKeyTranslations(ctx context.Context, path *extractor.Path[keyPath]) (espresso.JSON[[]translationDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermTranslationsRead, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[[]translationDTO]{}, err
	}
	key, err := s.store.GetKey(ctx, path.Data.KID)
	if err != nil || key.ProjectID != pid {
		return espresso.JSON[[]translationDTO]{}, espresso.ErrNotFound("key not found")
	}
	trs, err := s.store.ListTranslationsForKey(ctx, key.ID)
	if err != nil {
		return espresso.JSON[[]translationDTO]{}, espresso.ErrInternal("could not list translations")
	}
	out := make([]translationDTO, 0, len(trs))
	for _, t := range trs {
		out = append(out, toTranslationDTO(t))
	}
	return espresso.JSON[[]translationDTO]{Data: out}, nil
}

func (s *Server) setTranslation(ctx context.Context, path *extractor.Path[transPath], body *espresso.JSON[setTextReq]) (espresso.JSON[translationDTO], error) {
	d := path.Data
	key, lang, baseID, err := s.loadKeyLang(ctx, d.PID, d.KID, d.Lang)
	if err != nil {
		return espresso.JSON[translationDTO]{}, err
	}
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermTranslationsWrite, auth.Check{ProjectID: d.PID, LanguageID: lang.ID})); err != nil {
		return espresso.JSON[translationDTO]{}, err
	}
	res, err := service.SetTranslation(ctx, s.store, service.SetTranslationInput{
		Key: key, Language: lang, BaseLanguageID: baseID, Text: body.Data.Text,
		Action: service.SetText, Actor: actorOf(auth.FromContext(ctx)),
	})
	if err != nil {
		return espresso.JSON[translationDTO]{}, mapServiceErr(err)
	}
	return espresso.JSON[translationDTO]{Data: toTranslationDTO(res.Translation)}, nil
}

func (s *Server) transitionTranslation(ctx context.Context, path *extractor.Path[transPath], body *espresso.JSON[transitionReq]) (espresso.JSON[translationDTO], error) {
	d := path.Data
	key, lang, baseID, err := s.loadKeyLang(ctx, d.PID, d.KID, d.Lang)
	if err != nil {
		return espresso.JSON[translationDTO]{}, err
	}
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermReview, auth.Check{ProjectID: d.PID, LanguageID: lang.ID})); err != nil {
		return espresso.JSON[translationDTO]{}, err
	}
	var action service.Action
	switch body.Data.Action {
	case "approve":
		action = service.Approve
	case "reject":
		action = service.Reject
	default:
		return espresso.JSON[translationDTO]{}, espresso.ErrBadRequest("action must be 'approve' or 'reject'")
	}
	res, err := service.SetTranslation(ctx, s.store, service.SetTranslationInput{
		Key: key, Language: lang, BaseLanguageID: baseID,
		Action: action, Actor: actorOf(auth.FromContext(ctx)),
	})
	if err != nil {
		return espresso.JSON[translationDTO]{}, mapServiceErr(err)
	}
	return espresso.JSON[translationDTO]{Data: toTranslationDTO(res.Translation)}, nil
}

func mapServiceErr(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalid):
		return espresso.ErrBadRequest(err.Error())
	case errors.Is(err, service.ErrConflict):
		return espresso.ErrPreconditionFailed("the translation was modified concurrently")
	default:
		return espresso.ErrInternal("could not save translation")
	}
}
