package tui

import (
	"os"
	"path/filepath"
	"testing"
)

// ── T-011: TUI model permission path userpath 마이그레이션 ────────────────────

// TestDefaultPermStorePath_UsesMinkDir는 defaultPermStorePath() 가
// ~/.mink/permissions.json 경로를 반환함을 검증한다.
// REQ-MINK-UDM-002. AC-005.
func TestDefaultPermStorePath_UsesMinkDir(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	os.Unsetenv("MINK_HOME")
	t.Cleanup(func() { os.Unsetenv("MINK_HOME") })

	got := defaultPermStorePath()
	expected := filepath.Join(fakeHome, ".mink", "permissions.json")
	if got != expected {
		t.Errorf("defaultPermStorePath() = %q, want %q (must use .mink, REQ-MINK-UDM-002)", got, expected)
	}
}
