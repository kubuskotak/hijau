// Package server wires the espresso router, middleware, and HTTP handlers.
package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suryakencana007/espresso/v2"
	httpmiddleware "github.com/suryakencana007/espresso/v2/middleware/http"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/config"
	"github.com/portierglobal/hijau/apps/api/internal/store"
)

const sessionTTL = 30 * 24 * time.Hour

type Server struct {
	cfg   config.Config
	store *store.Store
}

func New(cfg config.Config, st *store.Store) *Server {
	return &Server{cfg: cfg, store: st}
}

// Router builds the HTTP router with global middleware and all routes.
func (s *Server) Router() *espresso.Router {
	return espresso.Portafilter().
		Use(httpmiddleware.RequestIDMiddleware()).
		Use(httpmiddleware.RecoverMiddleware()).
		Use(httpmiddleware.LoggingMiddleware()).
		Use(httpmiddleware.CORSMiddleware(httpmiddleware.DefaultCORSConfig)).
		Use(auth.Middleware(s.store)).
		Get("/health", espresso.Ristretto(s.health)).
		Get("/health/ready", espresso.HandlerCtx(s.ready)).
		Post("/api/v1/auth/signup", espresso.Doppio(s.signup)).
		Post("/api/v1/auth/login", espresso.Doppio(s.login)).
		Post("/api/v1/auth/logout", espresso.HandlerCtx(s.logout)).
		Get("/api/v1/auth/me", espresso.HandlerCtx(s.me))
}

func (s *Server) sessionCookie(raw string) *http.Cookie {
	return &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    raw,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   strings.HasPrefix(s.cfg.PublicURL, "https"),
		Expires:  time.Now().Add(sessionTTL),
		MaxAge:   int(sessionTTL.Seconds()),
	}
}

func clearedSessionCookie() *http.Cookie {
	return &http.Cookie{Name: auth.SessionCookieName, Value: "", Path: "/", HttpOnly: true, MaxAge: -1}
}

func pgText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func pgTS(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
