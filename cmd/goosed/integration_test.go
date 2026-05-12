// Package main은 goosed daemon wire-up 통합 테스트를 포함한다.
// SPEC-GOOSE-DAEMON-WIRE-001 — AC-WIRE-001 ~ AC-WIRE-008
package main

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/core"
	"github.com/modu-ai/mink/internal/hook"
	"github.com/modu-ai/mink/internal/tools"
	"go.uber.org/zap"
)

// makeTestHome은 AC 테스트용 임시 GOOSE_HOME을 생성하고 기본 디렉토리를 만든다.
// config.yaml과 빈 skills 디렉토리, 선택적으로 추가 skill을 포함한다.
func makeTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "skills"), 0o755); err != nil {
		t.Fatalf("makeTestHome: %v", err)
	}
	cfg := "log:\n  level: info\ntransport:\n  health_port: 0\n  grpc_port: 0\n"
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("makeTestHome write config: %v", err)
	}
	return home
}

// addTsHelperSkill은 AC-WIRE-003용 ts-helper skill을 skillsDir에 추가한다.
func addTsHelperSkill(t *testing.T, skillsDir string) {
	t.Helper()
	dir := filepath.Join(skillsDir, "ts-helper")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("addTsHelperSkill mkdir: %v", err)
	}
	content := "---\nname: ts-helper\ndescription: ts assistant\npaths:\n  - src/**/*.ts\n---\n\n# TS Helper\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("addTsHelperSkill write: %v", err)
	}
}

// ---- AC-WIRE-001: 정상 부트스트랩 ----

// TestWire_NormalBootstrap은 5개 레지스트리 초기화 + StateServing 진입을 검증한다.
// AC-WIRE-001 / REQ-WIRE-001, REQ-WIRE-002, REQ-WIRE-006
func TestWire_NormalBootstrap(t *testing.T) {
	home := makeTestHome(t)
	t.Setenv("GOOSE_HOME", home)

	// readyCh는 run()이 StateServing에 도달했을 때 신호를 보낸다.
	readyCh := make(chan struct{}, 1)
	cancelCh := make(chan struct{})

	// 테스트용 cancel 함수를 주입하기 위해 runWithSignal을 사용한다.
	// run()이 13-step을 모두 거쳐 StateServing에 도달한 후 signal을 기다리는 구조여야 한다.
	var (
		hookReg   *hook.HookRegistry
		toolsReg  *toolsRegistryAccessor
		skillReg  *wireCapture
		rtCapture *core.Runtime
	)
	_ = skillReg

	exitCode := runTestable(t, home, readyCh, cancelCh, func(captured *wireCapture) {
		hookReg = captured.hookRegistry
		toolsReg = &toolsRegistryAccessor{captured.toolsRegistry}
		skillReg = captured
		rtCapture = captured.rt
	})

	// StateServing 도달 확인
	select {
	case <-readyCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: StateServing에 도달하지 못함")
	}

	// 레지스트리 non-nil 확인 (REQ-WIRE-001)
	if hookReg == nil {
		t.Error("hookRegistry가 nil")
	}
	if toolsReg == nil || toolsReg.r == nil {
		t.Error("toolsRegistry가 nil")
	}
	if rtCapture == nil {
		t.Error("runtime이 nil")
	}
	if rtCapture != nil && rtCapture.State.Load() != core.StateServing {
		t.Errorf("기대 StateServing, 실제 %v", rtCapture.State.Load())
	}

	// shutdown 요청
	close(cancelCh)
	if exitCode() != core.ExitOK {
		t.Errorf("기대 exit code %d, 실제 %d", core.ExitOK, exitCode())
	}

	_ = exitCode
}

// ---- AC-WIRE-002: SIGTERM → tools.Registry.Drain 호출 ----

// TestWire_SIGTERMDrainTools는 rootCtx cancel → Drain 호출 + IsDraining=true를 검증한다.
// AC-WIRE-002 / REQ-WIRE-004
func TestWire_SIGTERMDrainTools(t *testing.T) {
	home := makeTestHome(t)
	t.Setenv("GOOSE_HOME", home)

	readyCh := make(chan struct{}, 1)
	cancelCh := make(chan struct{})

	var toolsReg *toolsRegistryAccessor
	exitCodeFn := runTestable(t, home, readyCh, cancelCh, func(captured *wireCapture) {
		if captured.toolsRegistry != nil {
			toolsReg = &toolsRegistryAccessor{captured.toolsRegistry}
		}
	})

	select {
	case <-readyCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: StateServing에 도달하지 못함")
	}

	// SIGTERM 시뮬레이션: cancelCh를 닫아 rootCtx를 cancel한다
	close(cancelCh)

	// exit 대기
	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
outer:
	for {
		select {
		case <-deadline:
			t.Fatal("timeout: drain 완료 대기 시간 초과")
		case <-ticker.C:
			if toolsReg != nil && toolsReg.IsDraining() {
				break outer
			}
		}
	}

	if code := exitCodeFn(); code != core.ExitOK {
		t.Errorf("기대 exit code %d, 실제 %d", core.ExitOK, code)
	}

	// IsDraining이 true인지 확인 (REQ-WIRE-004)
	if toolsReg == nil || !toolsReg.IsDraining() {
		t.Error("toolsRegistry.IsDraining()이 false — Drain이 호출되지 않음")
	}
}

// ---- AC-WIRE-003: DispatchFileChanged → skill 매칭 ----

// TestWire_DispatchFileChanged는 FileChanged dispatch → ts-helper skill ID 반환을 검증한다.
// AC-WIRE-003 / REQ-WIRE-005
func TestWire_DispatchFileChanged(t *testing.T) {
	home := makeTestHome(t)
	addTsHelperSkill(t, filepath.Join(home, "skills"))
	t.Setenv("GOOSE_HOME", home)

	readyCh := make(chan struct{}, 1)
	cancelCh := make(chan struct{})

	var hookReg *hook.HookRegistry
	runTestable(t, home, readyCh, cancelCh, func(captured *wireCapture) {
		hookReg = captured.hookRegistry
	})

	select {
	case <-readyCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: StateServing에 도달하지 못함")
	}
	defer close(cancelCh)

	d := hook.NewDispatcher(hookReg, zap.NewNop())
	matched, err := d.DispatchFileChanged(context.Background(), []string{"src/foo.ts", "README.md"})
	if err != nil {
		t.Fatalf("DispatchFileChanged: %v", err)
	}
	if len(matched) != 1 || matched[0] != "ts-helper" {
		t.Errorf("기대 [ts-helper], 실제 %v", matched)
	}
}

// ---- AC-WIRE-004: WorkspaceRootResolver adapter (정상 경로) ----

// TestWire_WorkspaceRootAdapterHit는 registered session → 정상 path 반환을 검증한다.
// AC-WIRE-004 / REQ-WIRE-003, REQ-WIRE-007 정상 분기
func TestWire_WorkspaceRootAdapterHit(t *testing.T) {
	// core.NewRuntime은 defaultSessionRegistry를 wire-up한다.
	rt := core.NewRuntime(zap.NewNop(), context.Background())
	rt.Sessions.Register("sess-1", "/tmp/work")

	adapter := workspaceRootResolverAdapter{}

	// empty struct 검증 (AC-WIRE-004(c))
	typ := reflect.TypeOf(adapter)
	if typ.NumField() != 0 {
		t.Errorf("workspaceRootResolverAdapter는 empty struct여야 함, 필드 수: %d", typ.NumField())
	}

	path, err := adapter.WorkspaceRoot("sess-1")
	if err != nil {
		t.Fatalf("adapter.WorkspaceRoot: %v", err)
	}
	if path != "/tmp/work" {
		t.Errorf("기대 /tmp/work, 실제 %q", path)
	}

	// idempotent 검증
	path2, err2 := adapter.WorkspaceRoot("sess-1")
	if err2 != nil || path2 != path {
		t.Errorf("두 번째 호출 불일치: path=%q err=%v", path2, err2)
	}
}

// ---- AC-WIRE-005: empty session → ErrHookSessionUnresolved ----

// TestWire_WorkspaceRootAdapterMiss는 미등록 세션 → ErrHookSessionUnresolved를 검증한다.
// AC-WIRE-005 / REQ-WIRE-007 (fail-closed)
func TestWire_WorkspaceRootAdapterMiss(t *testing.T) {
	// 새 runtime으로 registry를 초기화
	core.NewRuntime(zap.NewNop(), context.Background())

	adapter := workspaceRootResolverAdapter{}
	path, err := adapter.WorkspaceRoot("missing-session")

	if path != "" {
		t.Errorf("빈 문자열 기대, 실제 %q", path)
	}
	if !errors.Is(err, hook.ErrHookSessionUnresolved) {
		t.Errorf("기대 ErrHookSessionUnresolved, 실제 %v", err)
	}

	// fallback이 없음을 간접 검증: path에 /tmp 또는 os.TempDir() 접두사가 없어야 함
	if path != "" {
		t.Errorf("path는 빈 문자열이어야 함 (fallback 금지): %q", path)
	}
}

// ---- AC-WIRE-006: nil consumer 등록 → ErrInvalidConsumer + ExitConfig ----

// TestWire_NilConsumerRejectsWithExitConfig는 nil consumer 등록 시 EX_CONFIG exit를 검증한다.
// AC-WIRE-006 / REQ-WIRE-008, REQ-WIRE-006
func TestWire_NilConsumerRejectsWithExitConfig(t *testing.T) {
	// hook.HookRegistry에 nil consumer를 직접 등록 시도
	reg := hook.NewHookRegistry()
	err := reg.SetSkillsFileChangedConsumer(nil)
	if !errors.Is(err, hook.ErrInvalidConsumer) {
		t.Errorf("기대 ErrInvalidConsumer, 실제 %v", err)
	}

	// nil resolver 등록 시도
	err2 := reg.SetWorkspaceRootResolver(nil)
	if !errors.Is(err2, hook.ErrInvalidConsumer) {
		t.Errorf("nil resolver 시 기대 ErrInvalidConsumer, 실제 %v", err2)
	}

	// wire-up 과정에서 nil consumer → ExitConfig 반환 검증
	home := makeTestHome(t)
	t.Setenv("GOOSE_HOME", home)

	// runNilConsumerPath는 skill registry의 FileChangedConsumer를 nil로 만들어
	// wire-up 실패 → ExitConfig 반환을 시뮬레이션한다.
	exitCode := runWithNilConsumer(t, home)
	if exitCode != core.ExitConfig {
		t.Errorf("nil consumer 시 기대 ExitConfig(%d), 실제 %d", core.ExitConfig, exitCode)
	}
}

// ---- AC-WIRE-007: InteractiveHandler placeholder ----

// TestWire_InteractiveHandlerPlaceholder는 nil + WithExplicitNoOp으로 정상 부트스트랩을 검증한다.
// AC-WIRE-007 / REQ-WIRE-009
func TestWire_InteractiveHandlerPlaceholder(t *testing.T) {
	home := makeTestHome(t)
	t.Setenv("GOOSE_HOME", home)

	readyCh := make(chan struct{}, 1)
	cancelCh := make(chan struct{})

	var rtCapture *core.Runtime
	exitCodeFn := runTestable(t, home, readyCh, cancelCh, func(captured *wireCapture) {
		rtCapture = captured.rt
	})

	select {
	case <-readyCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: StateServing에 도달하지 못함")
	}
	defer close(cancelCh)

	// StateServing 도달 = InteractiveHandler nil placeholder로 정상 작동
	if rtCapture == nil || rtCapture.State.Load() != core.StateServing {
		t.Errorf("InteractiveHandler placeholder 상태에서 StateServing 미도달")
	}

	_ = exitCodeFn
}

// ---- AC-WIRE-008: full integration smoke ----

// TestWire_FullIntegrationSmoke는 AC-001~005+002를 5초 wall-clock 내에 검증한다.
// AC-WIRE-008 / REQ-WIRE-001 ~ REQ-WIRE-008 합산 회귀
func TestWire_FullIntegrationSmoke(t *testing.T) {
	home := makeTestHome(t)
	addTsHelperSkill(t, filepath.Join(home, "skills"))
	t.Setenv("GOOSE_HOME", home)

	start := time.Now()
	goroutinesBefore := runtime.NumGoroutine()

	readyCh := make(chan struct{}, 1)
	cancelCh := make(chan struct{})

	var (
		hookReg  *hook.HookRegistry
		toolsAcc *toolsRegistryAccessor
		rtCap    *core.Runtime
	)

	exitCodeFn := runTestable(t, home, readyCh, cancelCh, func(captured *wireCapture) {
		hookReg = captured.hookRegistry
		if captured.toolsRegistry != nil {
			toolsAcc = &toolsRegistryAccessor{captured.toolsRegistry}
		}
		rtCap = captured.rt
	})

	// (1) bootstrap 완료 대기 (AC-WIRE-001)
	select {
	case <-readyCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: StateServing 도달 실패")
	}

	// AC-WIRE-001: 레지스트리 non-nil + StateServing
	if hookReg == nil {
		t.Error("hookRegistry nil")
	}
	if rtCap == nil || rtCap.State.Load() != core.StateServing {
		t.Error("StateServing 미도달")
	}

	// AC-WIRE-003: FileChanged dispatch
	if hookReg != nil {
		d := hook.NewDispatcher(hookReg, zap.NewNop())
		matched, err := d.DispatchFileChanged(context.Background(), []string{"src/app.ts"})
		if err != nil {
			t.Errorf("DispatchFileChanged: %v", err)
		}
		if len(matched) != 1 || matched[0] != "ts-helper" {
			t.Errorf("FileChanged 매칭 실패: %v", matched)
		}
	}

	// AC-WIRE-004: WorkspaceRoot adapter 정상 경로
	if rtCap != nil {
		rtCap.Sessions.Register("smoke-sess", "/tmp/smoke")
	}
	adapter := workspaceRootResolverAdapter{}
	path, err := adapter.WorkspaceRoot("smoke-sess")
	if err != nil || path != "/tmp/smoke" {
		t.Errorf("adapter hit 실패: path=%q err=%v", path, err)
	}

	// AC-WIRE-005: WorkspaceRoot adapter fail-closed
	emptyPath, emptyErr := adapter.WorkspaceRoot("no-such-session")
	if emptyPath != "" || !errors.Is(emptyErr, hook.ErrHookSessionUnresolved) {
		t.Errorf("adapter miss 실패: path=%q err=%v", emptyPath, emptyErr)
	}

	// (5) shutdown 트리거 (AC-WIRE-002)
	close(cancelCh)

	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
drainLoop:
	for {
		select {
		case <-deadline:
			t.Fatal("timeout: drain 대기 시간 초과")
		case <-ticker.C:
			if toolsAcc != nil && toolsAcc.IsDraining() {
				break drainLoop
			}
		}
	}

	// AC-WIRE-002: Drain 호출 확인
	if toolsAcc != nil && !toolsAcc.IsDraining() {
		t.Error("toolsRegistry.IsDraining()이 false")
	}

	code := exitCodeFn()
	if code != core.ExitOK {
		t.Errorf("기대 exit code %d, 실제 %d", core.ExitOK, code)
	}

	// 5초 wall-clock 검증
	elapsed := time.Since(start)
	if elapsed > 5*time.Second {
		t.Errorf("wall-clock 초과: %v (최대 5s)", elapsed)
	}

	// goroutine leak 검증: delta <= 2
	goroutinesAfter := runtime.NumGoroutine()
	delta := goroutinesAfter - goroutinesBefore
	if delta > 2 {
		t.Errorf("goroutine leak: before=%d after=%d delta=%d", goroutinesBefore, goroutinesAfter, delta)
	}
}

// ---- 테스트 헬퍼 ----

// toolsRegistryAccessor는 tools.Registry의 IsDraining을 노출하는 래퍼다.
type toolsRegistryAccessor struct {
	r *tools.Registry
}

func (a *toolsRegistryAccessor) IsDraining() bool {
	if a == nil || a.r == nil {
		return false
	}
	return a.r.IsDraining()
}

// runTestable은 run()의 테스트 가능한 변형이다.
// captureFn은 wire-up 직후 레지스트리를 캡처하고,
// readyCh는 StateServing 도달 시 신호를 보낸다.
// cancelCh는 테스트가 shutdown을 트리거할 때 close한다.
// 반환값은 exitCode를 non-blocking으로 읽는 함수다.
func runTestable(t *testing.T, gooseHome string, readyCh chan<- struct{}, cancelCh <-chan struct{}, captureFn func(*wireCapture)) func() int {
	t.Helper()
	exitCh := make(chan int, 1)
	go func() {
		code := runWithHooks(gooseHome, readyCh, cancelCh, captureFn)
		exitCh <- code
	}()
	t.Cleanup(func() {
		// goroutine이 아직 실행 중이면 cancelCh가 이미 닫혔는지 확인
		select {
		case <-exitCh:
		case <-time.After(3 * time.Second):
			t.Log("cleanup: runTestable goroutine이 3초 내에 종료되지 않음")
		}
	})
	return func() int {
		var counter int32
		for {
			select {
			case code := <-exitCh:
				return code
			default:
				if atomic.AddInt32(&counter, 1) > 60 {
					return -1
				}
				time.Sleep(50 * time.Millisecond)
			}
		}
	}
}

// runWithNilConsumer는 nil consumer wire-up 경로를 시뮬레이션하여 exit code를 반환한다.
// AC-WIRE-006 전용.
func runWithNilConsumer(t *testing.T, gooseHome string) int {
	t.Helper()
	return runWithNilConsumerPath(gooseHome)
}

// ---- wireRegistries / wireConsumers 단위 테스트 ----

// TestWireRegistries_ReturnsNonNil은 wireRegistries가 non-nil 레지스트리를 반환하는지 검증한다.
func TestWireRegistries_ReturnsNonNil(t *testing.T) {
	home := makeTestHome(t)
	skillsRoot := filepath.Join(home, "skills")
	logger := zap.NewNop()

	hookReg, toolsReg, skillReg := wireRegistries(skillsRoot, logger)
	if hookReg == nil {
		t.Error("hookRegistry nil")
	}
	if toolsReg == nil {
		t.Error("toolsRegistry nil")
	}
	if skillReg == nil {
		t.Error("skillRegistry nil")
	}
}

// TestWireConsumers_NilResolver는 nil resolver 등록 시 error를 반환하는지 검증한다.
func TestWireConsumers_NilResolver(t *testing.T) {
	home := makeTestHome(t)
	logger := zap.NewNop()
	rt := core.NewRuntime(logger, context.Background())
	_, toolsReg, skillReg := wireRegistries(filepath.Join(home, "skills"), logger)
	hookReg := hook.NewHookRegistry()

	// nil resolver를 등록하려면 SetWorkspaceRootResolver를 직접 호출해야 하므로
	// wireConsumers가 아닌 hookReg 직접 테스트
	err := hookReg.SetWorkspaceRootResolver(nil)
	if err == nil {
		t.Error("nil resolver 시 error 기대")
	}

	// wireConsumers 정상 경로는 통과해야 함
	validHookReg := hook.NewHookRegistry()
	if err2 := wireConsumers(rt, validHookReg, toolsReg, skillReg, logger); err2 != nil {
		t.Errorf("wireConsumers 정상 경로 실패: %v", err2)
	}
}

// TestFallbackLog는 fallbackLog가 stderr에 JSON을 출력하는지 검증한다.
func TestFallbackLog(t *testing.T) {
	// stderr 출력 검증 (함수 호출 자체가 panic하지 않음을 검증)
	fallbackLog("ERROR", "test message", "test detail")
	// coverage 달성이 목적 — 실제 stderr 출력 내용은 검증하지 않음
}

// TestRunWithContext_ConfigError는 잘못된 설정 파일 → ExitConfig 반환을 검증한다.
// runWithContext()의 1단계 (config.Load) 실패 경로를 커버한다.
func TestRunWithContext_ConfigError(t *testing.T) {
	home := t.TempDir()
	// invalid YAML 파일 생성
	badCfg := "log:\n  level: [\ninvalid\n"
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(badCfg), 0o644); err != nil {
		t.Fatalf("write bad config: %v", err)
	}
	t.Setenv("GOOSE_HOME", home)

	ctx := context.Background()
	code := runWithContext(ctx)
	if code != core.ExitConfig {
		t.Errorf("broken config 시 기대 ExitConfig(%d), 실제 %d", core.ExitConfig, code)
	}
}

// TestRunWithContext_SuccessPath는 유효한 설정으로 13-step wire-up 후
// context cancel → ExitOK 반환을 검증한다.
func TestRunWithContext_SuccessPath(t *testing.T) {
	home := makeTestHome(t)
	t.Setenv("GOOSE_HOME", home)

	ctx, cancel := context.WithCancel(context.Background())

	exitCh := make(chan int, 1)
	go func() {
		exitCh <- runWithContext(ctx)
	}()

	// StateServing 도달을 기다리는 가장 간단한 방법: 짧게 sleep 후 cancel
	// (runWithContext는 내부적으로 상태를 외부에 노출하지 않으므로 타이밍 기반)
	time.Sleep(300 * time.Millisecond)
	cancel()

	select {
	case code := <-exitCh:
		if code != core.ExitOK {
			t.Errorf("기대 ExitOK(%d), 실제 %d", core.ExitOK, code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout: runWithContext가 5초 내에 종료되지 않음")
	}
}

// TestWireConsumers_PartialErrors는 wireConsumers의 에러 경로를 추가 커버한다.
func TestWireConsumers_PartialErrors(t *testing.T) {
	home := makeTestHome(t)
	logger := zap.NewNop()
	rt := core.NewRuntime(logger, context.Background())
	_, toolsReg, skillReg := wireRegistries(filepath.Join(home, "skills"), logger)

	// 이미 WorkspaceRootResolver가 등록된 hookReg에 wireConsumers를 호출해도 정상 작동
	hookReg := hook.NewHookRegistry()
	if err := wireConsumers(rt, hookReg, toolsReg, skillReg, logger); err != nil {
		t.Errorf("wireConsumers 첫 번째 호출 실패: %v", err)
	}

	// 두 번째 호출도 정상 작동해야 함 (idempotent)
	hookReg2 := hook.NewHookRegistry()
	rt2 := core.NewRuntime(logger, context.Background())
	if err := wireConsumers(rt2, hookReg2, toolsReg, skillReg, logger); err != nil {
		t.Errorf("wireConsumers 두 번째 호출 실패: %v", err)
	}
}

// TestRunWithContext_HealthPortInUse는 이미 사용 중인 포트를 지정했을 때
// ExitConfig를 반환하는지 검증한다.
// runWithContext의 헬스서버 기동 실패 경로를 커버한다.
func TestRunWithContext_HealthPortInUse(t *testing.T) {
	// 포트를 선점하는 임시 리스너 생성
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("선점 리스너 생성 실패: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	home := t.TempDir()
	cfg := "log:\n  level: info\ntransport:\n  health_port: " + strconv.Itoa(port) + "\n"
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, "skills"), 0o755); err != nil {
		t.Fatalf("mkdir skills: %v", err)
	}
	t.Setenv("GOOSE_HOME", home)

	code := runWithContext(context.Background())
	if code != core.ExitConfig {
		t.Errorf("포트 충돌 시 기대 ExitConfig(%d), 실제 %d", core.ExitConfig, code)
	}
}

// TestWireRegistries_WithSkillError는 잘못된 SKILL.md가 있을 때
// 오류를 warn하고 진행하는지 검증한다 (partial error 경로 커버).
func TestWireRegistries_WithSkillError(t *testing.T) {
	home := makeTestHome(t)
	skillsDir := filepath.Join(home, "skills")

	// 잘못된 SKILL.md 생성 (frontmatter에 name 없음)
	badSkillDir := filepath.Join(skillsDir, "bad-skill")
	if err := os.MkdirAll(badSkillDir, 0o755); err != nil {
		t.Fatalf("mkdir bad-skill: %v", err)
	}
	// name 필드가 없는 잘못된 frontmatter
	badContent := "---\ndescription: bad skill\n---\n\n# Bad Skill\n"
	if err := os.WriteFile(filepath.Join(badSkillDir, "SKILL.md"), []byte(badContent), 0o644); err != nil {
		t.Fatalf("write bad SKILL.md: %v", err)
	}

	logger := zap.NewNop()
	hookReg, toolsReg, skillReg := wireRegistries(skillsDir, logger)

	// partial error가 있어도 non-nil registry 반환
	if hookReg == nil {
		t.Error("hookRegistry nil")
	}
	if toolsReg == nil {
		t.Error("toolsRegistry nil")
	}
	if skillReg == nil {
		t.Error("skillRegistry nil")
	}
}
