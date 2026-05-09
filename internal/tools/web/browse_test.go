package web_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/modu-ai/goose/internal/tools/web"
	"github.com/modu-ai/goose/internal/tools/web/common"
)

// failingLauncher always returns ErrPlaywrightNotInstalled — simulates an
// environment where the Playwright driver / chromium binary is absent.
type failingLauncher struct {
	err error
}

func (f *failingLauncher) Launch(_ context.Context) (web.PlaywrightSession, error) {
	return nil, f.err
}

// stubSession is a no-op PlaywrightSession used by tests that need the
// launcher to succeed (verifying the post-launch stub branch).
type stubSession struct {
	closed bool
}

func (s *stubSession) Close() error {
	s.closed = true
	return nil
}

// successLauncher returns a stubSession on every call. Used to verify the
// "browse_not_implemented" stub path (M2b deferral marker for M2c).
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
	var found bool
	for _, n := range names {
		if n == "web_browse" {
			found = true
			break
		}
	}
	if !found {
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

	// successLauncher would normally drive the stub branch; the blocklist
	// must short-circuit before Launch is ever called.
	launcher := &successLauncher{}
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
	if launcher.session != nil {
		t.Errorf("Launch must not be invoked when blocklist matches")
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

// TestWebBrowse_StubBranchAfterSuccessfulLaunch verifies that when the
// launcher succeeds, M2b returns the documented "browse_not_implemented"
// response and the session's Close() is called (resource cleanup). This
// branch will be replaced by the production page-navigation pipeline in M2c.
func TestWebBrowse_StubBranchAfterSuccessfulLaunch(t *testing.T) {
	t.Parallel()

	launcher := &successLauncher{}
	tool := web.NewBrowseForTest(&common.Deps{}, launcher)

	input := json.RawMessage(`{"url":"https://example.com"}`)
	result, err := tool.Call(context.Background(), input)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	var res common.Response
	if err := json.Unmarshal(result.Content, &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if res.OK {
		t.Fatalf("expected ok=false (M2b stub), got success: %s", string(res.Data))
	}
	if res.Error == nil || res.Error.Code != "browse_not_implemented" {
		t.Errorf("expected error.code=browse_not_implemented, got %+v", res.Error)
	}
	if launcher.session == nil || !launcher.session.closed {
		t.Errorf("expected session.Close() to be invoked")
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
