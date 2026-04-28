// Package builtin provides the built-in slash commands for AI.GOOSE.
// SPEC: SPEC-GOOSE-COMMAND-001
package builtin

import (
	"context"
	"testing"

	"github.com/modu-ai/goose/internal/command"
	"github.com/modu-ai/goose/internal/command/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlanCommand_Name verifies Name() returns "plan".
func TestPlanCommand_Name(t *testing.T) {
	cmd := &planCommand{}
	assert.Equal(t, "plan", cmd.Name())
}

// TestPlanCommand_Metadata verifies metadata fields.
func TestPlanCommand_Metadata(t *testing.T) {
	cmd := &planCommand{}
	md := cmd.Metadata()

	assert.False(t, md.Mutates, "Mutates must be false to prevent deadlock in plan mode")
	assert.Equal(t, "[on|off|toggle|status]", md.ArgumentHint)
	assert.Equal(t, command.SourceBuiltin, md.Source)
	assert.NotEmpty(t, md.Description, "Description must be non-empty")
}

// TestPlanCommand_Status verifies /plan and /plan status return current state.
func TestPlanCommand_Status(t *testing.T) {
	tests := []struct {
		name         string
		planModeActive bool
		args         []string
		wantText     string
	}{
		{
			name:         "no args when inactive",
			planModeActive: false,
			args:         nil,
			wantText:     "plan mode: off",
		},
		{
			name:         "no args when active",
			planModeActive: true,
			args:         nil,
			wantText:     "plan mode: on",
		},
		{
			name:         "status subcommand when inactive",
			planModeActive: false,
			args:         []string{"status"},
			wantText:     "plan mode: off",
		},
		{
			name:         "status subcommand when active",
			planModeActive: true,
			args:         []string{"status"},
			wantText:     "plan mode: on",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := adapter.New(adapter.Options{})
			a.SetPlanMode(tt.planModeActive)
			ctx := context.WithValue(context.Background(), command.SctxContextKey(), a)

			result, err := (&planCommand{}).Execute(ctx, command.Args{Positional: tt.args})
			require.NoError(t, err)
			assert.Equal(t, command.ResultLocalReply, result.Kind)
			assert.Equal(t, tt.wantText, result.Text)
			// Verify state unchanged
			assert.Equal(t, tt.planModeActive, a.PlanModeActive())
		})
	}
}

// TestPlanCommand_On verifies /plan on activates plan mode.
func TestPlanCommand_On(t *testing.T) {
	tests := []struct {
		name         string
		initialActive bool
		wantText     string
		wantFinal    bool
	}{
		{
			name:         "activate when inactive",
			initialActive: false,
			wantText:     "plan mode: on",
			wantFinal:    true,
		},
		{
			name:         "already active",
			initialActive: true,
			wantText:     "plan mode: on (already active)",
			wantFinal:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := adapter.New(adapter.Options{})
			a.SetPlanMode(tt.initialActive)
			ctx := context.WithValue(context.Background(), command.SctxContextKey(), a)

			result, err := (&planCommand{}).Execute(ctx, command.Args{Positional: []string{"on"}})
			require.NoError(t, err)
			assert.Equal(t, command.ResultLocalReply, result.Kind)
			assert.Equal(t, tt.wantText, result.Text)
			assert.Equal(t, tt.wantFinal, a.PlanModeActive())
		})
	}
}

// TestPlanCommand_On_CaseInsensitive verifies case-insensitive matching.
func TestPlanCommand_On_CaseInsensitive(t *testing.T) {
	a := adapter.New(adapter.Options{})
	ctx := context.WithValue(context.Background(), command.SctxContextKey(), a)

	cases := []string{"ON", "On", "oN"}
	for _, arg := range cases {
		t.Run(arg, func(t *testing.T) {
			result, err := (&planCommand{}).Execute(ctx, command.Args{Positional: []string{arg}})
			require.NoError(t, err)
			assert.Equal(t, command.ResultLocalReply, result.Kind)
			assert.Contains(t, result.Text, "plan mode: on")
			assert.True(t, a.PlanModeActive())
			// Reset for next test
			a.SetPlanMode(false)
		})
	}
}

// TestPlanCommand_Off verifies /plan off deactivates plan mode.
func TestPlanCommand_Off(t *testing.T) {
	tests := []struct {
		name         string
		initialActive bool
		wantText     string
		wantFinal    bool
	}{
		{
			name:         "deactivate when active",
			initialActive: true,
			wantText:     "plan mode: off",
			wantFinal:    false,
		},
		{
			name:         "already inactive",
			initialActive: false,
			wantText:     "plan mode: off (already inactive)",
			wantFinal:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := adapter.New(adapter.Options{})
			a.SetPlanMode(tt.initialActive)
			ctx := context.WithValue(context.Background(), command.SctxContextKey(), a)

			result, err := (&planCommand{}).Execute(ctx, command.Args{Positional: []string{"off"}})
			require.NoError(t, err)
			assert.Equal(t, command.ResultLocalReply, result.Kind)
			assert.Equal(t, tt.wantText, result.Text)
			assert.Equal(t, tt.wantFinal, a.PlanModeActive())
		})
	}
}

// TestPlanCommand_Off_CaseInsensitive verifies case-insensitive matching.
func TestPlanCommand_Off_CaseInsensitive(t *testing.T) {
	a := adapter.New(adapter.Options{})
	a.SetPlanMode(true)
	ctx := context.WithValue(context.Background(), command.SctxContextKey(), a)

	cases := []string{"OFF", "Off", "oFf"}
	for _, arg := range cases {
		t.Run(arg, func(t *testing.T) {
			result, err := (&planCommand{}).Execute(ctx, command.Args{Positional: []string{arg}})
			require.NoError(t, err)
			assert.Equal(t, command.ResultLocalReply, result.Kind)
			assert.Contains(t, result.Text, "plan mode: off")
			assert.False(t, a.PlanModeActive())
			// Reset for next test
			a.SetPlanMode(true)
		})
	}
}

// TestPlanCommand_Toggle verifies toggle inverts current state.
func TestPlanCommand_Toggle(t *testing.T) {
	tests := []struct {
		name         string
		initialActive bool
		wantText     string
		wantFinal    bool
	}{
		{
			name:         "toggle off to on",
			initialActive: false,
			wantText:     "plan mode: on",
			wantFinal:    true,
		},
		{
			name:         "toggle on to off",
			initialActive: true,
			wantText:     "plan mode: off",
			wantFinal:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := adapter.New(adapter.Options{})
			a.SetPlanMode(tt.initialActive)
			ctx := context.WithValue(context.Background(), command.SctxContextKey(), a)

			result, err := (&planCommand{}).Execute(ctx, command.Args{Positional: []string{"toggle"}})
			require.NoError(t, err)
			assert.Equal(t, command.ResultLocalReply, result.Kind)
			assert.Equal(t, tt.wantText, result.Text)
			assert.Equal(t, tt.wantFinal, a.PlanModeActive())
		})
	}
}

// TestPlanCommand_InvalidArgs verifies usage text for invalid arguments.
func TestPlanCommand_InvalidArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"unknown subcommand", []string{"foo"}},
		{"unknown word", []string{"unknown"}},
		{"too many args on", []string{"on", "extra"}},
		{"too many args toggle", []string{"toggle", "now"}},
		{"three args", []string{"on", "extra", "more"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := adapter.New(adapter.Options{})
			ctx := context.WithValue(context.Background(), command.SctxContextKey(), a)

			result, err := (&planCommand{}).Execute(ctx, command.Args{Positional: tt.args})
			require.NoError(t, err)
			assert.Equal(t, command.ResultLocalReply, result.Kind)
			assert.Contains(t, result.Text, "usage:")
			// Verify plan mode unchanged
			assert.False(t, a.PlanModeActive())
		})
	}
}

// TestPlanCommand_NilSctx verifies graceful handling when sctx is nil.
func TestPlanCommand_NilSctx(t *testing.T) {
	ctx := context.Background() // No sctx injected

	result, err := (&planCommand{}).Execute(ctx, command.Args{Positional: []string{"on"}})
	require.NoError(t, err)
	assert.Equal(t, command.ResultLocalReply, result.Kind)
	assert.Equal(t, "plan: context unavailable", result.Text)
}

// mockSctxNoSetter implements SlashCommandContext but NOT PlanModeSetter.
// Used to test graceful degradation when PlanModeSetter is not available.
type mockSctxNoSetter struct{}

func (m *mockSctxNoSetter) OnClear() error { return nil }
func (m *mockSctxNoSetter) OnModelChange(command.ModelInfo) error { return nil }
func (m *mockSctxNoSetter) OnCompactRequest(int) error { return nil }
func (m *mockSctxNoSetter) ResolveModelAlias(string) (*command.ModelInfo, error) {
	return nil, command.ErrUnknownModel
}
func (m *mockSctxNoSetter) SessionSnapshot() command.SessionSnapshot {
	return command.SessionSnapshot{}
}
func (m *mockSctxNoSetter) PlanModeActive() bool { return false }

// TestPlanCommand_NoPlanModeSetter verifies graceful handling when sctx doesn't implement PlanModeSetter.
func TestPlanCommand_NoPlanModeSetter(t *testing.T) {
	m := &mockSctxNoSetter{}
	ctx := context.WithValue(context.Background(), command.SctxContextKey(), command.SlashCommandContext(m))

	result, err := (&planCommand{}).Execute(ctx, command.Args{Positional: []string{"on"}})
	require.NoError(t, err)
	assert.Equal(t, command.ResultLocalReply, result.Kind)
	assert.Contains(t, result.Text, "does not support plan mode toggling")
}

// TestPlanCommand_ExecuteNeverErrors verifies all Execute branches return nil error.
func TestPlanCommand_ExecuteNeverErrors(t *testing.T) {
	tests := []struct {
		name string
		setup func() (context.Context, command.Args)
	}{
		{
			name: "status with valid sctx",
			setup: func() (context.Context, command.Args) {
				a := adapter.New(adapter.Options{})
				ctx := context.WithValue(context.Background(), command.SctxContextKey(), a)
				return ctx, command.Args{Positional: []string{"status"}}
			},
		},
		{
			name: "on with valid sctx",
			setup: func() (context.Context, command.Args) {
				a := adapter.New(adapter.Options{})
				ctx := context.WithValue(context.Background(), command.SctxContextKey(), a)
				return ctx, command.Args{Positional: []string{"on"}}
			},
		},
		{
			name: "off with valid sctx",
			setup: func() (context.Context, command.Args) {
				a := adapter.New(adapter.Options{})
				ctx := context.WithValue(context.Background(), command.SctxContextKey(), a)
				return ctx, command.Args{Positional: []string{"off"}}
			},
		},
		{
			name: "toggle with valid sctx",
			setup: func() (context.Context, command.Args) {
				a := adapter.New(adapter.Options{})
				ctx := context.WithValue(context.Background(), command.SctxContextKey(), a)
				return ctx, command.Args{Positional: []string{"toggle"}}
			},
		},
		{
			name: "invalid args",
			setup: func() (context.Context, command.Args) {
				a := adapter.New(adapter.Options{})
				ctx := context.WithValue(context.Background(), command.SctxContextKey(), a)
				return ctx, command.Args{Positional: []string{"invalid"}}
			},
		},
		{
			name: "nil sctx",
			setup: func() (context.Context, command.Args) {
				return context.Background(), command.Args{Positional: []string{"on"}}
			},
		},
		{
			name: "no PlanModeSetter",
			setup: func() (context.Context, command.Args) {
				m := &mockSctxNoSetter{}
				ctx := context.WithValue(context.Background(), command.SctxContextKey(), command.SlashCommandContext(m))
				return ctx, command.Args{Positional: []string{"on"}}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, args := tt.setup()
			_, err := (&planCommand{}).Execute(ctx, args)
			assert.NoError(t, err, "Execute must never return an error")
		})
	}
}
