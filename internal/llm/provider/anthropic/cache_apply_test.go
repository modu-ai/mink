package anthropic_test

import (
	"testing"

	"github.com/modu-ai/goose/internal/llm/cache"
	"github.com/modu-ai/goose/internal/llm/provider/anthropic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplyCacheMarkers_EmptyMarkers는 빈 Markers일 때 메시지가 변경되지 않는지 검증한다.
// REQ-ADAPTER-015: empty markers 시 cache_control 필드 없음
func TestApplyCacheMarkers_EmptyMarkers(t *testing.T) {
	t.Parallel()

	msgs := []anthropic.AnthropicMessage{
		{Role: "user", Content: []map[string]any{{"type": "text", "text": "hello"}}},
	}
	plan := cache.CachePlan{Markers: []cache.CacheMarker{}}

	result := anthropic.ApplyCacheMarkers(msgs, plan)

	require.Len(t, result, 1)
	// cache_control 없음
	for _, block := range result[0].Content {
		_, hasCacheControl := block["cache_control"]
		assert.False(t, hasCacheControl, "빈 markers에서 cache_control이 없어야 함")
	}
}

// TestApplyCacheMarkers_WithMarker는 마커가 있을 때 마지막 블록에 cache_control을 추가하는지 검증한다.
func TestApplyCacheMarkers_WithMarker(t *testing.T) {
	t.Parallel()

	msgs := []anthropic.AnthropicMessage{
		{
			Role: "user",
			Content: []map[string]any{
				{"type": "text", "text": "block1"},
				{"type": "text", "text": "block2"},
			},
		},
	}
	plan := cache.CachePlan{
		Markers: []cache.CacheMarker{
			{MessageIndex: 0, TTL: cache.TTLEphemeral},
		},
	}

	result := anthropic.ApplyCacheMarkers(msgs, plan)

	require.Len(t, result, 1)
	// 마지막 블록에 cache_control 추가
	lastBlock := result[0].Content[len(result[0].Content)-1]
	cc, ok := lastBlock["cache_control"]
	require.True(t, ok, "마지막 블록에 cache_control이 있어야 함")
	ccMap, ok := cc.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ephemeral", ccMap["type"])
}

// TestApplyCacheMarkers_InvalidIndex는 범위 밖 인덱스를 무시하는지 검증한다.
func TestApplyCacheMarkers_InvalidIndex(t *testing.T) {
	t.Parallel()

	msgs := []anthropic.AnthropicMessage{
		{Role: "user", Content: []map[string]any{{"type": "text", "text": "hello"}}},
	}
	plan := cache.CachePlan{
		Markers: []cache.CacheMarker{
			{MessageIndex: 99, TTL: cache.TTLEphemeral}, // 범위 밖
		},
	}

	// 패닉 없이 처리되어야 함
	result := anthropic.ApplyCacheMarkers(msgs, plan)
	require.Len(t, result, 1)
}

// TestApplyCacheMarkers_NilPlan은 nil plan이 안전하게 처리되는지 검증한다.
func TestApplyCacheMarkers_NilPlan(t *testing.T) {
	t.Parallel()

	msgs := []anthropic.AnthropicMessage{
		{Role: "user", Content: []map[string]any{{"type": "text", "text": "hello"}}},
	}
	plan := cache.CachePlan{} // nil Markers

	result := anthropic.ApplyCacheMarkers(msgs, plan)
	require.Len(t, result, 1)
}
