// Package store owns the PostgreSQL connection pool and the queries the API
// runs against it. The pool is the single shared database handle of the
// process; every feature store receives the same *Store instance.
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store wraps the pgx connection pool. Feature stores embed or receive it and
// add their own parameterised queries. Every query must be parameterised —
// no SQL text is ever assembled from user input.
type Store struct {
	Pool *pgxpool.Pool
}

// New opens a connection pool for the given DSN. Opening is lazy; call Ping to
// fail fast when the database is unreachable.
func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	return &Store{Pool: pool}, nil
}

// Close releases every pooled connection.
func (s *Store) Close() {
	if s == nil || s.Pool == nil {
		return
	}
	s.Pool.Close()
}

// Ping verifies that the database is reachable and authenticating.
func (s *Store) Ping(ctx context.Context) error {
	return s.Pool.Ping(ctx)
}

// Exec runs a parameterised statement (or, with no arguments, a statement
// script such as a migration). Arguments are always passed as bind parameters.
func (s *Store) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := s.Pool.Exec(ctx, sql, args...)
	return err
}
