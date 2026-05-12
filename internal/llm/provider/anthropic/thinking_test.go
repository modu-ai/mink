package anthropic_test

import (
	"testing"

	"github.com/modu-ai/mink/internal/llm/provider"
	"github.com/modu-ai/mink/internal/llm/provider/anthropic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnthropic_ThinkingMode_AdaptiveVsBudget는 AC-ADAPTER-012를 커버한다.
// Adaptive Thinking 모델은 effort, 비-Adaptive 모델은 budget_tokens를 사용해야 한다.
func TestAnthropic_ThinkingMode_AdaptiveVsBudget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		model            string
		cfg              *provider.ThinkingConfig
		wantNil          bool
		wantEffort       string
		wantBudgetTokens int
		wantType         string
	}{
		{
			name:    "nil config returns nil",
			model:   "claude-opus-4-7",
			cfg:     nil,
			wantNil: true,
		},
		{
			name:    "disabled thinking returns nil",
			model:   "claude-opus-4-7",
			cfg:     &provider.ThinkingConfig{Enabled: false},
			wantNil: true,
		},
		{
			name:  "opus-4-7 adaptive: effort=high -> type:enabled effort:high NO budget_tokens",
			model: "claude-opus-4-7",
			cfg: &provider.ThinkingConfig{
				Enabled: true,
				Effort:  "high",
			},
			wantNil:    false,
			wantType:   "enabled",
			wantEffort: "high",
		},
		{
			name:  "opus-4-7 adaptive: effort=max -> type:enabled effort:max",
			model: "claude-opus-4-7",
			cfg: &provider.ThinkingConfig{
				Enabled: true,
				Effort:  "max",
			},
			wantNil:    false,
			wantType:   "enabled",
			wantEffort: "max",
		},
		{
			name:  "non-adaptive model with budget_tokens",
			model: "claude-3-7-sonnet-20250219",
			cfg: &provider.ThinkingConfig{
				Enabled:      true,
				BudgetTokens: 8000,
			},
			wantNil:          false,
			wantType:         "enabled",
			wantBudgetTokens: 8000,
		},
		{
			name:  "non-adaptive model with effort only -> nil (unknown effort mode)",
			model: "claude-3-7-sonnet-20250219",
			cfg: &provider.ThinkingConfig{
				Enabled: true,
				Effort:  "high",
			},
			// non-adaptive 모델에서 Effort만 설정하고 BudgetTokens=0이면 nil 반환
			wantNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			param := anthropic.BuildThinkingParam(tc.cfg, tc.model)

			if tc.wantNil {
				assert.Nil(t, param)
				return
			}

			require.NotNil(t, param)
			assert.Equal(t, tc.wantType, param.Type)

			if tc.wantEffort != "" {
				assert.Equal(t, tc.wantEffort, param.Effort)
				assert.Zero(t, param.BudgetTokens, "adaptive thinking must not have budget_tokens")
			}

			if tc.wantBudgetTokens > 0 {
				assert.Equal(t, tc.wantBudgetTokens, param.BudgetTokens)
				assert.Empty(t, param.Effort, "non-adaptive thinking must not have effort")
			}
		})
	}
}
