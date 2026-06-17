package store

import (
	"context"
	"fmt"

	"github.com/bonukr/phantom-exporter/internal/metrics"
)

// Seed inserts default demo data when the database has no metric groups yet.
// It is a no-op on subsequent runs (existing data is left untouched).
func (s *Store) Seed(ctx context.Context) error {
	var existing int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM metric_groups`).Scan(&existing); err != nil {
		return fmt.Errorf("count groups: %w", err)
	}
	if existing > 0 {
		return nil
	}

	g, err := s.CreateGroup(ctx, "demo", "Demo metrics")
	if err != nil {
		return fmt.Errorf("seed group: %w", err)
	}

	defaults := []metrics.Metric{
		{
			Name:        "http_requests_total",
			Type:        metrics.TypeCounter,
			Description: "Total number of HTTP requests",
			ValueMode:   metrics.ModeRange,
			MinValue:    1,
			MaxValue:    10,
			Labels: []metrics.Label{
				{Key: "method", Value: "GET"},
				{Key: "code", Value: "200"},
			},
		},
		{
			Name:        "temperature_celsius",
			Type:        metrics.TypeGauge,
			Description: "Simulated temperature in Celsius",
			ValueMode:   metrics.ModeRange,
			MinValue:    10,
			MaxValue:    30,
			Labels:      []metrics.Label{{Key: "sensor", Value: "room-1"}},
		},
		{
			Name:        "memory_usage_bytes",
			Type:        metrics.TypeGauge,
			Description: "Simulated memory usage in bytes",
			ValueMode:   metrics.ModeRange,
			MinValue:    100_000_000,
			MaxValue:    900_000_000,
			Labels:      []metrics.Label{},
		},
		{
			Name:        "up",
			Type:        metrics.TypeGauge,
			Description: "Whether the target is up (1) or down (0)",
			ValueMode:   metrics.ModeFixed,
			FixedValue:  1,
			Labels:      []metrics.Label{},
		},
	}

	for i := range defaults {
		m := defaults[i]
		m.GroupID = g.ID
		if err := s.CreateMetric(ctx, &m); err != nil {
			return fmt.Errorf("seed metric %q: %w", m.Name, err)
		}
	}

	s.log.Info("seeded default data", "group", g.Path, "metrics", len(defaults))
	return nil
}
