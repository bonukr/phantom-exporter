package store

import (
	"context"

	"github.com/bonukr/phantom-exporter/internal/metrics"
)

// ---- Groups ----

// ListGroups returns all groups (without metrics) ordered by path.
func (s *Store) ListGroups(ctx context.Context) ([]metrics.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]metrics.Group, 0, len(s.groups))
	for _, g := range s.groups {
		out = append(out, metrics.Group{ID: g.ID, Path: g.Path, Name: g.Name})
	}
	return out, nil
}

// GetGroup returns a single group including its metrics and labels.
func (s *Store) GetGroup(ctx context.Context, id int64) (*metrics.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.findGroup(id)
	if g == nil {
		return nil, ErrNotFound
	}
	out := cloneGroup(*g)
	return &out, nil
}

// GetGroupByPath returns a group (with metrics) by its scrape path.
func (s *Store) GetGroupByPath(ctx context.Context, path string) (*metrics.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.groups {
		if s.groups[i].Path == path {
			out := cloneGroup(s.groups[i])
			return &out, nil
		}
	}
	return nil, ErrNotFound
}

// CreateGroup inserts a new group and writes its settings file.
func (s *Store) CreateGroup(ctx context.Context, path, name string) (*metrics.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.groups {
		if s.groups[i].Path == path {
			return nil, ErrConflict
		}
	}
	g := metrics.Group{ID: s.nextGroupID, Path: path, Name: name, Metrics: []metrics.Metric{}}
	s.nextGroupID++
	s.groups = append(s.groups, g)
	if err := s.saveGroup(&s.groups[len(s.groups)-1]); err != nil {
		return nil, err
	}
	out := cloneGroup(g)
	return &out, nil
}

// UpdateGroup changes a group's path and name, renaming its file if needed.
func (s *Store) UpdateGroup(ctx context.Context, id int64, path, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.findGroup(id)
	if g == nil {
		return ErrNotFound
	}
	for i := range s.groups {
		if s.groups[i].Path == path && s.groups[i].ID != id {
			return ErrConflict
		}
	}
	oldPath := g.Path
	g.Path, g.Name = path, name
	if err := s.saveGroup(g); err != nil {
		return err
	}
	if oldPath != path {
		if err := s.removeGroupFile(oldPath); err != nil {
			return err
		}
	}
	return nil
}

// DeleteGroup removes a group, its metrics and its settings file.
func (s *Store) DeleteGroup(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.groups {
		if s.groups[i].ID == id {
			path := s.groups[i].Path
			s.groups = append(s.groups[:i], s.groups[i+1:]...)
			return s.removeGroupFile(path)
		}
	}
	return ErrNotFound
}

// ---- Metrics ----

// CreateMetric inserts a metric into its group, populating m.ID.
func (s *Store) CreateMetric(ctx context.Context, m *metrics.Metric) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.findGroup(m.GroupID)
	if g == nil {
		return ErrNotFound
	}
	m.ID = s.nextMetricID
	s.nextMetricID++
	if m.Labels == nil {
		m.Labels = []metrics.Label{}
	}
	g.Metrics = append(g.Metrics, cloneMetric(*m))
	return s.saveGroup(g)
}

// GetMetric returns a single metric with its labels.
func (s *Store) GetMetric(ctx context.Context, id int64) (*metrics.Metric, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, m := s.findMetric(id); m != nil {
		out := cloneMetric(*m)
		return &out, nil
	}
	return nil, ErrNotFound
}

// UpdateMetric updates a metric's fields and labels (group membership and any
// active override are preserved).
func (s *Store) UpdateMetric(ctx context.Context, m *metrics.Metric) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, existing := s.findMetric(m.ID)
	if existing == nil {
		return ErrNotFound
	}
	updated := cloneMetric(*m)
	updated.ID = existing.ID
	updated.GroupID = existing.GroupID
	updated.Override = existing.Override
	if updated.Labels == nil {
		updated.Labels = []metrics.Label{}
	}
	*existing = updated
	return s.saveGroup(g)
}

// SetOverride sets (or clears, when value is nil) a metric's real-time override.
func (s *Store) SetOverride(ctx context.Context, id int64, value *float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, m := s.findMetric(id)
	if m == nil {
		return ErrNotFound
	}
	if value == nil {
		m.Override = nil
	} else {
		v := *value
		m.Override = &v
	}
	return s.saveGroup(g)
}

// DeleteMetric removes a metric from its group.
func (s *Store) DeleteMetric(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for gi := range s.groups {
		ms := s.groups[gi].Metrics
		for mi := range ms {
			if ms[mi].ID == id {
				s.groups[gi].Metrics = append(ms[:mi], ms[mi+1:]...)
				return s.saveGroup(&s.groups[gi])
			}
		}
	}
	return ErrNotFound
}

// CountMetrics returns the total number of metric definitions.
func (s *Store) CountMetrics(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for i := range s.groups {
		n += len(s.groups[i].Metrics)
	}
	return n, nil
}

// findMetric returns the owning group and a pointer to the metric (or nils).
func (s *Store) findMetric(id int64) (*metrics.Group, *metrics.Metric) {
	for gi := range s.groups {
		for mi := range s.groups[gi].Metrics {
			if s.groups[gi].Metrics[mi].ID == id {
				return &s.groups[gi], &s.groups[gi].Metrics[mi]
			}
		}
	}
	return nil, nil
}
