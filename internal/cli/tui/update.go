// Package tui provides the interactive chat TUI using bubbletea.
package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/modu-ai/goose/internal/cli/tui/permission"
	"github.com/modu-ai/goose/internal/cli/tui/sessionmenu"
)

// handleKeyMsg processes keyboard input.
// @MX:ANCHOR This function routes all key presses to appropriate handlers.
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Permission modal has input priority over all other handlers.
	// SPEC-GOOSE-CLI-TUI-002 P3 AC-CLITUI-003
	if m.permissionState.Active {
		var cmd tea.Cmd
		m.permissionState, cmd = m.permissionState.Update(msg)
		return m, cmd
	}

	// Sessionmenu overlay captures all input when open (2nd priority, after permission modal).
	// SPEC-GOOSE-CLI-TUI-003 P2 REQ-CLITUI3-008, -009
	if m.sessionMenuState.IsOpen() {
		var cmd tea.Cmd
		m.sessionMenuState, cmd = m.sessionMenuState.Update(msg)
		return m, cmd
	}

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

			// Try dispatcher first when App is available
			if m.app != nil {
				result, err := DispatchInput(m.app, context.Background(), input)
				if err == nil && result != nil {
					m.input.Reset()

					switch result.Kind {
					case ProcessExit:
						m.quitting = true
						return m, tea.Quit
					case ProcessAbort:
						return m, nil
					case ProcessLocal:
						// Display local response
						if response := FormatLocalResult(result); response != "" {
							m.messages = append(m.messages, ChatMessage{
								Role:    "system",
								Content: response,
							})
							m.updateViewport()
						}
						return m, nil
					case ProcessProceed:
						// Fall through to send message with (possibly expanded) prompt
						if result.Prompt != "" {
							m.input.SetValue(result.Prompt)
						}
						return m.sendMessage()
					}
				}
				// Dispatcher error: fall through to legacy handling
			}

			// Legacy slash command handling (no App or dispatcher failed)
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

	case tea.KeyCtrlN:
		// Ctrl+N toggles editor mode (single ↔ multi-line), syncing buffer from legacy input.
		// Sync current legacy input value to editor before toggle.
		currentVal := m.input.Value()
		if currentVal != "" {
			m.editor = m.editor.SetValue(currentVal)
		}
		m.editor, _ = m.editor.Update(msg)
		// Keep legacy input in sync.
		m.input.SetValue(m.editor.Value())
		return m, nil

	case tea.KeyCtrlS:
		// Ctrl+S saves session
		return m, m.saveSession()

	case tea.KeyCtrlL:
		// Ctrl+L clears screen
		m.viewport.GotoTop()
		return m, nil

	case tea.KeyCtrlR:
		// Ctrl+R opens the session picker overlay (permission modal takes priority above).
		// SPEC-GOOSE-CLI-TUI-003 P2 REQ-CLITUI3-003, -004
		entries := sessionmenu.Load()
		m.sessionMenuState = sessionmenu.Open(entries)
		if len(entries) == 0 {
			// Auto-dismiss: queue a CloseMsg so the overlay is closed in the next Update cycle.
			return m, func() tea.Msg { return sessionmenu.CloseMsg{} }
		}
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

// permissionRequestPayload is the JSON payload from a permission_request stream event.
type permissionRequestPayload struct {
	ToolName  string `json:"tool_name"`
	ToolUseID string `json:"tool_use_id"`
}

// handleStreamEvent processes events from the daemon stream.
// @MX:ANCHOR This function handles all streaming response events.
// @MX:REASON fan_in >= 3: text/error/done/permission_request all route here; pause/resume branch added.
func (m *Model) handleStreamEvent(msg StreamEventMsg) (tea.Model, tea.Cmd) {
	switch msg.Event.Type {
	case "permission_request":
		return m.handlePermissionRequest(msg.Event)

	case "text":
		// If modal is active, buffer the chunk instead of rendering it.
		if m.permissionState.Active {
			m.pendingChunks = append(m.pendingChunks, msg.Event)
			return m, nil
		}
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

// handlePermissionRequest processes a permission_request stream event.
// If the tool has a persisted allow decision, the request is resolved automatically
// without showing the modal (fast-path).
// SPEC-GOOSE-CLI-TUI-002 P3 AC-CLITUI-004
func (m *Model) handlePermissionRequest(event StreamEvent) (tea.Model, tea.Cmd) {
	var payload permissionRequestPayload
	if err := json.Unmarshal([]byte(event.Content), &payload); err != nil {
		// Malformed payload: ignore and continue streaming.
		return m, nil
	}

	// Fast-path: tool has a persisted allow decision.
	if m.permStore != nil {
		if decision, ok := m.permStore.Has(payload.ToolName); ok && decision == permission.DecisionAllowAlways {
			// Auto-resolve without showing modal via tea.Cmd (no goroutine leak).
			client := m.client
			toolUseID := payload.ToolUseID
			toolName := payload.ToolName
			return m, func() tea.Msg {
				if client != nil {
					_, _ = client.ResolvePermission(context.Background(), toolUseID, toolName, "allow_always")
				}
				return nil
			}
		}
	}

	// Show modal for user decision.
	m.permissionState = permission.PermissionModel{
		Active:    true,
		ToolName:  payload.ToolName,
		ToolUseID: payload.ToolUseID,
	}
	return m, nil
}

// handleResolveMsg processes the ResolveMsg produced by the permission modal.
// It calls the ResolvePermission RPC, persists allow_always decisions, and
// flushes any buffered stream chunks.
// SPEC-GOOSE-CLI-TUI-002 P3 AC-CLITUI-004, AC-CLITUI-005, AC-CLITUI-006
func (m *Model) handleResolveMsg(msg permission.ResolveMsg) (tea.Model, tea.Cmd) {
	m.permissionState.Active = false

	// Call ResolvePermission RPC via tea.Cmd to stay on the bubbletea
	// message loop — avoids goroutine-vs-test race conditions.
	resolveCmd := func() tea.Msg {
		if m.client != nil {
			_, _ = m.client.ResolvePermission(context.Background(), msg.ToolUseID, msg.ToolName, msg.ModalDecision)
		}
		return nil
	}

	// Persist allow_always decisions to disk.
	if msg.ModalDecision == "allow_always" && m.permStore != nil {
		_ = m.permStore.Save(msg.ToolName, permission.DecisionAllowAlways)
	}

	// Flush buffered chunks in arrival order.
	if len(m.pendingChunks) > 0 {
		for _, chunk := range m.pendingChunks {
			if len(m.messages) == 0 || m.messages[len(m.messages)-1].Role != "assistant" {
				m.messages = append(m.messages, ChatMessage{
					Role:    "assistant",
					Content: chunk.Content,
				})
			} else {
				m.messages[len(m.messages)-1].Content += chunk.Content
			}
		}
		m.pendingChunks = nil
		m.updateViewport()
	}

	return m, resolveCmd
}

// handleSessionMenuLoad processes a sessionmenu.LoadMsg emitted when the user
// presses Enter in the Ctrl-R overlay. Closes the overlay, loads the selected
// session, and appends a status message.
// SPEC-GOOSE-CLI-TUI-003 P2 REQ-CLITUI3-003, -008
func (m *Model) handleSessionMenuLoad(msg sessionmenu.LoadMsg) (tea.Model, tea.Cmd) {
	m.sessionMenuState = sessionmenu.New() // close overlay
	if m.streaming {
		// Cannot load a session while streaming; show error hint.
		m.messages = append(m.messages, ChatMessage{
			Role:    "system",
			Content: "[Cannot load session while streaming]",
		})
		m.updateViewport()
		return m, nil
	}
	status := m.loadSessionByName(msg.Name)
	m.messages = append(m.messages, ChatMessage{Role: "system", Content: status})
	m.updateViewport()
	return m, nil
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
// Assistant messages are rendered through glamour for markdown formatting.
func (m *Model) updateViewport() {
	var sb strings.Builder
	for _, msg := range m.messages {
		role := msg.Role
		switch role {
		case "user":
			role = "You"
		case "assistant":
			role = "AI"
		}

		content := msg.Content

		// Render assistant messages through glamour for markdown support.
		if msg.Role == "assistant" {
			content = renderMarkdown(content)
		}

		if !m.noColor {
			var style lipgloss.Style
			switch msg.Role {
			case "user":
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("86")) // Green
			case "assistant":
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("228")) // Yellow
			default:
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("241")) // Gray
			}
			sb.WriteString(style.Render(role+": ") + content + "\n\n")
		} else {
			sb.WriteString(role + ": " + content + "\n\n")
		}
	}

	m.viewport.SetContent(sb.String())
	m.viewport.GotoBottom()
}

// renderMarkdown renders markdown content using glamour with ascii style.
// Returns the original content on error to avoid losing information.
func renderMarkdown(content string) string {
	rendered, err := glamour.Render(content, "ascii")
	if err != nil {
		return content
	}
	return rendered
}
