// Package aliasconfig 검증 테스트
package aliasconfig

import (
	"testing"

	"github.com/modu-ai/goose/internal/llm/router"
)

// TestValidate_ValidAliases 유효한 alias 검증 테스트
func TestValidate_ValidAliases(t *testing.T) {
	registry := router.DefaultRegistry()
	aliasMap := map[string]string{
		"gpt4":   "openai/gpt-4o",     // Fixed: gpt-4o (not gpt-4) is in SuggestedModels
		"claude": "anthropic/claude-sonnet-4-6",
		"gemini": "google/gemini-2.0-flash",
	}

	errs := Validate(aliasMap, registry, true)
	if errs != nil {
		t.Fatalf("Validate() returned %d errors, want 0", len(errs))
	}
}

// TestValidate_EmptyAlias 빈 별칭 검증 테스트
func TestValidate_EmptyAlias(t *testing.T) {
	registry := router.DefaultRegistry()
	aliasMap := map[string]string{
		"": "openai/gpt-4",
	}

	errs := Validate(aliasMap, registry, false)
	if len(errs) != 1 {
		t.Fatalf("Validate() returned %d errors, want 1", len(errs))
	}
}

// TestValidate_InvalidTarget 잘못된 대상 형식 검증 테스트
func TestValidate_InvalidTarget(t *testing.T) {
	registry := router.DefaultRegistry()
	aliasMap := map[string]string{
		"bad": "invalid-format",
	}

	errs := Validate(aliasMap, registry, false)
	if len(errs) != 1 {
		t.Fatalf("Validate() returned %d errors, want 1", len(errs))
	}
}

// TestValidate_UnknownProvider 존재하지 않는 provider 검증 테스트
func TestValidate_UnknownProvider(t *testing.T) {
	registry := router.DefaultRegistry()
	aliasMap := map[string]string{
		"unknown": "nonexistent/provider",
	}

	errs := Validate(aliasMap, registry, true)
	if len(errs) != 1 {
		t.Fatalf("Validate() returned %d errors, want 1", len(errs))
	}
}

// TestValidate_NoRegistry registry 없이면 provider 검증 스킵 테스트
func TestValidate_NoRegistry(t *testing.T) {
	aliasMap := map[string]string{
		"test": "openai/gpt-4",
	}

	errs := Validate(aliasMap, nil, false)
	if errs != nil {
		t.Fatalf("Validate() with nil registry returned %d errors, want 0", len(errs))
	}
}

// TestValidate_MixedErrors 여러 에러 병합 검증 테스트
func TestValidate_MixedErrors(t *testing.T) {
	registry := router.DefaultRegistry()
	aliasMap := map[string]string{
		"":        "openai/gpt-4o",      // empty alias
		"bad":     "invalid",            // invalid target (no slash)
		"unknown": "nonexistent/model",  // unknown provider
		"good":    "openai/gpt-4o",      // valid
	}

	errs := Validate(aliasMap, registry, true) // Changed to strict mode for provider/model validation
	if len(errs) < 3 {
		t.Fatalf("Validate() returned %d errors, want at least 3", len(errs))
	}
}

// TestValidate_StrictModeStrict 모드에서 모든 에러 반환 테스트
func TestValidate_StrictMode(t *testing.T) {
	registry := router.DefaultRegistry()
	aliasMap := map[string]string{
		"good":    "openai/gpt-4o",      // Fixed: gpt-4o is in SuggestedModels
		"unknown": "nonexistent/model",  // unknown provider
	}

	// Strict mode: unknown provider는 에러
	errs := Validate(aliasMap, registry, true)
	if len(errs) != 1 {
		t.Fatalf("Validate(strict) returned %d errors, want 1", len(errs))
	}
}

// TestValidate_LenientModeLenient 모드에서 에러 반환하지만 계속 진행 테스트
func TestValidate_LenientMode(t *testing.T) {
	registry := router.DefaultRegistry()
	aliasMap := map[string]string{
		"good":    "openai/gpt-4o",     // valid
		"badfmt":  "invalid-format",    // invalid format (no slash)
	}

	// Lenient mode: 에러는 반환하지만 호출자가 무시할 수 있음
	// strict=false이므로 provider/model 검증은 하지 않음
	errs := Validate(aliasMap, registry, false)
	if len(errs) != 1 {
		t.Fatalf("Validate(lenient) returned %d errors, want 1", len(errs))
	}
	// Lenient 모드에서는 에러가 있어도 aliasMap은 유효할 수 있음
}
