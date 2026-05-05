// Package tui provides shared session loading helpers.
package tui

import (
	"fmt"

	"github.com/modu-ai/goose/internal/cli/session"
)

// loadSessionByName loads a session into the model and returns the status message.
// Reused by both /load slash command and sessionmenu Enter handler to avoid
// duplication. AC-CLITUI-013, AC-CLITUI3-002.
// @MX:ANCHOR loadSessionByName is the shared session-restore path.
// @MX:REASON fan_in >= 3: handleLoad (slash.go), handleSessionMenuLoad (update.go), session_test.go.
func (m *Model) loadSessionByName(name string) string {
	msgs, err := session.Load(name)
	if err != nil {
		return fmt.Sprintf("[Error loading session: %v]", err)
	}

	chatMsgs := make([]ChatMessage, 0, len(msgs))
	for _, sm := range msgs {
		chatMsgs = append(chatMsgs, ChatMessage{
			Role:    sm.Role,
			Content: sm.Content,
		})
	}

	// Restore messages into model.
	m.messages = chatMsgs
	// Store as initialMsgs for next ChatStream call.
	m.initialMsgs = make([]ChatMessage, len(chatMsgs))
	copy(m.initialMsgs, chatMsgs)

	m.updateViewport()

	return fmt.Sprintf(m.catalog.Loaded, name, len(chatMsgs))
}
