// Package install — handler.go implements the HTTP handler for the 7-step MINK
// onboarding install wizard Web UI. It exposes a thin REST API over
// internal/onboarding.OnboardingFlow via a per-session SessionStore.
//
// Security model:
//   - CSRF: double-submit cookie pattern (X-MINK-CSRF header + mink_csrf cookie).
//   - Origin check: all POST requests are origin-restricted to 127.0.0.1.
//   - Session TTL: 30 minutes of inactivity; background sweep every 60 seconds.
//   - Per-session mutex: all OnboardingFlow access is serialised under entry.mu.
//
// SPEC: SPEC-MINK-ONBOARDING-001 Phase 3A
// REQ: REQ-OB-031, REQ-OB-032, REQ-OB-033
package install

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modu-ai/mink/internal/onboarding"
)

// ---------------------------------------------------------------------------
// JSON envelope types
// ---------------------------------------------------------------------------

// errorResponse is the canonical error envelope for all 4xx/5xx responses.
type errorResponse struct {
	Error errorBody `json:"error"`
}

// errorBody carries a machine-readable code and a human-readable message.
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// sessionStartResponse is returned by POST /install/api/session/start.
type sessionStartResponse struct {
	SessionID   string                    `json:"session_id"`
	CSRFToken   string                    `json:"csrf_token"`
	CurrentStep int                       `json:"current_step"`
	TotalSteps  int                       `json:"total_steps"`
	Data        onboarding.OnboardingData `json:"data"`
}

// sessionStateResponse is returned by GET /install/api/session/{id}/state.
type sessionStateResponse struct {
	SessionID   string                    `json:"session_id"`
	CurrentStep int                       `json:"current_step"`
	TotalSteps  int                       `json:"total_steps"`
	Data        onboarding.OnboardingData `json:"data"`
	CompletedAt *time.Time                `json:"completed_at"`
}

// stepSubmitResponse is returned by POST /install/api/session/{id}/step/{n}/submit.
type stepSubmitResponse struct {
	CurrentStep int                       `json:"current_step"`
	Data        onboarding.OnboardingData `json:"data"`
}

// stepSkipResponse is returned by POST /install/api/session/{id}/step/{n}/skip.
type stepSkipResponse struct {
	CurrentStep int `json:"current_step"`
}

// backResponse is returned by POST /install/api/session/{id}/back.
type backResponse struct {
	CurrentStep int `json:"current_step"`
}

// completeResponse is returned by POST /install/api/session/{id}/complete.
type completeResponse struct {
	CompletedAt *time.Time                `json:"completed_at"`
	Data        onboarding.OnboardingData `json:"data"`
}

// ---------------------------------------------------------------------------
// Session store
// ---------------------------------------------------------------------------

// sessionEntry holds a single onboarding session and its associated metadata.
// All access to flow must be serialised under mu.
//
// @MX:WARN: [AUTO] entry.mu serialises all OnboardingFlow mutations; callers MUST
// acquire entry.mu.Lock before calling any flow method.
// @MX:REASON: OnboardingFlow is not goroutine-safe; without the lock, concurrent
// POST /step/N/submit requests on the same session can corrupt CurrentStep.
type sessionEntry struct {
	mu             sync.Mutex
	flow           *onboarding.OnboardingFlow
	csrfToken      string    // 32 hex chars generated at session start
	lastActivityAt time.Time // updated on every authenticated request
}

// SessionStore maps session IDs to entries and runs a background TTL sweep.
//
// @MX:ANCHOR: [AUTO] SessionStore is the central registry for all active onboarding
// sessions — created by NewHandler, used by every HTTP handler method.
// @MX:REASON: Incorrect TTL sweep or race on the outer mu can silently drop
// sessions or allow expired sessions to persist; changes require race-detector tests.
type SessionStore struct {
	mu            sync.RWMutex
	sessions      map[string]*sessionEntry
	clock         func() time.Time
	ttl           time.Duration
	sweepInterval time.Duration
	stopSweep     chan struct{}
}

// NewSessionStore initialises a SessionStore with the supplied clock and TTL.
// A background goroutine runs TTL sweeps on sweepInterval (default 60s).
func NewSessionStore(clock func() time.Time, ttl time.Duration) *SessionStore {
	if clock == nil {
		clock = time.Now
	}
	s := &SessionStore{
		sessions:      make(map[string]*sessionEntry),
		clock:         clock,
		ttl:           ttl,
		sweepInterval: 60 * time.Second,
		stopSweep:     make(chan struct{}),
	}
	go s.sweepLoop()
	return s
}

// Create adds a new session entry and returns it together with a freshly generated
// CSRF token (32 random hex chars).
func (s *SessionStore) Create(flow *onboarding.OnboardingFlow) (*sessionEntry, string) {
	token := generateToken()
	entry := &sessionEntry{
		flow:           flow,
		csrfToken:      token,
		lastActivityAt: s.clock(),
	}
	s.mu.Lock()
	s.sessions[flow.SessionID] = entry
	s.mu.Unlock()
	return entry, token
}

// Get returns the session entry for the given ID, or (nil, false) when the session
// does not exist or has exceeded its TTL. The entry's lastActivityAt is NOT updated
// here — callers update it under entry.mu after authentication.
func (s *SessionStore) Get(sessionID string) (*sessionEntry, bool) {
	s.mu.RLock()
	entry, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	// TTL check under entry.mu to avoid TOCTOU between Get and lastActivityAt.
	entry.mu.Lock()
	expired := s.clock().Sub(entry.lastActivityAt) > s.ttl
	entry.mu.Unlock()
	if expired {
		return nil, false
	}
	return entry, true
}

// Delete removes the session from the store.
func (s *SessionStore) Delete(sessionID string) {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
}

// Sweep removes all sessions whose lastActivityAt is older than ttl.
func (s *SessionStore) Sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	for id, entry := range s.sessions {
		entry.mu.Lock()
		expired := now.Sub(entry.lastActivityAt) > s.ttl
		entry.mu.Unlock()
		if expired {
			delete(s.sessions, id)
		}
	}
}

// Close stops the background sweep goroutine.
func (s *SessionStore) Close() {
	close(s.stopSweep)
}

// sweepLoop runs Sweep on sweepInterval until stopSweep is closed.
func (s *SessionStore) sweepLoop() {
	ticker := time.NewTicker(s.sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.Sweep()
		case <-s.stopSweep:
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Handler
// ---------------------------------------------------------------------------

// HandlerOptions configures Handler construction. All fields are optional; defaults
// are applied for nil/zero values.
type HandlerOptions struct {
	// Clock overrides time.Now for TTL and sweep calculations (useful in tests).
	Clock func() time.Time

	// TTL is the session inactivity timeout. Defaults to 30 minutes.
	TTL time.Duration

	// Keyring is the secret store used by CompleteAndPersist for provider API keys.
	// Defaults to onboarding.SystemKeyring{}.
	Keyring onboarding.KeyringClient

	// CompletionOptions overrides file paths and enables dry-run in tests.
	CompletionOptions onboarding.CompletionOptions

	// DevMode enables relaxed Origin checking for the Vite dev server on :5173.
	// Defaults to os.Getenv("MINK_DEV") == "1".
	DevMode bool

	// StaticHandler serves the embedded React bundle. Defaults to StaticHandler().
	StaticHandler http.Handler

	// PullFn overrides the default onboarding.PullModel for test injection.
	// When nil, onboarding.PullModel is used.
	PullFn func(ctx context.Context, modelName string, progress chan<- onboarding.ProgressUpdate) error
}

// Handler is the HTTP handler for the MINK install wizard Web UI.
// It is safe for concurrent use once constructed.
//
// @MX:ANCHOR: [AUTO] Handler is the integration point wiring SessionStore + OnboardingFlow
// to HTTP — used by RunServer (CLI --web) and handler_test.go.
// @MX:REASON: ServeHTTP delegates to mux; mux route registration order and path
// patterns (Go 1.22+ enhanced ServeMux) must be preserved to avoid shadowing.
//
// @MX:NOTE: [AUTO] CSRF protection uses the double-submit cookie pattern: the server
// sets mink_csrf cookie (HttpOnly=false, SameSite=Strict) at session start and
// requires both the X-MINK-CSRF header and the cookie to match on every POST.
// Origin is additionally checked against the 127.0.0.1 allowlist (or :5173 in
// MINK_DEV=1) to defend against CORS preflights from off-host pages.
type Handler struct {
	store    *SessionStore
	keyring  onboarding.KeyringClient
	complete onboarding.CompletionOptions
	devMode  bool
	static   http.Handler
	mux      *http.ServeMux
	pullFn   func(ctx context.Context, modelName string, progress chan<- onboarding.ProgressUpdate) error
}

// NewHandler constructs a Handler, applies defaults, and registers all routes on mux.
func NewHandler(opts HandlerOptions) *Handler {
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	if opts.TTL == 0 {
		opts.TTL = 30 * time.Minute
	}
	if opts.Keyring == nil {
		opts.Keyring = onboarding.SystemKeyring{}
	}
	if opts.StaticHandler == nil {
		opts.StaticHandler = StaticHandler()
	}
	// DevMode defaults to environment variable if not explicitly set.
	devMode := opts.DevMode || os.Getenv("MINK_DEV") == "1"

	// Default PullFn to the real onboarding.PullModel when not injected.
	pullFn := opts.PullFn
	if pullFn == nil {
		pullFn = onboarding.PullModel
	}

	h := &Handler{
		store:    NewSessionStore(opts.Clock, opts.TTL),
		keyring:  opts.Keyring,
		complete: opts.CompletionOptions,
		devMode:  devMode,
		static:   opts.StaticHandler,
		pullFn:   pullFn,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /install", h.serveIndex)
	mux.Handle("GET /install/assets/", h.static)
	mux.HandleFunc("POST /install/api/session/start", h.startSession)
	mux.HandleFunc("GET /install/api/session/{id}/state", h.getState)
	mux.HandleFunc("POST /install/api/session/{id}/step/{n}/submit", h.submitStep)
	mux.HandleFunc("POST /install/api/session/{id}/step/{n}/skip", h.skipStep)
	mux.HandleFunc("POST /install/api/session/{id}/back", h.back)
	mux.HandleFunc("POST /install/api/session/{id}/complete", h.complete_)
	mux.HandleFunc("GET /install/api/session/{id}/pull/stream", h.pullStream)
	h.mux = mux

	return h
}

// ServeHTTP implements http.Handler by delegating to the internal mux.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// Close stops the session store's background sweep goroutine.
func (h *Handler) Close() {
	h.store.Close()
}

// ---------------------------------------------------------------------------
// Route handlers
// ---------------------------------------------------------------------------

// serveIndex delegates to the static handler for the SPA root.
func (h *Handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	h.static.ServeHTTP(w, r)
}

// startSession creates a new OnboardingFlow, registers it in the SessionStore, and
// returns the session metadata together with a fresh CSRF token.
//
// POST /install/api/session/start
func (h *Handler) startSession(w http.ResponseWriter, r *http.Request) {
	flow, err := onboarding.StartFlow(r.Context(), nil,
		onboarding.WithKeyring(h.keyring),
		onboarding.WithCompletionOptions(h.complete),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", fmt.Sprintf("failed to start flow: %v", err))
		return
	}

	entry, token := h.store.Create(flow)
	_ = entry // entry stored in store; accessed by subsequent requests

	// Set double-submit CSRF cookie (HttpOnly=false so JS can read it for the header).
	http.SetCookie(w, &http.Cookie{
		Name:     "mink_csrf",
		Value:    token,
		Path:     "/install",
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
	})

	writeJSON(w, http.StatusOK, sessionStartResponse{
		SessionID:   flow.SessionID,
		CSRFToken:   token,
		CurrentStep: flow.CurrentStep,
		TotalSteps:  onboarding.TotalSteps(),
		Data:        flow.Data,
	})
}

// getState returns the current step and accumulated data for a session.
//
// GET /install/api/session/{id}/state
func (h *Handler) getState(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	entry, ok := h.store.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "session_not_found", "session not found or expired")
		return
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.lastActivityAt = h.store.clock()

	writeJSON(w, http.StatusOK, sessionStateResponse{
		SessionID:   entry.flow.SessionID,
		CurrentStep: entry.flow.CurrentStep,
		TotalSteps:  onboarding.TotalSteps(),
		Data:        entry.flow.Data,
		CompletedAt: entry.flow.CompletedAt,
	})
}

// submitStep decodes the step-specific JSON payload and calls SubmitStep on the flow.
//
// POST /install/api/session/{id}/step/{n}/submit
func (h *Handler) submitStep(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	nStr := r.PathValue("n")

	if !h.checkCSRF(w, r) {
		return
	}

	n, err := strconv.Atoi(nStr)
	if err != nil || n < 1 || n > onboarding.TotalSteps() {
		writeError(w, http.StatusBadRequest, "step_range", "step number out of range")
		return
	}

	entry, ok := h.store.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "session_not_found", "session not found or expired")
		return
	}

	payload, decodeErr := decodeStepPayload(r, n)
	if decodeErr != nil {
		writeError(w, http.StatusBadRequest, "invalid_payload", decodeErr.Error())
		return
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.lastActivityAt = h.store.clock()

	if err := entry.flow.SubmitStep(n, payload); err != nil {
		writeError(w, mapFlowError(err), errorCode(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, stepSubmitResponse{
		CurrentStep: entry.flow.CurrentStep,
		Data:        entry.flow.Data,
	})
}

// skipStep advances CurrentStep without storing data for step n.
//
// POST /install/api/session/{id}/step/{n}/skip
func (h *Handler) skipStep(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	nStr := r.PathValue("n")

	if !h.checkCSRF(w, r) {
		return
	}

	n, err := strconv.Atoi(nStr)
	if err != nil || n < 1 || n > onboarding.TotalSteps() {
		writeError(w, http.StatusBadRequest, "step_range", "step number out of range")
		return
	}

	entry, ok := h.store.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "session_not_found", "session not found or expired")
		return
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.lastActivityAt = h.store.clock()

	if err := entry.flow.SkipStep(n); err != nil {
		writeError(w, mapFlowError(err), errorCode(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, stepSkipResponse{CurrentStep: entry.flow.CurrentStep})
}

// back decrements CurrentStep by 1.
//
// POST /install/api/session/{id}/back
func (h *Handler) back(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if !h.checkCSRF(w, r) {
		return
	}

	entry, ok := h.store.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "session_not_found", "session not found or expired")
		return
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.lastActivityAt = h.store.clock()

	if err := entry.flow.Back(); err != nil {
		writeError(w, mapFlowError(err), errorCode(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, backResponse{CurrentStep: entry.flow.CurrentStep})
}

// complete_ finalises the session, calls CompleteAndPersist, and GCs the store entry.
//
// POST /install/api/session/{id}/complete
//
// Named complete_ to avoid shadowing the h.complete field.
func (h *Handler) complete_(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if !h.checkCSRF(w, r) {
		return
	}

	entry, ok := h.store.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "session_not_found", "session not found or expired")
		return
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()

	data, err := entry.flow.CompleteAndPersist()
	if err != nil {
		writeError(w, mapFlowError(err), errorCode(err), err.Error())
		return
	}

	// GC: remove session from store after successful completion.
	h.store.Delete(id)

	writeJSON(w, http.StatusOK, completeResponse{
		CompletedAt: entry.flow.CompletedAt,
		Data:        *data,
	})
}

// ---------------------------------------------------------------------------
// CSRF and security helpers
// ---------------------------------------------------------------------------

// checkCSRF validates the double-submit CSRF pattern:
//  1. X-MINK-CSRF header must be present and non-empty.
//  2. mink_csrf cookie must be present and non-empty.
//  3. Header and cookie values must match (constant-time compare).
//  4. Origin header must be from 127.0.0.1 (or :5173 in dev mode).
//
// Returns true when all checks pass, false when a 403 has already been written.
func (h *Handler) checkCSRF(w http.ResponseWriter, r *http.Request) bool {
	// Origin check (defence-in-depth against CORS preflights from off-host pages).
	if err := h.verifyOrigin(r); err != nil {
		writeError(w, http.StatusForbidden, "csrf_failed", err.Error())
		return false
	}

	headerToken := r.Header.Get("X-MINK-CSRF")
	if headerToken == "" {
		writeError(w, http.StatusForbidden, "csrf_failed", "missing X-MINK-CSRF header")
		return false
	}

	cookie, err := r.Cookie("mink_csrf")
	if err != nil || cookie.Value == "" {
		writeError(w, http.StatusForbidden, "csrf_failed", "missing or empty mink_csrf cookie")
		return false
	}

	// Constant-time comparison to prevent timing oracle.
	if subtle.ConstantTimeCompare([]byte(headerToken), []byte(cookie.Value)) != 1 {
		writeError(w, http.StatusForbidden, "csrf_failed", "CSRF token mismatch")
		return false
	}

	return true
}

// verifyOrigin checks the Origin header against the allowed list.
// Returns nil when the origin is allowed (including empty/same-origin requests).
func (h *Handler) verifyOrigin(r *http.Request) error {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return nil
	}
	if !h.originAllowed(origin) {
		return fmt.Errorf("origin not allowed")
	}
	return nil
}

// verifyCsrfCookie checks that the mink_csrf cookie is present and matches expected.
// Returns nil on success. Used by SSE endpoint (cookie-only, no X-MINK-CSRF header).
func (h *Handler) verifyCsrfCookie(r *http.Request, expected string) error {
	cookie, err := r.Cookie("mink_csrf")
	if err != nil || cookie.Value == "" {
		return fmt.Errorf("missing or empty mink_csrf cookie")
	}
	if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(expected)) != 1 {
		return fmt.Errorf("CSRF token mismatch")
	}
	return nil
}

// originAllowed returns true when the Origin header value is on the allowlist.
// Allowlist: any http://127.0.0.1:* origin; additionally http://127.0.0.1:5173 in dev mode.
func (h *Handler) originAllowed(origin string) bool {
	// Always allow empty origin (same-origin requests from the embedded SPA).
	if origin == "" {
		return true
	}
	// Allow any 127.0.0.1 origin in production (backend always binds to loopback).
	if strings.HasPrefix(origin, "http://127.0.0.1:") {
		return true
	}
	// Dev mode: also allow localhost variants used by Vite.
	if h.devMode && (origin == "http://localhost:5173" || strings.HasPrefix(origin, "http://localhost:")) {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Step payload decoding
// ---------------------------------------------------------------------------

// decodeStepPayload decodes the request body into the expected type for step n.
// Returns an error for unknown steps, invalid JSON, or unknown JSON fields.
func decodeStepPayload(r *http.Request, n int) (any, error) {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	switch n {
	case 1:
		var v onboarding.LocaleChoice
		if err := dec.Decode(&v); err != nil {
			return nil, fmt.Errorf("step 1: %w", err)
		}
		return v, nil
	case 2:
		var v onboarding.ModelSetup
		if err := dec.Decode(&v); err != nil {
			return nil, fmt.Errorf("step 2: %w", err)
		}
		return v, nil
	case 3:
		var v onboarding.CLIToolsDetection
		if err := dec.Decode(&v); err != nil {
			return nil, fmt.Errorf("step 3: %w", err)
		}
		return v, nil
	case 4:
		var v onboarding.PersonaProfile
		if err := dec.Decode(&v); err != nil {
			return nil, fmt.Errorf("step 4: %w", err)
		}
		return v, nil
	case 5:
		var v onboarding.ProviderStepInput
		if err := dec.Decode(&v); err != nil {
			return nil, fmt.Errorf("step 5: %w", err)
		}
		return v, nil
	case 6:
		var v onboarding.MessengerChannel
		if err := dec.Decode(&v); err != nil {
			return nil, fmt.Errorf("step 6: %w", err)
		}
		return v, nil
	case 7:
		var v onboarding.ConsentFlags
		if err := dec.Decode(&v); err != nil {
			return nil, fmt.Errorf("step 7: %w", err)
		}
		return v, nil
	default:
		return nil, fmt.Errorf("unknown step %d", n)
	}
}

// ---------------------------------------------------------------------------
// Error mapping helpers
// ---------------------------------------------------------------------------

// mapFlowError maps OnboardingFlow sentinel errors to HTTP status codes.
func mapFlowError(err error) int {
	switch {
	case errors.Is(err, onboarding.ErrGDPRConsentRequired):
		return http.StatusUnprocessableEntity
	case errors.Is(err, onboarding.ErrStepOutOfOrder):
		return http.StatusBadRequest
	case errors.Is(err, onboarding.ErrStepRange):
		return http.StatusBadRequest
	case errors.Is(err, onboarding.ErrStepDataMismatch):
		return http.StatusBadRequest
	case errors.Is(err, onboarding.ErrCannotGoBack):
		return http.StatusBadRequest
	case errors.Is(err, onboarding.ErrNotReadyToComplete):
		return http.StatusBadRequest
	case errors.Is(err, onboarding.ErrPersistFailed):
		return http.StatusInternalServerError
	case errors.Is(err, onboarding.ErrMarkerFailed):
		return http.StatusInternalServerError
	default:
		return http.StatusBadRequest
	}
}

// errorCode maps OnboardingFlow sentinel errors to short machine-readable codes.
func errorCode(err error) string {
	switch {
	case errors.Is(err, onboarding.ErrGDPRConsentRequired):
		return "gdpr_consent_required"
	case errors.Is(err, onboarding.ErrStepOutOfOrder):
		return "step_out_of_order"
	case errors.Is(err, onboarding.ErrStepRange):
		return "step_range"
	case errors.Is(err, onboarding.ErrStepDataMismatch):
		return "step_data_mismatch"
	case errors.Is(err, onboarding.ErrCannotGoBack):
		return "cannot_go_back"
	case errors.Is(err, onboarding.ErrNotReadyToComplete):
		return "not_ready_to_complete"
	case errors.Is(err, onboarding.ErrPersistFailed):
		return "persist_failed"
	case errors.Is(err, onboarding.ErrMarkerFailed):
		return "marker_failed"
	default:
		return "validation_failed"
	}
}

// ---------------------------------------------------------------------------
// Response helpers
// ---------------------------------------------------------------------------

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error envelope with the given status, code, and message.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: errorBody{Code: code, Message: message}})
}

// ---------------------------------------------------------------------------
// Crypto helpers
// ---------------------------------------------------------------------------

// generateToken returns a 32-character lowercase hex string from 16 random bytes.
func generateToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is fatal; panic is appropriate here.
		panic(fmt.Sprintf("install: crypto/rand failure: %v", err))
	}
	return hex.EncodeToString(b)
}
