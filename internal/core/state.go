package core

import "sync/atomic"

// ProcessState는 goosed 데몬의 생애주기 상태를 나타낸다.
// (SPEC-GOOSE-CORE-001 REQ-CORE-003)
type ProcessState int32

const (
	// StateInit은 프로세스 초기화 단계다.
	StateInit ProcessState = iota
	// StateBootstrap은 설정·로거 초기화 단계다.
	StateBootstrap
	// StateServing은 헬스서버가 요청을 받고 있는 정상 동작 단계다.
	StateServing
	// StateDraining은 SIGINT/SIGTERM 수신 후 cleanup hook 실행 중인 단계다.
	StateDraining
	// StateStopped는 모든 cleanup이 완료된 종료 단계다.
	StateStopped
)

// String은 ProcessState를 헬스 응답에 사용할 문자열로 변환한다.
func (s ProcessState) String() string {
	switch s {
	case StateInit:
		return "init"
	case StateBootstrap:
		return "bootstrap"
	case StateServing:
		return "serving"
	case StateDraining:
		return "draining"
	case StateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// StateHolder는 atomic하게 ProcessState를 읽고 쓴다.
// @MX:ANCHOR: [AUTO] 헬스서버와 shutdown 핸들러가 공유하는 상태 원천
// @MX:REASON: fan_in >= 3 (health server, shutdown handler, bootstrap)
type StateHolder struct {
	val atomic.Int32
}

// Load는 현재 상태를 원자적으로 읽는다.
func (h *StateHolder) Load() ProcessState {
	return ProcessState(h.val.Load())
}

// Store는 새 상태를 원자적으로 기록한다.
func (h *StateHolder) Store(s ProcessState) {
	h.val.Store(int32(s))
}
