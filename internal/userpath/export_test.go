package userpath

import (
	"os"
	"sync"
)

// ResetForTesting은 UserHome 의 sync.Once 캐시를 초기화한다.
func ResetForTesting() {
	userHomeOnce = sync.Once{}
	userHomeCached = ""
	userHomePanic = nil
}

// ResetMigrateForTesting은 MigrateOnce 의 sync.Once + 카운터 캐시를 초기화하고
// 모든 테스트 seam 을 기본값으로 복원한다.
func ResetMigrateForTesting() {
	migrateOnce = sync.Once{}
	migrateFirstResult = MigrationResult{}
	migrateFirstErr = nil
	migrateCallCount.Store(0)
	renameFunc = os.Rename
	copyFileFunc = defaultCopyFile
	verifyHashFunc = defaultVerifyHash
}

// SetRenameFunc는 renameFunc 테스트 seam 을 교체한다.
// T-005 EXDEV 시뮬레이션 및 rename 실패 경로 테스트에 사용한다.
func SetRenameFunc(fn func(string, string) error) {
	renameFunc = fn
}

// SetCopyFileFunc는 copyFileFunc 테스트 seam 을 교체한다.
// T-005 mid-copy 실패 시뮬레이션에 사용한다.
func SetCopyFileFunc(fn func(src, dst string, mode os.FileMode) error) {
	copyFileFunc = fn
}

// SetVerifyHashFunc는 verifyHashFunc 테스트 seam 을 교체한다.
// T-005 checksum mismatch 시뮬레이션에 사용한다.
func SetVerifyHashFunc(fn func(src, dst string) error) {
	verifyHashFunc = fn
}
