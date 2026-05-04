// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-004
// AC: AC-BR-004
// M5-T2 — OpenTelemetry metric instruments.
//
// Two-tier model:
//   - Atomic counters live inside bridgeMetrics regardless of OTel
//     availability. Bridge.Metrics() snapshots them so callers without
//     an OTel exporter still see live values.
//   - OTel instruments are registered against an injected metric.Meter
//     and observe the same atomic counters. ActiveSessions and
//     FlushGateStalls are surfaced as observable gauges/counters that
//     read from the registry and flush-gate respectively.
//
// Five instruments per spec.md §3.1 item 10 / REQ-BR-004:
//   - bridge.sessions.active            (observable up-down counter)
//   - bridge.messages.inbound.total     (counter)
//   - bridge.messages.outbound.total    (counter)
//   - bridge.flush_gate.stalls          (observable counter)
//   - bridge.reconnect.attempts         (counter)

package bridge

import (
	"context"
	"fmt"
	"sync/atomic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// metricNames holds the OTel metric names exported by bridgeMetrics. Kept
// as constants so dashboards and integration tests pin against a stable
// vocabulary.
const (
	MetricSessionsActive  = "bridge.sessions.active"
	MetricInboundTotal    = "bridge.messages.inbound.total"
	MetricOutboundTotal   = "bridge.messages.outbound.total"
	MetricFlushGateStalls = "bridge.flush_gate.stalls"
	MetricReconnectTotal  = "bridge.reconnect.attempts"
)

// bridgeMetrics owns the bridge's OTel instruments and the atomic
// counters that back them.
//
// @MX:ANCHOR
// @MX:REASON Cross-cutting observability surface — every transport,
// dispatcher, and auth path increments through this struct so the
// registered instruments stay consistent with the snapshot exposed
// by Bridge.Metrics().
type bridgeMetrics struct {
	registry *Registry
	gate     *flushGate

	// Atomic mirrors of the OTel counters; consulted by Snapshot.
	inbound   atomic.Uint64
	outbound  atomic.Uint64
	reconnect atomic.Uint64

	meter metric.Meter

	// OTel instrument handles.
	inboundCounter   metric.Int64Counter
	outboundCounter  metric.Int64Counter
	reconnectCounter metric.Int64Counter

	// Observable callback registration; Close releases it.
	registered metric.Registration
}

// newBridgeMetrics constructs the metrics holder. registry and gate may
// be nil; the observable callbacks return zero in that case.
//
// If meter is nil, otel.GetMeterProvider() is consulted. The default
// provider is a noop in unit tests, which is fine — instrument
// registration never fails for the noop meter.
func newBridgeMetrics(registry *Registry, gate *flushGate, meter metric.Meter) (*bridgeMetrics, error) {
	if meter == nil {
		meter = otel.GetMeterProvider().Meter("github.com/modu-ai/goose/internal/bridge")
	}
	m := &bridgeMetrics{
		registry: registry,
		gate:     gate,
		meter:    meter,
	}

	// Each instrument creation is wrapped with a context-rich error so
	// startup diagnostics show which instrument failed (CodeRabbit
	// Finding #1).
	var err error
	m.inboundCounter, err = meter.Int64Counter(MetricInboundTotal,
		metric.WithDescription("Total inbound messages accepted by the bridge"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return nil, fmt.Errorf("bridge: create inbound counter (%s): %w", MetricInboundTotal, err)
	}
	m.outboundCounter, err = meter.Int64Counter(MetricOutboundTotal,
		metric.WithDescription("Total outbound messages emitted by the bridge"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return nil, fmt.Errorf("bridge: create outbound counter (%s): %w", MetricOutboundTotal, err)
	}
	m.reconnectCounter, err = meter.Int64Counter(MetricReconnectTotal,
		metric.WithDescription("Total reconnect attempts observed at the bridge auth path"),
		metric.WithUnit("{attempt}"),
	)
	if err != nil {
		return nil, fmt.Errorf("bridge: create reconnect counter (%s): %w", MetricReconnectTotal, err)
	}

	active, err := meter.Int64ObservableGauge(MetricSessionsActive,
		metric.WithDescription("Number of currently registered Web UI sessions"),
		metric.WithUnit("{session}"),
	)
	if err != nil {
		return nil, fmt.Errorf("bridge: create observable gauge (%s): %w", MetricSessionsActive, err)
	}
	stalls, err := meter.Int64ObservableCounter(MetricFlushGateStalls,
		metric.WithDescription("Cumulative flush-gate high-watermark transitions since process start"),
		metric.WithUnit("{stall}"),
	)
	if err != nil {
		return nil, fmt.Errorf("bridge: create observable counter (%s): %w", MetricFlushGateStalls, err)
	}

	reg, err := meter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			if m.registry != nil {
				o.ObserveInt64(active, int64(m.registry.Len()))
			} else {
				o.ObserveInt64(active, 0)
			}
			if m.gate != nil {
				// Stalls() is monotonic; cumulative semantics matches the
				// observable counter contract.
				o.ObserveInt64(stalls, int64(m.gate.Stalls()))
			} else {
				o.ObserveInt64(stalls, 0)
			}
			return nil
		},
		active, stalls,
	)
	if err != nil {
		return nil, fmt.Errorf("bridge: register observable callback: %w", err)
	}
	m.registered = reg
	return m, nil
}

// RecordInbound increments the inbound message counter. Safe for
// concurrent use.
func (m *bridgeMetrics) RecordInbound(ctx context.Context, n int64) {
	if m == nil {
		return
	}
	m.inbound.Add(uint64(n))
	if m.inboundCounter != nil {
		m.inboundCounter.Add(ctx, n)
	}
}

// RecordOutbound increments the outbound message counter.
func (m *bridgeMetrics) RecordOutbound(ctx context.Context, n int64) {
	if m == nil {
		return
	}
	m.outbound.Add(uint64(n))
	if m.outboundCounter != nil {
		m.outboundCounter.Add(ctx, n)
	}
}

// RecordReconnect increments the reconnect attempts counter.
func (m *bridgeMetrics) RecordReconnect(ctx context.Context, n int64) {
	if m == nil {
		return
	}
	m.reconnect.Add(uint64(n))
	if m.reconnectCounter != nil {
		m.reconnectCounter.Add(ctx, n)
	}
}

// Snapshot returns a Metrics value reflecting the current counter state.
// Safe for concurrent use.
func (m *bridgeMetrics) Snapshot() Metrics {
	out := Metrics{
		InboundMessagesTotal:  m.inbound.Load(),
		OutboundMessagesTotal: m.outbound.Load(),
		ReconnectAttempts:     m.reconnect.Load(),
	}
	if m.registry != nil {
		out.ActiveSessions = int64(m.registry.Len())
	}
	if m.gate != nil {
		out.FlushGateStalls = m.gate.Stalls()
	}
	return out
}

// Close unregisters the observable callback. Safe to call on a
// double-Close — second call is a no-op.
func (m *bridgeMetrics) Close() error {
	if m == nil || m.registered == nil {
		return nil
	}
	err := m.registered.Unregister()
	m.registered = nil
	return err
}
