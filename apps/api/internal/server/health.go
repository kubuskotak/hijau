package server

import (
	"context"

	"github.com/suryakencana007/espresso/v2"
)

// health is a liveness probe; it does not touch the database.
func (s *Server) health(_ context.Context) espresso.Text {
	return espresso.Text{Body: "ok"}
}

// ready is a readiness probe; it verifies database connectivity.
func (s *Server) ready(ctx context.Context) (espresso.JSON[map[string]string], error) {
	if err := s.store.Pool.Ping(ctx); err != nil {
		return espresso.JSON[map[string]string]{}, espresso.ErrServiceUnavailable("database unavailable")
	}
	return espresso.JSON[map[string]string]{Data: map[string]string{"status": "ready"}}, nil
}
