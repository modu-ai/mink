// Package install — handler_test.go covers the HTTP handler for the 7-step
// MINK onboarding install wizard Web UI.
// SPEC: SPEC-MINK-ONBOARDING-001 Phase 3A
package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
// Test helpers
// ---------------------------------------------------------------------------

// fakeClock is an injectable clock for TTL and sweep tests.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Now()}
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

// newTestHandler builds a Handler wired with the fake clock and in-memory keyring.
// The returned handler uses DryRun=true and temp-dir overrides so tests never touch disk.
func newTestHandler(t *testing.T) (*Handler, *fakeClock, *onboarding.InMemoryKeyring) {
	t.Helper()
	clock := newFakeClock()
	kr := onboarding.NewInMemoryKeyring()

	tmpDir := t.TempDir()
	opts := HandlerOptions{
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
	}

	h := NewHandler(opts)
	t.Cleanup(h.Close)
	return h, clock, kr
}

// ---------------------------------------------------------------------------
// Session helper: POST /install/api/session/start and extract tokens.
// ---------------------------------------------------------------------------

type startResult struct {
	sessionID string
	csrf      string
	cookie    *http.Cookie
}

func doStart(t *testing.T, h *Handler) startResult {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/install/api/session/start", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "start must return 200: %s", rec.Body.String())

	var resp sessionStartResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.NotEmpty(t, resp.SessionID)
	require.NotEmpty(t, resp.CSRFToken)

	// Locate the Set-Cookie header.
	var csrfCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "mink_csrf" {
			csrfCookie = c
			break
		}
	}
	require.NotNil(t, csrfCookie, "mink_csrf cookie must be set")

	return startResult{sessionID: resp.SessionID, csrf: resp.CSRFToken, cookie: csrfCookie}
}

// postWithCSRF sends a POST request with CSRF headers and cookie attached.
func postWithCSRF(h *Handler, path string, body any, sr startResult, extra ...func(*http.Request)) *httptest.ResponseRecorder {
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(http.MethodPost, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MINK-CSRF", sr.csrf)
	req.AddCookie(sr.cookie)
	for _, fn := range extra {
		fn(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestHappyPath_FullFlow exercises the complete 7-step path from start to complete.
func TestHappyPath_FullFlow(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler(t)

	sr := doStart(t, h)
	sid := sr.sessionID

	// Step 1 — Locale US.
	rec := postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/1/submit", sid),
		onboarding.LocaleChoice{Country: "US", Language: "en", Timezone: "America/New_York"},
		sr)
	require.Equal(t, http.StatusOK, rec.Code, "step 1: %s", rec.Body.String())

	// Step 2 — ModelSetup (zero value).
	rec = postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/2/submit", sid),
		onboarding.ModelSetup{}, sr)
	require.Equal(t, http.StatusOK, rec.Code, "step 2: %s", rec.Body.String())

	// Step 3 — Skip.
	rec = postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/3/skip", sid), nil, sr)
	require.Equal(t, http.StatusOK, rec.Code, "step 3 skip: %s", rec.Body.String())

	// Step 4 — Persona.
	rec = postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/4/submit", sid),
		onboarding.PersonaProfile{Name: "Alex"}, sr)
	require.Equal(t, http.StatusOK, rec.Code, "step 4: %s", rec.Body.String())

	// Step 5 — Provider (Ollama: no API key required).
	rec = postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/5/submit", sid),
		onboarding.ProviderStepInput{
			Choice: onboarding.ProviderChoice{Provider: onboarding.ProviderOllama, AuthMethod: onboarding.AuthMethodEnv},
		}, sr)
	require.Equal(t, http.StatusOK, rec.Code, "step 5: %s", rec.Body.String())

	// Step 6 — Messenger.
	rec = postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/6/submit", sid),
		onboarding.MessengerChannel{Type: onboarding.MessengerLocalTerminal}, sr)
	require.Equal(t, http.StatusOK, rec.Code, "step 6: %s", rec.Body.String())

	// Step 7 — Consent.
	rec = postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/7/submit", sid),
		onboarding.ConsentFlags{ConversationStorageLocal: true}, sr)
	require.Equal(t, http.StatusOK, rec.Code, "step 7: %s", rec.Body.String())

	// Complete.
	rec = postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/complete", sid), nil, sr)
	require.Equal(t, http.StatusOK, rec.Code, "complete: %s", rec.Body.String())

	var cr completeResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&cr))
	assert.NotNil(t, cr.CompletedAt, "completed_at must be non-nil after complete")
}

// TestCSRF_MissingToken verifies that a POST without X-MINK-CSRF returns 403.
func TestCSRF_MissingToken(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler(t)
	sr := doStart(t, h)

	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/install/api/session/%s/step/1/submit", sr.sessionID),
		strings.NewReader(`{"Country":"US","Language":"en","Timezone":"UTC"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sr.cookie) // cookie present but no header
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assertErrorCode(t, rec, "csrf_failed")
}

// TestCSRF_WrongToken verifies that a header token not matching the cookie returns 403.
func TestCSRF_WrongToken(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler(t)
	sr := doStart(t, h)

	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/install/api/session/%s/step/1/submit", sr.sessionID),
		strings.NewReader(`{"Country":"US","Language":"en","Timezone":"UTC"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MINK-CSRF", "definitely-not-the-right-token")
	req.AddCookie(sr.cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assertErrorCode(t, rec, "csrf_failed")
}

// TestCSRF_OriginRejected verifies that a cross-origin POST returns 403 in prod mode.
func TestCSRF_OriginRejected(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler(t) // devMode=false

	sr := doStart(t, h)
	rec := postWithCSRF(h,
		fmt.Sprintf("/install/api/session/%s/step/1/submit", sr.sessionID),
		onboarding.LocaleChoice{Country: "US", Language: "en", Timezone: "UTC"},
		sr,
		func(r *http.Request) { r.Header.Set("Origin", "http://evil.example") })

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assertErrorCode(t, rec, "csrf_failed")
}

// TestCSRF_DevModeAcceptsLocalhost verifies that MINK_DEV=1 mode accepts :5173 origin.
func TestCSRF_DevModeAcceptsLocalhost(t *testing.T) {
	t.Parallel()

	clock := newFakeClock()
	kr := onboarding.NewInMemoryKeyring()
	tmpDir := t.TempDir()
	h := NewHandler(HandlerOptions{
		Clock:   clock.Now,
		TTL:     30 * time.Minute,
		Keyring: kr,
		CompletionOptions: onboarding.CompletionOptions{
			DryRun:                    true,
			GlobalConfigPathOverride:  tmpDir + "/g.yaml",
			ProjectConfigPathOverride: tmpDir + "/p.yaml",
			CompletedMarkerOverride:   tmpDir + "/.done",
		},
		DevMode:       true,
		StaticHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
	})
	t.Cleanup(h.Close)

	sr := doStart(t, h)
	rec := postWithCSRF(h,
		fmt.Sprintf("/install/api/session/%s/step/1/submit", sr.sessionID),
		onboarding.LocaleChoice{Country: "US", Language: "en", Timezone: "UTC"},
		sr,
		func(r *http.Request) { r.Header.Set("Origin", "http://127.0.0.1:5173") })

	assert.Equal(t, http.StatusOK, rec.Code, "dev mode should allow :5173: %s", rec.Body.String())
}

// TestTTL_Expired verifies that advancing the clock beyond TTL returns 404 on GET state.
func TestTTL_Expired(t *testing.T) {
	t.Parallel()
	h, clock, _ := newTestHandler(t)

	sr := doStart(t, h)

	// Advance clock beyond 30-minute TTL.
	clock.Advance(31 * time.Minute)

	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/install/api/session/%s/state", sr.sessionID), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assertErrorCode(t, rec, "session_not_found")
}

// TestSessionUnknown verifies that an unknown session ID returns 404.
func TestSessionUnknown(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/install/api/session/deadbeefdeadbeef/state", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestGDPR_SkipBlocked verifies that skipping step 7 in a GDPR locale returns 422.
func TestGDPR_SkipBlocked(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler(t)

	sr := doStart(t, h)
	sid := sr.sessionID

	// Step 1 — FR locale with GDPR flag.
	rec := postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/1/submit", sid),
		onboarding.LocaleChoice{Country: "FR", Language: "fr", Timezone: "Europe/Paris", LegalFlags: []string{"GDPR"}},
		sr)
	require.Equal(t, http.StatusOK, rec.Code)

	// Steps 2–6 skip.
	for step := 2; step <= 6; step++ {
		rec = postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/%d/skip", sid, step), nil, sr)
		require.Equal(t, http.StatusOK, rec.Code, "skip step %d: %s", step, rec.Body.String())
	}

	// Step 7 skip must be blocked.
	rec = postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/7/skip", sid), nil, sr)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assertErrorCode(t, rec, "gdpr_consent_required")
}

// TestBackFromStep1 verifies that calling back at step 1 returns 400.
func TestBackFromStep1(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler(t)
	sr := doStart(t, h)

	rec := postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/back", sr.sessionID), nil, sr)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assertErrorCode(t, rec, "cannot_go_back")
}

// TestStepOutOfOrder verifies that submitting step 3 when current is step 1 returns 400.
func TestStepOutOfOrder(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler(t)
	sr := doStart(t, h)

	rec := postWithCSRF(h,
		fmt.Sprintf("/install/api/session/%s/step/3/submit", sr.sessionID),
		onboarding.CLIToolsDetection{}, sr)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assertErrorCode(t, rec, "step_out_of_order")
}

// TestConcurrent_SameSession_OneFails verifies that exactly one of two concurrent
// submit requests succeeds and the other is rejected as out-of-order.
func TestConcurrent_SameSession_OneFails(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler(t)
	sr := doStart(t, h)

	payload := onboarding.LocaleChoice{Country: "US", Language: "en", Timezone: "UTC"}

	results := make([]int, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	submit := func(idx int) {
		defer wg.Done()
		rec := postWithCSRF(h,
			fmt.Sprintf("/install/api/session/%s/step/1/submit", sr.sessionID),
			payload, sr)
		results[idx] = rec.Code
	}

	go submit(0)
	go submit(1)
	wg.Wait()

	successCount := 0
	badRequestCount := 0
	for _, code := range results {
		switch code {
		case http.StatusOK:
			successCount++
		case http.StatusBadRequest:
			badRequestCount++
		}
	}
	assert.Equal(t, 1, successCount, "exactly one concurrent submit should succeed")
	assert.Equal(t, 1, badRequestCount, "exactly one concurrent submit should fail with 400")
}

// TestInvalidJSON_400 verifies that malformed JSON body returns 400 invalid_payload.
func TestInvalidJSON_400(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler(t)
	sr := doStart(t, h)

	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/install/api/session/%s/step/1/submit", sr.sessionID),
		strings.NewReader(`{not valid json`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MINK-CSRF", sr.csrf)
	req.AddCookie(sr.cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assertErrorCode(t, rec, "invalid_payload")
}

// TestStep5ProviderInput_StoresInKeyring verifies that a ProviderStepInput with
// AuthMethodAPIKey causes the API key to be written to the injected keyring.
func TestStep5ProviderInput_StoresInKeyring(t *testing.T) {
	t.Parallel()
	h, _, kr := newTestHandler(t)
	sr := doStart(t, h)
	sid := sr.sessionID

	// Steps 1–4 submit.
	steps14(t, h, sr, sid)

	// Step 5 — Anthropic API key (must match ^sk-ant-[A-Za-z0-9_-]{20,}$).
	apiKey := "sk-ant-testkey1234567890abcdef"
	rec := postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/5/submit", sid),
		onboarding.ProviderStepInput{
			Choice: onboarding.ProviderChoice{
				Provider:   onboarding.ProviderAnthropic,
				AuthMethod: onboarding.AuthMethodAPIKey,
			},
			APIKey: apiKey,
		}, sr)
	require.Equal(t, http.StatusOK, rec.Code, "step 5: %s", rec.Body.String())

	// Verify keyring contains the key.
	got, err := onboarding.GetProviderAPIKey(kr, string(onboarding.ProviderAnthropic))
	require.NoError(t, err, "keyring must contain provider API key after step 5")
	assert.Equal(t, apiKey, got)
}

// TestComplete_WritesConfig_DryRun verifies that completing with DryRun=true
// returns 200 without writing any files.
func TestComplete_WritesConfig_DryRun(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler(t) // DryRun=true by default in newTestHandler

	sr := doStart(t, h)
	sid := sr.sessionID

	// Steps 1–4.
	steps14(t, h, sr, sid)

	// Step 5 — Ollama (no API key required by the validator).
	rec := postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/5/submit", sid),
		onboarding.ProviderStepInput{
			Choice: onboarding.ProviderChoice{Provider: onboarding.ProviderOllama, AuthMethod: onboarding.AuthMethodEnv},
		}, sr)
	require.Equal(t, http.StatusOK, rec.Code)

	// Step 6.
	rec = postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/6/submit", sid),
		onboarding.MessengerChannel{Type: onboarding.MessengerLocalTerminal}, sr)
	require.Equal(t, http.StatusOK, rec.Code)

	// Step 7.
	rec = postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/7/submit", sid),
		onboarding.ConsentFlags{ConversationStorageLocal: true}, sr)
	require.Equal(t, http.StatusOK, rec.Code)

	// Complete.
	rec = postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/complete", sid), nil, sr)
	assert.Equal(t, http.StatusOK, rec.Code, "complete: %s", rec.Body.String())

	var cr completeResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&cr))
	assert.NotNil(t, cr.CompletedAt)

	// Session must be GC'd after complete.
	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/install/api/session/%s/state", sid), nil)
	recState := httptest.NewRecorder()
	h.ServeHTTP(recState, req)
	assert.Equal(t, http.StatusNotFound, recState.Code, "session must be deleted after complete")
}

// TestGetState_ReturnsCurrentData verifies GET /state returns the correct step and data.
func TestGetState_ReturnsCurrentData(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler(t)
	sr := doStart(t, h)

	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/install/api/session/%s/state", sr.sessionID), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp sessionStateResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, 1, resp.CurrentStep)
	assert.Equal(t, 7, resp.TotalSteps)
	assert.Nil(t, resp.CompletedAt)
}

// TestBackNavigationWorks verifies that Back correctly decrements CurrentStep.
func TestBackNavigationWorks(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler(t)
	sr := doStart(t, h)
	sid := sr.sessionID

	// Submit step 1.
	rec := postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/1/submit", sid),
		onboarding.LocaleChoice{Country: "US", Language: "en", Timezone: "UTC"}, sr)
	require.Equal(t, http.StatusOK, rec.Code)

	// Now at step 2; go back.
	rec = postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/back", sid), nil, sr)
	require.Equal(t, http.StatusOK, rec.Code)

	var br backResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&br))
	assert.Equal(t, 1, br.CurrentStep)
}

// ---------------------------------------------------------------------------
// Internal helpers for multi-step test setup
// ---------------------------------------------------------------------------

// steps14 submits steps 1–4 with minimal valid payloads.
func steps14(t *testing.T, h *Handler, sr startResult, sid string) {
	t.Helper()

	rec := postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/1/submit", sid),
		onboarding.LocaleChoice{Country: "US", Language: "en", Timezone: "UTC"}, sr)
	require.Equal(t, http.StatusOK, rec.Code, "steps14 step 1: %s", rec.Body.String())

	rec = postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/2/submit", sid),
		onboarding.ModelSetup{}, sr)
	require.Equal(t, http.StatusOK, rec.Code, "steps14 step 2: %s", rec.Body.String())

	rec = postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/3/skip", sid), nil, sr)
	require.Equal(t, http.StatusOK, rec.Code, "steps14 step 3 skip: %s", rec.Body.String())

	rec = postWithCSRF(h, fmt.Sprintf("/install/api/session/%s/step/4/submit", sid),
		onboarding.PersonaProfile{Name: "TestUser"}, sr)
	require.Equal(t, http.StatusOK, rec.Code, "steps14 step 4: %s", rec.Body.String())
}

// assertErrorCode decodes the JSON body and asserts that error.code equals want.
func assertErrorCode(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	var resp errorResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err, "response must be valid JSON error envelope")
	assert.Equal(t, want, resp.Error.Code, "unexpected error code")
}
