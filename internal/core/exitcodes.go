// Package core는 goosed 데몬의 핵심 런타임 컴포넌트를 포함한다.
package core

// Exit code 계약 (SPEC-GOOSE-CORE-001 §8)
// sysexits.h 관례를 따른다.
const (
	// ExitOK는 정상 종료를 나타낸다 (SIGINT/SIGTERM 후 모든 hook 성공).
	ExitOK = 0
	// ExitHookPanic은 cleanup hook 실행 중 panic이 발생한 경우다.
	ExitHookPanic = 1
	// ExitConfig는 설정 오류를 나타낸다 (EX_CONFIG).
	// 파싱 실패 또는 포트 충돌 시 사용된다.
	ExitConfig = 78
)
