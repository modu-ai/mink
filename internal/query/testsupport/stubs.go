//go:build integration

// Package testsupport는 SPEC-GOOSE-QUERY-001 통합 테스트용 스텁 구현을 제공한다.
// 모든 스텁은 인터페이스 계약을 충족하되 실제 외부 호출을 수행하지 않는다.
// SPEC-GOOSE-QUERY-001 S0 T0.4 (research.md §8.3 규격)
package testsupport

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/permissions"
	"github.com/modu-ai/goose/internal/query"
	"github.com/modu-ai/goose/internal/query/loop"
)

// --- StubLLMCall ---

// StubLLMResponse는 StubLLMCall이 반환할 단일 응답 시퀀스이다.
type StubLLMResponse struct {
	// Events는 스트림으로 전달할 이벤트 목록이다.
	Events []message.StreamEvent
	// Err는 응답 채널 반환 시 발생할 에러이다 (nil이면 정상).
	Err error
	// UsageInputTokens는 이 응답이 소비한 입력 토큰 수이다.
	// REQ-QUERY-011: budget 차감 시뮬레이션용. 0이면 차감 없음.
	UsageInputTokens int
	// UsageOutputTokens는 이 응답이 소비한 출력 토큰 수이다.
	UsageOutputTokens int
}

// StubLLMCall은 LLMCallFunc 타입의 스텁 구현이다.
// 미리 등록한 응답 시퀀스를 순서대로 반환한다.
type StubLLMCall struct {
	// responses는 호출 순서대로 반환할 응답 목록이다.
	responses []StubLLMResponse
	// callCount는 호출 횟수를 추적한다.
	callCount atomic.Int64
	// mu는 responses 슬라이스 보호용이다.
	mu sync.Mutex
	// InitialDelay는 goroutine 내부에서 지연할 시간이다 (0이면 지연 없음).
	// AC-QUERY-014: SubmitMessage 10ms 마감 테스트용.
	InitialDelay int64 // time.Duration as int64 nanoseconds
	// RecordedRequests는 수신된 LLMCallReq 목록이다 (payload recorder 모드).
	RecordedRequests []query.LLMCallReq
	// recordMu는 RecordedRequests 보호용이다.
	recordMu sync.Mutex
}

// NewStubLLMCall은 지정한 응답 시퀀스를 반환하는 StubLLMCall을 생성한다.
func NewStubLLMCall(responses ...StubLLMResponse) *StubLLMCall {
	return &StubLLMCall{responses: responses}
}

// NewStubLLMCallSimple은 단일 assistant text delta와 stop으로 응답하는 스텁을 생성한다.
// AC-QUERY-001 기본 시나리오용.
func NewStubLLMCallSimple(delta string) *StubLLMCall {
	return &StubLLMCall{
		responses: []StubLLMResponse{
			{
				Events: []message.StreamEvent{
					{Type: message.TypeTextDelta, Delta: delta},
					{Type: message.TypeMessageStop, StopReason: "end_turn"},
				},
			},
		},
	}
}

// MakeToolUseEvents는 tool_use 블록 이벤트 시퀀스를 생성한다.
// content_block_start(tool_use) → input_json_delta(json) → content_block_stop → message_stop 순서.
// AC-QUERY-002/003 tool roundtrip 테스트용.
func MakeToolUseEvents(toolUseID, toolName string, inputJSON string) []message.StreamEvent {
	return []message.StreamEvent{
		{
			Type:      message.TypeContentBlockStart,
			BlockType: "tool_use",
			ToolUseID: toolUseID,
			Delta:     toolName,
		},
		{
			Type:  message.TypeInputJSONDelta,
			Delta: inputJSON,
		},
		{
			Type: message.TypeContentBlockStop,
		},
		{
			Type:       message.TypeMessageStop,
			StopReason: "tool_use",
		},
	}
}

// MakeStopEvents는 단순 stop 이벤트 시퀀스를 생성한다.
// tool roundtrip 후 2번째 LLM 응답 시뮬레이션용.
func MakeStopEvents(delta string) []message.StreamEvent {
	events := []message.StreamEvent{}
	if delta != "" {
		events = append(events, message.StreamEvent{Type: message.TypeTextDelta, Delta: delta})
	}
	events = append(events, message.StreamEvent{Type: message.TypeMessageStop, StopReason: "end_turn"})
	return events
}

// MakeMaxOutputTokensEvents는 max_output_tokens StopReason으로 종료하는 이벤트 시퀀스를 생성한다.
// AC-QUERY-005: max_output_tokens 재시도 시나리오용.
func MakeMaxOutputTokensEvents(partial string) []message.StreamEvent {
	events := []message.StreamEvent{}
	if partial != "" {
		events = append(events, message.StreamEvent{Type: message.TypeTextDelta, Delta: partial})
	}
	// TypeMessageDelta에 StopReason="max_output_tokens"로 종료 신호 전달
	events = append(events, message.StreamEvent{
		Type:       message.TypeMessageDelta,
		StopReason: "max_output_tokens",
	})
	events = append(events, message.StreamEvent{
		Type:       message.TypeMessageStop,
		StopReason: "max_output_tokens",
	})
	return events
}

// Call은 LLMCallFunc 시그니처를 구현한다.
func (s *StubLLMCall) Call(ctx context.Context, req query.LLMCallReq) (<-chan message.StreamEvent, error) {
	// payload 기록
	s.recordMu.Lock()
	s.RecordedRequests = append(s.RecordedRequests, req)
	s.recordMu.Unlock()

	idx := int(s.callCount.Add(1)) - 1

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.responses) == 0 {
		return nil, fmt.Errorf("StubLLMCall: no responses configured")
	}

	resp := s.responses[idx%len(s.responses)]
	if resp.Err != nil {
		return nil, resp.Err
	}

	events := make([]message.StreamEvent, len(resp.Events))
	copy(events, resp.Events)

	// usage가 설정된 경우 TypeMessageDelta 이벤트를 스트림 끝에 추가한다.
	// REQ-QUERY-011: budget 차감을 위해 queryLoop이 이 이벤트를 소비한다.
	if resp.UsageInputTokens > 0 || resp.UsageOutputTokens > 0 {
		events = append(events, message.StreamEvent{
			Type:         message.TypeMessageDelta,
			InputTokens:  resp.UsageInputTokens,
			OutputTokens: resp.UsageOutputTokens,
		})
	}

	delay := time.Duration(s.InitialDelay)
	ch := make(chan message.StreamEvent, len(events)+1)
	go func() {
		defer close(ch)
		// InitialDelay가 설정된 경우 goroutine 내부에서 지연한다.
		// SubmitMessage 10ms 마감 테스트(T3.3)에서 dial 비용 시뮬레이션용.
		if delay > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
		for _, ev := range events {
			select {
			case <-ctx.Done():
				return
			case ch <- ev:
			}
		}
	}()
	return ch, nil
}

// CallCount는 호출 횟수를 반환한다.
func (s *StubLLMCall) CallCount() int {
	return int(s.callCount.Load())
}

// AsFunc는 LLMCallFunc 타입으로 변환한다.
func (s *StubLLMCall) AsFunc() query.LLMCallFunc {
	return s.Call
}

// --- StubExecutor ---

// StubExecutor는 Executor 인터페이스의 스텁 구현이다.
type StubExecutor struct {
	// handlers는 toolName별 처리 함수이다.
	handlers map[string]func(ctx context.Context, toolUseID string, input map[string]any) (string, error)
	// callGuard는 Run 호출 시 t.Fatal을 발생시키는 guard이다 (nil이면 비활성).
	// AC-QUERY-003: Deny 시 executor가 호출되지 않음을 검증.
	callGuard func(toolName string)
	// mu는 handlers 보호용이다.
	mu sync.Mutex
}

// NewStubExecutor는 새 StubExecutor를 생성한다.
func NewStubExecutor() *StubExecutor {
	return &StubExecutor{
		handlers: make(map[string]func(ctx context.Context, toolUseID string, input map[string]any) (string, error)),
	}
}

// Register는 특정 toolName에 대한 처리 함수를 등록한다.
func (e *StubExecutor) Register(toolName string, handler func(ctx context.Context, toolUseID string, input map[string]any) (string, error)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers[toolName] = handler
}

// SetCallGuard는 Run 호출 시 guard 함수를 설정한다.
// guard가 설정된 경우 Run이 호출되면 guard를 먼저 실행한다.
func (e *StubExecutor) SetCallGuard(guard func(toolName string)) {
	e.callGuard = guard
}

// Run은 Executor.Run을 구현한다.
func (e *StubExecutor) Run(ctx context.Context, toolUseID, toolName string, input map[string]any) (string, error) {
	if e.callGuard != nil {
		e.callGuard(toolName)
	}
	e.mu.Lock()
	handler, ok := e.handlers[toolName]
	e.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("StubExecutor: no handler for tool %q", toolName)
	}
	return handler(ctx, toolUseID, input)
}

// --- StubCanUseTool ---

// StubCanUseTool는 CanUseTool 인터페이스의 스텁 구현이다.
type StubCanUseTool struct {
	// defaultDecision은 Check 호출 시 반환할 기본 결정이다.
	defaultDecision permissions.Decision
	// overrides는 toolName별 결정 오버라이드이다.
	overrides map[string]permissions.Decision
	// mu는 overrides 보호용이다.
	mu sync.Mutex
}

// NewStubCanUseToolAllow는 항상 Allow를 반환하는 스텁을 생성한다.
func NewStubCanUseToolAllow() *StubCanUseTool {
	return &StubCanUseTool{
		defaultDecision: permissions.Decision{Behavior: permissions.Allow},
		overrides:       make(map[string]permissions.Decision),
	}
}

// NewStubCanUseToolDeny는 항상 Deny를 반환하는 스텁을 생성한다.
func NewStubCanUseToolDeny(reason string) *StubCanUseTool {
	return &StubCanUseTool{
		defaultDecision: permissions.Decision{Behavior: permissions.Deny, Reason: reason},
		overrides:       make(map[string]permissions.Decision),
	}
}

// NewStubCanUseToolAsk는 항상 Ask를 반환하는 스텁을 생성한다.
// S4: Ask 분기는 Deny로 대체 처리된다 (S6에서 실제 Ask 구현 예정).
func NewStubCanUseToolAsk(reason string) *StubCanUseTool {
	return &StubCanUseTool{
		defaultDecision: permissions.Decision{Behavior: permissions.Ask, Reason: reason},
		overrides:       make(map[string]permissions.Decision),
	}
}

// SetOverride는 특정 toolName에 대한 결정 오버라이드를 등록한다.
func (s *StubCanUseTool) SetOverride(toolName string, decision permissions.Decision) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.overrides[toolName] = decision
}

// Check는 CanUseTool.Check를 구현한다.
func (s *StubCanUseTool) Check(ctx context.Context, tpc permissions.ToolPermissionContext) permissions.Decision {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.overrides[tpc.ToolName]; ok {
		return d
	}
	return s.defaultDecision
}

// --- StubCompactor ---

// StubCompactor는 Compactor 인터페이스의 스텁 구현이다.
type StubCompactor struct {
	// shouldCompactFn은 ShouldCompact 호출 시 실행할 함수이다 (nil이면 항상 false).
	shouldCompactFn func(state loop.State) bool
	// compactFn은 Compact 호출 시 실행할 함수이다.
	compactFn func(state loop.State) (loop.State, query.CompactBoundary, error)
}

// NewStubCompactorNoop는 항상 false를 반환하는 (no-op) 스텁을 생성한다.
func NewStubCompactorNoop() *StubCompactor {
	return &StubCompactor{
		shouldCompactFn: func(_ loop.State) bool { return false },
	}
}

// ShouldCompact는 Compactor.ShouldCompact를 구현한다.
func (c *StubCompactor) ShouldCompact(state loop.State) bool {
	if c.shouldCompactFn == nil {
		return false
	}
	return c.shouldCompactFn(state)
}

// Compact는 Compactor.Compact를 구현한다.
func (c *StubCompactor) Compact(state loop.State) (loop.State, query.CompactBoundary, error) {
	if c.compactFn == nil {
		return state, query.CompactBoundary{}, nil
	}
	return c.compactFn(state)
}

// SetShouldCompact는 ShouldCompact 동작을 재정의한다.
func (c *StubCompactor) SetShouldCompact(fn func(state loop.State) bool) {
	c.shouldCompactFn = fn
}

// SetCompact는 Compact 동작을 재정의한다.
func (c *StubCompactor) SetCompact(fn func(state loop.State) (loop.State, query.CompactBoundary, error)) {
	c.compactFn = fn
}
