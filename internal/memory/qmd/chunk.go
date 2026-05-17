// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package qmd

import (
	"strings"
)

// ChunkOpts configures the chunking behaviour.
type ChunkOpts struct {
	// MaxTokens is the hard token cap per chunk.  A "token" is approximated as
	// a whitespace-delimited word.  Production-grade tokenization is out of
	// scope for M1.
	//
	// Default: 512
	MaxTokens int
}

// defaultMaxTokens is used when ChunkOpts.MaxTokens is zero.
const defaultMaxTokens = 512

// headingPrefixes lists the heading markers that trigger a chunk boundary.
// H1, H2, and H3 are considered boundary markers per REQ-MEM-012.
var headingPrefixes = []string{"# ", "## ", "### "}

// ChunkMarkdown splits markdown content into a slice of Chunk values.
//
// Segmentation order (REQ-MEM-012):
//  1. Heading boundary (H1/H2/H3 — lines starting with #, ##, or ###)
//  2. Blank-line (paragraph) boundary within each heading section
//  3. 512-token (whitespace-word) hard cap within a paragraph block
//
// The caller is responsible for filling in FileID, SourcePath, Collection,
// ChunkID, and CreatedAt.  PrevChunkID / NextChunkID are populated by
// LinkNeighbors.
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T1.3
// REQ:  REQ-MEM-012
func ChunkMarkdown(content string, opts ChunkOpts) []Chunk {
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = defaultMaxTokens
	}

	if strings.TrimSpace(content) == "" {
		return nil
	}

	lines := strings.Split(content, "\n")

	// Phase 1: split into heading sections.
	sections := splitByHeadings(lines)

	// Phase 2: within each section, split by blank lines (paragraphs).
	// Phase 3: apply the token hard cap within each paragraph block.
	var chunks []Chunk
	for _, sec := range sections {
		paraChunks := splitByParagraphs(sec, opts.MaxTokens)
		chunks = append(chunks, paraChunks...)
	}

	return chunks
}

// lineSection holds a contiguous range of lines that form a heading section.
type lineSection struct {
	lines     []string
	startLine int // 1-based
}

// splitByHeadings groups lines into heading sections.  Each H1/H2/H3 line
// starts a new section together with all following lines until the next
// heading of equal or lesser depth.
func splitByHeadings(lines []string) []lineSection {
	var sections []lineSection
	var current []string
	startLine := 1

	for i, line := range lines {
		if isHeading(line) && len(current) > 0 {
			// Flush the accumulated section before this heading.
			sections = append(sections, lineSection{lines: current, startLine: startLine})
			current = nil
			startLine = i + 1 // 1-based
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		sections = append(sections, lineSection{lines: current, startLine: startLine})
	}
	return sections
}

// isHeading reports whether line begins with an H1, H2, or H3 marker.
func isHeading(line string) bool {
	for _, prefix := range headingPrefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

// splitByParagraphs further subdivides a lineSection by blank lines and
// applies the token hard cap.
func splitByParagraphs(sec lineSection, maxTokens int) []Chunk {
	var chunks []Chunk

	// Collect non-empty paragraph blocks separated by blank lines.
	var paraLines []string
	paraStart := sec.startLine

	flushParagraph := func(end int) {
		if len(paraLines) == 0 {
			return
		}
		paraContent := strings.Join(paraLines, "\n")
		tokenChunks := splitByTokenCap(paraContent, paraStart, end, maxTokens)
		chunks = append(chunks, tokenChunks...)
		paraLines = nil
	}

	for i, line := range sec.lines {
		lineNum := sec.startLine + i
		if strings.TrimSpace(line) == "" {
			// Blank line → flush current paragraph.
			flushParagraph(lineNum - 1)
			paraStart = lineNum + 1
		} else {
			paraLines = append(paraLines, line)
		}
	}
	// Flush the last paragraph.
	flushParagraph(sec.startLine + len(sec.lines) - 1)

	return chunks
}

// splitByTokenCap splits a paragraph block into sub-chunks whose word count
// does not exceed maxTokens.
func splitByTokenCap(content string, startLine, endLine, maxTokens int) []Chunk {
	words := strings.Fields(content)
	if len(words) == 0 {
		return nil
	}

	// Fast path: the paragraph fits within the token cap.
	if len(words) <= maxTokens {
		return []Chunk{{
			StartLine: startLine,
			EndLine:   endLine,
			Content:   strings.TrimSpace(content),
		}}
	}

	// Slow path: the paragraph exceeds the cap; split into word-bounded
	// sub-chunks.  We do not attempt to track per-word line numbers here;
	// the sub-chunk line range is an approximation that uses proportional
	// distribution.
	totalLines := max(endLine-startLine+1, 1)

	var chunks []Chunk
	for offset := 0; offset < len(words); offset += maxTokens {
		end := min(offset+maxTokens, len(words))
		slice := words[offset:end]
		chunkContent := strings.Join(slice, " ")

		// Approximate line range proportionally.
		chunkStart := startLine + int(float64(offset)/float64(len(words))*float64(totalLines))
		chunkEnd := startLine + int(float64(end)/float64(len(words))*float64(totalLines)) - 1
		chunkEnd = max(chunkEnd, chunkStart)
		if end == len(words) {
			chunkEnd = endLine
		}

		chunks = append(chunks, Chunk{
			StartLine: chunkStart,
			EndLine:   chunkEnd,
			Content:   chunkContent,
		})
	}
	return chunks
}
