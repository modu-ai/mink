package userpath_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modu-ai/mink/internal/userpath"
)

// TestLegacyHome은 $HOME/.goose 경로를 반환함을 검증한다.
// REQ-MINK-UDM-006: .goose 리터럴은 legacy.go 한 곳에만 존재한다.
// AC-005 #2: single-source-of-truth.
// AC-007 #2: legacy_test.go 에 ".goose" 리터럴 1건이 있으며 MINK migration fallback test 주석 동반.
func TestLegacyHome(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// MINK migration fallback test — 이 주석은 AC-007 #2 marker 로 brand-lint 가 검사한다.
	got := userpath.LegacyHome()

	// .goose 리터럴을 포함하는 단일 기대값 (AC-007 #2 marker 필요)
	want := filepath.Join(homeDir, ".goose")
	assert.Equal(t, want, got, "LegacyHome must return $HOME/.goose")
}

// TestLegacyHome_UsesOsGetenvHOME는 HOME 환경 변수 변경 시 반영됨을 검증한다.
func TestLegacyHome_UsesOsGetenvHOME(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	t.Setenv("HOME", dir1)
	got1 := userpath.LegacyHome()
	assert.Equal(t, filepath.Join(dir1, ".goose"), got1)

	t.Setenv("HOME", dir2)
	got2 := userpath.LegacyHome()
	assert.Equal(t, filepath.Join(dir2, ".goose"), got2)

	assert.NotEqual(t, got1, got2, "LegacyHome must reflect current HOME value")
}

// TestLegacyHome_EmptyHOME는 HOME 미설정 시 ".goose" 상대 경로가 됨을 검증한다.
func TestLegacyHome_EmptyHOME(t *testing.T) {
	original := os.Getenv("HOME")
	os.Unsetenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", original) })

	got := userpath.LegacyHome()
	// HOME 이 빈 문자열이면 filepath.Join("", ".goose") == ".goose"
	assert.Equal(t, ".goose", got, "LegacyHome with empty HOME must return relative .goose path")
}

// TestLegacyHome_DoesNotCreateDirectory는 LegacyHome() 이 디렉토리를 생성하지 않음을 검증한다.
func TestLegacyHome_DoesNotCreateDirectory(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	legacyPath := userpath.LegacyHome()
	// LegacyHome 은 read-only resolver — 디렉토리를 생성해서는 안 된다
	_, err := os.Stat(legacyPath)
	require.Error(t, err, "LegacyHome must not create the directory")
	assert.True(t, os.IsNotExist(err), "LegacyHome directory should not exist")
}
