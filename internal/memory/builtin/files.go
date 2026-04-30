// Package builtin implements the BuiltinProvider with SQLite FTS5 backend.
package builtin

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/modu-ai/goose/internal/memory"
	"go.uber.org/zap"
)

const (
	// maxSystemPromptSize is the maximum size for SystemPromptBlock in bytes.
	maxSystemPromptSize = 8 * 1024 // 8KB
)

// SetUserMdPath sets the path to USER.md file.
// This is primarily used for testing.
func (b *BuiltinProvider) SetUserMdPath(path string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.userMdPath = path
}

// SetMemoryMdPath sets the path to MEMORY.md file.
// This is primarily used for testing.
func (b *BuiltinProvider) SetMemoryMdPath(path string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.memoryMdPath = path
}

// WriteUserMd always returns ErrUserMdReadOnly.
// USER.md is read-only and cannot be written through the provider (AC-014).
func (b *BuiltinProvider) WriteUserMd(content string) error {
	return memory.ErrUserMdReadOnly
}

// SystemPromptBlock reads USER.md (if exists) and recent MEMORY.md (truncated to 8KB).
// Returns concatenated content with blank line separator (AC-014).
func (b *BuiltinProvider) SystemPromptBlock() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	var result string

	// Read USER.md if it exists
	if b.userMdPath != "" {
		if userMdContent, err := os.ReadFile(b.userMdPath); err == nil {
			result += string(userMdContent)
		}
		// Ignore errors - file may not exist
	}

	// Read MEMORY.md if it exists
	if b.memoryMdPath != "" {
		if memoryMdContent, err := os.ReadFile(b.memoryMdPath); err == nil {
			content := string(memoryMdContent)

			// Calculate available space (account for potential "\n\n" separator)
			separatorLen := 0
			if result != "" && content != "" {
				separatorLen = 2 // "\n\n"
			}
			availableSpace := maxSystemPromptSize - len(result) - separatorLen

			// Truncate if too large to fit within 8KB
			if len(content) > availableSpace {
				// Take the last N bytes that fit
				content = content[len(content)-availableSpace:]
			}

			// Add blank line separator if we have USER.md content
			if result != "" && content != "" {
				result += "\n\n"
			}

			result += content
		}
		// Ignore errors - file may not exist
	}

	return result
}

// SyncTurn saves a fact from the turn to the database and appends to MEMORY.md.
// MEMORY.md format: "- [sessionID] userContent\n" (AC-015).
func (b *BuiltinProvider) SyncTurn(sessionID, userContent, assistantContent string) error {
	// First, save to database (existing logic)
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.db == nil {
		return ErrNotInitialized
	}

	// Generate a key from the content (use first 50 chars as key)
	key := userContent
	if len(key) > 50 {
		key = key[:50]
	}

	// Check current row count for FIFO eviction
	count, err := b.countFactsBySession(sessionID)
	if err != nil {
		return err
	}

	if count >= b.maxRows {
		// Delete oldest row (FIFO eviction)
		evictedID, err := b.deleteOldestFact(sessionID)
		if err != nil {
			return err
		}

		b.logger.Warn("fifo eviction",
			zap.String("provider", "builtin"),
			zap.String("event", "fifo_evict"),
			zap.Int64("evicted_id", evictedID),
			zap.String("session_id", sessionID),
		)
	}

	// Insert or update fact in database
	now := time.Now().Unix()
	if err := b.insertFact(sessionID, key, userContent, "user", now, now); err != nil {
		return err
	}

	// Append to MEMORY.md file
	if b.memoryMdPath != "" {
		if err := b.appendToMemoryMd(sessionID, userContent); err != nil {
			// Log error but don't fail - database write succeeded
			b.logger.Error("failed to append to MEMORY.md",
				zap.String("provider", "builtin"),
				zap.Error(err),
				zap.String("session_id", sessionID),
			)
		}
	}

	return nil
}

// appendToMemoryMd appends a formatted line to MEMORY.md.
// Creates directory with permission 0700 and file with permission 0600 if needed.
func (b *BuiltinProvider) appendToMemoryMd(sessionID, userContent string) error {
	// Ensure directory exists with permission 0700
	memoryDir := filepath.Dir(b.memoryMdPath)
	if err := os.MkdirAll(memoryDir, 0700); err != nil {
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	// Open file with append mode, create if not exists, permission 0600
	file, err := os.OpenFile(b.memoryMdPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open MEMORY.md: %w", err)
	}
	defer file.Close()

	// Format: "- [sessionID] userContent\n"
	line := fmt.Sprintf("- [%s] %s\n", sessionID, userContent)

	// Write line
	if _, err := file.WriteString(line); err != nil {
		return fmt.Errorf("failed to write to MEMORY.md: %w", err)
	}

	return nil
}
