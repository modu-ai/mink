// Package noop_test verifies the noop metrics.Sink implementation.
// SPEC-GOOSE-OBS-METRICS-001 T-002.
package noop_test

import (
	"expvar"
	"testing"

	"github.com/modu-ai/mink/internal/observability/metrics"
	"github.com/modu-ai/mink/internal/observability/metrics/noop"
)

// TestNoopSink_AllOps_NoSideEffect verifies that all noop operations:
//   - return without panic,
//   - do not register any expvar variables (AC-011).
func TestNoopSink_AllOps_NoSideEffect(t *testing.T) {
	t.Parallel()

	// Snapshot expvar before any noop ops.
	varsBefore := map[string]struct{}{}
	expvar.Do(func(kv expvar.KeyValue) {
		varsBefore[kv.Key] = struct{}{}
	})

	s := noop.New()

	c := s.Counter("noop.counter", metrics.Labels{"op": "test"})
	c.Inc()
	c.Add(100.5)

	h := s.Histogram("noop.hist", nil, []float64{1, 10, 100})
	h.Observe(5.0)

	g := s.Gauge("noop.gauge", metrics.Labels{"k": "v"})
	g.Set(42.0)
	g.Add(-10.0)

	// Verify no new expvar variables were registered.
	expvar.Do(func(kv expvar.KeyValue) {
		if _, existed := varsBefore[kv.Key]; !existed {
			t.Errorf("noop sink unexpectedly registered expvar %q", kv.Key)
		}
	})
}

// TestNoopSink_ImplementsSink verifies noop.New() returns a non-nil Sink (AC-005).
func TestNoopSink_ImplementsSink(t *testing.T) {
	t.Parallel()
	s := noop.New()
	if s == nil {
		t.Fatal("noop.New() should not return nil")
	}
}

// TestNoopSink_NilLabels_NoPanic verifies nil labels do not panic (AC-005).
func TestNoopSink_NilLabels_NoPanic(t *testing.T) {
	t.Parallel()
	s := noop.New()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	s.Counter("x", nil).Inc()
	s.Histogram("x", nil, nil).Observe(1)
	s.Gauge("x", nil).Set(1)
}
