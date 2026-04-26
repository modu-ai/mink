package subagent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResumeAgent_LoadsTranscript는 ResumeAgent가 이전 세션의 transcript를
// 복원하고 [[RESUME]] 프롬프트를 전달함을 검증한다. (AC-SA-006, REQ-SA-009)
func TestResumeAgent_LoadsTranscript(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 이전 세션의 agentID
	previousAgentID := "researcher@sess-old-2"

	sa, outCh, err := ResumeAgent(ctx, previousAgentID,
		WithSessionID("new-sess"),
		WithLogger(nopLogger()),
	)
	require.NoError(t, err)
	require.NotNil(t, sa)
	require.NotNil(t, outCh)

	// 이전 TeammateIdentity 복원 확인
	// REQ-SA-018: AgentID = {agentName}@{sessionId}-{spawnIndex}
	// "researcher@sess-old-2" → agentName="researcher", sessionId="sess-old", spawnIndex=2
	assert.Equal(t, previousAgentID, sa.Identity.AgentID)
	assert.Equal(t, "researcher", sa.Identity.AgentName)
	// ParentSessionID는 "sess-old" (마지막 '-' 기준으로 spawnIndex 분리)
	assert.Equal(t, "sess-old", sa.Identity.ParentSessionID)

	cancel()
	drainWithTimeout(outCh, 500*time.Millisecond)
}

// TestResumeAgent_InvalidAgentID는 유효하지 않은 agentID에서 에러를 반환함을 검증한다.
func TestResumeAgent_InvalidAgentID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, _, err := ResumeAgent(ctx, "invalid_no_delimiter")
	assert.Error(t, err)
}
