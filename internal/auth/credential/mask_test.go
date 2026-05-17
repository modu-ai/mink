package credential_test

import (
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/auth/credential"
)

// TestMaskedString validates the MaskedString helper against the AC-CR-024
// specification (UN-1):
//   - len >= 5: "***" + last-4 runes
//   - len < 5: "***"
//   - empty: "***"
func TestMaskedString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: "***"},
		{name: "one_char", input: "a", want: "***"},
		{name: "four_chars", input: "abcd", want: "***"},
		{name: "exactly_five", input: "abcde", want: "***bcde"},
		{name: "ten_chars", input: "0123456789", want: "***6789"},
		{name: "api_key_style", input: "sk-ant-1234567890", want: "***7890"},
		// Unicode: "안녕하세요" has 5 runes; masked should yield last 4 runes.
		{name: "unicode_five_runes", input: "안녕하세요", want: "***녕하세요"},
		{name: "unicode_four_runes", input: "한글테스", want: "***"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := credential.MaskedString(tc.input)
			if got != tc.want {
				t.Errorf("MaskedString(%q) = %q; want %q", tc.input, got, tc.want)
			}

			// Safety invariant: the result must never contain the full input
			// when the input has 5 or more characters.
			if len([]rune(tc.input)) >= 5 && strings.Contains(got, tc.input) {
				t.Errorf("MaskedString returned full plaintext for input %q", tc.input)
			}

			// Result always starts with "***".
			if !strings.HasPrefix(got, "***") {
				t.Errorf("MaskedString(%q) = %q; expected *** prefix", tc.input, got)
			}
		})
	}
}

// TestMaskedStringNoPlaintext asserts that a real-world sk- prefixed key is
// never returned verbatim.  This directly maps to AC-CR-024.
func TestMaskedStringNoPlaintext(t *testing.T) {
	t.Parallel()

	const key = "sk-ant-api03-XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
	masked := credential.MaskedString(key)

	if strings.Contains(masked, "sk-") {
		t.Errorf("MaskedString returned value containing sk- prefix: %q", masked)
	}
	if masked == key {
		t.Errorf("MaskedString returned full plaintext key")
	}
}
