// Package tui provides the interactive chat TUI using bubbletea.
package tui

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/modu-ai/goose/internal/cli/tui/editor"
	"github.com/modu-ai/goose/internal/cli/tui/i18n"
	"github.com/modu-ai/goose/internal/cli/tui/permission"
	"github.com/modu-ai/goose/internal/cli/tui/sessionmenu"
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

// StreamProgressMsg carries incremental streaming metrics for statusbar updates.
// @MX:NOTE: [AUTO] StreamProgressMsg tick — updates throughput at 4 Hz. REQ-CLITUI-011
type StreamProgressMsg struct {
	TokensDelta int
	Elapsed     time.Duration
}

// PricingConfig holds per-token pricing for cost estimation.
type PricingConfig struct {
	InputPerMillion  float64 // USD per 1M input tokens
	OutputPerMillion float64 // USD per 1M output tokens
}

// DaemonClient defines the interface the TUI needs from the transport layer.
// @MX:ANCHOR This abstraction allows mocking and testing without real gRPC connections.
// @MX:REASON fan_in >= 3: client.go (adapter), tui_test.go (mock), permission_test.go (mockPermClient)
type DaemonClient interface {
	ChatStream(ctx context.Context, messages []ChatMessage) (<-chan StreamEvent, error)
	Close() error
	// ResolvePermission records the user's decision for a tool permission request.
	// Returns true when the daemon acknowledged the decision.
	// SPEC-GOOSE-CLI-TUI-002 P3 AC-CLITUI-004, AC-CLITUI-005
	ResolvePermission(ctx context.Context, toolUseID, toolName, decision string) (bool, error)
}

// clockFn is the time source used for elapsed calculation.
// Overridable in tests for deterministic output.
type clockFn func() time.Time

// Model is the main bubbletea model for the TUI.
// @MX:ANCHOR This model manages all TUI state and implements tea.Model interface.
type Model struct {
	// Client connection
	client     DaemonClient // Daemon client interface
	daemonAddr string       // Daemon address

	// App integration
	app AppInterface // Dispatcher integration (may be nil)

	// Session state
	sessionName string        // Current session name
	messages    []ChatMessage // Chat history
	initialMsgs []ChatMessage // Messages loaded from /load, prepended on next ChatStream call

	// UI components
	input    textinput.Model    // Text input field (legacy single-line)
	editor   editor.EditorModel // Enhanced editor (single/multi-line)
	viewport viewport.Model     // Scrollable message area

	// Dimensions
	width  int // Terminal width
	height int // Terminal height

	// State flags
	streaming   bool // Currently streaming response
	quitting    bool // Exiting the TUI
	confirmQuit bool // Prompting for quit confirmation during stream
	noColor     bool // Disable colored output

	// streaming state
	streamStartTime   time.Time // When streaming started
	tokenCount        int       // Cumulative tokens received
	lastTickTime      time.Time // Last throughput calculation time
	currentThroughput float64   // Tokens per second (rolling estimate)

	// cost tracking (optional — graceful no-op when nil)
	pricing        map[string]PricingConfig // nil = no pricing configured
	cumulativeCost float64                  // Running cost estimate in USD

	// clock is used for elapsed time; defaults to time.Now, overridable in tests.
	clock clockFn

	// permission state — modal and persistent store for tool permission decisions.
	// SPEC-GOOSE-CLI-TUI-002 P3 AC-CLITUI-003/004/005/006
	permissionState permission.PermissionModel
	permStore       *permission.Store
	pendingChunks   []StreamEvent // buffered while modal is active

	// catalog holds locale-specific display strings. Loaded lazily in Init().
	// SPEC-GOOSE-CLI-TUI-003 P1 REQ-CLITUI3-001
	catalog i18n.Catalog

	// sessionMenuState is the Ctrl-R session picker overlay.
	// SPEC-GOOSE-CLI-TUI-003 P2 REQ-CLITUI3-003
	sessionMenuState sessionmenu.Model
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
		editor:      editor.New(),
		viewport:    vp,
		streaming:   false,
		quitting:    false,
		confirmQuit: false,
		noColor:     noColor,
		clock:       time.Now,
		// Pre-populate with the default (en) catalog so the model is usable
		// before Init() is called (e.g., in unit tests that call View() directly).
		// Init() will overwrite with the locale-appropriate catalog.
		catalog: i18n.Default(),
	}
}

// defaultPermStorePath returns the default permissions.json path (~/.goose/permissions.json).
func defaultPermStorePath() string {
	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return filepath.Join(home, ".goose", "permissions.json")
}

// Init returns the initial command.
// Implements tea.Model interface.
// @MX:ANCHOR This is called by bubbletea when the program starts.
func (m *Model) Init() tea.Cmd {
	// Initialise persistent permission store if not already set (e.g., by tests).
	if m.permStore == nil {
		m.permStore = permission.NewStore(defaultPermStorePath())
	}
	// Lazy-load locale catalog. Tests may pre-populate m.catalog to avoid FS access.
	if m.catalog.Lang == "" {
		m.catalog = i18n.Load()
	}
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

	case permission.ResolveMsg:
		return m.handleResolveMsg(msg)

	case sessionmenu.LoadMsg:
		// Session selected from Ctrl-R overlay.
		// SPEC-GOOSE-CLI-TUI-003 P2 REQ-CLITUI3-003, -008
		return m.handleSessionMenuLoad(msg)

	case sessionmenu.CloseMsg:
		// Overlay dismissed without selection.
		// SPEC-GOOSE-CLI-TUI-003 P2 REQ-CLITUI3-003, -008
		m.sessionMenuState = sessionmenu.New()
		return m, nil

	default:
		// Pass through to input and viewport
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}
