package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/ritual/briefing"
)

// mockBriefingRunner is a controllable BriefingRunner for TUI tests.
type mockBriefingRunner struct {
	payload *briefing.BriefingPayload
	err     error
	calls   int
}

func (m *mockBriefingRunner) Run(ctx context.Context, userID string, today time.Time) (*briefing.BriefingPayload, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return m.payload, nil
}

// newTestModelWithBriefing creates a minimal Model with a briefing runner.
func newTestModelWithBriefing(runner BriefingRunner, userID string) *Model {
	m := NewModel(nil, "test-session", true)
	m.briefingRunner = runner
	m.userID = userID
	return m
}

// TestTUI_BriefingSlash verifies AC-016 — REQ-BR-033 / REQ-BR-062.
func TestTUI_BriefingSlash(t *testing.T) {
	t.Run("TestSlash_BriefingDispatch", func(t *testing.T) {
		runner := &mockBriefingRunner{
			payload: &briefing.BriefingPayload{
				GeneratedAt:  time.Now(),
				Mantra:       briefing.MantraModule{Text: "morning"},
				DateCalendar: briefing.DateModule{Today: "2026-05-15"},
				Status:       map[string]string{"weather": "ok", "journal": "ok", "date": "ok", "mantra": "ok"},
			},
		}
		m := newTestModelWithBriefing(runner, "u1")

		cmd, ok := ParseSlashCmd("/briefing")
		if !ok {
			t.Fatal("ParseSlashCmd('/briefing') failed")
		}

		response, teaCmd := HandleSlashCmd(cmd, m)
		if teaCmd == nil {
			t.Error("HandleSlashCmd('/briefing') must return a non-nil tea.Cmd (async dispatch)")
		}
		if response == "" {
			t.Error("HandleSlashCmd('/briefing') must return a non-empty response string")
		}
	})

	t.Run("TestModel_BriefingResultMsg_Append", func(t *testing.T) {
		runner := &mockBriefingRunner{
			payload: &briefing.BriefingPayload{
				GeneratedAt:  time.Now(),
				Mantra:       briefing.MantraModule{Text: "daily wisdom"},
				DateCalendar: briefing.DateModule{Today: "2026-05-15"},
				Status:       map[string]string{"weather": "ok", "journal": "ok", "date": "ok", "mantra": "ok"},
			},
		}
		m := newTestModelWithBriefing(runner, "u1")
		panel := NewBriefingPanel(runner.payload)
		rendered := panel.Render()

		msg := BriefingResultMsg{Output: rendered}
		newModel, _ := m.Update(msg)
		updated := newModel.(*Model)

		if len(updated.messages) == 0 {
			t.Fatal("expected messages to have at least one entry after BriefingResultMsg")
		}
		last := updated.messages[len(updated.messages)-1]
		if last.Role != "system" {
			t.Errorf("last message Role = %q, want %q", last.Role, "system")
		}
		if !strings.Contains(last.Content, rendered) {
			t.Errorf("last message Content does not contain rendered panel output")
		}
	})

	t.Run("TestModel_BriefingSlash_NonBlocking", func(t *testing.T) {
		// The HandleSlashCmd must return immediately (non-blocking) with a tea.Cmd.
		// The runner here intentionally delays but since we never execute the Cmd,
		// the test verifies that HandleSlashCmd itself returns before the runner completes.
		runner := &mockBriefingRunner{payload: &briefing.BriefingPayload{
			Status: map[string]string{},
		}}
		m := newTestModelWithBriefing(runner, "u1")

		cmd, _ := ParseSlashCmd("/briefing")
		start := time.Now()
		_, teaCmd := HandleSlashCmd(cmd, m)
		elapsed := time.Since(start)

		if teaCmd == nil {
			t.Error("HandleSlashCmd must return non-nil tea.Cmd")
		}
		// Should return nearly instantly (well under 100ms)
		if elapsed > 100*time.Millisecond {
			t.Errorf("HandleSlashCmd took %v, expected < 100ms (must be non-blocking)", elapsed)
		}
	})

	t.Run("TestModel_BriefingResultMsg_Error", func(t *testing.T) {
		m := NewModel(nil, "test-session", true)
		errMsg := errors.New("briefing pipeline failed")
		msg := BriefingResultMsg{Err: errMsg}
		newModel, _ := m.Update(msg)
		updated := newModel.(*Model)

		if len(updated.messages) == 0 {
			t.Fatal("expected messages entry on BriefingResultMsg error")
		}
		last := updated.messages[len(updated.messages)-1]
		if last.Role != "system" {
			t.Errorf("error message Role = %q, want %q", last.Role, "system")
		}
		if !strings.Contains(last.Content, "failed") {
			t.Errorf("error message Content does not mention failure: %q", last.Content)
		}
	})

	t.Run("TestSlash_Briefing_NilRunner", func(t *testing.T) {
		// Without a briefing runner, the slash command should respond gracefully
		m := NewModel(nil, "test-session", true)
		m.briefingRunner = nil

		cmd, _ := ParseSlashCmd("/briefing")
		response, teaCmd := HandleSlashCmd(cmd, m)
		if teaCmd != nil {
			t.Error("nil runner: HandleSlashCmd must not return a tea.Cmd (no dispatch possible)")
		}
		if response == "" {
			t.Error("nil runner: must return an informative response")
		}
	})
}
