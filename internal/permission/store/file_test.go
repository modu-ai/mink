package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/modu-ai/mink/internal/permission"
)

// ── T-011: permission store userpath 마이그레이션 ─────────────────────────────

// TestNewFileStore_DefaultPath_UsesMinkDir는 NewFileStore 기본 경로가
// ~/.mink/permissions/grants.json 임을 검증한다.
// REQ-MINK-UDM-002. AC-005.
func TestNewFileStore_DefaultPath_UsesMinkDir(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	os.Unsetenv("MINK_HOME")
	t.Cleanup(func() { os.Unsetenv("MINK_HOME") })

	f, err := NewFileStore("", nil)
	require.NoError(t, err)
	require.NoError(t, f.Open())
	defer f.Close()

	// 기본 경로 = fakeHome/.mink/permissions/grants.json
	expected := filepath.Join(fakeHome, ".mink", "permissions", "grants.json")
	if _, statErr := os.Stat(expected); statErr != nil {
		t.Errorf("expected grants.json at %q, got stat error: %v", expected, statErr)
	}
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// makeTestGrant는 테스트용 Grant를 생성한다.
func makeTestGrant(subjectID string, cap permission.Capability, scope string) permission.Grant {
	return permission.Grant{
		ID:          "test-id-" + subjectID,
		SubjectID:   subjectID,
		SubjectType: permission.SubjectSkill,
		Capability:  cap,
		Scope:       scope,
		GrantedAt:   time.Now().UTC(),
		GrantedBy:   "user:test",
	}
}

// TestFileStore_OpenAndSave는 기본 open + save + lookup을 검증한다.
func TestFileStore_OpenAndSave(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "grants.json")

	f, err := NewFileStore(path, nil)
	require.NoError(t, err)
	require.NoError(t, f.Open())
	defer f.Close()

	g := makeTestGrant("skill:a", permission.CapNet, "api.openai.com")
	require.NoError(t, f.Save(g))

	found, ok := f.Lookup("skill:a", permission.CapNet, "api.openai.com")
	assert.True(t, ok)
	assert.Equal(t, g.ID, found.ID)
}

// TestFileStore_FilePermissions_0644Rejected는 AC-PE-007을 검증한다.
// 파일 모드가 0644이면 ErrStoreFilePermissions를 반환해야 한다.
// Covers: REQ-PE-004
func TestFileStore_FilePermissions_0644Rejected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "grants.json")

	// 0644로 파일 생성
	err := os.WriteFile(path, []byte(`{"schema_version":1,"grants":[]}`), 0o644)
	require.NoError(t, err)

	f, err := NewFileStore(path, nil)
	require.NoError(t, err)

	openErr := f.Open()
	require.Error(t, openErr)

	var permErr permission.ErrStoreFilePermissions
	require.ErrorAs(t, openErr, &permErr)
	assert.Contains(t, permErr.Error(), "0644")

	// 이후 Lookup은 miss
	_, ready := f.Lookup("x", permission.CapNet, "y")
	assert.False(t, ready)
}

// TestFileStore_SchemaVersionMismatch_Rejected는 AC-PE-015를 검증한다.
// schema_version이 다르면 ErrIncompatibleStoreVersion을 반환해야 한다.
// Covers: REQ-PE-017
func TestFileStore_SchemaVersionMismatch_Rejected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "grants.json")

	err := os.WriteFile(path, []byte(`{"schema_version":99,"grants":[]}`), 0o600)
	require.NoError(t, err)

	f, err := NewFileStore(path, nil)
	require.NoError(t, err)

	openErr := f.Open()
	require.Error(t, openErr)

	var verErr permission.ErrIncompatibleStoreVersion
	require.ErrorAs(t, openErr, &verErr)
	assert.Equal(t, 99, verErr.Got)
	assert.Equal(t, CurrentSchemaVersion, verErr.Expected)
	assert.Contains(t, verErr.Error(), "99")
	assert.Contains(t, verErr.Error(), "1")
}

// TestFileStore_AtomicWrite_RaceFree는 AC-PE-012를 검증한다.
// 10개 goroutine이 100회씩 동시 Save해도 파일이 손상되지 않아야 한다.
// Covers: REQ-PE-014, REQ-PE-004
func TestFileStore_AtomicWrite_RaceFree(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "grants.json")

	f, err := NewFileStore(path, nil)
	require.NoError(t, err)
	require.NoError(t, f.Open())
	defer f.Close()

	const goroutines = 10
	const writesPerGoroutine = 10 // 총 100 write

	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := range writesPerGoroutine {
				g := permission.Grant{
					ID:          "id-" + string(rune('a'+idx)) + string(rune('a'+j)),
					SubjectID:   "skill:concurrent",
					SubjectType: permission.SubjectSkill,
					Capability:  permission.CapNet,
					Scope:       "scope",
					GrantedAt:   time.Now().UTC(),
					GrantedBy:   "user:test",
				}
				_ = f.Save(g)
			}
		}(i)
	}
	wg.Wait()

	// 파일을 raw 재파싱해서 JSON 손상 여부 확인
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var gf grantsFile
	require.NoError(t, json.Unmarshal(data, &gf), "file must be valid JSON after concurrent writes")

	// 파일 권한 0600 확인
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

// TestFileStore_Revoke는 revoke 동작을 검증한다.
// Covers: REQ-PE-008
func TestFileStore_Revoke(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "grants.json")

	f, err := NewFileStore(path, nil)
	require.NoError(t, err)
	require.NoError(t, f.Open())
	defer f.Close()

	g1 := makeTestGrant("skill:a", permission.CapNet, "api.openai.com")
	g2 := makeTestGrant("skill:b", permission.CapExec, "git")
	require.NoError(t, f.Save(g1))
	require.NoError(t, f.Save(g2))

	n, err := f.Revoke("skill:a")
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	// revoke 후 Lookup miss
	_, ok := f.Lookup("skill:a", permission.CapNet, "api.openai.com")
	assert.False(t, ok)

	// skill:b는 영향 없음
	_, ok = f.Lookup("skill:b", permission.CapExec, "git")
	assert.True(t, ok)
}

// TestFileStore_ExpiredGrant_LookupMiss는 만료된 grant를 Lookup에서 미스로 처리함을 검증한다.
// Covers: REQ-PE-013
func TestFileStore_ExpiredGrant_LookupMiss(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "grants.json")

	f, err := NewFileStore(path, nil)
	require.NoError(t, err)
	require.NoError(t, f.Open())
	defer f.Close()

	past := time.Now().Add(-time.Hour)
	g := makeTestGrant("skill:bar", permission.CapNet, "api.example.com")
	g.ExpiresAt = &past
	require.NoError(t, f.Save(g))

	_, ok := f.Lookup("skill:bar", permission.CapNet, "api.example.com")
	assert.False(t, ok, "expired grant must be a miss")
}

// TestFileStore_GC_PrunesExpired는 GC가 만료 grant를 제거함을 검증한다.
// Covers: REQ-PE-013, AC-PE-011
func TestFileStore_GC_PrunesExpired(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "grants.json")

	f, err := NewFileStore(path, nil)
	require.NoError(t, err)
	require.NoError(t, f.Open())
	defer f.Close()

	past := time.Now().Add(-time.Hour)
	g := makeTestGrant("skill:bar", permission.CapNet, "old.example.com")
	g.ExpiresAt = &past
	require.NoError(t, f.Save(g))

	pruned, err := f.GC(time.Now())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, pruned, 1)

	// GC 후 List에 없음
	grants, err := f.List(permission.Filter{IncludeRevoked: true, IncludeExpired: true})
	require.NoError(t, err)
	for _, gr := range grants {
		assert.NotEqual(t, g.ID, gr.ID, "pruned grant must not be in list")
	}
}

// TestFileStore_List_Filter는 Filter 조건 동작을 검증한다.
// Covers: REQ-PE-008 (CLI 표면 검증)
func TestFileStore_List_Filter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "grants.json")

	f, err := NewFileStore(path, nil)
	require.NoError(t, err)
	require.NoError(t, f.Open())
	defer f.Close()

	// 3개 subject의 grants 생성
	require.NoError(t, f.Save(makeTestGrant("skill:a", permission.CapNet, "a.com")))
	require.NoError(t, f.Save(makeTestGrant("mcp:b", permission.CapExec, "git")))
	require.NoError(t, f.Save(makeTestGrant("agent:c", permission.CapFSRead, "/tmp")))

	// 전체 목록
	all, err := f.List(permission.Filter{})
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// capability 필터
	capNet := permission.CapNet
	netGrants, err := f.List(permission.Filter{Capability: &capNet})
	require.NoError(t, err)
	assert.Len(t, netGrants, 1)
	assert.Equal(t, "skill:a", netGrants[0].SubjectID)

	// subject 필터
	aGrants, err := f.List(permission.Filter{SubjectID: "skill:a"})
	require.NoError(t, err)
	assert.Len(t, aGrants, 1)
}

// TestFileStore_Persist_And_Reload는 파일 재오픈 후 grant가 유지됨을 검증한다.
func TestFileStore_Persist_And_Reload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "grants.json")

	// 첫 번째 open + save
	f1, err := NewFileStore(path, nil)
	require.NoError(t, err)
	require.NoError(t, f1.Open())
	g := makeTestGrant("skill:a", permission.CapNet, "api.openai.com")
	require.NoError(t, f1.Save(g))
	require.NoError(t, f1.Close())

	// 두 번째 open — 동일 데이터 복원
	f2, err := NewFileStore(path, nil)
	require.NoError(t, err)
	require.NoError(t, f2.Open())
	defer f2.Close()

	found, ok := f2.Lookup("skill:a", permission.CapNet, "api.openai.com")
	assert.True(t, ok)
	assert.Equal(t, g.ID, found.ID)
}
