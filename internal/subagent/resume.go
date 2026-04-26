package subagent

import (
	"context"
	"fmt"

	"github.com/modu-ai/goose/internal/message"
)

// resumePrompt는 ResumeAgent가 전달하는 재개 큐 프롬프트이다.
// REQ-SA-009(d)
const resumePrompt = "[[RESUME]]"

// ResumeAgent는 이전 세션에서 중단된 sub-agent를 재개한다.
// REQ-SA-009 / AC-SA-006
//
// @MX:ANCHOR: [AUTO] 중단된 sub-agent 재개의 단일 진입점
// @MX:REASON: SPEC-GOOSE-SUBAGENT-001 REQ-SA-009 — transcript 기반 복원 + [[RESUME]] 큐
func ResumeAgent(
	parentCtx context.Context,
	agentID string,
	opts ...RunOption,
) (*Subagent, <-chan message.SDKMessage, error) {
	cfg := buildRunConfig(opts)

	// REQ-SA-009(a): agentID에서 agentName 추출
	agentName, originalSessionID, _, err := parseAgentID(agentID)
	if err != nil {
		return nil, nil, fmt.Errorf("resumeAgent: invalid agentID %q: %w", agentID, err)
	}

	// REQ-SA-009(b): AgentDefinition 재구성
	// 실제 구현에서는 metadata.json에서 복원하지만, 여기서는 최소한으로 구성.
	def := AgentDefinition{
		AgentType: agentName,
		Name:      agentName,
		Isolation: IsolationFork,
	}

	// REQ-SA-009(c): 원래 TeammateIdentity 복원
	restoredIdentity := TeammateIdentity{
		AgentID:         agentID,
		AgentName:       agentName,
		ParentSessionID: originalSessionID,
	}

	// TEammateIdentity를 ctx에 주입
	resumeCtx := WithTeammateIdentity(parentCtx, restoredIdentity)

	// transcript 로드 시뮬레이션 (REQ-SA-009a)
	if cfg.homeDir != "" {
		tDir := transcriptDir(agentID, agentName, cfg.homeDir)
		// 실제 transcript 복원 (stub)
		_ = tDir
	}

	// REQ-SA-009(d): input.Prompt = "[[RESUME]]"
	input := SubagentInput{
		Prompt: resumePrompt,
	}

	// RunAgent를 통해 실제 재개
	sa, ch, err := RunAgent(resumeCtx, def, input, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("resumeAgent: RunAgent failed: %w", err)
	}

	// 원래 identity 복원
	sa.Identity = restoredIdentity
	return sa, ch, nil
}
