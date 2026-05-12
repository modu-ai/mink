package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"github.com/modu-ai/mink/internal/hook"
	"github.com/modu-ai/mink/internal/skill"
	"github.com/modu-ai/mink/internal/subagent"
)

// Loader는 플러그인 로드를 오케스트레이트하는 구조체이다.
// LoadPlugin, LoadMCPB, ReloadFromDirs를 제공한다.
// REQ-PL-005
//
// @MX:ANCHOR: [AUTO] Loader — 플러그인 로드 파이프라인의 오케스트레이터
// @MX:REASON: REQ-PL-005/006/007 — discover→parse→validate→distribute를 단일 진입점에서 조율. fan_in >= 3
// @MX:SPEC: REQ-PL-005, REQ-PL-007
type Loader struct {
	logger *zap.Logger
	cfg    *PluginsYAML

	// 주입 가능한 primitive registries
	skillRegistry *skill.SkillRegistry
	hookRegistry  *hook.HookRegistry
}

// NewLoader는 새 Loader를 생성한다.
func NewLoader(logger *zap.Logger, cfg *PluginsYAML) *Loader {
	if logger == nil {
		logger = zap.NewNop()
	}
	if cfg == nil {
		cfg = &PluginsYAML{}
	}
	return &Loader{
		logger: logger,
		cfg:    cfg,
	}
}

// WithSkillRegistry는 skill registry를 주입한다.
func (l *Loader) WithSkillRegistry(r *skill.SkillRegistry) *Loader {
	l.skillRegistry = r
	return l
}

// WithHookRegistry는 hook registry를 주입한다.
func (l *Loader) WithHookRegistry(r *hook.HookRegistry) *Loader {
	l.hookRegistry = r
	return l
}

// LoadPlugin은 지정된 디렉토리에서 플러그인을 로드한다.
// 파이프라인: manifest 읽기 → 파싱 → 검증 → primitive 로드 → PluginInstance 반환.
// REQ-PL-005
func (l *Loader) LoadPlugin(src PluginSource, dir string) (*PluginInstance, error) {
	return l.loadPluginInternal(src, dir, "")
}

// LoadPluginIfEnabled는 plugins.yaml에서 enabled인 경우에만 LoadPlugin을 실행한다.
// REQ-PL-008: enabled=false면 nil 반환 (에러 없음).
func (l *Loader) LoadPluginIfEnabled(src PluginSource, dir string) (*PluginInstance, error) {
	m, err := ParseManifestFile(dir)
	if err != nil {
		return nil, err
	}
	if !l.cfg.IsEnabled(m.Name) {
		l.logger.Info("플러그인 비활성화 — 스킵", zap.String("plugin", m.Name))
		return nil, nil
	}
	return l.LoadPlugin(src, dir)
}

// LoadMCPB는 .mcpb 파일을 로드한다.
// 파이프라인: zip 해제 → dxt-manifest → user config 치환 → LoadPlugin 위임.
// REQ-PL-006
//
// @MX:WARN: [AUTO] 임시 디렉토리 누수 위험 — 반드시 TempDir를 cleanup 경로에 설정
// @MX:REASON: R7 리스크 — 실패 시 os.RemoveAll(tmpDir) 호출, 성공 시 inst.TempDir에 기록
func (l *Loader) LoadMCPB(mcpbPath string) (*PluginInstance, error) {
	tmpDir, err := extractMCPB(mcpbPath)
	if err != nil {
		return nil, fmt.Errorf("MCPB 압축 해제 실패: %w", err)
	}

	dxt, err := parseDXTManifest(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir) //nolint:errcheck
		return nil, fmt.Errorf("DXT manifest 파싱 실패: %w", err)
	}

	// manifest에서 plugin name을 읽어 userConfigVars 조회
	m, err := ParseManifestFile(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir) //nolint:errcheck
		return nil, fmt.Errorf("MCPB manifest 파싱 실패: %w", err)
	}

	userVars := l.cfg.UserConfigVars(m.Name)
	if userVars == nil {
		userVars = map[string]string{}
	}

	if err := applyUserConfigVars(tmpDir, dxt, userVars); err != nil {
		os.RemoveAll(tmpDir) //nolint:errcheck
		return nil, fmt.Errorf("user config 변수 치환 실패: %w", err)
	}

	inst, err := l.loadPluginInternal(SourceUser, tmpDir, tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir) //nolint:errcheck
		return nil, err
	}
	return inst, nil
}

// loadPluginInternal은 실제 로드 파이프라인이다.
func (l *Loader) loadPluginInternal(src PluginSource, dir string, tempDir string) (*PluginInstance, error) {
	// [1] manifest.json 읽기
	m, err := ParseManifestFile(dir)
	if err != nil {
		return nil, err
	}
	// [2][3] 검증
	if err := ValidateManifest(m); err != nil {
		return nil, err
	}

	inst := &PluginInstance{
		ID:          PluginID(m.Name),
		Source:      src,
		BaseDir:     dir,
		Manifest:    m,
		Permissions: m.Permissions,
		TempDir:     tempDir,
	}

	// [4][5] primitive 로드 (실패 시 전체 롤백)
	if err := l.loadPrimitives(inst, dir, m); err != nil {
		return nil, err
	}
	return inst, nil
}

// loadPrimitives는 4-primitive를 순차 로드한다.
// REQ-PL-011: 부분 등록 방지 — 실패 시 에러를 반환하고 호출자가 rollback 처리.
func (l *Loader) loadPrimitives(inst *PluginInstance, dir string, m PluginManifest) error {
	// Skills: manifest path의 SKILL.md 파일 또는 디렉토리
	for _, ref := range m.Skills {
		skillPath := filepath.Join(dir, ref.Path)
		skillIDs, err := l.resolveSkillIDs(skillPath)
		if err != nil {
			return ErrPrimitiveLoad{Primitive: "skill:" + ref.ID, Cause: err}
		}
		inst.LoadedSkills = append(inst.LoadedSkills, skillIDs...)
	}

	// Agents: manifest path의 .md 파일 또는 디렉토리
	for _, ref := range m.Agents {
		agentPath := filepath.Join(dir, ref.Path)
		agentIDs, err := l.resolveAgentIDs(agentPath, ref.ID)
		if err != nil {
			return ErrPrimitiveLoad{Primitive: "agent:" + ref.ID, Cause: err}
		}
		inst.LoadedAgents = append(inst.LoadedAgents, agentIDs...)
	}

	// MCP Servers: 설정 기록만 (runtime은 mcp 패키지가 관리)
	for _, srv := range m.MCPServers {
		inst.LoadedMCP = append(inst.LoadedMCP, srv.Name)
	}

	// Hooks: hook registry에 등록
	for eventName, groups := range m.Hooks {
		for _, group := range groups {
			for _, entry := range group.Hooks {
				handle := HookBindingHandle{
					Event:   eventName,
					Matcher: group.Matcher,
				}
				if l.hookRegistry != nil {
					h := &pluginHookHandler{command: entry.Command}
					if regErr := l.hookRegistry.Register(hook.HookEvent(eventName), group.Matcher, h); regErr != nil {
						return ErrPrimitiveLoad{Primitive: "hook:" + eventName, Cause: regErr}
					}
				}
				inst.LoadedHooks = append(inst.LoadedHooks, handle)
			}
		}
	}

	// Commands: COMMAND-001 stub — 이름만 기록
	for _, cmd := range m.Commands {
		inst.LoadedCommands = append(inst.LoadedCommands, cmd.Name)
	}

	return nil
}

// resolveSkillIDs는 skill 경로(파일 또는 디렉토리)에서 skill ID 목록을 반환한다.
func (l *Loader) resolveSkillIDs(skillPath string) ([]string, error) {
	info, err := os.Stat(skillPath)
	if err != nil {
		return nil, fmt.Errorf("skill 경로 없음 %s: %w", skillPath, err)
	}

	if info.IsDir() {
		// SKILL.md 탐색
		reg, errs := skill.LoadSkillsDir(skillPath, skill.WithLogger(l.logger))
		if reg == nil && len(errs) > 0 {
			return nil, fmt.Errorf("skill 디렉토리 로드 실패: %v", errs)
		}
		// SkillRegistry에 All() 메서드가 없으므로 파일 시스템 탐색으로 ID 수집
		return collectSkillIDsFromDir(skillPath)
	}

	// 단일 SKILL.md
	content, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, fmt.Errorf("skill 파일 읽기 실패 %s: %w", skillPath, err)
	}
	def, err := skill.ParseSkillFile(skillPath, content)
	if err != nil {
		return nil, fmt.Errorf("skill 파싱 실패 %s: %w", skillPath, err)
	}
	return []string{def.ID}, nil
}

// collectSkillIDsFromDir는 디렉토리에서 SKILL.md 파일을 탐색하여 skill ID 목록을 반환한다.
func collectSkillIDsFromDir(dir string) ([]string, error) {
	var ids []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || d.Name() != "SKILL.md" {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		def, parseErr := skill.ParseSkillFile(path, content)
		if parseErr != nil {
			return parseErr // 파싱 실패는 전체 오류
		}
		ids = append(ids, def.ID)
		return nil
	})
	return ids, err
}

// resolveAgentIDs는 agent 경로(파일 또는 디렉토리)에서 agent ID 목록을 반환한다.
func (l *Loader) resolveAgentIDs(agentPath, refID string) ([]string, error) {
	info, err := os.Stat(agentPath)
	if err != nil {
		return nil, fmt.Errorf("agent 경로 없음 %s: %w", agentPath, err)
	}

	if info.IsDir() {
		defs, _ := subagent.LoadAgentsDir(agentPath)
		ids := make([]string, 0, len(defs))
		for _, d := range defs {
			ids = append(ids, d.AgentType)
		}
		return ids, nil
	}

	// 단일 파일: 파일명에서 slug 추출
	slug := strings.TrimSuffix(filepath.Base(agentPath), ".md")
	if refID != "" {
		slug = refID
	}
	// LoadAgentsDir를 single-file 디렉토리로 실행
	defs, errs := subagent.LoadAgentsDir(filepath.Dir(agentPath))
	if len(errs) > 0 && len(defs) == 0 {
		return nil, fmt.Errorf("agent 로드 실패: %v", errs)
	}
	for _, d := range defs {
		if d.AgentType == slug || d.AgentType == strings.TrimSuffix(filepath.Base(agentPath), ".md") {
			return []string{d.AgentType}, nil
		}
	}
	return []string{slug}, nil
}

// ReloadFromDirs는 여러 디렉토리를 스캔하여 enabled 플러그인을 로드하고 registry를 업데이트한다.
// REQ-PL-007 simplified (단일 tier 소스 목록).
func (l *Loader) ReloadFromDirs(reg *PluginRegistry, dirs []string) error {
	snapshot := make(map[PluginID]*PluginInstance)

	for _, dir := range dirs {
		inst, err := l.LoadPluginIfEnabled(SourceProject, dir)
		if err != nil {
			l.logger.Warn("플러그인 로드 실패 — 건너뜀",
				zap.String("dir", dir),
				zap.Error(err),
			)
			continue
		}
		if inst == nil {
			continue
		}
		if _, exists := snapshot[inst.ID]; exists {
			l.logger.Warn("중복 플러그인 이름 — 건너뜀",
				zap.String("plugin", string(inst.ID)),
			)
			continue
		}
		snapshot[inst.ID] = inst
	}

	return reg.ClearThenRegister(snapshot)
}
