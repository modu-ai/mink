// Package substitute implements the template substitution engine for custom commands.
// SPEC: SPEC-GOOSE-COMMAND-001
package substitute_test

import (
	"testing"

	"github.com/modu-ai/mink/internal/command"
	"github.com/modu-ai/mink/internal/command/substitute"
	"github.com/stretchr/testify/require"
)

// makeCtx is a helper to build a substitute.Context from raw and positional args.
func makeCtx(rawArgs string, positional []string, env map[string]string) substitute.Context {
	return substitute.Context{
		Args: command.Args{
			RawArgs:    rawArgs,
			Positional: positional,
		},
		Env: env,
	}
}

// TestExpand_Arguments_FullSubstring verifies $ARGUMENTS is replaced with rawArgs.
// RED #11 — AC-CMD-004.
func TestExpand_Arguments_FullSubstring(t *testing.T) {
	t.Parallel()
	ctx := makeCtx("Alice", []string{"Alice"}, nil)
	got, err := substitute.Expand("Hello $ARGUMENTS, welcome to GOOSE.", ctx)
	require.NoError(t, err)
	require.Equal(t, "Hello Alice, welcome to GOOSE.", got)
}

// TestExpand_Positional_OneTwoNine verifies $1..$9 positional substitution.
// RED #12 — AC-CMD-005.
func TestExpand_Positional_OneTwoNine(t *testing.T) {
	t.Parallel()
	ctx := makeCtx("foo bar baz", []string{"foo", "bar", "baz"}, nil)
	got, err := substitute.Expand("First: $1, Second: $2, All: $ARGUMENTS", ctx)
	require.NoError(t, err)
	require.Equal(t, "First: foo, Second: bar, All: foo bar baz", got)
}

// TestExpand_NoRecursion verifies that substitution is a single pass —
// the expanded value of $ARGUMENTS is not itself expanded.
// RED #13 — AC-CMD-012, REQ-CMD-013.
func TestExpand_NoRecursion(t *testing.T) {
	t.Parallel()
	// User passes "$ARGUMENTS" as literal text in the command input.
	ctx := makeCtx("$ARGUMENTS", []string{"$ARGUMENTS"}, nil)
	// Body template expands $ARGUMENTS once; the result "$ARGUMENTS" must stay literal.
	got, err := substitute.Expand("Echo: $ARGUMENTS", ctx)
	require.NoError(t, err)
	require.Equal(t, "Echo: $ARGUMENTS", got)
}

// TestExpand_DollarDollar_LiteralDollar verifies $$ becomes a literal $.
func TestExpand_DollarDollar_LiteralDollar(t *testing.T) {
	t.Parallel()
	ctx := makeCtx("", nil, nil)
	got, err := substitute.Expand("Price: $$100", ctx)
	require.NoError(t, err)
	require.Equal(t, "Price: $100", got)
}

// TestExpand_UnknownVariable_LeftLiteral verifies that unrecognised $NAME tokens
// are preserved as-is without error.
func TestExpand_UnknownVariable_LeftLiteral(t *testing.T) {
	t.Parallel()
	ctx := makeCtx("", nil, nil)
	got, err := substitute.Expand("Value: $UNKNOWN", ctx)
	require.NoError(t, err)
	require.Equal(t, "Value: $UNKNOWN", got)
}

// TestExpand_EnvCWD verifies $CWD is expanded from the Env map.
func TestExpand_EnvCWD(t *testing.T) {
	t.Parallel()
	ctx := makeCtx("", nil, map[string]string{"CWD": "/home/user"})
	got, err := substitute.Expand("Working dir: $CWD", ctx)
	require.NoError(t, err)
	require.Equal(t, "Working dir: /home/user", got)
}

// TestExpand_MissingPositional_EmptyString verifies that $N for out-of-range N
// expands to an empty string rather than panicking.
func TestExpand_MissingPositional_EmptyString(t *testing.T) {
	t.Parallel()
	ctx := makeCtx("foo", []string{"foo"}, nil)
	// $2 has no corresponding positional arg.
	got, err := substitute.Expand("$1 and $2", ctx)
	require.NoError(t, err)
	require.Equal(t, "foo and ", got)
}

// TestExpand_EmptyTemplate verifies an empty template returns an empty string.
func TestExpand_EmptyTemplate(t *testing.T) {
	t.Parallel()
	ctx := makeCtx("", nil, nil)
	got, err := substitute.Expand("", ctx)
	require.NoError(t, err)
	require.Equal(t, "", got)
}

// TestExpand_NoDollar verifies templates with no substitution tokens are unchanged.
func TestExpand_NoDollar(t *testing.T) {
	t.Parallel()
	ctx := makeCtx("ignored", nil, nil)
	got, err := substitute.Expand("plain text", ctx)
	require.NoError(t, err)
	require.Equal(t, "plain text", got)
}

// TestExpand_GooseHome verifies $GOOSE_HOME is expanded from the Env map.
func TestExpand_GooseHome(t *testing.T) {
	t.Parallel()
	ctx := makeCtx("", nil, map[string]string{"GOOSE_HOME": "/opt/goose"})
	got, err := substitute.Expand("Home: $GOOSE_HOME", ctx)
	require.NoError(t, err)
	require.Equal(t, "Home: /opt/goose", got)
}

// TestExpand_MultipleArguments verifies multiple $ARGUMENTS occurrences in a template.
func TestExpand_MultipleArguments(t *testing.T) {
	t.Parallel()
	ctx := makeCtx("hello", nil, nil)
	got, err := substitute.Expand("$ARGUMENTS and $ARGUMENTS", ctx)
	require.NoError(t, err)
	require.Equal(t, "hello and hello", got)
}

// TestExpand_TrailingDollar verifies a lone trailing $ is emitted literally.
func TestExpand_TrailingDollar(t *testing.T) {
	t.Parallel()
	ctx := makeCtx("", nil, nil)
	got, err := substitute.Expand("price$", ctx)
	require.NoError(t, err)
	require.Equal(t, "price$", got)
}

// TestExpand_DollarFollowedByLower verifies that $lower (lowercase) is emitted literally.
func TestExpand_DollarFollowedByLower(t *testing.T) {
	t.Parallel()
	ctx := makeCtx("", nil, nil)
	got, err := substitute.Expand("$foo", ctx)
	require.NoError(t, err)
	require.Equal(t, "$foo", got)
}

// TestExpand_PositionalAll verifies $1 through $9 positional substitution.
func TestExpand_PositionalAll(t *testing.T) {
	t.Parallel()
	positional := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
	ctx := makeCtx("a b c d e f g h i", positional, nil)
	got, err := substitute.Expand("$1$2$3$4$5$6$7$8$9", ctx)
	require.NoError(t, err)
	require.Equal(t, "abcdefghi", got)
}
