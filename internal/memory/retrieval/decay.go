// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package retrieval

import (
	"math"
	"time"
)

// DefaultDecayHalfLife is the default half-life for the temporal decay function.
// At this elapsed duration, DecayFactor returns approximately 0.5.
const DefaultDecayHalfLife = 30 * 24 * time.Hour

// DecayFactor returns exp(-elapsed / halfLife) bounded to [0, 1].
//
// Special cases:
//   - elapsed <= 0 (future timestamp) → 1.0 (no penalty)
//   - halfLife <= 0 → 1.0 (decay disabled)
//   - elapsed = halfLife → ~0.5 (within float64 epsilon)
//
// The function is pure and deterministic given fixed inputs.
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T4.1
// REQ:  REQ-MEM-009 (AC-MEM-009 temporal_factor)
func DecayFactor(createdAt time.Time, now time.Time, halfLife time.Duration) float64 {
	// Decay disabled when halfLife is non-positive.
	if halfLife <= 0 {
		return 1.0
	}

	elapsed := now.Sub(createdAt)

	// No penalty for future timestamps.
	if elapsed <= 0 {
		return 1.0
	}

	// exp(-elapsed / halfLife).
	// Using ln(2) / halfLife as the decay constant so that
	// at elapsed == halfLife the result is exactly exp(-ln(2)) == 0.5.
	decayConst := math.Log(2) / halfLife.Seconds()
	factor := math.Exp(-decayConst * elapsed.Seconds())

	// Clamp to [0, 1] to guard against floating-point edge cases.
	if factor > 1.0 {
		return 1.0
	}
	if factor < 0.0 {
		return 0.0
	}
	return factor
}
