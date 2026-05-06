package web

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modu-ai/goose/internal/audit"
	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"github.com/modu-ai/goose/internal/permission"
	"github.com/modu-ai/goose/internal/tools"
	"github.com/modu-ai/goose/internal/tools/web/common"
)

// webSearchSchema is the JSON Schema for the web_search tool input.
// Exact definition per plan.md §2.3 and contract.md DC-02.
// provider is an optional enum; omitting it triggers brave default.
// additionalProperties is false to prevent injection.
var webSearchSchema = json.RawMessage(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["query"],
  "properties": {
    "query": { "type": "string", "minLength": 1, "maxLength": 500 },
    "max_results": { "type": "integer", "minimum": 1, "maximum": 50, "default": 10 },
    "provider": { "type": "string", "enum": ["brave", "tavily", "exa"] }
  }
}`)

const (
	// braveAPIHost is the Brave Search API hostname used for permission + robots exemption.
	braveAPIHost = "api.search.brave.com"

	// searchCacheTTL is the default TTL for cached search results.
	searchCacheTTL = 24 * time.Hour
)

// webSearchInput is the parsed input for a web_search call.
type webSearchInput struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
	Provider   string `json:"provider"`
}

// searchResult is a single result entry in the web_search response.
type searchResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score,omitempty"`
}

// webSearchData is the successful response payload for web_search.
type webSearchData struct {
	Results []searchResult `json:"results"`
}

// WebConfig holds settings loaded from ~/.goose/config/web.yaml.
type WebConfig struct {
	// DefaultSearchProvider is the fallback provider when none is specified.
	// Defaults to "brave" when absent.
	DefaultSearchProvider string `json:"default_search_provider,omitempty"`
}

// LoadWebConfig loads the web config from path.
// Returns a zero WebConfig (brave default) when the file is missing or empty.
func LoadWebConfig(path string) (*WebConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &WebConfig{}, nil
		}
		return nil, fmt.Errorf("web config: read %s: %w", path, err)
	}
	// Minimal YAML: parse lines of the form "key: value".
	cfg := &WebConfig{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "default_search_provider:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "default_search_provider:"))
			cfg.DefaultSearchProvider = val
		}
	}
	return cfg, nil
}

// webSearch implements the web_search tool.
// It follows the 11-step call sequence defined in strategy.md §3.2.
//
// @MX:ANCHOR: [AUTO] web_search tool — steps: blocklist > robots-exempt > permission > ratelimit > cache > fetch > audit
// @MX:REASON: SPEC-GOOSE-TOOLS-WEB-001 AC-WEB-003/008/012/017/018 — fan_in >= 3 (tests + bootstrap + executor)
type webSearch struct {
	deps        *common.Deps
	providerURL string // override for testing; empty means use real Brave API
}

// NewWebSearch constructs a web_search Tool instance with the provided
// dependency injection container. providerURL, when non-empty, overrides the
// Brave API URL (used in tests to inject a mock server).
func NewWebSearch(deps *common.Deps, providerURL string) tools.Tool {
	return &webSearch{deps: deps, providerURL: providerURL}
}

// Name returns the canonical tool name used in the Registry.
func (s *webSearch) Name() string { return "web_search" }

// Schema returns the JSON Schema that the Executor uses for input validation.
func (s *webSearch) Schema() json.RawMessage { return webSearchSchema }

// Scope returns ScopeShared — web_search is available to all agent types.
func (s *webSearch) Scope() tools.Scope { return tools.ScopeShared }

// Call executes the 11-step web_search pipeline.
// Input must have been schema-validated by the Executor before Call is invoked.
//
// @MX:WARN: [AUTO] Calls permission.Manager.Check — Register must precede first Call
// @MX:REASON: ErrSubjectNotReady is returned as permission_denied if Register was not called (DC-14)
func (s *webSearch) Call(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	start := s.deps.Now()

	parsed, err := parseWebSearchInput(input)
	if err != nil {
		return toToolResult(common.ErrResponse("invalid_input", err.Error(), false, 0, elapsed(start))), nil
	}

	// Resolve provider (explicit > config default > "brave").
	provider := resolveProvider(parsed.Provider, s.deps.Cwd)

	// Determine the provider API host for permission + robots checks.
	apiHost := providerAPIHost(provider)

	// Step 1: Blocklist check — must happen before any permission gate.
	if s.deps.Blocklist != nil && s.deps.Blocklist.IsBlocked(apiHost) {
		s.writeAudit(ctx, apiHost, "GET", 0, false, "denied", "host_blocked", start)
		return toToolResult(common.ErrResponse("host_blocked",
			fmt.Sprintf("host %q is blocked by security policy", apiHost),
			false, 0, elapsed(start))), nil
	}

	// Step 2: robots.txt — provider API endpoints are exempt.
	// api.search.brave.com / api.tavily.com / api.exa.ai are all machine-to-machine
	// API endpoints; robots.txt does not apply (REQ-WEB-005 proviso).
	// We use IsAllowedExempt which skips the robots fetch entirely.
	// (RobotsChecker is nil in most unit tests that don't need robots.)

	// Step 3: Permission check (CapNet, scope=apiHost).
	if s.deps.PermMgr != nil {
		subjectID := s.deps.SubjectID(ctx)
		req := permission.PermissionRequest{
			SubjectID:   subjectID,
			SubjectType: permission.SubjectAgent,
			Capability:  permission.CapNet,
			Scope:       apiHost,
			RequestedAt: start,
		}
		dec, checkErr := s.deps.PermMgr.Check(ctx, req)
		if checkErr != nil || !dec.Allow {
			msg := "permission denied"
			if checkErr != nil {
				msg = checkErr.Error()
			}
			s.writeAudit(ctx, apiHost, "GET", 0, false, "denied", "permission_denied", start)
			return toToolResult(common.ErrResponse("permission_denied", msg, false, 0, elapsed(start))), nil
		}
	}

	// Step 4: Rate limit check.
	if s.deps.RateTracker != nil {
		now := s.deps.Now()
		state := s.deps.RateTracker.State(provider)
		if state.RequestsMin.UsagePct() >= 100 {
			retryAfter := int(math.Ceil(state.RequestsMin.RemainingSecondsNow(now)))
			if retryAfter < 0 {
				retryAfter = 0
			}
			s.writeAudit(ctx, apiHost, "GET", 0, false, "denied", "ratelimit_exhausted", start)
			return toToolResult(common.ErrResponse("ratelimit_exhausted",
				fmt.Sprintf("brave rate limit exhausted; retry after %d seconds", retryAfter),
				true, retryAfter, elapsed(start))), nil
		}
	}

	// Steps 5–11: cache lookup → fetch → cache write → ratelimit parse → audit.
	return s.fetchAndReturn(ctx, apiHost, provider, parsed, start)
}

// fetchAndReturn handles steps 5–11: cache lookup, outbound fetch (on miss),
// cache write, ratelimit tracker update, audit, and response construction.
func (s *webSearch) fetchAndReturn(
	ctx context.Context, apiHost, provider string, in webSearchInput, start time.Time,
) (tools.ToolResult, error) {
	cacheKey := searchCacheKey(in)
	cache := s.openCache()
	if cache != nil {
		defer func() { _ = cache.Close() }()

		// Step 5: Cache lookup.
		if cached, hit, cErr := cache.Get(cacheKey); cErr == nil && hit {
			meta := elapsed(start)
			meta.CacheHit = true
			var data webSearchData
			if json.Unmarshal(cached, &data) == nil {
				okResp, _ := common.OKResponse(data, meta)
				s.writeAudit(ctx, apiHost, "GET", 200, true, "ok", "", start)
				return toToolResult(okResp), nil
			}
		}
	}

	// Step 6: Outbound search call.
	results, respHeaders, fetchErr := s.doSearch(ctx, provider, in)
	if fetchErr != nil {
		s.writeAudit(ctx, apiHost, "GET", 0, false, "error", "fetch_failed", start)
		return toToolResult(common.ErrResponse("fetch_failed", fetchErr.Error(), true, 0, elapsed(start))), nil
	}

	data := webSearchData{Results: results}

	// Step 8: Cache write (best-effort).
	if cache != nil {
		if dataBytes, mErr := json.Marshal(data); mErr == nil {
			_ = cache.Set(cacheKey, dataBytes, searchCacheTTL)
		}
	}

	// Step 9: Update rate limit tracker.
	if s.deps.RateTracker != nil {
		_ = s.deps.RateTracker.Parse(provider, flattenHeadersToMap(respHeaders), s.deps.Now())
	}

	// Steps 10–11: Audit and return.
	meta := elapsed(start)
	okResp, mErr := common.OKResponse(data, meta)
	if mErr != nil {
		return toToolResult(common.ErrResponse("marshal_error", mErr.Error(), false, 0, meta)), nil
	}
	s.writeAudit(ctx, apiHost, "GET", 200, false, "ok", "", start)
	return toToolResult(okResp), nil
}

// --------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------

// openCache opens (or creates) the bbolt cache for web_search results.
// Returns nil when Cwd is empty or the open fails (cache is optional).
func (s *webSearch) openCache() *common.Cache {
	if s.deps.Cwd == "" {
		return nil
	}
	dir := filepath.Join(s.deps.Cwd, "web_search")
	_ = os.MkdirAll(dir, 0755)
	cache, err := common.OpenCache(filepath.Join(dir, "cache.db"), s.deps.Clock)
	if err != nil {
		return nil
	}
	return cache
}

// doSearch performs the outbound HTTP search request and returns parsed results
// along with the response headers (for ratelimit tracking).
func (s *webSearch) doSearch(ctx context.Context, provider string, in webSearchInput) ([]searchResult, http.Header, error) {
	switch provider {
	case "brave":
		return s.doBraveSearch(ctx, in)
	default:
		// For providers not yet implemented (tavily, exa), delegate to brave
		// as a fallback in M1. Future milestones add provider-specific paths.
		return s.doBraveSearch(ctx, in)
	}
}

// doBraveSearch calls the Brave Search API and returns normalised results.
func (s *webSearch) doBraveSearch(ctx context.Context, in webSearchInput) ([]searchResult, http.Header, error) {
	apiURL := s.braveAPIURL()

	maxResults := in.MaxResults
	if maxResults == 0 {
		maxResults = 10
	}

	reqURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, nil, fmt.Errorf("brave: invalid api url: %w", err)
	}
	q := reqURL.Query()
	q.Set("q", in.Query)
	q.Set("count", fmt.Sprintf("%d", maxResults))
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", common.UserAgent())
	if apiKey := os.Getenv("BRAVE_SEARCH_API_KEY"); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{
		CheckRedirect: common.NewRedirectGuard(5),
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("brave: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, resp.Header, fmt.Errorf("brave: read body: %w", err)
	}

	var parsed struct {
		Web struct {
			Results []struct {
				Title       string  `json:"title"`
				URL         string  `json:"url"`
				Description string  `json:"description"`
				Score       float64 `json:"score,omitempty"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, resp.Header, fmt.Errorf("brave: parse response: %w", err)
	}

	results := make([]searchResult, 0, len(parsed.Web.Results))
	for _, r := range parsed.Web.Results {
		results = append(results, searchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Description,
			Score:   r.Score,
		})
	}
	return results, resp.Header, nil
}

// braveAPIURL returns the Brave API base URL, using the test override when set.
func (s *webSearch) braveAPIURL() string {
	if s.providerURL != "" {
		return s.providerURL
	}
	return "https://api.search.brave.com/res/v1/web/search"
}

// writeAudit emits a tool.web.invoke audit event. Errors are log-only.
func (s *webSearch) writeAudit(
	_ context.Context, host, method string, statusCode int,
	cacheHit bool, outcome, reason string, start time.Time,
) {
	if s.deps.AuditWriter == nil {
		return
	}
	meta := map[string]string{
		"tool":        "web_search",
		"host":        host,
		"method":      method,
		"status_code": fmt.Sprintf("%d", statusCode),
		"cache_hit":   fmt.Sprintf("%t", cacheHit),
		"duration_ms": fmt.Sprintf("%d", s.deps.Now().Sub(start).Milliseconds()),
		"outcome":     outcome,
	}
	if reason != "" {
		meta["reason"] = reason
	}
	ev := audit.NewAuditEvent(s.deps.Now(), audit.EventTypeToolWebInvoke, audit.SeverityInfo,
		"web_search invoked", meta)
	_ = s.deps.AuditWriter.Write(ev)
}

// --------------------------------------------------------------------------
// Package-level helpers
// --------------------------------------------------------------------------

// parseWebSearchInput parses and defaults the input JSON.
func parseWebSearchInput(raw json.RawMessage) (webSearchInput, error) {
	in := webSearchInput{MaxResults: 10}
	if err := json.Unmarshal(raw, &in); err != nil {
		return webSearchInput{}, err
	}
	if in.MaxResults == 0 {
		in.MaxResults = 10
	}
	return in, nil
}

// resolveProvider determines the active provider from explicit input, config, or default.
func resolveProvider(explicit, cwd string) string {
	if explicit != "" {
		return explicit
	}
	// Try loading web.yaml for default_search_provider.
	paths := []string{
		filepath.Join(os.Getenv("HOME"), ".goose", "config", "web.yaml"),
	}
	if cwd != "" {
		paths = append([]string{filepath.Join(cwd, "web.yaml")}, paths...)
	}
	for _, p := range paths {
		if cfg, err := LoadWebConfig(p); err == nil && cfg.DefaultSearchProvider != "" {
			return cfg.DefaultSearchProvider
		}
	}
	return "brave"
}

// providerAPIHost returns the API hostname for a given provider.
func providerAPIHost(provider string) string {
	switch provider {
	case "tavily":
		return "api.tavily.com"
	case "exa":
		return "api.exa.ai"
	default:
		return braveAPIHost
	}
}

// searchCacheKey computes a deterministic cache key from the search input.
func searchCacheKey(in webSearchInput) string {
	h := sha256.New()
	b, _ := json.Marshal(in)
	_, _ = fmt.Fprintf(h, "web_search:%s", b)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// flattenHeadersToMap converts http.Header to map[string]string for ratelimit.Parse.
func flattenHeadersToMap(h http.Header) map[string]string {
	if h == nil {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, vs := range h {
		if len(vs) > 0 {
			out[k] = vs[0]
		}
	}
	return out
}

// init registers web_search in the global web tools list.
// The tool is constructed with a nil deps — callers must use web.WithDeps(deps)
// to inject the real dependency container before any tool.Call() invocation.
// ISSUE-01 resolution: Manager.Register must be called before any Call().
func init() {
	RegisterWebTool(&webSearch{deps: &common.Deps{}, providerURL: ""})
}

// Ensure ratelimit is used via RegisterBraveParser path.
var _ = ratelimit.BucketRequestsMin
