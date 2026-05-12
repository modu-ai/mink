// Package builtin tests for BuiltinProvider.
package builtin

import (
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/memory"
	"go.uber.org/zap"
)

// TestBuiltinProvider_FTS5Recall tests FTS5 full-text search (AC-013).
func TestBuiltinProvider_FTS5Recall(t *testing.T) {
	t.Parallel()

	// Create temporary database
	dbPath := t.TempDir() + "/test.db"
	logger := zap.NewNop()

	provider, err := NewBuiltin(dbPath, logger)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	// Initialize provider
	ctx := memory.SessionContext{
		HermesHome:   t.TempDir(),
		Platform:     "darwin",
		AgentContext: make(map[string]string),
	}

	if err := provider.Initialize("test-session", ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer provider.Close()

	// Insert test facts
	now := time.Now().Unix()
	testCases := []struct {
		key     string
		content string
		source  string
	}{
		{"key1", "The quick brown fox jumps over the lazy dog", "user"},
		{"key2", "Fast brown foxes are quick animals", "user"},
		{"key3", "Lazy dogs sleep all day", "user"},
	}

	for _, tc := range testCases {
		if err := provider.insertFact("test-session", tc.key, tc.content, tc.source, now, now); err != nil {
			t.Fatalf("insertFact failed: %v", err)
		}
	}

	// Test FTS5 search for "quick fox"
	result, err := provider.Prefetch("quick fox", "test-session")
	if err != nil {
		t.Fatalf("Prefetch failed: %v", err)
	}

	// Should return at least 2 results (key1 and key2 match "quick" and "fox")
	if len(result.Items) < 2 {
		t.Errorf("Expected at least 2 results, got %d", len(result.Items))
	}

	// Verify results contain expected content
	found := make(map[string]bool)
	for _, item := range result.Items {
		if item.Content == testCases[0].content || item.Content == testCases[1].content {
			found[item.Content] = true
		}
	}

	if !found[testCases[0].content] {
		t.Errorf("Expected to find content %q", testCases[0].content)
	}
	if !found[testCases[1].content] {
		t.Errorf("Expected to find content %q", testCases[1].content)
	}
}

// TestBuiltinProvider_SessionIsolation tests session isolation (AC-005).
func TestBuiltinProvider_SessionIsolation(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/test.db"
	logger := zap.NewNop()

	provider, err := NewBuiltin(dbPath, logger)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	ctx := memory.SessionContext{
		HermesHome:   t.TempDir(),
		Platform:     "darwin",
		AgentContext: make(map[string]string),
	}

	if err := provider.Initialize("session-a", ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer provider.Close()

	// Insert fact for session-a
	now := time.Now().Unix()
	if err := provider.insertFact("session-a", "key1", "Secret data for session A", "user", now, now); err != nil {
		t.Fatalf("insertFact failed for session-a: %v", err)
	}

	// Search from session-b should not find session-a's data
	result, err := provider.Prefetch("Secret", "session-b")
	if err != nil {
		t.Fatalf("Prefetch failed: %v", err)
	}

	if len(result.Items) != 0 {
		t.Errorf("Expected 0 results for different session, got %d", len(result.Items))
	}

	// Search from session-a should find the data
	result, err = provider.Prefetch("Secret", "session-a")
	if err != nil {
		t.Fatalf("Prefetch failed: %v", err)
	}

	if len(result.Items) != 1 {
		t.Errorf("Expected 1 result for same session, got %d", len(result.Items))
	}
}

// TestBuiltinProvider_FIFOEviction tests FIFO eviction when maxRows is reached (AC-023).
func TestBuiltinProvider_FIFOEviction(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/test.db"
	logger := zap.NewNop()

	provider, err := NewBuiltin(dbPath, logger)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	// Set maxRows to 3 for testing
	provider.SetMaxRows(3)

	ctx := memory.SessionContext{
		HermesHome:   t.TempDir(),
		Platform:     "darwin",
		AgentContext: make(map[string]string),
	}

	if err := provider.Initialize("test-session", ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer provider.Close()

	// Insert 4 facts (should trigger FIFO eviction on 4th)
	for i := 1; i <= 4; i++ {
		content := string(rune('A' + i - 1))
		if err := provider.SyncTurn("test-session", content, "assistant response"); err != nil {
			t.Fatalf("SyncTurn failed for fact %d: %v", i, err)
		}
	}

	// Verify only 3 facts remain
	count, err := provider.countFactsBySession("test-session")
	if err != nil {
		t.Fatalf("countFactsBySession failed: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 facts after FIFO eviction, got %d", count)
	}

	// Verify oldest fact was evicted (first one should be gone)
	result, err := provider.Prefetch("A", "test-session")
	if err != nil {
		t.Fatalf("Prefetch failed: %v", err)
	}

	if len(result.Items) != 0 {
		t.Errorf("Expected oldest fact 'A' to be evicted, but found %d results", len(result.Items))
	}
}

// TestBuiltinProvider_SchemaCreation tests schema creation on initialization.
func TestBuiltinProvider_SchemaCreation(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/test.db"
	logger := zap.NewNop()

	provider, err := NewBuiltin(dbPath, logger)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	ctx := memory.SessionContext{
		HermesHome:   t.TempDir(),
		Platform:     "darwin",
		AgentContext: make(map[string]string),
	}

	if err := provider.Initialize("test-session", ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer provider.Close()

	// Verify tables exist by attempting to query them
	rows, err := provider.db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		t.Fatalf("Query sqlite_master failed: %v", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		tables = append(tables, name)
	}

	// Should have facts table and facts_fts virtual table
	expectedTables := map[string]bool{
		"facts":     false,
		"facts_fts": false,
	}

	for _, table := range tables {
		if _, exists := expectedTables[table]; exists {
			expectedTables[table] = true
		}
	}

	for table, found := range expectedTables {
		if !found {
			t.Errorf("Expected table %q not found", table)
		}
	}
}

// TestBuiltinProvider_FuzzyMatch tests fuzzy matching via FTS5 porter stemmer.
func TestBuiltinProvider_FuzzyMatch(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/test.db"
	logger := zap.NewNop()

	provider, err := NewBuiltin(dbPath, logger)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	ctx := memory.SessionContext{
		HermesHome:   t.TempDir(),
		Platform:     "darwin",
		AgentContext: make(map[string]string),
	}

	if err := provider.Initialize("test-session", ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer provider.Close()

	// Insert fact with "running"
	now := time.Now().Unix()
	if err := provider.insertFact("test-session", "key1", "The user is running tests", "user", now, now); err != nil {
		t.Fatalf("insertFact failed: %v", err)
	}

	// Search for "run" (porter stemmer should match "running")
	result, err := provider.Prefetch("run", "test-session")
	if err != nil {
		t.Fatalf("Prefetch failed: %v", err)
	}

	if len(result.Items) == 0 {
		t.Error("Expected fuzzy match for 'run' to find 'running', got 0 results")
	}
}

// TestBuiltinProvider_Upsert tests upsert behavior (insert or update on conflict).
func TestBuiltinProvider_Upsert(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/test.db"
	logger := zap.NewNop()

	provider, err := NewBuiltin(dbPath, logger)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	ctx := memory.SessionContext{
		HermesHome:   t.TempDir(),
		Platform:     "darwin",
		AgentContext: make(map[string]string),
	}

	if err := provider.Initialize("test-session", ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer provider.Close()

	// Insert initial fact
	now := time.Now().Unix()
	if err := provider.insertFact("test-session", "key1", "Original content", "user", now, now); err != nil {
		t.Fatalf("insertFact failed: %v", err)
	}

	// Verify insertion
	count, err := provider.countFactsBySession("test-session")
	if err != nil {
		t.Fatalf("countFactsBySession failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected 1 fact after insert, got %d", count)
	}

	// Update same key (should upsert, not insert new row)
	updatedTime := now + 100
	if err := provider.insertFact("test-session", "key1", "Updated content", "user", now, updatedTime); err != nil {
		t.Fatalf("insertFact update failed: %v", err)
	}

	// Verify still only 1 fact
	count, err = provider.countFactsBySession("test-session")
	if err != nil {
		t.Fatalf("countFactsBySession failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected 1 fact after upsert, got %d", count)
	}

	// Verify content was updated
	result, err := provider.Prefetch("Updated", "test-session")
	if err != nil {
		t.Fatalf("Prefetch failed: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(result.Items))
	}
	if result.Items[0].Content != "Updated content" {
		t.Errorf("Expected updated content, got %q", result.Items[0].Content)
	}
}

// TestBuiltinProvider_ConcurrentAccess tests concurrent read access safety.
// Note: SQLite with default connection limits writes to single connection,
// so we test concurrent reads instead.
func TestBuiltinProvider_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/test.db"
	logger := zap.NewNop()

	provider, err := NewBuiltin(dbPath, logger)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	ctx := memory.SessionContext{
		HermesHome:   t.TempDir(),
		Platform:     "darwin",
		AgentContext: make(map[string]string),
	}

	if err := provider.Initialize("test-session", ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer provider.Close()

	// Insert test data first
	now := time.Now().Unix()
	for i := 0; i < 10; i++ {
		key := string(rune('A' + i))
		if err := provider.insertFact("test-session", key, "Content", "user", now, now); err != nil {
			t.Fatalf("insertFact failed: %v", err)
		}
	}

	// Launch concurrent read goroutines (Prefetch is thread-safe due to mutex)
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			defer func() { done <- true }()
			_, err := provider.Prefetch("Content", "test-session")
			if err != nil {
				t.Errorf("Concurrent Prefetch failed: %v", err)
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
