package userpath

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
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
var (
	migrateOnce        sync.Once
	migrateFirstResult MigrationResult
	migrateFirstErr    error
	migrateCallCount   atomic.Int64
)

// renameFunc는 os.Rename 의 테스트 seam 이다.
//
// @MX:WARN: [AUTO] 패키지 레벨 가변 함수 포인터 — 테스트 전용 seam, 프로덕션에서 재할당 금지
// @MX:REASON: T-005 EXDEV 테스트 격리에 필요; ResetMigrateForTesting() 이 항상 복원
var renameFunc = os.Rename

// copyFileFunc는 단일 파일 복사의 테스트 seam 이다.
// T-005 mid-copy 실패 시뮬레이션에 사용한다.
var copyFileFunc = defaultCopyFile

// verifyHashFunc는 src↔dst SHA-256 비교의 테스트 seam 이다.
// T-005 checksum mismatch 시뮬레이션에 사용한다.
var verifyHashFunc = defaultVerifyHash

// migrationNotice는 AC-001 #6 gate 를 만족하는 마이그레이션 완료 메시지이다.
// - 'goose' 단어 0건
// - 'mink' 또는 '밍크' ≥ 1건 포함
const migrationNotice = "INFO: 사용자 데이터가 이전 디렉토리에서 새 mink 디렉토리(밍크)로 마이그레이션되었습니다."

// MigrateOnce는 ~/.goose/ → ~/.mink/ 의 최초 1회 자동 마이그레이션을 수행한다.
//
// @MX:ANCHOR: [AUTO] process-lifetime 마이그레이션 invariant — CLI + daemon 진입점에서 1회 호출
// @MX:REASON: fan_in expected 2 (cmd/mink T-015, cmd/minkd T-016); 중요 사용자 데이터 이동 경로
func MigrateOnce(ctx context.Context) (MigrationResult, error) {
	callNum := migrateCallCount.Add(1)
	migrateOnce.Do(func() {
		migrateFirstResult, migrateFirstErr = doMigrate(ctx)
	})
	if callNum > 1 {
		return MigrationResult{
			Migrated:   false,
			SourcePath: migrateFirstResult.SourcePath,
			DestPath:   migrateFirstResult.DestPath,
		}, migrateFirstErr
	}
	return migrateFirstResult, migrateFirstErr
}

// resolveUserHomePath는 MkdirAll 없이 MINK 홈 경로만 계산한다.
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
func doMigrate(ctx context.Context) (MigrationResult, error) {
	_ = ctx

	legacyHome := LegacyHome()
	userHome, err := resolveUserHomePath()
	if err != nil {
		return MigrationResult{Err: err}, err
	}

	// 1. T-007: symlink 감지
	lstatInfo, lstatErr := os.Lstat(legacyHome)
	if lstatErr == nil && lstatInfo.Mode()&os.ModeSymlink != 0 {
		return MigrationResult{Err: ErrSymlinkPath, SourcePath: legacyHome}, ErrSymlinkPath
	}

	// 2. 레거시 디렉토리 존재 확인
	if os.IsNotExist(lstatErr) {
		return MigrationResult{Migrated: false}, nil
	}
	if lstatErr != nil {
		return MigrationResult{Err: lstatErr}, lstatErr
	}

	// 3. 이미 마이그레이션됐는지 확인
	markerPath := filepath.Join(userHome, ".migrated-from-goose")
	if _, markerErr := os.Stat(markerPath); markerErr == nil {
		return MigrationResult{Migrated: false, SourcePath: legacyHome, DestPath: userHome}, nil
	}

	// 4. atomic rename 시도
	renameErr := renameFunc(legacyHome, userHome)
	if renameErr == nil {
		// rename 성공
		_ = writeMigrationMarker(markerPath, true)
		return MigrationResult{
			Migrated:   true,
			Notice:     migrationNotice,
			SourcePath: legacyHome,
			DestPath:   userHome,
			Method:     "rename",
		}, nil
	}

	// 5. EXDEV 감지 → copy fallback
	if isEXDEV(renameErr) {
		return doCopyFallback(legacyHome, userHome, markerPath)
	}

	// 기타 rename 실패: no-op (에러 미전파, T-004 범위)
	return MigrationResult{Migrated: false, SourcePath: legacyHome, DestPath: userHome}, nil
}

// isEXDEV는 에러가 cross-device rename (syscall.EXDEV) 인지 판별한다.
func isEXDEV(err error) bool {
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return errors.Is(linkErr.Err, syscall.EXDEV)
	}
	return false
}

// doCopyFallback는 EXDEV 오류 시 io.Copy + SHA-256 verify + cleanup 을 수행한다.
//
// @MX:WARN: [AUTO] 데이터 손실 위험 구간 — verify-before-remove 필수 (R2, REQ-015)
// @MX:REASON: SHA-256 hash 불일치 시 source 보존 필수; 실패 시 partial dst 즉시 제거 (cleanup-on-failure)
func doCopyFallback(src, dst, markerPath string) (MigrationResult, error) {
	// Walk 후 각 파일 복사 + hash 검증
	if err := filepath.Walk(src, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, srcPath)
		if relErr != nil {
			return relErr
		}
		dstPath := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(dstPath, 0o700)
		}

		mode := info.Mode().Perm()
		if copyErr := copyFileFunc(srcPath, dstPath, mode); copyErr != nil {
			return copyErr
		}
		// SHA-256 검증
		if hashErr := verifyHashFunc(srcPath, dstPath); hashErr != nil {
			return hashErr
		}
		return nil
	}); err != nil {
		// 실패: partial dst 정리 (source 보존)
		_ = os.RemoveAll(dst)
		if errors.Is(err, ErrChecksumMismatch) {
			return MigrationResult{Err: ErrChecksumMismatch, SourcePath: src, DestPath: dst}, ErrChecksumMismatch
		}
		return MigrationResult{Err: err, SourcePath: src, DestPath: dst}, err
	}

	// 모든 파일 복사 + 검증 성공 → source 제거 (verify-before-remove)
	if removeErr := os.RemoveAll(src); removeErr != nil {
		return MigrationResult{Err: removeErr, SourcePath: src, DestPath: dst}, removeErr
	}

	_ = writeMigrationMarker(markerPath, true)

	return MigrationResult{
		Migrated:   true,
		Notice:     migrationNotice,
		SourcePath: src,
		DestPath:   dst,
		Method:     "copy",
	}, nil
}

// defaultCopyFile는 단일 파일을 src → dst 로 복사하고 mode bits 를 적용한다.
// REQ-MINK-UDM-019: mode bits 보존. R13.
func defaultCopyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	// umask 간섭 방지를 위해 Chmod 를 명시적으로 호출
	return os.Chmod(dst, mode)
}

// defaultVerifyHash는 src 와 dst 파일의 SHA-256 해시를 비교한다.
// 불일치 시 ErrChecksumMismatch 를 반환한다.
func defaultVerifyHash(src, dst string) error {
	srcHash, err := sha256File(src)
	if err != nil {
		return err
	}
	dstHash, err := sha256File(dst)
	if err != nil {
		return err
	}
	if srcHash != dstHash {
		return ErrChecksumMismatch
	}
	return nil
}

// sha256File는 파일의 SHA-256 hex digest 를 반환한다.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// writeMigrationMarker는 마이그레이션 marker 파일을 작성한다.
func writeMigrationMarker(path string, brandVerified bool) error {
	binaryName := filepath.Base(os.Args[0])
	content := fmt.Sprintf("migrated_at=%s binary=%s brand_verified=%v\n",
		time.Now().UTC().Format(time.RFC3339),
		binaryName,
		brandVerified,
	)
	return os.WriteFile(path, []byte(content), 0o600)
}
