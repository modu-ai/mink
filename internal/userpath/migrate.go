package userpath

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// MigrationResult는 MigrateOnce 호출의 결과를 담는다.
// Migrated 가 true 이면 이번 실행에서 마이그레이션이 성공했다.
// Notice 는 Migrated=true 일 때 caller 가 stderr 로 출력해야 할 한 줄 메시지이다.
//
// T-004: 코어 구조. T-005/T-006/T-007 에서 확장.
type MigrationResult struct {
	// Migrated 는 이번 실행에서 마이그레이션이 수행됐으면 true.
	Migrated bool
	// Notice 는 마이그레이션 완료 시 stdout/stderr 로 출력할 한 줄 메시지 (Korean primary).
	// AC-001 #6: 'goose' 단어 0건 + 'mink'|'밍크' ≥ 1건.
	Notice string
	// SourcePath 는 마이그레이션 전 원본 디렉토리 경로.
	SourcePath string
	// DestPath 는 마이그레이션 후 대상 디렉토리 경로.
	DestPath string
	// Method 는 마이그레이션 방법 ("rename" | "copy").
	Method string
	// Err 는 마이그레이션 중 발생한 에러 (caller-decided policy: fail-fast vs graceful).
	Err error
}

// 마이그레이션 process-level 캐시.
// sync.Once 는 doMigrate 의 첫 실행을 보장한다.
// migrateCallCount 는 호출 횟수를 추적하여 두 번째 이후 Migrated=false 를 반환하는 데 사용된다.
var (
	migrateOnce        sync.Once
	migrateFirstResult MigrationResult
	migrateFirstErr    error
	migrateCallCount   atomic.Int64 // 첫 번째(1) 이후에는 Migrated=false
)

// renameFunc는 os.Rename 의 테스트 seam 이다.
// T-005 에서 EXDEV 시뮬레이션에 사용된다.
//
// @MX:WARN: [AUTO] 패키지 레벨 가변 함수 포인터 — 테스트 전용 seam, 프로덕션에서 재할당 금지
// @MX:REASON: T-005 EXDEV 테스트 격리에 필요; ResetMigrateForTesting() 이 항상 복원
var renameFunc = os.Rename

// migrationNotice는 AC-001 #6 gate 를 만족하는 마이그레이션 완료 메시지이다.
// - 'goose' 단어 0건
// - 'mink' 또는 '밍크' ≥ 1건 포함
const migrationNotice = "INFO: 사용자 데이터가 이전 디렉토리에서 새 mink 디렉토리(밍크)로 마이그레이션되었습니다."

// MigrateOnce는 ~/.goose/ → ~/.mink/ 의 최초 1회 자동 마이그레이션을 수행한다.
// process-level 멱등성은 sync.Once 로 보장한다.
// 두 번째 이후 호출은 항상 Migrated=false 를 반환한다.
// cross-process 안전성은 T-006 에서 파일 락으로 추가 구현된다.
//
// REQ-MINK-UDM-007: 첫 실행 시 MigrateOnce 호출.
// REQ-MINK-UDM-013: typed error 계약 (caller 가 fail-fast vs graceful 결정).
//
// @MX:ANCHOR: [AUTO] process-lifetime 마이그레이션 invariant — CLI + daemon 진입점에서 1회 호출
// @MX:REASON: fan_in expected 2 (cmd/mink T-015, cmd/minkd T-016); 중요 사용자 데이터 이동 경로
func MigrateOnce(ctx context.Context) (MigrationResult, error) {
	callNum := migrateCallCount.Add(1)
	migrateOnce.Do(func() {
		migrateFirstResult, migrateFirstErr = doMigrate(ctx)
	})
	if callNum > 1 {
		// 두 번째 이후 호출: Migrated=false (process-level 멱등)
		return MigrationResult{
			Migrated:   false,
			SourcePath: migrateFirstResult.SourcePath,
			DestPath:   migrateFirstResult.DestPath,
		}, migrateFirstErr
	}
	return migrateFirstResult, migrateFirstErr
}

// resolveUserHomePath는 MkdirAll 없이 MINK 홈 경로만 계산한다.
// doMigrate 에서 rename 대상 경로 계산 시 사용한다
// (UserHomeE 는 MkdirAll 을 호출하여 rename 을 방해할 수 있기 때문).
func resolveUserHomePath() (string, error) {
	if value, ok := os.LookupEnv("MINK_HOME"); ok {
		if value == "" {
			return "", ErrMinkHomeEmpty
		}
		if containsDotDot(value) {
			return "", ErrMinkHomePathTraversal
		}
		cleaned := filepath.Clean(value)
		if isLegacyGoosePath(cleaned) {
			return "", ErrMinkHomeIsLegacyPath
		}
		return cleaned, nil
	}
	return filepath.Join(os.Getenv("HOME"), ".mink"), nil
}

// doMigrate는 실제 마이그레이션 로직을 수행한다.
// MigrateOnce 의 sync.Once 안에서 정확히 한 번 실행된다.
func doMigrate(ctx context.Context) (MigrationResult, error) {
	_ = ctx // 향후 context 취소 지원을 위해 예약

	legacyHome := LegacyHome()
	userHome, err := resolveUserHomePath()
	if err != nil {
		return MigrationResult{Err: err}, err
	}

	// 1. T-007: symlink 감지 (lstat — 심볼릭 링크를 따라가지 않음)
	lstatInfo, lstatErr := os.Lstat(legacyHome)
	if lstatErr == nil && lstatInfo.Mode()&os.ModeSymlink != 0 {
		return MigrationResult{Err: ErrSymlinkPath, SourcePath: legacyHome}, ErrSymlinkPath
	}

	// 2. 레거시 디렉토리 존재 확인
	if os.IsNotExist(lstatErr) {
		// .goose 없음 — no-op
		return MigrationResult{Migrated: false}, nil
	}
	if lstatErr != nil {
		return MigrationResult{Err: lstatErr}, lstatErr
	}

	// 3. 이미 마이그레이션됐는지 확인 (marker 파일 + .mink 디렉토리 존재)
	markerPath := filepath.Join(userHome, ".migrated-from-goose")
	if _, markerErr := os.Stat(markerPath); markerErr == nil {
		// marker 있음 → 이미 마이그레이션 완료 (T-007 dual-existence)
		return MigrationResult{Migrated: false, SourcePath: legacyHome, DestPath: userHome}, nil
	}

	// 4. atomic rename 시도
	if renameErr := renameFunc(legacyHome, userHome); renameErr != nil {
		// T-005: EXDEV 감지 → copy fallback (T-005 에서 구현)
		// T-004 scope: placeholder — no-op 반환 (T-005 가 EXDEV 분기를 채움)
		return MigrationResult{Migrated: false, SourcePath: legacyHome, DestPath: userHome}, nil
	}

	// 5. 성공: marker 파일 작성
	_ = writeMigrationMarker(markerPath, true) // marker 실패는 non-fatal

	return MigrationResult{
		Migrated:   true,
		Notice:     migrationNotice,
		SourcePath: legacyHome,
		DestPath:   userHome,
		Method:     "rename",
	}, nil
}

// writeMigrationMarker는 마이그레이션 marker 파일을 작성한다.
// 포맷: migrated_at=<RFC3339> binary=<binary basename> brand_verified=<bool>
func writeMigrationMarker(path string, brandVerified bool) error {
	binaryName := filepath.Base(os.Args[0])
	content := fmt.Sprintf("migrated_at=%s binary=%s brand_verified=%v\n",
		time.Now().UTC().Format(time.RFC3339),
		binaryName,
		brandVerified,
	)
	return os.WriteFile(path, []byte(content), 0o600)
}
