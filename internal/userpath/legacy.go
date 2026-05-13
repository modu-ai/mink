package userpath

import (
	"os"
	"path/filepath"
)

// LegacyHome은 MINK 가 마이그레이션할 레거시 on-disk 경로를 반환한다.
//
// 이 파일은 production 코드에서 ".goose" 리터럴이 존재하는 유일한 위치이다.
// AC-005 #2 single-source-of-truth — 이 파일 외부에서 ".goose" 리터럴을 복제하지 말 것.
// scripts/check-brand.sh 가 이 파일(및 legacy_test.go)을 whitelist 로 관리한다.
//
// @MX:NOTE: [AUTO] brand-lint 의도적 예외 — REQ-MINK-UDM-006 의 단일 .goose 리터럴 위치
func LegacyHome() string {
	return filepath.Join(os.Getenv("HOME"), ".goose")
}
