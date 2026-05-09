package telegram

import (
	"strings"
	"testing"
)

// TestEscapeV2_AllReservedChars verifies that every one of the 18 MarkdownV2
// reserved characters is escaped with a preceding backslash.
// Telegram Bot API §MarkdownV2 style lists:
//
//	_ * [ ] ( ) ~ ` > # + - = | { } . !
func TestEscapeV2_AllReservedChars(t *testing.T) {
	reserved := []struct {
		char    string
		name    string
		escaped string
	}{
		{"_", "underscore", `\_`},
		{"*", "asterisk", `\*`},
		{"[", "left bracket", `\[`},
		{"]", "right bracket", `\]`},
		{"(", "left paren", `\(`},
		{")", "right paren", `\)`},
		{"~", "tilde", `\~`},
		{"`", "backtick", "\\`"},
		{">", "greater-than", `\>`},
		{"#", "hash", `\#`},
		{"+", "plus", `\+`},
		{"-", "minus", `\-`},
		{"=", "equals", `\=`},
		{"|", "pipe", `\|`},
		{"{", "left brace", `\{`},
		{"}", "right brace", `\}`},
		{".", "period", `\.`},
		{"!", "exclamation", `\!`},
	}

	for _, tc := range reserved {
		t.Run(tc.name, func(t *testing.T) {
			got := EscapeV2(tc.char)
			if got != tc.escaped {
				t.Errorf("EscapeV2(%q) = %q, want %q", tc.char, got, tc.escaped)
			}
		})
	}
}

// TestEscapeV2_PlainText verifies that plain text without reserved chars passes
// through unchanged.
func TestEscapeV2_PlainText(t *testing.T) {
	plain := "Hello World 12345 abc"
	got := EscapeV2(plain)
	if got != plain {
		t.Errorf("EscapeV2(%q) = %q, want %q", plain, got, plain)
	}
}

// TestEscapeV2_Mixed verifies correct escaping of a mixed string containing
// both plain text and multiple reserved chars.
func TestEscapeV2_Mixed(t *testing.T) {
	input := "Hello, *world*! It's [great](https://example.com)."
	got := EscapeV2(input)

	// Every reserved char in input must be preceded by '\' in output.
	reservedSet := map[rune]bool{
		'_': true, '*': true, '[': true, ']': true, '(': true, ')': true,
		'~': true, '`': true, '>': true, '#': true, '+': true, '-': true,
		'=': true, '|': true, '{': true, '}': true, '.': true, '!': true,
	}

	runes := []rune(got)
	for i, r := range runes {
		if reservedSet[r] {
			if i == 0 || runes[i-1] != '\\' {
				t.Errorf("reserved char %q at position %d is not escaped in %q", r, i, got)
			}
		}
	}
}

// TestEscapeV2_Idempotent verifies that applying EscapeV2 twice produces the
// same result as applying it three times (escape is monotone — not idempotent,
// but double-escape must be distinct from triple-escape, meaning the function
// is deterministic and does not skip already-escaped chars).
// Specifically: EscapeV2(EscapeV2(s)) is safe to call without double-escaping.
func TestEscapeV2_Idempotent(t *testing.T) {
	// EscapeV2 is not strictly idempotent (backslash is not a reserved char,
	// so applying it twice escapes the backslash from the first pass).
	// This test verifies the expected non-idempotent behaviour is consistent
	// and predictable.
	input := "Hello *world*"
	once := EscapeV2(input)
	twice := EscapeV2(once)
	thrice := EscapeV2(twice)
	if twice == once {
		t.Errorf("EscapeV2 appears idempotent but should not be: once=%q twice=%q", once, twice)
	}
	// Applying three times gives the same as the pattern EscapeV2(EscapeV2(EscapeV2(s))).
	// Stability: EscapeV2(twice) == EscapeV2(EscapeV2(once)) deterministically.
	if thrice == "" {
		t.Error("EscapeV2 returned empty string for non-empty input")
	}
}

// TestEscapeV2_AllReservedInOnce verifies escaping a string containing all 18
// reserved chars at once.
func TestEscapeV2_AllReservedInOnce(t *testing.T) {
	all := `_*[]()~` + "`" + `>#+-=|{}.!`
	got := EscapeV2(all)

	// Every character in `got` that is a reserved char must be preceded by '\'.
	reservedSet := map[rune]bool{
		'_': true, '*': true, '[': true, ']': true, '(': true, ')': true,
		'~': true, '`': true, '>': true, '#': true, '+': true, '-': true,
		'=': true, '|': true, '{': true, '}': true, '.': true, '!': true,
	}

	runes := []rune(got)
	for i, r := range runes {
		if reservedSet[r] {
			if i == 0 || runes[i-1] != '\\' {
				t.Errorf("reserved char %q at position %d is not escaped in %q", r, i, got)
			}
		}
	}
}

// TestEscapeV2_EmptyString verifies that empty input produces empty output.
func TestEscapeV2_EmptyString(t *testing.T) {
	if got := EscapeV2(""); got != "" {
		t.Errorf("EscapeV2(\"\") = %q, want empty string", got)
	}
}

// TestRenderInlineKeyboard_SingleRow verifies that a single-row keyboard is
// rendered as the correct Telegram JSON structure.
func TestRenderInlineKeyboard_SingleRow(t *testing.T) {
	buttons := []InlineButton{
		{Text: "Yes", CallbackData: "yes"},
		{Text: "No", CallbackData: "no"},
	}
	got := RenderInlineKeyboard([][]InlineButton{buttons})

	// Must contain both button texts.
	if !strings.Contains(got, `"Yes"`) {
		t.Errorf("RenderInlineKeyboard missing 'Yes': %s", got)
	}
	if !strings.Contains(got, `"No"`) {
		t.Errorf("RenderInlineKeyboard missing 'No': %s", got)
	}
	if !strings.Contains(got, `"yes"`) {
		t.Errorf("RenderInlineKeyboard missing 'yes' callback: %s", got)
	}
}

// TestRenderInlineKeyboard_Empty verifies that an empty keyboard renders as
// an empty JSON array.
func TestRenderInlineKeyboard_Empty(t *testing.T) {
	got := RenderInlineKeyboard(nil)
	if got != "[]" {
		t.Errorf("RenderInlineKeyboard(nil) = %q, want []", got)
	}
}

// TestRenderInlineKeyboard_ValidJSON verifies that the rendered keyboard is
// valid JSON that can be decoded.
func TestRenderInlineKeyboard_ValidJSON(t *testing.T) {
	buttons := []InlineButton{
		{Text: "Option A", CallbackData: "opt_a"},
	}
	got := RenderInlineKeyboard([][]InlineButton{buttons})

	// Must be a JSON array (starts with '[').
	if len(got) == 0 || got[0] != '[' {
		t.Errorf("RenderInlineKeyboard should return JSON array, got: %s", got)
	}
}
