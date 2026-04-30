// Package agent provides tests for the outer orchestration layer.
package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/permissions"
	"github.com/modu-ai/goose/internal/query"
	"github.com/modu-ai/goose/internal/query/loop"
	"go.uber.org/zap"
)

// mockCanUseTool is a mock permission checker that always allows.
type mockCanUseTool struct{}

func (m *mockCanUseTool) Check(ctx context.Context, tpc permissions.ToolPermissionContext) permissions.Decision {
	return permissions.Decision{
		Behavior: permissions.Allow,
		Reason:   "",
	}
}

// mockEngineConfigFactory creates a mock QueryEngineConfig.
func mockEngineConfigFactory(task Task) (query.QueryEngineConfig, error) {
	return query.QueryEngineConfig{
		LLMCall: func(ctx context.Context, req query.LLMCallReq) (<-chan message.StreamEvent, error) {
			ch := make(chan message.StreamEvent, 1)
			close(ch)
			return ch, nil
		},
		Tools:      []query.ToolDefinition{},
		CanUseTool: &mockCanUseTool{},
		Executor:   &mockExecutor{},
		Logger:     zap.NewNop(),
	}, nil
}

// mockExecutor is a mock tool executor.
type mockExecutor struct{}

func (m *mockExecutor) Run(ctx context.Context, toolUseID, toolName string, input map[string]any) (string, error) {
	return "mock result", nil
}

// TestNewAgentRunner tests the AgentRunner constructor.
func TestNewAgentRunner(t *testing.T) {
	tests := []struct {
		name    string
		cfg     AgentRunnerConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: AgentRunnerConfig{
				NewEngineConfig: mockEngineConfigFactory,
				Logger:          zap.NewNop(),
				MaxReplans:      2,
			},
			wantErr: false,
		},
		{
			name: "missing factory",
			cfg: AgentRunnerConfig{
				Logger:     zap.NewNop(),
				MaxReplans: 2,
			},
			wantErr: true,
		},
		{
			name: "default max replans",
			cfg: AgentRunnerConfig{
				NewEngineConfig: mockEngineConfigFactory,
				Logger:          zap.NewNop(),
				MaxReplans:      0, // Should default to 2
			},
			wantErr: false,
		},
		{
			name: "nil logger uses nop",
			cfg: AgentRunnerConfig{
				NewEngineConfig: mockEngineConfigFactory,
				Logger:          nil,
				MaxReplans:      2,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner, err := NewAgentRunner(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAgentRunner() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if runner == nil {
					t.Error("NewAgentRunner() returned nil runner")
					return
				}
				if runner.cfg.MaxReplans == 0 && tt.cfg.MaxReplans == 0 {
					// Should have defaulted to 2
					if runner.cfg.MaxReplans != 2 {
						t.Errorf("MaxReplans should default to 2, got %d", runner.cfg.MaxReplans)
					}
				}
			}
		})
	}
}

// TestRunTask_Success tests successful task execution without reflection.
func TestRunTask_Success(t *testing.T) {
	cfg := AgentRunnerConfig{
		NewEngineConfig: mockEngineConfigFactory,
		ReflectHooks:    nil,
		MaxReplans:      2,
		Logger:          zap.NewNop(),
	}

	runner, err := NewAgentRunner(cfg)
	if err != nil {
		t.Fatalf("NewAgentRunner() failed: %v", err)
	}

	task := &Task{
		ID:     "test-1",
		Prompt: "test prompt",
		State:  TaskPending,
	}

	ctx := context.Background()
	result, err := runner.RunTask(ctx, task)

	if err != nil {
		t.Errorf("RunTask() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("RunTask() returned nil result")
	}
	if result.TaskID != task.ID {
		t.Errorf("TaskID = %s, want %s", result.TaskID, task.ID)
	}
	if result.Error != nil {
		t.Errorf("Error = %v, want nil", result.Error)
	}
	if task.State != TaskReflected {
		t.Errorf("State = %s, want TaskReflected", task.State)
	}
}

// TestRunTask_WithReflection tests task execution with reflection hooks.
func TestRunTask_WithReflection(t *testing.T) {
	// Mock reflect hook that returns high score
	highScoreHook := func(ctx context.Context, task Task, finalState loop.State) (*ReflectResult, error) {
		return &ReflectResult{
			Score:         0.9,
			Gap:           "",
			Inconsistency: "",
			Unsupported:   "",
			RawOutput:     "SCORE: 0.9",
		}, nil
	}

	cfg := AgentRunnerConfig{
		NewEngineConfig: mockEngineConfigFactory,
		ReflectHooks:    []ReflectHook{highScoreHook},
		MaxReplans:      2,
		Logger:          zap.NewNop(),
	}

	runner, err := NewAgentRunner(cfg)
	if err != nil {
		t.Fatalf("NewAgentRunner() failed: %v", err)
	}

	task := &Task{
		ID:     "test-2",
		Prompt: "test prompt",
		State:  TaskPending,
	}

	ctx := context.Background()
	result, err := runner.RunTask(ctx, task)

	if err != nil {
		t.Errorf("RunTask() unexpected error: %v", err)
	}
	if result.Reflect == nil {
		t.Error("Reflect result is nil")
	} else if result.Reflect.Score != 0.9 {
		t.Errorf("Reflect.Score = %f, want 0.9", result.Reflect.Score)
	}
	if result.ReplanCount != 0 {
		t.Errorf("ReplanCount = %d, want 0 (high score, no re-plan)", result.ReplanCount)
	}
}

// TestRunTask_ReplanTriggered tests that low reflection score triggers re-plan.
func TestRunTask_ReplanTriggered(t *testing.T) {
	callCount := 0

	// Mock reflect hook that returns low score on first call, high on second
	lowThenHighHook := func(ctx context.Context, task Task, finalState loop.State) (*ReflectResult, error) {
		callCount++
		if callCount == 1 {
			return &ReflectResult{
				Score:         0.5,
				Gap:           "missing implementation",
				Inconsistency: "",
				Unsupported:   "",
				RawOutput:     "SCORE: 0.5",
			}, nil
		}
		return &ReflectResult{
			Score:         0.9,
			Gap:           "",
			Inconsistency: "",
			Unsupported:   "",
			RawOutput:     "SCORE: 0.9",
		}, nil
	}

	cfg := AgentRunnerConfig{
		NewEngineConfig: mockEngineConfigFactory,
		ReflectHooks:    []ReflectHook{lowThenHighHook},
		MaxReplans:      2,
		Logger:          zap.NewNop(),
	}

	runner, err := NewAgentRunner(cfg)
	if err != nil {
		t.Fatalf("NewAgentRunner() failed: %v", err)
	}

	task := &Task{
		ID:     "test-3",
		Prompt: "test prompt",
		State:  TaskPending,
	}

	ctx := context.Background()
	result, err := runner.RunTask(ctx, task)

	if err != nil {
		t.Errorf("RunTask() unexpected error: %v", err)
	}
	if result.ReplanCount != 1 {
		t.Errorf("ReplanCount = %d, want 1 (one re-plan triggered)", result.ReplanCount)
	}
	if callCount != 2 {
		t.Errorf("Reflect hook called %d times, want 2", callCount)
	}
	// Verify that critique feedback was appended to prompt
	if !contains(task.Prompt, "Self-Critique Feedback") {
		t.Error("Critique feedback not appended to prompt")
	}
}

// TestRunTask_ContextCancellation tests context cancellation handling.
func TestRunTask_ContextCancellation(t *testing.T) {
	cfg := AgentRunnerConfig{
		NewEngineConfig: mockEngineConfigFactory,
		ReflectHooks:    nil,
		MaxReplans:      2,
		Logger:          zap.NewNop(),
	}

	runner, err := NewAgentRunner(cfg)
	if err != nil {
		t.Fatalf("NewAgentRunner() failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	task := &Task{
		ID:     "test-4",
		Prompt: "test prompt",
		State:  TaskPending,
	}

	result, err := runner.RunTask(ctx, task)

	if err != nil {
		t.Errorf("RunTask() unexpected error: %v", err)
	}
	if task.State != TaskCancelled {
		t.Errorf("State = %s, want TaskCancelled", task.State)
	}
	if result.Error == nil {
		t.Error("Error is nil, want context cancellation error")
	}
}

// TestRunTask_EngineConfigFailure tests engine config creation failure.
func TestRunTask_EngineConfigFailure(t *testing.T) {
	failingFactory := func(task Task) (query.QueryEngineConfig, error) {
		return query.QueryEngineConfig{}, errors.New("config creation failed")
	}

	cfg := AgentRunnerConfig{
		NewEngineConfig: failingFactory,
		ReflectHooks:    nil,
		MaxReplans:      2,
		Logger:          zap.NewNop(),
	}

	runner, err := NewAgentRunner(cfg)
	if err != nil {
		t.Fatalf("NewAgentRunner() failed: %v", err)
	}

	task := &Task{
		ID:     "test-5",
		Prompt: "test prompt",
		State:  TaskPending,
	}

	ctx := context.Background()
	result, err := runner.RunTask(ctx, task)

	if err != nil {
		t.Errorf("RunTask() unexpected error: %v", err)
	}
	if task.State != TaskFailed {
		t.Errorf("State = %s, want TaskFailed", task.State)
	}
	if result.Error == nil {
		t.Error("Error is nil, want engine config error")
	}
}

// TestTaskState_String tests TaskState string representation.
func TestTaskState_String(t *testing.T) {
	tests := []struct {
		state TaskState
		want  string
	}{
		{TaskPending, "pending"},
		{TaskRunning, "running"},
		{TaskCompleted, "completed"},
		{TaskReflected, "reflected"},
		{TaskCancelled, "cancelled"},
		{TaskFailed, "failed"},
		{TaskState(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("String() = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestParseCritiqueOutput tests the critique response parser.
func TestParseCritiqueOutput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *ReflectResult
	}{
		{
			name:  "valid output",
			input: "SCORE: 0.8\nGAP: missing tests\nINCONSISTENCY: none\nUNSUPPORTED: claims A and B",
			want: &ReflectResult{
				Score:         0.8,
				Gap:           "missing tests",
				Inconsistency: "none",
				Unsupported:   "claims A and B",
			},
		},
		{
			name:  "missing fields",
			input: "SCORE: 0.5",
			want: &ReflectResult{
				Score:         0.5,
				Gap:           "",
				Inconsistency: "",
				Unsupported:   "",
			},
		},
		{
			name:  "invalid score defaults to 0.5",
			input: "SCORE: invalid",
			want: &ReflectResult{
				Score:         0.5,
				Gap:           "",
				Inconsistency: "",
				Unsupported:   "",
			},
		},
		{
			name:  "multiline values",
			input: "SCORE: 0.7\nGAP: line 1\nline 2\nINCONSISTENCY: none\nUNSUPPORTED: claim 1\nclaim 2",
			want: &ReflectResult{
				Score:         0.7,
				Gap:           "line 1\nline 2",
				Inconsistency: "none",
				Unsupported:   "claim 1\nclaim 2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCritiqueOutput(tt.input)
			if got.Score != tt.want.Score {
				t.Errorf("Score = %f, want %f", got.Score, tt.want.Score)
			}
			if got.Gap != tt.want.Gap {
				t.Errorf("Gap = %q, want %q", got.Gap, tt.want.Gap)
			}
			if got.Inconsistency != tt.want.Inconsistency {
				t.Errorf("Inconsistency = %q, want %q", got.Inconsistency, tt.want.Inconsistency)
			}
			if got.Unsupported != tt.want.Unsupported {
				t.Errorf("Unsupported = %q, want %q", got.Unsupported, tt.want.Unsupported)
			}
		})
	}
}

// TestBuildCritiquePrompt tests the critique prompt builder.
func TestBuildCritiquePrompt(t *testing.T) {
	taskPrompt := "Write a function that adds two numbers"
	messages := []message.Message{
		{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Write a function"}}},
		{Role: "assistant", Content: []message.ContentBlock{{Type: "text", Text: "Here is the function"}}},
	}

	prompt := buildCritiquePrompt(taskPrompt, messages)

	if !contains(prompt, "You are an AI output quality evaluator") {
		t.Error("Prompt missing system instruction")
	}
	if !contains(prompt, taskPrompt) {
		t.Error("Prompt missing task description")
	}
	if !contains(prompt, "SCORE:") {
		t.Error("Prompt missing SCORE format")
	}
	if !contains(prompt, "GAP:") {
		t.Error("Prompt missing GAP format")
	}
	if !contains(prompt, "INCONSISTENCY:") {
		t.Error("Prompt missing INCONSISTENCY format")
	}
	if !contains(prompt, "UNSUPPORTED:") {
		t.Error("Prompt missing UNSUPPORTED format")
	}
}

// TestNewSelfCritiqueHook tests the self-critique hook factory.
func TestNewSelfCritiqueHook(t *testing.T) {
	tests := []struct {
		name    string
		cfg     SelfCritiqueConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: SelfCritiqueConfig{
				LLMCall: func(ctx context.Context, req query.LLMCallReq) (<-chan message.StreamEvent, error) {
					ch := make(chan message.StreamEvent, 1)
					close(ch)
					return ch, nil
				},
				Logger: zap.NewNop(),
			},
			wantErr: false,
		},
		{
			name: "missing LLMCall",
			cfg: SelfCritiqueConfig{
				LLMCall: nil,
				Logger:  zap.NewNop(),
			},
			wantErr: true,
		},
		{
			name: "nil logger uses nop",
			cfg: SelfCritiqueConfig{
				LLMCall: func(ctx context.Context, req query.LLMCallReq) (<-chan message.StreamEvent, error) {
					ch := make(chan message.StreamEvent, 1)
					close(ch)
					return ch, nil
				},
				Logger: nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook, err := NewSelfCritiqueHook(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSelfCritiqueHook() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && hook == nil {
				t.Error("NewSelfCritiqueHook() returned nil hook")
			}
		})
	}
}

// TestRunTask_MaxReplansExhausted tests behavior when max replans is reached.
func TestRunTask_MaxReplansExhausted(t *testing.T) {
	// Mock hook that always returns low score
	lowScoreHook := func(ctx context.Context, task Task, finalState loop.State) (*ReflectResult, error) {
		return &ReflectResult{
			Score:         0.5,
			Gap:           "always missing",
			Inconsistency: "",
			Unsupported:   "",
			RawOutput:     "SCORE: 0.5",
		}, nil
	}

	cfg := AgentRunnerConfig{
		NewEngineConfig: mockEngineConfigFactory,
		ReflectHooks:    []ReflectHook{lowScoreHook},
		MaxReplans:      1, // Allow only one re-plan
		Logger:          zap.NewNop(),
	}

	runner, err := NewAgentRunner(cfg)
	if err != nil {
		t.Fatalf("NewAgentRunner() failed: %v", err)
	}

	task := &Task{
		ID:     "test-6",
		Prompt: "test prompt",
		State:  TaskPending,
	}

	ctx := context.Background()
	result, err := runner.RunTask(ctx, task)

	if err != nil {
		t.Errorf("RunTask() unexpected error: %v", err)
	}
	// Should complete after max replans exhausted
	if task.State != TaskReflected {
		t.Errorf("State = %s, want TaskReflected (max replans exhausted)", task.State)
	}
	if result.ReplanCount != 1 {
		t.Errorf("ReplanCount = %d, want 1 (max replans exhausted)", result.ReplanCount)
	}
}

// TestRunTask_ContextTimeout tests context timeout handling.
func TestRunTask_ContextTimeout(t *testing.T) {
	cfg := AgentRunnerConfig{
		NewEngineConfig: mockEngineConfigFactory,
		ReflectHooks:    nil,
		MaxReplans:      2,
		Logger:          zap.NewNop(),
	}

	runner, err := NewAgentRunner(cfg)
	if err != nil {
		t.Fatalf("NewAgentRunner() failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond) // Ensure timeout occurs

	task := &Task{
		ID:     "test-7",
		Prompt: "test prompt",
		State:  TaskPending,
	}

	result, err := runner.RunTask(ctx, task)

	if err != nil {
		t.Errorf("RunTask() unexpected error: %v", err)
	}
	if task.State != TaskCancelled {
		t.Errorf("State = %s, want TaskCancelled (timeout)", task.State)
	}
	if result.Error == nil {
		t.Error("Error is nil, want timeout error")
	}
}

// TestRunTask_MultipleReflectHooks tests execution of multiple reflect hooks.
func TestRunTask_MultipleReflectHooks(t *testing.T) {
	callOrder := []string{}

	hook1 := func(ctx context.Context, task Task, finalState loop.State) (*ReflectResult, error) {
		callOrder = append(callOrder, "hook1")
		return &ReflectResult{Score: 0.8, RawOutput: "SCORE: 0.8"}, nil
	}

	hook2 := func(ctx context.Context, task Task, finalState loop.State) (*ReflectResult, error) {
		callOrder = append(callOrder, "hook2")
		return &ReflectResult{Score: 0.85, RawOutput: "SCORE: 0.85"}, nil
	}

	hook3 := func(ctx context.Context, task Task, finalState loop.State) (*ReflectResult, error) {
		callOrder = append(callOrder, "hook3")
		return &ReflectResult{Score: 0.9, RawOutput: "SCORE: 0.9"}, nil
	}

	cfg := AgentRunnerConfig{
		NewEngineConfig: mockEngineConfigFactory,
		ReflectHooks:    []ReflectHook{hook1, hook2, hook3},
		MaxReplans:      2,
		Logger:          zap.NewNop(),
	}

	runner, err := NewAgentRunner(cfg)
	if err != nil {
		t.Fatalf("NewAgentRunner() failed: %v", err)
	}

	task := &Task{
		ID:     "test-8",
		Prompt: "test prompt",
		State:  TaskPending,
	}

	ctx := context.Background()
	result, err := runner.RunTask(ctx, task)

	if err != nil {
		t.Errorf("RunTask() unexpected error: %v", err)
	}

	// All hooks should be called
	if len(callOrder) != 3 {
		t.Errorf("Hook call count = %d, want 3", len(callOrder))
	}

	// Hooks should be called in order
	if callOrder[0] != "hook1" || callOrder[1] != "hook2" || callOrder[2] != "hook3" {
		t.Errorf("Hooks called in wrong order: %v", callOrder)
	}

	// Minimum score should be used (0.8 from hook1)
	if result.Reflect == nil {
		t.Fatal("Reflect result is nil")
	}
	if result.Reflect.Score != 0.8 {
		t.Errorf("Reflect.Score = %f, want 0.8 (minimum)", result.Reflect.Score)
	}
}

// TestRunTask_ReflectHookError tests error handling in reflect hooks.
func TestRunTask_ReflectHookError(t *testing.T) {
	// First hook succeeds, second fails, third succeeds
	hook1 := func(ctx context.Context, task Task, finalState loop.State) (*ReflectResult, error) {
		return &ReflectResult{Score: 0.7, RawOutput: "SCORE: 0.7"}, nil
	}

	hook2 := func(ctx context.Context, task Task, finalState loop.State) (*ReflectResult, error) {
		return nil, errors.New("hook2 failed")
	}

	hook3 := func(ctx context.Context, task Task, finalState loop.State) (*ReflectResult, error) {
		return &ReflectResult{Score: 0.8, RawOutput: "SCORE: 0.8"}, nil
	}

	cfg := AgentRunnerConfig{
		NewEngineConfig: mockEngineConfigFactory,
		ReflectHooks:    []ReflectHook{hook1, hook2, hook3},
		MaxReplans:      2,
		Logger:          zap.NewNop(),
	}

	runner, err := NewAgentRunner(cfg)
	if err != nil {
		t.Fatalf("NewAgentRunner() failed: %v", err)
	}

	task := &Task{
		ID:     "test-9",
		Prompt: "test prompt",
		State:  TaskPending,
	}

	ctx := context.Background()
	result, err := runner.RunTask(ctx, task)

	if err != nil {
		t.Errorf("RunTask() unexpected error: %v", err)
	}

	// Should use minimum of successful hooks (0.7 from hook1)
	if result.Reflect == nil {
		t.Fatal("Reflect result is nil")
	}
	if result.Reflect.Score != 0.7 {
		t.Errorf("Reflect.Score = %f, want 0.7 (minimum of successful hooks)", result.Reflect.Score)
	}
}

// Helper function to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr ||
		s[len(s)-len(substr):] == substr ||
		containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
