package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/modu-ai/goose/internal/audit"
	"github.com/modu-ai/goose/internal/permission"
	"github.com/modu-ai/goose/internal/tools"
	"github.com/modu-ai/goose/internal/tools/web/common"
)

const (
	// waybackProductionBase is the Wayback Machine availability API root.
	waybackProductionBase = "https://archive.org"

	// waybackHost is the hostname used for blocklist and permission checks.
	waybackHost = "archive.org"
)

// waybackTimestampPattern mirrors the schema's timestamp regex for early input
// rejection before network activity.
var waybackTimestampPattern = regexp.MustCompile(`^[0-9]{14}$`)

// waybackSchema is the JSON Schema for the web_wayback tool input.
// Per plan.md §5.4 and AC-WEB-016.
var waybackSchema = json.RawMessage(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["url"],
  "properties": {
    "url": {
      "type": "string",
      "format": "uri",
      "pattern": "^https?://"
    },
    "timestamp": {
      "type": "string",
      "pattern": "^[0-9]{14}$"
    }
  }
}`)

// waybackInput is the parsed input for a web_wayback call.
type waybackInput struct {
	URL       string `json:"url"`
	Timestamp string `json:"timestamp,omitempty"`
}

// waybackClosest mirrors the "closest" snapshot object in the Wayback API response.
type waybackClosest struct {
	Available bool   `json:"available"`
	URL       string `json:"url"`
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
}

// waybackAPIResponse mirrors the top-level Wayback availability API response.
type waybackAPIResponse struct {
	ArchivedSnapshots struct {
		Closest *waybackClosest `json:"closest"`
	} `json:"archived_snapshots"`
}

// waybackData is the successful output payload for web_wayback.
type waybackData struct {
	SnapshotURL string `json:"snapshot_url"`
	Timestamp   string `json:"timestamp"`
	Status      string `json:"status"` // "available" | "unavailable"
}

// webWayback implements the web_wayback tool.
//
// @MX:ANCHOR: [AUTO] web_wayback tool — Wayback Machine snapshot availability check
// @MX:REASON: SPEC-GOOSE-TOOLS-WEB-001 AC-WEB-016 — fan_in >= 3 (tests + bootstrap + executor)
type webWayback struct {
	deps           *common.Deps
	apiBaseBuilder apiBaseBuilder
}

// Name returns the canonical tool name used in the Registry.
func (w *webWayback) Name() string { return "web_wayback" }

// Schema returns the JSON Schema that the Executor uses for input validation.
func (w *webWayback) Schema() json.RawMessage { return waybackSchema }

// Scope returns ScopeShared — web_wayback is available to all agent types.
func (w *webWayback) Scope() tools.Scope { return tools.ScopeShared }

// Call executes the web_wayback pipeline.
//
// Pipeline:
//  1. Parse + defensive schema guard (url scheme, timestamp pattern)
//  2. Build Wayback API URL (optional timestamp parameter)
//  3. Blocklist + permission gate (host = archive.org)
//  4. HTTP GET with User-Agent
//  5. Unmarshal JSON → normalise to waybackData with status "available"|"unavailable"
//  6. writeAudit + return
func (w *webWayback) Call(ctx context.Context, raw json.RawMessage) (tools.ToolResult, error) {
	start := w.deps.Now()

	in, err := parseWaybackInput(raw)
	if err != nil {
		return toToolResult(common.ErrResponse("invalid_input", err.Error(), false, 0, elapsed(start))), nil
	}

	base := w.apiBaseBuilder()

	// The blocklist and permission gate always use the canonical production host
	// regardless of the injected base URL. This mirrors the arxiv pattern: the
	// mock server URL is only used for the outbound HTTP call, not for security
	// decisions.
	host := waybackHost

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
	target := w.buildWaybackURL(base, in)

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
			fmt.Sprintf("Wayback Machine returned HTTP %d", resp.StatusCode),
			resp.StatusCode >= 500, 0, elapsed(start))), nil
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, common.MaxResponseBytes))
	if readErr != nil {
		w.writeAudit(ctx, host, resp.StatusCode, "error", "read_error", start)
		return toToolResult(common.ErrResponse("fetch_failed", readErr.Error(), true, 0, elapsed(start))), nil
	}

	var apiResp waybackAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		w.writeAudit(ctx, host, resp.StatusCode, "error", "decode_error", start)
		return toToolResult(common.ErrResponse("fetch_failed",
			fmt.Sprintf("decode Wayback response: %v", err), false, 0, elapsed(start))), nil
	}

	data := waybackData{
		SnapshotURL: "",
		Timestamp:   "",
		Status:      "unavailable",
	}
	if c := apiResp.ArchivedSnapshots.Closest; c != nil && c.Available {
		data.SnapshotURL = c.URL
		data.Timestamp = c.Timestamp
		data.Status = "available"
	}

	w.writeAudit(ctx, host, resp.StatusCode, "ok", "", start)
	out, marshalErr := common.OKResponse(data, common.Metadata{
		CacheHit:   false,
		DurationMs: elapsed(start).DurationMs,
	})
	if marshalErr != nil {
		return toToolResult(common.ErrResponse("internal_error", marshalErr.Error(), false, 0, elapsed(start))), nil
	}
	return toToolResult(out), nil
}

// buildWaybackURL constructs the Wayback Machine availability API URL.
// The optional timestamp parameter is appended when provided.
func (w *webWayback) buildWaybackURL(base string, in waybackInput) string {
	params := url.Values{}
	params.Set("url", in.URL)
	if in.Timestamp != "" {
		params.Set("timestamp", in.Timestamp)
	}
	return base + "/wayback/available?" + params.Encode()
}

// writeAudit records a single audit event for the call.
func (w *webWayback) writeAudit(_ context.Context, host string, statusCode int,
	outcome, reason string, start time.Time,
) {
	if w.deps.AuditWriter == nil {
		return
	}
	meta := map[string]string{
		"tool":        "web_wayback",
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
		"web_wayback invoked", meta)
	_ = w.deps.AuditWriter.Write(ev)
}

// parseWaybackInput parses the JSON payload and applies defensive guards.
// url is required and must start with http:// or https://.
// timestamp, when provided, must match the 14-digit pattern.
func parseWaybackInput(raw json.RawMessage) (waybackInput, error) {
	var in waybackInput
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			return waybackInput{}, err
		}
	}

	if in.URL == "" {
		return waybackInput{}, fmt.Errorf("url is required")
	}

	// Validate scheme: must be http:// or https://
	if len(in.URL) < 7 ||
		(in.URL[:7] != "http://" && (len(in.URL) < 8 || in.URL[:8] != "https://")) {
		return waybackInput{}, fmt.Errorf("url must start with http:// or https://")
	}

	if in.Timestamp != "" && !waybackTimestampPattern.MatchString(in.Timestamp) {
		return waybackInput{}, fmt.Errorf("timestamp %q must match pattern ^[0-9]{14}$", in.Timestamp)
	}

	return in, nil
}

// NewWaybackForTest constructs a web_wayback tool with an injected apiBaseBuilder
// so tests can redirect requests through an httptest.Server.
func NewWaybackForTest(deps *common.Deps, builder apiBaseBuilder) tools.Tool {
	if builder == nil {
		builder = func() string { return waybackProductionBase }
	}
	return &webWayback{deps: deps, apiBaseBuilder: builder}
}

// init registers web_wayback in the global web tools list.
func init() {
	RegisterWebTool(&webWayback{
		deps:           &common.Deps{},
		apiBaseBuilder: func() string { return waybackProductionBase },
	})
}
