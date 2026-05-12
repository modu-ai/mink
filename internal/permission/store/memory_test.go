package store_test

import (
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/permission"
	"github.com/modu-ai/mink/internal/permission/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemoryStore_OpenLookupSaveRevokeList는 MemoryStore의 기본 라이프사이클을 검증한다.
func TestMemoryStore_OpenLookupSaveRevokeList(t *testing.T) {
	t.Parallel()

	ms := store.NewMemoryStore()
	require.NoError(t, ms.Open())
	t.Cleanup(func() { _ = ms.Close() })

	// 빈 store에서 lookup 미스
	_, hit := ms.Lookup("skill:foo", permission.CapNet, "api.openai.com")
	assert.False(t, hit)

	// 저장
	now := time.Now()
	g := permission.Grant{
		ID:          "g1",
		SubjectID:   "skill:foo",
		SubjectType: permission.SubjectSkill,
		Capability:  permission.CapNet,
		Scope:       "api.openai.com",
		GrantedAt:   now,
		GrantedBy:   "user",
	}
	require.NoError(t, ms.Save(g))

	// hit 검증
	got, hit := ms.Lookup("skill:foo", permission.CapNet, "api.openai.com")
	assert.True(t, hit)
	assert.Equal(t, "g1", got.ID)

	// AllGrants 1건
	all := ms.AllGrants()
	assert.Len(t, all, 1)

	// Revoke
	count, err := ms.Revoke("skill:foo")
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// revoke 후 lookup 미스
	_, hit = ms.Lookup("skill:foo", permission.CapNet, "api.openai.com")
	assert.False(t, hit)

	// AllGrants는 revoked도 포함해 1건
	all = ms.AllGrants()
	assert.Len(t, all, 1)
	assert.True(t, all[0].Revoked)

	// List with IncludeRevoked
	listed, err := ms.List(permission.Filter{IncludeRevoked: true})
	require.NoError(t, err)
	assert.Len(t, listed, 1)

	// List without IncludeRevoked → 0
	listed, err = ms.List(permission.Filter{})
	require.NoError(t, err)
	assert.Len(t, listed, 0)
}

// TestMemoryStore_FilterBySubject는 List filter의 SubjectID 필터링을 검증한다.
func TestMemoryStore_FilterBySubject(t *testing.T) {
	t.Parallel()

	ms := store.NewMemoryStore()
	require.NoError(t, ms.Open())

	now := time.Now()
	require.NoError(t, ms.Save(permission.Grant{ID: "a", SubjectID: "skill:a", Capability: permission.CapNet, Scope: "x", GrantedAt: now}))
	require.NoError(t, ms.Save(permission.Grant{ID: "b", SubjectID: "skill:b", Capability: permission.CapNet, Scope: "x", GrantedAt: now}))

	listed, err := ms.List(permission.Filter{SubjectID: "skill:a"})
	require.NoError(t, err)
	assert.Len(t, listed, 1)
	assert.Equal(t, "a", listed[0].ID)
}

// TestMemoryStore_FilterByCapability는 List filter의 Capability 필터링을 검증한다.
func TestMemoryStore_FilterByCapability(t *testing.T) {
	t.Parallel()

	ms := store.NewMemoryStore()
	require.NoError(t, ms.Open())

	now := time.Now()
	require.NoError(t, ms.Save(permission.Grant{ID: "n1", SubjectID: "s", Capability: permission.CapNet, Scope: "h1", GrantedAt: now}))
	require.NoError(t, ms.Save(permission.Grant{ID: "f1", SubjectID: "s", Capability: permission.CapFSRead, Scope: "/p", GrantedAt: now}))

	cap := permission.CapNet
	listed, err := ms.List(permission.Filter{Capability: &cap})
	require.NoError(t, err)
	assert.Len(t, listed, 1)
	assert.Equal(t, "n1", listed[0].ID)
}

// TestMemoryStore_GC는 만료/revoked grant 삭제를 검증한다.
func TestMemoryStore_GC(t *testing.T) {
	t.Parallel()

	ms := store.NewMemoryStore()
	require.NoError(t, ms.Open())

	now := time.Now()
	expired := now.Add(-time.Hour)
	live := permission.Grant{ID: "live", SubjectID: "s", Capability: permission.CapNet, Scope: "x", GrantedAt: now}
	old := permission.Grant{ID: "expired", SubjectID: "s", Capability: permission.CapNet, Scope: "y", GrantedAt: expired, ExpiresAt: &expired}

	require.NoError(t, ms.Save(live))
	require.NoError(t, ms.Save(old))

	pruned, err := ms.GC(now)
	require.NoError(t, err)
	assert.Equal(t, 1, pruned)
	assert.Len(t, ms.AllGrants(), 1)
	assert.Equal(t, "live", ms.AllGrants()[0].ID)
}

// TestMemoryStore_LookupExpired는 expired grant가 미스로 처리됨을 검증한다.
func TestMemoryStore_LookupExpired(t *testing.T) {
	t.Parallel()

	ms := store.NewMemoryStore()
	require.NoError(t, ms.Open())

	now := time.Now()
	past := now.Add(-time.Hour)
	require.NoError(t, ms.Save(permission.Grant{
		ID: "exp", SubjectID: "s", Capability: permission.CapNet, Scope: "x",
		GrantedAt: past, ExpiresAt: &past,
	}))

	_, hit := ms.Lookup("s", permission.CapNet, "x")
	assert.False(t, hit)
}
