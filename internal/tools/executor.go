package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/modu-ai/goose/internal/permissions"
	"github.com/modu-ai/goose/internal/tools/permission"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.uber.org/zap"
)

// ExecRequest는 Executor.Run의 요청 타입이다.
type ExecRequest struct {
	// ToolName은 실행할 tool 이름이다.
	ToolName string
	// Input은 tool 입력 JSON이다.
	Input json.RawMessage
	// ToolUseID는 LLM 응답의 tool_use 블록 ID이다.
	ToolUseID string
	// PermissionCtx는 CanUseTool.Check에 전달할 컨텍스트이다.
	PermissionCtx permissions.ToolPermissionContext
}

// Executor는 Registry + Preflight + Call dispatch를 조율한다.
//
// @MX:ANCHOR: [AUTO] QUERY-001이 tool 실행을 위임하는 단일 진입점
// @MX:REASON: SPEC-GOOSE-TOOLS-001 REQ-TOOLS-006 - schema → preapproval → canUseTool → call 순서 보장. fan_in >= 3
type Executor struct {
	registry       *Registry
	matcher        permission.Matcher
	canUseTool     permissions.CanUseTool
	permCfg        permission.Config
	logger         *zap.Logger
	logInvocations bool
}

// ExecutorConfig는 Executor 설정이다.
type ExecutorConfig struct {
	// Registry는 tool 저장소이다.
	Registry *Registry
	// Matcher는 permission pre-approval 매처이다.
	Matcher permission.Matcher
	// CanUseTool은 tool 실행 권한 게이트이다.
	CanUseTool permissions.CanUseTool
	// PermConfig는 permission 설정이다.
	PermConfig permission.Config
	// Logger는 zap 로거이다.
	Logger *zap.Logger
	// LogInvocations는 REQ-TOOLS-020 구조화 로그 활성화 여부이다.
	LogInvocations bool
}

// NewExecutor는 새 Executor를 생성한다.
func NewExecutor(cfg ExecutorConfig) *Executor {
	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	matcher := cfg.Matcher
	if matcher == nil {
		matcher = &permission.GlobMatcher{}
	}
	return &Executor{
		registry:       cfg.Registry,
		matcher:        matcher,
		canUseTool:     cfg.CanUseTool,
		permCfg:        cfg.PermConfig,
		logger:         logger,
		logInvocations: cfg.LogInvocations,
	}
}

// Run은 tool을 실행하고 결과를 반환한다.
// REQ-TOOLS-006: schema → preapproval → canUseTool → call 순서.
func (e *Executor) Run(ctx context.Context, req ExecRequest) ToolResult {
	start := time.Now()

	// REQ-TOOLS-011: Draining 상태 확인
	if e.registry.IsDraining() {
		return ToolResult{IsError: true, Content: []byte("registry draining")}
	}

	// Step 1: Registry.Resolve
	tool, ok := e.registry.Resolve(req.ToolName)
	if !ok {
		result := ToolResult{IsError: true, Content: []byte("tool_not_found: " + req.ToolName)}
		e.logInvocation(req.ToolName, "error", time.Since(start), len(req.Input), 0)
		return result
	}

	// Step 2: JSON Schema validation (REQ-TOOLS-014)
	if err := e.validateInput(req.ToolName, req.Input); err != nil {
		result := ToolResult{IsError: true, Content: []byte("schema_validation_failed: " + err.Error())}
		e.logInvocation(req.ToolName, "error", time.Since(start), len(req.Input), len(result.Content))
		return result
	}

	// Step 3: PermissionMatcher.Preapproved (REQ-TOOLS-018)
	if approved, reason := e.matcher.Preapproved(req.ToolName, req.Input, e.permCfg); approved {
		result, _ := tool.Call(ctx, req.Input)
		e.logInvocation(req.ToolName, "preapproved", time.Since(start), len(req.Input), len(result.Content))
		_ = reason
		return result
	}

	// Step 4: CanUseTool gate (REQ-QUERY-006)
	if e.canUseTool != nil {
		decision := e.canUseTool.Check(ctx, req.PermissionCtx)
		if decision.Behavior == permissions.Deny {
			result := ToolResult{IsError: true, Content: []byte("denied: " + decision.Reason)}
			e.logInvocation(req.ToolName, "deny", time.Since(start), len(req.Input), len(result.Content))
			return result
		}
		if decision.Behavior == permissions.Ask {
			// Ask: HOOK-001/CLI-001이 처리. 지금은 deny와 동일하게 처리.
			result := ToolResult{IsError: true, Content: []byte("permission_required: " + decision.Reason)}
			e.logInvocation(req.ToolName, "deny", time.Since(start), len(req.Input), len(result.Content))
			return result
		}
	}

	// Step 5: Tool.Call
	result, err := tool.Call(ctx, req.Input)
	if err != nil {
		result = ToolResult{IsError: true, Content: []byte(fmt.Sprintf("tool_error: %v", err))}
		e.logInvocation(req.ToolName, "error", time.Since(start), len(req.Input), len(result.Content))
		return result
	}

	outcome := "allow"
	if result.IsError {
		outcome = "error"
	}
	e.logInvocation(req.ToolName, outcome, time.Since(start), len(req.Input), len(result.Content))
	return result
}

// validateInput은 req.Input을 tool의 JSON Schema에 대해 검증한다.
// REQ-TOOLS-014
func (e *Executor) validateInput(toolName string, input json.RawMessage) error {
	entry, ok := e.registry.ResolveEntry(toolName)
	if !ok {
		return nil // 이미 Resolve에서 not found 처리됨
	}
	if entry.compiled == nil {
		return nil
	}

	// JSON 파싱
	var v any
	if len(input) > 0 {
		if err := json.Unmarshal(input, &v); err != nil {
			return fmt.Errorf("invalid JSON: %v", err)
		}
	} else {
		v = map[string]any{}
	}

	// Schema 검증
	if err := entry.compiled.Validate(v); err != nil {
		if ve, ok := err.(*jsonschema.ValidationError); ok {
			return fmt.Errorf("%v", ve.Error())
		}
		return err
	}
	return nil
}

// logInvocation은 REQ-TOOLS-020 구조화 로그를 출력한다.
func (e *Executor) logInvocation(tool, outcome string, duration time.Duration, inputSize, outputSize int) {
	if !e.logInvocations {
		return
	}
	e.logger.Info("tool invocation",
		zap.String("tool", tool),
		zap.String("outcome", outcome),
		zap.Int64("duration_ms", duration.Milliseconds()),
		zap.Int("input_size", inputSize),
		zap.Int("output_size", outputSize),
	)
}
