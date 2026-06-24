package server

import (
	"context"
	"strings"

	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/db"
	"github.com/portierglobal/hijau/apps/api/internal/id"
)

type namespaceDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type createNamespaceReq struct {
	Name string `json:"name"`
}

func (s *Server) listNamespaces(ctx context.Context, path *extractor.Path[projectPath]) (espresso.JSON[[]namespaceDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectRead, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[[]namespaceDTO]{}, err
	}
	nss, err := s.store.ListNamespaces(ctx, pid)
	if err != nil {
		return espresso.JSON[[]namespaceDTO]{}, espresso.ErrInternal("could not list namespaces")
	}
	out := make([]namespaceDTO, 0, len(nss))
	for _, n := range nss {
		out = append(out, namespaceDTO{ID: n.ID, Name: n.Name})
	}
	return espresso.JSON[[]namespaceDTO]{Data: out}, nil
}

func (s *Server) createNamespace(ctx context.Context, path *extractor.Path[projectPath], body *espresso.JSON[createNamespaceReq]) (espresso.JSON[namespaceDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectWrite, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[namespaceDTO]{}, err
	}
	name := strings.TrimSpace(body.Data.Name)
	if name == "" {
		return espresso.JSON[namespaceDTO]{}, espresso.ErrBadRequest("name is required")
	}
	ns, err := s.store.CreateNamespace(ctx, db.CreateNamespaceParams{ID: id.New(), ProjectID: pid, Name: name})
	if err != nil {
		if isUniqueViolation(err) {
			return espresso.JSON[namespaceDTO]{}, espresso.ErrConflict("namespace already exists")
		}
		return espresso.JSON[namespaceDTO]{}, espresso.ErrInternal("could not create namespace")
	}
	return espresso.JSON[namespaceDTO]{StatusCode: 201, Data: namespaceDTO{ID: ns.ID, Name: ns.Name}}, nil
}

// resolveNamespaceID returns the namespace id for a name within a project,
// creating the namespace if needed. Empty name => no namespace (NULL).
func resolveNamespaceID(ctx context.Context, q *db.Queries, projectID, name string) (db.Namespace, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return db.Namespace{}, false, nil
	}
	ns, err := q.GetNamespaceByName(ctx, db.GetNamespaceByNameParams{ProjectID: projectID, Name: name})
	if err == nil {
		return ns, true, nil
	}
	ns, err = q.CreateNamespace(ctx, db.CreateNamespaceParams{ID: id.New(), ProjectID: projectID, Name: name})
	if err != nil {
		return db.Namespace{}, false, err
	}
	return ns, true, nil
}
