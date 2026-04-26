package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig_ParsePluginsYAML는 plugins.yaml 파싱을 검증한다.
func TestConfig_ParsePluginsYAML(t *testing.T) {
	content := `
plugins:
  foo:
    enabled: true
    source: user
    userConfigVariables:
      API_KEY: "secret-123"
  bar:
    enabled: false
marketplace:
  enabled: false
  registries:
    - https://plugins.goose.ai/
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "plugins.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(content), 0644))

	cfg, err := ParsePluginsYAML(cfgPath)
	require.NoError(t, err)
	assert.True(t, cfg.Plugins["foo"].Enabled)
	assert.Equal(t, "secret-123", cfg.Plugins["foo"].UserConfigVariables["API_KEY"])
	assert.False(t, cfg.Plugins["bar"].Enabled)
	assert.False(t, cfg.Marketplace.Enabled)
}

// TestConfig_ParsePluginsYAML_MissingFile은 파일 없을 때 빈 설정 반환을 검증한다.
func TestConfig_ParsePluginsYAML_MissingFile(t *testing.T) {
	cfg, err := ParsePluginsYAML("/nonexistent/plugins.yaml")
	require.NoError(t, err) // 오류 없이 빈 설정 반환
	assert.Empty(t, cfg.Plugins)
}

// TestConfig_ParsePluginsYAML_InvalidYAML은 손상된 YAML 처리를 검증한다.
func TestConfig_ParsePluginsYAML_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "plugins.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("{ invalid yaml ["), 0644))

	_, err := ParsePluginsYAML(cfgPath)
	assert.Error(t, err)
}

// TestConfig_EnabledFalse_Skip는 AC-PL-011을 검증한다.
// enabled=false인 플러그인은 ReloadAll 시 포함되지 않아야 한다.
func TestConfig_EnabledFalse_Skip(t *testing.T) {
	baseDir := t.TempDir()

	// foo 플러그인 디렉토리 생성 (enabled=false)
	fooDir := filepath.Join(baseDir, "foo")
	require.NoError(t, os.MkdirAll(fooDir, 0755))
	writeManifest(t, fooDir, `{"name":"foo","version":"1.0.0"}`)

	// bar 플러그인 디렉토리 생성 (enabled=true)
	barDir := filepath.Join(baseDir, "bar")
	require.NoError(t, os.MkdirAll(barDir, 0755))
	writeManifest(t, barDir, `{"name":"bar","version":"1.0.0"}`)

	cfg := &PluginsYAML{
		Plugins: map[string]PluginConfig{
			"foo": {Enabled: false},
			"bar": {Enabled: true},
		},
	}

	reg := NewPluginRegistry(nil)
	loader := NewLoader(nil, cfg)

	// 두 디렉토리를 소스로 ReloadAll 실행
	err := loader.ReloadFromDirs(reg, []string{barDir, fooDir})
	require.NoError(t, err)

	list := reg.List()
	// foo는 포함되지 않아야 한다
	for _, p := range list {
		assert.NotEqual(t, "foo", string(p.ID), "disabled plugin should not be in registry")
	}
	// bar는 포함되어야 한다
	found := false
	for _, p := range list {
		if string(p.ID) == "bar" {
			found = true
			break
		}
	}
	assert.True(t, found, "enabled plugin should be in registry")
}
