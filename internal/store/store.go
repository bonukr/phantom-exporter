// Package store persists metric groups and definitions in PostgreSQL.
package store

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store wraps a PostgreSQL connection pool.
type Store struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

const schema = `
CREATE TABLE IF NOT EXISTS metric_groups (
    id         BIGSERIAL PRIMARY KEY,
    path       TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS metric_defs (
    id            BIGSERIAL PRIMARY KEY,
    group_id      BIGINT NOT NULL REFERENCES metric_groups(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    type          TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    value_mode    TEXT NOT NULL,
    min_value     DOUBLE PRECISION NOT NULL DEFAULT 0,
    max_value     DOUBLE PRECISION NOT NULL DEFAULT 0,
    fixed_value   DOUBLE PRECISION NOT NULL DEFAULT 0,
    override_value DOUBLE PRECISION,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS metric_labels (
    id        BIGSERIAL PRIMARY KEY,
    metric_id BIGINT NOT NULL REFERENCES metric_defs(id) ON DELETE CASCADE,
    key       TEXT NOT NULL,
    value     TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_metric_defs_group ON metric_defs(group_id);
CREATE INDEX IF NOT EXISTS idx_metric_labels_metric ON metric_labels(metric_id);
`

// New connects to PostgreSQL, verifies connectivity and applies the schema.
func New(ctx context.Context, dsn string, log *slog.Logger) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	s := &Store{pool: pool, log: log}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := s.Seed(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	log.Info("postgres connected and schema ready")
	return s, nil
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}

// Ping reports database connectivity (used by the status endpoint).
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Close releases the connection pool.
func (s *Store) Close() {
	s.pool.Close()
}
