// Package adapter — metrics helpers for ContextAdapter.
// SPEC: SPEC-GOOSE-CMDCTX-TELEMETRY-001
package adapter

import (
	"errors"
	"time"

	"github.com/modu-ai/mink/internal/command"
	"github.com/modu-ai/mink/internal/observability/metrics"
)

// Type aliases — single source of truth lives at OBS-METRICS-001.
// Callers use MetricsSink, Counter, Histogram, and Labels directly from
// this package; no need to import internal/observability/metrics separately.
//
// SPEC: SPEC-GOOSE-CMDCTX-TELEMETRY-001 REQ-CMDCTX-TEL-004
//
// @MX:ANCHOR: [AUTO] vendor-neutral metrics surface for cmdctx layer.
// @MX:REASON: MetricsSink is consumed by adapter.go (6 instrument calls),
//
//	metrics_test.go, and future CLI/daemon wiring — fan_in >= 3.
//
// @MX:SPEC: SPEC-GOOSE-CMDCTX-TELEMETRY-001 REQ-CMDCTX-TEL-004
type (
	// MetricsSink is an alias for the upstream sink contract.
	// nil sink triggers graceful emission skip per REQ-CMDCTX-TEL-009.
	MetricsSink = metrics.Sink
	// Counter is a monotonically-increasing metric handle.
	Counter = metrics.Counter
	// Histogram is a value-distribution observer.
	Histogram = metrics.Histogram
	// Labels is a static dimension map.
	Labels = metrics.Labels
)

// classifyError maps err to one of the three allowed error_type label values.
// REQ-CMDCTX-TEL-007 — only 3 static labels to prevent cardinality explosion
// (REQ-CMDCTX-TEL-013).
func classifyError(err error) string {
	switch {
	case errors.Is(err, command.ErrUnknownModel):
		return "ErrUnknownModel"
	case errors.Is(err, ErrLoopControllerUnavailable):
		return "ErrLoopControllerUnavailable"
	default:
		return "other"
	}
}

// instrumentVoid wraps a T-returning method (no error) with metrics emission.
// REQ-CMDCTX-TEL-001, REQ-CMDCTX-TEL-002, REQ-CMDCTX-TEL-009, REQ-CMDCTX-TEL-011.
//
// Nil-sink fast path: if a.metrics == nil, fn() is called directly with zero
// overhead (NFR-TEL-003 ≤ 10ns).
func instrumentVoid[T any](a *ContextAdapter, method string, fn func() T) T {
	if a.metrics == nil {
		return fn()
	}
	var result T
	safeEmit(a, func() {
		a.metrics.Counter("cmdctx.method.calls", metrics.Labels{"method": method}).Inc()
		start := time.Now()
		defer func() {
			a.metrics.Histogram(
				"cmdctx.method.duration_ms",
				metrics.Labels{"method": method},
				nil, // use backend default buckets
			).Observe(float64(time.Since(start).Milliseconds()))
		}()
		result = fn()
	})
	return result
}

// instrumentErr wraps a (T, error)-returning method with metrics emission.
// REQ-CMDCTX-TEL-001, REQ-CMDCTX-TEL-002, REQ-CMDCTX-TEL-006, REQ-CMDCTX-TEL-011.
func instrumentErr[T any](a *ContextAdapter, method string, fn func() (T, error)) (T, error) {
	if a.metrics == nil {
		return fn()
	}
	var (
		result T
		err    error
	)
	safeEmit(a, func() {
		a.metrics.Counter("cmdctx.method.calls", metrics.Labels{"method": method}).Inc()
		start := time.Now()
		defer func() {
			a.metrics.Histogram(
				"cmdctx.method.duration_ms",
				metrics.Labels{"method": method},
				nil,
			).Observe(float64(time.Since(start).Milliseconds()))
		}()
		result, err = fn()
		if err != nil {
			a.metrics.Counter("cmdctx.method.errors", metrics.Labels{
				"method":     method,
				"error_type": classifyError(err),
			}).Inc()
		}
	})
	return result, err
}

// safeEmit executes emitFn inside a deferred recover so that a panicking sink
// never propagates to the method caller.
// REQ-CMDCTX-TEL-011: if Logger is set, Warn is called exactly once on panic.
//
// @MX:WARN: [AUTO] recover() boundary; only emission code runs inside.
// @MX:REASON: Sink implementations may panic; caller method must remain
//
//	unaffected per REQ-CMDCTX-TEL-011.
func safeEmit(a *ContextAdapter, emitFn func()) {
	defer func() {
		if r := recover(); r != nil {
			if a.logger != nil {
				a.logger.Warn("metrics emission panic", "panic", r)
			}
		}
	}()
	emitFn()
}
