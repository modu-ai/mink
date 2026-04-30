package memory

import (
	"encoding/json"
	"time"
)

// SessionContext contains session initialization parameters passed to providers.
type SessionContext struct {
	HermesHome    string            // Legacy: Hermes home directory (GOOSE_HOME equivalent)
	Platform      string            // "darwin" | "linux" | "windows"
	AgentContext  map[string]string // Context from QueryEngine (user_id, preferences, etc.)
	AgentIdentity string            // Teammate mode identifier
}

// RecallResult contains recalled items from a provider.
type RecallResult struct {
	Items       []RecallItem // Recalled facts
	TotalTokens int          // Total token count of all items
}

// RecallItem represents a single recalled fact.
type RecallItem struct {
	Content   string    // Fact content
	Source    string    // Source identifier (e.g., "builtin:sqlite", "honcho:endpoint")
	Score     float64   // Relevance score [0, 1]
	SessionID string    // Session that owns this fact
	Timestamp time.Time // When the fact was created
}

// ToolSchema represents a tool that a provider exposes to QueryEngine.
type ToolSchema struct {
	Name        string          // Tool name (must be unique across all providers)
	Description string          // Human-readable description
	Parameters  json.RawMessage // JSON Schema for parameters
	Owner       string          // Provider name that owns this tool (internal field)
}

// ToolContext contains context for tool calls.
type ToolContext struct {
	SessionID  string // Current session ID
	TurnNumber int    // Current turn number
}

// Message represents a single message in a conversation.
type Message struct {
	Role    string // "user" | "assistant" | "system" | "tool"
	Content string // Message content
}
