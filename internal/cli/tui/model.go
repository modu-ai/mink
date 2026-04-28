// Package tui provides the interactive chat TUI using bubbletea.
package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// ChatMessage represents a chat message with role and content.
// @MX:ANCHOR This type is the core data structure for chat messages.
type ChatMessage struct {
	Role    string // "user", "assistant", "system"
	Content string
}

// StreamEvent represents a streaming response event from the daemon.
// @MX:ANCHOR This type defines the contract for streaming events.
type StreamEvent struct {
	Type    string // "text", "error", "done"
	Content string
}

// StreamEventMsg is a bubbletea message that wraps StreamEvent.
// @MX:NOTE This bridges bubbletea Msg system with our stream events.
type StreamEventMsg struct {
	Event StreamEvent
}

// DaemonClient defines the interface the TUI needs from the transport layer.
// @MX:ANCHOR This abstraction allows mocking and testing without real gRPC connections.
type DaemonClient interface {
	ChatStream(ctx context.Context, messages []ChatMessage) (<-chan StreamEvent, error)
	Close() error
}

// Model is the main bubbletea model for the TUI.
// @MX:ANCHOR This model manages all TUI state and implements tea.Model interface.
type Model struct {
	// Client connection
	client      DaemonClient // Daemon client interface
	daemonAddr  string       // Daemon address

	// App integration
	app AppInterface // Dispatcher integration (may be nil)

	// Session state
	sessionName string // Current session name
	messages    []ChatMessage // Chat history

	// UI components
	input    textinput.Model // Text input field
	viewport viewport.Model  // Scrollable message area

	// Dimensions
	width  int // Terminal width
	height int // Terminal height

	// State flags
	streaming   bool // Currently streaming response
	quitting    bool // Exiting the TUI
	confirmQuit bool // Prompting for quit confirmation during stream
	noColor     bool // Disable colored output
}

// NewModel creates a new TUI model with default state.
// @MX:ANCHOR This is the primary constructor for the TUI model.
func NewModel(client DaemonClient, sessionName string, noColor bool) *Model {
	// Initialize text input
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()

	// Initialize viewport
	vp := viewport.New(0, 0)

	return &Model{
		client:      client,
		sessionName: sessionName,
		messages:    make([]ChatMessage, 0),
		input:       ti,
		viewport:    vp,
		streaming:   false,
		quitting:    false,
		confirmQuit: false,
		noColor:     noColor,
	}
}

// Init returns the initial command.
// Implements tea.Model interface.
// @MX:ANCHOR This is called by bubbletea when the program starts.
func (m *Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles incoming messages and updates model state.
// Implements tea.Model interface.
// @MX:ANCHOR This is the core event handler for the TUI.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.handleWindowSize(msg)
		return m, nil

	case StreamEventMsg:
		return m.handleStreamEvent(msg)

	default:
		// Pass through to input and viewport
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

