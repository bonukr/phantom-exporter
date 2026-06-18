// Package store persists metric groups in a settings directory: one YAML file
// per group, named after the group's path (e.g. settings/demo.yml).
//
// The whole set of groups is kept in memory (guarded by a mutex). Each mutation
// rewrites (or removes/renames) only the affected group's file, so the GUI keeps
// full CRUD capability while the source of truth remains human-editable YAML.
package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/bonukr/phantom-exporter/internal/metrics"
)

// ErrNotFound is returned when a requested group or metric does not exist.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when a unique constraint (e.g. group path) is violated.
var ErrConflict = errors.New("conflict")

// Store is a settings-directory backed metric store.
type Store struct {
	mu     sync.Mutex
	dir    string
	log    *slog.Logger
	groups []metrics.Group

	nextGroupID  int64
	nextMetricID int64
}

// New loads every group file from the settings directory. If the directory is
// missing or contains no group files, it is created with default demo data.
func New(ctx context.Context, dir string, log *slog.Logger) (*Store, error) {
	s := &Store{dir: dir, log: log, nextGroupID: 1, nextMetricID: 1}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create settings dir: %w", err)
	}

	loaded, err := s.load()
	if err != nil {
		return nil, err
	}

	if loaded == 0 {
		s.groups = defaultGroups()
		s.assignIDs()
		for i := range s.groups {
			if err := s.saveGroup(&s.groups[i]); err != nil {
				return nil, err
			}
		}
		log.Info("no settings found, created defaults", "dir", dir, "groups", len(s.groups))
		return s, nil
	}

	log.Info("settings loaded", "dir", dir, "groups", len(s.groups))
	return s, nil
}

// Ping reports settings availability (always healthy once loaded).
func (s *Store) Ping(ctx context.Context) error { return nil }

// Source returns a human-readable description of the config source.
func (s *Store) Source() string { return s.dir }

// Close is a no-op for the YAML store (kept for API compatibility).
func (s *Store) Close() {}

// ---- persistence ----

// load reads all *.yml/*.yaml files in the settings dir into memory and returns
// the number of group files read.
func (s *Store) load() (int, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0, fmt.Errorf("read settings dir: %w", err)
	}

	var groups []metrics.Group
	for _, e := range entries {
		if e.IsDir() || !isYAML(e.Name()) {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return 0, fmt.Errorf("read %s: %w", path, err)
		}
		var g metrics.Group
		if err := yaml.Unmarshal(data, &g); err != nil {
			return 0, fmt.Errorf("parse %s: %w", path, err)
		}
		groups = append(groups, g)
	}

	sort.Slice(groups, func(i, j int) bool { return groups[i].Path < groups[j].Path })
	s.groups = groups
	s.assignIDs()
	return len(groups), nil
}

// saveGroup writes a single group to settings/<path>.yml atomically.
func (s *Store) saveGroup(g *metrics.Group) error {
	data, err := yaml.Marshal(g)
	if err != nil {
		return fmt.Errorf("marshal group %q: %w", g.Path, err)
	}
	file := s.fileFor(g.Path)
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", file, err)
	}
	if err := os.Rename(tmp, file); err != nil {
		return fmt.Errorf("replace %s: %w", file, err)
	}
	return nil
}

// removeGroupFile deletes the file backing a group path (ignores missing file).
func (s *Store) removeGroupFile(path string) error {
	if err := os.Remove(s.fileFor(path)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove settings file for %q: %w", path, err)
	}
	return nil
}

func (s *Store) fileFor(path string) string {
	return filepath.Join(s.dir, path+".yml")
}

func isYAML(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml")
}

// assignIDs fills in missing IDs and computes next-ID counters, keeping each
// metric's GroupID consistent with the group it lives in.
func (s *Store) assignIDs() {
	var maxG, maxM int64
	for gi := range s.groups {
		if s.groups[gi].ID > maxG {
			maxG = s.groups[gi].ID
		}
		for mi := range s.groups[gi].Metrics {
			if s.groups[gi].Metrics[mi].ID > maxM {
				maxM = s.groups[gi].Metrics[mi].ID
			}
		}
	}
	nextG, nextM := maxG+1, maxM+1
	for gi := range s.groups {
		g := &s.groups[gi]
		if g.ID == 0 {
			g.ID = nextG
			nextG++
		}
		for mi := range g.Metrics {
			m := &g.Metrics[mi]
			if m.ID == 0 {
				m.ID = nextM
				nextM++
			}
			m.GroupID = g.ID
			if m.Labels == nil {
				m.Labels = []metrics.Label{}
			}
		}
	}
	s.nextGroupID, s.nextMetricID = nextG, nextM
}

// ---- helpers ----

func cloneGroup(g metrics.Group) metrics.Group {
	out := g
	out.Metrics = make([]metrics.Metric, len(g.Metrics))
	for i, m := range g.Metrics {
		out.Metrics[i] = cloneMetric(m)
	}
	return out
}

func cloneMetric(m metrics.Metric) metrics.Metric {
	out := m
	out.Labels = append([]metrics.Label(nil), m.Labels...)
	if m.Override != nil {
		v := *m.Override
		out.Override = &v
	}
	return out
}

func (s *Store) findGroup(id int64) *metrics.Group {
	for i := range s.groups {
		if s.groups[i].ID == id {
			return &s.groups[i]
		}
	}
	return nil
}

func defaultGroups() []metrics.Group {
	return []metrics.Group{{
		Path: "demo",
		Name: "Demo metrics",
		Metrics: []metrics.Metric{
			{
				Name: "http_requests_total", Type: metrics.TypeCounter,
				Description: "Total number of HTTP requests",
				ValueMode:   metrics.ModeRange, MinValue: 1, MaxValue: 10,
				Labels: []metrics.Label{{Key: "method", Value: "GET"}, {Key: "code", Value: "200"}},
			},
			{
				Name: "temperature_celsius", Type: metrics.TypeGauge,
				Description: "Simulated temperature in Celsius",
				ValueMode:   metrics.ModeRange, MinValue: 10, MaxValue: 30,
				Labels: []metrics.Label{{Key: "sensor", Value: "room-1"}},
			},
			{
				Name: "memory_usage_bytes", Type: metrics.TypeGauge,
				Description: "Simulated memory usage in bytes",
				ValueMode:   metrics.ModeRange, MinValue: 100_000_000, MaxValue: 900_000_000,
				Labels: []metrics.Label{},
			},
			{
				Name: "up", Type: metrics.TypeGauge,
				Description: "Whether the target is up (1) or down (0)",
				ValueMode:   metrics.ModeFixed, FixedValue: 1,
				Labels: []metrics.Label{},
			},
		},
	}}
}
