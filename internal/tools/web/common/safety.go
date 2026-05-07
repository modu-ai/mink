package common

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// MaxResponseBytes is the hard response body size limit (10 MB).
	// REQ-WEB-012: abort and return response_too_large when exceeded.
	MaxResponseBytes = 10 * 1024 * 1024

	// MaxRedirects is the maximum allowed redirect count per request.
	// The schema enforces max_redirects <= 10; this constant matches.
	MaxRedirects = 10
)

// ErrResponseTooLarge is returned when an HTTP response body exceeds MaxResponseBytes.
var ErrResponseTooLarge = errors.New("response_too_large")

// ErrTooManyRedirects is the sentinel returned by the redirect guard when
// the configured cap is reached or exceeded.
var ErrTooManyRedirects = errors.New("too_many_redirects")

// Blocklist holds a set of host patterns used to block outbound requests.
// Patterns are either exact hostnames or glob patterns starting with "*.".
//
// @MX:ANCHOR: [AUTO] Pre-permission host blocklist for all web tools
// @MX:REASON: SPEC-GOOSE-TOOLS-WEB-001 AC-WEB-009 — checked before permission.Manager.Check; fan_in >= 2
type Blocklist struct {
	exact map[string]struct{}
	globs []string // patterns of the form "*.suffix"
}

// NewBlocklist constructs a Blocklist from a slice of patterns.
// Patterns beginning with "*." are treated as subdomain globs.
// All other patterns are treated as exact hostname matches.
func NewBlocklist(patterns []string) *Blocklist {
	bl := &Blocklist{exact: make(map[string]struct{})}
	for _, p := range patterns {
		if strings.HasPrefix(p, "*.") {
			bl.globs = append(bl.globs, p[1:]) // store ".suffix" for suffix matching
		} else {
			bl.exact[p] = struct{}{}
		}
	}
	return bl
}

// IsBlocked reports whether the given hostname matches any entry in the blocklist.
// Glob patterns match any subdomain of the suffix (e.g. "*.evil.com" matches
// "sub.evil.com" but not "evil.com" itself).
func (bl *Blocklist) IsBlocked(host string) bool {
	if _, ok := bl.exact[host]; ok {
		return true
	}
	for _, suffix := range bl.globs {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

// NewRedirectGuard returns an http.Client.CheckRedirect function that aborts
// when the number of followed redirects exceeds cap.
// cap must be in [0, MaxRedirects]; panics on out-of-range values.
func NewRedirectGuard(cap int) func(req *http.Request, via []*http.Request) error {
	if cap > MaxRedirects {
		panic("common.NewRedirectGuard: cap exceeds MaxRedirects (10)")
	}
	return func(req *http.Request, via []*http.Request) error {
		if len(via) > cap {
			return ErrTooManyRedirects
		}
		return nil
	}
}

// LimitedRead reads at most MaxResponseBytes from r.
// If the body exceeds MaxResponseBytes, it returns ErrResponseTooLarge.
// The caller is responsible for closing r.
func LimitedRead(r io.Reader) ([]byte, error) {
	// Read up to MaxResponseBytes + 1 to detect if the limit is exceeded.
	limited := io.LimitReader(r, int64(MaxResponseBytes)+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read limited response body: %w", err)
	}
	if len(data) > MaxResponseBytes {
		return nil, ErrResponseTooLarge
	}
	return data, nil
}
