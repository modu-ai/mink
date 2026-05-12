package query_test

import (
	"context"
	"testing"

	"github.com/modu-ai/mink/internal/llm/router"
	"github.com/modu-ai/mink/internal/message"
	"github.com/modu-ai/mink/internal/query"
	"github.com/modu-ai/mink/internal/tool"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestLLMCallReq_Fields는 LLMCallReq 구조체가 필요한 필드를 가지는지 검증한다.
func TestLLMCallReq_Fields(t *testing.T) {
	t.Parallel()

	req := query.LLMCallReq{
		Route: router.Route{
			Model:    "claude-opus-4-7",
			Provider: "anthropic",
		},
		Messages: []message.Message{
			{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hello"}}},
		},
		Tools:           []tool.Definition{{Name: "test_tool"}},
		MaxOutputTokens: 1024,
		Temperature:     0.7,
		FallbackModels:  []string{"claude-sonnet"},
	}

	if req.Route.Provider != "anthropic" {
		t.Errorf("Route.Provider: got %q, want %q", req.Route.Provider, "anthropic")
	}
	if len(req.Messages) != 1 {
		t.Errorf("Messages length: got %d, want 1", len(req.Messages))
	}
	if req.MaxOutputTokens != 1024 {
		t.Errorf("MaxOutputTokens: got %d, want 1024", req.MaxOutputTokens)
	}
}

// TestThinkingConfig_Fields는 ThinkingConfig 구조체를 검증한다.
func TestThinkingConfig_Fields(t *testing.T) {
	t.Parallel()

	cfg := query.ThinkingConfig{
		Enabled:      true,
		Effort:       "high",
		BudgetTokens: 8000,
	}

	if !cfg.Enabled {
		t.Error("Enabled: got false, want true")
	}
	if cfg.Effort != "high" {
		t.Errorf("Effort: got %q, want %q", cfg.Effort, "high")
	}
	if cfg.BudgetTokens != 8000 {
		t.Errorf("BudgetTokens: got %d, want 8000", cfg.BudgetTokens)
	}
}

// TestLLMCallFunc_Assignable는 LLMCallFunc 타입이 올바른 시그니처를 가지는지 검증한다.
func TestLLMCallFunc_Assignable(t *testing.T) {
	t.Parallel()

	// stub 함수를 LLMCallFunc에 할당할 수 있어야 한다.
	fn := query.LLMCallFunc(func(_ context.Context, _ query.LLMCallReq) (<-chan message.StreamEvent, error) {
		ch := make(chan message.StreamEvent)
		close(ch)
		return ch, nil
	})

	ch, err := fn(context.Background(), query.LLMCallReq{})
	if err != nil {
		t.Errorf("stub LLMCallFunc 호출 에러: %v", err)
	}
	// drain
	for range ch {
	}
}
