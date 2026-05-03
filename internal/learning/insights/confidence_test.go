package insights

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConfidence_ZeroObservations returns 0.
func TestConfidence_ZeroObservations(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 0.0, CalculateConfidence(0, 0.5, 1.0))
	assert.Equal(t, 0.0, DefaultConfidence(0, 0.5))
}

// TestConfidence_HighObservationsLowVariance approaches 1.0.
func TestConfidence_HighObservationsLowVariance(t *testing.T) {
	t.Parallel()
	// N=10, σ²=0.1, p=1 → 10/(10+0.1) ≈ 0.990
	c := CalculateConfidence(10, 0.1, 1.0)
	assert.InDelta(t, 0.990, c, 0.001)
}

// TestConfidence_LowObservationsHighVariance produces low score.
func TestConfidence_LowObservationsHighVariance(t *testing.T) {
	t.Parallel()
	// N=3, σ²=1.0, p=1 → 3/(3+1) = 0.75
	c := DefaultConfidence(3, 1.0)
	assert.InDelta(t, 0.75, c, 0.001)
}

// TestConfidence_ZeroVarianceSingleObservation returns 1.0 (certain single event).
func TestConfidence_ZeroVarianceSingleObservation(t *testing.T) {
	t.Parallel()
	// N=1, σ²=0, p=1 → 1/(1+0) = 1.0
	c := CalculateConfidence(1, 0.0, 1.0)
	assert.Equal(t, 1.0, c)
}

// TestConfidence_NeverExceedsOne guarantees the [0,1] range.
func TestConfidence_NeverExceedsOne(t *testing.T) {
	t.Parallel()
	// Very high N, zero variance → should cap at 1.0.
	c := CalculateConfidence(1000, 0.0, 1.0)
	assert.LessOrEqual(t, c, 1.0)
	assert.GreaterOrEqual(t, c, 0.0)
}

// TestConfidence_FormulaMatchesSpec verifies spec §6.5 example values.
func TestConfidence_FormulaMatchesSpec(t *testing.T) {
	t.Parallel()
	tests := []struct {
		n        int
		variance float64
		penalty  float64
		expected float64
	}{
		// spec §6.5: N=3, σ²=0.1, p=1 → 3/(3+0.1) ≈ 0.968
		{3, 0.1, 1.0, 3.0 / 3.1},
		// spec §6.5: N=3, σ²=1.0, p=1 → 0.750
		{3, 1.0, 1.0, 0.750},
		// spec §6.5: N=1, σ²=0, p=1 → 1.0
		{1, 0.0, 1.0, 1.0},
	}
	for _, tc := range tests {
		got := CalculateConfidence(tc.n, tc.variance, tc.penalty)
		assert.InDelta(t, tc.expected, got, 0.001)
	}
}
