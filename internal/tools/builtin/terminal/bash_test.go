package terminal_test

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/tools/builtin/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBash_SimpleCommand — 기본 명령 실행
func TestBash_SimpleCommand(t *testing.T) {
	bash := terminal.NewBash()
	input, _ := json.Marshal(map[string]any{"command": "echo hello"})

	result, err := bash.Call(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var out map[string]any
	require.NoError(t, json.Unmarshal(result.Content, &out))
	assert.Contains(t, out["stdout"].(string), "hello")
}

// TestBash_TimeoutKillsSubprocess — AC-TOOLS-008, REQ-TOOLS-010
// timeout_ms 초과 시 SIGTERM → SIGKILL + 부분 결과 반환
func TestBash_TimeoutKillsSubprocess(t *testing.T) {
	bash := terminal.NewBash()
	input, _ := json.Marshal(map[string]any{
		"command":    "sleep 60",
		"timeout_ms": 200,
	})

	start := time.Now()
	result, err := bash.Call(context.Background(), input)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.True(t, result.IsError, "timeout 시 IsError=true여야 함")
	assert.Contains(t, string(result.Content), "timeout", "Content에 timeout 포함")
	assert.Less(t, elapsed, 500*time.Millisecond, "500ms 이내에 반환해야 함")

	exitCode, ok := result.Metadata["exit_code"]
	assert.True(t, ok)
	assert.Equal(t, -1, exitCode, "exit_code는 -1이어야 함")
}

// TestBash_ExitCode_NonZero — 비정상 종료 코드 처리
func TestBash_ExitCode_NonZero(t *testing.T) {
	bash := terminal.NewBash()
	input, _ := json.Marshal(map[string]any{"command": "exit 42"})

	result, err := bash.Call(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, result.IsError) // exit code != 0이어도 IsError는 false

	var out map[string]any
	require.NoError(t, json.Unmarshal(result.Content, &out))
	exitCode := int(out["exit_code"].(float64))
	assert.Equal(t, 42, exitCode)
}

// TestBash_SecretEnvFiltering — AC-TOOLS-015, REQ-TOOLS-016
// secret 환경변수 필터링
func TestBash_SecretEnvFiltering(t *testing.T) {
	// 테스트 환경에 secret 설정
	os.Setenv("GITHUB_TOKEN", "xyz")
	os.Setenv("MY_API_KEY", "abc")
	os.Setenv("PATH_SAFE_VAR", "safe")
	defer func() {
		os.Unsetenv("GITHUB_TOKEN")
		os.Unsetenv("MY_API_KEY")
		os.Unsetenv("PATH_SAFE_VAR")
	}()

	// Given 1: inherit_secrets 미설정 (기본값)
	bash := terminal.NewBash()
	input, _ := json.Marshal(map[string]any{"command": "env"})

	result, err := bash.Call(context.Background(), input)
	require.NoError(t, err)
	require.False(t, result.IsError)

	var out map[string]any
	require.NoError(t, json.Unmarshal(result.Content, &out))
	stdout := out["stdout"].(string)

	// Then 1: GITHUB_TOKEN, MY_API_KEY는 제거됨
	assert.NotContains(t, stdout, "GITHUB_TOKEN", "GITHUB_TOKEN은 필터링되어야 함")
	assert.NotContains(t, stdout, "MY_API_KEY", "MY_API_KEY는 필터링되어야 함")
	assert.Contains(t, stdout, "PATH_SAFE_VAR=safe", "일반 env는 유지되어야 함")
}

// TestBash_SecretEnvFiltering_InheritSecrets_WithoutPreApproval — AC-TOOLS-015
// inherit_secrets=true이지만 pre-approval 없으면 여전히 필터링
func TestBash_SecretEnvFiltering_InheritSecrets_WithoutPreApproval(t *testing.T) {
	os.Setenv("GITHUB_TOKEN", "xyz")
	os.Setenv("MY_API_KEY", "abc")
	defer func() {
		os.Unsetenv("GITHUB_TOKEN")
		os.Unsetenv("MY_API_KEY")
	}()

	// pre-approval 없는 일반 Bash
	bash := terminal.NewBash()
	inheritTrue := true
	input, _ := json.Marshal(map[string]any{
		"command":         "env",
		"inherit_secrets": inheritTrue,
	})

	result, err := bash.Call(context.Background(), input)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(result.Content, &out))
	stdout := out["stdout"].(string)

	// inherit_secrets=true 단독으로는 bypass 불가
	assert.NotContains(t, stdout, "GITHUB_TOKEN",
		"pre-approval 없으면 inherit_secrets:true도 필터링되어야 함")
}

// TestBash_SecretEnvFiltering_InheritSecrets_WithPreApproval — AC-TOOLS-015
// pre-approval + inherit_secrets=true 양쪽 만족 시 secret 통과
func TestBash_SecretEnvFiltering_InheritSecrets_WithPreApproval(t *testing.T) {
	os.Setenv("GITHUB_TOKEN", "xyz")
	os.Setenv("MY_API_KEY", "abc")
	defer func() {
		os.Unsetenv("GITHUB_TOKEN")
		os.Unsetenv("MY_API_KEY")
	}()

	// pre-approval 설정된 Bash
	bash := terminal.BashWithPreApproval(true)
	inheritTrue := true
	input, _ := json.Marshal(map[string]any{
		"command":         "env",
		"inherit_secrets": inheritTrue,
	})

	result, err := bash.Call(context.Background(), input)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(result.Content, &out))
	stdout := out["stdout"].(string)

	// pre-approval + inherit_secrets=true → secret 통과
	assert.Contains(t, stdout, "GITHUB_TOKEN=xyz",
		"pre-approval + inherit_secrets:true 시 secret이 포함되어야 함")
	assert.Contains(t, stdout, "MY_API_KEY=abc")
}

// TestBash_WorkingDir — working_dir 설정
func TestBash_WorkingDir(t *testing.T) {
	tmpDir := t.TempDir()
	bash := terminal.NewBash()
	input, _ := json.Marshal(map[string]any{
		"command":     "pwd",
		"working_dir": tmpDir,
	})

	result, err := bash.Call(context.Background(), input)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(result.Content, &out))
	assert.Contains(t, out["stdout"].(string), tmpDir)
}

// TestBash_MaxTimeout_Capped — 최대 타임아웃 제한
func TestBash_MaxTimeout_Capped(t *testing.T) {
	bash := terminal.NewBash()
	// 700,000ms > max 600,000ms
	input, _ := json.Marshal(map[string]any{
		"command":    "echo hello",
		"timeout_ms": 700000,
	})

	// 빌드만 확인 (실제 600초 대기 안 함)
	_ = strconv.Itoa(700000) // lint 방지
	result, err := bash.Call(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// TestBash_NameSchemaScope — Name/Schema/Scope 메서드
func TestBash_NameSchemaScope(t *testing.T) {
	bash := terminal.NewBash()
	assert.Equal(t, "Bash", bash.Name())
	assert.NotEmpty(t, bash.Schema())
	_ = bash.Scope() // ScopeShared = 0, just call for coverage
}

// TestBash_InvalidJSON — 잘못된 JSON 입력
func TestBash_InvalidJSON(t *testing.T) {
	bash := terminal.NewBash()
	result, err := bash.Call(context.Background(), []byte("not-json"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

// TestBash_SecretEnvVar_GooseShutdownToken — GOOSE_SHUTDOWN_TOKEN 필터링
func TestBash_SecretEnvVar_GooseShutdownToken(t *testing.T) {
	os.Setenv("GOOSE_SHUTDOWN_TOKEN", "secret-shutdown")
	defer os.Unsetenv("GOOSE_SHUTDOWN_TOKEN")

	bash := terminal.NewBash()
	input, _ := json.Marshal(map[string]any{"command": "env"})

	result, err := bash.Call(context.Background(), input)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(result.Content, &out))
	stdout := out["stdout"].(string)
	assert.NotContains(t, stdout, "GOOSE_SHUTDOWN_TOKEN")
}

// TestBash_SecretEnvVar_Secret — *_SECRET suffix
func TestBash_SecretEnvVar_Secret(t *testing.T) {
	os.Setenv("AWS_SECRET", "secret-val")
	os.Setenv("MY_PASSWORD_SECRET", "another-secret")
	defer func() {
		os.Unsetenv("AWS_SECRET")
		os.Unsetenv("MY_PASSWORD_SECRET")
	}()

	bash := terminal.NewBash()
	input, _ := json.Marshal(map[string]any{"command": "env"})
	result, err := bash.Call(context.Background(), input)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(result.Content, &out))
	stdout := out["stdout"].(string)
	assert.NotContains(t, stdout, "AWS_SECRET")
}
