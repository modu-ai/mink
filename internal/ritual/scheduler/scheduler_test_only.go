//go:build test_only

// Package scheduler — FastForward symbol gated by the test_only build tag.
// SPEC-GOOSE-SCHEDULER-001 P4b T-030 / REQ-SCHED-020.
//
// This file is NOT linked into production binaries (no `test_only` tag in
// `go build` defaults). Callers must compile with `go test -tags=test_only`
// or `go build -tags=test_only` to access FastForward.
package scheduler

import (
	"time"

	"github.com/jonboulle/clockwork"
)

// FastForward advances the underlying clockwork.FakeClock by d. It is a no-op
// when the Scheduler was constructed with a real clock.
//
// REQ-SCHED-020 mandates that this symbol must NOT be present in a production
// binary. The build tag at the top of this file enforces that constraint.
func (s *Scheduler) FastForward(d time.Duration) {
	if fc, ok := s.clock.(clockwork.FakeClock); ok {
		fc.Advance(d)
	}
}
