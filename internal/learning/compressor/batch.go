package compressor

import (
	"context"
	"sync"
	"time"

	"github.com/modu-ai/mink/internal/learning/trajectory"
)

// CompressBatch compresses a slice of trajectories in parallel, bounded by
// cfg.MaxConcurrentRequests.
//
// REQ-COMPRESSOR-009: semaphore bounds in-flight LLM calls.
// REQ-COMPRESSOR-010: per-trajectory 300s timeout via context.WithTimeout.
// REQ-COMPRESSOR-016: individual failures do not abort the batch.
//
// @MX:WARN: [AUTO] Spawns up to MaxConcurrentRequests goroutines per call.
// @MX:REASON: Semaphore bounds concurrency; individual trajectory errors are isolated per REQ-COMPRESSOR-016.
func (c *TrajectoryCompressor) CompressBatch(
	ctx context.Context,
	trajectories []*trajectory.Trajectory,
) []BatchResult {
	results := make([]BatchResult, len(trajectories))
	if len(trajectories) == 0 {
		return results
	}

	sem := make(chan struct{}, c.cfg.MaxConcurrentRequests)
	var wg sync.WaitGroup

	for i, t := range trajectories {
		wg.Add(1)
		go func(idx int, traj *trajectory.Trajectory) {
			defer wg.Done()

			// Acquire semaphore slot.
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results[idx] = BatchResult{
					Index:      idx,
					Trajectory: traj,
					Metrics:    &TrajectoryMetrics{StartedAt: time.Now(), TimedOut: true, EndedAt: time.Now()},
					Err:        ctx.Err(),
				}
				return
			}
			defer func() { <-sem }()

			compressed, metrics, err := c.Compress(ctx, traj)
			results[idx] = BatchResult{
				Index:      idx,
				Trajectory: compressed,
				Metrics:    metrics,
				Err:        err,
			}
		}(i, t)
	}

	wg.Wait()
	return results
}
