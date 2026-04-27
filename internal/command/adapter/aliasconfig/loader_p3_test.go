// Package aliasconfig P3 optional features test
// SPEC-GOOSE-ALIAS-CONFIG-001 P3: 프로젝트 로컬 오버레이, fs.FS 주입, 로거 관찰자 테스트
package aliasconfig

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// TestProjectLocalAliasFile 프로젝트 로컬 별칭 파일 감지 테스트
func TestProjectLocalAliasFile(t *testing.T) {
	t.Run("project local file exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectLocalPath := filepath.Join(tmpDir, ".goose")
		if err := os.MkdirAll(projectLocalPath, 0o755); err != nil {
			t.Fatalf("mkdir .goose: %v", err)
		}

		configPath := filepath.Join(projectLocalPath, "aliases.yaml")
		yamlContent := `aliases:
  gpt4: openai/gpt-4
`
		if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		// Change working directory to temp dir
		originalWd, _ := os.Getwd()
		defer os.Chdir(originalWd)
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("chdir: %v", err)
		}

		opts := Options{Logger: zaptest.NewLogger(t)}
		loader := New(opts)

		// Verify project-local path is detected (use filepath.EvalSymlinks for macOS)
		realConfigPath, err := filepath.EvalSymlinks(configPath)
		if err != nil {
			t.Fatalf("EvalSymlinks: %v", err)
		}
		realLoaderPath, err := filepath.EvalSymlinks(loader.configPath)
		if err != nil {
			t.Fatalf("EvalSymlinks loader: %v", err)
		}
		if realLoaderPath != realConfigPath {
			t.Errorf("configPath = %s, want %s", realLoaderPath, realConfigPath)
		}

		// Verify loading works
		aliasMap, err := loader.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if aliasMap == nil {
			t.Fatal("aliasMap is nil")
		}
		if aliasMap["gpt4"] != "openai/gpt-4" {
			t.Errorf("aliasMap[\"gpt4\"] = %s, want \"openai/gpt-4\"", aliasMap["gpt4"])
		}
	})

	t.Run("project local file does not exist, fallback to home", func(t *testing.T) {
		tmpDir := t.TempDir()

		originalWd, _ := os.Getwd()
		defer os.Chdir(originalWd)
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("chdir: %v", err)
		}

		// Set GOOSE_HOME to temp dir
		t.Setenv("GOOSE_HOME", tmpDir)

		// Create global config in GOOSE_HOME
		globalConfigPath := filepath.Join(tmpDir, "aliases.yaml")
		yamlContent := `aliases:
  claude: anthropic/claude-sonnet-4-6
`
		if err := os.WriteFile(globalConfigPath, []byte(yamlContent), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		opts := Options{Logger: zaptest.NewLogger(t)}
		loader := New(opts)

		// Verify global path is used (no .goose/aliases.yaml exists)
		if loader.configPath != globalConfigPath {
			t.Errorf("configPath = %s, want %s", loader.configPath, globalConfigPath)
		}
	})

	t.Run("custom path overrides project local", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectLocalPath := filepath.Join(tmpDir, ".goose")
		if err := os.MkdirAll(projectLocalPath, 0o755); err != nil {
			t.Fatalf("mkdir .goose: %v", err)
		}

		configPath := filepath.Join(projectLocalPath, "aliases.yaml")
		if err := os.WriteFile(configPath, []byte("aliases:\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		originalWd, _ := os.Getwd()
		defer os.Chdir(originalWd)
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("chdir: %v", err)
		}

		customPath := "/custom/path/aliases.yaml"
		opts := Options{
			ConfigPath: customPath,
			Logger:     zaptest.NewLogger(t),
		}
		loader := New(opts)

		// Custom path should override project-local detection
		if loader.configPath != customPath {
			t.Errorf("configPath = %s, want %s", loader.configPath, customPath)
		}
	})
}

// TestFSInjection fs.FS 인터페이스 주입 테스트
func TestFSInjection(t *testing.T) {
	t.Run("default filesystem when nil (uses real OS)", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "aliases.yaml")
		yamlContent := `aliases:
  test: openai/gpt-4
`
		if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		opts := Options{
			ConfigPath: configPath,
			FS:         nil, // Should default to osFS
			Logger:     zaptest.NewLogger(t),
		}
		loader := New(opts)

		// Verify default filesystem is used
		if loader.fsys == nil {
			t.Fatal("loader.fsys is nil, want non-nil filesystem")
		}

		aliasMap, err := loader.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if aliasMap == nil {
			t.Fatal("aliasMap is nil")
		}
		if aliasMap["test"] != "openai/gpt-4" {
			t.Errorf("aliasMap[\"test\"] = %s, want \"openai/gpt-4\"", aliasMap["test"])
		}
	})

	t.Run("default filesystem with file not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "nonexistent.yaml")

		opts := Options{
			ConfigPath: configPath,
			FS:         nil,
			Logger:     zaptest.NewLogger(t),
		}
		loader := New(opts)

		aliasMap, err := loader.Load()
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if aliasMap != nil {
			t.Errorf("aliasMap = %v, want nil", aliasMap)
		}
	})

	t.Run("default filesystem with invalid YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "aliases.yaml")

		if err := os.WriteFile(configPath, []byte("invalid:\n  - unclosed: [\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		opts := Options{
			ConfigPath: configPath,
			FS:         nil,
			Logger:     zaptest.NewLogger(t),
		}
		loader := New(opts)

		_, err := loader.Load()
		if err == nil {
			t.Fatal("Load() error = nil, want error")
		}
	})

	t.Run("default filesystem with large file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "aliases.yaml")

		largeContent := make([]byte, maxAliasFileSize+1)
		if err := os.WriteFile(configPath, largeContent, 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		opts := Options{
			ConfigPath: configPath,
			FS:         nil,
			Logger:     zaptest.NewLogger(t),
		}
		loader := New(opts)

		_, err := loader.Load()
		if err == nil {
			t.Fatal("Load() error = nil, want error for large file")
		}
	})
}

// TestLoggerObserver 로거 관찰자 테스트
func TestLoggerObserver(t *testing.T) {
	t.Run("std logger captures file loaded", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "aliases.yaml")
		yamlContent := `aliases:
  gpt4: openai/gpt-4
`
		if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		// Capture log output
		var logBuf bytes.Buffer
		testLogger := log.New(&logBuf, "[aliasconfig] ", log.LstdFlags)

		opts := Options{
			ConfigPath: configPath,
			Logger:     zap.NewNop(),
			StdLogger:  testLogger,
		}
		loader := New(opts)

		aliasMap, err := loader.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		logOutput := logBuf.String()
		if logOutput == "" {
			t.Error("std logger produced no output")
		}

		// Verify log contains expected messages
		if !contains(logOutput, "loading config from:") {
			t.Error("log output missing 'loading config from:' message")
		}
		if !contains(logOutput, "loaded 1 alias entries") {
			t.Error("log output missing 'loaded 1 alias entries' message")
		}

		// Verify aliases are still loaded correctly
		if aliasMap["gpt4"] != "openai/gpt-4" {
			t.Errorf("aliasMap[\"gpt4\"] = %s, want \"openai/gpt-4\"", aliasMap["gpt4"])
		}
	})

	t.Run("std logger captures validation error", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "aliases.yaml")

		// Write invalid YAML
		if err := os.WriteFile(configPath, []byte("invalid:\n  - unclosed: [\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		var logBuf bytes.Buffer
		testLogger := log.New(&logBuf, "[aliasconfig] ", log.LstdFlags)

		opts := Options{
			ConfigPath: configPath,
			Logger:     zap.NewNop(),
			StdLogger:  testLogger,
		}
		loader := New(opts)

		_, err := loader.Load()
		if err == nil {
			t.Fatal("Load() error = nil, want error")
		}

		logOutput := logBuf.String()
		if !contains(logOutput, "yaml parsing error:") {
			t.Error("log output missing 'yaml parsing error:' message")
		}
	})

	t.Run("std logger captures file not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "nonexistent.yaml")

		var logBuf bytes.Buffer
		testLogger := log.New(&logBuf, "[aliasconfig] ", log.LstdFlags)

		opts := Options{
			ConfigPath: configPath,
			Logger:     zap.NewNop(),
			StdLogger:  testLogger,
		}
		loader := New(opts)

		aliasMap, err := loader.Load()
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if aliasMap != nil {
			t.Errorf("aliasMap = %v, want nil", aliasMap)
		}

		logOutput := logBuf.String()
		if !contains(logOutput, "config file not found:") {
			t.Error("log output missing 'config file not found:' message")
		}
	})

	t.Run("std logger disabled when nil", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "aliases.yaml")
		yamlContent := `aliases:
  test: openai/gpt-4
`
		if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		opts := Options{
			ConfigPath: configPath,
			Logger:     zaptest.NewLogger(t),
			StdLogger:  nil, // No std logger
		}
		loader := New(opts)

		// Should not panic when stdLogger is nil
		aliasMap, err := loader.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if aliasMap == nil {
			t.Fatal("aliasMap is nil")
		}
	})

	t.Run("std logger captures large file error", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "aliases.yaml")

		// Write large file
		largeContent := make([]byte, maxAliasFileSize+1)
		if err := os.WriteFile(configPath, largeContent, 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		var logBuf bytes.Buffer
		testLogger := log.New(&logBuf, "[aliasconfig] ", log.LstdFlags)

		opts := Options{
			ConfigPath: configPath,
			Logger:     zap.NewNop(),
			StdLogger:  testLogger,
		}
		loader := New(opts)

		_, err := loader.Load()
		if err == nil {
			t.Fatal("Load() error = nil, want error for large file")
		}

		logOutput := logBuf.String()
		if !contains(logOutput, "file too large:") {
			t.Error("log output missing 'file too large:' message")
		}
	})
}

// contains는 strings.Contains 함수의 간단한 구현체이다.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

// findSubstring은 부분 문자열 검색을 수행한다.
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
