package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/modu-ai/mink/internal/audit"
	"github.com/modu-ai/mink/internal/permission"
	"github.com/modu-ai/mink/internal/tools"
	"github.com/modu-ai/mink/internal/tools/web/common"
)

const (
	// mapsProductionBase is the Nominatim OSM API root.
	// REQ-WEB-003: User-Agent header is mandatory for all Nominatim requests.
	mapsProductionBase = "https://nominatim.openstreetmap.org"

	// mapsHost is the hostname used for blocklist and permission checks.
	mapsHost = "nominatim.openstreetmap.org"
)

// mapsSchema is the JSON Schema for the web_maps tool input.
// Per plan.md §5.3 and AC-WEB-015.
var mapsSchema = json.RawMessage(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["operation"],
  "properties": {
    "operation": {
      "type": "string",
      "enum": ["geocode", "reverse"]
    },
    "query": {
      "type": "string",
      "minLength": 1,
      "maxLength": 500
    },
    "lat": {
      "type": "number",
      "minimum": -90,
      "maximum": 90
    },
    "lon": {
      "type": "number",
      "minimum": -180,
      "maximum": 180
    }
  }
}`)

// mapsInput is the parsed input for a web_maps call.
type mapsInput struct {
	Operation string  `json:"operation"`
	Query     string  `json:"query,omitempty"`
	Lat       float64 `json:"lat,omitempty"`
	Lon       float64 `json:"lon,omitempty"`
}

// mapsSearchItem mirrors a single result from the Nominatim /search endpoint.
// Nominatim returns lat/lon as JSON strings, so we parse them manually.
type mapsSearchItem struct {
	LatStr      string          `json:"lat"`
	LonStr      string          `json:"lon"`
	DisplayName string          `json:"display_name"`
	Importance  float64         `json:"importance"`
	Type        string          `json:"type"`
	Address     json.RawMessage `json:"address"`
}

// mapsGeocodeResult is the normalised single entry in the geocode results array.
type mapsGeocodeResult struct {
	Lat         float64         `json:"lat"`
	Lon         float64         `json:"lon"`
	DisplayName string          `json:"display_name"`
	Importance  float64         `json:"importance"`
	Type        string          `json:"type"`
	Address     json.RawMessage `json:"address"`
}

// mapsReverseResponse mirrors the Nominatim /reverse endpoint JSON shape.
type mapsReverseResponse struct {
	DisplayName string          `json:"display_name"`
	Address     json.RawMessage `json:"address"`
}

// mapsGeocodeData is the geocode operation output payload.
type mapsGeocodeData struct {
	Operation string              `json:"operation"`
	Results   []mapsGeocodeResult `json:"results"`
}

// mapsReverseData is the reverse operation output payload.
type mapsReverseData struct {
	Operation   string          `json:"operation"`
	DisplayName string          `json:"display_name"`
	Address     json.RawMessage `json:"address"`
}

// webMaps implements the web_maps tool.
//
// @MX:ANCHOR: [AUTO] web_maps tool — Nominatim OSM geocode/reverse lookup
// @MX:REASON: SPEC-GOOSE-TOOLS-WEB-001 AC-WEB-015 — fan_in >= 3 (tests + bootstrap + executor)
type webMaps struct {
	deps           *common.Deps
	apiBaseBuilder apiBaseBuilder
}

// Name returns the canonical tool name used in the Registry.
func (w *webMaps) Name() string { return "web_maps" }

// Schema returns the JSON Schema that the Executor uses for input validation.
func (w *webMaps) Schema() json.RawMessage { return mapsSchema }

// Scope returns ScopeShared — web_maps is available to all agent types.
func (w *webMaps) Scope() tools.Scope { return tools.ScopeShared }

// Call executes the web_maps pipeline.
//
// Pipeline:
//  1. Parse + defensive schema guard
//  2. Build Nominatim URL based on operation (geocode / reverse)
//  3. Blocklist + permission gate (host = nominatim.openstreetmap.org)
//  4. HTTP GET with User-Agent (required by Nominatim policy)
//  5. Unmarshal JSON, normalise lat/lon strings to float64
//  6. writeAudit + return
func (w *webMaps) Call(ctx context.Context, raw json.RawMessage) (tools.ToolResult, error) {
	start := w.deps.Now()

	in, err := parseMapsInput(raw)
	if err != nil {
		return toToolResult(common.ErrResponse("invalid_input", err.Error(), false, 0, elapsed(start))), nil
	}

	base := w.apiBaseBuilder()

	// The blocklist and permission gate always use the canonical production host
	// regardless of the injected base URL. This mirrors the arxiv pattern: the
	// mock server URL is only used for the outbound HTTP call, not for security
	// decisions.
	host := mapsHost

	// Blocklist (pre-permission, AC-WEB-009).
	if w.deps.Blocklist != nil && w.deps.Blocklist.IsBlocked(host) {
		w.writeAudit(ctx, host, 0, "denied", "host_blocked", start)
		return toToolResult(common.ErrResponse("host_blocked",
			fmt.Sprintf("host %q is blocked by security policy", host),
			false, 0, elapsed(start))), nil
	}

	// Permission gate.
	if w.deps.PermMgr != nil {
		subjectID := w.deps.SubjectID(ctx)
		req := permission.PermissionRequest{
			SubjectID:   subjectID,
			SubjectType: permission.SubjectAgent,
			Capability:  permission.CapNet,
			Scope:       host,
			RequestedAt: start,
		}
		dec, checkErr := w.deps.PermMgr.Check(ctx, req)
		if checkErr != nil || !dec.Allow {
			msg := "permission denied"
			if checkErr != nil {
				msg = checkErr.Error()
			}
			w.writeAudit(ctx, host, 0, "denied", "permission_denied", start)
			return toToolResult(common.ErrResponse("permission_denied", msg, false, 0, elapsed(start))), nil
		}
	}

	// Build target URL.
	target := w.buildMapsURL(base, in)

	httpReq, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if reqErr != nil {
		return toToolResult(common.ErrResponse("fetch_failed", reqErr.Error(), true, 0, elapsed(start))), nil
	}
	httpReq.Header.Set("User-Agent", common.UserAgent())
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, fetchErr := client.Do(httpReq)
	if fetchErr != nil {
		w.writeAudit(ctx, host, 0, "error", "fetch_failed", start)
		return toToolResult(common.ErrResponse("fetch_failed", fetchErr.Error(), true, 0, elapsed(start))), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		w.writeAudit(ctx, host, resp.StatusCode, "error", "fetch_failed", start)
		return toToolResult(common.ErrResponse("fetch_failed",
			fmt.Sprintf("Nominatim returned HTTP %d", resp.StatusCode),
			resp.StatusCode >= 500, 0, elapsed(start))), nil
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, common.MaxResponseBytes))
	if readErr != nil {
		w.writeAudit(ctx, host, resp.StatusCode, "error", "read_error", start)
		return toToolResult(common.ErrResponse("fetch_failed", readErr.Error(), true, 0, elapsed(start))), nil
	}

	w.writeAudit(ctx, host, resp.StatusCode, "ok", "", start)

	var out common.Response
	var marshalErr error

	switch in.Operation {
	case "geocode":
		out, marshalErr = w.buildGeocodeResponse(body, elapsed(start))
	default: // "reverse"
		out, marshalErr = w.buildReverseResponse(body, elapsed(start))
	}

	if marshalErr != nil {
		return toToolResult(common.ErrResponse("fetch_failed", marshalErr.Error(), false, 0, elapsed(start))), nil
	}
	return toToolResult(out), nil
}

// buildMapsURL constructs the Nominatim API URL for the given operation.
func (w *webMaps) buildMapsURL(base string, in mapsInput) string {
	params := url.Values{}
	params.Set("format", "json")
	params.Set("addressdetails", "1")

	if in.Operation == "geocode" {
		params.Set("q", in.Query)
		params.Set("limit", "10")
		return base + "/search?" + params.Encode()
	}
	// reverse
	params.Set("lat", strconv.FormatFloat(in.Lat, 'f', -1, 64))
	params.Set("lon", strconv.FormatFloat(in.Lon, 'f', -1, 64))
	return base + "/reverse?" + params.Encode()
}

// buildGeocodeResponse parses the Nominatim /search JSON array and returns
// a normalised common.Response. lat/lon are converted from strings to float64.
func (w *webMaps) buildGeocodeResponse(body []byte, meta common.Metadata) (common.Response, error) {
	var items []mapsSearchItem
	if err := json.Unmarshal(body, &items); err != nil {
		return common.Response{}, fmt.Errorf("decode Nominatim geocode response: %w", err)
	}

	results := make([]mapsGeocodeResult, 0, len(items))
	for _, item := range items {
		lat, err := strconv.ParseFloat(item.LatStr, 64)
		if err != nil {
			lat = 0
		}
		lon, err := strconv.ParseFloat(item.LonStr, 64)
		if err != nil {
			lon = 0
		}
		results = append(results, mapsGeocodeResult{
			Lat:         lat,
			Lon:         lon,
			DisplayName: item.DisplayName,
			Importance:  item.Importance,
			Type:        item.Type,
			Address:     item.Address,
		})
	}

	return common.OKResponse(mapsGeocodeData{Operation: "geocode", Results: results}, meta)
}

// buildReverseResponse parses the Nominatim /reverse JSON object and returns
// a normalised common.Response.
func (w *webMaps) buildReverseResponse(body []byte, meta common.Metadata) (common.Response, error) {
	var rev mapsReverseResponse
	if err := json.Unmarshal(body, &rev); err != nil {
		return common.Response{}, fmt.Errorf("decode Nominatim reverse response: %w", err)
	}

	return common.OKResponse(mapsReverseData{
		Operation:   "reverse",
		DisplayName: rev.DisplayName,
		Address:     rev.Address,
	}, meta)
}

// writeAudit records a single audit event for the call.
func (w *webMaps) writeAudit(_ context.Context, host string, statusCode int,
	outcome, reason string, start time.Time,
) {
	if w.deps.AuditWriter == nil {
		return
	}
	meta := map[string]string{
		"tool":        "web_maps",
		"host":        host,
		"method":      http.MethodGet,
		"status_code": fmt.Sprintf("%d", statusCode),
		"cache_hit":   "false",
		"duration_ms": fmt.Sprintf("%d", w.deps.Now().Sub(start).Milliseconds()),
		"outcome":     outcome,
	}
	if reason != "" {
		meta["reason"] = reason
	}
	ev := audit.NewAuditEvent(w.deps.Now(), audit.EventTypeToolWebInvoke, audit.SeverityInfo,
		"web_maps invoked", meta)
	_ = w.deps.AuditWriter.Write(ev)
}

// parseMapsInput parses the JSON payload and applies defensive guards.
// operation enum, geocode-requires-query, reverse-requires-lat+lon, range checks.
func parseMapsInput(raw json.RawMessage) (mapsInput, error) {
	var in mapsInput
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			return mapsInput{}, err
		}
	}

	switch in.Operation {
	case "geocode":
		if len(in.Query) < 1 || len(in.Query) > 500 {
			return mapsInput{}, fmt.Errorf("geocode requires query with length 1..500")
		}
	case "reverse":
		if in.Lat < -90 || in.Lat > 90 {
			return mapsInput{}, fmt.Errorf("lat %g out of range [-90, 90]", in.Lat)
		}
		if in.Lon < -180 || in.Lon > 180 {
			return mapsInput{}, fmt.Errorf("lon %g out of range [-180, 180]", in.Lon)
		}
	default:
		return mapsInput{}, fmt.Errorf("operation %q must be one of: geocode, reverse", in.Operation)
	}

	return in, nil
}

// NewMapsForTest constructs a web_maps tool with an injected apiBaseBuilder
// so tests can redirect requests through an httptest.Server.
func NewMapsForTest(deps *common.Deps, builder apiBaseBuilder) tools.Tool {
	if builder == nil {
		builder = func() string { return mapsProductionBase }
	}
	return &webMaps{deps: deps, apiBaseBuilder: builder}
}

// init registers web_maps in the global web tools list.
func init() {
	RegisterWebTool(&webMaps{
		deps:           &common.Deps{},
		apiBaseBuilder: func() string { return mapsProductionBase },
	})
}
