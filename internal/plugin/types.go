package plugin

// PluginSource는 플러그인 출처 tier를 나타낸다.
// 3-tier discovery: user > project > marketplace.
// REQ-PL-005
type PluginSource int

const (
	// SourceUser는 사용자 홈 디렉토리 기반 플러그인이다 (~/.goose/plugins/).
	SourceUser PluginSource = iota
	// SourceProject는 프로젝트 기반 플러그인이다(./.goose/plugins/).
	SourceProject
	// SourceMarketplace는 원격 마켓플레이스 플러그인이다 (Phase 5+ 스텁).
	SourceMarketplace
)

// PluginID는 플러그인의 고유 식별자이다 (= plugin.Name).
type PluginID string

// PluginManifest는 manifest.json의 스키마이다.
// REQ-PL-001: name과 version은 필수.
type PluginManifest struct {
	Name        string                       `json:"name"`
	Version     string                       `json:"version"`
	Description string                       `json:"description,omitempty"`
	Skills      []PluginSkillRef             `json:"skills,omitempty"`
	Agents      []PluginAgentRef             `json:"agents,omitempty"`
	MCPServers  []PluginMCPServerConfig      `json:"mcpServers,omitempty"`
	Commands    []PluginCommandRef           `json:"commands,omitempty"`
	Hooks       map[string][]PluginHookGroup `json:"hooks,omitempty"`
	Permissions []string                     `json:"permissions,omitempty"`
}

// PluginSkillRef는 manifest 내 skill 참조이다.
type PluginSkillRef struct {
	ID   string `json:"id"`
	Path string `json:"path"` // manifest 디렉토리 기준 상대 경로
}

// PluginAgentRef는 manifest 내 agent 참조이다.
type PluginAgentRef struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

// PluginMCPServerConfig는 manifest 내 MCP 서버 설정이다.
type PluginMCPServerConfig struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	URI       string            `json:"uri,omitempty"`
}

// PluginCommandRef는 manifest 내 slash command 참조이다.
type PluginCommandRef struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Path        string `json:"path"`
}

// PluginHookGroup은 manifest hooks 맵의 값 항목이다.
type PluginHookGroup struct {
	Matcher string            `json:"matcher,omitempty"`
	Hooks   []PluginHookEntry `json:"hooks"`
}

// PluginHookEntry는 단일 hook 명령어 항목이다.
// REQ-PL-012: load 시점에는 실행하지 않으며, HOOK-001 dispatcher가 런타임에 실행한다.
type PluginHookEntry struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// HookBindingHandle은 등록된 hook binding의 핸들 (참조용).
type HookBindingHandle struct {
	Event   string
	Matcher string
}

// PluginInstance는 로드된 플러그인 인스턴스이다.
type PluginInstance struct {
	ID             PluginID
	Source         PluginSource
	BaseDir        string // 플러그인 파일 루트
	Manifest       PluginManifest
	LoadedSkills   []string // skill IDs
	LoadedAgents   []string // agent types
	LoadedHooks    []HookBindingHandle
	LoadedMCP      []string // server names
	LoadedCommands []string
	Permissions    []string
	TempDir        string // MCPB extraction 임시 디렉토리
}

// UserConfigVariable은 MCPB dxt-manifest의 user config variable 정의이다.
type UserConfigVariable struct {
	Name     string  `json:"name"`
	Required bool    `json:"required"`
	Default  *string `json:"default,omitempty"`
}

// DXTManifest는 MCPB 파일 내 dxt-manifest.json의 스키마이다.
type DXTManifest struct {
	UserConfigVariables []UserConfigVariable `json:"userConfigVariables,omitempty"`
}

// PluginConfig는 plugins.yaml의 개별 플러그인 설정이다.
type PluginConfig struct {
	Enabled             bool              `yaml:"enabled"`
	Source              string            `yaml:"source,omitempty"` // "user" | "project" | "marketplace"
	UserConfigVariables map[string]string `yaml:"userConfigVariables,omitempty"`
}

// MarketplaceConfig는 plugins.yaml의 marketplace 설정이다.
type MarketplaceConfig struct {
	Enabled    bool     `yaml:"enabled"`
	Registries []string `yaml:"registries,omitempty"`
}

// PluginsYAML은 plugins.yaml 파일의 전체 스키마이다.
type PluginsYAML struct {
	Plugins     map[string]PluginConfig `yaml:"plugins,omitempty"`
	Marketplace MarketplaceConfig       `yaml:"marketplace,omitempty"`
}
