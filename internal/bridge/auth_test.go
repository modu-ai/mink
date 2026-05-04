// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-002, REQ-BR-014
// AC: AC-BR-002, AC-BR-012
// M1-T1, M1-T2, M1-T4 — HMAC session cookie + CSRF + auth rejection.

package bridge

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
)

func newTestAuth(t *testing.T) (*Authenticator, clockwork.FakeClock) {
	t.Helper()
	clk := clockwork.NewFakeClockAt(time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC))
	a, err := NewAuthenticator(AuthConfig{
		HMACSecret: []byte("test-secret-32-bytes-padding-xx!"),
		Clock:      clk,
	})
	if err != nil {
		t.Fatalf("NewAuthenticator err = %v", err)
	}
	return a, clk
}

func TestNewAuthenticator_RejectsShortSecret(t *testing.T) {
	t.Parallel()
	_, err := NewAuthenticator(AuthConfig{HMACSecret: []byte("too-short")})
	if err == nil {
		t.Fatalf("NewAuthenticator(short secret) err = nil, want non-nil")
	}
}

func TestNewAuthenticator_GeneratesSecretWhenAbsent(t *testing.T) {
	t.Parallel()
	a, err := NewAuthenticator(AuthConfig{})
	if err != nil {
		t.Fatalf("NewAuthenticator(no secret) err = %v, want nil (auto-generate)", err)
	}
	if a == nil {
		t.Fatalf("NewAuthenticator returned nil")
	}
}

func TestIssueAndVerifySessionCookie_Roundtrip(t *testing.T) {
	t.Parallel()

	a, clk := newTestAuth(t)
	cookie, expiresAt, err := a.IssueSessionCookie()
	if err != nil {
		t.Fatalf("IssueSessionCookie err = %v", err)
	}
	if cookie == "" {
		t.Fatalf("IssueSessionCookie returned empty cookie")
	}
	want := clk.Now().Add(24 * time.Hour)
	if !expiresAt.Equal(want) {
		t.Errorf("expiresAt = %v, want %v (+24h)", expiresAt, want)
	}

	sid, gotExp, err := a.VerifySessionCookie(cookie)
	if err != nil {
		t.Fatalf("VerifySessionCookie err = %v", err)
	}
	if sid == "" {
		t.Errorf("VerifySessionCookie returned empty sessionID")
	}
	if !gotExp.Equal(expiresAt) {
		t.Errorf("verified expiresAt = %v, want %v", gotExp, expiresAt)
	}
}

func TestVerifySessionCookie_RejectsAfterExpiry(t *testing.T) {
	t.Parallel()

	a, clk := newTestAuth(t)
	cookie, _, err := a.IssueSessionCookie()
	if err != nil {
		t.Fatalf("issue err = %v", err)
	}

	clk.Advance(24*time.Hour + time.Second)

	_, _, err = a.VerifySessionCookie(cookie)
	if !errors.Is(err, ErrCookieExpired) {
		t.Fatalf("VerifySessionCookie after 24h err = %v, want ErrCookieExpired", err)
	}
}

func TestVerifySessionCookie_RejectsTamperedHMAC(t *testing.T) {
	t.Parallel()

	a, _ := newTestAuth(t)
	cookie, _, _ := a.IssueSessionCookie()

	// Flip the last character of the HMAC segment.
	parts := strings.Split(cookie, ".")
	if len(parts) != 2 {
		t.Fatalf("cookie format unexpected: %q", cookie)
	}
	hmacBytes := []byte(parts[1])
	if hmacBytes[len(hmacBytes)-1] == 'A' {
		hmacBytes[len(hmacBytes)-1] = 'B'
	} else {
		hmacBytes[len(hmacBytes)-1] = 'A'
	}
	tampered := parts[0] + "." + string(hmacBytes)

	_, _, err := a.VerifySessionCookie(tampered)
	if !errors.Is(err, ErrCookieInvalid) {
		t.Fatalf("VerifySessionCookie(tampered) err = %v, want ErrCookieInvalid", err)
	}
}

func TestVerifySessionCookie_RejectsTamperedPayload(t *testing.T) {
	t.Parallel()

	a, _ := newTestAuth(t)
	cookie, _, _ := a.IssueSessionCookie()

	parts := strings.Split(cookie, ".")
	payloadBytes := []byte(parts[0])
	payloadBytes[0] ^= 0x01
	tampered := string(payloadBytes) + "." + parts[1]

	_, _, err := a.VerifySessionCookie(tampered)
	if !errors.Is(err, ErrCookieInvalid) {
		t.Fatalf("VerifySessionCookie(tampered payload) err = %v, want ErrCookieInvalid", err)
	}
}

func TestVerifySessionCookie_RejectsMalformed(t *testing.T) {
	t.Parallel()

	a, _ := newTestAuth(t)
	cases := []string{
		"",
		"only-one-segment",
		"a.b.c.d",
		"!!!.???",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			t.Parallel()
			_, _, err := a.VerifySessionCookie(c)
			if !errors.Is(err, ErrCookieInvalid) {
				t.Fatalf("VerifySessionCookie(%q) err = %v, want ErrCookieInvalid", c, err)
			}
		})
	}
}

func TestVerifySessionCookie_DifferentSecretRejects(t *testing.T) {
	t.Parallel()

	a1, _ := newTestAuth(t)
	cookie, _, _ := a1.IssueSessionCookie()

	a2, err := NewAuthenticator(AuthConfig{
		HMACSecret: []byte("different-secret-xx-padding-321!"),
	})
	if err != nil {
		t.Fatalf("NewAuthenticator a2 err = %v", err)
	}
	_, _, err = a2.VerifySessionCookie(cookie)
	if !errors.Is(err, ErrCookieInvalid) {
		t.Fatalf("verify with different secret err = %v, want ErrCookieInvalid", err)
	}
}

func TestCookieHash_StableForSameCookie(t *testing.T) {
	t.Parallel()

	a, _ := newTestAuth(t)
	cookie, _, _ := a.IssueSessionCookie()
	h1 := a.CookieHash(cookie)
	h2 := a.CookieHash(cookie)
	if len(h1) == 0 {
		t.Fatalf("CookieHash returned empty")
	}
	if string(h1) != string(h2) {
		t.Errorf("CookieHash not deterministic")
	}

	// Different cookie → different hash.
	cookie2, _, _ := a.IssueSessionCookie()
	h3 := a.CookieHash(cookie2)
	if string(h1) == string(h3) {
		t.Errorf("distinct cookies produced identical hash")
	}
}

func TestIssueCSRFToken_NonEmptyAndUnique(t *testing.T) {
	t.Parallel()

	a, _ := newTestAuth(t)
	t1, err := a.IssueCSRFToken()
	if err != nil {
		t.Fatalf("IssueCSRFToken err = %v", err)
	}
	t2, _ := a.IssueCSRFToken()
	if t1 == "" {
		t.Fatalf("CSRF token empty")
	}
	if t1 == t2 {
		t.Errorf("CSRF tokens collide")
	}
}

func TestVerifyCSRFToken_AcceptsExactMatch(t *testing.T) {
	t.Parallel()
	a, _ := newTestAuth(t)
	tok, _ := a.IssueCSRFToken()
	if !a.VerifyCSRFToken(tok, tok) {
		t.Errorf("VerifyCSRFToken(equal) = false, want true")
	}
}

func TestVerifyCSRFToken_RejectsMismatch(t *testing.T) {
	t.Parallel()
	a, _ := newTestAuth(t)
	if a.VerifyCSRFToken("aaaa", "bbbb") {
		t.Errorf("VerifyCSRFToken(mismatch) = true, want false")
	}
}

func TestVerifyCSRFToken_RejectsEmpty(t *testing.T) {
	t.Parallel()
	a, _ := newTestAuth(t)
	if a.VerifyCSRFToken("", "") {
		t.Errorf("VerifyCSRFToken(empty,empty) = true, want false")
	}
	if a.VerifyCSRFToken("aaa", "") {
		t.Errorf("VerifyCSRFToken(non-empty,empty) = true, want false")
	}
}
