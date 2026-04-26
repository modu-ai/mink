package plugin

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ParsePluginsYAML은 plugins.yaml 파일을 읽어 PluginsYAML 설정을 반환한다.
// 파일이 없으면 빈 설정을 반환한다 (R5 리스크 완화).
//
// @MX:ANCHOR: [AUTO] ParsePluginsYAML — plugins.yaml 로드의 단일 진입점
// @MX:REASON: REQ-PL-008/011 — enabled 여부, userConfigVariables 소스. fan_in >= 3 (loader, mcpb, test)
// @MX:SPEC: REQ-PL-008
func ParsePluginsYAML(path string) (*PluginsYAML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// 파일 없으면 빈 설정 반환 (R5: 부팅 우선)
			return &PluginsYAML{}, nil
		}
		return nil, fmt.Errorf("plugins.yaml 읽기 실패: %w", err)
	}

	var cfg PluginsYAML
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("plugins.yaml 파싱 실패: %w", err)
	}
	return &cfg, nil
}

// IsEnabled는 주어진 플러그인 이름이 plugins.yaml에서 활성화되어 있는지 반환한다.
// REQ-PL-008: enabled=false면 false, 설정 없으면 true (기본 활성화).
func (cfg *PluginsYAML) IsEnabled(name string) bool {
	if cfg == nil || cfg.Plugins == nil {
		return true
	}
	pc, ok := cfg.Plugins[name]
	if !ok {
		return true // 설정 없으면 기본 활성화
	}
	return pc.Enabled
}

// UserConfigVars는 주어진 플러그인의 userConfigVariables 맵을 반환한다.
func (cfg *PluginsYAML) UserConfigVars(name string) map[string]string {
	if cfg == nil || cfg.Plugins == nil {
		return nil
	}
	return cfg.Plugins[name].UserConfigVariables
}
