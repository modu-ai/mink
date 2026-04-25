package tools

// Scope는 tool의 coordinator/worker 가시성을 제어한다.
// REQ-TOOLS-012: CoordinatorMode=true 시 ScopeLeaderOnly tool은 숨겨진다.
type Scope int

const (
	// ScopeShared는 leader+worker 모두에게 노출된다 (기본값).
	ScopeShared Scope = iota
	// ScopeLeaderOnly는 coordinator 모드에서 숨겨진다.
	ScopeLeaderOnly
	// ScopeWorkerShareable는 coordinator 모드에서도 노출된다.
	ScopeWorkerShareable
)

// Source는 tool 출처를 나타낸다.
type Source int

const (
	// SourceBuiltin은 내장 tool임을 나타낸다.
	SourceBuiltin Source = iota
	// SourceMCP는 MCP server에서 채택된 tool임을 나타낸다.
	SourceMCP
	// SourcePlugin은 plugin에서 등록된 tool임을 나타낸다.
	SourcePlugin
)
