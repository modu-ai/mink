package userpath

import "sync"

// ResetForTesting은 UserHome 의 sync.Once 캐시를 초기화한다.
// 서브테스트 간 환경 변수 변경 후 독립적인 해석을 보장하기 위해 사용한다.
// 이 함수는 export_test.go (build constraint: test-only) 패턴으로만 접근 가능하다.
func ResetForTesting() {
	userHomeOnce = sync.Once{}
	userHomeCached = ""
	userHomePanic = nil
}
