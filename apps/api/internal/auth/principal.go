package auth

import (
	"context"

	"github.com/portierglobal/hijau/apps/api/internal/db"
)

// PrincipalKind distinguishes how a caller authenticated.
type PrincipalKind int

const (
	Anonymous PrincipalKind = iota
	UserPrincipal
	APIKeyPrincipal
)

// Principal is the authenticated caller resolved by the auth middleware from a
// session cookie or a bearer token.
type Principal struct {
	Kind       PrincipalKind
	UserID     string        // set for user sessions and for PAT owners
	Email      string        // convenience for user principals
	APIKeyID         string        // set for API-key principals
	APIKeyType       db.ApiKeyType // pat | project | editor
	ProjectID        string        // PROJECT/EDITOR keys: the single project they're scoped to
	Scopes           []string      // token scopes; empty means "inherit full role permissions"
	SessionTokenHash string        // user sessions: hash of the cookie token (for logout)
}

func (p Principal) IsAuthenticated() bool { return p.Kind != Anonymous }

type ctxKey struct{}

// WithPrincipal stores p in ctx.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// FromContext returns the principal stored in ctx, or an anonymous principal.
func FromContext(ctx context.Context) Principal {
	if p, ok := ctx.Value(ctxKey{}).(Principal); ok {
		return p
	}
	return Principal{Kind: Anonymous}
}
