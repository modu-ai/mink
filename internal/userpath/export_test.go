package userpath

import (
	"os"
	"sync"
)

// ResetForTesting은 UserHome 의 sync.Once 캐시를 초기화한다.
// 서브테스트 간 환경 변수 변경 후 독립적인 해석을 보장하기 위해 사용한다.
// 이 함수는 export_test.go (build constraint: test-only) 패턴으로만 접근 가능하다.
func ResetForTesting() {
	userHomeOnce = sync.Once{}
	userHomeCached = ""
	userHomePanic = nil
}

// ResetMigrateForTesting은 MigrateOnce 의 sync.Once + 카운터 캐시를 초기화한다.
// 테스트 간 독립적인 마이그레이션 상태를 보장하기 위해 사용한다.
func ResetMigrateForTesting() {
	migrateOnce = sync.Once{}
	migrateFirstResult = MigrationResult{}
	migrateFirstErr = nil
	migrateCallCount.Store(0)
	renameFunc = os.Rename // seam 복원
}

// SetRenameFunc는 renameFunc 테스트 seam 을 교체한다.
// T-005 EXDEV 시뮬레이션 및 rename 실패 경로 테스트에 사용한다.
// 반드시 ResetMigrateForTesting() 으로 복원해야 한다.
func SetRenameFunc(fn func(string, string) error) {
	renameFunc = fn
}
