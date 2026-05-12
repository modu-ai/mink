package adapter

import (
	"context"
	"sync"
	"testing"

	"github.com/modu-ai/mink/internal/command"
	"github.com/modu-ai/mink/internal/llm/router"
	"github.com/modu-ai/mink/internal/observability/metrics"
	"github.com/modu-ai/mink/internal/subagent"
	"github.com/stretchr/testify/assert"
)

// TestContextAdapter_ConcurrentAccess verifies AC-CMDCTX-014:
// 100 goroutines invoking all 6 methods concurrently with 1000 iterations each.
// This test must be run with -race to catch data races.
func TestContextAdapter_ConcurrentAccess(t *testing.T) {
	fake := &fakeLoopController{
		snapshotVal: LoopSnapshot{TurnCount: 42, Model: "anthropic/claude-opus-4-7"},
	}
	a := New(Options{
		Registry:       router.DefaultRegistry(),
		LoopController: fake,
		AliasMap:       map[string]string{"opus": "anthropic/claude-opus-4-7"},
	})

	ctx := subagent.WithTeammateIdentity(context.Background(), subagent.TeammateIdentity{
		PlanModeRequired: true,
	})
	child := a.WithContext(ctx)

	const goroutines = 100
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func() {
			defer wg.Done()
			for j := range iterations {
				switch (i + j) % 7 {
				case 0:
					_ = a.OnClear()
				case 1:
					_ = a.OnCompactRequest(j)
				case 2:
					_ = a.OnModelChange(command.ModelInfo{ID: "anthropic/claude-opus-4-7"})
				case 3:
					_, _ = a.ResolveModelAlias("anthropic/claude-opus-4-7")
				case 4:
					_ = a.SessionSnapshot()
				case 5:
					_ = child.PlanModeActive()
				case 6:
					// SetPlanMode from various goroutines — tests atomic write.
					a.SetPlanMode(j%2 == 0)
				}
			}
		}()
	}

	wg.Wait()
	// No assertions needed: the race detector is the judge.
	// If the test completes without -race flag warnings, it passes.
}

// TestRace_Metrics_ConcurrentEmission verifies AC-TEL-011:
// 100 goroutines call all 6 methods concurrently; race detector must pass and
// final counter values must be deterministic.
func TestRace_Metrics_ConcurrentEmission(t *testing.T) {
	sink := newFakeMetricsSink()
	lc := &fakeLoopController{
		snapshotVal: LoopSnapshot{TurnCount: 1, Model: "anthropic/claude-opus-4-7"},
	}
	a := New(Options{
		Metrics:        sink,
		LoopController: lc,
		AliasMap:       map[string]string{"opus": "anthropic/claude-opus-4-7"},
	})

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func() {
			defer wg.Done()
			_ = a.OnClear()
			_ = a.OnCompactRequest(0)
			_ = a.OnModelChange(command.ModelInfo{ID: "anthropic/claude-opus-4-7"})
			_, _ = a.ResolveModelAlias("anthropic/claude-opus-4-7")
			_ = a.SessionSnapshot()
			_ = a.PlanModeActive()
			_ = i // suppress unused variable warning
		}()
	}

	wg.Wait()

	// Each goroutine calls each method once → total calls per method == 100.
	for _, method := range []string{"OnClear", "OnCompactRequest", "OnModelChange", "ResolveModelAlias", "SessionSnapshot", "PlanModeActive"} {
		calls := sink.counterVal("cmdctx.method.calls", metrics.Labels{"method": method})
		assert.Equal(t, int64(goroutines), calls,
			"method %s: calls counter must be %d after %d goroutines", method, goroutines, goroutines)
	}
}

// TestContextAdapter_WithContext_SharedPlanMode verifies the pointer-sharing
// invariant for planMode across WithContext children under concurrency.
func TestContextAdapter_WithContext_SharedPlanMode(t *testing.T) {
	parent := New(Options{})

	const goroutines = 50
	children := make([]*ContextAdapter, goroutines)
	for i := range goroutines {
		children[i] = parent.WithContext(context.Background())
	}

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i, child := range children {
		go func() {
			defer wg.Done()
			for range 100 {
				_ = child.PlanModeActive()
				if i%2 == 0 {
					parent.SetPlanMode(true)
				} else {
					parent.SetPlanMode(false)
				}
			}
		}()
	}

	wg.Wait()
}
