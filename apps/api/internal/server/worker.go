package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/portierglobal/hijau/apps/api/internal/db"
	"github.com/portierglobal/hijau/apps/api/internal/id"
)

// taskEvent is published on the per-project SSE stream as the worker runs, so
// the UI can show a live progress bar. SSE ticks may be dropped (the broker
// drops on a full subscriber buffer), so the UI also polls GET /tasks/{id} for
// authoritative status — a terminal task.completed always carries the outcome.
type taskEvent struct {
	Event     string `json:"event"` // task.progress | task.completed
	ProjectID string `json:"projectId"`
	TaskID    string `json:"taskId"`
	TaskType  string `json:"taskType"`
	Status    string `json:"status"`
	Processed int    `json:"processed"`
	Total     int    `json:"total"`
	Progress  int    `json:"progress"`
	Timestamp string `json:"timestamp"`
}

// enqueue persists a queued task and nudges the worker to pick it up promptly.
func (s *Server) enqueue(ctx context.Context, pid string, typ db.TaskType, payload any, createdBy string) (db.Task, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return db.Task{}, err
	}
	t, err := s.store.CreateTask(ctx, db.CreateTaskParams{
		ID: id.New(), ProjectID: pgText(pid), Type: typ, Payload: b, CreatedBy: pgText(createdBy),
	})
	if err != nil {
		return db.Task{}, err
	}
	s.wakeWorker()
	return t, nil
}

func (s *Server) wakeWorker() {
	select {
	case s.workerWake <- struct{}{}:
	default: // a nudge is already pending
	}
}

// StartWorker requeues orphaned tasks then launches the background loop bound to
// ctx (canceled on SIGTERM). Call once at startup, before Router().
func (s *Server) StartWorker(ctx context.Context) {
	if err := s.store.RecoverRunningTasks(context.Background()); err != nil {
		log.Printf("worker: recover running tasks: %v", err)
	}
	s.workerDone = make(chan struct{})
	go s.workerLoop(ctx)
}

// StopWorker blocks until the worker loop has exited — its in-flight task (if
// any) runs to completion on its detached context, then the loop returns once
// the signal context that StartWorker was given is canceled. Register as an
// OnShutdown hook BEFORE store.Close so the pool outlives the drain.
//
// It deliberately ignores the passed shutdown context and waits on a FRESH
// deadline: espresso derives the hook context from the already-canceled signal
// context, so honoring it would return instantly and let the pool close out
// from under the running task. A task that overruns the deadline is left
// 'running' and requeued on the next boot (the operations are idempotent).
func (s *Server) StopWorker(context.Context) {
	if s.workerDone == nil {
		return
	}
	wait, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	select {
	case <-s.workerDone:
	case <-wait.Done():
		log.Printf("worker: drain timed out after 30s; in-flight task will be requeued on next boot")
	}
}

func (s *Server) workerLoop(ctx context.Context) {
	defer close(s.workerDone)
	ticker := time.NewTicker(5 * time.Second) // safety net for missed wakeups
	defer ticker.Stop()
	for {
		s.drainTasks(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.workerWake:
		}
	}
}

// drainTasks claims and runs queued tasks until none remain (or shutdown). It
// stops claiming new work once ctx is canceled, but lets the current task finish.
func (s *Server) drainTasks(ctx context.Context) {
	for ctx.Err() == nil {
		t, err := s.store.ClaimNextTask(context.Background())
		if errors.Is(err, pgx.ErrNoRows) {
			return // queue empty
		}
		if err != nil {
			log.Printf("worker: claim task: %v", err)
			return
		}
		s.runTask(t)
	}
}

// runTask dispatches one claimed task by type and records its outcome. It uses a
// detached context (the enqueuing request is long gone) and recovers from panics
// so a bad job marks itself failed rather than crashing the process.
func (s *Server) runTask(t db.Task) {
	defer func() {
		if r := recover(); r != nil {
			if err := s.store.FailTask(context.Background(), db.FailTaskParams{ID: t.ID, Error: pgText(fmt.Sprintf("panic: %v", r))}); err != nil {
				log.Printf("worker: task %s panicked (%v) and could not be marked failed: %v", t.ID, r, err)
			}
			s.publishTask(t, "task.completed", db.TaskStatusFailed, 0, 0)
		}
	}()

	ctx := context.Background()
	var result any
	var err error
	switch t.Type {
	case db.TaskTypeImport:
		result, err = s.runImportTask(ctx, t)
	case db.TaskTypeAutoTranslate:
		result, err = s.runAutoTranslateTask(ctx, t)
	default:
		err = fmt.Errorf("unsupported task type %q", t.Type)
	}

	if err != nil {
		_ = s.store.FailTask(ctx, db.FailTaskParams{ID: t.ID, Error: pgText(err.Error())})
		s.publishTask(t, "task.completed", db.TaskStatusFailed, 0, 0)
		return
	}
	b, mErr := json.Marshal(result)
	if mErr != nil {
		_ = s.store.FailTask(ctx, db.FailTaskParams{ID: t.ID, Error: pgText("could not encode result: " + mErr.Error())})
		s.publishTask(t, "task.completed", db.TaskStatusFailed, 0, 0)
		return
	}
	_ = s.store.CompleteTask(ctx, db.CompleteTaskParams{ID: t.ID, Result: b})
	s.publishTask(t, "task.completed", db.TaskStatusSucceeded, 0, 0)
}

// taskProgress returns a throttled progress callback for a running task: it
// writes processed/total/percent to the row and pushes an SSE tick, but only at
// ~1% granularity (and always at completion) to avoid a DB write per item.
func (s *Server) taskProgress(t db.Task) func(done, total int) {
	last := -1
	return func(done, total int) {
		step := total / 100
		if step < 1 {
			step = 1
		}
		if done != total && done-last < step {
			return
		}
		last = done
		pct := 0
		if total > 0 {
			pct = done * 100 / total
		}
		_ = s.store.SetTaskProgress(context.Background(), db.SetTaskProgressParams{
			ID: t.ID, Processed: pgInt(done), Total: pgInt(total), Progress: int32(pct),
		})
		s.publishTask(t, "task.progress", db.TaskStatusRunning, done, total)
	}
}

func (s *Server) publishTask(t db.Task, event string, status db.TaskStatus, processed, total int) {
	if !t.ProjectID.Valid {
		return
	}
	pct := 0
	if total > 0 {
		pct = processed * 100 / total
	} else if status == db.TaskStatusSucceeded {
		pct = 100
	}
	s.broker.publish(t.ProjectID.String, taskEvent{
		Event: event, ProjectID: t.ProjectID.String, TaskID: t.ID, TaskType: string(t.Type),
		Status: string(status), Processed: processed, Total: total, Progress: pct,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func pgInt(n int) pgtype.Int4 { return pgtype.Int4{Int32: int32(n), Valid: true} }
