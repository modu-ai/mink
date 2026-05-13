package userpath

import (
	"os"
	"path/filepath"
)

// LegacyHome은 MINK 가 마이그레이션할 레거시 on-disk 경로를 반환한다.
//
// 본 파일은 production 코드에서 레거시 brand 리터럴이 존재하는 유일한 위치이다.
// AC-005 #2 single-source-of-truth — 본 파일 외부에서 레거시 리터럴을 복제하지 말 것.
// scripts/check-brand.sh 가 본 파일(및 legacy_test.go)을 whitelist 로 관리한다.
//
// @MX:NOTE: [AUTO] brand-lint 의도적 예외 — REQ-MINK-UDM-006 의 단일 레거시 리터럴 위치
func LegacyHome() string {
	return filepath.Join(os.Getenv("HOME"), ".goose")
}

// LegacyBaseName은 LegacyHome 의 basename 부분을 반환한다.
// userpath 내부 다른 파일에서 레거시 basename 이 필요할 때 (예: MINK_HOME
// 경계 검증의 prefix 비교) 본 함수를 통해 우회 — 이를 통해 production code
// 의 다른 *.go 에서 레거시 리터럴이 직접 등장하지 않는다 (AC-005 single source).
func LegacyBaseName() string {
	return filepath.Base(LegacyHome())
}
