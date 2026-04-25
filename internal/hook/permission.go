package hook

import (
	"context"
	"os"
	"sync"

	"go.uber.org/zap"
)

// permissionState는 useCanUseTool 호출 시 사용하는 YOLO classifier 상태이다.
// REQ-HK-012: SetYoloClassifierApproval 후 Clear까지 유지.
type permissionState struct {
	mu           sync.RWMutex
	yoloPatterns map[string]struct{}
}

// DefaultPermissionQueue는 인메모리 PermissionQueueOps 구현체이다.
// 단순 로깅 기반 구현; 실제 세션 상태는 세션 종료 시 reset.
type DefaultPermissionQueue struct {
	mu     sync.RWMutex
	yolo   map[string]struct{}
	logger *zap.Logger
}

// NewDefaultPermissionQueue는 DefaultPermissionQueue를 생성한다.
func NewDefaultPermissionQueue(logger *zap.Logger) *DefaultPermissionQueue {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DefaultPermissionQueue{
		yolo:   make(map[string]struct{}),
		logger: logger,
	}
}

// SetYoloClassifierApproval은 toolPattern에 대한 YOLO 자동 승인을 설정한다.
// REQ-HK-012
func (q *DefaultPermissionQueue) SetYoloClassifierApproval(toolPattern string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.yolo[toolPattern] = struct{}{}
}

// ClearYoloApprovals는 모든 YOLO 승인을 초기화한다.
func (q *DefaultPermissionQueue) ClearYoloApprovals() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.yolo = make(map[string]struct{})
}

// IsYoloApproved는 toolName이 YOLO 자동 승인 패턴에 매치되는지 반환한다.
// REQ-HK-012
func (q *DefaultPermissionQueue) IsYoloApproved(toolName string) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	for pattern := range q.yolo {
		if matcherMatches(pattern, toolName) {
			return true
		}
	}
	return false
}

// RecordAutoModeDenial은 자동 모드 denial을 기록한다.
// REQ-HK-009 c
func (q *DefaultPermissionQueue) RecordAutoModeDenial(toolName string, reason string) {
	q.logger.Info("auto_mode_denial",
		zap.String("tool_name", toolName),
		zap.String("reason", reason),
	)
}

// LogPermissionDecision은 권한 결정을 로그에 기록한다.
func (q *DefaultPermissionQueue) LogPermissionDecision(result PermissionResult, toolName string) {
	dr := ""
	if result.DecisionReason != nil {
		dr = result.DecisionReason.Reason
	}
	q.logger.Info("permission_decision",
		zap.String("tool_name", toolName),
		zap.String("behavior", result.Behavior.String()),
		zap.String("decision_reason", dr),
	)
}

// UseCanUseTool은 tool 사용 권한을 결정하는 워크플로이다.
// REQ-HK-009: YOLO → PermissionRequest → Role-based routing
// REQ-HK-012: YOLO auto-approve
// REQ-HK-017: non-TTY safe default = Deny
//
// @MX:ANCHOR: [AUTO] tool 권한 결정의 단일 진입점
// @MX:REASON: SPEC-GOOSE-HOOK-001 REQ-HK-009 — fan_in >= 3 (dispatcher, query, tests)
func (d *Dispatcher) UseCanUseTool(
	ctx context.Context,
	toolName string,
	input map[string]any,
	permCtx PermissionContext,
) PermissionResult {
	queue, _ := d.PermQueue.(*DefaultPermissionQueue)

	// (a) YOLO classifier — auto-approve? (REQ-HK-012)
	if queue != nil && queue.IsYoloApproved(toolName) {
		result := PermissionResult{
			Behavior: PermAllow,
			DecisionReason: &DecisionReason{
				Type:   "yolo_auto",
				Reason: "yolo classifier approved",
			},
		}
		if d.PermQueue != nil {
			d.PermQueue.LogPermissionDecision(result, toolName)
		}
		return result
	}

	// (b) PermissionRequest 훅 디스패치 (REQ-HK-009 b)
	permInput := HookInput{
		HookEvent: EvPermissionRequest,
		Input:     input,
		CustomData: map[string]any{
			"tool_name": toolName,
		},
	}
	permHandlers := d.Registry.Handlers(EvPermissionRequest, permInput)

	for _, h := range permHandlers {
		in := deepCopyInput(permInput)
		out, err := h.Handle(ctx, in)
		if err != nil {
			d.Logger.Error("permission_request_handler_error",
				zap.String("tool_name", toolName),
				zap.Error(err),
			)
			continue
		}

		// handler deny short-circuit (REQ-HK-009 c)
		if out.PermissionDecision != nil && !out.PermissionDecision.Approve {
			result := PermissionResult{
				Behavior: PermDeny,
				DecisionReason: &DecisionReason{
					Type:   "handler",
					Reason: out.PermissionDecision.Reason,
				},
			}
			// RecordAutoModeDenial (AC-HK-008)
			if d.PermQueue != nil {
				d.PermQueue.RecordAutoModeDenial(toolName, out.PermissionDecision.Reason)
			}
			// DispatchPermissionDenied 정확히 1회 (REQ-HK-009 c/d)
			d.DispatchPermissionDenied(ctx, result)
			if d.PermQueue != nil {
				d.PermQueue.LogPermissionDecision(result, toolName)
			}
			return result
		}
	}

	// (c) Role-based routing (REQ-HK-009 e)
	role := permCtx.Role
	if role == "" {
		role = RoleNonTTY
	}

	// 알 수 없는 role은 RoleInteractive로 fallback하되 WARN 로그 (§6.2)
	switch role {
	case RoleCoordinator, RoleSwarmWorker, RoleInteractive, RoleNonTTY:
		// valid
	default:
		d.Logger.Warn("unknown_role fallback to interactive",
			zap.String("role", string(role)),
		)
		role = RoleInteractive
	}

	switch role {
	case RoleCoordinator:
		if d.Coordinator != nil {
			res, err := d.Coordinator.RequestPermission(ctx, toolName, input)
			if err == nil {
				if res.Behavior == PermDeny {
					d.DispatchPermissionDenied(ctx, res)
				}
				if d.PermQueue != nil {
					d.PermQueue.LogPermissionDecision(res, toolName)
				}
				return res
			}
		}

	case RoleSwarmWorker:
		if d.SwarmWorker != nil {
			res, err := d.SwarmWorker.BubbleUpPermission(ctx, toolName, input)
			if err == nil {
				if res.Behavior == PermDeny {
					d.DispatchPermissionDenied(ctx, res)
				}
				if d.PermQueue != nil {
					d.PermQueue.LogPermissionDecision(res, toolName)
				}
				return res
			}
		}

	case RoleInteractive:
		// TTY 확인 (REQ-HK-017)
		if d.isTTY() && d.Interactive != nil {
			res, err := d.Interactive.PromptUser(ctx, toolName, input)
			if err == nil {
				if res.Behavior == PermDeny {
					d.DispatchPermissionDenied(ctx, res)
				}
				if d.PermQueue != nil {
					d.PermQueue.LogPermissionDecision(res, toolName)
				}
				return res
			}
		}
		// TTY가 아니거나 Interactive 핸들러 없음 → Deny (REQ-HK-017)
		fallthrough

	case RoleNonTTY:
		// non-TTY safe default = Deny (REQ-HK-017)
		result := PermissionResult{
			Behavior: PermDeny,
			DecisionReason: &DecisionReason{
				Type:   "no_interactive_fallback",
				Reason: "no_interactive_fallback",
			},
		}
		d.DispatchPermissionDenied(ctx, result)
		if d.PermQueue != nil {
			d.PermQueue.LogPermissionDecision(result, toolName)
		}
		return result
	}

	// 모든 핸들러가 abstain — non-TTY safe default (REQ-HK-017)
	result := PermissionResult{
		Behavior: PermDeny,
		DecisionReason: &DecisionReason{
			Type:   "no_interactive_fallback",
			Reason: "no_interactive_fallback",
		},
	}
	d.DispatchPermissionDenied(ctx, result)
	if d.PermQueue != nil {
		d.PermQueue.LogPermissionDecision(result, toolName)
	}
	return result
}

// isTTY는 stdout이 TTY인지 반환한다.
// REQ-HK-017 / §6.11.6: GOOSE_HOOK_NON_INTERACTIVE=1이면 false 반환.
func (d *Dispatcher) isTTY() bool {
	// §6.11.6: GOOSE_HOOK_NON_INTERACTIVE=1이면 non-TTY 강제
	if os.Getenv("GOOSE_HOOK_NON_INTERACTIVE") == "1" {
		return false
	}
	if d.IsTTY != nil {
		return d.IsTTY()
	}
	return false // safe default
}
