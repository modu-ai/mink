// Package userpath provides central path resolution for MINK on-disk
// state (~/.mink/, ./.mink/). All callsites that previously hardcoded
// ~/.goose/ patterns route through this package, enabling a single
// migration point and test isolation via t.Setenv("MINK_HOME", t.TempDir()).
//
// SPEC: SPEC-MINK-USERDATA-MIGRATE-001
package userpath

import "errors"

// ErrReadOnlyFilesystem은 대상 파일시스템이 읽기 전용일 때 반환된다.
// REQ-MINK-UDM-013: 타입 기반 에러 계약.
var ErrReadOnlyFilesystem = errors.New("userpath: filesystem is read-only")

// ErrPermissionDenied는 필요한 파일 권한이 없을 때 반환된다.
// REQ-MINK-UDM-013: 타입 기반 에러 계약.
var ErrPermissionDenied = errors.New("userpath: permission denied")

// ErrLockTimeout은 마이그레이션 락 획득 시도가 제한 시간(30s)을 초과했을 때 반환된다.
// REQ-MINK-UDM-011: 락 + 30s 블로킹 대기.
var ErrLockTimeout = errors.New("userpath: migration lock acquisition timed out")

// ErrMinkHomeEmpty는 MINK_HOME 환경 변수가 빈 문자열로 설정되었을 때 반환된다.
// REQ-MINK-UDM-018: MINK_HOME 경계 검증. AC-008b case 1.
var ErrMinkHomeEmpty = errors.New("userpath: MINK_HOME is set but empty")

// ErrMinkHomeIsLegacyPath는 MINK_HOME 이 $HOME/.goose 경로를 가리킬 때 반환된다.
// 레거시 경로 사용 시 조용한 fallback 이 아닌 명시적 에러를 반환한다.
// REQ-MINK-UDM-018: MINK_HOME 경계 검증. AC-008b case 2.
var ErrMinkHomeIsLegacyPath = errors.New("userpath: MINK_HOME points to the legacy path; update to a .mink path")

// ErrMinkHomePathTraversal은 MINK_HOME 원시 값에 ".." 세그먼트가 포함될 때 반환된다.
// filepath.Clean 호출 전에 raw input 을 검사하는 OWASP Path Traversal 완화 규칙 적용.
// REQ-MINK-UDM-018: MINK_HOME 경계 검증. AC-008b case 3.
var ErrMinkHomePathTraversal = errors.New("userpath: MINK_HOME contains path traversal sequence (..)")

// ErrLockUnsupported는 현재 플랫폼(Windows)에서 파일 락이 지원되지 않을 때 반환된다.
// REQ-MINK-UDM-011: Windows fallback, R9.
var ErrLockUnsupported = errors.New("userpath: file locking is not supported on this platform")

// ErrChecksumMismatch는 cross-filesystem copy 후 SHA-256 체크섬 검증이 실패했을 때 반환된다.
// 데이터 무결성 보장: verify-before-remove 정책.
// REQ-MINK-UDM-009: copy fallback + SHA-256 verify. R2.
var ErrChecksumMismatch = errors.New("userpath: checksum mismatch after copy — source preserved, partial destination removed")

// ErrSymlinkPath는 레거시 디렉토리가 심볼릭 링크일 때 반환된다.
// 자동 resolve 없이 graceful error 를 반환한다 (자동 마이그레이션 0건).
// R3, EC-001.
var ErrSymlinkPath = errors.New("userpath: legacy path is a symlink; automatic migration skipped")
