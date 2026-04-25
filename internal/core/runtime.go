// Package core는 goosed 데몬의 핵심 런타임을 제공한다.
// SPEC-GOOSE-CORE-001 — goosed 데몬 부트스트랩 및 Graceful Shutdown
package core

import (
	"context"

	"go.uber.org/zap"
)

// Runtime은 goosed 프로세스의 핵심 컴포넌트를 묶는 컨테이너다.
// GREEN 단계에서 bootstrap() 함수가 이를 초기화한다.
type Runtime struct {
	// State는 프로세스 생애주기 상태 홀더다.
	State *StateHolder
	// Logger는 구조화 JSON 로거다.
	Logger *zap.Logger
	// Shutdown은 cleanup hook 관리자다.
	Shutdown *ShutdownManager
	// RootCtx는 SIGINT/SIGTERM 수신 시 cancel되는 데몬 생애주기 컨텍스트다.
	// 후속 SPEC의 hook이 이 컨텍스트를 구독하여 graceful shutdown에 참여할 수 있다.
	// (REQ-CORE-004(b))
	RootCtx context.Context
	// Sessions는 sessionID → workspace root 매핑 레지스트리다.
	// HOOK-001 dispatcher가 WorkspaceRoot(sessionID)를 통해 접근한다.
	// (SPEC-GOOSE-CORE-001 REQ-CORE-013)
	Sessions SessionRegistry
	// Drain은 CleanupHook 이전에 실행되는 drain consumer 관리자다.
	// TOOLS-001 Registry.Drain() 등 in-flight 작업 마감에 사용한다.
	// (SPEC-GOOSE-CORE-001 REQ-CORE-014)
	Drain *DrainCoordinator
}

// NewRuntime은 기본값으로 초기화된 Runtime을 반환한다.
// logger가 nil이면 nop 로거를 사용한다.
// rootCtx가 nil이면 context.Background()를 사용한다.
func NewRuntime(logger *zap.Logger, rootCtx context.Context) *Runtime {
	if logger == nil {
		logger = zap.NewNop()
	}
	if rootCtx == nil {
		rootCtx = context.Background()
	}
	state := &StateHolder{}
	state.Store(StateInit)

	sessions := NewSessionRegistry()
	drain := NewDrainCoordinator(logger)

	// 패키지 레벨 default session registry wire-up.
	// HOOK-001이 core.WorkspaceRoot(sessionID) 형태로 직접 호출할 수 있도록 한다.
	setDefaultSessionRegistry(sessions)

	return &Runtime{
		State:    state,
		Logger:   logger,
		Shutdown: NewShutdownManager(logger),
		RootCtx:  rootCtx,
		Sessions: sessions,
		Drain:    drain,
	}
}
