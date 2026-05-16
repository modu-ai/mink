// Package install — sse_test.go covers the SSE pull/stream endpoint.
// SPEC: SPEC-MINK-ONBOARDING-001 Phase 3C
package install

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/onboarding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// SSE test helpers
// ---------------------------------------------------------------------------

// sseEvent represents a single parsed Server-Sent Event.
type sseEvent struct {
	Event string // contents of "event:" line; empty for anonymous data events
	Data  string // contents of "data:" line (raw JSON string)
}

// streamScanner reads from r line-by-line and returns all parsed SSE events.
// It terminates when the reader reaches EOF.
func streamScanner(t *testing.T, body string) []sseEvent {
	t.Helper()
	var events []sseEvent
	var cur sseEvent
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event:"):
			cur.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			cur.Data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		case line == "":
			// Blank line = end of event block.
			if cur.Data != "" || cur.Event != "" {
				events = append(events, cur)
				cur = sseEvent{}
			}
		}
	}
	return events
}

// newTestHandlerWithPullFn builds a Handler wired with a custom PullFn for SSE tests.
func newTestHandlerWithPullFn(
	t *testing.T,
	pullFn func(ctx context.Context, modelName string, progress chan<- onboarding.ProgressUpdate) error,
) (*Handler, *fakeClock) {
	t.Helper()
	clock := newFakeClock()
	kr := onboarding.NewInMemoryKeyring()
	tmpDir := t.TempDir()
	h := NewHandler(HandlerOptions{
		Clock:   clock.Now,
		TTL:     30 * time.Minute,
		Keyring: kr,
		CompletionOptions: onboarding.CompletionOptions{
			DryRun:                    true,
			GlobalConfigPathOverride:  tmpDir + "/global.yaml",
			ProjectConfigPathOverride: tmpDir + "/project.yaml",
			CompletedMarkerOverride:   tmpDir + "/.mink-onboarding-done",
		},
		DevMode:       false,
		StaticHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
		PullFn:        pullFn,
	})
	t.Cleanup(h.Close)
	return h, clock
}

// doSSERequest issues a GET to the pull stream endpoint with cookie CSRF attached.
// Returns the recorder body as a string after the handler returns.
// The csrfToken parameter is accepted for caller signature parity with the POST
// helpers (which require X-MINK-CSRF). EventSource cannot set headers, so the
// SSE handler validates the cookie value only — the parameter is intentionally
// unused inside this helper.
func doSSERequest(h *Handler, sessionID, _ string, cookie *http.Cookie, modelName string, mutators ...func(*http.Request)) *httptest.ResponseRecorder {
	path := fmt.Sprintf("/install/api/session/%s/pull/stream", sessionID)
	if modelName != "" {
		path += "?model=" + modelName
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	for _, fn := range mutators {
		fn(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestSSE_HappyPath verifies that a successful pull emits 3 data events + final done event.
func TestSSE_HappyPath(t *testing.T) {
	t.Parallel()

	// Fake PullFn: emits 3 ProgressUpdates, then closes channel, returns nil.
	fakePull := func(ctx context.Context, modelName string, progress chan<- onboarding.ProgressUpdate) error {
		defer close(progress)
		updates := []onboarding.ProgressUpdate{
			{Phase: "pulling manifest", PercentDone: -1, Raw: "pulling manifest"},
			{Phase: "downloading layer", Layer: "sha256:abc", BytesTotal: 1000, BytesDone: 500, PercentDone: 50, Raw: "pulling sha256:abc 500B / 1000B"},
			{Phase: "success", PercentDone: 100, Raw: "success"},
		}
		for _, u := range updates {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case progress <- u:
			}
		}
		return nil
	}

	h, _ := newTestHandlerWithPullFn(t, fakePull)
	sr := doStart(t, h)

	rec := doSSERequest(h, sr.sessionID, sr.csrf, sr.cookie, "test-model")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))

	events := streamScanner(t, rec.Body.String())
	// Expect 3 data events + 1 done event.
	require.Len(t, events, 4, "expected 3 data events + 1 done event, got: %v", events)

	// First 3 are anonymous data events (no event: line).
	for i, ev := range events[:3] {
		assert.Empty(t, ev.Event, "event[%d] should be anonymous data event", i)
		assert.NotEmpty(t, ev.Data, "event[%d] data should not be empty", i)

		var update onboarding.ProgressUpdate
		require.NoError(t, json.Unmarshal([]byte(ev.Data), &update), "event[%d] data must be valid JSON ProgressUpdate", i)
	}

	// Verify the first update.
	var first onboarding.ProgressUpdate
	require.NoError(t, json.Unmarshal([]byte(events[0].Data), &first))
	assert.Equal(t, "pulling manifest", first.Phase)

	// Last event is the done event.
	doneEv := events[3]
	assert.Equal(t, "done", doneEv.Event)
	var donePayload map[string]bool
	require.NoError(t, json.Unmarshal([]byte(doneEv.Data), &donePayload))
	assert.True(t, donePayload["ok"])
}

// TestSSE_PullError verifies that a PullFn error emits 1 data event + error event.
func TestSSE_PullError(t *testing.T) {
	t.Parallel()

	const errMessage = "ollama pull failed: exit status 1"

	fakePull := func(ctx context.Context, _ string, progress chan<- onboarding.ProgressUpdate) error {
		defer close(progress)
		progress <- onboarding.ProgressUpdate{Phase: "pulling manifest", PercentDone: -1, Raw: "pulling manifest"}
		return fmt.Errorf(errMessage)
	}

	h, _ := newTestHandlerWithPullFn(t, fakePull)
	sr := doStart(t, h)

	rec := doSSERequest(h, sr.sessionID, sr.csrf, sr.cookie, "test-model")

	require.Equal(t, http.StatusOK, rec.Code)

	events := streamScanner(t, rec.Body.String())
	require.GreaterOrEqual(t, len(events), 2, "expected at least 1 data event + 1 error event")

	// Last event must be error.
	last := events[len(events)-1]
	assert.Equal(t, "error", last.Event)
	var errPayload map[string]string
	require.NoError(t, json.Unmarshal([]byte(last.Data), &errPayload))
	assert.Contains(t, errPayload["message"], "ollama pull failed")
}

// TestSSE_ClientDisconnect verifies that cancelling the client context propagates
// to PullModel's context (ctx.Done() fires inside PullFn).
func TestSSE_ClientDisconnect(t *testing.T) {
	t.Parallel()

	// cancelObserved is closed when the fake PullFn observes ctx cancellation.
	cancelObserved := make(chan struct{})
	ready := make(chan struct{}) // signals that PullFn has started and sent first update

	fakePull := func(ctx context.Context, _ string, progress chan<- onboarding.ProgressUpdate) error {
		defer close(progress)
		// Send one update to prove streaming started.
		select {
		case progress <- onboarding.ProgressUpdate{Phase: "pulling manifest", PercentDone: -1}:
		case <-ctx.Done():
			close(cancelObserved)
			return ctx.Err()
		}
		close(ready)
		// Now block until context is cancelled.
		<-ctx.Done()
		close(cancelObserved)
		return ctx.Err()
	}

	h, _ := newTestHandlerWithPullFn(t, fakePull)
	sr := doStart(t, h)

	// Create a request with a cancellable context.
	ctx, clientCancel := context.WithCancel(context.Background())
	path := fmt.Sprintf("/install/api/session/%s/pull/stream?model=test-model", sr.sessionID)
	req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	req.AddCookie(sr.cookie)

	rec := httptest.NewRecorder()

	// Run handler in a goroutine so we can cancel the context mid-stream.
	// WaitGroup.Go (Go 1.25+) absorbs Add(1)+defer Done() into a single call.
	var wg sync.WaitGroup
	wg.Go(func() {
		h.ServeHTTP(rec, req)
	})

	// Wait for PullFn to start sending before cancelling.
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("PullFn did not send first update within 5s")
	}

	// Cancel the client; handler should observe r.Context().Done() and cancel PullFn.
	clientCancel()

	// Wait for the PullFn to observe cancellation.
	select {
	case <-cancelObserved:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("PullModel context was not cancelled within 5s after client disconnect")
	}

	// Wait for the handler to finish.
	wg.Wait()
}

// TestSSE_SessionUnknown verifies that a request with a random session ID returns 404.
func TestSSE_SessionUnknown(t *testing.T) {
	t.Parallel()

	h, _ := newTestHandlerWithPullFn(t, nil)

	// Fabricate a cookie with a dummy CSRF value.
	cookie := &http.Cookie{Name: "mink_csrf", Value: "dummy"}
	rec := doSSERequest(h, "00000000-0000-0000-0000-000000000000", "dummy", cookie, "test-model")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assertErrorCode(t, rec, "session_not_found")
}

// TestSSE_ModelFromSession verifies that omitting ?model= picks up SelectedModel from flow.
func TestSSE_ModelFromSession(t *testing.T) {
	t.Parallel()

	const wantModel = "ai-mink/gemma4-e4b-rl-v1:q5_k_m"

	var capturedModel string
	fakePull := func(ctx context.Context, modelName string, progress chan<- onboarding.ProgressUpdate) error {
		capturedModel = modelName
		close(progress)
		return nil
	}

	h, _ := newTestHandlerWithPullFn(t, fakePull)
	sr := doStart(t, h)
	sid := sr.sessionID

	// Submit step 1 first (required before step 2).
	rec := postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/1/submit", sid),
		onboarding.LocaleChoice{Country: "US", Language: "en", Timezone: "UTC"}, sr)
	require.Equal(t, http.StatusOK, rec.Code, "step 1 submit: %s", rec.Body.String())

	// Submit step 2 with a SelectedModel to wire it into the session.
	rec = postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/2/submit", sid),
		onboarding.ModelSetup{SelectedModel: wantModel}, sr)
	require.Equal(t, http.StatusOK, rec.Code, "step 2 submit: %s", rec.Body.String())

	// Issue stream request WITHOUT ?model= query param.
	path := fmt.Sprintf("/install/api/session/%s/pull/stream", sid)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(sr.cookie)
	recSSE := httptest.NewRecorder()
	h.ServeHTTP(recSSE, req)

	require.Equal(t, http.StatusOK, recSSE.Code)
	assert.Equal(t, wantModel, capturedModel, "handler must pick up SelectedModel from session when ?model= is omitted")
}

// TestSSE_ModelMissing verifies that omitting ?model= with no SelectedModel/DetectedModel returns 400.
func TestSSE_ModelMissing(t *testing.T) {
	t.Parallel()

	h, _ := newTestHandlerWithPullFn(t, nil)
	sr := doStart(t, h)

	// Do NOT submit step 2, so Model fields remain zero values.
	// Issue stream request without ?model=.
	path := fmt.Sprintf("/install/api/session/%s/pull/stream", sr.sessionID)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(sr.cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assertErrorCode(t, rec, "model_required")
}

// TestSSE_OriginCheck verifies that a foreign Origin header is rejected with 403.
func TestSSE_OriginCheck(t *testing.T) {
	t.Parallel()

	h, _ := newTestHandlerWithPullFn(t, nil)
	sr := doStart(t, h)

	rec := doSSERequest(h, sr.sessionID, sr.csrf, sr.cookie, "test-model",
		func(r *http.Request) {
			r.Header.Set("Origin", "http://evil.example.com")
		})

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assertErrorCode(t, rec, "csrf_failed")
}
