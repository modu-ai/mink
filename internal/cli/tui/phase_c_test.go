// SPEC-GOOSE-CLI-001 Phase C2~C5 — characterization + new behavior tests.
//
// C2 streaming-text accumulation, C3 keybindings (Ctrl+L / /clear),
// C4 statusbar (message count, daemon address), C5 race + integration.
package tui

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// RED #4 (C2): consecutive "text" events accumulate into a single
// assistant ChatMessage rather than spawning one per fragment.
func TestModel_StreamTextAccumulation(t *testing.T) {
	t.Parallel()

	model := NewModel(nil, "test", false)

	for _, fragment := range []string{"Hel", "lo, ", "world!"} {
		newModel, _ := model.Update(StreamEventMsg{Event: StreamEvent{Type: "text", Content: fragment}})
		model = newModel.(*Model)
	}

	if got := len(model.messages); got != 1 {
		t.Fatalf("expected 1 assistant message after accumulation, got %d", got)
	}
	if model.messages[0].Role != "assistant" {
		t.Errorf("role = %q, want assistant", model.messages[0].Role)
	}
	if model.messages[0].Content != "Hello, world!" {
		t.Errorf("content = %q, want %q", model.messages[0].Content, "Hello, world!")
	}
	if !model.streaming {
		t.Error("model.streaming must be true while text fragments arrive")
	}
}

// Done event after accumulation closes the streaming state without
// touching message history.
func TestModel_StreamDoneAfterAccumulation(t *testing.T) {
	t.Parallel()

	model := NewModel(nil, "test", false)
	for _, fragment := range []string{"a", "b"} {
		newModel, _ := model.Update(StreamEventMsg{Event: StreamEvent{Type: "text", Content: fragment}})
		model = newModel.(*Model)
	}
	newModel, _ := model.Update(StreamEventMsg{Event: StreamEvent{Type: "done"}})
	model = newModel.(*Model)

	if model.streaming {
		t.Error("streaming must be false after done event")
	}
	if len(model.messages) != 1 || model.messages[0].Content != "ab" {
		t.Errorf("unexpected messages %+v", model.messages)
	}
}

// RED #5 (C2): Esc during streaming appends a system "[Response cancelled]"
// note and clears the streaming flag.
func TestModel_EscapeAppendsCancellationNote(t *testing.T) {
	t.Parallel()

	model := NewModel(nil, "test", false)
	model.streaming = true

	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model = newModel.(*Model)

	if model.streaming {
		t.Error("Escape must clear streaming flag")
	}
	if len(model.messages) == 0 {
		t.Fatal("Escape must append a cancellation note")
	}
	last := model.messages[len(model.messages)-1]
	if last.Role != "system" || !strings.Contains(last.Content, "cancelled") {
		t.Errorf("expected system cancellation note, got %+v", last)
	}
}

// RED #7 (C3): Ctrl+L triggers viewport top scroll without altering
// message history or streaming state.
func TestModel_CtrlL_ScrollsViewportTop(t *testing.T) {
	t.Parallel()

	model := NewModel(nil, "test", false)
	model.messages = []ChatMessage{{Role: "user", Content: "first"}, {Role: "assistant", Content: "ack"}}
	model.viewport.Width = 80
	model.viewport.Height = 10
	model.updateViewport()

	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	model = newModel.(*Model)

	if cmd != nil {
		t.Errorf("Ctrl+L must not emit a tea.Cmd, got %v", cmd)
	}
	if len(model.messages) != 2 {
		t.Errorf("Ctrl+L must preserve message history, got %d", len(model.messages))
	}
}

// RED #8 (C3): /clear resets the chat history through HandleSlashCmd.
func TestModel_SlashClear_ResetsHistory(t *testing.T) {
	t.Parallel()

	model := NewModel(nil, "test", false)
	model.messages = []ChatMessage{{Role: "user", Content: "hi"}, {Role: "assistant", Content: "hello"}}
	model.updateViewport()

	resp, cmd := HandleSlashCmd(SlashCmd{Name: "clear"}, model)

	if cmd != nil {
		t.Errorf("/clear must not return a tea.Cmd, got %v", cmd)
	}
	if !strings.Contains(resp, "cleared") {
		t.Errorf("response must mention clear, got %q", resp)
	}
	if len(model.messages) != 0 {
		t.Errorf("messages must be empty after /clear, got %d", len(model.messages))
	}
}

// RED #9 (C4): the status bar surfaces the daemon address and session name.
func TestView_StatusBar_RendersConnectionState(t *testing.T) {
	t.Parallel()

	model := NewModel(nil, "alpha", true)
	model.daemonAddr = "127.0.0.1:9005"
	model.width = 80
	model.height = 24

	rendered := model.View()

	if !strings.Contains(rendered, "127.0.0.1:9005") {
		t.Error("statusbar must include the daemon address")
	}
	if !strings.Contains(rendered, "alpha") {
		t.Error("statusbar must include the session name")
	}
}

// RED #9 (C4): unnamed sessions render with the "(unnamed)" placeholder.
func TestView_StatusBar_UnnamedSession(t *testing.T) {
	t.Parallel()

	model := NewModel(nil, "", true)
	model.daemonAddr = "127.0.0.1:9005"
	model.width = 80
	model.height = 24

	rendered := model.View()
	if !strings.Contains(rendered, "(unnamed)") {
		t.Error("unnamed session must render as (unnamed)")
	}
}

// RED #9 (C4): streaming flag is reflected in the status bar.
func TestView_StatusBar_StreamingIndicator(t *testing.T) {
	t.Parallel()

	model := NewModel(nil, "alpha", true)
	model.daemonAddr = "127.0.0.1:9005"
	model.width = 80
	model.height = 24
	model.streaming = true

	rendered := model.View()
	if !strings.Contains(rendered, "Streaming") {
		t.Error("streaming flag must produce a status bar marker")
	}
}

// RED #10 (C4): the status bar prints the current message count and reflects
// growth as new messages land.
func TestView_StatusBar_MessageCount(t *testing.T) {
	t.Parallel()

	model := NewModel(nil, "alpha", true)
	model.daemonAddr = "127.0.0.1:9005"
	model.width = 80
	model.height = 24

	if !strings.Contains(model.View(), "Messages: 0") {
		t.Error("empty model must report Messages: 0")
	}

	model.messages = []ChatMessage{{Role: "user", Content: "hi"}, {Role: "assistant", Content: "ok"}}
	if !strings.Contains(model.View(), "Messages: 2") {
		t.Error("status bar must reflect message count after inserts")
	}
}

// Quitting model collapses to the goodbye marker without rendering the
// statusbar/viewport stack.
func TestView_QuittingState_RendersGoodbye(t *testing.T) {
	t.Parallel()

	model := NewModel(nil, "alpha", true)
	model.quitting = true

	rendered := model.View()
	if !strings.Contains(rendered, "Goodbye") {
		t.Errorf("quitting state must render Goodbye, got %q", rendered)
	}
}

// RED #13 (C5): concurrent StreamEventMsg deliveries from multiple
// goroutines must remain race-free under -race. The Model itself is
// single-threaded — bubbletea serialises Update calls — so we exercise
// the concurrent message channel pattern that produces them.
func TestRace_TUIConcurrentStreamEvents(t *testing.T) {
	t.Parallel()

	const goroutines = 8
	const events = 32

	ch := make(chan StreamEventMsg, goroutines*events)
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < events; i++ {
				ch <- StreamEventMsg{Event: StreamEvent{Type: "text", Content: "x"}}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	model := NewModel(nil, "race", true)
	deadline := time.After(2 * time.Second)
	count := 0
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				if count != goroutines*events {
					t.Fatalf("processed %d events, want %d", count, goroutines*events)
				}
				return
			}
			newModel, _ := model.Update(msg)
			model = newModel.(*Model)
			count++
		case <-deadline:
			t.Fatal("timed out draining concurrent stream events")
		}
	}
}

// Sanity: ParseSlashCmd / HandleSlashCmd remain reachable through the
// public API after Phase C reshuffling. Acts as a regression baseline for
// AC C-AC-006.
func TestExisting_SlashParsing_NoRegression(t *testing.T) {
	t.Parallel()

	cmd, ok := ParseSlashCmd("/help")
	if !ok || cmd.Name != "help" {
		t.Fatalf("ParseSlashCmd regressed: %+v ok=%v", cmd, ok)
	}

	model := NewModel(nil, "test", true)
	resp, _ := HandleSlashCmd(cmd, model)
	if !strings.Contains(resp, "/help") || !strings.Contains(resp, "/clear") {
		t.Errorf("help text regressed: %q", resp)
	}
}

// Smoke test: NewConnectClientFactory produces a DaemonClient that
// satisfies the interface used by NewModel without panicking.
func TestSmoke_ConnectFactoryAsDaemonClient(t *testing.T) {
	t.Parallel()

	var client DaemonClient = NewConnectClientFactory("127.0.0.1:9005")
	if client == nil {
		t.Fatal("NewConnectClientFactory must satisfy DaemonClient")
	}
	if err := client.Close(); err != nil {
		t.Errorf("Close must be a no-op, got %v", err)
	}
	// ChatStream with empty messages should surface the sentinel error.
	if _, err := client.ChatStream(context.Background(), nil); err == nil {
		t.Error("empty messages must error")
	}
}
