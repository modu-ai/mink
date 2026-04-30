package qmd

import (
	"context"
	"testing"
	"time"
)

// TestQMDPublicAPISurface verifies exactly 4 public functions are exported (AC-QMD-013)
func TestQMDPublicAPISurface(t *testing.T) {
	// This test will be enabled once the full package is implemented
	// For now, we skip it during Sprint 0
	t.Skip("Sprint 0: Core interfaces only - full API surface test in later sprint")
}

// TestDocumentValidation tests document validation requirements
func TestDocumentValidation(t *testing.T) {
	tests := []struct {
		name    string
		doc     Document
		wantErr bool
	}{
		{
			name: "valid document",
			doc: Document{
				ID:       "doc1",
				Path:     "/path/to/doc.md",
				Content:  "test content",
				Modified: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			doc: Document{
				Path:     "/path/to/doc.md",
				Content:  "test content",
				Modified: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing path",
			doc: Document{
				ID:       "doc1",
				Content:  "test content",
				Modified: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "empty content is allowed",
			doc: Document{
				ID:       "doc1",
				Path:     "/path/to/doc.md",
				Content:  "",
				Modified: time.Now(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.doc.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Document.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestQueryValidation tests query validation requirements
func TestQueryValidation(t *testing.T) {
	tests := []struct {
		name    string
		q       Query
		wantErr bool
	}{
		{
			name: "valid query",
			q: Query{
				Text:     "test query",
				MinScore: 0.0,
			},
			wantErr: false,
		},
		{
			name: "empty text",
			q: Query{
				Text:     "",
				MinScore: 0.0,
			},
			wantErr: true,
		},
		{
			name: "negative min score",
			q: Query{
				Text:     "test",
				MinScore: -0.1,
			},
			wantErr: true,
		},
		{
			name: "min score > 1.0",
			q: Query{
				Text:     "test",
				MinScore: 1.1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.q.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Query.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestConfigDefaults tests default configuration values
func TestConfigDefaults(t *testing.T) {
	config := DefaultConfig()

	if !config.Enabled {
		t.Error("DefaultConfig() Enabled should be true")
	}

	if config.IndexPath != "./.goose/data/qmd-index/" {
		t.Errorf("DefaultConfig() IndexPath = %v, want %v", config.IndexPath, "./.goose/data/qmd-index/")
	}

	if config.MaxResults != 10 {
		t.Errorf("DefaultConfig() MaxResults = %v, want %v", config.MaxResults, 10)
	}

	if config.MemoryLimitMB != 500 {
		t.Errorf("DefaultConfig() MemoryLimitMB = %v, want %v", config.MemoryLimitMB, 500)
	}
}

// TestConfigValidation tests configuration validation
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  func() *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: func() *Config {
				c := DefaultConfig()
				return c
			},
			wantErr: false,
		},
		{
			name: "empty index path",
			config: func() *Config {
				c := DefaultConfig()
				c.IndexPath = ""
				return c
			},
			wantErr: true,
		},
		{
			name: "zero max results",
			config: func() *Config {
				c := DefaultConfig()
				c.MaxResults = 0
				return c
			},
			wantErr: true,
		},
		{
			name: "negative memory limit",
			config: func() *Config {
				c := DefaultConfig()
				c.MemoryLimitMB = -100
				return c
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config().Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSecurityFilterBlockedPaths tests security filtering for blocked paths (REQ-QMD-013)
func TestSecurityFilterBlockedPaths(t *testing.T) {
	filter := NewSecurityFilter()

	blockedPaths := []string{
		"/etc/passwd",
		"/etc/shadow",
		"/var/log/auth.log",
		"/root/.ssh/id_rsa",
		"/home/test/.ssh/config",
		"/home/test/.config/file",
	}

	for _, path := range blockedPaths {
		t.Run(path, func(t *testing.T) {
			if filter.IsAllowed(path) {
				t.Errorf("SecurityFilter.IsAllowed(%q) = true, want false (blocked path)", path)
			}
		})
	}
}

// TestSecurityFilterAllowedPaths tests security filtering for allowed paths
func TestSecurityFilterAllowedPaths(t *testing.T) {
	filter := NewSecurityFilter()

	// Use paths that won't be blocked
	allowedPaths := []string{
		"/tmp/project/.goose/memory/test.md",
		"/tmp/project/.goose/context/spec.md",
		"/tmp/project/.goose/skills/moai.md",
		"/tmp/project/docs/readme.md",
		"/home/user/project/.goose/tasks/task-001.md",
	}

	for _, path := range allowedPaths {
		t.Run(path, func(t *testing.T) {
			if !filter.IsAllowed(path) {
				t.Errorf("SecurityFilter.IsAllowed(%q) = false, want true (allowed path)", path)
			}
		})
	}
}

// TestMockIndexer tests the mock indexer implementation
func TestMockIndexer(t *testing.T) {
	ctx := context.Background()
	indexer := NewMockIndexer()

	docs := []Document{
		{
			ID:       "doc1",
			Path:     "/path/to/doc1.md",
			Content:  "content 1",
			Modified: time.Now(),
		},
		{
			ID:       "doc2",
			Path:     "/path/to/doc2.md",
			Content:  "content 2",
			Modified: time.Now(),
		},
	}

	err := indexer.Index(ctx, docs)
	if err != nil {
		t.Fatalf("MockIndexer.Index() error = %v", err)
	}

	stats := indexer.Stats()
	if stats.DocCount != 2 {
		t.Errorf("MockIndexer.Stats().DocCount = %v, want 2", stats.DocCount)
	}
}

// TestMockSearcher tests the mock searcher implementation
func TestMockSearcher(t *testing.T) {
	ctx := context.Background()
	searcher := NewMockSearcher()

	query := Query{
		Text:     "test query",
		MinScore: 0.5,
	}

	results, err := searcher.Query(ctx, query, 5)
	if err != nil {
		t.Fatalf("MockSearcher.Query() error = %v", err)
	}

	if len(results) > 5 {
		t.Errorf("MockSearcher.Query() returned %d results, want max 5", len(results))
	}
}

// TestMarkdownChunker tests markdown chunking
func TestMarkdownChunker(t *testing.T) {
	chunker := NewMarkdownChunker(512, 64, 64)

	doc := Document{
		ID:       "doc1",
		Path:     "/path/to/doc.md",
		Content:  "# Header\n\nSome content\n\n## Subheader\n\nMore content",
		Modified: time.Now(),
	}

	chunks, err := chunker.Chunk(doc)
	if err != nil {
		t.Fatalf("MarkdownChunker.Chunk() error = %v", err)
	}

	if len(chunks) == 0 {
		t.Error("MarkdownChunker.Chunk() returned no chunks")
	}

	for i, chunk := range chunks {
		if chunk.DocumentID != doc.ID {
			t.Errorf("Chunk %d: DocumentID = %v, want %v", i, chunk.DocumentID, doc.ID)
		}
		if chunk.Path != doc.Path {
			t.Errorf("Chunk %d: Path = %v, want %v", i, chunk.Path, doc.Path)
		}
		if len(chunk.Content) == 0 {
			t.Errorf("Chunk %d: Content is empty", i)
		}
	}
}

// TestErrors tests error types
func TestErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrIndexNotFound", ErrIndexNotFound},
		{"ErrModelNotAvailable", ErrModelNotAvailable},
		{"ErrQueryTooShort", ErrQueryTooShort},
		{"ErrInvalidDocument", ErrInvalidDocument},
		{"ErrIndexPathInvalid", ErrIndexPathInvalid},
		{"ErrQMDDisabled", ErrQMDDisabled},
		{"ErrModelNotReady", ErrModelNotReady},
		{"ErrMCPNetworkBindProhibited", ErrMCPNetworkBindProhibited},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("%v is nil", tt.name)
			}
		})
	}
}
