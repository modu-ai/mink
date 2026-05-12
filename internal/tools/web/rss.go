package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"golang.org/x/sync/errgroup"

	"github.com/modu-ai/mink/internal/audit"
	"github.com/modu-ai/mink/internal/permission"
	"github.com/modu-ai/mink/internal/tools"
	"github.com/modu-ai/mink/internal/tools/web/common"
)

// rssSchema is the JSON Schema for the web_rss tool input.
// Per plan.md §4.3 and AC-WEB-014.
var rssSchema = json.RawMessage(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["feeds"],
  "properties": {
    "feeds": {
      "type": "array",
      "items": {"type": "string", "format": "uri"},
      "minItems": 1,
      "maxItems": 20
    },
    "max_items": {
      "type": "integer",
      "minimum": 1,
      "maximum": 200,
      "default": 20
    },
    "since": {
      "type": "string",
      "format": "date-time"
    }
  }
}`)

// rssInput is the parsed input for a web_rss call.
type rssInput struct {
	Feeds    []string   `json:"feeds"`
	MaxItems int        `json:"max_items"`
	Since    *time.Time // parsed from JSON string
}

// rssItem is a single feed item in the successful response payload.
type rssItem struct {
	Title      string `json:"title"`
	Link       string `json:"link"`
	Published  string `json:"published"`
	SourceFeed string `json:"source_feed"`
	Summary    string `json:"summary"`
}

// rssData is the successful response payload for web_rss.
type rssData struct {
	Items []rssItem `json:"items"`
}

// FeedFetcher abstracts the gofeed parsing operation for testability.
// Production uses goFeedFetcher; tests inject a pre-loaded stub.
type FeedFetcher interface {
	Fetch(ctx context.Context, feedURL string) (*gofeed.Feed, error)
}

// goFeedFetcher is the production FeedFetcher backed by the gofeed library.
type goFeedFetcher struct{}

// Fetch parses the RSS/Atom feed at feedURL using gofeed.
func (f *goFeedFetcher) Fetch(ctx context.Context, feedURL string) (*gofeed.Feed, error) {
	return gofeed.NewParser().ParseURLWithContext(feedURL, ctx)
}

// webRSS implements the web_rss tool.
//
// @MX:ANCHOR: [AUTO] web_rss tool — parallel multi-feed fetch with since-filter and descending sort
// @MX:REASON: SPEC-GOOSE-TOOLS-WEB-001 AC-WEB-014 — fan_in >= 3 (tests + bootstrap + executor)
type webRSS struct {
	deps    *common.Deps
	fetcher FeedFetcher
}

// Name returns the canonical tool name used in the Registry.
func (w *webRSS) Name() string { return "web_rss" }

// Schema returns the JSON Schema that the Executor uses for input validation.
func (w *webRSS) Schema() json.RawMessage { return rssSchema }

// Scope returns ScopeShared — web_rss is available to all agent types.
func (w *webRSS) Scope() tools.Scope { return tools.ScopeShared }

// Call executes the web_rss pipeline.
//
// Pipeline:
//  1. Parse + defensive schema guard
//  2. Blocklist: check all feed hosts; deny if any is blocked
//  3. Permission gate: single confirm for scope = "rss:" + first feed host
//  4. Parallel fetch of all feeds via errgroup
//  5. since filter (published >= since)
//  6. Merge + descending published sort
//  7. Truncate to max_items
//  8. writeAudit + return
func (w *webRSS) Call(ctx context.Context, raw json.RawMessage) (tools.ToolResult, error) {
	start := w.deps.Now()

	in, err := parseRSSInput(raw)
	if err != nil {
		return toToolResult(common.ErrResponse("invalid_input", err.Error(), false, 0, elapsed(start))), nil
	}

	// Step 2: blocklist check — all feed hosts must pass before any fetch.
	for _, feedURL := range in.Feeds {
		host, hErr := extractURLHost(feedURL)
		if hErr != nil {
			return toToolResult(common.ErrResponse("invalid_input", hErr.Error(), false, 0, elapsed(start))), nil
		}
		if w.deps.Blocklist != nil && w.deps.Blocklist.IsBlocked(stripPort(host)) {
			w.writeAudit(ctx, host, 0, "denied", "host_blocked", start)
			return toToolResult(common.ErrResponse("host_blocked",
				fmt.Sprintf("host %q is blocked by security policy", host),
				false, 0, elapsed(start))), nil
		}
	}

	// Step 3: permission gate — one confirm using the first feed's host as scope.
	firstHost, _ := extractURLHost(in.Feeds[0])
	if w.deps.PermMgr != nil {
		subjectID := w.deps.SubjectID(ctx)
		scope := "rss:" + stripPort(firstHost)
		req := permission.PermissionRequest{
			SubjectID:   subjectID,
			SubjectType: permission.SubjectAgent,
			Capability:  permission.CapNet,
			Scope:       scope,
			RequestedAt: start,
		}
		dec, checkErr := w.deps.PermMgr.Check(ctx, req)
		if checkErr != nil || !dec.Allow {
			msg := "permission denied"
			if checkErr != nil {
				msg = checkErr.Error()
			}
			w.writeAudit(ctx, firstHost, 0, "denied", "permission_denied", start)
			return toToolResult(common.ErrResponse("permission_denied", msg, false, 0, elapsed(start))), nil
		}
	}

	// Step 4: parallel fetch via errgroup.
	type feedResult struct {
		feedURL string
		feed    *gofeed.Feed
	}
	results := make([]feedResult, len(in.Feeds))
	eg, egCtx := errgroup.WithContext(ctx)
	for i, feedURL := range in.Feeds {
		eg.Go(func() error {
			feed, fetchErr := w.fetcher.Fetch(egCtx, feedURL)
			if fetchErr != nil {
				return fmt.Errorf("fetch %q: %w", feedURL, fetchErr)
			}
			results[i] = feedResult{feedURL: feedURL, feed: feed}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		w.writeAudit(ctx, firstHost, 0, "error", "fetch_failed", start)
		return toToolResult(common.ErrResponse("fetch_failed", err.Error(), true, 0, elapsed(start))), nil
	}

	// Steps 5-7: filter, merge, sort, truncate.
	var allItems []rssItem
	for _, r := range results {
		if r.feed == nil {
			continue
		}
		for _, item := range r.feed.Items {
			if item == nil {
				continue
			}
			pub := resolvePublished(item)
			// since filter: include only items published at or after since.
			if in.Since != nil && pub != nil && pub.Before(*in.Since) {
				continue
			}
			pubStr := ""
			if pub != nil {
				pubStr = pub.UTC().Format(time.RFC3339)
			}
			allItems = append(allItems, rssItem{
				Title:      item.Title,
				Link:       item.Link,
				Published:  pubStr,
				SourceFeed: r.feedURL,
				Summary:    summarizeItem(item),
			})
		}
	}

	// Sort by published descending (newest first). Items without a published
	// date sort after items with one.
	sort.SliceStable(allItems, func(i, j int) bool {
		pi, pj := allItems[i].Published, allItems[j].Published
		if pi == "" && pj == "" {
			return false
		}
		if pi == "" {
			return false
		}
		if pj == "" {
			return true
		}
		return pi > pj // RFC3339 strings sort lexicographically (newest = largest)
	})

	// Truncate to max_items.
	if in.MaxItems > 0 && len(allItems) > in.MaxItems {
		allItems = allItems[:in.MaxItems]
	}

	if allItems == nil {
		allItems = []rssItem{}
	}

	w.writeAudit(ctx, firstHost, 0, "ok", "", start)
	out, marshalErr := common.OKResponse(rssData{Items: allItems}, common.Metadata{
		CacheHit:   false,
		DurationMs: elapsed(start).DurationMs,
	})
	if marshalErr != nil {
		return toToolResult(common.ErrResponse("internal_error", marshalErr.Error(), false, 0, elapsed(start))), nil
	}
	return toToolResult(out), nil
}

// resolvePublished returns the best available published timestamp for item,
// preferring PublishedParsed over UpdatedParsed.
func resolvePublished(item *gofeed.Item) *time.Time {
	if item.PublishedParsed != nil {
		return item.PublishedParsed
	}
	return item.UpdatedParsed
}

// summarizeItem returns a short summary from the feed item.
// Prefers Description; falls back to the first 500 characters of Content.
func summarizeItem(item *gofeed.Item) string {
	const maxSummaryChars = 500
	if item.Description != "" {
		return truncateText(item.Description, maxSummaryChars)
	}
	return truncateText(item.Content, maxSummaryChars)
}

// writeAudit records a single audit event for the web_rss call.
func (w *webRSS) writeAudit(_ context.Context, host string, statusCode int,
	outcome, reason string, start time.Time,
) {
	if w.deps.AuditWriter == nil {
		return
	}
	meta := map[string]string{
		"tool":        "web_rss",
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
		"web_rss invoked", meta)
	_ = w.deps.AuditWriter.Write(ev)
}

// parseRSSInput parses the JSON payload, applies defaults, and enforces schema
// constraints so that direct Call() callers cannot bypass validation.
func parseRSSInput(raw json.RawMessage) (rssInput, error) {
	// Use a raw struct to capture the "since" string before parsing.
	var wire struct {
		Feeds    []string `json:"feeds"`
		MaxItems int      `json:"max_items"`
		Since    string   `json:"since"`
	}
	wire.MaxItems = 20 // schema default
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &wire); err != nil {
			return rssInput{}, err
		}
	}

	// Validate feeds.
	if len(wire.Feeds) < 1 {
		return rssInput{}, fmt.Errorf("feeds must have at least 1 item")
	}
	if len(wire.Feeds) > 20 {
		return rssInput{}, fmt.Errorf("feeds must have at most 20 items")
	}
	// Validate each feed URL is non-empty and starts with http.
	for i, f := range wire.Feeds {
		if f == "" {
			return rssInput{}, fmt.Errorf("feeds[%d] is empty", i)
		}
		if !strings.HasPrefix(f, "http://") && !strings.HasPrefix(f, "https://") {
			return rssInput{}, fmt.Errorf("feeds[%d] must start with http:// or https://", i)
		}
	}
	// Deduplicate — preserve first occurrence order.
	seen := make(map[string]struct{}, len(wire.Feeds))
	unique := wire.Feeds[:0:len(wire.Feeds)]
	for _, f := range wire.Feeds {
		if _, ok := seen[f]; !ok {
			seen[f] = struct{}{}
			unique = append(unique, f)
		}
	}
	wire.Feeds = unique

	// Validate max_items.
	if wire.MaxItems == 0 {
		wire.MaxItems = 20
	}
	if wire.MaxItems < 1 || wire.MaxItems > 200 {
		return rssInput{}, fmt.Errorf("max_items %d out of range [1, 200]", wire.MaxItems)
	}

	// Parse since (optional).
	var since *time.Time
	if wire.Since != "" {
		t, err := time.Parse(time.RFC3339, wire.Since)
		if err != nil {
			return rssInput{}, fmt.Errorf("since %q is not a valid RFC3339 date-time: %w", wire.Since, err)
		}
		since = &t
	}

	return rssInput{
		Feeds:    wire.Feeds,
		MaxItems: wire.MaxItems,
		Since:    since,
	}, nil
}

// RSSForTest is the test-accessible interface for web_rss, allowing tests to
// call Call() without going through the Executor's permission layer.
type RSSForTest interface {
	tools.Tool
}

// NewRSSForTest constructs a web_rss tool with injected mock dependencies.
// It accepts any FeedFetcher implementation so tests can avoid real HTTP calls.
func NewRSSForTest(deps *common.Deps, fetcher FeedFetcher) RSSForTest {
	if fetcher == nil {
		fetcher = &goFeedFetcher{}
	}
	return &webRSS{deps: deps, fetcher: fetcher}
}

// init registers web_rss in the global web tools list.
func init() {
	RegisterWebTool(&webRSS{deps: &common.Deps{}, fetcher: &goFeedFetcher{}})
}
