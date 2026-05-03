// SPEC-GOOSE-CLI-001 Phase D — TUI dispatcher integration tests.
//
// COMMAND-001 (completed) provides the App.ProcessInput dispatcher; the
// TUI's dispatch.go layer wraps that contract for slash command pre-dispatch.
// This file covers the helpers and integration paths around AppInterface.
package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// stubApp is a minimal AppInterface used to drive dispatch.go from tests.
type stubApp struct {
	processFn func(ctx context.Context, input string) (*ProcessResult, error)
	calls     []string
}

func (s *stubApp) ProcessInput(ctx context.Context, input string) (*ProcessResult, error) {
	s.calls = append(s.calls, input)
	if s.processFn != nil {
		return s.processFn(ctx, input)
	}
	return &ProcessResult{Kind: ProcessProceed, Prompt: input}, nil
}

// RED #D1: nil App falls back to a synthetic ProcessProceed result so the
// caller can run the legacy slash handler without a dispatcher.
func TestDispatchInput_NilApp_FallsBackToProceed(t *testing.T) {
	t.Parallel()

	got, err := DispatchInput(nil, context.Background(), "/help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.Kind != ProcessProceed {
		t.Fatalf("expected ProcessProceed, got %+v", got)
	}
	if got.Prompt != "/help" {
		t.Errorf("Prompt = %q, want /help", got.Prompt)
	}
}

// RED #D2: when an App is supplied, DispatchInput defers to ProcessInput
// and surfaces its result + any error verbatim.
func TestDispatchInput_AppCalled(t *testing.T) {
	t.Parallel()

	want := &ProcessResult{Kind: ProcessLocal, Messages: []ProcessMessage{{Type: "message", Content: "ok"}}}
	app := &stubApp{processFn: func(_ context.Context, input string) (*ProcessResult, error) {
		if input != "/help" {
			t.Errorf("input = %q, want /help", input)
		}
		return want, nil
	}}

	got, err := DispatchInput(app, context.Background(), "/help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("DispatchInput must return the App result verbatim, got %+v", got)
	}
	if len(app.calls) != 1 || app.calls[0] != "/help" {
		t.Errorf("App must be invoked exactly once with the verbatim input, got %v", app.calls)
	}
}

// DispatchInput propagates App errors without wrapping.
func TestDispatchInput_PropagatesAppError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	app := &stubApp{processFn: func(_ context.Context, _ string) (*ProcessResult, error) {
		return nil, wantErr
	}}

	_, err := DispatchInput(app, context.Background(), "/explode")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped sentinel, got %v", err)
	}
}

// FormatLocalResult tolerates nil and empty results.
func TestFormatLocalResult_NilAndEmpty(t *testing.T) {
	t.Parallel()

	if got := FormatLocalResult(nil); got != "" {
		t.Errorf("nil result must format to empty string, got %q", got)
	}
	if got := FormatLocalResult(&ProcessResult{}); got != "" {
		t.Errorf("empty messages must format to empty string, got %q", got)
	}
}

// FormatLocalResult only forwards messages with Type == "message" and
// concatenates them with newlines, in submission order.
func TestFormatLocalResult_MessageFiltering(t *testing.T) {
	t.Parallel()

	res := &ProcessResult{
		Kind: ProcessLocal,
		Messages: []ProcessMessage{
			{Type: "message", Content: "alpha"},
			{Type: "telemetry", Content: "ignored"},
			{Type: "message", Content: "beta"},
		},
	}

	got := FormatLocalResult(res)
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "beta") {
		t.Errorf("must include all 'message' entries, got %q", got)
	}
	if strings.Contains(got, "ignored") {
		t.Errorf("non-'message' entries must be filtered, got %q", got)
	}
	// alpha must precede beta in output (preserves dispatcher order).
	if strings.Index(got, "alpha") >= strings.Index(got, "beta") {
		t.Errorf("ordering broken: alpha should precede beta in %q", got)
	}
}

// ShouldExit / ShouldAbort / GetPrompt reflect ProcessResult.Kind.
func TestProcessResultHelpers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		result     *ProcessResult
		wantExit   bool
		wantAbort  bool
		wantPrompt string
	}{
		{"nil", nil, false, false, ""},
		{"local", &ProcessResult{Kind: ProcessLocal, Prompt: "ignored"}, false, false, "ignored"},
		{"proceed", &ProcessResult{Kind: ProcessProceed, Prompt: "go"}, false, false, "go"},
		{"exit", &ProcessResult{Kind: ProcessExit}, true, false, ""},
		{"abort", &ProcessResult{Kind: ProcessAbort}, false, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShouldExit(tc.result); got != tc.wantExit {
				t.Errorf("ShouldExit = %v, want %v", got, tc.wantExit)
			}
			if got := ShouldAbort(tc.result); got != tc.wantAbort {
				t.Errorf("ShouldAbort = %v, want %v", got, tc.wantAbort)
			}
			if got := GetPrompt(tc.result); got != tc.wantPrompt {
				t.Errorf("GetPrompt = %q, want %q", got, tc.wantPrompt)
			}
		})
	}
}

// DispatchSlashCmd refuses to operate without an App and returns a clear
// error message — callers fall back to the legacy slash handler.
func TestDispatchSlashCmd_NilApp_Error(t *testing.T) {
	t.Parallel()

	_, _, err := DispatchSlashCmd(nil, context.Background(), SlashCmd{Name: "help"}, NewModel(nil, "", true))
	if err == nil {
		t.Fatal("nil App must produce an error")
	}
	if !strings.Contains(err.Error(), "no app") {
		t.Errorf("error must mention missing app, got %v", err)
	}
}

// RED #D3: DispatchSlashCmd reconstructs "/<name> <arg> <arg>" before
// invoking the dispatcher so users see the same input that they typed.
func TestDispatchSlashCmd_ReconstructsInput(t *testing.T) {
	t.Parallel()

	app := &stubApp{processFn: func(_ context.Context, input string) (*ProcessResult, error) {
		if input != "/save morning brew" {
			t.Errorf("reconstructed input = %q, want %q", input, "/save morning brew")
		}
		return &ProcessResult{Kind: ProcessLocal, Messages: []ProcessMessage{{Type: "message", Content: "saved"}}}, nil
	}}

	resp, result, err := DispatchSlashCmd(app, context.Background(), SlashCmd{Name: "save", Args: []string{"morning", "brew"}}, NewModel(nil, "", true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp, "saved") {
		t.Errorf("formatted response must include dispatcher message, got %q", resp)
	}
	if result == nil || result.Kind != ProcessLocal {
		t.Errorf("expected ProcessLocal result, got %+v", result)
	}
}

// DispatchSlashCmd wraps App-side errors with a "dispatch failed" prefix
// so callers can distinguish dispatcher outages from result-bearing errors.
func TestDispatchSlashCmd_PropagatesAppError(t *testing.T) {
	t.Parallel()

	app := &stubApp{processFn: func(_ context.Context, _ string) (*ProcessResult, error) {
		return nil, errors.New("dispatcher down")
	}}

	_, _, err := DispatchSlashCmd(app, context.Background(), SlashCmd{Name: "save"}, NewModel(nil, "", true))
	if err == nil {
		t.Fatal("expected wrapped error")
	}
	if !strings.Contains(err.Error(), "dispatch failed") {
		t.Errorf("expected 'dispatch failed' wrap, got %v", err)
	}
}

// RED #D4 (integration): the TUI Model uses the dispatcher path before
// falling back to the legacy slash handler. Sending Enter with /help routed
// through a stubApp must invoke ProcessInput exactly once and not produce
// a chat send.
func TestModel_EnterKey_RoutesSlashThroughDispatcher(t *testing.T) {
	t.Parallel()

	app := &stubApp{processFn: func(_ context.Context, input string) (*ProcessResult, error) {
		if !strings.HasPrefix(input, "/") {
			t.Errorf("non-slash input reached dispatcher: %q", input)
		}
		return &ProcessResult{Kind: ProcessLocal, Messages: []ProcessMessage{{Type: "message", Content: "help text"}}}, nil
	}}

	model := NewModel(nil, "test", true)
	model.app = app
	model.input.SetValue("/help")

	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = newModel.(*Model)

	if len(app.calls) != 1 || app.calls[0] != "/help" {
		t.Errorf("dispatcher must be invoked once with /help, got %v", app.calls)
	}
	if len(model.messages) == 0 {
		t.Fatal("expected dispatcher response to land in chat history")
	}
	last := model.messages[len(model.messages)-1]
	if last.Role != "system" || !strings.Contains(last.Content, "help text") {
		t.Errorf("expected system message with dispatcher text, got %+v", last)
	}
}

// RED #D5 (integration): ProcessExit kind makes the Model quit cleanly.
func TestModel_DispatchExit_Quits(t *testing.T) {
	t.Parallel()

	app := &stubApp{processFn: func(_ context.Context, _ string) (*ProcessResult, error) {
		return &ProcessResult{Kind: ProcessExit}, nil
	}}

	model := NewModel(nil, "test", true)
	model.app = app
	model.input.SetValue("/quit")

	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*Model)

	if !m.quitting {
		t.Error("ProcessExit must set quitting flag")
	}
	if cmd == nil {
		t.Error("ProcessExit must return a tea.Cmd (tea.Quit)")
	}
}

// RED #D6 (integration): ProcessAbort kind quietly drops the input without
// quitting or appending a chat entry.
func TestModel_DispatchAbort_QuietDrop(t *testing.T) {
	t.Parallel()

	app := &stubApp{processFn: func(_ context.Context, _ string) (*ProcessResult, error) {
		return &ProcessResult{Kind: ProcessAbort}, nil
	}}

	model := NewModel(nil, "test", true)
	model.app = app
	model.input.SetValue("/secret")

	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*Model)

	if m.quitting {
		t.Error("ProcessAbort must not quit")
	}
	if len(m.messages) != 0 {
		t.Errorf("ProcessAbort must not append chat history, got %+v", m.messages)
	}
	if m.input.Value() != "" {
		t.Errorf("input must be cleared after dispatch, got %q", m.input.Value())
	}
}
