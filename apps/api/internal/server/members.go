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
)

type memberDTO struct {
	ID          string   `json:"id"`
	UserID      string   `json:"userId"`
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	LanguageIDs []string `json:"languageIds"` // per-language scope (translator/reviewer)
}

func validProjectRole(r string) bool {
	switch db.ProjectRole(r) {
	case db.ProjectRoleOwner, db.ProjectRoleAdmin, db.ProjectRoleDeveloper, db.ProjectRoleTranslator, db.ProjectRoleReviewer:
		return true
	}
	return false
}

func (s *Server) listMembers(ctx context.Context, path *extractor.Path[projectPath]) (espresso.JSON[[]memberDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectAdmin, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[[]memberDTO]{}, err
	}
	rows, err := s.store.ListProjectMembers(ctx, pid)
	if err != nil {
		return espresso.JSON[[]memberDTO]{}, espresso.ErrInternal("could not list members")
	}
	out := make([]memberDTO, 0, len(rows))
	for _, m := range rows {
		langs, _ := s.store.ListProjectMemberLanguageIDs(ctx, m.ID)
		out = append(out, memberDTO{
			ID: m.ID, UserID: m.UserID, Email: m.Email, Name: m.Name.String,
			Role: string(m.Role), LanguageIDs: langs,
		})
	}
	return espresso.JSON[[]memberDTO]{Data: out}, nil
}

type addMemberReq struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// addMember adds an existing user (by email) to the project with a role.
func (s *Server) addMember(ctx context.Context, path *extractor.Path[projectPath], body *espresso.JSON[addMemberReq]) (espresso.JSON[memberDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectAdmin, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[memberDTO]{}, err
	}
	role := body.Data.Role
	if role == "" {
		role = string(db.ProjectRoleTranslator)
	}
	if !validProjectRole(role) {
		return espresso.JSON[memberDTO]{}, espresso.ErrBadRequest("invalid role")
	}
	email := strings.TrimSpace(strings.ToLower(body.Data.Email))
	u, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		return espresso.JSON[memberDTO]{}, espresso.ErrNotFound("no user with that email — they need to sign up first")
	}
	m, err := s.store.CreateProjectMember(ctx, db.CreateProjectMemberParams{
		ID: id.New(), ProjectID: pid, UserID: u.ID, Role: db.ProjectRole(role),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return espresso.JSON[memberDTO]{}, espresso.ErrConflict("that user is already a member")
		}
		return espresso.JSON[memberDTO]{}, espresso.ErrInternal("could not add member")
	}
	return espresso.JSON[memberDTO]{StatusCode: http.StatusCreated, Data: memberDTO{
		ID: m.ID, UserID: u.ID, Email: u.Email, Name: u.Name.String, Role: role, LanguageIDs: []string{},
	}}, nil
}

type memberPath struct {
	PID string `path:"pid"`
	MID string `path:"mid"`
}

// memberInProject loads a member and confirms it belongs to the project.
func (s *Server) memberInProject(ctx context.Context, pid, mid string) (db.ProjectMember, error) {
	m, err := s.store.GetProjectMemberByID(ctx, mid)
	if err != nil || m.ProjectID != pid {
		return db.ProjectMember{}, espresso.ErrNotFound("member not found")
	}
	return m, nil
}

type updateRoleReq struct {
	Role string `json:"role"`
}

func (s *Server) updateMemberRole(ctx context.Context, path *extractor.Path[memberPath], body *espresso.JSON[updateRoleReq]) (espresso.JSON[okDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectAdmin, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[okDTO]{}, err
	}
	if !validProjectRole(body.Data.Role) {
		return espresso.JSON[okDTO]{}, espresso.ErrBadRequest("invalid role")
	}
	m, err := s.memberInProject(ctx, pid, path.Data.MID)
	if err != nil {
		return espresso.JSON[okDTO]{}, err
	}
	if err := s.store.UpdateProjectMemberRole(ctx, db.UpdateProjectMemberRoleParams{ID: m.ID, Role: db.ProjectRole(body.Data.Role)}); err != nil {
		return espresso.JSON[okDTO]{}, espresso.ErrInternal("could not update role")
	}
	return espresso.JSON[okDTO]{Data: okDTO{OK: true}}, nil
}

func (s *Server) removeMember(ctx context.Context, path *extractor.Path[memberPath]) (espresso.JSON[okDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectAdmin, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[okDTO]{}, err
	}
	m, err := s.memberInProject(ctx, pid, path.Data.MID)
	if err != nil {
		return espresso.JSON[okDTO]{}, err
	}
	if err := s.store.DeleteProjectMember(ctx, m.ID); err != nil {
		return espresso.JSON[okDTO]{}, espresso.ErrInternal("could not remove member")
	}
	return espresso.JSON[okDTO]{Data: okDTO{OK: true}}, nil
}

type setLanguagesReq struct {
	LanguageIDs []string `json:"languageIds"`
}

// setMemberLanguages replaces a member's per-language scope (used to limit
// translators/reviewers to specific languages). Language ids are validated
// against the project's languages.
func (s *Server) setMemberLanguages(ctx context.Context, path *extractor.Path[memberPath], body *espresso.JSON[setLanguagesReq]) (espresso.JSON[okDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectAdmin, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[okDTO]{}, err
	}
	m, err := s.memberInProject(ctx, pid, path.Data.MID)
	if err != nil {
		return espresso.JSON[okDTO]{}, err
	}
	langs, err := s.store.ListLanguages(ctx, pid)
	if err != nil {
		return espresso.JSON[okDTO]{}, espresso.ErrInternal("language lookup failed")
	}
	valid := make(map[string]bool, len(langs))
	for _, l := range langs {
		valid[l.ID] = true
	}
	if err := s.store.ClearProjectMemberLanguages(ctx, m.ID); err != nil {
		return espresso.JSON[okDTO]{}, espresso.ErrInternal("could not update languages")
	}
	for _, lid := range body.Data.LanguageIDs {
		if !valid[lid] {
			continue // ignore languages not in this project
		}
		if err := s.store.AddProjectMemberLanguage(ctx, db.AddProjectMemberLanguageParams{MemberID: m.ID, LanguageID: lid}); err != nil {
			return espresso.JSON[okDTO]{}, espresso.ErrInternal("could not set languages")
		}
	}
	return espresso.JSON[okDTO]{Data: okDTO{OK: true}}, nil
}
