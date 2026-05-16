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

// setFakeNominatimServer injects a fake Nominatim server into the package-level
// variables and restores originals via t.Cleanup. Mirror of setFakeIPAPIServer.
func setFakeNominatimServer(t *testing.T, srv *httptest.Server) {
	t.Helper()
	origURL := reverseGeocodeBaseURL
	origClient := reverseGeocodeHTTPClient
	reverseGeocodeBaseURL = srv.URL
	reverseGeocodeHTTPClient = srv.Client()
	t.Cleanup(func() {
		reverseGeocodeBaseURL = origURL
		reverseGeocodeHTTPClient = origClient
	})
}

// nominatimBody builds a minimal Nominatim-style JSON response with the given country_code.
func nominatimBody(countryCode string) []byte {
	b, _ := json.Marshal(nominatimResponse{
		Address: nominatimAddress{CountryCode: countryCode},
	})
	return b
}

// TestReverseGeocode_Seoul verifies a successful lookup for Seoul, South Korea.
func TestReverseGeocode_Seoul(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(nominatimBody("kr"))
	}))
	defer srv.Close()
	setFakeNominatimServer(t, srv)

	result, err := ReverseGeocode(context.Background(), ReverseGeocodeInput{Lat: 37.5, Lng: 127.0})
	require.NoError(t, err)
	assert.Equal(t, "KR", result.Country, "country_code must be uppercased")
	assert.Equal(t, "ko", result.Language, "KR must map to ko")
	assert.Equal(t, "Asia/Seoul", result.Timezone, "KR primary timezone must be Asia/Seoul")
}

// TestReverseGeocode_NewYork verifies a successful lookup for New York, USA.
func TestReverseGeocode_NewYork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(nominatimBody("us"))
	}))
	defer srv.Close()
	setFakeNominatimServer(t, srv)

	result, err := ReverseGeocode(context.Background(), ReverseGeocodeInput{Lat: 40.7, Lng: -74.0})
	require.NoError(t, err)
	assert.Equal(t, "US", result.Country)
	assert.Equal(t, "en", result.Language, "US must map to en")
	assert.Equal(t, "America/New_York", result.Timezone, "US primary timezone must be America/New_York")
}

// TestReverseGeocode_InvalidLat verifies that latitude > 90 returns ErrReverseInvalidCoords
// without making any HTTP call.
func TestReverseGeocode_InvalidLat(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	setFakeNominatimServer(t, srv)

	_, err := ReverseGeocode(context.Background(), ReverseGeocodeInput{Lat: 91.0, Lng: 0.0})
	assert.ErrorIs(t, err, ErrReverseInvalidCoords)
	assert.Equal(t, 0, callCount, "invalid lat must not reach the external server")
}

// TestReverseGeocode_InvalidLng verifies that longitude > 180 returns ErrReverseInvalidCoords
// without making any HTTP call.
func TestReverseGeocode_InvalidLng(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	setFakeNominatimServer(t, srv)

	_, err := ReverseGeocode(context.Background(), ReverseGeocodeInput{Lat: 0.0, Lng: 181.0})
	assert.ErrorIs(t, err, ErrReverseInvalidCoords)
	assert.Equal(t, 0, callCount, "invalid lng must not reach the external server")
}

// TestReverseGeocode_HTTP500 verifies that a non-2xx status returns ErrReverseHTTP.
func TestReverseGeocode_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	setFakeNominatimServer(t, srv)

	_, err := ReverseGeocode(context.Background(), ReverseGeocodeInput{Lat: 37.5, Lng: 127.0})
	assert.ErrorIs(t, err, ErrReverseHTTP)
}

// TestReverseGeocode_InvalidJSON verifies that malformed JSON returns ErrReverseParse.
func TestReverseGeocode_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{not valid json"))
	}))
	defer srv.Close()
	setFakeNominatimServer(t, srv)

	_, err := ReverseGeocode(context.Background(), ReverseGeocodeInput{Lat: 37.5, Lng: 127.0})
	assert.ErrorIs(t, err, ErrReverseParse)
}

// TestReverseGeocode_ContextDeadline verifies that a cancelled context returns ErrReverseTimeout.
func TestReverseGeocode_ContextDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Stall long enough for the context to cancel.
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	setFakeNominatimServer(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := ReverseGeocode(ctx, ReverseGeocodeInput{Lat: 37.5, Lng: 127.0})
	assert.ErrorIs(t, err, ErrReverseTimeout)
}

// TestReverseGeocode_EmptyCountryCode verifies that an empty country_code field
// in an otherwise valid JSON response returns ErrReverseParse.
func TestReverseGeocode_EmptyCountryCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Valid JSON but country_code is absent (zero-value "").
		_, _ = w.Write(nominatimBody(""))
	}))
	defer srv.Close()
	setFakeNominatimServer(t, srv)

	_, err := ReverseGeocode(context.Background(), ReverseGeocodeInput{Lat: 37.5, Lng: 127.0})
	assert.ErrorIs(t, err, ErrReverseParse, "empty country_code must return ErrReverseParse")
}

// TestReverseGeocode_UserAgentHeader verifies that the Nominatim User-Agent policy
// header is sent on every request.
func TestReverseGeocode_UserAgentHeader(t *testing.T) {
	var capturedUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(nominatimBody("de"))
	}))
	defer srv.Close()
	setFakeNominatimServer(t, srv)

	_, err := ReverseGeocode(context.Background(), ReverseGeocodeInput{Lat: 52.5, Lng: 13.4})
	require.NoError(t, err)
	assert.Contains(t, capturedUA, "MINK", "User-Agent must identify MINK per Nominatim policy")
}
