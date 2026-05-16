// Package install — sse.go implements the Server-Sent Events endpoint that
// streams ollama pull progress from the Go server to the React frontend.
//
// Route: GET /install/api/session/{id}/pull/stream
//
// Security model (SSE-specific):
//   - EventSource API cannot set custom headers; X-MINK-CSRF header is therefore
//     NOT required for this GET endpoint.
//   - Defense in depth: (a) Origin header allowlist (same rules as POST endpoints),
//     (b) mink_csrf cookie value verified against the session's csrfToken (SameSite=Strict).
//   - The browser sends cookies automatically on EventSource requests to the same origin.
//
// Concurrency:
//   - One goroutine per connection runs pullFn (PullModel).
//   - The select loop in the handler goroutine forwards progress to the client.
//   - Client disconnect is detected via r.Context().Done().
//   - Buffered progress channel (size 16) avoids blocking the stdout scanner.
//
// SPEC: SPEC-MINK-ONBOARDING-001 Phase 3C
// REQ: REQ-OB-034, REQ-OB-035
package install

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/modu-ai/mink/internal/onboarding"
)

// sseProgressChannelSize is the buffer size for the progress channel passed to
// PullModel. A buffer of 16 prevents the stdout scanner goroutine from blocking
// while the HTTP write + Flush is in progress.
//
// @MX:NOTE: [AUTO] 16 is a deliberate tradeoff: large enough to absorb burst lines
// from ollama stdout, small enough to bound memory per connection.
const sseProgressChannelSize = 16

// pullStream is the SSE handler that runs PullModel and forwards progress to the client.
//
// @MX:ANCHOR: [AUTO] pullStream is the single HTTP entry point for SSE ollama pull progress.
// @MX:REASON: Wired into mux in NewHandler; changes to auth logic, SSE framing, or
// concurrency model here affect every Web UI pull operation.
func (h *Handler) pullStream(w http.ResponseWriter, r *http.Request) {
	// --- 1. Origin check ---
	if err := h.verifyOrigin(r); err != nil {
		writeError(w, http.StatusForbidden, "csrf_failed", err.Error())
		return
	}

	// --- 2. Session lookup ---
	id := r.PathValue("id")
	entry, ok := h.store.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "session_not_found", "session not found or expired")
		return
	}

	// --- 3. Cookie-only CSRF verification ---
	// Acquire entry.mu to read csrfToken and update lastActivityAt atomically.
	entry.mu.Lock()
	expectedToken := entry.csrfToken
	entry.lastActivityAt = h.store.clock()
	entry.mu.Unlock()

	if err := h.verifyCsrfCookie(r, expectedToken); err != nil {
		writeError(w, http.StatusForbidden, "csrf_failed", err.Error())
		return
	}

	// --- 4. Resolve model name ---
	// Priority: ?model= query param > flow.Data.Model.SelectedModel > DetectedModel > 400.
	modelName := r.URL.Query().Get("model")
	if modelName == "" {
		entry.mu.Lock()
		modelName = entry.flow.Data.Model.SelectedModel
		if modelName == "" {
			modelName = entry.flow.Data.Model.DetectedModel
		}
		entry.mu.Unlock()
	}
	if modelName == "" {
		writeError(w, http.StatusBadRequest, "model_required", "model name is required: provide ?model= or set SelectedModel in session")
		return
	}

	// --- 5. Assert Flusher ---
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal", "streaming not supported by this server")
		return
	}

	// --- 6. Set SSE headers and write 200 status ---
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering if present
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// --- 7. Spawn PullModel goroutine ---
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	progress := make(chan onboarding.ProgressUpdate, sseProgressChannelSize)
	errCh := make(chan error, 1)

	// @MX:WARN: [AUTO] Goroutine lifecycle tied to client connection via ctx cancellation.
	// @MX:REASON: If cancel() is not called on all exit paths (client disconnect, error,
	// or normal completion) the PullModel goroutine may block indefinitely on channel send.
	go func() {
		errCh <- h.pullFn(ctx, modelName, progress)
	}()

	// --- 8. Select loop: forward progress to client ---
	for {
		select {
		case <-r.Context().Done():
			// Client disconnected; cancel PullModel and exit.
			cancel()
			return

		case update, open := <-progress:
			if !open {
				// Channel closed: PullModel finished (success or error).
				// Wait for the error result.
				pullErr := <-errCh
				if pullErr != nil {
					writeSSEEvent(w, "error", map[string]string{"message": pullErr.Error()})
				} else {
					writeSSEEvent(w, "done", map[string]bool{"ok": true})
				}
				flusher.Flush()
				return
			}
			// Normal progress update: marshal and emit as a data event.
			if err := writeSSEData(w, update); err != nil {
				// Write failure means the client disconnected mid-stream.
				cancel()
				return
			}
			flusher.Flush()

		case pullErr := <-errCh:
			// PullModel returned an error without closing the channel first (rare path).
			// Drain any remaining updates then emit the error event.
			drainProgress(w, progress, flusher)
			if pullErr != nil {
				writeSSEEvent(w, "error", map[string]string{"message": pullErr.Error()})
			} else {
				writeSSEEvent(w, "done", map[string]bool{"ok": true})
			}
			flusher.Flush()
			return
		}
	}
}

// ---------------------------------------------------------------------------
// SSE framing helpers
// ---------------------------------------------------------------------------

// writeSSEData emits a single SSE "data:" event containing the JSON-encoded
// ProgressUpdate. Returns an error if the write fails (client disconnected).
func writeSSEData(w http.ResponseWriter, update onboarding.ProgressUpdate) error {
	b, err := json.Marshal(update)
	if err != nil {
		// Marshal of ProgressUpdate should never fail; treat as internal error.
		return fmt.Errorf("sse: marshal failed: %w", err)
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", b)
	return err
}

// writeSSEEvent emits a named SSE event (event: <name>\ndata: <json>\n\n).
// payload must be JSON-serialisable. Write errors are silently discarded because
// the connection may already be closed.
func writeSSEEvent(w http.ResponseWriter, event string, payload any) {
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b) //nolint:errcheck
}

// drainProgress reads and emits any remaining items on the progress channel
// without blocking (non-blocking drain via default case).
func drainProgress(w http.ResponseWriter, progress <-chan onboarding.ProgressUpdate, flusher http.Flusher) {
	for {
		select {
		case update, open := <-progress:
			if !open {
				return
			}
			_ = writeSSEData(w, update)
			flusher.Flush()
		default:
			return
		}
	}
}
