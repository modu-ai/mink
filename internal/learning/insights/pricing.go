package insights

// ModelPricing holds per-model token pricing in USD per 1K tokens.
type ModelPricing struct {
	InputPer1K      float64 `yaml:"input_per_1k"  json:"input_per_1k"`
	OutputPer1K     float64 `yaml:"output_per_1k" json:"output_per_1k"`
	CacheReadPer1K  float64 `yaml:"cache_read_per_1k,omitempty"  json:"cache_read_per_1k,omitempty"`
	CacheWritePer1K float64 `yaml:"cache_write_per_1k,omitempty" json:"cache_write_per_1k,omitempty"`
}

// PricingTable maps model identifier to pricing data.
type PricingTable map[string]ModelPricing

// Lookup returns the pricing for a model and whether it was found.
func (pt PricingTable) Lookup(model string) (ModelPricing, bool) {
	p, ok := pt[model]
	return p, ok
}

// DefaultPricingTable returns the hardcoded default pricing table.
// Source: Hermes pricing table (spec.md §6.6).
// Unknown models return HasPricing=false with zero cost.
func DefaultPricingTable() PricingTable {
	return PricingTable{
		"anthropic/claude-opus-4-7": {
			InputPer1K:      0.015,
			OutputPer1K:     0.075,
			CacheReadPer1K:  0.0015,
			CacheWritePer1K: 0.01875,
		},
		"anthropic/claude-sonnet-4-6": {
			InputPer1K:  0.003,
			OutputPer1K: 0.015,
		},
		"anthropic/claude-sonnet-4-5": {
			InputPer1K:  0.003,
			OutputPer1K: 0.015,
		},
		"anthropic/claude-haiku-3-5": {
			InputPer1K:  0.0008,
			OutputPer1K: 0.004,
		},
		"openai/gpt-4o": {
			InputPer1K:  0.0025,
			OutputPer1K: 0.010,
		},
		"openai/gpt-4o-mini": {
			InputPer1K:  0.00015,
			OutputPer1K: 0.0006,
		},
		"google/gemini-2.0-flash": {
			InputPer1K:  0.00010,
			OutputPer1K: 0.00040,
		},
		"google/gemini-3-flash-preview": {
			InputPer1K:  0.0001,
			OutputPer1K: 0.0004,
		},
	}
}

// ComputeCost calculates the total cost for a given model usage.
// Returns 0.0 and false if the model is not in the pricing table.
func (pt PricingTable) ComputeCost(model string, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int) (cost float64, hasPricing bool) {
	p, ok := pt.Lookup(model)
	if !ok {
		return 0.0, false
	}
	cost = float64(inputTokens)/1000.0*p.InputPer1K +
		float64(outputTokens)/1000.0*p.OutputPer1K +
		float64(cacheReadTokens)/1000.0*p.CacheReadPer1K +
		float64(cacheWriteTokens)/1000.0*p.CacheWritePer1K
	return cost, true
}
