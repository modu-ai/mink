package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/modu-ai/goose/internal/audit"
	"github.com/modu-ai/goose/internal/permission"
	"github.com/modu-ai/goose/internal/tools"
	"github.com/modu-ai/goose/internal/tools/web/common"
)

// httpFetchSchema is the JSON Schema for the http_fetch tool input.
// Exact definition per plan.md §2.4 and contract.md DC-02.
// Only GET and HEAD are allowed to prevent server-side effects.
// max_redirects is bounded [0, 10] and defaults to 5.
// additionalProperties is false to prevent injection.
var httpFetchSchema = json.RawMessage(`{
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
    "method": {
      "type": "string",
      "enum": ["GET", "HEAD"],
      "default": "GET"
    },
    "headers": {
      "type": "object",
      "additionalProperties": {"type": "string"},
      "maxProperties": 20
    },
    "max_redirects": {
      "type": "integer",
      "minimum": 0,
      "maximum": 10,
      "default": 5
    }
  }
}`)

// httpFetchInput is the parsed input for an http_fetch call.
type httpFetchInput struct {
	URL          string            `json:"url"`
	Method       string            `json:"method"`
	Headers      map[string]string `json:"headers"`
	MaxRedirects int               `json:"max_redirects"`
}

// httpFetchData is the successful response payload for http_fetch.
type httpFetchData struct {
	// StatusCode is the HTTP response status code.
	StatusCode int `json:"status_code"`
	// Headers contains the response headers (first value per name).
	Headers map[string]string `json:"headers"`
	// BodyText is the UTF-8 decoded response body (may be empty for HEAD).
	BodyText string `json:"body_text,omitempty"`
	// BodyTruncated is true if the body was cut at MaxResponseBytes.
	// For http_fetch it is always false (LimitedRead returns error on overflow).
	BodyTruncated bool `json:"body_truncated"`
}

// httpFetch implements the http_fetch tool.
// It follows the 11-step call sequence defined in strategy.md §3.2.
//
// @MX:ANCHOR: [AUTO] http_fetch tool — steps: blocklist > robots > permission > cache > fetch > audit
// @MX:REASON: SPEC-GOOSE-TOOLS-WEB-001 AC-WEB-005/006/009/010/012 — fan_in >= 3 (tests + bootstrap + executor)
type httpFetch struct {
	deps *common.Deps
}

// NewHTTPFetch constructs an http_fetch Tool instance with the provided
// dependency injection container. Returns a ready-to-register tools.Tool.
func NewHTTPFetch(deps *common.Deps) tools.Tool {
	return &httpFetch{deps: deps}
}

// Name returns the canonical tool name used in the Registry.
func (h *httpFetch) Name() string { return "http_fetch" }

// Schema returns the JSON Schema that the Executor uses for input validation.
func (h *httpFetch) Schema() json.RawMessage { return httpFetchSchema }

// Scope returns ScopeShared — http_fetch is available to all agent types.
func (h *httpFetch) Scope() tools.Scope { return tools.ScopeShared }

// Call executes the 11-step http_fetch pipeline.
// Input must have been schema-validated by the Executor before Call is invoked.
//
// @MX:WARN: [AUTO] Calls permission.Manager.Check — Register must precede first Call
// @MX:REASON: ErrSubjectNotReady is returned as permission_denied if Register was not called (DC-14)
func (h *httpFetch) Call(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	start := h.deps.Now()

	// Parse the pre-validated input.
	parsed, err := parseHTTPFetchInput(input)
	if err != nil {
		return toToolResult(common.ErrResponse("invalid_input", err.Error(), false, 0, elapsed(start))), nil
	}

	// Extract the hostname from the URL for blocklist and permission checks.
	host, err := extractURLHost(parsed.URL)
	if err != nil {
		return toToolResult(common.ErrResponse("invalid_url", err.Error(), false, 0, elapsed(start))), nil
	}
	urlPath := extractURLPath(parsed.URL)

	// Step 1: Blocklist check — must happen before any permission gate.
	// AC-WEB-009: blocklist pre-permission.
	// IsBlocked compares plain hostnames; strip the port so a port-suffixed
	// host (e.g. "evil.com:8080") cannot bypass glob suffix patterns.
	if h.deps.Blocklist != nil && h.deps.Blocklist.IsBlocked(stripPort(host)) {
		h.writeAudit(ctx, "http_fetch", host, parsed.Method, 0, false, "denied", "host_blocked", start)
		return toToolResult(common.ErrResponse("host_blocked",
			fmt.Sprintf("host %q is blocked by security policy", host),
			false, 0, elapsed(start))), nil
	}

	// Step 2: robots.txt check — before permission gate.
	// AC-WEB-004: robots.txt enforcement.
	// When deps.RobotsChecker is nil, the robots check is skipped (production bootstrap
	// must inject a real RobotsChecker; tests that don't need robots can leave it nil).
	if robotsChecker := h.deps.RobotsChecker; robotsChecker != nil {
		// Pass only scheme+host as baseURL so fetchRobots builds the correct robots.txt URL.
		baseURL := extractBaseURL(parsed.URL)
		allowed, robotsErr := robotsChecker.IsAllowed(baseURL, urlPath, common.UserAgent())
		if robotsErr == nil && !allowed {
			h.writeAudit(ctx, "http_fetch", host, parsed.Method, 0, false, "denied", "robots_disallow", start)
			return toToolResult(common.ErrResponse("robots_disallow",
				fmt.Sprintf("path %q is disallowed by %s robots.txt", urlPath, host),
				false, 0, elapsed(start))), nil
		}
	}

	// Step 3: Permission check (CapNet, scope=host).
	// DC-14: Register must be called before Check; ErrSubjectNotReady surfaces as permission_denied.
	// Bootstrap code is responsible for calling Manager.Register before any tool Call().
	if h.deps.PermMgr != nil {
		subjectID := h.deps.SubjectID(ctx)
		req := permission.PermissionRequest{
			SubjectID:   subjectID,
			SubjectType: permission.SubjectAgent,
			Capability:  permission.CapNet,
			Scope:       host,
			RequestedAt: start,
		}
		dec, checkErr := h.deps.PermMgr.Check(ctx, req)
		if checkErr != nil || !dec.Allow {
			msg := "permission denied"
			if checkErr != nil {
				msg = checkErr.Error()
			}
			h.writeAudit(ctx, "http_fetch", host, parsed.Method, 0, false, "denied", "permission_denied", start)
			return toToolResult(common.ErrResponse("permission_denied", msg, false, 0, elapsed(start))), nil
		}
	}

	// Steps 4 (ratelimit) and 5 (cache) are skipped for http_fetch — it is a
	// generic HTTP tool, not a provider-bound API. Ratelimit and cache are used
	// by web_search (Round C). Cache may be added to http_fetch in a later milestone.

	// Step 6: Outbound HTTP request.
	resp, fetchErr := h.doFetch(ctx, parsed)
	if fetchErr != nil {
		code, msg := classifyFetchError(fetchErr)
		h.writeAudit(ctx, "http_fetch", host, parsed.Method, 0, false, "error", code, start)
		return toToolResult(common.ErrResponse(code, msg, false, 0, elapsed(start))), nil
	}
	defer func() { _ = resp.Body.Close() }()

	// Step 7: Read body with size cap.
	body, readErr := common.LimitedRead(resp.Body)
	if readErr != nil {
		if errors.Is(readErr, common.ErrResponseTooLarge) {
			h.writeAudit(ctx, "http_fetch", host, parsed.Method, resp.StatusCode, false, "error", "response_too_large", start)
			return toToolResult(common.ErrResponse("response_too_large",
				"response body exceeds 10 MB limit", false, 0, elapsed(start))), nil
		}
		h.writeAudit(ctx, "http_fetch", host, parsed.Method, resp.StatusCode, false, "error", "read_error", start)
		return toToolResult(common.ErrResponse("fetch_failed", readErr.Error(), true, 0, elapsed(start))), nil
	}

	// Step 8: Normalize response to standard data shape.
	responseHeaders := flattenHeaders(resp.Header)
	data := httpFetchData{
		StatusCode:    resp.StatusCode,
		Headers:       responseHeaders,
		BodyTruncated: false,
	}
	// HEAD responses have no body by protocol.
	if parsed.Method != http.MethodHead {
		data.BodyText = string(body)
	}

	// Steps 9 (cache write), 10 (ratelimit parse) skipped for http_fetch.

	// Step 10/11: Audit and return.
	meta := elapsed(start)
	okResp, marshalErr := common.OKResponse(data, meta)
	if marshalErr != nil {
		return toToolResult(common.ErrResponse("marshal_error", marshalErr.Error(), false, 0, meta)), nil
	}
	h.writeAudit(ctx, "http_fetch", host, parsed.Method, resp.StatusCode, false, "ok", "", start)
	return toToolResult(okResp), nil
}

// doFetch performs the outbound HTTP request with the configured redirect guard
// and User-Agent header. Returns the raw *http.Response (caller closes Body).
func (h *httpFetch) doFetch(ctx context.Context, in httpFetchInput) (*http.Response, error) {
	maxRedirects := in.MaxRedirects
	client := &http.Client{
		CheckRedirect: common.NewRedirectGuard(maxRedirects),
	}

	req, err := http.NewRequestWithContext(ctx, in.Method, in.URL, nil)
	if err != nil {
		return nil, err
	}
	// Apply caller-supplied headers first, but skip reserved headers managed
	// by the tool itself. User-Agent is enforced last to guarantee
	// REQ-WEB-003 ("no anonymous outbound requests"); Host is dropped to
	// prevent host-header spoofing.
	for k, v := range in.Headers {
		if strings.EqualFold(k, "User-Agent") || strings.EqualFold(k, "Host") {
			continue
		}
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", common.UserAgent())

	return client.Do(req)
}

// writeAudit emits an audit event. Errors are logged only (never propagated).
func (h *httpFetch) writeAudit(
	_ context.Context, toolName, host, method string, statusCode int,
	cacheHit bool, outcome, reason string, start time.Time,
) {
	if h.deps.AuditWriter == nil {
		return
	}
	meta := map[string]string{
		"tool":        toolName,
		"host":        host,
		"method":      method,
		"status_code": fmt.Sprintf("%d", statusCode),
		"cache_hit":   fmt.Sprintf("%t", cacheHit),
		"duration_ms": fmt.Sprintf("%d", h.deps.Now().Sub(start).Milliseconds()),
		"outcome":     outcome,
	}
	if reason != "" {
		meta["reason"] = reason
	}
	ev := audit.NewAuditEvent(h.deps.Now(), audit.EventTypeToolWebInvoke, audit.SeverityInfo,
		"http_fetch invoked", meta)
	_ = h.deps.AuditWriter.Write(ev) // audit failures are log-only
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// parseHTTPFetchInput parses and defaults the input JSON.
// Assumes schema validation was already performed by the Executor.
func parseHTTPFetchInput(raw json.RawMessage) (httpFetchInput, error) {
	// Set defaults before unmarshaling.
	in := httpFetchInput{
		Method:       "GET",
		MaxRedirects: 5,
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return httpFetchInput{}, err
	}
	if in.Method == "" {
		in.Method = "GET"
	}
	return in, nil
}

// extractURLHost returns the host (host:port form) from rawURL.
//
// The host preserves the port because permission scope and audit logs are
// keyed off this exact value. Use stripPort before passing the host to
// Blocklist.IsBlocked — that lookup compares plain hostnames, and a port
// would silently bypass glob suffix patterns ("*.evil.com" stored as
// ".evil.com") and exact matches alike.
func extractURLHost(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("URL %q has no host", rawURL)
	}
	return u.Host, nil
}

// stripPort returns hostPort with any ":port" suffix removed. Used to feed
// Blocklist.IsBlocked a plain hostname so port-suffixed hosts cannot bypass
// glob/exact matches (e.g. "evil.com:8080" must still match "*.evil.com").
func stripPort(hostPort string) string {
	if h, _, err := net.SplitHostPort(hostPort); err == nil {
		return h
	}
	return hostPort
}

// extractBaseURL returns scheme + host of rawURL (no path/query/fragment).
func extractBaseURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	return u.Scheme + "://" + u.Host
}

// extractURLPath returns the path component of rawURL, defaulting to "/".
func extractURLPath(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Path == "" {
		return "/"
	}
	return u.Path
}

// elapsed returns a Metadata with duration computed from start.
func elapsed(start time.Time) common.Metadata {
	return common.Metadata{DurationMs: time.Since(start).Milliseconds()}
}

// toToolResult converts a common.Response to tools.ToolResult.
func toToolResult(resp common.Response) tools.ToolResult {
	b, _ := json.Marshal(resp)
	return tools.ToolResult{
		Content: b,
		IsError: !resp.OK,
	}
}

// classifyFetchError maps a net/http client error to an error code and message.
func classifyFetchError(err error) (code, msg string) {
	if err == nil {
		return "ok", ""
	}
	// Check for the redirect sentinel wrapped inside a *url.Error.
	if errors.Is(err, common.ErrTooManyRedirects) {
		return "too_many_redirects", err.Error()
	}
	// url.Error wraps the underlying error; unwrap to check sentinel.
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if errors.Is(urlErr.Err, common.ErrTooManyRedirects) {
			return "too_many_redirects", err.Error()
		}
		// Check if the inner error is our sentinel via string match as fallback.
		if strings.Contains(urlErr.Err.Error(), "too_many_redirects") {
			return "too_many_redirects", err.Error()
		}
	}
	return "fetch_failed", err.Error()
}

// flattenHeaders converts http.Header ([]string values) to map[string]string
// by taking the first value per header name.
func flattenHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, vs := range h {
		if len(vs) > 0 {
			out[k] = vs[0]
		}
	}
	return out
}

// init registers http_fetch in the global web tools list.
// The tool is constructed with zero-value Deps; callers must inject the real
// dependency container at bootstrap before any tool.Call() invocation.
// Mirrors web_search/init() at search.go.
func init() {
	RegisterWebTool(&httpFetch{deps: &common.Deps{}})
}
