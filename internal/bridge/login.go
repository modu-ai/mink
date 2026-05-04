// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-002, REQ-BR-014, REQ-BR-016 (v0.2.0)
// AC: AC-BR-002, AC-BR-012, AC-BR-014
// M1-T3, M1-T4, M1-T5 — login / logout HTTP handlers + auth rejection helper.

package bridge

import (
	"encoding/json"
	"net/http"
	"strings"
)

const (
	// SessionCookieName is the browser-visible session cookie key.
	SessionCookieName = "goose_session"

	// CSRFCookieName is the readable CSRF cookie (not HttpOnly so the
	// browser-side script can include it as the X-CSRF-Token header).
	CSRFCookieName = "csrf_token"

	// CSRFHeaderName is the inbound header carrying the double-submit token.
	CSRFHeaderName = "X-CSRF-Token"
)

// loginIntent enumerates the accepted bodies for POST /bridge/login.
// WEBUI v0.2.1 OI-A specifies "first_install" or "resume".
type loginIntent string

const (
	loginIntentFirstInstall loginIntent = "first_install"
	loginIntentResume       loginIntent = "resume"
)

type loginRequest struct {
	Intent loginIntent `json:"intent"`
}

type loginResponse struct {
	CSRFToken string `json:"csrf_token"`
	ExpiresAt string `json:"expires_at"` // RFC 3339
}

// LoginHandler is the http.Handler for POST /bridge/login.
// On success it Set-Cookies goose_session (HttpOnly, SameSite=Strict, 24h)
// and csrf_token, then returns 200 with { csrf_token, expires_at }.
//
// Method other than POST → 405. Malformed body or unknown intent → 400.
type LoginHandler struct {
	auth *Authenticator
}

// NewLoginHandler binds a LoginHandler to the given Authenticator.
func NewLoginHandler(auth *Authenticator) *LoginHandler {
	return &LoginHandler{auth: auth}
}

func (h *LoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}

	var req loginRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	switch req.Intent {
	case loginIntentFirstInstall, loginIntentResume:
		// accepted
	default:
		writeJSONError(w, http.StatusBadRequest, "invalid_intent")
		return
	}

	cookie, expiresAt, err := h.auth.IssueSessionCookie()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "cookie_issue_failed")
		return
	}
	csrf, err := h.auth.IssueCSRFToken()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "csrf_issue_failed")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    cookie,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // loopback HTTP only; spec.md §10 explicitly excludes TLS
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(cookieLifetime.Seconds()),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    csrf,
		Path:     "/",
		HttpOnly: false, // double-submit pattern requires JS access
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(cookieLifetime.Seconds()),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(loginResponse{
		CSRFToken: csrf,
		ExpiresAt: expiresAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	})
}

// LogoutHandler is the http.Handler for POST /bridge/logout.
// Reads the session cookie, marks its hash as revoked, and closes any
// active sessions (registered Closers) bound to that cookie hash with
// CloseSessionRevoked (4403). Always returns 204 even when the cookie is
// missing or already invalid — logout is idempotent and must not leak
// session state through differential responses.
type LogoutHandler struct {
	auth       *Authenticator
	registry   *Registry
	revocation *RevocationStore
}

// NewLogoutHandler wires the dependencies for /bridge/logout.
func NewLogoutHandler(auth *Authenticator, registry *Registry, revocation *RevocationStore) *LogoutHandler {
	return &LogoutHandler{auth: auth, registry: registry, revocation: revocation}
}

func (h *LogoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}

	c, err := r.Cookie(SessionCookieName)
	if err == nil && c != nil && c.Value != "" {
		hash := h.auth.CookieHash(c.Value)
		h.revocation.Revoke(hash)
		h.registry.CloseSessionsByCookieHash(hash, CloseSessionRevoked)
	}

	// Clear browser-side cookies regardless of validity.
	clearCookie(w, SessionCookieName)
	clearCookie(w, CSRFCookieName)
	w.WriteHeader(http.StatusNoContent)
}

// AuthError classifies the reason a request was rejected.
type AuthError struct {
	Reason string // unauthenticated | csrf_mismatch | revoked | bad_origin
}

func (e *AuthError) Error() string { return "bridge: auth rejected: " + e.Reason }

// AuthRequest validates the session cookie + CSRF token + (optional) Origin
// against the stored revocation state. Returns the decoded session ID +
// cookie hash on success. The returned AuthError signals which dimension
// failed; transport-specific rejection (close 4401 vs HTTP 401) is the
// caller's responsibility.
func AuthRequest(r *http.Request, auth *Authenticator, revocation *RevocationStore, requireCSRF bool) (sessionID string, cookieHash []byte, err *AuthError) {
	c, cerr := r.Cookie(SessionCookieName)
	if cerr != nil || c == nil || c.Value == "" {
		return "", nil, &AuthError{Reason: "unauthenticated"}
	}
	sid, _, verr := auth.VerifySessionCookie(c.Value)
	if verr != nil {
		return "", nil, &AuthError{Reason: "unauthenticated"}
	}
	hash := auth.CookieHash(c.Value)
	if revocation != nil && revocation.IsRevoked(hash) {
		return "", nil, &AuthError{Reason: "revoked"}
	}
	if requireCSRF {
		csrfCookie, err := r.Cookie(CSRFCookieName)
		if err != nil || csrfCookie == nil {
			return "", nil, &AuthError{Reason: "csrf_mismatch"}
		}
		header := r.Header.Get(CSRFHeaderName)
		if !auth.VerifyCSRFToken(csrfCookie.Value, header) {
			return "", nil, &AuthError{Reason: "csrf_mismatch"}
		}
	}
	if !isLoopbackOrigin(r) {
		return "", nil, &AuthError{Reason: "bad_origin"}
	}
	return sid, hash, nil
}

// isLoopbackOrigin returns true when the request's Host and (if present)
// Origin headers both point to a loopback alias. spec.md §6.4 item 2.
func isLoopbackOrigin(r *http.Request) bool {
	if !hostIsLoopback(r.Host) {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // Origin header is optional on same-origin POST
	}
	// Strip "scheme://" prefix.
	if i := strings.Index(origin, "://"); i >= 0 {
		origin = origin[i+3:]
	}
	return hostIsLoopback(origin)
}

func hostIsLoopback(hostport string) bool {
	host := hostport
	if i := strings.LastIndex(hostport, ":"); i > 0 && !strings.HasPrefix(hostport, "[") {
		host = hostport[:i]
	} else if strings.HasPrefix(hostport, "[") {
		// IPv6 form: [::1]:port
		end := strings.Index(hostport, "]")
		if end > 0 {
			host = hostport[1:end]
		}
	}
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return true
	}
	return false
}

func writeJSONError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: name == SessionCookieName,
		SameSite: http.SameSiteStrictMode,
	})
}
