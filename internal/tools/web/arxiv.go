package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/modu-ai/mink/internal/audit"
	"github.com/modu-ai/mink/internal/permission"
	"github.com/modu-ai/mink/internal/tools"
	"github.com/modu-ai/mink/internal/tools/web/common"
)

const (
	// arxivProductionBase is the standard arXiv API query endpoint.
	arxivProductionBase = "http://export.arxiv.org/api/query"

	// arxivHost is the hostname used for blocklist and permission checks.
	arxivHost = "export.arxiv.org"

	// arxivSortByRelevance is the sortBy parameter value for relevance ordering.
	arxivSortByRelevance = "relevance"

	// arxivSortByDate is the sortBy parameter value for submission date ordering.
	arxivSortByDate = "submittedDate"
)

// arxivSchema is the JSON Schema for the web_arxiv tool input.
// Per plan.md §4.4.
var arxivSchema = json.RawMessage(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["query"],
  "properties": {
    "query": {
      "type": "string",
      "minLength": 1,
      "maxLength": 500
    },
    "max_results": {
      "type": "integer",
      "minimum": 1,
      "maximum": 100,
      "default": 10
    },
    "sort_by": {
      "type": "string",
      "enum": ["relevance", "submitted_date"],
      "default": "relevance"
    }
  }
}`)

// arxivInput is the parsed input for a web_arxiv call.
type arxivInput struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
	SortBy     string `json:"sort_by"`
}

// arxivResult is a single paper entry in the successful response payload.
type arxivResult struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Authors         []string `json:"authors"`
	Abstract        string   `json:"abstract"`
	Submitted       string   `json:"submitted"`
	PDFURL          string   `json:"pdf_url"`
	PrimaryCategory string   `json:"primary_category"`
}

// arxivData is the successful response payload for web_arxiv.
type arxivData struct {
	Results []arxivResult `json:"results"`
}

// apiBaseBuilder returns the base URL used to construct the arXiv API request.
// Production always returns arxivProductionBase; tests inject a mock server URL.
type apiBaseBuilder func() string

// productionAPIBase returns the standard arXiv query endpoint.
func productionAPIBase() string { return arxivProductionBase }

// webArxiv implements the web_arxiv tool.
//
// @MX:ANCHOR: [AUTO] web_arxiv tool — arXiv Atom XML search via gofeed
// @MX:REASON: SPEC-GOOSE-TOOLS-WEB-001 — fan_in >= 3 (tests + bootstrap + executor)
type webArxiv struct {
	deps           *common.Deps
	apiBaseBuilder apiBaseBuilder
}

// Name returns the canonical tool name used in the Registry.
func (w *webArxiv) Name() string { return "web_arxiv" }

// Schema returns the JSON Schema that the Executor uses for input validation.
func (w *webArxiv) Schema() json.RawMessage { return arxivSchema }

// Scope returns ScopeShared — web_arxiv is available to all agent types.
func (w *webArxiv) Scope() tools.Scope { return tools.ScopeShared }

// Call executes the web_arxiv pipeline.
//
// Pipeline:
//  1. Parse + defensive schema guard
//  2. Build arXiv API URL from query parameters
//  3. Blocklist + permission gate (host = export.arxiv.org)
//  4. HTTP GET with User-Agent
//  5. Parse Atom XML response via gofeed
//  6. Map items to arxivResult structs
//  7. writeAudit + return
func (w *webArxiv) Call(ctx context.Context, raw json.RawMessage) (tools.ToolResult, error) {
	start := w.deps.Now()

	in, err := parseArxivInput(raw)
	if err != nil {
		return toToolResult(common.ErrResponse("invalid_input", err.Error(), false, 0, elapsed(start))), nil
	}

	// Build the API URL.
	target := w.buildArxivURL(in)

	// Use the fixed arxiv host for blocklist and permission checks.
	host := arxivHost

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

	// Outbound HTTP GET.
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if reqErr != nil {
		return toToolResult(common.ErrResponse("fetch_failed", reqErr.Error(), true, 0, elapsed(start))), nil
	}
	req.Header.Set("User-Agent", common.UserAgent())
	req.Header.Set("Accept", "application/atom+xml")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, fetchErr := client.Do(req)
	if fetchErr != nil {
		w.writeAudit(ctx, host, 0, "error", "fetch_failed", start)
		return toToolResult(common.ErrResponse("fetch_failed", fetchErr.Error(), true, 0, elapsed(start))), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		w.writeAudit(ctx, host, resp.StatusCode, "error", "fetch_failed", start)
		return toToolResult(common.ErrResponse("fetch_failed",
			fmt.Sprintf("arXiv API returned HTTP %d", resp.StatusCode),
			resp.StatusCode >= 500, 0, elapsed(start))), nil
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, common.MaxResponseBytes))
	if readErr != nil {
		w.writeAudit(ctx, host, resp.StatusCode, "error", "read_error", start)
		return toToolResult(common.ErrResponse("fetch_failed", readErr.Error(), true, 0, elapsed(start))), nil
	}

	// Parse the Atom XML response using gofeed.
	feed, parseErr := gofeed.NewParser().ParseString(string(body))
	if parseErr != nil {
		w.writeAudit(ctx, host, resp.StatusCode, "error", "decode_error", start)
		return toToolResult(common.ErrResponse("fetch_failed",
			fmt.Sprintf("parse arXiv response: %v", parseErr), false, 0, elapsed(start))), nil
	}

	// Map feed items to arxivResult.
	results := make([]arxivResult, 0, len(feed.Items))
	for _, item := range feed.Items {
		if item == nil {
			continue
		}
		results = append(results, mapArxivItem(item))
	}

	w.writeAudit(ctx, host, resp.StatusCode, "ok", "", start)
	out, marshalErr := common.OKResponse(arxivData{Results: results}, common.Metadata{
		CacheHit:   false,
		DurationMs: elapsed(start).DurationMs,
	})
	if marshalErr != nil {
		return toToolResult(common.ErrResponse("internal_error", marshalErr.Error(), false, 0, elapsed(start))), nil
	}
	return toToolResult(out), nil
}

// buildArxivURL constructs the full arXiv API query URL from parsed input.
// The sortBy field maps the user-facing "submitted_date" to the arXiv API's
// "submittedDate" parameter value.
func (w *webArxiv) buildArxivURL(in arxivInput) string {
	base := w.apiBaseBuilder()

	sortBy := arxivSortByRelevance
	if in.SortBy == "submitted_date" {
		sortBy = arxivSortByDate
	}

	params := url.Values{}
	params.Set("search_query", "all:"+in.Query)
	params.Set("start", "0")
	params.Set("max_results", fmt.Sprintf("%d", in.MaxResults))
	params.Set("sortBy", sortBy)
	params.Set("sortOrder", "descending")

	return base + "?" + params.Encode()
}

// mapArxivItem converts a gofeed.Item from the arXiv Atom feed into arxivResult.
func mapArxivItem(item *gofeed.Item) arxivResult {
	// Extract ID from GUID (arXiv sets GUID to the abs URL, e.g.
	// "http://arxiv.org/abs/2301.12345v1").
	id := extractArxivID(item)

	// Collect author names.
	authors := make([]string, 0, len(item.Authors))
	for _, a := range item.Authors {
		if a != nil && a.Name != "" {
			authors = append(authors, a.Name)
		}
	}

	// Abstract is in Description for the arXiv Atom feed.
	abstract := strings.TrimSpace(item.Description)

	// Submitted date from PublishedParsed.
	submitted := ""
	if item.PublishedParsed != nil {
		submitted = item.PublishedParsed.UTC().Format(time.RFC3339)
	}

	// PDF URL: arXiv includes a "related" link with type application/pdf.
	pdfURL := extractArxivPDFURL(item)

	// Primary category: arXiv stores it in Categories[0] or the
	// arxiv:primary_category extension.
	primaryCategory := extractArxivPrimaryCategory(item)

	return arxivResult{
		ID:              id,
		Title:           strings.TrimSpace(item.Title),
		Authors:         authors,
		Abstract:        abstract,
		Submitted:       submitted,
		PDFURL:          pdfURL,
		PrimaryCategory: primaryCategory,
	}
}

// extractArxivID derives the arXiv paper ID from GUID or link.
// GUID is typically "http://arxiv.org/abs/2301.12345v1"; we extract the
// last path segment (2301.12345v1) as the canonical ID.
func extractArxivID(item *gofeed.Item) string {
	raw := item.GUID
	if raw == "" {
		raw = item.Link
	}
	// Extract last path segment.
	if idx := strings.LastIndex(raw, "/"); idx >= 0 {
		seg := raw[idx+1:]
		if seg != "" {
			return seg
		}
	}
	return raw
}

// extractArxivPDFURL finds the PDF download link from item.Links.
// arXiv Atom feeds include a link with rel="related" and type="application/pdf".
// If not found, derive it from the abs URL by replacing /abs/ with /pdf/.
func extractArxivPDFURL(item *gofeed.Item) string {
	// gofeed populates item.Links with all href attributes of <link> elements.
	for _, l := range item.Links {
		if strings.Contains(l, "/pdf/") {
			return l
		}
	}
	// Derive from GUID or Link: replace /abs/ → /pdf/.
	if item.GUID != "" {
		if derived := strings.ReplaceAll(item.GUID, "/abs/", "/pdf/"); derived != item.GUID {
			return derived
		}
	}
	if item.Link != "" {
		if derived := strings.ReplaceAll(item.Link, "/abs/", "/pdf/"); derived != item.Link {
			return derived
		}
	}
	return ""
}

// extractArxivPrimaryCategory returns the primary category for the paper.
// gofeed exposes Categories as a string slice; we return the first one.
// For more precision, one can inspect Extensions["arxiv"]["primary_category"],
// but the first Categories entry is reliable for arXiv Atom feeds.
func extractArxivPrimaryCategory(item *gofeed.Item) string {
	if len(item.Categories) > 0 {
		return item.Categories[0]
	}
	return ""
}

// writeAudit records a single audit event for the web_arxiv call.
func (w *webArxiv) writeAudit(_ context.Context, host string, statusCode int,
	outcome, reason string, start time.Time,
) {
	if w.deps.AuditWriter == nil {
		return
	}
	meta := map[string]string{
		"tool":        "web_arxiv",
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
		"web_arxiv invoked", meta)
	_ = w.deps.AuditWriter.Write(ev)
}

// parseArxivInput parses the JSON payload, applies defaults, and enforces
// schema constraints so that direct Call() callers cannot bypass validation.
func parseArxivInput(raw json.RawMessage) (arxivInput, error) {
	in := arxivInput{
		MaxResults: 10,
		SortBy:     "relevance",
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			return arxivInput{}, err
		}
	}

	if in.MaxResults == 0 {
		in.MaxResults = 10
	}
	if in.SortBy == "" {
		in.SortBy = "relevance"
	}

	if l := len(in.Query); l < 1 || l > 500 {
		return arxivInput{}, fmt.Errorf("query length must be between 1 and 500 characters")
	}
	if in.MaxResults < 1 || in.MaxResults > 100 {
		return arxivInput{}, fmt.Errorf("max_results %d out of range [1, 100]", in.MaxResults)
	}
	if in.SortBy != "relevance" && in.SortBy != "submitted_date" {
		return arxivInput{}, fmt.Errorf("sort_by %q must be one of: relevance, submitted_date", in.SortBy)
	}

	return in, nil
}

// ArxivForTest is the test-accessible interface for web_arxiv.
type ArxivForTest interface {
	tools.Tool
}

// NewArxivForTest constructs a web_arxiv tool with an injected apiBaseBuilder,
// allowing tests to redirect requests through an httptest.Server.
func NewArxivForTest(deps *common.Deps, builder apiBaseBuilder) ArxivForTest {
	if builder == nil {
		builder = productionAPIBase
	}
	return &webArxiv{deps: deps, apiBaseBuilder: builder}
}

// init registers web_arxiv in the global web tools list.
func init() {
	RegisterWebTool(&webArxiv{deps: &common.Deps{}, apiBaseBuilder: productionAPIBase})
}
