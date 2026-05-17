// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package retrieval

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDecayFactor_zeroElapsed(t *testing.T) {
	// When createdAt == now, elapsed == 0 → result must be 1.0.
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	got := DecayFactor(now, now, DefaultDecayHalfLife)
	assert.InDelta(t, 1.0, got, 1e-9)
}

func TestDecayFactor_halfLifeElapsed(t *testing.T) {
	// At exactly one half-life elapsed, factor ≈ 0.5.
	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	createdAt := now.Add(-DefaultDecayHalfLife)
	got := DecayFactor(createdAt, now, DefaultDecayHalfLife)
	assert.InDelta(t, 0.5, got, 1e-9, "at one half-life elapsed, factor must be ~0.5")
}

func TestDecayFactor_twoHalfLivesElapsed(t *testing.T) {
	// At two half-lives, factor ≈ 0.25.
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	createdAt := now.Add(-2 * DefaultDecayHalfLife)
	got := DecayFactor(createdAt, now, DefaultDecayHalfLife)
	assert.InDelta(t, 0.25, got, 1e-9, "at two half-lives elapsed, factor must be ~0.25")
}

func TestDecayFactor_futureTimestamp(t *testing.T) {
	// createdAt is in the future → elapsed < 0 → no penalty → 1.0.
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	createdAt := now.Add(24 * time.Hour) // one day in the future
	got := DecayFactor(createdAt, now, DefaultDecayHalfLife)
	assert.InDelta(t, 1.0, got, 1e-9, "future timestamp must return 1.0")
}

func TestDecayFactor_halfLifeZero(t *testing.T) {
	// halfLife == 0 → decay disabled → 1.0.
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	createdAt := now.Add(-365 * 24 * time.Hour)
	got := DecayFactor(createdAt, now, 0)
	assert.InDelta(t, 1.0, got, 1e-9, "halfLife=0 must return 1.0 (decay disabled)")
}

func TestDecayFactor_negativeHalfLife(t *testing.T) {
	// Negative halfLife is treated as disabled → 1.0.
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	createdAt := now.Add(-7 * 24 * time.Hour)
	got := DecayFactor(createdAt, now, -DefaultDecayHalfLife)
	assert.InDelta(t, 1.0, got, 1e-9, "negative halfLife must return 1.0")
}

func TestDecayFactor_veryOld(t *testing.T) {
	// 10 years old with 30-day half-life → factor ≈ 0.
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	createdAt := now.Add(-10 * 365 * 24 * time.Hour)
	got := DecayFactor(createdAt, now, DefaultDecayHalfLife)
	assert.True(t, got < 1e-6, "10-year-old chunk must have near-zero decay factor, got %g", got)
	assert.True(t, got >= 0.0, "decay factor must not be negative")
}

func TestDecayFactor_deterministic(t *testing.T) {
	// Same inputs always produce the same output.
	now := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	createdAt := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	got1 := DecayFactor(createdAt, now, DefaultDecayHalfLife)
	got2 := DecayFactor(createdAt, now, DefaultDecayHalfLife)
	assert.InDelta(t, got1, got2, 1e-15, "DecayFactor must be deterministic")
}

func TestDecayFactor_resultBoundedToUnitInterval(t *testing.T) {
	// Sanity: result is always in [0, 1] for a range of inputs.
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, days := range []int{0, 1, 7, 30, 90, 180, 365, 3650} {
		createdAt := now.Add(-time.Duration(days) * 24 * time.Hour)
		got := DecayFactor(createdAt, now, DefaultDecayHalfLife)
		assert.True(t, got >= 0.0 && got <= 1.0,
			"DecayFactor(%d days) = %g, expected [0,1]", days, got)
	}
}

func TestDecayFactor_smallElapsed_resultNearOne(t *testing.T) {
	// Very small elapsed (1 second) with default 30-day half-life → result very close to 1.
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-1 * time.Second)
	got := DecayFactor(createdAt, now, DefaultDecayHalfLife)
	assert.True(t, got > 0.999 && got <= 1.0,
		"1-second-old chunk with 30-day half-life must have factor > 0.999, got %g", got)
}

func TestDecayFactor_customHalfLife(t *testing.T) {
	// Verify formula with a custom half-life of 7 days.
	halfLife := 7 * 24 * time.Hour
	now := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	createdAt := now.Add(-halfLife)
	got := DecayFactor(createdAt, now, halfLife)
	assert.InDelta(t, 0.5, got, 1e-9, "custom 7-day half-life at 7 days elapsed must be ~0.5")

	// At 14 days with 7-day half-life → ~0.25.
	createdAt2 := now.Add(-2 * halfLife)
	got2 := DecayFactor(createdAt2, now, halfLife)
	assert.InDelta(t, 0.25, got2, 1e-9, "custom 7-day half-life at 14 days elapsed must be ~0.25")

	// Verify exponential: ln(factor) / elapsed = -ln(2) / halfLife.
	got3 := DecayFactor(now.Add(-30*24*time.Hour), now, halfLife)
	expected := math.Exp(-math.Log(2) * 30 * 24 / (7 * 24))
	assert.InDelta(t, expected, got3, 1e-9)
}
