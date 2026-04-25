// Package terminal은 터미널 관련 내장 tool을 제공한다.
// SPEC-GOOSE-TOOLS-001 §3.1 #3 terminal/*
package terminal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/modu-ai/goose/internal/tools"
	"github.com/modu-ai/goose/internal/tools/builtin"
)

func init() {
	builtin.Register(NewBash())
}

const (
	defaultTimeoutMs = 120_000 // 2분
	maxTimeoutMs     = 600_000 // 10분 (agent-common-protocol.md)
	gracePeriod      = 2 * time.Second
)

// secretPatterns는 secret 환경변수 이름 heuristic 패턴이다.
// REQ-TOOLS-016
var secretPatterns = []string{
	"_TOKEN",
	"_KEY",
	"_SECRET",
	"GOOSE_SHUTDOWN_TOKEN",
}

// bashInput은 Bash tool 입력이다.
type bashInput struct {
	Command        string `json:"command"`
	TimeoutMs      *int   `json:"timeout_ms,omitempty"`
	WorkingDir     string `json:"working_dir,omitempty"`
	InheritSecrets *bool  `json:"inherit_secrets,omitempty"`
}

// bashTool은 셸 명령을 실행하는 tool이다.
type bashTool struct {
	preApproved bool // pre-approval 상태 (테스트 지원)
}

// NewBash는 새 Bash tool을 반환한다.
func NewBash() tools.Tool {
	return &bashTool{}
}

// BashWithPreApproval은 pre-approval 상태를 설정한 Bash tool을 반환한다.
// 테스트 및 Executor 내부에서 pre-approval 여부 주입을 위해 사용.
func BashWithPreApproval(preApproved bool) tools.Tool {
	return &bashTool{preApproved: preApproved}
}

func (t *bashTool) Name() string { return "Bash" }

func (t *bashTool) Schema() json.RawMessage {
	return json.RawMessage(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "command": {
      "type": "string",
      "description": "실행할 셸 명령"
    },
    "timeout_ms": {
      "type": "integer",
      "description": "타임아웃 (밀리초, 기본 120000, 최대 600000)",
      "minimum": 1,
      "maximum": 600000
    },
    "working_dir": {
      "type": "string",
      "description": "명령 실행 디렉토리"
    },
    "inherit_secrets": {
      "type": "boolean",
      "description": "secret 환경변수 상속 여부 (pre-approval 필수)"
    }
  },
  "required": ["command"],
  "additionalProperties": false
}`)
}

func (t *bashTool) Scope() tools.Scope { return tools.ScopeShared }

// waitResult는 cmd.Wait() 결과를 담는 구조체이다.
type waitResult struct {
	stdout []byte
	stderr []byte
	err    error
}

func (t *bashTool) Call(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	var inp bashInput
	if err := json.Unmarshal(input, &inp); err != nil {
		return tools.ToolResult{IsError: true, Content: []byte("invalid input: " + err.Error())}, nil
	}

	// 타임아웃 결정
	timeoutMs := defaultTimeoutMs
	if inp.TimeoutMs != nil {
		timeoutMs = *inp.TimeoutMs
		if timeoutMs > maxTimeoutMs {
			timeoutMs = maxTimeoutMs
		}
	}
	timeout := time.Duration(timeoutMs) * time.Millisecond

	// REQ-TOOLS-016: inherit_secrets=true AND pre-approval 양쪽 만족 시에만 secret 전달
	inheritSecrets := inp.InheritSecrets != nil && *inp.InheritSecrets && t.preApproved
	env := filterEnv(os.Environ(), inheritSecrets)

	cmd := exec.Command("sh", "-c", inp.Command)
	if inp.WorkingDir != "" {
		cmd.Dir = inp.WorkingDir
	}
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return tools.ToolResult{IsError: true, Content: []byte(fmt.Sprintf("start_error: %v", err))}, nil
	}

	// Wait를 단일 goroutine에서만 호출 (race 방지)
	doneCh := make(chan waitResult, 1)
	go func() {
		waitErr := cmd.Wait()
		// stdout/stderr은 Wait 완료 후 안전하게 읽을 수 있음
		doneCh <- waitResult{
			stdout: append([]byte{}, stdoutBuf.Bytes()...),
			stderr: append([]byte{}, stderrBuf.Bytes()...),
			err:    waitErr,
		}
	}()

	select {
	case wr := <-doneCh:
		// 정상 완료
		exitCode := 0
		if wr.err != nil {
			if exitErr, ok := wr.err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}
		content, _ := json.Marshal(map[string]any{
			"stdout":    string(wr.stdout),
			"stderr":    string(wr.stderr),
			"exit_code": exitCode,
		})
		return tools.ToolResult{
			Content: content,
			Metadata: map[string]any{
				"exit_code": exitCode,
				"stdout":    string(wr.stdout),
				"stderr":    string(wr.stderr),
			},
		}, nil

	case <-time.After(timeout):
		// REQ-TOOLS-010: SIGTERM → 2s grace → SIGKILL
		// 프로세스 그룹 전체에 SIGTERM 전송
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		}

		// 2초 동안 graceful exit 대기
		select {
		case wr := <-doneCh:
			// grace 기간 내 종료됨
			stdout := wr.stdout
			stderr := wr.stderr
			return tools.ToolResult{
				IsError: true,
				Content: []byte(fmt.Sprintf("timeout: %v", timeout)),
				Metadata: map[string]any{
					"exit_code":      -1,
					"stdout_partial": string(stdout),
					"stderr_partial": string(stderr),
				},
			}, nil
		case <-time.After(gracePeriod):
			// SIGKILL
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
			// Wait 완료 대기 (짧게)
			select {
			case wr := <-doneCh:
				return tools.ToolResult{
					IsError: true,
					Content: []byte(fmt.Sprintf("timeout: %v", timeout)),
					Metadata: map[string]any{
						"exit_code":      -1,
						"stdout_partial": string(wr.stdout),
						"stderr_partial": string(wr.stderr),
					},
				}, nil
			case <-time.After(500 * time.Millisecond):
				// 마지막 수단
				return tools.ToolResult{
					IsError: true,
					Content: []byte(fmt.Sprintf("timeout: %v", timeout)),
					Metadata: map[string]any{
						"exit_code":      -1,
						"stdout_partial": "",
						"stderr_partial": "",
					},
				}, nil
			}
		}
	}
}

// filterEnv는 환경변수에서 secret을 필터링한다.
// REQ-TOOLS-016
func filterEnv(environ []string, inheritSecrets bool) []string {
	if inheritSecrets {
		return environ
	}
	filtered := make([]string, 0, len(environ))
	for _, kv := range environ {
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			filtered = append(filtered, kv)
			continue
		}
		key := kv[:idx]
		if isSecretEnvVar(key) {
			continue
		}
		filtered = append(filtered, kv)
	}
	return filtered
}

// isSecretEnvVar는 환경변수 이름이 secret heuristic 패턴에 해당하는지 확인한다.
func isSecretEnvVar(key string) bool {
	for _, pattern := range secretPatterns {
		if strings.HasSuffix(key, pattern) {
			return true
		}
		if key == pattern {
			return true
		}
	}
	return false
}
