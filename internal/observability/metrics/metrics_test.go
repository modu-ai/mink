// Package metrics_test verifies the interface contract for the metrics package.
// SPEC-GOOSE-OBS-METRICS-001 T-001.
package metrics_test

import (
	"reflect"
	"testing"

	"github.com/modu-ai/goose/internal/observability/metrics"
)

// TestSink_InterfaceMethodSet asserts that Sink exposes exactly the three
// factory methods required by REQ-OBS-METRICS-002 and no others.
// This is a compile-time + reflection check (AC-002).
func TestSink_InterfaceMethodSet(t *testing.T) {
	t.Parallel()

	sinkType := reflect.TypeOf((*metrics.Sink)(nil)).Elem()
	want := map[string]bool{
		"Counter":   true,
		"Histogram": true,
		"Gauge":     true,
	}

	if sinkType.NumMethod() != len(want) {
		t.Errorf("Sink has %d method(s), want %d", sinkType.NumMethod(), len(want))
	}
	for i := range sinkType.NumMethod() {
		name := sinkType.Method(i).Name
		if !want[name] {
			t.Errorf("Sink has unexpected method %q", name)
		}
	}
}

// TestCounter_InterfaceMethodSet verifies Counter has exactly Inc and Add (AC-003).
func TestCounter_InterfaceMethodSet(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf((*metrics.Counter)(nil)).Elem()
	want := map[string]bool{"Inc": true, "Add": true}

	if typ.NumMethod() != len(want) {
		t.Errorf("Counter has %d method(s), want %d", typ.NumMethod(), len(want))
	}
	for i := range typ.NumMethod() {
		name := typ.Method(i).Name
		if !want[name] {
			t.Errorf("Counter has unexpected method %q", name)
		}
	}
}

// TestHistogram_InterfaceMethodSet verifies Histogram has exactly Observe (AC-003).
func TestHistogram_InterfaceMethodSet(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf((*metrics.Histogram)(nil)).Elem()
	want := map[string]bool{"Observe": true}

	if typ.NumMethod() != len(want) {
		t.Errorf("Histogram has %d method(s), want %d", typ.NumMethod(), len(want))
	}
}

// TestGauge_InterfaceMethodSet verifies Gauge has exactly Set and Add (AC-003).
func TestGauge_InterfaceMethodSet(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf((*metrics.Gauge)(nil)).Elem()
	want := map[string]bool{"Set": true, "Add": true}

	if typ.NumMethod() != len(want) {
		t.Errorf("Gauge has %d method(s), want %d", typ.NumMethod(), len(want))
	}
	for i := range typ.NumMethod() {
		name := typ.Method(i).Name
		if !want[name] {
			t.Errorf("Gauge has unexpected method %q", name)
		}
	}
}

// TestLabels_IsMapStringString verifies the Labels type is map[string]string (AC-001).
func TestLabels_IsMapStringString(t *testing.T) {
	t.Parallel()

	labelsType := reflect.TypeOf(metrics.Labels{})
	if labelsType.Kind() != reflect.Map {
		t.Fatalf("Labels is not a map, got %s", labelsType.Kind())
	}
	if labelsType.Key().Kind() != reflect.String {
		t.Errorf("Labels key is not string, got %s", labelsType.Key().Kind())
	}
	if labelsType.Elem().Kind() != reflect.String {
		t.Errorf("Labels value is not string, got %s", labelsType.Elem().Kind())
	}
}
