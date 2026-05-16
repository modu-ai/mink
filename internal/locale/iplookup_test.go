package locale

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTransport replaces ipLookupHTTPClient.Transport in tests.
// Each test restores the original client via t.Cleanup.

func setFakeIPAPIServer(t *testing.T, srv *httptest.Server) {
	t.Helper()
	origURL := ipAPIBaseURL
	origClient := ipLookupHTTPClient
	ipAPIBaseURL = srv.URL
	ipLookupHTTPClient = srv.Client()
	t.Cleanup(func() {
		ipAPIBaseURL = origURL
		ipLookupHTTPClient = origClient
	})
}

// ipapiSuccessBody builds a minimal ipapi.co-style JSON response.
func ipapiSuccessBody(country, timezone, languages string) []byte {
	b, _ := json.Marshal(ipapiResponse{
		CountryCode: country,
		Timezone:    timezone,
		Languages:   languages,
	})
	return b
}

// TestLookupIP_HappyPath_US verifies a successful US lookup.
func TestLookupIP_HappyPath_US(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(ipapiSuccessBody("US", "America/New_York", "en"))
	}))
	defer srv.Close()
	setFakeIPAPIServer(t, srv)

	result, err := LookupIP(context.Background(), LookupIPInput{ClientIP: "8.8.8.8"})
	require.NoError(t, err)
	assert.Equal(t, "US", result.Country)
	assert.Equal(t, "America/New_York", result.Timezone)
	assert.Equal(t, "en", result.Language)
}

// TestLookupIP_PrivateIPRejected_RFC1918 verifies that RFC 1918 and special addresses
// are rejected without making any external call.
func TestLookupIP_PrivateIPRejected_RFC1918(t *testing.T) {
	// These addresses must never reach the fake server.
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	setFakeIPAPIServer(t, srv)

	cases := []string{
		"10.0.0.1",    // RFC 1918 class A
		"172.16.0.1",  // RFC 1918 class B
		"192.168.1.1", // RFC 1918 class C
		"127.0.0.1",   // loopback IPv4
		"169.254.1.1", // link-local IPv4
		"100.64.0.1",  // CGNAT
		"::1",         // loopback IPv6
		"fc00::1",     // ULA IPv6
		"fe80::1",     // link-local IPv6
	}

	for _, ip := range cases {
		t.Run(ip, func(t *testing.T) {
			_, err := LookupIP(context.Background(), LookupIPInput{ClientIP: ip})
			assert.ErrorIs(t, err, ErrPrivateIP, "expected ErrPrivateIP for %s", ip)
		})
	}

	assert.Equal(t, 0, callCount, "private IPs must not reach external server")
}

// TestLookupIP_Timeout verifies that a cancelled context produces ErrLookupTimeout.
func TestLookupIP_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Stall long enough for the context to cancel.
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	setFakeIPAPIServer(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := LookupIP(ctx, LookupIPInput{ClientIP: "8.8.8.8"})
	assert.ErrorIs(t, err, ErrLookupTimeout)
}

// TestLookupIP_HTTP500 verifies that a non-2xx response produces ErrLookupHTTP.
func TestLookupIP_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	setFakeIPAPIServer(t, srv)

	_, err := LookupIP(context.Background(), LookupIPInput{ClientIP: "8.8.8.8"})
	assert.ErrorIs(t, err, ErrLookupHTTP)
}

// TestLookupIP_ParseError verifies that invalid JSON produces ErrLookupParse.
func TestLookupIP_ParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{not valid json"))
	}))
	defer srv.Close()
	setFakeIPAPIServer(t, srv)

	_, err := LookupIP(context.Background(), LookupIPInput{ClientIP: "8.8.8.8"})
	assert.ErrorIs(t, err, ErrLookupParse)
}

// TestLookupIP_LanguageBestEffort verifies that a missing languages field returns an
// empty Language string rather than an error.
func TestLookupIP_LanguageBestEffort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(ipapiSuccessBody("KR", "Asia/Seoul", ""))
	}))
	defer srv.Close()
	setFakeIPAPIServer(t, srv)

	result, err := LookupIP(context.Background(), LookupIPInput{ClientIP: "1.1.1.1"})
	require.NoError(t, err)
	assert.Equal(t, "KR", result.Country)
	assert.Empty(t, result.Language, "missing languages field must return empty string, not error")
}

// TestLookupIP_TimezoneIANA verifies that the timezone field is forwarded as-is.
func TestLookupIP_TimezoneIANA(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(ipapiSuccessBody("JP", "Asia/Tokyo", "ja"))
	}))
	defer srv.Close()
	setFakeIPAPIServer(t, srv)

	result, err := LookupIP(context.Background(), LookupIPInput{ClientIP: "203.0.113.1"})
	require.NoError(t, err)
	assert.Equal(t, "Asia/Tokyo", result.Timezone)
	assert.Equal(t, "JP", result.Country)
}

// TestLookupIP_EmptyClientIP verifies that an empty IP is treated as private.
func TestLookupIP_EmptyClientIP(t *testing.T) {
	_, err := LookupIP(context.Background(), LookupIPInput{ClientIP: ""})
	assert.ErrorIs(t, err, ErrPrivateIP)
}

// TestLookupIP_UnparseableIP verifies that a garbage IP string is treated as private.
func TestLookupIP_UnparseableIP(t *testing.T) {
	_, err := LookupIP(context.Background(), LookupIPInput{ClientIP: "not-an-ip"})
	assert.ErrorIs(t, err, ErrPrivateIP)
}

// TestPrimaryLanguageTag_MultipleLanguages verifies that only the first tag is returned.
func TestPrimaryLanguageTag_MultipleLanguages(t *testing.T) {
	assert.Equal(t, "ko", primaryLanguageTag("ko,en"))
	assert.Equal(t, "en", primaryLanguageTag("en"))
	assert.Equal(t, "", primaryLanguageTag(""))
}
