// Package aliasconfig — Metrics interface and noop implementation.
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-021, REQ-AMEND-022
package aliasconfig

import "time"

// Metrics is the observability hook for aliasconfig operations.
// Zero-value Options.Metrics (nil) uses noopMetrics — no allocation in steady state.
//
// Implementations must be safe for concurrent use.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-021
type Metrics interface {
	// IncLoadCount increments the load attempt counter.
	// success=true on successful load; success=false on any error path.
	IncLoadCount(success bool)

	// IncValidationError increments the validation error counter.
	// code is the stable ErrorCode string (e.g. "ALIAS-010"), or empty for unrecognized errors.
	IncValidationError(code string)

	// RecordLoadDuration records the wall-clock duration of a single load operation.
	RecordLoadDuration(d time.Duration)

	// ObserveEntryCount records the number of alias entries returned by a successful load.
	ObserveEntryCount(n int)
}

// noopMetrics is the zero-cost default Metrics implementation used when
// Options.Metrics is nil. All methods are no-ops and add no allocation
// in steady state.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-022
type noopMetrics struct{}

func (noopMetrics) IncLoadCount(_ bool)                {}
func (noopMetrics) IncValidationError(_ string)        {}
func (noopMetrics) RecordLoadDuration(_ time.Duration) {}
func (noopMetrics) ObserveEntryCount(_ int)            {}
