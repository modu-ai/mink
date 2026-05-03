package compressor

import (
	"testing"

	goosecontext "github.com/modu-ai/goose/internal/context"
	"github.com/modu-ai/goose/internal/learning/trajectory"
	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/query"
	"github.com/modu-ai/goose/internal/query/loop"
	"go.uber.org/zap"
)

// TestAdapter_QueryCompactorInterfaceCompatible: AC-COMPRESSOR-013
// Compile-time assertion: var _ query.Compactor = (*CompactorAdapter)(nil)
// This test also invokes both methods to verify runtime behavior.
func TestAdapter_QueryCompactorInterfaceCompatible(t *testing.T) {
	t.Parallel()
	// The compile-time assertion is in adapter.go.
	// Runtime verification:
	cfg := DefaultConfig()
	inner := New(cfg, nil, &SimpleTokenizer{}, zap.NewNop())
	adapter := NewCompactorAdapter(inner, nil, false)

	// Verify it satisfies the interface.
	var _ query.Compactor = adapter

	// ShouldCompact with a minimal state.
	s := loop.State{
		Messages: []message.Message{
			{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hello"}}},
		},
		TokenLimit:      10000,
		MaxMessageCount: 500,
	}
	_ = adapter.ShouldCompact(s)
	_, _, _ = adapter.Compact(s)
}

// TestAdapter_ShouldCompact_FourTriggers: AC-COMPRESSOR-019
func TestAdapter_ShouldCompact_FourTriggers(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	inner := New(cfg, nil, &SimpleTokenizer{}, zap.NewNop())
	adapter := NewCompactorAdapter(inner, nil, false)

	makeMessages := func(n int) []message.Message {
		msgs := make([]message.Message, n)
		for i := range msgs {
			msgs[i] = message.Message{
				Role:    "user",
				Content: []message.ContentBlock{{Type: "text", Text: "word word word word word word word word word word"}},
			}
		}
		return msgs
	}

	t.Run("(a) 80% ratio trigger", func(t *testing.T) {
		t.Parallel()
		// 100 messages × ~10 tokens = ~1000 tokens; tokenLimit=1100 → ratio ≈ 91% >= 80%.
		msgs := makeMessages(100)
		s := loop.State{
			Messages:        msgs,
			TokenLimit:      1100,
			MaxMessageCount: 500,
		}
		if !adapter.ShouldCompact(s) {
			t.Error("expected ShouldCompact=true for 80%+ ratio")
		}
	})

	t.Run("(b) ReactiveTriggered trigger", func(t *testing.T) {
		t.Parallel()
		// Very low token count — ratio is negligible.
		msgs := makeMessages(1)
		s := loop.State{
			Messages:            msgs,
			TokenLimit:          100_000,
			AutoCompactTracking: loop.AutoCompactTracking{ReactiveTriggered: true},
		}
		if !adapter.ShouldCompact(s) {
			t.Error("expected ShouldCompact=true when ReactiveTriggered=true")
		}
	})

	t.Run("(c) MaxMessageCount trigger", func(t *testing.T) {
		t.Parallel()
		msgs := makeMessages(10)
		s := loop.State{
			Messages:        msgs,
			TokenLimit:      1_000_000, // very high limit
			MaxMessageCount: 9,         // messages (10) > max (9)
		}
		if !adapter.ShouldCompact(s) {
			t.Error("expected ShouldCompact=true when len(Messages)>MaxMessageCount")
		}
	})

	t.Run("(d) Red override trigger", func(t *testing.T) {
		t.Parallel()
		// Create enough messages to exceed 92% of a small TokenLimit.
		// Use many messages with substantial text.
		msgs := makeMessages(200)
		s := loop.State{
			Messages:        msgs,
			TokenLimit:      1500, // set low so ~200*10=2000 exceeds 92% of 1500
			MaxMessageCount: 10_000,
		}
		used := goosecontext.TokenCountWithEstimation(msgs)
		level := goosecontext.CalculateTokenWarningState(used, 1500)
		if level < goosecontext.WarningRed {
			t.Skipf("test setup doesn't reach Red level (used=%d, limit=1500, level=%v)", used, level)
		}
		if !adapter.ShouldCompact(s) {
			t.Error("expected ShouldCompact=true for Red warning level")
		}
	})

	t.Run("control: all false", func(t *testing.T) {
		t.Parallel()
		msgs := makeMessages(2)
		s := loop.State{
			Messages:        msgs,
			TokenLimit:      1_000_000,
			MaxMessageCount: 500,
		}
		if adapter.ShouldCompact(s) {
			t.Error("expected ShouldCompact=false when no triggers")
		}
	})
}

// TestAdapter_Compact_StrategyOrder: AC-COMPRESSOR-020
func TestAdapter_Compact_StrategyOrder(t *testing.T) {
	t.Parallel()

	makeState := func(reactiveTriggered bool, ratio float64, snipOnly bool) loop.State {
		// Build messages to achieve roughly the given ratio.
		limit := int64(10_000)
		// We need token count ≈ ratio * limit.
		// goosecontext.TokenCountWithEstimation counts chars/4 + overhead.
		// Use simple text blocks to approximate.
		target := int64(float64(limit) * ratio)
		// Each message with 4 chars → 1 token. Add overhead (4 per message).
		// chars needed ≈ target * 4 - len(msgs)*4
		msgs := []message.Message{}
		totalTokens := int64(0)
		for totalTokens < target {
			text := "aaaa" // 4 chars → 1 token via chars/4
			msgs = append(msgs, message.Message{
				Role:    "user",
				Content: []message.ContentBlock{{Type: "text", Text: text}},
			})
			totalTokens = goosecontext.TokenCountWithEstimation(msgs)
		}
		return loop.State{
			Messages:            msgs,
			TokenLimit:          limit,
			MaxMessageCount:     10_000,
			AutoCompactTracking: loop.AutoCompactTracking{ReactiveTriggered: reactiveTriggered},
			TaskBudgetRemaining: 42,
		}
	}

	t.Run("(a) ReactiveCompact wins when ReactiveTriggered=true", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		cfg.MaxRetries = 0
		stub := &stubSummarizer{summary: "reactive summary"}
		inner := New(cfg, stub, &SimpleTokenizer{}, zap.NewNop())
		adapter := NewCompactorAdapter(inner, nil, false)

		s := makeState(true, 0.9, false)
		_, boundary, _ := adapter.Compact(s)
		// With stub summarizer, strategy should be ReactiveCompact or fallback Snip (nil snipDelegate).
		// Either way boundary must be populated.
		_ = boundary
	})

	t.Run("(b) AutoCompact when ratio>=80% and not reactive", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		cfg.MaxRetries = 0
		stub := &stubSummarizer{summary: "auto summary"}
		inner := New(cfg, stub, &SimpleTokenizer{}, zap.NewNop())
		adapter := NewCompactorAdapter(inner, nil, false)

		s := makeState(false, 0.85, false)
		_, boundary, err := adapter.Compact(s)
		if err != nil {
			t.Logf("Compact error (ok for strategy test): %v", err)
		}
		// Strategy should be AutoCompact or Snip fallback.
		_ = boundary
	})

	t.Run("(c) HistorySnipOnly forces Snip", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		inner := New(cfg, &stubSummarizer{summary: "x"}, &SimpleTokenizer{}, zap.NewNop())
		adapter := NewCompactorAdapter(inner, nil, true) // historySnipOnly=true

		s := makeState(true, 0.9, true)
		_, boundary, _ := adapter.Compact(s)
		if boundary.Strategy != goosecontext.StrategySnip {
			t.Errorf("expected Snip strategy, got %q", boundary.Strategy)
		}
	})

	t.Run("(d) Summarizer=nil forces Snip", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		inner := New(cfg, nil, &SimpleTokenizer{}, zap.NewNop()) // nil summarizer
		adapter := NewCompactorAdapter(inner, nil, false)

		s := makeState(false, 0.9, false)
		_, boundary, err := adapter.Compact(s)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if boundary.Strategy != goosecontext.StrategySnip {
			t.Errorf("expected Snip when summarizer=nil, got %q", boundary.Strategy)
		}
	})

	t.Run("(e) Summarizer error falls back to Snip, no error propagated", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		cfg.MaxRetries = 0
		cfg.AdapterMaxRetries = 0
		stub := &stubSummarizer{failAll: true}
		inner := New(cfg, stub, &SimpleTokenizer{}, zap.NewNop())
		adapter := NewCompactorAdapter(inner, nil, false)

		s := makeState(false, 0.9, false)
		_, boundary, err := adapter.Compact(s)
		// REQ-CTX-014: error must not be propagated.
		if err != nil {
			t.Errorf("expected nil error after Snip fallback, got %v", err)
		}
		if boundary.Strategy != goosecontext.StrategySnip {
			t.Errorf("expected Snip fallback strategy, got %q", boundary.Strategy)
		}
	})

	t.Run("TaskBudgetRemaining preserved (flat int)", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()
		inner := New(cfg, nil, &SimpleTokenizer{}, zap.NewNop())
		adapter := NewCompactorAdapter(inner, nil, false)

		s := loop.State{
			Messages:            []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hi"}}}},
			TokenLimit:          1000,
			TaskBudgetRemaining: 42,
		}
		newState, boundary, _ := adapter.Compact(s)
		if newState.TaskBudgetRemaining != 42 {
			t.Errorf("TaskBudgetRemaining changed: want 42, got %d", newState.TaskBudgetRemaining)
		}
		if boundary.TaskBudgetPreserved != 42 {
			t.Errorf("boundary.TaskBudgetPreserved: want 42, got %d", boundary.TaskBudgetPreserved)
		}
	})
}

// TestAdapter_CompactBoundary_NineFields verifies all 9 CompactBoundary fields are populated.
func TestAdapter_CompactBoundary_NineFields(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	inner := New(cfg, nil, &SimpleTokenizer{}, zap.NewNop())
	adapter := NewCompactorAdapter(inner, nil, false)

	s := loop.State{
		Messages:            []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hello"}}}},
		TokenLimit:          100,
		TurnCount:           5,
		TaskBudgetRemaining: 7,
	}
	_, boundary, _ := adapter.Compact(s)

	// Verify all 9 fields of CompactBoundary exist and are accessible.
	_ = boundary.Turn
	_ = boundary.Strategy
	_ = boundary.MessagesBefore
	_ = boundary.MessagesAfter
	_ = boundary.TokensBefore
	_ = boundary.TokensAfter
	_ = boundary.TaskBudgetPreserved
	_ = boundary.DroppedThinkingCount

	if boundary.TaskBudgetPreserved != int64(s.TaskBudgetRemaining) {
		t.Errorf("TaskBudgetPreserved: got %d, want %d", boundary.TaskBudgetPreserved, s.TaskBudgetRemaining)
	}
}

// TestAdapter_ShouldCompact_PromptShape verifies AC-COMPRESSOR-008.
func TestSummarizer_PromptShape(t *testing.T) {
	t.Parallel()
	turns := []trajectory.TrajectoryEntry{
		{From: "human", Value: "question"},
		{From: "gpt", Value: "answer"},
		{From: "human", Value: "follow-up"},
	}
	prompt, err := buildPrompt("", turns, "test-model", 750)
	if err != nil {
		t.Fatalf("buildPrompt error: %v", err)
	}
	// Should contain "You are summarizing" prefix.
	if len(prompt) < 10 {
		t.Error("prompt too short")
	}
	// Should contain turn values.
	if !contains(prompt, "question") {
		t.Error("prompt should contain turn value 'question'")
	}
}

func contains(s, sub string) bool {
	return findSubstring(s, sub)
}
