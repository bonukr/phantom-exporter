package metrics

import (
	"sort"
	"strconv"
	"strings"
)

// Render produces the Prometheus text exposition format for a group's metrics,
// using gen to compute each metric's current value.
func Render(g *Group, gen *Generator) string {
	var b strings.Builder

	// Group metrics by name so HELP/TYPE headers are emitted once per name.
	byName := map[string][]Metric{}
	order := []string{}
	for _, m := range g.Metrics {
		if _, ok := byName[m.Name]; !ok {
			order = append(order, m.Name)
		}
		byName[m.Name] = append(byName[m.Name], m)
	}
	sort.Strings(order)

	for _, name := range order {
		series := byName[name]
		first := series[0]
		if first.Description != "" {
			b.WriteString("# HELP ")
			b.WriteString(name)
			b.WriteByte(' ')
			b.WriteString(escapeHelp(first.Description))
			b.WriteByte('\n')
		}
		b.WriteString("# TYPE ")
		b.WriteString(name)
		b.WriteByte(' ')
		b.WriteString(string(first.Type))
		b.WriteByte('\n')

		for _, m := range series {
			b.WriteString(name)
			b.WriteString(renderLabels(m.Labels))
			b.WriteByte(' ')
			b.WriteString(strconv.FormatFloat(gen.Value(m), 'g', -1, 64))
			b.WriteByte('\n')
		}
	}

	return b.String()
}

func renderLabels(labels []Label) string {
	if len(labels) == 0 {
		return ""
	}
	parts := make([]string, 0, len(labels))
	for _, l := range labels {
		parts = append(parts, l.Key+`="`+escapeLabelValue(l.Value)+`"`)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func escapeHelp(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

func escapeLabelValue(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}
