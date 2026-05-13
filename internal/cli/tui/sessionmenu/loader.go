// Package sessionmenu provides the session file loader for the Ctrl-R overlay.
package sessionmenu

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/modu-ai/mink/internal/userpath"
)

// maxEntries is the maximum number of sessions shown in the overlay.
// REQ-CLITUI3-002: at most 10 entries.
const maxEntries = 10

// Load reads ~/.goose/sessions/*.jsonl sorted by modification time descending.
// Returns an empty slice if the directory is absent or contains no .jsonl files.
// Never returns an error — silently ignores IO failures.
// @MX:ANCHOR Load is called by tui.handleKeyMsg (Ctrl-R) and tests.
// @MX:REASON fan_in >= 3: update.go KeyCtrlR handler, sessionmenu_tui_test.go, loader_test.go.
func Load() []Entry {
	dir := sessionsDir()
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var result []Entry
	for _, de := range dirEntries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".jsonl") {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(de.Name(), ".jsonl")
		result = append(result, Entry{
			Name:    name,
			Path:    filepath.Join(dir, de.Name()),
			ModTime: info.ModTime(),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ModTime.After(result[j].ModTime)
	})

	if len(result) > maxEntries {
		result = result[:maxEntries]
	}
	return result
}

// sessionsDir returns the path to ~/.mink/sessions/.
// REQ-MINK-UDM-002: userpath.UserHomeE() 경유.
func sessionsDir() string {
	home, err := userpath.UserHomeE()
	if err != nil {
		// fallback: $HOME/.mink/sessions
		return filepath.Join(os.Getenv("HOME"), ".mink", "sessions")
	}
	return filepath.Join(home, "sessions")
}
