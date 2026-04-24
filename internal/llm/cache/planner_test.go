package cache_test

import (
	"testing"

	"github.com/modu-ai/goose/internal/llm/cache"
	"github.com/modu-ai/goose/internal/message"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestCacheStrategy_Constants는 CacheStrategy 상수가 정의되어 있는지 검증한다.
func TestCacheStrategy_Constants(t *testing.T) {
	t.Parallel()

	// 각 전략이 고유한 값을 가져야 한다.
	strategies := []cache.CacheStrategy{
		cache.StrategyNone,
		cache.StrategySystemOnly,
		cache.StrategySystemAnd3,
	}

	seen := make(map[cache.CacheStrategy]bool)
	for _, s := range strategies {
		if seen[s] {
			t.Errorf("중복 CacheStrategy 값: %d", s)
		}
		seen[s] = true
	}
}

// TestTTL_Constants는 TTL 상수가 정의되어 있는지 검증한다.
func TestTTL_Constants(t *testing.T) {
	t.Parallel()

	if cache.TTLEphemeral == "" {
		t.Error("TTLEphemeral: 빈 문자열")
	}
	if cache.TTL1Hour == "" {
		t.Error("TTL1Hour: 빈 문자열")
	}
	if cache.TTLEphemeral == cache.TTL1Hour {
		t.Error("TTLEphemeral == TTL1Hour: 서로 달라야 함")
	}
}

// TestBreakpointPlanner_PlanReturnsEmptyMarkers는 Plan이 항상 빈 Markers를 반환하는지 검증한다.
func TestBreakpointPlanner_PlanReturnsEmptyMarkers(t *testing.T) {
	t.Parallel()

	planner := &cache.BreakpointPlanner{}
	msgs := []message.Message{
		{Role: "system", Content: []message.ContentBlock{{Type: "text", Text: "You are helpful"}}},
		{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hello"}}},
	}

	plan := planner.Plan(msgs, cache.StrategySystemOnly, cache.TTLEphemeral)

	if len(plan.Markers) != 0 {
		t.Errorf("Plan: Markers 길이 %d, 빈 슬라이스 기대", len(plan.Markers))
	}
}

// TestBreakpointPlanner_PlanWithNilMessages는 nil messages에서도 안전한지 검증한다.
func TestBreakpointPlanner_PlanWithNilMessages(t *testing.T) {
	t.Parallel()

	planner := &cache.BreakpointPlanner{}
	plan := planner.Plan(nil, cache.StrategyNone, cache.TTLEphemeral)

	if plan.Markers == nil {
		t.Error("Plan: Markers는 nil이 아닌 빈 슬라이스여야 함")
	}
}

// TestCachePlan_Fields는 CachePlan과 CacheMarker 구조체를 검증한다.
func TestCachePlan_Fields(t *testing.T) {
	t.Parallel()

	marker := cache.CacheMarker{
		MessageIndex: 2,
		TTL:          cache.TTLEphemeral,
	}

	if marker.MessageIndex != 2 {
		t.Errorf("MessageIndex: got %d, want 2", marker.MessageIndex)
	}
	if marker.TTL != cache.TTLEphemeral {
		t.Errorf("TTL: got %q, want %q", marker.TTL, cache.TTLEphemeral)
	}

	plan := cache.CachePlan{
		Markers: []cache.CacheMarker{marker},
	}
	if len(plan.Markers) != 1 {
		t.Errorf("plan.Markers: got %d, want 1", len(plan.Markers))
	}
}
