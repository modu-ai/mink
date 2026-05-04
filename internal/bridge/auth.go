// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-002, REQ-BR-014
// AC: AC-BR-002, AC-BR-012
// M1-T1, M1-T2 — HMAC session cookie + CSRF double-submit.

package bridge

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jonboulle/clockwork"
)

// Sentinel errors returned by VerifySessionCookie. Distinguishable via errors.Is.
var (
	// ErrCookieInvalid signals a malformed cookie, HMAC mismatch, or any other
	// non-expiry rejection. Used as the catch-all rejection cause for caller
	// classification (4401 / 401).
	ErrCookieInvalid = errors.New("bridge: session cookie invalid")

	// ErrCookieExpired signals a cookie whose payload-encoded expiry has passed.
	// Distinguished from ErrCookieInvalid because the cookie was authentic but
	// has aged out of the 24h window (REQ-BR-002).
	ErrCookieExpired = errors.New("bridge: session cookie expired")
)

// Cookie/CSRF byte budgets.
const (
	sessionIDLen      = 16               // bytes
	cookiePayloadLen  = sessionIDLen + 8 // sessionID || expiresAt(uint64 nanos)
	csrfTokenBytes    = 32
	cookieLifetime    = 24 * time.Hour
	minHMACSecretLen  = 32 // bytes — bound to SHA-256 block size derivation
	authSecretRandLen = 32 // auto-generated secret length when caller omits one
)

// AuthConfig parametrises the Authenticator constructor.
type AuthConfig struct {
	// HMACSecret is the keying material for cookie signing. If empty, a
	// 32-byte secret is generated from crypto/rand. If non-empty, it MUST
	// be at least 32 bytes long.
	HMACSecret []byte

	// Clock controls the time source for issue / expiry checks. Zero defaults
	// to clockwork.NewRealClock().
	Clock clockwork.Clock
}

// Authenticator issues and verifies HMAC-signed session cookies and CSRF
// double-submit tokens.
//
// @MX:ANCHOR
// @MX:WARN HMAC misuse risk; secret material is only kept in memory.
// @MX:REASON Crypto primitive boundary; misuse breaks REQ-BR-002 and REQ-BR-014.
type Authenticator struct {
	secret []byte
	clock  clockwork.Clock
}

// NewAuthenticator constructs an Authenticator. Returns an error if
// HMACSecret is non-empty but shorter than 32 bytes.
func NewAuthenticator(cfg AuthConfig) (*Authenticator, error) {
	secret := cfg.HMACSecret
	if len(secret) == 0 {
		secret = make([]byte, authSecretRandLen)
		if _, err := rand.Read(secret); err != nil {
			return nil, fmt.Errorf("bridge: HMAC secret generation failed: %w", err)
		}
	} else if len(secret) < minHMACSecretLen {
		return nil, fmt.Errorf("bridge: HMAC secret too short (%d bytes, need >=%d)",
			len(secret), minHMACSecretLen)
	}
	clk := cfg.Clock
	if clk == nil {
		clk = clockwork.NewRealClock()
	}
	return &Authenticator{
		secret: append([]byte(nil), secret...),
		clock:  clk,
	}, nil
}

// IssueSessionCookie returns a base64url-encoded cookie of the form
// "payload.hmac" along with the absolute expiry time (now + 24h).
func (a *Authenticator) IssueSessionCookie() (cookie string, expiresAt time.Time, err error) {
	sid := make([]byte, sessionIDLen)
	if _, err := rand.Read(sid); err != nil {
		return "", time.Time{}, fmt.Errorf("bridge: session id rand: %w", err)
	}
	now := a.clock.Now()
	expiresAt = now.Add(cookieLifetime)

	payload := make([]byte, cookiePayloadLen)
	copy(payload[:sessionIDLen], sid)
	binary.BigEndian.PutUint64(payload[sessionIDLen:], uint64(expiresAt.UnixNano()))

	mac := a.sign(payload)

	cookie = base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(mac)
	return cookie, expiresAt, nil
}

// VerifySessionCookie parses and authenticates a cookie. Returns the
// session ID and expiry on success. Returns ErrCookieInvalid for malformed
// or HMAC-mismatched cookies, ErrCookieExpired for valid-but-aged cookies.
func (a *Authenticator) VerifySessionCookie(cookie string) (sessionID string, expiresAt time.Time, err error) {
	parts := strings.Split(cookie, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", time.Time{}, fmt.Errorf("%w: bad segment count", ErrCookieInvalid)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(payload) != cookiePayloadLen {
		return "", time.Time{}, fmt.Errorf("%w: payload decode", ErrCookieInvalid)
	}
	mac, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", time.Time{}, fmt.Errorf("%w: mac decode", ErrCookieInvalid)
	}

	expected := a.sign(payload)
	if !hmac.Equal(expected, mac) {
		return "", time.Time{}, fmt.Errorf("%w: hmac mismatch", ErrCookieInvalid)
	}

	expiresAt = time.Unix(0, int64(binary.BigEndian.Uint64(payload[sessionIDLen:])))
	if !a.clock.Now().Before(expiresAt) {
		return "", time.Time{}, fmt.Errorf("%w: at %s", ErrCookieExpired, expiresAt.UTC())
	}

	sessionID = base64.RawURLEncoding.EncodeToString(payload[:sessionIDLen])
	return sessionID, expiresAt, nil
}

// CookieHash returns SHA-256(cookie) — used as the registry-side cookie
// reference so the raw cookie value is never stored or logged
// (spec.md §6.4 item 4).
func (a *Authenticator) CookieHash(cookie string) []byte {
	h := sha256.Sum256([]byte(cookie))
	return h[:]
}

// IssueCSRFToken returns a fresh 32-byte random token, base64url-encoded.
// The same token is delivered both as a cookie and inside the response body;
// the client echoes it back in the X-CSRF-Token header (double-submit).
func (a *Authenticator) IssueCSRFToken() (string, error) {
	b := make([]byte, csrfTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("bridge: csrf token rand: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// VerifyCSRFToken compares two CSRF token values in constant time. Empty
// inputs always return false to prevent trivial-bypass on missing headers.
func (a *Authenticator) VerifyCSRFToken(cookieValue, headerValue string) bool {
	if cookieValue == "" || headerValue == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookieValue), []byte(headerValue)) == 1
}

func (a *Authenticator) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, a.secret)
	mac.Write(payload)
	return mac.Sum(nil)
}
