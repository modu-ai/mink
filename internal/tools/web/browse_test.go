package web_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/tools/web"
	"github.com/modu-ai/mink/internal/tools/web/common"
)

// failingLauncher always returns ErrPlaywrightNotInstalled — simulates an
// environment where the Playwright driver / chromium binary is absent.
type failingLauncher struct {
	err error
}

func (f *failingLauncher) Launch(_ context.Context) (web.PlaywrightSession, error) {
	return nil, f.err
}

// stubSession is a configurable PlaywrightSession used by tests that need the
// launcher to succeed without a real browser.
type stubSession struct {
	closed    bool
	gotoErr   error
	gotoCalls []struct {
		URL       string
		TimeoutMs int
	}
	title        string
	content      string
	contentErr   error
	innerText    map[string]string
	innerTextErr error
}

func (s *stubSession) Goto(_ context.Context, rawURL string, timeoutMs int) error {
	s.gotoCalls = append(s.gotoCalls, struct {
		URL       string
		TimeoutMs int
	}{rawURL, timeoutMs})
	return s.gotoErr
}

func (s *stubSession) Title() (string, error) { return s.title, nil }

func (s *stubSession) Content() (string, error) { return s.content, s.contentErr }

func (s *stubSession) InnerText(selector string) (string, error) {
	if s.innerTextErr != nil {
		return "", s.innerTextErr
	}
	if v, ok := s.innerText[selector]; ok {
		return v, nil
	}
	return "", nil
}

func (s *stubSession) Close() error {
	s.closed = true
	return nil
}

// successLauncher returns the provided stubSession on every call.
type successLauncher struct {
	session *stubSession
}

func (s *successLauncher) Launch(_ context.Context) (web.PlaywrightSession, error) {
	if s.session == nil {
		s.session = &stubSession{}
	}
	return s.session, nil
}

// TestWebBrowse_PlaywrightNotInstalled verifies AC-SCHED-WEB-011: when the
// Playwright launcher reports ErrPlaywrightNotInstalled the tool must respond
// with `{ok: false, error.code: "playwright_not_installed"}` and never panic.
//
// The test injects a launcher that returns the canonical sentinel; we accept
// both the exported sentinel and any wrapped error that classifies to the
// same code via classifyLaunchError().
func TestWebBrowse_PlaywrightNotInstalled(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{name: "exact_sentinel", err: web.ErrPlaywrightNotInstalled},
		{name: "wrapped_sentinel", err: errors.New("please install the driver (v1.57.0) first")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tool := web.NewBrowseForTest(&common.Deps{}, &failingLauncher{err: tc.err})

			input := json.RawMessage(`{"url":"https://example.com"}`)

			// Defensive guard: panic must never escape Call.
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Call panicked with %v", r)
				}
			}()

			result, err := tool.Call(context.Background(), input)
			if err != nil {
				t.Fatalf("Call: %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected IsError=true on missing playwright, got success")
			}
			var res common.Response
			if err := json.Unmarshal(result.Content, &res); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if res.OK {
				t.Fatalf("expected ok=false, got %s", string(res.Data))
			}
			if res.Error == nil || res.Error.Code != "playwright_not_installed" {
				t.Errorf("expected error.code=playwright_not_installed, got %+v", res.Error)
			}
			if res.Error == nil || !strings.Contains(strings.ToLower(res.Error.Message), "playwright") {
				t.Errorf("error.message should mention playwright, got %q", res.Error.Message)
			}
		})
	}
}

// TestWebBrowse_SchemaValidation verifies the defensive parser guards reject
// malformed inputs at the parser level, mirroring the schema constraints.
func TestWebBrowse_SchemaValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "missing_url",
			input: `{}`,
			want:  "invalid_input",
		},
		{
			name:  "extract_invalid_enum",
			input: `{"url":"https://example.com","extract":"video"}`,
			want:  "invalid_input",
		},
		{
			name:  "timeout_below_minimum",
			input: `{"url":"https://example.com","timeout_ms":500}`,
			want:  "invalid_input",
		},
		{
			name:  "timeout_above_maximum",
			input: `{"url":"https://example.com","timeout_ms":120000}`,
			want:  "invalid_input",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tool := web.NewBrowseForTest(&common.Deps{}, &failingLauncher{err: web.ErrPlaywrightNotInstalled})
			result, err := tool.Call(context.Background(), json.RawMessage(tc.input))
			if err != nil {
				t.Fatalf("Call: %v", err)
			}
			var res common.Response
			if err := json.Unmarshal(result.Content, &res); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if res.OK {
				t.Fatalf("expected schema rejection for %q, got success", tc.input)
			}
			if res.Error == nil || res.Error.Code != tc.want {
				t.Errorf("expected error.code=%q, got %+v", tc.want, res.Error)
			}
		})
	}
}

// TestWebBrowse_RegisteredInWebTools verifies that web_browse is registered
// in the global web tools list at package init time.
func TestWebBrowse_RegisteredInWebTools(t *testing.T) {
	t.Parallel()
	names := web.RegisteredWebToolNamesForTest()
	if !slices.Contains(names, "web_browse") {
		t.Errorf("web_browse not in RegisteredWebToolNames: %v", names)
	}
}

// TestWebBrowse_BlocklistPriority verifies that the pre-permission blocklist
// guard rejects a request before reaching the Playwright launcher (parity
// with http_fetch / web_search blocklist semantics, AC-WEB-009).
func TestWebBrowse_BlocklistPriority(t *testing.T) {
	t.Parallel()

	bl := common.NewBlocklist([]string{"evil.example"})
	deps := &common.Deps{Blocklist: bl}

	// successLauncher would normally drive the navigation path; the blocklist
	// must short-circuit before Launch is ever called.
	stub := &stubSession{}
	launcher := &successLauncher{session: stub}
	tool := web.NewBrowseForTest(deps, launcher)

	input := json.RawMessage(`{"url":"https://evil.example/path"}`)
	result, err := tool.Call(context.Background(), input)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	var res common.Response
	if err := json.Unmarshal(result.Content, &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if res.OK {
		t.Fatalf("expected ok=false for blocklisted host")
	}
	if res.Error == nil || res.Error.Code != "host_blocked" {
		t.Errorf("expected error.code=host_blocked, got %+v", res.Error)
	}
	if len(stub.gotoCalls) > 0 {
		t.Errorf("Goto must not be invoked when blocklist matches")
	}
}

// TestWebBrowse_InvalidURL drives the extractURLHost error branch — the
// schema-level guard (`pattern: ^https?://`) is bypassed when callers invoke
// Call directly with a structurally valid scheme but malformed authority.
func TestWebBrowse_InvalidURL(t *testing.T) {
	t.Parallel()

	tool := web.NewBrowseForTest(&common.Deps{}, &failingLauncher{err: web.ErrPlaywrightNotInstalled})
	// "https:///path" parses but has no host — extractURLHost rejects it.
	input := json.RawMessage(`{"url":"https:///no-host"}`)
	result, err := tool.Call(context.Background(), input)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	var res common.Response
	if err := json.Unmarshal(result.Content, &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if res.OK {
		t.Fatalf("expected ok=false for hostless URL")
	}
	if res.Error == nil || res.Error.Code != "invalid_url" {
		t.Errorf("expected error.code=invalid_url, got %+v", res.Error)
	}
}

// TestClassifyLaunchError covers the launcher error → tool error code map.
func TestClassifyLaunchError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil", err: nil, want: ""},
		{name: "sentinel", err: web.ErrPlaywrightNotInstalled, want: "playwright_not_installed"},
		{name: "driver_missing_message", err: errors.New("please install the driver (v1.57.0) first"), want: "playwright_not_installed"},
		{name: "binary_not_found_message", err: errors.New("could not get driver instance: chromium binary not found"), want: "playwright_not_installed"},
		{name: "generic_io", err: errors.New("network unreachable"), want: "playwright_launch_failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := web.ClassifyLaunchErrorForTest(tc.err)
			if got != tc.want {
				t.Errorf("ClassifyLaunchError(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

// TestWebBrowse_ExtractText verifies that extract="text" calls InnerText("body")
// and returns the expected text payload with correct metadata fields.
func TestWebBrowse_ExtractText(t *testing.T) {
	t.Parallel()

	stub := &stubSession{
		title: "Test Page",
		innerText: map[string]string{
			"body": "hello world from page",
		},
	}
	launcher := &successLauncher{session: stub}
	tool := web.NewBrowseForTest(&common.Deps{}, launcher)

	input := json.RawMessage(`{"url":"https://example.com","extract":"text"}`)
	result, err := tool.Call(context.Background(), input)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	var res common.Response
	if err := json.Unmarshal(result.Content, &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !res.OK {
		t.Fatalf("expected ok=true, got error: %+v", res.Error)
	}

	// Parse the data payload.
	var data map[string]any
	if err := json.Unmarshal(res.Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}

	if data["content"] != "hello world from page" {
		t.Errorf("content=%q, want %q", data["content"], "hello world from page")
	}
	if data["content_type"] != "text" {
		t.Errorf("content_type=%q, want %q", data["content_type"], "text")
	}
	// word_count is float64 after JSON round-trip.
	if wc, ok := data["word_count"].(float64); !ok || wc != 4 {
		t.Errorf("word_count=%v, want 4", data["word_count"])
	}

	// Verify Goto was called exactly once with the right URL.
	if len(stub.gotoCalls) != 1 {
		t.Fatalf("expected 1 Goto call, got %d", len(stub.gotoCalls))
	}
	if stub.gotoCalls[0].URL != "https://example.com" {
		t.Errorf("Goto URL=%q, want %q", stub.gotoCalls[0].URL, "https://example.com")
	}
	// Default timeout_ms = 30000.
	if stub.gotoCalls[0].TimeoutMs != 30000 {
		t.Errorf("Goto TimeoutMs=%d, want 30000", stub.gotoCalls[0].TimeoutMs)
	}
}

// TestWebBrowse_ExtractHtml verifies that extract="html" returns the raw HTML
// content exactly as returned by session.Content().
func TestWebBrowse_ExtractHtml(t *testing.T) {
	t.Parallel()

	rawHTML := `<html><body>raw <b>html</b></body></html>`
	stub := &stubSession{
		title:   "HTML Page",
		content: rawHTML,
	}
	launcher := &successLauncher{session: stub}
	tool := web.NewBrowseForTest(&common.Deps{}, launcher)

	input := json.RawMessage(`{"url":"https://example.com","extract":"html"}`)
	result, err := tool.Call(context.Background(), input)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	var res common.Response
	if err := json.Unmarshal(result.Content, &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !res.OK {
		t.Fatalf("expected ok=true, got error: %+v", res.Error)
	}

	var data map[string]any
	if err := json.Unmarshal(res.Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}

	if data["content"] != rawHTML {
		t.Errorf("content=%q, want %q", data["content"], rawHTML)
	}
	if data["content_type"] != "html" {
		t.Errorf("content_type=%q, want %q", data["content_type"], "html")
	}
}

// TestWebBrowse_ExtractArticle verifies that extract="article" runs the raw
// HTML through go-readability and returns an article text payload.
func TestWebBrowse_ExtractArticle(t *testing.T) {
	t.Parallel()

	// Provide enough markup for readability to recognise an article body.
	articleHTML := `<html><head><title>Test Article</title></head>` +
		`<body><article>` +
		`<h1>Heading</h1>` +
		`<p>This is a test article body with multiple sentences. ` +
		`It exists so readability can extract it. ` +
		`The body has enough text for readability to recognise it as an article.</p>` +
		`</article></body></html>`

	stub := &stubSession{
		title:   "Test Article",
		content: articleHTML,
	}
	launcher := &successLauncher{session: stub}
	tool := web.NewBrowseForTest(&common.Deps{}, launcher)

	input := json.RawMessage(`{"url":"https://example.com/article","extract":"article"}`)
	result, err := tool.Call(context.Background(), input)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	var res common.Response
	if err := json.Unmarshal(result.Content, &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !res.OK {
		t.Fatalf("expected ok=true, got error: %+v", res.Error)
	}

	var data map[string]any
	if err := json.Unmarshal(res.Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}

	if data["content_type"] != "article" {
		t.Errorf("content_type=%q, want %q", data["content_type"], "article")
	}
	content, _ := data["content"].(string)
	if !strings.Contains(strings.ToLower(content), "test article body") {
		t.Errorf("article content missing expected text, got: %q", content)
	}
	if wc, ok := data["word_count"].(float64); !ok || wc <= 0 {
		t.Errorf("word_count=%v, want > 0", data["word_count"])
	}
}

// TestWebBrowse_NavigationFailure verifies that a Goto error produces the
// "navigation_failed" error code with retryable=true.
func TestWebBrowse_NavigationFailure(t *testing.T) {
	t.Parallel()

	stub := &stubSession{
		gotoErr: errors.New("net::ERR_NAME_NOT_RESOLVED"),
	}
	launcher := &successLauncher{session: stub}
	tool := web.NewBrowseForTest(&common.Deps{}, launcher)

	input := json.RawMessage(`{"url":"https://nonexistent.invalid/"}`)
	result, err := tool.Call(context.Background(), input)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	var res common.Response
	if err := json.Unmarshal(result.Content, &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if res.OK {
		t.Fatalf("expected ok=false on navigation failure")
	}
	if res.Error == nil || res.Error.Code != "navigation_failed" {
		t.Errorf("expected error.code=navigation_failed, got %+v", res.Error)
	}
	if !res.Error.Retryable {
		t.Errorf("expected Retryable=true for navigation failure")
	}
}

// TestWebBrowse_ExtractFailure verifies that a session.Content() error
// produces the "extract_failed" error code.
func TestWebBrowse_ExtractFailure(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		extract string
	}{
		{name: "html_content_error", extract: "html"},
		{name: "article_content_error", extract: "article"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubSession{
				contentErr: errors.New("page closed"),
			}
			launcher := &successLauncher{session: stub}
			tool := web.NewBrowseForTest(&common.Deps{}, launcher)

			input := json.RawMessage(`{"url":"https://example.com","extract":"` + tc.extract + `"}`)
			result, err := tool.Call(context.Background(), input)
			if err != nil {
				t.Fatalf("Call: %v", err)
			}

			var res common.Response
			if err := json.Unmarshal(result.Content, &res); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if res.OK {
				t.Fatalf("expected ok=false on extract failure")
			}
			if res.Error == nil || res.Error.Code != "extract_failed" {
				t.Errorf("expected error.code=extract_failed, got %+v", res.Error)
			}
		})
	}
}

// TestWebBrowse_ExtractText_InnerTextError verifies that a session.InnerText
// error produces the "extract_failed" error code for text extraction.
func TestWebBrowse_ExtractText_InnerTextError(t *testing.T) {
	t.Parallel()

	stub := &stubSession{
		innerTextErr: errors.New("element not found"),
	}
	launcher := &successLauncher{session: stub}
	tool := web.NewBrowseForTest(&common.Deps{}, launcher)

	input := json.RawMessage(`{"url":"https://example.com","extract":"text"}`)
	result, err := tool.Call(context.Background(), input)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	var res common.Response
	if err := json.Unmarshal(result.Content, &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if res.OK {
		t.Fatalf("expected ok=false on innerText error")
	}
	if res.Error == nil || res.Error.Code != "extract_failed" {
		t.Errorf("expected error.code=extract_failed, got %+v", res.Error)
	}
}
