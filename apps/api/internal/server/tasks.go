package server

import (
	"context"
	"encoding/json"
	"time"

	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/db"
)

type taskDTO struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Status     string          `json:"status"`
	Progress   int32           `json:"progress"`
	Processed  *int32          `json:"processed,omitempty"`
	Total      *int32          `json:"total,omitempty"`
	Result     json.RawMessage `json:"result,omitempty"`
	Error      string          `json:"error,omitempty"`
	CreatedAt  string          `json:"createdAt"`
	StartedAt  string          `json:"startedAt,omitempty"`
	FinishedAt string          `json:"finishedAt,omitempty"`
}

func toTaskDTO(t db.Task) taskDTO {
	d := taskDTO{
		ID: t.ID, Type: string(t.Type), Status: string(t.Status), Progress: t.Progress,
		Error: t.Error.String, CreatedAt: t.CreatedAt.Time.UTC().Format(time.RFC3339),
	}
	if t.Processed.Valid {
		v := t.Processed.Int32
		d.Processed = &v
	}
	if t.Total.Valid {
		v := t.Total.Int32
		d.Total = &v
	}
	if len(t.Result) > 0 {
		d.Result = json.RawMessage(t.Result)
	}
	if t.StartedAt.Valid {
		d.StartedAt = t.StartedAt.Time.UTC().Format(time.RFC3339)
	}
	if t.FinishedAt.Valid {
		d.FinishedAt = t.FinishedAt.Time.UTC().Format(time.RFC3339)
	}
	return d
}

// listTasks returns the most recent tasks for a project (newest first).
func (s *Server) listTasks(ctx context.Context, path *extractor.Path[projectPath]) (espresso.JSON[[]taskDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectRead, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[[]taskDTO]{}, err
	}
	rows, err := s.store.ListTasksByProject(ctx, db.ListTasksByProjectParams{ProjectID: pgText(pid), Limit: 50})
	if err != nil {
		return espresso.JSON[[]taskDTO]{}, espresso.ErrInternal("could not list tasks")
	}
	out := make([]taskDTO, 0, len(rows))
	for _, t := range rows {
		out = append(out, toTaskDTO(t))
	}
	return espresso.JSON[[]taskDTO]{Data: out}, nil
}

type taskPath struct {
	PID string `path:"pid"`
	TID string `path:"tid"`
}

// getTask returns one task's status, progress and (when finished) result/error
// — the endpoint clients poll after enqueuing an async import/auto-translate.
func (s *Server) getTask(ctx context.Context, path *extractor.Path[taskPath]) (espresso.JSON[taskDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectRead, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[taskDTO]{}, err
	}
	t, err := s.store.GetTask(ctx, path.Data.TID)
	// Require a valid, matching project id — a NULL-project task must not match
	// an empty pid (pgtype.Text zero value is "" with Valid=false).
	if err != nil || !t.ProjectID.Valid || t.ProjectID.String != pid {
		return espresso.JSON[taskDTO]{}, espresso.ErrNotFound("task not found")
	}
	return espresso.JSON[taskDTO]{Data: toTaskDTO(t)}, nil
}
