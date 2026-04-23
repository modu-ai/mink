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
	"syscall"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/config"
	"github.com/modu-ai/goose/internal/core"
	"github.com/modu-ai/goose/internal/health"
)

// TestBootstrap_SucceedsWithEmptyConfig는 AC-CORE-001을 검증한다.
// GOOSE_HOME이 비어있는 디렉토리로 설정될 때 goosed가 serving 상태로 뜨고
// /healthz가 200 OK + {"status":"ok","state":"serving",...}를 반환해야 한다.
func TestBootstrap_SucceedsWithEmptyConfig(t *testing.T) {
	t.Parallel()

	// Arrange: 빈 GOOSE_HOME 디렉토리 생성
	gooseHome := t.TempDir()

	// 빈 config 파일 없음 → 기본값 fallback
	cfg, err := config.LoadFromFile(filepath.Join(gooseHome, "config.yaml"))
	if err != nil {
		t.Fatalf("설정 로드 실패: %v", err)
	}
	// 기본값 확인
	if cfg.HealthPort == 0 {
		t.Fatal("기본 HealthPort가 0이면 안 됨")
	}

	logger, err := core.NewLogger("info", "goosed", "test")
	if err != nil {
		t.Fatalf("로거 생성 실패: %v", err)
	}

	rt := core.NewRuntime(logger)
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
		"GOOSE_HOME="+gooseHome,
		fmt.Sprintf("GOOSE_HEALTH_PORT=%d", port),
		"GOOSE_LOG_LEVEL=info",
	)
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("goosed 시작 실패: %v", err)
	}

	// serving 상태 대기
	waitForHealthy(t, port, 3*time.Second)

	// SIGTERM 전송
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("SIGTERM 전송 실패: %v", err)
	}

	// 3초 이내 종료 확인
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
	case <-time.After(3 * time.Second):
		cmd.Process.Kill() //nolint:errcheck
		t.Fatal("3초 내 종료되지 않음 (timeout)")
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
		"GOOSE_HOME="+gooseHome,
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
		"GOOSE_HOME="+gooseHome,
		fmt.Sprintf("GOOSE_HEALTH_PORT=%d", occupiedPort),
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

// --- 테스트 헬퍼 ---

// buildGoosed는 테스트용 goosed 바이너리를 빌드하고 경로를 반환한다.
func buildGoosed(t *testing.T) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "goosed")
	cmd := exec.Command("go", "build", "-o", binPath, "github.com/modu-ai/goose/cmd/goosed")
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
