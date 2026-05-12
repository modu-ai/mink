// Package builtin implements the BuiltinProvider with SQLite FTS5 backend.
package builtin

import (
	"database/sql"
	"sync"

	_ "modernc.org/sqlite" // SQLite driver

	"github.com/modu-ai/mink/internal/memory"
	"go.uber.org/zap"
)

// BuiltinProvider is the built-in memory provider using SQLite FTS5.
// It implements memory.MemoryProvider interface.
type BuiltinProvider struct {
	// BaseProvider provides no-op implementations for optional methods.
	// memory.BaseProvider

	// Configuration
	dbPath  string // Path to SQLite database file
	maxRows int    // Maximum number of facts before FIFO eviction (default: 10000)
	logger  *zap.Logger

	// File paths
	userMdPath   string // Path to USER.md file (read-only)
	memoryMdPath string // Path to MEMORY.md file (append-only)

	// Runtime state
	db *sql.DB // SQLite connection (nil when not initialized)
	mu sync.Mutex
}

// NewBuiltin creates a new BuiltinProvider instance.
// The database is initialized on the first Initialize() call.
func NewBuiltin(dbPath string, logger *zap.Logger) (*BuiltinProvider, error) {
	if dbPath == "" {
		return nil, ErrDBPathRequired
	}

	return &BuiltinProvider{
		dbPath:  dbPath,
		maxRows: 10000, // Default from SPEC
		logger:  logger,
	}, nil
}

// SetMaxRows sets the maximum number of facts before FIFO eviction.
// This is primarily used for testing.
func (b *BuiltinProvider) SetMaxRows(maxRows int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.maxRows = maxRows
}

// Name returns the provider name "builtin".
func (b *BuiltinProvider) Name() string {
	return "builtin"
}

// IsAvailable returns true if the database connection is open.
// No I/O is performed (per REQ-MEMORY-004).
func (b *BuiltinProvider) IsAvailable() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.db != nil
}

// Initialize opens the database connection and creates the schema.
// Safe to call multiple times for the same session.
func (b *BuiltinProvider) Initialize(sessionID string, ctx memory.SessionContext) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Already initialized
	if b.db != nil {
		return nil
	}

	db, err := sql.Open("sqlite", b.dbPath)
	if err != nil {
		return err
	}

	b.db = db

	// Create schema
	if err := b.createSchema(); err != nil {
		b.db.Close()
		b.db = nil
		return err
	}

	return nil
}

// Close closes the database connection.
func (b *BuiltinProvider) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.db == nil {
		return nil
	}

	err := b.db.Close()
	b.db = nil
	return err
}

// Prefetch performs FTS5 full-text search for the given query.
func (b *BuiltinProvider) Prefetch(query, sessionID string) (memory.RecallResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.db == nil {
		return memory.RecallResult{}, ErrNotInitialized
	}

	items, err := b.searchFacts(query, sessionID, 10)
	if err != nil {
		return memory.RecallResult{}, err
	}

	return memory.RecallResult{Items: items}, nil
}

// SystemPromptBlock returns USER.md (if exists) + recent MEMORY.md (truncated to 8KB).
// Implemented in files.go.

// OnTurnStart is called at the beginning of each turn.
func (b *BuiltinProvider) OnTurnStart(sessionID string, turnNumber int, message memory.Message) {
	// No-op for builtin provider
}

// OnSessionEnd is called when a session terminates.
func (b *BuiltinProvider) OnSessionEnd(sessionID string, messages []memory.Message) {
	// No-op for builtin provider
}

// OnPreCompress is called before boundary compaction.
func (b *BuiltinProvider) OnPreCompress(sessionID string, messages []memory.Message) string {
	return "" // No-op for builtin provider
}

// OnDelegation is called after a delegation completes.
func (b *BuiltinProvider) OnDelegation(sessionID, task, result string) {
	// No-op for builtin provider
}

// QueuePrefetch asynchronously prefetches data for future queries.
func (b *BuiltinProvider) QueuePrefetch(query, sessionID string) {
	// No-op for builtin provider (synchronous only)
}
