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

type patReq struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

// createPAT mints a Personal Access Token for the signed-in user, for use by
// the CLI (`hijau login`) and the MCP server (HIJAU_TOKEN). A PAT carries the
// owner's full project authority unless narrowed by explicit scopes. Only a
// real user session may mint one — not another token.
func (s *Server) createPAT(ctx context.Context, body *espresso.JSON[patReq]) (espresso.JSON[editorTokenDTO], error) {
	p := auth.FromContext(ctx)
	if p.Kind != auth.UserPrincipal || p.UserID == "" {
		return espresso.JSON[editorTokenDTO]{}, espresso.ErrUnauthorized("sign in to create a personal access token")
	}
	gen, err := auth.GenerateToken(auth.KindPAT)
	if err != nil {
		return espresso.JSON[editorTokenDTO]{}, espresso.ErrInternal("could not mint token")
	}
	name := body.Data.Name
	if name == "" {
		name = "Personal access token"
	}
	scopes := body.Data.Scopes
	if scopes == nil {
		scopes = []string{} // scopes is NOT NULL; empty = inherit the owner's full role
	}
	if _, err := s.store.CreateAPIKey(ctx, db.CreateAPIKeyParams{
		ID: id.New(), Type: db.ApiKeyTypePat, Name: name,
		KeyHash: gen.Hash, Prefix: gen.Prefix, Scopes: scopes,
		OwnerUserID: pgText(p.UserID), ProjectID: pgtype.Text{}, ExpiresAt: pgtype.Timestamptz{},
	}); err != nil {
		return espresso.JSON[editorTokenDTO]{}, espresso.ErrInternal("could not create token")
	}
	return espresso.JSON[editorTokenDTO]{Data: editorTokenDTO{Token: gen.Raw, Prefix: gen.Prefix, Scopes: body.Data.Scopes}}, nil
}

type patListDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Prefix     string `json:"prefix"`
	CreatedAt  string `json:"createdAt"`
	LastUsedAt string `json:"lastUsedAt,omitempty"`
}

// listMyTokens lists the signed-in user's active PATs (never the secret).
func (s *Server) listMyTokens(ctx context.Context) (espresso.JSON[[]patListDTO], error) {
	p := auth.FromContext(ctx)
	if p.Kind != auth.UserPrincipal || p.UserID == "" {
		return espresso.JSON[[]patListDTO]{}, espresso.ErrUnauthorized("sign in to manage tokens")
	}
	rows, err := s.store.ListPATsByUser(ctx, pgText(p.UserID))
	if err != nil {
		return espresso.JSON[[]patListDTO]{}, espresso.ErrInternal("could not list tokens")
	}
	out := make([]patListDTO, 0, len(rows))
	for _, k := range rows {
		d := patListDTO{ID: k.ID, Name: k.Name, Prefix: k.Prefix, CreatedAt: k.CreatedAt.Time.UTC().Format(time.RFC3339)}
		if k.LastUsedAt.Valid {
			d.LastUsedAt = k.LastUsedAt.Time.UTC().Format(time.RFC3339)
		}
		out = append(out, d)
	}
	return espresso.JSON[[]patListDTO]{Data: out}, nil
}

type tokenIDPath struct {
	ID string `path:"id"`
}

// revokeMyToken revokes one of the signed-in user's own PATs.
func (s *Server) revokeMyToken(ctx context.Context, path *extractor.Path[tokenIDPath]) (espresso.JSON[okDTO], error) {
	p := auth.FromContext(ctx)
	if p.Kind != auth.UserPrincipal || p.UserID == "" {
		return espresso.JSON[okDTO]{}, espresso.ErrUnauthorized("sign in to manage tokens")
	}
	k, err := s.store.GetAPIKeyByID(ctx, path.Data.ID)
	if err != nil || k.Type != db.ApiKeyTypePat || !k.OwnerUserID.Valid || k.OwnerUserID.String != p.UserID {
		return espresso.JSON[okDTO]{}, espresso.ErrNotFound("token not found")
	}
	if err := s.store.RevokeAPIKey(ctx, k.ID); err != nil {
		return espresso.JSON[okDTO]{}, espresso.ErrInternal("could not revoke token")
	}
	return espresso.JSON[okDTO]{Data: okDTO{OK: true}}, nil
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

	// 2. Re-authenticate the person. Mirror the login path exactly: a
	//    deactivated (is_active=false) or password-less account must be rejected
	//    here too, otherwise deactivation — the off-boarding mechanism — would
	//    not revoke in-context editing.
	email := strings.TrimSpace(strings.ToLower(body.Data.Email))
	u, err := s.store.GetUserByEmail(ctx, email)
	if err != nil || !u.IsActive || !u.PasswordHash.Valid {
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
		string(auth.PermScreenshotWrite),
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
