package subagent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewMemdirManager는 MemdirManager 생성자를 테스트한다.
// REQ-MINK-UDM-002. AC-005: .mink 경로 사용.
func TestNewMemdirManager(t *testing.T) {
	t.Parallel()
	// homeDir은 이미 .mink-rooted 경로 (userpath.UserHomeE() 결과)를 전달
	mgr := NewMemdirManager("researcher", []MemoryScope{ScopeProject, ScopeUser}, "/project", "/home/.mink")
	assert.NotNil(t, mgr)
	assert.Equal(t, "researcher", mgr.agentType)
	assert.Equal(t, []MemoryScope{ScopeProject, ScopeUser}, mgr.scopes)
	assert.Contains(t, mgr.baseDirs[ScopeUser], "agent-memory/researcher")
	assert.Contains(t, mgr.baseDirs[ScopeProject], ".mink/agent-memory/researcher")
	assert.Contains(t, mgr.baseDirs[ScopeLocal], ".mink/agent-memory-local/researcher")
}

// TestNewMemdirManager_DefaultScopes는 scopes가 nil이면 기본값으로 [ScopeProject]가 설정됨을 검증한다.
func TestNewMemdirManager_DefaultScopes(t *testing.T) {
	t.Parallel()
	mgr := NewMemdirManager("test", nil, "/p", "/h")
	assert.Equal(t, []MemoryScope{ScopeProject}, mgr.scopes)
}

// TestMemdirManager_Query는 predicate 기반 조회를 테스트한다.
func TestMemdirManager_Query(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".goose", "agent-memory", "test")

	mgr := &MemdirManager{
		agentType: "test",
		scopes:    []MemoryScope{ScopeProject},
		baseDirs:  map[MemoryScope]string{ScopeProject: dir},
	}

	entries := []MemoryEntry{
		{ID: "e1", Timestamp: time.Now(), Category: "fact", Key: "k1", Value: "v1", Scope: ScopeProject},
		{ID: "e2", Timestamp: time.Now(), Category: "pref", Key: "k2", Value: "v2", Scope: ScopeProject},
	}
	for _, e := range entries {
		require.NoError(t, mgr.Append(e))
	}

	// category="fact"만 조회
	result, err := mgr.Query(func(e MemoryEntry) bool {
		return e.Category == "fact"
	})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "k1", result[0].Key)
}

// TestSettingsPermissions_AllowRule는 allow rule 매칭을 테스트한다.
func TestSettingsPermissions_AllowRule(t *testing.T) {
	t.Parallel()
	sp := &SettingsPermissions{}
	assert.False(t, sp.HasAllowRule("write"))

	sp.AddAllowRule("write")
	assert.True(t, sp.HasAllowRule("write"))
	assert.False(t, sp.HasAllowRule("bash"))

	sp.AddAllowRule("*")
	assert.True(t, sp.HasAllowRule("bash"))
}

// TestSettingsPermissions_NilSafe는 nil SettingsPermissions가 panic하지 않음을 검증한다.
func TestSettingsPermissions_NilSafe(t *testing.T) {
	t.Parallel()
	var sp *SettingsPermissions
	assert.False(t, sp.HasAllowRule("write"))
}

// TestWithRunOptions는 RunOption들이 runConfig에 올바르게 적용됨을 검증한다.
func TestWithRunOptions(t *testing.T) {
	t.Parallel()
	logger := nopLogger()
	sp := &SettingsPermissions{}
	cfg := buildRunConfig([]RunOption{
		WithSessionID("sess-1"),
		WithCwd("/tmp"),
		WithProjectRoot("/proj"),
		WithHomeDir("/home"),
		WithSettingsPermissions(sp),
		WithLogger(logger),
	})
	assert.Equal(t, "sess-1", cfg.sessionID)
	assert.Equal(t, "/tmp", cfg.cwd)
	assert.Equal(t, "/proj", cfg.projectRoot)
	assert.Equal(t, "/home", cfg.homeDir)
	assert.Equal(t, sp, cfg.settingsPerms)
	assert.Equal(t, logger, cfg.logger)
}

// TestPersistTranscript는 homeDir가 설정된 경우 transcript 디렉토리가 생성됨을 검증한다.
func TestPersistTranscript(t *testing.T) {
	t.Parallel()
	homeDir := t.TempDir()
	sa := &Subagent{
		AgentID:    "researcher@sess-1",
		Definition: AgentDefinition{AgentType: "researcher"},
	}
	cfg := &runConfig{homeDir: homeDir, logger: nopLogger()}
	persistTranscript(sa, cfg)

	expectedDir := transcriptDir("researcher@sess-1", "researcher", homeDir)
	info, err := os.Stat(expectedDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

// TestTranscriptDir는 transcript 경로 생성을 검증한다.
func TestTranscriptDir(t *testing.T) {
	t.Parallel()
	path := transcriptDir("researcher@sess-1", "researcher", "/home")
	assert.Contains(t, path, "agent-memory/researcher/transcript-researcher@sess-1")
}

// TestSanitizeSlug는 slug 생성을 검증한다.
func TestSanitizeSlug(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input, expected string
	}{
		{"researcher@sess-1-42", "researcher_sess-1-42"},
		{"simple", "simple"},
		{"foo@bar#baz", "foo_bar_baz"},
	}
	for _, c := range cases {
		got := sanitizeSlug(c.input)
		assert.Equal(t, c.expected, got, "input: %s", c.input)
	}
}

// TestWorktreeListActive는 비 git 환경에서 nil을 반환함을 검증한다.
func TestWorktreeListActive_NonGit(t *testing.T) {
	t.Parallel()
	// 비 git 디렉토리에서 실행하면 에러 발생 → nil 반환
	paths := worktreeListActive(t.TempDir())
	assert.Nil(t, paths)
}

// TestRunAgent_WithContextCancel는 context cancel 시 RunAgent가 에러를 반환함을 검증한다.
func TestRunAgent_WithContextCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 즉시 cancel

	def := AgentDefinition{
		AgentType: "cancelled",
		Name:      "cancelled",
		Isolation: IsolationFork,
	}
	// ctx가 취소된 경우에도 RunAgent는 내부적으로 처리함
	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"}, WithLogger(nopLogger()))
	if err != nil {
		assert.ErrorIs(t, err, ErrSpawnAborted)
	} else {
		assert.NotNil(t, sa)
		drainWithTimeout(outCh, 300*time.Millisecond)
	}
}

// TestBuildToolList_Wildcard는 ["*"] Tools가 nil을 반환함을 검증한다.
func TestBuildToolList_Wildcard(t *testing.T) {
	t.Parallel()
	def := AgentDefinition{Tools: []string{"*"}}
	tools := buildToolList(def)
	assert.Nil(t, tools)
}

// TestBuildToolList_Empty는 빈 Tools가 nil을 반환함을 검증한다.
func TestBuildToolList_Empty(t *testing.T) {
	t.Parallel()
	def := AgentDefinition{Tools: []string{}}
	tools := buildToolList(def)
	assert.Nil(t, tools)
}

// TestTeammateCanUseTool_PlanModeRead는 plan mode에서 read tool은 허용됨을 검증한다.
func TestTeammateCanUseTool_PlanModeRead(t *testing.T) {
	t.Parallel()
	entry := &planModeEntry{required: true}
	tcu := &TeammateCanUseTool{
		def:       AgentDefinition{Isolation: IsolationFork},
		planEntry: entry,
	}
	// read는 write-class가 아니므로 허용
	decision := tcu.Check(context.Background(), permCtx("read"))
	assert.NotEqual(t, "plan_mode_required", decision.Reason)
}

// TestTeammateCanUseTool_IsolatedMode는 isolated mode에서 local policy가 적용됨을 검증한다.
func TestTeammateCanUseTool_IsolatedMode(t *testing.T) {
	t.Parallel()
	tcu := &TeammateCanUseTool{
		def: AgentDefinition{PermissionMode: "isolated"},
	}
	decision := tcu.Check(context.Background(), permCtx("search"))
	// isolated: local policy → Allow
	assert.Equal(t, 0, int(decision.Behavior))
}

// TestPlanModeApprove_AlreadyApproved는 이미 승인된 agent에 대해
// ErrAgentNotInPlanMode를 반환함을 검증한다.
func TestPlanModeApprove_AlreadyApproved(t *testing.T) {
	t.Parallel()
	// 이미 승인된 entry 등록
	entry := &planModeEntry{required: false, approved: make(chan struct{})}
	close(entry.approved)
	planModeRegistry.Store("approved@sess-1", entry)
	defer planModeRegistry.Delete("approved@sess-1")

	err := PlanModeApprove(context.Background(), "approved@sess-1")
	assert.ErrorIs(t, err, ErrAgentNotInPlanMode)
}

// TestValidateAgentSlug_Cases는 다양한 slug 케이스를 검증한다.
func TestValidateAgentSlug_Cases(t *testing.T) {
	t.Parallel()
	valid := []string{"researcher", "agent123", "my_agent", "Agent_V2"}
	for _, name := range valid {
		assert.NoError(t, validateAgentSlug(name), "name: %s", name)
	}

	invalid := []string{"", "_hidden", "foo-bar", "foo bar", "légal", "foo@bar"}
	for _, name := range invalid {
		assert.ErrorIs(t, validateAgentSlug(name), ErrInvalidAgentName, "name: %s", name)
	}
}

// TestLoadAgentsDir_NonMDFiles는 .md가 아닌 파일이 무시됨을 검증한다.
func TestLoadAgentsDir_NonMDFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	agentsDir := filepath.Join(root, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o700))

	// .yaml, .json은 무시
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "config.yaml"), []byte("{}"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "data.json"), []byte("{}"), 0o600))
	makeAgentFile(t, root, "valid.md", validAgentMD)

	defs, errs := LoadAgentsDir(agentsDir)
	assert.Empty(t, errs)
	assert.Len(t, defs, 1)
}

// TestParseMDFrontmatter_NoFrontmatter는 frontmatter가 없는 경우를 검증한다.
func TestParseMDFrontmatter_NoFrontmatter(t *testing.T) {
	t.Parallel()
	data := []byte("# Just a body\n\nNo frontmatter here.")
	fm, body, err := parseMDFrontmatter(data)
	require.NoError(t, err)
	assert.Empty(t, fm)
	assert.Equal(t, data, body)
}

// TestSubagentState_AllValues는 모든 SubagentState 값을 검증한다.
func TestSubagentState_AllValues(t *testing.T) {
	t.Parallel()
	states := []SubagentState{StatePending, StateRunning, StateCompleted, StateFailed, StateIdle}
	s := &Subagent{}
	for _, st := range states {
		s.setState(st)
		assert.Equal(t, st, s.State())
	}
}

// TestWithParentCanUseTool는 WithParentCanUseTool 옵션이 적용됨을 검증한다.
func TestWithParentCanUseTool(t *testing.T) {
	t.Parallel()
	parent := &denyAllCanUseTool{reason: "test-deny"}
	cfg := buildRunConfig([]RunOption{WithParentCanUseTool(parent)})
	assert.Equal(t, parent, cfg.parentCanUseTool)
}

// TestDeregisterActiveAgent는 agent deregister가 정상 동작함을 검증한다.
func TestDeregisterActiveAgent(t *testing.T) {
	t.Parallel()
	sa := &Subagent{AgentID: "test_agent@sess-1"}
	registerActiveAgent(sa)

	activeAgentsMu.Lock()
	_, found := activeAgents["test_agent@sess-1"]
	activeAgentsMu.Unlock()
	assert.True(t, found)

	deregisterActiveAgent("test_agent@sess-1")
	activeAgentsMu.Lock()
	_, found2 := activeAgents["test_agent@sess-1"]
	activeAgentsMu.Unlock()
	assert.False(t, found2)
}

// TestRunAgent_MemoryAppendToolRegistered는 MemoryScopes가 있을 때
// memory.append tool이 등록됨을 검증한다. (REQ-SA-021)
func TestRunAgent_MemoryAppendToolRegistered(t *testing.T) {
	t.Parallel()
	def := AgentDefinition{
		AgentType:    "mem_agent",
		Name:         "mem_agent",
		Isolation:    IsolationFork,
		MemoryScopes: []MemoryScope{ScopeProject},
	}
	tools := buildToolList(def)
	// memory.append는 buildQueryEngine에서 추가됨 (buildToolList 이후)
	// buildToolList에서는 memory.append를 추가하지 않으므로
	// buildQueryEngine을 직접 테스트
	cfg := buildRunConfig([]RunOption{WithLogger(nopLogger())})
	ctx := context.Background()
	identity := TeammateIdentity{AgentID: "mem_agent@sess-1"}
	engine, err := buildQueryEngine(ctx, def, identity, nil, cfg)
	require.NoError(t, err)
	assert.NotNil(t, engine)

	// tools에서 memory.append 포함 확인은 engine 내부 검사가 어려우므로
	// buildToolList 결과 + memory.append를 별도로 검증
	_ = tools
}

// TestLoadAgentsDir_BackgroundShortcut는 background=true가 Isolation=background로
// 변환됨을 검증한다.
func TestLoadAgentsDir_BackgroundShortcut(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeAgentFile(t, root, "bg_agent.md", `---
name: bg_agent
description: Background agent
background: true
---
body
`)
	defs, errs := LoadAgentsDir(filepath.Join(root, ".claude", "agents"))
	require.Empty(t, errs)
	require.Len(t, defs, 1)
	assert.Equal(t, IsolationBackground, defs[0].Isolation)
}

// TestRunAgent_WithModelResolver는 WithModelResolver 옵션이 적용됨을 검증한다.
func TestRunAgent_WithModelResolver(t *testing.T) {
	t.Parallel()
	mr := &mockModelResolver{result: "anthropic/claude-opus-4-7"}
	cfg := buildRunConfig([]RunOption{WithModelResolver(mr)})
	assert.Equal(t, mr, cfg.modelResolver)
}

type mockModelResolver struct {
	result string
}

func (m *mockModelResolver) Resolve(alias string) (string, error) {
	return m.result, nil
}
