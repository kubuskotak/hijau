package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/portierglobal/hijau/apps/api/internal/db"
)

// SessionCookieName is the cookie that carries the raw session token.
const SessionCookieName = "hj_session"

// Resolver loads principals from the database. *db.Queries (and therefore
// *store.Store) satisfies it.
type Resolver interface {
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (db.GetSessionByTokenHashRow, error)
	GetAPIKeyByHash(ctx context.Context, keyHash string) (db.ApiKey, error)
	TouchAPIKey(ctx context.Context, id string) error
}

// Middleware resolves a Principal from the Authorization bearer token or the
// session cookie and stores it in the request context. Requests without valid
// credentials continue as anonymous — enforcement is each handler's job (via
// Authorize), so that public routes still work behind the same middleware.
func Middleware(r Resolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			p := resolve(req, r)
			next.ServeHTTP(w, req.WithContext(WithPrincipal(req.Context(), p)))
		})
	}
}

func resolve(req *http.Request, r Resolver) Principal {
	ctx := req.Context()
	// Bearer token (PAT / project / editor) takes precedence for API clients.
	if h := req.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		if raw := strings.TrimSpace(strings.TrimPrefix(h, "Bearer ")); raw != "" {
			if p, ok := resolveAPIKey(ctx, r, raw); ok {
				return p
			}
		}
	}
	// Session cookie (browser).
	if c, err := req.Cookie(SessionCookieName); err == nil && c.Value != "" {
		if p, ok := resolveSession(ctx, r, c.Value); ok {
			return p
		}
	}
	return Principal{Kind: Anonymous}
}

func resolveSession(ctx context.Context, r Resolver, rawToken string) (Principal, bool) {
	hash := HashToken(rawToken)
	row, err := r.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		return Principal{}, false
	}
	return Principal{
		Kind:             UserPrincipal,
		UserID:           row.User.ID,
		Email:            row.User.Email,
		SessionTokenHash: hash,
	}, true
}

func resolveAPIKey(ctx context.Context, r Resolver, raw string) (Principal, bool) {
	k, err := r.GetAPIKeyByHash(ctx, HashToken(raw))
	if err != nil {
		return Principal{}, false
	}
	_ = r.TouchAPIKey(ctx, k.ID) // best-effort last-used bump
	p := Principal{Kind: APIKeyPrincipal, APIKeyID: k.ID, APIKeyType: k.Type, Scopes: k.Scopes}
	if k.OwnerUserID.Valid {
		p.UserID = k.OwnerUserID.String
	}
	if k.ProjectID.Valid {
		p.ProjectID = k.ProjectID.String
	}
	return p, true
}
