package ratelimit_test

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestNewTracker는 NewTracker가 nil이 아닌 Tracker를 반환하는지 검증한다.
func TestNewTracker(t *testing.T) {
	t.Parallel()

	tracker := ratelimit.NewTracker()
	if tracker == nil {
		t.Error("NewTracker: nil 반환")
	}
}

// TestTracker_Parse_NoopDoesNotPanic은 Parse 호출 시 패닉이 발생하지 않는지 검증한다.
func TestTracker_Parse_NoopDoesNotPanic(t *testing.T) {
	t.Parallel()

	tracker := ratelimit.NewTracker()

	headers := http.Header{
		"X-Ratelimit-Limit-Requests":     []string{"1000"},
		"X-Ratelimit-Remaining-Requests": []string{"999"},
	}

	// Parse는 noop이므로 패닉 없이 완료해야 한다.
	tracker.Parse("anthropic", headers, time.Now())
	tracker.Parse("openai", headers, time.Now())
	tracker.Parse("", http.Header{}, time.Now())
}

// TestTracker_Parse_ThreadSafe는 Parse가 동시 호출에도 안전한지 검증한다.
func TestTracker_Parse_ThreadSafe(t *testing.T) {
	t.Parallel()

	tracker := ratelimit.NewTracker()
	headers := http.Header{}

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tracker.Parse("anthropic", headers, time.Now())
		}()
	}
	wg.Wait()
}
