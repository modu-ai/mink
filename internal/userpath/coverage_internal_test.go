package userpath

// 본 파일은 internal/userpath 패키지의 unexported 함수에 대한 직접 unit test 모음이다.
// 목적: contract.md 의 coverage strict ≥ 90% 임계 달성 (SPEC-MINK-USERDATA-MIGRATE-001).
// 대상: defaultCopyFile, defaultVerifyHash, isEXDEV, resolveUserHomePath,
//       isLegacyGoosePath, isLockStale, parsePIDFromLock.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- defaultCopyFile ---

// TestDefaultCopyFile_SrcMissing은 src 가 없을 때 os.Open 에러를 그대로 반환함을 검증한다.
// migrate.go:325-327 cover.
func TestDefaultCopyFile_SrcMissing(t *testing.T) {
	dir := t.TempDir()
	err := defaultCopyFile(filepath.Join(dir, "nonexistent"), filepath.Join(dir, "dst"), 0o600)
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err) || strings.Contains(err.Error(), "no such file"),
		"src missing must surface ENOENT, got: %v", err)
}

// TestDefaultCopyFile_DstExists는 dst 가 이미 존재할 때 O_EXCL 로 인해 에러를 반환함을 검증한다.
// migrate.go:331-334 cover.
func TestDefaultCopyFile_DstExists(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	require.NoError(t, os.WriteFile(src, []byte("hello"), 0o600))
	require.NoError(t, os.WriteFile(dst, []byte("existing"), 0o600))

	err := defaultCopyFile(src, dst, 0o600)
	require.Error(t, err)
	assert.True(t, errors.Is(err, os.ErrExist) || strings.Contains(err.Error(), "exists"),
		"dst exists must surface EEXIST due to O_EXCL, got: %v", err)
}

// TestDefaultCopyFile_HappyChmod는 happy path 가 mode bits 를 보존함을 검증한다.
// migrate.go:340 (Chmod) cover.
func TestDefaultCopyFile_HappyChmod(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	require.NoError(t, os.WriteFile(src, []byte("payload"), 0o600))

	err := defaultCopyFile(src, dst, 0o644)
	require.NoError(t, err)

	info, statErr := os.Stat(dst)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm(), "mode bits 보존 (chmod 호출 검증)")

	got, _ := os.ReadFile(dst)
	assert.Equal(t, "payload", string(got))
}

// --- defaultVerifyHash ---

// TestDefaultVerifyHash_SrcMissing은 src 가 없을 때 sha256File(src) 에러를 반환함을 검증한다.
// migrate.go:345-347 cover.
func TestDefaultVerifyHash_SrcMissing(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "dst")
	require.NoError(t, os.WriteFile(dst, []byte("x"), 0o600))

	err := defaultVerifyHash(filepath.Join(dir, "missing"), dst)
	require.Error(t, err)
}

// TestDefaultVerifyHash_DstMissing은 dst 가 없을 때 sha256File(dst) 에러를 반환함을 검증한다.
// migrate.go:348-351 cover.
func TestDefaultVerifyHash_DstMissing(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	require.NoError(t, os.WriteFile(src, []byte("x"), 0o600))

	err := defaultVerifyHash(src, filepath.Join(dir, "missing"))
	require.Error(t, err)
}

// TestDefaultVerifyHash_Mismatch는 src/dst 내용이 다를 때 ErrChecksumMismatch 를 반환함을 검증한다.
// migrate.go:352-354 cover.
func TestDefaultVerifyHash_Mismatch(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	require.NoError(t, os.WriteFile(src, []byte("hello"), 0o600))
	require.NoError(t, os.WriteFile(dst, []byte("world"), 0o600))

	err := defaultVerifyHash(src, dst)
	assert.ErrorIs(t, err, ErrChecksumMismatch)
}

// TestDefaultVerifyHash_Equal는 같은 내용일 때 nil 을 반환함을 검증한다 (happy path).
func TestDefaultVerifyHash_Equal(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	require.NoError(t, os.WriteFile(src, []byte("identical"), 0o600))
	require.NoError(t, os.WriteFile(dst, []byte("identical"), 0o600))

	assert.NoError(t, defaultVerifyHash(src, dst))
}

// --- isEXDEV ---

// TestIsEXDEV는 isEXDEV 가 LinkError + syscall.EXDEV 만 true 를 반환함을 검증한다.
// migrate.go:265-271 cover.
func TestIsEXDEV(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil_err", nil, false},
		{"plain_err", errors.New("generic"), false},
		{"linkerr_other", &os.LinkError{Op: "rename", Old: "a", New: "b", Err: errors.New("other")}, false},
		{"linkerr_exdev", &os.LinkError{Op: "rename", Old: "a", New: "b", Err: syscall.EXDEV}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isEXDEV(tc.err))
		})
	}
}

// --- resolveUserHomePath ---

// TestResolveUserHomePath_Traversal는 MINK_HOME 에 ".." 세그먼트가 있을 때 ErrMinkHomePathTraversal 반환을 검증한다.
// migrate.go:84-86 cover.
func TestResolveUserHomePath_Traversal(t *testing.T) {
	t.Setenv("MINK_HOME", "/tmp/../etc/bad")
	_, err := resolveUserHomePath()
	assert.ErrorIs(t, err, ErrMinkHomePathTraversal)
}

// TestResolveUserHomePath_Empty는 MINK_HOME 이 빈 문자열일 때 ErrMinkHomeEmpty 반환을 검증한다.
// migrate.go:81-83 cover (이미 다른 경로로 cover 되나 명시적 단위 테스트 추가).
func TestResolveUserHomePath_Empty(t *testing.T) {
	t.Setenv("MINK_HOME", "")
	_, err := resolveUserHomePath()
	assert.ErrorIs(t, err, ErrMinkHomeEmpty)
}

// TestResolveUserHomePath_Legacy는 MINK_HOME 이 $HOME/.goose 일 때 ErrMinkHomeIsLegacyPath 반환을 검증한다.
// migrate.go:87-90 cover.
func TestResolveUserHomePath_Legacy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MINK_HOME", filepath.Join(home, ".goose"))
	_, err := resolveUserHomePath()
	assert.ErrorIs(t, err, ErrMinkHomeIsLegacyPath)
}

// TestResolveUserHomePath_ValidOverride는 MINK_HOME 이 유효할 때 cleaned 경로를 그대로 반환함을 검증한다.
// migrate.go:91 cover.
func TestResolveUserHomePath_ValidOverride(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "mink-data")
	t.Setenv("MINK_HOME", custom)
	t.Setenv("HOME", dir)
	got, err := resolveUserHomePath()
	require.NoError(t, err)
	assert.Equal(t, custom, got)
}

// TestResolveUserHomePath_Default는 MINK_HOME 미설정 시 $HOME/.mink 를 반환함을 검증한다.
// migrate.go:93 cover (default branch).
func TestResolveUserHomePath_Default(t *testing.T) {
	dir := t.TempDir()
	os.Unsetenv("MINK_HOME")
	t.Cleanup(func() { os.Unsetenv("MINK_HOME") })
	t.Setenv("HOME", dir)
	got, err := resolveUserHomePath()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, ".mink"), got)
}

// --- isLegacyGoosePath ---

// TestIsLegacyGoosePath_EmptyHome는 HOME 이 빈 문자열일 때 false 를 반환함을 검증한다.
// userpath.go:111-113 cover.
func TestIsLegacyGoosePath_EmptyHome(t *testing.T) {
	t.Setenv("HOME", "")
	assert.False(t, isLegacyGoosePath("/anything"))
}

// TestIsLegacyGoosePath_Match는 HOME/.goose prefix 일 때 true 를 반환함을 검증한다.
func TestIsLegacyGoosePath_Match(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	legacy := filepath.Join(home, ".goose")

	assert.True(t, isLegacyGoosePath(legacy), "exact match")
	assert.True(t, isLegacyGoosePath(filepath.Join(legacy, "sub", "deep")), "prefix match")
	assert.False(t, isLegacyGoosePath(filepath.Join(home, ".mink")), ".mink must not match")
}

// --- isLockStale ---

// TestIsLockStale_FileMissing은 lock 파일이 없을 때 true (stale) 를 반환함을 검증한다.
// migrate.go:234-236 cover.
func TestIsLockStale_FileMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nonexistent.lock")
	assert.True(t, isLockStale(missing), "missing lock file must be treated as stale")
}

// TestIsLockStale_InvalidPID는 pid 가 음수/0 일 때 true (stale) 를 반환함을 검증한다.
// migrate.go:238-240 cover.
func TestIsLockStale_InvalidPID(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"no_pid_token", "started_at=2020\n"},
		{"non_numeric_pid", "pid=not-a-number\n"},
		{"zero_pid", "pid=0\n"},
		{"negative_pid", "pid=-1\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lockPath := filepath.Join(t.TempDir(), "lock")
			require.NoError(t, os.WriteFile(lockPath, []byte(tc.body), 0o600))
			assert.True(t, isLockStale(lockPath), "invalid pid must be treated as stale: %q", tc.body)
		})
	}
}

// TestIsLockStale_LiveSelf는 현재 프로세스의 PID 를 lock 에 기록하면 stale 아님을 검증한다.
// migrate.go:246-247 happy/live cover.
func TestIsLockStale_LiveSelf(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "lock")
	body := "pid=" + itoa(os.Getpid()) + "\n"
	require.NoError(t, os.WriteFile(lockPath, []byte(body), 0o600))
	assert.False(t, isLockStale(lockPath), "self pid must not be stale")
}

// --- parsePIDFromLock ---

// TestParsePIDFromLock는 다양한 입력에 대해 정확한 pid 또는 -1 을 반환함을 검증한다.
// migrate.go:251-262 boundary cover.
func TestParsePIDFromLock(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"empty", "", -1},
		{"no_pid", "started_at=2020-01-01", -1},
		{"non_numeric", "pid=abc", -1},
		{"happy_int", "pid=12345\nstarted_at=2020", 12345},
		{"trailing_whitespace", "pid=  77  \n", 77},
		{"multiple_lines_first_match", "started_at=2020\npid=99\npid=1\n", 99},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, parsePIDFromLock(tc.body))
		})
	}
}

// --- hasMinkUserData ---

// TestHasMinkUserData_Missing은 디렉토리가 없을 때 false 를 반환함을 검증한다.
// migrate.go:385-388 cover (ReadDir 에러).
func TestHasMinkUserData_Missing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nonexistent-dir")
	assert.False(t, hasMinkUserData(missing))
}

// TestHasMinkUserData_EmptyDir는 빈 디렉토리에 대해 false 를 반환함을 검증한다.
func TestHasMinkUserData_EmptyDir(t *testing.T) {
	assert.False(t, hasMinkUserData(t.TempDir()))
}

// TestHasMinkUserData_LockOnly는 .migration.lock 만 있으면 false 를 반환함을 검증한다.
// dual-existence 판정에서 lock 만 있는 경우는 진행 중 마이그레이션이므로 제외.
func TestHasMinkUserData_LockOnly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".migration.lock"), []byte("x"), 0o600))
	assert.False(t, hasMinkUserData(dir))
}

// TestHasMinkUserData_RealFile은 실제 사용자 파일이 있으면 true 를 반환함을 검증한다.
func TestHasMinkUserData_RealFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("x"), 0o600))
	assert.True(t, hasMinkUserData(dir))
}

// --- sha256File ---

// TestSha256File_Missing은 없는 파일에 대해 에러를 반환함을 검증한다.
// migrate.go:359-361 cover (os.Open 에러).
func TestSha256File_Missing(t *testing.T) {
	_, err := sha256File(filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
}

// TestSha256File_Happy는 알려진 내용에 대해 결정적 hash 를 반환함을 검증한다.
func TestSha256File_Happy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "src")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0o600))
	h1, err := sha256File(path)
	require.NoError(t, err)
	h2, err := sha256File(path)
	require.NoError(t, err)
	assert.Equal(t, h1, h2, "동일 파일은 동일 hash")
	assert.Len(t, h1, 64, "SHA-256 hex 길이 = 64")
}

// itoa는 strconv 의존성을 피해 작성한 작은 헬퍼이다.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
