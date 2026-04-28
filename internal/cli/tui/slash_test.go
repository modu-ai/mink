// Package tui provides unit tests for slash command parsing and handling.
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestParseSlashCmd verifies slash command parsing.
func TestParseSlashCmd(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectCmd   SlashCmd
		expectValid bool
	}{
		{
			name:        "simple command",
			input:       "/help",
			expectCmd:   SlashCmd{Name: "help", Args: []string{}},
			expectValid: true,
		},
		{
			name:        "command with single argument",
			input:       "/save my-session",
			expectCmd:   SlashCmd{Name: "save", Args: []string{"my-session"}},
			expectValid: true,
		},
		{
			name:        "command with multiple arguments",
			input:       "/test arg1 arg2 arg3",
			expectCmd:   SlashCmd{Name: "test", Args: []string{"arg1", "arg2", "arg3"}},
			expectValid: true,
		},
		{
			name:        "command with extra spaces",
			input:       "  /save   my-session  ",
			expectCmd:   SlashCmd{Name: "save", Args: []string{"my-session"}},
			expectValid: true,
		},
		{
			name:        "non-slash input",
			input:       "hello world",
			expectCmd:   SlashCmd{},
			expectValid: false,
		},
		{
			name:        "empty input",
			input:       "",
			expectCmd:   SlashCmd{},
			expectValid: false,
		},
		{
			name:        "just slash",
			input:       "/",
			expectCmd:   SlashCmd{},
			expectValid: false,
		},
		{
			name:        "only spaces",
			input:       "   ",
			expectCmd:   SlashCmd{},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, valid := ParseSlashCmd(tt.input)

			if valid != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, valid)
			}

			if valid {
				if cmd.Name != tt.expectCmd.Name {
					t.Errorf("expected name '%s', got '%s'", tt.expectCmd.Name, cmd.Name)
				}

				if len(cmd.Args) != len(tt.expectCmd.Args) {
					t.Errorf("expected %d args, got %d", len(tt.expectCmd.Args), len(cmd.Args))
				} else {
					for i, arg := range cmd.Args {
						if arg != tt.expectCmd.Args[i] {
							t.Errorf("arg %d: expected '%s', got '%s'", i, tt.expectCmd.Args[i], arg)
						}
					}
				}
			}
		})
	}
}

// TestHandleSlashCmd verifies slash command handling.
func TestHandleSlashCmd(t *testing.T) {
	tests := []struct {
		name           string
		cmd            SlashCmd
		expectResponse string
		expectQuit     bool
	}{
		{
			name:           "/help command",
			cmd:            SlashCmd{Name: "help", Args: []string{}},
			expectResponse: "Available slash commands:\n  /help       Show this help\n  /save <name> Save session\n  /load <name> Load session\n  /clear      Clear chat history\n  /quit       Exit TUI\n  /session    Show current session name",
		},
		{
			name:           "/clear command",
			cmd:            SlashCmd{Name: "clear", Args: []string{}},
			expectResponse: "Chat history cleared.",
		},
		{
			name:           "/session command with name",
			cmd:            SlashCmd{Name: "session", Args: []string{}},
			expectResponse: "Current session: (unnamed)",
		},
		{
			name:           "/quit command",
			cmd:            SlashCmd{Name: "quit", Args: []string{}},
			expectResponse: "Exiting...",
			expectQuit:     true,
		},
		{
			name:           "unknown command",
			cmd:            SlashCmd{Name: "unknown", Args: []string{}},
			expectResponse: "Unknown slash command: unknown. Type /help for available commands.",
		},
		{
			name:           "/save without args",
			cmd:            SlashCmd{Name: "save", Args: []string{}},
			expectResponse: "Usage: /save <name>",
		},
		{
			name:           "/load without args",
			cmd:            SlashCmd{Name: "load", Args: []string{}},
			expectResponse: "Usage: /load <name>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel(nil, "", false)

			response, quitCmd := HandleSlashCmd(tt.cmd, model)

			if response != tt.expectResponse {
				t.Errorf("expected response '%s', got '%s'", tt.expectResponse, response)
			}

			if tt.expectQuit && quitCmd == nil {
				t.Error("expected quit command, got nil")
			}

			if !tt.expectQuit && quitCmd != nil {
				// Execute the command to check if it's tea.Quit
				msg := quitCmd()
				if msg != nil {
					if _, ok := msg.(tea.QuitMsg); !ok {
						t.Error("expected no quit command for non-quit slash command")
					}
				}
			}
		})
	}
}

// TestHandleSlashCmdSaveLoad verifies save/load slash commands.
func TestHandleSlashCmdSaveLoad(t *testing.T) {
	t.Run("/save command", func(t *testing.T) {
		model := NewModel(nil, "", false)
		cmd := SlashCmd{Name: "save", Args: []string{"test-session"}}

		response, _ := HandleSlashCmd(cmd, model)

		// Save is not yet implemented
		if response == "" {
			t.Error("expected response for /save command")
		}
	})

	t.Run("/load command", func(t *testing.T) {
		model := NewModel(nil, "", false)
		cmd := SlashCmd{Name: "load", Args: []string{"test-session"}}

		response, _ := HandleSlashCmd(cmd, model)

		// Load is not yet implemented
		if response == "" {
			t.Error("expected response for /load command")
		}
	})
}
