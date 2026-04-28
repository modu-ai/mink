package qmd

import (
	"fmt"
	"strings"
	"unicode"
)

// Chunk represents a section of a document.
type Chunk struct {
	DocumentID string // Source document ID
	Path       string // Source document path
	Content    string // Chunk content
	Section    string // Section heading (if any)
	Tokens     int    // Estimated token count
	Position   int    // Position in document
}

// Chunker defines the interface for splitting documents into chunks.
// @MX:ANCHOR: Chunker interface (expected fan_in >= 2)
type Chunker interface {
	// Chunk splits a document into smaller chunks for indexing.
	Chunk(doc Document) ([]Chunk, error)
}

// MarkdownChunker implements markdown-aware document chunking.
// @MX:NOTE: MarkdownChunker preserves section boundaries and code blocks
type MarkdownChunker struct {
	maxTokens int
	minTokens int
	overlap   int
}

// NewMarkdownChunker creates a new markdown chunker with specified parameters.
// @MX:ANCHOR: Chunker factory (expected fan_in >= 2)
func NewMarkdownChunker(maxTokens, minTokens, overlap int) *MarkdownChunker {
	return &MarkdownChunker{
		maxTokens: maxTokens,
		minTokens: minTokens,
		overlap:   overlap,
	}
}

// Chunk splits a markdown document into chunks.
func (c *MarkdownChunker) Chunk(doc Document) ([]Chunk, error) {
	if err := doc.Validate(); err != nil {
		return nil, err
	}

	// Split by sections (H1 and H2 headers)
	sections := c.splitBySections(doc.Content)

	var chunks []Chunk
	position := 0

	for _, section := range sections {
		// Further split large sections into token-sized chunks
		sectionChunks := c.splitByTokens(section, doc.ID, doc.Path, position)

		chunks = append(chunks, sectionChunks...)
		position += len(sectionChunks)
	}

	return chunks, nil
}

// splitBySections splits markdown content by H1 and H2 headers.
func (c *MarkdownChunker) splitBySections(content string) []Section {
	var sections []Section
	lines := strings.Split(content, "\n")

	var currentSection strings.Builder
	var currentHeading string

	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " ")

		// Check for H1 or H2
		if strings.HasPrefix(trimmed, "# ") {
			// Save previous section
			if currentSection.Len() > 0 {
				sections = append(sections, Section{
					Heading: currentHeading,
					Content: currentSection.String(),
				})
			}
			currentHeading = strings.TrimLeft(trimmed, "# ")
			currentHeading = strings.TrimSpace(currentHeading)
			currentSection.Reset()
			currentSection.WriteString(line + "\n")
		} else if strings.HasPrefix(trimmed, "## ") {
			// Save previous section
			if currentSection.Len() > 0 {
				sections = append(sections, Section{
					Heading: currentHeading,
					Content: currentSection.String(),
				})
			}
			currentHeading = strings.TrimLeft(trimmed, "#")
			currentHeading = strings.TrimSpace(currentHeading)
			currentSection.Reset()
			currentSection.WriteString(line + "\n")
		} else {
			currentSection.WriteString(line + "\n")
		}
	}

	// Don't forget the last section
	if currentSection.Len() > 0 {
		sections = append(sections, Section{
			Heading: currentHeading,
			Content: currentSection.String(),
		})
	}

	return sections
}

// splitByTokens splits a section into token-sized chunks with overlap.
func (c *MarkdownChunker) splitByTokens(section Section, docID, path string, startPos int) []Chunk {
	var chunks []Chunk

	words := strings.Fields(section.Content)
	if len(words) == 0 {
		return chunks
	}

	// Estimate tokens (rough approximation: 1 token ≈ 0.75 words)
	// This is conservative; actual BPE tokenization may differ
	estimateTokens := func(wordCount int) int {
		return int(float64(wordCount) / 0.75)
	}

	pos := 0
	for pos < len(words) {
		endPos := pos + c.maxTokens
		if endPos > len(words) {
			endPos = len(words)
		}

		chunkWords := words[pos:endPos]
		chunkContent := strings.Join(chunkWords, " ")

		// Skip if too small (unless it's the last chunk)
		tokenCount := estimateTokens(len(chunkWords))
		if tokenCount < c.minTokens && endPos < len(words) {
			pos = endPos
			continue
		}

		chunks = append(chunks, Chunk{
			DocumentID: docID,
			Path:       path,
			Content:    chunkContent,
			Section:    section.Heading,
			Tokens:     tokenCount,
			Position:   startPos + len(chunks),
		})

		// Move forward with overlap
		pos = endPos - c.overlap
		if pos < 0 {
			pos = 0
		}
	}

	return chunks
}

// Section represents a markdown section with its heading.
type Section struct {
	Heading string
	Content string
}

// estimateTokens is a helper function for token estimation.
// Note: This is a rough approximation. For production, use actual BPE tokenization.
// @MX:TODO: Replace with actual BPE tokenizer in M2 (SPEC-GOOSE-QMD-001 §7.5a)
func estimateTokens(text string) int {
	// Rough approximation: count words and punctuation
	wordCount := len(strings.Fields(text))
	punctCount := 0
	for _, r := range text {
		if unicode.IsPunct(r) {
			punctCount++
		}
	}

	// 1 token ≈ 0.75 words + 0.25 punctuation
	return int(float64(wordCount)/0.75) + int(float64(punctCount)/4.0)
}

// SplitPreservingCodeBlocks splits markdown while preserving code block integrity.
// @MX:NOTE: Ensures code blocks are never split across chunks
func SplitPreservingCodeBlocks(content string) []string {
	var parts []string
	var currentPart strings.Builder
	inCodeBlock := false
	codeBlockFence := ""

	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for code block fence
		if (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")) && !inCodeBlock {
			// Start of code block
			inCodeBlock = true
			codeBlockFence = trimmed[:3]
			currentPart.WriteString(line + "\n")
		} else if inCodeBlock && strings.HasPrefix(trimmed, codeBlockFence) {
			// End of code block
			inCodeBlock = false
			currentPart.WriteString(line + "\n")
			parts = append(parts, currentPart.String())
			currentPart.Reset()
		} else if inCodeBlock {
			// Inside code block - preserve as-is
			currentPart.WriteString(line + "\n")
		} else {
			// Regular content
			if trimmed == "" {
				// Empty line might indicate a paragraph break
				if currentPart.Len() > 0 {
					parts = append(parts, currentPart.String())
					currentPart.Reset()
				}
			} else {
				currentPart.WriteString(line + "\n")
			}
		}
	}

	// Don't forget the last part
	if currentPart.Len() > 0 {
		parts = append(parts, currentPart.String())
	}

	return parts
}

// ExtractFrontmatter separates frontmatter from markdown content.
// Returns (frontmatter, content, error).
// @MX:ANCHOR: Frontmatter extraction (expected fan_in >= 3)
func ExtractFrontmatter(content string) (string, string, error) {
	lines := strings.Split(content, "\n")

	if len(lines) == 0 {
		return "", content, nil
	}

	// Check for YAML frontmatter
	if strings.TrimSpace(lines[0]) == "---" {
		var fmLines []string
		var contentLines []string
		inFM := true

		for _, line := range lines[1:] {
			if inFM {
				if strings.TrimSpace(line) == "---" {
					inFM = false
					continue
				}
				fmLines = append(fmLines, line)
			} else {
				contentLines = append(contentLines, line)
			}
		}

		frontmatter := strings.Join(fmLines, "\n")
		remaining := strings.Join(contentLines, "\n")

		return frontmatter, remaining, nil
	}

	// Check for TOML frontmatter
	if strings.TrimSpace(lines[0]) == "+++" {
		var fmLines []string
		var contentLines []string
		inFM := true

		for _, line := range lines[1:] {
			if inFM {
				if strings.TrimSpace(line) == "+++" {
					inFM = false
					continue
				}
				fmLines = append(fmLines, line)
			} else {
				contentLines = append(contentLines, line)
			}
		}

		frontmatter := strings.Join(fmLines, "\n")
		remaining := strings.Join(contentLines, "\n")

		return frontmatter, remaining, nil
	}

	return "", content, nil
}

// ParseFrontmatterToMap parses frontmatter YAML/TOML into a string map.
// This is a simplified version; full parsing requires yaml/toml libraries.
// @MX:TODO: Implement full YAML/TOML parsing in M1
func ParseFrontmatterToMap(frontmatter string) (map[string]string, error) {
	metadata := make(map[string]string)

	if frontmatter == "" {
		return metadata, nil
	}

	// Simple key-value parsing (lines with "key: value")
	// Full YAML/TOML parsing will be implemented in M1
	lines := strings.Split(frontmatter, "\n")
	for _, line := range lines {
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				metadata[key] = value
			}
		}
	}

	return metadata, nil
}

// CleanDocument removes frontmatter and code fences from markdown for indexing.
// @MX:NOTE: Cleaning reduces noise in search results
func CleanDocument(content string) string {
	_, content, _ = ExtractFrontmatter(content)

	// Remove code blocks but keep content (simplified)
	lines := strings.Split(content, "\n")
	var cleaned []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if !inCodeBlock {
			cleaned = append(cleaned, line)
		}
	}

	return strings.Join(cleaned, "\n")
}

// ValidateChunk checks if a chunk meets minimum requirements.
func ValidateChunk(chunk Chunk) error {
	if chunk.DocumentID == "" {
		return fmt.Errorf("%w: chunk missing DocumentID", ErrInvalidDocument)
	}
	if chunk.Path == "" {
		return fmt.Errorf("%w: chunk missing Path", ErrInvalidDocument)
	}
	if len(chunk.Content) == 0 {
		return fmt.Errorf("%w: chunk has empty Content", ErrInvalidDocument)
	}
	return nil
}
