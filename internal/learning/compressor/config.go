// Package compressor implements trajectory compression for the Mink self-evolution pipeline.
// SPEC-GOOSE-COMPRESSOR-001 v0.2.1
package compressor

import "time"

// CompressionConfig holds all tunable parameters for the trajectory compressor.
// Hermes original constants are applied via DefaultConfig().
type CompressionConfig struct {
	// TargetMaxTokens is the post-compression token budget (default 15_250).
	TargetMaxTokens int
	// SummaryTargetTokens is the requested summary length in tokens (default 750).
	SummaryTargetTokens int
	// TailProtectedTurns is the number of tail entries to protect from compression (default 4).
	TailProtectedTurns int
	// MaxConcurrentRequests bounds parallel LLM calls in CompressBatch (default 5).
	MaxConcurrentRequests int
	// MaxRetries is the max retry count for offline batch Summarizer calls (default 3).
	MaxRetries int
	// AdapterMaxRetries is the max retry count for in-session adapter path (default 1).
	// Lower than MaxRetries to avoid blocking the UI.
	AdapterMaxRetries int
	// BaseDelay is the initial backoff delay for retry jitter (default 2s).
	BaseDelay time.Duration
	// PerTrajectoryTimeout is the per-trajectory processing deadline (default 300s).
	PerTrajectoryTimeout time.Duration
	// SummarizerPromptTemplate is an optional Go text/template string.
	// Supports variables: {{.Turns}}, {{.ModelName}}, {{.TargetTokens}}.
	// If empty, the built-in template is used.
	SummarizerPromptTemplate string
	// SummarizerModel is an optional hint for model selection inside Summarizer implementations.
	SummarizerModel string
	// SummaryOvershootFactor: if the summary exceeds SummaryTargetTokens * factor, it is rejected.
	// Default 2.0. Rationale: 1.25x for stop-token boundary jitter + 0.75x for metadata overhead.
	SummaryOvershootFactor float64
}

// DefaultConfig returns a CompressionConfig populated with Hermes-derived constants.
func DefaultConfig() CompressionConfig {
	return CompressionConfig{
		TargetMaxTokens:        15_250,
		SummaryTargetTokens:    750,
		TailProtectedTurns:     4,
		MaxConcurrentRequests:  5,
		MaxRetries:             3,
		AdapterMaxRetries:      1,
		BaseDelay:              2 * time.Second,
		PerTrajectoryTimeout:   300 * time.Second,
		SummaryOvershootFactor: 2.0,
	}
}
