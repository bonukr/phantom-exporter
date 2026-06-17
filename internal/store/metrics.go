package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/bonukr/phantom-exporter/internal/metrics"
	"github.com/jackc/pgx/v5"
)

// metricsForGroup loads all metric definitions (with labels) for a group.
func (s *Store) metricsForGroup(ctx context.Context, groupID int64) ([]metrics.Metric, error) {
	rows, err := s.pool.Query(ctx, `
        SELECT id, group_id, name, type, description, value_mode,
               min_value, max_value, fixed_value, override_value
        FROM metric_defs WHERE group_id = $1 ORDER BY name, id`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list metrics: %w", err)
	}
	defer rows.Close()

	var out []metrics.Metric
	for rows.Next() {
		m, err := scanMetric(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range out {
		if out[i].Labels, err = s.labelsForMetric(ctx, out[i].ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Store) labelsForMetric(ctx context.Context, metricID int64) ([]metrics.Label, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT key, value FROM metric_labels WHERE metric_id = $1 ORDER BY key`, metricID)
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	defer rows.Close()

	labels := []metrics.Label{}
	for rows.Next() {
		var l metrics.Label
		if err := rows.Scan(&l.Key, &l.Value); err != nil {
			return nil, fmt.Errorf("scan label: %w", err)
		}
		labels = append(labels, l)
	}
	return labels, rows.Err()
}

// GetMetric returns a single metric with its labels.
func (s *Store) GetMetric(ctx context.Context, id int64) (*metrics.Metric, error) {
	row := s.pool.QueryRow(ctx, `
        SELECT id, group_id, name, type, description, value_mode,
               min_value, max_value, fixed_value, override_value
        FROM metric_defs WHERE id = $1`, id)
	m, err := scanMetric(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if m.Labels, err = s.labelsForMetric(ctx, m.ID); err != nil {
		return nil, err
	}
	return &m, nil
}

// CreateMetric inserts a metric and its labels, populating m.ID.
func (s *Store) CreateMetric(ctx context.Context, m *metrics.Metric) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
        INSERT INTO metric_defs
            (group_id, name, type, description, value_mode, min_value, max_value, fixed_value)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		m.GroupID, m.Name, m.Type, m.Description, m.ValueMode,
		m.MinValue, m.MaxValue, m.FixedValue,
	).Scan(&m.ID)
	if err != nil {
		return fmt.Errorf("insert metric: %w", err)
	}
	if err := insertLabels(ctx, tx, m.ID, m.Labels); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UpdateMetric updates a metric's fields and replaces its labels.
func (s *Store) UpdateMetric(ctx context.Context, m *metrics.Metric) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
        UPDATE metric_defs SET
            name=$2, type=$3, description=$4, value_mode=$5,
            min_value=$6, max_value=$7, fixed_value=$8, updated_at=now()
        WHERE id=$1`,
		m.ID, m.Name, m.Type, m.Description, m.ValueMode,
		m.MinValue, m.MaxValue, m.FixedValue,
	)
	if err != nil {
		return fmt.Errorf("update metric: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `DELETE FROM metric_labels WHERE metric_id=$1`, m.ID); err != nil {
		return fmt.Errorf("clear labels: %w", err)
	}
	if err := insertLabels(ctx, tx, m.ID, m.Labels); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetOverride sets (or clears, when value is nil) a metric's real-time override.
func (s *Store) SetOverride(ctx context.Context, id int64, value *float64) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE metric_defs SET override_value=$2, updated_at=now() WHERE id=$1`, id, value)
	if err != nil {
		return fmt.Errorf("set override: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteMetric removes a metric and its labels.
func (s *Store) DeleteMetric(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM metric_defs WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete metric: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CountMetrics returns the total number of metric definitions.
func (s *Store) CountMetrics(ctx context.Context) (int, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM metric_defs`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count metrics: %w", err)
	}
	return n, nil
}

func insertLabels(ctx context.Context, tx pgx.Tx, metricID int64, labels []metrics.Label) error {
	for _, l := range labels {
		if l.Key == "" {
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO metric_labels (metric_id, key, value) VALUES ($1,$2,$3)`,
			metricID, l.Key, l.Value); err != nil {
			return fmt.Errorf("insert label: %w", err)
		}
	}
	return nil
}

// rowScanner is satisfied by both pgx.Row and pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanMetric(row rowScanner) (metrics.Metric, error) {
	var m metrics.Metric
	var override *float64
	err := row.Scan(
		&m.ID, &m.GroupID, &m.Name, &m.Type, &m.Description, &m.ValueMode,
		&m.MinValue, &m.MaxValue, &m.FixedValue, &override,
	)
	if err != nil {
		return m, err
	}
	m.Override = override
	m.Labels = []metrics.Label{}
	return m, nil
}
