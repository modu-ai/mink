package cache_test

import (
	"testing"

	"github.com/modu-ai/mink/internal/llm/cache"
	"github.com/modu-ai/mink/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.Equal(t, cache.TTL("5m"), cache.TTLDefault, "TTLDefault == '5m'")
	assert.Equal(t, cache.TTL("1h"), cache.TTL1Hour, "TTL1Hour == '1h'")
}

// TestCachePlan_Fields는 CachePlan과 CacheMarker 구조체를 검증한다.
func TestCachePlan_Fields(t *testing.T) {
	t.Parallel()

	marker := cache.CacheMarker{
		MessageIndex:      2,
		ContentBlockIndex: 1,
		TTL:               cache.TTLEphemeral,
	}

	assert.Equal(t, 2, marker.MessageIndex)
	assert.Equal(t, 1, marker.ContentBlockIndex)
	assert.Equal(t, cache.TTLEphemeral, marker.TTL)

	plan := cache.CachePlan{
		Strategy: cache.StrategySystemAnd3,
		Markers:  []cache.CacheMarker{marker},
	}
	assert.Equal(t, 1, len(plan.Markers))
	assert.Equal(t, cache.StrategySystemAnd3, plan.Strategy)
}

// ── 헬퍼 함수 ────────────────────────────────────────────────────────────────

// makeMsg는 테스트용 메시지를 생성한다.
func makeMsg(role string, blocks int) message.Message {
	content := make([]message.ContentBlock, blocks)
	for i := range content {
		content[i] = message.ContentBlock{Type: "text", Text: "block"}
	}
	return message.Message{Role: role, Content: content}
}

// makeMsgs는 role 배열로부터 1-block 메시지 슬라이스를 생성한다.
func makeMsgs(roles ...string) []message.Message {
	msgs := make([]message.Message, len(roles))
	for i, r := range roles {
		msgs[i] = makeMsg(r, 1)
	}
	return msgs
}

// ── AC-PC-001 ─────────────────────────────────────────────────────────────────

// TestPlanner_SystemAnd3_FullMessages — AC-PC-001
// Given: messages 6개: [system, user, assistant, user, assistant, user]
// When: Plan(SystemAnd3, TTLDefault)
// Then: markers 4개, 인덱스 [0,3,4,5], 각 TTL == "5m"
func TestPlanner_SystemAnd3_FullMessages(t *testing.T) {
	t.Parallel()

	msgs := makeMsgs("system", "user", "assistant", "user", "assistant", "user")
	p := cache.NewBreakpointPlanner()

	plan, err := p.Plan(msgs, cache.StrategySystemAnd3, cache.TTLDefault)
	require.NoError(t, err)
	require.NotNil(t, plan)

	assert.Equal(t, 4, len(plan.Markers), "markers 4개 기대")
	wantIndices := []int{0, 3, 4, 5}
	gotIndices := make([]int, len(plan.Markers))
	for i, m := range plan.Markers {
		gotIndices[i] = m.MessageIndex
	}
	assert.Equal(t, wantIndices, gotIndices, "marker MessageIndex 순서")
	for _, m := range plan.Markers {
		assert.Equal(t, cache.TTLDefault, m.TTL, "TTL == TTLDefault")
	}
}

// ── AC-PC-002 ─────────────────────────────────────────────────────────────────

// TestPlanner_SystemAnd3_FewMessages — AC-PC-002
// Given: [system, user, assistant] (non-system 2개)
// Then: len(markers) == 3, 인덱스 [0,1,2]
func TestPlanner_SystemAnd3_FewMessages(t *testing.T) {
	t.Parallel()

	msgs := makeMsgs("system", "user", "assistant")
	p := cache.NewBreakpointPlanner()

	plan, err := p.Plan(msgs, cache.StrategySystemAnd3, cache.TTLDefault)
	require.NoError(t, err)

	require.Equal(t, 3, len(plan.Markers), "non-system 2개 → markers 3개")
	wantIndices := []int{0, 1, 2}
	for i, m := range plan.Markers {
		assert.Equal(t, wantIndices[i], m.MessageIndex)
	}
}

// ── AC-PC-003 ─────────────────────────────────────────────────────────────────

// TestPlanner_SystemAnd3_NoSystem — AC-PC-003
// Given: [user, assistant, user] (system 없음)
// When: Plan(SystemAnd3, TTL1Hour)
// Then: len(markers) == 3, 각 TTL == "1h"
func TestPlanner_SystemAnd3_NoSystem(t *testing.T) {
	t.Parallel()

	msgs := makeMsgs("user", "assistant", "user")
	p := cache.NewBreakpointPlanner()

	plan, err := p.Plan(msgs, cache.StrategySystemAnd3, cache.TTL1Hour)
	require.NoError(t, err)

	assert.Equal(t, 3, len(plan.Markers))
	for _, m := range plan.Markers {
		assert.Equal(t, cache.TTL1Hour, m.TTL)
	}
}

// ── AC-PC-004 ─────────────────────────────────────────────────────────────────

// TestPlanner_SystemOnly — AC-PC-004
// Given: messages 6개 (system 포함)
// Then: len(markers) == 1, 인덱스 [0]
func TestPlanner_SystemOnly(t *testing.T) {
	t.Parallel()

	msgs := makeMsgs("system", "user", "assistant", "user", "assistant", "user")
	p := cache.NewBreakpointPlanner()

	plan, err := p.Plan(msgs, cache.StrategySystemOnly, cache.TTLDefault)
	require.NoError(t, err)

	require.Equal(t, 1, len(plan.Markers))
	assert.Equal(t, 0, plan.Markers[0].MessageIndex)
}

// ── AC-PC-005 ─────────────────────────────────────────────────────────────────

// TestPlanner_None — AC-PC-005
// Given: messages 6개
// Then: len(markers) == 0, error 없음
func TestPlanner_None(t *testing.T) {
	t.Parallel()

	msgs := makeMsgs("system", "user", "assistant", "user", "assistant", "user")
	p := cache.NewBreakpointPlanner()

	plan, err := p.Plan(msgs, cache.StrategyNone, cache.TTLDefault)
	require.NoError(t, err)

	assert.Equal(t, 0, len(plan.Markers))
}

// ── AC-PC-006 ─────────────────────────────────────────────────────────────────

// TestPlanner_EmptyMessages — AC-PC-006
// Given: messages []
// Then: len(markers) == 0, error 없음
func TestPlanner_EmptyMessages(t *testing.T) {
	t.Parallel()

	p := cache.NewBreakpointPlanner()

	plan, err := p.Plan(nil, cache.StrategySystemAnd3, cache.TTLDefault)
	require.NoError(t, err)
	assert.NotNil(t, plan)
	assert.Equal(t, 0, len(plan.Markers))

	plan2, err2 := p.Plan([]message.Message{}, cache.StrategySystemAnd3, cache.TTLDefault)
	require.NoError(t, err2)
	assert.Equal(t, 0, len(plan2.Markers))
}

// ── AC-PC-007 ─────────────────────────────────────────────────────────────────

// TestPlanner_MultipleContentBlocks_UsesLastIndex — AC-PC-007
// Given: messages[5]이 3개 content block 포함
// Then: 해당 메시지 marker ContentBlockIndex == 2 (마지막 블록)
func TestPlanner_MultipleContentBlocks_UsesLastIndex(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		makeMsg("system", 1),
		makeMsg("user", 1),
		makeMsg("assistant", 1),
		makeMsg("user", 1),
		makeMsg("assistant", 1),
		makeMsg("user", 3), // 3개 content block
	}
	p := cache.NewBreakpointPlanner()

	plan, err := p.Plan(msgs, cache.StrategySystemAnd3, cache.TTLDefault)
	require.NoError(t, err)
	require.NotEmpty(t, plan.Markers, "markers가 비어있음")

	// msgs[5]가 마지막 non-system marker
	lastMarker := plan.Markers[len(plan.Markers)-1]
	assert.Equal(t, 5, lastMarker.MessageIndex)
	assert.Equal(t, 2, lastMarker.ContentBlockIndex, "3-block 메시지의 마지막 블록 인덱스 == 2")
}

// ── AC-PC-008 ─────────────────────────────────────────────────────────────────

// TestPlanner_TTL1Hour_Propagates — AC-PC-008
// Given: strategy=SystemAnd3, ttl=TTL1Hour
// Then: 모든 marker의 TTL == "1h"
func TestPlanner_TTL1Hour_Propagates(t *testing.T) {
	t.Parallel()

	msgs := makeMsgs("system", "user", "assistant", "user", "assistant", "user")
	p := cache.NewBreakpointPlanner()

	plan, err := p.Plan(msgs, cache.StrategySystemAnd3, cache.TTL1Hour)
	require.NoError(t, err)

	for i, m := range plan.Markers {
		assert.Equal(t, cache.TTL1Hour, m.TTL, "marker[%d].TTL == '1h'", i)
	}
}

// ── 추가 경계값 테스트 ─────────────────────────────────────────────────────────

// TestPlanner_SystemAnd3_OnlySystem — non-system 0개
// Given: messages [system]
// Then: len(markers) == 1 (system만)
func TestPlanner_SystemAnd3_OnlySystem(t *testing.T) {
	t.Parallel()

	msgs := makeMsgs("system")
	p := cache.NewBreakpointPlanner()

	plan, err := p.Plan(msgs, cache.StrategySystemAnd3, cache.TTLDefault)
	require.NoError(t, err)

	require.Equal(t, 1, len(plan.Markers))
	assert.Equal(t, 0, plan.Markers[0].MessageIndex)
}

// TestPlanner_SystemAnd3_OneNonSystem — non-system 1개
// Given: [system, user]
// Then: len(markers) == 2
func TestPlanner_SystemAnd3_OneNonSystem(t *testing.T) {
	t.Parallel()

	msgs := makeMsgs("system", "user")
	p := cache.NewBreakpointPlanner()

	plan, err := p.Plan(msgs, cache.StrategySystemAnd3, cache.TTLDefault)
	require.NoError(t, err)

	assert.Equal(t, 2, len(plan.Markers))
}

// TestPlanner_Uniqueness_NoduplicateMarkers — REQ-PC-009
// system 메시지 하나뿐: system marker와 non-system marker가 중복되지 않아야 한다
func TestPlanner_Uniqueness_NoDuplicateMarkers(t *testing.T) {
	t.Parallel()

	msgs := makeMsgs("system", "user", "assistant", "user", "assistant", "user")
	p := cache.NewBreakpointPlanner()

	plan, err := p.Plan(msgs, cache.StrategySystemAnd3, cache.TTLDefault)
	require.NoError(t, err)

	// (MessageIndex, ContentBlockIndex) 튜플 중복 확인
	seen := make(map[[2]int]bool)
	for _, m := range plan.Markers {
		key := [2]int{m.MessageIndex, m.ContentBlockIndex}
		assert.False(t, seen[key], "중복 marker: (%d, %d)", m.MessageIndex, m.ContentBlockIndex)
		seen[key] = true
	}
}

// TestPlanner_InputNotMutated — REQ-PC-011
func TestPlanner_InputNotMutated(t *testing.T) {
	t.Parallel()

	msgs := makeMsgs("system", "user", "assistant", "user", "assistant", "user")
	original := make([]message.Message, len(msgs))
	copy(original, msgs)

	p := cache.NewBreakpointPlanner()
	_, err := p.Plan(msgs, cache.StrategySystemAnd3, cache.TTLDefault)
	require.NoError(t, err)

	for i, m := range msgs {
		assert.Equal(t, original[i].Role, m.Role, "msgs[%d].Role가 변경됨", i)
		assert.Equal(t, len(original[i].Content), len(m.Content), "msgs[%d].Content 길이가 변경됨", i)
	}
}

// TestPlanner_Determinism — REQ-PC-001
func TestPlanner_Determinism(t *testing.T) {
	t.Parallel()

	msgs := makeMsgs("system", "user", "assistant", "user", "assistant", "user")
	p := cache.NewBreakpointPlanner()

	plan1, err1 := p.Plan(msgs, cache.StrategySystemAnd3, cache.TTLDefault)
	plan2, err2 := p.Plan(msgs, cache.StrategySystemAnd3, cache.TTLDefault)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, plan1.Markers, plan2.Markers, "같은 입력 → 같은 출력")
}

// TestPlanner_SystemOnly_NoSystem — system 없는 경우 SystemOnly 빈 plan
func TestPlanner_SystemOnly_NoSystem(t *testing.T) {
	t.Parallel()

	msgs := makeMsgs("user", "assistant", "user")
	p := cache.NewBreakpointPlanner()

	plan, err := p.Plan(msgs, cache.StrategySystemOnly, cache.TTLDefault)
	require.NoError(t, err)

	assert.Equal(t, 0, len(plan.Markers), "system 없는 SystemOnly → 빈 plan")
}

// TestPlanner_InvalidStrategy — 알 수 없는 전략 값 시 에러 반환
func TestPlanner_InvalidStrategy(t *testing.T) {
	t.Parallel()

	msgs := makeMsgs("system", "user")
	p := cache.NewBreakpointPlanner()

	plan, err := p.Plan(msgs, cache.CacheStrategy(99), cache.TTLDefault)
	assert.Nil(t, plan)
	assert.ErrorIs(t, err, cache.ErrInvalidStrategy)
}

// TestPlanner_EmptyContentBlock — content block이 없는 메시지 처리
func TestPlanner_EmptyContentBlock(t *testing.T) {
	t.Parallel()

	// content block이 없는 메시지
	msgs := []message.Message{
		{Role: "system", Content: []message.ContentBlock{}}, // 빈 content
		{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hi"}}},
	}
	p := cache.NewBreakpointPlanner()

	plan, err := p.Plan(msgs, cache.StrategySystemAnd3, cache.TTLDefault)
	require.NoError(t, err)
	// ContentBlockIndex는 0이어야 한다 (빈 content 처리)
	require.Equal(t, 2, len(plan.Markers))
	assert.Equal(t, 0, plan.Markers[0].ContentBlockIndex, "빈 content block → index 0")
}
