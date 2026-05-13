package userpath

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/modu-ai/mink/internal/envalias"
)

// tempPrefix는 MINK 임시 파일의 단일 소스 prefix 이다.
// REQ-MINK-UDM-004: tmp prefix '.mink-'. AC-006.
const tempPrefix = ".mink-"

// userHomeOnce는 UserHome() 결과를 process 레벨에서 캐시한다.
var (
	userHomeOnce   sync.Once
	userHomeCached string
	userHomePanic  interface{}
)

// UserHomeE는 MINK 사용자 홈 디렉토리의 절대 경로와 에러를 반환한다.
//
// 해석 우선순위:
//  1. MINK_HOME 환경 변수 (LookupEnv 로 explicit-empty 구분).
//  2. envalias 로더의 "HOME" 키 (MINK_HOME/GOOSE_HOME alias).
//  3. 기본값: $HOME/.mink/.
//
// MINK_HOME 부정적 케이스 (REQ-MINK-UDM-018, AC-008b):
//   - empty string: ErrMinkHomeEmpty
//   - ".." 세그먼트 포함 (raw input): ErrMinkHomePathTraversal
//   - $HOME/.goose prefix: ErrMinkHomeIsLegacyPath
//   - mkdir 실패 (권한): ErrPermissionDenied 또는 ErrReadOnlyFilesystem
//
// @MX:ANCHOR: [AUTO] MINK 사용자 홈 경로 에러 반환 진입점 — P2 callsite 에서 직접 사용
// @MX:REASON: fan_in expected 30+ (P2 migration callsites across 18 distinct files)
func UserHomeE() (string, error) {
	// 1. MINK_HOME 환경 변수 검사 (explicit-empty 와 unset 구별을 위해 LookupEnv 사용)
	if value, ok := os.LookupEnv("MINK_HOME"); ok {
		// ok=true 이면 변수가 설정됨 (빈 문자열 포함)
		if value == "" {
			return "", ErrMinkHomeEmpty
		}
		// filepath.Clean 호출 전에 raw input 에서 ".." 세그먼트 검사 (OWASP Path Traversal 완화)
		if containsDotDot(value) {
			return "", ErrMinkHomePathTraversal
		}
		cleaned := filepath.Clean(value)
		// $HOME/.goose 접두어 확인
		if isLegacyGoosePath(cleaned) {
			return "", ErrMinkHomeIsLegacyPath
		}
		// 디렉토리 생성 시도 (REQ-019: 0700 모드)
		if err := os.MkdirAll(cleaned, 0o700); err != nil {
			if os.IsPermission(err) {
				return "", ErrPermissionDenied
			}
			return "", ErrReadOnlyFilesystem
		}
		return cleaned, nil
	}

	// 2. envalias 로더의 "HOME" 키 시도 (MINK_HOME/GOOSE_HOME alias)
	loader := envalias.New(envalias.Options{})
	if val, _, found := loader.Get("HOME"); found && val != "" {
		if containsDotDot(val) {
			return "", ErrMinkHomePathTraversal
		}
		cleaned := filepath.Clean(val)
		if isLegacyGoosePath(cleaned) {
			return "", ErrMinkHomeIsLegacyPath
		}
		if err := os.MkdirAll(cleaned, 0o700); err != nil {
			if os.IsPermission(err) {
				return "", ErrPermissionDenied
			}
			return "", ErrReadOnlyFilesystem
		}
		return cleaned, nil
	}

	// 3. 기본값: $HOME/.mink/
	sysHome := os.Getenv("HOME")
	home := filepath.Join(sysHome, ".mink")
	if err := os.MkdirAll(home, 0o700); err != nil {
		if os.IsPermission(err) {
			return "", ErrPermissionDenied
		}
		return "", ErrReadOnlyFilesystem
	}
	return home, nil
}

// containsDotDot는 raw 경로 문자열에서 "/" 로 분리한 세그먼트 중
// ".." 가 있는지 검사한다 (OWASP Path Traversal 완화).
// filepath.Clean 호출 전에 사용해야 한다.
func containsDotDot(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}

// isLegacyGoosePath는 cleaned 경로가 LegacyHome() 접두어를 가지는지 확인한다.
// 레거시 basename 리터럴은 legacy.go 의 LegacyBaseName() 에 단일 위치로 흡수되어 있다 (AC-005).
func isLegacyGoosePath(cleaned string) bool {
	sysHome := os.Getenv("HOME")
	if sysHome == "" {
		return false
	}
	legacyBase := filepath.Join(sysHome, LegacyBaseName())
	return cleaned == legacyBase || strings.HasPrefix(cleaned, legacyBase+string(filepath.Separator))
}

// UserHome은 MINK 사용자 홈 디렉토리의 절대 경로를 반환한다.
// 결과는 process 레벨에서 sync.Once 로 캐시된다.
// 초기화 실패 시 패닉한다 (보안 위반은 startup 에서 즉시 표면화).
//
// @MX:ANCHOR: [AUTO] process-wide MINK 홈 경로 single source — 모든 P2 callsite 가 여기 수렴
// @MX:REASON: fan_in expected 30+ (18 distinct files in P2 migration); cached entry point
func UserHome() string {
	userHomeOnce.Do(func() {
		result, err := UserHomeE()
		if err != nil {
			userHomePanic = err
			return
		}
		userHomeCached = result
	})
	if userHomePanic != nil {
		panic(userHomePanic)
	}
	return userHomeCached
}

// ProjectLocal은 프로젝트 로컬 MINK 디렉토리 (cwd/.mink/) 의 절대 경로를 반환한다.
// cwd 가 빈 문자열이면 빈 문자열을 반환한다.
// REQ-MINK-UDM-001, REQ-MINK-UDM-010.
func ProjectLocal(cwd string) string {
	if cwd == "" {
		return ""
	}
	return filepath.Join(cwd, ".mink")
}

// SubDir는 UserHome() 의 하위 디렉토리 절대 경로를 반환한다.
func SubDir(name string) string {
	return filepath.Join(UserHome(), name)
}

// TempPrefix는 MINK 임시 파일의 표준 prefix 인 ".mink-" 를 반환한다.
// single source of truth for tmp prefix.
// REQ-MINK-UDM-004. AC-006.
func TempPrefix() string {
	return tempPrefix
}
