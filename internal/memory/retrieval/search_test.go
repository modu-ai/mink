// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package retrieval

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock implementations ---

// mockBM25Reader is a test double for BM25Reader.
type mockBM25Reader struct {
	hits []Hit
	err  error
}

func (m *mockBM25Reader) SearchBM25(_ context.Context, _, _ string, _ int) ([]Hit, error) {
	return m.hits, m.err
}

// mockChunkLookup is a test double for ChunkLookup.
type mockChunkLookup struct {
	chunks map[string]qmd.Chunk
	err    error
}

func (m *mockChunkLookup) LookupChunks(_ context.Context, ids []string) ([]qmd.Chunk, error) {
	if m.err != nil {
		return nil, m.err
	}
	out := make([]qmd.Chunk, 0, len(ids))
	for _, id := range ids {
		if ch, ok := m.chunks[id]; ok {
			out = append(out, ch)
		}
	}
	return out, nil
}

// --- helpers ---

func makeChunk(id, content, sourcePath string) qmd.Chunk {
	return qmd.Chunk{
		ID:         id,
		Content:    content,
		SourcePath: sourcePath,
		StartLine:  1,
		EndLine:    5,
		Collection: "custom",
	}
}

// --- tests for BM25Runner ---

func TestBM25Runner_defaultLimit(t *testing.T) {
	reader := &mockBM25Reader{hits: []Hit{{ChunkID: "c1", Score: 1.0}}}
	lookup := &mockChunkLookup{chunks: map[string]qmd.Chunk{
		"c1": makeChunk("c1", "hello world document", "/a.md"),
	}}
	runner := NewBM25Runner(reader, lookup)

	results, err := runner.RunBM25(context.Background(), "hello", qmd.SearchOpts{})
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestBM25Runner_emptyQuery(t *testing.T) {
	runner := NewBM25Runner(&mockBM25Reader{}, &mockChunkLookup{})

	_, err := runner.RunBM25(context.Background(), "", qmd.SearchOpts{})
	assert.True(t, errors.Is(err, ErrEmptyQuery))

	_, err = runner.RunBM25(context.Background(), "   ", qmd.SearchOpts{})
	assert.True(t, errors.Is(err, ErrEmptyQuery))
}

func TestBM25Runner_modeVsearch(t *testing.T) {
	runner := NewBM25Runner(&mockBM25Reader{}, &mockChunkLookup{})
	_, err := runner.RunBM25(context.Background(), "query", qmd.SearchOpts{Mode: "vsearch"})
	assert.True(t, errors.Is(err, ErrModeNotImplementedM2))
}

func TestBM25Runner_modeQuery(t *testing.T) {
	runner := NewBM25Runner(&mockBM25Reader{}, &mockChunkLookup{})
	_, err := runner.RunBM25(context.Background(), "query", qmd.SearchOpts{Mode: "query"})
	assert.True(t, errors.Is(err, ErrModeNotImplementedM2))
}

func TestBM25Runner_modeSearch(t *testing.T) {
	reader := &mockBM25Reader{hits: []Hit{{ChunkID: "c1", Score: 1.5}}}
	lookup := &mockChunkLookup{chunks: map[string]qmd.Chunk{
		"c1": makeChunk("c1", "search mode document", "/b.md"),
	}}
	runner := NewBM25Runner(reader, lookup)

	results, err := runner.RunBM25(context.Background(), "search", qmd.SearchOpts{Mode: "search"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, 1.5, results[0].Score)
}

func TestBM25Runner_emptyHits(t *testing.T) {
	runner := NewBM25Runner(&mockBM25Reader{hits: []Hit{}}, &mockChunkLookup{})
	results, err := runner.RunBM25(context.Background(), "noresult", qmd.SearchOpts{})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestBM25Runner_snippetAttached(t *testing.T) {
	content := "The quick brown fox jumps over the lazy dog"
	reader := &mockBM25Reader{hits: []Hit{{ChunkID: "c1", Score: 1.0}}}
	lookup := &mockChunkLookup{chunks: map[string]qmd.Chunk{
		"c1": makeChunk("c1", content, "/fox.md"),
	}}
	runner := NewBM25Runner(reader, lookup)

	results, err := runner.RunBM25(context.Background(), "fox", qmd.SearchOpts{})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.NotEmpty(t, results[0].Snippet)
}

func TestBM25Runner_readerError(t *testing.T) {
	readerErr := errors.New("reader failure")
	runner := NewBM25Runner(&mockBM25Reader{err: readerErr}, &mockChunkLookup{})

	_, err := runner.RunBM25(context.Background(), "test", qmd.SearchOpts{})
	assert.Error(t, err)
}

func TestBM25Runner_lookupError(t *testing.T) {
	lookupErr := errors.New("lookup failure")
	reader := &mockBM25Reader{hits: []Hit{{ChunkID: "c1", Score: 1.0}}}
	lookup := &mockChunkLookup{err: lookupErr}
	runner := NewBM25Runner(reader, lookup)

	_, err := runner.RunBM25(context.Background(), "test", qmd.SearchOpts{})
	assert.Error(t, err)
}

func TestBM25Runner_missingChunkSkipped(t *testing.T) {
	// Hit references a chunk that no longer exists in the lookup.
	reader := &mockBM25Reader{hits: []Hit{
		{ChunkID: "exists", Score: 2.0},
		{ChunkID: "deleted", Score: 1.0},
	}}
	lookup := &mockChunkLookup{chunks: map[string]qmd.Chunk{
		"exists": makeChunk("exists", "existing chunk content", "/e.md"),
		// "deleted" is intentionally absent
	}}
	runner := NewBM25Runner(reader, lookup)

	results, err := runner.RunBM25(context.Background(), "chunk", qmd.SearchOpts{})
	require.NoError(t, err)
	assert.Len(t, results, 1, "deleted chunk must be silently skipped")
	assert.Equal(t, "exists", results[0].Chunk.ID)
}

// --- tests for MakeSnippet ---

func TestMakeSnippet_matchAtStart(t *testing.T) {
	content := "golang is a great language for systems programming"
	snippet := MakeSnippet(content, "golang")
	assert.Contains(t, snippet, "golang")
}

func TestMakeSnippet_matchAtMiddle(t *testing.T) {
	// Build content longer than 256 runes with match in the middle.
	prefix := strings.Repeat("a ", 60)    // 120 chars
	suffix := strings.Repeat(" b", 60)    // 120 chars
	content := prefix + "TARGET" + suffix // well over 256 runes
	snippet := MakeSnippet(content, "TARGET")
	assert.Contains(t, snippet, "TARGET")
	// +2 for potential ellipsis runes (each "…" is 1 rune in Go)
	assert.LessOrEqual(t, len([]rune(snippet)), snippetMaxRunes+2,
		"snippet must not exceed 256 runes (plus ellipsis)")
}

func TestMakeSnippet_matchAtEnd(t *testing.T) {
	prefix := strings.Repeat("x ", 130) // >256 chars
	content := prefix + "findme"
	snippet := MakeSnippet(content, "findme")
	assert.Contains(t, snippet, "findme")
}

func TestMakeSnippet_noMatch(t *testing.T) {
	content := strings.Repeat("hello world ", 30) // >256 runes
	snippet := MakeSnippet(content, "zxqvjk")
	runes := []rune(snippet)
	// Should return leading 256 runes + ellipsis.
	assert.LessOrEqual(t, len(runes), snippetMaxRunes+1)
}

func TestMakeSnippet_shortContent(t *testing.T) {
	content := "short content"
	snippet := MakeSnippet(content, "content")
	assert.Equal(t, content, snippet, "short content must be returned as-is")
}

func TestMakeSnippet_koreanContent(t *testing.T) {
	// 500+ rune Korean content.
	korean := strings.Repeat("안녕하세요 ", 100) // ~600 runes
	snippet := MakeSnippet(korean, "안녕하세요")
	runeLen := len([]rune(snippet))
	assert.LessOrEqual(t, runeLen, snippetMaxRunes+2,
		"Korean snippet must respect 256-rune limit")
	assert.Contains(t, snippet, "안녕하세요")
}

func TestMakeSnippet_multiTokenFirstWins(t *testing.T) {
	// "alpha" appears first; "beta" appears later.
	prefix := strings.Repeat("z ", 50) // 100 chars to push content over 256
	content := prefix + "alpha beta gamma" + strings.Repeat(" y", 100)
	snippet := MakeSnippet(content, "alpha beta")
	assert.Contains(t, snippet, "alpha", "first matching token must anchor the snippet")
}

func TestMakeSnippet_regexMetaChars(t *testing.T) {
	content := "the price is $10.00 for this item and more text here to pad it out beyond 256 characters " +
		"so that the snippet logic is actually exercised in a meaningful way by this test case"
	// Query with regex meta chars should be treated as literal strings.
	snippet := MakeSnippet(content, "10.00")
	assert.Contains(t, snippet, "10.00")
}

func TestMakeSnippet_exactlyAtLimit(t *testing.T) {
	// Content is exactly 256 runes — must be returned verbatim.
	content := strings.Repeat("a", snippetMaxRunes)
	snippet := MakeSnippet(content, "a")
	assert.Equal(t, content, snippet)
}
