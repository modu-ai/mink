package briefing

import (
	"context"
	"sync"
	"time"
)

// Collector is the interface for collecting briefing modules.
// Each collector implements its own Collect method with specific signature.
type Collector interface {
	Collect(ctx context.Context, userID string, today time.Time) (any, string)
}

// Orchestrator coordinates parallel collection of all briefing modules.
type Orchestrator struct {
	weather Collector
	journal Collector
	date    Collector
	mantra  Collector
}

// NewOrchestrator creates a new Orchestrator with the given collectors.
func NewOrchestrator(weather, journal, date, mantra Collector) *Orchestrator {
	return &Orchestrator{
		weather: weather,
		journal: journal,
		date:    date,
		mantra:  mantra,
	}
}

// Run executes all collectors in parallel and returns the complete briefing payload.
// Each collector gets a 30s timeout. If all fail, returns minimal payload with all statuses="error".
func (o *Orchestrator) Run(ctx context.Context, userID string, today time.Time) (*BriefingPayload, error) {
	payload := &BriefingPayload{
		GeneratedAt:   time.Now(),
		Weather:       WeatherModule{Offline: true},
		JournalRecall: RecallModule{Offline: true},
		DateCalendar:  DateModule{},
		Mantra:        MantraModule{},
		Status:        make(map[string]string),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Timeout for each collector
	collectorTimeout := 30 * time.Second

	// Launch collectors in parallel
	collectors := map[string]Collector{
		"weather": o.weather,
		"journal": o.journal,
		"date":    o.date,
		"mantra":  o.mantra,
	}

	for name, collector := range collectors {
		wg.Add(1)
		go func(name string, collector Collector) {
			defer wg.Done()

			// Create timeout context for this collector
			collectCtx, cancel := context.WithTimeout(ctx, collectorTimeout)
			defer cancel()

			// Collect data
			module, status := collector.Collect(collectCtx, userID, today)

			// Update payload (thread-safe)
			mu.Lock()
			defer mu.Unlock()

			payload.Status[name] = status

			// Type-assert and populate the appropriate field
			switch name {
			case "weather":
				if wm, ok := module.(*WeatherModule); ok {
					payload.Weather = *wm
				}
			case "journal":
				if rm, ok := module.(*RecallModule); ok {
					payload.JournalRecall = *rm
				}
			case "date":
				if dm, ok := module.(*DateModule); ok {
					payload.DateCalendar = *dm
				}
			case "mantra":
				if mm, ok := module.(*MantraModule); ok {
					payload.Mantra = *mm
				}
			}
		}(name, collector)
	}

	// Wait for all collectors to complete
	wg.Wait()

	return payload, nil
}
