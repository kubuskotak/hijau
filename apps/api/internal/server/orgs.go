package server

import (
	"context"

	"github.com/suryakencana007/espresso/v2"
)

type orgDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// listOrgs returns the organizations the current user belongs to.
func (s *Server) listOrgs(ctx context.Context) (espresso.JSON[[]orgDTO], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return espresso.JSON[[]orgDTO]{}, err
	}
	orgs, err := s.store.ListUserOrganizations(ctx, uid)
	if err != nil {
		return espresso.JSON[[]orgDTO]{}, espresso.ErrInternal("could not list organizations")
	}
	out := make([]orgDTO, 0, len(orgs))
	for _, o := range orgs {
		out = append(out, orgDTO{ID: o.ID, Name: o.Name, Slug: o.Slug})
	}
	return espresso.JSON[[]orgDTO]{Data: out}, nil
}
