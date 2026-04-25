package hook_test

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/modu-ai/goose/internal/hook"
)

// TestPermissionBehavior_String 커버리지용
func TestPermissionBehavior_String(t *testing.T) {
	assert.Equal(t, "allow", hook.PermAllow.String())
	assert.Equal(t, "deny", hook.PermDeny.String())
	assert.Equal(t, "ask", hook.PermAsk.String())
	// unknown
	var unk hook.PermissionBehavior = 99
	assert.Equal(t, "unknown", unk.String())
}

// TestDispatchGeneric covers DispatchGeneric
func TestDispatchGeneric(t *testing.T) {
	reg := hook.NewHookRegistry()
	d := hook.NewDispatcher(reg, zap.NewNop())

	h := &countingHandler{}
	require.NoError(t, reg.Register(hook.EvStop, "*", h))

	input := hook.HookInput{HookEvent: hook.EvStop}
	res, err := d.DispatchGeneric(context.Background(), hook.EvStop, input)
	require.NoError(t, err)
	assert.Equal(t, 1, res.HandlerCount)
	assert.Equal(t, "ok", res.Outcome)
	assert.Equal(t, int64(1), h.Count())
}

// TestNoopPluginLoader covers NoopPluginLoader and LoadingPluginLoader
func TestNoopPluginLoader(t *testing.T) {
	noop := &hook.NoopPluginLoader{}
	assert.False(t, noop.IsLoading())
	assert.NoError(t, noop.Load(nil, hook.NewHookRegistry()))

	loading := &hook.LoadingPluginLoader{}
	assert.True(t, loading.IsLoading())
	assert.NoError(t, loading.Load(nil, hook.NewHookRegistry()))
}

// TestHandlerBindings covers HandlerBindings
func TestHandlerBindings(t *testing.T) {
	reg := hook.NewHookRegistry()
	h1 := &countingHandler{}
	h2 := &countingHandler{}
	require.NoError(t, reg.Register(hook.EvStop, "a", h1))
	require.NoError(t, reg.Register(hook.EvStop, "b", h2))

	bindings := reg.HandlerBindings(hook.EvStop)
	assert.Len(t, bindings, 2)
	assert.Equal(t, "a", bindings[0].Matcher)
	assert.Equal(t, "b", bindings[1].Matcher)
}

// TestDispatchPostToolUse_Async covers PostToolUse async path
func TestDispatchPostToolUse_Async(t *testing.T) {
	reg := hook.NewHookRegistry()
	d := hook.NewDispatcher(reg, zap.NewNop())

	// async handler
	h := &hook.InlineFuncHandler{
		Fn: func(_ context.Context, _ hook.HookInput) (hook.HookJSONOutput, error) {
			return hook.HookJSONOutput{
				Async:             true,
				AsyncTimeout:      1,
				AdditionalContext: "async_ctx",
			}, nil
		},
	}
	require.NoError(t, reg.Register(hook.EvPostToolUse, "*", h))

	input := hook.HookInput{
		HookEvent: hook.EvPostToolUse,
		Tool:      &hook.ToolInfo{Name: "t"},
	}
	result, err := d.DispatchPostToolUse(context.Background(), input)
	require.NoError(t, err)
	_ = result
}

// TestDefaultPermissionQueue_ClearYoloApprovals covers ClearYoloApprovals
func TestDefaultPermissionQueue_ClearYoloApprovals(t *testing.T) {
	q := hook.NewDefaultPermissionQueue(zap.NewNop())
	q.SetYoloClassifierApproval("tool_x")
	assert.True(t, q.IsYoloApproved("tool_x"))

	q.ClearYoloApprovals()
	assert.False(t, q.IsYoloApproved("tool_x"))
}

// TestUseCanUseTool_CoordinatorRoute covers Coordinator role path
func TestUseCanUseTool_CoordinatorRoute(t *testing.T) {
	reg := hook.NewHookRegistry()
	d := hook.NewDispatcher(reg, zap.NewNop())

	coordSpy := &countingHandler{}
	d.Coordinator = &spyCoordinatorHandler{counter: coordSpy, result: hook.PermissionResult{
		Behavior:       hook.PermAllow,
		DecisionReason: &hook.DecisionReason{Type: "coordinator"},
	}}

	result := d.UseCanUseTool(
		context.Background(),
		"some_tool",
		map[string]any{},
		hook.PermissionContext{Role: hook.RoleCoordinator},
	)

	assert.Equal(t, hook.PermAllow, result.Behavior)
	assert.Equal(t, int64(1), coordSpy.Count())
}

// TestUseCanUseTool_SwarmWorkerRoute covers SwarmWorker role path
func TestUseCanUseTool_SwarmWorkerRoute(t *testing.T) {
	reg := hook.NewHookRegistry()
	d := hook.NewDispatcher(reg, zap.NewNop())

	workerSpy := &countingHandler{}
	d.SwarmWorker = &spySwarmWorkerHandler{counter: workerSpy, result: hook.PermissionResult{
		Behavior:       hook.PermAllow,
		DecisionReason: &hook.DecisionReason{Type: "swarm"},
	}}

	result := d.UseCanUseTool(
		context.Background(),
		"some_tool",
		map[string]any{},
		hook.PermissionContext{Role: hook.RoleSwarmWorker},
	)

	assert.Equal(t, hook.PermAllow, result.Behavior)
	assert.Equal(t, int64(1), workerSpy.Count())
}

// TestUseCanUseTool_InteractiveRoute_TTY covers Interactive role with TTY=true
func TestUseCanUseTool_InteractiveRoute_TTY(t *testing.T) {
	reg := hook.NewHookRegistry()
	d := hook.NewDispatcher(reg, zap.NewNop())
	d.IsTTY = func() bool { return true }

	interactiveSpy := &countingHandler{}
	d.Interactive = &spyInteractiveHandler{counter: interactiveSpy}

	result := d.UseCanUseTool(
		context.Background(),
		"some_tool",
		map[string]any{},
		hook.PermissionContext{Role: hook.RoleInteractive},
	)

	assert.Equal(t, hook.PermAllow, result.Behavior)
	assert.Equal(t, int64(1), interactiveSpy.Count())
}

// TestUseCanUseTool_UnknownRole covers unknown role → RoleInteractive fallback
func TestUseCanUseTool_UnknownRole(t *testing.T) {
	reg := hook.NewHookRegistry()
	logger, logs := newObservedLogger()
	d := hook.NewDispatcher(reg, logger)
	d.IsTTY = func() bool { return false } // non-TTY → Deny

	result := d.UseCanUseTool(
		context.Background(),
		"some_tool",
		map[string]any{},
		hook.PermissionContext{Role: "unknown_role_xyz"},
	)

	assert.Equal(t, hook.PermDeny, result.Behavior)
	// WARN 로그 확인
	hasWarn := false
	for _, entry := range logs.All() {
		if entry.Level == zap.WarnLevel {
			hasWarn = true
			break
		}
	}
	assert.True(t, hasWarn)
}

// TestUseCanUseTool_EmptyRole covers empty role → RoleNonTTY
func TestUseCanUseTool_EmptyRole(t *testing.T) {
	reg := hook.NewHookRegistry()
	d := hook.NewDispatcher(reg, zap.NewNop())

	result := d.UseCanUseTool(
		context.Background(),
		"some_tool",
		map[string]any{},
		hook.PermissionContext{Role: ""},
	)

	assert.Equal(t, hook.PermDeny, result.Behavior)
}

// TestDispatchSessionStart_MultipleInitialMessages covers WARN log path
func TestDispatchSessionStart_MultipleInitialMessages(t *testing.T) {
	logger, logs := newObservedLogger()
	reg := hook.NewHookRegistry(hook.WithLogger(logger))
	d := hook.NewDispatcher(reg, logger)

	h1 := &hook.InlineFuncHandler{
		Fn: func(_ context.Context, _ hook.HookInput) (hook.HookJSONOutput, error) {
			return hook.HookJSONOutput{InitialUserMessage: "first"}, nil
		},
	}
	h2 := &hook.InlineFuncHandler{
		Fn: func(_ context.Context, _ hook.HookInput) (hook.HookJSONOutput, error) {
			return hook.HookJSONOutput{InitialUserMessage: "second"}, nil
		},
	}
	require.NoError(t, reg.Register(hook.EvSessionStart, "*", h1))
	require.NoError(t, reg.Register(hook.EvSessionStart, "*", h2))

	result, err := d.DispatchSessionStart(context.Background(), hook.HookInput{})
	require.NoError(t, err)
	assert.Equal(t, "second", result.InitialUserMessage)

	// WARN log 확인 (multiple handlers set initialUserMessage)
	hasWarn := false
	for _, entry := range logs.All() {
		if entry.Level == zap.WarnLevel {
			hasWarn = true
			break
		}
	}
	assert.True(t, hasWarn, "should WARN when multiple handlers set InitialUserMessage")
}

// TestShellHook_ExitCode2 covers exit code 2 → blocking signal (REQ-HK-006 e)
func TestShellHook_ExitCode2(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	h := &hook.InlineCommandHandler{
		Command: "echo 'blocked by policy' >&2; exit 2",
		Matcher: "*",
		Shell:   "/bin/sh",
		Timeout: 5 * time.Second,
	}

	ctx := context.Background()
	out, err := h.Handle(ctx, hook.HookInput{
		HookEvent: hook.EvPreToolUse,
		Tool:      &hook.ToolInfo{Name: "t"},
	})

	// exit code 2는 handler_error가 아니다 (REQ-HK-006 e)
	assert.NoError(t, err)
	require.NotNil(t, out.Continue)
	assert.False(t, *out.Continue, "exit code 2 should produce Continue:false")
}

// TestDispatchFileChanged_WithHandlers covers FileChanged with internal handlers
func TestDispatchFileChanged_WithHandlers(t *testing.T) {
	reg := hook.NewHookRegistry()
	d := hook.NewDispatcher(reg, zap.NewNop())

	h := &countingHandler{}
	require.NoError(t, reg.Register(hook.EvFileChanged, "*.ts", h))

	activated, err := d.DispatchFileChanged(context.Background(), []string{"foo.ts", "bar.go"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), h.Count(), "handler should match *.ts file")
	assert.Nil(t, activated) // no consumer registered
}

// TestShellHook_NonZeroExitCode covers non-2 non-zero exit code → handler_error
func TestShellHook_NonZeroExitCode(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	h := &hook.InlineCommandHandler{
		Command: "exit 1",
		Matcher: "*",
		Shell:   "/bin/sh",
		Timeout: 5 * time.Second,
	}

	ctx := context.Background()
	_, err := h.Handle(ctx, hook.HookInput{
		HookEvent: hook.EvPreToolUse,
		Tool:      &hook.ToolInfo{Name: "t"},
	})
	// exit 1은 handler_error이므로 에러 반환
	assert.Error(t, err)
}

// TestDispatchPreToolUse_Async_PostToolUse covers PostToolUse with async appending AdditionalContext
func TestDispatchPostToolUse_SyncResult(t *testing.T) {
	reg := hook.NewHookRegistry()
	d := hook.NewDispatcher(reg, zap.NewNop())

	h := &hook.InlineFuncHandler{
		Fn: func(_ context.Context, _ hook.HookInput) (hook.HookJSONOutput, error) {
			return hook.HookJSONOutput{
				AdditionalContext: "extra",
				SuppressOutput:    true,
			}, nil
		},
	}
	require.NoError(t, reg.Register(hook.EvPostToolUse, "*", h))

	input := hook.HookInput{
		HookEvent: hook.EvPostToolUse,
		Tool:      &hook.ToolInfo{Name: "t"},
	}
	result, err := d.DispatchPostToolUse(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, "extra", result.AdditionalContext)
	assert.True(t, result.SuppressOutput)
}

// TestRegistry_BindingMatches_DefaultEvent covers default case in bindingMatches
func TestRegistry_BindingMatches_DefaultEvent(t *testing.T) {
	reg := hook.NewHookRegistry()
	h := &countingHandler{}
	require.NoError(t, reg.Register(hook.EvStop, "Stop", h))

	// default case: event is not PreToolUse/PostToolUse/FileChanged
	handlers := reg.Handlers(hook.EvStop, hook.HookInput{HookEvent: hook.EvStop})
	assert.Len(t, handlers, 1)
}

// TestDispatchPermissionDenied_NoHandlers covers PermissionDenied with no handlers
func TestDispatchPermissionDenied_NoHandlers(t *testing.T) {
	reg := hook.NewHookRegistry()
	d := hook.NewDispatcher(reg, zap.NewNop())

	result := d.DispatchPermissionDenied(context.Background(), hook.PermissionResult{
		Behavior: hook.PermDeny,
	})
	assert.Equal(t, 0, result.HandlerCount)
	assert.Equal(t, "ok", result.Outcome)
}

// spyCoordinatorHandler wraps countingHandler as CoordinatorHandler
type spyCoordinatorHandler struct {
	counter *countingHandler
	result  hook.PermissionResult
}

func (s *spyCoordinatorHandler) RequestPermission(_ context.Context, _ string, _ map[string]any) (hook.PermissionResult, error) {
	s.counter.count.Add(1)
	return s.result, nil
}

// spySwarmWorkerHandler wraps countingHandler as SwarmWorkerHandler
type spySwarmWorkerHandler struct {
	counter *countingHandler
	result  hook.PermissionResult
}

func (s *spySwarmWorkerHandler) BubbleUpPermission(_ context.Context, _ string, _ map[string]any) (hook.PermissionResult, error) {
	s.counter.count.Add(1)
	return s.result, nil
}

// TestInlineFuncHandler_MatcherFn covers Matches() with non-nil MatcherFn
func TestInlineFuncHandler_MatcherFn(t *testing.T) {
	called := false
	h := &hook.InlineFuncHandler{
		Fn: func(_ context.Context, _ hook.HookInput) (hook.HookJSONOutput, error) {
			return hook.HookJSONOutput{}, nil
		},
		MatcherFn: func(input hook.HookInput) bool {
			called = true
			return input.Tool != nil && input.Tool.Name == "target"
		},
	}
	input := hook.HookInput{HookEvent: hook.EvPreToolUse, Tool: &hook.ToolInfo{Name: "target"}}
	assert.True(t, h.Matches(input))
	assert.True(t, called)

	input2 := hook.HookInput{HookEvent: hook.EvPreToolUse, Tool: &hook.ToolInfo{Name: "other"}}
	assert.False(t, h.Matches(input2))
}

// TestInlineCommandHandler_Matches covers Matches() for command handler
func TestInlineCommandHandler_Matches(t *testing.T) {
	h := &hook.InlineCommandHandler{Matcher: "bash", Command: "echo '{}'"}

	// PreToolUse: matches tool name "bash"
	input := hook.HookInput{HookEvent: hook.EvPreToolUse, Tool: &hook.ToolInfo{Name: "bash"}}
	assert.True(t, h.Matches(input))

	// PreToolUse: no match for "python"
	input2 := hook.HookInput{HookEvent: hook.EvPreToolUse, Tool: &hook.ToolInfo{Name: "python"}}
	assert.False(t, h.Matches(input2))

	// FileChanged: matches path
	h2 := &hook.InlineCommandHandler{Matcher: "*.go", Command: "echo '{}'"}
	input3 := hook.HookInput{
		HookEvent:    hook.EvFileChanged,
		ChangedPaths: []string{"main.go"},
	}
	assert.True(t, h2.Matches(input3))

	// FileChanged: no match
	input4 := hook.HookInput{
		HookEvent:    hook.EvFileChanged,
		ChangedPaths: []string{"main.ts"},
	}
	assert.False(t, h2.Matches(input4))

	// Default event: matches event name
	h3 := &hook.InlineCommandHandler{Matcher: "Stop", Command: "echo '{}'"}
	input5 := hook.HookInput{HookEvent: hook.EvStop}
	assert.True(t, h3.Matches(input5))
}

// TestNewDefaultPermissionQueue_NilLogger covers nil logger path
func TestNewDefaultPermissionQueue_NilLogger(t *testing.T) {
	q := hook.NewDefaultPermissionQueue(nil)
	require.NotNil(t, q)
	// Should not panic with nil logger
	q.SetYoloClassifierApproval("tool")
	assert.True(t, q.IsYoloApproved("tool"))
}

// TestValidateInput_NilChangedPaths covers validateInput nil ChangedPaths branch
func TestDispatchFileChanged_NilChangedPaths_Error(t *testing.T) {
	reg := hook.NewHookRegistry()
	d := hook.NewDispatcher(reg, zap.NewNop())

	// nil ChangedPaths → ErrInvalidHookInput
	_, err := d.DispatchFileChanged(context.Background(), nil)
	require.Error(t, err)
}
