// Package session provides session file I/O for the goose CLI.
// Session files are stored as JSONL (JSON Lines) in ~/.goose/sessions/.
// @MX:NOTE JSONL format allows git-friendly line-based diffs and efficient appends.
package session

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/modu-ai/mink/internal/userpath"
)

// Message represents a single message in a session.
// @MX:ANCHOR Message is used across session, TUI, and transport layers.
type Message struct {
	Role      string `json:"role"`    // "user", "assistant", "system"
	Content   string `json:"content"` // Message content
	Timestamp int64  `json:"ts"`      // Unix timestamp in milliseconds
}

// testDir is overridden in tests to use temporary directories.
// @MX:NOTE Using a variable allows test isolation without changing Dir() signature.
var testDir string

// Dir returns the session directory path, creating it if not exists.
// @MX:ANCHOR Dir is called by all session operations to resolve the directory path.
// @MX:REASON fan_in >= 3 (Save, Load, List all call Dir).
func Dir() string {
	if testDir != "" {
		return testDir
	}

	// REQ-MINK-UDM-002: userpath.UserHome() 경유 — .mink/sessions
	home, err := userpath.UserHomeE()
	if err != nil {
		// fallback: $HOME/.mink/sessions
		return filepath.Join(os.Getenv("HOME"), ".mink", "sessions")
	}
	return filepath.Join(home, "sessions")
}

// Save writes messages to a session file as JSONL (one JSON object per line).
// Uses atomic write pattern (temp file + rename) to prevent corruption.
// @MX:ANCHOR Save is the primary interface for persisting sessions.
// @MX:REASON fan_in >= 3 (called by CLI command, TUI /save, and tests).
// @MX:NOTE Atomic writes prevent data corruption during concurrent access (REQ-CLI-012).
func Save(name string, messages []Message) error {
	if err := ValidateName(name); err != nil {
		return fmt.Errorf("invalid session name: %w", err)
	}

	dir := Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Create temp file in the same directory for atomic rename
	// @MX:NOTE Temp file in same directory ensures rename works across filesystems.
	// REQ-MINK-UDM-004: tmp prefix .mink-session-* (AC-006)
	tmpPath := filepath.Join(dir, userpath.TempPrefix()+"session-"+name+".tmp")
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	// Encode each message as a separate line
	encoder := json.NewEncoder(tmpFile)
	for _, msg := range messages {
		if err := encoder.Encode(msg); err != nil {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("failed to encode message: %w", err)
		}
	}

	_ = tmpFile.Close()
	// Atomic rename
	finalPath := filepath.Join(dir, name+".jsonl")
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// Load reads messages from a session file.
// Returns error if the session does not exist or cannot be parsed.
// @MX:ANCHOR Load is the primary interface for restoring sessions.
// @MX:REASON fan_in >= 3 (called by CLI load command, TUI resume, and tests).
func Load(name string) ([]Message, error) {
	if err := ValidateName(name); err != nil {
		return nil, fmt.Errorf("invalid session name: %w", err)
	}

	path := filepath.Join(Dir(), name+".jsonl")

	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("session not found: %s", name)
		}
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var messages []Message
	scanner := bufio.NewScanner(file)

	// Read line by line to handle JSONL format
	// @MX:NOTE Line-based parsing allows partial recovery if file is corrupted.
	for scanner.Scan() {
		line := scanner.Bytes()
		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			return nil, fmt.Errorf("failed to decode message: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	return messages, nil
}

// Delete removes a session file.
// Returns error if the session does not exist or cannot be deleted.
func Delete(name string) error {
	if err := ValidateName(name); err != nil {
		return fmt.Errorf("invalid session name: %w", err)
	}

	path := filepath.Join(Dir(), name+".jsonl")

	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("session not found: %s", name)
		}
		return fmt.Errorf("failed to delete session file: %w", err)
	}

	return nil
}

// List returns all session names (filenames without .jsonl extension).
// Returns empty list if no sessions exist or directory does not exist.
func List() ([]string, error) {
	dir := Dir()

	// If directory doesn't exist, return empty list (not an error)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return []string{}, nil
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read session directory: %w", err)
	}

	var names []string
	for _, file := range files {
		// Skip hidden files and temp files
		if strings.HasPrefix(file.Name(), ".") {
			continue
		}

		// Only process .jsonl files
		if filepath.Ext(file.Name()) == ".jsonl" {
			// Remove .jsonl extension
			name := strings.TrimSuffix(file.Name(), ".jsonl")
			names = append(names, name)
		}
	}

	return names, nil
}

// ValidateName checks if a session name is safe (no path traversal).
// REQ-CLI-020: reject ../foo attempts to prevent writing outside sessions directory.
// @MX:ANCHOR ValidateName is the security boundary for session file operations.
// @MX:REASON fan_in >= 3 (called by Save, Load, Delete).
func ValidateName(name string) error {
	if name == "" {
		return errors.New("session name cannot be empty")
	}

	// Reject path separators and parent directory references
	// @MX:WARN This check prevents directory traversal attacks (REQ-CLI-020).
	// @MX:REASON Security: path injection could write to arbitrary filesystem locations.
	if strings.ContainsAny(name, "/\\") {
		return errors.New("session name cannot contain path separators")
	}

	if strings.Contains(name, "..") {
		return errors.New("session name cannot contain parent directory references")
	}

	// Reject names that would resolve to dot files
	if strings.HasPrefix(name, ".") {
		return errors.New("session name cannot start with a dot")
	}

	return nil
}
