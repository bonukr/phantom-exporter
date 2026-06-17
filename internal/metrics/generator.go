package metrics

import (
	"math/rand"
	"sync"
)

// Generator produces simulated values for metrics. It keeps per-metric state
// (e.g. monotonically increasing counters) in memory and is safe for
// concurrent use.
type Generator struct {
	mu       sync.Mutex
	counters map[int64]float64
}

// NewGenerator returns an empty Generator.
func NewGenerator() *Generator {
	return &Generator{counters: make(map[int64]float64)}
}

// Value computes the value to expose for a metric for the current scrape.
// An override, when present, always takes precedence.
func (g *Generator) Value(m Metric) float64 {
	if m.Override != nil {
		// Keep counter state aligned with the override so it never decreases later.
		if m.Type == TypeCounter {
			g.mu.Lock()
			if *m.Override > g.counters[m.ID] {
				g.counters[m.ID] = *m.Override
			}
			g.mu.Unlock()
		}
		return *m.Override
	}

	if m.Type == TypeCounter {
		return g.advanceCounter(m)
	}
	return g.sample(m)
}

// advanceCounter increments and returns the monotonic counter for a metric.
func (g *Generator) advanceCounter(m Metric) float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counters[m.ID] += g.increment(m)
	return g.counters[m.ID]
}

// increment is the step added to a counter per scrape.
func (g *Generator) increment(m Metric) float64 {
	switch m.ValueMode {
	case ModeFixed:
		if m.FixedValue > 0 {
			return m.FixedValue
		}
		return 1
	case ModeRange:
		return m.MinValue + rand.Float64()*(m.MaxValue-m.MinValue)
	default: // ModeRandom
		return rand.Float64()
	}
}

// sample returns a one-off value for gauge/histogram/summary metrics.
func (g *Generator) sample(m Metric) float64 {
	switch m.ValueMode {
	case ModeFixed:
		return m.FixedValue
	case ModeRange:
		return m.MinValue + rand.Float64()*(m.MaxValue-m.MinValue)
	default: // ModeRandom
		return rand.Float64()
	}
}

// Reset clears any stored counter state for a metric (use on delete/update).
func (g *Generator) Reset(metricID int64) {
	g.mu.Lock()
	delete(g.counters, metricID)
	g.mu.Unlock()
}
