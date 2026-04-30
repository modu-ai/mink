package memory

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// dispatcher handles lifecycle hook dispatch with timeout and panic recovery.
type dispatcher struct {
	logger    *zap.Logger
	providers []MemoryProvider
	initState sync.Map // sessionID -> providerName -> initialized (bool)
	mu        sync.RWMutex
}

// newDispatcher creates a new dispatcher.
func newDispatcher(logger *zap.Logger, providers []MemoryProvider) *dispatcher {
	return &dispatcher{
		logger:    logger,
		providers: providers,
	}
}

// dispatchWithTimeout calls the function with a per-provider timeout.
//
// @MX:WARN: Spawns a goroutine per dispatch and uses recover() to catch
// provider panics. The provider's fn is invoked in the goroutine; if fn blocks
// past the timeout, the goroutine leaks until fn unblocks (best-effort
// cancellation only — Go has no goroutine kill primitive).
// @MX:REASON: Required by AC-007 (panic isolation) and AC-011 (50ms budget).
// Replacing this pattern requires re-verifying both ACs and confirming no
// goroutine accumulation under sustained timeouts.
// @MX:SPEC: SPEC-GOOSE-MEMORY-001
func (d *dispatcher) dispatchWithTimeout(ctx context.Context, p MemoryProvider, timeout time.Duration, fn func(MemoryProvider)) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				d.logger.Error("provider panic recovered",
					zap.String("provider", p.Name()),
					zap.Any("panic", r))
			}
			close(done)
		}()
		fn(p)
	}()

	select {
	case <-done:
		// OK
	case <-ctx.Done():
		d.logger.Warn("provider hook timeout",
			zap.String("provider", p.Name()),
			zap.Duration("timeout", timeout))
	}
}

// markInitFailed records that a provider failed initialization for a session.
func (d *dispatcher) markInitFailed(sessionID, providerName string) {
	key := sessionID + ":" + providerName
	d.initState.Store(key, true)
}

// clearInitState clears all initialization state for a session.
func (d *dispatcher) clearInitState(sessionID string) {
	// Clear all entries for this session
	prefix := sessionID + ":"
	d.initState.Range(func(key, value any) bool {
		if keyStr, ok := key.(string); ok {
			if len(keyStr) >= len(prefix) && keyStr[:len(prefix)] == prefix {
				d.initState.Delete(key)
			}
		}
		return true
	})
}

// didInitFail checks if a provider failed initialization for a session.
func (d *dispatcher) didInitFail(sessionID, providerName string) bool {
	key := sessionID + ":" + providerName
	if val, ok := d.initState.Load(key); ok {
		if failed, ok := val.(bool); ok && failed {
			return true
		}
	}
	return false
}
