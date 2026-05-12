// Package context_test — SPEC-GOOSE-CONTEXT-001 token 관련 테스트.
// AC-CTX-003: TokenCountWithEstimation 근사 정확도
// AC-CTX-004: CalculateTokenWarningState 임계값 테스트
package context_test

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	goosecontext "github.com/modu-ai/mink/internal/context"
	"github.com/modu-ai/mink/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tokenFixture는 testdata/tokens/*.json의 fixture 형식이다.
type tokenFixture struct {
	Input             string `json:"input"`
	GroundTruthTokens int    `json:"ground_truth_tokens"`
	Source            string `json:"source"`
}

// TestTokenCountWithEstimation_Within5Percent는 AC-CTX-003을 검증한다.
// 알려진 ground truth fixture 대비 ±5% 내 근사 정확도.
func TestTokenCountWithEstimation_Within5Percent(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/tokens/ko_en_mixed.json")
	require.NoError(t, err, "fixture 파일을 읽을 수 없음")

	var fixture tokenFixture
	require.NoError(t, json.Unmarshal(data, &fixture))
	require.NotEmpty(t, fixture.Input)
	require.Positive(t, fixture.GroundTruthTokens)

	msg := message.Message{
		Role: "user",
		Content: []message.ContentBlock{
			{Type: "text", Text: fixture.Input},
		},
	}

	got := goosecontext.TokenCountWithEstimation([]message.Message{msg})

	groundTruth := int64(fixture.GroundTruthTokens)
	tolerance := math.Ceil(float64(groundTruth) * 0.05)

	assert.InDelta(t, float64(groundTruth), float64(got), tolerance,
		"TokenCountWithEstimation이 ground_truth±5%를 벗어남: got=%d, want≈%d",
		got, groundTruth)
}

// TestTokenCountWithEstimation_Deterministic은 REQ-CTX-004를 검증한다.
// 동일 입력에 대해 반환값이 결정적(deterministic)임을 확인.
func TestTokenCountWithEstimation_Deterministic(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Hello, world!"}}},
		{Role: "assistant", Content: []message.ContentBlock{{Type: "text", Text: "Hi there!"}}},
	}

	result1 := goosecontext.TokenCountWithEstimation(msgs)
	result2 := goosecontext.TokenCountWithEstimation(msgs)
	result3 := goosecontext.TokenCountWithEstimation(msgs)

	assert.Equal(t, result1, result2, "1회와 2회 결과가 다름")
	assert.Equal(t, result2, result3, "2회와 3회 결과가 다름")
}

// TestCalculateTokenWarningState_Thresholds는 AC-CTX-004를 검증한다.
// 경계값에서의 WarningLevel 결정 테스트.
func TestCalculateTokenWarningState_Thresholds(t *testing.T) {
	t.Parallel()

	const limit = 100_000

	tests := []struct {
		name string
		used int64
		want goosecontext.WarningLevel
	}{
		{"59999 → Green", 59_999, goosecontext.WarningGreen},
		{"60000 → Yellow (경계)", 60_000, goosecontext.WarningYellow},
		{"60001 → Yellow", 60_001, goosecontext.WarningYellow},
		{"79999 → Yellow", 79_999, goosecontext.WarningYellow},
		{"80000 → Orange (경계)", 80_000, goosecontext.WarningOrange},
		{"80001 → Orange", 80_001, goosecontext.WarningOrange},
		{"92000 → Orange", 92_000, goosecontext.WarningOrange},
		{"92001 → Red (>92%)", 92_001, goosecontext.WarningRed},
		{"100000 → Red", 100_000, goosecontext.WarningRed},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := goosecontext.CalculateTokenWarningState(tc.used, limit)
			assert.Equal(t, tc.want, got,
				"CalculateTokenWarningState(%d, %d) = %v, want %v",
				tc.used, limit, got, tc.want)
		})
	}
}

// TestCalculateTokenWarningState_ZeroLimit은 limit=0인 경우 Green 반환을 검증한다.
func TestCalculateTokenWarningState_ZeroLimit(t *testing.T) {
	t.Parallel()
	got := goosecontext.CalculateTokenWarningState(100_000, 0)
	assert.Equal(t, goosecontext.WarningGreen, got, "limit=0이면 Green이어야 함")
}
