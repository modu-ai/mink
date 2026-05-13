package userpath

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// MigrationResult는 MigrateOnce 호출의 결과를 담는다.
type MigrationResult struct {
	Migrated   bool
	Notice     string
	SourcePath string
	DestPath   string
	Method     string
	Err        error
}

var (
	migrateOnce        sync.Once
	migrateFirstResult MigrationResult
	migrateFirstErr    error
	migrateCallCount   atomic.Int64
)

// renameFunc 테스트 seam.
//
// @MX:WARN: [AUTO] 패키지 레벨 가변 함수 포인터 — 테스트 전용 seam, 프로덕션에서 재할당 금지
// @MX:REASON: T-005 EXDEV 테스트 격리에 필요; ResetMigrateForTesting() 이 항상 복원
var renameFunc = os.Rename

// copyFileFunc 테스트 seam.
var copyFileFunc = defaultCopyFile

// verifyHashFunc 테스트 seam.
var verifyHashFunc = defaultVerifyHash

// lockTimeout 은 마이그레이션 락 대기 최대 시간 (기본 30s).
// 테스트에서 SetLockTimeout 으로 교체 가능.
var lockTimeout = 30 * time.Second

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

	// Windows: 파일 락 미지원
	if runtime.GOOS == "windows" {
		return MigrationResult{Err: ErrLockUnsupported}, ErrLockUnsupported
	}

	legacyHome := LegacyHome()
	userHome, err := resolveUserHomePath()
	if err != nil {
		return MigrationResult{Err: err}, err
	}

	// 1. symlink 감지
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

	// 3. .mink 디렉토리 생성 (lock 파일 위치가 필요)
	if mkErr := os.MkdirAll(userHome, 0o700); mkErr != nil {
		return MigrationResult{Err: mkErr}, mkErr
	}

	// 4. 이미 마이그레이션됐는지 확인 (marker)
	markerPath := filepath.Join(userHome, ".migrated-from-goose")
	if _, markerErr := os.Stat(markerPath); markerErr == nil {
		return MigrationResult{Migrated: false, SourcePath: legacyHome, DestPath: userHome}, nil
	}

	// 5. T-006: 파일 락 획득
	lockPath := filepath.Join(userHome, ".migration.lock")
	releaseLock, lockErr := acquireMigrationLock(lockPath, userHome)
	if lockErr != nil {
		return MigrationResult{Err: lockErr}, lockErr
	}

	// 6. 이미 마이그레이션됐는지 재확인 (락 획득 후)
	if _, markerErr := os.Stat(markerPath); markerErr == nil {
		releaseLock()
		return MigrationResult{Migrated: false, SourcePath: legacyHome, DestPath: userHome}, nil
	}

	// 7. rename 직전 lock 해제 + userHome 제거
	// macOS 에서 dst 디렉토리가 존재하면 rename 은 항상 실패 ("file exists").
	// lock 해제 후 비어있는 userHome 을 제거해야 rename(legacyHome → userHome) 성공.
	// process-level sync.Once 가 재진입을 막으므로 이 짧은 구간의 unlock 은 안전하다.
	releaseLock()
	if removeErr := os.RemoveAll(userHome); removeErr != nil {
		return MigrationResult{Err: removeErr, SourcePath: legacyHome, DestPath: userHome}, removeErr
	}

	// 8. atomic rename 시도
	renameErr := renameFunc(legacyHome, userHome)
	if renameErr == nil {
		_ = writeMigrationMarker(markerPath, true)
		return MigrationResult{
			Migrated:   true,
			Notice:     migrationNotice,
			SourcePath: legacyHome,
			DestPath:   userHome,
			Method:     "rename",
		}, nil
	}

	// 8. EXDEV → copy fallback
	if isEXDEV(renameErr) {
		return doCopyFallback(legacyHome, userHome, markerPath)
	}

	return MigrationResult{Migrated: false, SourcePath: legacyHome, DestPath: userHome}, nil
}

// acquireMigrationLock은 .migration.lock 파일을 획득한다.
// stale lock (dead PID) 발견 시 정리 후 재시도한다.
// 획득 성공 시 release 함수와 nil 에러를 반환한다.
//
// @MX:WARN: [AUTO] cleanup-on-failure 경로 — stale lock + partial userHome 정리 필수
// @MX:REASON: stale lock + partial .mink 조합은 다음 실행 시 쓰레기 상태 방지 (REQ-015)
func acquireMigrationLock(lockPath, userHome string) (func(), error) {
	deadline := time.Now().Add(lockTimeout)
	for {
		// O_EXCL: 원자적 생성 — 이미 존재하면 실패
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			// 락 획득 성공: PID + timestamp 기록
			_, _ = fmt.Fprintf(f, "pid=%d\nstarted_at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
			f.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}

		if !os.IsExist(err) {
			return nil, err
		}

		// 락 파일 존재 → stale 확인
		if isLockStale(lockPath) {
			// stale lock + partial .mink → 정리 후 retry
			_ = os.Remove(lockPath)
			// marker 없는 partial .mink 정리 (REQ-015)
			markerPath := filepath.Join(userHome, ".migrated-from-goose")
			if _, markerErr := os.Stat(markerPath); os.IsNotExist(markerErr) {
				_ = os.RemoveAll(userHome)
				_ = os.MkdirAll(userHome, 0o700)
			}
			continue
		}

		// live lock — timeout 대기
		if time.Now().After(deadline) {
			return nil, ErrLockTimeout
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// isLockStale는 lock 파일의 PID 가 실행 중이 아닌지 확인한다.
// os.FindProcess + Signal(0) 를 사용한다 (POSIX: Signal 0 은 프로세스 존재 확인).
func isLockStale(lockPath string) bool {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return true // 읽기 실패 → 일단 stale 로 처리
	}
	pid := parsePIDFromLock(string(data))
	if pid <= 0 {
		return true
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return true
	}
	// Signal(0): 프로세스 존재 확인 (POSIX)
	err = proc.Signal(syscall.Signal(0))
	return err != nil // 에러 = 프로세스 없음 = stale
}

// parsePIDFromLock은 lock 파일 내용에서 pid= 값을 파싱한다.
func parsePIDFromLock(content string) int {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "pid=") {
			pidStr := strings.TrimPrefix(line, "pid=")
			pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
			if err == nil {
				return pid
			}
		}
	}
	return -1
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
// @MX:REASON: SHA-256 hash 불일치 시 source 보존 필수; 실패 시 partial dst 즉시 제거
func doCopyFallback(src, dst, markerPath string) (MigrationResult, error) {
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
		if hashErr := verifyHashFunc(srcPath, dstPath); hashErr != nil {
			return hashErr
		}
		return nil
	}); err != nil {
		_ = os.RemoveAll(dst)
		if errors.Is(err, ErrChecksumMismatch) {
			return MigrationResult{Err: ErrChecksumMismatch, SourcePath: src, DestPath: dst}, ErrChecksumMismatch
		}
		return MigrationResult{Err: err, SourcePath: src, DestPath: dst}, err
	}

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
	return os.Chmod(dst, mode)
}

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

func writeMigrationMarker(path string, brandVerified bool) error {
	binaryName := filepath.Base(os.Args[0])
	content := fmt.Sprintf("migrated_at=%s binary=%s brand_verified=%v\n",
		time.Now().UTC().Format(time.RFC3339),
		binaryName,
		brandVerified,
	)
	return os.WriteFile(path, []byte(content), 0o600)
}
