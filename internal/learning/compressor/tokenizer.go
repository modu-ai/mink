package compressor

import (
	"strings"

	"github.com/modu-ai/goose/internal/learning/trajectory"
)

// Tokenizer is the sole source of token counts within the compressor.
// REQ-COMPRESSOR-004: hardcoded character-to-token ratios must not appear outside Tokenizer implementations.
type Tokenizer interface {
	// Count returns the approximate token count for a single string value.
	Count(value string) int
	// CountTrajectory returns the total token count for all conversation entries.
	CountTrajectory(t *trajectory.Trajectory) int
}

// SimpleTokenizer implements Tokenizer using a word-count approximation.
// Approximation: words * 1.3 rounded to nearest int.
// For production, inject a tiktoken-go based implementation.
type SimpleTokenizer struct{}

// Count approximates token count as len(strings.Fields(value)) * 1.3.
// The 1.3 factor accounts for sub-word tokenization overhead on average English text.
func (s *SimpleTokenizer) Count(value string) int {
	words := len(strings.Fields(value))
	// Use integer arithmetic to avoid floating-point drift: multiply by 13, divide by 10.
	return (words*13 + 9) / 10
}

// CountTrajectory sums Count over all conversation entries.
func (s *SimpleTokenizer) CountTrajectory(t *trajectory.Trajectory) int {
	total := 0
	for _, entry := range t.Conversations {
		total += s.Count(entry.Value)
	}
	return total
}
