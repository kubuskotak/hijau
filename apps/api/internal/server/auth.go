package server

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/suryakencana007/espresso/v2"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/db"
	"github.com/portierglobal/hijau/apps/api/internal/id"
)

type signupReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userDTO struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type okDTO struct {
	OK bool `json:"ok"`
}

// signup creates a user, a personal organization (owner), and a session, then
// sets the session cookie. All database writes happen in one transaction.
func (s *Server) signup(ctx context.Context, req *espresso.JSON[signupReq]) (espresso.JSON[userDTO], error) {
	in := req.Data
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	if _, err := mail.ParseAddress(in.Email); err != nil {
		return espresso.JSON[userDTO]{}, espresso.ErrBadRequest("a valid email is required")
	}
	if len(in.Password) < 8 {
		return espresso.JSON[userDTO]{}, espresso.ErrBadRequest("password must be at least 8 characters")
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return espresso.JSON[userDTO]{}, espresso.ErrInternal("could not hash password")
	}

	userID := id.New()
	var rawToken string
	err = s.store.WithTx(ctx, func(q *db.Queries) error {
		if _, e := q.CreateUser(ctx, db.CreateUserParams{
			ID: userID, Email: in.Email, PasswordHash: pgText(hash), Name: pgText(in.Name),
		}); e != nil {
			return e
		}
		orgID := id.New()
		if _, e := q.CreateOrganization(ctx, db.CreateOrganizationParams{
			ID: orgID, Name: orgNameFor(in), Slug: "org-" + strings.ToLower(orgID),
		}); e != nil {
			return e
		}
		if _, e := q.CreateOrgMembership(ctx, db.CreateOrgMembershipParams{
			ID: id.New(), OrgID: orgID, UserID: userID, Role: db.OrgRoleOwner,
		}); e != nil {
			return e
		}
		tok, e := auth.GenerateToken(auth.KindSession)
		if e != nil {
			return e
		}
		rawToken = tok.Raw
		_, e = q.CreateSession(ctx, db.CreateSessionParams{
			ID: id.New(), UserID: userID, TokenHash: tok.Hash, ExpiresAt: pgTS(time.Now().Add(sessionTTL)),
		})
		return e
	})
	if err != nil {
		if isUniqueViolation(err) {
			return espresso.JSON[userDTO]{}, espresso.ErrConflict("email already registered")
		}
		return espresso.JSON[userDTO]{}, espresso.ErrInternal("signup failed")
	}

	return espresso.JSON[userDTO]{
		StatusCode: http.StatusCreated,
		Data:       userDTO{ID: userID, Email: in.Email, Name: in.Name},
		Cookies:    []*http.Cookie{s.sessionCookie(rawToken)},
	}, nil
}

// login verifies credentials and issues a new session.
func (s *Server) login(ctx context.Context, req *espresso.JSON[loginReq]) (espresso.JSON[userDTO], error) {
	in := req.Data
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))

	u, err := s.store.GetUserByEmail(ctx, in.Email)
	if err != nil || !u.IsActive || !u.PasswordHash.Valid {
		return espresso.JSON[userDTO]{}, espresso.ErrUnauthorized("invalid email or password")
	}
	ok, err := auth.VerifyPassword(in.Password, u.PasswordHash.String)
	if err != nil || !ok {
		return espresso.JSON[userDTO]{}, espresso.ErrUnauthorized("invalid email or password")
	}

	tok, err := auth.GenerateToken(auth.KindSession)
	if err != nil {
		return espresso.JSON[userDTO]{}, espresso.ErrInternal("login failed")
	}
	if _, err := s.store.CreateSession(ctx, db.CreateSessionParams{
		ID: id.New(), UserID: u.ID, TokenHash: tok.Hash, ExpiresAt: pgTS(time.Now().Add(sessionTTL)),
	}); err != nil {
		return espresso.JSON[userDTO]{}, espresso.ErrInternal("login failed")
	}

	return espresso.JSON[userDTO]{
		Data:    userDTO{ID: u.ID, Email: u.Email, Name: u.Name.String},
		Cookies: []*http.Cookie{s.sessionCookie(tok.Raw)},
	}, nil
}

// logout revokes the current session and clears the cookie.
func (s *Server) logout(ctx context.Context) (espresso.JSON[okDTO], error) {
	if p := auth.FromContext(ctx); p.SessionTokenHash != "" {
		_ = s.store.DeleteSession(ctx, p.SessionTokenHash)
	}
	return espresso.JSON[okDTO]{
		Data:    okDTO{OK: true},
		Cookies: []*http.Cookie{clearedSessionCookie()},
	}, nil
}

// me returns the authenticated user (session or PAT owner).
func (s *Server) me(ctx context.Context) (espresso.JSON[userDTO], error) {
	p := auth.FromContext(ctx)
	if p.UserID == "" {
		return espresso.JSON[userDTO]{}, espresso.ErrUnauthorized("not authenticated")
	}
	u, err := s.store.GetUserByID(ctx, p.UserID)
	if err != nil {
		return espresso.JSON[userDTO]{}, espresso.ErrUnauthorized("not authenticated")
	}
	return espresso.JSON[userDTO]{Data: userDTO{ID: u.ID, Email: u.Email, Name: u.Name.String}}, nil
}

func orgNameFor(in signupReq) string {
	if n := strings.TrimSpace(in.Name); n != "" {
		return n + "'s Organization"
	}
	return "My Organization"
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
