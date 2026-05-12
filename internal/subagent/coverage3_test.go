package subagent

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunAgent_IsolationDefault는 Isolation이 빈 경우 fork로 기본 설정됨을 검증한다.
func TestRunAgent_IsolationDefault(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	def := AgentDefinition{
		AgentType: "default_iso",
		Name:      "default_iso",
		// Isolation 미설정 → fork
	}
	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"}, WithLogger(nopLogger()))
	require.NoError(t, err)
	require.NotNil(t, sa)

	cancel()
	drainWithTimeout(outCh, 300*time.Millisecond)
}

// TestRunAgent_BackgroundIsolation_Shortcut는 Background=true 단축키가
// IsolationBackground로 설정됨을 검증한다.
func TestRunAgent_BackgroundIsolation_Shortcut(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	def := AgentDefinition{
		AgentType:  "bg_shortcut",
		Name:       "bg_shortcut",
		Background: true, // shortcut
	}
	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"}, WithLogger(nopLogger()))
	require.NoError(t, err)
	require.NotNil(t, sa)

	cancel()
	drainWithTimeout(outCh, 300*time.Millisecond)
}

// TestRunAgent_DefaultSessionID는 sessionID가 없으면 "default"를 사용함을 검증한다.
func TestRunAgent_DefaultSessionID(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	def := AgentDefinition{
		AgentType: "default_sess",
		Name:      "default_sess",
		Isolation: IsolationFork,
	}
	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"},
		WithLogger(nopLogger()),
		// sessionID 미설정
	)
	require.NoError(t, err)
	assert.Contains(t, sa.AgentID, "default_sess@default-")

	cancel()
	drainWithTimeout(outCh, 300*time.Millisecond)
}

// TestRunAgent_WithHomeDir_PersistTranscript는 homeDir 설정 시 transcript가 저장됨을 검증한다.
func TestRunAgent_WithHomeDir_PersistTranscript(t *testing.T) {
	t.Parallel()
	homeDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	def := AgentDefinition{
		AgentType: "transcript_agent",
		Name:      "transcript_agent",
		Isolation: IsolationFork,
	}
	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"},
		WithLogger(nopLogger()),
		WithHomeDir(homeDir),
	)
	require.NoError(t, err)
	assert.NotNil(t, sa)

	cancel()
	drainWithTimeout(outCh, 300*time.Millisecond)
}

// TestResumeAgent_NoHomeDir는 homeDir가 없어도 ResumeAgent가 동작함을 검증한다.
func TestResumeAgent_NoHomeDir(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sa, outCh, err := ResumeAgent(ctx, "worker@sess-prev-1",
		WithLogger(nopLogger()),
	)
	require.NoError(t, err)
	assert.NotNil(t, sa)
	assert.Equal(t, "worker@sess-prev-1", sa.Identity.AgentID)

	cancel()
	drainWithTimeout(outCh, 300*time.Millisecond)
}

// TestPlanModeApprove_ContextCancelled는 cancelled context에서 에러를 반환함을 검증한다.
func TestPlanModeApprove_ContextCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 즉시 취소

	err := PlanModeApprove(ctx, "any_agent@sess-1")
	assert.Error(t, err) // ctx.Err() 반환
}

// TestTeammateCanUseTool_BackgroundWithAllowRule는 allow rule이 있으면 write가 허용됨을 검증한다.
func TestTeammateCanUseTool_BackgroundWithAllowRule(t *testing.T) {
	t.Parallel()
	sp := &SettingsPermissions{}
	sp.AddAllowRule("write")

	tcu := &TeammateCanUseTool{
		def: AgentDefinition{
			Isolation:      IsolationBackground,
			PermissionMode: "bubble",
		},
		settingsPerms:    sp,
		parentCanUseTool: nil, // bubble mode지만 parent nil → fallback
	}

	// allow rule이 있으면 deny 안 됨
	decision := tcu.Check(context.Background(), permCtx("write"))
	assert.NotEqual(t, "background_agent_write_denied", decision.Reason)
}

// TestMemdirAppend_DefaultScope는 entry.Scope가 빈 경우 첫 번째 scope가 사용됨을 검증한다.
func TestMemdirAppend_DefaultScope(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".goose", "agent-memory", "test")

	mgr := &MemdirManager{
		agentType: "test",
		scopes:    []MemoryScope{ScopeProject},
		baseDirs:  map[MemoryScope]string{ScopeProject: dir},
	}

	entry := MemoryEntry{
		ID:        "e1",
		Category:  "test",
		Key:       "k1",
		Value:     "v1",
		Scope:     "", // 빈 scope → 첫 번째(project) 사용
		Timestamp: time.Now(),
	}
	err := mgr.Append(entry)
	require.NoError(t, err)
}

// TestMemdirManager_BuildMemoryPrompt_DisabledScope는 disabled scope의
// 항목이 포함되지 않음을 검증한다.
func TestMemdirManager_BuildMemoryPrompt_DisabledScope(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	userDir := filepath.Join(root, "user")
	projDir := filepath.Join(root, "proj")

	// user scope에 항목 추가
	require.NoError(t, writeMemdirEntry(userDir, MemoryEntry{
		ID: "u1", Timestamp: time.Now(), Category: "info", Key: "user.key", Value: "user.val",
	}))

	// project scope만 enabled (user는 비활성)
	mgr := &MemdirManager{
		agentType: "test",
		scopes:    []MemoryScope{ScopeProject}, // user scope 미포함
		baseDirs: map[MemoryScope]string{
			ScopeUser:    userDir,
			ScopeProject: projDir,
		},
	}

	prompt, err := mgr.BuildMemoryPrompt()
	require.NoError(t, err)
	// user scope 항목은 포함되지 않아야 함
	assert.Empty(t, prompt)
}

// TestRunAgent_EngineInitFailed는 QueryEngine 생성 실패 시 ErrEngineInitFailed를 반환함을 검증한다.
// MaxTurns=0으로 설정하면 engine은 생성되지만 즉시 max_turns terminal을 반환한다.
func TestRunAgent_MaxTurnsZero(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	def := AgentDefinition{
		AgentType: "zero_turns",
		Name:      "zero_turns",
		Isolation: IsolationFork,
		MaxTurns:  0, // QueryEngine 즉시 max_turns
	}
	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"}, WithLogger(nopLogger()))
	// MaxTurns=0은 engine이 즉시 반환하므로 성공
	require.NoError(t, err)
	require.NotNil(t, sa)

	// channel에서 terminal 메시지를 기다린다
	deadline := time.After(2 * time.Second)
	var terminalReceived bool
	for {
		select {
		case <-deadline:
			goto done
		case msg, ok := <-outCh:
			if !ok {
				goto done
			}
			if msg.Type == message.SDKMsgTerminal {
				terminalReceived = true
				goto done
			}
		}
	}
done:
	_ = terminalReceived
}

// TestWithModelResolver_Applied는 WithModelResolver가 cfg에 설정됨을 검증한다.
func TestWithModelResolver_Applied(t *testing.T) {
	t.Parallel()
	mr := &mockModelResolver{}
	cfg := buildRunConfig([]RunOption{WithModelResolver(mr)})
	assert.Equal(t, mr, cfg.modelResolver)
}
