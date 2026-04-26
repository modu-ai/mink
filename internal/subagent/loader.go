package subagent

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// agentNameRE는 유효한 에이전트 이름 패턴이다.
// REQ-SA-018: alphanumeric + underscore만 허용; '-'와 '@'는 AgentID delimiter로 예약.
var agentNameRE = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// allowedFrontmatterKeys는 AgentDefinition에서 허용되는 YAML frontmatter 키 집합이다.
// REQ-SA-004: SKILLS-001 REQ-SK-001과 동일한 allowlist-default-deny 정책.
var allowedFrontmatterKeys = map[string]bool{
	"agent_type":       true,
	"name":             true,
	"description":      true,
	"allowed-tools":    true,
	"use-exact-tools":  true,
	"model":            true,
	"max-turns":        true,
	"permission-mode":  true,
	"effort":           true,
	"mcp-servers":      true,
	"memory-scopes":    true,
	"isolation":        true,
	"source":           true,
	"background":       true,
	"coordinator-mode": true,
	"when-to-use":      true,
	"argument-hint":    true,
	"tools":            true, // alias for allowed-tools
}

// LoadAgentsDir는 지정된 디렉토리에서 *.md 파일을 로드하여 AgentDefinition 슬라이스를 반환한다.
// REQ-SA-018: 유효하지 않은 이름은 ErrInvalidAgentName 누적 에러로 처리한다.
// REQ-SA-004: 허용되지 않은 frontmatter 속성은 ErrUnsafeAgentProperty 누적 에러로 처리한다.
//
// @MX:ANCHOR: [AUTO] 모든 AgentDefinition 로드의 단일 진입점
// @MX:REASON: SPEC-GOOSE-SUBAGENT-001 REQ-SA-004/018 — LoadAgentsDir를 통해서만 definition이 로드됨
func LoadAgentsDir(dir string) ([]AgentDefinition, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []error{fmt.Errorf("loadAgentsDir: read dir: %w", err)}
	}

	var defs []AgentDefinition
	var errs []error

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}

		slug := strings.TrimSuffix(e.Name(), ".md")
		fpath := filepath.Join(dir, e.Name())

		def, defErrs := loadAgentFile(fpath, slug)
		if len(defErrs) > 0 {
			errs = append(errs, defErrs...)
			continue
		}
		defs = append(defs, *def)
	}

	return defs, errs
}

// loadAgentFile는 단일 agent MD 파일을 로드하여 AgentDefinition을 반환한다.
func loadAgentFile(fpath, slug string) (*AgentDefinition, []error) {
	var errs []error

	// REQ-SA-018: 파일명 기반 slug 검증 (frontmatter name보다 우선)
	// slug는 파일명에서 .md를 제거한 것이다.
	// 파일명에 공백, 비ASCII, '/' 등이 있으면 slug 자체가 잘못됨.
	if err := validateAgentSlug(slug); err != nil {
		return nil, []error{err}
	}

	data, err := os.ReadFile(fpath)
	if err != nil {
		return nil, []error{fmt.Errorf("loadAgentFile: read %s: %w", fpath, err)}
	}

	frontmatter, body, err := parseMDFrontmatter(data)
	if err != nil {
		return nil, []error{fmt.Errorf("loadAgentFile: parse frontmatter %s: %w", fpath, err)}
	}

	// REQ-SA-004: allowlist 검증
	rawMap := make(map[string]any)
	if len(frontmatter) > 0 {
		if err2 := yaml.Unmarshal(frontmatter, &rawMap); err2 != nil {
			return nil, []error{fmt.Errorf("loadAgentFile: yaml unmarshal %s: %w", fpath, err2)}
		}
		for k := range rawMap {
			if !allowedFrontmatterKeys[k] {
				errs = append(errs, fmt.Errorf("%w: agent=%q key=%q", ErrUnsafeAgentProperty, slug, k))
			}
		}
		if len(errs) > 0 {
			return nil, errs
		}
	}

	var def AgentDefinition
	if len(frontmatter) > 0 {
		if err2 := yaml.Unmarshal(frontmatter, &def); err2 != nil {
			return nil, []error{fmt.Errorf("loadAgentFile: yaml unmarshal def %s: %w", fpath, err2)}
		}
	}

	// slug를 AgentType으로 설정
	def.AgentType = slug
	if def.Name == "" {
		def.Name = slug
	}

	// REQ-SA-018: frontmatter의 name 필드도 검증
	if err2 := validateAgentSlug(def.Name); err2 != nil {
		return nil, []error{err2}
	}

	// markdown body를 SystemPrompt로 설정
	def.SystemPrompt = strings.TrimSpace(string(body))

	// §6.2: memory-scopes 기본값 설정
	if len(def.MemoryScopes) == 0 {
		def.MemoryScopes = []MemoryScope{ScopeProject}
	}

	// Background 단축키 처리
	if def.Background && def.Isolation == "" {
		def.Isolation = IsolationBackground
	}

	return &def, nil
}

// validateAgentSlug는 에이전트 이름(slug)이 유효한지 검증한다.
// REQ-SA-018: [a-zA-Z0-9_]만 허용; '-', '@', '_' prefix는 금지.
func validateAgentSlug(name string) error {
	if name == "" {
		return fmt.Errorf("%w: empty name", ErrInvalidAgentName)
	}
	if strings.HasPrefix(name, "_") {
		return fmt.Errorf("%w: name starts with '_' (reserved): %q", ErrInvalidAgentName, name)
	}
	if !agentNameRE.MatchString(name) {
		return fmt.Errorf("%w: name contains invalid characters (only [a-zA-Z0-9_] allowed): %q", ErrInvalidAgentName, name)
	}
	return nil
}

// parseMDFrontmatter는 ---로 구분된 YAML frontmatter와 body를 분리한다.
func parseMDFrontmatter(data []byte) (frontmatter []byte, body []byte, err error) {
	if !bytes.HasPrefix(data, []byte("---")) {
		return nil, data, nil
	}
	// 첫 번째 --- 이후에서 두 번째 ---를 찾는다.
	rest := data[3:]
	// 첫 번째 줄바꿈 다음부터 탐색
	nl := bytes.IndexByte(rest, '\n')
	if nl >= 0 {
		rest = rest[nl+1:]
	}
	end := bytes.Index(rest, []byte("\n---"))
	if end < 0 {
		return nil, data, nil
	}
	frontmatter = rest[:end]
	body = rest[end+4:]
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	}
	return frontmatter, body, nil
}

// defaultLogger는 nil-safe 로거를 반환한다.
var defaultLogger *zap.Logger

func init() {
	defaultLogger, _ = zap.NewProduction()
}

// logWarn은 warn 레벨 로그를 출력한다 (nil-safe).
func logWarn(msg string, fields ...zap.Field) {
	if defaultLogger != nil {
		defaultLogger.Warn(msg, fields...)
	}
}
