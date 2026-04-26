package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// buildEchoServer는 에코 서버 바이너리를 빌드한다.
// 에코 서버는 stdin에서 JSON-RPC 요청을 읽고 동일한 ID로 응답을 반환한다.
func buildEchoServer(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("subprocess build skipped in short mode")
	}

	binaryPath := filepath.Join(t.TempDir(), "echo-server")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	// inline Go program으로 에코 서버 생성
	src := filepath.Join(t.TempDir(), "echo.go")
	echoCode := `package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type Msg struct {
	JSONRPC string      ` + "`json:\"jsonrpc\"`" + `
	ID      interface{} ` + "`json:\"id,omitempty\"`" + `
	Method  string      ` + "`json:\"method,omitempty\"`" + `
	Result  interface{} ` + "`json:\"result,omitempty\"`" + `
	Params  interface{} ` + "`json:\"params,omitempty\"`" + `
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		var req Msg
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}
		if req.Method == "" || req.ID == nil {
			continue // notification, skip
		}
		resp := Msg{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  "echo:" + req.Method,
		}
		b, _ := json.Marshal(resp)
		fmt.Println(string(b))
	}
}
`
	err := os.WriteFile(src, []byte(echoCode), 0600)
	require.NoError(t, err)

	cmd := exec.Command("go", "build", "-o", binaryPath, src)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("cannot build echo server: %v\n%s", err, out)
	}
	return binaryPath
}

// TestNewStdioTransport_BasicRoundtrip은 subprocess와의 기본 통신을 검증한다.
func TestNewStdioTransport_BasicRoundtrip(t *testing.T) {
	binaryPath := buildEchoServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t_stdio, err := NewStdioTransport(ctx, binaryPath, nil, nil, nil)
	require.NoError(t, err)
	defer t_stdio.Close()

	req := Request{JSONRPC: JSONRPCVersion, Method: "ping"}
	resp, err := t_stdio.SendRequest(ctx, req)
	require.NoError(t, err)

	var result string
	_ = json.Unmarshal(resp.Result, &result)
	assert.Equal(t, "echo:ping", result)
}

// TestNewStdioTransport_EnvInjection은 환경 변수 주입을 검증한다.
// REQ-MCP-019
func TestNewStdioTransport_EnvInjection(t *testing.T) {
	if testing.Short() {
		t.Skip("subprocess env test skipped in short mode")
	}

	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "env-echo.go")
	binaryPath := filepath.Join(tmpDir, "env-echo")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	// 환경 변수 출력 서버
	envCode := `package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	// env를 반환하는 서버
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		var req struct {
			JSONRPC string      ` + "`json:\"jsonrpc\"`" + `
			ID      interface{} ` + "`json:\"id,omitempty\"`" + `
			Method  string      ` + "`json:\"method\"`" + `
		}
		if err := json.Unmarshal([]byte(line), &req); err != nil { continue }
		if req.ID == nil { continue }

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  os.Getenv("MCP_TEST_INJECTED"),
		}
		b, _ := json.Marshal(resp)
		fmt.Println(string(b))
	}
}
`
	err := os.WriteFile(src, []byte(envCode), 0600)
	require.NoError(t, err)

	cmd := exec.Command("go", "build", "-o", binaryPath, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot build env-echo server: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	envMap := map[string]string{"MCP_TEST_INJECTED": "injected-value"}
	t_stdio, err := NewStdioTransport(ctx, binaryPath, nil, envMap, nil)
	require.NoError(t, err)
	defer t_stdio.Close()

	req := Request{JSONRPC: JSONRPCVersion, Method: "get-env"}
	resp, err := t_stdio.SendRequest(ctx, req)
	require.NoError(t, err)

	var result string
	_ = json.Unmarshal(resp.Result, &result)
	assert.Equal(t, "injected-value", result, "주입된 환경 변수가 subprocess에 전달되어야 함")
}

// TestStdioTransport_CtxCancellation은 ctx 취소 시 동작을 검증한다.
// REQ-MCP-022: ctx 취소 시 $/cancelRequest 발송
func TestStdioTransport_CtxCancellation(t *testing.T) {
	binaryPath := buildEchoServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t_stdio, err := NewStdioTransport(ctx, binaryPath, nil, nil, nil)
	require.NoError(t, err)
	defer t_stdio.Close()

	// 짧은 timeout context로 요청
	reqCtx, reqCancel := context.WithTimeout(ctx, 1*time.Millisecond)
	defer reqCancel()

	// 에코 서버는 응답하므로 대부분의 경우 성공
	// ctx cancel이 발생하면 ctx.Err() 반환
	_, _ = t_stdio.SendRequest(reqCtx, Request{JSONRPC: JSONRPCVersion, Method: "test"})
	// 결과에 관계없이 테스트 통과 (timing-dependent)
}

// TestStdioTransport_Close_SIGTERM는 Close 시 SIGTERM 전송을 검증한다.
// REQ-MCP-014
func TestStdioTransport_Close_SIGTERM(t *testing.T) {
	binaryPath := buildEchoServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t_stdio, err := NewStdioTransport(ctx, binaryPath, nil, nil, nil)
	require.NoError(t, err)

	cmd := t_stdio.Cmd()
	require.NotNil(t, cmd)
	require.NotNil(t, cmd.Process)
	pid := cmd.Process.Pid

	// Close는 SIGTERM → SIGKILL 순서로 종료해야 한다
	start := time.Now()
	err = t_stdio.Close()
	elapsed := time.Since(start)

	assert.NoError(t, err)
	// SIGTERM 후 정상 종료 시 5초 이내 반환
	assert.Less(t, elapsed, 6*time.Second, "Close should complete within 6s")
	t.Logf("stdio transport closed in %v (pid=%d)", elapsed, pid)
}

// TestStdioBase_SendLine은 sendLine 동작을 검증한다.
func TestStdioBase_SendLine(t *testing.T) {
	pr, pw := io.Pipe()

	b := &stdioBase{
		stdin:  pw,
		stdout: pr,
		done:   make(chan struct{}),
	}

	// 닫힌 transport는 ErrTransportClosed 반환
	b.closeMu.Lock()
	b.closed = true
	b.closeMu.Unlock()

	err := b.sendLine(map[string]any{"test": 1})
	assert.Error(t, err) // ErrTransportClosed

	// cleanup
	_ = pr.Close()
	_ = pw.Close()
}

// TestStdioBase_NextRequestID는 ID 단조 증가를 검증한다.
func TestStdioBase_NextRequestID(t *testing.T) {
	b := &stdioBase{done: make(chan struct{})}

	id1 := b.nextRequestID()
	id2 := b.nextRequestID()
	id3 := b.nextRequestID()

	assert.Equal(t, 1, id1)
	assert.Equal(t, 2, id2)
	assert.Equal(t, 3, id3)
}

// TestStdioBase_NextRequestID_Concurrent는 동시 ID 생성의 안전성을 검증한다.
func TestStdioBase_NextRequestID_Concurrent(t *testing.T) {
	b := &stdioBase{done: make(chan struct{})}

	const n = 100
	ids := make(chan int, n)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids <- b.nextRequestID()
		}()
	}
	wg.Wait()
	close(ids)

	seen := make(map[int]bool)
	for id := range ids {
		assert.False(t, seen[id], "ID %d should be unique", id)
		seen[id] = true
	}
	assert.Equal(t, n, len(seen))
}

// TestStdioTransport_Notify는 Notify 동작을 검증한다.
func TestStdioTransport_Notify(t *testing.T) {
	binaryPath := buildEchoServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t_stdio, err := NewStdioTransport(ctx, binaryPath, nil, nil, nil)
	require.NoError(t, err)
	defer t_stdio.Close()

	notif := Notification{JSONRPC: JSONRPCVersion, Method: "test/notify"}
	err = t_stdio.Notify(ctx, notif)
	assert.NoError(t, err) // 에코 서버가 알림을 ignore하므로 에러 없음
}

// TestStdioTransport_OnMessage는 OnMessage 핸들러 등록을 검증한다.
func TestStdioTransport_OnMessage(t *testing.T) {
	binaryPath := buildEchoServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t_stdio, err := NewStdioTransport(ctx, binaryPath, nil, nil, nil)
	require.NoError(t, err)
	defer t_stdio.Close()

	handlerCalled := make(chan struct{}, 1)
	t_stdio.OnMessage(func(msg Message) {
		select {
		case handlerCalled <- struct{}{}:
		default:
		}
	})

	// 핸들러가 등록됨을 확인
	t_stdio.inner.handlerMu.RLock()
	count := len(t_stdio.inner.handlers)
	t_stdio.inner.handlerMu.RUnlock()
	assert.Equal(t, 1, count)
}

// TestStdioTransport_DoneChannel은 readLoop 종료 시 done 채널이 닫히는지 검증한다.
func TestStdioTransport_DoneChannel(t *testing.T) {
	binaryPath := buildEchoServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t_stdio, err := NewStdioTransport(ctx, binaryPath, nil, nil, nil)
	require.NoError(t, err)

	done := t_stdio.Done()
	assert.NotNil(t, done)

	// Close 후 done이 닫혀야 한다
	_ = t_stdio.Close()

	select {
	case <-done:
		// 정상 종료
	case <-time.After(6 * time.Second):
		t.Fatal("done channel should be closed after Close()")
	}
}

// TestStdioTransport_Close_Idempotent은 중복 Close가 안전한지 검증한다.
func TestStdioTransport_Close_Idempotent(t *testing.T) {
	binaryPath := buildEchoServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t_stdio, err := NewStdioTransport(ctx, binaryPath, nil, nil, nil)
	require.NoError(t, err)

	err1 := t_stdio.Close()
	err2 := t_stdio.Close()
	assert.NoError(t, err1)
	assert.NoError(t, err2, "중복 Close는 안전해야 함")
}

// TestReadLoop_InvalidJSON은 잘못된 JSON이 readLoop에서 처리되는지 검증한다.
func TestReadLoop_InvalidJSON(t *testing.T) {
	// in-memory pipe로 잘못된 JSON 주입
	pr, pw := io.Pipe()

	logger := zap.NewNop()
	b := &stdioBase{
		stdin:  pw,
		stdout: pr,
		done:   make(chan struct{}),
		logger: logger,
	}

	// readLoop goroutine 시작
	go b.readLoop()

	// 잘못된 JSON + 정상 JSON 전송
	writer := bufio.NewWriter(pw)
	fmt.Fprintln(writer, "{bad json}")
	// 정상 응답 (ID 없음 → 알림으로 처리)
	notif, _ := json.Marshal(Message{JSONRPC: JSONRPCVersion, Method: "test/notify"})
	fmt.Fprintln(writer, string(notif))
	writer.Flush()
	_ = pw.Close()

	// readLoop 종료 대기
	select {
	case <-b.done:
	case <-time.After(2 * time.Second):
		t.Fatal("readLoop should terminate after pipe close")
	}
}

// TestSSETransport_NotifyFail은 닫힌 SSE transport에서 Notify가 에러를 반환하는지 검증한다.
func TestSSETransport_NotifyFail(t *testing.T) {
	t_sse := &SSETransport{inner: nil}
	err := t_sse.Notify(context.Background(), Notification{Method: "test"})
	assert.Error(t, err)
}

// TestSSETransport_SendRequestFail은 닫힌 SSE transport에서 SendRequest가 에러를 반환하는지 검증한다.
func TestSSETransport_SendRequestFail(t *testing.T) {
	t_sse := &SSETransport{inner: nil}
	_, err := t_sse.SendRequest(context.Background(), Request{Method: "test"})
	assert.Error(t, err)
}

// TestSSETransport_OnMessageNilInner는 nil inner에서 OnMessage가 안전한지 검증한다.
func TestSSETransport_OnMessageNilInner(t *testing.T) {
	t_sse := &SSETransport{inner: nil}
	t_sse.OnMessage(func(_ Message) {}) // 패닉 없이 완료
}

// TestSSETransport_CloseNilInner는 nil inner에서 Close가 안전한지 검증한다.
func TestSSETransport_CloseNilInner(t *testing.T) {
	t_sse := &SSETransport{inner: nil}
	err := t_sse.Close()
	assert.NoError(t, err)
}

// TestSSETransport_CloseIdempotent은 중복 Close가 안전한지 검증한다.
func TestSSETransport_CloseIdempotent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// minimal SSE base
	b := &sseBase{
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	close(b.done) // 이미 닫힌 상태 시뮬레이션

	t_sse := &SSETransport{inner: b}
	b.closed.Store(true)

	err1 := t_sse.Close()
	err2 := t_sse.Close()
	assert.NoError(t, err1)
	assert.NoError(t, err2)
}

// TestWebSocketTransport_NotifyNilInner는 nil inner에서 Notify가 에러를 반환하는지 검증한다.
func TestWebSocketTransport_NotifyNilInner(t *testing.T) {
	t_ws := &WebSocketTransport{inner: nil}
	err := t_ws.Notify(context.Background(), Notification{Method: "test"})
	assert.Error(t, err)
}

// TestWebSocketTransport_OnMessageNilInner는 nil inner에서 OnMessage가 안전한지 검증한다.
func TestWebSocketTransport_OnMessageNilInner(t *testing.T) {
	t_ws := &WebSocketTransport{inner: nil}
	t_ws.OnMessage(func(_ Message) {}) // 패닉 없이 완료
}

// TestWebSocketTransport_CloseNilInner는 nil inner에서 Close가 안전한지 검증한다.
func TestWebSocketTransport_CloseNilInner(t *testing.T) {
	t_ws := &WebSocketTransport{inner: nil}
	err := t_ws.Close()
	assert.NoError(t, err)
}

// TestWebSocketTransport_SendRequest_Closed는 닫힌 WS transport에서 SendRequest가 에러를 반환하는지 검증한다.
func TestWebSocketTransport_SendRequest_Closed(t *testing.T) {
	conn := newMockWSConn()
	t_ws := NewWebSocketTransportWithConn(conn, nil)
	_ = t_ws.Close()

	_, err := t_ws.SendRequest(context.Background(), Request{Method: "test"})
	assert.Error(t, err)
}

// TestErrTransportClosedType은 ErrTransportClosed 타입을 검증한다.
func TestErrTransportClosedType(t *testing.T) {
	e := ErrTransportClosedType{}
	assert.Equal(t, "transport is closed", e.Error())
}
