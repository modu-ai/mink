// Package parser implements the slash command line parser.
// SPEC: SPEC-GOOSE-COMMAND-001
package parser_test

import (
	"testing"

	"github.com/modu-ai/mink/internal/command/parser"
	"github.com/stretchr/testify/require"
)

// TestParser_DetectsSlash verifies that Parse correctly identifies slash commands.
// RED #1 — REQ-CMD-001.
func TestParser_DetectsSlash(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input   string
		name    string
		rawArgs string
		ok      bool
	}{
		{"/foo bar", "foo", "bar", true},
		{"hello", "", "", false},
		{"/help", "help", "", true},
		{"/compact target", "compact", "target", true},
		{"", "", "", false},
		{"plain prompt", "", "", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			name, rawArgs, ok := parser.Parse(tc.input)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.name, name)
			require.Equal(t, tc.rawArgs, rawArgs)
		})
	}
}

// TestParser_RejectsNonLetterAfterSlash verifies that a slash not followed by
// an ASCII letter is not treated as a command. RED #2 — REQ-CMD-001.
func TestParser_RejectsNonLetterAfterSlash(t *testing.T) {
	t.Parallel()

	rejects := []string{
		"//",
		"/1",
		"/ foo",
		"/-x",
		"/_underscore",
		"/",
	}

	for _, input := range rejects {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			_, _, ok := parser.Parse(input)
			require.False(t, ok, "expected ok=false for input %q", input)
		})
	}
}

// TestParser_LowercasesName ensures the command name is normalised to lowercase.
// REQ-CMD-003.
func TestParser_LowercasesName(t *testing.T) {
	t.Parallel()
	name, _, ok := parser.Parse("/HELP")
	require.True(t, ok)
	require.Equal(t, "help", name)
}

// TestParser_TrimsLeftWhitespace verifies that leading whitespace before / is ignored.
// REQ-CMD-001.
func TestParser_TrimsLeftWhitespace(t *testing.T) {
	t.Parallel()
	name, rawArgs, ok := parser.Parse("   /help")
	require.True(t, ok)
	require.Equal(t, "help", name)
	require.Equal(t, "", rawArgs)
}

// TestParser_MultiWordArgs verifies rawArgs captures everything after the name.
func TestParser_MultiWordArgs(t *testing.T) {
	t.Parallel()
	name, rawArgs, ok := parser.Parse("/model gpt-4o extra")
	require.True(t, ok)
	require.Equal(t, "model", name)
	require.Equal(t, "gpt-4o extra", rawArgs)
}

// TestSplitArgs_Quoted verifies double-quoted tokens are treated as one positional.
// RED test for AC-CMD-004.
func TestSplitArgs_Quoted(t *testing.T) {
	t.Parallel()
	args, err := parser.SplitArgs(`"a b" c`)
	require.NoError(t, err)
	require.Equal(t, []string{"a b", "c"}, args.Positional)
}

// TestSplitArgs_FlagEqValue verifies --key=value flag parsing.
func TestSplitArgs_FlagEqValue(t *testing.T) {
	t.Parallel()
	args, err := parser.SplitArgs("--mode=fast x")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"mode": "fast"}, args.Flags)
	require.Equal(t, []string{"x"}, args.Positional)
}

// TestSplitArgs_SingleQuote verifies single-quoted tokens are treated as one positional.
func TestSplitArgs_SingleQuote(t *testing.T) {
	t.Parallel()
	args, err := parser.SplitArgs("'hello world' foo")
	require.NoError(t, err)
	require.Equal(t, []string{"hello world", "foo"}, args.Positional)
}

// TestSplitArgs_BackslashEscape verifies backslash-escaped spaces.
func TestSplitArgs_BackslashEscape(t *testing.T) {
	t.Parallel()
	args, err := parser.SplitArgs(`hello\ world`)
	require.NoError(t, err)
	require.Equal(t, []string{"hello world"}, args.Positional)
}

// TestSplitArgs_Empty verifies empty input produces empty args.
func TestSplitArgs_Empty(t *testing.T) {
	t.Parallel()
	args, err := parser.SplitArgs("")
	require.NoError(t, err)
	require.Empty(t, args.Positional)
	require.Empty(t, args.Flags)
}

// TestSplitArgs_RawArgs verifies RawArgs is preserved in parsed output.
func TestSplitArgs_RawArgs(t *testing.T) {
	t.Parallel()
	args, err := parser.SplitArgs("foo bar")
	require.NoError(t, err)
	require.Equal(t, "foo bar", args.RawArgs)
}

// TestSplitArgs_FlagSpaceSeparated verifies --key value flag parsing.
func TestSplitArgs_FlagSpaceSeparated(t *testing.T) {
	t.Parallel()
	args, err := parser.SplitArgs("--mode fast")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"mode": "fast"}, args.Flags)
	require.Empty(t, args.Positional)
}

// TestParser_NoIO verifies the parser does not perform IO — it is a pure string operation.
// REQ-CMD-015: calling Parse with a path-like string must not open files.
// The name token is the entire first whitespace-delimited token after "/", so
// "/nonexistent/path/to/something args" yields name="nonexistent/path/to/something".
func TestParser_NoIO(t *testing.T) {
	t.Parallel()
	// If Parse tried to open this path it would fail or panic.
	name, rawArgs, ok := parser.Parse("/nonexistent/path/to/something args")
	// The name after the leading "/" is everything up to the first whitespace.
	require.True(t, ok)
	require.Equal(t, "nonexistent/path/to/something", name)
	require.Equal(t, "args", rawArgs)
}
