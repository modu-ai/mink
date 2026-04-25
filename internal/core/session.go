package core

import "sync"

// SessionRegistry는 sessionID → workspace root 매핑을 관리한다.
// HOOK-001 dispatcher가 shell hook subprocess의 working directory를 결정할 때
// WorkspaceRoot(sessionID) 형태로 호출한다.
// (SPEC-GOOSE-CORE-001 REQ-CORE-013, AC-CORE-010)
type SessionRegistry interface {
	Register(sessionID, workspaceRoot string)
	Unregister(sessionID string)
	WorkspaceRoot(sessionID string) string // 매핑 없으면 빈 문자열 반환
}

// sessionRegistryImpl은 동시성 안전한 in-memory SessionRegistry 구현체다.
// sync.RWMutex를 사용하여 읽기에 최적화한다 (캐시 hit 시 블로킹 없음).
type sessionRegistryImpl struct {
	mu       sync.RWMutex
	sessions map[string]string // sessionID → workspaceRoot
}

// NewSessionRegistry는 새로운 동시성 안전 SessionRegistry를 반환한다.
func NewSessionRegistry() SessionRegistry {
	return &sessionRegistryImpl{
		sessions: make(map[string]string),
	}
}

// Register는 sessionID에 workspaceRoot를 매핑한다.
// 세션 시작 시 호출하여 매핑을 등록한다.
func (r *sessionRegistryImpl) Register(sessionID, workspaceRoot string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[sessionID] = workspaceRoot
}

// Unregister는 sessionID 매핑을 제거한다.
// 세션 종료 시 호출한다.
func (r *sessionRegistryImpl) Unregister(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, sessionID)
}

// WorkspaceRoot는 sessionID에 대응하는 workspace root 경로를 반환한다.
// 매핑이 없으면 빈 문자열을 반환한다.
// 메모리 캐시 hit 기준 1ms 이내 응답 (blocking I/O 없음).
func (r *sessionRegistryImpl) WorkspaceRoot(sessionID string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessions[sessionID]
}

// defaultSessionRegistry는 프로세스 단일 default registry다.
// NewRuntime 호출 시 wire-up된다. nil-safe 접근을 위해 별도 체크 사용.
// 동시성 안전을 위해 mu로 보호한다.
var (
	defaultSessionRegistry SessionRegistry
	defaultRegistryMu      sync.RWMutex
)

// WorkspaceRoot는 HOOK-001 dispatcher가 직접 호출하는 패키지 레벨 헬퍼다.
// defaultSessionRegistry를 통해 sessionID에 매핑된 workspace root를 반환한다.
// registry가 초기화되지 않은 경우 빈 문자열을 반환한다 (nil-safe).
//
// @MX:ANCHOR: [AUTO] HOOK-001 dispatcher가 직접 호출하는 cross-package surface
// @MX:REASON: fan_in >= 3 — HOOK-001 dispatcher, TOOLS-001, runtime_test가 호출
// @MX:SPEC: SPEC-GOOSE-CORE-001 REQ-CORE-013
func WorkspaceRoot(sessionID string) string {
	defaultRegistryMu.RLock()
	reg := defaultSessionRegistry
	defaultRegistryMu.RUnlock()
	if reg == nil {
		return ""
	}
	return reg.WorkspaceRoot(sessionID)
}

// setDefaultSessionRegistry는 패키지 레벨 default registry를 설정한다.
// NewRuntime 내부에서 wire-up 시 사용한다.
func setDefaultSessionRegistry(reg SessionRegistry) {
	defaultRegistryMu.Lock()
	defaultSessionRegistry = reg
	defaultRegistryMu.Unlock()
}
