package server

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/db"
	"github.com/portierglobal/hijau/apps/api/internal/id"
)

// editorUnlockTTL bounds how long an unlocked in-context editing token lives.
// Short by design: the token rides in the browser, so a leak is limited.
const editorUnlockTTL = 30 * time.Minute

type editorTokenReq struct {
	Name string `json:"name"`
}

type editorTokenDTO struct {
	Token     string   `json:"token"`
	Prefix    string   `json:"prefix"`
	Scopes    []string `json:"scopes"`
	ExpiresAt string   `json:"expiresAt,omitempty"`
}

// createEditorToken mints a read-only, project-scoped editor token. It carries
// no user binding and only read scopes, so it is the one token type safe to
// embed in a shipped browser bundle: it can resolve translations for in-context
// display but cannot change anything. Writing requires unlockEditor.
func (s *Server) createEditorToken(ctx context.Context, path *extractor.Path[projectPath], body *espresso.JSON[editorTokenReq]) (espresso.JSON[editorTokenDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectAdmin, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[editorTokenDTO]{}, err
	}
	gen, err := auth.GenerateToken(auth.KindEditor)
	if err != nil {
		return espresso.JSON[editorTokenDTO]{}, espresso.ErrInternal("could not mint token")
	}
	scopes := []string{string(auth.PermProjectRead), string(auth.PermTranslationsRead)}
	name := body.Data.Name
	if name == "" {
		name = "In-context editor (read-only)"
	}
	if _, err := s.store.CreateAPIKey(ctx, db.CreateAPIKeyParams{
		ID: id.New(), Type: db.ApiKeyTypeEditor, Name: name,
		KeyHash: gen.Hash, Prefix: gen.Prefix, Scopes: scopes,
		OwnerUserID: pgtype.Text{}, ProjectID: pgText(pid), ExpiresAt: pgtype.Timestamptz{},
	}); err != nil {
		return espresso.JSON[editorTokenDTO]{}, espresso.ErrInternal("could not create token")
	}
	return espresso.JSON[editorTokenDTO]{Data: editorTokenDTO{Token: gen.Raw, Prefix: gen.Prefix, Scopes: scopes}}, nil
}

type unlockReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type unlockDTO struct {
	Token     string   `json:"token"`
	Prefix    string   `json:"prefix"`
	Scopes    []string `json:"scopes"`
	ExpiresAt string   `json:"expiresAt"`
	User      userDTO  `json:"user"`
}

// unlockEditor turns a read-only editor session into a short-lived writable one,
// attributable to a real person. The caller must already hold this project's
// editor token (proving they're the embedded editor) and must re-authenticate
// with their account. The minted token is user-bound, so every write it makes
// is authorized against — and can never exceed — that user's project role, and
// is recorded in history under their name.
func (s *Server) unlockEditor(ctx context.Context, path *extractor.Path[projectPath], body *espresso.JSON[unlockReq]) (espresso.JSON[unlockDTO], error) {
	pid := path.Data.PID

	// 1. Must be invoked by this project's editor token.
	p := auth.FromContext(ctx)
	if p.Kind != auth.APIKeyPrincipal || p.APIKeyType != db.ApiKeyTypeEditor || p.ProjectID != pid {
		return espresso.JSON[unlockDTO]{}, espresso.ErrUnauthorized("a project editor token is required to unlock editing")
	}

	// 2. Re-authenticate the person.
	u, err := s.store.GetUserByEmail(ctx, body.Data.Email)
	if err != nil {
		return espresso.JSON[unlockDTO]{}, espresso.ErrUnauthorized("invalid email or password")
	}
	ok, err := auth.VerifyPassword(body.Data.Password, u.PasswordHash.String)
	if err != nil || !ok {
		return espresso.JSON[unlockDTO]{}, espresso.ErrUnauthorized("invalid email or password")
	}

	// 3. They must be a member of this project (per-language write limits are
	//    still enforced on each individual save, since the token is user-bound).
	userCtx := auth.WithPrincipal(ctx, auth.Principal{Kind: auth.UserPrincipal, UserID: u.ID, Email: u.Email})
	if err := authErr(auth.Authorize(userCtx, s.store, auth.PermProjectRead, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[unlockDTO]{}, err
	}

	// 4. Mint the short-lived, user-bound write token.
	gen, err := auth.GenerateToken(auth.KindEditor)
	if err != nil {
		return espresso.JSON[unlockDTO]{}, espresso.ErrInternal("could not mint token")
	}
	scopes := []string{
		string(auth.PermProjectRead), string(auth.PermTranslationsRead),
		string(auth.PermTranslationsWrite), string(auth.PermComment),
	}
	exp := time.Now().Add(editorUnlockTTL)
	if _, err := s.store.CreateAPIKey(ctx, db.CreateAPIKeyParams{
		ID: id.New(), Type: db.ApiKeyTypeEditor, Name: "In-context edit · " + u.Email,
		KeyHash: gen.Hash, Prefix: gen.Prefix, Scopes: scopes,
		OwnerUserID: pgText(u.ID), ProjectID: pgText(pid), ExpiresAt: pgTS(exp),
	}); err != nil {
		return espresso.JSON[unlockDTO]{}, espresso.ErrInternal("could not create token")
	}

	return espresso.JSON[unlockDTO]{Data: unlockDTO{
		Token: gen.Raw, Prefix: gen.Prefix, Scopes: scopes,
		ExpiresAt: exp.UTC().Format(time.RFC3339),
		User:      userDTO{ID: u.ID, Email: u.Email, Name: u.Name.String},
	}}, nil
}
