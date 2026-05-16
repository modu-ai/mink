// Package install — progress.go implements the bubbletea progress UI shown
// while PullModel downloads a model from Ollama.
//
// RunPullWithProgress replaces the silent buffered+drain pattern introduced in
// Phase 2A. Users now see a live spinner, animated progress bar, bytes
// done/total, and a phase label while the download runs.
//
// SPEC: SPEC-MINK-ONBOARDING-001 §6 (Phase 2C polish)
package install

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/modu-ai/mink/internal/onboarding"
)

// pullModelFunc is the indirection var for onboarding.PullModel.
// Tests substitute a fake implementation here to avoid real Ollama calls.
//
// @MX:NOTE: [AUTO] Indirection var enables test injection without changing public API.
var pullModelFunc = onboarding.PullModel

// runProgramFunc is the indirection var for (*tea.Program).Run.
// Tests can substitute a no-op to avoid launching a real terminal program.
var runProgramFunc = func(p *tea.Program) (tea.Model, error) {
	return p.Run()
}

// progressUpdateMsg carries a single ProgressUpdate from the pull goroutine
// into the bubbletea event loop.
type progressUpdateMsg struct {
	update onboarding.ProgressUpdate
}

// pullDoneMsg signals that the pull goroutine has finished (success or error).
type pullDoneMsg struct {
	err error
}

// pullProgressModel is the bubbletea Model for the download UI.
type pullProgressModel struct {
	spinner    spinner.Model
	bar        progress.Model
	styles     MINKStyles
	modelName  string
	phase      string
	layer      string
	bytesDone  int64
	bytesTotal int64
	percent    float64
	done       bool
	err        error
	cancelFn   context.CancelFunc
	// updateCh receives progress updates from the pull goroutine.
	updateCh <-chan onboarding.ProgressUpdate
	// doneCh receives the final pull error (buffered 1) — read by waitForDone cmd.
	doneCh <-chan error
}

// Init satisfies tea.Model. Kicks off the spinner tick, update reader, and done waiter.
func (m pullProgressModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, waitForUpdate(m.updateCh), waitForDone(m.doneCh))
}

// waitForUpdate returns a tea.Cmd that reads one update from the channel.
// Returns nil when the channel is closed (signals end of stream).
func waitForUpdate(ch <-chan onboarding.ProgressUpdate) tea.Cmd {
	return func() tea.Msg {
		u, ok := <-ch
		if !ok {
			// Channel closed — pull goroutine has finished; done msg will arrive separately.
			return nil
		}
		return progressUpdateMsg{update: u}
	}
}

// waitForDone returns a tea.Cmd that reads the final pull error from doneCh.
func waitForDone(ch <-chan error) tea.Cmd {
	return func() tea.Msg {
		err := <-ch
		return pullDoneMsg{err: err}
	}
}

// Update handles incoming tea.Msg events.
func (m pullProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case progressUpdateMsg:
		u := msg.update
		m.phase = u.Phase
		if u.Layer != "" {
			m.layer = shortLayer(u.Layer)
		}
		if u.BytesTotal > 0 {
			m.bytesTotal = u.BytesTotal
		}
		if u.BytesDone > 0 {
			m.bytesDone = u.BytesDone
		}
		if u.PercentDone >= 0 {
			m.percent = u.PercentDone
		}

		var cmds []tea.Cmd
		if u.PercentDone >= 0 {
			cmds = append(cmds, m.bar.SetPercent(u.PercentDone/100.0))
		}
		// Continue reading the channel.
		cmds = append(cmds, waitForUpdate(m.updateCh))
		return m, tea.Batch(cmds...)

	case pullDoneMsg:
		m.done = true
		m.err = msg.err
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		newBar, cmd := m.bar.Update(msg)
		if b, ok := newBar.(progress.Model); ok {
			m.bar = b
		}
		return m, cmd

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			if m.cancelFn != nil {
				m.cancelFn()
			}
			return m, tea.Quit
		}
	}

	return m, nil
}

// View renders the current download state.
//
// Layout:
//
//	<spinner> <phase> [<layer>]
//	<progress bar>  <bytes-done>/<bytes-total>  <pct>%
//	<muted: model name>
func (m pullProgressModel) View() string {
	if m.done {
		if m.err != nil {
			return m.styles.Error.Render("  Download failed: "+m.err.Error()) + "\n"
		}
		return m.styles.Success.Render("  Download complete.") + "\n"
	}

	var sb strings.Builder

	// Line 1: spinner + phase + layer
	spinnerStr := m.spinner.View()
	phaseStr := m.phase
	if phaseStr == "" {
		phaseStr = "connecting"
	}
	line1 := spinnerStr + " " + m.styles.Accent.Render(phaseStr)
	if m.layer != "" {
		line1 += " " + m.styles.Muted.Render("["+m.layer+"]")
	}
	sb.WriteString(line1 + "\n")

	// Line 2: progress bar + bytes counter
	barStr := m.bar.View()
	bytesStr := formatBytes(m.bytesDone) + " / " + formatBytes(m.bytesTotal)
	pctStr := fmt.Sprintf("%.0f%%", m.percent)
	sb.WriteString(barStr + "  " + m.styles.Info.Render(bytesStr) + "  " + m.styles.Muted.Render(pctStr) + "\n")

	// Line 3: model name
	sb.WriteString(m.styles.Muted.Render("  model: "+m.modelName) + "\n")

	return sb.String()
}

// RunPullWithProgress executes onboarding.PullModel and displays a live bubbletea
// progress UI (spinner + progress bar + bytes counter + phase label) until the
// download completes, fails, or the context is cancelled.
//
// Returns the wrapped error from PullModel (success → nil). The progress UI is
// torn down before returning so subsequent huh forms render cleanly.
//
// This function replaces the silent buffered+drain pattern used by Phase 2A's
// runStep2Model/pullModelWithSpinner.
//
// @MX:ANCHOR: [AUTO] Single entry point for model pull UI — called from runStep2Model.
// @MX:REASON: Owns the channel lifecycle; callers must not construct ProgressUpdate channels directly.
func RunPullWithProgress(ctx context.Context, modelName string) error {
	// Create a child context so Ctrl+C from the TUI can cancel the pull.
	pullCtx, cancel := context.WithCancel(ctx)

	// Buffered channels to receive progress updates and the final result.
	updateCh := make(chan onboarding.ProgressUpdate, 32)
	doneCh := make(chan error, 1)

	// Launch pull goroutine.
	//
	// @MX:WARN: [AUTO] Goroutine runs PullModel concurrently with the bubbletea event loop.
	// @MX:REASON: PullModel is synchronous; the bubbletea loop must own the main goroutine.
	go func() {
		err := pullModelFunc(pullCtx, modelName, updateCh)
		close(updateCh) // signal waitForUpdate to stop
		doneCh <- err   // consumed by waitForDone cmd inside the event loop
	}()

	// Initialize spinner.
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = NewMINKStyles().Accent

	// Initialize progress bar with MINK accent gradient.
	bar := progress.New(
		progress.WithGradient(colorPrimary, colorAccent),
		progress.WithWidth(40),
	)

	m := pullProgressModel{
		spinner:   s,
		bar:       bar,
		styles:    NewMINKStyles(),
		modelName: modelName,
		cancelFn:  cancel,
		updateCh:  updateCh,
		doneCh:    doneCh,
	}

	prog := tea.NewProgram(m)
	finalModel, runErr := runProgramFunc(prog)
	cancel() // cancel pull goroutine context when TUI exits

	// Always drain doneCh to ensure the pull goroutine finishes before we return.
	// This prevents goroutine leaks and data races in tests (goroutine must not
	// read pullModelFunc after t.Cleanup restores it).
	pullErr := <-doneCh

	if runErr != nil {
		return fmt.Errorf("progress UI error: %w", runErr)
	}

	// Prefer the error stored in the final model (set by pullDoneMsg handler).
	if fm, ok := finalModel.(pullProgressModel); ok && fm.err != nil {
		return fmt.Errorf("model download failed: %w", fm.err)
	}

	// Fall back to the error received directly from doneCh.
	if pullErr != nil {
		return fmt.Errorf("model download failed: %w", pullErr)
	}

	return nil
}

// formatBytes formats a byte count as a human-readable string.
// Uses binary units: KB = 1024, MB = 1024^2, GB = 1024^3, TB = 1024^4.
func formatBytes(n int64) string {
	const (
		kb = int64(1024)
		mb = kb * 1024
		gb = mb * 1024
		tb = gb * 1024
	)
	switch {
	case n >= tb:
		return fmt.Sprintf("%.1f TB", float64(n)/float64(tb))
	case n >= gb:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(gb))
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// shortLayer extracts the first 7 hex characters after a "sha256:" prefix.
// If no prefix is found, the first 7 characters of the string are returned.
// Returns "" for empty input.
func shortLayer(digest string) string {
	const prefix = "sha256:"
	s := strings.TrimPrefix(digest, prefix)
	if s == "" {
		return ""
	}
	if len(s) <= 7 {
		return s
	}
	return s[:7]
}
