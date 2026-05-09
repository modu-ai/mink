package web_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modu-ai/goose/internal/permission"
	permstore "github.com/modu-ai/goose/internal/permission/store"
	"github.com/modu-ai/goose/internal/tools/web"
	"github.com/modu-ai/goose/internal/tools/web/common"
)

// mockFeedFetcher is a test double for the web.FeedFetcher interface.
// It returns a pre-configured *gofeed.Feed for each registered URL.
// calls is incremented atomically to support concurrent errgroup invocations.
type mockFeedFetcher struct {
	feeds map[string]*gofeed.Feed
	calls atomic.Int64
}

func (m *mockFeedFetcher) Fetch(_ context.Context, feedURL string) (*gofeed.Feed, error) {
	m.calls.Add(1)
	if feed, ok := m.feeds[feedURL]; ok {
		return feed, nil
	}
	return &gofeed.Feed{Items: nil}, nil
}

// Ensure mockFeedFetcher satisfies the exported FeedFetcher interface.
var _ web.FeedFetcher = (*mockFeedFetcher)(nil)

// buildTime parses an RFC3339 string into *time.Time for use in feed fixtures.
func buildTime(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic("buildTime: invalid time string: " + s)
	}
	return &t
}

// makeItem constructs a minimal gofeed.Item with the given fields.
func makeItem(title, link, description, published string) *gofeed.Item {
	item := &gofeed.Item{
		Title:       title,
		Link:        link,
		Description: description,
	}
	if published != "" {
		item.PublishedParsed = buildTime(published)
	}
	return item
}

// TestRSS_MultiFeedSinceFilter verifies AC-WEB-014:
//   - Multiple feeds are merged into a single item list
//   - Items published before "since" are excluded
//   - Items are sorted in descending published order
//   - Each item carries source_feed = the originating feed URL
func TestRSS_MultiFeedSinceFilter(t *testing.T) {
	t.Parallel()

	feed1URL := "https://example.com/feed1.xml"
	feed2URL := "https://example.com/feed2.xml"

	// Feed 1: 3 items (2 after since, 1 before)
	feed1 := &gofeed.Feed{Items: []*gofeed.Item{
		makeItem("New A", "https://example.com/a", "Summary A", "2026-05-10T10:00:00Z"),
		makeItem("New B", "https://example.com/b", "Summary B", "2026-05-08T12:00:00Z"),
		makeItem("Old C", "https://example.com/c", "Summary C", "2026-04-30T09:00:00Z"), // before since
	}}
	// Feed 2: 2 items (1 after since, 1 before)
	feed2 := &gofeed.Feed{Items: []*gofeed.Item{
		makeItem("New D", "https://example.com/d", "Summary D", "2026-05-09T06:00:00Z"),
		makeItem("Old E", "https://example.com/e", "Summary E", "2026-04-28T14:00:00Z"), // before since
	}}

	fetcher := &mockFeedFetcher{feeds: map[string]*gofeed.Feed{
		feed1URL: feed1,
		feed2URL: feed2,
	}}

	deps := &common.Deps{}
	tool := web.NewRSSForTest(deps, fetcher)

	input := json.RawMessage(`{
		"feeds": ["https://example.com/feed1.xml","https://example.com/feed2.xml"],
		"max_items": 100,
		"since": "2026-05-01T00:00:00Z"
	}`)
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	require.True(t, res.OK, "expected ok=true, got error: %+v", res.Error)

	var data struct {
		Items []struct {
			Title      string `json:"title"`
			SourceFeed string `json:"source_feed"`
			Published  string `json:"published"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(res.Data, &data))

	// Expect 3 items: New A (2026-05-10), New D (2026-05-09), New B (2026-05-08).
	require.Equal(t, 3, len(data.Items), "expected 3 items after since filter, got %v", data.Items)

	// Descending order verification.
	assert.Equal(t, "New A", data.Items[0].Title)
	assert.Equal(t, "New D", data.Items[1].Title)
	assert.Equal(t, "New B", data.Items[2].Title)

	// source_feed attribution.
	assert.Equal(t, feed1URL, data.Items[0].SourceFeed)
	assert.Equal(t, feed2URL, data.Items[1].SourceFeed)
	assert.Equal(t, feed1URL, data.Items[2].SourceFeed)
}

// TestRSS_MaxItemsTruncate verifies that when 2 feeds produce more than
// max_items combined, the result is truncated to exactly max_items.
func TestRSS_MaxItemsTruncate(t *testing.T) {
	t.Parallel()

	// Build 15 items per feed.
	buildFeed := func(prefix string) *gofeed.Feed {
		items := make([]*gofeed.Item, 15)
		for i := range 15 {
			// Published in descending order so newer items appear first.
			pub := time.Now().Add(-time.Duration(i) * time.Hour).UTC().Format(time.RFC3339)
			items[i] = makeItem(
				prefix+"-item",
				"https://example.com/"+prefix,
				"desc",
				pub,
			)
		}
		return &gofeed.Feed{Items: items}
	}

	fetcher := &mockFeedFetcher{feeds: map[string]*gofeed.Feed{
		"https://feed1.example.com/rss": buildFeed("f1"),
		"https://feed2.example.com/rss": buildFeed("f2"),
	}}

	deps := &common.Deps{}
	tool := web.NewRSSForTest(deps, fetcher)

	input := json.RawMessage(`{
		"feeds": ["https://feed1.example.com/rss","https://feed2.example.com/rss"],
		"max_items": 10
	}`)
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	require.True(t, res.OK, "expected ok=true, got error: %+v", res.Error)

	var data struct {
		Items []json.RawMessage `json:"items"`
	}
	require.NoError(t, json.Unmarshal(res.Data, &data))
	assert.Equal(t, 10, len(data.Items), "expected exactly max_items=10 items, got %d", len(data.Items))
}

// TestRSS_SchemaValidation verifies that invalid inputs are rejected with
// "invalid_input" before any network activity.
func TestRSS_SchemaValidation(t *testing.T) {
	t.Parallel()

	// fetcher must never be called for schema-rejected inputs.
	fetcher := &mockFeedFetcher{feeds: map[string]*gofeed.Feed{}}
	deps := &common.Deps{}
	tool := web.NewRSSForTest(deps, fetcher)

	cases := []struct {
		name  string
		input string
	}{
		{
			name:  "empty_feeds",
			input: `{"feeds":[]}`,
		},
		{
			name:  "too_many_feeds",
			input: buildFeedsJSON(21),
		},
		{
			name:  "max_items_too_large",
			input: `{"feeds":["https://example.com/f"],"max_items":201}`,
		},
		{
			name:  "invalid_since_format",
			input: `{"feeds":["https://example.com/f"],"since":"not-a-datetime"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := tool.Call(context.Background(), json.RawMessage(tc.input))
			require.NoError(t, err)

			var res common.Response
			require.NoError(t, json.Unmarshal(result.Content, &res))
			assert.False(t, res.OK, "expected ok=false for input %q", tc.input)
			require.NotNil(t, res.Error)
			assert.Equal(t, "invalid_input", res.Error.Code, "expected invalid_input, got %q", res.Error.Code)
			assert.Equal(t, int64(0), fetcher.calls.Load(), "fetcher must not be called on schema rejection")
		})
	}
}

// buildFeedsJSON generates a JSON array of n feed URLs for schema testing.
func buildFeedsJSON(n int) string {
	feeds := make([]string, n)
	for i := range n {
		feeds[i] = `"https://example.com/feed` + string(rune('0'+i)) + `.xml"`
	}
	return `{"feeds":[` + strings.Join(feeds, ",") + `]}`
}

// TestRSS_BlocklistPriority verifies that when any feed's host appears in the
// blocklist, the tool returns host_blocked without calling Fetch.
func TestRSS_BlocklistPriority(t *testing.T) {
	t.Parallel()

	fetcher := &mockFeedFetcher{feeds: map[string]*gofeed.Feed{}}
	bl := common.NewBlocklist([]string{"blocked.example.com"})
	deps := &common.Deps{Blocklist: bl}
	tool := web.NewRSSForTest(deps, fetcher)

	// Second feed's host is blocked.
	input := json.RawMessage(`{
		"feeds": ["https://good.example.com/feed.xml","https://blocked.example.com/feed.xml"]
	}`)
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	assert.False(t, res.OK)
	require.NotNil(t, res.Error)
	assert.Equal(t, "host_blocked", res.Error.Code)
	assert.Equal(t, int64(0), fetcher.calls.Load(), "Fetch must not be called when host is blocked")
}

// TestRSS_PermissionDenied verifies that a permission denial returns
// "permission_denied" without calling Fetch.
func TestRSS_PermissionDenied(t *testing.T) {
	t.Parallel()

	fetcher := &mockFeedFetcher{feeds: map[string]*gofeed.Feed{}}

	// Build a manager with a deny-all confirmer and NO registered subject,
	// so Check returns Allow=false for any capability.
	store := permstore.NewMemoryStore()
	require.NoError(t, store.Open())
	mgr, err := permission.New(store, permission.DefaultDenyConfirmer{}, nil, nil, nil)
	require.NoError(t, err)

	deps := &common.Deps{PermMgr: mgr}
	tool := web.NewRSSForTest(deps, fetcher)

	input := json.RawMessage(`{"feeds":["https://example.com/feed.xml"]}`)
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	assert.False(t, res.OK)
	require.NotNil(t, res.Error)
	assert.Equal(t, "permission_denied", res.Error.Code)
	assert.Equal(t, int64(0), fetcher.calls.Load(), "Fetch must not be called when permission is denied")
}

// TestRSS_RegisteredInWebTools verifies that web_rss is registered in the
// global web tools list at package init time.
func TestRSS_RegisteredInWebTools(t *testing.T) {
	t.Parallel()
	names := web.RegisteredWebToolNamesForTest()
	assert.True(t, slices.Contains(names, "web_rss"),
		"web_rss not found in RegisteredWebToolNames: %v", names)
}

// TestRSS_AuditWriter verifies that audit events are written when a successful
// call completes (covers the writeAudit ok-path).
func TestRSS_AuditWriter(t *testing.T) {
	t.Parallel()

	feed := &gofeed.Feed{Items: []*gofeed.Item{
		makeItem("Title", "https://example.com/a", "Desc", "2026-05-10T00:00:00Z"),
	}}
	fetcher := &mockFeedFetcher{feeds: map[string]*gofeed.Feed{
		"https://example.com/feed.xml": feed,
	}}

	// noopAuditWriter is defined in http_test.go (same test package web_test).
	deps := &common.Deps{AuditWriter: noopAuditWriter{}}
	tool := web.NewRSSForTest(deps, fetcher)

	input := json.RawMessage(`{"feeds":["https://example.com/feed.xml"]}`)
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	require.True(t, res.OK, "expected ok=true, got error: %+v", res.Error)
}

// TestRSS_ItemWithContent verifies that summarizeItem falls back to Content
// when Description is empty (covers the content branch of summarizeItem).
func TestRSS_ItemWithContent(t *testing.T) {
	t.Parallel()

	feed := &gofeed.Feed{Items: []*gofeed.Item{
		{
			Title:   "Content Only",
			Link:    "https://example.com/content",
			Content: "This is the content of the item.",
			// Description intentionally empty.
		},
	}}
	fetcher := &mockFeedFetcher{feeds: map[string]*gofeed.Feed{
		"https://example.com/feed.xml": feed,
	}}

	deps := &common.Deps{}
	tool := web.NewRSSForTest(deps, fetcher)

	input := json.RawMessage(`{"feeds":["https://example.com/feed.xml"]}`)
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	require.True(t, res.OK, "expected ok=true")

	var data struct {
		Items []struct {
			Summary string `json:"summary"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(res.Data, &data))
	require.Equal(t, 1, len(data.Items))
	assert.Equal(t, "This is the content of the item.", data.Items[0].Summary)
}

// TestRSS_ItemWithUpdateParsed verifies that resolvePublished falls back to
// UpdatedParsed when PublishedParsed is nil.
func TestRSS_ItemWithUpdateParsed(t *testing.T) {
	t.Parallel()

	updated := buildTime("2026-05-05T12:00:00Z")
	feed := &gofeed.Feed{Items: []*gofeed.Item{
		{
			Title:         "Updated Item",
			Link:          "https://example.com/updated",
			Description:   "Description",
			UpdatedParsed: updated,
			// PublishedParsed intentionally nil.
		},
	}}
	fetcher := &mockFeedFetcher{feeds: map[string]*gofeed.Feed{
		"https://example.com/feed.xml": feed,
	}}

	deps := &common.Deps{}
	tool := web.NewRSSForTest(deps, fetcher)

	input := json.RawMessage(`{"feeds":["https://example.com/feed.xml"]}`)
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	require.True(t, res.OK, "expected ok=true")

	var data struct {
		Items []struct {
			Published string `json:"published"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(res.Data, &data))
	require.Equal(t, 1, len(data.Items))
	assert.Equal(t, "2026-05-05T12:00:00Z", data.Items[0].Published)
}
