// Package metrics provides a vendor-neutral metrics emission surface used
// across goose's runtime. Implementations include:
//   - expvar (default Phase 1 backend, stdlib-only)
//   - noop (zero-cost fallback)
//
// Future Phase 2/3 SPECs will add OpenTelemetry and Prometheus adapters.
// SPEC-GOOSE-OBS-METRICS-001.
package metrics

// Sink is the abstract metrics emission interface.
//
// Implementations MUST:
//   - Be thread-safe across all factory and handle methods.
//   - Cap label cardinality per metric name (silent drop on overflow).
//   - Avoid panics regardless of caller input (cardinality, label values).
//
// Implementations MUST NOT:
//   - Mutate, normalize, hash, or redact caller-provided labels.
//   - Require lifecycle calls (init/shutdown/flush) from callers.
//
// Caller responsibility (NOT enforced by Sink):
//   - PII firewall: caller must not include user prompts, model outputs,
//     credentials, raw user identifiers, or absolute home paths in labels.
//   - Cardinality discipline: caller should keep dynamic label combinations
//     bounded; Sink will silently drop overflow and emit a single warn log.
//
// @MX:ANCHOR: [AUTO] vendor-neutral metrics surface, fan_in >= 3 expected
// @MX:REASON: TELEMETRY-001 consumer, future router emission, and future loop
//
//	emission all depend on this interface boundary.
//
// @MX:SPEC: SPEC-GOOSE-OBS-METRICS-001 REQ-OBS-METRICS-002
type Sink interface {
	// Counter returns a monotonically-increasing counter handle for the given
	// (name, labels) combination. Repeated calls with the same (name, labels)
	// return a handle that accumulates into the same underlying counter.
	Counter(name string, labels Labels) Counter

	// Histogram returns a value-distribution observer for the given
	// (name, labels) combination. buckets defines the upper bounds of each
	// histogram bucket; nil or empty uses the backend default.
	Histogram(name string, labels Labels, buckets []float64) Histogram

	// Gauge returns an instantaneous-value handle for the given
	// (name, labels) combination.
	Gauge(name string, labels Labels) Gauge
}

// Counter is a monotonically-increasing metric handle. Thread-safe.
type Counter interface {
	// Inc increments the counter by 1.
	Inc()
	// Add increments the counter by delta. delta SHOULD be >= 0.
	Add(delta float64)
}

// Histogram is a value-distribution observer. Thread-safe.
// Note: percentile estimation is not supported in Phase 1 (expvar backend).
// Use Phase 2 OTel adapter for percentile aggregation.
type Histogram interface {
	// Observe records a single observation of value into the appropriate bucket.
	Observe(value float64)
}

// Gauge is a settable instantaneous-value handle. Thread-safe.
type Gauge interface {
	// Set replaces the current gauge value with value.
	Set(value float64)
	// Add adjusts the current gauge value by delta (may be negative).
	Add(delta float64)
}

// Labels is a static dimension map. PII and high-cardinality dynamic values
// (err.Error(), user IDs, prompt content) MUST NOT appear here per
// REQ-OBS-METRICS-013 (caller responsibility).
//
// Keys should be snake_case (e.g. "method", "error_type"); values should be
// static enum strings or backend-safe identifiers.
type Labels map[string]string
