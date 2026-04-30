// Package qmd provides embedded hybrid memory search for GOOSE.
//
// QMD (Quarto Markdown) implements a 3-stage hybrid search pipeline:
// BM25 full-text retrieval → vector similarity search → LLM reranking.
//
// This package defines the core public API interfaces. The actual CGO Rust
// integration is provided in subpackages and is opt-in via build tags.
package qmd

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// Document represents a markdown document to be indexed.
// @MX:NOTE: Document is the primary data structure for indexing (fan_in >= 3)
type Document struct {
	ID       string            // Unique document identifier
	Path     string            // File path (absolute or relative to project root)
	Content  string            // Full document content
	Metadata map[string]string // Frontmatter and other metadata
	Modified time.Time         // Last modification timestamp
}

// Validate checks if the document has required fields.
// Returns ErrInvalidDocument if ID or Path is missing.
func (d *Document) Validate() error {
	if d.ID == "" {
		return fmt.Errorf("%w: missing ID", ErrInvalidDocument)
	}
	if d.Path == "" {
		return fmt.Errorf("%w: missing Path", ErrInvalidDocument)
	}
	return nil
}

// Query represents a search query with optional filters.
type Query struct {
	Text     string            // Query text (required)
	Filters  map[string]string // Metadata filters (optional)
	MinScore float64           // Minimum relevance score (0.0 to 1.0)
}

// Validate checks if the query is valid.
// Returns ErrQueryTooShort if text is empty or score is out of range.
func (q *Query) Validate() error {
	if strings.TrimSpace(q.Text) == "" {
		return fmt.Errorf("%w: query text cannot be empty", ErrQueryTooShort)
	}
	if q.MinScore < 0.0 || q.MinScore > 1.0 {
		return fmt.Errorf("%w: MinScore must be between 0.0 and 1.0", ErrQueryTooShort)
	}
	return nil
}

// Result represents a single search result with relevance scores.
type Result struct {
	DocumentID string  // ID of the matching document
	Path       string  // Path to the document
	Content    string  // Snippet of matching content
	Score      float64 // Final relevance score (after rerank)
	Source     string  // Source stage: "bm25", "vector", or "rerank"
}

// IndexStats represents index statistics.
type IndexStats struct {
	DocCount    int       // Total number of indexed documents
	IndexSize   int64     // Index size in bytes
	LastUpdated time.Time // Last update timestamp
}

// Indexer defines the interface for document indexing operations.
// @MX:ANCHOR: Core indexing interface (expected fan_in >= 3)
type Indexer interface {
	// Index adds or updates documents in the search index.
	Index(ctx context.Context, docs []Document) error

	// Reindex rebuilds the index for all documents under the given path.
	Reindex(ctx context.Context, path string) error

	// Stats returns current index statistics.
	Stats() IndexStats
}

// Searcher defines the interface for search/query operations.
// @MX:ANCHOR: Core search interface (expected fan_in >= 3)
type Searcher interface {
	// Query performs a hybrid search and returns top-k results.
	Query(ctx context.Context, q Query, k int) ([]Result, error)
}

// Watcher defines the interface for file system watching.
// @MX:ANCHOR: Core watcher interface (expected fan_in >= 2)
type Watcher interface {
	// Watch starts watching the given path for changes.
	// Returns a stop function that can be called to stop watching.
	Watch(ctx context.Context, path string) (stop func(), err error)
}

// SecurityFilter defines the interface for path security filtering.
type SecurityFilter interface {
	// IsAllowed checks if a path is allowed to be indexed.
	IsAllowed(path string) bool
}

// DefaultSecurityFilter implements security filtering based on blocked paths.
// @MX:NOTE: Security filter prevents indexing of sensitive system paths
type DefaultSecurityFilter struct {
	blockedPatterns []string
	mu              sync.RWMutex
}

// NewSecurityFilter creates a new security filter with default blocked paths.
func NewSecurityFilter() *DefaultSecurityFilter {
	return &DefaultSecurityFilter{
		blockedPatterns: []string{
			"/etc",
			"/etc/*",
			"/etc/**",
			"/var/log",
			"/var/log/*",
			"/var/log/**",
			"**/.ssh",
			"**/.ssh/*",
			"**/.ssh/**",
			"**/.gnupg",
			"**/.gnupg/*",
			"**/.gnupg/**",
			"**/.config",
			"**/.config/*",
			"**/.config/**",
			"**/node_modules",
			"**/node_modules/**",
			"**/vendor",
			"**/vendor/**",
			"**/.git",
			"**/.git/**",
			"C:\\Windows\\System32",
			"C:\\Windows\\System32\\**",
			"/Windows/System32",
			"/Windows/System32/**",
			"/Program Files",
			"/Program Files/**",
		},
	}
}

// IsAllowed checks if a path matches any blocked pattern.
// Normalizes the path and checks for path traversal attempts.
func (f *DefaultSecurityFilter) IsAllowed(path string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Clean the path to resolve any . or .. components
	cleanPath := filepath.Clean(path)

	// Convert to absolute path for consistent matching
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return false
	}

	// Check for path traversal attempts after normalization
	if containsPathTraversal(absPath) {
		return false
	}

	for _, pattern := range f.blockedPatterns {
		// Check if the pattern matches the path or any parent directory
		if f.matchesPattern(absPath, pattern) {
			return false
		}
	}

	return true
}

// containsPathTraversal checks if a path contains .. components after cleaning.
// This is a defense-in-depth check to catch potential path traversal attempts.
func containsPathTraversal(path string) bool {
	// After filepath.Clean, any remaining .. indicates traversal outside root
	// Check if the cleaned path still contains .. as a path component
	return slices.Contains(strings.Split(filepath.ToSlash(path), "/"), "..")
}

// matchesPattern checks if a path matches a blocked pattern.
func (f *DefaultSecurityFilter) matchesPattern(path, pattern string) bool {
	// Direct match
	if matched, err := filepath.Match(pattern, path); err == nil && matched {
		return true
	}

	// Special handling for **/.ssh/** style patterns
	if strings.Contains(pattern, "**/") && strings.Contains(pattern, "/.") {
		// Extract the hidden directory name
		parts := strings.Split(pattern, "/.")
		if len(parts) > 1 {
			hiddenDir := "." + strings.Split(parts[1], "/")[0]
			// Check if any path component is this hidden directory
			for part := range strings.SplitSeq(path, "/") {
				if part == hiddenDir || strings.HasPrefix(part, hiddenDir+"/") {
					return true
				}
			}
		}
	}

	// Check if any parent directory matches
	checkPath := path
	for {
		if matched, err := filepath.Match(pattern, checkPath); err == nil && matched {
			return true
		}
		parent := filepath.Dir(checkPath)
		if parent == checkPath {
			break
		}
		checkPath = parent
	}

	// Check if path starts with pattern prefix (for ** patterns)
	if suffix, ok := strings.CutPrefix(pattern, "**/"); ok {
		// Check if any component matches the suffix
		for part := range strings.SplitSeq(path, "/") {
			if matched, err := filepath.Match(suffix, part); err == nil && matched {
				return true
			}
		}
	}

	return false
}

// MockIndexer provides an in-memory implementation for testing.
// @MX:NOTE: Mock indexer enables TDD without CGO dependency
type MockIndexer struct {
	docs map[string]Document
	mu   sync.RWMutex
}

// NewMockIndexer creates a new mock indexer.
func NewMockIndexer() *MockIndexer {
	return &MockIndexer{
		docs: make(map[string]Document),
	}
}

// Index adds documents to the mock index.
func (m *MockIndexer) Index(ctx context.Context, docs []Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, doc := range docs {
		if err := doc.Validate(); err != nil {
			return err
		}
		m.docs[doc.ID] = doc
	}
	return nil
}

// Reindex is a no-op for the mock indexer.
func (m *MockIndexer) Reindex(ctx context.Context, path string) error {
	return nil
}

// Stats returns current index statistics.
func (m *MockIndexer) Stats() IndexStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return IndexStats{
		DocCount: len(m.docs),
	}
}

// MockSearcher provides an in-memory search implementation for testing.
type MockSearcher struct{}

// NewMockSearcher creates a new mock searcher.
func NewMockSearcher() *MockSearcher {
	return &MockSearcher{}
}

// Query performs a mock search, returning empty results.
func (m *MockSearcher) Query(ctx context.Context, q Query, k int) ([]Result, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}
	return []Result{}, nil
}

// MockWatcher provides a mock file watcher for testing.
type MockWatcher struct{}

// NewMockWatcher creates a new mock watcher.
func NewMockWatcher() *MockWatcher {
	return &MockWatcher{}
}

// Watch returns a no-op stop function for the mock watcher.
func (m *MockWatcher) Watch(ctx context.Context, path string) (stop func(), err error) {
	return func() {}, nil
}
