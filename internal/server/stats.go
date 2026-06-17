package server

import (
	"sync"
	"time"
)

// Stats tracks in-memory service usage for the monitoring dashboard.
type Stats struct {
	mu           sync.Mutex
	startedAt    time.Time
	totalScrapes int64
	perPath      map[string]*pathStat
}

type pathStat struct {
	Scrapes  int64     `json:"scrapes"`
	LastTime time.Time `json:"lastTime"`
}

// NewStats returns an initialized Stats with the start time set to now.
func NewStats() *Stats {
	return &Stats{startedAt: time.Now(), perPath: make(map[string]*pathStat)}
}

// RecordScrape increments scrape counters for a given path.
func (s *Stats) RecordScrape(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalScrapes++
	ps := s.perPath[path]
	if ps == nil {
		ps = &pathStat{}
		s.perPath[path] = ps
	}
	ps.Scrapes++
	ps.LastTime = time.Now()
}

// Snapshot is a serializable view of current usage statistics.
type Snapshot struct {
	UptimeSeconds float64             `json:"uptimeSeconds"`
	StartedAt     time.Time           `json:"startedAt"`
	TotalScrapes  int64               `json:"totalScrapes"`
	PerPath       map[string]pathStat `json:"perPath"`
}

// Snapshot returns a copy of the current statistics.
func (s *Stats) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	perPath := make(map[string]pathStat, len(s.perPath))
	for k, v := range s.perPath {
		perPath[k] = *v
	}
	return Snapshot{
		UptimeSeconds: time.Since(s.startedAt).Seconds(),
		StartedAt:     s.startedAt,
		TotalScrapes:  s.totalScrapes,
		PerPath:       perPath,
	}
}
