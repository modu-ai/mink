// Package context_test — ReactiveCompact 추가 커버리지 테스트.
// AC-CTX-005: ReactiveCompact의 Summarizer 에러 시 Snip fallback 경로
package context_test

import (
	"errors"
	"testing"

	goosecontext "github.com/modu-ai/goose/internal/context"
	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/query/loop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompactor_ReactiveCompact_SummarizerError_FallsBackToSnip는
// ReactiveCompact 경로에서 Summarizer 에러 시 Snip fallback을 검증한다.
// REQ-CTX-014: Summarizer 에러는 로그하고 Snip fallback.
func TestCompactor_ReactiveCompact_SummarizerError_FallsBackToSnip(t *testing.T) {
	t.Parallel()

	stub := &stubSummarizer{
		err: errors.New("reactive llm unavailable"),
	}

	compactor := &goosecontext.DefaultCompactor{
		Summarizer:      stub,
		HistorySnipOnly: false,
		ProtectedHead:   3,
		ProtectedTail:   5,
		TokenLimit:      100_000,
	}

	state := loop.State{
		Messages: append(
			makeMessages(15),
			message.Message{
				Role:    "user",
				Content: []message.ContentBlock{{Type: "text", Text: "extra"}},
			},
		),
		TokenLimit:          100_000,
		TaskBudgetRemaining: 500,
		AutoCompactTracking: loop.AutoCompactTracking{ReactiveTriggered: true},
	}

	_, boundary, err := compactor.Compact(state)
	require.NoError(t, err, "ReactiveCompact 에러가 호출자에게 전파되면 안 됨")

	// Summarizer 에러 → Snip fallback
	assert.Equal(t, goosecontext.StrategySnip, boundary.Strategy,
		"ReactiveCompact Summarizer 에러 시 Snip fallback이어야 함")
}

// TestCompactor_ReactiveCompact_NilSummarizer_FallsBackToSnip는
// ReactiveTriggered=true이지만 Summarizer=nil일 때 Snip을 검증한다.
func TestCompactor_ReactiveCompact_NilSummarizer_FallsBackToSnip(t *testing.T) {
	t.Parallel()

	compactor := &goosecontext.DefaultCompactor{
		Summarizer:      nil,
		HistorySnipOnly: false,
		ProtectedHead:   3,
		ProtectedTail:   5,
		TokenLimit:      100_000,
	}

	state := loop.State{
		Messages:            makeMessages(15),
		TokenLimit:          100_000,
		AutoCompactTracking: loop.AutoCompactTracking{ReactiveTriggered: true},
	}

	assert.True(t, compactor.ShouldCompact(state), "ReactiveTriggered=true이면 ShouldCompact==true")

	_, boundary, err := compactor.Compact(state)
	require.NoError(t, err)

	assert.Equal(t, goosecontext.StrategySnip, boundary.Strategy,
		"ReactiveTriggered=true이지만 Summarizer=nil이면 Snip이어야 함")
}
