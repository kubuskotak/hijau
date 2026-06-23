// Package server wires the espresso router, middleware, and HTTP handlers.
package server

import (
	"context"
	"errors"
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
		Get("/api/v1/auth/me", espresso.HandlerCtx(s.me)).
		Get("/api/v1/orgs", espresso.HandlerCtx(s.listOrgs)).
		Get("/api/v1/projects", espresso.HandlerCtx(s.listProjects)).
		Post("/api/v1/projects", espresso.Doppio(s.createProject)).
		Get("/api/v1/projects/{pid}", espresso.Doppio(s.getProject)).
		Patch("/api/v1/projects/{pid}", espresso.Lungo(s.updateProject)).
		Get("/api/v1/projects/{pid}/languages", espresso.Doppio(s.listLanguages)).
		Post("/api/v1/projects/{pid}/languages", espresso.Lungo(s.createLanguage)).
		Put("/api/v1/projects/{pid}/base-language", espresso.Lungo(s.setBaseLanguage)).
		Get("/api/v1/projects/{pid}/namespaces", espresso.Doppio(s.listNamespaces)).
		Post("/api/v1/projects/{pid}/namespaces", espresso.Lungo(s.createNamespace)).
		Get("/api/v1/projects/{pid}/keys", espresso.Lungo(s.listKeys)).
		Post("/api/v1/projects/{pid}/keys", espresso.Lungo(s.createKey)).
		Delete("/api/v1/projects/{pid}/keys/{kid}", espresso.Doppio(s.deleteKey)).
		Get("/api/v1/projects/{pid}/keys/{kid}/translations", espresso.Doppio(s.listKeyTranslations)).
		Put("/api/v1/projects/{pid}/keys/{kid}/translations/{lang}", espresso.Lungo(s.setTranslation)).
		Post("/api/v1/projects/{pid}/keys/{kid}/translations/{lang}/transition", espresso.Lungo(s.transitionTranslation))
}

// authErr maps an authorization result to the right HTTP error.
func authErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, auth.ErrForbidden):
		return espresso.ErrForbidden("you don't have permission to do that")
	default:
		return espresso.ErrInternal("authorization check failed")
	}
}

// requireUser returns the user/PAT-owner id, or an unauthorized error.
func requireUser(ctx context.Context) (string, error) {
	if p := auth.FromContext(ctx); p.UserID != "" {
		return p.UserID, nil
	}
	return "", espresso.ErrUnauthorized("authentication required")
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
