package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// MemoryManager coordinates multiple MemoryProvider instances.
// Builtin is always first, at most one plugin allowed.
type MemoryManager struct {
	cfg        MemoryConfig
	providers  []MemoryProvider
	toolIndex  map[string]int // tool_name -> provider index
	mu         sync.RWMutex
	dispatcher *dispatcher
	logger     *zap.Logger
}

// New creates a new MemoryManager.
func New(cfg MemoryConfig, logger *zap.Logger) (*MemoryManager, error) {
	m := &MemoryManager{
		cfg:       cfg,
		providers: make([]MemoryProvider, 0, 2), // Builtin + at most 1 plugin
		toolIndex: make(map[string]int),
		logger:    logger,
	}
	m.dispatcher = newDispatcher(logger, m.providers)
	return m, nil
}

// RegisterBuiltin registers the builtin provider (must be first).
func (m *MemoryManager) RegisterBuiltin(p MemoryProvider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate name
	if !isValidProviderName(p.Name()) {
		return fmt.Errorf("%w: %q", ErrInvalidProviderName, p.Name())
	}

	// Builtin must be registered first (list must be empty)
	if len(m.providers) > 0 {
		return ErrBuiltinRequired
	}

	m.providers = append(m.providers, p)

	// Update dispatcher providers
	m.dispatcher.providers = m.providers

	// Index tools
	for idx, schema := range p.GetToolSchemas() {
		m.toolIndex[schema.Name] = idx
	}

	return nil
}

// RegisterPlugin registers an external plugin provider.
func (m *MemoryManager) RegisterPlugin(p MemoryProvider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Builtin must be registered first
	if len(m.providers) == 0 || m.providers[0].Name() != "builtin" {
		return ErrBuiltinRequired
	}

	// At most one plugin
	if len(m.providers) >= 2 {
		return ErrOnlyOnePluginAllowed
	}

	// Check name collision (case-insensitive) before validation
	for _, existing := range m.providers {
		if strings.EqualFold(existing.Name(), p.Name()) {
			return fmt.Errorf("%w: %q", ErrNameCollision, p.Name())
		}
	}

	// Validate name format
	if !isValidProviderName(p.Name()) {
		return fmt.Errorf("%w: %q", ErrInvalidProviderName, p.Name())
	}

	// Check tool name collision
	existingToolNames := make(map[string]bool)
	for _, existingProvider := range m.providers {
		for _, schema := range existingProvider.GetToolSchemas() {
			existingToolNames[schema.Name] = true
		}
	}

	for _, newSchema := range p.GetToolSchemas() {
		if existingToolNames[newSchema.Name] {
			return fmt.Errorf("%w: tool %q already registered", ErrToolNameCollision, newSchema.Name)
		}
	}

	m.providers = append(m.providers, p)

	// Update dispatcher providers
	m.dispatcher.providers = m.providers

	// Index tools
	pluginIdx := len(m.providers) - 1
	for _, schema := range p.GetToolSchemas() {
		m.toolIndex[schema.Name] = pluginIdx
	}

	return nil
}

// Initialize initializes all providers for a session (forward order).
//
// @MX:ANCHOR: Memory subsystem session-lifecycle entry point.
// @MX:REASON: Called by QueryEngine at every session start; failure here marks
// the provider as failed for the entire session via dispatcher.markInitFailed,
// which suppresses subsequent hooks until next Initialize. Changing forward
// dispatch order or removing the markInitFailed contract breaks AC-006/AC-019.
// @MX:SPEC: SPEC-GOOSE-MEMORY-001
func (m *MemoryManager) Initialize(ctx context.Context, sessionID string, sctx SessionContext) error {
	m.mu.RLock()
	providers := make([]MemoryProvider, len(m.providers))
	copy(providers, m.providers)
	m.mu.RUnlock()

	if len(providers) == 0 {
		return ErrBuiltinRequired
	}

	// Clear previous init state for this session
	m.dispatcher.clearInitState(sessionID)

	// Initialize in forward order
	var multiErr []error
	for _, p := range providers {
		if err := p.Initialize(sessionID, sctx); err != nil {
			multiErr = append(multiErr, err)
			// Mark this provider as failed for this session
			m.dispatcher.markInitFailed(sessionID, p.Name())
		}
	}

	if len(multiErr) > 0 {
		return fmt.Errorf("provider initialization errors: %v", multiErr)
	}

	return nil
}

// GetAllToolSchemas returns all tool schemas from all providers.
func (m *MemoryManager) GetAllToolSchemas() []ToolSchema {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var schemas []ToolSchema
	for _, p := range m.providers {
		schemas = append(schemas, p.GetToolSchemas()...)
	}
	return schemas
}

// OnTurnStart dispatches OnTurnStart to all providers with 50ms total budget.
//
// @MX:NOTE: 50ms total budget with 40ms per-provider timeout is contractual
// (AC-011). Providers exceeding the budget are aborted but not marked failed —
// next turn retries them. Do not raise the timeout without updating AC-011.
// @MX:SPEC: SPEC-GOOSE-MEMORY-001
func (m *MemoryManager) OnTurnStart(ctx context.Context, sessionID string, turn int, msg Message) {
	ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	m.mu.RLock()
	providers := make([]MemoryProvider, len(m.providers))
	copy(providers, m.providers)
	m.mu.RUnlock()

	for _, p := range providers {
		if !p.IsAvailable() {
			continue
		}

		// Skip if initialization failed for this session
		if m.dispatcher.didInitFail(sessionID, p.Name()) {
			continue
		}

		// 40ms per-provider timeout
		m.dispatcher.dispatchWithTimeout(ctx, p, 40*time.Millisecond, func(prov MemoryProvider) {
			prov.OnTurnStart(sessionID, turn, msg)
		})

		// Check if total budget exceeded
		if ctx.Err() != nil {
			return
		}
	}
}

// OnSessionEnd dispatches OnSessionEnd to all providers in reverse order (LIFO).
func (m *MemoryManager) OnSessionEnd(ctx context.Context, sessionID string, messages []Message) {
	m.mu.RLock()
	providers := make([]MemoryProvider, len(m.providers))
	copy(providers, m.providers)
	m.mu.RUnlock()

	// Dispatch in reverse order (LIFO)
	for i := len(providers) - 1; i >= 0; i-- {
		p := providers[i]
		if !p.IsAvailable() {
			continue
		}

		// Deep copy messages for each provider to prevent cross-provider retention (AC-020)
		messagesCopy := make([]Message, len(messages))
		for j, msg := range messages {
			messagesCopy[j] = Message{
				Role:    msg.Role,
				Content: msg.Content,
			}
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					m.logger.Error("provider panic in OnSessionEnd",
						zap.String("provider", p.Name()),
						zap.Any("panic", r))
				}
			}()
			p.OnSessionEnd(sessionID, messagesCopy)
		}()
	}
}

// OnPreCompress dispatches OnPreCompress to all providers and aggregates results.
func (m *MemoryManager) OnPreCompress(ctx context.Context, sessionID string, messages []Message) string {
	m.mu.RLock()
	providers := make([]MemoryProvider, len(m.providers))
	copy(providers, m.providers)
	m.mu.RUnlock()

	var results []string
	for _, p := range providers {
		if !p.IsAvailable() {
			continue
		}

		// Deep copy messages for each provider to prevent cross-provider retention (AC-020)
		messagesCopy := make([]Message, len(messages))
		for j, msg := range messages {
			messagesCopy[j] = Message{
				Role:    msg.Role,
				Content: msg.Content,
			}
		}

		hint := p.OnPreCompress(sessionID, messagesCopy)
		if hint != "" {
			results = append(results, hint)
		}
	}

	if len(results) == 0 {
		return ""
	}

	// Aggregate with blank line separator using strings.Builder
	var sb strings.Builder
	for i, r := range results {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(r)
	}

	return sb.String()
}

// HandleToolCall routes a tool call to the correct provider.
//
// @MX:ANCHOR: Tool routing invariant for the memory subsystem.
// @MX:REASON: toolIndex is built at registration time (RegisterBuiltin/Plugin)
// and is the single source of truth for tool ownership. AC-004 requires no two
// providers to share a tool name; AC-010 requires routing to the original owner
// even after plugin registration. Changing index semantics requires re-verifying
// both ACs.
// @MX:SPEC: SPEC-GOOSE-MEMORY-001
func (m *MemoryManager) HandleToolCall(ctx context.Context, toolName string, args json.RawMessage, tctx ToolContext) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providerIdx, ok := m.toolIndex[toolName]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}

	return m.providers[providerIdx].HandleToolCall(toolName, args, tctx)
}

// SystemPromptBlock aggregates SystemPromptBlock from all providers with "\n\n" separator (AC-009).
func (m *MemoryManager) SystemPromptBlock(sessionID string) string {
	m.mu.RLock()
	providers := make([]MemoryProvider, len(m.providers))
	copy(providers, m.providers)
	m.mu.RUnlock()

	var blocks []string
	for _, p := range providers {
		if !p.IsAvailable() {
			continue
		}

		block := p.SystemPromptBlock()
		if block != "" {
			blocks = append(blocks, block)
		}
	}

	if len(blocks) == 0 {
		return ""
	}

	// Aggregate with "\n\n" separator
	var sb strings.Builder
	for i, block := range blocks {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(block)
	}

	return sb.String()
}

// QueuePrefetch asynchronously prefetches data from all providers (AC-012).
// Launches goroutines with panic recovery.
func (m *MemoryManager) QueuePrefetch(ctx context.Context, query, sessionID string) {
	m.mu.RLock()
	providers := make([]MemoryProvider, len(m.providers))
	copy(providers, m.providers)
	m.mu.RUnlock()

	for _, p := range providers {
		if !p.IsAvailable() {
			continue
		}

		// Launch goroutine for async prefetch
		go func(provider MemoryProvider) {
			defer func() {
				if r := recover(); r != nil {
					m.logger.Error("provider panic in QueuePrefetch",
						zap.String("provider", provider.Name()),
						zap.Any("panic", r))
				}
			}()

			provider.QueuePrefetch(query, sessionID)
		}(p)
	}
}
