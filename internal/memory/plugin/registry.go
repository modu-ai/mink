// Package plugin provides the plugin adapter and factory registry for external memory providers.
package plugin

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/modu-ai/goose/internal/memory"
)

// FactoryFunc is a factory function that creates a MemoryProvider instance.
// The config parameter is plugin-specific configuration (e.g., API endpoint, auth token).
type FactoryFunc func(config any) (memory.MemoryProvider, error)

var (
	// registry holds the global factory registry.
	registry = make(map[string]FactoryFunc)

	// mu protects concurrent access to the registry.
	mu sync.RWMutex
)

// RegisterFactory registers a plugin factory function under the given name.
// Panics if a factory with the same name is already registered (safety check).
func RegisterFactory(name string, fn FactoryFunc) {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("plugin factory %q already registered", name))
	}

	registry[name] = fn
}

// Lookup looks up a plugin factory by name and creates a new instance.
// Returns memory.ErrUnknownPlugin if the name is not registered (AC-021).
func Lookup(name string, config any) (memory.MemoryProvider, error) {
	mu.RLock()
	factory, exists := registry[name]
	mu.RUnlock()

	if !exists {
		return nil, memory.ErrUnknownPlugin
	}

	return factory(config)
}

// PluginProvider is an adapter that wraps an external provider to ensure
// it satisfies the MemoryProvider interface.
//
// This adapter is useful when external plugins implement a subset of
// MemoryProvider methods but not all. The adapter provides default
// implementations for missing optional methods.
//
// However, for the initial implementation, we simply use the external
// provider directly since it must implement the full interface.
// This adapter is kept for future extensibility.
type PluginProvider struct {
	provider memory.MemoryProvider
}

// NewPluginProvider creates a new PluginProvider adapter.
func NewPluginProvider(provider memory.MemoryProvider) *PluginProvider {
	return &PluginProvider{
		provider: provider,
	}
}

// Name returns the provider name.
func (p *PluginProvider) Name() string {
	return p.provider.Name()
}

// IsAvailable checks if the provider is available.
func (p *PluginProvider) IsAvailable() bool {
	return p.provider.IsAvailable()
}

// Initialize initializes the provider for a session.
func (p *PluginProvider) Initialize(sessionID string, ctx memory.SessionContext) error {
	return p.provider.Initialize(sessionID, ctx)
}

// GetToolSchemas returns the tool schemas provided by this plugin.
func (p *PluginProvider) GetToolSchemas() []memory.ToolSchema {
	return p.provider.GetToolSchemas()
}

// HandleToolCall executes a tool call owned by this plugin.
func (p *PluginProvider) HandleToolCall(toolName string, args json.RawMessage, ctx memory.ToolContext) (string, error) {
	return p.provider.HandleToolCall(toolName, args, ctx)
}

// Prefetch performs a prefetch query (optional method).
func (p *PluginProvider) Prefetch(query, sessionID string) (memory.RecallResult, error) {
	// Try to call Prefetch if implemented
	type prefetchProvider interface {
		Prefetch(query, sessionID string) (memory.RecallResult, error)
	}

	if pf, ok := p.provider.(prefetchProvider); ok {
		return pf.Prefetch(query, sessionID)
	}

	// Default: return empty result
	return memory.RecallResult{}, nil
}

// SyncTurn saves a turn to memory (optional method).
func (p *PluginProvider) SyncTurn(sessionID, userContent, assistantContent string) error {
	// Try to call SyncTurn if implemented
	type syncTurnProvider interface {
		SyncTurn(sessionID, userContent, assistantContent string) error
	}

	if st, ok := p.provider.(syncTurnProvider); ok {
		return st.SyncTurn(sessionID, userContent, assistantContent)
	}

	// Default: no-op
	return nil
}

// SystemPromptBlock returns provider-specific system prompt content (optional method).
func (p *PluginProvider) SystemPromptBlock() string {
	// Try to call SystemPromptBlock if implemented
	type systemPromptProvider interface {
		SystemPromptBlock() string
	}

	if sp, ok := p.provider.(systemPromptProvider); ok {
		return sp.SystemPromptBlock()
	}

	// Default: empty string
	return ""
}

// OnTurnStart is called at the beginning of each turn (optional method).
func (p *PluginProvider) OnTurnStart(sessionID string, turnNumber int, message memory.Message) {
	// Try to call OnTurnStart if implemented
	type onTurnStartProvider interface {
		OnTurnStart(sessionID string, turnNumber int, message memory.Message)
	}

	if ots, ok := p.provider.(onTurnStartProvider); ok {
		ots.OnTurnStart(sessionID, turnNumber, message)
	}
	// Default: no-op
}

// OnSessionEnd is called when a session terminates (optional method).
func (p *PluginProvider) OnSessionEnd(sessionID string, messages []memory.Message) {
	// Try to call OnSessionEnd if implemented
	type onSessionEndProvider interface {
		OnSessionEnd(sessionID string, messages []memory.Message)
	}

	if ose, ok := p.provider.(onSessionEndProvider); ok {
		ose.OnSessionEnd(sessionID, messages)
	}
	// Default: no-op
}

// OnPreCompress is called before boundary compaction (optional method).
func (p *PluginProvider) OnPreCompress(sessionID string, messages []memory.Message) string {
	// Try to call OnPreCompress if implemented
	type onPreCompressProvider interface {
		OnPreCompress(sessionID string, messages []memory.Message) string
	}

	if opc, ok := p.provider.(onPreCompressProvider); ok {
		return opc.OnPreCompress(sessionID, messages)
	}

	// Default: empty string
	return ""
}

// OnDelegation is called after a delegation completes (optional method).
func (p *PluginProvider) OnDelegation(sessionID, task, result string) {
	// Try to call OnDelegation if implemented
	type onDelegationProvider interface {
		OnDelegation(sessionID, task, result string)
	}

	if od, ok := p.provider.(onDelegationProvider); ok {
		od.OnDelegation(sessionID, task, result)
	}
	// Default: no-op
}

// QueuePrefetch asynchronously prefetches data (optional method).
func (p *PluginProvider) QueuePrefetch(query, sessionID string) {
	// Try to call QueuePrefetch if implemented
	type queuePrefetchProvider interface {
		QueuePrefetch(query, sessionID string)
	}

	if qp, ok := p.provider.(queuePrefetchProvider); ok {
		qp.QueuePrefetch(query, sessionID)
	}
	// Default: no-op
}
