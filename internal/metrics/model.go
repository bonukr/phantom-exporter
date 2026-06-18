// Package metrics defines the simulated-metric domain model and value generation.
package metrics

import (
	"fmt"
	"regexp"
)

// Type is a Prometheus metric type.
type Type string

const (
	TypeCounter   Type = "counter"
	TypeGauge     Type = "gauge"
	TypeHistogram Type = "histogram"
	TypeSummary   Type = "summary"
)

// ValueMode controls how a metric's value is simulated.
type ValueMode string

const (
	// ModeFixed always returns FixedValue.
	ModeFixed ValueMode = "fixed"
	// ModeRandom returns a random value in [0,1).
	ModeRandom ValueMode = "random"
	// ModeRange returns a random value in [MinValue, MaxValue].
	ModeRange ValueMode = "range"
	// ModeIncrement adds Step (default 1) every scrape, unbounded.
	ModeIncrement ValueMode = "increment"
	// ModeRamp adds Step (default 1) every scrape, wrapping within [MinValue, MaxValue] (sawtooth).
	ModeRamp ValueMode = "ramp"
	// ModeStep cycles through Step discrete levels evenly spaced in [MinValue, MaxValue] (staircase).
	ModeStep ValueMode = "step"
	// ModeRate changes linearly with elapsed wall-clock time: MinValue + Step per second
	// (Step > 0 increases, Step < 0 decreases).
	ModeRate ValueMode = "rate"
)

// Group is a set of metrics exposed under a single scrape path.
type Group struct {
	ID      int64    `json:"id" yaml:"id"`
	Path    string   `json:"path" yaml:"path"` // e.g. "app-a" -> served at /metrics/app-a
	Name    string   `json:"name" yaml:"name"`
	Metrics []Metric `json:"metrics,omitempty" yaml:"metrics,omitempty"`
}

// Label is a single key/value pair attached to a metric.
type Label struct {
	Key   string `json:"key" yaml:"key"`
	Value string `json:"value" yaml:"value"`
}

// Metric is a single simulated metric definition.
type Metric struct {
	ID          int64     `json:"id" yaml:"id"`
	GroupID     int64     `json:"groupId" yaml:"groupId"`
	Name        string    `json:"name" yaml:"name"`
	Type        Type      `json:"type" yaml:"type"`
	Description string    `json:"description" yaml:"description"`
	Labels      []Label   `json:"labels" yaml:"labels"`
	ValueMode   ValueMode `json:"valueMode" yaml:"valueMode"`
	MinValue    float64   `json:"minValue" yaml:"minValue"`
	MaxValue    float64   `json:"maxValue" yaml:"maxValue"`
	FixedValue  float64   `json:"fixedValue" yaml:"fixedValue"`
	// Step is the increment per scrape (increment/ramp), the number of discrete
	// levels (step), or the per-second rate (rate). Defaults are applied when zero
	// for increment/ramp/step; rate uses Step as-is (may be negative).
	Step float64 `json:"step" yaml:"step"`
	// Override, when non-nil, forces the exposed value (set in real time via the GUI/API).
	Override *float64 `json:"override" yaml:"override,omitempty"`
}

var nameRe = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)

// Validate checks the metric definition for correctness.
func (m *Metric) Validate() error {
	if !nameRe.MatchString(m.Name) {
		return fmt.Errorf("invalid metric name %q (must match [a-zA-Z_:][a-zA-Z0-9_:]*)", m.Name)
	}
	switch m.Type {
	case TypeCounter, TypeGauge, TypeHistogram, TypeSummary:
	default:
		return fmt.Errorf("invalid metric type %q", m.Type)
	}
	switch m.ValueMode {
	case ModeFixed, ModeRandom, ModeIncrement, ModeRate:
	case ModeRange, ModeRamp, ModeStep:
		if m.MinValue > m.MaxValue {
			return fmt.Errorf("minValue (%g) must be <= maxValue (%g)", m.MinValue, m.MaxValue)
		}
	default:
		return fmt.Errorf("invalid value mode %q", m.ValueMode)
	}
	for _, l := range m.Labels {
		if !nameRe.MatchString(l.Key) {
			return fmt.Errorf("invalid label name %q", l.Key)
		}
	}
	return nil
}

// ValidatePath checks a group path is usable in a URL segment.
var pathRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// ValidatePath validates a group scrape path.
func ValidatePath(path string) error {
	if !pathRe.MatchString(path) {
		return fmt.Errorf("invalid path %q (allowed: letters, digits, . _ -)", path)
	}
	return nil
}
