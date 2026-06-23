package api

import (
	"context"
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func gooseInit() error {
	goose.SetBaseFS(migrationsFS)
	return goose.SetDialect("postgres")
}

// Migrate applies all pending up migrations. Called on server boot.
func Migrate(ctx context.Context, db *sql.DB) error {
	if err := gooseInit(); err != nil {
		return err
	}
	return goose.UpContext(ctx, db, "migrations")
}

// RunGoose runs an arbitrary goose command (up, down, status, reset, version, ...).
func RunGoose(ctx context.Context, db *sql.DB, command string, args ...string) error {
	if err := gooseInit(); err != nil {
		return err
	}
	return goose.RunContext(ctx, command, db, "migrations", args...)
}
