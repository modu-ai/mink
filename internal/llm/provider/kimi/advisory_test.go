package kimi

import (
	"strings"
	"testing"

	"github.com/modu-ai/goose/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// TestEstimateInputTokens_Empty는 빈 메시지 배열에 대해 0을 반환하는지 검증한다.
func TestEstimateInputTokens_Empty(t *testing.T) {
	t.Parallel()
	assert.Equal(t, int64(0), estimateInputTokens(nil))
	assert.Equal(t, int64(0), estimateInputTokens([]message.Message{}))
}

// TestEstimateInputTokens_TextScaling는 텍스트 길이에 비례한 추정값을 검증한다.
func TestEstimateInputTokens_TextScaling(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 4000) // 4000 chars / 4 = 1000 tokens + 4 role overhead
	msgs := []message.Message{
		{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: long}}},
	}
	tokens := estimateInputTokens(msgs)
	assert.GreaterOrEqual(t, tokens, int64(1000))
	assert.LessOrEqual(t, tokens, int64(1010))
}

// TestMaybeLogLongContextAdvisory_Triggers는 AC-ADP2-013을 검증한다.
// moonshot-v1-128k 모델 + 65K 토큰 초과 시 INFO 로그 1건이 발생해야 한다.
func TestMaybeLogLongContextAdvisory_Triggers(t *testing.T) {
	t.Parallel()
	core, recorded := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	// ~70K tokens, exceeds 64K threshold: 70000 * 4 chars
	huge := strings.Repeat("x", 70000*4)
	msgs := []message.Message{
		{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: huge}}},
	}
	maybeLogLongContextAdvisory(logger, "moonshot-v1-128k", msgs)

	entries := recorded.All()
	require.Len(t, entries, 1, "INFO 로그 정확히 1건이어야 함")
	entry := entries[0]
	assert.Equal(t, "kimi.long_context_advisory", entry.Message)
	assert.Equal(t, zap.InfoLevel, entry.Level)
	assert.Equal(t, "moonshot-v1-128k", entry.ContextMap()["model"])
	assert.GreaterOrEqual(t, entry.ContextMap()["estimated_input_tokens"], int64(64*1024+1))
}

// TestMaybeLogLongContextAdvisory_BelowThreshold는 64K 이하 입력 시 로그 없음을 검증한다.
func TestMaybeLogLongContextAdvisory_BelowThreshold(t *testing.T) {
	t.Parallel()
	core, recorded := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	// ~30K tokens
	msgs := []message.Message{
		{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: strings.Repeat("y", 30000*4)}}},
	}
	maybeLogLongContextAdvisory(logger, "moonshot-v1-128k", msgs)

	assert.Empty(t, recorded.All(), "임계 미만 시 로그 없음")
}

// TestMaybeLogLongContextAdvisory_NonLongContextModel는 marker 미포함 모델은 로그 없음을 검증한다.
func TestMaybeLogLongContextAdvisory_NonLongContextModel(t *testing.T) {
	t.Parallel()
	core, recorded := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	huge := strings.Repeat("z", 70000*4)
	msgs := []message.Message{
		{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: huge}}},
	}
	maybeLogLongContextAdvisory(logger, "moonshot-v1-32k", msgs)

	assert.Empty(t, recorded.All(), "non-128k 모델은 advisory 발생하지 않음")
}

// TestMaybeLogLongContextAdvisory_NilLogger는 logger nil 시 panic 없이 no-op임을 검증한다.
func TestMaybeLogLongContextAdvisory_NilLogger(t *testing.T) {
	t.Parallel()
	huge := strings.Repeat("a", 70000*4)
	msgs := []message.Message{
		{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: huge}}},
	}
	assert.NotPanics(t, func() {
		maybeLogLongContextAdvisory(nil, "moonshot-v1-128k", msgs)
	})
}
