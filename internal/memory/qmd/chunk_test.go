// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package qmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ----- ChunkMarkdown test cases (8 cases per T1.3) -----

// Case 1: pure prose (no headings) produces one chunk when under the cap.
func TestChunkMarkdown_pureProse(t *testing.T) {
	content := "This is a simple paragraph with just a few words.\nIt spans two lines."
	chunks := ChunkMarkdown(content, ChunkOpts{MaxTokens: 512})
	require.Len(t, chunks, 1)
	assert.Equal(t, 1, chunks[0].StartLine)
	assert.Contains(t, chunks[0].Content, "simple paragraph")
}

// Case 2: single H1 heading — heading line is included in the first chunk.
func TestChunkMarkdown_singleH1(t *testing.T) {
	content := "# Introduction\n\nHello world."
	chunks := ChunkMarkdown(content, ChunkOpts{MaxTokens: 512})
	require.NotEmpty(t, chunks)
	assert.Contains(t, chunks[0].Content, "# Introduction")
}

// Case 3: multiple headings produce separate chunks.
func TestChunkMarkdown_multipleHeadings(t *testing.T) {
	content := strings.Join([]string{
		"# Chapter 1",
		"Content of chapter one.",
		"",
		"## Section 1.1",
		"Sub-section content.",
		"",
		"# Chapter 2",
		"Content of chapter two.",
	}, "\n")
	chunks := ChunkMarkdown(content, ChunkOpts{MaxTokens: 512})
	// Expect at least 3 chunks (one per heading section, split by paragraphs).
	assert.GreaterOrEqual(t, len(chunks), 3)
}

// Case 4: paragraph with more than 512 whitespace-words is split.
func TestChunkMarkdown_over512TokenParagraph(t *testing.T) {
	// Build a 600-word paragraph.
	words := make([]string, 600)
	for i := range words {
		words[i] = "word"
	}
	content := strings.Join(words, " ")
	chunks := ChunkMarkdown(content, ChunkOpts{MaxTokens: 512})
	require.GreaterOrEqual(t, len(chunks), 2, "600-word paragraph must produce at least 2 chunks")
	for _, c := range chunks {
		wordCount := len(strings.Fields(c.Content))
		assert.LessOrEqual(t, wordCount, 512, "each chunk must not exceed 512 tokens")
	}
}

// Case 5: mixed Korean and English content is handled correctly.
func TestChunkMarkdown_mixedKoreanEnglish(t *testing.T) {
	content := "# 소개\n\n안녕하세요. This is a bilingual document.\n한국어와 English를 혼합합니다."
	chunks := ChunkMarkdown(content, ChunkOpts{MaxTokens: 512})
	require.NotEmpty(t, chunks)
	// Content should not be corrupted.
	full := ""
	for _, c := range chunks {
		full += c.Content
	}
	assert.Contains(t, full, "안녕하세요")
	assert.Contains(t, full, "English")
}

// Case 6: code fence is preserved as a single chunk when under the cap.
func TestChunkMarkdown_codeFencePreserved(t *testing.T) {
	content := "## Example\n\n```go\nfunc hello() {\n\tfmt.Println(\"hello\")\n}\n```"
	chunks := ChunkMarkdown(content, ChunkOpts{MaxTokens: 512})
	require.NotEmpty(t, chunks)
	// The code fence block must appear in some chunk.
	found := false
	for _, c := range chunks {
		if strings.Contains(c.Content, "```go") {
			found = true
			break
		}
	}
	assert.True(t, found, "code fence must be preserved in a chunk")
}

// Case 7: nested list items are treated as paragraph content.
func TestChunkMarkdown_nestedList(t *testing.T) {
	content := "# Shopping List\n\n- Apples\n  - Fuji\n  - Granny Smith\n- Bananas\n- Cherries"
	chunks := ChunkMarkdown(content, ChunkOpts{MaxTokens: 512})
	require.NotEmpty(t, chunks)
	found := false
	for _, c := range chunks {
		if strings.Contains(c.Content, "Apples") {
			found = true
			break
		}
	}
	assert.True(t, found, "list items must appear in a chunk")
}

// Case 8: empty input returns nil (not an empty slice or panic).
func TestChunkMarkdown_emptyInput(t *testing.T) {
	chunks := ChunkMarkdown("", ChunkOpts{MaxTokens: 512})
	assert.Nil(t, chunks)
}

// Case 8b: whitespace-only input also returns nil.
func TestChunkMarkdown_whitespaceOnlyInput(t *testing.T) {
	chunks := ChunkMarkdown("   \n\t\n  ", ChunkOpts{MaxTokens: 512})
	assert.Nil(t, chunks)
}

// Case 9: default MaxTokens is applied when opts.MaxTokens == 0.
func TestChunkMarkdown_defaultMaxTokens(t *testing.T) {
	words := make([]string, 600)
	for i := range words {
		words[i] = "token"
	}
	content := strings.Join(words, " ")
	chunks := ChunkMarkdown(content, ChunkOpts{}) // MaxTokens intentionally 0
	require.GreaterOrEqual(t, len(chunks), 2)
}

// ----- LinkNeighbors test cases (T1.5) -----

func TestLinkNeighbors_singleChunk(t *testing.T) {
	chunks := []Chunk{{ID: "aaa"}}
	result := LinkNeighbors(chunks)
	assert.Empty(t, result[0].PrevChunkID)
	assert.Empty(t, result[0].NextChunkID)
}

func TestLinkNeighbors_twoChunks(t *testing.T) {
	chunks := []Chunk{
		{ID: "aaa"},
		{ID: "bbb"},
	}
	result := LinkNeighbors(chunks)
	assert.Empty(t, result[0].PrevChunkID)
	assert.Equal(t, "bbb", result[0].NextChunkID)
	assert.Equal(t, "aaa", result[1].PrevChunkID)
	assert.Empty(t, result[1].NextChunkID)
}

func TestLinkNeighbors_threeChunks(t *testing.T) {
	chunks := []Chunk{
		{ID: "c1"},
		{ID: "c2"},
		{ID: "c3"},
	}
	result := LinkNeighbors(chunks)
	assert.Empty(t, result[0].PrevChunkID)
	assert.Equal(t, "c2", result[0].NextChunkID)

	assert.Equal(t, "c1", result[1].PrevChunkID)
	assert.Equal(t, "c3", result[1].NextChunkID)

	assert.Equal(t, "c2", result[2].PrevChunkID)
	assert.Empty(t, result[2].NextChunkID)
}

func TestLinkNeighbors_emptySlice(t *testing.T) {
	result := LinkNeighbors(nil)
	assert.Nil(t, result)
}
