// Package config의 source 추적 컴포넌트.
// SPEC-GOOSE-CONFIG-001 §6.5 Source Tracking
package config

// Source는 설정 필드의 출처를 나타내는 타입이다.
type Source string

const (
	// SourceDefault는 기본값에서 온 필드다.
	SourceDefault Source = "default"
	// SourceProject는 프로젝트 설정 파일(.goose/config.yaml)에서 온 필드다.
	SourceProject Source = "project"
	// SourceUser는 사용자 설정 파일($GOOSE_HOME/config.yaml)에서 온 필드다.
	SourceUser Source = "user"
	// SourceEnv는 환경변수에서 온 필드다.
	SourceEnv Source = "env"
	// SourceOverride는 LoadOptions.OverrideFiles에서 온 필드다.
	SourceOverride Source = "override"
)

// sourceMap은 dot-joined 경로를 키로 사용하는 소스 추적 맵이다.
// 예: "log.level" → SourceUser
type sourceMap map[string]Source

// set은 경로에 소스를 기록한다.
func (m sourceMap) set(path string, src Source) {
	m[path] = src
}

// get은 경로에 대한 소스를 반환한다. 등록되지 않으면 SourceDefault를 반환한다.
func (m sourceMap) get(path string) Source {
	if s, ok := m[path]; ok {
		return s
	}
	return SourceDefault
}

