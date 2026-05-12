package plugin

import (
	"context"

	"github.com/modu-ai/mink/internal/hook"
)

// pluginHookHandler는 manifest 선언 hook 명령어를 HOOK-001 HookHandler 인터페이스로 래핑한다.
// REQ-PL-012: load 시점 실행 없음 — command는 data로만 저장.
// HOOK-001 dispatcher가 런타임에 실제 실행을 담당한다.
type pluginHookHandler struct {
	command string
	matcher string
}

// Handle은 HookHandler 인터페이스를 구현한다.
// plugin hook은 HOOK-001 shell dispatcher가 실행하므로 여기서는 no-op이다.
func (h *pluginHookHandler) Handle(_ context.Context, _ hook.HookInput) (hook.HookJSONOutput, error) {
	return hook.HookJSONOutput{}, nil
}

// Matches는 HookHandler 인터페이스를 구현한다.
func (h *pluginHookHandler) Matches(_ hook.HookInput) bool {
	return true
}

// Command는 등록된 hook 명령어를 반환한다 (테스트용).
func (h *pluginHookHandler) Command() string {
	return h.command
}
