package plugin

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegistry_ReloadAll_Atomic는 AC-PL-009를 검증한다.
// ClearThenRegister 중 concurrent reader는 완전한 pre/post 상태만 관찰해야 한다.
func TestRegistry_ReloadAll_Atomic(t *testing.T) {
	reg := NewPluginRegistry(nil)

	// 초기 상태: 3개 플러그인
	for _, name := range []string{"pluginA", "pluginB", "pluginC"} {
		reg.registerInstance(&PluginInstance{ //nolint:errcheck
			ID:       PluginID(name),
			Manifest: PluginManifest{Name: name, Version: "1.0.0"},
		})
	}

	var wg sync.WaitGroup
	errs := make(chan error, 100)
	reads := make(chan int, 100) // 읽은 플러그인 수

	// 100개 concurrent reader goroutine 실행
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			list := reg.List()
			reads <- len(list)
		}()
	}

	// atomic swap: 2개 플러그인으로 교체
	snapshot := map[PluginID]*PluginInstance{
		"pluginA": {ID: "pluginA", Manifest: PluginManifest{Name: "pluginA", Version: "2.0.0"}},
		"pluginB": {ID: "pluginB", Manifest: PluginManifest{Name: "pluginB", Version: "2.0.0"}},
	}
	err := reg.ClearThenRegister(snapshot)
	require.NoError(t, err)

	wg.Wait()
	close(errs)
	close(reads)

	// reader가 관찰한 플러그인 수는 2 또는 3이어야 한다 (부분 상태 없음)
	for count := range reads {
		assert.True(t, count == 2 || count == 3,
			"reader saw %d plugins — must be 2 (post) or 3 (pre), not partial", count)
	}
}

// TestRegistry_ClearThenRegister_Rollback는 AC-PL-012 유사 rollback 검증이다.
// snapshot이 nil인 경우 빈 상태로 초기화되어야 한다.
func TestRegistry_ClearThenRegister_EmptySnapshot(t *testing.T) {
	reg := NewPluginRegistry(nil)
	reg.registerInstance(&PluginInstance{ID: "old", Manifest: PluginManifest{Name: "old", Version: "1.0.0"}}) //nolint:errcheck

	err := reg.ClearThenRegister(map[PluginID]*PluginInstance{})
	require.NoError(t, err)
	assert.Empty(t, reg.List())
}
