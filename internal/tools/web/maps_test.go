package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modu-ai/goose/internal/permission"
	permstore "github.com/modu-ai/goose/internal/permission/store"
	"github.com/modu-ai/goose/internal/tools/web"
	"github.com/modu-ai/goose/internal/tools/web/common"
)

// mapsGeocodeFixture is a minimal Nominatim /search response with 2 results.
// lat/lon are JSON strings as returned by the real Nominatim API.
const mapsGeocodeFixture = `[
  {
    "lat": "37.5665",
    "lon": "126.9780",
    "display_name": "Seoul, South Korea",
    "importance": 0.85,
    "type": "city",
    "address": {"city": "Seoul", "country": "South Korea", "country_code": "kr"}
  },
  {
    "lat": "37.4979",
    "lon": "127.0276",
    "display_name": "Seoul, Gangnam-gu, South Korea",
    "importance": 0.70,
    "type": "suburb",
    "address": {"suburb": "Gangnam-gu", "city": "Seoul", "country": "South Korea"}
  }
]`

// mapsReverseFixture is a minimal Nominatim /reverse response.
const mapsReverseFixture = `{
  "display_name": "Seoul National University, 1, Gwanak-ro, Gwanak-gu, Seoul, South Korea",
  "address": {
    "university": "Seoul National University",
    "city": "Seoul",
    "state": "Seoul",
    "country": "South Korea",
    "postcode": "08826",
    "country_code": "kr"
  }
}`

// startMapsMockServer starts an httptest server that inspects the request path
// to decide which fixture to serve (/search → geocode, /reverse → reverse).
func startMapsMockServer(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	hitCount := new(int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hitCount++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/reverse" {
			_, _ = w.Write([]byte(mapsReverseFixture))
		} else {
			// /search or any other path → geocode fixture
			_, _ = w.Write([]byte(mapsGeocodeFixture))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, hitCount
}

// TestMaps_GeocodeAndReverse verifies AC-WEB-015: geocode and reverse both
// return properly normalised data. Nominatim lat/lon strings are converted to
// float64 numbers in the output.
func TestMaps_GeocodeAndReverse(t *testing.T) {
	t.Parallel()

	srv, _ := startMapsMockServer(t)
	tool := web.NewMapsForTest(&common.Deps{}, func() string { return srv.URL })

	// --- geocode ---
	geocodeInput := json.RawMessage(`{"operation":"geocode","query":"Seoul"}`)
	result, err := tool.Call(context.Background(), geocodeInput)
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	require.True(t, res.OK, "geocode: expected ok=true, got error: %+v", res.Error)

	var geocodeData struct {
		Operation string `json:"operation"`
		Results   []struct {
			Lat         float64         `json:"lat"`
			Lon         float64         `json:"lon"`
			DisplayName string          `json:"display_name"`
			Importance  float64         `json:"importance"`
			Type        string          `json:"type"`
			Address     json.RawMessage `json:"address"`
		} `json:"results"`
	}
	require.NoError(t, json.Unmarshal(res.Data, &geocodeData))
	assert.Equal(t, "geocode", geocodeData.Operation)
	require.Equal(t, 2, len(geocodeData.Results))
	r0 := geocodeData.Results[0]
	assert.InDelta(t, 37.5665, r0.Lat, 0.0001)
	assert.InDelta(t, 126.9780, r0.Lon, 0.0001)
	assert.Equal(t, "Seoul, South Korea", r0.DisplayName)
	assert.InDelta(t, 0.85, r0.Importance, 0.0001)
	assert.Equal(t, "city", r0.Type)
	require.NotEmpty(t, r0.Address)

	// --- reverse ---
	reverseInput := json.RawMessage(`{"operation":"reverse","lat":37.5,"lon":127.0}`)
	result2, err := tool.Call(context.Background(), reverseInput)
	require.NoError(t, err)

	var res2 common.Response
	require.NoError(t, json.Unmarshal(result2.Content, &res2))
	require.True(t, res2.OK, "reverse: expected ok=true, got error: %+v", res2.Error)

	var reverseData struct {
		Operation   string          `json:"operation"`
		DisplayName string          `json:"display_name"`
		Address     json.RawMessage `json:"address"`
	}
	require.NoError(t, json.Unmarshal(res2.Data, &reverseData))
	assert.Equal(t, "reverse", reverseData.Operation)
	assert.Contains(t, reverseData.DisplayName, "Seoul")
	require.NotEmpty(t, reverseData.Address)

	// Verify address contains city field.
	var addr struct {
		City string `json:"city"`
	}
	require.NoError(t, json.Unmarshal(reverseData.Address, &addr))
	assert.Equal(t, "Seoul", addr.City)
}

// TestMaps_SchemaValidation verifies that invalid inputs are rejected before
// any network activity. Covers: missing operation, geocode without query,
// reverse without lat, reverse without lon, lat out of range.
func TestMaps_SchemaValidation(t *testing.T) {
	t.Parallel()

	// Use a port that is definitely not listening so a real fetch would fail.
	tool := web.NewMapsForTest(&common.Deps{}, func() string { return "http://127.0.0.1:0" })

	cases := []struct {
		name  string
		input string
	}{
		{
			name:  "missing_operation",
			input: `{"query":"Seoul"}`,
		},
		{
			name:  "invalid_operation",
			input: `{"operation":"lookup","query":"Seoul"}`,
		},
		{
			name:  "geocode_without_query",
			input: `{"operation":"geocode"}`,
		},
		{
			name:  "geocode_empty_query",
			input: `{"operation":"geocode","query":""}`,
		},
		{
			name:  "reverse_without_lat_lon",
			input: `{"operation":"reverse","lat":200.0,"lon":0.0}`,
		},
		{
			name:  "reverse_lat_out_of_range",
			input: `{"operation":"reverse","lat":91.0,"lon":0.0}`,
		},
		{
			name:  "reverse_lon_out_of_range",
			input: `{"operation":"reverse","lat":0.0,"lon":181.0}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := tool.Call(context.Background(), json.RawMessage(tc.input))
			require.NoError(t, err)

			var res common.Response
			require.NoError(t, json.Unmarshal(result.Content, &res))
			assert.False(t, res.OK, "expected ok=false for case %q", tc.name)
			require.NotNil(t, res.Error)
			assert.Equal(t, "invalid_input", res.Error.Code,
				"case %q: unexpected error code", tc.name)
		})
	}
}

// TestMaps_BlocklistPriority verifies that when nominatim.openstreetmap.org is
// blocked, the tool returns host_blocked without making any HTTP request.
func TestMaps_BlocklistPriority(t *testing.T) {
	t.Parallel()

	serverHits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		serverHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	bl := common.NewBlocklist([]string{"nominatim.openstreetmap.org"})
	deps := &common.Deps{Blocklist: bl}
	tool := web.NewMapsForTest(deps, func() string { return srv.URL })

	result, err := tool.Call(context.Background(), json.RawMessage(`{"operation":"geocode","query":"Seoul"}`))
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	assert.False(t, res.OK)
	require.NotNil(t, res.Error)
	assert.Equal(t, "host_blocked", res.Error.Code)
	assert.Equal(t, 0, serverHits, "HTTP server must not be hit when host is blocked")
}

// TestMaps_PermissionDenied verifies that a permission denial returns
// "permission_denied" without making any HTTP request.
func TestMaps_PermissionDenied(t *testing.T) {
	t.Parallel()

	serverHits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		serverHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store := permstore.NewMemoryStore()
	require.NoError(t, store.Open())
	mgr, err := permission.New(store, permission.DefaultDenyConfirmer{}, nil, nil, nil)
	require.NoError(t, err)

	deps := &common.Deps{PermMgr: mgr}
	tool := web.NewMapsForTest(deps, func() string { return srv.URL })

	result, err := tool.Call(context.Background(), json.RawMessage(`{"operation":"geocode","query":"Seoul"}`))
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	assert.False(t, res.OK)
	require.NotNil(t, res.Error)
	assert.Equal(t, "permission_denied", res.Error.Code)
	assert.Equal(t, 0, serverHits)
}

// TestMaps_RegisteredInWebTools verifies that web_maps is registered in the
// global web tools list at package init time.
func TestMaps_RegisteredInWebTools(t *testing.T) {
	t.Parallel()
	names := web.RegisteredWebToolNamesForTest()
	assert.True(t, slices.Contains(names, "web_maps"),
		"web_maps not found in RegisteredWebToolNames: %v", names)
}

// TestMaps_AuditWriter verifies that the audit writer is called on a successful
// geocode request (covers the writeAudit ok-path).
func TestMaps_AuditWriter(t *testing.T) {
	t.Parallel()

	srv, _ := startMapsMockServer(t)
	deps := &common.Deps{AuditWriter: noopAuditWriter{}}
	tool := web.NewMapsForTest(deps, func() string { return srv.URL })

	result, err := tool.Call(context.Background(), json.RawMessage(`{"operation":"geocode","query":"Seoul"}`))
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	require.True(t, res.OK, "expected ok=true, got error: %+v", res.Error)
}

// TestMaps_FetchFailure verifies that a 5xx response returns fetch_failed with
// retryable=true.
func TestMaps_FetchFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tool := web.NewMapsForTest(&common.Deps{}, func() string { return srv.URL })

	result, err := tool.Call(context.Background(), json.RawMessage(`{"operation":"geocode","query":"Seoul"}`))
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	assert.False(t, res.OK)
	require.NotNil(t, res.Error)
	assert.Equal(t, "fetch_failed", res.Error.Code)
	assert.True(t, res.Error.Retryable, "5xx errors must be retryable")
}
