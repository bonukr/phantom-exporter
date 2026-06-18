package metrics

import (
	"math/rand"
	"sync"
	"time"
)

// Generator produces simulated values for metrics. It keeps per-metric state
// (monotonic counters, ramp/increment positions, staircase indexes, rate start
// times) in memory and is safe for concurrent use.
type Generator struct {
	mu       sync.Mutex
	counters map[int64]float64   // counter type accumulator
	values   map[int64]float64   // increment/ramp running value
	steps    map[int64]int       // step mode level index
	starts   map[int64]time.Time // rate mode start time
}

// NewGenerator returns an empty Generator.
func NewGenerator() *Generator {
	return &Generator{
		counters: make(map[int64]float64),
		values:   make(map[int64]float64),
		steps:    make(map[int64]int),
		starts:   make(map[int64]time.Time),
	}
}

// Value computes the value to expose for a metric for the current scrape.
// An override, when present, always takes precedence.
func (g *Generator) Value(m Metric) float64 {
	if m.Override != nil {
		if m.Type == TypeCounter {
			g.mu.Lock()
			if *m.Override > g.counters[m.ID] {
				g.counters[m.ID] = *m.Override
			}
			g.mu.Unlock()
		}
		return *m.Override
	}

	switch m.ValueMode {
	case ModeIncrement:
		return g.advanceIncrement(m)
	case ModeRamp:
		return g.advanceRamp(m)
	case ModeStep:
		return g.advanceStep(m)
	case ModeRate:
		return g.advanceRate(m)
	}

	if m.Type == TypeCounter {
		return g.advanceCounter(m)
	}
	return g.sample(m)
}

// advanceIncrement adds Step (default 1) every scrape without bound.
func (g *Generator) advanceIncrement(m Metric) float64 {
	step := m.Step
	if step == 0 {
		step = 1
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	v, ok := g.values[m.ID]
	if !ok {
		v = m.MinValue
	}
	v += step
	g.values[m.ID] = v
	return v
}

// advanceRamp adds Step (default 1) every scrape, wrapping to MinValue once it
// exceeds MaxValue (sawtooth).
func (g *Generator) advanceRamp(m Metric) float64 {
	step := m.Step
	if step == 0 {
		step = 1
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	v, ok := g.values[m.ID]
	if !ok {
		v = m.MinValue
	} else {
		v += step
		if v > m.MaxValue {
			v = m.MinValue
		}
	}
	g.values[m.ID] = v
	return v
}

// advanceStep cycles through Step discrete levels evenly spaced in
// [MinValue, MaxValue] (staircase), one level per scrape.
func (g *Generator) advanceStep(m Metric) float64 {
	n := int(m.Step)
	if n < 2 {
		n = 5
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	idx := g.steps[m.ID]
	if idx >= n {
		idx = 0
	}
	val := m.MinValue + float64(idx)*(m.MaxValue-m.MinValue)/float64(n-1)
	g.steps[m.ID] = (idx + 1) % n
	return val
}

// advanceRate returns MinValue plus Step units per elapsed second since the
// metric was first scraped (Step > 0 increases, Step < 0 decreases).
func (g *Generator) advanceRate(m Metric) float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	start, ok := g.starts[m.ID]
	if !ok {
		start = time.Now()
		g.starts[m.ID] = start
	}
	elapsed := time.Since(start).Seconds()
	return m.MinValue + m.Step*elapsed
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

// Reset clears any stored state for a metric (use on delete/update).
func (g *Generator) Reset(metricID int64) {
	g.mu.Lock()
	delete(g.counters, metricID)
	delete(g.values, metricID)
	delete(g.steps, metricID)
	delete(g.starts, metricID)
	g.mu.Unlock()
}
