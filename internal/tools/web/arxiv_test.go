package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modu-ai/goose/internal/permission"
	permstore "github.com/modu-ai/goose/internal/permission/store"
	"github.com/modu-ai/goose/internal/tools/web"
	"github.com/modu-ai/goose/internal/tools/web/common"
)

// arxivAtomFixture is a minimal Atom XML response with 3 papers, matching the
// arXiv API format. The entry GUIDs use the /abs/ URL pattern so pdf_url
// derivation can be tested.
const arxivAtomFixture = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:arxiv="http://arxiv.org/schemas/atom">
  <title>arXiv Query Results</title>
  <entry>
    <id>http://arxiv.org/abs/2301.00001v1</id>
    <title>Quantum Computing Basics</title>
    <summary>An introduction to quantum computing concepts.</summary>
    <published>2023-01-01T00:00:00Z</published>
    <author><name>Alice Smith</name></author>
    <author><name>Bob Jones</name></author>
    <category term="quant-ph" scheme="http://arxiv.org/schemas/atom"/>
  </entry>
  <entry>
    <id>http://arxiv.org/abs/2302.00002v2</id>
    <title>Deep Learning Survey</title>
    <summary>A comprehensive survey of deep learning methods.</summary>
    <published>2023-02-01T00:00:00Z</published>
    <author><name>Carol White</name></author>
    <category term="cs.LG" scheme="http://arxiv.org/schemas/atom"/>
  </entry>
  <entry>
    <id>http://arxiv.org/abs/2303.00003v1</id>
    <title>Graph Neural Networks</title>
    <summary>Graph neural networks for molecular prediction.</summary>
    <published>2023-03-01T00:00:00Z</published>
    <author><name>Dave Brown</name></author>
    <category term="cs.AI" scheme="http://arxiv.org/schemas/atom"/>
  </entry>
</feed>`

// startArxivMockServer starts an httptest server that captures request query
// parameters and returns the provided fixture body.
func startArxivMockServer(t *testing.T, fixture string) (*httptest.Server, *url.Values) {
	t.Helper()
	captured := &url.Values{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*captured = r.URL.Query()
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(fixture))
	}))
	t.Cleanup(srv.Close)
	return srv, captured
}

// newArxivForTest creates a web_arxiv tool with a mock server as the API base.
func newArxivForTest(deps *common.Deps, baseURL string) web.ArxivForTest {
	return web.NewArxivForTest(deps, func() string { return baseURL })
}

// TestArxiv_QuerySuccess verifies that a successful query returns all 3 papers
// with correctly mapped fields (id, title, authors, abstract, submitted,
// pdf_url, primary_category).
func TestArxiv_QuerySuccess(t *testing.T) {
	t.Parallel()

	srv, _ := startArxivMockServer(t, arxivAtomFixture)
	tool := newArxivForTest(&common.Deps{}, srv.URL)

	input := json.RawMessage(`{"query":"quantum","max_results":10}`)
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	require.True(t, res.OK, "expected ok=true, got error: %+v", res.Error)

	var data struct {
		Results []struct {
			ID              string   `json:"id"`
			Title           string   `json:"title"`
			Authors         []string `json:"authors"`
			Abstract        string   `json:"abstract"`
			Submitted       string   `json:"submitted"`
			PDFURL          string   `json:"pdf_url"`
			PrimaryCategory string   `json:"primary_category"`
		} `json:"results"`
	}
	require.NoError(t, json.Unmarshal(res.Data, &data))
	require.Equal(t, 3, len(data.Results), "expected 3 results from fixture")

	// Verify first result fields.
	r0 := data.Results[0]
	assert.Equal(t, "2301.00001v1", r0.ID)
	assert.Equal(t, "Quantum Computing Basics", r0.Title)
	assert.Equal(t, []string{"Alice Smith", "Bob Jones"}, r0.Authors)
	assert.Equal(t, "An introduction to quantum computing concepts.", r0.Abstract)
	assert.Equal(t, "2023-01-01T00:00:00Z", r0.Submitted)
	// pdf_url is derived by replacing /abs/ with /pdf/ in the GUID.
	assert.Equal(t, "http://arxiv.org/pdf/2301.00001v1", r0.PDFURL)
	assert.Equal(t, "quant-ph", r0.PrimaryCategory)

	// Verify second result.
	r1 := data.Results[1]
	assert.Equal(t, "2302.00002v2", r1.ID)
	assert.Equal(t, "Deep Learning Survey", r1.Title)
	assert.Equal(t, []string{"Carol White"}, r1.Authors)
	assert.Equal(t, "cs.LG", r1.PrimaryCategory)

	// Verify third result.
	r2 := data.Results[2]
	assert.Equal(t, "cs.AI", r2.PrimaryCategory)
}

// TestArxiv_SortBy verifies that the sortBy parameter is correctly translated
// to the arXiv API sortBy parameter values.
func TestArxiv_SortBy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		userValue string
		apiValue  string
	}{
		{userValue: "relevance", apiValue: "relevance"},
		{userValue: "submitted_date", apiValue: "submittedDate"},
	}

	for _, tc := range cases {
		t.Run(tc.userValue, func(t *testing.T) {
			t.Parallel()

			srv, captured := startArxivMockServer(t, arxivAtomFixture)
			tool := newArxivForTest(&common.Deps{}, srv.URL)

			input := json.RawMessage(`{"query":"test","sort_by":"` + tc.userValue + `"}`)
			result, err := tool.Call(context.Background(), input)
			require.NoError(t, err)

			var res common.Response
			require.NoError(t, json.Unmarshal(result.Content, &res))
			require.True(t, res.OK, "expected ok=true for sort_by=%q", tc.userValue)

			gotSortBy := captured.Get("sortBy")
			assert.Equal(t, tc.apiValue, gotSortBy,
				"sortBy param mismatch for user value %q", tc.userValue)
			assert.Equal(t, "descending", captured.Get("sortOrder"))
		})
	}
}

// TestArxiv_SchemaValidation verifies that invalid inputs are rejected before
// any network activity.
func TestArxiv_SchemaValidation(t *testing.T) {
	t.Parallel()

	tool := newArxivForTest(&common.Deps{}, "http://127.0.0.1:0")

	cases := []struct {
		name  string
		input string
	}{
		{
			name:  "empty_query",
			input: `{"query":""}`,
		},
		{
			name:  "max_results_too_large",
			input: `{"query":"test","max_results":101}`,
		},
		{
			name:  "invalid_sort_by",
			input: `{"query":"test","sort_by":"popularity"}`,
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
			assert.Equal(t, "invalid_input", res.Error.Code)
		})
	}
}

// TestArxiv_BlocklistPriority verifies that when export.arxiv.org is blocked,
// the tool returns host_blocked without making any HTTP request.
func TestArxiv_BlocklistPriority(t *testing.T) {
	t.Parallel()

	// Keep a count of actual HTTP calls to the mock server.
	serverHits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		serverHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	bl := common.NewBlocklist([]string{"export.arxiv.org"})
	deps := &common.Deps{Blocklist: bl}

	// The tool targets the real arxiv host for blocklist check regardless of
	// the injected base URL — the blocklist checks "export.arxiv.org".
	// We still inject the mock URL to prevent accidental real network calls.
	tool := web.NewArxivForTest(deps, func() string { return srv.URL })

	input := json.RawMessage(`{"query":"quantum"}`)
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	assert.False(t, res.OK)
	require.NotNil(t, res.Error)
	assert.Equal(t, "host_blocked", res.Error.Code)
	assert.Equal(t, 0, serverHits, "HTTP server must not be hit when host is blocked")
}

// TestArxiv_PermissionDenied verifies that a permission denial returns
// "permission_denied" without making any HTTP request.
func TestArxiv_PermissionDenied(t *testing.T) {
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
	tool := web.NewArxivForTest(deps, func() string { return srv.URL })

	input := json.RawMessage(`{"query":"quantum"}`)
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	assert.False(t, res.OK)
	require.NotNil(t, res.Error)
	assert.Equal(t, "permission_denied", res.Error.Code)
	assert.Equal(t, 0, serverHits, "HTTP server must not be hit when permission is denied")
}

// TestArxiv_RegisteredInWebTools verifies that web_arxiv is registered in the
// global web tools list at package init time.
func TestArxiv_RegisteredInWebTools(t *testing.T) {
	t.Parallel()
	names := web.RegisteredWebToolNamesForTest()
	assert.True(t, slices.Contains(names, "web_arxiv"),
		"web_arxiv not found in RegisteredWebToolNames: %v", names)
}

// TestArxiv_AuditWriter verifies that the audit writer is called on a
// successful arXiv query (covers the writeAudit ok-path and NoCategory branch).
func TestArxiv_AuditWriter(t *testing.T) {
	t.Parallel()

	srv, _ := startArxivMockServer(t, arxivAtomFixture)
	// noopAuditWriter is defined in http_test.go (same test package web_test).
	deps := &common.Deps{AuditWriter: noopAuditWriter{}}
	tool := web.NewArxivForTest(deps, func() string { return srv.URL })

	input := json.RawMessage(`{"query":"quantum"}`)
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	require.True(t, res.OK, "expected ok=true, got error: %+v", res.Error)
}

// TestArxiv_EmptyCategories verifies extractArxivPrimaryCategory returns ""
// when the item has no categories (covers the empty-slice branch).
func TestArxiv_EmptyCategories(t *testing.T) {
	t.Parallel()

	// Fixture with no category element.
	noCategoryFixture := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/abs/2301.99999v1</id>
    <title>No Category Paper</title>
    <summary>Abstract without category.</summary>
    <published>2023-01-15T00:00:00Z</published>
    <author><name>Eve Anon</name></author>
  </entry>
</feed>`

	srv, _ := startArxivMockServer(t, noCategoryFixture)
	tool := newArxivForTest(&common.Deps{}, srv.URL)

	input := json.RawMessage(`{"query":"test"}`)
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	require.True(t, res.OK, "expected ok=true")

	var data struct {
		Results []struct {
			PrimaryCategory string `json:"primary_category"`
		} `json:"results"`
	}
	require.NoError(t, json.Unmarshal(res.Data, &data))
	require.Equal(t, 1, len(data.Results))
	assert.Equal(t, "", data.Results[0].PrimaryCategory)
}
