// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-004
// AC: AC-BR-004
// M5-T2 — OTel instrument registration + atomic snapshot fidelity.

package bridge

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// newTestMetrics builds a bridgeMetrics backed by an in-memory SDK reader
// so tests can assert against collected metric data.
func newTestMetrics(t *testing.T, reg *Registry, gate *flushGate) (*bridgeMetrics, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")
	m, err := newBridgeMetrics(reg, gate, meter)
	if err != nil {
		t.Fatalf("newBridgeMetrics: %v", err)
	}
	t.Cleanup(func() {
		_ = m.Close()
		_ = provider.Shutdown(context.Background())
	})
	return m, reader
}

func collect(t *testing.T, reader *sdkmetric.ManualReader) map[string]any {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	out := make(map[string]any)
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			out[m.Name] = m.Data
		}
	}
	return out
}

func sumInt64(data any) int64 {
	switch d := data.(type) {
	case metricdata.Sum[int64]:
		var total int64
		for _, p := range d.DataPoints {
			total += p.Value
		}
		return total
	case metricdata.Gauge[int64]:
		var total int64
		for _, p := range d.DataPoints {
			total += p.Value
		}
		return total
	default:
		return -1
	}
}

func TestMetrics_RegistersFiveInstruments(t *testing.T) {
	t.Parallel()
	m, reader := newTestMetrics(t, NewRegistry(), newFlushGate())
	// Trigger one Add per counter so the SDK keeps a data point.
	ctx := context.Background()
	m.RecordInbound(ctx, 1)
	m.RecordOutbound(ctx, 1)
	m.RecordReconnect(ctx, 1)

	got := collect(t, reader)
	for _, name := range []string{
		MetricInboundTotal,
		MetricOutboundTotal,
		MetricReconnectTotal,
		MetricSessionsActive,
		MetricFlushGateStalls,
	} {
		if _, ok := got[name]; !ok {
			t.Errorf("missing instrument: %s", name)
		}
	}
}

func TestMetrics_InboundCounterMatchesSnapshot(t *testing.T) {
	t.Parallel()
	m, reader := newTestMetrics(t, NewRegistry(), newFlushGate())
	ctx := context.Background()
	for range 7 {
		m.RecordInbound(ctx, 1)
	}
	if got := m.Snapshot().InboundMessagesTotal; got != 7 {
		t.Fatalf("snapshot inbound = %d, want 7", got)
	}
	if got := sumInt64(collect(t, reader)[MetricInboundTotal]); got != 7 {
		t.Fatalf("OTel inbound counter = %d, want 7", got)
	}
}

func TestMetrics_OutboundCounterMatchesSnapshot(t *testing.T) {
	t.Parallel()
	m, reader := newTestMetrics(t, NewRegistry(), newFlushGate())
	ctx := context.Background()
	for range 3 {
		m.RecordOutbound(ctx, 1)
	}
	if got := m.Snapshot().OutboundMessagesTotal; got != 3 {
		t.Fatalf("snapshot outbound = %d, want 3", got)
	}
	if got := sumInt64(collect(t, reader)[MetricOutboundTotal]); got != 3 {
		t.Fatalf("OTel outbound = %d, want 3", got)
	}
}

func TestMetrics_ReconnectCounterMatchesSnapshot(t *testing.T) {
	t.Parallel()
	m, reader := newTestMetrics(t, NewRegistry(), newFlushGate())
	ctx := context.Background()
	for range 5 {
		m.RecordReconnect(ctx, 1)
	}
	if got := m.Snapshot().ReconnectAttempts; got != 5 {
		t.Fatalf("snapshot reconnect = %d, want 5", got)
	}
	if got := sumInt64(collect(t, reader)[MetricReconnectTotal]); got != 5 {
		t.Fatalf("OTel reconnect = %d, want 5", got)
	}
}

func TestMetrics_ActiveSessionsObservesRegistry(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	m, reader := newTestMetrics(t, reg, newFlushGate())
	for i := range 3 {
		_ = reg.Add(WebUISession{ID: string(rune('a' + i))})
	}
	if got := m.Snapshot().ActiveSessions; got != 3 {
		t.Fatalf("snapshot active = %d, want 3", got)
	}
	if got := sumInt64(collect(t, reader)[MetricSessionsActive]); got != 3 {
		t.Fatalf("OTel active = %d, want 3", got)
	}
}

func TestMetrics_FlushGateStallsObservesGate(t *testing.T) {
	t.Parallel()
	gate := newFlushGate()
	m, reader := newTestMetrics(t, NewRegistry(), gate)
	// Force two stall transitions.
	gate.ObserveWrite("s", HighWatermarkBytes+1)
	gate.ObserveDrain("s", HighWatermarkBytes+1)
	gate.ObserveWrite("s", HighWatermarkBytes+1)
	if got := m.Snapshot().FlushGateStalls; got != 2 {
		t.Fatalf("snapshot stalls = %d, want 2", got)
	}
	if got := sumInt64(collect(t, reader)[MetricFlushGateStalls]); got != 2 {
		t.Fatalf("OTel stalls = %d, want 2", got)
	}
}

func TestMetrics_NilMeterFallsBackToGlobal(t *testing.T) {
	t.Parallel()
	m, err := newBridgeMetrics(NewRegistry(), newFlushGate(), nil)
	if err != nil {
		t.Fatalf("nil meter must succeed via global provider: %v", err)
	}
	defer func() { _ = m.Close() }()
	// Atomic snapshot still works without an exporter.
	m.RecordInbound(context.Background(), 4)
	if got := m.Snapshot().InboundMessagesTotal; got != 4 {
		t.Fatalf("snapshot inbound = %d, want 4", got)
	}
}

func TestMetrics_CloseIsIdempotent(t *testing.T) {
	t.Parallel()
	m, _ := newTestMetrics(t, NewRegistry(), newFlushGate())
	if err := m.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// Compile-time guard: ensure the metric.Meter import path is what the
// SDK provides; broken upgrades trip this assertion.
var _ metric.Meter = (sdkmetric.NewMeterProvider()).Meter("test")
