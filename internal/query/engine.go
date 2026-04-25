// Package query는 QueryEngine과 관련 타입을 포함한다.
// SPEC-GOOSE-QUERY-001 S0 → S3+에서 구현된다.
package query

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/query/loop"
	"go.uber.org/zap"
)

// ErrUnknownPermissionRequest는 알 수 없는 toolUseID로 ResolvePermission 호출 시 반환된다.
// REQ-QUERY-013: silent drop 금지.
var ErrUnknownPermissionRequest = errors.New("unknown permission request: toolUseID not pending")

// QueryEngine는 단일 대화 세션 = 단일 인스턴스의 agentic core 런타임이다.
// REQ-QUERY-001: 인스턴스 하나가 대화 세션 하나에 1:1 대응한다.
//
// @MX:ANCHOR: [AUTO] 모든 상위 레이어의 단일 진입점 (CLI, TRANSPORT, SUBAGENT)
// @MX:REASON: REQ-QUERY-001 - 대화 세션과 1:1 대응. fan_in >= 3 (CLI, test, future transport)
type QueryEngine struct {
	// cfg는 엔진 설정이다.
	cfg QueryEngineConfig
	// mu는 SubmitMessage 직렬화를 위한 뮤텍스이다.
	// REQ-QUERY-001: 동일 인스턴스에서 동시 SubmitMessage 호출 방지.
	mu sync.Mutex
	// stateMu는 e.state 읽기/쓰기를 보호하는 전용 뮤텍스이다.
	// S5: loop goroutine의 OnComplete와 SubmitMessage 간 race 방지.
	// mu와 별도로 유지하여 loop goroutine이 mu 없이 state를 갱신할 수 있도록 한다.
	stateMu sync.Mutex
	// state는 SubmitMessage 호출 간 누적되는 대화 상태이다.
	// REQ-QUERY-015: loop goroutine만 직접 변경. 외부는 stateMu로 보호.
	state loop.State
	// permInbox는 Ask permission 결정을 loop에 전달하는 채널이다.
	// REQ-QUERY-013: 외부에서 ResolvePermission을 통해 전송.
	//
	// @MX:WARN: [AUTO] 여러 goroutine에서 send, loop만 receive
	// @MX:REASON: REQ-QUERY-013 - Ask 분기 재개 단일 경로. cap 4 buffering으로 backpressure 방지
	permInbox chan loop.PermissionDecision
	// pendingPermsMu는 pendingPerms 맵 보호용 뮤텍스이다.
	pendingPermsMu sync.Mutex
	// pendingPerms는 현재 loop에서 Ask 대기 중인 toolUseID 집합이다.
	// REQ-QUERY-013: ResolvePermission에서 unknown ID silent drop 방지를 위해 추적한다.
	// loop goroutine이 등록/해제, ResolvePermission이 조회한다.
	//
	// @MX:WARN: [AUTO] pendingPerms는 loop goroutine(등록) + 외부 goroutine(조회) 공유 상태
	// @MX:REASON: REQ-QUERY-013 - unknown ID 감지. pendingPermsMu로 보호 필수
	pendingPerms map[string]struct{}
}

// New는 QueryEngineConfig로 새 QueryEngine을 생성한다.
// REQ-QUERY-001: 유효성 검증 실패 시 에러 반환.
func New(cfg QueryEngineConfig) (*QueryEngine, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid QueryEngineConfig: %w", err)
	}
	return &QueryEngine{
		cfg: cfg,
		// REQ-QUERY-013: cap 4로 buffering — 동일 turn 내 최대 4개의 Ask tool까지 backpressure 없이 처리.
		permInbox:    make(chan loop.PermissionDecision, 4),
		pendingPerms: make(map[string]struct{}),
		state: loop.State{
			TaskBudgetRemaining: cfg.TaskBudget.Remaining,
		},
	}, nil
}

// SubmitMessage는 사용자 메시지를 제출하고 SDKMessage 스트림 채널을 반환한다.
// REQ-QUERY-005: 10ms 이내 반환 필수.
// REQ-QUERY-016: LLM 연결은 goroutine 내부에서 수행.
//
// @MX:ANCHOR: [AUTO] 모든 상위 레이어의 단일 스트리밍 진입점
// @MX:REASON: REQ-QUERY-001/005 - 10ms 마감 + 세션 1:1 대응. fan_in >= 3
// @MX:WARN: [AUTO] goroutine spawn 지점 - State 단독 소유
// @MX:REASON: REQ-QUERY-015 - spawned goroutine이 State를 단독 소유. 외부 mutation 금지
func (e *QueryEngine) SubmitMessage(ctx context.Context, prompt string) (<-chan message.SDKMessage, error) {
	// REQ-QUERY-004: SubmitMessage 호출을 직렬화하여 동시 실행 방지.
	e.mu.Lock()
	defer e.mu.Unlock()

	// unbuffered 채널: REQ-QUERY-002 - 송신 불가(receive-only) 계약
	out := make(chan message.SDKMessage)

	// S5: user message를 state에 포함시킨 스냅샷을 loop에 전달한다.
	// REQ-QUERY-004: messages 누적 보존. loop 완료 후 OnComplete로 갱신됨.
	userMsg := message.Message{
		Role: "user",
		Content: []message.ContentBlock{
			{Type: "text", Text: prompt},
		},
	}
	e.stateMu.Lock()
	currentState := e.state
	e.stateMu.Unlock()
	currentState.Messages = append(append([]message.Message(nil), currentState.Messages...), userMsg)

	// LLM 호출 클로저: messages + tools를 바인딩하여 loop에 전달한다.
	// REQ-QUERY-016: LLM 연결(dial) 비용은 goroutine 내부에서 처리한다.
	// currentState.Messages는 이미 user message를 포함한 스냅샷이다.
	callLLM := e.buildLLMStreamFuncFromMsgs(currentState.Messages)

	// S9: TeammateIdentity meta 주입 채널 분리
	// REQ-QUERY-020: 모든 SDKMessage.Meta에 teammate identity 주입.
	// loopOut = loop가 쓰는 채널, returnCh = caller에게 반환하는 채널.
	// TeammateIdentity가 nil이면 동일 채널 사용.
	//
	// @MX:NOTE: [AUTO] TeammateIdentity Meta 주입 래핑 goroutine - loop out → meta inject → returnCh.
	loopOut := out
	returnCh := (<-chan message.SDKMessage)(out)
	teammateMeta := e.buildTeammateMeta()
	if teammateMeta != nil {
		loopOut = make(chan message.SDKMessage)
		wrappedReturnCh := make(chan message.SDKMessage)
		go func() {
			defer close(wrappedReturnCh)
			for msg := range loopOut {
				if msg.Meta == nil {
					msg.Meta = make(map[string]any)
				}
				for k, v := range teammateMeta {
					msg.Meta[k] = v
				}
				wrappedReturnCh <- msg
			}
		}()
		returnCh = wrappedReturnCh
	}

	cfg := loop.LoopConfig{
		Out:          loopOut,
		InitialState: currentState,
		Prompt:       prompt,
		MaxTurns:     e.cfg.MaxTurns,
		PermInbox:    e.permInbox,
		CallLLM:      callLLM,
		// S4: tool roundtrip 의존성 주입
		CallLLMFactory: e.buildLLMStreamFuncFactory(),
		CanUseTool:     e.cfg.CanUseTool,
		Execute:        e.cfg.Executor.Run,
		ToolResultCap:  e.cfg.TaskBudget.ToolResultCap,
		// S5: loop 종료 후 최종 State를 engine.state에 반영한다.
		// REQ-QUERY-004: SubmitMessage 호출 간 messages/turnCount/budget 누적 보존.
		//
		// @MX:NOTE: [AUTO] OnComplete는 loop goroutine의 defer 체인에서 호출된다.
		// stateMu로 e.state 쓰기를 보호하여 SubmitMessage의 e.state 읽기와의 경합을 방지한다.
		OnComplete: func(finalState loop.State) {
			e.stateMu.Lock()
			e.state = finalState
			e.stateMu.Unlock()
		},
		// S6: Ask permission pending 등록/해제 콜백 — ResolvePermission의 unknown ID 감지에 사용.
		// REQ-QUERY-013: loop goroutine이 Ask 분기 진입/해제 시 engine에 알린다.
		OnAskPending:  e.registerPendingPerm,
		OnAskResolved: e.unregisterPendingPerm,
		// S7: Compactor 주입 — import cycle 방지를 위해 함수 타입으로 래핑.
		// cfg.Compactor가 nil이면 compaction 비활성.
		ShouldCompact: e.buildShouldCompactFunc(),
		Compact:       e.buildCompactFunc(),
		// S9: PostSamplingHooks 주입 — import cycle 방지를 위해 loop.PostSamplingHookFunc로 변환.
		// REQ-QUERY-018: FIFO 순 적용.
		PostSamplingHooks: e.buildPostSamplingHooks(),
	}

	// loop.Run은 goroutine을 spawn하고 즉시 반환한다.
	// loopOut 채널 close 책임은 queryLoop 단독.
	loop.Run(ctx, cfg)

	return returnCh, nil
}

// buildSystemHeader는 TeammateIdentity가 nil이 아닌 경우 system header map을 반환한다.
// REQ-QUERY-020: nil이면 LLM payload에 주입하지 않는다.
func (e *QueryEngine) buildSystemHeader() map[string]any {
	if e.cfg.TeammateIdentity == nil {
		return nil
	}
	return map[string]any{
		"agent_id":  e.cfg.TeammateIdentity.AgentID,
		"team_name": e.cfg.TeammateIdentity.TeamName,
	}
}

// buildTeammateMeta는 TeammateIdentity가 nil이 아닌 경우 SDKMessage Meta에 주입할 맵을 반환한다.
// REQ-QUERY-020: nil이면 Meta 주입 없음.
func (e *QueryEngine) buildTeammateMeta() map[string]any {
	if e.cfg.TeammateIdentity == nil {
		return nil
	}
	return map[string]any{
		"agent_id":  e.cfg.TeammateIdentity.AgentID,
		"team_name": e.cfg.TeammateIdentity.TeamName,
	}
}

// buildLLMStreamFuncFromMsgs는 주어진 messages 슬라이스를 사용하는 LLMStreamFunc를 생성한다.
// REQ-QUERY-016: LLM 연결 비용이 이 클로저 호출 시점(goroutine 내부)에서 발생한다.
// S5: currentState.Messages (user message 포함)를 직접 전달하여 e.state 의존을 제거한다.
// S9: FallbackModels 지원 — primary 실패 시 fallback 모델로 순차 재시도.
//
// @MX:NOTE: [AUTO] FallbackModels chain 진입점 - primary LLMCall 실패 시 FallbackModels 순회.
// @MX:REASON: REQ-QUERY-019 - ROUTER-001 책임이 아닌 engine level fallback.
func (e *QueryEngine) buildLLMStreamFuncFromMsgs(msgs []message.Message) loop.LLMStreamFunc {
	llmCall := e.cfg.LLMCall
	fallbackModels := e.cfg.FallbackModels
	systemHeader := e.buildSystemHeader()
	logger := e.cfg.Logger

	return func(ctx context.Context) (<-chan message.StreamEvent, error) {
		req := LLMCallReq{
			Messages:     msgs,
			SystemHeader: systemHeader,
		}
		ch, err := llmCall(ctx, req)
		if err == nil {
			return ch, nil
		}

		// primary 실패: fallback 모델 순차 시도
		// @MX:WARN: [AUTO] fallback 순회 시 각 모델 실패는 다음 모델로 진행 — 최종 실패만 반환.
		// @MX:REASON: REQ-QUERY-019 - FallbackModels 소진 시 마지막 에러 surface.
		for i, model := range fallbackModels {
			fallbackReq := LLMCallReq{
				Messages:     msgs,
				SystemHeader: systemHeader,
			}
			fallbackReq.Route.Model = model
			fCh, fErr := llmCall(ctx, fallbackReq)
			if fErr == nil {
				logger.Info("fallback used",
					zap.String("fallback_model", model),
					zap.Int("fallback_index", i),
					zap.Error(err),
				)
				return fCh, nil
			}
			err = fErr
		}
		return nil, err
	}
}

// buildLLMStreamFuncFactory는 messages를 갱신하여 새 LLMStreamFunc를 생성하는 factory를 반환한다.
// S4 after_tool_results continue site에서 tool_result를 포함한 다음 LLM 호출 클로저를 만들 때 사용된다.
// S9: FallbackModels 및 TeammateIdentity systemHeader 주입 포함.
func (e *QueryEngine) buildLLMStreamFuncFactory() func(msgs []message.Message) loop.LLMStreamFunc {
	llmCall := e.cfg.LLMCall
	fallbackModels := e.cfg.FallbackModels
	systemHeader := e.buildSystemHeader()
	logger := e.cfg.Logger

	return func(msgs []message.Message) loop.LLMStreamFunc {
		return func(ctx context.Context) (<-chan message.StreamEvent, error) {
			req := LLMCallReq{
				Messages:     msgs,
				SystemHeader: systemHeader,
			}
			ch, err := llmCall(ctx, req)
			if err == nil {
				return ch, nil
			}
			for i, model := range fallbackModels {
				fallbackReq := LLMCallReq{
					Messages:     msgs,
					SystemHeader: systemHeader,
				}
				fallbackReq.Route.Model = model
				fCh, fErr := llmCall(ctx, fallbackReq)
				if fErr == nil {
					logger.Info("fallback used",
						zap.String("fallback_model", model),
						zap.Int("fallback_index", i),
						zap.Error(err),
					)
					return fCh, nil
				}
				err = fErr
			}
			return nil, err
		}
	}
}

// ResolvePermission은 Ask permission 대기 중인 loop에 결정을 전달한다.
// REQ-QUERY-013: 외부 결정을 permInbox 채널을 통해 loop에 전달.
// unknown toolUseID 전달 시 ErrUnknownPermissionRequest 반환 (silent drop 금지).
//
// @MX:ANCHOR: [AUTO] Ask permission 외부 결정 단일 진입점
// @MX:REASON: REQ-QUERY-013 - fan_in >= 3 (CLI, test, future SDK client). unknown ID 감지 필수
func (e *QueryEngine) ResolvePermission(toolUseID string, behavior int, reason string) error {
	e.pendingPermsMu.Lock()
	_, ok := e.pendingPerms[toolUseID]
	e.pendingPermsMu.Unlock()

	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownPermissionRequest, toolUseID)
	}

	decision := loop.PermissionDecision{
		ToolUseID: toolUseID,
		Behavior:  behavior,
		Reason:    reason,
	}

	// permInbox에 non-blocking 전송 시도.
	// 채널이 가득 찬 경우(cap 4 초과)는 설계상 발생하지 않아야 하나,
	// ctx 취소로 loop가 이미 종료된 경우에도 pending은 이미 해제되어 이 분기에 도달하지 않는다.
	select {
	case e.permInbox <- decision:
		return nil
	default:
		// permInbox 포화: 이미 종료된 loop에 전송 시도하는 비정상 상황
		return fmt.Errorf("%w: permInbox full, loop may have terminated: %s", ErrUnknownPermissionRequest, toolUseID)
	}
}

// buildShouldCompactFunc는 cfg.Compactor.ShouldCompact를 loop 패키지용 함수 타입으로 래핑한다.
// cfg.Compactor가 nil이면 nil을 반환한다 (compaction 비활성).
func (e *QueryEngine) buildShouldCompactFunc() func(loop.State) bool {
	if e.cfg.Compactor == nil {
		return nil
	}
	c := e.cfg.Compactor
	return func(state loop.State) bool {
		return c.ShouldCompact(state)
	}
}

// buildCompactFunc는 cfg.Compactor.Compact를 loop 패키지용 함수 타입으로 래핑한다.
// cfg.Compactor가 nil이면 nil을 반환한다.
// CompactBoundary → message.PayloadCompactBoundary로 변환하여 import cycle 없이 전달한다.
func (e *QueryEngine) buildCompactFunc() func(loop.State) (loop.State, message.PayloadCompactBoundary, error) {
	if e.cfg.Compactor == nil {
		return nil
	}
	c := e.cfg.Compactor
	return func(state loop.State) (loop.State, message.PayloadCompactBoundary, error) {
		newState, boundary, err := c.Compact(state)
		if err != nil {
			return state, message.PayloadCompactBoundary{}, err
		}
		return newState, message.PayloadCompactBoundary{
			Turn:           boundary.Turn,
			MessagesBefore: boundary.MessagesBefore,
			MessagesAfter:  boundary.MessagesAfter,
		}, nil
	}
}

// registerPendingPerm은 loop goroutine이 Ask 분기 진입 시 호출한다.
// pendingPerms에 toolUseID를 등록한다.
func (e *QueryEngine) registerPendingPerm(toolUseID string) {
	e.pendingPermsMu.Lock()
	e.pendingPerms[toolUseID] = struct{}{}
	e.pendingPermsMu.Unlock()
}

// unregisterPendingPerm은 loop goroutine이 Ask 분기 해결 후 호출한다.
// pendingPerms에서 toolUseID를 제거한다.
func (e *QueryEngine) unregisterPendingPerm(toolUseID string) {
	e.pendingPermsMu.Lock()
	delete(e.pendingPerms, toolUseID)
	e.pendingPermsMu.Unlock()
}

// buildPostSamplingHooks는 cfg.PostSamplingHooks를 loop.PostSamplingHookFunc 슬라이스로 변환한다.
// REQ-QUERY-018: import cycle 방지를 위해 타입 변환만 수행.
func (e *QueryEngine) buildPostSamplingHooks() []loop.PostSamplingHookFunc {
	if len(e.cfg.PostSamplingHooks) == 0 {
		return nil
	}
	hooks := make([]loop.PostSamplingHookFunc, len(e.cfg.PostSamplingHooks))
	for i, h := range e.cfg.PostSamplingHooks {
		h := h // loop variable capture
		hooks[i] = loop.PostSamplingHookFunc(h)
	}
	return hooks
}

// validateConfig는 QueryEngineConfig 필수 필드를 검증한다.
func validateConfig(cfg QueryEngineConfig) error {
	if cfg.LLMCall == nil {
		return fmt.Errorf("field LLMCall is required")
	}
	if cfg.CanUseTool == nil {
		return fmt.Errorf("field CanUseTool is required")
	}
	if cfg.Executor == nil {
		return fmt.Errorf("field Executor is required")
	}
	if cfg.Logger == nil {
		return fmt.Errorf("field Logger is required")
	}
	return nil
}
