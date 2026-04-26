package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ParseManifestFile은 지정된 디렉토리의 manifest.json을 읽어 PluginManifest를 반환한다.
// REQ-PL-005 step (a)(b)
//
// @MX:ANCHOR: [AUTO] ParseManifestFile — 플러그인 manifest 파싱의 단일 진입점
// @MX:REASON: REQ-PL-005 — LoadPlugin, LoadMCPB 모두 이 함수를 통해 manifest를 읽음. fan_in >= 3
// @MX:SPEC: REQ-PL-001, REQ-PL-005
func ParseManifestFile(dir string) (PluginManifest, error) {
	path := filepath.Join(dir, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return PluginManifest{}, fmt.Errorf("manifest.json 읽기 실패 (%s): %w", path, err)
	}

	var m PluginManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return PluginManifest{}, fmt.Errorf("manifest.json 파싱 실패: %w", err)
	}
	return m, nil
}
