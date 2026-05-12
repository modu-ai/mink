// Package noop_test — benchmark tests for noop adapter.
// SPEC-GOOSE-OBS-METRICS-001 T-009.
package noop_test

import (
	"testing"

	"github.com/modu-ai/mink/internal/observability/metrics/noop"
)

// BenchmarkNoopSink_CounterInc measures the overhead of noop.Counter.Inc().
// Target: <= 5ns/op (NFR-OBS-METRICS-003, AC-012).
//
// Handle caching is recommended for hot paths: obtain the Counter handle once
// and reuse it across calls to avoid repeated factory overhead.
func BenchmarkNoopSink_CounterInc(b *testing.B) {
	s := noop.New()
	c := s.Counter("bench.counter", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		c.Inc()
	}
}

// BenchmarkNoopSink_CounterAdd measures noop.Counter.Add() overhead.
func BenchmarkNoopSink_CounterAdd(b *testing.B) {
	s := noop.New()
	c := s.Counter("bench.counter.add", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		c.Add(1.0)
	}
}

// BenchmarkNoopSink_HistogramObserve measures noop.Histogram.Observe() overhead.
func BenchmarkNoopSink_HistogramObserve(b *testing.B) {
	s := noop.New()
	h := s.Histogram("bench.hist", nil, nil)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		h.Observe(42.0)
	}
}

// BenchmarkNoopSink_GaugeSet measures noop.Gauge.Set() overhead.
func BenchmarkNoopSink_GaugeSet(b *testing.B) {
	s := noop.New()
	g := s.Gauge("bench.gauge", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		g.Set(1.0)
	}
}
