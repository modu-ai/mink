// Package builtin provides the built-in slash commands for AI.GOOSE.
// SPEC: SPEC-GOOSE-COMMAND-001
package builtin_test

import (
	"context"
	"strings"
	"testing"

	"github.com/modu-ai/goose/internal/command"
	"github.com/modu-ai/goose/internal/command/builtin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockSctx is a test double for SlashCommandContext.
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

// buildRegistry creates a registry with all builtin commands registered.
func buildRegistry(t *testing.T) *command.Registry {
	t.Helper()
	logger := zap.NewNop()
	reg, err := command.NewRegistry(command.WithLogger(logger))
	require.NoError(t, err)
	builtin.Register(reg)
	return reg
}

// TestBuiltinHelp_ListsAllCommands verifies that /help output mentions all 7 builtin names.
// RED #6 — AC-CMD-001.
func TestBuiltinHelp_ListsAllCommands(t *testing.T) {
	t.Parallel()

	reg := buildRegistry(t)
	sctx := &mockSctx{}

	helpCmd, ok := reg.Resolve("help")
	require.True(t, ok)
	_ = helpCmd

	// Use Dispatcher integration for the full test.
	logger := zap.NewNop()
	d := command.NewDispatcher(reg, command.Config{}, logger)
	out, err := d.ProcessUserInput(context.Background(), "/help", sctx)
	require.NoError(t, err)
	require.Equal(t, command.ProcessLocal, out.Kind)
	require.Greater(t, len(out.Messages), 0)

	// Collect all text from messages.
	var sb strings.Builder
	for _, msg := range out.Messages {
		if payload, ok := msg.Payload.(string); ok {
			sb.WriteString(payload)
		}
	}
	text := sb.String()

	for _, name := range []string{"help", "clear", "exit", "model", "compact", "status", "version"} {
		require.Contains(t, text, name, "expected %q in /help output", name)
	}
}

// TestBuiltinClear_InvokesOnClear verifies /clear calls sctx.OnClear exactly once.
// RED #7 — AC-CMD-006.
func TestBuiltinClear_InvokesOnClear(t *testing.T) {
	t.Parallel()

	reg := buildRegistry(t)
	sctx := &mockSctx{}
	logger := zap.NewNop()
	d := command.NewDispatcher(reg, command.Config{}, logger)

	out, err := d.ProcessUserInput(context.Background(), "/clear", sctx)
	require.NoError(t, err)
	require.Equal(t, command.ProcessLocal, out.Kind)
	require.Equal(t, 1, sctx.clearCount, "OnClear must be called exactly once")

	// Verify result text.
	var found bool
	for _, msg := range out.Messages {
		if payload, ok := msg.Payload.(string); ok &&
			strings.Contains(payload, "conversation cleared") {
			found = true
			break
		}
	}
	require.True(t, found)
}

// TestBuiltinExit_ReturnsExit0 verifies /exit returns ProcessExit with code 0.
// RED #8 — AC-CMD-007.
func TestBuiltinExit_ReturnsExit0(t *testing.T) {
	t.Parallel()

	reg := buildRegistry(t)
	sctx := &mockSctx{}
	logger := zap.NewNop()
	d := command.NewDispatcher(reg, command.Config{}, logger)

	out, err := d.ProcessUserInput(context.Background(), "/exit", sctx)
	require.NoError(t, err)
	require.Equal(t, command.ProcessExit, out.Kind)
	require.Equal(t, 0, out.ExitCode)
}

// TestBuiltinQuit_AliasForExit verifies /quit is an alias for /exit.
func TestBuiltinQuit_AliasForExit(t *testing.T) {
	t.Parallel()

	reg := buildRegistry(t)
	sctx := &mockSctx{}
	logger := zap.NewNop()
	d := command.NewDispatcher(reg, command.Config{}, logger)

	out, err := d.ProcessUserInput(context.Background(), "/quit", sctx)
	require.NoError(t, err)
	require.Equal(t, command.ProcessExit, out.Kind)
}

// TestBuiltinModel_Valid_InvokesOnModelChange verifies that /model with a valid alias
// calls OnModelChange. RED #9 — AC-CMD-008.
func TestBuiltinModel_Valid_InvokesOnModelChange(t *testing.T) {
	t.Parallel()

	reg := buildRegistry(t)
	sctx := &mockSctx{
		resolveAlias: func(alias string) (*command.ModelInfo, error) {
			if alias == "gpt-4o" {
				return &command.ModelInfo{ID: "gpt-4o-2024-08-06"}, nil
			}
			return nil, command.ErrUnknownModel
		},
	}
	logger := zap.NewNop()
	d := command.NewDispatcher(reg, command.Config{}, logger)

	out, err := d.ProcessUserInput(context.Background(), "/model gpt-4o", sctx)
	require.NoError(t, err)
	require.Equal(t, command.ProcessLocal, out.Kind)
	require.Len(t, sctx.modelChangeCalls, 1)
	require.Equal(t, "gpt-4o-2024-08-06", sctx.modelChangeCalls[0].ID)

	var found bool
	for _, msg := range out.Messages {
		if payload, ok := msg.Payload.(string); ok &&
			strings.Contains(payload, "gpt-4o-2024-08-06") {
			found = true
			break
		}
	}
	require.True(t, found)
}

// TestBuiltinModel_Invalid_NoOnModelChange verifies that /model with an unknown alias
// does not call OnModelChange. RED #10 — AC-CMD-009.
func TestBuiltinModel_Invalid_NoOnModelChange(t *testing.T) {
	t.Parallel()

	reg := buildRegistry(t)
	sctx := &mockSctx{} // resolveAlias returns ErrUnknownModel by default
	logger := zap.NewNop()
	d := command.NewDispatcher(reg, command.Config{}, logger)

	out, err := d.ProcessUserInput(context.Background(), "/model xxx", sctx)
	require.NoError(t, err)
	require.Equal(t, command.ProcessLocal, out.Kind)
	require.Empty(t, sctx.modelChangeCalls, "OnModelChange must not be called for invalid alias")

	var found bool
	for _, msg := range out.Messages {
		if payload, ok := msg.Payload.(string); ok &&
			strings.Contains(payload, "unknown model: xxx") {
			found = true
			break
		}
	}
	require.True(t, found)
}

// TestBuiltinStatus_ReturnsSessionInfo verifies /status returns session snapshot data.
func TestBuiltinStatus_ReturnsSessionInfo(t *testing.T) {
	t.Parallel()

	reg := buildRegistry(t)
	sctx := &mockSctx{
		snapshot: command.SessionSnapshot{TurnCount: 3, Model: "claude-3", CWD: "/work"},
	}
	logger := zap.NewNop()
	d := command.NewDispatcher(reg, command.Config{}, logger)

	out, err := d.ProcessUserInput(context.Background(), "/status", sctx)
	require.NoError(t, err)
	require.Equal(t, command.ProcessLocal, out.Kind)

	var sb strings.Builder
	for _, msg := range out.Messages {
		if payload, ok := msg.Payload.(string); ok {
			sb.WriteString(payload)
		}
	}
	text := sb.String()
	require.Contains(t, text, "claude-3")
	require.Contains(t, text, "/work")
}

// TestBuiltinVersion_ReturnsVersion verifies /version returns version text.
func TestBuiltinVersion_ReturnsVersion(t *testing.T) {
	t.Parallel()

	reg := buildRegistry(t)
	sctx := &mockSctx{}
	logger := zap.NewNop()
	d := command.NewDispatcher(reg, command.Config{}, logger)

	out, err := d.ProcessUserInput(context.Background(), "/version", sctx)
	require.NoError(t, err)
	require.Equal(t, command.ProcessLocal, out.Kind)

	var found bool
	for _, msg := range out.Messages {
		if payload, ok := msg.Payload.(string); ok && strings.Contains(payload, "goose") {
			found = true
			break
		}
	}
	require.True(t, found)
}

// TestBuiltinCompact_InvokesOnCompactRequest verifies /compact calls OnCompactRequest.
func TestBuiltinCompact_InvokesOnCompactRequest(t *testing.T) {
	t.Parallel()

	reg := buildRegistry(t)
	sctx := &mockSctx{}
	logger := zap.NewNop()
	d := command.NewDispatcher(reg, command.Config{}, logger)

	out, err := d.ProcessUserInput(context.Background(), "/compact", sctx)
	require.NoError(t, err)
	require.Equal(t, command.ProcessLocal, out.Kind)
	require.Len(t, sctx.compactCalls, 1)
}

// TestBuiltinCompact_WithTargetTokens verifies /compact 1024 calls OnCompactRequest
// with target=1024 and returns a LocalReply mentioning the target. Task A-2.
func TestBuiltinCompact_WithTargetTokens(t *testing.T) {
	t.Parallel()

	reg := buildRegistry(t)
	sctx := &mockSctx{}
	logger := zap.NewNop()
	d := command.NewDispatcher(reg, command.Config{}, logger)

	out, err := d.ProcessUserInput(context.Background(), "/compact 1024", sctx)
	require.NoError(t, err)
	require.Equal(t, command.ProcessLocal, out.Kind)

	// OnCompactRequest must be called with target=1024.
	require.Len(t, sctx.compactCalls, 1)
	require.Equal(t, 1024, sctx.compactCalls[0], "OnCompactRequest must receive target=1024")

	// The reply text must mention "compaction requested" and "1024".
	var sb strings.Builder
	for _, msg := range out.Messages {
		if payload, ok := msg.Payload.(string); ok {
			sb.WriteString(payload)
		}
	}
	text := sb.String()
	require.Contains(t, text, "compaction requested")
	require.Contains(t, text, "1024")
}

// TestBuiltinCompact_InvalidIntegerArg verifies /compact abc (non-integer) gracefully
// falls back to target=0 and still calls OnCompactRequest. Task A-2.
func TestBuiltinCompact_InvalidIntegerArg(t *testing.T) {
	t.Parallel()

	reg := buildRegistry(t)
	sctx := &mockSctx{}
	logger := zap.NewNop()
	d := command.NewDispatcher(reg, command.Config{}, logger)

	// "abc" cannot be parsed as an integer; Execute should fall back to target=0.
	out, err := d.ProcessUserInput(context.Background(), "/compact abc", sctx)
	require.NoError(t, err)
	require.Equal(t, command.ProcessLocal, out.Kind)

	// OnCompactRequest must still be called — with target=0 (the graceful fallback).
	require.Len(t, sctx.compactCalls, 1)
	require.Equal(t, 0, sctx.compactCalls[0], "invalid integer arg must fall back to target=0")

	// Reply text must still contain "compaction requested".
	var sb strings.Builder
	for _, msg := range out.Messages {
		if payload, ok := msg.Payload.(string); ok {
			sb.WriteString(payload)
		}
	}
	require.Contains(t, sb.String(), "compaction requested")
}
