package memory

import (
	"encoding/json"
)

// MemoryProvider defines the interface for pluggable memory backends.
// It has 4 required methods and 9 optional methods.
// Optional methods have default no-op implementations via BaseProvider.
//
// @MX:ANCHOR: Public extension boundary for the memory subsystem.
// @MX:REASON: Implemented by BuiltinProvider, plugin.PluginProvider adapter,
// and any external plugin loaded via plugin.RegisterFactory. Adding a required
// method or changing an existing signature is a breaking change for all
// downstream implementers and must propagate to BaseProvider's no-op defaults.
// @MX:SPEC: SPEC-GOOSE-MEMORY-001
type MemoryProvider interface {
	// ===== Required Methods =====

	// Name returns a unique identifier for this provider.
	// Must match ^[a-z][a-z0-9_-]{0,31}$ and be case-insensitive unique.
	Name() string

	// IsAvailable returns whether this provider is ready to use.
	// Must not perform network I/O (config + credential presence only).
	IsAvailable() bool

	// Initialize sets up the provider for a new session.
	// Called once per session at session start.
	Initialize(sessionID string, ctx SessionContext) error

	// GetToolSchemas returns the list of tools this provider exposes.
	// Tool names must be unique across all providers.
	GetToolSchemas() []ToolSchema

	// ===== Optional Methods (no-op defaults via BaseProvider) =====

	// SystemPromptBlock returns static context to inject into system prompts.
	// Called by QueryEngine when building prompts.
	SystemPromptBlock() string

	// Prefetch recalls facts matching a query for the current session.
	// Synchronous recall operation.
	Prefetch(query string, sessionID string) (RecallResult, error)

	// QueuePrefetch asynchronously prefetches data for future queries.
	// Best-effort cache warming, no result returned.
	QueuePrefetch(query string, sessionID string)

	// SyncTurn saves turn context for a session.
	// Called after each turn completes.
	SyncTurn(sessionID, userContent, assistantContent string) error

	// HandleToolCall executes a tool call owned by this provider.
	// Returns JSON string response.
	HandleToolCall(toolName string, args json.RawMessage, ctx ToolContext) (string, error)

	// OnTurnStart is called at the beginning of each turn.
	// Dispatch timeout: 40ms per provider.
	OnTurnStart(sessionID string, turnNumber int, message Message)

	// OnSessionEnd is called when a session terminates.
	// Dispatch order: reverse (LIFO) for cleanup.
	OnSessionEnd(sessionID string, messages []Message)

	// OnPreCompress is called before boundary compaction.
	// Returns hints to prepend to compaction prompt.
	OnPreCompress(sessionID string, messages []Message) string

	// OnDelegation is called after a delegation completes.
	OnDelegation(sessionID, task, result string)
}

// BaseProvider provides no-op implementations of optional MemoryProvider methods.
// Embed this struct in concrete providers to avoid implementing all optional methods.
//
// Example:
//
//	type MyProvider struct {
//	    memory.BaseProvider
//	    // ... provider-specific fields
//	}
//
//	func (p *MyProvider) Name() string { return "myprovider" }
//	func (p *MyProvider) IsAvailable() bool { return true }
//	// ... implement required methods only
type BaseProvider struct{}

// SystemPromptBlock returns empty string (no-op).
func (BaseProvider) SystemPromptBlock() string {
	return ""
}

// Prefetch returns empty result (no-op).
func (BaseProvider) Prefetch(query, sessionID string) (RecallResult, error) {
	return RecallResult{}, nil
}

// QueuePrefetch does nothing (no-op).
func (BaseProvider) QueuePrefetch(query, sessionID string) {
}

// SyncTurn returns nil (no-op).
func (BaseProvider) SyncTurn(sessionID, userContent, assistantContent string) error {
	return nil
}

// HandleToolCall returns error indicating tool not implemented (no-op).
func (BaseProvider) HandleToolCall(toolName string, args json.RawMessage, ctx ToolContext) (string, error) {
	return "", &ErrToolNotHandled{ToolName: toolName}
}

// OnTurnStart does nothing (no-op).
func (BaseProvider) OnTurnStart(sessionID string, turnNumber int, message Message) {
}

// OnSessionEnd does nothing (no-op).
func (BaseProvider) OnSessionEnd(sessionID string, messages []Message) {
}

// OnPreCompress returns empty string (no-op).
func (BaseProvider) OnPreCompress(sessionID string, messages []Message) string {
	return ""
}

// OnDelegation does nothing (no-op).
func (BaseProvider) OnDelegation(sessionID, task, result string) {
}

// isValidProviderName validates provider name format: ^[a-z][a-z0-9_-]{0,31}$
func isValidProviderName(name string) bool {
	if len(name) == 0 || len(name) > 32 {
		return false
	}
	if name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for i := 1; i < len(name); i++ {
		c := name[i]
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// ErrToolNotHandled indicates that a tool call is not handled by this provider.
type ErrToolNotHandled struct {
	ToolName string
}

func (e *ErrToolNotHandled) Error() string {
	return "tool not handled: " + e.ToolName
}
