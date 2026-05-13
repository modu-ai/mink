package userpath_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modu-ai/mink/internal/userpath"
)

// resetForTesting은 sync.Once 캐시를 초기화한다.
// 패키지 외부에서는 접근할 수 없으므로 테스트용 seam 이 export 돼야 한다.
// 이 함수는 userpath_testing.go 에서 정의된 ResetForTesting 을 호출한다.

// TestUserHomeE_MINK_HOME_Happy는 유효한 MINK_HOME 설정 시 해당 경로를 반환함을 검증한다.
// REQ-MINK-UDM-018: MINK_HOME 경계 검증. AC-008a happy path.
func TestUserHomeE_MINK_HOME_Happy(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MINK_HOME", dir)
	userpath.ResetForTesting()

	home, err := userpath.UserHomeE()
	require.NoError(t, err)
	assert.Equal(t, dir, home)
}

// TestUserHomeE_MINK_HOME_Empty는 MINK_HOME="" 시 ErrMinkHomeEmpty 를 반환함을 검증한다.
// REQ-MINK-UDM-018. AC-008b case 1.
func TestUserHomeE_MINK_HOME_Empty(t *testing.T) {
	t.Setenv("MINK_HOME", "")
	userpath.ResetForTesting()

	_, err := userpath.UserHomeE()
	assert.ErrorIs(t, err, userpath.ErrMinkHomeEmpty,
		"MINK_HOME set to empty string must return ErrMinkHomeEmpty")
}

// TestUserHomeE_MINK_HOME_LegacyPath는 MINK_HOME=$HOME/.goose 시 ErrMinkHomeIsLegacyPath 를 반환함을 검증한다.
// REQ-MINK-UDM-018. AC-008b case 2.
func TestUserHomeE_MINK_HOME_LegacyPath(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	legacyPath := filepath.Join(homeDir, ".goose")
	t.Setenv("MINK_HOME", legacyPath)
	userpath.ResetForTesting()

	_, err := userpath.UserHomeE()
	assert.ErrorIs(t, err, userpath.ErrMinkHomeIsLegacyPath,
		"MINK_HOME pointing to .goose path must return ErrMinkHomeIsLegacyPath")
}

// TestUserHomeE_MINK_HOME_PathTraversal는 MINK_HOME=/tmp/../etc/foo 시 ErrMinkHomePathTraversal 을 반환함을 검증한다.
// filepath.Clean 호출 전에 raw input 을 검사해야 한다 (OWASP 완화).
// REQ-MINK-UDM-018. AC-008b case 3.
func TestUserHomeE_MINK_HOME_PathTraversal(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"dotdot_middle", "/tmp/../etc/foo"},
		{"dotdot_at_start", "../relative"},
		{"dotdot_segment", "/home/user/../../root"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MINK_HOME", tc.path)
			userpath.ResetForTesting()

			_, err := userpath.UserHomeE()
			assert.ErrorIs(t, err, userpath.ErrMinkHomePathTraversal,
				"MINK_HOME with .. segment must return ErrMinkHomePathTraversal (raw input: %s)", tc.path)
		})
	}
}

// TestUserHomeE_Default는 MINK_HOME 미설정 시 $HOME/.mink 를 반환함을 검증한다.
// REQ-MINK-UDM-003: ~/.mink/ 가 기본 홈.
func TestUserHomeE_Default(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	// MINK_HOME 을 명시적으로 해제한다 (unset).
	t.Setenv("MINK_HOME", "") // LookupEnv 가 ok=true && value="" → ErrMinkHomeEmpty 가 되므로
	// 아예 환경 변수를 해제해야 한다.
	os.Unsetenv("MINK_HOME") //nolint:tenv // 의도적 unset, t.Cleanup 으로 복원
	t.Cleanup(func() { os.Unsetenv("MINK_HOME") })
	userpath.ResetForTesting()

	home, err := userpath.UserHomeE()
	require.NoError(t, err)
	want := filepath.Join(homeDir, ".mink")
	assert.Equal(t, want, home)
	// 디렉토리가 0700 모드로 생성됐는지 확인
	info, err2 := os.Stat(home)
	require.NoError(t, err2)
	assert.True(t, info.IsDir())
}

// TestUserHome_Panics_Not는 유효한 HOME 환경에서 UserHome() 이 패닉하지 않음을 검증한다.
func TestUserHome_Panics_Not(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	os.Unsetenv("MINK_HOME")
	t.Cleanup(func() { os.Unsetenv("MINK_HOME") })
	userpath.ResetForTesting()

	assert.NotPanics(t, func() {
		_ = userpath.UserHome()
	})
}

// TestProjectLocal는 cwd/.mink 를 반환함을 검증한다.
// REQ-MINK-UDM-001, REQ-MINK-UDM-010.
func TestProjectLocal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cwd  string
		want string
	}{
		{"typical", "/home/user/myproject", "/home/user/myproject/.mink"},
		{"empty_cwd", "", ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := userpath.ProjectLocal(tc.cwd)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestSubDir는 UserHome() + name 을 반환함을 검증한다.
func TestSubDir(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	os.Unsetenv("MINK_HOME")
	t.Cleanup(func() { os.Unsetenv("MINK_HOME") })
	userpath.ResetForTesting()

	got := userpath.SubDir("sessions")
	want := filepath.Join(homeDir, ".mink", "sessions")
	assert.Equal(t, want, got)
}

// TestTempPrefix는 ".mink-" 를 반환함을 검증한다.
// REQ-MINK-UDM-004: tmp prefix. AC-006.
func TestTempPrefix(t *testing.T) {
	t.Parallel()
	prefix := userpath.TempPrefix()
	assert.Equal(t, ".mink-", prefix, "TempPrefix must return the canonical .mink- prefix")
	// single source of truth: 리터럴이 이 파일 (테스트) 과 userpath.go 에만 있어야 한다
	assert.True(t, strings.HasPrefix(prefix, ".mink"),
		"prefix must start with .mink (not .goose or other legacy)")
}

// TestUserHomeE_NonWritable는 MINK_HOME 이 읽기 전용 디렉토리를 가리킬 때
// ErrPermissionDenied 또는 ErrReadOnlyFilesystem 을 반환함을 검증한다.
// EC-4: AC-008b case 4.
func TestUserHomeE_NonWritable(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root 는 권한 검사 우회 — CI 에서는 non-root 기대")
	}
	roDir := t.TempDir()
	// 0400 → 읽기 전용
	require.NoError(t, os.Chmod(roDir, 0o400))
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o700) })
	roHome := filepath.Join(roDir, "minkHome")
	t.Setenv("MINK_HOME", roHome)
	userpath.ResetForTesting()

	_, err := userpath.UserHomeE()
	isExpected := err != nil && (isErrPermissionOrReadOnly(err))
	assert.True(t, isExpected, "non-writable MINK_HOME must return permission/read-only error, got: %v", err)
}

func isErrPermissionOrReadOnly(err error) bool {
	return strings.Contains(err.Error(), "permission") ||
		strings.Contains(err.Error(), "read-only") ||
		err.Error() == userpath.ErrPermissionDenied.Error() ||
		err.Error() == userpath.ErrReadOnlyFilesystem.Error()
}

// TestUserHomeE_EnvAlias_GooseHome은 GOOSE_HOME 이 설정됐을 때 envalias 를 통해 값을 얻음을 검증한다.
func TestUserHomeE_EnvAlias_GooseHome(t *testing.T) {
	dir := t.TempDir()
	// MINK_HOME unset, GOOSE_HOME set → envalias 의 "HOME" 키가 GOOSE_HOME 을 반환한다
	os.Unsetenv("MINK_HOME")
	t.Cleanup(func() { os.Unsetenv("MINK_HOME") })
	t.Setenv("GOOSE_HOME", dir)
	userpath.ResetForTesting()

	home, err := userpath.UserHomeE()
	require.NoError(t, err)
	assert.Equal(t, dir, home)
}

// TestUserHome_CachedResult는 UserHome() 이 동일 값을 두 번 반환함을 검증한다 (sync.Once 캐시).
func TestUserHome_CachedResult(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	os.Unsetenv("MINK_HOME")
	t.Cleanup(func() { os.Unsetenv("MINK_HOME") })
	userpath.ResetForTesting()

	result1 := userpath.UserHome()
	result2 := userpath.UserHome()
	assert.Equal(t, result1, result2, "UserHome must return the same cached result on repeated calls")
}

// TestUserHome_Panic_OnError는 UserHomeE 가 에러를 반환하면 UserHome 이 패닉함을 검증한다.
func TestUserHome_Panic_OnError(t *testing.T) {
	// MINK_HOME="" → ErrMinkHomeEmpty → UserHome 이 패닉해야 한다
	t.Setenv("MINK_HOME", "")
	userpath.ResetForTesting()

	assert.Panics(t, func() {
		_ = userpath.UserHome()
	}, "UserHome must panic when UserHomeE returns an error")
}

// TestUserHomeE_EnvAlias_TraversalRejected는 GOOSE_HOME 이 traversal 경로일 때 에러를 반환함을 검증한다.
func TestUserHomeE_EnvAlias_TraversalRejected(t *testing.T) {
	os.Unsetenv("MINK_HOME")
	t.Cleanup(func() { os.Unsetenv("MINK_HOME") })
	t.Setenv("GOOSE_HOME", "/tmp/../etc/bad")
	userpath.ResetForTesting()

	_, err := userpath.UserHomeE()
	assert.ErrorIs(t, err, userpath.ErrMinkHomePathTraversal)
}

// TestUserHomeE_EnvAlias_LegacyRejected는 GOOSE_HOME 이 .goose 경로일 때 에러를 반환함을 검증한다.
func TestUserHomeE_EnvAlias_LegacyRejected(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	os.Unsetenv("MINK_HOME")
	t.Cleanup(func() { os.Unsetenv("MINK_HOME") })
	legacyPath := filepath.Join(homeDir, ".goose")
	t.Setenv("GOOSE_HOME", legacyPath)
	userpath.ResetForTesting()

	_, err := userpath.UserHomeE()
	assert.ErrorIs(t, err, userpath.ErrMinkHomeIsLegacyPath)
}

// TestUserHomeE_MINK_HOME_ValidMinkPath는 .mink 가 포함된 유효한 경로를 받아들임을 검증한다.
func TestUserHomeE_MINK_HOME_ValidMinkPath(t *testing.T) {
	dir := t.TempDir()
	minkDir := filepath.Join(dir, ".mink")
	t.Setenv("MINK_HOME", minkDir)
	userpath.ResetForTesting()

	home, err := userpath.UserHomeE()
	require.NoError(t, err)
	assert.Equal(t, minkDir, home)
	// 디렉토리 생성 확인
	info, err2 := os.Stat(home)
	require.NoError(t, err2)
	assert.True(t, info.IsDir())
}
