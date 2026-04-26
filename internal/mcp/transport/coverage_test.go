package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTLSValidationError_Methods는 TLSValidationError 메서드를 검증한다.
func TestTLSValidationError_Methods(t *testing.T) {
	cause := fmt.Errorf("x509 error")
	err := TLSValidationError{Cause: cause}

	assert.Contains(t, err.Error(), "TLS certificate validation failed")
	assert.Equal(t, cause, err.Unwrap())

	// nil cause
	errNil := TLSValidationError{}
	assert.Equal(t, "TLS certificate validation failed", errNil.Error())
	assert.Nil(t, errNil.Unwrap())
}

// TestError_ErrorMethod는 transport.Error.Error() 메서드를 검증한다.
func TestError_ErrorMethod(t *testing.T) {
	e := &Error{Code: ErrCodeInternal, Message: "test error"}
	assert.Equal(t, "test error", e.Error())
}

// TestSSETransport_SendRequest_JSON는 SSE POST + JSON 응답 경로를 검증한다.
func TestSSETransport_SendRequest_JSON(t *testing.T) {
	var requestCount int32 // atomic

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		if r.Header.Get("Accept") == "text/event-stream" {
			// SSE 구독 요청 - 연결 유지
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			<-r.Context().Done()
			return
		}

		// POST 요청 처리 - JSON 응답
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Result:  json.RawMessage(`"sse-result"`),
		})
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	t_sse, err := NewSSETransport(ctx, srv.URL, nil, nil)
	require.NoError(t, err)
	defer t_sse.Close()

	// 잠시 대기하여 구독 goroutine이 시작되도록 함
	time.Sleep(50 * time.Millisecond)

	req := Request{JSONRPC: JSONRPCVersion, Method: "test"}
	resp, err := t_sse.SendRequest(ctx, req)
	require.NoError(t, err)

	var result string
	_ = json.Unmarshal(resp.Result, &result)
	assert.Equal(t, "sse-result", result)
}

// TestSSETransport_SendRequest_CtxCancel는 닫힌 SSE transport에서 SendRequest를 검증한다.
func TestSSETransport_SendRequest_CtxCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	b := &sseBase{
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	close(b.done)
	b.closed.Store(true)

	t_sse := &SSETransport{inner: b}
	_, err := t_sse.SendRequest(context.Background(), Request{Method: "test"})
	assert.Error(t, err) // ErrTransportClosed
}

// TestHTTPWebSocket_WriteMessage는 httpWebSocket.WriteMessage를 검증한다.
func TestHTTPWebSocket_WriteMessage(t *testing.T) {
	// noopWriter를 사용
	conn := &httpWebSocket{
		writer: noopWriter{},
		closer: nopCloser{},
	}
	err := conn.WriteMessage(wsTextMessage, []byte("test data"))
	assert.NoError(t, err)
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// TestHTTPWebSocket_Close는 httpWebSocket.Close를 검증한다.
func TestHTTPWebSocket_Close(t *testing.T) {
	closed := false
	conn := &httpWebSocket{
		closer: &trackingCloser{closeFn: func() error {
			closed = true
			return nil
		}},
	}
	err := conn.Close()
	assert.NoError(t, err)
	assert.True(t, closed)
}

type trackingCloser struct {
	closeFn func() error
}

func (t *trackingCloser) Close() error { return t.closeFn() }

// TestWebSocketBase_SendCancelRequest는 $/cancelRequest 전송을 검증한다.
func TestWebSocketBase_SendCancelRequest(t *testing.T) {
	conn := newMockWSConn()
	t_ws := NewWebSocketTransportWithConn(conn, nil)
	defer t_ws.Close()

	// ctx 취소로 $/cancelRequest 트리거
	reqCtx, cancel := context.WithCancel(context.Background())

	resultCh := make(chan error, 1)
	go func() {
		// SendRequest는 응답을 기다리다가 ctx 취소 시 $/cancelRequest 전송
		_, err := t_ws.SendRequest(reqCtx, Request{JSONRPC: JSONRPCVersion, Method: "slow"})
		resultCh <- err
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-resultCh:
		assert.Error(t, err) // ctx.Err()
	case <-time.After(500 * time.Millisecond):
		t.Fatal("SendRequest should return after ctx cancel")
	}

	// $/cancelRequest가 전송되었는지 확인
	conn.mu.Lock()
	outgoing := conn.outgoing
	conn.mu.Unlock()

	found := false
	for _, msg := range outgoing {
		var m map[string]any
		_ = json.Unmarshal(msg, &m)
		if method, ok := m["method"].(string); ok && method == "$/cancelRequest" {
			found = true
			break
		}
	}
	assert.True(t, found, "$/cancelRequest notification should have been sent")
}

// TestStdioTransport_SendRequest_Closed는 닫힌 transport에서 SendRequest를 검증한다.
func TestStdioTransport_SendRequest_Closed(t *testing.T) {
	t_stdio := &StdioTransport{inner: nil}
	_, err := t_stdio.SendRequest(context.Background(), Request{Method: "test"})
	assert.Error(t, err)
}

// TestNewHTTPWebSocket_HTTPError는 HTTP 에러 응답을 검증한다.
func TestNewHTTPWebSocket_HTTPError(t *testing.T) {
	// 401을 반환하는 서버
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	wsURI := strings.Replace(srv.URL, "http://", "ws://", 1)

	ctx := context.Background()
	_, err := NewWebSocketTransport(ctx, wsURI, nil, nil)
	// 401은 TLSValidationError로 반환됨
	assert.Error(t, err)
}

// TestSSETransport_Notify_ClosedTransport는 closed SSE에서 Notify를 검증한다.
func TestSSETransport_Notify_ClosedTransport(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	b := &sseBase{
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	close(b.done)
	b.closed.Store(true)

	t_sse := &SSETransport{inner: b}
	err := t_sse.Notify(context.Background(), Notification{Method: "test"})
	assert.Error(t, err)
}
