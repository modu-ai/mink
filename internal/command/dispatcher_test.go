// Package command implements the slash command system for AI.GOOSE.
// SPEC: SPEC-GOOSE-COMMAND-001
package command_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/command"
	"github.com/modu-ai/mink/internal/command/custom"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockSctx is a hand-rolled mock of SlashCommandContext.
type mockSctx struct {
	clearCount       int
	modelChangeCalls []command.ModelInfo
	compactCalls     []int
	resolveAlias     func(alias string) (*command.ModelInfo, error)
	planMode         bool
	snapshot         command.SessionSnapshot
}

func (m *mockSctx) OnClear() error {
	m.clearCount++
	return nil
}

func (m *mockSctx) OnModelChange(info command.ModelInfo) error {
	m.modelChangeCalls = append(m.modelChangeCalls, info)
	return nil
}

func (m *mockSctx) OnCompactRequest(target int) error {
	m.compactCalls = append(m.compactCalls, target)
	return nil
}

func (m *mockSctx) ResolveModelAlias(alias string) (*command.ModelInfo, error) {
	if m.resolveAlias != nil {
		return m.resolveAlias(alias)
	}
	return nil, command.ErrUnknownModel
}

func (m *mockSctx) SessionSnapshot() command.SessionSnapshot { return m.snapshot }
func (m *mockSctx) PlanModeActive() bool                     { return m.planMode }

// buildDispatcher creates a Dispatcher wired with builtin commands for integration tests.
func buildDispatcher(t *testing.T, cfg command.Config) *command.Dispatcher {
	t.Helper()
	logger := zap.NewNop()
	reg, err := command.NewRegistry(command.WithLogger(logger))
	require.NoError(t, err)
	return command.NewDispatcher(reg, cfg, logger)
}

// TestDispatcher_Unknown_ReturnsLocalReply verifies that an unregistered command
// produces a LocalMessage with an informative text. RED #4 — AC-CMD-003.
func TestDispatcher_Unknown_ReturnsLocalReply(t *testing.T) {
	t.Parallel()

	d := buildDispatcher(t, command.Config{})
	sctx := &mockSctx{}
	out, err := d.ProcessUserInput(context.Background(), "/nonexistent", sctx)
	require.NoError(t, err)
	require.Equal(t, command.ProcessLocal, out.Kind)
	require.Greater(t, len(out.Messages), 0)

	// Verify the error text mentions the command name.
	var found bool
	for _, msg := range out.Messages {
		if strings.Contains(msg.Payload.(string), "unknown command") &&
			strings.Contains(msg.Payload.(string), "nonexistent") {
			found = true
			break
		}
	}
	require.True(t, found, "expected 'unknown command: /nonexistent' in messages")
}

// TestDispatcher_PlainPrompt_Proceeds verifies that non-slash input is returned unchanged.
// RED #5 — AC-CMD-002.
func TestDispatcher_PlainPrompt_Proceeds(t *testing.T) {
	t.Parallel()

	d := buildDispatcher(t, command.Config{})
	sctx := &mockSctx{}
	out, err := d.ProcessUserInput(context.Background(), "hello world", sctx)
	require.NoError(t, err)
	require.Equal(t, command.ProcessProceed, out.Kind)
	require.Equal(t, "hello world", out.Prompt)
}

// TestDispatcher_MaxSizeExceeded verifies that an expanded prompt exceeding the
// configured limit is rejected. RED #15 — AC-CMD-013, REQ-CMD-014.
func TestDispatcher_MaxSizeExceeded(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	reg, err := command.NewRegistry(command.WithLogger(logger))
	require.NoError(t, err)

	// Register a command that always returns a large PromptExpansion.
	largeCmd := &bigPromptCommand{size: 200}
	require.NoError(t, reg.Register(largeCmd, command.SourceBuiltin))

	cfg := command.Config{MaxExpandedPromptBytes: 100}
	d := command.NewDispatcher(reg, cfg, logger)

	sctx := &mockSctx{}
	out, err := d.ProcessUserInput(context.Background(), "/big", sctx)
	require.NoError(t, err)
	require.Equal(t, command.ProcessLocal, out.Kind)

	var found bool
	for _, msg := range out.Messages {
		if payload, ok := msg.Payload.(string); ok &&
			strings.Contains(payload, "expanded prompt exceeds size limit") {
			found = true
			break
		}
	}
	require.True(t, found, "expected size limit message")
}

// bigPromptCommand produces a ResultPromptExpansion with a prompt of the given byte size.
type bigPromptCommand struct {
	size int
}

func (b *bigPromptCommand) Name() string { return "big" }
func (b *bigPromptCommand) Metadata() command.Metadata {
	return command.Metadata{Source: command.SourceBuiltin}
}
func (b *bigPromptCommand) Execute(_ context.Context, _ command.Args) (command.Result, error) {
	return command.Result{
		Kind:   command.ResultPromptExpansion,
		Prompt: strings.Repeat("x", b.size),
	}, nil
}

// TestDispatcher_PlanModeBlock verifies that mutating commands are blocked in plan mode.
// REQ-CMD-011.
func TestDispatcher_PlanModeBlock(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	reg, err := command.NewRegistry(command.WithLogger(logger))
	require.NoError(t, err)

	mutCmd := &mutatingCommand{}
	require.NoError(t, reg.Register(mutCmd, command.SourceBuiltin))

	d := command.NewDispatcher(reg, command.Config{}, logger)
	sctx := &mockSctx{planMode: true}

	out, err := d.ProcessUserInput(context.Background(), "/mutate", sctx)
	require.NoError(t, err)
	require.Equal(t, command.ProcessLocal, out.Kind)

	var blocked bool
	for _, msg := range out.Messages {
		if payload, ok := msg.Payload.(string); ok &&
			strings.Contains(payload, "disabled in plan mode") {
			blocked = true
			break
		}
	}
	require.True(t, blocked)
}

// TestDispatcher_PromptExpansion_Proceeds verifies ResultPromptExpansion within limit proceeds.
func TestDispatcher_PromptExpansion_Proceeds(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	reg, err := command.NewRegistry(command.WithLogger(logger))
	require.NoError(t, err)

	expandCmd := &expandingCommand{prompt: "expanded text"}
	require.NoError(t, reg.Register(expandCmd, command.SourceBuiltin))

	d := command.NewDispatcher(reg, command.Config{}, logger)
	sctx := &mockSctx{}

	out, err := d.ProcessUserInput(context.Background(), "/expand", sctx)
	require.NoError(t, err)
	require.Equal(t, command.ProcessProceed, out.Kind)
	require.Equal(t, "expanded text", out.Prompt)
}

// TestDispatcher_ExitResult propagates ResultExit.
func TestDispatcher_ExitResult(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	reg, err := command.NewRegistry(command.WithLogger(logger))
	require.NoError(t, err)

	exitCmd := &exitingCommand{code: 42}
	require.NoError(t, reg.Register(exitCmd, command.SourceBuiltin))

	d := command.NewDispatcher(reg, command.Config{}, logger)
	out, err := d.ProcessUserInput(context.Background(), "/myexit", &mockSctx{})
	require.NoError(t, err)
	require.Equal(t, command.ProcessExit, out.Kind)
	require.Equal(t, 42, out.ExitCode)
}

// TestDispatcher_AbortResult propagates ResultAbort.
func TestDispatcher_AbortResult(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	reg, err := command.NewRegistry(command.WithLogger(logger))
	require.NoError(t, err)

	abortCmd := &abortingCommand{}
	require.NoError(t, reg.Register(abortCmd, command.SourceBuiltin))

	d := command.NewDispatcher(reg, command.Config{}, logger)
	out, err := d.ProcessUserInput(context.Background(), "/abort", &mockSctx{})
	require.NoError(t, err)
	require.Equal(t, command.ProcessAbort, out.Kind)
}

// expandingCommand returns ResultPromptExpansion.
type expandingCommand struct {
	prompt string
}

func (e *expandingCommand) Name() string { return "expand" }
func (e *expandingCommand) Metadata() command.Metadata {
	return command.Metadata{Source: command.SourceBuiltin}
}
func (e *expandingCommand) Execute(_ context.Context, _ command.Args) (command.Result, error) {
	return command.Result{Kind: command.ResultPromptExpansion, Prompt: e.prompt}, nil
}

// exitingCommand returns ResultExit with a specific code.
type exitingCommand struct{ code int }

func (e *exitingCommand) Name() string { return "myexit" }
func (e *exitingCommand) Metadata() command.Metadata {
	return command.Metadata{Source: command.SourceBuiltin}
}
func (e *exitingCommand) Execute(_ context.Context, _ command.Args) (command.Result, error) {
	return command.Result{Kind: command.ResultExit, Exit: e.code}, nil
}

// abortingCommand returns ResultAbort.
type abortingCommand struct{}

func (a *abortingCommand) Name() string { return "abort" }
func (a *abortingCommand) Metadata() command.Metadata {
	return command.Metadata{Source: command.SourceBuiltin}
}
func (a *abortingCommand) Execute(_ context.Context, _ command.Args) (command.Result, error) {
	return command.Result{Kind: command.ResultAbort}, nil
}

// TestDispatcher_CustomCommand_EndToEnd_Greet verifies that a custom Markdown command
// loaded from a temp directory is resolved and executed end-to-end via the Dispatcher.
// AC-CMD-004, REQ-CMD-005.
func TestDispatcher_CustomCommand_EndToEnd_Greet(t *testing.T) {
	t.Parallel()

	// Step 1: Create a temp dir with a greet.md command file.
	dir := t.TempDir()
	content := "---\nname: greet\ndescription: greet user\nargument-hint: \"<name>\"\n---\nHello $ARGUMENTS, welcome to GOOSE."
	require.NoError(t, os.WriteFile(filepath.Join(dir, "greet.md"), []byte(content), 0o600))

	logger := zap.NewNop()

	// Step 2: Build Registry, load the custom command dir, register commands.
	reg, err := command.NewRegistry(command.WithLogger(logger))
	require.NoError(t, err)

	cmds, err := custom.LoadDir(dir, command.SourceCustomProject, logger)
	require.NoError(t, err)
	require.NotEmpty(t, cmds, "greet.md must produce at least one command")

	for _, cmd := range cmds {
		require.NoError(t, reg.Register(cmd, command.SourceCustomProject))
	}

	// Step 3: Build Dispatcher.
	d := command.NewDispatcher(reg, command.Config{}, logger)

	// Step 4: Call ProcessUserInput with /greet Alice.
	sctx := &mockSctx{}
	out, err := d.ProcessUserInput(context.Background(), "/greet Alice", sctx)

	// Step 5: Assert result.
	require.NoError(t, err)
	require.Equal(t, command.ProcessProceed, out.Kind)
	require.Equal(t, "Hello Alice, welcome to GOOSE.", out.Prompt)
}

// mutatingCommand is a command with Mutates=true.
type mutatingCommand struct{}

func (m *mutatingCommand) Name() string { return "mutate" }
func (m *mutatingCommand) Metadata() command.Metadata {
	return command.Metadata{Source: command.SourceBuiltin, Mutates: true}
}
func (m *mutatingCommand) Execute(_ context.Context, _ command.Args) (command.Result, error) {
	return command.Result{Kind: command.ResultLocalReply, Text: "executed"}, nil
}
