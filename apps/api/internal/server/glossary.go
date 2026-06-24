package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/db"
	"github.com/portierglobal/hijau/apps/api/internal/id"
	"github.com/portierglobal/hijau/apps/api/internal/mt"
)

type glossaryTermDTO struct {
	ID             string            `json:"id"`
	Term           string            `json:"term"`
	Description    string            `json:"description"`
	CaseSensitive  bool              `json:"caseSensitive"`
	DoNotTranslate bool              `json:"doNotTranslate"`
	Translations   map[string]string `json:"translations"` // languageId -> text
}

func (s *Server) listGlossary(ctx context.Context, path *extractor.Path[projectPath]) (espresso.JSON[[]glossaryTermDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectRead, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[[]glossaryTermDTO]{}, err
	}
	terms, err := s.store.ListGlossaryTerms(ctx, pid)
	if err != nil {
		return espresso.JSON[[]glossaryTermDTO]{}, espresso.ErrInternal("could not list glossary")
	}
	trs, err := s.store.ListGlossaryTranslationsByProject(ctx, pid)
	if err != nil {
		return espresso.JSON[[]glossaryTermDTO]{}, espresso.ErrInternal("could not load glossary translations")
	}
	byTerm := map[string]map[string]string{}
	for _, t := range trs {
		m := byTerm[t.TermID]
		if m == nil {
			m = map[string]string{}
			byTerm[t.TermID] = m
		}
		m[t.LanguageID] = t.Text
	}
	out := make([]glossaryTermDTO, 0, len(terms))
	for _, t := range terms {
		tr := byTerm[t.ID]
		if tr == nil {
			tr = map[string]string{}
		}
		out = append(out, glossaryTermDTO{
			ID: t.ID, Term: t.Term, Description: t.Description,
			CaseSensitive: t.CaseSensitive, DoNotTranslate: t.DoNotTranslate, Translations: tr,
		})
	}
	return espresso.JSON[[]glossaryTermDTO]{Data: out}, nil
}

type createGlossaryReq struct {
	Term           string `json:"term"`
	Description    string `json:"description"`
	CaseSensitive  bool   `json:"caseSensitive"`
	DoNotTranslate bool   `json:"doNotTranslate"`
}

func (s *Server) createGlossaryTerm(ctx context.Context, path *extractor.Path[projectPath], body *espresso.JSON[createGlossaryReq]) (espresso.JSON[glossaryTermDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectWrite, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[glossaryTermDTO]{}, err
	}
	term := strings.TrimSpace(body.Data.Term)
	if term == "" {
		return espresso.JSON[glossaryTermDTO]{}, espresso.ErrBadRequest("term is required")
	}
	t, err := s.store.CreateGlossaryTerm(ctx, db.CreateGlossaryTermParams{
		ID: id.New(), ProjectID: pid, Term: term, Description: body.Data.Description,
		CaseSensitive: body.Data.CaseSensitive, DoNotTranslate: body.Data.DoNotTranslate,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return espresso.JSON[glossaryTermDTO]{}, espresso.ErrConflict("that term already exists")
		}
		return espresso.JSON[glossaryTermDTO]{}, espresso.ErrInternal("could not create term")
	}
	return espresso.JSON[glossaryTermDTO]{StatusCode: http.StatusCreated, Data: glossaryTermDTO{
		ID: t.ID, Term: t.Term, Description: t.Description,
		CaseSensitive: t.CaseSensitive, DoNotTranslate: t.DoNotTranslate, Translations: map[string]string{},
	}}, nil
}

type glossaryTermPath struct {
	PID    string `path:"pid"`
	TermID string `path:"termId"`
}

func (s *Server) deleteGlossaryTerm(ctx context.Context, path *extractor.Path[glossaryTermPath]) (espresso.JSON[okDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectWrite, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[okDTO]{}, err
	}
	t, err := s.store.GetGlossaryTerm(ctx, path.Data.TermID)
	if err != nil || t.ProjectID != pid {
		return espresso.JSON[okDTO]{}, espresso.ErrNotFound("term not found")
	}
	if err := s.store.DeleteGlossaryTerm(ctx, t.ID); err != nil {
		return espresso.JSON[okDTO]{}, espresso.ErrInternal("could not delete term")
	}
	return espresso.JSON[okDTO]{Data: okDTO{OK: true}}, nil
}

type glossaryTransPath struct {
	PID    string `path:"pid"`
	TermID string `path:"termId"`
	Lang   string `path:"lang"`
}

type glossaryTransReq struct {
	Text string `json:"text"`
}

func (s *Server) setGlossaryTranslation(ctx context.Context, path *extractor.Path[glossaryTransPath], body *espresso.JSON[glossaryTransReq]) (espresso.JSON[okDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectWrite, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[okDTO]{}, err
	}
	t, err := s.store.GetGlossaryTerm(ctx, path.Data.TermID)
	if err != nil || t.ProjectID != pid {
		return espresso.JSON[okDTO]{}, espresso.ErrNotFound("term not found")
	}
	lang, err := s.store.GetLanguageByTag(ctx, db.GetLanguageByTagParams{ProjectID: pid, Tag: path.Data.Lang})
	if err != nil {
		return espresso.JSON[okDTO]{}, espresso.ErrNotFound("language not found")
	}
	if _, err := s.store.UpsertGlossaryTranslation(ctx, db.UpsertGlossaryTranslationParams{
		ID: id.New(), TermID: t.ID, LanguageID: lang.ID, Text: body.Data.Text,
	}); err != nil {
		return espresso.JSON[okDTO]{}, espresso.ErrInternal("could not save translation")
	}
	return espresso.JSON[okDTO]{Data: okDTO{OK: true}}, nil
}

// buildGlossaryHints finds glossary terms present in the source text and turns
// them into MT hints: do-not-translate terms, or the approved target-language
// translation when one exists.
func (s *Server) buildGlossaryHints(ctx context.Context, pid, targetLangID, source string) []mt.GlossaryHint {
	rows, err := s.store.MatchGlossary(ctx, db.MatchGlossaryParams{
		TargetLanguageID: targetLangID, ProjectID: pid, SourceText: source,
	})
	if err != nil {
		return nil
	}
	hints := make([]mt.GlossaryHint, 0, len(rows))
	for _, r := range rows {
		switch {
		case r.DoNotTranslate:
			hints = append(hints, mt.GlossaryHint{Term: r.Term, DoNotTranslate: true})
		case r.Translation.Valid && r.Translation.String != "":
			hints = append(hints, mt.GlossaryHint{Term: r.Term, Translation: r.Translation.String})
		}
	}
	return hints
}
