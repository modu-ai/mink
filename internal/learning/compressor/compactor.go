package compressor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/modu-ai/mink/internal/learning/trajectory"
	"go.uber.org/zap"
)

// TrajectoryCompressor compresses a Trajectory by replacing the middle section
// with an LLM-generated summary while preserving head and tail entries.
//
// @MX:ANCHOR: [AUTO] Primary compression API — Compress and CompressBatch
// @MX:REASON: SPEC-GOOSE-COMPRESSOR-001 entry point; called from INSIGHTS-001, CompactorAdapter; fan_in >= 3
type TrajectoryCompressor struct {
	cfg        CompressionConfig
	summarizer Summarizer
	tokenizer  Tokenizer
	logger     *zap.Logger
}

// New creates a new TrajectoryCompressor.
// summarizer may be nil (Snip-only mode when used via CompactorAdapter).
// tokenizer must not be nil; use &SimpleTokenizer{} as default.
func New(
	cfg CompressionConfig,
	summarizer Summarizer,
	tokenizer Tokenizer,
	logger *zap.Logger,
) *TrajectoryCompressor {
	if tokenizer == nil {
		tokenizer = &SimpleTokenizer{}
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &TrajectoryCompressor{
		cfg:        cfg,
		summarizer: summarizer,
		tokenizer:  tokenizer,
		logger:     logger,
	}
}

// Compress compresses a single trajectory.
// Always returns a non-nil *TrajectoryMetrics, even on error paths (REQ-COMPRESSOR-001).
// Does not mutate the input trajectory (REQ-COMPRESSOR-002).
//
// @MX:ANCHOR: [AUTO] Core compress algorithm — called by CompressBatch and CompactorAdapter
// @MX:REASON: SPEC-GOOSE-COMPRESSOR-001 REQ-COMPRESSOR-001/002; fan_in >= 3
func (c *TrajectoryCompressor) Compress(
	ctx context.Context,
	t *trajectory.Trajectory,
) (result *trajectory.Trajectory, metrics *TrajectoryMetrics, retErr error) {
	metrics = &TrajectoryMetrics{StartedAt: time.Now()}

	// Ensure EndedAt is always set and metrics is always non-nil (REQ-COMPRESSOR-001).
	defer func() {
		if r := recover(); r != nil {
			retErr = fmt.Errorf("compressor panic: %v", r)
			metrics.SummarizationErrors++
			metrics.WasCompressed = false
		}
		if metrics.EndedAt.IsZero() {
			metrics.EndedAt = time.Now()
		}
	}()

	n := len(t.Conversations)
	metrics.OriginalTurns = n

	// Count tokens for each entry.
	turnTokens := make([]int, n)
	total := 0
	for i, entry := range t.Conversations {
		tok := c.tokenizer.Count(entry.Value)
		turnTokens[i] = tok
		total += tok
	}
	metrics.OriginalTokens = total

	// REQ-COMPRESSOR-005: skip if already under target.
	if total <= c.cfg.TargetMaxTokens {
		metrics.SkippedUnderTarget = true
		metrics.CompressedTokens = total
		metrics.CompressedTurns = n
		result = cloneTrajectory(t)
		metrics.EndedAt = time.Now()
		return
	}

	// Find protected indices.
	protected := findProtectedIndices(t, c.cfg.TailProtectedTurns)
	compressStart, compressEnd := findCompressibleRegion(protected, n)

	// REQ-COMPRESSOR-011: no compressible region.
	if compressStart >= compressEnd {
		metrics.SkippedNoCompressibleRegion = true
		metrics.StillOverLimit = true
		result = cloneTrajectory(t)
		metrics.EndedAt = time.Now()
		return
	}

	// Determine how many tokens to accumulate from the middle region.
	tokensToSave := total - c.cfg.TargetMaxTokens
	targetCompress := tokensToSave + c.cfg.SummaryTargetTokens

	accumulated := 0
	compressUntil := compressStart
	for i := compressStart; i < compressEnd; i++ {
		accumulated += turnTokens[i]
		compressUntil = i + 1
		if accumulated >= targetCompress {
			break
		}
	}

	middle := t.Conversations[compressStart:compressUntil]

	// REQ-COMPRESSOR-014: skip if middle slice is too small.
	if len(middle) <= 1 {
		metrics.SkippedNoCompressibleRegion = true
		result = cloneTrajectory(t)
		metrics.EndedAt = time.Now()
		return
	}

	// REQ-COMPRESSOR-021: filter redacted_thinking entries from the summarizer input,
	// collect them for re-attachment after the summary.
	cleanMiddle, redactedEntries := separateRedactedThinking(middle)

	// Apply per-trajectory context timeout.
	compressCtx, cancel := context.WithTimeout(ctx, c.cfg.PerTrajectoryTimeout)
	defer cancel()

	summary, apiCalls, errCount, err := summarizeWithRetry(
		compressCtx,
		c.summarizer,
		cleanMiddle,
		c.cfg.SummaryTargetTokens,
		c.cfg.MaxRetries,
		c.cfg.BaseDelay,
	)
	metrics.SummarizationApiCalls = apiCalls
	metrics.SummarizationErrors = errCount

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			metrics.TimedOut = true
		}
		result = cloneTrajectory(t)
		metrics.EndedAt = time.Now()
		retErr = err
		return
	}

	// REQ-COMPRESSOR-015: reject overshoot summaries.
	summaryTokens := c.tokenizer.Count(summary)
	maxAllowed := int(float64(c.cfg.SummaryTargetTokens) * c.cfg.SummaryOvershootFactor)
	if summaryTokens > maxAllowed {
		c.logger.Warn("summarizer overshot budget",
			zap.Int("summary_tokens", summaryTokens),
			zap.Int("max_allowed", maxAllowed),
		)
		metrics.SummarizerOvershot = true
		result = cloneTrajectory(t)
		metrics.EndedAt = time.Now()
		return
	}

	// Reconstruct: head + [redacted entries] + summary entry + [redacted entries] + tail
	summaryEntry := trajectory.TrajectoryEntry{
		From:  trajectory.RoleHuman,
		Value: summary,
	}

	// Build the new Conversations slice:
	//   head portion ([:compressStart])
	//   preserved redacted_thinking entries (before summary)
	//   summary entry
	//   preserved redacted_thinking entries (after summary) — same entries, placed after
	//   tail portion ([compressUntil:])
	// Per spec REQ-021(b): attach redacted entries around the summary entry.
	var newConversations []trajectory.TrajectoryEntry
	newConversations = append(newConversations, t.Conversations[:compressStart]...)
	newConversations = append(newConversations, redactedEntries...)
	newConversations = append(newConversations, summaryEntry)
	newConversations = append(newConversations, t.Conversations[compressUntil:]...)

	// Track dropped thinking count for the boundary report.
	metrics.DroppedThinkingCount = len(redactedEntries)

	compressed := &trajectory.Trajectory{
		Conversations: newConversations,
		Timestamp:     t.Timestamp,
		Model:         t.Model,
		Completed:     t.Completed,
		SessionID:     t.SessionID,
		Metadata:      deepCopyMetadata(t.Metadata),
	}

	metrics.WasCompressed = true
	metrics.CompressedTurns = len(compressed.Conversations)
	metrics.CompressedTokens = c.tokenizer.CountTrajectory(compressed)
	metrics.TokensSaved = metrics.OriginalTokens - metrics.CompressedTokens
	if metrics.OriginalTokens > 0 {
		metrics.CompressionRatio = float64(metrics.CompressedTokens) / float64(metrics.OriginalTokens)
	}
	metrics.TurnsCompressedStartIdx = compressStart
	metrics.TurnsInCompressedRegion = compressUntil - compressStart
	metrics.StillOverLimit = metrics.CompressedTokens > c.cfg.TargetMaxTokens
	metrics.EndedAt = time.Now()
	result = compressed
	return
}

// CompressWithRetries compresses a single trajectory with an explicit retry limit override.
// Used by CompactorAdapter to apply AdapterMaxRetries instead of MaxRetries.
func (c *TrajectoryCompressor) CompressWithRetries(
	ctx context.Context,
	t *trajectory.Trajectory,
	maxRetries int,
) (*trajectory.Trajectory, *TrajectoryMetrics, error) {
	// Temporarily override MaxRetries for this call.
	origRetries := c.cfg.MaxRetries
	c.cfg.MaxRetries = maxRetries
	r, m, err := c.Compress(ctx, t)
	c.cfg.MaxRetries = origRetries
	return r, m, err
}

// BatchResult holds the result for a single trajectory in a batch.
type BatchResult struct {
	Index      int
	Trajectory *trajectory.Trajectory
	Metrics    *TrajectoryMetrics
	Err        error
}

// separateRedactedThinking splits middle entries into clean (no redacted_thinking) and
// those containing redacted_thinking markers.
// REQ-COMPRESSOR-021: LLM only sees the clean slice; redacted entries are re-attached later.
func separateRedactedThinking(entries []trajectory.TrajectoryEntry) (clean []trajectory.TrajectoryEntry, redacted []trajectory.TrajectoryEntry) {
	for _, e := range entries {
		if hasRedactedThinking(e.Value) {
			redacted = append(redacted, e)
		} else {
			clean = append(clean, e)
		}
	}
	return
}

// cloneTrajectory creates a shallow copy of Trajectory with a deep-copied Metadata.
// REQ-COMPRESSOR-002: must not mutate input.
func cloneTrajectory(t *trajectory.Trajectory) *trajectory.Trajectory {
	clone := &trajectory.Trajectory{
		Conversations: make([]trajectory.TrajectoryEntry, len(t.Conversations)),
		Timestamp:     t.Timestamp,
		Model:         t.Model,
		Completed:     t.Completed,
		SessionID:     t.SessionID,
		Metadata:      deepCopyMetadata(t.Metadata),
	}
	copy(clone.Conversations, t.Conversations)
	return clone
}

// deepCopyMetadata produces a deep copy of TrajectoryMetadata.
// REQ-COMPRESSOR-002 / AC-COMPRESSOR-022: Tags slice and nested fields are copied.
func deepCopyMetadata(src trajectory.TrajectoryMetadata) trajectory.TrajectoryMetadata {
	dst := src
	if src.Tags != nil {
		dst.Tags = make([]string, len(src.Tags))
		copy(dst.Tags, src.Tags)
	}
	return dst
}
