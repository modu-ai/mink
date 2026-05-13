package userpath_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/modu-ai/mink/internal/userpath"
)

// TestErrors_NonNil는 모든 sentinel error 가 non-nil 임을 검증한다.
// REQ-MINK-UDM-013, REQ-MINK-UDM-018, AC-008b backing.
func TestErrors_NonNil(t *testing.T) {
	t.Parallel()

	allErrors := []struct {
		name string
		err  error
	}{
		{"ErrReadOnlyFilesystem", userpath.ErrReadOnlyFilesystem},
		{"ErrPermissionDenied", userpath.ErrPermissionDenied},
		{"ErrLockTimeout", userpath.ErrLockTimeout},
		{"ErrMinkHomeEmpty", userpath.ErrMinkHomeEmpty},
		{"ErrMinkHomeIsLegacyPath", userpath.ErrMinkHomeIsLegacyPath},
		{"ErrMinkHomePathTraversal", userpath.ErrMinkHomePathTraversal},
		{"ErrLockUnsupported", userpath.ErrLockUnsupported},
		{"ErrChecksumMismatch", userpath.ErrChecksumMismatch},
		{"ErrSymlinkPath", userpath.ErrSymlinkPath},
	}

	for _, tc := range allErrors {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.NotNil(t, tc.err, "%s must be non-nil", tc.name)
		})
	}
}

// TestErrors_Distinct는 각 sentinel error 가 서로 다른 값임을 검증한다.
// errors.Is 비교 시 교차 매칭이 없어야 한다.
func TestErrors_Distinct(t *testing.T) {
	t.Parallel()

	allErrors := []struct {
		name string
		err  error
	}{
		{"ErrReadOnlyFilesystem", userpath.ErrReadOnlyFilesystem},
		{"ErrPermissionDenied", userpath.ErrPermissionDenied},
		{"ErrLockTimeout", userpath.ErrLockTimeout},
		{"ErrMinkHomeEmpty", userpath.ErrMinkHomeEmpty},
		{"ErrMinkHomeIsLegacyPath", userpath.ErrMinkHomeIsLegacyPath},
		{"ErrMinkHomePathTraversal", userpath.ErrMinkHomePathTraversal},
		{"ErrLockUnsupported", userpath.ErrLockUnsupported},
		{"ErrChecksumMismatch", userpath.ErrChecksumMismatch},
		{"ErrSymlinkPath", userpath.ErrSymlinkPath},
	}

	for i := 0; i < len(allErrors); i++ {
		for j := i + 1; j < len(allErrors); j++ {
			ei, ej := allErrors[i], allErrors[j]
			if errors.Is(ei.err, ej.err) {
				t.Errorf("%s and %s must be distinct (errors.Is cross-match detected)", ei.name, ej.name)
			}
		}
	}
}

// TestErrors_StableMessage는 각 sentinel error 의 메시지가 비어 있지 않고
// 안정적임을 검증한다 (다중 호출에서 동일한 결과).
func TestErrors_StableMessage(t *testing.T) {
	t.Parallel()

	allErrors := []struct {
		name string
		err  error
	}{
		{"ErrReadOnlyFilesystem", userpath.ErrReadOnlyFilesystem},
		{"ErrPermissionDenied", userpath.ErrPermissionDenied},
		{"ErrLockTimeout", userpath.ErrLockTimeout},
		{"ErrMinkHomeEmpty", userpath.ErrMinkHomeEmpty},
		{"ErrMinkHomeIsLegacyPath", userpath.ErrMinkHomeIsLegacyPath},
		{"ErrMinkHomePathTraversal", userpath.ErrMinkHomePathTraversal},
		{"ErrLockUnsupported", userpath.ErrLockUnsupported},
		{"ErrChecksumMismatch", userpath.ErrChecksumMismatch},
		{"ErrSymlinkPath", userpath.ErrSymlinkPath},
	}

	for _, tc := range allErrors {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			msg1 := tc.err.Error()
			msg2 := tc.err.Error()
			assert.NotEmpty(t, msg1, "%s must have a non-empty message", tc.name)
			assert.Equal(t, msg1, msg2, "%s message must be stable across calls", tc.name)
		})
	}
}

// TestErrors_SelfIs는 각 sentinel error 가 자기 자신과 errors.Is 매칭됨을 검증한다.
func TestErrors_SelfIs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{"ErrReadOnlyFilesystem", userpath.ErrReadOnlyFilesystem},
		{"ErrPermissionDenied", userpath.ErrPermissionDenied},
		{"ErrLockTimeout", userpath.ErrLockTimeout},
		{"ErrMinkHomeEmpty", userpath.ErrMinkHomeEmpty},
		{"ErrMinkHomeIsLegacyPath", userpath.ErrMinkHomeIsLegacyPath},
		{"ErrMinkHomePathTraversal", userpath.ErrMinkHomePathTraversal},
		{"ErrLockUnsupported", userpath.ErrLockUnsupported},
		{"ErrChecksumMismatch", userpath.ErrChecksumMismatch},
		{"ErrSymlinkPath", userpath.ErrSymlinkPath},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.True(t, errors.Is(tc.err, tc.err), "%s must match itself via errors.Is", tc.name)
		})
	}
}

// TestErrors_WrappedIs는 fmt.Errorf %w 로 감싼 경우 errors.Is 가 동작함을 검증한다.
func TestErrors_WrappedIs(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("outer context: %w", userpath.ErrMinkHomeEmpty)
	assert.True(t, errors.Is(wrapped, userpath.ErrMinkHomeEmpty),
		"wrapped ErrMinkHomeEmpty must match via errors.Is")
}
