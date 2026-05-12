package insights

import (
	"github.com/modu-ai/mink/internal/learning/trajectory"
)

// aggregateOverview computes the Overview dimension from a set of trajectories.
func aggregateOverview(trajectories []*trajectory.Trajectory, pricing PricingTable) Overview {
	var ov Overview
	allPriced := true

	for _, t := range trajectories {
		ov.TotalSessions++
		if t.Completed {
			ov.TotalSuccessful++
		} else {
			ov.TotalFailed++
		}

		tokens := t.Metadata.TokensInput + t.Metadata.TokensOutput
		ov.TotalTokens += tokens

		durationSec := float64(t.Metadata.DurationMs) / 1000.0
		ov.TotalHours += durationSec / 3600.0

		if pricing != nil && t.Model != "" {
			cost, hasPricing := pricing.ComputeCost(
				t.Model,
				t.Metadata.TokensInput,
				t.Metadata.TokensOutput,
				0, // cache tokens not tracked in trajectory metadata
				0,
			)
			if hasPricing {
				ov.EstimatedCost += cost
			} else {
				allPriced = false
			}
		} else if t.Model != "" {
			allPriced = false
		}
	}

	if ov.TotalSessions > 0 {
		totalDurationSec := ov.TotalHours * 3600.0
		ov.AvgSessionDuration = totalDurationSec / float64(ov.TotalSessions)
	}

	ov.HasFullPricing = allPriced && ov.TotalSessions > 0

	return ov
}
