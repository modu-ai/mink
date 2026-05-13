package userpath_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modu-ai/mink/internal/userpath"
)

// setupMigrationEnv는 테스트용 HOME 디렉토리를 설정하고 캐시를 초기화한다.
func setupMigrationEnv(t *testing.T) (homeDir string) {
	t.Helper()
	homeDir = t.TempDir()
	t.Setenv("HOME", homeDir)
	os.Unsetenv("MINK_HOME")
	t.Cleanup(func() { os.Unsetenv("MINK_HOME") })
	userpath.ResetForTesting()
	userpath.ResetMigrateForTesting()
	return homeDir
}

// createLegacyDir는 HOME 아래 .goose 디렉토리를 생성하고 3개 파일을 추가한다.
func createLegacyDir(t *testing.T, homeDir string) string {
	t.Helper()
	legacyDir := filepath.Join(homeDir, ".goose")
	require.NoError(t, os.MkdirAll(filepath.Join(legacyDir, "memory"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(legacyDir, "permissions"), 0o700))

	// 3개 파일 생성
	writeFile(t, filepath.Join(legacyDir, "memory", "memory.db"), "memory content")
	writeFile(t, filepath.Join(legacyDir, "permissions", "grants.json"), `{"grants":[]}`)
	writeFile(t, filepath.Join(legacyDir, "config.yaml"), "version: 1")
	return legacyDir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

// TestMigrateOnce_HappyPath는 .goose 존재 시 .mink 로 이동하고 marker + notice 를 생성함을 검증한다.
// REQ-MINK-UDM-007, REQ-MINK-UDM-008. AC-001 코어.
func TestMigrateOnce_HappyPath(t *testing.T) {
	homeDir := setupMigrationEnv(t)
	createLegacyDir(t, homeDir)

	ctx := context.Background()
	result, err := userpath.MigrateOnce(ctx)

	require.NoError(t, err)
	assert.True(t, result.Migrated, "Migrated must be true on successful migration")
	assert.Equal(t, "rename", result.Method, "successful rename must set Method='rename'")

	// 1. ~/.mink/ 존재
	minkDir := filepath.Join(homeDir, ".mink")
	info, err2 := os.Stat(minkDir)
	require.NoError(t, err2, ".mink must exist after migration")
	assert.True(t, info.IsDir())

	// 2. ~/.goose/ 제거됨 (atomic rename)
	_, err3 := os.Stat(filepath.Join(homeDir, ".goose"))
	assert.True(t, os.IsNotExist(err3), ".goose must not exist after migration")

	// 3. 3개 파일 존재
	assert.FileExists(t, filepath.Join(minkDir, "memory", "memory.db"))
	assert.FileExists(t, filepath.Join(minkDir, "permissions", "grants.json"))
	assert.FileExists(t, filepath.Join(minkDir, "config.yaml"))

	// 4. marker 파일 존재 + 내용 검증
	markerPath := filepath.Join(minkDir, ".migrated-from-goose")
	assert.FileExists(t, markerPath)
	markerContent, err4 := os.ReadFile(markerPath)
	require.NoError(t, err4)
	markerStr := string(markerContent)
	assert.Contains(t, markerStr, "migrated_at=", "marker must contain migrated_at field")
	assert.Contains(t, markerStr, "binary=", "marker must contain binary field")
	assert.Contains(t, markerStr, "brand_verified=true", "marker must contain brand_verified=true")

	// 5. notice 메시지 검증 (AC-001 #6 gate)
	notice := result.Notice
	assert.NotEmpty(t, notice, "Notice must be non-empty on successful migration")
	assert.Equal(t, 0, strings.Count(notice, "goose"),
		"notice must not contain 'goose' (AC-001 #6 gate 1)")
	assert.GreaterOrEqual(t, countMinkOrMinkKor(notice), 1,
		"notice must contain 'mink' or '밍크' (AC-001 #6 gate 2)")
}

// countMinkOrMinkKor는 notice 에서 'mink' 또는 '밍크' 의 등장 횟수를 센다.
func countMinkOrMinkKor(s string) int {
	return strings.Count(s, "mink") + strings.Count(s, "밍크")
}

// TestMigrateOnce_Idempotency는 2번째 호출 시 Migrated=false (sync.Once 캐시) 를 검증한다.
// REQ-MINK-UDM-007. AC-001 #7.
func TestMigrateOnce_Idempotency(t *testing.T) {
	homeDir := setupMigrationEnv(t)
	createLegacyDir(t, homeDir)

	ctx := context.Background()
	result1, err1 := userpath.MigrateOnce(ctx)
	require.NoError(t, err1)
	assert.True(t, result1.Migrated)

	// 2번째 호출 — sync.Once 로 캐시된 결과를 반환
	result2, err2 := userpath.MigrateOnce(ctx)
	require.NoError(t, err2)
	assert.False(t, result2.Migrated,
		"second MigrateOnce call must return Migrated=false (idempotent, cached)")
}

// TestMigrateOnce_NoOp은 양쪽 디렉토리 모두 없을 때 Migrated=false 를 검증한다.
func TestMigrateOnce_NoOp(t *testing.T) {
	homeDir := setupMigrationEnv(t)
	_ = homeDir // homeDir exists but no .goose subdir

	ctx := context.Background()
	result, err := userpath.MigrateOnce(ctx)

	require.NoError(t, err)
	assert.False(t, result.Migrated, "no-op must return Migrated=false")
	assert.Empty(t, result.Notice, "no-op must have empty Notice")
}

// TestMigrateOnce_AlreadyMigrated는 .mink + marker 파일 이미 존재 시 no-op 를 검증한다.
// T-007 dual-existence 의 일부이지만 marker 있는 케이스는 T-004 에서 선제 처리.
func TestMigrateOnce_AlreadyMigrated(t *testing.T) {
	homeDir := setupMigrationEnv(t)

	// .mink + marker 가 이미 있고 .goose 는 없는 경우 → no-op
	minkDir := filepath.Join(homeDir, ".mink")
	require.NoError(t, os.MkdirAll(minkDir, 0o700))
	writeFile(t, filepath.Join(minkDir, ".migrated-from-goose"), "migrated_at=2026-01-01T00:00:00Z binary=mink brand_verified=true")

	ctx := context.Background()
	result, err := userpath.MigrateOnce(ctx)

	require.NoError(t, err)
	assert.False(t, result.Migrated, "already-migrated state must return Migrated=false")
}

// TestMigrateOnce_SourcePaths는 결과 구조체에 SourcePath, DestPath 가 올바르게 채워짐을 검증한다.
func TestMigrateOnce_SourcePaths(t *testing.T) {
	homeDir := setupMigrationEnv(t)
	createLegacyDir(t, homeDir)

	ctx := context.Background()
	result, err := userpath.MigrateOnce(ctx)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(homeDir, ".goose"), result.SourcePath)
	assert.Equal(t, filepath.Join(homeDir, ".mink"), result.DestPath)
}

// TestMigrateOnce_MINK_HOME_Error는 MINK_HOME 이 유효하지 않을 때 에러를 반환함을 검증한다.
func TestMigrateOnce_MINK_HOME_Error(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("MINK_HOME", "")
	userpath.ResetForTesting()
	userpath.ResetMigrateForTesting()

	ctx := context.Background()
	_, err := userpath.MigrateOnce(ctx)
	assert.ErrorIs(t, err, userpath.ErrMinkHomeEmpty,
		"invalid MINK_HOME must propagate error from resolveUserHomePath")
}

// TestMigrateOnce_ExdevPlaceholder는 rename 실패 시 placeholder no-op 를 반환함을 검증한다.
// T-005 에서 copy fallback 이 이 경로를 채운다.
func TestMigrateOnce_ExdevPlaceholder(t *testing.T) {
	homeDir := setupMigrationEnv(t)
	createLegacyDir(t, homeDir)

	// renameFunc 를 항상 실패하도록 교체
	userpath.SetRenameFunc(func(src, dst string) error {
		return &os.LinkError{Op: "rename", Old: src, New: dst, Err: os.ErrInvalid}
	})
	// t.Cleanup 에서 복원 (ResetMigrateForTesting 이 복원함)

	ctx := context.Background()
	result, err := userpath.MigrateOnce(ctx)
	require.NoError(t, err)
	// T-004 placeholder: rename 실패 시 Migrated=false (T-005 에서 copy fallback 으로 교체)
	assert.False(t, result.Migrated, "rename failure must return Migrated=false in T-004 placeholder")
	assert.Equal(t, filepath.Join(homeDir, ".goose"), result.SourcePath)
}

// TestMigrateOnce_Notice_GateCompliance는 Notice 메시지가 AC-001 #6 gate 를 만족함을 검증한다.
func TestMigrateOnce_Notice_GateCompliance(t *testing.T) {
	homeDir := setupMigrationEnv(t)
	createLegacyDir(t, homeDir)

	ctx := context.Background()
	result, err := userpath.MigrateOnce(ctx)
	require.NoError(t, err)
	require.True(t, result.Migrated)

	notice := result.Notice
	// gate 1: 'goose' 단어 0건
	assert.Equal(t, 0, strings.Count(notice, "goose"),
		"Notice must not contain 'goose' (AC-001 #6 gate 1)")
	// gate 2: 'mink' 또는 '밍크' ≥ 1건
	minkCount := strings.Count(notice, "mink") + strings.Count(notice, "밍크")
	assert.GreaterOrEqual(t, minkCount, 1,
		"Notice must contain 'mink' or '밍크' (AC-001 #6 gate 2)")
}
