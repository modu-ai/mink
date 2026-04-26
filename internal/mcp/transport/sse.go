package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

// sseBase는 SSETransport의 내부 상태이다.
type sseBase struct {
	ctx       context.Context
	cancel    context.CancelFunc
	baseURI   string
	client    *http.Client
	logger    *zap.Logger
	pending   sync.Map // map[int]*pendingRequest
	handlers  []func(Message)
	handlerMu sync.RWMutex
	nextID    atomic.Int64
	closed    atomic.Bool
	done      chan struct{}
}

// NewSSETransport는 SSE transport를 생성한다.
// REQ-MCP-020: server-initiated notification을 message event stream으로 수신
//
// @MX:ANCHOR: [AUTO] NewSSETransport — SSE transport 생성자
// @MX:REASON: REQ-MCP-020, AC-MCP-020 — SSE server-initiated notification의 단일 진입점. fan_in >= 3
func NewSSETransport(ctx context.Context, uri string, tlsCfg *TLSConfig, logger *zap.Logger) (*SSETransport, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	childCtx, cancel := context.WithCancel(ctx)

	if tlsCfg != nil && tlsCfg.Insecure {
		logger.Warn("mcp-sse: TLS verification disabled (insecure mode)")
	}

	b := &sseBase{
		ctx:     childCtx,
		cancel:  cancel,
		baseURI: uri,
		client:  &http.Client{},
		logger:  logger,
		done:    make(chan struct{}),
	}

	t := &SSETransport{inner: b}

	// SSE event stream 구독 goroutine
	// @MX:WARN: [AUTO] SSE event stream goroutine — context cancel 시 종료됨
	// @MX:REASON: REQ-MCP-020 — SSE server-initiated notification 수신 루프. done channel로 종료 추적
	go b.subscribeLoop()

	return t, nil
}

// subscribeLoop은 SSE event stream을 구독하여 server-initiated notification을 처리한다.
// REQ-MCP-020: message event stream에서 알림을 수신하여 OnMessage 핸들러에 dispatch
func (b *sseBase) subscribeLoop() {
	defer close(b.done)

	req, err := http.NewRequestWithContext(b.ctx, http.MethodGet, b.baseURI, nil)
	if err != nil {
		b.logger.Error("mcp-sse: failed to create SSE request", zap.Error(err))
		return
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := b.client.Do(req)
	if err != nil {
		if b.ctx.Err() != nil {
			return
		}
		b.logger.Error("mcp-sse: SSE subscription failed", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	var eventType, data string

	for scanner.Scan() {
		if b.ctx.Err() != nil {
			return
		}

		line := scanner.Text()

		if line == "" {
			if data != "" {
				b.dispatchSSEEvent(eventType, data)
			}
			eventType = ""
			data = ""
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
}

// dispatchSSEEvent는 SSE 이벤트를 파싱하여 핸들러에 dispatch한다.
func (b *sseBase) dispatchSSEEvent(_, data string) {
	var msg Message
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		b.logger.Error("mcp-sse: failed to unmarshal SSE event data", zap.Error(err))
		return
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
		// REQ-MCP-020: server-initiated notification dispatch
		b.handlerMu.RLock()
		handlers := make([]func(Message), len(b.handlers))
		copy(handlers, b.handlers)
		b.handlerMu.RUnlock()
		for _, h := range handlers {
			h(msg)
		}
	}
}

// SendRequest는 HTTP POST를 통해 JSON-RPC 요청을 전송한다.
func (t *SSETransport) SendRequest(ctx context.Context, req Request) (Response, error) {
	b := t.inner
	if b == nil || b.closed.Load() {
		return Response{}, ErrTransportClosed
	}

	id := int(b.nextID.Add(1))
	req.ID = id
	req.JSONRPC = JSONRPCVersion

	ch := make(chan Response, 1)
	b.pending.Store(id, &pendingRequest{ch: ch})
	defer b.pending.Delete(id)

	data, err := json.Marshal(req)
	if err != nil {
		return Response{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURI, bytes.NewReader(data))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	// 동기 JSON 응답 처리
	if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		var jsonResp Response
		if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
			return Response{}, fmt.Errorf("decode SSE response: %w", err)
		}
		return jsonResp, nil
	}

	// SSE 스트림에서 응답 대기
	_, _ = io.ReadAll(resp.Body) // body 소비

	select {
	case r := <-ch:
		return r, nil
	case <-ctx.Done():
		return Response{}, ctx.Err()
	case <-b.done:
		return Response{}, ErrTransportClosed
	}
}

// Notify는 알림 메시지를 HTTP POST로 전송한다.
func (t *SSETransport) Notify(ctx context.Context, msg Notification) error {
	b := t.inner
	if b == nil || b.closed.Load() {
		return ErrTransportClosed
	}
	msg.JSONRPC = JSONRPCVersion
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURI, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("notify POST: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	return nil
}

// OnMessage는 서버 발송 메시지 핸들러를 등록한다.
// REQ-MCP-020
func (t *SSETransport) OnMessage(handler func(Message)) {
	if t.inner == nil {
		return
	}
	t.inner.handlerMu.Lock()
	defer t.inner.handlerMu.Unlock()
	t.inner.handlers = append(t.inner.handlers, handler)
}

// Close는 SSE 스트림 구독을 중단하고 transport를 닫는다.
func (t *SSETransport) Close() error {
	b := t.inner
	if b == nil {
		return nil
	}

	if !b.closed.CompareAndSwap(false, true) {
		return nil
	}

	b.cancel()
	<-b.done
	return nil
}
