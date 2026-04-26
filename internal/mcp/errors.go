package mcp

import "errors"

// MCP 에러 변수들.
// REQ-MCP-001, REQ-MCP-003, REQ-MCP-007..022

var (
	// ErrDuplicateMCPToolName은 단일 서버 내에서 동일한 tool 이름이 두 번 노출될 때 반환된다.
	// REQ-MCP-001
	ErrDuplicateMCPToolName = errors.New("duplicate MCP tool name within server")

	// ErrTransportReset은 재연결 5회 시도가 모두 실패했을 때 반환된다.
	// REQ-MCP-009
	ErrTransportReset = errors.New("transport reset: reconnect attempts exhausted")

	// ErrReauthRequired는 token refresh가 invalid_grant로 실패했을 때 반환된다.
	// REQ-MCP-008
	ErrReauthRequired = errors.New("reauthentication required: refresh token invalid")

	// ErrAuthFlowTimeout은 OAuth 플로우가 60초 내에 완료되지 않았을 때 반환된다.
	// REQ-MCP-012
	ErrAuthFlowTimeout = errors.New("OAuth auth flow timeout (60s)")

	// ErrUnsupportedProtocolVersion은 서버가 지원하지 않는 프로토콜 버전을 반환할 때 사용된다.
	// REQ-MCP-018
	ErrUnsupportedProtocolVersion = errors.New("unsupported MCP protocol version")

	// ErrCapabilityNotSupported는 서버가 해당 capability를 선언하지 않았을 때 반환된다.
	// REQ-MCP-021
	ErrCapabilityNotSupported = errors.New("server capability not supported")

	// ErrRequestTimeout은 요청이 cfg.RequestTimeout 내에 완료되지 않았을 때 반환된다.
	// REQ-MCP-022
	ErrRequestTimeout = errors.New("MCP request timeout")

	// ErrToolNotFound는 CallTool에서 tool이 캐시에 없을 때 반환된다.
	ErrToolNotFound = errors.New("MCP tool not found")

	// ErrTransportClosed는 이미 닫힌 transport에 요청할 때 반환된다.
	ErrTransportClosed = errors.New("transport is closed")

	// ErrTLSValidation은 TLS 검증 실패 시 반환된다.
	// REQ-MCP-015
	ErrTLSValidation = errors.New("TLS certificate validation failed")

	// ErrReservedToolName은 '/', ':', '__'를 포함하는 tool 이름에 대해 반환된다.
	// REQ-MCP-016
	ErrReservedToolName = errors.New("tool name contains reserved characters")

	// ErrOAuthStateMismatch는 OAuth 콜백의 state가 불일치할 때 반환된다.
	// REQ-MCP-017
	ErrOAuthStateMismatch = errors.New("OAuth state parameter mismatch")

	// ErrCredentialFilePermissions는 credential 파일 mode가 0600을 초과할 때 반환된다.
	// REQ-MCP-003
	ErrCredentialFilePermissions = errors.New("credential file mode exceeds 0600")

	// ErrSessionNotConnected는 Connected 상태가 아닌 세션에 연산을 시도할 때 반환된다.
	ErrSessionNotConnected = errors.New("session is not in Connected state")
)
