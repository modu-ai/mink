package compressor

import "time"

// TrajectoryMetrics records the outcome of a single Compress call.
// REQ-COMPRESSOR-001: always non-nil, even on error paths.
type TrajectoryMetrics struct {
	// Token counts
	OriginalTokens   int
	CompressedTokens int
	TokensSaved      int
	CompressionRatio float64

	// Turn counts and region info
	OriginalTurns           int
	CompressedTurns         int
	TurnsCompressedStartIdx int
	TurnsInCompressedRegion int

	// Status flags
	WasCompressed               bool
	SkippedUnderTarget          bool
	SkippedNoCompressibleRegion bool
	StillOverLimit              bool
	SummarizerOvershot          bool
	TimedOut                    bool

	// Summarizer call accounting
	SummarizationApiCalls int
	SummarizationErrors   int

	// Timing
	StartedAt time.Time
	EndedAt   time.Time

	// redacted_thinking preservation count
	DroppedThinkingCount int
}
