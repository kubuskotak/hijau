package server

import (
	"context"

	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
)

// streamEvents is the SSE endpoint for live updates. A browser EventSource
// subscribes (cookie-authenticated, same-origin) and receives an "update" event
// whenever a translation in the project changes, until it disconnects.
func (s *Server) streamEvents(ctx context.Context, path *extractor.Path[projectPath], stream *espresso.SSEStream) error {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectRead, auth.Check{ProjectID: pid})); err != nil {
		return err
	}
	ch, unsubscribe := s.broker.subscribe(pid)
	defer unsubscribe()

	_ = stream.Comment("connected")
	for {
		select {
		case <-stream.Context().Done():
			return nil // client disconnected (or server shutting down)
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.SendJSON("update", msg); err != nil {
				return err
			}
		}
	}
}
