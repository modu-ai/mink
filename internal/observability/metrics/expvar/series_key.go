// Package expvar — seriesKey computation.
// SPEC-GOOSE-OBS-METRICS-001.
package expvar

import (
	"sort"
	"strings"

	"github.com/modu-ai/goose/internal/observability/metrics"
)

// seriesKey builds a stable string key from a metric name and label map.
// Format: "name|key1=val1,key2=val2" with label pairs sorted lexicographically
// by key. Identical logical label sets always produce the same key regardless
// of map iteration order (REQ-OBS-METRICS-004).
//
// Labels are preserved verbatim; no normalization is applied (REQ-OBS-METRICS-014).
func seriesKey(name string, labels metrics.Labels) string {
	if len(labels) == 0 {
		return name
	}

	// Sort keys for deterministic ordering.
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString(name)
	sb.WriteByte('|')
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(labels[k])
	}
	return sb.String()
}
