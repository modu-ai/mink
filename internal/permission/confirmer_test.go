package permission_test

import (
	"context"
	"testing"

	"github.com/modu-ai/goose/internal/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStubConfirmers는 4종 Confirmer 구현의 결정값을 검증한다.
func TestStubConfirmers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	req := permission.PermissionRequest{
		SubjectID:  "skill:test",
		Capability: permission.CapNet,
		Scope:      "api.example.com",
	}

	t.Run("AlwaysAllowConfirmer", func(t *testing.T) {
		t.Parallel()
		dec, err := permission.AlwaysAllowConfirmer{}.Ask(ctx, req)
		require.NoError(t, err)
		assert.True(t, dec.Allow)
		assert.Equal(t, permission.DecisionAlwaysAllow, dec.Choice)
	})

	t.Run("DefaultDenyConfirmer", func(t *testing.T) {
		t.Parallel()
		dec, err := permission.DefaultDenyConfirmer{}.Ask(ctx, req)
		require.NoError(t, err)
		assert.False(t, dec.Allow)
		assert.Equal(t, permission.DecisionDeny, dec.Choice)
		assert.NotEmpty(t, dec.Reason)
	})

	t.Run("OnceOnlyConfirmer", func(t *testing.T) {
		t.Parallel()
		dec, err := permission.OnceOnlyConfirmer{}.Ask(ctx, req)
		require.NoError(t, err)
		assert.True(t, dec.Allow)
		assert.Equal(t, permission.DecisionOnceOnly, dec.Choice)
	})
}

// TestNoopAuditor는 Record가 항상 nil을 반환함을 검증한다.
func TestNoopAuditor(t *testing.T) {
	t.Parallel()
	err := permission.NoopAuditor{}.Record(permission.PermissionEvent{Type: "grant_created"})
	assert.NoError(t, err)
}
