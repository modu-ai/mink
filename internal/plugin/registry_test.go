package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegistry_DuplicatePluginName는 AC-PL-005를 검증한다.
// 동일 이름의 두 번째 플러그인 등록은 ErrDuplicatePluginName을 반환하고
// 첫 번째 인스턴스는 그대로 유지해야 한다.
func TestRegistry_DuplicatePluginName(t *testing.T) {
	reg := NewPluginRegistry(nil)

	inst1 := &PluginInstance{
		ID: PluginID("foo"),
		Manifest: PluginManifest{
			Name: "foo", Version: "1.0.0",
		},
	}
	inst2 := &PluginInstance{
		ID: PluginID("foo"),
		Manifest: PluginManifest{
			Name: "foo", Version: "2.0.0",
		},
	}

	err := reg.registerInstance(inst1)
	require.NoError(t, err)

	err = reg.registerInstance(inst2)
	var dupErr ErrDuplicatePluginName
	require.ErrorAs(t, err, &dupErr)
	assert.Equal(t, "foo", dupErr.Name)

	// 첫 번째 인스턴스는 그대로 유지되어야 한다
	list := reg.List()
	require.Len(t, list, 1)
	assert.Equal(t, "1.0.0", list[0].Manifest.Version)
}

// TestRegistry_List는 등록된 플러그인 목록을 반환함을 검증한다.
func TestRegistry_List(t *testing.T) {
	reg := NewPluginRegistry(nil)
	assert.Empty(t, reg.List())

	reg.registerInstance(&PluginInstance{ID: PluginID("a"), Manifest: PluginManifest{Name: "a", Version: "1.0.0"}}) //nolint:errcheck
	reg.registerInstance(&PluginInstance{ID: PluginID("b"), Manifest: PluginManifest{Name: "b", Version: "1.0.0"}}) //nolint:errcheck

	list := reg.List()
	assert.Len(t, list, 2)
}

// TestRegistry_Unload는 플러그인 언로드를 검증한다.
func TestRegistry_Unload(t *testing.T) {
	reg := NewPluginRegistry(nil)
	reg.registerInstance(&PluginInstance{ID: PluginID("foo"), Manifest: PluginManifest{Name: "foo", Version: "1.0.0"}}) //nolint:errcheck

	err := reg.Unload(PluginID("foo"))
	require.NoError(t, err)
	assert.Empty(t, reg.List())
}

// TestRegistry_Unload_NotFound는 존재하지 않는 플러그인 언로드 오류를 검증한다.
func TestRegistry_Unload_NotFound(t *testing.T) {
	reg := NewPluginRegistry(nil)
	err := reg.Unload(PluginID("nonexistent"))
	assert.Error(t, err)
}

// TestRegistry_ClearThenRegister는 atomic swap이 동작함을 검증한다.
func TestRegistry_ClearThenRegister(t *testing.T) {
	reg := NewPluginRegistry(nil)
	reg.registerInstance(&PluginInstance{ID: PluginID("old"), Manifest: PluginManifest{Name: "old", Version: "1.0.0"}}) //nolint:errcheck

	snapshot := map[PluginID]*PluginInstance{
		"new": {ID: PluginID("new"), Manifest: PluginManifest{Name: "new", Version: "2.0.0"}},
	}
	err := reg.ClearThenRegister(snapshot)
	require.NoError(t, err)

	list := reg.List()
	require.Len(t, list, 1)
	assert.Equal(t, PluginID("new"), list[0].ID)
}
