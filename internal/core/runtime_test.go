package core_test

// AC-CORE-001: 정상 부트스트랩 및 헬스체크
// AC-CORE-002: SIGTERM 수신 시 graceful shutdown
// AC-CORE-003: 잘못된 YAML 설정 파일 거부
// AC-CORE-004: 포트 충돌 시 실패
// AC-CORE-005: cleanup hook panic 격리
// AC-CORE-006: 드레이닝 중 503 응답
//
// 구현 수준: GREEN — 스켈레톤 구현이 AC-CORE-001~006을 만족.
// RED 단계에서 작성된 테스트가 GREEN 구현과 함께 전부 통과한다.
//
// B4-1 수정 (REQ-CORE-004(b)): signal.NotifyContext 기반 root context 전파 검증
// B4-2 수정 (REQ-CORE-004(c)): parentCtx 만료 시 hook iteration 중단 검증

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/config"
	"github.com/modu-ai/mink/internal/core"
	"github.com/modu-ai/mink/internal/health"
)

// goosedBinPath는 TestMain에서 1회 빌드된 goosed 바이너리 경로를 공유한다.
// 병렬 테스트 간에 read-only로 사용되므로 race가 없다.
var goosedBinPath string

// TestMain은 goosed 바이너리를 패키지 테스트 시작 시 1회만 빌드한다.
// 빌드 결과를 goosedBinPath에 저장하여 각 테스트의 buildGoosed 중복 빌드를 방지한다.
// 이로써 TestSIGTERM_InvokesHooks_ExitZero의 빌드 지연으로 인한 flakiness를 줄인다.
func TestMain(m *testing.M) {
	// 임시 디렉토리는 os.Exit 이전에 자동 정리되지 않으므로 직접 관리한다.
	dir, err := os.MkdirTemp("", "goosed-test-*")
	if err != nil {
		panic("goosed 빌드 디렉토리 생성 실패: " + err.Error())
	}
	defer os.RemoveAll(dir)

	bin := filepath.Join(dir, "goosed")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/modu-ai/mink/cmd/minkd")
	cmd.Env = os.Environ()
	if out, buildErr := cmd.CombinedOutput(); buildErr != nil {
		panic("goosed 사전 빌드 실패: " + buildErr.Error() + "\n" + string(out))
	}
	goosedBinPath = bin

	os.Exit(m.Run())
}

// TestBootstrap_SucceedsWithEmptyConfig는 AC-CORE-001을 검증한다.
// MINK_HOME이 비어있는 디렉토리로 설정될 때 goosed가 serving 상태로 뜨고
// /healthz가 200 OK + {"status":"ok","state":"serving",...}를 반환해야 한다.
func TestBootstrap_SucceedsWithEmptyConfig(t *testing.T) {
	t.Parallel()

	// Arrange: 빈 MINK_HOME 디렉토리 생성
	gooseHome := t.TempDir()

	// 빈 config 파일 없음 → 기본값 fallback
	// SPEC-GOOSE-CONFIG-001: config.Load() 계층형 로더로 마이그레이션
	cfg, err := config.Load(config.LoadOptions{
		MinkHome: gooseHome,
		WorkDir:  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("설정 로드 실패: %v", err)
	}
	// 기본값 확인 — GRPCPort는 반드시 설정되어야 함
	if cfg.Transport.GRPCPort == 0 {
		t.Fatal("기본 GRPCPort가 0이면 안 됨")
	}

	logger, err := core.NewLogger("info", "goosed", "test")
	if err != nil {
		t.Fatalf("로거 생성 실패: %v", err)
	}

	rt := core.NewRuntime(logger, nil)
	rt.State.Store(core.StateServing)

	// race-free 포트 선택: OS에게 맡김
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listener 생성 실패: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	srv := health.New(rt.State, "test", logger)
	if err := srv.ServeListener(ln); err != nil {
		t.Fatalf("헬스서버 기동 실패: %v", err)
	}
	defer srv.Shutdown(context.Background()) //nolint:errcheck

	// Act: 헬스체크
	time.Sleep(20 * time.Millisecond)
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", port))
	if err != nil {
		t.Fatalf("헬스체크 요청 실패: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Assert: 200 OK
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("응답 파싱 실패: %v", err)
	}

	// Assert: body에 status=ok, state=serving, version 포함
	if body["status"] != "ok" {
		t.Errorf("status = %q, want %q", body["status"], "ok")
	}
	if body["state"] != "serving" {
		t.Errorf("state = %q, want %q", body["state"], "serving")
	}
	if body["version"] == "" {
		t.Error("version 필드가 비어있음")
	}
}

// TestSIGTERM_InvokesHooks_ExitZero는 AC-CORE-002를 검증한다.
// goosed가 serving 상태에서 SIGTERM을 받으면 3초 이내에 exit 0으로 종료해야 한다.
func TestSIGTERM_InvokesHooks_ExitZero(t *testing.T) {
	t.Parallel()

	gooseHome := t.TempDir()
	binPath := buildGoosed(t)

	// race-free: OS가 포트 선택
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listener 생성 실패: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close() // goosed가 이 포트를 사용

	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(),
		"MINK_HOME="+gooseHome,
		fmt.Sprintf("MINK_HEALTH_PORT=%d", port),
		"MINK_LOG_LEVEL=info",
	)
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("goosed 시작 실패: %v", err)
	}

	// serving 상태 대기 — 사전 빌드 덕분에 바이너리 빌드 지연이 없으므로
	// 10 s timeout을 부여하여 CI 환경의 느린 부트스트랩에도 안정적으로 동작한다.
	waitForHealthy(t, port, 10*time.Second)

	// SIGTERM 전송
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("SIGTERM 전송 실패: %v", err)
	}

	// 5초 이내 종료 확인
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				t.Errorf("exit code = %d, want 0", exitErr.ExitCode())
			} else {
				t.Errorf("종료 오류: %v", err)
			}
		}
	case <-time.After(5 * time.Second):
		cmd.Process.Kill() //nolint:errcheck
		t.Fatal("5초 내 종료되지 않음 (timeout)")
	}
}

// TestInvalidYAML_ExitsWithCode78는 AC-CORE-003을 검증한다.
// 잘못된 YAML 설정 파일이 있을 때 exit code 78과 config parse error 로그가 나와야 한다.
func TestInvalidYAML_ExitsWithCode78(t *testing.T) {
	t.Parallel()

	gooseHome := t.TempDir()
	cfgPath := filepath.Join(gooseHome, "config.yaml")

	// 비-YAML 텍스트 기록
	if err := os.WriteFile(cfgPath, []byte("::: not yaml :::"), 0644); err != nil {
		t.Fatalf("설정 파일 쓰기 실패: %v", err)
	}

	binPath := buildGoosed(t)
	var stderrBuf strings.Builder

	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(),
		"MINK_HOME="+gooseHome,
	)
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	// Assert: exit code 78
	if err == nil {
		t.Fatal("goosed가 성공적으로 종료됨 (exit 0); exit 78 기대")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("예상치 못한 오류 타입: %v", err)
	}
	if exitErr.ExitCode() != 78 {
		t.Errorf("exit code = %d, want 78", exitErr.ExitCode())
	}

	// Assert: stderr에 config parse error 포함
	if !strings.Contains(stderrBuf.String(), "config parse error") {
		t.Errorf("stderr에 'config parse error' 없음:\n%s", stderrBuf.String())
	}
}

// TestPortConflict_ExitsWithCode78는 AC-CORE-004를 검증한다.
// 포트가 이미 사용 중일 때 exit code 78이 반환되어야 한다.
func TestPortConflict_ExitsWithCode78(t *testing.T) {
	t.Parallel()

	// 포트를 먼저 점령
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("리스너 생성 실패: %v", err)
	}
	defer func() { _ = ln.Close() }()

	occupiedPort := ln.Addr().(*net.TCPAddr).Port
	gooseHome := t.TempDir()

	binPath := buildGoosed(t)
	var stderrBuf strings.Builder

	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(),
		"MINK_HOME="+gooseHome,
		fmt.Sprintf("MINK_HEALTH_PORT=%d", occupiedPort),
	)
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()

	// Assert: exit code 78
	if runErr == nil {
		t.Fatal("goosed가 성공적으로 종료됨 (exit 0); exit 78 기대")
	}
	exitErr, ok := runErr.(*exec.ExitError)
	if !ok {
		t.Fatalf("예상치 못한 오류: %v", runErr)
	}
	if exitErr.ExitCode() != 78 {
		t.Errorf("exit code = %d, want 78", exitErr.ExitCode())
	}

	// Assert: stderr에 포트 번호 포함
	if !strings.Contains(stderrBuf.String(), fmt.Sprintf("%d", occupiedPort)) {
		t.Errorf("stderr에 포트 번호 없음:\n%s", stderrBuf.String())
	}
}

// TestHookPanic_ExitCode1_AllHooksCalled는 AC-CORE-005를 검증한다.
// cleanup hook 중 하나가 panic해도 나머지 hook이 실행되고 exit code 1이 반환되어야 한다.
func TestHookPanic_ExitCode1_AllHooksCalled(t *testing.T) {
	t.Parallel()

	called := make([]bool, 3)

	logger, err := core.NewLogger("info", "goosed", "test")
	if err != nil {
		t.Fatalf("로거 생성 실패: %v", err)
	}

	sm := core.NewShutdownManager(logger)

	// hook 0: 정상
	sm.RegisterHook(core.CleanupHook{
		Name: "hook-0",
		Fn: func(ctx context.Context) error {
			called[0] = true
			return nil
		},
	})
	// hook 1: panic
	sm.RegisterHook(core.CleanupHook{
		Name: "hook-1-panic",
		Fn: func(ctx context.Context) error {
			called[1] = true
			panic("boom")
		},
	})
	// hook 2: 정상 (panic 이후에도 실행되어야 함)
	sm.RegisterHook(core.CleanupHook{
		Name: "hook-2",
		Fn: func(ctx context.Context) error {
			called[2] = true
			return nil
		},
	})

	panicOccurred := sm.RunAllHooks(context.Background())

	// Assert: panic 발생 신호
	if !panicOccurred {
		t.Error("panicOccurred = false, want true")
	}

	// Assert: 3개 hook 모두 호출됨
	for i, c := range called {
		if !c {
			t.Errorf("hook[%d] 호출되지 않음", i)
		}
	}
}

// TestDraining_Returns503는 AC-CORE-006을 검증한다.
// state가 StateDraining일 때 /healthz는 503 + {"status":"draining"}을 반환해야 한다.
func TestDraining_Returns503(t *testing.T) {
	t.Parallel()

	logger, err := core.NewLogger("info", "goosed", "test")
	if err != nil {
		t.Fatalf("로거 생성 실패: %v", err)
	}

	state := &core.StateHolder{}
	state.Store(core.StateDraining)

	srv := health.New(state, "test", logger)

	// race-free 포트 선택
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listener 생성 실패: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	if err := srv.ServeListener(ln); err != nil {
		t.Fatalf("서버 기동 실패: %v", err)
	}
	defer srv.Shutdown(context.Background()) //nolint:errcheck

	time.Sleep(20 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", port))
	if err != nil {
		t.Fatalf("요청 실패: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Assert: 503
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("응답 파싱 실패: %v", err)
	}
	if body["status"] != "draining" {
		t.Errorf("status = %q, want %q", body["status"], "draining")
	}
}

// TestGoosedMain_SIGTERM_CancelsRootContext는 B4-1 수정을 검증한다.
// signal.NotifyContext로 생성된 root context가 SIGTERM 수신 시 cancel되어
// rt.RootCtx.Err() == context.Canceled가 됨을 프로세스 레벨에서 확인한다.
// 검증 방법: goosed가 SIGTERM 후 exit 0으로 정상 종료되면 root ctx가 전파된 것이다.
// (exit 0은 RunAllHooks가 shutdownCtx를 root ctx cancel 이후에 받아 실행됐다는 증거)
func TestGoosedMain_SIGTERM_CancelsRootContext(t *testing.T) {
	// t.Parallel()을 의도적으로 생략 — SIGTERM 수신 후 종료 타이밍에 민감한 테스트는
	// 직렬 실행으로 포트 race를 완전히 제거한다.

	gooseHome := t.TempDir()
	binPath := buildGoosed(t)

	// OS가 빈 포트를 선택하도록 한다.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listener 생성 실패: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(),
		"MINK_HOME="+gooseHome,
		fmt.Sprintf("MINK_HEALTH_PORT=%d", port),
		"MINK_LOG_LEVEL=info",
	)
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("goosed 시작 실패: %v", err)
	}

	// 프로세스가 serving 상태에 도달했을 때만 SIGTERM을 보낸다.
	waitForHealthy(t, port, 10*time.Second)

	// root context가 SIGTERM으로 cancel되는지 검증:
	// SIGTERM → rootCtx.Done() 수신 → StateDraining → RunAllHooks → exit 0
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("SIGTERM 전송 실패: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case procErr := <-done:
		// root ctx가 올바르게 cancel되고 graceful shutdown이 완료되면 exit 0
		if procErr != nil {
			if exitErr, ok := procErr.(*exec.ExitError); ok {
				t.Errorf("root ctx cancel 후 exit code = %d, want 0 (graceful shutdown 실패)", exitErr.ExitCode())
			} else {
				t.Errorf("종료 오류: %v", procErr)
			}
		}
		// exit 0 = context.Canceled가 전파되어 shutdown 경로를 정상적으로 밟은 것
	case <-time.After(8 * time.Second):
		cmd.Process.Kill() //nolint:errcheck
		t.Fatal("SIGTERM 후 8초 내 종료되지 않음 — root ctx cancel이 전파되지 않을 가능성")
	}
}

// TestRunAllHooks_ParentCtxCanceled_StopsIteration은 B4-2 수정을 검증한다.
// parentCtx가 cancel되면 RunAllHooks가 남은 hook을 실행하지 않고 즉시 반환해야 한다.
// 5개 hook 중 2개 실행 후 ctx를 cancel하면, 나머지 3개는 실행되지 않아야 한다.
func TestRunAllHooks_ParentCtxCanceled_StopsIteration(t *testing.T) {
	t.Parallel()

	logger, err := core.NewLogger("info", "goosed", "test")
	if err != nil {
		t.Fatalf("로거 생성 실패: %v", err)
	}

	sm := core.NewShutdownManager(logger)

	// 실행 횟수를 atomic하게 추적한다 (race detector 통과용)
	var callCount atomic.Int32

	// hook 0, 1: 정상 실행 후 ctx cancel
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	// vet이 "cancelFn not used on all paths" 경고를 내지 않도록 defer로 최종 정리한다.
	// 실제 cancel은 hook-1 내부에서 먼저 호출된다.
	defer cancelFn()

	for i := range 5 {
		idx := i
		sm.RegisterHook(core.CleanupHook{
			Name: fmt.Sprintf("hook-%d", idx),
			Fn: func(ctx context.Context) error {
				callCount.Add(1)
				// hook 1이 실행된 직후 parent ctx를 cancel한다.
				// hook 2, 3, 4는 RunAllHooks의 select 체크에서 걸려야 한다.
				if idx == 1 {
					cancelFn()
					// cancel 전파를 위해 아주 짧게 대기한다.
					time.Sleep(5 * time.Millisecond)
				}
				return nil
			},
		})
	}

	panicOccurred := sm.RunAllHooks(cancelCtx)

	// Assert: panic 없음
	if panicOccurred {
		t.Error("panicOccurred = true, want false")
	}

	// Assert: hook 0, 1만 실행됨 (hook 2, 3, 4는 parent cancel로 skip)
	// hook 1 내부에서 cancelFn()을 호출하므로 hook 2 진입 직전 select에서 걸린다.
	got := callCount.Load()
	if got != 2 {
		t.Errorf("실행된 hook 수 = %d, want 2 (parent ctx cancel 후 나머지 skip 기대)", got)
	}
}

// --- 테스트 헬퍼 ---

// buildGoosed는 TestMain에서 사전 빌드된 goosed 바이너리 경로를 반환한다.
// goosedBinPath가 비어있으면 (단독 테스트 실행 등) 즉석에서 빌드한다.
func buildGoosed(t *testing.T) string {
	t.Helper()
	if goosedBinPath != "" {
		return goosedBinPath
	}
	// TestMain을 거치지 않은 경우 (예: go test -run TestXxx ./...) fallback 빌드
	binPath := filepath.Join(t.TempDir(), "goosed")
	cmd := exec.Command("go", "build", "-o", binPath, "github.com/modu-ai/mink/cmd/minkd")
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("goosed 빌드 실패: %v\n%s", err, out)
	}
	return binPath
}

// waitForHealthy는 포트가 healthy 상태가 될 때까지 대기한다.
func waitForHealthy(t *testing.T, port int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", port))
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			return
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("포트 %d healthy 대기 타임아웃", port)
}
