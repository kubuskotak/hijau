// Package store wraps the pgx connection pool and the sqlc-generated queries.
package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/portierglobal/hijau/apps/api/internal/db"
)

// Store holds the connection pool and the generated query set. The embedded
// *db.Queries means callers can use store.GetUserByID(...) directly, and the
// Pool is available for transactions via WithTx.
type Store struct {
	Pool *pgxpool.Pool
	*db.Queries
}

// New opens a pgx pool and verifies connectivity.
func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{Pool: pool, Queries: db.New(pool)}, nil
}

// WithTx runs fn inside a database transaction, committing on success and
// rolling back on error. The *db.Queries passed to fn is bound to the tx — this
// is how the translation write path keeps the mutation + history + activity
// writes atomic.
func (s *Store) WithTx(ctx context.Context, fn func(q *db.Queries) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit
	if err := fn(s.Queries.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) Close() { s.Pool.Close() }
