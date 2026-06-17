package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/bonukr/phantom-exporter/internal/metrics"
	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

// ListGroups returns all groups (without their metrics) ordered by path.
func (s *Store) ListGroups(ctx context.Context) ([]metrics.Group, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, path, name FROM metric_groups ORDER BY path`)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close()

	var groups []metrics.Group
	for rows.Next() {
		var g metrics.Group
		if err := rows.Scan(&g.ID, &g.Path, &g.Name); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// GetGroup returns a single group including its metrics and labels.
func (s *Store) GetGroup(ctx context.Context, id int64) (*metrics.Group, error) {
	var g metrics.Group
	err := s.pool.QueryRow(ctx,
		`SELECT id, path, name FROM metric_groups WHERE id = $1`, id,
	).Scan(&g.ID, &g.Path, &g.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get group: %w", err)
	}
	if g.Metrics, err = s.metricsForGroup(ctx, g.ID); err != nil {
		return nil, err
	}
	return &g, nil
}

// GetGroupByPath returns a group (with metrics) by its scrape path.
func (s *Store) GetGroupByPath(ctx context.Context, path string) (*metrics.Group, error) {
	var g metrics.Group
	err := s.pool.QueryRow(ctx,
		`SELECT id, path, name FROM metric_groups WHERE path = $1`, path,
	).Scan(&g.ID, &g.Path, &g.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get group by path: %w", err)
	}
	if g.Metrics, err = s.metricsForGroup(ctx, g.ID); err != nil {
		return nil, err
	}
	return &g, nil
}

// CreateGroup inserts a new group.
func (s *Store) CreateGroup(ctx context.Context, path, name string) (*metrics.Group, error) {
	g := metrics.Group{Path: path, Name: name}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO metric_groups (path, name) VALUES ($1, $2) RETURNING id`,
		path, name,
	).Scan(&g.ID)
	if err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}
	return &g, nil
}

// UpdateGroup changes a group's path and name.
func (s *Store) UpdateGroup(ctx context.Context, id int64, path, name string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE metric_groups SET path = $2, name = $3 WHERE id = $1`, id, path, name)
	if err != nil {
		return fmt.Errorf("update group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteGroup removes a group and (via cascade) its metrics and labels.
func (s *Store) DeleteGroup(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM metric_groups WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
