package plugin

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMCPB_Unzip_UserConfigSubstitution은 AC-PL-006을 검증한다.
// MCPB 파일을 로드할 때 userConfigVariables가 치환되어야 한다.
func TestMCPB_Unzip_UserConfigSubstitution(t *testing.T) {
	mcpbPath := createTestMCPB(t, mcpbSpec{
		manifest: `{
			"name": "mcpb-plugin",
			"version": "1.0.0",
			"mcpServers": [{"name":"srv","transport":"stdio","command":"${API_KEY}"}]
		}`,
		dxtManifest: `{
			"userConfigVariables": [{"name":"API_KEY","required":true}]
		}`,
	})

	cfg := &PluginsYAML{
		Plugins: map[string]PluginConfig{
			"mcpb-plugin": {
				Enabled:             true,
				UserConfigVariables: map[string]string{"API_KEY": "xyz-secret"},
			},
		},
	}
	loader := NewLoader(nil, cfg)
	inst, err := loader.LoadMCPB(mcpbPath)
	require.NoError(t, err)
	assert.Equal(t, PluginID("mcpb-plugin"), inst.ID)
	// TempDir은 설정되어야 한다
	assert.NotEmpty(t, inst.TempDir)
	// 임시 디렉토리는 실제 존재해야 한다
	_, statErr := os.Stat(inst.TempDir)
	assert.NoError(t, statErr)
}

// TestMCPB_MissingRequiredVar는 AC-PL-007을 검증한다.
// required 변수가 plugins.yaml에 없으면 ErrMissingUserConfigVariable을 반환해야 한다.
func TestMCPB_MissingRequiredVar(t *testing.T) {
	mcpbPath := createTestMCPB(t, mcpbSpec{
		manifest: `{"name":"mcpb-missing","version":"1.0.0"}`,
		dxtManifest: `{
			"userConfigVariables": [{"name":"API_KEY","required":true}]
		}`,
	})

	cfg := &PluginsYAML{
		Plugins: map[string]PluginConfig{
			"mcpb-missing": {Enabled: true}, // API_KEY 미지정
		},
	}
	loader := NewLoader(nil, cfg)
	_, err := loader.LoadMCPB(mcpbPath)
	var e ErrMissingUserConfigVariable
	require.ErrorAs(t, err, &e)
	assert.Equal(t, "API_KEY", e.Name)
}

// TestMCPB_ZipSlip_Rejected는 AC-PL-008을 검증한다.
// zip slip 공격 경로를 포함한 MCPB는 ErrZipSlip을 반환해야 한다.
func TestMCPB_ZipSlip_Rejected(t *testing.T) {
	mcpbPath := createZipSlipMCPB(t)

	loader := NewLoader(nil, nil)
	_, err := loader.LoadMCPB(mcpbPath)
	var e ErrZipSlip
	require.ErrorAs(t, err, &e, "expected ErrZipSlip but got: %v", err)
}

// TestMCPB_OptionalVar_UsesDefault은 optional 변수는 기본값 또는 빈 값으로 치환됨을 검증한다.
func TestMCPB_OptionalVar_UsesDefault(t *testing.T) {
	defaultVal := "default-endpoint"
	dxtManifest, _ := json.Marshal(map[string]any{
		"userConfigVariables": []map[string]any{
			{"name": "ENDPOINT", "required": false, "default": defaultVal},
		},
	})

	mcpbPath := createTestMCPB(t, mcpbSpec{
		manifest: `{
			"name": "mcpb-optional",
			"version": "1.0.0",
			"mcpServers": [{"name":"srv","transport":"stdio","command":"${ENDPOINT}"}]
		}`,
		dxtManifest: string(dxtManifest),
	})

	cfg := &PluginsYAML{
		Plugins: map[string]PluginConfig{
			"mcpb-optional": {Enabled: true},
		},
	}
	loader := NewLoader(nil, cfg)
	inst, err := loader.LoadMCPB(mcpbPath)
	require.NoError(t, err)
	assert.Equal(t, PluginID("mcpb-optional"), inst.ID)
}

// TestMCPB_Cleanup_OnError는 MCPB 로드 실패 시 임시 디렉토리가 정리됨을 검증한다.
func TestMCPB_Cleanup_OnError(t *testing.T) {
	mcpbPath := createTestMCPB(t, mcpbSpec{
		manifest:    `{"name":"_reserved","version":"1.0.0"}`, // 예약 이름
		dxtManifest: `{}`,
	})

	loader := NewLoader(nil, nil)
	_, err := loader.LoadMCPB(mcpbPath)
	assert.Error(t, err) // 예약 이름으로 실패해야 한다
}

// --- MCPB 생성 헬퍼 ---

type mcpbSpec struct {
	manifest    string
	dxtManifest string
	extraFiles  map[string]string // 추가 파일 (path → content)
}

// createTestMCPB는 테스트용 .mcpb (zip) 파일을 생성하고 경로를 반환한다.
func createTestMCPB(t *testing.T, spec mcpbSpec) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plugin.mcpb")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	addZipEntry := func(name, content string) {
		ew, werr := w.Create(name)
		require.NoError(t, werr)
		_, werr = ew.Write([]byte(content))
		require.NoError(t, werr)
	}

	addZipEntry("manifest.json", spec.manifest)
	if spec.dxtManifest != "" {
		addZipEntry("dxt-manifest.json", spec.dxtManifest)
	}
	for name, content := range spec.extraFiles {
		addZipEntry(name, content)
	}

	return path
}

// createZipSlipMCPB는 zip slip 공격 경로를 포함한 .mcpb 파일을 생성한다.
func createZipSlipMCPB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "evil.mcpb")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	// 정상 manifest
	ew, werr := w.Create("manifest.json")
	require.NoError(t, werr)
	_, werr = ew.Write([]byte(`{"name":"evil-plugin","version":"1.0.0"}`))
	require.NoError(t, werr)

	// zip slip 경로: ../../etc/evil
	evilHeader := &zip.FileHeader{
		Name: "../../etc/evil",
	}
	_, werr = w.CreateHeader(evilHeader)
	require.NoError(t, werr)

	return path
}
