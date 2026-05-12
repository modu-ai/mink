package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/modu-ai/mink/internal/audit"
	"github.com/modu-ai/mink/internal/permission"
	"github.com/modu-ai/mink/internal/tools"
	"github.com/modu-ai/mink/internal/tools/web/common"
)

// wikipediaSchema is the JSON Schema for the web_wikipedia tool input.
// Exact definition per plan.md §3.4 and AC-WEB-013.
// language follows the pattern ^[a-z]{2,3}$ to allow ISO 639-1/639-2 codes.
// extract_chars caps the size of the returned summary text.
var wikipediaSchema = json.RawMessage(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["query"],
  "properties": {
    "query": {
      "type": "string",
      "minLength": 1,
      "maxLength": 200
    },
    "language": {
      "type": "string",
      "pattern": "^[a-z]{2,3}$",
      "default": "en"
    },
    "extract_chars": {
      "type": "integer",
      "minimum": 100,
      "maximum": 10000,
      "default": 2000
    }
  }
}`)

// languagePattern mirrors the schema's language regex for early input
// rejection. Production deployments rely on the Executor's schema validator;
// the parser guards against direct Call() invocation in tests as well.
var languagePattern = regexp.MustCompile(`^[a-z]{2,3}$`)

// wikipediaInput is the parsed input for a web_wikipedia call.
type wikipediaInput struct {
	Query        string `json:"query"`
	Language     string `json:"language"`
	ExtractChars int    `json:"extract_chars"`
}

// wikipediaData is the successful response payload for web_wikipedia.
type wikipediaData struct {
	Title    string `json:"title"`
	URL      string `json:"url"`
	Summary  string `json:"summary"`
	Language string `json:"language"`
	Type     string `json:"type,omitempty"`
}

// wikipediaSummaryResponse mirrors the relevant fields of the Wikipedia REST
// summary endpoint (https://{lang}.wikipedia.org/api/rest_v1/page/summary/...).
// Other fields are intentionally ignored.
type wikipediaSummaryResponse struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	Extract     string `json:"extract"`
	ContentURLs struct {
		Desktop struct {
			Page string `json:"page"`
		} `json:"desktop"`
	} `json:"content_urls"`
}

// hostBuilder maps a language code to the base URL prefix used for the
// REST API request. Production hosts return "https://{lang}.wikipedia.org",
// while tests inject a httptest.Server URL with the language as the first
// path segment so a single test server can serve every variant.
type hostBuilder func(language string) string

// productionHostBuilder returns the standard Wikipedia REST endpoint host
// for the given language code.
func productionHostBuilder(language string) string {
	return fmt.Sprintf("https://%s.wikipedia.org", language)
}

// webWikipedia implements the web_wikipedia tool.
//
// @MX:ANCHOR: [AUTO] web_wikipedia tool — language-routed REST summary fetch
// @MX:REASON: SPEC-GOOSE-TOOLS-WEB-001 AC-WEB-013 — fan_in >= 3 (tests + bootstrap + executor)
type webWikipedia struct {
	deps        *common.Deps
	hostBuilder hostBuilder
}

// NewWikipedia constructs a web_wikipedia tool that resolves URLs against
// the production Wikipedia REST host. Bootstrap code uses this constructor.
func NewWikipedia(deps *common.Deps) tools.Tool {
	return &webWikipedia{deps: deps, hostBuilder: productionHostBuilder}
}

// NewWikipediaForTest constructs a web_wikipedia tool with a pluggable
// hostBuilder so tests can route requests through an httptest.Server. It is
// not exported through the package init() registration path.
func NewWikipediaForTest(deps *common.Deps, hb hostBuilder) tools.Tool {
	if hb == nil {
		hb = productionHostBuilder
	}
	return &webWikipedia{deps: deps, hostBuilder: hb}
}

// Name returns the canonical tool name used in the Registry.
func (w *webWikipedia) Name() string { return "web_wikipedia" }

// Schema returns the JSON Schema that the Executor uses for input validation.
func (w *webWikipedia) Schema() json.RawMessage { return wikipediaSchema }

// Scope returns ScopeShared — web_wikipedia is available to all agent types.
func (w *webWikipedia) Scope() tools.Scope { return tools.ScopeShared }

// Call executes the web_wikipedia pipeline. The input must have been
// schema-validated by the Executor before Call is invoked, but the parser
// re-applies critical guards so direct test invocation also fails closed.
//
// Pipeline:
//  1. Parse + defensive schema guard
//  2. Build target URL via hostBuilder + URL-encoded title
//  3. Blocklist + permission gate (CapNet, scope=host)
//  4. Outbound GET with User-Agent
//  5. Decode JSON summary, project to wikipediaData, audit + return
func (w *webWikipedia) Call(ctx context.Context, raw json.RawMessage) (tools.ToolResult, error) {
	start := w.deps.Now()

	in, err := parseWikipediaInput(raw)
	if err != nil {
		return toToolResult(common.ErrResponse("invalid_input", err.Error(), false, 0, elapsed(start))), nil
	}

	target, err := w.buildSummaryURL(in)
	if err != nil {
		return toToolResult(common.ErrResponse("invalid_input", err.Error(), false, 0, elapsed(start))), nil
	}

	host, err := extractURLHost(target)
	if err != nil {
		return toToolResult(common.ErrResponse("invalid_url", err.Error(), false, 0, elapsed(start))), nil
	}

	// Blocklist (pre-permission, AC-WEB-009).
	if w.deps.Blocklist != nil && w.deps.Blocklist.IsBlocked(stripPort(host)) {
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

	resp, fetchErr := w.doFetch(ctx, target)
	if fetchErr != nil {
		w.writeAudit(ctx, host, 0, "error", "fetch_failed", start)
		return toToolResult(common.ErrResponse("fetch_failed", fetchErr.Error(), true, 0, elapsed(start))), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		w.writeAudit(ctx, host, resp.StatusCode, "error", "fetch_failed", start)
		return toToolResult(common.ErrResponse("fetch_failed",
			fmt.Sprintf("Wikipedia returned HTTP %d", resp.StatusCode),
			resp.StatusCode >= 500, 0, elapsed(start))), nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, common.MaxResponseBytes))
	if err != nil {
		w.writeAudit(ctx, host, resp.StatusCode, "error", "read_error", start)
		return toToolResult(common.ErrResponse("fetch_failed", err.Error(), true, 0, elapsed(start))), nil
	}

	var summary wikipediaSummaryResponse
	if err := json.Unmarshal(body, &summary); err != nil {
		w.writeAudit(ctx, host, resp.StatusCode, "error", "decode_error", start)
		return toToolResult(common.ErrResponse("fetch_failed",
			fmt.Sprintf("decode wikipedia response: %v", err), false, 0, elapsed(start))), nil
	}

	data := wikipediaData{
		Title:    summary.Title,
		URL:      summary.ContentURLs.Desktop.Page,
		Summary:  truncateText(summary.Extract, in.ExtractChars),
		Language: in.Language,
		Type:     summary.Type,
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

// doFetch performs the outbound GET with the standard User-Agent header.
func (w *webWikipedia) doFetch(ctx context.Context, target string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", common.UserAgent())
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	return client.Do(req)
}

// buildSummaryURL composes the Wikipedia REST summary URL for the given
// language and query, percent-encoding the title once. The hostBuilder allows
// tests to redirect requests through an httptest.Server.
func (w *webWikipedia) buildSummaryURL(in wikipediaInput) (string, error) {
	base := strings.TrimRight(w.hostBuilder(in.Language), "/")
	if base == "" {
		return "", fmt.Errorf("empty host for language %q", in.Language)
	}
	title := strings.TrimSpace(in.Query)
	if title == "" {
		return "", fmt.Errorf("empty query")
	}
	// path segment is RFC 3986 percent-encoded; url.PathEscape is the right
	// helper for path segments (PathEscape leaves %2F unsafe, but Wikipedia
	// titles never legitimately contain a forward slash).
	encoded := url.PathEscape(title)
	return base + "/api/rest_v1/page/summary/" + encoded, nil
}

// writeAudit records a single audit event for the call.
func (w *webWikipedia) writeAudit(_ context.Context, host string, statusCode int,
	outcome, reason string, start time.Time,
) {
	if w.deps.AuditWriter == nil {
		return
	}
	meta := map[string]string{
		"tool":        "web_wikipedia",
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
		"web_wikipedia invoked", meta)
	_ = w.deps.AuditWriter.Write(ev)
}

// parseWikipediaInput parses the JSON payload, applies defaults, and enforces
// the schema's language pattern + query length so that direct Call() callers
// (which skip the Executor) cannot bypass validation.
func parseWikipediaInput(raw json.RawMessage) (wikipediaInput, error) {
	in := wikipediaInput{
		Language:     "en",
		ExtractChars: 2000,
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			return wikipediaInput{}, err
		}
	}
	if in.Language == "" {
		in.Language = "en"
	}
	if in.ExtractChars == 0 {
		in.ExtractChars = 2000
	}
	if l := len(in.Query); l < 1 || l > 200 {
		return wikipediaInput{}, fmt.Errorf("query length must be between 1 and 200 characters")
	}
	if !languagePattern.MatchString(in.Language) {
		return wikipediaInput{}, fmt.Errorf("language %q does not match pattern ^[a-z]{2,3}$", in.Language)
	}
	if in.ExtractChars < 100 || in.ExtractChars > 10000 {
		return wikipediaInput{}, fmt.Errorf("extract_chars %d out of range [100, 10000]", in.ExtractChars)
	}
	return in, nil
}

// truncateText returns text capped at maxRunes runes (not bytes) so multibyte
// characters are not split mid-codepoint. When maxRunes is non-positive, the
// original string is returned unchanged.
func truncateText(text string, maxRunes int) string {
	if maxRunes <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes])
}

// init registers web_wikipedia in the global web tools list.
// Mirrors http_fetch and web_search registration patterns.
func init() {
	RegisterWebTool(&webWikipedia{deps: &common.Deps{}, hostBuilder: productionHostBuilder})
}
