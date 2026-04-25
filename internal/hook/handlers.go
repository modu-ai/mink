package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
)

// InlineCommandHandler는 shell command를 실행하는 HookHandler 구현체이다.
// REQ-HK-006: stdin에 HookInput JSON 전달, stdout에서 HookJSONOutput 파싱.
//
// @MX:WARN: [AUTO] subprocess 실행 시 env scrub / CWD pin / rlimit 적용 필요
// @MX:REASON: REQ-HK-021 4가지 isolation 보장이 명시적으로 요구됨 — 누락 시 보안 취약점
type InlineCommandHandler struct {
	// Command는 실행할 shell command이다.
	Command string
	// Matcher는 이 핸들러가 매치되는 조건이다.
	Matcher string
	// Shell은 command를 실행할 shell이다. 기본값은 /bin/sh.
	Shell string
	// Timeout은 subprocess 실행 타임아웃이다. 0이면 defaultShellTimeout 적용.
	// D17: cfg.Timeout <= 0이면 30s 기본값 사용.
	Timeout time.Duration
	// ID는 핸들러 식별자로 로그에 사용된다.
	ID string
	// Resolver는 WorkspaceRoot resolver이다 (REQ-HK-021 b).
	// nil이면 CWD pin을 건너뛴다 (테스트 편의).
	Resolver WorkspaceRootResolver
	// Logger는 구조화 로거이다.
	Logger *zap.Logger
}

// Matches는 입력에 대한 matcher 평가 결과를 반환한다.
// REQ-HK-020: glob 또는 regex: 접두사.
func (h *InlineCommandHandler) Matches(input HookInput) bool {
	var target string
	switch input.HookEvent {
	case EvPreToolUse, EvPostToolUse, EvPostToolUseFailure:
		if input.Tool != nil {
			target = input.Tool.Name
		}
	case EvFileChanged:
		// FileChanged는 path glob matching — 경로 하나라도 매치되면 true
		for _, p := range input.ChangedPaths {
			if matcherMatches(h.Matcher, p) {
				return true
			}
		}
		return false
	default:
		target = string(input.HookEvent)
	}
	return matcherMatches(h.Matcher, target)
}

// Handle은 shell command를 실행하고 결과를 반환한다.
// REQ-HK-006 전체 클로즈 구현.
func (h *InlineCommandHandler) Handle(ctx context.Context, input HookInput) (HookJSONOutput, error) {
	logger := h.logger()

	// 1. 타임아웃 바인딩 (D17: Timeout <= 0이면 defaultShellTimeout 적용)
	timeout := h.Timeout
	configWarn := false
	if timeout <= 0 {
		timeout = time.Duration(defaultShellTimeout) * time.Second
		configWarn = true
	}

	// 2. HookInput JSON 직렬화 + 4 MiB cap 검증 (REQ-HK-022)
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return HookJSONOutput{}, fmt.Errorf("marshal HookInput: %w", err)
	}
	if len(inputJSON) > maxPayloadBytes {
		logger.Warn("hook payload too large",
			zap.String("event", string(input.HookEvent)),
			zap.String("handler_id", h.id()),
			zap.Int("payload_bytes", len(inputJSON)),
		)
		return HookJSONOutput{}, ErrHookPayloadTooLarge
	}

	// config_warn 로그 (D17 edge case)
	if configWarn {
		logger.Warn("hook timeout config_warn: cfg.Timeout <= 0, using default 30s",
			zap.String("event", string(input.HookEvent)),
			zap.String("handler_id", h.id()),
			zap.String("outcome", "config_warn"),
		)
	}

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 3. Shell 결정
	shell := h.Shell
	if shell == "" {
		shell = "/bin/sh"
	}

	// 4. 명령 준비 — no sudo (REQ-HK-016)
	cmd := buildCmd(cctx, shell, h.Command)

	// 5. Env scrub (REQ-HK-021 a)
	cmd.Env = scrubEnv(os.Environ())

	// 6. CWD pin (REQ-HK-021 b)
	if h.Resolver != nil {
		wspath, rerr := h.Resolver.WorkspaceRoot(input.SessionID)
		if rerr != nil || wspath == "" {
			return HookJSONOutput{}, ErrHookSessionUnresolved
		}
		cmd.Dir = wspath
	}

	// 7. rlimit + close-on-exec (플랫폼별 — isolation_unix.go / isolation_other.go)
	applySysProcAttr(cmd, timeout, logger)

	// 8. stdin/stdout/stderr 파이프
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return HookJSONOutput{}, fmt.Errorf("stdin pipe: %w", err)
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = limitWriter(&stderrBuf, 4096)

	// GOOSE_HOOK_TRACE 활성화 시 DEBUG 로그 (REQ-HK-019)
	traceEnabled := isTraceEnabled()

	if err := cmd.Start(); err != nil {
		return HookJSONOutput{}, fmt.Errorf("start subprocess: %w", err)
	}

	// 9. stdin goroutine write (REQ-HK-022: slow-reading child 데드락 방지)
	go func() {
		defer stdinPipe.Close()
		select {
		case <-cctx.Done():
			return
		default:
		}
		_, _ = stdinPipe.Write(inputJSON)
	}()

	// 10. Wait + exit code 분기 (REQ-HK-006 e/f/g)
	waitErr := cmd.Wait()
	exitCode := exitCodeOf(waitErr)

	out := HookJSONOutput{}
	var handlerErr error

	switch {
	case exitCode == 0:
		// stdout이 있으면 HookJSONOutput으로 파싱 (clause d, g)
		if stdoutBuf.Len() > 0 {
			if jerr := json.Unmarshal(stdoutBuf.Bytes(), &out); jerr != nil {
				handlerErr = fmt.Errorf("exit 0 malformed JSON: %w", jerr)
				logger.Error("handler_error: malformed stdout JSON",
					zap.String("event", string(input.HookEvent)),
					zap.String("handler_id", h.id()),
					zap.Error(jerr),
				)
			}
		}

	case exitCode == 2:
		// blocking signal (REQ-HK-006 e): stderr → PermissionDecision.Reason
		reason := strings.TrimSpace(stderrBuf.String())
		f := false
		out = HookJSONOutput{
			Continue: &f,
			PermissionDecision: &PermissionDecision{
				Approve: false,
				Reason:  reason,
			},
		}

	default:
		// non-zero exit != 2: handler_error, log and continue (REQ-HK-006 f)
		handlerErr = fmt.Errorf("subprocess exited %d", exitCode)
		logger.Error("handler_error: subprocess non-zero exit",
			zap.String("event", string(input.HookEvent)),
			zap.String("handler_id", h.id()),
			zap.Int("exit_code", exitCode),
		)
	}

	// trace 로그 (REQ-HK-019)
	if traceEnabled {
		inputStr, _ := json.Marshal(input)
		outStr, _ := json.Marshal(out)
		logger.Debug("hook trace",
			zap.String("handler_id", h.id()),
			zap.String("input", string(inputStr)),
			zap.String("output", string(outStr)),
		)
	}

	return out, handlerErr
}

func (h *InlineCommandHandler) id() string {
	if h.ID != "" {
		return h.ID
	}
	return "inline:" + h.Command
}

func (h *InlineCommandHandler) logger() *zap.Logger {
	if h.Logger != nil {
		return h.Logger
	}
	return zap.NewNop()
}

// InlineFuncHandler는 Go 함수를 HookHandler로 래핑한다.
type InlineFuncHandler struct {
	// Fn은 실행할 Go 함수이다.
	Fn func(ctx context.Context, input HookInput) (HookJSONOutput, error)
	// MatcherFn은 매처 함수이다. nil이면 항상 true를 반환한다.
	MatcherFn func(input HookInput) bool
}

// Matches는 MatcherFn이 nil이면 true를 반환한다.
func (h *InlineFuncHandler) Matches(input HookInput) bool {
	if h.MatcherFn == nil {
		return true
	}
	return h.MatcherFn(input)
}

// Handle은 Fn을 호출한다.
func (h *InlineFuncHandler) Handle(ctx context.Context, input HookInput) (HookJSONOutput, error) {
	return h.Fn(ctx, input)
}

// limitWriter는 최대 maxBytes 바이트까지만 쓰는 io.Writer 래퍼이다.
func limitWriter(w io.Writer, maxBytes int64) io.Writer {
	return &limitedWriter{w: w, n: maxBytes}
}

type limitedWriter struct {
	w io.Writer
	n int64
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.n <= 0 {
		return len(p), nil // 초과 데이터는 조용히 버린다
	}
	if int64(len(p)) > l.n {
		p = p[:l.n]
	}
	n, err := l.w.Write(p)
	l.n -= int64(n)
	return n, err
}

// isTraceEnabled는 GOOSE_HOOK_TRACE 환경변수 활성화 여부를 반환한다.
// REQ-HK-019 / §6.11.5: "1", "true", "on" (case-insensitive).
func isTraceEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("GOOSE_HOOK_TRACE")))
	return v == "1" || v == "true" || v == "on"
}
