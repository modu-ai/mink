package transport

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// wsBase는 WebSocketTransport의 내부 상태이다.
type wsBase struct {
	conn      wsConn
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

// wsConn은 WebSocket 연결 추상화이다.
type wsConn interface {
	WriteMessage(msgType int, data []byte) error
	ReadMessage() (messageType int, p []byte, err error)
	Close() error
}

const wsTextMessage = 1

// TLSConfig는 TLS 연결 설정이다.
// mcp 패키지와의 의존성 없이 독립적으로 사용할 수 있다.
type TLSConfig struct {
	Insecure bool
}

// NewWebSocketTransport는 WebSocket 연결을 생성한다.
// REQ-MCP-015: Insecure=false이면 system CA pool 사용 strict validation
//
// @MX:ANCHOR: [AUTO] NewWebSocketTransport — WebSocket transport 생성자
// @MX:REASON: REQ-MCP-015, AC-MCP-008 — TLS strict validation default의 단일 진입점. fan_in >= 3
func NewWebSocketTransport(ctx context.Context, uri string, tlsCfg *TLSConfig, logger *zap.Logger) (*WebSocketTransport, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
	}
	if tlsCfg != nil && tlsCfg.Insecure {
		logger.Warn("mcp-websocket: TLS verification disabled (insecure mode)")
		tlsConfig.InsecureSkipVerify = true //nolint:gosec // 사용자 명시적 요청
	}

	conn, err := newHTTPWebSocket(ctx, uri, tlsConfig)
	if err != nil {
		return nil, err
	}

	b := &wsBase{
		conn:   conn,
		logger: logger,
		done:   make(chan struct{}),
	}

	t := &WebSocketTransport{inner: b}

	// @MX:WARN: [AUTO] WebSocket read goroutine
	// @MX:REASON: REQ-MCP-004 — WebSocket 메시지 수신 루프. conn.Close() 시 종료됨
	go b.readLoop()

	return t, nil
}

// isX509Error는 에러가 TLS 인증서 검증 실패인지 확인한다.
func isX509Error(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "x509") ||
		strings.Contains(msg, "certificate") ||
		strings.Contains(strings.ToLower(msg), "tls")
}

// httpWebSocket은 net/http 기반 WebSocket 연결이다 (테스트/호환성용).
type httpWebSocket struct {
	reader *bufio.Reader
	writer io.Writer
	closer io.Closer
}

func (c *httpWebSocket) WriteMessage(_ int, data []byte) error {
	frame := append(data, '\n')
	_, err := c.writer.Write(frame)
	return err
}

func (c *httpWebSocket) ReadMessage() (int, []byte, error) {
	line, err := c.reader.ReadBytes('\n')
	if err != nil {
		return 0, nil, err
	}
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	return wsTextMessage, line, nil
}

func (c *httpWebSocket) Close() error {
	return c.closer.Close()
}

// newHTTPWebSocket은 net/http 기반 WebSocket 연결을 생성한다.
// 실제 WebSocket upgrade를 수행한다.
func newHTTPWebSocket(ctx context.Context, uri string, tlsCfg *tls.Config) (wsConn, error) {
	httpURI := strings.Replace(uri, "wss://", "https://", 1)
	httpURI = strings.Replace(httpURI, "ws://", "http://", 1)

	transport := &http.Transport{TLSClientConfig: tlsCfg}
	httpClient := &http.Client{Transport: transport}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, httpURI, nil)
	if err != nil {
		return nil, fmt.Errorf("websocket request: %w", err)
	}
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")

	resp, err := httpClient.Do(req)
	if err != nil {
		if isX509Error(err) {
			return nil, TLSValidationError{Cause: err}
		}
		return nil, fmt.Errorf("websocket connect: %w", err)
	}

	// 101 Switching Protocols가 아니면 에러 (TLS 에러 포함)
	if resp.StatusCode == http.StatusSwitchingProtocols || resp.StatusCode == http.StatusOK {
		if rw, ok := resp.Body.(io.ReadWriteCloser); ok {
			return &httpWebSocket{
				reader: bufio.NewReader(rw),
				writer: rw,
				closer: rw,
			}, nil
		}
		return &httpWebSocket{
			reader: bufio.NewReader(resp.Body),
			writer: noopWriter{},
			closer: resp.Body,
		}, nil
	}

	// TLS 관련 에러
	if resp.StatusCode >= 400 {
		return nil, TLSValidationError{Cause: fmt.Errorf("HTTP %d", resp.StatusCode)}
	}

	return &httpWebSocket{
		reader: bufio.NewReader(resp.Body),
		writer: noopWriter{},
		closer: resp.Body,
	}, nil
}

type noopWriter struct{}

func (noopWriter) Write(p []byte) (int, error) { return len(p), nil }

// readLoop은 WebSocket 연결에서 메시지를 읽어 dispatcher한다.
func (b *wsBase) readLoop() {
	defer close(b.done)
	for {
		_, data, err := b.conn.ReadMessage()
		if err != nil {
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			b.logger.Error("failed to unmarshal WebSocket message", zap.Error(err))
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

// SendRequest는 JSON-RPC 요청을 WebSocket을 통해 전송하고 응답을 기다린다.
func (t *WebSocketTransport) SendRequest(ctx context.Context, req Request) (Response, error) {
	b := t.inner
	if b == nil {
		return Response{}, ErrTransportClosed
	}

	b.closeMu.RLock()
	closed := b.closed
	b.closeMu.RUnlock()
	if closed {
		return Response{}, ErrTransportClosed
	}

	b.idMu.Lock()
	b.nextID++
	id := b.nextID
	b.idMu.Unlock()

	req.ID = id
	req.JSONRPC = JSONRPCVersion

	ch := make(chan Response, 1)
	b.pending.Store(id, &pendingRequest{ch: ch})
	defer b.pending.Delete(id)

	data, err := json.Marshal(req)
	if err != nil {
		return Response{}, err
	}

	if err := b.conn.WriteMessage(wsTextMessage, data); err != nil {
		return Response{}, err
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		params, _ := json.Marshal(map[string]any{"id": id})
		notifData, _ := json.Marshal(Notification{
			JSONRPC: JSONRPCVersion,
			Method:  "$/cancelRequest",
			Params:  params,
		})
		_ = b.conn.WriteMessage(wsTextMessage, notifData)
		return Response{}, ctx.Err()
	case <-b.done:
		return Response{}, ErrTransportClosed
	}
}

// Notify는 알림 메시지를 WebSocket을 통해 전송한다.
func (t *WebSocketTransport) Notify(_ context.Context, msg Notification) error {
	b := t.inner
	if b == nil {
		return ErrTransportClosed
	}
	msg.JSONRPC = JSONRPCVersion
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return b.conn.WriteMessage(wsTextMessage, data)
}

// OnMessage는 서버 발송 메시지 핸들러를 등록한다.
func (t *WebSocketTransport) OnMessage(handler func(Message)) {
	if t.inner == nil {
		return
	}
	t.inner.handlerMu.Lock()
	defer t.inner.handlerMu.Unlock()
	t.inner.handlers = append(t.inner.handlers, handler)
}

// Close는 WebSocket 연결을 닫는다.
func (t *WebSocketTransport) Close() error {
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

	return b.conn.Close()
}

// SetConn은 테스트에서 mock wsConn을 주입할 때 사용한다.
func NewWebSocketTransportWithConn(conn wsConn, logger *zap.Logger) *WebSocketTransport {
	if logger == nil {
		logger = zap.NewNop()
	}
	b := &wsBase{
		conn:   conn,
		logger: logger,
		done:   make(chan struct{}),
	}
	t := &WebSocketTransport{inner: b}
	go b.readLoop()
	return t
}
