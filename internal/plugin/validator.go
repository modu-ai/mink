package plugin

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/modu-ai/goose/internal/hook"
)

// pluginNameRE는 유효한 플러그인 이름 패턴이다.
// REQ-PL-001: ^[a-z][a-z0-9-]{1,63}$
var pluginNameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{1,63}$`)

// reservedPluginNames는 예약된 플러그인 이름 집합이다.
// REQ-PL-016
//
// @MX:ANCHOR: [AUTO] reservedPluginNames — 예약 이름 차단 집합
// @MX:REASON: REQ-PL-016 — ValidateManifest의 예약 이름 검사에서 참조. fan_in >= 3 (validator, test, loader)
// @MX:SPEC: REQ-PL-016
var reservedPluginNames = map[string]struct{}{
	"goose":  {},
	"claude": {},
	"mcp":    {},
	"plugin": {},
}

// hookEventSet은 HOOK-001의 24개 이벤트 이름을 빠른 조회를 위해 set으로 변환한다.
var hookEventSet func() map[string]struct{}

func init() {
	hookEventSet = func() map[string]struct{} {
		names := hook.HookEventNames()
		set := make(map[string]struct{}, len(names))
		for _, n := range names {
			set[n] = struct{}{}
		}
		return set
	}
}

// ValidateManifest는 PluginManifest의 모든 유효성 검사를 수행한다.
// REQ-PL-001: name 형식 + semver version
// REQ-PL-004: hook event 이름
// REQ-PL-013: mcpServer URI 자격증명
// REQ-PL-016: 예약 이름
//
// @MX:ANCHOR: [AUTO] ValidateManifest — 모든 manifest 유효성 검사의 단일 진입점
// @MX:REASON: REQ-PL-001/004/013/016 — LoadPlugin 파이프라인에서 단일 호출로 검증 완료. fan_in >= 3
// @MX:SPEC: REQ-PL-001, REQ-PL-004, REQ-PL-013, REQ-PL-016
func ValidateManifest(m PluginManifest) error {
	// 예약 이름 검사 (REQ-PL-016)
	// 밑줄 prefix도 예약 처리
	if strings.HasPrefix(m.Name, "_") {
		return ErrReservedPluginName{Name: m.Name}
	}
	if _, ok := reservedPluginNames[m.Name]; ok {
		return ErrReservedPluginName{Name: m.Name}
	}

	// 이름 형식 검사 (REQ-PL-001)
	if !pluginNameRE.MatchString(m.Name) {
		return ErrInvalidManifest{Reason: "name must match ^[a-z][a-z0-9-]{1,63}$, got: " + m.Name}
	}

	// 버전 semver 검사 (REQ-PL-001)
	if err := validateSemver(m.Version); err != nil {
		return ErrInvalidManifest{Reason: "version: " + err.Error()}
	}

	// hook event 이름 검사 (REQ-PL-004)
	validEvents := hookEventSet()
	for event := range m.Hooks {
		if _, ok := validEvents[event]; !ok {
			return ErrUnknownHookEvent{Event: event}
		}
	}

	// mcpServer URI 자격증명 검사 (REQ-PL-013)
	for _, srv := range m.MCPServers {
		if srv.URI != "" {
			if err := validateNoCredentialsInURI(srv.URI); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateSemver는 version 문자열이 유효한 semver인지 검사한다.
// 외부 라이브러리 없이 stdlib로 구현한다.
func validateSemver(v string) error {
	if v == "" {
		return ErrInvalidManifest{Reason: "version is required"}
	}
	// "v" 접두사 허용
	s := strings.TrimPrefix(v, "v")
	// major.minor.patch 최소 형식 검사
	// pre-release (+build 메타) 허용
	// 간단한 정규식으로 검증
	semverRE := regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
		`(-([a-zA-Z0-9]+(\.[-a-zA-Z0-9]+)*))?` +
		`(\+[a-zA-Z0-9]+(\.[-a-zA-Z0-9]+)*)?$`)
	if !semverRE.MatchString(s) {
		return ErrInvalidManifest{Reason: "invalid semver: " + v}
	}
	return nil
}

// validateNoCredentialsInURI는 URI에 user:password@ 패턴이 없는지 검사한다.
// REQ-PL-013
func validateNoCredentialsInURI(rawURI string) error {
	u, err := url.Parse(rawURI)
	if err != nil {
		return nil // 파싱 실패는 다른 단계에서 처리
	}
	if u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			return ErrCredentialsInURI{URI: rawURI}
		}
		// username만 있어도 자격증명으로 간주
		if u.User.Username() != "" {
			return ErrCredentialsInURI{URI: rawURI}
		}
	}
	return nil
}
