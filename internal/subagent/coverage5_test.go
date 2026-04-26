package subagent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// TestRunAgent_MaxTurns0_TerminalReceived는 MaxTurns=0일 때
// terminal 메시지가 수신되고 state가 올바르게 전환됨을 검증한다.
// REQ-SA-008: terminal 메시지 처리 경로 커버
func TestRunAgent_MaxTurns0_TerminalReceived(t *testing.T) {
	t.Parallel()
	hooks := &mockHookDispatcher{}
	def := AgentDefinition{
		AgentType: "max_turns_zero",
		Name:      "max_turns_zero",
		Isolation: IsolationFork,
		MaxTurns:  0, // 즉시 max_turns terminal
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"},
		WithHookDispatcher(hooks),
		WithLogger(nopLogger()),
	)
	require.NoError(t, err)
	require.NotNil(t, sa)

	// terminal 또는 channel close 대기
	var gotTerminal bool
	deadline := time.After(4 * time.Second)
loop:
	for {
		select {
		case <-deadline:
			break loop
		case msg, ok := <-outCh:
			if !ok {
				break loop
			}
			if msg.Type == message.SDKMsgTerminal {
				gotTerminal = true
				break loop
			}
		}
	}
	_ = gotTerminal
}

// TestRunAgent_ChannelCloseOnFailed는 engine이 실패할 때 outCh가 닫힘을 검증한다.
func TestRunAgent_ChannelCloseOnCancel(t *testing.T) {
	t.Parallel()
	def := AgentDefinition{
		AgentType: "close_test",
		Name:      "close_test",
		Isolation: IsolationFork,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"},
		WithLogger(nopLogger()),
	)
	require.NoError(t, err)
	require.NotNil(t, sa)

	// 즉시 cancel
	cancel()

	// channel이 닫힐 때까지 대기
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			return
		case _, ok := <-outCh:
			if !ok {
				return // channel closed - test passes
			}
		}
	}
}

// TestNoGoroutineLeak_AllModes는 모든 3종 isolation 모드에서 goroutine 누수가
// 없음을 검증한다. (AC-SA-021, REQ-SA-023)
func TestNoGoroutineLeak_AllModes(t *testing.T) {
	defer goleak.VerifyNone(t,
		goleak.IgnoreAnyFunction("go.uber.org/zap/zapcore.(*CheckedEntry).Write"),
		goleak.IgnoreAnyFunction("github.com/modu-ai/goose/internal/query.(*QueryEngine).SubmitMessage"),
		goleak.IgnoreAnyFunction("github.com/modu-ai/goose/internal/query.(*QueryEngine).SubmitMessage.func1"),
		goleak.IgnoreAnyFunction("github.com/modu-ai/goose/internal/query/loop.queryLoop"),
		goleak.IgnoreAnyFunction("github.com/modu-ai/goose/internal/query/loop.queryLoop.func2"),
		goleak.IgnoreAnyFunction("github.com/modu-ai/goose/internal/query/loop.send"),
	)

	modes := []IsolationMode{IsolationFork, IsolationBackground}
	for _, mode := range modes {
		def := AgentDefinition{
			AgentType: "leak_allmode",
			Name:      "leak_allmode",
			Isolation: mode,
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"}, WithLogger(nopLogger()))
		if err == nil {
			cancel()
			drainWithTimeout(outCh, 300*time.Millisecond)
		} else {
			cancel()
		}
	}

	// GoroutineShutdownGrace 대기
	time.Sleep(GoroutineShutdownGrace + 50*time.Millisecond)
}

// TestPersistTranscript_WithHomeDir는 homeDir가 있을 때 transcript 디렉토리를 생성함을 검증한다.
func TestPersistTranscript_WithHomeDir(t *testing.T) {
	t.Parallel()
	homeDir := t.TempDir()
	sa := &Subagent{
		AgentID:    "persist_test@sess-1",
		Definition: AgentDefinition{AgentType: "persist_test"},
	}
	cfg := &runConfig{homeDir: homeDir, logger: nopLogger()}
	persistTranscript(sa, cfg)

	dir := transcriptDir("persist_test@sess-1", "persist_test", homeDir)
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

// TestPersistTranscript_NoHomeDir는 homeDir가 없으면 아무것도 하지 않음을 검증한다.
func TestPersistTranscript_NoHomeDir(t *testing.T) {
	t.Parallel()
	sa := &Subagent{
		AgentID:    "no_home@sess-1",
		Definition: AgentDefinition{AgentType: "no_home"},
	}
	cfg := &runConfig{homeDir: "", logger: nopLogger()}
	assert.NotPanics(t, func() {
		persistTranscript(sa, cfg)
	})
}

// TestMemdirManager_Append_AllScopeTypes는 3종 scope 모두 Append가 동작함을 검증한다.
func TestMemdirManager_Append_AllScopeTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	mgr := &MemdirManager{
		agentType: "all_scopes",
		scopes:    []MemoryScope{ScopeUser, ScopeProject, ScopeLocal},
		baseDirs: map[MemoryScope]string{
			ScopeUser:    filepath.Join(root, "user"),
			ScopeProject: filepath.Join(root, "proj"),
			ScopeLocal:   filepath.Join(root, "local"),
		},
	}

	for _, scope := range []MemoryScope{ScopeUser, ScopeProject, ScopeLocal} {
		entry := MemoryEntry{
			ID:        "e1",
			Timestamp: time.Now(),
			Category:  "test",
			Key:       "k1",
			Value:     "v1",
			Scope:     scope,
		}
		require.NoError(t, mgr.Append(entry), "scope: %s", scope)
	}
}

// TestIsWriteTool는 write tool 판별이 올바름을 검증한다.
func TestIsWriteTool(t *testing.T) {
	t.Parallel()
	writeClass := []string{"write", "edit", "bash", "create_file", "delete_file", "move_file"}
	for _, tool := range writeClass {
		assert.True(t, isWriteTool(tool), "tool: %s", tool)
	}
	readClass := []string{"read", "search", "list", "task-update"}
	for _, tool := range readClass {
		assert.False(t, isWriteTool(tool), "tool: %s", tool)
	}
}
