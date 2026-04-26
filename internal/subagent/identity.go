package subagent

import (
	"fmt"
	"sync/atomic"
)

// sessionSpawnCounter는 부모 세션 내에서 spawn된 agent 수를 atomic으로 추적한다.
// REQ-SA-001: 동시 RunAgent 호출에서 단조 증가하는 non-overlapping index 보장.
//
// @MX:WARN: [AUTO] 전역 atomic counter — 단일 프로세스에서 유일성 보장
// @MX:REASON: REQ-SA-001 — 부모 세션 내 AgentID collision 방지. atomic.AddInt64으로 동시성 안전
var sessionSpawnCounter int64

// generateAgentID는 REQ-SA-001에 따라 AgentID를 생성한다.
// 형식: {agentName}@{sessionId}-{spawnIndex}
// '@'는 agentName과 sessionId-spawnIndex 사이의 예약 구분자이다.
func generateAgentID(agentName, sessionID string) string {
	idx := atomic.AddInt64(&sessionSpawnCounter, 1)
	return fmt.Sprintf("%s%s%s-%d", agentName, agentIDDelimiter, sessionID, idx)
}

// parseAgentID는 AgentID를 agentName, sessionID, spawnIndex로 분리한다.
// REQ-SA-018: '@'로 먼저 분리, 그다음 마지막 '-'로 spawnIndex 분리.
func parseAgentID(id string) (agentName, sessionID string, spawnIndex int64, err error) {
	// agentName@sessionId-spawnIndex
	atIdx := -1
	for i, c := range id {
		if c == '@' {
			atIdx = i
			break
		}
	}
	if atIdx < 0 {
		return "", "", 0, fmt.Errorf("invalid agentID: missing '@' delimiter: %q", id)
	}
	agentName = id[:atIdx]
	rest := id[atIdx+1:]

	// 마지막 '-'로 분리
	lastDash := -1
	for i := len(rest) - 1; i >= 0; i-- {
		if rest[i] == '-' {
			lastDash = i
			break
		}
	}
	if lastDash < 0 {
		return "", "", 0, fmt.Errorf("invalid agentID: missing '-' in session-index: %q", id)
	}
	sessionID = rest[:lastDash]
	_, err = fmt.Sscanf(rest[lastDash+1:], "%d", &spawnIndex)
	return agentName, sessionID, spawnIndex, err
}
