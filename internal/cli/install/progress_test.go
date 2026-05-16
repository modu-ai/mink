// Package install — progress_test.go tests the bubbletea progress UI helpers.
//
// SPEC: SPEC-MINK-ONBOARDING-001 §6 (Phase 2C polish)
package install

import (
	"context"
	"errors"
	"testing"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/modu-ai/mink/internal/onboarding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errFakePull is a sentinel error used in pull failure tests.
var errFakePull = errors.New("fake pull error")

// -----------------------------------------------------------------------
// TestFormatBytes_Table — unit tests for byte formatter
// -----------------------------------------------------------------------

func TestFormatBytes_Table(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{999, "999 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},    // 1.5 * 1024
		{1048576, "1.0 MB"}, // 1 MB
		{int64(1.5 * 1024 * 1024 * 1024), "1.5 GB"},
		{int64(1024 * 1024 * 1024 * 1024), "1.0 TB"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := formatBytes(tc.input)
			assert.Equal(t, tc.want, got, "formatBytes(%d)", tc.input)
		})
	}
}

// -----------------------------------------------------------------------
// TestShortLayer_Table — unit tests for layer digest shortener
// -----------------------------------------------------------------------

func TestShortLayer_Table(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sha256:abc1234567890", "abc1234"},
		{"abc1234", "abc1234"},
		{"", ""},
		{"sha256:", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := shortLayer(tc.input)
			assert.Equal(t, tc.want, got, "shortLayer(%q)", tc.input)
		})
	}
}

// -----------------------------------------------------------------------
// helpers to build a minimal pullProgressModel for unit tests
// -----------------------------------------------------------------------

func newTestProgressModel() pullProgressModel {
	ch := make(chan onboarding.ProgressUpdate, 8)
	done := make(chan error, 1)
	s := spinner.New()
	bar := progress.New(progress.WithoutPercentage())
	return pullProgressModel{
		spinner:   s,
		bar:       bar,
		styles:    NewMINKStyles(),
		modelName: "test-model",
		updateCh:  ch,
		doneCh:    done,
	}
}

// -----------------------------------------------------------------------
// TestPullProgressModel_HandlesProgressUpdate
// -----------------------------------------------------------------------

// TestPullProgressModel_HandlesProgressUpdate verifies that a progressUpdateMsg
// correctly updates the model's internal state fields.
func TestPullProgressModel_HandlesProgressUpdate(t *testing.T) {
	m := newTestProgressModel()

	update := onboarding.ProgressUpdate{
		Phase:       "downloading layer",
		Layer:       "sha256:abc1234567890",
		BytesTotal:  1024 * 1024,
		BytesDone:   512 * 1024,
		PercentDone: 50.0,
	}

	newModel, _ := m.Update(progressUpdateMsg{update: update})
	result, ok := newModel.(pullProgressModel)
	require.True(t, ok, "Update should return pullProgressModel")

	assert.Equal(t, "downloading layer", result.phase)
	assert.Equal(t, "abc1234", result.layer, "layer should be shortened to 7 chars")
	assert.Equal(t, int64(1024*1024), result.bytesTotal)
	assert.Equal(t, int64(512*1024), result.bytesDone)
	assert.InDelta(t, 50.0, result.percent, 0.001)
}

// -----------------------------------------------------------------------
// TestPullProgressModel_HandlesPullDoneMsg_Success
// -----------------------------------------------------------------------

// TestPullProgressModel_HandlesPullDoneMsg_Success verifies that a pullDoneMsg
// with no error marks the model as done and returns tea.Quit.
func TestPullProgressModel_HandlesPullDoneMsg_Success(t *testing.T) {
	m := newTestProgressModel()

	newModel, cmd := m.Update(pullDoneMsg{err: nil})
	result, ok := newModel.(pullProgressModel)
	require.True(t, ok, "Update should return pullProgressModel")

	assert.True(t, result.done, "model should be marked done")
	assert.NoError(t, result.err, "error should be nil on success")

	// cmd should return tea.QuitMsg when called.
	require.NotNil(t, cmd, "Quit command should not be nil")
	msg := cmd()
	_, isQuit := msg.(tea.QuitMsg)
	assert.True(t, isQuit, "command should return tea.QuitMsg")
}

// -----------------------------------------------------------------------
// TestPullProgressModel_HandlesPullDoneMsg_Error
// -----------------------------------------------------------------------

// TestPullProgressModel_HandlesPullDoneMsg_Error verifies that a pullDoneMsg
// with an error stores the error and returns tea.Quit.
func TestPullProgressModel_HandlesPullDoneMsg_Error(t *testing.T) {
	m := newTestProgressModel()

	newModel, cmd := m.Update(pullDoneMsg{err: errFakePull})
	result, ok := newModel.(pullProgressModel)
	require.True(t, ok, "Update should return pullProgressModel")

	assert.True(t, result.done, "model should be marked done")
	assert.ErrorIs(t, result.err, errFakePull, "error should be stored")

	require.NotNil(t, cmd, "Quit command should not be nil")
	msg := cmd()
	_, isQuit := msg.(tea.QuitMsg)
	assert.True(t, isQuit, "command should return tea.QuitMsg")
}

// -----------------------------------------------------------------------
// TestRunPullWithProgress_ContextCancellation
// -----------------------------------------------------------------------

// TestRunPullWithProgress_ContextCancellation verifies that cancelling the
// context causes the pull to eventually complete with a context error.
// This test exercises the pullModelFunc substitution path only — it does not
// launch a real bubbletea TUI.
func TestRunPullWithProgress_ContextCancellation(t *testing.T) {
	// Substitute pullModelFunc with a function that blocks until context is done,
	// then returns the context error.
	origPull := pullModelFunc
	t.Cleanup(func() { pullModelFunc = origPull })

	pullModelFunc = func(ctx context.Context, _ string, _ chan<- onboarding.ProgressUpdate) error {
		<-ctx.Done()
		return ctx.Err()
	}

	// Substitute runProgramFunc to return a model that signals cancellation,
	// without starting a real terminal program.
	origRun := runProgramFunc
	t.Cleanup(func() { runProgramFunc = origRun })

	runProgramFunc = func(_ *tea.Program) (tea.Model, error) {
		// Simulate the model after a cancelled download.
		m := newTestProgressModel()
		m.done = true
		m.err = context.Canceled
		return m, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so pullModelFunc unblocks right away

	err := RunPullWithProgress(ctx, "test-model:latest")
	// RunPullWithProgress should propagate the cancellation error from the model.
	assert.Error(t, err, "RunPullWithProgress should return an error when cancelled")
}

// -----------------------------------------------------------------------
// TestRunPullWithProgress_FastSuccess
// -----------------------------------------------------------------------

// TestRunPullWithProgress_FastSuccess verifies that RunPullWithProgress returns
// nil when PullModel succeeds and the program exits cleanly.
func TestRunPullWithProgress_FastSuccess(t *testing.T) {
	// Substitute pullModelFunc with a function that emits 3 updates then succeeds.
	origPull := pullModelFunc
	t.Cleanup(func() { pullModelFunc = origPull })

	pullModelFunc = func(_ context.Context, _ string, ch chan<- onboarding.ProgressUpdate) error {
		for i := 1; i <= 3; i++ {
			ch <- onboarding.ProgressUpdate{
				Phase:       "downloading layer",
				PercentDone: float64(i) * 33.0,
				BytesDone:   int64(i) * 1024,
				BytesTotal:  3 * 1024,
			}
		}
		return nil
	}

	// Substitute runProgramFunc with a no-op that returns a successful "done" model
	// without starting a real terminal program.
	origRun := runProgramFunc
	t.Cleanup(func() { runProgramFunc = origRun })

	runProgramFunc = func(_ *tea.Program) (tea.Model, error) {
		// Simulate the model after a successful download completes.
		m := newTestProgressModel()
		m.done = true
		m.err = nil
		return m, nil
	}

	err := RunPullWithProgress(context.Background(), "test-model:latest")
	assert.NoError(t, err, "RunPullWithProgress should return nil on success")
}
