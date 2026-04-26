// wire.go는 goosed daemon wire-up 헬퍼와 adapter를 제공한다.
// SPEC-GOOSE-DAEMON-WIRE-001
package main

import (
	"context"
	"time"

	"github.com/modu-ai/goose/internal/core"
	"github.com/modu-ai/goose/internal/hook"
	"github.com/modu-ai/goose/internal/skill"
	"github.com/modu-ai/goose/internal/tools"
	"go.uber.org/zap"
)

// workspaceRootResolverAdapter는 CORE의 패키지 레벨 헬퍼
//
//	core.WorkspaceRoot(sessionID string) string
//
// 를 HOOK이 요구하는 interface
//
//	hook.WorkspaceRootResolver { WorkspaceRoot(sessionID) (string, error) }
//
// 로 변환한다.
//
// 빈 문자열은 "session not found" 의미이며, HOOK-001 REQ-HK-021(b)의
// fail-closed 의무에 따라 ErrHookSessionUnresolved로 변환된다.
//
// adapter는 무상태(empty struct)이며 CORE의 default SessionRegistry에 의존한다.
//
// @MX:ANCHOR: [AUTO] HOOK ↔ CORE 시그니처 브리지 — WorkspaceRootResolver interface 충족
// @MX:REASON: SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-007 — fail-closed 의무 + empty struct 보장
// @MX:SPEC: SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-002 step 8
type workspaceRootResolverAdapter struct{}

// WorkspaceRoot는 sessionID에 대응하는 workspace root를 반환한다.
// core.WorkspaceRoot가 빈 문자열을 반환하면 hook.ErrHookSessionUnresolved를 반환한다.
// os.Getenv, process CWD, /tmp 등 어떤 fallback도 사용하지 않는다 (REQ-WIRE-007).
func (workspaceRootResolverAdapter) WorkspaceRoot(sessionID string) (string, error) {
	path := core.WorkspaceRoot(sessionID)
	if path == "" {
		return "", hook.ErrHookSessionUnresolved
	}
	return path, nil
}

// wireRegistries는 5개 레지스트리를 초기화하고 반환한다.
// SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-001, REQ-WIRE-002 step 5~7
func wireRegistries(skillsRoot string, logger *zap.Logger) (
	hookRegistry *hook.HookRegistry,
	toolsRegistry *tools.Registry,
	skillRegistry *skill.SkillRegistry,
) {
	// 5. Hook registry
	hookRegistry = hook.NewHookRegistry(hook.WithLogger(logger))
	logger.Info("hook registry initialized")

	// 6. Tools registry
	toolsRegistry = tools.NewRegistryWithConfig(tools.RegistryConfig{Logger: logger})
	logger.Info("tools registry initialized")

	// 7. Skill registry
	var skillErrs []error
	skillRegistry, skillErrs = skill.LoadSkillsDir(skillsRoot, skill.WithLogger(logger))
	for _, e := range skillErrs {
		logger.Warn("skill load partial error", zap.Error(e))
	}
	logger.Info("skills loaded", zap.String("root", skillsRoot))
	return
}

// wireConsumers는 레지스트리 간 consumer 연결을 수행한다.
// 실패 시 error를 반환한다 (nil consumer → ErrInvalidConsumer + ExitConfig 경로).
// SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-002 step 8~10
func wireConsumers(
	rt *core.Runtime,
	hookRegistry *hook.HookRegistry,
	toolsRegistry *tools.Registry,
	skillRegistry *skill.SkillRegistry,
	logger *zap.Logger,
) error {
	// 8. WorkspaceRoot adapter 등록
	if err := hookRegistry.SetWorkspaceRootResolver(workspaceRootResolverAdapter{}); err != nil {
		logger.Error("wire-up failed: nil workspace resolver", zap.Error(err))
		return err
	}

	// 9. tools.Registry.Drain → core.Drain 등록 (REQ-WIRE-004)
	rt.Drain.RegisterDrainConsumer(core.DrainConsumer{
		Name:    "tools.Registry",
		Fn:      func(ctx context.Context) error { toolsRegistry.Drain(); return nil },
		Timeout: 10 * time.Second,
	})

	// 10. skills.FileChangedConsumer → hook 등록 (REQ-WIRE-005)
	// skill.FileChangedConsumer는 func([]string)[]string 시그니처이므로
	// hook.SkillsFileChangedConsumer (func(ctx, []string)[]string)로 래핑한다.
	skillsConsumer := hook.SkillsFileChangedConsumer(func(_ context.Context, changed []string) []string {
		return skillRegistry.FileChangedConsumer(changed)
	})
	if err := hookRegistry.SetSkillsFileChangedConsumer(skillsConsumer); err != nil {
		logger.Error("wire-up failed: nil skills consumer", zap.Error(err))
		return err
	}

	return nil
}

// wireInteractiveHandler는 InteractiveHandler 등록 hook point이다.
// CLI-001이 구현체를 제공할 때까지 nil + WithExplicitNoOp으로 no-op 처리한다.
// SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-009
func wireInteractiveHandler(_ *core.Runtime, _ *hook.HookRegistry, handler hook.InteractiveHandler, opts ...hook.InteractiveOpt) {
	var o hook.InteractiveOptsInternal
	for _, opt := range opts {
		opt(&o)
	}
	// nil handler + ExplicitNoOp은 정상 no-op (REQ-WIRE-009)
	// nil handler without ExplicitNoOp은 REQ-WIRE-008 위반 — 단 주입 경로는 main.go에서 제어
	_ = handler
}
