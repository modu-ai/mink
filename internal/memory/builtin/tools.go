// Package builtin implements the BuiltinProvider with SQLite FTS5 backend.
package builtin

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/modu-ai/goose/internal/memory"
	"go.uber.org/zap"
)

// Tool schema definitions as JSON Schema (RFC 7800).
const (
	// memoryRecallSchema is the JSON Schema for memory_recall tool.
	memoryRecallSchema = `{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Search query for full-text search"
			},
			"limit": {
				"type": "integer",
				"description": "Maximum number of results to return (default: 10)",
				"minimum": 1,
				"maximum": 100
			}
		},
		"required": ["query"]
	}`

	// memorySaveSchema is the JSON Schema for memory_save tool.
	memorySaveSchema = `{
		"type": "object",
		"properties": {
			"key": {
				"type": "string",
				"description": "Unique key for this fact (first 50 chars of content if not provided)"
			},
			"content": {
				"type": "string",
				"description": "Fact content to save"
			},
			"source": {
				"type": "string",
				"description": "Source identifier (e.g., 'user', 'assistant', 'tool')"
			}
		},
		"required": ["content"]
	}`
)

// GetToolSchemas returns the tool schemas for memory_recall and memory_save.
func (b *BuiltinProvider) GetToolSchemas() []memory.ToolSchema {
	return []memory.ToolSchema{
		{
			Name:        "memory_recall",
			Description: "Recall facts by keyword search using FTS5 full-text search",
			Parameters:  json.RawMessage(memoryRecallSchema),
			Owner:       "builtin",
		},
		{
			Name:        "memory_save",
			Description: "Save a fact for future recall",
			Parameters:  json.RawMessage(memorySaveSchema),
			Owner:       "builtin",
		},
	}
}

// HandleToolCall executes a tool call owned by this provider.
func (b *BuiltinProvider) HandleToolCall(toolName string, args json.RawMessage, ctx memory.ToolContext) (string, error) {
	switch toolName {
	case "memory_recall":
		return b.handleRecall(args, ctx)
	case "memory_save":
		return b.handleSave(args, ctx)
	default:
		return "", &memory.ErrToolNotHandled{ToolName: toolName}
	}
}

// handleRecall processes memory_recall tool calls.
func (b *BuiltinProvider) handleRecall(args json.RawMessage, ctx memory.ToolContext) (string, error) {
	// Parse arguments
	var params struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("failed to parse recall arguments: %w", err)
	}

	// Set default limit
	if params.Limit <= 0 {
		params.Limit = 10
	}

	// Perform prefetch
	result, err := b.Prefetch(params.Query, ctx.SessionID)
	if err != nil {
		return "", fmt.Errorf("recall failed: %w", err)
	}

	// Format result as JSON
	response := map[string]interface{}{
		"success": true,
		"query":   params.Query,
		"count":   len(result.Items),
		"items":   result.Items,
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal recall result: %w", err)
	}

	return string(jsonResponse), nil
}

// handleSave processes memory_save tool calls.
func (b *BuiltinProvider) handleSave(args json.RawMessage, ctx memory.ToolContext) (string, error) {
	// Parse arguments
	var params struct {
		Key     string `json:"key"`
		Content string `json:"content"`
		Source  string `json:"source"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("failed to parse save arguments: %w", err)
	}

	// Validate required fields
	if params.Content == "" {
		return "", fmt.Errorf("content is required")
	}

	// Set default key if not provided
	if params.Key == "" {
		params.Key = params.Content
		if len(params.Key) > 50 {
			params.Key = params.Key[:50]
		}
	}

	// Set default source if not provided
	if params.Source == "" {
		params.Source = "tool"
	}

	// Save to database via SyncTurn
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.db == nil {
		return "", ErrNotInitialized
	}

	// Check current row count for FIFO eviction
	count, err := b.countFactsBySession(ctx.SessionID)
	if err != nil {
		return "", fmt.Errorf("failed to check row count: %w", err)
	}

	if count >= b.maxRows {
		// Delete oldest row (FIFO eviction)
		evictedID, err := b.deleteOldestFact(ctx.SessionID)
		if err != nil {
			return "", fmt.Errorf("failed to evict oldest fact: %w", err)
		}

		b.logger.Warn("fifo eviction",
			zap.String("provider", "builtin"),
			zap.String("event", "fifo_evict"),
			zap.Int64("evicted_id", evictedID),
			zap.String("session_id", ctx.SessionID),
		)
	}

	// Insert fact
	now := time.Now().Unix()
	if err := b.insertFact(ctx.SessionID, params.Key, params.Content, params.Source, now, now); err != nil {
		return "", fmt.Errorf("failed to save fact: %w", err)
	}

	// Format success response
	response := map[string]interface{}{
		"success": true,
		"key":     params.Key,
		"source":  params.Source,
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal save result: %w", err)
	}

	return string(jsonResponse), nil
}
