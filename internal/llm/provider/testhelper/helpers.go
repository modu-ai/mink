// Package testhelper는 provider 테스트를 위한 공유 도우미 함수를 제공한다.
// SPEC-GOOSE-ADAPTER-001 plan.md §5 Shared test helpers
package testhelper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/llm/credential"
	"github.com/modu-ai/goose/internal/message"
)

// FakePool은 주어진 credID 목록으로 구성된 테스트용 CredentialPool을 생성한다.
func FakePool(t *testing.T, credIDs []string) *credential.CredentialPool {
	t.Helper()
	creds := make([]*credential.PooledCredential, len(credIDs))
	for i, id := range credIDs {
		creds[i] = &credential.PooledCredential{
			ID:        id,
			Provider:  "anthropic",
			KeyringID: "kr-" + id,
			Status:    credential.CredOK,
		}
	}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("FakePool 생성 실패: %v", err)
	}
	return pool
}

// NewSSEServer는 SSE 이벤트 목록을 반환하는 httptest.Server를 생성한다.
func NewSSEServer(events []string) *httptest.Server {
	body := strings.Join(events, "\n") + "\n"
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Request-Id", "req-test-123")
		_, _ = fmt.Fprint(w, body)
	}))
}

// NewSlowSSEServer는 지정된 지연 후에 응답하는 httptest.Server를 생성한다.
func NewSlowSSEServer(delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(delay):
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprint(w, "data: {\"type\":\"message_stop\"}\n\n")
		}
	}))
}

// New429Server는 첫 번째 요청에 429를 반환하고 두 번째 요청에 SSE를 반환하는 서버이다.
func New429Server(sseEvents []string) *httptest.Server {
	callCount := 0
	body := strings.Join(sseEvents, "\n") + "\n"
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Retry-After", "120")
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, body)
	}))
}

// DrainStream은 채널에서 최대 max개의 이벤트를 수집한다.
// max<=0이면 채널이 닫힐 때까지 수집한다.
func DrainStream(ctx context.Context, ch <-chan message.StreamEvent, max int) []message.StreamEvent {
	var events []message.StreamEvent
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, evt)
			if max > 0 && len(events) >= max {
				return events
			}
		case <-ctx.Done():
			return events
		}
	}
}
