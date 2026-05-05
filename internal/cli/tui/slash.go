// Package tui provides slash command parsing and handling for the TUI.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/modu-ai/goose/internal/cli/session"
)

// SlashCmd represents a parsed slash command.
// @MX:ANCHOR This type defines the structure of parsed slash commands.
type SlashCmd struct {
	Name string
	Args []string
}

// ParseSlashCmd parses "/command arg1 arg2" into SlashCmd.
// @MX:ANCHOR This function is the primary parser for slash commands.
// Returns (SlashCmd, true) if input is a valid slash command, (SlashCmd{}, false) otherwise.
func ParseSlashCmd(input string) (SlashCmd, bool) {
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, "/") {
		return SlashCmd{}, false
	}

	// Remove leading slash
	trimmed = trimmed[1:]

	// Split by spaces
	parts := strings.Fields(trimmed)

	if len(parts) == 0 {
		return SlashCmd{}, false
	}

	cmd := SlashCmd{
		Name: parts[0],
	}

	if len(parts) > 1 {
		cmd.Args = parts[1:]
	}

	return cmd, true
}

// HandleSlashCmd processes a slash command and returns response text.
// @MX:ANCHOR This function handles all slash commands and returns appropriate responses.
// Returns (response string, tea.Cmd). The tea.Cmd may be tea.Quit for /quit command.
func HandleSlashCmd(cmd SlashCmd, m *Model) (string, tea.Cmd) {
	switch cmd.Name {
	case "help":
		return handleHelp(m), nil

	case "save":
		return handleSave(cmd, m)

	case "load":
		return handleLoad(cmd, m)

	case "clear":
		return handleClear(m), nil

	case "quit":
		return "Exiting...", tea.Quit

	case "session":
		return handleSession(m), nil

	default:
		return fmt.Sprintf("Unknown slash command: %s. Type /help for available commands.", cmd.Name), nil
	}
}

// handleHelp displays available slash commands using the locale catalog header.
// @MX:NOTE This provides in-TUI help without external documentation.
func handleHelp(m *Model) string {
	return m.catalog.SlashHelpHeader + "\n" + `  /help       Show this help
  /save <name> Save session
  /load <name> Load session
  /clear      Clear chat history
  /quit       Exit TUI
  /session    Show current session name`
}

// handleSave saves the current session to ~/.goose/sessions/<name>.jsonl.
// Uses atomic write (temp file + rename) via session.Save. AC-CLITUI-012.
func handleSave(cmd SlashCmd, m *Model) (string, tea.Cmd) {
	if len(cmd.Args) == 0 {
		return "Usage: /save <name>", nil
	}

	name := cmd.Args[0]

	// Convert model messages to session messages (skip system messages).
	var msgs []session.Message
	for _, cm := range m.messages {
		if cm.Role == "system" {
			continue
		}
		msgs = append(msgs, session.Message{
			Role:    cm.Role,
			Content: cm.Content,
		})
	}

	if err := session.Save(name, msgs); err != nil {
		return fmt.Sprintf("[Error saving session: %v]", err), nil
	}

	return fmt.Sprintf(m.catalog.Saved, name), nil
}

// handleLoad loads a session from ~/.goose/sessions/<name>.jsonl into the model.
// Restores messages and sets initialMsgs for the next ChatStream call. AC-CLITUI-013.
// Delegates to loadSessionByName (session_ops.go) to share logic with sessionmenu handler.
func handleLoad(cmd SlashCmd, m *Model) (string, tea.Cmd) {
	if len(cmd.Args) == 0 {
		return "Usage: /load <name>", nil
	}
	return m.loadSessionByName(cmd.Args[0]), nil
}

// handleClear clears the chat history.
func handleClear(m *Model) string {
	m.messages = make([]ChatMessage, 0)
	m.updateViewport()
	return "Chat history cleared."
}

// handleSession shows the current session name.
func handleSession(m *Model) string {
	if m.sessionName == "" {
		return "Current session: (unnamed)"
	}
	return fmt.Sprintf("Current session: %s", m.sessionName)
}
