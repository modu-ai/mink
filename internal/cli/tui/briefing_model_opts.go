package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// WithBriefingRunner wires a BriefingRunner into the Model so that the
// /briefing slash command can execute the pipeline asynchronously.
//
// REQ-BR-062 / AC-016 / T-308.
func (m *Model) WithBriefingRunner(runner BriefingRunner) *Model {
	m.briefingRunner = runner
	return m
}

// WithUserID sets the user identifier used by the briefing pipeline.
//
// REQ-BR-062 / T-308.
func (m *Model) WithUserID(userID string) *Model {
	m.userID = userID
	return m
}

// handleBriefingResult processes a BriefingResultMsg delivered by the async
// briefing pipeline goroutine. On success, appends the rendered panel as a
// system message. On error, appends a user-facing error notice.
//
// REQ-BR-033 / REQ-BR-062 / AC-016.
func (m *Model) handleBriefingResult(msg BriefingResultMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.messages = append(m.messages, ChatMessage{
			Role:    "system",
			Content: fmt.Sprintf("[Briefing failed: %v]", msg.Err),
		})
	} else {
		m.messages = append(m.messages, ChatMessage{
			Role:    "system",
			Content: msg.Output,
		})
	}
	m.updateViewport()
	return m, nil
}
