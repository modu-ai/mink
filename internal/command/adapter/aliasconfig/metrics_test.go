// Package aliasconfig — metrics.go tests (T-007).
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 AC-AMEND-021, AC-AMEND-022
package aliasconfig

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// fakeMetrics is a thread-safe spy implementation of the Metrics interface.
// Records each method invocation for assertion.
type fakeMetrics struct {
	mu sync.Mutex

	loadCountSuccess int
	loadCountFailure int
	validationErrs   []string
	loadDurations    []time.Duration
	entryCounts      []int
}

func (m *fakeMetrics) IncLoadCount(success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if success {
		m.loadCountSuccess++
	} else {
		m.loadCountFailure++
	}
}

func (m *fakeMetrics) IncValidationError(code string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validationErrs = append(m.validationErrs, code)
}

func (m *fakeMetrics) RecordLoadDuration(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadDurations = append(m.loadDurations, d)
}

func (m *fakeMetrics) ObserveEntryCount(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entryCounts = append(m.entryCounts, n)
}

func (m *fakeMetrics) snapshot() (success, failure, validationErrs, durations int, entryCounts []int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ec := make([]int, len(m.entryCounts))
	copy(ec, m.entryCounts)
	return m.loadCountSuccess, m.loadCountFailure, len(m.validationErrs), len(m.loadDurations), ec
}

// writeMetricsAliasFile writes a yaml file under a temp dir for metrics tests.
func writeMetricsAliasFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "aliases.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write alias file: %v", err)
	}
	return p
}

// TestMetrics_LoadDefaultEmitsCountAndDuration verifies that a successful
// LoadDefault triggers IncLoadCount(true), RecordLoadDuration once, and
// ObserveEntryCount with the result map size.
// AC-AMEND-021 — REQ-AMEND-021.
func TestMetrics_LoadDefaultEmitsCountAndDuration(t *testing.T) {
	yaml := "aliases:\n  opus: anthropic/claude-opus-4-7\n  sonnet: anthropic/claude-sonnet-4-6\n"
	path := writeMetricsAliasFile(t, yaml)
	spy := &fakeMetrics{}

	loader := New(Options{ConfigPath: path, Metrics: spy})
	result, err := loader.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}

	success, failure, _, durations, entryCounts := spy.snapshot()
	if success != 1 {
		t.Errorf("loadCountSuccess = %d, want 1", success)
	}
	if failure != 0 {
		t.Errorf("loadCountFailure = %d, want 0", failure)
	}
	if durations != 1 {
		t.Errorf("RecordLoadDuration calls = %d, want 1", durations)
	}
	if len(entryCounts) != 1 || entryCounts[0] != len(result) {
		t.Errorf("entryCounts = %v, want [%d]", entryCounts, len(result))
	}
}

// TestMetrics_NilOptionsMetricsUsesNoop verifies the nil → noopMetrics fallback.
// LoadDefault must not panic and must not emit any spy events because the
// caller provided nil metrics.
// AC-AMEND-022 — REQ-AMEND-022.
func TestMetrics_NilOptionsMetricsUsesNoop(t *testing.T) {
	yaml := "aliases:\n  opus: anthropic/claude-opus-4-7\n"
	path := writeMetricsAliasFile(t, yaml)

	loader := New(Options{ConfigPath: path, Metrics: nil})
	if _, err := loader.LoadDefault(); err != nil {
		t.Fatalf("LoadDefault with nil metrics: %v", err)
	}
	// Calling additional Loader methods to confirm noop fallback never panics.
	_, _ = loader.Load()
	_, _ = loader.LoadEntries()
}

// TestMetrics_NoopZeroAlloc verifies that the noop default contributes
// no per-call allocation when used directly — protects against accidental
// boxing on the hot path.
// NFR derived from REQ-AMEND-022.
func TestMetrics_NoopZeroAlloc(t *testing.T) {
	var m Metrics = noopMetrics{}
	allocs := testing.AllocsPerRun(100, func() {
		m.IncLoadCount(true)
		m.IncValidationError("ALIAS-010")
		m.RecordLoadDuration(time.Millisecond)
		m.ObserveEntryCount(3)
	})
	if allocs != 0 {
		t.Errorf("noopMetrics allocs/run = %.1f, want 0", allocs)
	}
}

// TestMetrics_NoopMethodsAreNoOps explicitly invokes each noopMetrics method
// to ensure coverage instrumentation records them and to assert that none
// panic on representative inputs.
// AC-AMEND-022 (coverage governance).
func TestMetrics_NoopMethodsAreNoOps(t *testing.T) {
	n := noopMetrics{}
	n.IncLoadCount(true)
	n.IncLoadCount(false)
	n.IncValidationError("ALIAS-010")
	n.IncValidationError("")
	n.RecordLoadDuration(0)
	n.RecordLoadDuration(time.Hour)
	n.ObserveEntryCount(0)
	n.ObserveEntryCount(1024)
}
