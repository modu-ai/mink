// Package tui provides the interactive chat TUI using bubbletea.
package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// handleKeyMsg processes keyboard input.
// @MX:ANCHOR This function routes all key presses to appropriate handlers.
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle quit confirmation during streaming
	if m.confirmQuit {
		if msg.Type == tea.KeyCtrlC {
			m.quitting = true
			return m, tea.Quit
		}
		// Any other key cancels quit confirmation
		m.confirmQuit = false
		return m, nil
	}

	// Handle quit requests
	switch msg.Type {
	case tea.KeyCtrlC:
		if m.streaming {
			// Require confirmation during streaming
			m.confirmQuit = true
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit

	case tea.KeyEscape:
		// Escape cancels streaming
		if m.streaming {
			m.streaming = false
			// Add a cancellation message
			m.messages = append(m.messages, ChatMessage{
				Role:    "system",
				Content: "[Response cancelled]",
			})
			m.updateViewport()
		}
		return m, nil

	case tea.KeyEnter:
		// Enter sends message (if not streaming)
		if !m.streaming && m.input.Value() != "" {
			input := m.input.Value()

			// Check for slash command
			if cmd, ok := ParseSlashCmd(input); ok {
				// Handle slash command
				response, quitCmd := HandleSlashCmd(cmd, m)

				// Clear input
				m.input.Reset()

				// Add response as system message
				m.messages = append(m.messages, ChatMessage{
					Role:    "system",
					Content: response,
				})
				m.updateViewport()

				// Return quit command if /quit
				if quitCmd != nil {
					return m, quitCmd
				}
				return m, nil
			}

			// Normal message send
			return m.sendMessage()
		}
		return m, nil

	case tea.KeyCtrlS:
		// Ctrl+S saves session
		return m, m.saveSession()

	case tea.KeyCtrlL:
		// Ctrl+L clears screen
		m.viewport.GotoTop()
		return m, nil
	}

	// Update input field
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// handleWindowSize handles terminal resize events.
// @MX:NOTE This ensures the TUI adapts to window size changes.
func (m *Model) handleWindowSize(msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height

	// Update viewport size (leave room for status bar and input)
	viewportHeight := m.height - 3 // Status bar (1) + input area (2)
	m.viewport.Width = m.width
	m.viewport.Height = max(viewportHeight, 5) // Minimum 5 lines
	m.updateViewport()
}

// handleStreamEvent processes events from the daemon stream.
// @MX:ANCHOR This function handles all streaming response events.
func (m *Model) handleStreamEvent(msg StreamEventMsg) (tea.Model, tea.Cmd) {
	switch msg.Event.Type {
	case "text":
		// Append text to current assistant message
		m.streaming = true
		m.confirmQuit = false

		// If last message is not from assistant, create new one
		if len(m.messages) == 0 || m.messages[len(m.messages)-1].Role != "assistant" {
			m.messages = append(m.messages, ChatMessage{
				Role:    "assistant",
				Content: msg.Event.Content,
			})
		} else {
			// Append to existing assistant message
			m.messages[len(m.messages)-1].Content += msg.Event.Content
		}
		m.updateViewport()

	case "error":
		// Handle error
		m.streaming = false
		m.messages = append(m.messages, ChatMessage{
			Role:    "system",
			Content: fmt.Sprintf("[Error: %s]", msg.Event.Content),
		})
		m.updateViewport()

	case "done":
		// Stream completed
		m.streaming = false
		m.confirmQuit = false
	}

	return m, nil
}

// sendMessage sends the current input to the daemon.
// @MX:ANCHOR This function initiates a new chat message.
func (m *Model) sendMessage() (tea.Model, tea.Cmd) {
	userMsg := m.input.Value()
	if userMsg == "" {
		return m, nil
	}

	// Add user message to history
	m.messages = append(m.messages, ChatMessage{
		Role:    "user",
		Content: userMsg,
	})
	m.updateViewport()

	// Clear input
	m.input.Reset()

	// Create messages slice for the daemon
	history := make([]ChatMessage, len(m.messages))
	copy(history, m.messages)

	// Return command to start streaming
	return m, func() tea.Msg {
		return m.startStreaming(history)
	}
}

// startStreaming initiates a streaming chat request to the daemon.
// @MX:NOTE This runs in a goroutine and sends events back to the model.
func (m *Model) startStreaming(messages []ChatMessage) tea.Msg {
	if m.client == nil {
		return StreamEventMsg{
			Event: StreamEvent{
				Type:    "error",
				Content: "no daemon client connected",
			},
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start streaming
	eventCh, err := m.client.ChatStream(ctx, messages)
	if err != nil {
		return StreamEventMsg{
			Event: StreamEvent{
				Type:    "error",
				Content: err.Error(),
			},
		}
	}

	// Pump events to bubbletea
	for event := range eventCh {
		return StreamEventMsg{Event: event}
	}

	return StreamEventMsg{
		Event: StreamEvent{Type: "done", Content: ""},
	}
}

// saveSession saves the current chat history to a session file.
// @MX:NOTE This is a placeholder for Phase D. Session persistence comes later.
func (m *Model) saveSession() tea.Cmd {
	return func() tea.Msg {
		// Placeholder: In Phase E, this will save to session file
		// For now, just return a status message
		return tea.Msg("[Session save not yet implemented]")
	}
}

// updateViewport refreshes the viewport content with current messages.
// @MX:NOTE This regenerates the viewport content from the messages slice.
func (m *Model) updateViewport() {
	// Build viewport content from messages
	content := ""
	for _, msg := range m.messages {
		role := msg.Role
		if role == "user" {
			role = "You"
		} else if role == "assistant" {
			role = "AI"
		}

		// Add role prefix with color if enabled
		if !m.noColor {
			var style lipgloss.Style
			if msg.Role == "user" {
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("86")) // Green
			} else if msg.Role == "assistant" {
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("228")) // Yellow
			} else {
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("241")) // Gray
			}
			content += style.Render(role+": ") + msg.Content + "\n\n"
		} else {
			content += role + ": " + msg.Content + "\n\n"
		}
	}

	m.viewport.SetContent(content)
	// Auto-scroll to bottom
	m.viewport.GotoBottom()
}

// max returns the maximum of two integers.
// @MX:NOTE Go 1.21 has min/max built-in, but we support 1.26.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
