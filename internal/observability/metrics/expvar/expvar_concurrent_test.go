// Package expvar_test — concurrent safety tests.
// SPEC-GOOSE-OBS-METRICS-001 T-007.
package expvar_test

import (
	"math/rand/v2"
	"sync"
	"testing"

	metricsexpvar "github.com/modu-ai/mink/internal/observability/metrics/expvar"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestExpvarSink_ConcurrentInc_RaceFree runs 100 goroutines each calling
// Inc() 1000 times on the same counter and verifies the total is exactly
// 100,000 (AC-013, REQ-OBS-METRICS-010).
func TestExpvarSink_ConcurrentInc_RaceFree(t *testing.T) {
	t.Parallel()

	s := metricsexpvar.New(zap.NewNop())
	// Obtain handle once; goroutines share it.
	c := s.Counter("concurrent.inc", nil)

	const (
		goroutines = 100
		iters      = 1000
	)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iters {
				c.Inc()
			}
		}()
	}
	wg.Wait()

	got := metricsexpvar.CounterValue(c)
	assert.InDelta(t, float64(goroutines*iters), got, 0.001,
		"100 goroutines × 1000 Inc() must equal 100,000")
}

// TestExpvarSink_ConcurrentHistogram_RaceFree runs 100 goroutines each calling
// Observe() 100 times and verifies the total observation count equals 10,000
// (AC-014, REQ-OBS-METRICS-010).
func TestExpvarSink_ConcurrentHistogram_RaceFree(t *testing.T) {
	t.Parallel()

	s := metricsexpvar.New(zap.NewNop())
	h := s.Histogram("concurrent.hist", nil, []float64{10, 50, 100})

	const (
		goroutines = 100
		iters      = 100
	)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iters {
				// rand.Float64() * 100 produces values in [0, 100).
				metricsexpvar.ObserveHistogram(h, rand.Float64()*100)
			}
		}()
	}
	wg.Wait()

	counts := metricsexpvar.HistogramCounts(h)
	var total int64
	for _, c := range counts {
		total += c
	}
	assert.Equal(t, int64(goroutines*iters), total,
		"sum of all bucket counts must equal total observations (10,000)")
}
