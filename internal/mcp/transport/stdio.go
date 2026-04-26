package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// pendingRequest는 pending 요청의 응답 채널이다.
type pendingRequest struct {
	ch chan Response
}

// stdioBase는 StdioTransport의 내부 상태이다.
type stdioBase struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	logger    *zap.Logger
	pending   sync.Map // map[int]*pendingRequest
	handlers  []func(Message)
	handlerMu sync.RWMutex
	nextID    int
	idMu      sync.Mutex
	closed    bool
	closeMu   sync.RWMutex
	done      chan struct{}
}

// NewStdioTransport는 subprocess를 spawn하여 stdio transport를 생성한다.
// REQ-MCP-005: command + args로 subprocess 기동, stdin/stdout 파이프
// REQ-MCP-019: env가 비어있지 않으면 부모 환경 위에 merge
//
// @MX:ANCHOR: [AUTO] NewStdioTransport — stdio subprocess transport 생성자
// @MX:REASON: REQ-MCP-005, REQ-MCP-014 — subprocess 생명주기와 transport의 단일 진입점. fan_in >= 3
func NewStdioTransport(ctx context.Context, command string, args []string, env map[string]string, logger *zap.Logger) (*StdioTransport, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	cmd := exec.CommandContext(ctx, command, args...)

	// REQ-MCP-019: 부모 환경 inherit 후 merge
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// subprocess가 별도 process group으로 실행되도록 설정
	// REQ-MCP-014: process group kill로 자식 프로세스 정리
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("subprocess start: %w", err)
	}

	b := &stdioBase{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		logger: logger,
		done:   make(chan struct{}),
	}

	t := &StdioTransport{inner: b}

	// stderr → zap 포워드 goroutine
	// @MX:WARN: [AUTO] subprocess stderr forward goroutine
	// @MX:REASON: REQ-MCP-005 — 로그 포워드. scanner.Scan()이 EOF에 도달하면 자동 종료
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			logger.Debug("mcp-subprocess stderr", zap.String("line", scanner.Text()))
		}
	}()

	// stdout → JSON-RPC dispatcher goroutine
	// @MX:WARN: [AUTO] subprocess stdout read goroutine
	// @MX:REASON: REQ-MCP-005 — subprocess stdout에서 JSON-RPC 메시지를 읽는 루프. done channel로 종료 추적
	go b.readLoop()

	return t, nil
}

// readLoop은 subprocess stdout에서 JSON-RPC 메시지를 읽어 dispatcher한다.
func (b *stdioBase) readLoop() {
	defer close(b.done)
	scanner := bufio.NewScanner(b.stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			b.logger.Error("failed to unmarshal MCP message", zap.Error(err))
			continue
		}

		if msg.IsResponse() {
			if ch, ok := b.pending.Load(normalizeID(msg.ID)); ok {
				resp := Response{
					JSONRPC: msg.JSONRPC,
					ID:      msg.ID,
					Result:  msg.Result,
					Error:   msg.Error,
				}
				ch.(*pendingRequest).ch <- resp
			}
		} else {
			b.handlerMu.RLock()
			handlers := make([]func(Message), len(b.handlers))
			copy(handlers, b.handlers)
			b.handlerMu.RUnlock()
			for _, h := range handlers {
				h(msg)
			}
		}
	}
}

// normalizeID는 JSON 언마샬 시 float64로 변환된 numeric ID를 정수로 정규화한다.
func normalizeID(id any) any {
	switch v := id.(type) {
	case float64:
		return int(v)
	default:
		return id
	}
}

// sendLine은 JSON-RPC 메시지를 stdin에 line-delimited JSON으로 전송한다.
func (b *stdioBase) sendLine(v any) error {
	b.closeMu.RLock()
	closed := b.closed
	b.closeMu.RUnlock()
	if closed {
		return ErrTransportClosed
	}

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	data = append(data, '\n')
	_, err = b.stdin.Write(data)
	return err
}

// nextRequestID는 새 요청 ID를 생성한다.
func (b *stdioBase) nextRequestID() int {
	b.idMu.Lock()
	defer b.idMu.Unlock()
	b.nextID++
	return b.nextID
}

// sendCancelRequest는 $/cancelRequest 알림을 전송한다.
// REQ-MCP-022
func (b *stdioBase) sendCancelRequest(id any) error {
	params, _ := json.Marshal(map[string]any{"id": id})
	notif := Notification{
		JSONRPC: JSONRPCVersion,
		Method:  "$/cancelRequest",
		Params:  params,
	}
	return b.sendLine(notif)
}

// SendRequest는 JSON-RPC 요청을 stdin에 전송하고 응답을 기다린다.
// REQ-MCP-022: ctx 취소/deadline 시 $/cancelRequest 발송
func (t *StdioTransport) SendRequest(ctx context.Context, req Request) (Response, error) {
	b := t.inner
	if b == nil {
		return Response{}, ErrTransportClosed
	}

	id := b.nextRequestID()
	req.ID = id
	req.JSONRPC = JSONRPCVersion

	ch := make(chan Response, 1)
	b.pending.Store(id, &pendingRequest{ch: ch})
	defer b.pending.Delete(id)

	if err := b.sendLine(req); err != nil {
		return Response{}, err
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		_ = b.sendCancelRequest(id)
		return Response{}, ctx.Err()
	case <-b.done:
		return Response{}, ErrTransportClosed
	}
}

// Notify는 알림 메시지를 stdin에 전송한다.
func (t *StdioTransport) Notify(ctx context.Context, msg Notification) error {
	if t.inner == nil {
		return ErrTransportClosed
	}
	msg.JSONRPC = JSONRPCVersion
	return t.inner.sendLine(msg)
}

// OnMessage는 서버 발송 메시지 핸들러를 등록한다.
func (t *StdioTransport) OnMessage(handler func(Message)) {
	if t.inner == nil {
		return
	}
	t.inner.handlerMu.Lock()
	defer t.inner.handlerMu.Unlock()
	t.inner.handlers = append(t.inner.handlers, handler)
}

// Close는 transport를 닫는다.
// REQ-MCP-014: SIGTERM → 5s grace → SIGKILL
func (t *StdioTransport) Close() error {
	b := t.inner
	if b == nil {
		return nil
	}

	b.closeMu.Lock()
	if b.closed {
		b.closeMu.Unlock()
		return nil
	}
	b.closed = true
	b.closeMu.Unlock()

	_ = b.stdin.Close()

	if b.cmd != nil && b.cmd.Process != nil {
		b.logger.Debug("mcp-stdio: sending SIGTERM", zap.Int("pid", b.cmd.Process.Pid))
		_ = b.cmd.Process.Signal(syscall.SIGTERM)

		done := make(chan error, 1)
		go func() { done <- b.cmd.Wait() }()

		select {
		case <-done:
			b.logger.Debug("mcp-stdio: subprocess exited after SIGTERM")
		case <-time.After(5 * time.Second):
			b.logger.Debug("mcp-stdio: sending SIGKILL after 5s grace",
				zap.String("event", "sigkill_sent"))
			_ = b.cmd.Process.Kill()
			<-done
		}
	}

	return nil
}

// Cmd는 내부 exec.Cmd를 반환한다 (테스트 전용).
func (t *StdioTransport) Cmd() *exec.Cmd {
	if t.inner == nil {
		return nil
	}
	return t.inner.cmd
}

// Done은 readLoop이 종료될 때 닫히는 채널이다 (테스트 전용).
func (t *StdioTransport) Done() <-chan struct{} {
	if t.inner == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return t.inner.done
}
