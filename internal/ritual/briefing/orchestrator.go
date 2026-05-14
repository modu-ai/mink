package briefing

import (
	"context"
	"sync"
	"time"

	"github.com/modu-ai/mink/internal/llm"
)

// Collector is the interface for collecting briefing modules.
// Each collector implements its own Collect method with specific signature.
type Collector interface {
	Collect(ctx context.Context, userID string, today time.Time) (any, string)
}

// Option is a functional option for Orchestrator configuration.
// T-303: Functional Options pattern for LLM wiring (REQ-BR-060).
type Option func(*Orchestrator)

// WithLLMProvider sets the LLM provider for optional summary generation.
// When nil, no LLM call is made regardless of Config.LLMSummary.
func WithLLMProvider(p llm.LLMProvider) Option {
	return func(o *Orchestrator) {
		o.llmProvider = p
	}
}

// WithLLMModel sets the model name used for LLM summary generation.
// Empty string falls back to provider default ("default").
func WithLLMModel(model string) Option {
	return func(o *Orchestrator) {
		o.llmModel = model
	}
}

// WithConfig sets the briefing configuration for the orchestrator.
// When nil, LLM summary is disabled and defaults apply.
func WithConfig(cfg *Config) Option {
	return func(o *Orchestrator) {
		o.cfg = cfg
	}
}

// Orchestrator coordinates parallel collection of all briefing modules.
type Orchestrator struct {
	weather Collector
	journal Collector
	date    Collector
	mantra  Collector

	// LLM summary fields (T-303, REQ-BR-060)
	llmProvider llm.LLMProvider
	llmModel    string
	cfg         *Config
}

// NewOrchestrator creates a new Orchestrator with the given collectors.
// Accepts optional Option values for LLM wiring (T-303).
func NewOrchestrator(weather, journal, date, mantra Collector, opts ...Option) *Orchestrator {
	o := &Orchestrator{
		weather: weather,
		journal: journal,
		date:    date,
		mantra:  mantra,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
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

	// T-304: Optional LLM summary phase (REQ-BR-060).
	// Runs only when cfg.LLMSummary == true AND a provider is wired.
	// Failure is graceful: status["llm_summary"] = "error", pipeline continues.
	if o.cfg != nil && o.cfg.LLMSummary && o.llmProvider != nil {
		summary, err := GenerateLLMSummary(ctx, o.llmProvider, payload, o.cfg, o.llmModel)
		if err != nil {
			// Log only error category — never log raw provider response (REQ-BR-050).
			payload.Status["llm_summary"] = "error"
		} else {
			payload.LLMSummary = summary
			if summary != "" {
				payload.Status["llm_summary"] = "ok"
			}
		}
	}

	return payload, nil
}
