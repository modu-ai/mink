package subagent

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/modu-ai/mink/internal/message"
	"github.com/modu-ai/mink/internal/permissions"
	"github.com/modu-ai/mink/internal/query"
	"go.uber.org/zap"
)

// RunOption은 RunAgent의 옵션 함수 타입이다.
type RunOption func(*runConfig)

// runConfig는 RunAgent의 내부 설정이다.
type runConfig struct {
	sessionID     string
	cwd           string
	homeDir       string
	projectRoot   string
	parentEngine  *query.QueryEngine
	hookDispatch  HookDispatcher
	settingsPerms *SettingsPermissions
	logger        *zap.Logger
	// parentCanUseTool은 bubble mode에서 사용하는 부모 권한 게이트이다.
	parentCanUseTool permissions.CanUseTool
	// modelResolver는 model alias를 해석한다 (optional).
	modelResolver ModelResolver
}

// HookDispatcher는 hook 이벤트 디스패치 인터페이스이다.
// HOOK-001 Dispatcher를 소비하는 adapter.
type HookDispatcher interface {
	DispatchSubagentStart(ctx context.Context, agentID string) error
	DispatchSubagentStop(ctx context.Context, agentID string, success bool) error
	DispatchWorktreeCreate(ctx context.Context, path string) error
	DispatchWorktreeRemove(ctx context.Context, path string) error
	DispatchTeammateIdle(ctx context.Context, agentID string) error
	DispatchSessionEnd(ctx context.Context) error
}

// ModelResolver는 model alias를 실제 model ID로 변환하는 인터페이스이다.
// ROUTER-001이 구현한다.
type ModelResolver interface {
	Resolve(alias string) (string, error)
}

// WithSessionID는 부모 세션 ID를 설정한다.
func WithSessionID(sessionID string) RunOption {
	return func(c *runConfig) { c.sessionID = sessionID }
}

// WithCwd는 현재 작업 디렉토리를 설정한다.
func WithCwd(cwd string) RunOption {
	return func(c *runConfig) { c.cwd = cwd }
}

// WithProjectRoot는 프로젝트 루트 디렉토리를 설정한다.
func WithProjectRoot(root string) RunOption {
	return func(c *runConfig) { c.projectRoot = root }
}

// WithHomeDir는 홈 디렉토리를 설정한다.
func WithHomeDir(homeDir string) RunOption {
	return func(c *runConfig) { c.homeDir = homeDir }
}

// WithHookDispatcher는 hook dispatcher를 설정한다.
func WithHookDispatcher(d HookDispatcher) RunOption {
	return func(c *runConfig) { c.hookDispatch = d }
}

// WithSettingsPermissions는 settings.json 기반 권한 설정을 주입한다.
func WithSettingsPermissions(p *SettingsPermissions) RunOption {
	return func(c *runConfig) { c.settingsPerms = p }
}

// WithParentCanUseTool는 bubble mode에서 사용할 부모 권한 게이트를 주입한다.
func WithParentCanUseTool(c permissions.CanUseTool) RunOption {
	return func(cfg *runConfig) { cfg.parentCanUseTool = c }
}

// WithModelResolver는 model alias resolver를 주입한다.
func WithModelResolver(r ModelResolver) RunOption {
	return func(c *runConfig) { c.modelResolver = r }
}

// WithLogger는 zap 로거를 설정한다.
func WithLogger(l *zap.Logger) RunOption {
	return func(c *runConfig) { c.logger = l }
}

// activeAgents는 현재 active한 Subagent의 전역 레지스트리이다.
// REQ-SA-014: cyclic spawn 방지용 spawnDepth, PlanModeApprove에서 조회.
var (
	activeAgentsMu sync.Mutex
	activeAgents   = map[string]*Subagent{}
)

func registerActiveAgent(s *Subagent) {
	activeAgentsMu.Lock()
	defer activeAgentsMu.Unlock()
	activeAgents[s.AgentID] = s
}

func deregisterActiveAgent(agentID string) {
	activeAgentsMu.Lock()
	defer activeAgentsMu.Unlock()
	delete(activeAgents, agentID)
}

// RunAgent는 AgentDefinition에 따라 sub-agent를 spawn한다.
// REQ-SA-001/005/006/007/014/022/023
//
// @MX:ANCHOR: [AUTO] 모든 sub-agent spawn의 단일 진입점
// @MX:REASON: SPEC-GOOSE-SUBAGENT-001 REQ-SA-005/006/007 — 3종 isolation 모드의 공통 진입점
func RunAgent(
	parentCtx context.Context,
	def AgentDefinition,
	input SubagentInput,
	opts ...RunOption,
) (*Subagent, <-chan message.SDKMessage, error) {
	cfg := buildRunConfig(opts)

	// REQ-SA-014: spawn depth 검증
	depth := spawnDepthFromContext(parentCtx)
	if depth >= MaxSpawnDepth {
		return nil, nil, fmt.Errorf("%w: depth=%d", ErrSpawnDepthExceeded, depth)
	}

	// Isolation 기본값 설정
	isolation := def.Isolation
	if isolation == "" {
		if def.Background {
			isolation = IsolationBackground
		} else {
			isolation = IsolationFork
		}
	}

	// AgentID 생성 (REQ-SA-001)
	sessionID := cfg.sessionID
	if sessionID == "" {
		sessionID = "default"
	}
	agentID := generateAgentID(def.AgentType, sessionID)

	// TeammateIdentity 구성 (REQ-SA-005b)
	identity := TeammateIdentity{
		AgentID:          agentID,
		AgentName:        def.AgentType,
		TeamName:         "",
		PlanModeRequired: def.PermissionMode == "plan",
		ParentSessionID:  sessionID,
	}

	// plan mode 레지스트리 등록 (REQ-SA-022)
	var planEntry *planModeEntry
	if identity.PlanModeRequired {
		planEntry = registerPlanMode(agentID)
	}

	// CoordinatorMode 처리 (REQ-SA-011)
	if def.CoordinatorMode {
		parentID, ok := TeammateIdentityFromContext(parentCtx)
		if ok && parentID.AgentID != "" {
			cfg.logger.Warn("nested coordinator mode is rarely correct",
				zap.String("parentAgent", parentID.AgentID),
				zap.String("childAgent", agentID),
			)
		}
	}

	// ctx에 TeammateIdentity와 spawnDepth 주입
	childCtx := WithTeammateIdentity(parentCtx, identity)
	childCtx = withSpawnDepth(childCtx)

	// QueryEngine 생성 (REQ-SA-005a)
	engine, err := buildQueryEngine(childCtx, def, identity, planEntry, cfg)
	if err != nil {
		if planEntry != nil {
			deregisterPlanMode(agentID)
		}
		return nil, nil, fmt.Errorf("%w: %v", ErrEngineInitFailed, err)
	}

	// SubagentStart hook 발동 (REQ-SA-005c)
	if cfg.hookDispatch != nil {
		if herr := cfg.hookDispatch.DispatchSubagentStart(parentCtx, agentID); herr != nil {
			if planEntry != nil {
				deregisterPlanMode(agentID)
			}
			return nil, nil, fmt.Errorf("%w: SubagentStart: %v", ErrHookDispatchFailed, herr)
		}
	}

	// ctx 취소 확인 (REQ-SA-005-F iii)
	if childCtx.Err() != nil {
		if cfg.hookDispatch != nil {
			_ = cfg.hookDispatch.DispatchSubagentStop(parentCtx, agentID, false)
		}
		if planEntry != nil {
			deregisterPlanMode(agentID)
		}
		return nil, nil, fmt.Errorf("%w: context cancelled before spawn", ErrSpawnAborted)
	}

	// Subagent 인스턴스 생성
	sa := &Subagent{
		AgentID:    agentID,
		Definition: def,
		Engine:     engine,
		Identity:   identity,
		StartedAt:  time.Now(),
	}

	// worktree 생성 (REQ-SA-006)
	var worktreePath string
	var worktreeCleanup func()
	if isolation == IsolationWorktree {
		cwd := cfg.cwd
		if cwd == "" {
			cwd, _ = os.Getwd()
		}
		var werr error
		worktreePath, worktreeCleanup, werr = createWorktree(parentCtx, agentID, cwd)
		if werr != nil {
			cfg.logger.Warn("worktree creation failed, falling back to fork isolation",
				zap.Error(werr),
			)
			isolation = IsolationFork
		} else {
			if cfg.hookDispatch != nil {
				_ = cfg.hookDispatch.DispatchWorktreeCreate(parentCtx, worktreePath)
			}
		}
	}

	registerActiveAgent(sa)
	sa.setState(StateRunning)

	// 출력 채널 생성
	outCh := make(chan message.SDKMessage, 64)

	// goroutine spawn (REQ-SA-005d, REQ-SA-023)
	go func() {
		defer func() {
			now := time.Now()
			sa.FinishedAt = &now
			deregisterActiveAgent(agentID)
			if planEntry != nil {
				deregisterPlanMode(agentID)
			}

			// worktree cleanup (REQ-SA-006)
			if worktreeCleanup != nil {
				worktreeCleanup()
				if cfg.hookDispatch != nil {
					_ = cfg.hookDispatch.DispatchWorktreeRemove(context.Background(), worktreePath)
				}
			}
		}()

		// REQ-SA-023: ctx.Done() 선택
		done := make(chan struct{})
		var engineCh <-chan message.SDKMessage
		var engineErr error

		func() {
			engineCh, engineErr = engine.SubmitMessage(childCtx, input.Prompt)
		}()

		if engineErr != nil {
			sa.setState(StateFailed)
			if cfg.hookDispatch != nil {
				_ = cfg.hookDispatch.DispatchSubagentStop(context.Background(), agentID, false)
			}
			close(outCh)
			return
		}

		close(done)
		_ = done

		var idleTimer *time.Timer
		if isolation == IsolationBackground {
			idleTimer = time.AfterFunc(DefaultBackgroundIdleThreshold, func() {
				if cfg.hookDispatch != nil {
					_ = cfg.hookDispatch.DispatchTeammateIdle(context.Background(), agentID)
				}
				sa.setState(StateIdle)
			})
		}

		terminal := false
		success := false

		for {
			select {
			case <-childCtx.Done():
				// REQ-SA-023: ctx cancel 시 goroutine 종료
				if idleTimer != nil {
					idleTimer.Stop()
				}
				sa.setState(StateFailed)
				if cfg.hookDispatch != nil {
					_ = cfg.hookDispatch.DispatchSubagentStop(context.Background(), agentID, false)
				}
				close(outCh)
				return
			case msg, ok := <-engineCh:
				if !ok {
					// 채널 종료
					if !terminal {
						sa.setState(StateFailed)
						if cfg.hookDispatch != nil {
							_ = cfg.hookDispatch.DispatchSubagentStop(context.Background(), agentID, false)
						}
					}
					if idleTimer != nil {
						idleTimer.Stop()
					}
					close(outCh)
					return
				}
				// REQ-SA-008: terminal 메시지 처리
				if msg.Type == message.SDKMsgTerminal {
					if t, ok := msg.Payload.(*message.PayloadTerminal); ok {
						success = t != nil && t.Success
					}
					// REQ-SA-008(d)→(c) 순서: state 먼저 설정 후 채널 전달
					if success {
						sa.setState(StateCompleted)
					} else {
						sa.setState(StateFailed)
					}
					terminal = true

					// (b) transcript 저장
					persistTranscript(sa, cfg)

					// (b) SubagentStop hook
					if cfg.hookDispatch != nil {
						_ = cfg.hookDispatch.DispatchSubagentStop(context.Background(), agentID, success)
					}

					// idle timer 리셋
					if idleTimer != nil {
						idleTimer.Reset(DefaultBackgroundIdleThreshold)
					}
				}

				// idle timer 리셋
				if isolation == IsolationBackground && idleTimer != nil {
					idleTimer.Reset(DefaultBackgroundIdleThreshold)
				}

				select {
				case outCh <- msg:
				case <-childCtx.Done():
					if idleTimer != nil {
						idleTimer.Stop()
					}
					sa.setState(StateFailed)
					if cfg.hookDispatch != nil {
						_ = cfg.hookDispatch.DispatchSubagentStop(context.Background(), agentID, false)
					}
					close(outCh)
					return
				}
			}
		}
	}()

	return sa, outCh, nil
}

// buildRunConfig는 RunOption들을 적용하여 runConfig를 구성한다.
func buildRunConfig(opts []RunOption) *runConfig {
	cfg := &runConfig{
		logger: zap.NewNop(),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.logger == nil {
		cfg.logger = zap.NewNop()
	}
	return cfg
}

// buildQueryEngine은 AgentDefinition으로 QueryEngine을 생성한다.
// REQ-SA-005(a): 부모 설정 override.
func buildQueryEngine(
	ctx context.Context,
	def AgentDefinition,
	identity TeammateIdentity,
	planEntry *planModeEntry,
	cfg *runConfig,
) (*query.QueryEngine, error) {
	// permission gate 구성
	perm := &TeammateCanUseTool{
		def:              def,
		parentCanUseTool: cfg.parentCanUseTool,
		settingsPerms:    cfg.settingsPerms,
		planEntry:        planEntry,
	}

	// tool 목록 구성 (REQ-SA-013)
	tools := buildToolList(def)

	// memory.append tool 등록 (REQ-SA-021)
	if len(def.MemoryScopes) > 0 {
		tools = append(tools, query.ToolDefinition{
			Name:        "memory.append",
			Description: "Append a memory entry to the agent's memory store",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"scope":    map[string]any{"type": "string"},
					"category": map[string]any{"type": "string"},
					"key":      map[string]any{"type": "string"},
					"value":    map[string]any{"type": "string"},
				},
				"required": []string{"scope", "category", "key", "value"},
			},
		})
	}

	// MCP servers 처리 (REQ-SA-020): stub이면 skip

	qCfg := query.QueryEngineConfig{
		LLMCall:         stubLLMCall,
		Tools:           tools,
		CanUseTool:      perm,
		Executor:        stubExecutor{},
		Logger:          cfg.logger,
		MaxTurns:        def.MaxTurns,
		CoordinatorMode: def.CoordinatorMode,
		TeammateIdentity: &query.TeammateIdentity{
			AgentID:  identity.AgentID,
			TeamName: identity.TeamName,
		},
		TaskBudget: query.TaskBudget{
			Total:     10000,
			Remaining: 10000,
		},
	}

	engine, err := query.New(qCfg)
	if err != nil {
		return nil, err
	}
	return engine, nil
}

// buildToolList는 AgentDefinition.Tools에 따라 tool 목록을 구성한다.
// REQ-SA-013: ["*"]이면 부모 상속; 명시 목록이면 baseline 포함.
func buildToolList(def AgentDefinition) []query.ToolDefinition {
	if len(def.Tools) == 0 || (len(def.Tools) == 1 && def.Tools[0] == "*") {
		// 부모 상속 — 실제 구현에서는 부모 tools를 복사. 여기선 빈 슬라이스로 처리.
		return nil
	}
	// baseline tools: read, task-update
	baselineTools := map[string]bool{"read": true, "task-update": true}
	toolNames := make(map[string]bool)
	for _, t := range def.Tools {
		toolNames[t] = true
	}
	for t := range baselineTools {
		toolNames[t] = true
	}

	var result []query.ToolDefinition
	for name := range toolNames {
		result = append(result, query.ToolDefinition{
			Name:        name,
			Description: name,
		})
	}
	return result
}

// persistTranscript는 sub-agent의 transcript를 디스크에 저장한다.
// REQ-SA-002: 완료 여부와 관계없이 저장.
func persistTranscript(sa *Subagent, cfg *runConfig) {
	// 실제 구현에서는 message들을 jsonl로 저장.
	// 이 stub은 디렉토리만 생성한다.
	if cfg.homeDir == "" {
		return
	}
	dir := transcriptDir(sa.AgentID, sa.Definition.AgentType, cfg.homeDir)
	_ = os.MkdirAll(dir, 0o700)
}

// transcriptDir는 transcript 디렉토리 경로를 반환한다.
// REQ-SA-002: {homeDir}/agent-memory/{agentType}/transcript-{agentId}/
// REQ-MINK-UDM-002: homeDir은 userpath.UserHomeE() 결과 (.mink 경로).
func transcriptDir(agentID, agentType, homeDir string) string {
	return fmt.Sprintf("%s/agent-memory/%s/transcript-%s",
		homeDir, agentType, agentID)
}

// stubLLMCall은 테스트용 LLM 호출 스텁이다.
var stubLLMCall query.LLMCallFunc = func(ctx context.Context, req query.LLMCallReq) (<-chan message.StreamEvent, error) {
	ch := make(chan message.StreamEvent)
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
		}
	}()
	return ch, nil
}

// stubExecutor는 테스트용 Executor 스텁이다.
type stubExecutor struct{}

func (stubExecutor) Run(_ context.Context, _, toolName string, _ map[string]any) (string, error) {
	return fmt.Sprintf("stub result for %s", toolName), nil
}
