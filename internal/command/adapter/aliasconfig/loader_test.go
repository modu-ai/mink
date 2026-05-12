// Package aliasconfig 테스트
package aliasconfig

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

// TestNew_Loader 기본 생성자 테스트
func TestNew_Loader(t *testing.T) {
	opts := Options{
		Logger: zap.NewNop(),
	}
	loader := New(opts)

	if loader == nil {
		t.Fatal("New() returned nil")
	}
	if loader.configPath == "" {
		t.Error("configPath is empty")
	}
}

// TestNew_Loader_CustomPath 사용자 정의 경로 테스트
func TestNew_Loader_CustomPath(t *testing.T) {
	customPath := "/custom/path/aliases.yaml"
	opts := Options{
		ConfigPath: customPath,
		Logger:     zap.NewNop(),
	}
	loader := New(opts)

	if loader.configPath != customPath {
		t.Errorf("configPath = %s, want %s", loader.configPath, customPath)
	}
}

// TestNew_Loader_MinkHome MINK_HOME 환경변수 우선 테스트.
// SPEC-MINK-ENV-MIGRATE-001 Phase 4: GOOSE_HOME → MINK_HOME 마이그레이션.
// GOOSE_HOME backward compat 는 TestNew_Loader_AliasLoader_GooseOnly_WarnsOnce 가 별도 검증.
func TestNew_Loader_MinkHome(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("MINK_HOME", tmpDir)

	opts := Options{Logger: zap.NewNop()}
	loader := New(opts)

	expectedPath := filepath.Join(tmpDir, "aliases.yaml")
	if loader.configPath != expectedPath {
		t.Errorf("configPath = %s, want %s", loader.configPath, expectedPath)
	}
}

// --- Phase 3 alias migration sub-tests for callsite 9: homeEnv ---

// TestNew_Loader_AliasLoader_MinkOnly verifies MINK_HOME is respected.
// REQ-MINK-EM-003 callsite 9: homeEnv value changed to short key "HOME".
func TestNew_Loader_AliasLoader_MinkOnly(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("MINK_HOME", tmpDir)
	t.Setenv("GOOSE_HOME", "")

	opts := Options{Logger: zap.NewNop()}
	loader := New(opts)

	expectedPath := filepath.Join(tmpDir, "aliases.yaml")
	if loader.configPath != expectedPath {
		t.Errorf("configPath (MINK_HOME) = %s, want %s", loader.configPath, expectedPath)
	}
}

// TestNew_Loader_AliasLoader_GooseOnly_WarnsOnce verifies GOOSE_HOME alias backward compat.
// REQ-MINK-EM-002 callsite 9: GOOSE_HOME 단독 설정 시 alias 통해 동작.
func TestNew_Loader_AliasLoader_GooseOnly_WarnsOnce(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("GOOSE_HOME", tmpDir)
	t.Setenv("MINK_HOME", "")

	opts := Options{Logger: zap.NewNop()}
	loader := New(opts)

	expectedPath := filepath.Join(tmpDir, "aliases.yaml")
	if loader.configPath != expectedPath {
		t.Errorf("configPath (GOOSE_HOME alias) = %s, want %s", loader.configPath, expectedPath)
	}
}

// TestLoad_FileNotFound 파일 없으면 nil 반환 테스트
func TestLoad_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yaml")

	opts := Options{
		ConfigPath: configPath,
		Logger:     zap.NewNop(),
	}
	loader := New(opts)

	aliasMap, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if aliasMap != nil {
		t.Errorf("aliasMap = %v, want nil", aliasMap)
	}
}

// TestLoad_EmptyFile 빈 파일 처리 테스트
func TestLoad_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "aliases.yaml")

	// 빈 파일 생성
	if err := os.WriteFile(configPath, []byte(""), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	opts := Options{
		ConfigPath: configPath,
		Logger:     zap.NewNop(),
	}
	loader := New(opts)

	aliasMap, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	// 빈 파일은 nil을 반환 (별칭 없음)
	if aliasMap != nil {
		t.Errorf("aliasMap = %v, want nil", aliasMap)
	}
}

// TestLoad_ValidYAML 유효한 YAML 파싱 테스트
func TestLoad_ValidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "aliases.yaml")

	yamlContent := `aliases:
  gpt4: openai/gpt-4
  claude: anthropic/claude-sonnet-4-6
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	opts := Options{
		ConfigPath: configPath,
		Logger:     zap.NewNop(),
	}
	loader := New(opts)

	aliasMap, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if len(aliasMap) != 2 {
		t.Errorf("len(aliasMap) = %d, want 2", len(aliasMap))
	}
	if aliasMap["gpt4"] != "openai/gpt-4" {
		t.Errorf("aliasMap[\"gpt4\"] = %s, want \"openai/gpt-4\"", aliasMap["gpt4"])
	}
}

// TestLoad_InvalidYAML 잘못된 YAML 처리 테스트
func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "aliases.yaml")

	invalidYAML := `
aliases:
  - invalid: yaml
    format:
`
	if err := os.WriteFile(configPath, []byte(invalidYAML), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	opts := Options{
		ConfigPath: configPath,
		Logger:     zap.NewNop(),
	}
	loader := New(opts)

	_, err := loader.Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

// TestLoadDefault 기본 경로 로드 테스트
func TestLoadDefault(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("MINK_HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "aliases.yaml")
	yamlContent := `aliases:
  test: openai/gpt-4
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	opts := Options{Logger: zap.NewNop()}
	loader := New(opts)

	aliasMap, err := loader.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error = %v, want nil", err)
	}
	if aliasMap == nil {
		t.Error("aliasMap is nil")
	}
}

// TestValidate_EmptyMap 빈 맵 검증 테스트
func TestValidate_EmptyMap(t *testing.T) {
	errs := Validate(nil, nil, false)
	if errs != nil {
		t.Errorf("Validate(nil, _, false) = %v, want nil", errs)
	}

	errs = Validate(map[string]string{}, nil, false)
	if errs != nil {
		t.Errorf("Validate({}, _, false) = %v, want nil", errs)
	}
}

// TestParseModelTarget_Valid 올바른 형식 파싱 테스트
func TestParseModelTarget_Valid(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		provider string
		model    string
	}{
		{"simple", "openai/gpt-4", "openai", "gpt-4"},
		{"with-version", "anthropic/claude-sonnet-4-6", "anthropic", "claude-sonnet-4-6"},
		{"complex", "glm/glm-4.7-flash", "glm", "glm-4.7-flash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, model, ok := parseModelTarget(tt.target)
			if !ok {
				t.Fatalf("parseModelTarget(%q) ok = false, want true", tt.target)
			}
			if provider != tt.provider {
				t.Errorf("provider = %s, want %s", provider, tt.provider)
			}
			if model != tt.model {
				t.Errorf("model = %s, want %s", model, tt.model)
			}
		})
	}
}

// TestParseModelTarget_Invalid 잘못된 형식 파싱 테스트
func TestParseModelTarget_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{"no slash", "invalid"},
		{"no provider", "/gpt-4"},
		{"no model", "openai/"},
		{"multiple slashes", "openai/gpt/4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, ok := parseModelTarget(tt.target)
			if ok {
				t.Errorf("parseModelTarget(%q) ok = true, want false", tt.target)
			}
		})
	}
}
