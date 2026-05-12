package permission_test

import (
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/permission"
	"github.com/stretchr/testify/assert"
)

// TestErrorTypes는 모든 sentinel/typed error의 Error() 메시지 형식을 검증한다.
func TestErrorTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "ErrUnknownCapability",
			err:  permission.ErrUnknownCapability{Key: "weird_key"},
			want: `unknown capability category: "weird_key"`,
		},
		{
			name: "ErrInvalidScopeShape_nested",
			err:  permission.ErrInvalidScopeShape{Nested: true},
			want: "invalid requires: shape: nested requires: is not allowed",
		},
		{
			name: "ErrInvalidScopeShape_scalar",
			err:  permission.ErrInvalidScopeShape{Category: "net", Value: 123},
			want: `invalid scope shape for category "net"`,
		},
		{
			name: "ErrUndeclaredCapability",
			err:  permission.ErrUndeclaredCapability{Capability: permission.CapNet, Scope: "api.openai.com"},
			want: "is not declared in manifest requires:",
		},
		{
			name: "ErrBlockedByPolicy",
			err:  permission.ErrBlockedByPolicy{Capability: permission.CapExec, Scope: "rm"},
			want: "blocked by security policy",
		},
		{
			name: "ErrSubjectNotReady",
			err:  permission.ErrSubjectNotReady{SubjectID: "skill:foo"},
			want: "is not registered",
		},
		{
			name: "ErrStoreFilePermissions",
			err:  permission.ErrStoreFilePermissions{Path: "/tmp/grants.json", Mode: 0644},
			want: "insecure permissions",
		},
		{
			name: "ErrIncompatibleStoreVersion",
			err:  permission.ErrIncompatibleStoreVersion{Path: "/tmp/g.json", Got: 1, Expected: 2},
			want: "schema_version 1 but expected 2",
		},
		{
			name: "ErrStoreNotReady",
			err:  permission.ErrStoreNotReady{Reason: "open failed"},
			want: "grant store is not ready: open failed",
		},
		{
			name: "ErrConfirmerRequired",
			err:  permission.ErrConfirmerRequired,
			want: "confirmer must not be nil",
		},
		{
			name: "ErrIntegrityCheckFailed",
			err:  permission.ErrIntegrityCheckFailed{SubjectID: "plugin:x", Reason: "checksum mismatch"},
			want: "integrity check failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			msg := tt.err.Error()
			assert.True(t, strings.Contains(msg, tt.want),
				"expected %q to contain %q", msg, tt.want)
		})
	}
}
