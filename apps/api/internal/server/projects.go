package server

import (
	"context"
	"strings"
	"time"

	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/db"
	"github.com/portierglobal/hijau/apps/api/internal/id"
)

type projectPath struct {
	PID string `path:"pid"`
}

type createProjectReq struct {
	OrgID       string `json:"orgId"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

type updateProjectReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type projectDTO struct {
	ID             string `json:"id"`
	OrgID          string `json:"orgId"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	Description    string `json:"description"`
	BaseLanguageID string `json:"baseLanguageId"`
	CreatedAt      string `json:"createdAt"`
}

func toProjectDTO(p db.Project) projectDTO {
	return projectDTO{
		ID: p.ID, OrgID: p.OrgID, Name: p.Name, Slug: p.Slug,
		Description:    p.Description.String,
		BaseLanguageID: p.BaseLanguageID.String,
		CreatedAt:      p.CreatedAt.Time.UTC().Format(time.RFC3339),
	}
}

func (s *Server) listProjects(ctx context.Context) (espresso.JSON[[]projectDTO], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return espresso.JSON[[]projectDTO]{}, err
	}
	projs, err := s.store.ListProjectsForUser(ctx, uid)
	if err != nil {
		return espresso.JSON[[]projectDTO]{}, espresso.ErrInternal("could not list projects")
	}
	out := make([]projectDTO, 0, len(projs))
	for _, p := range projs {
		out = append(out, toProjectDTO(p))
	}
	return espresso.JSON[[]projectDTO]{Data: out}, nil
}

func (s *Server) createProject(ctx context.Context, body *espresso.JSON[createProjectReq]) (espresso.JSON[projectDTO], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return espresso.JSON[projectDTO]{}, err
	}
	in := body.Data
	in.Name = strings.TrimSpace(in.Name)
	if in.OrgID == "" || in.Name == "" {
		return espresso.JSON[projectDTO]{}, espresso.ErrBadRequest("orgId and name are required")
	}
	if _, err := s.store.GetOrgMembership(ctx, db.GetOrgMembershipParams{OrgID: in.OrgID, UserID: uid}); err != nil {
		return espresso.JSON[projectDTO]{}, espresso.ErrForbidden("you are not a member of that organization")
	}

	projID := id.New()
	var proj db.Project
	err = s.store.WithTx(ctx, func(q *db.Queries) error {
		var e error
		proj, e = q.CreateProject(ctx, db.CreateProjectParams{
			ID: projID, OrgID: in.OrgID, Name: in.Name,
			Slug: slugify(in.Slug, in.Name), Description: pgText(in.Description),
		})
		if e != nil {
			return e
		}
		_, e = q.CreateProjectMember(ctx, db.CreateProjectMemberParams{
			ID: id.New(), ProjectID: projID, UserID: uid, Role: db.ProjectRoleOwner,
		})
		return e
	})
	if err != nil {
		if isUniqueViolation(err) {
			return espresso.JSON[projectDTO]{}, espresso.ErrConflict("a project with that slug already exists")
		}
		return espresso.JSON[projectDTO]{}, espresso.ErrInternal("could not create project")
	}
	return espresso.JSON[projectDTO]{StatusCode: 201, Data: toProjectDTO(proj)}, nil
}

func (s *Server) getProject(ctx context.Context, path *extractor.Path[projectPath]) (espresso.JSON[projectDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectRead, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[projectDTO]{}, err
	}
	proj, err := s.store.GetProject(ctx, pid)
	if err != nil {
		return espresso.JSON[projectDTO]{}, espresso.ErrNotFound("project not found")
	}
	return espresso.JSON[projectDTO]{Data: toProjectDTO(proj)}, nil
}

func (s *Server) updateProject(ctx context.Context, path *extractor.Path[projectPath], body *espresso.JSON[updateProjectReq]) (espresso.JSON[projectDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectWrite, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[projectDTO]{}, err
	}
	in := body.Data
	proj, err := s.store.UpdateProject(ctx, db.UpdateProjectParams{
		ID: pid, Name: strings.TrimSpace(in.Name), Description: pgText(in.Description),
	})
	if err != nil {
		return espresso.JSON[projectDTO]{}, espresso.ErrNotFound("project not found")
	}
	return espresso.JSON[projectDTO]{Data: toProjectDTO(proj)}, nil
}

// slugify produces a URL-safe slug from a custom value or the name.
func slugify(custom, name string) string {
	base := strings.TrimSpace(custom)
	if base == "" {
		base = name
	}
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(base) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			dash = false
		default:
			if !dash {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		s = "p-" + strings.ToLower(id.New()[:8])
	}
	return s
}
