package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/modu-ai/mink/internal/ritual/briefing"
)

// BriefingRunner is the minimal interface the TUI needs to execute a briefing
// pipeline. It is satisfied by *briefing.Orchestrator and is defined here so
// that tests can substitute lightweight mocks without importing the full
// orchestrator dependency.
//
// REQ-BR-062: /briefing slash command triggers orchestrator via this interface.
//
// @MX:ANCHOR: BriefingRunner is the TUI-side interface for the briefing pipeline.
// @MX:REASON: [AUTO] fan_in >= 3: briefing_slash_test.go (mock), slash.go (dispatch), model.go (field). REQ-BR-062.
type BriefingRunner interface {
	Run(ctx context.Context, userID string, today time.Time) (*briefing.BriefingPayload, error)
}

// BriefingResultMsg is the bubbletea message delivered back to Update() when
// the briefing pipeline goroutine completes. Either Output or Err is set.
//
// REQ-BR-033 / REQ-BR-062 / AC-016.
type BriefingResultMsg struct {
	// Output holds the rendered briefing text on success.
	Output string
	// Err holds the pipeline error on failure.
	Err error
}

// briefingRunCmd executes the briefing pipeline in a goroutine and delivers
// BriefingResultMsg to the bubbletea runtime when done. This keeps
// HandleSlashCmd non-blocking (AC-016).
//
// REQ-BR-062: the tea.Cmd is returned immediately; the runner call is deferred.
func briefingRunCmd(runner BriefingRunner, userID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		payload, err := runner.Run(ctx, userID, time.Now())
		if err != nil {
			return BriefingResultMsg{Err: fmt.Errorf("briefing pipeline: %w", err)}
		}

		// NewBriefingPanel is in this (tui) package; call without qualifier.
		panel := NewBriefingPanel(payload)
		return BriefingResultMsg{Output: panel.Render()}
	}
}
