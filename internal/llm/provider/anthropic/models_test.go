package anthropic_test

import (
	"testing"

	"github.com/modu-ai/goose/internal/llm/provider/anthropic"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestNormalizeModel_TableDriven은 모델 별칭 정규화를 검증한다.
func TestNormalizeModel_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "claude-3.5-sonnet alias",
			input: "claude-3.5-sonnet",
			want:  "claude-3-5-sonnet-20241022",
		},
		{
			name:  "claude-3-5-sonnet alias",
			input: "claude-3-5-sonnet",
			want:  "claude-3-5-sonnet-20241022",
		},
		{
			name:  "claude-opus-4 alias",
			input: "claude-opus-4",
			want:  "claude-opus-4-7",
		},
		{
			name:  "claude-3.7-sonnet alias",
			input: "claude-3.7-sonnet",
			want:  "claude-3-7-sonnet-20250219",
		},
		{
			name:  "unknown model passes through",
			input: "claude-some-unknown-model",
			want:  "claude-some-unknown-model",
		},
		{
			name:  "concrete model passes through",
			input: "claude-3-5-sonnet-20241022",
			want:  "claude-3-5-sonnet-20241022",
		},
		{
			name:  "claude-opus-4-7 passes through",
			input: "claude-opus-4-7",
			want:  "claude-opus-4-7",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := anthropic.NormalizeModel(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestMaxOutputTokensFor_TableDriven은 모델별 최대 출력 토큰을 검증한다.
func TestMaxOutputTokensFor_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		model string
		want  int
	}{
		{
			name:  "claude-opus-4-7",
			model: "claude-opus-4-7",
			want:  16000,
		},
		{
			name:  "claude-3-7-sonnet",
			model: "claude-3-7-sonnet-20250219",
			want:  16000,
		},
		{
			name:  "claude-3-5-sonnet",
			model: "claude-3-5-sonnet-20241022",
			want:  8192,
		},
		{
			name:  "unknown model default",
			model: "claude-unknown",
			want:  4096,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := anthropic.MaxOutputTokensFor(tc.model)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestAdaptiveThinkingModels는 AdaptiveThinkingModels 맵에 claude-opus-4-7이 있는지 검증한다.
func TestAdaptiveThinkingModels(t *testing.T) {
	t.Parallel()

	assert.True(t, anthropic.IsAdaptiveThinkingModel("claude-opus-4-7"),
		"claude-opus-4-7 is an adaptive thinking model")
	assert.False(t, anthropic.IsAdaptiveThinkingModel("claude-3-5-sonnet-20241022"),
		"claude-3-5-sonnet is NOT an adaptive thinking model")
}
