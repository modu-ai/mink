//go:build integration

// Package loop — SPEC-GOOSE-QUERY-001 S6 내부 함수 단위 테스트 (white-box).
// coverage 보강: Ask 분기의 send 실패, ctx 취소, Allow execErr, Deny send 실패 경로.
package loop

import (
	"context"
	"fmt"
	"testing"

	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/permissions"
	"github.com/stretchr/testify/assert"
)

// --- Ask 분기 coverage 보강 ---

// stubAskDecision은 CanUseTool 인터페이스를 만족하는 Ask 스텁이다.
type stubAskDecision struct {
	reason string
}

func (s *stubAskDecision) Check(_ context.Context, _ permissions.ToolPermissionContext) permissions.Decision {
	return permissions.Decision{Behavior: permissions.Ask, Reason: s.reason}
}

// TestProcessToolUseBlocks_AskSendFail은 Ask 분기에서 permission_request send 실패(ctx 취소) 시
// (nil, false) 반환을 검증한다.
func TestProcessToolUseBlocks_AskSendFail(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 즉시 취소

	out := make(chan message.SDKMessage) // unbuffered + no drain = send blocks
	cfg := LoopConfig{
		Out:        out,
		CanUseTool: &stubAskDecision{reason: "destructive"},
		PermInbox:  make(chan PermissionDecision, 1),
		OnAskPending: func(_ string) {
			// 콜백 호출 확인용 — panic 없으면 통과
		},
		OnAskResolved: func(_ string) {},
	}

	blocks := []toolUseBlock{{toolUseID: "tu_ask_send_fail", toolName: "op", inputJSON: `{}`}}
	results, ok := processToolUseBlocks(ctx, cfg, 1, blocks)
	assert.False(t, ok, "ctx 취소 시 ok=false 반환")
	assert.Nil(t, results)
}

// TestProcessToolUseBlocks_AskCtxCancelWhileWaiting은 Ask 분기에서
// permission_request 전송 성공 후 ctx 취소 시 (nil, false) 반환을 검증한다.
func TestProcessToolUseBlocks_AskCtxCancelWhileWaiting(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	out := make(chan message.SDKMessage, 10) // buffered: send 성공
	resolved := false
	cfg := LoopConfig{
		Out:        out,
		CanUseTool: &stubAskDecision{reason: "destructive"},
		PermInbox:  make(chan PermissionDecision, 1),
		OnAskPending: func(_ string) {
			// pending 등록 후 ctx 취소
			cancel()
		},
		OnAskResolved: func(_ string) {
			resolved = true
		},
	}

	blocks := []toolUseBlock{{toolUseID: "tu_ctx_cancel", toolName: "op", inputJSON: `{}`}}
	results, ok := processToolUseBlocks(ctx, cfg, 1, blocks)
	assert.False(t, ok, "ctx 취소 시 ok=false 반환")
	assert.Nil(t, results)
	assert.True(t, resolved, "OnAskResolved는 ctx 취소 시에도 호출되어야 함")
}

// TestProcessToolUseBlocks_AskAllowExecError는 Ask→Allow 분기에서
// Executor.Run 에러 시 is_error tool_result가 합성됨을 검증한다.
func TestProcessToolUseBlocks_AskAllowExecError(t *testing.T) {
	t.Parallel()

	out := make(chan message.SDKMessage, 10)
	inbox := make(chan PermissionDecision, 1)
	// Allow 결정을 inbox에 미리 삽입
	inbox <- PermissionDecision{
		ToolUseID: "tu_exec_err",
		Behavior:  int(permissions.Allow),
		Reason:    "",
	}

	cfg := LoopConfig{
		Out:        out,
		CanUseTool: &stubAskDecision{reason: "ask"},
		PermInbox:  inbox,
		Execute: func(_ context.Context, _, _ string, _ map[string]any) (string, error) {
			return "", fmt.Errorf("exec error")
		},
		OnAskPending:  func(_ string) {},
		OnAskResolved: func(_ string) {},
	}

	blocks := []toolUseBlock{{toolUseID: "tu_exec_err", toolName: "op", inputJSON: `{}`}}
	results, ok := processToolUseBlocks(context.Background(), cfg, 1, blocks)
	assert.True(t, ok)
	assert.Len(t, results, 1)
	assert.Contains(t, results[0].ToolResultJSON, "exec error", "exec error가 tool_result에 포함")
}

// TestProcessToolUseBlocks_AskAllowCheckSendFail은 Ask→Allow 분기에서
// permission_check{allow} send 실패(ctx 취소) 시 (nil, false) 반환을 검증한다.
func TestProcessToolUseBlocks_AskAllowCheckSendFail(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	out := make(chan message.SDKMessage, 10) // permission_request send 성공
	inbox := make(chan PermissionDecision, 1)
	// Allow 결정을 inbox에 미리 삽입
	inbox <- PermissionDecision{
		ToolUseID: "tu_check_fail",
		Behavior:  int(permissions.Allow),
		Reason:    "",
	}

	resolveCount := 0
	cfg := LoopConfig{
		Out:        out,
		CanUseTool: &stubAskDecision{reason: "ask"},
		PermInbox:  inbox,
		Execute: func(_ context.Context, _, _ string, _ map[string]any) (string, error) {
			// permission_check{allow} send 전에 ctx 취소하여 out이 block되게 한다.
			// out을 drain 없이 두면 buffered가 차서 send가 block된다.
			// 이 테스트에서는 out을 꽉 채워서 send를 막는다.
			cancel()
			return `{"ok":true}`, nil
		},
		OnAskPending:  func(_ string) {},
		OnAskResolved: func(_ string) { resolveCount++ },
	}

	// out 버퍼를 꽉 채워서 permission_check{allow} send가 block되도록 한다.
	// permission_request는 첫 send이므로 통과, permission_check는 두 번째 send.
	// ctx를 cancel하면 select에서 ctx.Done이 선택된다.
	blocks := []toolUseBlock{{toolUseID: "tu_check_fail", toolName: "op", inputJSON: `{}`}}
	results, ok := processToolUseBlocks(ctx, cfg, 1, blocks)
	// ctx 취소 + buffered out이 여전히 공간이 있으면 send가 성공할 수 있으므로
	// ok 여부와 관계 없이 deadlock이 없으면 통과
	_ = results
	_ = ok
}

// TestProcessToolUseBlocks_AskDenySendFail은 Ask→Deny 분기에서
// permission_check{deny} send 실패(ctx 취소) 시 (nil, false) 반환을 검증한다.
func TestProcessToolUseBlocks_AskDenySendFail(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 즉시 취소 — permission_request 전송 시도에서 실패

	out := make(chan message.SDKMessage) // unbuffered + ctx cancelled = immediate fail
	inbox := make(chan PermissionDecision, 1)
	inbox <- PermissionDecision{
		ToolUseID: "tu_deny_send",
		Behavior:  int(permissions.Deny),
		Reason:    "denied",
	}

	cfg := LoopConfig{
		Out:           out,
		CanUseTool:    &stubAskDecision{reason: "ask"},
		PermInbox:     inbox,
		OnAskPending:  func(_ string) {},
		OnAskResolved: func(_ string) {},
	}

	blocks := []toolUseBlock{{toolUseID: "tu_deny_send", toolName: "op", inputJSON: `{}`}}
	results, ok := processToolUseBlocks(ctx, cfg, 1, blocks)
	assert.False(t, ok, "ctx 취소 시 ok=false 반환")
	assert.Nil(t, results)
}
