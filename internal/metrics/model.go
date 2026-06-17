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
)

// Group is a set of metrics exposed under a single scrape path.
type Group struct {
	ID      int64    `json:"id"`
	Path    string   `json:"path"` // e.g. "app-a" -> served at /metrics/app-a
	Name    string   `json:"name"`
	Metrics []Metric `json:"metrics,omitempty"`
}

// Label is a single key/value pair attached to a metric.
type Label struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Metric is a single simulated metric definition.
type Metric struct {
	ID          int64     `json:"id"`
	GroupID     int64     `json:"groupId"`
	Name        string    `json:"name"`
	Type        Type      `json:"type"`
	Description string    `json:"description"`
	Labels      []Label   `json:"labels"`
	ValueMode   ValueMode `json:"valueMode"`
	MinValue    float64   `json:"minValue"`
	MaxValue    float64   `json:"maxValue"`
	FixedValue  float64   `json:"fixedValue"`
	// Override, when non-nil, forces the exposed value (set in real time via the GUI/API).
	Override *float64 `json:"override"`
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
	case ModeFixed, ModeRandom:
	case ModeRange:
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
