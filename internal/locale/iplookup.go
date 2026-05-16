package locale

// iplookup.go provides IP geolocation via ipapi.co.
//
// SPEC: SPEC-MINK-LOCALE-001 amendment-v0.2
// REQ: REQ-LC-041, REQ-LC-042

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// Sentinel errors returned by LookupIP.
var (
	// ErrPrivateIP is returned when the caller IP is a private/loopback/link-local
	// address. No external lookup is performed.
	ErrPrivateIP = errors.New("locale: private/loopback IP, skipping external lookup")

	// ErrLookupTimeout is returned when the upstream ipapi.co call exceeds 3s.
	ErrLookupTimeout = errors.New("locale: ip lookup timeout")

	// ErrLookupHTTP is returned when ipapi.co responds with a non-2xx status code.
	ErrLookupHTTP = errors.New("locale: ip lookup non-2xx response")

	// ErrLookupParse is returned when the response body cannot be decoded as JSON.
	ErrLookupParse = errors.New("locale: ip lookup response decode failed")
)

// ipLookupHTTPClient is the HTTP client used by LookupIP. Tests may replace this
// variable with a custom client (pointing to an httptest.Server) before calling
// LookupIP. The replacement must be restored via t.Cleanup.
//
// @MX:WARN: [AUTO] Package-level mutable variable used for test injection of HTTP transport.
// @MX:REASON: Replacing a package-level var is the standard Go pattern for injecting
// HTTP transport in unit tests without changing the exported function signature.
var ipLookupHTTPClient = &http.Client{Timeout: 3 * time.Second}

// ipAPIBaseURL is the base URL for ipapi.co. Tests may replace this to point at
// an httptest.Server. Must be restored via t.Cleanup.
var ipAPIBaseURL = "https://ipapi.co"

// privateIPv4Nets lists RFC 1918 / loopback / link-local / CGNAT IPv4 CIDR blocks
// that must not be forwarded to external lookup services.
var privateIPv4Nets []*net.IPNet

// privateIPv6Nets lists loopback, ULA, and link-local IPv6 CIDR blocks that must
// not be forwarded to external lookup services.
var privateIPv6Nets []*net.IPNet

func init() {
	// Build private CIDR sets at package init so parsing errors surface early.
	for _, cidr := range []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"100.64.0.0/10", // CGNAT
	} {
		_, net, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("locale: bad private IPv4 CIDR %q: %v", cidr, err))
		}
		privateIPv4Nets = append(privateIPv4Nets, net)
	}

	for _, cidr := range []string{
		"::1/128",   // loopback
		"fc00::/7",  // ULA
		"fe80::/10", // link-local
	} {
		_, net, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("locale: bad private IPv6 CIDR %q: %v", cidr, err))
		}
		privateIPv6Nets = append(privateIPv6Nets, net)
	}
}

// LookupIPInput specifies what we know about the caller.
type LookupIPInput struct {
	// ClientIP is the first-hop public IP (X-Forwarded-For first untrusted hop or RemoteAddr).
	ClientIP string
}

// LookupIPResult is what ipapi.co returned, normalised to LocaleContext fields.
type LookupIPResult struct {
	// Country is the ISO 3166-1 alpha-2 code (e.g., "KR", "US").
	Country string

	// Language is the BCP 47 primary tag (best-effort; empty when unavailable).
	Language string

	// Timezone is the IANA timezone identifier (best-effort; empty when unavailable).
	Timezone string
}

// isPrivateIP returns true when ip falls within any RFC 1918, loopback,
// link-local, or CGNAT range. Both IPv4 and IPv6 are checked.
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		for _, n := range privateIPv4Nets {
			if n.Contains(v4) {
				return true
			}
		}
		return false
	}
	// IPv6
	for _, n := range privateIPv6Nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// ipapiResponse mirrors the relevant fields from ipapi.co JSON response.
type ipapiResponse struct {
	CountryCode string `json:"country_code"` // ISO 3166-1 alpha-2
	Timezone    string `json:"timezone"`     // IANA timezone
	Languages   string `json:"languages"`    // comma-separated BCP 47 tags
}

// LookupIP probes ipapi.co for the caller's country, language, and timezone.
//
// Returns ErrPrivateIP for RFC 1918/loopback/link-local addresses (no external call made).
// Returns ErrLookupTimeout when the upstream call exceeds 3s.
// Returns ErrLookupHTTP for non-2xx responses.
// Returns ErrLookupParse when the response body cannot be decoded.
//
// @MX:ANCHOR: [AUTO] IP geolocation entry point for LOCALE-001 Phase 2 backend probe.
// @MX:REASON: Called by handler.go /install/api/locale/probe endpoint; indirected via
// ipLookupHTTPClient and ipAPIBaseURL for test injection.
func LookupIP(ctx context.Context, in LookupIPInput) (LookupIPResult, error) {
	// Normalise and validate the IP.
	ipStr := strings.TrimSpace(in.ClientIP)
	if ipStr == "" {
		return LookupIPResult{}, ErrPrivateIP
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		// Unparseable address — treat as private to avoid leaking garbage to vendor.
		return LookupIPResult{}, ErrPrivateIP
	}

	if isPrivateIP(ip) {
		return LookupIPResult{}, ErrPrivateIP
	}

	// Build the request against ipapi.co.
	url := ipAPIBaseURL + "/json/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return LookupIPResult{}, fmt.Errorf("%w: build request: %v", ErrLookupHTTP, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := ipLookupHTTPClient.Do(req)
	if err != nil {
		// Check for context deadline/timeout.
		if ctx.Err() != nil {
			return LookupIPResult{}, ErrLookupTimeout
		}
		// HTTP client timeout (from ipLookupHTTPClient.Timeout) surfaces as a
		// url.Error with Timeout() == true.
		return LookupIPResult{}, ErrLookupTimeout
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return LookupIPResult{}, fmt.Errorf("%w: status %d", ErrLookupHTTP, resp.StatusCode)
	}

	var body ipapiResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return LookupIPResult{}, fmt.Errorf("%w: %v", ErrLookupParse, err)
	}

	result := LookupIPResult{
		Country:  strings.ToUpper(body.CountryCode),
		Timezone: body.Timezone,
		Language: primaryLanguageTag(body.Languages),
	}
	return result, nil
}

// primaryLanguageTag extracts the first BCP 47 language tag from the comma-separated
// languages field returned by ipapi.co (e.g., "ko,en" → "ko"). Returns "" when the
// field is empty or cannot be parsed.
func primaryLanguageTag(languages string) string {
	if languages == "" {
		return ""
	}
	parts := strings.SplitN(languages, ",", 2)
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}
