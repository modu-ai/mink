package briefing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/llm"
)

// mockLLMProvider is a controllable LLM provider for orchestrator tests.
type mockLLMProvider struct {
	response string
	err      error
	calls    int
}

func (m *mockLLMProvider) Complete(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	m.calls++
	if m.err != nil {
		return llm.CompletionResponse{}, m.err
	}
	return llm.CompletionResponse{Text: m.response}, nil
}

func (m *mockLLMProvider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.Chunk, error) {
	ch := make(chan llm.Chunk)
	close(ch)
	return ch, nil
}

func (m *mockLLMProvider) CountTokens(ctx context.Context, text string) (int, error) {
	return 0, nil
}

func (m *mockLLMProvider) Capabilities(ctx context.Context, model string) (llm.Capabilities, error) {
	return llm.Capabilities{}, nil
}

func (m *mockLLMProvider) Name() string {
	return "mock"
}

func newFixtureOrchestrator(_ llm.LLMProvider, _ *Config, opts ...Option) *Orchestrator {
	w := &mockWeatherForOrch{}
	j := &mockJournalForOrch{}
	d := &mockDateForOrch{}
	m := &mockMantraForOrch{}
	return NewOrchestrator(w, j, d, m, opts...)
}

// minimal stub collectors for orchestrator LLM tests
type mockWeatherForOrch struct{}

func (c *mockWeatherForOrch) Collect(ctx context.Context, userID string, today time.Time) (any, string) {
	return &WeatherModule{Current: &WeatherCurrent{Temp: 20, Condition: "Clear"}, Offline: false}, "ok"
}

type mockJournalForOrch struct{}

func (c *mockJournalForOrch) Collect(ctx context.Context, userID string, today time.Time) (any, string) {
	return &RecallModule{}, "ok"
}

type mockDateForOrch struct{}

func (c *mockDateForOrch) Collect(ctx context.Context, userID string, today time.Time) (any, string) {
	return &DateModule{Today: "2026-05-15", DayOfWeek: "목요일"}, "ok"
}

type mockMantraForOrch struct{}

func (c *mockMantraForOrch) Collect(ctx context.Context, userID string, today time.Time) (any, string) {
	return &MantraModule{Text: "Morning motivation"}, "ok"
}

// TestOrchestrator_LLMSummary verifies AC-014 — REQ-BR-032 / REQ-BR-060.
func TestOrchestrator_LLMSummary(t *testing.T) {
	today := time.Date(2026, 5, 15, 7, 0, 0, 0, time.UTC)

	// Case A: LLM enabled + provider returns summary
	t.Run("HappyPath", func(t *testing.T) {
		provider := &mockLLMProvider{response: "오늘은 평온한 하루입니다"}
		cfg := &Config{Mantra: "test", LLMSummary: true}
		o := newFixtureOrchestrator(provider, cfg,
			WithLLMProvider(provider),
			WithLLMModel("test-model"),
			WithConfig(cfg),
		)
		payload, err := o.Run(context.Background(), "u1", today)
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		if payload.LLMSummary != "오늘은 평온한 하루입니다" {
			t.Errorf("LLMSummary = %q, want %q", payload.LLMSummary, "오늘은 평온한 하루입니다")
		}
		if payload.Status["llm_summary"] != "ok" {
			t.Errorf("Status[llm_summary] = %q, want %q", payload.Status["llm_summary"], "ok")
		}
		if provider.calls != 1 {
			t.Errorf("provider.calls = %d, want 1", provider.calls)
		}
	})

	// Case B: LLM disabled / nil provider — no call, no error
	t.Run("Disabled", func(t *testing.T) {
		provider := &mockLLMProvider{response: "should not be called"}
		cfg := &Config{Mantra: "test", LLMSummary: false}
		o := newFixtureOrchestrator(provider, cfg,
			WithConfig(cfg),
			// No WithLLMProvider — or LLMSummary=false
		)
		payload, err := o.Run(context.Background(), "u1", today)
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		if payload.LLMSummary != "" {
			t.Errorf("LLMSummary should be empty when disabled, got %q", payload.LLMSummary)
		}
		if _, exists := payload.Status["llm_summary"]; exists {
			if payload.Status["llm_summary"] != "skipped" {
				t.Errorf("Status[llm_summary] = %q when disabled (expected absent or skipped)", payload.Status["llm_summary"])
			}
		}
		if provider.calls != 0 {
			t.Errorf("provider should not have been called, calls = %d", provider.calls)
		}
	})

	// Case C: LLM enabled but provider returns error — graceful degradation
	t.Run("ErrorPath", func(t *testing.T) {
		providerErr := errors.New("timeout: provider did not respond")
		provider := &mockLLMProvider{err: providerErr}
		cfg := &Config{Mantra: "test", LLMSummary: true}
		o := newFixtureOrchestrator(provider, cfg,
			WithLLMProvider(provider),
			WithConfig(cfg),
		)
		payload, err := o.Run(context.Background(), "u1", today)
		// Pipeline MUST NOT return error on LLM failure (graceful degradation)
		if err != nil {
			t.Fatalf("Run must not return error on LLM failure, got: %v", err)
		}
		if payload.LLMSummary != "" {
			t.Errorf("LLMSummary must be empty on error, got %q", payload.LLMSummary)
		}
		if payload.Status["llm_summary"] != "error" {
			t.Errorf("Status[llm_summary] = %q, want %q", payload.Status["llm_summary"], "error")
		}
		// Other module statuses must not be affected
		for _, mod := range []string{"weather", "journal", "date", "mantra"} {
			if s := payload.Status[mod]; s == "" {
				t.Errorf("Status[%s] must not be empty after LLM error", mod)
			}
		}
	})
}
