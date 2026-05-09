package telegram

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// updateGolden enables regeneration of testdata/ expected files. Invoke as
// `go test -run TestGolden -update-golden ./internal/messaging/telegram/...`.
var updateGolden = flag.Bool("update-golden", false,
	"regenerate testdata expected.txt files for golden tests")

// TestGolden_EscapeV2 verifies EscapeV2 against pinned input/expected pairs
// in testdata/markdown_v2/<name>/{input.txt,expected.txt}. Each fixture is
// auto-discovered by glob; adding a new directory under testdata/markdown_v2/
// with the two files is sufficient to register a new case.
func TestGolden_EscapeV2(t *testing.T) {
	matches, err := filepath.Glob("testdata/markdown_v2/*/input.txt")
	if err != nil {
		t.Fatalf("glob markdown_v2 fixtures: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no markdown_v2 fixtures found in testdata/")
	}

	for _, inputPath := range matches {
		dir := filepath.Dir(inputPath)
		name := filepath.Base(dir)
		expectedPath := filepath.Join(dir, "expected.txt")

		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatalf("read %s: %v", inputPath, err)
			}
			got := EscapeV2(string(input))

			if *updateGolden {
				if err := os.WriteFile(expectedPath, []byte(got), 0o644); err != nil {
					t.Fatalf("write %s: %v", expectedPath, err)
				}
				return
			}

			expected, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("read %s: %v", expectedPath, err)
			}
			if got != string(expected) {
				t.Errorf("EscapeV2 golden mismatch for %s:\ninput:    %q\nwant:     %q\ngot:      %q",
					name, string(input), string(expected), got)
			}
		})
	}
}

// TestGolden_RenderInlineKeyboard verifies RenderInlineKeyboard against pinned
// input.json/expected.txt fixture pairs. The input.json file holds a JSON
// array of arrays of InlineButton; "[]" is treated as a nil layout and
// returns "[]".
func TestGolden_RenderInlineKeyboard(t *testing.T) {
	matches, err := filepath.Glob("testdata/inline_keyboard/*/input.json")
	if err != nil {
		t.Fatalf("glob inline_keyboard fixtures: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no inline_keyboard fixtures found in testdata/")
	}

	for _, inputPath := range matches {
		dir := filepath.Dir(inputPath)
		name := filepath.Base(dir)
		expectedPath := filepath.Join(dir, "expected.txt")

		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatalf("read %s: %v", inputPath, err)
			}

			var rows [][]InlineButton
			trimmed := strings.TrimSpace(string(raw))
			if trimmed != "[]" {
				if err := json.Unmarshal(raw, &rows); err != nil {
					t.Fatalf("unmarshal %s: %v", inputPath, err)
				}
			}

			got := RenderInlineKeyboard(rows)

			if *updateGolden {
				if err := os.WriteFile(expectedPath, []byte(got), 0o644); err != nil {
					t.Fatalf("write %s: %v", expectedPath, err)
				}
				return
			}

			expected, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("read %s: %v", expectedPath, err)
			}
			if got != strings.TrimRight(string(expected), "\n") {
				t.Errorf("RenderInlineKeyboard golden mismatch for %s:\ninput:    %s\nwant:     %s\ngot:      %s",
					name, string(raw), string(expected), got)
			}
		})
	}
}
