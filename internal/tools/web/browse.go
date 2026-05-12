package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
	"github.com/modu-ai/mink/internal/audit"
	"github.com/modu-ai/mink/internal/permission"
	"github.com/modu-ai/mink/internal/tools"
	"github.com/modu-ai/mink/internal/tools/web/common"
)

// browseSchema is the JSON Schema for the web_browse tool input.
// Plan §3.3 + AC-WEB-011: extract enum {"text","article","html"},
// timeout_ms ∈ [1000, 60000], url required.
var browseSchema = json.RawMessage(`{
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
    "extract": {
      "type": "string",
      "enum": ["text", "article", "html"],
      "default": "article"
    },
    "timeout_ms": {
      "type": "integer",
      "minimum": 1000,
      "maximum": 60000,
      "default": 30000
    }
  }
}`)

// browseInput is the parsed input for web_browse.
type browseInput struct {
	URL       string `json:"url"`
	Extract   string `json:"extract"`
	TimeoutMS int    `json:"timeout_ms"`
}

// webBrowse implements the web_browse tool.
//
// @MX:ANCHOR: [AUTO] web_browse tool — Playwright launcher gate + extract pipeline
// @MX:REASON: SPEC-GOOSE-TOOLS-WEB-001 AC-WEB-011 — fan_in >= 3 (tests + bootstrap + executor)
type webBrowse struct {
	deps     *common.Deps
	launcher PlaywrightLauncher
}

// NewBrowse constructs a web_browse tool that uses the production
// Playwright launcher.
func NewBrowse(deps *common.Deps) tools.Tool {
	return &webBrowse{deps: deps, launcher: productionLauncher{}}
}

// NewBrowseForTest constructs a web_browse tool with a pluggable launcher.
// Used by tests to drive the AC-WEB-011 error path without a real browser.
func NewBrowseForTest(deps *common.Deps, launcher PlaywrightLauncher) tools.Tool {
	if launcher == nil {
		launcher = productionLauncher{}
	}
	return &webBrowse{deps: deps, launcher: launcher}
}

// Name returns the canonical tool name used in the Registry.
func (b *webBrowse) Name() string { return "web_browse" }

// Schema returns the JSON Schema that the Executor uses for input validation.
func (b *webBrowse) Schema() json.RawMessage { return browseSchema }

// Scope returns ScopeShared — web_browse is available to all agent types.
func (b *webBrowse) Scope() tools.Scope { return tools.ScopeShared }

// Call executes the web_browse pipeline. Input must have been schema-validated
// by the Executor before Call is invoked, but parseBrowseInput re-applies the
// critical guards so direct test invocation also fails closed.
//
// AC-WEB-011: when the launcher reports ErrPlaywrightNotInstalled, return
// `{ok: false, error.code: "playwright_not_installed"}` without panicking.
func (b *webBrowse) Call(ctx context.Context, raw json.RawMessage) (tools.ToolResult, error) {
	start := b.deps.Now()

	in, err := parseBrowseInput(raw)
	if err != nil {
		return toToolResult(common.ErrResponse("invalid_input", err.Error(), false, 0, elapsed(start))), nil
	}

	host, err := extractURLHost(in.URL)
	if err != nil {
		return toToolResult(common.ErrResponse("invalid_url", err.Error(), false, 0, elapsed(start))), nil
	}

	// Blocklist (pre-permission, AC-WEB-009 parity).
	if b.deps.Blocklist != nil && b.deps.Blocklist.IsBlocked(stripPort(host)) {
		b.writeAudit(ctx, host, "denied", "host_blocked", start)
		return toToolResult(common.ErrResponse("host_blocked",
			fmt.Sprintf("host %q is blocked by security policy", host),
			false, 0, elapsed(start))), nil
	}

	// Permission gate.
	if b.deps.PermMgr != nil {
		subjectID := b.deps.SubjectID(ctx)
		req := permission.PermissionRequest{
			SubjectID:   subjectID,
			SubjectType: permission.SubjectAgent,
			Capability:  permission.CapNet,
			Scope:       host,
			RequestedAt: start,
		}
		dec, checkErr := b.deps.PermMgr.Check(ctx, req)
		if checkErr != nil || !dec.Allow {
			msg := "permission denied"
			if checkErr != nil {
				msg = checkErr.Error()
			}
			b.writeAudit(ctx, host, "denied", "permission_denied", start)
			return toToolResult(common.ErrResponse("permission_denied", msg, false, 0, elapsed(start))), nil
		}
	}

	// Launch Playwright. AC-WEB-011: panic-free, classify driver-missing
	// errors into the canonical user-facing code.
	timeout := time.Duration(in.TimeoutMS) * time.Millisecond
	launchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	session, launchErr := b.launcher.Launch(launchCtx)
	if launchErr != nil {
		code := classifyLaunchError(launchErr)
		b.writeAudit(ctx, host, "error", code, start)
		retryable := code == "playwright_launch_failed"
		message := launchErr.Error()
		if code == "playwright_not_installed" {
			message = "playwright driver / browser binary not installed; run: playwright install chromium"
		}
		return toToolResult(common.ErrResponse(code, message, retryable, 0, elapsed(start))), nil
	}
	defer func() {
		_ = session.Close()
	}()

	// Navigate to the target URL.
	if err := session.Goto(launchCtx, in.URL, in.TimeoutMS); err != nil {
		b.writeAudit(ctx, host, "error", "navigation_failed", start)
		return toToolResult(common.ErrResponse(
			"navigation_failed",
			fmt.Sprintf("page navigation failed: %v", err),
			true, 0, elapsed(start),
		)), nil
	}

	title, _ := session.Title() // best-effort; ignore error

	// Extract content according to the extract enum.
	var (
		content     string
		contentType string
		extractErr  error
	)
	switch in.Extract {
	case "html":
		content, extractErr = session.Content()
		contentType = "html"
	case "text":
		content, extractErr = session.InnerText("body")
		contentType = "text"
	case "article":
		var raw string
		raw, extractErr = session.Content()
		if extractErr == nil {
			content, extractErr = extractArticle(raw, in.URL)
		}
		contentType = "article"
	}

	if extractErr != nil {
		b.writeAudit(ctx, host, "error", "extract_failed", start)
		return toToolResult(common.ErrResponse(
			"extract_failed",
			fmt.Sprintf("content extraction failed: %v", extractErr),
			false, 0, elapsed(start),
		)), nil
	}

	b.writeAudit(ctx, host, "ok", "", start)
	data := map[string]any{
		"title":        title,
		"url":          in.URL,
		"content":      content,
		"content_type": contentType,
		"word_count":   countWords(content),
	}
	okResp, marshalErr := common.OKResponse(data, elapsed(start))
	if marshalErr != nil {
		return toToolResult(common.ErrResponse("marshal_error", marshalErr.Error(), false, 0, elapsed(start))), nil
	}
	return toToolResult(okResp), nil
}

// writeAudit records a single audit event for the call.
func (b *webBrowse) writeAudit(_ context.Context, host string, outcome, reason string, start time.Time) {
	if b.deps.AuditWriter == nil {
		return
	}
	meta := map[string]string{
		"tool":        "web_browse",
		"host":        host,
		"method":      http.MethodGet,
		"status_code": "0",
		"cache_hit":   "false",
		"duration_ms": fmt.Sprintf("%d", b.deps.Now().Sub(start).Milliseconds()),
		"outcome":     outcome,
	}
	if reason != "" {
		meta["reason"] = reason
	}
	ev := audit.NewAuditEvent(b.deps.Now(), audit.EventTypeToolWebInvoke, audit.SeverityInfo,
		"web_browse invoked", meta)
	_ = b.deps.AuditWriter.Write(ev)
}

// parseBrowseInput parses the JSON payload, applies defaults, and enforces the
// schema's enum + range guards even when the Executor is bypassed.
func parseBrowseInput(raw json.RawMessage) (browseInput, error) {
	in := browseInput{
		Extract:   "article",
		TimeoutMS: 30000,
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			return browseInput{}, err
		}
	}
	if in.Extract == "" {
		in.Extract = "article"
	}
	if in.TimeoutMS == 0 {
		in.TimeoutMS = 30000
	}
	if strings.TrimSpace(in.URL) == "" {
		return browseInput{}, fmt.Errorf("url is required")
	}
	if !strings.HasPrefix(in.URL, "http://") && !strings.HasPrefix(in.URL, "https://") {
		return browseInput{}, fmt.Errorf("url scheme must be http or https")
	}
	switch in.Extract {
	case "text", "article", "html":
		// valid
	default:
		return browseInput{}, fmt.Errorf("extract must be one of text|article|html, got %q", in.Extract)
	}
	if in.TimeoutMS < 1000 || in.TimeoutMS > 60000 {
		return browseInput{}, fmt.Errorf("timeout_ms %d out of range [1000, 60000]", in.TimeoutMS)
	}
	return in, nil
}

// extractArticle parses raw HTML through go-readability and returns the
// extracted article body (TextContent). The base URL is used for resolving
// relative links inside the parser.
func extractArticle(rawHTML, pageURL string) (string, error) {
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}
	article, err := readability.FromReader(strings.NewReader(rawHTML), parsed)
	if err != nil {
		return "", fmt.Errorf("readability: %w", err)
	}
	return article.TextContent, nil
}

// countWords returns the whitespace-separated word count of s. Used for
// the metadata field on web_browse responses.
func countWords(s string) int {
	return len(strings.Fields(s))
}

// init registers web_browse in the global web tools list.
// Mirrors http_fetch / web_search / web_wikipedia registration patterns.
func init() {
	RegisterWebTool(&webBrowse{deps: &common.Deps{}, launcher: productionLauncher{}})
}
