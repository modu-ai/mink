// Package aliasconfig 통합 테스트
// 실제 aliases.yaml 파일로 전체 흐름 테스트
package aliasconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/modu-ai/goose/internal/llm/router"
	"go.uber.org/zap"
)

// TestIntegration_FullFlow 전체 흐름 통합 테스트
func TestIntegration_FullFlow(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("GOOSE_HOME", tmpDir)

	// 1. 유효한 aliases.yaml 생성
	configPath := filepath.Join(tmpDir, "aliases.yaml")
	yamlContent := `aliases:
  # 단축 별칭
  gpt4: openai/gpt-4
  claude: anthropic/claude-sonnet-4-6
  gemini: google/gemini-2.0-flash
  hyperclova: openai/gpt-4
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// 2. Loader 생성
	opts := Options{
		Logger: zap.NewNop(),
	}
	loader := New(opts)

	// 3. 로드
	aliasMap, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// 4. 기본 검증
	if aliasMap == nil {
		t.Fatal("aliasMap is nil")
	}

	// 5. 별칭 개수 확인
	if len(aliasMap) != 4 {
		t.Errorf("len(aliasMap) = %d, want 4", len(aliasMap))
	}

	// 6. 특정 별칭 확인
	if aliasMap["gpt4"] != "openai/gpt-4" {
		t.Errorf("aliasMap[\"gpt4\"] = %s, want \"openai/gpt-4\"", aliasMap["gpt4"])
	}
	if aliasMap["claude"] != "anthropic/claude-sonnet-4-6" {
		t.Errorf("aliasMap[\"claude\"] = %s, want \"anthropic/claude-sonnet-4-6\"", aliasMap["claude"])
	}

	// 7. provider registry 검증
	registry := router.DefaultRegistry()
	errs := Validate(aliasMap, registry, true)
	if errs != nil {
		t.Fatalf("Validate() returned %d errors, want 0", len(errs))
	}
}

// TestIntegration_InvalidConfig 잘못된 설정 파일 처리 통합 테스트
func TestIntegration_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("GOOSE_HOME", tmpDir)

	// 잘못된 형식의 aliases.yaml
	configPath := filepath.Join(tmpDir, "aliases.yaml")
	yamlContent := `aliases:
  bad-target: invalid-format
  empty-alias: ""
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	opts := Options{Logger: zap.NewNop()}
	loader := New(opts)

	// 로드는 성공해야 함 (형식은 유효하나 내용이 잘못됨)
	aliasMap, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil (YAML format is valid)", err)
	}

	// 검증에서 에러 발생해야 함
	registry := router.DefaultRegistry()
	errs := Validate(aliasMap, registry, true)
	if len(errs) < 2 {
		t.Errorf("Validate() returned %d errors, want at least 2", len(errs))
	}
}

// TestIntegration_UnknownProvider 존재하지 않는 provider 통합 테스트
func TestIntegration_UnknownProvider(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("GOOSE_HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "aliases.yaml")
	yamlContent := `aliases:
  unknown: nonexistent/model
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	opts := Options{Logger: zap.NewNop()}
	loader := New(opts)

	aliasMap, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	registry := router.DefaultRegistry()
	errs := Validate(aliasMap, registry, true)
	if len(errs) != 1 {
		t.Errorf("Validate() returned %d errors, want 1", len(errs))
	}
}

// TestIntegration_LenientModeLenient 모드에서 검증 실패 시에도 별칭 유지 테스트
func TestIntegration_LenientMode(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("GOOSE_HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "aliases.yaml")
	yamlContent := `aliases:
  valid: openai/gpt-4
  invalid: nonexistent/model
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	opts := Options{Logger: zap.NewNop()}
	loader := New(opts)

	aliasMap, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Lenient mode: 검증 에러가 있어도 aliasMap은 유지됨
	registry := router.DefaultRegistry()
	errs := Validate(aliasMap, registry, false) // lenient
	if len(errs) != 1 {
		t.Errorf("Validate(lenient) returned %d errors, want 1", len(errs))
	}

	// 별칭이 여전히 존재해야 함
	if len(aliasMap) != 2 {
		t.Errorf("len(aliasMap) = %d, want 2", len(aliasMap))
	}
	if aliasMap["valid"] != "openai/gpt-4" {
		t.Errorf("aliasMap[\"valid\"] = %s, want \"openai/gpt-4\"", aliasMap["valid"])
	}
}

// TestIntegration_CommentHandling 주석 처리 테스트
func TestIntegration_CommentHandling(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("GOOSE_HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "aliases.yaml")
	yamlContent := `aliases:
  # 주석 라인
  gpt4: openai/gpt-4

  # 여러 주석
  # 이것은 설명입니다
  claude: anthropic/claude-sonnet-4-6
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	opts := Options{Logger: zap.NewNop()}
	loader := New(opts)

	aliasMap, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(aliasMap) != 2 {
		t.Errorf("len(aliasMap) = %d, want 2 (comments should be ignored)", len(aliasMap))
	}
}

// TestIntegration_HookRegistrySetAliasMap HookRegistry에 aliasMap 설정 테스트
func TestIntegration_HookRegistrySetAliasMap(t *testing.T) {
	// HookRegistry는 internal/hook 패키지에 있으므로
	// 이 테스트는 loader 패키지에서는 skip하고
	// 실제 통합 테스트에서 검증됨
	t.Skip("HookRegistry integration is tested in cmd/goosed package")
}
