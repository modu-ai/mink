// Package metrics_test contains compile-time and reflection contract assertions
// verifying that all Sink implementations satisfy the metrics.Sink interface.
// SPEC-GOOSE-OBS-METRICS-001 T-003 (AC-018, audit D3 mandatory).
//
// The expvar assertions (lines referencing expvar.Sink) intentionally fail
// to compile until T-006 closes the expvar backend implementation.
package metrics_test

import (
	"reflect"
	"testing"

	"github.com/modu-ai/mink/internal/observability/metrics"
	metricsexpvar "github.com/modu-ai/mink/internal/observability/metrics/expvar"
	"github.com/modu-ai/mink/internal/observability/metrics/noop"
	"go.uber.org/zap"
)

// Compile-time assertions: both Sink implementations satisfy metrics.Sink.
// If either assertion fails the package will not compile.
var (
	_ metrics.Sink = metricsexpvar.New(zap.NewNop())
	_ metrics.Sink = noop.New()
)

// TestSink_MethodSetExact uses reflection to verify that the Sink interface
// has exactly the three factory methods and no lifecycle methods (REQ-016).
// This protects against accidental addition of Init/Shutdown/Flush etc.
func TestSink_MethodSetExact(t *testing.T) {
	t.Parallel()

	sinkType := reflect.TypeOf((*metrics.Sink)(nil)).Elem()
	const wantMethods = 3
	if sinkType.NumMethod() != wantMethods {
		t.Errorf("Sink has %d method(s), want exactly %d (Counter, Histogram, Gauge)",
			sinkType.NumMethod(), wantMethods)
	}

	methodNames := make(map[string]bool, wantMethods)
	for i := range sinkType.NumMethod() {
		methodNames[sinkType.Method(i).Name] = true
	}

	required := []string{"Counter", "Histogram", "Gauge"}
	for _, name := range required {
		if !methodNames[name] {
			t.Errorf("Sink is missing required method %q", name)
		}
	}

	forbidden := []string{"Init", "Shutdown", "Flush", "Close", "Start", "Stop"}
	for _, name := range forbidden {
		if methodNames[name] {
			t.Errorf("Sink has forbidden lifecycle method %q", name)
		}
	}
}

// TestExpvarSink_ImplementsSink verifies expvar.New returns a valid Sink at
// runtime and that it is not nil (AC-004).
func TestExpvarSink_ImplementsSink(t *testing.T) {
	t.Parallel()
	s := metricsexpvar.New(zap.NewNop())
	if s == nil {
		t.Fatal("expvar.New returned nil")
	}
}

// TestNoopSink_ImplementsSink verifies noop.New returns a valid Sink at
// runtime (AC-005) — duplicate here for completeness of contract_test.
func TestContractNoopSink_ImplementsSink(t *testing.T) {
	t.Parallel()
	s := noop.New()
	if s == nil {
		t.Fatal("noop.New returned nil")
	}
}
