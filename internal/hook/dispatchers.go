package hook

import (
	"context"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

// Dispatcher는 Dispatch* 함수군의 설정을 담는 구조체이다.
//
// @MX:ANCHOR: [AUTO] 모든 Dispatch* 함수의 공통 의존성 컨테이너
// @MX:REASON: SPEC-GOOSE-HOOK-001 REQ-HK-004 — fan_in >= 5 (각 Dispatch 함수)
type Dispatcher struct {
	Registry *HookRegistry
	Logger   *zap.Logger
	// PermQueue는 useCanUseTool 권한 큐 연산 인터페이스이다.
	PermQueue PermissionQueueOps
	// Interactive는 CLI-001의 InteractiveHandler이다.
	Interactive InteractiveHandler
	// Coordinator는 SUBAGENT-001의 CoordinatorHandler이다.
	Coordinator CoordinatorHandler
	// SwarmWorker는 SUBAGENT-001의 SwarmWorkerHandler이다.
	SwarmWorker SwarmWorkerHandler
	// IsTTY는 stdout이 TTY인지 반환하는 함수이다.
	// nil이면 기본값 false (non-TTY safe default).
	IsTTY func() bool
}

// NewDispatcher는 Dispatcher를 생성한다.
func NewDispatcher(registry *HookRegistry, logger *zap.Logger) *Dispatcher {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Dispatcher{
		Registry: registry,
		Logger:   logger,
	}
}

// DispatchPreToolUse는 PreToolUse 이벤트를 디스패치한다.
// REQ-HK-005 / REQ-HK-011: Continue:false 핸들러에서 blocking, 이후 핸들러 미호출.
// REQ-HK-004: 구조화 로그 emit.
func (d *Dispatcher) DispatchPreToolUse(ctx context.Context, input HookInput) (PreToolUseResult, error) {
	input.HookEvent = EvPreToolUse
	if err := validateInput(input); err != nil {
		return PreToolUseResult{}, err
	}

	handlers := d.Registry.Handlers(EvPreToolUse, input)
	start := time.Now()
	outcome := "ok"
	var handlerErr error
	var result PreToolUseResult

	for _, h := range handlers {
		in := deepCopyInput(input)

		// async 핸들러 처리 (REQ-HK-010)
		if out, err := d.invokeHandler(ctx, h, in); err != nil {
			outcome = "handler_error"
			handlerErr = err
			d.Logger.Error("handler_error",
				zap.String("event", string(EvPreToolUse)),
				zap.Error(err),
			)
			// 에러 핸들러는 건너뛰고 계속 진행 (REQ-HK-006 f)
			continue
		} else {
			// async 처리 (REQ-HK-010): Async:true이면 goroutine으로 실행하고 결과 무시
			if out.Async {
				asyncTimeout := out.AsyncTimeout
				if asyncTimeout <= 0 {
					asyncTimeout = defaultAsyncTimeout
				}
				go func(h HookHandler, in HookInput) {
					asyncCtx, cancel := context.WithTimeout(context.Background(), time.Duration(asyncTimeout)*time.Second)
					defer cancel()
					asyncOut, _ := h.Handle(asyncCtx, in)
					d.Logger.Debug("async hook completed (discarded for PreToolUse)",
						zap.String("event", string(EvPreToolUse)),
						zap.String("additional_context", asyncOut.AdditionalContext),
					)
				}(h, in)
				continue
			}

			// exit code 2 또는 Continue:false → blocking (REQ-HK-005 b/c/d)
			if out.Continue != nil && !*out.Continue {
				outcome = "blocked"
				result = PreToolUseResult{
					Blocked:            true,
					PermissionDecision: out.PermissionDecision,
				}
				d.emitDispatchLog(EvPreToolUse, len(handlers), outcome, time.Since(start))
				return result, nil // 즉시 반환, 이후 핸들러 미호출 (REQ-HK-011)
			}
		}
	}

	_ = handlerErr // 에러는 로그로만 기록, 반환은 nil
	d.emitDispatchLog(EvPreToolUse, len(handlers), outcome, time.Since(start))
	return result, nil
}

// DispatchPostToolUse는 PostToolUse 이벤트를 디스패치한다.
// REQ-HK-006: shell command hook stdin/stdout roundtrip.
// REQ-HK-010: async 핸들러의 AdditionalContext 누적.
func (d *Dispatcher) DispatchPostToolUse(ctx context.Context, input HookInput) (PostToolUseResult, error) {
	input.HookEvent = EvPostToolUse
	if err := validateInput(input); err != nil {
		return PostToolUseResult{}, err
	}

	handlers := d.Registry.Handlers(EvPostToolUse, input)
	start := time.Now()
	outcome := "ok"

	result := PostToolUseResult{}
	asyncCount := 0
	asyncCh := make(chan string, len(handlers))

	for _, h := range handlers {
		in := deepCopyInput(input)
		out, err := d.invokeHandler(ctx, h, in)
		if err != nil {
			outcome = "handler_error"
			d.Logger.Error("handler_error",
				zap.String("event", string(EvPostToolUse)),
				zap.Error(err),
			)
			continue
		}

		if out.Async {
			// PostToolUse: async 완료 시 AdditionalContext 누적 (REQ-HK-010)
			asyncTimeout := out.AsyncTimeout
			if asyncTimeout <= 0 {
				asyncTimeout = defaultAsyncTimeout
			}
			asyncCount++
			go func(h HookHandler, in HookInput, timeout int) {
				asyncCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
				defer cancel()
				asyncOut, _ := h.Handle(asyncCtx, in)
				asyncCh <- asyncOut.AdditionalContext
			}(h, in, asyncTimeout)
			continue
		}

		result.Outputs = append(result.Outputs, out)
		if out.SuppressOutput {
			result.SuppressOutput = true
		}
		if out.AdditionalContext != "" {
			result.AdditionalContext += out.AdditionalContext
		}
		if out.UpdatedMCPToolOutput != nil {
			result.UpdatedMCPToolOutput = out.UpdatedMCPToolOutput
		}
	}

	// async 핸들러 대기: 정확히 asyncCount 개만 수집 후 채널 닫기
	for i := 0; i < asyncCount; i++ {
		ac := <-asyncCh
		if ac != "" {
			result.AdditionalContext += ac
		}
	}
	close(asyncCh)

	d.emitDispatchLog(EvPostToolUse, len(handlers), outcome, time.Since(start))
	return result, nil
}

// DispatchSessionStart는 SessionStart 이벤트를 디스패치한다.
// REQ-HK-007: watchPaths 합집합 (절대경로 dedup), initialUserMessage는 마지막 non-empty.
func (d *Dispatcher) DispatchSessionStart(ctx context.Context, input HookInput) (SessionStartResult, error) {
	input.HookEvent = EvSessionStart
	handlers := d.Registry.Handlers(EvSessionStart, input)
	start := time.Now()
	outcome := "ok"

	result := SessionStartResult{}
	watchPathsSet := make(map[string]struct{})
	initialUserMessageSetCount := 0

	for _, h := range handlers {
		in := deepCopyInput(input)
		out, err := d.invokeHandler(ctx, h, in)
		if err != nil {
			outcome = "handler_error"
			d.Logger.Error("handler_error",
				zap.String("event", string(EvSessionStart)),
				zap.Error(err),
			)
			continue
		}

		// watchPaths 절대경로 dedup (REQ-HK-007)
		for _, p := range out.WatchPaths {
			abs := toAbsPath(p)
			if _, exists := watchPathsSet[abs]; !exists {
				watchPathsSet[abs] = struct{}{}
				result.WatchPaths = append(result.WatchPaths, p) // 원본 경로 보존
			}
		}

		// initialUserMessage: 마지막 non-empty (REQ-HK-007)
		if out.InitialUserMessage != "" {
			if result.InitialUserMessage != "" {
				initialUserMessageSetCount++
			}
			result.InitialUserMessage = out.InitialUserMessage
			initialUserMessageSetCount++
		}

		if out.AdditionalContext != "" {
			result.AdditionalContext += out.AdditionalContext
		}
	}

	// 여러 핸들러가 initialUserMessage를 설정한 경우 WARN (REQ-HK-007)
	if initialUserMessageSetCount > 1 {
		d.Logger.Warn("multiple handlers set initialUserMessage; using last non-empty value",
			zap.String("event", string(EvSessionStart)),
			zap.Int("setter_count", initialUserMessageSetCount),
		)
	}

	d.emitDispatchLog(EvSessionStart, len(handlers), outcome, time.Since(start))
	return result, nil
}

// DispatchFileChanged는 FileChanged 이벤트를 디스패치한다.
// REQ-HK-008: 내부 핸들러 완료 후 SkillsFileChangedConsumer 호출.
// AC-HK-016: changed == nil이면 ErrInvalidHookInput 반환.
func (d *Dispatcher) DispatchFileChanged(ctx context.Context, changed []string) ([]string, error) {
	if changed == nil {
		return nil, ErrInvalidHookInput
	}

	input := HookInput{
		HookEvent:    EvFileChanged,
		ChangedPaths: changed,
	}

	handlers := d.Registry.Handlers(EvFileChanged, input)
	start := time.Now()
	outcome := "ok"

	for _, h := range handlers {
		in := deepCopyInput(input)
		if _, err := d.invokeHandler(ctx, h, in); err != nil {
			outcome = "handler_error"
			d.Logger.Error("handler_error",
				zap.String("event", string(EvFileChanged)),
				zap.Error(err),
			)
		}
	}

	// SkillsFileChangedConsumer 호출 (REQ-HK-008)
	var activatedSkills []string
	consumer := d.Registry.SkillsConsumer()
	if consumer != nil {
		activatedSkills = consumer(ctx, changed)
	}

	d.emitDispatchLog(EvFileChanged, len(handlers), outcome, time.Since(start))
	return activatedSkills, nil
}

// DispatchPermissionDenied는 PermissionDenied 이벤트를 디스패치한다.
// REQ-HK-009 c/d: useCanUseTool이 Deny로 귀결될 때 자동 호출된다.
// enum 상수 EvPermissionDenied와 1:1 대응 보장.
func (d *Dispatcher) DispatchPermissionDenied(ctx context.Context, result PermissionResult) DispatchResult {
	input := HookInput{
		HookEvent: EvPermissionDenied,
		CustomData: map[string]any{
			"behavior":        result.Behavior.String(),
			"decision_reason": result.DecisionReason,
		},
	}

	handlers := d.Registry.Handlers(EvPermissionDenied, input)
	start := time.Now()
	outcome := "ok"

	for _, h := range handlers {
		in := deepCopyInput(input)
		if _, err := d.invokeHandler(ctx, h, in); err != nil {
			outcome = "handler_error"
			d.Logger.Error("handler_error",
				zap.String("event", string(EvPermissionDenied)),
				zap.Error(err),
			)
		}
	}

	d.emitDispatchLog(EvPermissionDenied, len(handlers), outcome, time.Since(start))
	return DispatchResult{
		HandlerCount: len(handlers),
		Outcome:      outcome,
	}
}

// DispatchGeneric은 이벤트 특화 로직이 없는 이벤트를 처리하는 공통 dispatcher이다.
func (d *Dispatcher) DispatchGeneric(ctx context.Context, event HookEvent, input HookInput) (DispatchResult, error) {
	input.HookEvent = event
	handlers := d.Registry.Handlers(event, input)
	start := time.Now()
	outcome := "ok"

	for _, h := range handlers {
		in := deepCopyInput(input)
		if _, err := d.invokeHandler(ctx, h, in); err != nil {
			outcome = "handler_error"
			d.Logger.Error("handler_error",
				zap.String("event", string(event)),
				zap.Error(err),
			)
		}
	}

	d.emitDispatchLog(event, len(handlers), outcome, time.Since(start))
	return DispatchResult{HandlerCount: len(handlers), Outcome: outcome}, nil
}

// invokeHandler는 핸들러를 호출하고 결과를 반환한다.
// async 감지는 호출자 책임.
func (d *Dispatcher) invokeHandler(ctx context.Context, h HookHandler, input HookInput) (HookJSONOutput, error) {
	return h.Handle(ctx, input)
}

// emitDispatchLog는 REQ-HK-004의 구조화 로그를 emit한다.
// 성공: INFO, handler_error 포함: ERROR.
func (d *Dispatcher) emitDispatchLog(event HookEvent, handlerCount int, outcome string, duration time.Duration) {
	durationMS := duration.Milliseconds()
	fields := []zap.Field{
		zap.String("event", string(event)),
		zap.Int("handler_count", handlerCount),
		zap.String("outcome", outcome),
		zap.Int64("duration_ms", durationMS),
	}
	if outcome == "handler_error" || outcome == "timeout" {
		d.Logger.Error("dispatch completed with error", fields...)
	} else {
		d.Logger.Info("dispatch completed", fields...)
	}
}

// validateInput은 HookInput의 기본 스키마 검증을 수행한다.
// REQ-HK-015 / AC-HK-016
func validateInput(input HookInput) error {
	switch input.HookEvent {
	case EvFileChanged:
		if input.ChangedPaths == nil {
			return ErrInvalidHookInput
		}
	}
	return nil
}

// deepCopyInput은 HookInput의 deep copy를 반환한다.
// REQ-HK-014: 각 핸들러에 독립적인 deep copy 전달.
func deepCopyInput(input HookInput) HookInput {
	cp := input

	// Tool deep copy
	if input.Tool != nil {
		toolCopy := *input.Tool
		if input.Tool.Input != nil {
			toolCopy.Input = make(map[string]any, len(input.Tool.Input))
			for k, v := range input.Tool.Input {
				toolCopy.Input[k] = v
			}
		}
		cp.Tool = &toolCopy
	}

	// Input map deep copy
	if input.Input != nil {
		cp.Input = make(map[string]any, len(input.Input))
		for k, v := range input.Input {
			cp.Input[k] = v
		}
	}

	// CustomData deep copy
	if input.CustomData != nil {
		cp.CustomData = make(map[string]any, len(input.CustomData))
		for k, v := range input.CustomData {
			cp.CustomData[k] = v
		}
	}

	// ChangedPaths deep copy
	if input.ChangedPaths != nil {
		cp.ChangedPaths = make([]string, len(input.ChangedPaths))
		copy(cp.ChangedPaths, input.ChangedPaths)
	}

	return cp
}

// toAbsPath는 경로를 절대경로로 변환한다 (REQ-HK-007 watchPaths dedup).
// 이미 절대경로이면 그대로 반환.
func toAbsPath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}
