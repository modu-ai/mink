package insights

import (
	"sort"

	"github.com/modu-ai/mink/internal/learning/trajectory"
)

// aggregateModels computes per-model statistics from a set of trajectories.
// Returns results sorted by TotalTokens descending (ties broken by model name).
func aggregateModels(trajectories []*trajectory.Trajectory, pricing PricingTable) []ModelStat {
	stats := make(map[string]*ModelStat)

	for _, t := range trajectories {
		model := t.Model
		if model == "" {
			model = "unknown"
		}
		s, ok := stats[model]
		if !ok {
			s = &ModelStat{Model: model}
			stats[model] = s
		}
		s.Sessions++
		s.InputTokens += t.Metadata.TokensInput
		s.OutputTokens += t.Metadata.TokensOutput
		s.TotalTokens += t.Metadata.TokensInput + t.Metadata.TokensOutput
	}

	// Compute cost for each model.
	for model, s := range stats {
		if pricing != nil {
			cost, hasPricing := pricing.ComputeCost(model, s.InputTokens, s.OutputTokens, s.CacheReadTokens, s.CacheWriteTokens)
			s.Cost = cost
			s.HasPricing = hasPricing
		}
	}

	// Convert to slice.
	result := make([]ModelStat, 0, len(stats))
	for _, s := range stats {
		result = append(result, *s)
	}

	// Sort by TotalTokens descending, then by model name ascending for determinism.
	sort.Slice(result, func(i, j int) bool {
		if result[i].TotalTokens != result[j].TotalTokens {
			return result[i].TotalTokens > result[j].TotalTokens
		}
		return result[i].Model < result[j].Model
	})

	return result
}
