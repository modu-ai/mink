package adapter

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/modu-ai/goose/internal/command"
	"github.com/modu-ai/goose/internal/llm/router"
	"github.com/modu-ai/goose/internal/subagent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AC-CMDCTX-001: compile-time assertion that ContextAdapter implements SlashCommandContext.
// This is verified by the var _ ... = (*ContextAdapter)(nil) line in adapter.go.
// The test below confirms it via type assertion.
func TestContextAdapter_ImplementsSlashCommandContext(t *testing.T) {
	t.Parallel()
	a := New(Options{})
	var iface command.SlashCommandContext = a
	assert.NotNil(t, iface)
}

// ---------------------------------------------------------------------------
// T-002: ResolveModelAlias
// AC-CMDCTX-002, -003, -011, -017
// ---------------------------------------------------------------------------

func TestContextAdapter_ResolveModelAlias(t *testing.T) {
	t.Parallel()

	defaultReg := router.DefaultRegistry()

	cases := []struct {
		name     string
		registry *router.ProviderRegistry
		aliasMap map[string]string
		input    string
		wantID   string
		wantErr  error
	}{
		{
			// AC-CMDCTX-002: happy path — provider/model form
			name:     "happy_provider_slash_model",
			registry: defaultReg,
			aliasMap: nil,
			input:    "anthropic/claude-opus-4-7",
			wantID:   "anthropic/claude-opus-4-7",
			wantErr:  nil,
		},
		{
			// AC-CMDCTX-017: alias map hit
			name:     "alias_map_hit",
			registry: defaultReg,
			aliasMap: map[string]string{"opus": "anthropic/claude-opus-4-7"},
			input:    "opus",
			wantID:   "anthropic/claude-opus-4-7",
			wantErr:  nil,
		},
		{
			// AC-CMDCTX-003: unknown provider
			name:     "unknown_provider",
			registry: defaultReg,
			aliasMap: nil,
			input:    "nonexistent/foo",
			wantID:   "",
			wantErr:  command.ErrUnknownModel,
		},
		{
			// AC-CMDCTX-003: unknown model (provider exists but model not in SuggestedModels)
			name:     "unknown_model",
			registry: defaultReg,
			aliasMap: nil,
			input:    "anthropic/not-a-model",
			wantID:   "",
			wantErr:  command.ErrUnknownModel,
		},
		{
			// AC-CMDCTX-011: nil registry returns ErrUnknownModel, no panic
			name:     "nil_registry",
			registry: nil,
			aliasMap: nil,
			input:    "anthropic/claude-opus-4-7",
			wantID:   "",
			wantErr:  command.ErrUnknownModel,
		},
		{
			// malformed alias: no slash
			name:     "malformed_alias_no_slash",
			registry: defaultReg,
			aliasMap: nil,
			input:    "no-slash",
			wantID:   "",
			wantErr:  command.ErrUnknownModel,
		},
		{
			// empty alias
			name:     "empty_alias",
			registry: defaultReg,
			aliasMap: nil,
			input:    "",
			wantID:   "",
			wantErr:  command.ErrUnknownModel,
		},
		{
			// alias map resolves to another provider model (openai)
			name:     "alias_map_openai",
			registry: defaultReg,
			aliasMap: map[string]string{"gpt4o": "openai/gpt-4o"},
			input:    "gpt4o",
			wantID:   "openai/gpt-4o",
			wantErr:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := New(Options{
				Registry: tc.registry,
				AliasMap: tc.aliasMap,
			})
			got, err := a.ResolveModelAlias(tc.input)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, tc.wantID, got.ID)
				assert.NotEmpty(t, got.DisplayName)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// T-003: SessionSnapshot
// AC-CMDCTX-004, -013, -016
// ---------------------------------------------------------------------------

func TestContextAdapter_SessionSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("with_loop_controller", func(t *testing.T) {
		// AC-CMDCTX-004: Snapshot returns TurnCount from loop controller.
		t.Parallel()
		fake := &fakeLoopController{
			snapshotVal: LoopSnapshot{TurnCount: 7, Model: "anthropic/claude-opus-4-7"},
		}
		a := New(Options{
			Registry:       router.DefaultRegistry(),
			LoopController: fake,
		})
		snap := a.SessionSnapshot()
		assert.Equal(t, 7, snap.TurnCount)
		assert.NotEmpty(t, snap.CWD) // either real cwd or "<unknown>"
	})

	t.Run("nil_loop_controller", func(t *testing.T) {
		// AC-CMDCTX-013: nil loopCtrl → TurnCount=0, Model="<unknown>".
		t.Parallel()
		a := New(Options{Registry: router.DefaultRegistry()})
		snap := a.SessionSnapshot()
		assert.Equal(t, 0, snap.TurnCount)
		assert.Equal(t, "<unknown>", snap.Model)
	})

	t.Run("getwd_error_returns_unknown_and_warns", func(t *testing.T) {
		// AC-CMDCTX-016: getwd error → CWD="<unknown>", WarnCount>=1, error in log args.
		t.Parallel()
		getwdErr := errors.New("getwd: permission denied")
		warnLog := &fakeWarnLogger{}
		fake := &fakeLoopController{
			snapshotVal: LoopSnapshot{TurnCount: 3, Model: "openai/gpt-4o"},
		}
		a := New(Options{
			Registry:       router.DefaultRegistry(),
			LoopController: fake,
			GetwdFn:        func() (string, error) { return "", getwdErr },
			Logger:         warnLog,
		})
		snap := a.SessionSnapshot()
		assert.Equal(t, "<unknown>", snap.CWD)
		assert.Equal(t, 3, snap.TurnCount)
		assert.GreaterOrEqual(t, warnLog.getWarnCount(), 1)
		// Verify error is present in logged fields.
		found := false
		for _, arg := range warnLog.getLastArgs() {
			if arg == getwdErr {
				found = true
				break
			}
		}
		assert.True(t, found, "expected getwd error to appear in Warn fields")
	})

	t.Run("getwd_error_nil_logger_no_panic", func(t *testing.T) {
		// REQ-CMDCTX-018: nil logger with getwd error must not panic.
		t.Parallel()
		a := New(Options{
			GetwdFn: func() (string, error) { return "", errors.New("no cwd") },
			Logger:  nil,
		})
		snap := a.SessionSnapshot()
		assert.Equal(t, "<unknown>", snap.CWD)
	})
}

// ---------------------------------------------------------------------------
// T-004: PlanModeActive
// AC-CMDCTX-009, -010
// ---------------------------------------------------------------------------

func TestContextAdapter_PlanModeActive(t *testing.T) {
	t.Parallel()

	t.Run("default_false", func(t *testing.T) {
		// AC-CMDCTX-010: no flag, no ctx TeammateIdentity → false.
		t.Parallel()
		a := New(Options{})
		assert.False(t, a.PlanModeActive())
	})

	t.Run("local_flag_true", func(t *testing.T) {
		t.Parallel()
		a := New(Options{})
		a.SetPlanMode(true)
		assert.True(t, a.PlanModeActive())
	})

	t.Run("ctx_teammate_identity_plan_required", func(t *testing.T) {
		// AC-CMDCTX-009: ctx carries TeammateIdentity{PlanModeRequired: true} → true.
		t.Parallel()
		a := New(Options{})
		ctx := subagent.WithTeammateIdentity(context.Background(), subagent.TeammateIdentity{
			PlanModeRequired: true,
		})
		child := a.WithContext(ctx)
		assert.True(t, child.PlanModeActive())
	})

	t.Run("ctx_teammate_identity_plan_not_required", func(t *testing.T) {
		// TeammateIdentity present but PlanModeRequired=false and local flag unset → false.
		t.Parallel()
		a := New(Options{})
		ctx := subagent.WithTeammateIdentity(context.Background(), subagent.TeammateIdentity{
			PlanModeRequired: false,
		})
		child := a.WithContext(ctx)
		assert.False(t, child.PlanModeActive())
	})

	t.Run("shared_plan_mode_pointer", func(t *testing.T) {
		// SetPlanMode on parent is observed by child (pointer sharing invariant).
		t.Parallel()
		parent := New(Options{})
		child := parent.WithContext(context.Background())
		assert.False(t, child.PlanModeActive())
		parent.SetPlanMode(true)
		assert.True(t, child.PlanModeActive())
	})
}

// ---------------------------------------------------------------------------
// T-005: OnClear / OnCompactRequest
// AC-CMDCTX-005, -006, -007, -012, -015, -018
// ---------------------------------------------------------------------------

func TestContextAdapter_OnClear(t *testing.T) {
	t.Parallel()

	t.Run("delegates_once_and_returns_nil", func(t *testing.T) {
		// AC-CMDCTX-005: clearCount increments by 1, error is nil.
		t.Parallel()
		fake := &fakeLoopController{}
		a := New(Options{LoopController: fake})
		err := a.OnClear()
		require.NoError(t, err)
		assert.Equal(t, 1, fake.getClearCount())
	})

	t.Run("nil_loop_controller_returns_sentinel", func(t *testing.T) {
		// AC-CMDCTX-012: nil loopCtrl → ErrLoopControllerUnavailable.
		t.Parallel()
		a := New(Options{})
		err := a.OnClear()
		assert.ErrorIs(t, err, ErrLoopControllerUnavailable)
	})

	t.Run("propagates_controller_error", func(t *testing.T) {
		// AC-CMDCTX-015: controller returns error → adapter propagates it.
		t.Parallel()
		sentinel := errors.New("loop busy")
		fake := &fakeLoopController{nextErr: sentinel}
		a := New(Options{LoopController: fake})
		err := a.OnClear()
		assert.ErrorIs(t, err, sentinel)
	})

	t.Run("plan_mode_flag_does_not_block_onclear", func(t *testing.T) {
		// AC-CMDCTX-018: plan mode flag true → OnClear still delegates (adapter doesn't block).
		t.Parallel()
		fake := &fakeLoopController{}
		a := New(Options{LoopController: fake})
		a.SetPlanMode(true)
		err := a.OnClear()
		require.NoError(t, err)
		assert.Equal(t, 1, fake.getClearCount())
	})
}

func TestContextAdapter_OnCompactRequest(t *testing.T) {
	t.Parallel()

	t.Run("delegates_with_target_50000", func(t *testing.T) {
		// AC-CMDCTX-006: captured target == 50000.
		t.Parallel()
		fake := &fakeLoopController{}
		a := New(Options{LoopController: fake})
		err := a.OnCompactRequest(50000)
		require.NoError(t, err)
		reqs := fake.getCompactRequests()
		require.Len(t, reqs, 1)
		assert.Equal(t, 50000, reqs[0])
	})

	t.Run("delegates_with_target_zero", func(t *testing.T) {
		// AC-CMDCTX-007: target == 0 (compactor default signal).
		t.Parallel()
		fake := &fakeLoopController{}
		a := New(Options{LoopController: fake})
		err := a.OnCompactRequest(0)
		require.NoError(t, err)
		reqs := fake.getCompactRequests()
		require.Len(t, reqs, 1)
		assert.Equal(t, 0, reqs[0])
	})

	t.Run("nil_loop_controller_returns_sentinel", func(t *testing.T) {
		t.Parallel()
		a := New(Options{})
		err := a.OnCompactRequest(100)
		assert.ErrorIs(t, err, ErrLoopControllerUnavailable)
	})
}

// ---------------------------------------------------------------------------
// T-006: OnModelChange
// AC-CMDCTX-008
// ---------------------------------------------------------------------------

func TestContextAdapter_OnModelChange(t *testing.T) {
	t.Parallel()

	t.Run("delegates_once_with_correct_info", func(t *testing.T) {
		// AC-CMDCTX-008: model change recorded once with correct ID.
		t.Parallel()
		fake := &fakeLoopController{}
		a := New(Options{LoopController: fake})
		info := command.ModelInfo{ID: "anthropic/claude-opus-4-7", DisplayName: "Anthropic claude-opus-4-7"}
		err := a.OnModelChange(info)
		require.NoError(t, err)
		changes := fake.getModelChanges()
		require.Len(t, changes, 1)
		assert.Equal(t, "anthropic/claude-opus-4-7", changes[0].ID)
	})

	t.Run("nil_loop_controller_returns_sentinel", func(t *testing.T) {
		t.Parallel()
		a := New(Options{})
		err := a.OnModelChange(command.ModelInfo{ID: "x"})
		assert.ErrorIs(t, err, ErrLoopControllerUnavailable)
	})
}

// ---------------------------------------------------------------------------
// T-007: nil paths + integration
// ---------------------------------------------------------------------------

func TestContextAdapter_NilRegistry_NoModelResolution(t *testing.T) {
	t.Parallel()
	// AC-CMDCTX-011: nil registry, non-nil loop → only ResolveModelAlias fails.
	fake := &fakeLoopController{}
	a := New(Options{LoopController: fake})
	got, err := a.ResolveModelAlias("anthropic/claude-opus-4-7")
	assert.ErrorIs(t, err, command.ErrUnknownModel)
	assert.Nil(t, got)
	// Other methods still work.
	assert.NoError(t, a.OnClear())
}

func TestContextAdapter_WithContext_DoesNotMutateParent(t *testing.T) {
	t.Parallel()
	parent := New(Options{})
	ctx := subagent.WithTeammateIdentity(context.Background(), subagent.TeammateIdentity{PlanModeRequired: true})
	child := parent.WithContext(ctx)
	// child sees plan mode from ctx
	assert.True(t, child.PlanModeActive())
	// parent does not see it (no ctxHook)
	assert.False(t, parent.PlanModeActive())
}

// TestContextAdapter_AC019_NoLoopStateMutation verifies the static analysis constraint
// from AC-CMDCTX-019. This test is intentionally simple — the real check is the grep
// in the quality gate. It primarily documents the invariant.
func TestContextAdapter_AC019_NoLoopStateMutation(t *testing.T) {
	// This test documents REQ-CMDCTX-016: the adapter never mutates loop.State directly.
	// The actual enforcement is via the grep quality gate in the CI pipeline:
	//   grep -rE 'loop\.State\.[A-Z][A-Za-z]*\s*=' internal/command/adapter/ --include='*.go' | grep -v '_test.go'
	// Expected: 0 matches.
	t.Log("AC-CMDCTX-019: loop.State direct mutation prohibition documented")
}

// TestContextAdapter_EffectiveCtx_Background tests that a context-less adapter
// uses context.Background() as the effective context, covering the nil ctxHook branch.
func TestContextAdapter_EffectiveCtx_Background(t *testing.T) {
	t.Parallel()
	fake := &fakeLoopController{}
	a := New(Options{LoopController: fake})
	// ctxHook is nil (default) — effectiveCtx returns context.Background().
	// OnClear exercises the nil ctxHook path.
	err := a.OnClear()
	require.NoError(t, err)
	assert.Equal(t, 1, fake.getClearCount())
}

// TestContextAdapter_EffectiveCtx_WithContextHook tests that a child adapter
// uses its ctxHook, covering the non-nil ctxHook branch of effectiveCtx.
func TestContextAdapter_EffectiveCtx_WithContextHook(t *testing.T) {
	t.Parallel()
	fake := &fakeLoopController{}
	parent := New(Options{LoopController: fake})
	child := parent.WithContext(context.Background())
	// child.ctxHook != nil — effectiveCtx returns ctxHook.
	err := child.OnClear()
	require.NoError(t, err)
	assert.Equal(t, 1, fake.getClearCount())
}

// ---------------------------------------------------------------------------
// Alias test helpers
// ---------------------------------------------------------------------------

func TestResolveAlias_AliasMapPriority(t *testing.T) {
	t.Parallel()
	// Alias map entry takes priority over plain "provider/model" lookup.
	reg := router.DefaultRegistry()
	aliasMap := map[string]string{
		"sonnet": "anthropic/claude-sonnet-4-6",
	}
	got, err := resolveAlias(reg, aliasMap, "sonnet")
	require.NoError(t, err)
	assert.Equal(t, "anthropic/claude-sonnet-4-6", got.ID)
}

func TestResolveAlias_MultipleProviders(t *testing.T) {
	t.Parallel()
	reg := router.DefaultRegistry()
	cases := []struct {
		input   string
		wantID  string
		wantErr error
	}{
		{"openai/gpt-4o", "openai/gpt-4o", nil},
		{"google/gemini-2.0-flash", "google/gemini-2.0-flash", nil},
		{"xai/grok-2", "xai/grok-2", nil},
		{"deepseek/deepseek-chat", "deepseek/deepseek-chat", nil},
		{"openai/unknown-model", "", command.ErrUnknownModel},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("input_%s", tc.input), func(t *testing.T) {
			t.Parallel()
			got, err := resolveAlias(reg, nil, tc.input)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantID, got.ID)
			}
		})
	}
}
