package compressor

import (
	"testing"

	"github.com/modu-ai/goose/internal/learning/trajectory"
)

// TestSimpleTokenizer_AsciiAndUnicode verifies AC-COMPRESSOR-002 (approximate ratio bounds).
func TestSimpleTokenizer_AsciiAndUnicode(t *testing.T) {
	tok := &SimpleTokenizer{}

	// ASCII: "hello world" = 2 words → 2*1.3 = 2.6 → should be approximately 2-3.
	got := tok.Count("hello world")
	if got < 2 || got > 4 {
		t.Errorf("Count('hello world'): got %d, want 2-4", got)
	}

	// Empty string → 0 tokens.
	if tok.Count("") != 0 {
		t.Errorf("Count(''): got %d, want 0", tok.Count(""))
	}

	// Single word.
	single := tok.Count("word")
	if single < 1 || single > 2 {
		t.Errorf("Count('word'): got %d, want 1-2", single)
	}

	// Unicode: Chinese characters are each a separate "word" when field-split.
	chinese := tok.Count("你好世界")
	// Single token by whitespace split — the whole string is 1 field.
	if chinese < 1 || chinese > 3 {
		t.Errorf("Count('你好世界'): got %d, want 1-3", chinese)
	}

	// CountTrajectory: two entries.
	tr := &trajectory.Trajectory{
		Conversations: []trajectory.TrajectoryEntry{
			{From: trajectory.RoleHuman, Value: "hello world"},
			{From: trajectory.RoleGPT, Value: "foo bar baz"},
		},
	}
	total := tok.CountTrajectory(tr)
	entry1 := tok.Count("hello world")
	entry2 := tok.Count("foo bar baz")
	if total != entry1+entry2 {
		t.Errorf("CountTrajectory: got %d, want %d", total, entry1+entry2)
	}
}
