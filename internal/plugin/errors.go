// Package plugin은 MINK의 Plugin Host를 구현한다.
// manifest.json 스키마 파싱, 3-tier discovery, MCPB 번들, atomic ClearThenRegister를 제공한다.
//
// SPEC: SPEC-GOOSE-PLUGIN-001 v0.1.0
package plugin

import "fmt"

// ErrInvalidManifest는 manifest.json 파싱 또는 스키마 검증 실패 시 반환된다.
// REQ-PL-001
type ErrInvalidManifest struct {
	Reason string
}

func (e ErrInvalidManifest) Error() string {
	return fmt.Sprintf("invalid plugin manifest: %s", e.Reason)
}

// ErrReservedPluginName은 예약된 이름의 플러그인 로드 시 반환된다.
// REQ-PL-016
type ErrReservedPluginName struct {
	Name string
}

func (e ErrReservedPluginName) Error() string {
	return fmt.Sprintf("plugin name %q is reserved and cannot be used", e.Name)
}

// ErrDuplicatePluginName은 동일 이름의 플러그인이 이미 등록된 경우 반환된다.
// REQ-PL-002
type ErrDuplicatePluginName struct {
	Name string
}

func (e ErrDuplicatePluginName) Error() string {
	return fmt.Sprintf("plugin name %q is already registered", e.Name)
}

// ErrUnknownHookEvent는 manifest의 hook 이벤트가 HOOK-001의 HookEventNames()에 없는 경우 반환된다.
// REQ-PL-004
type ErrUnknownHookEvent struct {
	Event string
}

func (e ErrUnknownHookEvent) Error() string {
	return fmt.Sprintf("unknown hook event: %q (not in HOOK-001 HookEventNames)", e.Event)
}

// ErrCredentialsInURI는 mcpServers URI에 자격증명이 포함된 경우 반환된다.
// REQ-PL-013
type ErrCredentialsInURI struct {
	URI string
}

func (e ErrCredentialsInURI) Error() string {
	return fmt.Sprintf("MCP server URI contains credentials: %q", e.URI)
}

// ErrZipSlip은 MCPB zip 파일에 zip slip 공격 경로가 포함된 경우 반환된다.
// REQ-PL-014
type ErrZipSlip struct {
	Path string
}

func (e ErrZipSlip) Error() string {
	return fmt.Sprintf("zip slip detected: path %q escapes extraction directory", e.Path)
}

// ErrMissingUserConfigVariable은 MCPB에서 required 변수가 plugins.yaml에 없는 경우 반환된다.
// REQ-PL-009
type ErrMissingUserConfigVariable struct {
	Name string
}

func (e ErrMissingUserConfigVariable) Error() string {
	return fmt.Sprintf("required user config variable %q is not set in plugins.yaml", e.Name)
}

// ErrNotImplemented는 미구현 기능 호출 시 반환된다.
// REQ-PL-018: marketplace fetch stub
var ErrNotImplemented = fmt.Errorf("not implemented")

// ErrPrimitiveLoad는 primitive 로드 실패 시 반환된다.
type ErrPrimitiveLoad struct {
	Primitive string
	Cause     error
}

func (e ErrPrimitiveLoad) Error() string {
	return fmt.Sprintf("failed to load primitive %s: %v", e.Primitive, e.Cause)
}

func (e ErrPrimitiveLoad) Unwrap() error { return e.Cause }
