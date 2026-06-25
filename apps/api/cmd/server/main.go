// Command server runs the Hijau HTTP API.
package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/suryakencana007/espresso/v2"

	api "github.com/portierglobal/hijau/apps/api"
	"github.com/portierglobal/hijau/apps/api/internal/config"
	"github.com/portierglobal/hijau/apps/api/internal/server"
	"github.com/portierglobal/hijau/apps/api/internal/store"
)

func main() {
	config.LoadDotenv() // best-effort for local dev; no-op in containers/CI

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := migrate(ctx, cfg.DatabaseURL); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	srv := server.New(cfg, st)
	srv.StartWorker(ctx) // background task worker (async import / auto-translate)

	router := srv.Router().
		// Drain the worker BEFORE closing the pool: hooks run in registration
		// order, and the worker needs the DB to finish any in-flight task.
		OnShutdown(func(shutdownCtx context.Context) error {
			srv.StopWorker(shutdownCtx)
			return nil
		}).
		OnShutdown(func(context.Context) error {
			st.Close()
			return nil
		})

	log.Printf("hijau api listening on :%s", cfg.Port)
	if err := router.BrewContext(ctx, espresso.WithAddr(":"+cfg.Port)); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server: %v", err)
	}
}

// migrate applies pending migrations on boot using a short-lived database/sql
// handle (goose requires database/sql; the app itself uses pgxpool).
func migrate(ctx context.Context, dsn string) error {
	sqldb, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer sqldb.Close()
	return api.Migrate(ctx, sqldb)
}
