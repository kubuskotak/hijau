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

type historyDTO struct {
	ID          string `json:"id"`
	OldText     string `json:"oldText"`
	NewText     string `json:"newText"`
	OldState    string `json:"oldState"`
	NewState    string `json:"newState"`
	Origin      string `json:"origin"`
	AuthorKind  string `json:"authorKind"`
	AuthorEmail string `json:"authorEmail"`
	CreatedAt   string `json:"createdAt"`
}

type commentDTO struct {
	ID          string `json:"id"`
	Body        string `json:"body"`
	ParentID    string `json:"parentId"`
	AuthorEmail string `json:"authorEmail"`
	AuthorName  string `json:"authorName"`
	Resolved    bool   `json:"resolved"`
	CreatedAt   string `json:"createdAt"`
}

type addCommentReq struct {
	Body     string `json:"body"`
	ParentID string `json:"parentId"`
}

type resolveReq struct {
	Resolved bool `json:"resolved"`
}

type commentPath struct {
	CID string `path:"cid"`
}

func nullStateStr(s db.NullTranslationState) string {
	if s.Valid {
		return string(s.TranslationState)
	}
	return ""
}

// resolveTranslation loads the translation row for a (project, key, language).
func (s *Server) resolveTranslation(ctx context.Context, pid, kid, langTag string) (db.Translation, error) {
	key, lang, _, err := s.loadKeyLang(ctx, pid, kid, langTag)
	if err != nil {
		return db.Translation{}, err
	}
	t, err := s.store.GetTranslation(ctx, db.GetTranslationParams{KeyID: key.ID, LanguageID: lang.ID})
	if err != nil {
		return db.Translation{}, espresso.ErrNotFound("translation not found")
	}
	return t, nil
}

func (s *Server) translationHistory(ctx context.Context, path *extractor.Path[transPath]) (espresso.JSON[[]historyDTO], error) {
	d := path.Data
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermTranslationsRead, auth.Check{ProjectID: d.PID})); err != nil {
		return espresso.JSON[[]historyDTO]{}, err
	}
	t, err := s.resolveTranslation(ctx, d.PID, d.KID, d.Lang)
	if err != nil {
		return espresso.JSON[[]historyDTO]{}, err
	}
	rows, err := s.store.ListTranslationHistory(ctx, db.ListTranslationHistoryParams{TranslationID: t.ID, Limit: 50})
	if err != nil {
		return espresso.JSON[[]historyDTO]{}, espresso.ErrInternal("could not load history")
	}
	out := make([]historyDTO, 0, len(rows))
	for _, h := range rows {
		out = append(out, historyDTO{
			ID: h.ID, OldText: h.OldText.String, NewText: h.NewText.String,
			OldState: nullStateStr(h.OldState), NewState: nullStateStr(h.NewState),
			Origin: string(h.Origin), AuthorKind: string(h.AuthorKind),
			AuthorEmail: h.AuthorEmail.String,
			CreatedAt:   h.CreatedAt.Time.UTC().Format(time.RFC3339),
		})
	}
	return espresso.JSON[[]historyDTO]{Data: out}, nil
}

func (s *Server) listTranslationComments(ctx context.Context, path *extractor.Path[transPath]) (espresso.JSON[[]commentDTO], error) {
	d := path.Data
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermTranslationsRead, auth.Check{ProjectID: d.PID})); err != nil {
		return espresso.JSON[[]commentDTO]{}, err
	}
	t, err := s.resolveTranslation(ctx, d.PID, d.KID, d.Lang)
	if err != nil {
		return espresso.JSON[[]commentDTO]{}, err
	}
	rows, err := s.store.ListCommentsForTranslation(ctx, pgtype.Text{String: t.ID, Valid: true})
	if err != nil {
		return espresso.JSON[[]commentDTO]{}, espresso.ErrInternal("could not load comments")
	}
	out := make([]commentDTO, 0, len(rows))
	for _, c := range rows {
		out = append(out, commentDTO{
			ID: c.ID, Body: c.Body, ParentID: c.ParentID.String,
			AuthorEmail: c.AuthorEmail, AuthorName: c.AuthorName.String,
			Resolved:  c.ResolvedAt.Valid,
			CreatedAt: c.CreatedAt.Time.UTC().Format(time.RFC3339),
		})
	}
	return espresso.JSON[[]commentDTO]{Data: out}, nil
}

func (s *Server) addTranslationComment(ctx context.Context, path *extractor.Path[transPath], body *espresso.JSON[addCommentReq]) (espresso.JSON[commentDTO], error) {
	d := path.Data
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermComment, auth.Check{ProjectID: d.PID})); err != nil {
		return espresso.JSON[commentDTO]{}, err
	}
	p := auth.FromContext(ctx)
	if p.UserID == "" {
		return espresso.JSON[commentDTO]{}, espresso.ErrForbidden("only signed-in users can comment")
	}
	text := strings.TrimSpace(body.Data.Body)
	if text == "" {
		return espresso.JSON[commentDTO]{}, espresso.ErrBadRequest("comment body is required")
	}
	t, err := s.resolveTranslation(ctx, d.PID, d.KID, d.Lang)
	if err != nil {
		return espresso.JSON[commentDTO]{}, err
	}
	var parent pgtype.Text
	if pidc := strings.TrimSpace(body.Data.ParentID); pidc != "" {
		parent = pgtype.Text{String: pidc, Valid: true}
	}
	c, err := s.store.CreateComment(ctx, db.CreateCommentParams{
		ID:            id.New(),
		TranslationID: pgtype.Text{String: t.ID, Valid: true},
		AuthorID:      p.UserID,
		Body:          text,
		ParentID:      parent,
	})
	if err != nil {
		return espresso.JSON[commentDTO]{}, espresso.ErrInternal("could not add comment")
	}
	return espresso.JSON[commentDTO]{
		StatusCode: 201,
		Data: commentDTO{
			ID: c.ID, Body: c.Body, ParentID: c.ParentID.String,
			AuthorEmail: p.Email, Resolved: false,
			CreatedAt: c.CreatedAt.Time.UTC().Format(time.RFC3339),
		},
	}, nil
}

func (s *Server) resolveComment(ctx context.Context, path *extractor.Path[commentPath], body *espresso.JSON[resolveReq]) (espresso.JSON[okDTO], error) {
	cid := path.Data.CID
	pid, err := s.store.GetCommentProjectID(ctx, cid)
	if err != nil {
		return espresso.JSON[okDTO]{}, espresso.ErrNotFound("comment not found")
	}
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermComment, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[okDTO]{}, err
	}
	p := auth.FromContext(ctx)
	if body.Data.Resolved {
		_, err = s.store.ResolveComment(ctx, db.ResolveCommentParams{
			ID: cid, ResolvedBy: pgtype.Text{String: p.UserID, Valid: p.UserID != ""},
		})
	} else {
		_, err = s.store.UnresolveComment(ctx, cid)
	}
	if err != nil {
		return espresso.JSON[okDTO]{}, espresso.ErrInternal("could not update comment")
	}
	return espresso.JSON[okDTO]{Data: okDTO{OK: true}}, nil
}
