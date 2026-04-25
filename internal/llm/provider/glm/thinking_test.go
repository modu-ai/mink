package glm_test

import (
	"testing"

	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/provider/glm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGLM_BuildThinkingField_EnabledModel는 thinking-capable 모델 + Enabled=true 시
// 올바른 thinking field를 생성하는지 검증한다.
func TestGLM_BuildThinkingField_EnabledModel(t *testing.T) {
	t.Parallel()
	cfg := &provider.ThinkingConfig{Enabled: true}
	field, ok, reason := glm.BuildThinkingField(cfg, "glm-4.6")

	require.True(t, ok, "glm-4.6은 thinking 지원 모델이어야 함")
	assert.Empty(t, reason)
	require.NotNil(t, field)

	thinkingMap, isMap := field["thinking"].(map[string]any)
	require.True(t, isMap)
	assert.Equal(t, "enabled", thinkingMap["type"])
}

// TestGLM_BuildThinkingField_DisabledConfig는 Enabled=false 시 nil field를 반환하는지 검증한다.
func TestGLM_BuildThinkingField_DisabledConfig(t *testing.T) {
	t.Parallel()
	cfg := &provider.ThinkingConfig{Enabled: false}
	field, ok, reason := glm.BuildThinkingField(cfg, "glm-4.6")

	assert.True(t, ok)
	assert.Empty(t, reason)
	assert.Nil(t, field, "Enabled=false 시 thinking field가 없어야 함")
}

// TestGLM_BuildThinkingField_NilConfig는 nil config 시 nil field를 반환하는지 검증한다.
func TestGLM_BuildThinkingField_NilConfig(t *testing.T) {
	t.Parallel()
	field, ok, reason := glm.BuildThinkingField(nil, "glm-4.6")

	assert.True(t, ok)
	assert.Empty(t, reason)
	assert.Nil(t, field)
}

// TestGLM_BuildThinkingField_AirModel_GracefulDegradation는 AC-ADP2-003을 검증한다.
// thinking 미지원 모델에서 ok=false + reason 반환 (REQ-ADP2-014).
func TestGLM_BuildThinkingField_AirModel_GracefulDegradation(t *testing.T) {
	t.Parallel()
	cfg := &provider.ThinkingConfig{Enabled: true}
	field, ok, reason := glm.BuildThinkingField(cfg, "glm-4.5-air")

	assert.False(t, ok, "glm-4.5-air는 thinking 미지원이어야 함")
	assert.NotEmpty(t, reason, "reason이 설명을 포함해야 함")
	assert.Nil(t, field)
}

// TestGLM_BuildThinkingField_BudgetTokens는 BudgetTokens > 0 시
// thinking 필드가 {type:"enabled", budget_tokens:N} 형태로 생성되는지 검증한다.
// REQ-ADP2-021 (OI-2 v0.3): budget-based thinking 지원.
func TestGLM_BuildThinkingField_BudgetTokens(t *testing.T) {
	t.Parallel()
	cfg := &provider.ThinkingConfig{Enabled: true, BudgetTokens: 4096}
	field, ok, reason := glm.BuildThinkingField(cfg, "glm-4.6")

	require.True(t, ok)
	assert.Empty(t, reason)
	require.NotNil(t, field)

	thinkingMap, isMap := field["thinking"].(map[string]any)
	require.True(t, isMap)
	assert.Equal(t, "enabled", thinkingMap["type"])
	assert.Equal(t, 4096, thinkingMap["budget_tokens"])
}

// TestGLM_BuildThinkingField_BudgetTokensZero는 BudgetTokens=0이면
// budget_tokens 키 없이 default enabled 형태를 유지하는지 검증한다.
func TestGLM_BuildThinkingField_BudgetTokensZero(t *testing.T) {
	t.Parallel()
	cfg := &provider.ThinkingConfig{Enabled: true, BudgetTokens: 0}
	field, ok, _ := glm.BuildThinkingField(cfg, "glm-4.6")

	require.True(t, ok)
	require.NotNil(t, field)

	thinkingMap := field["thinking"].(map[string]any)
	_, hasBudget := thinkingMap["budget_tokens"]
	assert.False(t, hasBudget, "BudgetTokens=0 시 budget_tokens 키가 없어야 함")
}

// TestGLM_ThinkingCapableModels는 ThinkingCapableModels 맵이 올바른 모델을 포함하는지 검증한다.
func TestGLM_ThinkingCapableModels(t *testing.T) {
	t.Parallel()
	supportedModels := []string{"glm-5", "glm-4.7", "glm-4.6", "glm-4.5"}
	unsupportedModels := []string{"glm-4.5-air", "unknown-model"}

	for _, model := range supportedModels {
		cfg := &provider.ThinkingConfig{Enabled: true}
		_, ok, _ := glm.BuildThinkingField(cfg, model)
		assert.True(t, ok, "모델 %s는 thinking 지원이어야 함", model)
	}

	for _, model := range unsupportedModels {
		cfg := &provider.ThinkingConfig{Enabled: true}
		_, ok, _ := glm.BuildThinkingField(cfg, model)
		assert.False(t, ok, "모델 %s는 thinking 미지원이어야 함", model)
	}
}
