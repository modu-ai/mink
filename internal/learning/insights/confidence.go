package insights

// CalculateConfidence computes a Bayesian-inspired confidence score.
//
// Formula: N / (N + σ² * penalty)
//
// Where:
//   - N is the observation count
//   - variance (σ²) represents spread of observations (lower = more consistent)
//   - penalty is a tunable factor (default 1.0)
//
// Returns a value in [0.0, 1.0].
// Returns 0.0 when observations == 0.
func CalculateConfidence(observations int, variance float64, penalty float64) float64 {
	if observations == 0 {
		return 0.0
	}
	n := float64(observations)
	adjusted := n / (n + variance*penalty)
	if adjusted > 1.0 {
		return 1.0
	}
	return adjusted
}

// DefaultConfidence computes confidence with penalty=1.0.
// Equivalent to count / (count + variance).
func DefaultConfidence(observations int, variance float64) float64 {
	return CalculateConfidence(observations, variance, 1.0)
}
